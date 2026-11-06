"""The push validator against P2's real exports.

The inputs are internal/deviceexport's golden files, read as written, so a change on the P2 side
that the agent doesn't accept fails here. Every refusal rule has a case that mutates the pushable
golden and checks the message."""
import copy
import re
import unittest

from helpers import catalog, export, golden
from qc_agent.export_validator import ExportRefused, _check_block, validate


def mutate(fn):
    e = export()
    fn(e["payload"])
    return e


def block(p, i):
    return p["rows"][0]["blocks"][i]


def param(p, bi, pi):
    return block(p, bi)["params"][pi]


class Valid(unittest.TestCase):
    def test_pushable_golden_plans(self):
        plan = validate(export(), catalog(), "4.1.0")
        self.assertEqual((plan.row, plan.in_port, plan.out_port), (0, 1, 19))
        self.assertEqual([b.model_id for b in plan.blocks], [1, 1001, 12045, 4003, 8006])
        self.assertEqual([b.param_count for b in plan.blocks], [3, 7, 21, 5, 5])
        self.assertEqual([b.bypass_index for b in plan.blocks], [4, 8, 22, 6, 6])
        gain = plan.blocks[0].params[0]
        self.assertEqual((gain.index, gain.label, gain.varies), (0, "GAIN", True))
        self.assertEqual(gain.wires, [0.2, 0.8, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2])
        self.assertFalse(plan.blocks[0].params[1].varies)
        self.assertEqual([p.label for p in plan.blocks[2].params], ["MIC 2 LEVEL", "HPF", "LPF"])
        self.assertEqual(plan.blocks[2].at_default, [("MIC 1 LEVEL", 2), ("MIC 1 PAN", 3)])
        self.assertEqual(plan.blocks[4].bypass, [False, True] + [False] * 6)
        self.assertIsNone(plan.blocks[0].plugin)

    def test_eval_goldens_are_refused_for_their_unresolved_items(self):
        for name in ("01_SRV_Clean", "03_British_Invasion", "05_Clapton"):
            with self.subTest(name), self.assertRaisesRegex(ExportRefused, "unresolved item"):
                validate(export(name), catalog(), "4.1.0")

    def test_every_block_of_every_golden_passes_the_block_checks(self):
        # The eval goldens can't be pushed, but each block P2 did place must pass the agent's
        # per-block checks: the label port and the taper recompute agree with Go on more models.
        n = 0
        for name in ("pushable_g3", "01_SRV_Clean", "03_British_Invasion", "05_Clapton"):
            for b in golden(name)["payload"]["rows"][0]["blocks"]:
                with self.subTest(name=name, col=b["col"]):
                    _check_block(b["col"], b, catalog())
                    n += 1
        self.assertGreater(n, 10)

    def test_plugin_model_with_its_plugin_is_accepted(self):
        m = next(m for m in catalog().models.values()
                 if m.get("plugin_id") is not None and m["category"] == "Guitar Overdrive"
                 and not m.get("clones"))
        e = mutate(lambda p: block(p, 0).update(model_id=m["id"], model=m["name"], category=m["category"],
                                                plugin="SomePlugin", params=[], at_default=[]))
        self.assertEqual(validate(e, catalog(), "4.1.0").blocks[0].plugin, "SomePlugin")


class Refusals(unittest.TestCase):
    def refused(self, e, needle, firmware="4.1.0"):
        with self.assertRaisesRegex(ExportRefused, re.escape(needle)):
            validate(e, catalog(), firmware)

    # ---- envelope and header ----

    def test_unknown_format(self):
        e = export()
        e["format"] = "qc2-device-export/2"
        self.refused(e, "unsupported export format")

    def test_payload_format_differs(self):
        self.refused(mutate(lambda p: p.update(format="qc2-device-export/9")), "payload.format")

    def test_extra_or_missing_keys(self):
        self.refused(mutate(lambda p: p.update(extra=1)), "payload has keys")
        self.refused(mutate(lambda p: p.pop("scene_sources")), "payload has keys")
        self.refused(mutate(lambda p: p["catalog"].update(x=1)), "payload.catalog has keys")
        self.refused(mutate(lambda p: p["rows"][0].update(x=1)), "rows[0] has keys")
        self.refused(mutate(lambda p: block(p, 0).update(stomp=1)), "block 0 has keys")
        self.refused(mutate(lambda p: block(p, 0).pop("at_default")), "block 0 has keys")
        self.refused(mutate(lambda p: param(p, 0, 0).update(name="x")), "param 0 has keys")
        self.refused(mutate(lambda p: param(p, 0, 0).pop("scene_mode")), "param 0 has keys")

    def test_other_catalog(self):
        self.refused(mutate(lambda p: p["catalog"].update(sha256="0" * 64)), "different device catalog")
        self.refused(mutate(lambda p: p["catalog"].update(sha256="ABC")), "64 lower-case hex")
        self.refused(mutate(lambda p: p["catalog"].update(origin="somewhere else")), "catalog header")

    def test_other_firmware_generation(self):
        self.refused(export(), "expects 4.2.0", firmware="4.2.0")
        self.refused(mutate(lambda p: p["catalog"].update(coros="4.0")), "targets CorOS 4.0")
        self.refused(mutate(lambda p: p["catalog"].update(coros="4.1.0")), "must look like 4.1")

    def test_unresolved_items(self):
        self.refused(mutate(lambda p: p["unresolved"].append({"block": "x", "param": "", "reason": "y"})),
                     "1 unresolved item")
        self.refused(mutate(lambda p: p.update(unresolved=None)), "must be a list")

    def test_guitar(self):
        self.refused(mutate(lambda p: p.update(guitar="")), "payload.guitar")
        self.refused(mutate(lambda p: p.update(guitar="Tele\ncaster")), "payload.guitar")
        self.refused(mutate(lambda p: p.update(guitar=None)), "payload.guitar")

    def test_scene_sources(self):
        self.refused(mutate(lambda p: p.update(scene_sources=["A", "B", "B", "A", "A", "A", "A", "A"])),
                     "scene_sources must be A,B,A,A,A,A,A,A")
        self.refused(mutate(lambda p: p.update(scene_sources=["A", "B"])), "scene_sources")

    # ---- rows and blocks ----

    def test_rows(self):
        self.refused(mutate(lambda p: p["rows"].append(copy.deepcopy(p["rows"][0]))), "exactly one row")
        self.refused(mutate(lambda p: p["rows"][0].update(row=1)), "row 0")
        self.refused(mutate(lambda p: p["rows"][0].update(out_port=20)), "output 19")
        self.refused(mutate(lambda p: p["rows"][0].update(in_port=2)), "input 1")
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
        self.refused(mutate(lambda p: block(p, 1).update(col=5)), "columns must run 0..n-1")
        self.refused(mutate(lambda p: p["rows"][0]["blocks"].pop(1)), "columns must run 0..n-1")

    def test_internal_and_unknown_models(self):
        for mid in (20000, 20001, 20002):
            with self.subTest(mid):
                self.refused(mutate(lambda p: block(p, 0).update(model_id=mid)), "internal capture-engine model")
        self.refused(mutate(lambda p: block(p, 0).update(model_id=99999)), "not in this PC's device catalog")
        self.refused(mutate(lambda p: block(p, 0).update(model="Klon")), "in the catalog, not")
        self.refused(mutate(lambda p: block(p, 0).update(category="Reverb")), "differs from the catalog's")

    def _as_model(self, mid, **extra):
        m = catalog().models[mid]
        return mutate(lambda p: block(p, 0).update(model_id=mid, model=m["name"], category=m["category"],
                                                   index_evidence="plain" if m.get("clones") is None
                                                   else f"clone-{m['clones']}-g1",
                                                   params=[], at_default=[], **extra))

    def test_categories(self):
        for mid in (13004, 14000, 29001, 10004, 11000, 27000):  # FX loop, capture, IR, splitter, mixer, looper
            with self.subTest(mid):
                self.refused(self._as_model(mid), "is not pushed in v1")

    def test_side_chain(self):
        sc = next(m for m in catalog().models.values() if m["name"].endswith("(S/C)")
                  and m.get("plugin_id") is None and m.get("clones") is None)
        self.refused(self._as_model(sc["id"]), "side-chain models")

    def test_plugin_name_must_match_the_catalog(self):
        pm = next(m for m in catalog().models.values() if m.get("plugin_id") is not None and not m.get("clones")
                  and m["category"] == "Guitar Overdrive")
        self.refused(self._as_model(pm["id"]), "plugin model without its plugin name")
        self.refused(mutate(lambda p: block(p, 0).update(plugin="Plini")), "plugin on a model that needs none")
        self.refused(self._as_model(pm["id"], plugin=""), "plugin")

    def test_unverified_clone_base(self):
        st = next(m for m in catalog().models.values() if m.get("clones") == 32000 and m.get("plugin_id") is None)
        self.refused(self._as_model(st["id"]), "cloning 32000 are not verified")

    def test_index_evidence(self):
        self.refused(mutate(lambda p: block(p, 2).update(index_evidence="plain")), "index_evidence")
        self.refused(mutate(lambda p: block(p, 0).update(index_evidence="clone-12000-g1")), "index_evidence")

    def test_bypass(self):
        self.refused(mutate(lambda p: block(p, 0).update(bypass=[False, True])), "list of 8")
        self.refused(mutate(lambda p: block(p, 0).update(bypass=[0] * 8)), "true/false")
        self.refused(mutate(lambda p: block(p, 0).update(bypass=[False, False, True] + [False] * 5)),
                     "scenes C-H must equal scene A")

    # ---- params ----

    def test_index_bounds(self):
        # 12045 has param_count 21; index 21 is the device's trailing slot, 19/20 its hidden IR names.
        lvl = lambda **kw: mutate(lambda p: param(p, 2, 0).update(**kw))  # noqa: E731
        self.refused(lvl(index=21), "at or past param_count 21")
        self.refused(lvl(index=22), "at or past param_count 21")
        self.refused(lvl(index=19), "hidden or unlisted")
        self.refused(lvl(index=-1), "at or past param_count")
        self.refused(lvl(index=1), "not a user control")      # ir selector
        self.refused(lvl(index=0), "not a user control")      # the per-mic bypass switch

    def test_label_must_match_the_index(self):
        self.refused(mutate(lambda p: param(p, 2, 0).update(label="MIC 1 LEVEL")),
                     "index 10 is 'MIC 2 LEVEL' on this model, not 'MIC 1 LEVEL'")
        self.refused(mutate(lambda p: param(p, 0, 1).update(label="LEVEL")), "is 'TREBLE' on this model")

    def test_units(self):
        self.refused(mutate(lambda p: param(p, 2, 1).update(units="kHz")), "units")
        self.refused(mutate(lambda p: param(p, 2, 1).pop("units")), "units")
        self.refused(mutate(lambda p: param(p, 0, 1).update(units="dB")), "units")

    def test_strictly_increasing_indexes(self):
        def dup(p):
            ps = block(p, 1)["params"]  # Brit 2203: 2 of its 7 params written
            ps.append(copy.deepcopy(ps[-1]))
        self.refused(mutate(dup), "strictly increasing")
        self.refused(mutate(lambda p: block(p, 0)["params"].reverse()), "strictly increasing")

        def too_many(p):  # Myth Drive writes all 3 of its params; a 4th can't fit
            ps = block(p, 0)["params"]
            ps.append(copy.deepcopy(ps[-1]))
        self.refused(mutate(too_many), "at most 3")

    def test_value_lists(self):
        self.refused(mutate(lambda p: param(p, 0, 1).update(wire=[0.55] * 7)), "list of 8")
        self.refused(mutate(lambda p: param(p, 0, 1).update(wire=[0.55] * 7 + [float("nan")])), "finite")
        self.refused(mutate(lambda p: param(p, 0, 1).update(wire=[True] * 8)), "finite number")
        self.refused(mutate(lambda p: param(p, 0, 1).update(display=["5.5"] * 8)), "finite number")
        self.refused(mutate(lambda p: param(p, 0, 1).update(scene_mode="no")), "true/false")

    def test_c_to_h_follow_a(self):
        def drift(p):
            param(p, 0, 0)["wire"][5] = 0.8
            param(p, 0, 0)["display"][5] = 8
        self.refused(mutate(drift), "scenes C-H must equal scene A")

    def test_scene_mode_must_match_the_values(self):
        self.refused(mutate(lambda p: param(p, 0, 0).update(scene_mode=False)), "scene_mode doesn't match")
        self.refused(mutate(lambda p: param(p, 0, 1).update(scene_mode=True)), "scene_mode doesn't match")

    def test_wire_outside_0_1(self):
        def out(p):
            param(p, 0, 1)["wire"] = [1.5] * 8
        self.refused(mutate(out), "outside 0..1")

    def test_wire_must_be_the_taper_of_display(self):
        def off(p):
            w = param(p, 1, 1)["wire"][0] + 1e-6
            param(p, 1, 1)["wire"] = [w] * 8
        self.refused(mutate(off), "is not the taper of display -10")

        def linear(p):  # the cab's MIC 2 LEVEL written linearly instead of with skew 4.9594844
            param(p, 2, 0)["wire"] = [(-21.8 + 40) / 46] * 8
        self.refused(mutate(linear), "is not the taper")

    def test_display_outside_the_range(self):
        def big(p):
            param(p, 0, 1)["display"] = [11] * 8
            param(p, 0, 1)["wire"] = [1.1] * 8
        self.refused(mutate(big), "outside 0..1")

        def big_display_only(p):
            param(p, 0, 1)["display"] = [11] * 8
        self.refused(mutate(big_display_only), "outside 0..10")

    def _one_param(self, mid, idx, display, wire):
        m = catalog().models[mid]
        from qc_agent.catalog import labels
        lab = labels(m)[idx]
        cp = m["params_by_index"][idx]
        prm = {"index": idx, "label": lab, "scene_mode": False, "display": [display] * 8, "wire": [wire] * 8}
        if cp.get("units"):
            prm["units"] = cp["units"]
        return mutate(lambda p: block(p, 0).update(
            model_id=mid, model=m["name"], category=m["category"],
            index_evidence="plain" if m.get("clones") is None else f"clone-{m['clones']}-g1",
            params=[prm], at_default=[]))

    def test_unconfirmed_tapers_and_scales(self):
        cat = catalog()

        def find(pred):
            for m in cat.models.values():
                if m.get("plugin_id") is None and m.get("clones") in (None, 12000) \
                        and m["category"] != "IRLoaders" and not m["name"].endswith("(S/C)") \
                        and m["category"] in ("Delay", "Reverb", "Filter", "Equalizer", "Cabsim Guitar (M)",
                                              "Modulation", "Guitar Amplifier", "Compressor"):
                    for p in m["params"]:
                        if p.get("type") == "float" and pred(p):
                            return m["id"], p["index"]
            self.fail("no such param in the catalog")

        cases = [
            (lambda p: p.get("skew") == 0.6, "taper skew 0.6 is not confirmed"),
            (lambda p: p.get("skew_symbol") == "LOG_SKEW", "LOG_SKEW"),
            (lambda p: p.get("min_label") == "L", "pan display scale"),
            (lambda p: p.get("min_symbol") == "MIN_EQ_DB", "symbolic and not confirmed"),
            (lambda p: not p.get("units") and p.get("min") == 0 and p.get("max") == 1 and not p.get("skew"),
             "unitless 0..1"),
        ]
        for pred, needle in cases:
            mid, idx = find(pred)
            with self.subTest(needle):
                self.refused(self._one_param(mid, idx, 0, 0), needle)

    def test_non_float_types(self):
        sw = self._one_param(8006, 4, 1, 1.0)   # Spring TRAILS, a switch
        self.refused(sw, "type 'switch' has no measured wire mapping")

    def test_at_default(self):
        self.refused(mutate(lambda p: block(p, 1).update(at_default=["NOPE"])), "not a control of this model")
        self.refused(mutate(lambda p: block(p, 1).update(at_default=["MASTER", "MASTER"])), "listed twice")
        self.refused(mutate(lambda p: block(p, 1).update(at_default=["GAIN"])), "also written")
        self.refused(mutate(lambda p: block(p, 1).update(at_default=None)), "must be a list")


if __name__ == "__main__":
    unittest.main()
