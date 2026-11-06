"""The push validator against the Go exporter's own output.

The base export is internal/deviceexport's golden for the one preset v1 can push
(pushable_g3, the shape of gate G3's test layout), loaded byte for byte by helpers.export().
Its blocks: col 0 Myth Drive (1), col 1 Brit 2203 (1001), col 2 110 US PRN C10R (M) (12045,
a clone of 12000), col 3 Low-High Cut (4003), col 4 Spring (M) (8006; bypassed in Scene B).
Every refusal path has its own case. tests/test_go_goldens.py covers the other goldens."""
import copy
import re
import unittest

from helpers import catalog, export
from qc_agent.export_validator import ExportRefused, validate

A8 = [0.0] * 8


def mutate(fn):
    e = export()
    fn(e["payload"])
    return e


def block(p, i):
    return p["rows"][0]["blocks"][i]


def eight(v):
    return [v] * 8


def swap(p, i, mid, **over):
    """Replace block i with an empty (all defaults) block of catalog model `mid`."""
    cm = catalog().models[mid]
    b = {"col": i, "model_id": mid, "model": cm["name"], "category": cm["category"], "index_evidence": "plain",
         "bypass": eight(False), "params": [], "at_default": []}
    b.update(over)
    p["rows"][0]["blocks"][i] = b


class Valid(unittest.TestCase):
    def test_go_export_plans(self):
        plan = validate(export(), catalog(), "4.1.0")
        self.assertEqual((plan.row, plan.in_port, plan.out_port), (0, 1, 19))
        self.assertEqual([b.model_id for b in plan.blocks], [1, 1001, 12045, 4003, 8006])
        gain = plan.blocks[0].params[0]
        self.assertEqual((gain.index, gain.name), (0, "GAIN"))
        self.assertEqual(gain.scene_wires, [0.2, 0.8, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2])
        self.assertIsNone(plan.blocks[0].params[1].scene_wires)
        self.assertEqual(plan.blocks[4].bypass_scenes, [False, True] + [False] * 6)
        self.assertEqual(plan.scene_labels, [])

    def test_dual_mic_label_and_symbolic_bound(self):
        # "MIC 2 LEVEL" is catalog index 10 "LEVEL" (the second LEVEL); its range is MIN/MAX_CABSIM_DB,
        # which the unit confirmed (CorOS 4.1.0). The planned name is the catalog's own.
        cab = validate(export(), catalog(), "4.1.0").blocks[2]
        self.assertEqual([(p.index, p.name) for p in cab.params][0], (10, "LEVEL"))

    def test_stomps_are_validated_and_carried(self):
        plan = validate(export(), catalog(), "4.1.0")
        self.assertEqual([(s.row, s.col, s.switch) for s in plan.stomps], [(0, 0, "C"), (0, 4, "G")])

    def test_empty_stomps(self):
        self.assertEqual(validate(mutate(lambda p: p.update(stomps=[])), catalog(), "4.1.0").stomps, [])

    def test_several_blocks_may_share_a_switch(self):
        e = mutate(lambda p: p["stomps"].append({"row": 0, "col": 1, "switch": "G"}))
        self.assertEqual(len(validate(e, catalog(), "4.1.0").stomps), 3)

    def test_scene_b_equal_to_a_is_not_scene_assigned(self):
        def f(p):
            prm = block(p, 0)["params"][0]
            prm.update(scene_mode=False, display=eight(2), wire=eight(0.2))
        self.assertIsNone(validate(mutate(f), catalog(), "4.1.0").blocks[0].params[0].scene_wires)

    def test_integers_are_numbers(self):
        def f(p):
            block(p, 4)["params"][0].update(scene_mode=False, display=eight(35), wire=eight(1))
        self.assertEqual(validate(mutate(f), catalog(), "4.1.0").blocks[4].params[0].wire, 1.0)


class Refusals(unittest.TestCase):
    def refused(self, e, needle, firmware="4.1.0"):
        with self.assertRaisesRegex(ExportRefused, re.escape(needle)):
            validate(e, catalog(), firmware)

    def test_unknown_format(self):
        e = export()
        e["format"] = "qc2-device-export/2"
        self.refused(e, "unsupported export format")

    def test_payload_format_differs(self):
        self.refused(mutate(lambda p: p.update(format="qc2-device-export/9")), "payload.format")

    def test_extra_or_missing_keys(self):
        self.refused(mutate(lambda p: p.update(extra=1)), "payload has keys")
        for k in ("scene_sources", "stomps", "guitar", "unresolved"):
            with self.subTest(k):
                self.refused(mutate(lambda p: p.pop(k)), "payload has keys")
        self.refused(mutate(lambda p: p.update(scenes=[])), "payload has keys")  # the pre-P2 draft shape
        self.refused(mutate(lambda p: block(p, 0).update(stomp=1)), "block 0 has keys")
        self.refused(mutate(lambda p: block(p, 0).pop("at_default")), "block 0 has keys")
        self.refused(mutate(lambda p: block(p, 0)["params"][0].update(extra="x")), "param 0 has keys")
        self.refused(mutate(lambda p: block(p, 0)["params"][0].pop("scene_mode")), "param 0 has keys")

    def test_other_catalog(self):
        self.refused(mutate(lambda p: p["catalog"].update(sha256="0" * 64)), "different device catalog")
        self.refused(mutate(lambda p: p["catalog"].update(sha256="ABC")), "64 lower-case hex")

    def test_other_firmware_generation(self):
        self.refused(export(), "expects 4.2.0", firmware="4.2.0")
        self.refused(mutate(lambda p: p["catalog"].update(coros="4.0")), "targets CorOS 4.0")
        self.refused(mutate(lambda p: p["catalog"].update(coros="4.1.0")), "must look like 4.1")

    def test_unresolved_items(self):
        self.refused(mutate(lambda p: p["unresolved"].append({"block": "x", "reason": "y"})), "unresolved")

    def test_guitar_and_scene_sources(self):
        self.refused(mutate(lambda p: p.update(guitar="")), "payload.guitar")
        self.refused(mutate(lambda p: p.update(guitar=7)), "payload.guitar")
        self.refused(mutate(lambda p: p.update(guitar="x" * 201)), "payload.guitar")
        self.refused(mutate(lambda p: p.update(scene_sources=["A"] * 8)), "scene_sources")
        self.refused(mutate(lambda p: p.update(scene_sources=["A", "B"])), "scene_sources")

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
        self.refused(mutate(lambda p: block(p, 0).update(category="Reverb")), "category is")
        self.refused(mutate(lambda p: swap(p, 0, 30)), "plugin")  # Plini Drive
        self.refused(mutate(lambda p: block(p, 0).update(plugin="Cory Wong")), "names a plugin")
        for mid in (13004, 14000, 29001, 10004, 11000, 27000):  # FX loop, capture, IR, splitter, mixer, looper
            name = catalog().models[mid]["name"]
            with self.subTest(mid):
                self.refused(mutate(lambda p: swap(p, 0, mid)), "is not pushed in v1")

    def test_index_evidence(self):
        self.refused(mutate(lambda p: block(p, 0).update(index_evidence="clone-12000-g1")), "index_evidence")
        self.refused(mutate(lambda p: block(p, 2).update(index_evidence="plain")), "index_evidence")
        self.refused(mutate(lambda p: block(p, 2).update(index_evidence="clone-32000-g1")), "index_evidence")
        self.refused(mutate(lambda p: block(p, 0).update(index_evidence=7)), "must be a string")
        # a clone of a base the unit never confirmed (a stereo cab on 32000) is refused outright
        self.refused(mutate(lambda p: swap(p, 0, 32001, index_evidence="clone-32000-g1")), "never checked")

    def test_params(self):
        def cab(param):
            return mutate(lambda p: block(p, 2).update(params=[param]))

        def prm(index, label, d, w, mode=False, **kw):
            return {"index": index, "label": label, "scene_mode": mode, "display": d, "wire": w, **kw}

        ok = prm(4, "MIC 1 DISTANCE", eight(1), eight(0.1))
        validate(cab(ok), catalog(), "4.1.0")  # the control case: this shape is accepted
        self.refused(cab(prm(99, "X", eight(0), eight(0))), "not a listed parameter")
        self.refused(cab(prm(1, "ir selector", eight(0), eight(0))), "not a user-facing control")
        self.refused(cab(prm(0, "bypass", eight(0), eight(0))), "not a user-facing control")
        self.refused(cab(prm(4, "DISTANCE", eight(1), eight(0.1))), "in the catalog, not")  # per-mic label
        self.refused(cab(prm(4, "MIC 2 DISTANCE", eight(1), eight(0.1))), "in the catalog, not")  # wrong mic
        self.refused(cab(prm(4, "mic 1 distance", eight(1), eight(0.1))), "in the catalog, not")
        self.refused(cab(prm(4, "MIC 1 DISTANCE", eight(11), eight(0.5))), "outside 0..10")
        self.refused(cab(prm(4, "MIC 1 DISTANCE", eight(1), eight(-0.01))), "outside 0..1")
        self.refused(cab(prm(4, "MIC 1 DISTANCE", eight(1), eight(float("nan")))), "finite")
        self.refused(cab(prm(4, "MIC 1 DISTANCE", eight(1), eight(True))), "finite number")
        self.refused(cab(prm(4, "MIC 1 DISTANCE", eight("1"), eight(0.1))), "finite number")
        self.refused(cab(ok | {"units": 5}), "units")
        self.refused(cab(ok | {"scene_mode": 1}), "scene_mode")
        dup = prm(4, "MIC 1 DISTANCE", eight(1), eight(0.1))
        self.refused(mutate(lambda p: block(p, 2).update(params=[dup, dup])), "appears twice")

    def test_symbolic_bounds_the_unit_never_confirmed(self):
        # Parametric-8's gains are MIN/MAX_EQ_DB: pyquadcortex's 4.0.1 numbers, not confirmed on 4.1.0.
        gain = next(x for x in catalog().models[4000]["params"] if x["name"] == "1 GAIN")
        self.assertEqual(gain["min_symbol"], "MIN_EQ_DB")
        e = mutate(lambda p: swap(p, 0, 4000, params=[
            {"index": gain["index"], "label": "1 GAIN", "scene_mode": False, "display": eight(0), "wire": eight(0.5)}]))
        self.refused(e, "unconfirmed or missing bound")

    def test_discrete_position_is_whole(self):
        trails = {"index": 4, "label": "TRAILS", "scene_mode": False, "display": eight(0.5), "wire": eight(0.5)}
        self.refused(mutate(lambda p: block(p, 4).update(params=[trails])), "whole number")
        validate(mutate(lambda p: block(p, 4).update(params=[trails | {"display": eight(1), "wire": eight(1)}])),
                 catalog(), "4.1.0")

    def test_scene_arrays(self):
        gain = lambda p: block(p, 0)["params"][0]  # noqa: E731
        self.refused(mutate(lambda p: gain(p).update(display=[2, 8])), "exactly 8 values")
        self.refused(mutate(lambda p: gain(p).update(wire=0.2)), "exactly 8 values")
        self.refused(mutate(lambda p: gain(p)["wire"].__setitem__(3, 0.9)), "scenes C-H must carry Scene A")
        self.refused(mutate(lambda p: gain(p)["display"].__setitem__(7, 9)), "scenes C-H must carry Scene A")
        self.refused(mutate(lambda p: gain(p).update(scene_mode=False)), "scene_mode is False")
        self.refused(mutate(lambda p: block(p, 0)["params"][1].update(scene_mode=True)), "scene_mode is True")

        def wire_only(p):  # equal displays but different wires: a lie about the taper
            block(p, 0)["params"][1]["wire"][1] = 0.9
        self.refused(mutate(wire_only), "different wire values")

    def test_bypass(self):
        self.refused(mutate(lambda p: block(p, 0).update(bypass={"A": False, "B": False})), "exactly 8 values")
        self.refused(mutate(lambda p: block(p, 0).update(bypass=[False] * 7)), "exactly 8 values")
        self.refused(mutate(lambda p: block(p, 0).update(bypass=[0] + [False] * 7)), "true/false")
        self.refused(mutate(lambda p: block(p, 0).update(bypass=[False, False, True] + [False] * 5)),
                     "scenes C-H must carry Scene A's flag")

    def test_stomps(self):
        def one(**kw):
            return mutate(lambda p: p.update(stomps=[{"row": 0, "col": 0, "switch": "C"} | kw]))
        self.refused(one(switch="H"), "H is never assigned")
        for sw in ("I", "", "c", "CC", 2, None):
            with self.subTest(sw):
                self.refused(one(switch=sw), "switch must be one of")
        self.refused(one(col=9), "no block is placed at column 9")
        self.refused(one(col=-1), "no block is placed")
        self.refused(one(row=1), "row must be 0")
        self.refused(one(row=True), "integer")
        self.refused(mutate(lambda p: p["stomps"][0].update(column=0)), "stomps[0] has keys")
        self.refused(mutate(lambda p: p["stomps"][0].update(momentary=True)), "stomps[0] has keys")
        self.refused(mutate(lambda p: p["stomps"].append({"row": 0, "col": 0, "switch": "D"})), "already has a switch")
        self.refused(mutate(lambda p: p.update(stomps=None)), "payload.stomps")
        self.refused(mutate(lambda p: p.update(stomps=[{"row": 0, "col": 0, "switch": "C"}] * 33)), "at most 32")


if __name__ == "__main__":
    unittest.main()
