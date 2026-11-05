"""The job loop (contract section 8), crash recovery, heartbeat and exit rules.

Runs in the foreground of the window the desktop shortcut opens. Exits when:
  * no job has run for QC_AGENT_IDLE_MINUTES (owner's rule: it shouldn't run most of the time);
  * the unit, once seen, disappears (switched off or unplugged), or a job finds it gone;
  * the window is closed or Ctrl+C is pressed.
It writes a signed heartbeat every 60 s, on every state change, and a final one with
state "exiting".
"""
from __future__ import annotations

import datetime as _dt
import logging
import threading
import traceback

from . import AGENT_VERSION, contract, jobs
from .device import SETLIST_NAME
from .ledger import JobLedger, OwnedSlots
from .s3 import JobStore, S3Error

log = logging.getLogger("qc_agent")

HEARTBEAT_S = 60
POLL_FAST_S = 5            # for FAST_WINDOW_S after start or after a job (contract 14: not cost-constrained)
POLL_SLOW_S = 15
FAST_WINDOW_S = 5 * 60
MAX_SCAN = 50              # well-formed pending objects read per poll; a spam flood can't stall the loop for long
MAX_LIST = 10_000          # keys listed per poll (up to 10 ListObjectsV2 pages; normally one page with a few
                           # keys). Malformed keys are skipped, not deleted (contract step 1), so this bounds how
                           # many a bucket-token holder must plant to crowd real jobs out; that holder can delete
                           # real jobs outright anyway (contract section 15).

DELETE_OUTCOMES = frozenset({"too_large", "bad_envelope", "unknown_kid", "bad_signature",
                             "bad_payload", "misrouted", "replay"})

EXIT_OK, EXIT_UNIT_GONE = 0, 3


def utcnow():
    return _dt.datetime.now(_dt.timezone.utc).replace(microsecond=0)


class Agent:
    def __init__(self, config, store: JobStore, device, catalog, home, now_fn=utcnow, sleep=None):
        self.config, self.store, self.device, self.catalog = config, store, device, catalog
        self.home = home
        self.now = now_fn
        self.stop = threading.Event()
        self._sleep = sleep or (lambda s: self.stop.wait(s))
        self.ledger = JobLedger(home / "executed_jobs.jsonl")
        self.owned = OwnedSlots(home / "owned_slots.jsonl")
        self.ctx = jobs.JobContext(config, catalog, device, self.owned, now_fn, sleep=self._job_sleep)
        self.started_at = self.now()
        self.last_activity = self.started_at
        self.state = "idle"
        self.current_job = None
        self.last_job = None
        self.unit_present = None
        self.unit_seen = False
        self.exit_reason = None
        self.exit_code = EXIT_OK
        self._unuploaded: dict[str, bytes] = {}
        self._logged_bad_keys: set[str] = set()
        self._hb_lock = threading.Lock()
        self._hb_wake = threading.Event()
        self._hb_thread = None

    def _job_sleep(self, s):
        # Device pacing inside a job is not cut short by a stop request: a half-built grid
        # is worse than a few seconds' delay. The job finishes; the loop then exits.
        threading.Event().wait(s)

    # ---- timing ----

    def idle_exit_at(self):
        return self.last_activity + _dt.timedelta(minutes=self.config.idle_minutes)

    def poll_interval(self):
        recent = (self.now() - self.last_activity).total_seconds() < FAST_WINDOW_S
        return POLL_FAST_S if recent else POLL_SLOW_S

    # ---- signed objects ----

    def _seal(self, tag, payload):
        return contract.seal_payload(self.config.signing_key, tag, payload)

    def heartbeat_payload(self, state=None):
        fw = self.ctx.firmware_seen
        return {
            "v": 1, "type": "qc2-device-heartbeat", "bucket": self.config.bucket,
            "agent_version": AGENT_VERSION,
            "started_at": contract.format_time(self.started_at),
            "written_at": contract.format_time(self.now()),
            "idle_exit_at": contract.format_time(self.idle_exit_at()),
            "poll_interval_s": self.poll_interval(),
            "state": state or self.state,
            "current_job_id": self.current_job,
            "last_job": self.last_job,
            "unit": {"present": self.unit_present,
                     "firmware": jobs._fw(fw[0]) if fw else None,
                     "firmware_seen_at": contract.format_time(fw[1]) if fw else None},
            "expected_firmware": self.config.expected_firmware,
            "allowed_slots": self.config.allowed_slots.labels(),
            "setlist": SETLIST_NAME,
        }

    def write_heartbeat(self, state=None):
        with self._hb_lock:
            data = self._seal(contract.TAG_HEARTBEAT, self.heartbeat_payload(state))
            if len(data) > contract.MAX_HEARTBEAT_BYTES:  # cannot happen with the fields above; never send one QC-2 would refuse
                log.error("heartbeat too large; not written")
                return False
            try:
                self.store.put(contract.HEARTBEAT_KEY, data)
                return True
            except S3Error as e:
                log.warning("heartbeat not written: %s", e)
                return False

    def _heartbeat_loop(self):
        while not self.stop.is_set():
            self._hb_wake.wait(HEARTBEAT_S)
            self._hb_wake.clear()
            if self.stop.is_set():
                break
            self.write_heartbeat()

    def kick_heartbeat(self):
        if self._hb_thread is None:
            self.write_heartbeat()
        else:
            self._hb_wake.set()

    def result_payload(self, job_id, kind, outcome, started, finished):
        msg = outcome.message[:500]
        return {
            "v": 1, "type": "qc2-device-result", "job_id": job_id, "bucket": self.config.bucket, "kind": kind,
            "status": outcome.status, "reason_code": outcome.reason_code, "message": msg,
            "started_at": contract.format_time(started), "finished_at": contract.format_time(finished),
            "agent_version": AGENT_VERSION, "firmware": outcome.firmware,
            "push": outcome.push, "slots": outcome.slots, "readback": outcome.readback,
        }

    # ---- results ----

    def _publish(self, job_id, data):
        """Upload a result already recorded in the ledger, then drop the pending job."""
        try:
            self.store.put(contract.done_key(job_id), data)
        except S3Error as e:
            log.warning("result for %s not uploaded yet (%s); will retry", job_id[:8], e)
            self._unuploaded[job_id] = data
            return
        self._unuploaded.pop(job_id, None)
        self.ledger.mark_uploaded(job_id, contract.format_time(self.now()))
        self._delete(contract.pending_key(job_id))

    def _delete(self, key):
        try:
            self.store.delete(key)
        except S3Error as e:
            log.warning("could not delete %s: %s", key, e)

    def finish(self, job_id, kind, outcome, started):
        finished = self.now()
        payload = self.result_payload(job_id, kind, outcome, started, finished)
        data = self._seal(contract.TAG_RESULT, payload)
        if len(data) > contract.MAX_RESULT_BYTES:
            log.error("result for %s too large; replacing it with a failure", job_id[:8])
            outcome = jobs.failed("device_error", "The result was too large to return.", firmware=outcome.firmware)
            data = self._seal(contract.TAG_RESULT, self.result_payload(job_id, kind, outcome, started, finished))
        self.ledger.finish(job_id, contract.format_time(finished), data)  # step 12: ledger before upload
        self.last_job = {"job_id": job_id, "status": outcome.status, "finished_at": contract.format_time(finished)}
        self._publish(job_id, data)
        return outcome

    def write_owned_slots(self):
        """The informational bucket copy of the owned-slot ledger (contract section 2). Nothing
        reads it to decide a write; the local owned_slots.jsonl is the record."""
        entries = self.owned.current()[:256]
        payload = {"v": 1, "type": "qc2-device-owned-slots", "bucket": self.config.bucket,
                   "written_at": contract.format_time(self.now()),
                   "slots": [{"slot": e["slot"], "name": e["name"], "job_id": e["job_id"], "at": e["at"]}
                             for e in entries]}
        try:
            self.store.put(contract.OWNED_SLOTS_KEY, self._seal(contract.TAG_OWNED_SLOTS, payload))
        except S3Error as e:
            log.warning("owned-slots copy not written: %s", e)

    # ---- crash recovery (local state only) ----

    def recover(self):
        for job_id in self.ledger.unfinished():
            kind = self.ledger.taken[job_id].get("kind", "")
            if kind not in ("push_preset", "list_slots", "read_preset"):
                kind = "push_preset"  # cannot happen: only parsed jobs are taken
            log.warning("job %s was interrupted by a crash or a closed window; reporting it, not retrying",
                        job_id[:8])
            at = contract.parse_time(self.ledger.taken[job_id]["at"])
            self.finish(job_id, kind, jobs.failed(
                "interrupted", "The bridge stopped in the middle of this job (crash, closed window or power). "
                "It is not retried. For a push, check the slot on the unit before sending again."), at)
        # Results whose upload was never confirmed. One whose upload the ledger records is not re-checked:
        # a bucket-token holder deleting it later is the "delete objects" power contract 15 already concedes.
        for job_id in self.ledger.not_uploaded():
            e = self.ledger.finished[job_id]
            key = contract.done_key(job_id)
            try:
                if self.store.exists(key):
                    continue
            except S3Error as err:
                log.warning("could not check %s: %s", key, err)
            self._publish(job_id, e["result"].encode("ascii"))  # the stored bytes, never a new result

    def retry_uploads(self):
        for job_id, data in list(self._unuploaded.items()):
            self._publish(job_id, data)

    # ---- the pending queue (section 8) ----

    def _seen(self, job_id):
        if self.ledger.contains(job_id):
            return True
        return self.store.exists(contract.done_key(job_id))  # S3Error propagates: skip this round

    def scan(self):
        """Steps 1-9 for up to MAX_SCAN pending objects. Returns verified jobs, oldest first."""
        try:
            items, more = self.store.list_prefix(contract.PENDING_PREFIX, MAX_LIST)
        except S3Error as e:
            log.warning("could not list pending jobs: %s", e)
            return []
        if more:
            log.warning("more than %d objects under jobs/pending/; looking at the first %d", MAX_LIST, MAX_LIST)
        ready, read = [], 0
        for key, size in items:
            if not contract.PENDING_RE.fullmatch(key):                                   # 1
                if key not in self._logged_bad_keys and len(self._logged_bad_keys) < MAX_LIST:
                    self._logged_bad_keys.add(key)
                    log.warning("ignoring an object with a malformed key under jobs/pending/")
                continue
            if read >= MAX_SCAN:
                break
            read += 1
            if size > contract.MAX_JOB_BYTES:                                             # 2 (from the listing)
                log.warning("deleting oversized pending object %s (%d bytes)", key[13:21], size)
                self._delete(key)
                continue
            try:
                data = self.store.get_bounded(key, contract.MAX_JOB_BYTES)
            except S3Error as e:
                log.warning("could not read %s: %s", key[13:21], e)
                continue
            if data is None:
                log.warning("deleting oversized pending object %s", key[13:21])
                self._delete(key)
                continue
            job = self._check(key, data)
            if job is not None:
                ready.append((job["created_at"], key, data))
        ready.sort()
        return [(k, d) for _, k, d in ready]

    def _check(self, key, data):
        """check_pending_job plus the action its outcome demands. Returns the job if it may run."""
        try:
            return contract.check_pending_job(self.config.keys, key, self.config.bucket, self.now(), data,
                                              self._seen)
        except contract.Reject as r:
            if r.outcome == "expired":
                self.handle_expired(r.job, key)
            elif r.outcome in DELETE_OUTCOMES:
                hint = (" (DEVICE_JOB_KEY on this PC differs from QC-2's: update agent.env)"
                        if r.outcome == "unknown_kid" else "")
                log.warning("deleting pending object %s: %s%s", key[13:21], r.outcome, hint)
                self._delete(key)
            else:
                log.warning("skipping pending object %s: %s", key[13:21], r.outcome)
        except S3Error as e:
            log.warning("could not check %s for replay: %s; will retry", key[13:21], e)
        return None

    def handle_expired(self, job, key):
        jid = job["job_id"]
        now = self.now()
        self.ledger.take(jid, job["kind"], contract.format_time(now))
        log.info("job %s %s: expired before it could run", jid[:8], job["kind"])
        self.finish(jid, job["kind"], jobs.Outcome("expired", "expired",
                                                   "The request expired before the bridge picked it up."), now)

    def run_job(self, key, data):
        job = self._check(key, data)  # again, with a fresh clock: expiry and replay as of now
        if job is None:
            return
        jid, kind = job["job_id"], job["kind"]
        started = self.now()
        self.ledger.take(jid, kind, contract.format_time(started))  # step 10, before any device contact
        self.state, self.current_job = "busy", jid
        self.last_activity = started
        self.kick_heartbeat()
        log.info("job %s %s: started", jid[:8], kind)
        try:
            outcome = jobs.run(self.ctx, job)
        except Exception:  # noqa: BLE001  never leave a taken job without a result
            log.error("job %s crashed:\n%s", jid[:8], traceback.format_exc())
            outcome = jobs.failed("device_error", "The bridge hit an internal error running this job.")
        outcome = self.finish(jid, kind, outcome, started)
        log.info("job %s %s: %s%s", jid[:8], kind, outcome.status,
                 f" ({outcome.reason_code})" if outcome.reason_code else "")
        if outcome.message:
            log.info("  %s", outcome.message)
        self.state, self.current_job = "idle", None
        self.last_activity = self.now()
        if kind == "push_preset" and outcome.push and "listed" in outcome.push:
            self.write_owned_slots()
        if outcome.unit_gone:
            self.unit_present = False
            self.request_exit("the Quad Cortex went away", EXIT_UNIT_GONE)
        self.kick_heartbeat()

    # ---- presence, exit ----

    def check_presence(self):
        try:
            p = self.device.probe()
        except Exception:  # noqa: BLE001
            return
        if p.present is None:
            return
        if p.present and not self.unit_present:
            log.info("Quad Cortex detected%s", " (held by another program)" if p.busy else "")
        if self.unit_seen and self.unit_present and not p.present:
            self.unit_present = False
            self.request_exit("the Quad Cortex was switched off or unplugged", EXIT_UNIT_GONE)
            return
        changed = self.unit_present != p.present
        self.unit_present = p.present
        self.unit_seen = self.unit_seen or p.present
        if changed:
            self.kick_heartbeat()

    def request_exit(self, reason, code=EXIT_OK):
        if self.exit_reason is None:
            self.exit_reason, self.exit_code = reason, code
        self.stop.set()

    def run(self):
        """The whole session. Returns the process exit code."""
        log.info("Bridge running: waiting for jobs (bucket %s, slots %s, firmware %s, exits after %d idle minutes)",
                 self.config.bucket, ",".join(self.config.allowed_slots.labels()),
                 self.config.expected_firmware, self.config.idle_minutes)
        self.recover()
        self.check_presence()
        self.write_heartbeat()
        self._hb_thread = threading.Thread(target=self._heartbeat_loop, name="heartbeat", daemon=True)
        self._hb_thread.start()
        try:
            while not self.stop.is_set():
                if self.now() >= self.idle_exit_at():
                    self.request_exit(f"no job for {self.config.idle_minutes} minutes")
                    break
                self.check_presence()
                if self.stop.is_set():
                    break
                self.retry_uploads()
                for key, data in self.scan():
                    if self.stop.is_set():
                        break
                    self.run_job(key, data)
                self._sleep(self.poll_interval())
        finally:
            self.stop.set()
            self._hb_wake.set()
            if self._hb_thread is not None:
                self._hb_thread.join(timeout=5)
            self.state = "exiting"
            self.write_heartbeat("exiting")
            log.info("Bridge stopped: %s.", self.exit_reason or "stop requested")
        return self.exit_code
