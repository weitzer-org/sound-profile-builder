"""What each job kind does on the unit (contract section 8 steps 11-12, plan section 5).

Every executor returns an Outcome; none raises for an expected failure. The rules
that bind every device write, in the order they run for a push:
  1. refuse anything the agent can't check: an unsupported export format, an export
     that fails validation against the agent's own catalog copy, a slot outside
     QC_AGENT_ALLOWED_SLOTS (all before the unit is opened);
  2. the unit's firmware equals QC_AGENT_EXPECTED_FIRMWARE exactly, and it is a full
     Quad Cortex (8 slots per bank);
  3. a FRESH directory listing shows the slot empty. A slot the bridge didn't create
     is never overwritten, and v1 never overwrites at all (editing existing presets
     is deferred: plan section 10, item 7);
  4. the slot is the one open on the unit and its live grid is blank. This is the
     only save path Phase 0 verified (G3): the agent never recalls a preset or
     clears a grid, so it can't disturb what the owner has open;
  5. the job has not expired (re-checked immediately before the first write);
  6. build, read back and diff; save only if the diff is clean, after a second fresh
     listing still shows the slot empty and the same slot still open.
Capture blocks (plan 4.2) are resolved against the unit's own capture list from the
rule-3 listing (qc_agent.captures): a capture that is missing, ambiguous, cloud-only
or of unknown version refuses the job before any write. The second listing must
resolve every capture to the same key and version, or nothing is saved.
"""
from __future__ import annotations

import json
import logging
import math
import time
from dataclasses import dataclass, field

from . import captures, contract, slots, text
from .device import SETLIST_KEY, SETLIST_NAME, DeviceError, UnitAbsent, UnitBusy
from .export_validator import CAPTURE_BLOCK_IDS, SUPPORTED_FORMATS, ExportRefused, check_preset_name, validate

log = logging.getLogger("qc_agent")

TOL = 1e-4                 # G3 read-back tolerance
PACE_S = 0.15              # between writes; G3 used the same pacing
SAVE_LIST_POLLS = 6        # G3: a listing 3 s after the save still showed the slot blank
SAVE_LIST_INTERVAL_S = 6.0
MAX_DIFF = 512
CAPTURE_SETTLE_S = 1.0     # after loading a capture, before its params (unverified timing; the read-back checks)
NAME_LIMIT = 64
# A read-back text value (contract section 9.1). A Neural Capture block's file_name is a 64-hex key plus the
# capture's name, so NAME_LIMIT would cut the name off entirely. internal/devicejobs.MaxReadbackStringRn is the
# same number; testdata/devicejobs/readback.json pins both sides.
VALUE_STRING_LIMIT = 192


@dataclass
class Outcome:
    status: str                 # succeeded | failed | refused | expired
    reason_code: str = ""
    message: str = ""
    firmware: str | None = None
    push: dict | None = None
    slots: dict | None = None
    readback: dict | None = None
    unit_gone: bool = False     # the agent exits after this job
    extra: dict = field(default_factory=dict)


def refused(code, msg, **kw):
    return Outcome("refused", code, msg, **kw)


def failed(code, msg, **kw):
    return Outcome("failed", code, msg, **kw)


class JobContext:
    def __init__(self, config, catalog, device, owned, now_fn, sleep=time.sleep):
        self.config, self.catalog, self.device, self.owned = config, catalog, device, owned
        self.now, self.sleep = now_fn, sleep
        self.firmware_seen = None  # (firmware, when) for the heartbeat


class _Gone(Exception):
    pass


def _same_name(listing, pos, name):
    """Another slot whose preset has this name (case and surrounding spaces ignored), or None."""
    want = name.strip().casefold()
    for p, n in listing.items():
        if p != pos and isinstance(n, str) and n.strip().casefold() == want:
            return p
    return None


def _occupied(listing, pos) -> bool:
    """Only an exactly blank name is an empty slot. A missing position or a name that is just
    spaces counts as occupied: the agent refuses rather than risk overwriting a preset."""
    name = listing.get(pos)
    return not isinstance(name, str) or name != ""


def _connect_checked(ctx):
    """Open the unit and check firmware and device type. Returns an Outcome on refusal."""
    dev = ctx.device
    try:
        dev.connect()
    except UnitBusy:
        return failed("unit_busy", "Cortex Control (or another program) has the Quad Cortex open. "
                      "Close it completely, then send the request again.")
    except UnitAbsent:
        return failed("unit_absent", "No Quad Cortex found on USB. Switch it on and connect it, then retry.",
                      unit_gone=True)
    except DeviceError as e:
        log.warning("connect failed: %s", text.clean(str(e), 200))
        return failed("device_error", "Could not open the Quad Cortex.")
    fw = dev.firmware
    ctx.firmware_seen = (fw, ctx.now())
    if fw != ctx.config.expected_firmware:
        return refused("firmware_mismatch",
                       f"The unit reports firmware {text.clean(fw or 'unknown', 32)}; this bridge only works with "
                       f"{ctx.config.expected_firmware}. Nothing was changed.", firmware=_fw(fw))
    if dev.device_type != "QC":
        return refused("firmware_mismatch",
                       f"The unit reports device type {text.clean(str(dev.device_type), 16)}; this bridge "
                       "supports a full Quad Cortex only. Nothing was changed.", firmware=_fw(fw))
    return None


def _fw(fw):
    return text.clean(fw, 32) if isinstance(fw, str) and fw else None


def _check_alive(ctx):
    if not ctx.device.alive():
        raise _Gone()


def run(ctx, job) -> Outcome:
    kind = job["kind"]
    executor = {"list_slots": list_slots, "read_preset": read_preset, "push_preset": push_preset}.get(kind)
    if executor is None:  # parse_job only admits the three kinds; keep the code anyway
        return refused("unsupported_kind", "This bridge doesn't run that kind of job.")
    try:
        return executor(ctx, job)
    finally:
        try:
            ctx.device.end_push()
        except Exception:  # noqa: BLE001
            log.exception("end_push failed")
        ctx.device.disconnect()


# ---- list_slots ----

def _slot_table(ctx, listing):
    out = []
    for pos in range(slots.SLOT_COUNT):
        raw = listing.get(pos, "")
        name = text.clean(raw, NAME_LIMIT)
        occupied = _occupied(listing, pos)
        out.append({"slot": slots.label(pos), "state": "occupied" if occupied else "empty",
                    "name": (name or "?") if occupied else None,
                    "owned": occupied and ctx.owned.owned(pos, raw)})
    return out


def list_slots(ctx, job) -> Outcome:
    bad = _connect_checked(ctx)
    if bad:
        return bad
    fw = _fw(ctx.device.firmware)
    try:
        listing = ctx.device.list_my_presets()
        _check_alive(ctx)
    except (_Gone, UnitAbsent):
        return failed("unit_absent", "The Quad Cortex went away during the listing.", firmware=fw, unit_gone=True)
    except DeviceError as e:
        log.warning("listing failed: %s", text.clean(str(e), 200))
        return failed("device_error", "The unit's directory listing failed.", firmware=fw)
    table = _slot_table(ctx, listing)
    empty = sum(1 for s in table if s["state"] == "empty")
    return Outcome("succeeded", "", f"My Presets: {empty} of {len(table)} slots empty.", firmware=fw,
                   slots={"setlist": SETLIST_NAME, "slots": table})


# ---- read_preset ----

def _plain_value(v):
    if isinstance(v, bool):
        return v
    if isinstance(v, int):
        return v
    if isinstance(v, float):
        return v if math.isfinite(v) else None
    if isinstance(v, str):
        return text.clean(v, VALUE_STRING_LIMIT)
    return None


def readback_payload(preset, position_info):
    chains = []
    for ch in preset.get("chains", []):
        blocks = []
        for b in ch.get("blocks", []):
            blocks.append({"col": b["col"], "model_id": b["model_id"],
                           "params": [{"index": p["index"], "scene_mode": bool(p.get("scene_mode")),
                                       "values": [_plain_value(v) for v in p.get("values", [])[:8]]}
                                      for p in b.get("params", [])]})
        chains.append({"row": ch["row"], "in_port": ch.get("in_port"), "out_port": ch.get("out_port"),
                       "blocks": blocks})
    bypass = preset.get("bypass")
    return {"setlist": position_info, "chains": chains,
            "bypass": [[[bool(x) for x in col[:8]] for col in row[:8]] for row in bypass[:4]] if bypass else None}


def read_preset(ctx, job) -> Outcome:
    if job["args"]["slot"] is not None:
        # Recalling a slot changes what the unit is playing (contract section 7).
        return refused("unsupported_arg", "This bridge reads the preset open on the unit only; "
                       "select the slot on the unit and read the current preset.")
    bad = _connect_checked(ctx)
    if bad:
        return bad
    fw = _fw(ctx.device.firmware)
    try:
        pos = ctx.device.setlist_position() or {}
        preset = ctx.device.read_current_preset()
        after = ctx.device.setlist_position() or {}
        _check_alive(ctx)
        if after != pos:
            return failed("device_error", "The selected preset changed during the read; read again.", firmware=fw)
    except (_Gone, UnitAbsent):
        return failed("unit_absent", "The Quad Cortex went away during the read.", firmware=fw, unit_gone=True)
    except DeviceError as e:
        log.warning("read failed: %s", text.clean(str(e), 200))
        return failed("device_error", "Reading the current preset failed.", firmware=fw)
    if preset is None:
        return failed("device_error", "The unit did not return the current preset in time.", firmware=fw)
    slot = None
    p = pos.get("position")
    if (pos.get("folder_key") == SETLIST_KEY and not pos.get("is_factory")
            and isinstance(p, int) and 0 <= p < slots.SLOT_COUNT):
        slot = slots.label(p)
    info = {"folder": SETLIST_NAME if pos.get("folder_key") == SETLIST_KEY and not pos.get("is_factory") else "other",
            "position": p if slot else None}
    name = text.clean(preset.get("name", ""), NAME_LIMIT)
    rb = {"format": "qc-device-readback/1", "slot": slot, "name": name or None,
          "payload": readback_payload(preset, info)}
    if len(json.dumps(rb, separators=(",", ":"))) > contract.MAX_RESULT_BYTES - 16 * 1024:
        return failed("device_error", "The open preset is too large to return.", firmware=fw)
    nblocks = sum(len(c["blocks"]) for c in rb["payload"]["chains"])
    return Outcome("succeeded", "", f"Read the open preset ({nblocks} blocks).", firmware=fw, readback=rb)


# ---- push_preset ----

def _expired(ctx, job) -> bool:
    return not ctx.now() < contract.parse_time(job["expires_at"])


def _diff_param(row, col, p, vals, diff) -> int:
    """Compare one written parameter's read-back values; append diff entries; return the mismatch count."""
    mismatches = 0
    expected = p.scene_wires or [p.wire]
    for s in (range(8) if p.scene_wires else [0]):
        v = vals[s] if s < len(vals) else None
        if not p.scene_wires:
            # Not scene-assigned: the unit replicates one value into all 8 slots (G1); need all eight.
            ok = len(vals) == 8 and all(isinstance(x, float) and abs(x - p.wire) <= TOL for x in vals)
        else:
            ok = isinstance(v, float) and abs(v - expected[s]) <= TOL
        mismatches += not ok
        if len(diff) < MAX_DIFF:
            e = {"row": row, "col": col, "param_index": p.index, "expected": expected[s],
                 "readback": v if isinstance(v, float) and math.isfinite(v) else None, "ok": ok}
            if p.scene_wires:
                e["scene"] = "ABCDEFGH"[s]
            diff.append(e)
    return mismatches


def _diff(plan, preset):
    """Compare the live grid with the plan. Returns (mismatches, diff entries, problems)."""
    diff, problems, mismatches = [], [], 0
    chain = next((c for c in preset.get("chains", []) if c.get("row") == plan.row), None)
    if chain is None:
        return 1, diff, ["row 0 missing from the read-back"]
    if (chain.get("in_port"), chain.get("out_port")) != (plan.in_port, plan.out_port):
        mismatches += 1
        problems.append(f"routing reads in={chain.get('in_port')} out={chain.get('out_port')}")
    by_col = {b["col"]: b for b in chain.get("blocks", [])}
    planned_cols = {b.col for b in plan.blocks}
    for c in sorted(set(by_col) - planned_cols):
        mismatches += 1
        problems.append(f"unexpected block at column {c}")
    for other in preset.get("chains", []):
        if other.get("row") != plan.row and other.get("blocks"):
            mismatches += 1
            problems.append(f"unexpected blocks on row {other.get('row')}")
    bypass = preset.get("bypass")
    for b in plan.blocks:
        got = by_col.get(b.col)
        if got is None or got.get("model_id") != b.model_id:
            mismatches += 1
            problems.append(f"column {b.col}: expected model {b.model_id}, found "
                            f"{None if got is None else got.get('model_id')}")
            continue
        params = {p["index"]: p for p in got.get("params", [])}
        if b.capture is not None:
            vals = (params.get(captures.FILE_NAME_PARAM) or {}).get("values") or []
            if not vals or any(not isinstance(v, str) or v != b.capture.file_name for v in vals):
                mismatches += 1
                problems.append(f"column {b.col}: the capture block's file_name doesn't read back as planned "
                                f"({text.clean(str(vals[0]) if vals else '', 80)!r})")
        for p in b.params:
            mismatches += _diff_param(plan.row, b.col, p, (params.get(p.index) or {}).get("values") or [], diff)
        want = b.bypass_scenes
        have = None
        if bypass and len(bypass) > plan.row and len(bypass[plan.row]) > b.col:
            have = [bool(x) for x in bypass[plan.row][b.col][:8]]
        if have is None or len(have) < 8:
            mismatches += 1  # G6: the unit reports eight flags for every grid position
            problems.append(f"column {b.col}: bypass flags missing from the read-back")
        elif have != want:
            mismatches += 1
            problems.append(f"column {b.col}: bypass reads {have}, expected {want}")
    return mismatches, diff, problems


class _Push:
    """One push, as the steps the module docstring lists. Each step returns an Outcome to stop
    with, or None to go on; run() owns the order and the error handling."""

    def __init__(self, ctx, job):
        self.ctx, self.job, self.dev = ctx, job, ctx.device
        a = job["args"]
        self.args, self.slot, self.name = a, a["slot"], a["name"]
        self.pos = slots.position(self.slot)
        self.result = {"slot": self.slot, "name": self.name, "mismatches": 0, "cpu_load": None, "diff": []}
        self.plan = None
        self.fw = None
        self.saved = False
        self.resolved = {}   # col -> captures.ResolvedCapture
        self.deps_note = ""

    def run(self) -> Outcome:
        out = self.check_job()                              # rule 1 (no device contact)
        if out:
            return out
        out = _connect_checked(self.ctx)                    # rule 2
        if out:
            return out
        self.fw = _fw(self.dev.firmware)
        try:
            for step in (self.check_unit, self.build, self.verify):
                outcome = step()
                if outcome is not None:  # a step stops the push by returning its Outcome
                    return outcome
            return self.save()
        except (_Gone, UnitAbsent):
            if self.saved:
                return self.done(f"Save to {self.slot} issued after a clean read-back, then the Quad Cortex went "
                                 f"away before the save could be confirmed. Check {self.slot} on the unit.",
                                 unit_gone=True)
            return failed("unit_absent", "The Quad Cortex went away during the push. If anything was written it "
                          "was not saved.", firmware=self.fw, push=self.result, unit_gone=True)
        except DeviceError as e:
            log.warning("push failed: %s", text.clean(str(e), 200))
            if self.saved:
                return self.done(f"Save to {self.slot} issued after a clean read-back, but confirming it failed. "
                                 f"Check {self.slot} on the unit.")
            return failed("device_error", "A device operation failed. Nothing was saved; re-select the slot on the "
                          "unit to discard any partial grid.", firmware=self.fw, push=self.result)

    def done(self, msg, **kw):
        msg = msg + captures.summary(self.resolved) + self.deps_note
        return Outcome("succeeded", "", msg, firmware=self.fw, push=self.result, **kw)

    def check_job(self):
        a, cfg = self.args, self.ctx.config
        if a["export"]["format"] not in SUPPORTED_FORMATS:
            return refused("unsupported_arg", f"This bridge can't push export format "
                           f"{text.clean(a['export']['format'], 48)}; update the bridge.")
        if not cfg.allowed_slots.contains(self.slot):
            return refused("slot_not_allowed", f"Slot {self.slot} is outside the bridge's allowed slots "
                           f"({', '.join(cfg.allowed_slots.labels())}).")
        try:
            check_preset_name(self.name)
            self.plan = validate(a["export"], self.ctx.catalog, cfg.expected_firmware)
        except ExportRefused as e:
            return refused("validation", "The export failed the bridge's own checks: " + text.clean(str(e), 400))
        return None

    def check_slot(self, listing, cur):
        """Rules 3 and 4 against a listing and the open position (run before writing and again before saving)."""
        slot, pos = self.slot, self.pos
        if _occupied(listing, pos):
            return refused("slot_not_empty", f"Slot {slot} is not empty on the unit "
                           f"(it holds {text.clean(listing.get(pos), NAME_LIMIT)!r}). The bridge never "
                           "overwrites a preset; pick an empty slot.", firmware=self.fw)
        dup = _same_name(listing, pos, self.name)
        if dup is not None:
            return refused("validation", f"Slot {slots.label(dup)} already has a preset named {self.name!r}. Pick a "
                           "different name: if the unit stores presets by name, a save could replace it.",
                           firmware=self.fw)
        if cur.get("folder_key") != SETLIST_KEY or cur.get("position") != pos or cur.get("is_factory"):
            return refused("unsupported_arg", f"Select slot {slot} (empty) on the Quad Cortex first, then send "
                           "again. The bridge only builds into the empty slot that is open on the unit, so it "
                           "never changes what you have loaded.", firmware=self.fw)
        return None

    def check_unit(self):
        """Rules 3-5: fresh listing, the slot open with a blank grid, not expired; captures resolved."""
        snap = self.dev.read_directory()
        _check_alive(self.ctx)
        out = self.check_slot(snap.my_presets, self.dev.setlist_position() or {})
        if out:
            return out
        if any(b.capture for b in self.plan.blocks):
            out = self.resolve_captures(snap)
            if out:
                return out
        live = self.dev.read_current_preset()
        _check_alive(self.ctx)
        if live is None:
            return failed("device_error", "The unit did not return the open slot's grid in time.", firmware=self.fw)
        if any(c.get("blocks") for c in live.get("chains", [])):
            return refused("slot_not_empty", f"Slot {self.slot} is open but its grid isn't blank (unsaved edits?). "
                           f"Re-select {self.slot} on the unit to discard them, then retry.", firmware=self.fw)
        if _expired(self.ctx, self.job):
            return Outcome("expired", "expired", "The request expired before the bridge got to it; "
                           "nothing was written.", firmware=self.fw)
        return None

    def resolve_captures(self, snap):
        if not snap.capture_folders:
            return failed("device_error", "The unit's directory listing had no capture folders (incomplete "
                          "listing?), so the bridge can't place captures. Nothing was written; send it again.",
                          firmware=self.fw)
        try:
            self.resolved = captures.resolve(self.plan.blocks, snap)
        except captures.CaptureRefused as e:
            return refused("validation", str(e), firmware=self.fw)
        self.plan = captures.with_resolution(self.plan, self.resolved, CAPTURE_BLOCK_IDS)
        return None

    def still_open(self) -> bool:
        cur = self.dev.setlist_position() or {}
        return (cur.get("folder_key") == SETLIST_KEY and cur.get("position") == self.pos
                and not cur.get("is_factory"))

    def build(self):
        """The writes, into the live edit buffer of the empty, open slot."""
        dev, plan, ctx = self.dev, self.plan, self.ctx
        dev.begin_push(self.pos, self.name, plan)
        dev.set_routing(plan.row, plan.in_port, plan.out_port)
        ctx.sleep(PACE_S)
        for b in plan.blocks:
            _check_alive(ctx)
            if not self.still_open():
                return failed("device_error", f"Another preset was selected on the unit during the build, so the "
                              f"bridge stopped. Nothing was saved; re-select {self.slot} or another preset to "
                              "discard the partial edits.", firmware=self.fw, push=self.result)
            if b.capture is not None:
                # file_name first, then the controls: a capture loaded after them could reset them.
                if not dev.place_capture(plan.row, b.col, b.model_id, b.capture.key, b.capture.name):
                    return failed("device_error", f"The unit did not confirm placing capture "
                                  f"{text.clean(b.capture.name, 64)!r} (a V{b.capture.version} capture block) in "
                                  f"column {b.col}. Nothing was saved; re-select the slot on the unit to discard "
                                  "the partial grid.", firmware=self.fw, push=self.result)
                ctx.sleep(CAPTURE_SETTLE_S)
            elif not dev.add_block(plan.row, b.col, b.model_id):
                return failed("device_error", f"The unit did not confirm placing {b.model} in column {b.col}. "
                              "Nothing was saved; re-select the slot on the unit to discard the partial grid.",
                              firmware=self.fw, push=self.result)
            ctx.sleep(PACE_S)
            for p in b.params:
                if p.scene_wires:
                    dev.set_param_scenes(plan.row, b.col, p.index, p.scene_wires)
                else:
                    dev.set_param(plan.row, b.col, p.index, p.wire)
                ctx.sleep(PACE_S)
            if any(b.bypass_scenes):  # a new block starts active; write flags only when some scene bypasses it
                dev.set_block_bypass(plan.row, b.col, b.bypass_scenes)
            _check_alive(ctx)
        return None

    def verify(self):
        """Read back and diff; any mismatch stops before the save."""
        self.ctx.sleep(0.5)
        back = self.dev.read_current_preset()
        _check_alive(self.ctx)
        if back is None:
            return failed("device_error", "The read-back timed out. Nothing was saved; re-select the slot on "
                          "the unit to discard the edits.", firmware=self.fw, push=self.result)
        mismatches, diff, problems = _diff(self.plan, back)
        self.result.update(mismatches=mismatches, diff=diff)
        if mismatches:
            for pr in problems[:10]:
                log.warning("read-back: %s", pr)
            return failed("readback_mismatch", f"The unit's read-back differs from the plan in {mismatches} "
                          "place(s). Nothing was saved; re-select the slot on the unit to discard the edits.",
                          firmware=self.fw, push=self.result)
        return None

    def save(self):
        """Rule 6: a second fresh listing, then File CREATE, then wait for the listing to show it."""
        dev, ctx, slot, pos, name = self.dev, self.ctx, self.slot, self.pos, self.name
        snap = dev.read_directory()
        cur = dev.setlist_position() or {}
        _check_alive(ctx)
        out = self.check_slot(snap.my_presets, cur)
        if out:
            out.message = f"The unit changed while the preset was being built; nothing was saved. {out.message}"
            out.push = self.result
            return out
        if self.resolved:
            try:
                again = captures.resolve(self.plan.blocks, snap)
            except captures.CaptureRefused:
                again = None
            if again != self.resolved:
                return failed("device_error", "The unit's capture library changed during the build (a capture no "
                              "longer resolves to the same file). Nothing was saved; re-select "
                              f"{slot} on the unit to discard the partial grid.", firmware=self.fw, push=self.result)
        back = dev.read_current_preset()  # the listing takes ~15 s: the grid must still be the verified one
        _check_alive(ctx)
        if back is None or _diff(self.plan, back)[0]:
            return failed("readback_mismatch", "The grid changed after it was verified (edited on the unit?). "
                          "Nothing was saved; re-select the slot to discard the edits.",
                          firmware=self.fw, push=self.result)
        if not self.still_open():  # File CREATE saves whatever grid is live: the slot must still be the open one
            return failed("device_error", "Another preset was selected on the unit just before the save. "
                          "Nothing was saved.", firmware=self.fw, push=self.result)
        dev.save_to_slot(pos, name)
        self.saved = True
        self.result["listed"] = False
        try:
            ctx.owned.record(slot, pos, name, self.job["job_id"], contract.format_time(ctx.now()))
        except OSError as e:  # the save happened; a local ledger problem mustn't turn it into a failure
            log.error("could not record the owned slot %s: %s", slot, e)
        for _ in range(SAVE_LIST_POLLS):
            ctx.sleep(SAVE_LIST_INTERVAL_S)
            _check_alive(ctx)
            snap = dev.read_directory()
            if snap.my_presets.get(pos) == name:
                self.result["listed"] = True
                self.deps_note = captures.deps_report(snap, pos, self.resolved)
                break
        if self.resolved and not self.result["listed"]:
            self.deps_note = captures.deps_report(captures.DirectorySnapshot(my_presets={}), pos, self.resolved)
        labels = " Scene labels are not written in v1." if self.plan.scene_labels else ""
        if self.result["listed"]:
            return self.done(f"Saved {name!r} to {slot}; read-back matched.{labels}")
        return self.done(f"Save to {slot} issued and the read-back matched, but the unit's listing didn't show "
                         f"{name!r} within {int(SAVE_LIST_POLLS * SAVE_LIST_INTERVAL_S)} s (the unit can lag for "
                         f"minutes). Check {slot} on the unit.{labels}")


def push_preset(ctx, job) -> Outcome:
    return _Push(ctx, job).run()
