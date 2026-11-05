"""The push validator against fixtures (the P2 export doesn't exist yet).

The fixture follows plan section 4's `qc2-device-export/1` sketch and uses the
models and wire values of the Phase 0 G3/G4/G6 test preset. Every refusal path has
its own case."""
import copy
import unittest

from helpers import catalog, export
from qc_agent.export_validator import ExportRefused, validate


def mutate(fn):
    e = export()
    fn(e["payload"])
    return e


def block(p, i):
    return p["rows"][0]["blocks"][i]


class Valid(unittest.TestCase):
    def test_fixture_plans(self):
        plan = validate(export(), catalog(), "4.1.0")
        self.assertEqual((plan.row, plan.in_port, plan.out_port), (0, 1, 19))
        self.assertEqual([b.model_id for b in plan.blocks], [1, 1001, 12001, 4003, 8006])
        gain = plan.blocks[0].params[0]
        self.assertEqual(gain.scene_wires, [0.2, 0.8, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2])
        self.assertIsNone(plan.blocks[0].params[1].scene_wires)
        self.assertEqual(plan.blocks[4].bypass_scenes, [False, True] + [False] * 6)
        self.assertEqual(plan.scene_labels, [(0, "Rhythm"), (1, "Lead")])

    def test_scene_b_equal_to_a_is_not_scene_assigned(self):
        e = mutate(lambda p: block(p, 0)["params"][1].update(scene_b={"display": 3, "wire": 0.3}))
        self.assertIsNone(validate(e, catalog(), "4.1.0").blocks[0].params[1].scene_wires)

    def test_integers_are_numbers(self):
        e = mutate(lambda p: block(p, 4)["params"][1].update(wire=1))
        self.assertEqual(validate(e, catalog(), "4.1.0").blocks[4].params[1].wire, 1.0)


class Refusals(unittest.TestCase):
    def refused(self, e, needle, firmware="4.1.0"):
        with self.assertRaises(ExportRefused) as cm:
            validate(e, catalog(), firmware)
        self.assertIn(needle, str(cm.exception))

    def test_unknown_format(self):
        e = export()
        e["format"] = "qc2-device-export/2"
        self.refused(e, "unsupported export format")

    def test_payload_format_differs(self):
        self.refused(mutate(lambda p: p.update(format="qc2-device-export/9")), "payload.format")

    def test_extra_or_missing_keys(self):
        self.refused(mutate(lambda p: p.update(extra=1)), "payload has keys")
        self.refused(mutate(lambda p: p.pop("scenes")), "payload has keys")
        self.refused(mutate(lambda p: block(p, 0).update(stomp=1)), "block 0 has keys")
        self.refused(mutate(lambda p: block(p, 0)["params"][0].update(units="x")), "param 0 has keys")

    def test_other_catalog(self):
        self.refused(mutate(lambda p: p["catalog"].update(sha256="0" * 64)), "different device catalog")
        self.refused(mutate(lambda p: p["catalog"].update(sha256="ABC")), "64 lower-case hex")

    def test_other_firmware_generation(self):
        self.refused(export(), "expects 4.2.0", firmware="4.2.0")
        self.refused(mutate(lambda p: p["catalog"].update(coros="4.0")), "targets CorOS 4.0")
        self.refused(mutate(lambda p: p["catalog"].update(coros="4.1.0")), "must look like 4.1")

    def test_unresolved_items(self):
        self.refused(mutate(lambda p: p["unresolved"].append({"block": "x", "reason": "y"})), "unresolved")

    def test_rows(self):
        self.refused(mutate(lambda p: p["rows"].append(copy.deepcopy(p["rows"][0]))), "exactly one row")
        self.refused(mutate(lambda p: p["rows"][0].update(row=1)), "row 0")
        self.refused(mutate(lambda p: p["rows"][0].update(out_port=20)), "output 19")
        self.refused(mutate(lambda p: p["rows"][0].update(in_port=True)), "integer")

    def test_block_count_and_columns(self):
        def nine(p):
            blocks = p["rows"][0]["blocks"]
            while len(blocks) < 9:
                b = copy.deepcopy(blocks[0])
                b["col"] = len(blocks)
                blocks.append(b)
        self.refused(mutate(nine), "1-8 blocks")
        self.refused(mutate(lambda p: p["rows"][0].update(blocks=[])), "1-8 blocks")
        self.refused(mutate(lambda p: block(p, 1).update(col=5)), "columns must run")

    def test_models(self):
        self.refused(mutate(lambda p: block(p, 0).update(model_id=20000)), "NC_Recorder")
        self.refused(mutate(lambda p: block(p, 0).update(model_id=99999)), "not in this PC's device catalog")
        self.refused(mutate(lambda p: block(p, 0).update(model="Klon")), "in the catalog, not")
        self.refused(mutate(lambda p: block(p, 0).update(model_id=30, model="Plini Drive", params=[])), "plugin")
        for mid in (13004, 14000, 29001, 10004, 11000, 27000):  # FX loop, capture, IR, splitter, mixer, looper
            name = catalog().models[mid]["name"]
            with self.subTest(mid):
                self.refused(mutate(lambda p: block(p, 0).update(model_id=mid, model=name, params=[])),
                             "is not pushed in v1")

    def test_params(self):
        def cab(param):
            return mutate(lambda p: block(p, 2).update(params=[param]))
        self.refused(cab({"index": 99, "name": "X", "display": 0, "wire": 0}), "not a listed parameter")
        self.refused(cab({"index": 1, "name": "ir selector", "display": 0, "wire": 0}), "not writable")
        self.refused(cab({"index": 2, "name": "LEVEL", "display": 0, "wire": 0.5}), "symbolic")
        self.refused(cab({"index": 4, "name": "distance", "display": 0, "wire": 0}), "in the catalog, not")
        self.refused(cab({"index": 4, "name": "DISTANCE", "display": 11, "wire": 0.5}), "outside 0..10")
        self.refused(cab({"index": 4, "name": "DISTANCE", "display": 1, "wire": -0.01}), "outside 0..1")
        self.refused(cab({"index": 4, "name": "DISTANCE", "display": 1, "wire": float("nan")}), "finite")
        self.refused(cab({"index": 4, "name": "DISTANCE", "display": 1, "wire": True}), "finite number")
        self.refused(cab({"index": 4, "name": "DISTANCE", "display": "1", "wire": 0.1}), "finite number")
        self.refused(cab({"index": 0, "name": "bypass", "display": 0.5, "wire": 0.5}), "whole number")
        dup = {"index": 4, "name": "DISTANCE", "display": 1, "wire": 0.1}
        self.refused(mutate(lambda p: block(p, 2).update(params=[dup, dup])), "appears twice")
        self.refused(cab({"index": 4, "name": "DISTANCE", "display": 1, "wire": 0.1,
                          "scene_b": {"display": 1, "wire": 2}}), "scene B")

    def test_meter_param_is_not_writable(self):
        m = next(m for m in catalog().models.values()
                 if any(p["type"] == "grMeter" for p in m["params"]) and m.get("plugin_id") is None)
        p = next(p for p in m["params"] if p["type"] == "grMeter")
        e = mutate(lambda pl: block(pl, 0).update(model_id=m["id"], model=m["name"], params=[
            {"index": p["index"], "name": p["name"], "display": p["min"], "wire": 0}]))
        self.refused(e, "not writable")

    def test_bypass(self):
        self.refused(mutate(lambda p: block(p, 0).update(bypass={"A": 0, "B": False})), "true/false")
        self.refused(mutate(lambda p: block(p, 0).update(bypass={"A": False})), "bypass has keys")

    def test_scenes(self):
        self.refused(mutate(lambda p: p["scenes"].append({"index": 0, "label": "dup"})), "unique 0..7")
        self.refused(mutate(lambda p: p["scenes"].append({"index": 3, "label": "x\n"})), "printable")


if __name__ == "__main__":
    unittest.main()
