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
        def f(p):  # Spring MIX 0..100 %, linear: display 100 is wire 1, both written as JSON integers
            block(p, 4)["params"][0].update(scene_mode=False, display=eight(100), wire=eight(1))
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
            return {"index": index, "label": label, "scene_mode": mode, "display": d, "wire": w,
                    "evidence": ["taper:g4"], **kw}

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
        # evidence: required, known labels only, and consistent with the catalog parameter
        self.refused(cab({k: v for k, v in ok.items() if k != "evidence"}), "has keys")
        self.refused(cab(ok | {"evidence": ["taper:guess"]}), "unknown labels")
        self.refused(cab(ok | {"evidence": []}), "short list")
        self.refused(cab(ok | {"evidence": ["taper:g4", "taper:g4"]}), "short list")
        self.refused(cab(ok | {"evidence": ["encoding:qc-mcp-linear"]}), "doesn't match a float")
        self.refused(cab(ok | {"evidence": ["taper:g4", "bounds:g4"]}), "doesn't match the catalog's bounds")
        # the wire must be the catalog's mapping of the display value, not just inside 0..1
        self.refused(cab(prm(4, "MIC 1 DISTANCE", eight(1), eight(0.2))), "not the catalog's mapping")

    def test_taper_is_rechecked(self):
        # Brit 2203 OUTPUT: -60..+12 dB, skew 3.8018. G4 read wire 0.25 as -10.0 dB on the unit.
        out = lambda d, w: {"index": 6, "label": "OUTPUT", "scene_mode": False, "display": eight(d),  # noqa: E731
                            "wire": eight(w), "units": "dB", "evidence": ["taper:g4"]}
        w = ((-10 + 60) / 72) ** 3.8018
        validate(mutate(lambda p: block(p, 1).update(params=[out(-10, w)])), catalog(), "4.1.0")
        self.refused(mutate(lambda p: block(p, 1).update(params=[out(-10, (50 / 72))])), "not the catalog's mapping")

    def test_symbolic_bounds(self):
        # Parametric-8's gains are MIN/MAX_EQ_DB: pyquadcortex's 4.0.1 numbers, not confirmed on 4.1.0,
        # allowed with their label (owner decision, plan 4.1). -12..+12 dB, linear: 0 dB is wire 0.5.
        gain = next(x for x in catalog().models[4000]["params"] if x["name"] == "1 GAIN")
        self.assertEqual(gain["min_symbol"], "MIN_EQ_DB")
        g = {"index": gain["index"], "label": "1 GAIN", "scene_mode": False, "display": eight(0), "wire": eight(0.5),
             "evidence": ["taper:g4", "bounds:pyquadcortex-4.0.1"]}
        validate(mutate(lambda p: swap(p, 0, 4000, params=[g])), catalog(), "4.1.0")
        self.refused(mutate(lambda p: swap(p, 0, 4000, params=[g | {"display": eight(13), "wire": eight(1)}])),
                     "outside -12..12")
        # A symbol nobody measured (the input trim) has no value at all: refused.
        cat = copy.deepcopy(catalog())
        cat.models[4000]["params_by_index"][gain["index"]]["min_symbol"] = "MIN_INPUT_TRIM"
        with self.assertRaisesRegex(ExportRefused, "unconfirmed or missing bound"):
            validate(mutate(lambda p: swap(p, 0, 4000, params=[g])), cat, "4.1.0")

    def test_discrete_position_is_whole(self):
        trails = {"index": 4, "label": "TRAILS", "scene_mode": False, "display": eight(0.5), "wire": eight(0.5),
                  "evidence": ["encoding:qc-mcp-linear"]}
        self.refused(mutate(lambda p: block(p, 4).update(params=[trails])), "whole number")
        validate(mutate(lambda p: block(p, 4).update(params=[trails | {"display": eight(1), "wire": eight(1)}])),
                 catalog(), "4.1.0")
        # a switch must say it uses the position encoding, and use it
        self.refused(mutate(lambda p: block(p, 4).update(params=[trails | {"display": eight(1), "wire": eight(1),
                                                                            "evidence": ["taper:g4"]}])),
                     "doesn't match a switch")
        self.refused(mutate(lambda p: block(p, 4).update(params=[trails | {"display": eight(1), "wire": eight(0.9)}])),
                     "not the catalog's mapping")

    def test_scene_arrays(self):
        gain = lambda p: block(p, 0)["params"][0]  # noqa: E731
        self.refused(mutate(lambda p: gain(p).update(display=[2, 8])), "exactly 8 values")
        self.refused(mutate(lambda p: gain(p).update(wire=0.2)), "exactly 8 values")
        def scene(i, d, w):  # a consistent display/wire pair, but in Scene C-H
            def f(p):
                gain(p)["display"][i], gain(p)["wire"][i] = d, w
            return f
        self.refused(mutate(scene(3, 9, 0.9)), "scenes C-H must carry Scene A")
        self.refused(mutate(scene(7, 9, 0.9)), "scenes C-H must carry Scene A")
        self.refused(mutate(lambda p: gain(p).update(scene_mode=False)), "scene_mode is False")
        self.refused(mutate(lambda p: block(p, 0)["params"][1].update(scene_mode=True)), "scene_mode is True")

        def wire_only(p):  # equal displays but different wires: a lie about the taper
            block(p, 0)["params"][1]["wire"][1] = 0.9
        self.refused(mutate(wire_only), "not the catalog's mapping")

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


class Additions(unittest.TestCase):
    """P2b (plan 4.1/4.2): warnings ride along, capture blocks are refused for now."""

    def refused(self, e, needle):
        with self.assertRaisesRegex(ExportRefused, re.escape(needle)):
            validate(e, catalog(), "4.1.0")

    def test_warnings_are_carried_not_executed(self):
        w = [{"block": "#3 110 US PRN C10R", "param": "Mic 2", "reason": "not applied: ..."}]
        plan = validate(mutate(lambda p: p.update(warnings=w)), catalog(), "4.1.0")
        self.assertEqual(len(plan.blocks), 5)
        self.refused(mutate(lambda p: p.update(warnings="x")), "warnings must be a list")
        self.refused(mutate(lambda p: p.update(warnings=[{"block": "x"}])), "has keys")
        self.refused(mutate(lambda p: p.update(warnings=[{"block": "x", "reason": 5}])), "short strings")
        self.refused(mutate(lambda p: p.pop("warnings")), "payload has keys")

    def test_capture_blocks(self):
        gain = {"index": 0, "label": "GAIN", "scene_mode": True, "display": [2, 3] + [2] * 6,
                "wire": [26 / 48, 27 / 48] + [26 / 48] * 6, "units": "dB", "evidence": ["taper:g4"]}

        def cap(**over):
            def f(p):
                swap(p, 0, 14000, capture={"name": "CA John's", "source": "factory", "resolved_on_unit": False},
                     category="Neural Capture", index_evidence="capture-block", params=[gain])
                block(p, 0)["model_id"] = 0
                block(p, 0)["model"] = "CA John's"
                for k, v in over.items():
                    if k == "capture":
                        block(p, 0)["capture"].update(v)
                    else:
                        block(p, 0)[k] = v
            return mutate(f)
        plan = validate(cap(), catalog(), "4.1.0")
        b = plan.blocks[0]
        self.assertEqual((b.model_id, b.model, b.capture.name, b.capture.source, b.capture.key, b.max_param_index),
                         (0, "CA John's", "CA John's", "factory", None, 4))
        self.assertEqual(b.params[0].scene_wires[:2], [26 / 48, 27 / 48])
        self.refused(cap(model_id=14000), "must have model_id 0")
        self.refused(cap(plugin="Nolly"), "can't name a plugin")
        self.refused(cap(category="Guitar Amplifier"), "category must be 'Neural Capture'")
        self.refused(cap(index_evidence="plain"), "index_evidence must be 'capture-block'")
        self.refused(cap(capture={"source": "cloud"}), "must be 'factory' or 'user'")
        self.refused(cap(capture={"source": ["user"]}), "must be 'factory' or 'user'")
        self.refused(cap(capture={"resolved_on_unit": True}), "must be false")
        self.refused(cap(capture={"name": " CA John's"}), "printable ASCII")
        self.refused(cap(capture={"name": "x" * 65}), "printable ASCII")
        self.refused(cap(capture={"name": "CA\nJohn's"}), "printable ASCII")
        self.refused(cap(model="CA Johns"), "model must equal capture.name")
        self.refused(cap(capture={"key": "ab"}), "has keys")
        # only the capture block's own controls: index 5 (file_name) and anything unlisted are refused
        self.refused(cap(params=[gain | {"index": 5, "label": "file_name"}]), "not a listed parameter")
        self.refused(cap(params=[gain | {"label": "VOLUME"}]), "in the catalog, not")
        # a bare capture block id without `capture` is not a placeable model
        self.refused(mutate(lambda p: swap(p, 0, 14000)), "not pushed in v1")

    def test_capture_layouts_must_match(self):
        cat = copy.deepcopy(catalog())
        cat.models[14001]["params"][0]["max"] = 30
        e = mutate(lambda p: (swap(p, 0, 14000, capture={"name": "CA John's", "source": "factory",
                                                        "resolved_on_unit": False},
                                   category="Neural Capture", index_evidence="capture-block"),
                              block(p, 0).update(model_id=0, model="CA John's")))
        with self.assertRaisesRegex(ExportRefused, "don't list the same controls"):
            validate(e, cat, "4.1.0")
