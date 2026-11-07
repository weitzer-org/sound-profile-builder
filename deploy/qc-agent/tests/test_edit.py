"""edit_preset (preset-editing E4): the agent's pre-write checks, the compare-and-swap and, in later
sections, the writes. In-memory bucket, fake unit, fixed clock. Proves the agent's logic and guards;
says nothing about the real unit (E-G1 on a QC-2-owned test slot is the first real check)."""
import copy
import json
import unittest
from unittest import mock

from fake_device import FakeDevice
from helpers import T0, catalog, config, job
from qc_agent import contract, jobs
from qc_agent import edit_checks
from qc_agent.device import SETLIST_KEY
from test_agent import Base

SNAP = "11111111-2222-4333-8444-555555555555"
OTHER_KEY = "/media/p4/Presets/Sounds profiles"
GAIN_EXPECT = [0.45] * 8
GAIN_WIRE = [0.5, 0.62] + [0.45] * 6


def setUpModule():
    for name in ("SAVE_LIST_INTERVAL_S", "PACE_S"):
        p = mock.patch.object(jobs, name, 0)
        p.start()
        unittest.addModuleCleanup(p.stop)


def find_model(pred):
    for m in catalog().models.values():
        if pred(m):
            return m
    raise AssertionError("no catalog model matches")


def base_plan():
    """A plan over a grid of two row-0 cells (an Exotic overdrive and a UK C30 TopBoost amp) and one block on
    row 1: one scene-varying GAIN change, one switch (BOOST) change and one bypass change."""
    return {"format": contract.EDIT_FORMAT, "catalog": {"coros": "4.1", "sha256": catalog().sha256}, "snapshot": SNAP,
            "expect": {"folder_key": SETLIST_KEY, "position": 152, "slot": "20A", "name": "Texas Blues",
                       "layout": [[0, 0, 6], [0, 1, 1011], [1, 0, 6]]},
            "param_ops": [
                {"row": 0, "col": 0, "model_id": 6, "index": 0, "label": "GAIN", "evidence": ["taper:g4"],
                 "scene_mode": True, "expect": list(GAIN_EXPECT), "wire": list(GAIN_WIRE)},
                {"row": 0, "col": 1, "model_id": 1011, "index": 1, "label": "BOOST", "evidence": ["encoding:qc-mcp-linear"],
                 "scene_mode": False, "expect": [0.0] * 8, "wire": [1.0] * 8}],
            "bypass_ops": [
                {"row": 0, "col": 1, "model_id": 1011, "why": "target", "expect": [False] * 8,
                 "bypass": [False, True] + [False] * 6}]}


def edit_args(plan, mode="live", slot=None, **kw):
    text = json.dumps(plan, separators=(",", ":"))
    return {"slot": slot or plan["expect"]["slot"], "mode": mode,
            "target_slot": kw.get("target_slot"), "name": kw.get("name"),
            "source": {"preset_id": "device-1", "guitar": "From the unit"},
            "plan": {"format": contract.EDIT_FORMAT, "sha256": contract.plan_sha256(contract.EDIT_FORMAT, text),
                     "payload": text}}


def rig_unit(dev, plan=None):
    """Make the fake unit hold exactly the preset base_plan() was built from."""
    dev.open_folder, dev.open_position, dev.preset_name = SETLIST_KEY, 152, "Texas Blues"
    dev.slots = {152: "Texas Blues"}
    dev.grid = {0: {"model_id": 6, "params": {0: list(GAIN_EXPECT), 1: [0.3] * 8, 2: [0.4] * 8, 3: [0.5] * 8},
                    "bypass": [False] * 8},
                1: {"model_id": 1011, "params": {0: [0.5] * 8, 1: [0.0] * 8, 2: [0.2] * 8, 3: [0.7] * 8, 4: [0.1] * 8,
                                                  5: [0.6] * 8},
                    "bypass": [False] * 8}}
    dev.extra_rows = {1: {0: {"model_id": 6, "params": {i: [0.25] * 8 for i in range(5)}, "bypass": [False] * 8,
                              "n_params": 5}}}


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
        return jid, self.result(jid)

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

    def test_copy_and_overwrite_are_not_honoured_even_when_listed(self):
        self.set_modes("live,copy,overwrite")
        _, r = self.run_edit(mode="copy", target_slot="21A", name="Copy")
        self.refused_before_unit(r, "unsupported_arg")
        self.dev.calls.clear()
        _, r = self.run_edit(mode="overwrite")
        self.refused_before_unit(r, "unsupported_arg")

    def test_slot_outside_the_allowed_range(self):
        self.set_modes("live", QC_AGENT_ALLOWED_SLOTS="1A-8H")
        _, r = self.run_edit()
        self.refused_before_unit(r, "slot_not_allowed")

    def test_protected_slot(self):
        self.set_modes("live", QC_AGENT_PROTECTED_SLOTS="19A-21H")
        _, r = self.run_edit()
        self.refused_before_unit(r, "slot_not_allowed")
        self.assertIn("protected", r["message"])


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
        self.assertEqual(self.dev.write_calls(), [])
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


class CasFunctionTable(unittest.TestCase):
    """cas_pins and its helpers on plain dicts (no agent)."""

    def test_floats_must_be_exact_real_floats(self):
        self.assertTrue(edit_checks.floats_equal([0.5] * 8, [0.5] * 8))
        self.assertTrue(edit_checks.floats_equal([0.0] * 8, [0] * 8))
        for bad in ([0.5] * 7, [0.5] * 7 + [True], [0.5] * 7 + [float("nan")], [0.5] * 7 + [0.5000001], "x", None):
            with self.subTest(bad):
                self.assertFalse(edit_checks.floats_equal(bad, [0.5] * 8))


if __name__ == "__main__":
    unittest.main()
