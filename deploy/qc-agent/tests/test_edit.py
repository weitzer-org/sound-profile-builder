"""edit_preset (preset-editing E4): the agent's pre-write checks, the compare-and-swap and, in later
sections, the writes. In-memory bucket, fake unit, fixed clock. Proves the agent's logic and guards;
says nothing about the real unit (E-G1 on a QC-2-owned test slot is the first real check)."""
import copy
import json
import os
import unittest
from unittest import mock

from fake_device import FakeDevice
from helpers import REPO, T0, catalog, config, job
from qc_agent import contract, jobs
from qc_agent import edit_checks
from qc_agent.device import SETLIST_KEY, DeviceError
from edit_fixtures import GAIN_EXPECT, GAIN_WIRE, OTHER_KEY, SNAP, base_plan, edit_args, find_model, rig_unit
from test_agent import Base

# The edit payload of every result a test built, by test id. They are pinned in
# testdata/devicejobs/edit_agent_results.json, which internal/devicejobs parses with ParseEditResultPayload
# (TestAgentEditResultsParse): the agent's results must pass the parser QC-2 reads them with.
# Rewrite the file with UPDATE_GOLDENS=1 (a full run of this module).
RESULTS = {}


def tearDownModule():
    path = REPO / "testdata" / "devicejobs" / "edit_agent_results.json"
    got = dict(sorted(RESULTS.items()))
    if os.environ.get("UPDATE_GOLDENS") == "1" and len(got) >= 25:
        path.write_text(json.dumps({"results": got}, indent=1, sort_keys=True) + "\n", encoding="utf-8")
        return
    want = json.loads(path.read_text(encoding="utf-8"))["results"]
    for name, payload in got.items():
        assert want.get(name) == payload, f"{name}: the agent's edit result differs from edit_agent_results.json (UPDATE_GOLDENS=1)"


def setUpModule():
    for name in ("SAVE_LIST_INTERVAL_S", "PACE_S"):
        p = mock.patch.object(jobs, name, 0)
        p.start()
        unittest.addModuleCleanup(p.stop)


class EditBase(Base):
    EDIT_MODES = "live"

    def setUp(self):
        super().setUp()
        self.set_modes(self.EDIT_MODES)
        rig_unit(self.dev)

    def set_modes(self, modes, **env):
        self.cfg = config(QC_AGENT_EDIT_MODES=modes, **env)
        self.agent = self.make_agent(self.cfg)

    def run_edit(self, plan=None, **kw):
        plan = plan or base_plan()
        jid, key = self.queue("edit_preset", args=edit_args(plan, **kw))
        self.poll()
        r = self.result(jid)
        if r["edit"] is not None:
            RESULTS[self.id().split(".", 1)[1]] = r["edit"]["payload"]
        return jid, r

    def refused_before_unit(self, r, code):
        self.assertEqual((r["kind"], r["status"], r["reason_code"]), ("edit_preset", "refused", code))
        self.assertIsNone(r["edit"])
        self.assertIsNone(r["firmware"])
        self.assertEqual(self.dev.calls, [])


class Config(unittest.TestCase):
    def test_edit_settings_default_to_nothing_allowed(self):
        c = config()
        self.assertEqual((c.edit_modes, c.protected_slots), (frozenset(), None))

    def test_modes_and_protected_slots_parse(self):
        c = config(QC_AGENT_EDIT_MODES=" live , copy ", QC_AGENT_PROTECTED_SLOTS="1A-8H, 32H")
        self.assertEqual(c.edit_modes, frozenset({"live", "copy"}))
        self.assertEqual(c.protected_slots.labels(), ["1A-8H", "32H"])
        self.assertIn("edit_modes=['copy', 'live']", repr(c))

    def test_bad_values_are_errors_not_silent(self):
        from qc_agent import config as cfg
        for env in ({"QC_AGENT_EDIT_MODES": "live,sideways"}, {"QC_AGENT_EDIT_MODES": "LIVE"},
                    {"QC_AGENT_PROTECTED_SLOTS": "99Z"}):
            with self.subTest(env), self.assertRaises(cfg.ConfigError):
                config(**env)


class Opt_in(EditBase):
    EDIT_MODES = ""

    def test_refused_when_the_pc_has_not_switched_editing_on(self):
        jid, r = self.run_edit()
        self.refused_before_unit(r, "edit_not_enabled")
        self.assertTrue(self.agent.ledger.contains(jid))

    def test_live_must_be_listed(self):
        self.set_modes("copy")
        jid, r = self.run_edit()
        self.refused_before_unit(r, "edit_not_enabled")

    def test_overwrite_is_not_honoured_even_when_listed(self):
        self.set_modes("live,copy,overwrite")
        _, r = self.run_edit(mode="overwrite")
        self.refused_before_unit(r, "unsupported_arg")

    def test_copy_must_be_listed_too(self):
        self.set_modes("live")
        _, r = self.run_edit(mode="copy", target_slot="21A", name="Copy")
        self.refused_before_unit(r, "edit_not_enabled")
        self.assertIn("copy", r["message"].lower())
        self.set_modes("copy")   # and listing copy alone does not switch live on
        self.dev.calls.clear()
        _, r = self.run_edit()
        self.refused_before_unit(r, "edit_not_enabled")

    def test_slot_outside_the_allowed_range(self):
        self.set_modes("live", QC_AGENT_ALLOWED_SLOTS="1A-8H")
        _, r = self.run_edit()
        self.refused_before_unit(r, "slot_not_allowed")

    def test_protected_slot(self):
        self.set_modes("live", QC_AGENT_PROTECTED_SLOTS="19A-21H")
        _, r = self.run_edit()
        self.refused_before_unit(r, "slot_not_allowed")
        self.assertIn("protected", r["message"])


class CopyPreChecks(EditBase):
    """Copy mode (E5a): every refusal happens before the first write and before any save."""
    EDIT_MODES = "live,copy"
    TARGET = "21A"          # position 160
    NAME = "Texas Blues Edit"

    def copy(self, plan=None, **kw):
        kw.setdefault("target_slot", self.TARGET)
        kw.setdefault("name", self.NAME)
        return self.run_edit(plan, mode="copy", **kw)

    def assertNoWrites(self, r):
        self.assertEqual(self.dev.write_calls(), [])
        self.assertNotIn("begin_edit", [c[0] for c in self.dev.calls])
        self.assertEqual(self.agent.edits.started, {})

    def passes_the_checks(self, r):
        self.assertEqual((r["status"], r["reason_code"]), ("succeeded", ""), r["message"])
        self.assertEqual(r["edit"]["payload"]["cas"], {"state": "matched", "failed": []})

    def refused_on_unit(self, r, code, needle):
        self.assertEqual((r["status"], r["reason_code"]), ("refused", code), r["message"])
        self.assertIn(needle, r["message"])
        self.assertNoWrites(r)
        self.assertEqual(r["edit"]["payload"]["save"],
                         {"saved": False, "slot": None, "name": None, "listed": False, "backup": None})

    # -- before the unit is opened --

    def test_an_empty_target_passes_every_check(self):
        _, r = self.copy()
        self.passes_the_checks(r)
        self.assertEqual(r["edit"]["payload"]["mode"], "copy")

    def test_target_outside_the_allowed_slots(self):
        self.set_modes("live,copy", QC_AGENT_ALLOWED_SLOTS="1A-8H,20A")
        _, r = self.copy()
        self.refused_before_unit(r, "slot_not_allowed")
        self.assertIn("Target slot 21A", r["message"])

    def test_target_in_a_protected_range(self):
        self.set_modes("live,copy", QC_AGENT_PROTECTED_SLOTS="21A-21H")
        _, r = self.copy()
        self.refused_before_unit(r, "slot_not_allowed")
        self.assertIn("Target slot 21A is protected", r["message"])

    def test_the_open_slot_being_protected_also_stops_a_copy(self):
        self.set_modes("live,copy", QC_AGENT_PROTECTED_SLOTS="20A")
        _, r = self.copy()
        self.refused_before_unit(r, "slot_not_allowed")
        self.assertIn("Slot 20A is protected", r["message"])

    def test_names_the_unit_rule_refuses(self):
        for bad in ("a/b", ".hidden", "a..b", "Copy:1"):
            with self.subTest(bad):
                self.dev.calls.clear()
                _, r = self.copy(name=bad)
                self.refused_before_unit(r, "validation")
                self.assertIn("copy's name", r["message"])

    def test_the_same_slot_as_the_original_is_refused_even_past_the_parser(self):
        # contract.parse_job refuses this (no result is ever written), so call the check the way run() does
        args = edit_args(base_plan(), mode="copy", target_slot="20A", name=self.NAME)
        out = jobs.edit_precheck(self.agent.ctx, {"job_id": "x", "kind": "edit_preset", "args": args})
        self.assertEqual((out.status, out.reason_code), ("refused", "validation"))
        self.assertEqual(self.dev.calls, [])

    # -- on the unit, still before any write --

    def test_target_filled(self):
        self.dev.slots[160] = "Somebody's preset"
        _, r = self.copy()
        self.refused_on_unit(r, "slot_not_empty", "Slot 21A")
        self.assertIn("Somebody's preset", r["message"])

    def test_target_name_is_only_spaces_counts_as_filled(self):
        self.dev.slots[160] = "  "
        _, r = self.copy()
        self.refused_on_unit(r, "slot_not_empty", "Slot 21A")

    def test_the_name_is_taken_by_another_slot(self):
        self.dev.slots[200] = "texas blues edit "   # case and surrounding spaces ignored, as for a push
        _, r = self.copy()
        self.refused_on_unit(r, "validation", "already has a preset named")
        self.assertIn("Slot 26A", r["message"])

    def test_the_name_is_the_originals(self):
        _, r = self.copy(name="Texas Blues")
        self.refused_on_unit(r, "validation", "already has a preset named")
        self.assertIn("Slot 20A", r["message"])

    def test_the_other_collections_listing_decides_not_my_presets(self):
        plan = base_plan()
        plan["expect"]["folder_key"] = OTHER_KEY
        self.dev.open_folder = OTHER_KEY
        self.dev.collections = {OTHER_KEY: ("Sounds profiles", {152: "Texas Blues"})}
        self.dev.slots[160] = "Occupied in My Presets only"
        _, r = self.copy(plan)
        self.passes_the_checks(r)
        self.dev.calls.clear()
        rig_unit(self.dev)
        self.dev.open_folder = OTHER_KEY
        self.dev.collections[OTHER_KEY][1][160] = "Occupied here"
        self.agent.edits.started.clear()
        _, r = self.copy(plan)
        self.refused_on_unit(r, "slot_not_empty", "Sounds profiles")

    def test_the_cas_still_comes_first(self):
        self.dev.open_position = 153
        _, r = self.copy()
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "preset_changed"))
        self.assertNoWrites(r)

    def test_a_factory_preset_or_an_unlisted_collection_is_not_copied_from(self):
        self.dev.is_factory = True
        _, r = self.copy()
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "unsupported_arg"))
        self.assertNoWrites(r)

    def test_live_mode_has_no_target_to_check(self):
        self.dev.slots[160] = "Occupied"
        _, r = self.run_edit()
        self.assertEqual(r["status"], "succeeded")


class CopySave(EditBase):
    """Copy mode (E5a) end to end on the fake unit: apply, verify, collateral, re-check, ONE File CREATE into the
    empty target, listing, ledgers. Proves the agent's logic; whether the unit saves the edited grid at a position
    other than the open one is UNVERIFIED (gate E-G3)."""
    EDIT_MODES = "live,copy"
    TARGET, TPOS, NAME = "21A", 160, "Texas Blues Edit"

    def copy(self, plan=None, **kw):
        kw.setdefault("target_slot", self.TARGET)
        kw.setdefault("name", self.NAME)
        return self.run_edit(plan, mode="copy", **kw)

    def names(self):
        return [c[0] for c in self.dev.calls]

    def saves(self):
        return [c for c in self.dev.calls if c[0] == "save_edit_copy"]

    NOT_SAVED = {"saved": False, "slot": None, "name": None, "listed": False, "backup": None}

    def assertNotSaved(self, r, others=None):
        self.assertEqual(self.saves(), [])
        self.assertEqual(r["edit"]["payload"]["save"], self.NOT_SAVED)
        self.assertEqual(self.dev.slots, {152: "Texas Blues", **(others or {})})
        self.assertNotIn("save_to_slot", self.names())

    def test_all_pass_writes_the_plan_then_exactly_one_create(self):
        jid, r = self.copy()
        self.assertEqual((r["status"], r["reason_code"]), ("succeeded", ""), r["message"])
        writes = [c for c in self.dev.calls if c[0] in ("set_param", "set_param_scenes", "set_block_bypass", "save_edit_copy")]
        self.assertEqual(writes, [("set_param_scenes", 0, 0, 0, GAIN_WIRE), ("set_param", 0, 1, 1, 1.0),
                                  ("set_block_bypass", 0, 1, [False, True] + [False] * 6, 7),
                                  ("save_edit_copy", self.TPOS, self.NAME)])
        n = self.names()
        self.assertLess(max(i for i, x in enumerate(n) if x == "set_block_bypass"), n.index("edit_verified"))
        self.assertLess(n.index("edit_verified"), n.index("save_edit_copy"))
        self.assertEqual(n.count("edit_verified"), 1)
        self.assertNotIn("enter_rollback", n)
        self.assertNotIn("save_to_slot", n)
        e = r["edit"]["payload"]
        self.assertEqual(e["mode"], "copy")
        self.assertEqual(e["save"], {"saved": True, "slot": self.TARGET, "name": self.NAME, "listed": True, "backup": None})
        self.assertEqual(e["rollback"], {"state": "not_needed", "restored": 0, "failed": 0})
        self.assertEqual(e["collateral"]["mismatches"], 0)
        for p in e["params"]:
            self.assertTrue(p["ok"], f"param not ok: {p}")
        for b in e["bypass"]:
            self.assertTrue(b["ok"], f"bypass not ok: {b}")
        self.assertIn("unverified", r["message"].lower())   # E-G3 is labelled in the result
        self.assertIn("itself was not saved over", r["message"])
        # the copy is the bridge's own; the original is untouched
        self.assertEqual(self.dev.slots, {152: "Texas Blues", self.TPOS: self.NAME})
        self.assertEqual(self.dev.preset_name, "Texas Blues")
        self.assertTrue(self.agent.owned.owned(self.TPOS, self.NAME))
        self.assertFalse(self.agent.owned.owned(152, "Texas Blues"))
        led = self.agent.edits
        self.assertEqual((led.started[jid]["mode"], led.started[jid]["target_slot"]), ("copy", self.TARGET))
        got = led.edited[jid]
        self.assertEqual((got["status"], got["saved"], got["listed"], got["target_slot"]), ("succeeded", True, True, self.TARGET))
        self.assertIn("jobs/owned_slots.json", self.store.objects)   # the informational bucket copy was refreshed

    def test_the_original_slot_is_never_written(self):
        self.copy()
        for c in self.dev.calls:
            if c[0] == "save_edit_copy":
                self.assertNotEqual(c[1], 152)
        self.assertEqual(self.dev.slots[152], "Texas Blues")

    def test_the_owners_scene_is_put_back_before_the_save_opens(self):
        self.dev.active_scene = 3
        _, r = self.copy()
        self.assertEqual(r["status"], "succeeded")
        n = self.names()
        self.assertLess(n.index("restore_scene"), n.index("edit_verified"))
        self.assertEqual(self.dev.active_scene, 3)

    def test_the_next_open_slot_is_reported_as_evidence(self):
        self.dev.open_after_save = "target"
        _, r = self.copy()
        self.assertEqual(r["status"], "succeeded")
        self.assertIn("21A", r["message"])
        self.assertIn("open now", r["message"])

    def test_live_mode_never_arms_or_opens_a_save(self):
        _, r = self.run_edit()
        self.assertEqual(r["status"], "succeeded")
        self.assertNotIn("edit_verified", self.names())
        self.assertNotIn("save_edit_copy", self.names())
        self.assertIsNone(self.dev.edit_save)

    # -- verify fails: rolled back, never saved --

    def test_a_write_that_does_not_read_back_is_rolled_back_and_not_saved(self):
        orig = self.dev.set_param

        def lost(row, col, index, wire):
            orig(row, col, index, wire)
            self.dev.grid[col]["params"][index] = [0.0] * 8
        self.dev.set_param = lost
        jid, r = self.copy()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "readback_mismatch"))
        self.assertEqual(r["edit"]["payload"]["rollback"]["state"], "restored")
        self.assertNotSaved(r)
        self.assertNotIn("edit_verified", self.names())
        self.assertEqual(self.dev.grid[0]["params"][0], list(GAIN_EXPECT))
        self.assertEqual(self.agent.edits.edited[jid]["saved"], False)
        self.assertFalse(self.agent.owned.owned(self.TPOS, self.NAME))

    def test_a_collateral_change_is_rolled_back_and_not_saved(self):
        orig = self.dev.set_block_bypass

        def bump(row, col, scenes, idx):
            orig(row, col, scenes, idx)
            self.dev.extra_rows[1][0]["params"][2] = [0.99] * 8
        self.dev.set_block_bypass = bump
        _, r = self.copy()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "readback_mismatch"))
        self.assertNotSaved(r)
        self.assertNotIn("edit_verified", self.names())

    def test_a_failed_rollback_is_rollback_failed_and_not_saved(self):
        def lost(row, col, index, wire):
            self.dev.grid[col]["params"][index] = [0.0] * 8
        self.dev.set_param = lost
        self.dev.set_param_scenes = lambda row, col, index, wires: None
        self.dev.set_block_bypass = lambda *a: None
        _, r = self.copy()
        self.assertEqual(r["status"], "failed")
        self.assertNotSaved(r)

    def test_a_write_error_midway_is_not_saved(self):
        left = [1]

        def boom(name, args):
            if name == "set_param" and left:
                left.pop()
                raise DeviceError("boom")
        self.dev.write_fail = boom
        _, r = self.copy()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertNotSaved(r)

    # -- changes between the checks and the save: edit kept, not rolled back, not saved --

    def unsaved(self, r, code, others=None):
        self.assertEqual((r["status"], r["reason_code"]), ("failed", code), r["message"])
        self.assertNotSaved(r, others)
        self.assertIn("NOT saved", r["message"])
        self.assertIn("NOT rolled back", r["message"])
        self.assertEqual(r["edit"]["payload"]["rollback"], {"state": "not_needed", "restored": 0, "failed": 0})
        self.assertNotIn("enter_rollback", self.names())
        self.assertFalse(self.agent.owned.owned(self.TPOS, self.NAME))

    def test_the_target_is_filled_between_the_checks_and_the_save(self):
        def fill(fake, n):
            if n == 2:   # the listing just before the save
                fake.slots[self.TPOS] = "Filled meanwhile"
        self.dev.on_listing = fill
        jid, r = self.copy()
        self.unsaved(r, "slot_not_empty", {self.TPOS: "Filled meanwhile"})
        self.assertEqual(self.agent.edits.edited[jid]["status"], "failed")
        self.assertFalse(self.agent.edits.edited[jid]["saved"])

    def test_the_name_is_taken_between_the_checks_and_the_save(self):
        def take(fake, n):
            if n == 2:
                fake.slots[200] = self.NAME
        self.dev.on_listing = take
        _, r = self.copy()
        self.unsaved(r, "validation", {200: self.NAME})

    def test_another_preset_is_selected_before_the_save(self):
        def switch(name):
            if name == "edit_verified":
                self.dev.open_position = 153
        self.dev.on_call = switch
        _, r = self.copy()
        self.unsaved(r, "device_error")

    def test_the_grid_is_edited_on_the_unit_after_the_verify(self):
        n = []

        def drift(name):
            if name == "edit_verified":
                self.dev.grid[0]["params"][2] = [0.9] * 8   # the owner touches a control of an op cell
        self.dev.on_call = drift
        _, r = self.copy()
        self.unsaved(r, "readback_mismatch")
        self.assertEqual(r["edit"]["payload"]["collateral"]["mismatches"], 1)

    def test_the_pre_save_reread_times_out(self):
        reads = []

        def none_on_last(name):
            pass
        orig = self.dev.read_current_preset

        def rd():
            reads.append(1)
            return None if self.dev.calls and self.names().count("edit_verified") else orig()
        self.dev.read_current_preset = rd
        _, r = self.copy()
        self.unsaved(r, "device_error")

    def test_the_pre_save_listing_fails(self):
        from qc_agent.device import PartialListing

        def fail(fake, n):
            if n == 2:
                raise PartialListing(1)
        self.dev.on_listing = fail
        _, r = self.copy()
        self.unsaved(r, "device_error")

    def test_the_unit_goes_away_before_the_save(self):
        def gone(fake, n):
            if n == 2:
                fake.gone_after = fake._alive_calls
        self.dev.on_listing = gone
        _, r = self.copy()
        self.unsaved(r, "unit_absent")

    def test_the_guard_refusing_to_open_the_save_saves_nothing(self):
        def refuse():
            raise DeviceError("guard says no")
        self.dev.edit_verified = refuse
        _, r = self.copy()
        self.unsaved(r, "device_error")

    def test_expiry_is_not_rechecked_before_the_save(self):
        # contract step 11: expiry is re-checked immediately before the first device write only
        orig = self.dev.set_block_bypass

        def slow(*a):
            orig(*a)
            self.clock.advance(7200)
        self.dev.set_block_bypass = slow
        _, r = self.copy()
        self.assertEqual(r["status"], "succeeded")

    # -- the save was issued --

    def test_a_save_that_is_never_listed_is_failed_and_the_slot_stays_owned(self):
        self.dev.listing_lag = 99
        jid, r = self.copy()
        self.assertNotIn("not saved over", r["message"])   # a lagging listing proves nothing about the original
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertEqual(r["edit"]["payload"]["save"],
                         {"saved": True, "slot": self.TARGET, "name": self.NAME, "listed": False, "backup": None})
        self.assertIn("may still land", r["message"])
        self.assertTrue(self.agent.owned.owned(self.TPOS, self.NAME))
        self.assertEqual(len(self.saves()), 1)
        self.assertEqual(self.agent.edits.edited[jid]["saved"], True)
        self.assertEqual(self.agent.edits.edited[jid]["listed"], False)
        self.assertIn("jobs/owned_slots.json", self.store.objects)

    def test_a_laggy_listing_still_shows_the_copy(self):
        self.dev.listing_lag = 2
        _, r = self.copy()
        self.assertEqual(r["status"], "succeeded")
        self.assertTrue(r["edit"]["payload"]["save"]["listed"])

    def test_partial_listings_after_the_save_do_not_count(self):
        self.dev.short_after_save = 2
        _, r = self.copy()
        self.assertEqual(r["status"], "succeeded")

    def test_only_partial_listings_after_the_save_never_count_as_listed(self):
        self.dev.short_after_save = 99
        _, r = self.copy()
        self.assertEqual((r["status"], r["edit"]["payload"]["save"]["saved"], r["edit"]["payload"]["save"]["listed"]),
                         ("failed", True, False))

    def test_a_device_error_in_the_polls_keeps_looking(self):
        def flaky(fake, n):
            if n == 3:
                raise DeviceError("My Presets is missing")
        self.dev.on_listing = flaky
        _, r = self.copy()
        self.assertEqual(r["status"], "succeeded")

    def test_the_unit_going_away_after_the_save_is_unconfirmed(self):
        self.dev.listing_lag = 99   # never shows the copy, so the polls go on until the unit vanishes

        def gone(fake, n):
            if n == 4:
                fake.gone_after = fake._alive_calls
        self.dev.on_listing = gone
        _, r = self.copy()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "unit_absent"))
        self.assertEqual(r["edit"]["payload"]["save"]["saved"], True)
        self.assertFalse(r["edit"]["payload"]["save"]["listed"])
        self.assertTrue(self.agent.owned.owned(self.TPOS, self.NAME))

    def test_a_save_call_that_raises_counts_as_possibly_issued(self):
        def boom(name, args):
            if name == "save_edit_copy":
                raise DeviceError("transport died after sending")
        self.dev.write_fail = boom
        _, r = self.copy()
        # the fake did write the slot before raising; the point: the job reports a save as issued and keeps it owned
        self.assertEqual(r["edit"]["payload"]["save"]["saved"], True)
        self.assertTrue(self.agent.owned.owned(self.TPOS, self.NAME))
        self.assertEqual(len(self.saves()), 1)

    def test_a_save_that_lands_on_the_original_slot_is_flagged(self):
        self.dev.save_lands_at_open = True
        _, r = self.copy()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("CHANGED", r["message"])
        self.assertIn("BOTH 20A and 21A", r["message"])
        self.assertNotIn("not saved over", r["message"])
        self.assertTrue(r["edit"]["payload"]["save"]["saved"])
        self.assertFalse(r["edit"]["payload"]["save"]["listed"])   # the copy never listed at the target

    def test_the_copy_in_another_collection_lists_there(self):
        plan = base_plan()
        plan["expect"]["folder_key"] = OTHER_KEY
        self.dev.open_folder = OTHER_KEY
        self.dev.collections = {OTHER_KEY: ("Sounds profiles", {152: "Texas Blues"})}
        _, r = self.copy(plan)
        self.assertEqual(r["status"], "succeeded", r["message"])
        self.assertEqual(self.dev.collections[OTHER_KEY][1][self.TPOS], self.NAME)
        self.assertNotIn(self.TPOS, self.dev.slots)

    def test_a_crash_mid_copy_names_both_slots(self):
        jid, key = self.queue("edit_preset", args=edit_args(base_plan(), mode="copy", target_slot=self.TARGET, name=self.NAME))
        self.agent.ledger.take(jid, "edit_preset", contract.format_time(T0))
        self.agent.edits.start(jid, "20A", SETLIST_KEY, "Texas Blues", "h", contract.format_time(T0),
                               mode="copy", target_slot=self.TARGET)
        self.agent.recover()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "interrupted"))
        self.assertIn("20A", r["message"])
        self.assertIn("21A", r["message"])
        self.assertIn("WITHOUT saving", r["message"])


class CatalogRefusals(EditBase):
    """Plans the contract parser accepts but the agent must refuse: each before any device contact."""

    def refuse(self, mutate, needle):
        plan = base_plan()
        mutate(plan)
        jid, r = self.run_edit(plan)
        self.refused_before_unit(r, "validation")
        self.assertIn(needle, r["message"])

    def test_catalog_hash_differs(self):
        self.refuse(lambda p: p["catalog"].update(sha256="0" * 64), "different device catalog")

    def test_catalog_generation_differs_from_the_firmware(self):
        self.refuse(lambda p: p["catalog"].update(coros="4.2"), "CorOS 4.2")

    def test_model_not_in_the_catalog(self):
        def m(p):
            p["expect"]["layout"][0][2] = 999999
            p["param_ops"][0]["model_id"] = 999999
        self.refuse(m, "not in this PC's device catalog")

    def test_index_at_or_past_param_count(self):
        def m(p):
            p["param_ops"][0].update(index=4)   # Exotic has param_count 4: index 4 is the unit's trailing slot
        self.refuse(m, "at or past param_count")

    def test_index_that_is_not_listed(self):
        self.refuse(lambda p: p["param_ops"][0].update(index=200), "at or past param_count")

    def test_label_must_be_the_catalogs(self):
        self.refuse(lambda p: p["param_ops"][0].update(label="BASS"), "calls it 'GAIN'")

    def test_hidden_or_internal_index(self):
        cm = find_model(lambda m: m["category"] == "Delay" and "clones" not in m and any(
            p["index"] not in edit_checks.control_labels(
                {**m, "params_by_index": {q["index"]: q for q in m["params"]}}) for p in m["params"]
            if p["index"] < m["param_count"]))
        labels = edit_checks.control_labels({**cm, "params_by_index": {q["index"]: q for q in cm["params"]}})
        hidden = next(p for p in cm["params"] if p["index"] not in labels and p["index"] < cm["param_count"])

        def m(p):
            p["expect"]["layout"][0][2] = cm["id"]
            p["param_ops"] = [{**p["param_ops"][0], "model_id": cm["id"], "index": hidden["index"], "label": "X",
                               "scene_mode": False, "wire": [0.9] * 8}]
            p["bypass_ops"] = []
        self.refuse(m, "hidden or internal")

    def test_types_an_edit_never_writes(self):
        for kind in ("comboBox", "floatWithLed"):  # a string/meter is hidden or internal: refused above
            def user_control(m, k=kind):
                lab = edit_checks.control_labels({**m, "params_by_index": {q["index"]: q for q in m["params"]}})
                return next((p for p in m["params"] if p["type"] == k and p["index"] in lab and p["index"] < m["param_count"]), None)
            cm = find_model(lambda m: m["category"] in edit_checks.ALLOWED_CATEGORIES and "clones" not in m
                            and "plugin_id" not in m and user_control(m) is not None)
            labels = edit_checks.control_labels({**cm, "params_by_index": {q["index"]: q for q in cm["params"]}})
            prm = user_control(cm)

            def m(p, cm=cm, prm=prm, labels=labels):
                p["expect"]["layout"][0][2] = cm["id"]
                p["param_ops"] = [{**p["param_ops"][0], "model_id": cm["id"], "index": prm["index"],
                                   "label": labels[prm["index"]], "scene_mode": False, "wire": [0.9] * 8}]
                p["bypass_ops"] = []
            with self.subTest(kind):
                self.refuse(m, "not writable")
                self.dev.calls.clear()

    def test_switch_wire_must_be_a_whole_position(self):
        self.refuse(lambda p: p["param_ops"][1].update(wire=[0.5] * 8), "not a whole position")

    def test_locked_cells(self):
        clone = find_model(lambda m: m.get("clones") and m["clones"] not in edit_checks.VERIFIED_CLONE_BASES
                           and any(q["type"] == "float" for q in m["params"]))
        for what, mid in (("unverified clone", clone["id"]), ("capture", 14000), ("IR loader", 29001),
                          ("routing", 10000)):
            cm = catalog().models[mid]
            prm = next(q for q in cm["params"] if q["type"] == "float" and q["index"] < cm["param_count"])

            def m(p, mid=mid, prm=prm, cm=cm):
                p["expect"]["layout"][0][2] = mid
                p["param_ops"] = [{**p["param_ops"][0], "model_id": mid, "index": prm["index"], "label": prm["name"],
                                   "scene_mode": False, "wire": [0.9] * 8}]
                p["bypass_ops"] = []
            with self.subTest(what):
                self.refuse(m, "locked")
                self.dev.calls.clear()

    def test_bypass_on_routing_is_refused_but_on_a_clone_it_is_not(self):
        def m(p):
            p["expect"]["layout"][1][2] = 10000
            p["param_ops"] = p["param_ops"][:1]
            p["bypass_ops"][0]["model_id"] = 10000
        self.refuse(m, "locked")

    def test_a_plan_the_parser_refuses_never_reaches_the_checks(self):
        plan = base_plan()
        plan["param_ops"][0]["row"] = 1   # outside D7: a bad_payload, so the job is never even taken
        jid, key, data = job("edit_preset", args=edit_args(plan))
        self.store.objects[key] = data
        self.poll()
        self.assertNotIn(f"jobs/done/{jid}.json", self.store.objects)
        self.assertEqual(self.dev.calls, [])


class Cas(EditBase):
    """One fresh read; the plan's pins against it."""

    def run_and_pins(self):
        jid, r = self.run_edit()
        return r

    def assertPins(self, r, *pins):
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "preset_changed"))
        e = r["edit"]["payload"]
        self.assertEqual(e["cas"], {"state": "mismatch", "failed": list(pins)})
        self.assertEqual((e["params"], e["bypass"], e["collateral"]), ([], [], None))
        self.assertEqual(self.dev.write_calls(), [])

    def test_everything_matches(self):
        r = self.run_and_pins()
        e = r["edit"]["payload"]
        self.assertEqual(e["cas"], {"state": "matched", "failed": []})
        self.assertEqual((e["plan_sha256"], e["snapshot"], e["mode"]),
                         (edit_args(base_plan())["plan"]["sha256"], SNAP, "live"))
        self.assertEqual(r["firmware"], "4.1.0")

    def test_other_collection(self):
        self.dev.open_folder = OTHER_KEY
        self.dev.collections = {OTHER_KEY: ("Sounds profiles", {152: "Texas Blues"})}
        self.assertPins(self.run_and_pins(), "folder_key")

    def test_other_position(self):
        self.dev.open_position = 153
        self.assertPins(self.run_and_pins(), "position")

    def test_other_name(self):
        self.dev.preset_name = "Texas Blues 2"
        self.assertPins(self.run_and_pins(), "name")

    def test_name_is_compared_after_cleaning(self):
        # The plan's name is what the read reported, i.e. text.clean of the unit's string; the agent compares the
        # same cleaned form, so a raw name with a doubled space still matches and a different one does not.
        self.dev.preset_name = "Texas  Blues"
        self.assertEqual(self.run_and_pins()["edit"]["payload"]["cas"]["state"], "matched")
        rig_unit(self.dev)   # that edit was applied: put the unit back as the plan expects it
        self.dev.calls.clear()
        self.dev.preset_name = "Texas Blues\u202e!"
        self.assertPins(self.run_and_pins(), "name")

    def test_a_block_changed_model(self):
        self.dev.grid[0]["model_id"] = 7
        self.assertPins(self.run_and_pins(), "layout")

    def test_a_block_added_on_another_row(self):
        self.dev.extra_rows[2] = {3: {"model_id": 6, "params": {}, "bypass": [False] * 8, "n_params": 5}}
        self.assertPins(self.run_and_pins(), "layout")

    def test_a_block_removed(self):
        del self.dev.extra_rows[1][0]
        self.assertPins(self.run_and_pins(), "layout")

    def test_a_planned_value_differs_in_one_scene(self):
        vals = list(GAIN_EXPECT)
        vals[7] = 0.451
        self.dev.grid[0]["params"][0] = vals
        self.assertPins(self.run_and_pins(), "param_expect")

    def test_a_plain_control_is_scene_assigned_on_the_unit(self):
        # the plan says BOOST is not scene-assigned (scene_mode false: one plain write); on the unit it is
        self.dev.grid[1]["scene_mode"] = {1}
        self.assertPins(self.run_and_pins(), "param_expect")

    def test_a_bypass_flag_differs(self):
        self.dev.grid[1]["bypass"] = [False, False, False, False, False, False, False, True]
        self.assertPins(self.run_and_pins(), "bypass_expect")

    def test_several_pins_at_once_in_the_contract_order(self):
        self.dev.open_position = 153
        self.dev.preset_name = "x"
        self.dev.grid[1]["bypass"][0] = True
        self.dev.grid[0]["params"][0] = [0.1] * 8
        self.assertPins(self.run_and_pins(), "position", "name", "param_expect", "bypass_expect")

    def test_a_factory_preset_is_never_edited(self):
        self.dev.is_factory = True
        r = self.run_and_pins()
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "unsupported_arg"))
        self.assertEqual(self.dev.write_calls(), [])

    def test_a_collection_that_is_not_a_listed_256_slot_table(self):
        self.dev.open_folder = OTHER_KEY   # not in the fake's listing
        r = self.run_and_pins()
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "unsupported_arg"))
        self.assertIsNotNone(r["edit"])
        self.assertEqual(r["edit"]["payload"]["cas"]["state"], "not_reached")

    def test_the_unit_reports_another_parameter_layout_than_the_catalog(self):
        self.dev.param_count_skew = {0: 1}
        r = self.run_and_pins()
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "validation"))
        self.assertIn("parameters", r["message"])
        self.assertEqual(r["edit"]["payload"]["cas"]["state"], "matched")

    def test_the_selected_preset_changes_during_the_read(self):
        def switch(name):
            if name == "read_current_preset":
                self.dev.open_position = 160
        self.dev.on_call = switch
        r = self.run_and_pins()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertEqual(self.dev.write_calls(), [])

    def test_an_expired_job_is_not_run(self):
        jid, key, data = job("edit_preset", args=edit_args(base_plan()), ttl_s=60)
        self.store.objects[key] = data
        self.clock.advance(61)
        self.poll()
        self.assertEqual(self.result(jid)["status"], "expired")
        self.assertEqual(self.dev.calls, [])


class Apply(EditBase):
    """A live edit end to end on the fake unit: what it writes, what it reads back, what it undoes."""

    def edit(self, plan=None):
        return self.run_edit(plan)

    def writes(self):
        return [c for c in self.dev.calls if c[0] in ("set_param", "set_param_scenes", "set_block_bypass")]

    def test_all_pass_writes_exactly_the_plan_and_saves_nothing(self):
        jid, r = self.edit()
        self.assertEqual((r["status"], r["reason_code"]), ("succeeded", ""))
        self.assertEqual(self.writes(), [
            ("set_param_scenes", 0, 0, 0, GAIN_WIRE),
            ("set_param", 0, 1, 1, 1.0),
            ("set_block_bypass", 0, 1, [False, True] + [False] * 6, 7)])
        names = [c[0] for c in self.dev.calls]
        self.assertNotIn("save_to_slot", names)
        self.assertNotIn("place_capture", names)
        self.assertNotIn("add_block", names)
        self.assertEqual(self.dev.slots, {152: "Texas Blues"})
        e = r["edit"]["payload"]
        self.assertEqual(e["cas"], {"state": "matched", "failed": []})
        self.assertTrue(all(p["ok"] for p in e["params"]) and all(b["ok"] for b in e["bypass"]))
        self.assertEqual(e["collateral"]["mismatches"], 0)
        self.assertEqual(e["rollback"], {"state": "not_needed", "restored": 0, "failed": 0})
        self.assertEqual(e["save"], {"saved": False, "slot": None, "name": None, "listed": False, "backup": None})
        self.assertIn("NOT saved", r["message"])
        # the arming is per plan and ends with the job
        self.assertEqual([c[0] for c in self.dev.calls if c[0] in ("begin_edit", "enter_rollback")], ["begin_edit"])
        self.assertEqual(self.dev.calls[-2:], [("end_push",), ("disconnect",)])
        led = self.agent.edits
        self.assertEqual(led.started[jid]["slot"], "20A")
        self.assertEqual(led.edited[jid]["status"], "succeeded")
        self.assertNotEqual(led.edited[jid]["layout_before"], None)
        self.assertEqual(led.edited[jid]["layout_before"], led.edited[jid]["layout_after"])

    def test_the_scene_the_owner_had_is_put_back(self):
        self.dev.active_scene = 3
        _, r = self.edit()
        self.assertEqual(r["status"], "succeeded")
        self.assertEqual(self.dev.active_scene, 3)
        self.assertIn(("restore_scene", 3), self.dev.calls)

    def test_scene_a_is_left_alone(self):
        _, r = self.edit()
        self.assertNotIn("restore_scene", [c[0] for c in self.dev.calls])

    def test_a_scene_that_cannot_be_put_back_is_reported(self):
        self.dev.active_scene = 3
        self.dev.restore_scene = lambda n: None   # the unit ignores it
        _, r = self.edit()
        self.assertEqual(r["status"], "succeeded")
        self.assertIn("Check the active scene", r["message"])

    # -- failures --

    def test_a_write_that_does_not_read_back_is_rolled_back(self):
        # the unit drops the BOOST write: verify fails, the already written GAIN and the bypass are put back
        orig = self.dev.set_param

        def lost(row, col, index, wire):
            orig(row, col, index, wire)
            self.dev.grid[col]["params"][index] = [0.0] * 8
        self.dev.set_param = lost
        _, r = self.edit()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "readback_mismatch"))
        e = r["edit"]["payload"]
        self.assertEqual(e["rollback"], {"state": "restored", "restored": 3, "failed": 0})
        self.assertFalse(all(p["ok"] for p in e["params"]))
        self.assertEqual(self.dev.grid[0]["params"][0], list(GAIN_EXPECT))
        self.assertEqual(self.dev.grid[1]["bypass"], [False] * 8)
        self.assertEqual(self.dev.grid[1]["params"][1], [0.0] * 8)
        self.assertIn("enter_rollback", [c[0] for c in self.dev.calls])
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_a_collateral_change_on_another_row_is_rolled_back(self):
        def drift(name, args=None):
            pass
        orig = self.dev.set_block_bypass

        def bump(row, col, scenes, idx):
            orig(row, col, scenes, idx)
            self.dev.extra_rows[1][0]["params"][2] = [0.99] * 8   # another row changes during the edit
        self.dev.set_block_bypass = bump
        _, r = self.edit()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "readback_mismatch"))
        e = r["edit"]["payload"]
        self.assertEqual(e["collateral"]["items"], [{"row": 1, "col": 0, "index": 2, "scene": 0}])
        self.assertEqual(e["rollback"]["state"], "restored")
        self.assertTrue(all(p["ok"] for p in e["params"]))   # the planned values read back; the collateral is what failed

    def test_a_collateral_change_on_an_op_cell(self):
        orig = self.dev.set_param

        def bump(row, col, index, wire):
            orig(row, col, index, wire)
            self.dev.grid[1]["params"][2] = [0.9] * 8
        self.dev.set_param = bump
        _, r = self.edit()
        self.assertEqual(r["reason_code"], "readback_mismatch")
        self.assertEqual(r["edit"]["payload"]["collateral"]["items"][0]["index"], 2)

    def test_a_write_error_midway_rolls_back_what_was_attempted(self):
        def boom(name, args):
            if name == "set_param" and ("enter_rollback",) not in self.dev.calls:
                raise DeviceError("fake: write failed")
        self.dev.write_fail = boom
        _, r = self.edit()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        e = r["edit"]["payload"]
        # GAIN was written, BOOST was started and failed (it may be half written), the bypass never started
        self.assertEqual(e["rollback"], {"state": "restored", "restored": 2, "failed": 0})
        self.assertEqual(self.dev.grid[0]["params"][0], list(GAIN_EXPECT))
        self.assertEqual(self.dev.grid[1]["params"][1], [0.0] * 8)
        self.assertEqual(self.dev.grid[1]["bypass"], [False] * 8)
        self.assertNotIn(("set_block_bypass", 0, 1, [False, True] + [False] * 6, 7), self.dev.calls)

    def test_a_failure_before_any_write_has_nothing_to_roll_back(self):
        self.dev.grid[1]["model_id"] = 1011
        calls = {"n": 0}
        orig = self.dev.read_current_preset

        def swap_after_cas():
            calls["n"] += 1
            r = orig()
            if calls["n"] >= 2:   # the re-check before column 0's first write
                r["chains"][0]["blocks"][0]["model_id"] = 7
            return r
        self.dev.read_current_preset = swap_after_cas
        _, r = self.edit()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertEqual(r["edit"]["payload"]["rollback"], {"state": "not_needed", "restored": 0, "failed": 0})
        self.assertEqual(self.writes(), [])

    def test_a_rollback_that_does_not_stick_is_rollback_failed(self):
        orig_scenes = self.dev.set_param_scenes
        state = {"rollback": False}
        self.dev.enter_rollback = lambda: state.update(rollback=True)

        def stuck(row, col, index, wires):
            if state["rollback"]:
                return   # the unit ignores the write that would put GAIN back
            orig_scenes(row, col, index, wires)
        self.dev.set_param_scenes = stuck
        orig = self.dev.set_param

        def lost(row, col, index, wire):
            orig(row, col, index, wire)
            self.dev.grid[col]["params"][index] = [0.0] * 8
        self.dev.set_param = lost
        _, r = self.edit()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "rollback_failed"))
        e = r["edit"]["payload"]
        self.assertEqual((e["rollback"]["state"], e["rollback"]["restored"], e["rollback"]["failed"]), ("partial", 2, 1))
        self.assertIn("WITHOUT saving", r["message"])
        self.assertTrue(self.agent.ledger.finished)

    def test_every_rollback_write_fails(self):
        def boom(name, args):
            if self.dev.calls and ("enter_rollback",) in self.dev.calls:
                raise DeviceError("fake: no")
        self.dev.write_fail = boom
        orig = self.dev.set_param

        def lost(row, col, index, wire):
            orig(row, col, index, wire)
            self.dev.grid[col]["params"][index] = [0.0] * 8
        self.dev.set_param = lost
        _, r = self.edit()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "rollback_failed"))
        self.assertEqual(r["edit"]["payload"]["rollback"]["state"], "failed")

    def test_the_owner_selects_another_preset_midway_and_nothing_more_is_written(self):
        n = {"writes": 0}

        def switch(name):
            if name in ("set_param", "set_param_scenes", "set_block_bypass"):
                n["writes"] += 1
                if n["writes"] == 1:
                    self.dev.open_position = 160   # after the first write the owner picks another preset
        self.dev.on_call = switch
        _, r = self.edit()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "rollback_failed"))
        self.assertEqual(len(self.writes()), 1)    # no further write, and NO rollback write, into the other preset
        self.assertNotIn("enter_rollback", [c[0] for c in self.dev.calls])
        e = r["edit"]["payload"]
        self.assertEqual(e["rollback"], {"state": "failed", "restored": 0, "failed": 1})
        self.assertIn("may have changed too", r["message"])

    def test_the_owner_selects_another_preset_before_the_first_write(self):
        orig = self.dev.setlist_position
        n = {"k": 0}

        def pos():
            p = orig()
            n["k"] += 1
            if n["k"] >= 3:   # past the two reads around the compare-and-swap
                p = {**p, "position": 160}
            return p
        self.dev.setlist_position = pos
        _, r = self.edit()
        self.assertEqual(r["status"], "failed")
        self.assertEqual(self.writes(), [])
        self.assertEqual(r["edit"]["payload"]["rollback"]["state"], "not_needed")

    def test_ops_left_wrong_count_as_failed_so_the_result_fits_its_state(self):
        # BOOST's write is lost (verify fails and rolls back), then the owner turns an op's control that the edit never
        # reached: restored > 0, nothing failed to write, but a planned op is wrong afterwards -> partial with failed > 0
        orig = self.dev.set_param

        def lost(row, col, index, wire):
            orig(row, col, index, wire)
            self.dev.grid[col]["params"][index] = [0.0] * 8
        self.dev.set_param = lost
        orig_b = self.dev.set_block_bypass

        def knob(row, col, scenes, idx):
            if ("enter_rollback",) in self.dev.calls:
                orig_b(row, col, scenes, idx)
                self.dev.grid[1]["bypass"] = [True] * 8   # the bypass op's control is touched during the rollback
                return
            orig_b(row, col, scenes, idx)
        self.dev.set_block_bypass = knob
        _, r = self.edit()
        e = r["edit"]["payload"]
        rb = e["rollback"]
        self.assertEqual(r["reason_code"], "rollback_failed")
        self.assertEqual(rb["state"], "partial")
        self.assertTrue(rb["restored"] > 0 and rb["failed"] > 0, rb)

    def test_the_unit_vanishes_during_the_edit(self):
        self.dev.gone_after = 7
        _, r = self.edit()
        self.assertEqual(r["status"], "failed")
        self.assertIn(r["reason_code"], ("unit_absent", "rollback_failed"))
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_the_read_after_the_writes_times_out_so_nothing_is_written_blind(self):
        orig = self.dev.read_current_preset
        n = {"k": 0}

        def flaky():
            n["k"] += 1
            return None if n["k"] >= 3 else orig()   # the CAS read and the re-checks answer, then silence
        self.dev.read_current_preset = flaky
        _, r = self.edit()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "rollback_failed"))
        self.assertEqual(r["edit"]["payload"]["rollback"]["state"], "failed")
        self.assertNotIn("enter_rollback", [c[0] for c in self.dev.calls])
        self.assertIsNone(r["edit"]["payload"]["collateral"])

    def test_a_crash_mid_edit_is_reported_with_the_slot(self):
        jid, key = self.queue("edit_preset", args=edit_args(base_plan()))
        self.agent.ledger.take(jid, "edit_preset", contract.format_time(T0))
        self.agent.edits.start(jid, "20A", SETLIST_KEY, "Texas Blues", "h", contract.format_time(T0))
        self.agent.recover()
        r = self.result(jid)
        self.assertEqual((r["status"], r["reason_code"], r["kind"]), ("failed", "interrupted", "edit_preset"))
        self.assertIn("20A", r["message"])
        self.assertIn("WITHOUT saving", r["message"])

    def test_the_edit_ledger_survives_a_restart(self):
        jid, _ = self.edit()
        from qc_agent.ledger import EditLedger
        again = EditLedger(self.home / "edits.jsonl")
        self.assertIn(jid, again.started)
        self.assertIn(jid, again.edited)

    def test_an_edit_that_cannot_be_recorded_writes_nothing(self):
        def broken(*a, **k):
            raise OSError("disk full")
        self.agent.edits.start = broken
        _, r = self.edit()
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertEqual(self.writes(), [])


class CasFunctionTable(unittest.TestCase):
    """cas_pins and its helpers on plain dicts (no agent)."""

    def test_a_null_bypass_cell_reads_as_no_flags_not_a_crash(self):
        # GSR #185 round 5: a device payload with a null cell used to raise TypeError in live_bypass
        for bp in ([[None]], [[None, [False] * 8]], [[5]], [["x"]], None, []):
            with self.subTest(bp):
                self.assertIsNone(edit_checks.live_bypass({"bypass": bp}, 0, 0))

    def test_floats_must_be_exact_real_floats(self):
        self.assertTrue(edit_checks.floats_equal([0.5] * 8, [0.5] * 8))
        self.assertTrue(edit_checks.floats_equal([0.0] * 8, [0] * 8))
        for bad in ([0.5] * 7, [0.5] * 7 + [True], [0.5] * 7 + [float("nan")], [0.5] * 7 + [0.5000001], "x", None):
            with self.subTest(bad):
                self.assertFalse(edit_checks.floats_equal(bad, [0.5] * 8))


if __name__ == "__main__":
    unittest.main()
