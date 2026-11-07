"""qc_agent.edit_diff: the read-back, collateral and rollback functions, table-tested on plain dicts."""
import copy
import unittest

from fake_device import FakeDevice
from qc_agent import edit_diff
from edit_fixtures import GAIN_EXPECT, GAIN_WIRE, base_plan, rig_unit


def read(mutate=None):
    dev = FakeDevice()
    rig_unit(dev)
    dev.connected = True
    if mutate:
        mutate(dev)
    return dev.read_current_preset()


def written(preset, plan):
    """The preset as it reads after the plan's writes."""
    out = copy.deepcopy(preset)
    for op in plan["param_ops"]:
        blk = next(b for b in out["chains"][0]["blocks"] if b["col"] == op["col"])
        blk["params"][op["index"]]["values"] = list(op["wire"])
    for op in plan["bypass_ops"]:
        out["bypass"][0][op["col"]] = list(op["bypass"])
    return out


class Collateral(unittest.TestCase):
    def setUp(self):
        self.plan = base_plan()
        self.pre = read()
        self.post = written(self.pre, self.plan)

    def check(self, post=None):
        return edit_diff.collateral(self.pre, post or self.post, self.plan)

    def test_clean_edit(self):
        c = self.check()
        # row 0: Exotic 5 slots + TopBoost 7 + row 1: Exotic 5 = 17 parameters, minus the 2 written
        self.assertEqual(c, {"params_compared": 15, "bypass_compared": 256 - 8, "layout_same": True,
                             "mismatches": 0, "items": []})

    def test_a_written_control_is_not_collateral_whatever_it_reads(self):
        self.post["chains"][0]["blocks"][0]["params"][0]["values"] = [0.9] * 8
        self.assertEqual(self.check()["mismatches"], 0)

    def test_the_bypass_cell_of_an_op_is_exempt_but_other_cells_are_not(self):
        self.post["bypass"][0][1] = [True] * 8          # the op's own cell: not compared
        self.assertEqual(self.check()["mismatches"], 0)
        self.post["bypass"][0][0][3] = True             # another cell
        c = self.check()
        self.assertEqual((c["mismatches"], c["items"]), (1, [{"row": 0, "col": 0, "index": None, "scene": 3}]))

    def test_an_unplanned_parameter_on_an_op_cell(self):
        self.post["chains"][0]["blocks"][0]["params"][1]["values"][5] = 0.31
        c = self.check()
        self.assertEqual(c["items"], [{"row": 0, "col": 0, "index": 1, "scene": 5}])

    def test_a_parameter_on_another_row(self):
        self.post["chains"][1]["blocks"][0]["params"][2]["values"][0] = 0.0
        c = self.check()
        self.assertEqual((c["mismatches"], c["items"]), (1, [{"row": 1, "col": 0, "index": 2, "scene": 0}]))

    def test_one_parameter_is_one_mismatch_with_the_first_differing_scene(self):
        self.post["chains"][1]["blocks"][0]["params"][2]["values"] = [0.9] * 8
        c = self.check()
        self.assertEqual((c["mismatches"], c["items"][0]["scene"]), (1, 0))

    def test_a_bypass_flag_on_an_empty_cell(self):
        self.post["bypass"][3][7][7] = True
        c = self.check()
        self.assertEqual(c["items"], [{"row": 3, "col": 7, "index": None, "scene": 7}])

    def test_layout_changes(self):
        for name, mutate in (
                ("model changed", lambda p: p["chains"][1]["blocks"][0].update(model_id=7)),
                ("block removed", lambda p: p["chains"][1]["blocks"].clear()),
                ("block added", lambda p: p["chains"][2]["blocks"].append({"col": 3, "model_id": 6, "params": []}))):
            with self.subTest(name):
                post = copy.deepcopy(self.post)
                mutate(post)
                c = self.check(post)
                self.assertFalse(c["layout_same"])
                self.assertEqual(c["mismatches"], 1)
                self.assertIsNone(c["items"][0]["index"])
                self.assertIsNone(c["items"][0]["scene"])

    def test_a_cell_without_a_model_id_that_disappears_is_a_layout_change_not_a_crash(self):
        # GSR #185: a pre-write block with no model_id and a post-write cell that is gone used to raise AttributeError
        # (None == None skipped the layout branch), aborting the job before any rollback.
        pre = {"chains": [{"row": 0, "blocks": [{"col": 0, "params": []}]}], "bypass": None}
        post = {"chains": [{"row": 0, "blocks": []}], "bypass": None}
        c = edit_diff.collateral(pre, post, base_plan())
        self.assertFalse(c["layout_same"])
        self.assertEqual(c["items"][0], {"row": 0, "col": 0, "index": None, "scene": None})

    def test_a_missing_param_slot(self):
        del self.post["chains"][1]["blocks"][0]["params"][4]
        c = self.check()
        self.assertEqual(c["items"], [{"row": 1, "col": 0, "index": 4, "scene": 0}])

    def test_no_bypass_in_the_second_read_differs_everywhere_outside_the_op_cell(self):
        self.post["bypass"] = None
        c = self.check()
        self.assertEqual(c["bypass_compared"], 248)
        self.assertEqual(c["mismatches"], 248)          # every flag read False before, None now
        self.assertEqual(len(c["items"]), edit_diff.MAX_ITEMS)

    def test_no_bypass_in_either_read_is_equal(self):
        self.pre["bypass"] = self.post["bypass"] = None
        self.assertEqual(self.check()["mismatches"], 0)

    def test_items_are_capped_but_the_count_is_complete(self):
        for blk in self.post["chains"][1]["blocks"]:
            blk["params"][:] = [{"index": i, "values": [0.5] * 8, "scene_mode": False} for i in range(5)]
        self.post["bypass"][2] = [[True] * 8 for _ in range(8)]
        c = self.check()
        self.assertEqual(len(c["items"]), edit_diff.MAX_ITEMS)
        self.assertEqual(c["mismatches"], 5 + 64)
        self.assertLessEqual(len(c["items"]), c["mismatches"])

    def test_a_nan_that_was_already_there_is_not_collateral(self):
        # GSR #185 round 7: a NaN in an untouched control, before and after, is no change
        self.pre["chains"][1]["blocks"][0]["params"][0]["values"][3] = float("nan")
        self.post["chains"][1]["blocks"][0]["params"][0]["values"][3] = float("nan")
        self.assertEqual(self.check()["mismatches"], 0)
        self.post["chains"][1]["blocks"][0]["params"][0]["values"][3] = 0.5   # but NaN -> a number is a change
        self.assertEqual(self.check()["mismatches"], 1)

    def test_a_non_finite_or_other_type_value_is_a_difference(self):
        self.post["chains"][1]["blocks"][0]["params"][0]["values"][0] = float("nan")
        self.assertEqual(self.check()["mismatches"], 1)
        self.post["chains"][1]["blocks"][0]["params"][0]["values"][0] = 0  # an int where the unit read a float
        self.assertEqual(self.check()["mismatches"], 1)


class VerifyOps(unittest.TestCase):
    def setUp(self):
        self.plan = base_plan()
        self.post = written(read(), self.plan)

    def test_as_written(self):
        params, bypass, ok = edit_diff.verify_ops(self.plan, self.post)
        self.assertTrue(ok)
        self.assertEqual(params[0], {"row": 0, "col": 0, "index": 0, "readback": GAIN_WIRE, "ok": True})
        self.assertEqual(bypass, [{"row": 0, "col": 1, "readback": [False, True] + [False] * 6, "ok": True}])

    def test_float32_rounding_is_within_tolerance(self):
        self.post["chains"][0]["blocks"][0]["params"][0]["values"] = [0.5, 0.6200000047683716] + [0.4499999880790710] * 6
        self.assertTrue(edit_diff.verify_ops(self.plan, self.post)[2])

    def test_one_scene_differs(self):
        self.post["chains"][0]["blocks"][0]["params"][0]["values"][1] = 0.7
        params, _, ok = edit_diff.verify_ops(self.plan, self.post)
        self.assertFalse(ok)
        self.assertFalse(params[0]["ok"])

    def test_values_that_are_not_finite_numbers_read_back_as_null(self):
        self.post["chains"][0]["blocks"][0]["params"][0]["values"] = [float("nan"), "x", None, True, 0.5, 0.5, 0.5]
        params, _, ok = edit_diff.verify_ops(self.plan, self.post)
        self.assertFalse(ok)
        self.assertEqual(params[0]["readback"], [None, None, None, None, 0.5, 0.5, 0.5, None])

    def test_a_missing_control_and_missing_bypass(self):
        del self.post["chains"][0]["blocks"][0]["params"][0]
        self.post["bypass"] = None
        params, bypass, ok = edit_diff.verify_ops(self.plan, self.post)
        self.assertFalse(ok)
        self.assertEqual(params[0]["readback"], [None] * 8)
        self.assertEqual(bypass[0], {"row": 0, "col": 1, "readback": None, "ok": False})


class Rollback(unittest.TestCase):
    def setUp(self):
        self.plan = base_plan()

    def test_everything_attempted_is_undone_newest_first(self):
        steps = edit_diff.rollback_plan(self.plan, [("param", 0, 0), ("param", 1, 1), ("bypass", 1)])
        self.assertEqual([s["kind"] for s in steps], ["bypass", "param", "param"])
        self.assertEqual(steps[0], {"kind": "bypass", "col": 1, "flags": [False] * 8})
        self.assertEqual(steps[1], {"kind": "param", "col": 1, "index": 1, "wires": [0.0] * 8, "scene_mode": False})
        self.assertEqual(steps[2], {"kind": "param", "col": 0, "index": 0, "wires": list(GAIN_EXPECT), "scene_mode": True})

    def test_only_what_was_attempted(self):
        self.assertEqual(edit_diff.rollback_plan(self.plan, []), [])
        steps = edit_diff.rollback_plan(self.plan, [("param", 0, 0)])
        self.assertEqual([(s["kind"], s["col"]) for s in steps], [("param", 0)])

    def test_never_anything_outside_the_plan_and_no_duplicates(self):
        steps = edit_diff.rollback_plan(self.plan, [("param", 0, 0), ("param", 0, 9), ("bypass", 5), ("param", 0, 0),
                                                    ("other", 1)])
        self.assertEqual(len(steps), 1)


if __name__ == "__main__":
    unittest.main()
