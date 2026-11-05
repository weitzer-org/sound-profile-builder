"""QcMcpBackend + WireGuard + the real vendored transport, against fake_hid.FakeHID.

Proves, offline: the backend's calls match the vendored transport's API; the
handshake passes the wire guard untouched; a full push sends only the plan's
writes and exactly one File CREATE; and the guard stops every forbidden message
(RecallPreset SAVE, recall, deletes, global writes, off-plan grid writes, a save to
another slot) before it reaches the device. Proves nothing about the real unit.
"""
import copy
import shutil
import tempfile
import unittest
from pathlib import Path

from helpers import Clock, catalog, config, export, job
from qc_agent import jobs, qcmcp_backend
from qc_agent.device import UnitAbsent
from qc_agent.export_validator import validate
from qc_agent.ledger import OwnedSlots
from qc_agent.qcmcp_backend import GuardError, Guarded, QcMcpBackend, WireRefused
from qc_agent.vendor_pins import VENDOR_DIR
import fake_hid

jobs.SAVE_LIST_INTERVAL_S = 0
jobs.PACE_S = 0
T = fake_hid.M["transport"]
P = fake_hid.P


class Base(unittest.TestCase):
    def setUp(self):
        self.hid = fake_hid.FakeHID()
        self._orig = T.open_hid
        T.open_hid = lambda **kw: self.hid
        self.dev = QcMcpBackend()
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        T.open_hid = self._orig
        self.dev.disconnect()
        self.tmp.cleanup()

    def names(self):
        return [(n, getattr(o, "action", None)) for n, o in self.hid.received]

    def ctx(self):
        return jobs.JobContext(config(), catalog(), self.dev, OwnedSlots(Path(self.tmp.name) / "o.jsonl"),
                               Clock(), sleep=lambda s: None)


class Session(Base):
    def test_connect_reads_firmware_through_the_guard(self):
        self.dev.connect()
        self.assertEqual((self.dev.firmware, self.dev.device_type), ("4.1.0", "QC"))
        self.assertEqual(self.dev._guard.refused, [], "the handshake must pass the guard untouched")
        self.assertTrue(self.hid.opened)
        self.assertTrue(self.dev.alive())

    def test_disconnect_closes(self):
        self.dev.connect()
        self.dev.disconnect()
        self.assertTrue(self.hid.closed)
        self.assertFalse(self.dev.alive())

    def test_listing_and_read(self):
        self.hid.slots = {8: "bb king test 2"}
        self.dev.connect()
        listing = self.dev.list_my_presets()
        self.assertEqual(listing[8], "bb king test 2")
        self.assertEqual(listing[9], "")
        self.assertEqual(self.dev.setlist_position()["position"], 152)
        pr = self.dev.read_current_preset()
        self.assertEqual(pr["chains"][0]["blocks"], [])
        self.assertEqual(len(pr["bypass"]), 4)

    def test_short_listing_is_refused(self):
        self.dev.connect()
        orig = self.hid._handle

        def short(name, m):
            if name == "File" and m.action == P.ACTION["READ"]:
                f = P.message_class("File")(action=P.ACTION["UPDATE"], type=0)
                f.folder.key, f.folder.name = fake_hid.SETLIST, "My Presets"
                for _ in range(100):
                    f.folder.files.add().name = ""
                return self.hid._reply("File", f)
            return orig(name, m)
        self.hid._handle = short
        from qc_agent.device import DeviceError
        with self.assertRaises(DeviceError):
            self.dev.list_my_presets()

    def test_dead_reader_thread_means_gone(self):
        self.dev.connect()
        self.hid._thread.running = False
        self.assertFalse(self.dev.alive())

    def test_rising_write_errors_mean_gone(self):
        self.dev.connect()
        for n in (1, 2):
            self.dev._raw.write_errors = n
            self.assertTrue(self.dev.alive())
        self.dev._raw.write_errors = 3
        self.assertFalse(self.dev.alive())

    def test_no_device_maps_to_unit_absent(self):
        def boom(**kw):
            raise RuntimeError("no HID device 0x152a:0x880a found")
        T.open_hid = boom
        with self.assertRaises(UnitAbsent):
            self.dev.connect()


class PushOverTheWire(Base):
    def test_full_push(self):
        jid, key, data = job("push_preset")
        import json
        from qc_agent import contract
        j = json.loads(contract.open_envelope(config().keys, contract.TAG_JOB, data))
        out = jobs.run(self.ctx(), j)
        self.assertEqual((out.status, out.reason_code), ("succeeded", ""), out.message)
        self.assertTrue(out.push["listed"])
        self.assertEqual(out.push["mismatches"], 0)
        self.assertEqual(self.hid.slots[152], "Texas Blues")
        sent = self.names()
        A = P.ACTION
        self.assertNotIn(("RecallPreset", A["UPDATE"]), sent)          # never save_preset
        self.assertNotIn(("SetlistPosition", A["UPDATE"]), sent)       # never recall
        self.assertNotIn(("Grid", A["DELETE"]), sent)                  # never delete_block / clear_grid
        for name in ("IOSettings", "GlobalEQ", "MasterVolume", "Mode", "ModelPreset"):
            self.assertFalse([1 for n, a in sent if n == name and a != A["READ"]], name)
        creates = [o for n, o in self.hid.received if n == "File" and o.action == A["CREATE"]]
        self.assertEqual(len(creates), 1)
        self.assertEqual((creates[0].folder.key, creates[0].folder.files[0].index, creates[0].folder.files[0].name),
                         (fake_hid.SETLIST, 152, "Texas Blues"))
        self.assertEqual(self.dev._guard.refused, [])
        # G6 shape on the simulated grid: GAIN scene-varying with all eight scenes written
        for got, want in zip(self.hid.grid[0]["params"][0], [0.2, 0.8] + [0.2] * 6, strict=True):
            self.assertAlmostEqual(got, want, places=6)  # float32 on the wire
        self.assertEqual(self.hid.grid[4]["bypass"], [False, True] + [False] * 6)
        self.assertEqual(self.hid.routing[0], (1, 19))


class Guard(Base):
    def setUp(self):
        super().setUp()
        self.dev.connect()
        self.qc = self.dev._raw  # the unguarded transport object, to try forbidden sends directly
        self.plan = validate(export(), catalog(), "4.1.0")

    def refused(self, fn, *a, **kw):
        n = len(self.hid.received)
        with self.assertRaises(WireRefused):
            fn(*a, **kw)
        self.assertEqual(len(self.hid.received), n, "nothing reached the device")

    def test_save_preset_never_passes(self):
        self.dev.begin_push(152, "x", self.plan)
        self.refused(self.qc.save_preset)

    def test_recall_never_passes(self):
        self.refused(self.qc.recall, fake_hid.SETLIST, 3)

    def test_globals_never_pass(self):
        self.dev.begin_push(152, "x", self.plan)
        self.refused(self.qc.set_mode, 1)
        self.refused(self.qc.set_io_port, "in", 1, level=1.0)
        self.refused(self.qc.write_global_eq, [])
        m = P.message_class("MasterVolume")(action=P.ACTION["UPDATE"])
        self.refused(self.qc.send, "MasterVolume", m)

    def test_delete_never_passes(self):
        self.dev.begin_push(152, "x", self.plan)
        self.refused(self.qc.delete_block, 0, 0)
        f = P.message_class("File")(action=P.ACTION["DELETE"])
        f.folder.key = fake_hid.SETLIST
        self.refused(self.qc.send, "File", f)

    def test_writes_need_an_armed_push(self):
        self.refused(self.qc.set_param, 0, 0, 0, 0.5)
        self.refused(self.qc.add_block, 1, row=0, column=0)
        self.refused(self.qc.set_scene, 2)

    def test_off_plan_writes_are_refused(self):
        self.dev.begin_push(152, "x", self.plan)
        self.refused(self.qc.add_block, 1001, row=0, column=0)    # wrong model for column 0
        self.refused(self.qc.add_block, 1, row=0, column=6)       # column not in the plan
        self.refused(self.qc.add_block, 1, row=1, column=0)       # another row
        self.refused(self.qc.set_param, 0, 0, 7, 0.5)             # index not in the plan
        self.refused(self.qc.set_param, 0, 0, 1, 1.5)             # wire outside 0..1
        self.refused(self.qc.set_routing, 0, in_portid=2, out_portid=19)
        self.refused(self.qc.set_split_points, 0, 2, 4)
        self.refused(self.qc.set_param_typed, 0, 0, 1, [[None, None, "x"]])
        self.qc.set_param(0, 0, 1, 0.3)                           # the plan's own write passes

    def test_save_only_to_the_armed_slot(self):
        self.dev.begin_push(152, "Texas Blues", self.plan)
        self.refused(self.qc.write_preset_file, fake_hid.SETLIST, 152, "Texas Blues")  # not armed for a save yet
        self.dev._guard.arm_save(fake_hid.SETLIST, 152, "Texas Blues")
        self.refused(self.qc.write_preset_file, fake_hid.SETLIST, 153, "Texas Blues")
        self.dev._guard.arm_save(fake_hid.SETLIST, 152, "Texas Blues")
        self.refused(self.qc.write_preset_file, fake_hid.SETLIST, 152, "Other")
        self.dev._guard.arm_save(fake_hid.SETLIST, 152, "Texas Blues")
        self.refused(self.qc.write_preset_file, "/media/p4/Presets/Other", 152, "Texas Blues")
        self.dev._guard.arm_save(fake_hid.SETLIST, 152, "Texas Blues")
        self.qc.write_preset_file(fake_hid.SETLIST, 152, "Texas Blues")
        self.refused(self.qc.write_preset_file, fake_hid.SETLIST, 152, "Texas Blues")  # one save per arming

    def test_backend_save_must_match_the_push(self):
        self.dev.begin_push(152, "Texas Blues", self.plan)
        with self.assertRaises(Exception):
            self.dev.save_to_slot(153, "Texas Blues")

    def test_end_push_disarms(self):
        self.dev.begin_push(152, "x", self.plan)
        self.dev.end_push()
        self.refused(self.qc.set_param, 0, 0, 1, 0.3)


class MethodAllowlist(unittest.TestCase):
    def test_never_methods_raise(self):
        g = Guarded(object())
        for name in ("save_preset", "recall", "clear_grid", "send", "request", "delete_block", "set_mode",
                     "write_global_eq", "set_io_port", "load_settings_preset", "delete_model_preset",
                     "set_scene_label", "set_preset_meta"):
            with self.subTest(name), self.assertRaises(GuardError):
                getattr(g, name)

    def test_grant_cannot_include_never(self):
        with self.assertRaises(GuardError):
            Guarded(object(), grant={"open", "save_preset"})

    def test_unlisted_methods_raise(self):
        with self.assertRaises(GuardError):
            Guarded(object()).load_model_preset

    def test_read_only(self):
        with self.assertRaises(GuardError):
            Guarded(object()).x = 1


class VendoredLoad(unittest.TestCase):
    def test_modified_vendored_file_is_refused(self):
        with tempfile.TemporaryDirectory() as d:
            copy_dir = Path(d) / "vendor"
            shutil.copytree(VENDOR_DIR, copy_dir)
            p = copy_dir / "qc_mcp" / "transport.py"
            p.write_bytes(p.read_bytes() + b"\n# local change\n")
            saved = qcmcp_backend._loaded
            qcmcp_backend._loaded = None
            try:
                with self.assertRaises(Exception) as cm:
                    qcmcp_backend.load_vendored(copy_dir)
                self.assertIn("differs from the reviewed f634ecd copy", str(cm.exception))
            finally:
                qcmcp_backend._loaded = saved

    def test_imports_are_the_vendored_copy(self):
        mods = qcmcp_backend.load_vendored()
        for m in mods.values():
            self.assertTrue(Path(m.__file__).resolve().is_relative_to(VENDOR_DIR.resolve()))


if __name__ == "__main__":
    unittest.main()
