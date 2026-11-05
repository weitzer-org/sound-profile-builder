"""Offline tests for the Phase 0 spikes. No device needed.

They run the real spike scripts against a FAKE Quad Cortex built on qc-mcp's real protobuf messages.
They prove the guards, the refusal paths, the arithmetic and the report logic. They prove NOTHING about
how a real unit behaves; that is what the spikes themselves are for.

Needs the pinned qc-mcp clone (reads its ModelRepo.xml and protobuf descriptors) and protobuf:

    set QC_MCP_DIR=C:\\Users\\bweit\\qc-trial\\qc-mcp
    C:\\Users\\bweit\\qc-trial\\venv\\Scripts\\python.exe -m unittest discover -s tools\\device-spikes

Skipped (not failed) when QC_MCP_DIR is unset.
"""
from __future__ import annotations

import contextlib
import io
import os
import sys
import tempfile
import types
import unittest
from pathlib import Path
from unittest import mock

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import spike_common as sc  # noqa: E402
import test_preset as tp  # noqa: E402

QC_DIR = os.environ.get("QC_MCP_DIR", "")
HAVE = bool(QC_DIR) and (Path(QC_DIR) / "src" / "qc_mcp" / "transport.py").is_file()


# --------------------------------------------------------------------------- pure helpers (no qc-mcp needed)

class SlotTests(unittest.TestCase):
    def test_labels_round_trip(self):
        for pos in range(256):
            self.assertEqual(sc.parse_slot(sc.slot_label(pos)), pos)

    def test_known_slots(self):
        self.assertEqual(sc.parse_slot("1A"), 0)
        self.assertEqual(sc.parse_slot("2A"), 8)
        self.assertEqual(sc.parse_slot("12H"), 95)  # matches tools/qc-mcp-trial/README.md
        self.assertEqual(sc.parse_slot("32H"), 255)

    def test_bad_labels(self):
        for bad in ("", "A", "2", "2I", "0A", "xA", "2AA"):
            with self.assertRaises(sc.SpikeRefused, msg=bad):
                sc.parse_slot(bad)

    def test_empty_detection(self):
        folder = {"files": [{"index": 3, "name": "x"}, {"index": 4, "name": ""}]}
        self.assertFalse(sc.slot_is_empty(folder, 3))
        self.assertTrue(sc.slot_is_empty(folder, 4))   # blank name
        self.assertTrue(sc.slot_is_empty(folder, 5))   # absent entry


class TaperTests(unittest.TestCase):
    """Golden vectors from the audit's P1 test list (pyquadcortex 4.0.1 measurements)."""

    def test_amp_output(self):
        self.assertAlmostEqual(sc.predict_display(-60, 12, 3.8018, 0.5), 0.0, delta=0.02)
        self.assertAlmostEqual(sc.predict_display(-60, 12, 3.8018, 1.0), 12.0, places=6)

    def test_cab_level(self):
        lo, hi = sc.CABSIM_DB_RANGE
        self.assertAlmostEqual(sc.predict_display(lo, hi, 4.9594844, 0.5), 0.0, delta=0.02)
        self.assertAlmostEqual(sc.predict_display(lo, hi, 4.9594844, 0.01), -21.8, delta=0.05)
        self.assertAlmostEqual(sc.predict_display(lo, hi, 4.9594844, 1.0), 6.0, places=6)

    def test_hpf_freq(self):
        self.assertAlmostEqual(sc.predict_display(20, 20000, 0.3, 0.25), 217, delta=1.0)

    def test_refuses_out_of_range(self):
        for bad in (-0.1, 1.1, float("nan")):
            with self.assertRaises(ValueError):
                sc.predict_display(0, 1, 1, bad)


class FmtTests(unittest.TestCase):
    def test_never_raises(self):
        self.assertEqual(sc.fmt_num(0.123456789), "0.123457")
        self.assertEqual(sc.fmt_num(None), "None")
        self.assertEqual(sc.fmt_num("oops"), "'oops'")
        self.assertEqual(sc.fmt_num(3), "3")


class GuardTests(unittest.TestCase):
    class Stub:
        firmware = "4.1.0"
        def read_state(self, *a): return "ok"
        def set_param(self, *a): return "wrote"
        def save_preset(self, *a): raise AssertionError("must never be reached")
        def send(self, *a): raise AssertionError("must never be reached")

    def test_grants(self):
        g = sc.Guarded(self.Stub(), sc.GRANT_G4)
        self.assertEqual(g.read_state(), "ok")
        self.assertEqual(g.set_param(1, 2, 3, 4), "wrote")
        self.assertEqual(g.firmware, "4.1.0")

    def test_read_grant_cannot_write(self):
        g = sc.Guarded(self.Stub(), sc.GRANT_READ)
        with self.assertRaises(sc.GuardError):
            g.set_param(0, 0, 0, 0.5)

    def test_never_list_is_unreachable(self):
        g = sc.Guarded(self.Stub(), sc.GRANT_G3)
        for name in sorted(sc.NEVER):
            with self.assertRaises(sc.GuardError, msg=name):
                getattr(g, name)

    def test_cannot_grant_never(self):
        for name in ("save_preset", "send", "request", "clear_grid", "recall", "set_io_port", "write_global_eq"):
            with self.assertRaises(sc.GuardError, msg=name):
                sc.Guarded(self.Stub(), frozenset({name}))

    def test_no_spike_grant_contains_never(self):
        for grant in (sc.GRANT_READ, sc.GRANT_G3, sc.GRANT_G4, sc.GRANT_G6):
            self.assertFalse(set(grant) & sc.NEVER)

    def test_only_g3_can_save(self):
        self.assertIn("write_preset_file", sc.GRANT_G3)
        for grant in (sc.GRANT_READ, sc.GRANT_G4, sc.GRANT_G6):
            self.assertNotIn("write_preset_file", grant)

    def test_read_only_attribute(self):
        g = sc.Guarded(self.Stub(), sc.GRANT_READ)
        with self.assertRaises(sc.GuardError):
            g.firmware = "9.9"


# --------------------------------------------------------------------------- fake device

@unittest.skipUnless(HAVE, "set QC_MCP_DIR to the pinned qc-mcp clone")
class FakeDeviceCase(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.mcp = sc.load_qc_mcp(QC_DIR)  # also enforces the pin
        import qc_mcp.preset as qp
        cls.qp = qp
        cls.mcp.preset = qp
        cls.xml = sc.load_xml_models(cls.mcp.xml_path)
        cls.ours = sc.load_our_catalog()

    # ---- builders
    def blank_preset(self, name=""):
        bp = self.qp.BinaryPreset()
        bp.name = name
        for r in range(4):
            ch = bp.chains.add()
            ch.row = 0
            for _ in range(8):
                ch.models.add()
        return bp

    def put_block(self, bp, row, col, model_id, n_params=None, value=0.5):
        m = bp.chains[row].models[col]
        m.hash = model_id
        n = n_params if n_params is not None else (max(p["index"] for p in self.ours[model_id]["params"]) + 1)
        for i in range(n):
            p = m.params.add()
            p.index = 0
            p.param_values.add().float_value = value
        return m

    def fake(self, **kw):
        return FakeQC(self, **kw)

    # ---- running a script's main() with a patched device and argv
    def run_script(self, module_name, argv, qc, stdin=None):
        mod = __import__(module_name)
        out = io.StringIO()
        with tempfile.TemporaryDirectory() as td:
            full = ["x", "--qc-mcp", QC_DIR, "--out", td, *argv]
            patches = [mock.patch.object(sys, "argv", full),
                       mock.patch.object(sc, "open_device", lambda mcp, grant: _open(qc, grant)),
                       mock.patch("time.sleep", lambda *_: None)]
            if stdin is not None:
                patches.append(mock.patch("builtins.input", side_effect=list(stdin)))
            with contextlib.ExitStack() as st, contextlib.redirect_stdout(out):
                for p in patches:
                    st.enter_context(p)
                code = sc.run_spike(mod.main, argv)
            files = sorted(Path(td).glob("*.json"))
            import json
            data = json.loads(files[-1].read_text(encoding="utf-8")) if files else None
        return code, out.getvalue(), data


def _open(qc, grant):
    """Mirror sc.open_device: wrap, then open (so a fake that fails to open fails here too)."""
    g = sc.Guarded(qc, grant)
    g.open()
    return g


class FakeQC:
    """Just enough of QuadCortex, backed by a real BinaryPreset."""

    def __init__(self, case, firmware="4.1.0", device_type="QC", open_slot=8, folder=sc.SETLIST,
                 library=None, bp=None, dirty=False, raise_on_open=None):
        self.case = case
        self.firmware, self.device_type, self.custom_name = firmware, device_type, ""
        self.protocol_version = "4.1"
        self.write_errors, self.last_write_error = 0, None
        self.library = library if library is not None else {}
        self.open_slot, self.folder = open_slot, folder
        self.bp = bp if bp is not None else case.blank_preset()
        self.dirty = dirty
        self.calls = []
        self.io = types.SimpleNamespace(_thread=types.SimpleNamespace(is_alive=lambda: True), _h=object())
        self.raise_on_open = raise_on_open
        self.scene = 0
        self.alive = True

    # lifecycle
    def open(self, handshake=True):
        if self.raise_on_open:
            raise RuntimeError(self.raise_on_open)
        return self

    def close(self):
        self.calls.append("close")

    # reads
    def get_setlist_position(self):
        if not self.alive:
            raise self.case.mcp.transport.QCError("no response to SetlistPosition within 2500ms")
        return {"folder_key": self.folder, "position": self.open_slot, "is_factory": False}

    def get_current_preset(self, timeout_ms=6000):
        return self.bp

    def read_state(self, command, timeout_ms=2500):
        if command == "PresetDirty":
            return types.SimpleNamespace(is_dirty=self.dirty)
        if command == "Scene":
            return types.SimpleNamespace(selected_scene=self.scene)
        return None

    def list_directory(self, timeout_ms=30000, quiet_ms=1500):
        files = [{"index": i, "name": n, "key": "", "cloud_id": "", "author": "", "version": "", "instrument": 0,
                  "is_readonly": False, "date_ms": 0} for i, n in sorted(self.library.items())]
        return {"presets": [{"key": self.folder, "name": "My Presets", "is_factory": False, "is_downloads": False,
                             "is_user_default": True, "files": files}], "irs": [], "captures": []}

    # writes
    def set_routing(self, row, in_portid=None, out_portid=None):
        self.calls.append(("set_routing", row, in_portid, out_portid))
        self.bp.chains[row].in_portid, self.bp.chains[row].out_portid = in_portid, out_portid

    def add_block(self, model_hash, row=0, column=0, wait_echo_ms=800):
        self.calls.append(("add_block", model_hash, row, column))
        self.case.put_block(self.bp, row, column, model_hash, value=0.5)
        return {"row": row, "column": column, "hash": model_hash}

    def set_param(self, row, column, param_index, value):
        self.calls.append(("set_param", row, column, param_index, value))
        self.bp.chains[row].models[column].params[param_index].param_values[0].float_value = value

    def set_param_scenes(self, row, column, param_index, values):
        self.calls.append(("set_param_scenes", row, column, param_index, tuple(values)))
        p = self.bp.chains[row].models[column].params[param_index]
        p.scene_mode = True
        del p.param_values[:]
        for v in values:
            p.param_values.add().float_value = v

    def set_scene(self, scene):
        self.calls.append(("set_scene", scene))
        self.scene = scene

    def set_block_bypass(self, row, column, bypassed=True, scenes=None, bypass_param=None):
        self.calls.append(("set_block_bypass", row, column, tuple(scenes or ())))

    def write_preset_file(self, folder_key, position, name, binary_preset=None):
        self.calls.append(("write_preset_file", folder_key, position, name))
        self.library[position] = name


# --------------------------------------------------------------------------- scenarios

class ModelChecks(FakeDeviceCase):
    def test_test_preset_matches_xml(self):
        self.assertEqual(tp.validate_against_xml(self.xml), [])

    def test_validation_catches_a_wrong_name_and_denied_model(self):
        bad = dict(self.xml)
        bad[1] = dict(bad[1], name="Something Else")
        self.assertTrue(any("expected" in p for p in tp.validate_against_xml(bad)))
        with mock.patch.object(tp, "DENY_MODEL_IDS", frozenset({1})):
            self.assertTrue(any("deny list" in p for p in tp.validate_against_xml(self.xml)))

    def test_pin_is_enforced(self):
        with mock.patch.object(sc, "PINNED_COMMIT", "0" * 40):
            with self.assertRaises(sc.SpikeRefused):
                sc.load_qc_mcp(QC_DIR)

    def test_clone_expansion_visible_in_both_catalogs(self):
        # The fact G1 tests: the unexpanded XML list is 2 params, our expansion is 19.
        self.assertEqual(len(self.xml[12001]["params"]), 2)
        self.assertEqual(self.xml[12001]["clones"], "12000")
        self.assertEqual(max(p["index"] for p in self.ours[12001]["params"]) + 1, 19)


class G0Tests(FakeDeviceCase):
    def test_pass(self):
        qc = self.fake(library={8: "bb king test 2", 9: "other"})
        code, out, data = self.run_script("g0_verify_slot", [], qc)
        self.assertEqual(code, sc.EXIT_OK, out)
        self.assertEqual(data["verdict"], "PASS")
        self.assertIn("calls", dir(qc))
        self.assertEqual([c for c in qc.calls if isinstance(c, tuple)], [])  # nothing written

    def test_fail_when_elsewhere(self):
        qc = self.fake(library={8: "other", 20: "bb king test 2"})
        code, out, data = self.run_script("g0_verify_slot", [], qc)
        self.assertEqual(code, sc.EXIT_CHECK_FAILED)
        self.assertEqual(data["verdict"], "PRESENT_ELSEWHERE")

    def test_fail_when_empty(self):
        qc = self.fake(library={})
        code, _, data = self.run_script("g0_verify_slot", [], qc)
        self.assertEqual(code, sc.EXIT_CHECK_FAILED)
        self.assertEqual(data["verdict"], "FAIL")

    def test_mini_is_refused(self):
        qc = self.fake(device_type="ATMA", library={8: "bb king test 2"})
        code, out, _ = self.run_script("g0_verify_slot", [], qc)
        self.assertEqual(code, sc.EXIT_REFUSED, out)


class G1Tests(FakeDeviceCase):
    def preset_with(self, cab_params, spring_params=5):
        bp = self.blank_preset("bb king test 2")
        self.put_block(bp, 0, 1, 1001)
        self.put_block(bp, 0, 2, 12001, n_params=cab_params)
        self.put_block(bp, 0, 3, 8006, n_params=spring_params)
        return bp

    def test_expansion_supported(self):
        qc = self.fake(bp=self.preset_with(19))
        code, out, data = self.run_script("g1_dump_params", [], qc)
        self.assertEqual(code, sc.EXIT_OK, out)
        self.assertEqual(data["verdict"], "COUNTS_SUPPORT_OUR_EXPANSION")
        cab = [b for b in data["blocks"] if b["hash"] == 12001][0]
        self.assertEqual((cab["n_device_params"], cab["n_x"], cab["n_o"]), (19, 2, 19))
        self.assertIn("binary_preset_hex", data)
        self.assertEqual(bytes.fromhex(data["binary_preset_hex"]), qc.bp.SerializeToString())

    def test_unexpanded_supported(self):
        qc = self.fake(bp=self.preset_with(2))
        _, _, data = self.run_script("g1_dump_params", [], qc)
        self.assertEqual(data["verdict"], "COUNTS_SUPPORT_UNEXPANDED")

    def test_neither(self):
        qc = self.fake(bp=self.preset_with(11))
        _, _, data = self.run_script("g1_dump_params", [], qc)
        self.assertEqual(data["verdict"], "COUNTS_DO_NOT_DECIDE")

    def test_refuses_wrong_open_slot(self):
        qc = self.fake(bp=self.preset_with(19), open_slot=9)
        code, out, data = self.run_script("g1_dump_params", [], qc)
        self.assertEqual(code, sc.EXIT_REFUSED, out)
        self.assertIn("never recalls", out)

    def test_refuses_wrong_name(self):
        bp = self.preset_with(19)
        bp.name = "something else"
        code, out, _ = self.run_script("g1_dump_params", [], self.fake(bp=bp))
        self.assertEqual(code, sc.EXIT_REFUSED, out)

    def test_refuses_dirty(self):
        code, out, _ = self.run_script("g1_dump_params", [], self.fake(bp=self.preset_with(19), dirty=True))
        self.assertEqual(code, sc.EXIT_REFUSED, out)

    def test_no_clone_preset_is_inconclusive(self):
        bp = self.blank_preset("bb king test 2")
        self.put_block(bp, 0, 0, 1)
        code, _, data = self.run_script("g1_dump_params", [], self.fake(bp=bp))
        self.assertEqual(code, sc.EXIT_CHECK_FAILED)
        self.assertEqual(data["verdict"], "INCONCLUSIVE_NO_CLONES")

    def test_never_writes(self):
        qc = self.fake(bp=self.preset_with(19))
        self.run_script("g1_dump_params", [], qc)
        self.assertEqual([c for c in qc.calls if isinstance(c, tuple)], [])


class G3Tests(FakeDeviceCase):
    ARGS = ["--slot", "24H", "--firmware", "4.1.0"]
    SLOT = sc.parse_slot("24H")

    def new(self, **kw):
        kw.setdefault("open_slot", self.SLOT)
        kw.setdefault("library", {8: "bb king test 2"})
        return self.fake(**kw)

    def writes(self, qc):
        return [c for c in qc.calls if isinstance(c, tuple)]

    def test_preflight_writes_nothing(self):
        qc = self.new()
        code, out, data = self.run_script("g3_build_and_save", self.ARGS, qc)
        self.assertEqual(code, sc.EXIT_OK, out)
        self.assertEqual(data["verdict"], "PREFLIGHT_OK")
        self.assertEqual(self.writes(qc), [])

    def test_firmware_required_and_exact(self):
        code, out, _ = self.run_script("g3_build_and_save", ["--slot", "24H"], self.new())
        self.assertEqual(code, sc.EXIT_REFUSED, out)
        code, out, _ = self.run_script("g3_build_and_save", ["--slot", "24H", "--firmware", "4.1.1"], self.new())
        self.assertEqual(code, sc.EXIT_REFUSED, out)
        self.assertIn("mismatch", out)

    def test_refuses_occupied_slot(self):
        qc = self.new(library={self.SLOT: "my precious preset"})
        code, out, _ = self.run_script("g3_build_and_save", self.ARGS + ["--apply"], qc)
        self.assertEqual(code, sc.EXIT_REFUSED, out)
        self.assertEqual(self.writes(qc), [])

    def test_refuses_when_other_slot_is_open(self):
        qc = self.new(open_slot=8)
        code, out, _ = self.run_script("g3_build_and_save", self.ARGS + ["--apply"], qc)
        self.assertEqual(code, sc.EXIT_REFUSED, out)
        self.assertEqual(self.writes(qc), [])

    def test_refuses_nonblank_grid(self):
        bp = self.blank_preset()
        self.put_block(bp, 0, 0, 1)
        qc = self.new(bp=bp)
        code, out, _ = self.run_script("g3_build_and_save", self.ARGS + ["--apply"], qc)
        self.assertEqual(code, sc.EXIT_REFUSED, out)
        self.assertEqual(self.writes(qc), [])

    def test_apply_builds_and_verifies_without_saving(self):
        qc = self.new()
        code, out, data = self.run_script("g3_build_and_save", self.ARGS + ["--apply"], qc)
        self.assertEqual(code, sc.EXIT_OK, out)
        self.assertEqual(data["verdict"], "APPLY_OK_NOT_SAVED")
        self.assertTrue(data["readback_ok"])
        self.assertFalse([c for c in qc.calls if isinstance(c, tuple) and c[0] == "write_preset_file"])
        kinds = {c[0] for c in self.writes(qc)}
        self.assertEqual(kinds, {"set_routing", "add_block", "set_param"})
        self.assertNotIn(self.SLOT, qc.library)
        # the cab LEVEL (index 2) was written with the plan's wire value
        self.assertIn(("set_param", 0, tp.CAB_COL, tp.CAB_LEVEL_IDX, 0.5), qc.calls)

    def test_save_requires_typed_confirmation(self):
        qc = self.new()
        code, out, data = self.run_script("g3_build_and_save", self.ARGS + ["--apply", "--save"], qc, stdin=["nope"])
        self.assertEqual(code, sc.EXIT_CHECK_FAILED, out)
        self.assertEqual(data["verdict"], "CANCELLED_AT_CONFIRM")
        self.assertNotIn(self.SLOT, qc.library)

    def test_save_happy_path(self):
        qc = self.new()
        code, out, data = self.run_script("g3_build_and_save", self.ARGS + ["--apply", "--save"], qc, stdin=["24h"])
        self.assertEqual(code, sc.EXIT_OK, out)
        self.assertEqual(data["verdict"], "SAVED_AND_LISTED")
        self.assertEqual(qc.library[self.SLOT], tp.TEST_PRESET_NAME)
        saves = [c for c in qc.calls if c[0] == "write_preset_file"]
        self.assertEqual(saves, [("write_preset_file", sc.SETLIST, self.SLOT, tp.TEST_PRESET_NAME)])

    def test_save_aborts_if_slot_filled_meanwhile(self):
        qc = self.new()
        real = qc.list_directory
        calls = {"n": 0}

        def sneaky(*a, **k):
            calls["n"] += 1
            if calls["n"] >= 2:                      # the re-check just before the save
                qc.library[self.SLOT] = "someone saved here"
            return real(*a, **k)
        qc.list_directory = sneaky
        code, out, _ = self.run_script("g3_build_and_save", self.ARGS + ["--apply", "--save"], qc, stdin=["24H"])
        self.assertEqual(code, sc.EXIT_REFUSED, out)
        self.assertFalse([c for c in qc.calls if c[0] == "write_preset_file"])
        self.assertEqual(qc.library[self.SLOT], "someone saved here")

    def test_readback_mismatch_blocks_save(self):
        qc = self.new()
        orig = qc.set_param

        def lossy(row, col, idx, value):
            orig(row, col, idx, value * 0.5 if (col, idx) == (tp.CAB_COL, tp.CAB_LEVEL_IDX) else value)
        qc.set_param = lossy
        code, out, data = self.run_script("g3_build_and_save", self.ARGS + ["--apply", "--save"], qc, stdin=["24H"])
        self.assertEqual(code, sc.EXIT_CHECK_FAILED, out)
        self.assertEqual(data["verdict"], "FAIL_READBACK")
        self.assertFalse([c for c in qc.calls if c[0] == "write_preset_file"])


class G4G6Tests(FakeDeviceCase):
    def saved_preset(self):
        bp = self.blank_preset(tp.TEST_PRESET_NAME)
        for b in tp.BLOCKS:
            self.put_block(bp, 0, b.col, b.model_id)
        return bp

    def test_g4_refuses_other_preset(self):
        bp = self.blank_preset("not the test one")
        code, out, _ = self.run_script("g4_taper_check", ["--slot", "24H", "--firmware", "4.1.0"],
                                       self.fake(bp=bp, open_slot=sc.parse_slot("24H")))
        self.assertEqual(code, sc.EXIT_REFUSED, out)

    def test_g4_confirms_when_screen_matches_taper(self):
        qc = self.fake(bp=self.saved_preset(), open_slot=sc.parse_slot("24H"))
        readings = []
        for _label, _col, _idx, vmin, vmax, skew, _unit, wires, _tol in __import__("g4_taper_check").CASES:
            for w in wires:
                readings.append(f"{sc.predict_display(vmin, vmax, skew, w):.2f}")
        code, out, data = self.run_script("g4_taper_check", ["--slot", "24H", "--firmware", "4.1.0"], qc, stdin=readings)
        self.assertEqual(code, sc.EXIT_OK, out)
        self.assertEqual(data["verdict"], "TAPER_CONFIRMED")
        self.assertFalse([c for c in qc.calls if isinstance(c, tuple) and c[0] != "set_param"])

    def test_g4_survives_non_float_readback(self):
        qc = self.fake(bp=self.saved_preset(), open_slot=sc.parse_slot("24H"))
        orig = sc.param_value
        with mock.patch.object(sc, "param_value", lambda p: "garbage"):
            code, out, data = self.run_script("g4_taper_check", ["--slot", "24H", "--firmware", "4.1.0", "--no-prompt"], qc)
        self.assertNotIn("UNEXPECTED ERROR", out)
        self.assertEqual(data["wire_roundtrip_failures"], 9)

    def test_g4_detects_linear_unit(self):
        qc = self.fake(bp=self.saved_preset(), open_slot=sc.parse_slot("24H"))
        readings = []
        for _l, _c, _i, vmin, vmax, _s, _u, wires, _t in __import__("g4_taper_check").CASES:
            for w in wires:
                readings.append(str(vmin + (vmax - vmin) * w))
        _, _, data = self.run_script("g4_taper_check", ["--slot", "24H", "--firmware", "4.1.0"], qc, stdin=readings)
        self.assertEqual(data["verdict"], "TAPER_NOT_CONFIRMED")

    def test_g6_fake_run(self):
        qc = self.fake(bp=self.saved_preset(), open_slot=sc.parse_slot("24H"))
        code, out, data = self.run_script("g6_scenes", ["--slot", "24H", "--firmware", "4.1.0", "--no-prompt"], qc)
        self.assertEqual(code, sc.EXIT_OK, out)
        self.assertEqual(len(data["stops"]), 3)
        kinds = {c[0] for c in qc.calls if isinstance(c, tuple)}
        self.assertEqual(kinds, {"set_param_scenes", "set_block_bypass", "set_scene"})

    def test_g6_refuses_wrong_preset(self):
        code, out, _ = self.run_script("g6_scenes", ["--slot", "24H", "--firmware", "4.1.0", "--no-prompt"],
                                       self.fake(bp=self.blank_preset("x"), open_slot=sc.parse_slot("24H")))
        self.assertEqual(code, sc.EXIT_REFUSED, out)


class G2Tests(FakeDeviceCase):
    def test_a_passes_twice(self):
        bp = self.blank_preset("p")
        self.put_block(bp, 0, 0, 1)
        code, out, data = self.run_script("g2_session", ["--check", "a"], self.fake(bp=bp))
        self.assertEqual(code, sc.EXIT_OK, out)
        self.assertEqual(len(data["sessions"]), 2)

    def test_b_clean_refusal(self):
        qc = self.fake(raise_on_open="the Quad Cortex is already open exclusively - quit Cortex Control")
        code, out, data = self.run_script("g2_session", ["--check", "b"], qc)
        self.assertEqual(code, sc.EXIT_OK, out)
        self.assertEqual(data["verdict"], "PASS")

    def test_b_flags_unexpected_open(self):
        code, _, data = self.run_script("g2_session", ["--check", "b"], self.fake())
        self.assertEqual(code, sc.EXIT_CHECK_FAILED)
        self.assertEqual(data["verdict"], "FAIL_OPENED_ANYWAY")

    def test_c_detects_unit_going_away(self):
        qc = self.fake()
        n = {"i": 0}
        real = qc.get_setlist_position

        def dying():
            n["i"] += 1
            if n["i"] == 3:
                qc.alive = False
                qc.io._thread = types.SimpleNamespace(is_alive=lambda: False)
            return real()
        qc.get_setlist_position = dying
        code, out, data = self.run_script("g2_session", ["--check", "c", "--max-wait", "30", "--dead-probes", "3"], qc)
        self.assertEqual(code, sc.EXIT_OK, out)
        self.assertIn("probe_failed", data["signals_first_seen_s"])
        self.assertIn("reader_thread_dead", data["signals_first_seen_s"])

    def test_c_reports_nothing_detected(self):
        code, _, data = self.run_script("g2_session", ["--check", "c", "--max-wait", "3"], self.fake())
        self.assertEqual(code, sc.EXIT_CHECK_FAILED)
        self.assertEqual(data["verdict"], "FAIL_NOT_DETECTED")


if __name__ == "__main__":
    unittest.main()
