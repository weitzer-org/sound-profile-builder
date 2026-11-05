"""The job loop end to end: in-memory bucket, fake unit, fixed clock.

Covers every refusal path of contract section 8 step 11 and the plan's device rules,
crash recovery from local state only, the heartbeat, and the exit rules. Proves the
agent's logic and guards; says nothing about the real unit (that's Phase 0 / P6).
"""
import copy
import datetime as dt
import json
import tempfile
import unittest
from pathlib import Path

from helpers import BUCKET, KEY_HEX, NEXT_HEX, T0, Clock, catalog, config, export, job, open_result
from qc_agent import agent as agent_mod
from qc_agent import contract, jobs
from qc_agent.agent import Agent
from qc_agent.s3 import MemoryStore, S3Error
from fake_device import FakeDevice

jobs.SAVE_LIST_INTERVAL_S = 0
jobs.PACE_S = 0


class Base(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.home = Path(self.tmp.name)
        self.store = MemoryStore(BUCKET)
        self.dev = FakeDevice()
        self.clock = Clock()
        self.cfg = config()
        self.agent = self.make_agent()

    def tearDown(self):
        self.tmp.cleanup()

    def make_agent(self, cfg=None):
        a = Agent(cfg or self.cfg, self.store, self.dev, catalog(), self.home, now_fn=self.clock, sleep=lambda s: None)
        a.ctx.sleep = lambda s: None
        return a

    def queue(self, *a, **kw):
        jid, key, data = job(*a, **kw)
        self.store.objects[key] = data
        return jid, key

    def poll(self):
        for key, data in self.agent.scan():
            self.agent.run_job(key, data)

    def result(self, jid):
        return open_result(self.store, jid)


class ListAndRead(Base):
    def test_list_slots(self):
        self.dev.slots = {0: "Clean", 8: "bb king test 2"}
        jid, key = self.queue("list_slots")
        self.poll()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"], r["firmware"]), ("succeeded", "", "4.1.0"))
        self.assertEqual(len(r["slots"]["slots"]), 256)
        self.assertEqual(r["slots"]["slots"][8], {"slot": "2A", "state": "occupied", "name": "bb king test 2",
                                                  "owned": False})
        self.assertEqual(r["slots"]["slots"][1]["state"], "empty")
        self.assertNotIn(key, self.store.objects)
        self.assertEqual(self.dev.write_calls(), [])
        self.assertEqual(r["agent_version"], agent_mod.AGENT_VERSION)

    def test_device_text_is_cleaned(self):
        self.dev.slots = {3: "evil‮`name`\nnext![p](https://x/t.png)" + "x" * 200}
        jid, _ = self.queue("list_slots")
        self.poll()
        name = self.result(jid)["slots"]["slots"][3]["name"]
        self.assertNotIn("`", name)
        self.assertNotIn("\n", name)
        self.assertNotIn("‮", name)
        self.assertLessEqual(len(name), 64)

    def test_read_current_preset(self):
        self.dev.connect()
        self.dev.add_block(0, 0, 1)
        self.dev.set_param(0, 0, 0, 0.4)
        self.dev.disconnect()
        self.dev.calls.clear()
        jid, _ = self.queue("read_preset")
        self.poll()
        r = self.result(jid)
        self.assertEqual(r["status"], "succeeded")
        rb = r["readback"]
        self.assertEqual((rb["format"], rb["slot"]), ("qc-device-readback/1", "20A"))
        self.assertEqual(rb["payload"]["chains"][0]["blocks"][0]["model_id"], 1)
        self.assertEqual(rb["payload"]["chains"][0]["blocks"][0]["params"][0]["values"], [0.4] * 8)
        self.assertEqual(self.dev.write_calls(), [])

    def test_read_named_slot_is_refused_without_touching_the_unit(self):
        jid, _ = self.queue("read_preset", args={"slot": "20A"})
        self.poll()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "unsupported_arg"))
        self.assertEqual(self.dev.calls, [("end_push",), ("disconnect",)])

    def test_firmware_mismatch_refuses_reads_too(self):
        self.dev.fw = "4.2.0"
        jid, _ = self.queue("list_slots")
        self.poll()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"], r["firmware"]), ("refused", "firmware_mismatch", "4.2.0"))
        self.assertNotIn(("list_my_presets",), self.dev.calls)

    def test_mini_is_refused(self):
        self.dev.dtype = "ATMA"
        jid, _ = self.queue("list_slots")
        self.poll()
        self.assertEqual(self.result(jid)["reason_code"], "firmware_mismatch")

    def test_unit_busy(self):
        self.dev.busy = True
        jid, _ = self.queue("list_slots")
        self.poll()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "unit_busy"))
        self.assertIsNone(r["firmware"])
        self.assertFalse(self.agent.stop.is_set(), "a busy unit is still attached; keep running")

    def test_unit_absent_ends_the_session(self):
        self.dev.absent_on_connect = True
        jid, _ = self.queue("list_slots")
        self.poll()
        self.assertEqual(self.result(jid)["reason_code"], "unit_absent")
        self.assertTrue(self.agent.stop.is_set())
        self.assertEqual(self.agent.exit_code, agent_mod.EXIT_UNIT_GONE)


class Push(Base):
    def push(self, **kw):
        jid, key = self.queue("push_preset", **kw)
        self.poll()
        return self.result(jid)

    def test_push_succeeds(self):
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("succeeded", ""), r["message"])
        p = r["push"]
        self.assertEqual((p["slot"], p["name"], p["mismatches"], p["listed"]), ("20A", "Texas Blues", 0, True))
        self.assertTrue(all(d["ok"] for d in p["diff"]))
        self.assertIn("Scene labels are not written", r["message"])
        names = [c[0] for c in self.dev.calls]
        # order: checks before any write; save last
        self.assertLess(names.index("list_my_presets"), names.index("set_routing"))
        self.assertLess(names.index("setlist_position"), names.index("set_routing"))
        self.assertEqual(names.count("save_to_slot"), 1)
        self.assertEqual(self.dev.slots[152], "Texas Blues")
        # scene-varying GAIN wrote all eight scenes; Spring bypass per scene (G6)
        self.assertIn(("set_param_scenes", 0, 0, 0, [0.2, 0.8] + [0.2] * 6), self.dev.calls)
        self.assertIn(("set_block_bypass", 0, 4, [False, True] + [False] * 6), self.dev.calls)
        self.assertNotIn("set_block_bypass", [c[0] for c in self.dev.calls if c[0] == "set_block_bypass" and c[2] != 4])
        # owned-slot ledger
        self.assertTrue(self.agent.owned.owned(152, "Texas Blues"))
        jid2, _ = self.queue("list_slots")
        self.poll()
        self.assertTrue(self.result(jid2)["slots"]["slots"][152]["owned"])
        # informational, signed bucket copy
        o = json.loads(contract.open_envelope(self.cfg.keys, contract.TAG_OWNED_SLOTS,
                                              self.store.objects[contract.OWNED_SLOTS_KEY]))
        self.assertEqual([(e["slot"], e["name"]) for e in o["slots"]], [("20A", "Texas Blues")])

    def test_owned_slots_copy_is_never_read(self):
        self.store.objects[contract.OWNED_SLOTS_KEY] = b"forged"
        self.dev.slots = {152: "Texas Blues"}  # a preset the bridge didn't make, with the same name
        jid, _ = self.queue("list_slots")
        self.poll()
        self.assertFalse(self.result(jid)["slots"]["slots"][152]["owned"])
        self.assertNotIn(("get", contract.OWNED_SLOTS_KEY), self.store.ops)

    def test_ledger_taken_before_device_contact(self):
        seen = {}

        def check(fake):
            text = (self.home / "executed_jobs.jsonl").read_text()
            seen["taken"] = '"event":"taken"' in text
            seen["finished"] = '"event":"finished"' in text
        self.dev.on_connect = check
        self.push()
        self.assertEqual(seen, {"taken": True, "finished": False})

    def assert_no_writes(self):
        self.assertEqual(self.dev.write_calls(), [])

    def test_slot_not_allowed_refused_before_connecting(self):
        self.agent = self.make_agent(config(QC_AGENT_ALLOWED_SLOTS="1A-19H,21A"))
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "slot_not_allowed"))
        self.assertNotIn(("connect",), self.dev.calls)

    def test_unknown_export_format_refused(self):
        args = {"slot": "20A", "name": "X", "source": {"preset_id": "p", "guitar": "g"},
                "export": {"format": "qc2-device-export/2", "payload": {}}}
        r = self.push(args=args)
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "unsupported_arg"))
        self.assertNotIn(("connect",), self.dev.calls)

    def test_invalid_export_refused_before_connecting(self):
        e = export()
        e["payload"]["rows"][0]["blocks"][0]["params"][0]["wire"] = 1.5
        args = {"slot": "20A", "name": "X", "source": {"preset_id": "p", "guitar": "g"}, "export": e}
        r = self.push(args=args)
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "validation"))
        self.assertIn("outside 0..1", r["message"])
        self.assertNotIn(("connect",), self.dev.calls)

    def test_firmware_mismatch_no_writes(self):
        self.dev.fw = "4.1.1"
        r = self.push()
        self.assertEqual(r["reason_code"], "firmware_mismatch")
        self.assert_no_writes()

    def test_occupied_slot_is_never_overwritten(self):
        self.dev.slots = {152: "My favourite tone"}
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "slot_not_empty"))
        self.assertIn("My favourite tone", r["message"])
        self.assert_no_writes()

    def test_unsafe_names_refused_before_connecting(self):
        for name in ("../../../opt/x", "a/b", "a\\b", ".hidden", "a..b", "x:y", 'q"q', "a*b"):
            with self.subTest(name):
                self.dev.calls.clear()
                args = {"slot": "20A", "name": name, "source": {"preset_id": "p", "guitar": "g"}, "export": export()}
                r = self.push(args=args)
                self.assertEqual((r["status"], r["reason_code"]), ("refused", "validation"))
                self.assertNotIn(("connect",), self.dev.calls)

    def test_duplicate_name_elsewhere_refused(self):
        self.dev.slots = {19: " texas BLUES "}
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "validation"))
        self.assertIn("3D", r["message"])
        self.assert_no_writes()

    def test_unknown_device_type_refused(self):
        self.dev.dtype = None
        r = self.push()
        self.assertEqual(r["reason_code"], "firmware_mismatch")
        self.assert_no_writes()

    def test_space_named_preset_counts_as_occupied(self):
        self.dev.slots = {152: "   "}
        r = self.push()
        self.assertEqual(r["reason_code"], "slot_not_empty")
        self.assert_no_writes()

    def test_owned_slot_is_not_overwritten_in_v1(self):
        self.push()
        self.dev.calls.clear()
        r = self.push()  # same slot again: the bridge's own preset is there now
        self.assertEqual(r["reason_code"], "slot_not_empty")
        self.assert_no_writes()

    def test_target_slot_must_be_open(self):
        self.dev.open_position = 0
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "unsupported_arg"))
        self.assertIn("Select slot 20A", r["message"])
        self.assert_no_writes()

    def test_factory_folder_open_is_refused(self):
        self.dev.open_folder = "/opt/neuraldsp/factory"
        r = self.push()
        self.assertEqual(r["reason_code"], "unsupported_arg")
        self.assert_no_writes()

    def test_live_grid_must_be_blank(self):
        self.dev.grid = {0: {"model_id": 1, "params": {}, "bypass": [False] * 8}}
        r = self.push()
        self.assertEqual(r["reason_code"], "slot_not_empty")
        self.assert_no_writes()

    def test_expiry_rechecked_before_first_write(self):
        jid, key = self.queue("push_preset", ttl_s=60)
        clock = self.clock

        def tick(name):
            if name == "read_current_preset":
                clock.advance(120)  # the job expires while the unit is being checked
        self.dev.on_call = tick
        self.poll()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"]), ("expired", "expired"))
        self.assert_no_writes()

    def test_readback_mismatch_does_not_save(self):
        self.dev.corrupt_readback = True
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "readback_mismatch"))
        self.assertGreater(r["push"]["mismatches"], 0)
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_missing_echo_does_not_save(self):
        self.dev.no_echo_cols = {2}
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_slot_filled_during_build_does_not_save(self):
        def fill(name):
            if name == "read_current_preset" and any(c[0] == "set_routing" for c in self.dev.calls):
                self.dev.slots[152] = "Someone else"
        self.dev.on_call = fill
        r = self.push()
        self.assertEqual(r["reason_code"], "slot_not_empty")
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_unit_gone_mid_push(self):
        self.dev.gone_after = 3
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "unit_absent"))
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])
        self.assertTrue(self.agent.stop.is_set())

    def test_save_not_yet_listed(self):
        self.dev.listing_lag = 99
        r = self.push()
        self.assertEqual(r["status"], "succeeded")
        self.assertFalse(r["push"]["listed"])
        self.assertIn("Check 20A on the unit", r["message"])

    def test_disconnect_always_called(self):
        self.dev.no_echo_cols = {0}
        self.push()
        self.assertEqual(self.dev.calls[-1], ("disconnect",))
        self.assertIn(("end_push",), self.dev.calls)


class Queue(Base):
    def test_forgery_deleted_without_a_result(self):
        jid, key, data = job("list_slots", key_hex=NEXT_HEX)  # unknown kid
        self.store.objects[key] = data
        self.poll()
        self.assertNotIn(key, self.store.objects)
        self.assertNotIn(f"jobs/done/{jid}.json", self.store.objects)
        self.assertEqual(self.dev.calls, [])

    def test_tampered_job_deleted(self):
        jid, key = self.queue("list_slots")
        env = json.loads(self.store.objects[key])
        env["payload"] = env["payload"].replace("list_slots", "read_preset")
        self.store.objects[key] = json.dumps(env).encode()
        self.poll()
        self.assertNotIn(key, self.store.objects)
        self.assertEqual(self.dev.calls, [])

    def test_oversized_listing_deleted_without_reading(self):
        key = "jobs/pending/" + "0" * 8 + "-0000-4000-8000-" + "0" * 12 + ".json"
        self.store.objects[key] = b"x" * (contract.MAX_JOB_BYTES + 1)
        self.poll()
        self.assertNotIn(key, self.store.objects)
        self.assertNotIn(("get", key), self.store.ops)

    def test_bad_key_skipped_not_deleted(self):
        self.store.objects["jobs/pending/../x.json"] = b"{}"
        self.poll()
        self.assertIn("jobs/pending/../x.json", self.store.objects)

    def test_malformed_keys_cannot_starve_real_jobs(self):
        for i in range(agent_mod.MAX_SCAN + 10):
            self.store.objects[f"jobs/pending/-{i:03d}.json"] = b"{}"  # sort before every UUID key
        jid, key = self.queue("list_slots")
        self.poll()
        self.assertEqual(self.result(jid)["status"], "succeeded")

    def test_replay_from_ledger(self):
        jid, key = self.queue("list_slots")
        self.poll()
        first = self.store.objects[f"jobs/done/{jid}.json"]
        _, key2, data = job("list_slots", job_id=jid)
        self.store.objects[key2] = data
        self.dev.calls.clear()
        self.poll()
        self.assertNotIn(key2, self.store.objects)
        self.assertEqual(self.store.objects[f"jobs/done/{jid}.json"], first, "no second result")
        self.assertEqual(self.dev.calls, [])

    def test_replay_from_done_object(self):
        jid, key = self.queue("list_slots")
        self.store.objects[f"jobs/done/{jid}.json"] = b"previous"
        self.poll()
        self.assertNotIn(key, self.store.objects)
        self.assertEqual(self.store.objects[f"jobs/done/{jid}.json"], b"previous")
        self.assertEqual(self.dev.calls, [])

    def test_expired_job_gets_expired_result_and_no_device_contact(self):
        jid, key = self.queue("push_preset", created=T0 - dt.timedelta(minutes=20))
        self.poll()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"]), ("expired", "expired"))
        self.assertEqual(self.dev.calls, [])
        self.assertTrue(self.agent.ledger.contains(jid))

    def test_oldest_first(self):
        late, _ = self.queue("list_slots", created=T0 - dt.timedelta(seconds=10))
        early, _ = self.queue("list_slots", created=T0 - dt.timedelta(seconds=60))
        self.poll()
        ledger = (self.home / "executed_jobs.jsonl").read_text()
        self.assertLess(ledger.index(early), ledger.index(late))

    def test_misrouted_bucket(self):
        jid, key = self.queue("list_slots", bucket="another-bucket")
        self.poll()
        self.assertNotIn(key, self.store.objects)
        self.assertNotIn(f"jobs/done/{jid}.json", self.store.objects)

    def test_rotation_accepts_next_key(self):
        self.agent = self.make_agent(config(DEVICE_JOB_KEY=NEXT_HEX, DEVICE_JOB_KEY_NEXT=KEY_HEX))
        jid, key = self.queue("list_slots")  # signed with the old (now NEXT) key
        self.poll()
        r = open_result(self.store, jid, key_hex=NEXT_HEX)  # results signed with the current key
        self.assertEqual(r["status"], "succeeded")

    def test_upload_failure_is_retried_from_ledger_bytes(self):
        jid, key = self.queue("list_slots")
        self.store.fail["put"] = S3Error("put", 500)
        self.poll()
        self.assertNotIn(f"jobs/done/{jid}.json", self.store.objects)
        self.assertIn(key, self.store.objects, "pending kept until the result is uploaded")
        del self.store.fail["put"]
        stored = self.agent.ledger.finished[jid]["result"].encode()
        self.agent.retry_uploads()
        self.assertEqual(self.store.objects[f"jobs/done/{jid}.json"], stored)
        self.assertNotIn(key, self.store.objects)
        self.dev.calls.clear()
        self.poll()  # the pending object is gone and the ledger has it: nothing re-runs
        self.assertEqual(self.dev.calls, [])

    def test_job_crash_still_gets_a_result(self):
        def boom(name):
            raise ZeroDivisionError("bug")
        self.dev.on_call = boom
        jid, _ = self.queue("list_slots")
        self.poll()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))


class Recovery(Base):
    def test_taken_without_finished_reports_interrupted_and_never_retries(self):
        jid, key = self.queue("push_preset")
        self.agent.ledger.take(jid, "push_preset", contract.format_time(T0))
        agent = self.make_agent()  # "restart"
        agent.recover()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"], r["kind"]), ("failed", "interrupted", "push_preset"))
        self.assertNotIn(key, self.store.objects)
        self.assertEqual(self.dev.calls, [])

    def test_finished_without_done_reuploads_stored_bytes(self):
        jid, key = self.queue("list_slots")
        self.store.fail["put"] = S3Error("put", None, "URLError")  # crash window: finished, never uploaded
        self.poll()
        stored = self.agent.ledger.finished[jid]["result"].encode()
        del self.store.fail["put"]
        agent = self.make_agent()  # restart; the in-memory retry list is gone
        self.clock.advance(3600)
        agent.recover()
        self.assertEqual(self.store.objects[f"jobs/done/{jid}.json"], stored, "never a newly minted result")
        self.assertNotIn(key, self.store.objects)
        self.assertIn(jid, self.make_agent().ledger.uploaded)

    def test_restart_does_not_recheck_confirmed_uploads(self):
        jid, _ = self.queue("list_slots")
        self.poll()
        self.store.ops.clear()
        self.make_agent().recover()
        self.assertNotIn(("head", f"jobs/done/{jid}.json"), self.store.ops)

    def test_recovery_ignores_the_bucket_for_decisions(self):
        # A done object for an unfinished ledger entry doesn't stop the interrupted report from the ledger.
        jid, _ = self.queue("push_preset")
        self.agent.ledger.take(jid, "push_preset", contract.format_time(T0))
        self.store.objects[f"jobs/done/{jid}.json"] = b"forged"
        self.make_agent().recover()
        self.assertEqual(self.result(jid)["reason_code"], "interrupted")

    def test_torn_final_line_is_ignored(self):
        jid, _ = self.queue("list_slots")
        self.poll()
        with open(self.home / "executed_jobs.jsonl", "ab") as f:
            f.write(b'{"event":"taken","job_id":"abc')
        agent = self.make_agent()
        self.assertTrue(agent.ledger.contains(jid))
        agent.ledger.take("11111111-1111-4111-8111-111111111111", "list_slots", "2026-11-05T20:00:00Z")
        self.assertIn("11111111-1111-4111-8111-111111111111", self.make_agent().ledger.taken)

    def test_corrupt_middle_line_stops_start(self):
        from qc_agent.ledger import LedgerError
        (self.home / "executed_jobs.jsonl").write_text('garbage\n{"event":"taken","job_id":"a","kind":"x","at":"t"}\n')
        with self.assertRaises(LedgerError):
            self.make_agent()


class HeartbeatAndExit(Base):
    def hb(self):
        data = self.store.objects[contract.HEARTBEAT_KEY]
        self.assertLessEqual(len(data), contract.MAX_HEARTBEAT_BYTES)
        return json.loads(contract.open_envelope(self.cfg.keys, contract.TAG_HEARTBEAT, data))

    def test_heartbeat_fields(self):
        self.agent.check_presence()
        self.agent.write_heartbeat()
        h = self.hb()
        self.assertEqual(h["type"], "qc2-device-heartbeat")
        self.assertEqual(h["bucket"], BUCKET)
        self.assertEqual(h["state"], "idle")
        self.assertEqual(h["written_at"], "2026-11-05T20:00:00Z")
        self.assertEqual(h["idle_exit_at"], "2026-11-05T20:30:00Z")
        self.assertEqual(h["allowed_slots"], ["1A-32H"])
        self.assertEqual(h["setlist"], "My Presets")
        self.assertEqual(h["unit"], {"present": True, "firmware": None, "firmware_seen_at": None})
        self.assertTrue(1 <= h["poll_interval_s"] <= 300)

    def test_idle_exit_with_exiting_heartbeat(self):
        clock = self.clock

        def sleep(s):
            clock.advance(60)
        self.agent._sleep = sleep
        code = self.agent.run()
        self.assertEqual(code, 0)
        self.assertIn("no job for 30 minutes", self.agent.exit_reason)
        self.assertEqual(self.hb()["state"], "exiting")

    def test_unit_disconnect_exits(self):
        calls = {"n": 0}

        def sleep(s):
            calls["n"] += 1
            if calls["n"] == 2:
                self.dev.present = False
        self.agent._sleep = sleep
        code = self.agent.run()
        self.assertEqual(code, agent_mod.EXIT_UNIT_GONE)
        self.assertIn("switched off or unplugged", self.agent.exit_reason)
        self.assertEqual(self.hb()["state"], "exiting")

    def test_absent_at_start_keeps_waiting(self):
        self.dev.present = False
        clock = self.clock
        self.agent._sleep = lambda s: clock.advance(600)
        self.agent.run()
        self.assertIn("no job", self.agent.exit_reason)

    def test_job_resets_idle_timer(self):
        self.clock.advance(25 * 60)
        jid, _ = self.queue("list_slots", created=self.clock.t)
        self.poll()
        self.assertEqual(self.agent.idle_exit_at(), self.clock.t + dt.timedelta(minutes=30))

    def test_busy_heartbeat_during_job(self):
        states = []
        orig = self.agent.write_heartbeat

        def record(state=None):
            states.append((state or self.agent.state, self.agent.current_job))
            return orig(state)
        self.agent.write_heartbeat = record
        jid, _ = self.queue("list_slots")
        self.poll()
        self.assertIn(("busy", jid), states)
        self.assertEqual(states[-1], ("idle", None))


if __name__ == "__main__":
    unittest.main()
