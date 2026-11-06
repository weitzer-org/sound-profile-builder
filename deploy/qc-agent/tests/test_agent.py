"""The job loop end to end: in-memory bucket, fake unit, fixed clock.

Covers every refusal path of contract section 8 step 11 and the plan's device rules,
crash recovery from local state only, the heartbeat, and the exit rules. Proves the
agent's logic and guards; says nothing about the real unit (that's Phase 0 / P6).
"""
import datetime as dt
import json
import tempfile
import unittest
from pathlib import Path

from helpers import BUCKET, KEY_HEX, NEXT_HEX, T0, Clock, catalog, config, export, job, open_result
from qc_agent import agent as agent_mod
from qc_agent import contract, jobs
from qc_agent.agent import Agent
from qc_agent.s3 import MemoryStore, StoreError
from fake_device import FakeDevice

def setUpModule():
    # Pacing off for this module only; restored afterwards so other modules see the real values.
    from unittest import mock
    for name in ("SAVE_LIST_INTERVAL_S", "PACE_S"):
        p = mock.patch.object(jobs, name, 0)
        p.start()
        unittest.addModuleCleanup(p.stop)


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

    def test_preset_switched_during_read(self):
        def switch(name):
            if name == "read_current_preset":
                self.dev.open_position = 3
        self.dev.on_call = switch
        jid, _ = self.queue("read_preset")
        self.poll()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))

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
        self.assertNotIn(("read_directory",), self.dev.calls)

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
        self.assertEqual((p["slot"], p["name"], p["mismatches"]), ("20A", "Texas Blues", 0))
        self.assertEqual(set(p), {"slot", "name", "mismatches", "cpu_load", "diff"})  # contract section 9 only
        self.assertEqual([d for d in p["diff"] if not d["ok"]], [])
        for d in p["diff"]:
            self.assertEqual(set(d), {"row", "col", "param_index", "expected", "readback", "ok"})
        self.assertNotIn("Scene labels", r["message"])  # the P2 export carries none
        names = [c[0] for c in self.dev.calls]
        # order: checks before any write; save last
        for n in ("read_directory", "setlist_position", "set_routing", "save_to_slot"):
            self.assertIn(n, names)
        self.assertLess(names.index("read_directory"), names.index("set_routing"))
        self.assertLess(names.index("setlist_position"), names.index("set_routing"))
        self.assertEqual(names.count("save_to_slot"), 1)
        self.assertEqual(self.dev.slots[152], "Texas Blues")
        # scene-varying GAIN wrote all eight scenes; Spring bypass per scene (G6)
        self.assertIn(("set_param_scenes", 0, 0, 0, [0.2, 0.8] + [0.2] * 6), self.dev.calls)
        # all eight bypass flags on every block, through its bypass control at param_count + 1 (G3's N)
        self.assertEqual([c for c in self.dev.calls if c[0] == "set_block_bypass"],
                         [("set_block_bypass", 0, 0, [False] * 8, 4), ("set_block_bypass", 0, 1, [False] * 8, 8),
                          ("set_block_bypass", 0, 2, [False] * 8, 22), ("set_block_bypass", 0, 3, [False] * 8, 6),
                          ("set_block_bypass", 0, 4, [False, True] + [False] * 6, 6)])
        # G1: place every block, then one read, then the first value write
        places = [i for i, n in enumerate(names) if n == "add_block"]
        values = [i for i, n in enumerate(names) if n in ("set_param", "set_param_scenes", "set_block_bypass")]
        self.assertEqual(len(places), 5)
        self.assertIn("read_current_preset", names[places[-1]:values[0]])
        # the open slot is re-read before every write
        writes = [i for i, n in enumerate(names) if n in ("set_routing", "add_block", "set_param", "set_param_scenes",
                                                          "set_block_bypass")]
        for i in writes:
            self.assertEqual(names[i - 1], "setlist_position", (i, names[i]))
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
        e["payload"]["rows"][0]["blocks"][0]["params"][0]["wire"][0] = 1.5
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
        self.assertIn("Slot 3D already has a preset named", r["message"])
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

    def test_incomplete_bypass_readback_is_a_mismatch(self):
        orig = self.dev.read_current_preset

        def short_bypass():
            p = orig()
            p["bypass"][0][1] = [False] * 3  # the Brit 2203 (active in every scene) reads three flags
            return p
        self.dev.read_current_preset = short_bypass
        r = self.push()
        self.assertEqual(r["reason_code"], "readback_mismatch")
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

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

    def test_name_taken_during_build_keeps_the_reason(self):
        def fill(name):
            if name == "read_current_preset" and any(c[0] == "set_routing" for c in self.dev.calls):
                self.dev.slots[7] = "Texas Blues"
        self.dev.on_call = fill
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "validation"))
        self.assertIn("nothing was saved", r["message"])
        self.assertIn("1H already has a preset named", r["message"])
        self.assertIsNotNone(r["push"])
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

    def test_save_never_seen_is_failed_and_the_slot_stays_owned(self):
        self.dev.listing_lag = 99
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("may still land", r["message"])
        self.assertIn("check 20A on the unit", r["message"])
        self.assertEqual(r["push"]["mismatches"], 0)
        self.assertEqual([c[0] for c in self.dev.calls].count("read_directory"), 2 + jobs.SAVE_LIST_POLLS)
        self.assertTrue(self.agent.owned.owned(152, "Texas Blues"))  # a late save is still the bridge's own
        o = json.loads(contract.open_envelope(self.cfg.keys, contract.TAG_OWNED_SLOTS,
                                              self.store.objects[contract.OWNED_SLOTS_KEY]))
        self.assertEqual([(e["slot"], e["name"]) for e in o["slots"]], [("20A", "Texas Blues")])

    def test_save_seen_on_the_last_poll_succeeds(self):
        self.dev.listing_lag = jobs.SAVE_LIST_POLLS - 1
        r = self.push()
        self.assertEqual(r["status"], "succeeded", r["message"])

    def test_unit_gone_after_the_save_is_failed_and_owned(self):
        def gone(name):
            if name == "save_to_slot":
                self.dev.gone_after = self.dev._alive_calls
        self.dev.on_call = gone
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "unit_absent"))
        self.assertIn("may still land", r["message"])
        self.assertTrue(self.agent.owned.owned(152, "Texas Blues"))
        self.assertTrue(self.agent.stop.is_set())

    def test_oversized_result_still_records_the_saved_slot(self):
        from unittest import mock
        with mock.patch.object(contract, "MAX_RESULT_BYTES", 1500):  # the push result's diff doesn't fit
            r = self.push()
        self.assertEqual((r["status"], r["message"]), ("failed", "The result was too large to return."))
        self.assertTrue(self.agent.owned.owned(152, "Texas Blues"))
        o = json.loads(contract.open_envelope(self.cfg.keys, contract.TAG_OWNED_SLOTS,
                                              self.store.objects[contract.OWNED_SLOTS_KEY]))
        self.assertEqual([(e["slot"], e["name"]) for e in o["slots"]], [("20A", "Texas Blues")])

    def no_value_writes(self):
        self.assertEqual([c for c in self.dev.calls if c[0] in ("set_param", "set_param_scenes", "set_block_bypass")],
                         [])
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_wrong_param_count_stops_before_any_value(self):
        for skew in (1, -1):
            with self.subTest(skew):
                self.dev = FakeDevice()
                self.agent = self.make_agent()
                self.dev.param_count_skew = {2: skew}  # the cab reads 23 (or 21) slots, not param_count + 1 = 22
                r = self.push()
                self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
                self.assertIn(f"reads {22 + skew} parameters; the catalog expects 22", r["message"])
                self.assertIn("No parameter was written", r["message"])
                self.no_value_writes()

    def test_wrong_model_at_layout_stops_before_any_value(self):
        def swap(name):
            if name == "read_current_preset" and 1 in self.dev.grid:
                self.dev.grid[1]["model_id"] = 1002
        self.dev.on_call = swap
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("column 1 holds model 1002", r["message"])
        self.no_value_writes()

    def test_layout_read_timeout_stops_before_any_value(self):
        orig, reads = self.dev.read_current_preset, {"n": 0}

        def read():
            reads["n"] += 1
            return None if reads["n"] == 2 else orig()  # 1: the blank check, 2: the layout read
        self.dev.read_current_preset = read
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("after placing the blocks", r["message"])
        self.no_value_writes()

    def test_block_swapped_on_the_unit_mid_build_stops_before_its_writes(self):
        def swap(name):
            if name == "set_param" and self.dev.grid.get(2, {}).get("model_id") == 12045:
                self.dev.grid[2]["model_id"] = 12046  # the owner picks another cab in column 2
        self.dev.on_call = swap
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("The grid changed on the unit during the build", r["message"])
        self.assertIn("column 2 holds model 12046", r["message"])
        writes = [c for c in self.dev.calls if c[0] in ("set_param", "set_param_scenes", "set_block_bypass")]
        self.assertTrue(writes and all(c[2] < 2 for c in writes))  # nothing written to the swapped block
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_each_block_is_rechecked_and_confirmed_before_its_writes(self):
        self.push()
        names = [c[0] for c in self.dev.calls]
        confirms = [c for c in self.dev.calls if c[0] == "confirm_layout"]
        self.assertEqual(confirms, [("confirm_layout", 0, 4), ("confirm_layout", 1, 8), ("confirm_layout", 2, 22),
                                    ("confirm_layout", 3, 6), ("confirm_layout", 4, 6)])
        for c in confirms:
            i = self.dev.calls.index(c)
            self.assertEqual(names[i - 1], "read_current_preset")
            first = next(k for k, x in enumerate(self.dev.calls)
                         if x[0] in ("set_param", "set_param_scenes", "set_block_bypass") and x[2] == c[1])
            self.assertLess(i, first)

    def test_switching_presets_between_value_writes_stops(self):
        def switch(name):
            if name == "set_param":
                self.dev.open_position = 0
        self.dev.on_call = switch
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("Another preset was selected", r["message"])
        values = [c[0] for c in self.dev.calls if c[0] in ("set_param", "set_param_scenes", "set_block_bypass")]
        self.assertEqual(values, ["set_param_scenes", "set_param"])  # the write after the switch never ran
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_unit_gone_between_value_writes_stops(self):
        def gone(name):
            if name == "set_param":
                self.dev.gone_after = self.dev._alive_calls
        self.dev.on_call = gone
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "unit_absent"))
        self.assertEqual([c[0] for c in self.dev.calls if c[0] in ("set_param", "set_param_scenes", "set_block_bypass")],
                         ["set_param_scenes", "set_param"])
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_defaults_are_logged_never_failed(self):
        with self.assertLogs("qc_agent", level="INFO") as logs:
            r = self.push()
        self.assertEqual(r["status"], "succeeded", r["message"])
        differ = [m for m in logs.output if "default differs" in m]
        # the fake unit starts every control at wire 0.0, so controls whose default isn't 0.0 differ
        self.assertTrue(differ)
        self.assertIn(f"{len(differ)} control(s) the preset leaves at default read back differently", r["message"])
        self.assertTrue(any("not compared" in m or "default differs" in m for m in logs.output))

    def test_switching_presets_mid_build_stops(self):
        def switch(name):
            if name == "add_block":
                self.dev.open_position = 0
        self.dev.on_call = switch
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("Another preset was selected", r["message"])
        self.assertEqual([c[0] for c in self.dev.calls].count("add_block"), 1)
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_grid_edited_after_verification_is_not_saved(self):

        def edit_on_unit(name):
            if name == "read_directory" and any(c[0] == "set_routing" for c in self.dev.calls):
                self.dev.grid[0]["params"][1] = [0.9] * 8  # the owner turns TREBLE during the save checks
        self.dev.on_call = edit_on_unit
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "readback_mismatch"))
        self.assertIn("changed after it was verified", r["message"])
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_owned_ledger_failure_keeps_the_success(self):
        def boom(*a, **k):
            raise OSError("disk full")
        self.agent.owned.record = boom
        r = self.push()
        self.assertEqual(r["status"], "succeeded")
        self.assertEqual(r["push"]["mismatches"], 0)

    def test_a_ledger_bug_after_the_save_keeps_the_success_and_logs_the_traceback(self):
        def boom(*a, **k):
            raise TypeError("ledger bug")
        self.agent.owned.record = boom
        with self.assertLogs("qc_agent", level="ERROR") as logs:
            r = self.push()
        self.assertEqual(r["status"], "succeeded")
        rec = next(x for x in logs.records if "could not record the owned slot" in x.getMessage())
        self.assertIsNotNone(rec.exc_info)  # the bug is visible in the log, with its traceback

    def test_selection_change_just_before_the_save_is_refused(self):
        def switch(name):
            # the grid re-read that follows the pre-save listing (the second), whatever ran before it
            if name == "read_current_preset" and self.dev._listings >= 2:
                self.dev.open_position = 0
        self.dev.on_call = switch
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("just before the save", r["message"])
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

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
        for i in range(1500):  # more than one ListObjectsV2 page
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
        taken = [e["job_id"] for e in map(json.loads, (self.home / "executed_jobs.jsonl").read_text().splitlines())
                 if e.get("event") == "taken"]
        self.assertLess(taken.index(early), taken.index(late))

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
        self.store.fail["put"] = StoreError("put", 500)
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
        self.store.fail["put"] = StoreError("put", None, "URLError")  # crash window: finished, never uploaded
        self.poll()
        stored = self.agent.ledger.finished[jid]["result"].encode()
        del self.store.fail["put"]
        agent = self.make_agent()  # restart; the in-memory retry list is gone
        self.clock.advance(3600)
        agent.recover()
        self.assertEqual(self.store.objects[f"jobs/done/{jid}.json"], stored, "never a newly minted result")
        self.assertNotIn(key, self.store.objects)
        self.assertIn(jid, self.make_agent().ledger.uploaded)

    def test_uploaded_result_bodies_are_not_kept_in_memory(self):
        jid, _ = self.queue("list_slots")
        self.poll()
        self.assertIsNone(self.agent.ledger.finished[jid]["result"])
        self.assertIsNone(self.make_agent().ledger.finished[jid]["result"])
        self.assertIn('"result":', (self.home / "executed_jobs.jsonl").read_text(), "the file keeps it")

    def test_restart_does_not_recheck_confirmed_uploads(self):
        jid, _ = self.queue("list_slots")
        self.poll()
        self.store.ops.clear()
        self.make_agent().recover()
        self.assertNotIn(("head", f"jobs/done/{jid}.json"), self.store.ops)

    def test_recovery_republishes_over_an_existing_done_object(self):
        jid, key = self.queue("list_slots")
        self.store.fail["put"] = StoreError("put", None, "URLError")
        self.poll()
        stored = self.agent.ledger.finished[jid]["result"].encode()
        del self.store.fail["put"]
        self.store.objects[f"jobs/done/{jid}.json"] = b"someone else's object"
        self.make_agent().recover()
        self.assertEqual(self.store.objects[f"jobs/done/{jid}.json"], stored)

    def test_pending_kept_while_its_result_waits_to_upload(self):
        jid, key = self.queue("list_slots")
        self.store.fail["put"] = StoreError("put", None, "URLError")
        self.poll()
        self.poll()  # the job is in the ledger now (a replay), but its result hasn't reached the bucket
        self.assertIn(key, self.store.objects)
        del self.store.fail["put"]
        self.agent.retry_uploads()  # what the run loop does before each scan
        self.assertNotIn(key, self.store.objects)
        self.assertIn(f"jobs/done/{jid}.json", self.store.objects)

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

    def test_crash_while_marking_a_torn_line(self):
        jid, _ = self.queue("list_slots")
        self.poll()
        with open(self.home / "executed_jobs.jsonl", "ab") as f:
            f.write(b'{"event":"taken","job_id":"abc\n{"event": "to')  # torn line, then a torn marker
        self.assertTrue(self.make_agent().ledger.contains(jid))

    def test_crash_after_the_separator_then_append_then_restart(self):
        jid, _ = self.queue("list_slots")
        self.poll()
        with open(self.home / "executed_jobs.jsonl", "ab") as f:
            f.write(b'{"event":"taken","job_id":"abc\n')  # torn entry, then a crash right after the newline
        agent = self.make_agent()
        new = "22222222-2222-4222-8222-222222222222"
        agent.ledger.take(new, "list_slots", "2026-11-05T20:00:00Z")  # must mark the torn line first
        again = self.make_agent()
        self.assertIn(new, again.ledger.taken)
        self.assertTrue(again.ledger.contains(jid))

    def test_unrelated_malformed_suffix_is_corruption(self):
        from qc_agent.ledger import LedgerError
        jid, _ = self.queue("list_slots")
        self.poll()
        with open(self.home / "executed_jobs.jsonl", "ab") as f:
            f.write(b'{"event":"taken","job_id":"abc\nnot a torn marker\n')
        with self.assertRaisesRegex(LedgerError, "is corrupt"):
            self.make_agent()

    def test_corrupt_middle_line_stops_start(self):
        from qc_agent.ledger import LedgerError
        (self.home / "executed_jobs.jsonl").write_text('garbage\n{"event":"taken","job_id":"a","kind":"x","at":"t"}\n')
        with self.assertRaisesRegex(LedgerError, "entry 1 is corrupt"):
            self.make_agent()


class ShortWrites(unittest.TestCase):
    def test_partial_os_writes_still_append_whole_lines(self):
        from unittest import mock
        from qc_agent import ledger as ledger_mod
        real = ledger_mod.os.write
        with tempfile.TemporaryDirectory() as d, \
                mock.patch.object(ledger_mod.os, "write", side_effect=lambda fd, b: real(fd, bytes(b)[:7])):
            path = Path(d) / "executed_jobs.jsonl"
            led = ledger_mod.JobLedger(path)
            led.take("job-1", "list_slots", "2026-11-06T00:00:00Z")
            led.finish("job-1", "2026-11-06T00:00:01Z", b"result-envelope")
            again = ledger_mod.JobLedger(path)
        self.assertTrue(again.contains("job-1"))
        self.assertEqual(again.finished["job-1"]["result"], "result-envelope")


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


class DefaultsLog(unittest.TestCase):
    """jobs._log_defaults: unwritten controls compared with the catalog default, logged, never failed."""

    def preset(self, plan, value_for):
        from qc_agent.export_validator import default_wire
        cat = catalog()
        blocks = []
        for b in plan.blocks:
            m = cat.models[b.model_id]
            params = []
            for i in range(b.param_count + 1):
                cp = m["params_by_index"].get(i)
                v = value_for(cp, default_wire(cp) if cp else None)
                params.append({"index": i, "values": [v] * 8, "scene_mode": False})
            blocks.append({"col": b.col, "model_id": b.model_id, "params": params})
        return {"chains": [{"row": 0, "in_port": 1, "out_port": 19, "blocks": blocks}], "bypass": None}

    def test_at_default_logs_nothing_and_counts_zero(self):
        from qc_agent.export_validator import validate
        plan = validate(export(), catalog(), "4.1.0")
        pr = self.preset(plan, lambda cp, dw: dw if dw is not None else 0.0)
        with self.assertNoLogs("qc_agent", level="WARNING"):
            self.assertEqual(jobs._log_defaults(plan, pr, catalog()), 0)

    def test_a_moved_control_is_logged_and_counted(self):
        from qc_agent.export_validator import default_wire, validate
        plan = validate(export(), catalog(), "4.1.0")
        amp = plan.blocks[1]  # Brit 2203: GAIN (0) and OUTPUT (6) written, the rest left at default
        written = {p.index for p in amp.params}
        target = next(cp for cp in sorted(catalog().models[amp.model_id]["params"], key=lambda q: q["index"])
                      if cp["index"] not in written and default_wire(cp) is not None)

        def value(cp, dw):
            if cp is target:
                return 1.0 if dw < 0.5 else 0.0
            return dw if dw is not None else 0.0
        pr = self.preset(plan, value)
        with self.assertLogs("qc_agent", level="WARNING") as logs:
            self.assertEqual(jobs._log_defaults(plan, pr, catalog()), 1)
        self.assertEqual(len(logs.output), 1)
        self.assertIn(f"col 1 Brit 2203 {target['name']} [{target['index']}]", logs.output[0])

    def test_a_crash_in_the_defaults_log_doesnt_change_the_push(self):
        from unittest import mock
        dev, store, clock = FakeDevice(), MemoryStore(BUCKET), Clock()
        with tempfile.TemporaryDirectory() as home:
            a = Agent(config(), store, dev, catalog(), Path(home), now_fn=clock, sleep=lambda s: None)
            a.ctx.sleep = lambda s: None
            jid, key, data = job("push_preset")
            store.objects[key] = data
            with mock.patch.object(jobs, "_log_defaults", side_effect=RuntimeError("boom")), \
                    self.assertLogs("qc_agent", level="ERROR"):
                for k, d in a.scan():
                    a.run_job(k, d)
            self.assertEqual(open_result(store, jid)["status"], "succeeded")

    def test_never_raises_on_a_short_or_odd_read(self):
        from qc_agent.export_validator import validate
        plan = validate(export(), catalog(), "4.1.0")
        for pr in ({}, {"chains": []}, {"chains": [{"row": 0, "blocks": [{"col": 0, "model_id": 1}]}]},
                   {"chains": [{"row": 0, "blocks": [{"col": 0, "model_id": 1,
                                                       "params": [{"index": 1, "values": ["x", None]}]}]}]}):
            with self.subTest(pr=pr):
                jobs._log_defaults(plan, pr, catalog())


class Captures(Base):
    """A push whose export holds a capture block (Go golden 03_British_Invasion: a user capture,
    '66 AC30 JMP OD', in column 3), against the fake unit's capture library (fake_device)."""

    def push(self, golden="03_British_Invasion", **kw):
        from helpers import go_golden
        args = {"slot": "20A", "name": "British Invasion", "source": {"preset_id": "p-9", "guitar": "ES-339"},
                "export": go_golden(golden)}
        jid, _key = self.queue("push_preset", args=args)
        self.poll()
        return self.result(jid)

    def test_capture_push(self):
        from fake_device import KEY_USER
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("succeeded", ""), r["message"])
        self.assertEqual(r["push"]["mismatches"], 0)
        calls = self.dev.calls
        place = [c for c in calls if c[0] == "place_capture"]
        self.assertEqual(place, [("place_capture", 0, 3, 14000, KEY_USER, "66 AC30 JMP OD")])
        self.assertNotIn(("add_block", 0, 3, 14000), calls)            # never placed twice
        names = [c[0] for c in calls]
        i = calls.index(place[0])
        col3_params = [k for k, c in enumerate(calls) if c[0].startswith("set_param") and c[2] == 3]
        self.assertTrue(col3_params and all(k > i for k in col3_params))  # file_name before the controls
        self.assertIn("Captures: col 3 '66 AC30 JMP OD' V1 (user).", r["message"])
        # the capture block gets the same treatment as any block: catalog param_count 6 for 14000, so the
        # layout read expects 7 slots, and its bypass control is 7
        self.assertIn(("set_block_bypass", 0, 3, [False] * 8, 7), calls)
        self.assertTrue(all(c[3] < 6 for c in calls if c[0] in ("set_param", "set_param_scenes") and c[2] == 3))
        self.assertIn("names every placed capture as a dependency", r["message"])
        self.assertEqual(names.count("save_to_slot"), 1)

    def refused_before_writing(self, r, needle):
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "validation"), r["message"])
        self.assertIn(needle, r["message"])
        self.assertEqual(self.dev.write_calls(), [])
        self.assertNotIn("begin_push", [c[0] for c in self.dev.calls])

    def test_missing_capture(self):
        self.dev.captures = [c for c in self.dev.captures if c.name != "66 AC30 JMP OD"]
        self.refused_before_writing(self.push(), "isn't on the unit")

    def test_ambiguous_capture(self):
        from fake_device import capture
        self.dev.captures.append(capture("66 AC30 JMP OD", "f6" * 32, "Downloads", version="1"))
        self.refused_before_writing(self.push(), "won't guess")

    def test_cloud_only_capture(self):
        from fake_device import capture
        self.dev.captures = [c for c in self.dev.captures if c.name != "66 AC30 JMP OD"]
        self.dev.captures.append(capture("66 AC30 JMP OD", "f6" * 32, "Cloud", cloud=True))
        self.refused_before_writing(self.push(), "Cortex Cloud")

    def test_unknown_version(self):
        from fake_device import capture
        self.dev.captures = [c for c in self.dev.captures if c.name != "66 AC30 JMP OD"]
        self.dev.captures.append(capture("66 AC30 JMP OD", "f6" * 32, version="9"))
        self.refused_before_writing(self.push(), "V1 or V2")

    def test_no_capture_folders(self):
        self.dev.captures = []
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertEqual(self.dev.write_calls(), [])

    def test_no_echo_is_not_saved(self):
        self.dev.no_echo_cols = {3}
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("did not confirm placing capture", r["message"])
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_capture_block_wrong_param_count_stops_before_any_value(self):
        self.dev.param_count_skew = {3: 1}
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("column 3 (66 AC30 JMP OD) reads 8 parameters; the catalog expects 7", r["message"])
        self.assertEqual([c for c in self.dev.calls if c[0] in ("set_param", "set_param_scenes", "set_block_bypass")],
                         [])
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_save_never_seen_keeps_the_capture_note(self):
        self.dev.listing_lag = 99
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("Captures: col 3", r["message"])

    def test_file_name_read_back_wrong_is_not_saved(self):
        self.dev.corrupt_file_name = True
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "readback_mismatch"))
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])
        self.assertTrue(all(isinstance(d["expected"], float) for d in r["push"]["diff"]))  # Go's DiffItem

    def test_library_changed_before_save(self):
        def change(fake, n):
            if n == 2:  # the save's listing: the capture now has a different key
                fake.captures = [c for c in fake.captures if c.name != "66 AC30 JMP OD"]
                from fake_device import capture
                fake.captures.append(capture("66 AC30 JMP OD", "f6" * 32, version="1"))
        self.dev.on_listing = change
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("capture library changed", r["message"])
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_dependencies_not_listed(self):
        self.dev.deps_on_save = False
        self.dev.preset_deps = {7: frozenset({"ee" * 32})}  # another preset has dependency lists
        r = self.push()
        self.assertEqual(r["status"], "succeeded")
        self.assertIn("Observation: the unit's listing doesn't name '66 AC30 JMP OD'", r["message"])

    def test_no_dependency_lists_at_all(self):
        self.dev.deps_on_save = False
        r = self.push()
        self.assertEqual(r["status"], "succeeded")
        self.assertIn("shows no dependency lists", r["message"])
