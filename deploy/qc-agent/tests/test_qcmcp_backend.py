"""QcMcpBackend + WireGuard + the real vendored transport, against fake_hid.FakeHID.

Proves, offline: the backend's calls match the vendored transport's API; the
handshake passes the wire guard untouched; a full push sends only the plan's
writes and exactly one File CREATE; and the guard stops every forbidden message
(RecallPreset SAVE, recall, deletes, global writes, off-plan grid writes, a save to
another slot) before it reaches the device. Proves nothing about the real unit.
"""
import shutil
import sys
import tempfile
import unittest
from unittest import mock
from pathlib import Path

from helpers import Clock, catalog, config, export, job
from qc_agent import jobs, qcmcp_backend
from qc_agent.device import DeviceError, UnitAbsent
from qc_agent.export_validator import validate
from qc_agent.ledger import OwnedSlots
from qc_agent.qcmcp_backend import GuardError, Guarded, QcMcpBackend, WireRefused
from qc_agent.vendor_pins import VENDOR_DIR
import fake_hid

def setUpModule():
    # Pacing off for this module only; restored afterwards so other modules see the real values.
    from unittest import mock
    for name in ("SAVE_LIST_INTERVAL_S", "PACE_S"):
        p = mock.patch.object(jobs, name, 0)
        p.start()
        unittest.addModuleCleanup(p.stop)
T = fake_hid.M["transport"]
P = fake_hid.P


class Base(unittest.TestCase):
    def setUp(self):
        self.hid = fake_hid.FakeHID()
        patcher = mock.patch.object(T, "open_hid", side_effect=lambda **kw: self.hid)
        patcher.start()
        self.addCleanup(patcher.stop)
        self.dev = QcMcpBackend()
        self.addCleanup(self.dev.disconnect)
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

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
        with self.assertRaisesRegex(DeviceError, "not 256"):
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
        with mock.patch.object(T, "open_hid", side_effect=RuntimeError("no HID device 0x152a:0x880a found")):
            with self.assertRaisesRegex(UnitAbsent, "no HID device"):
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
            self.assertEqual([(n, a) for n, a in sent if n == name and a != A["READ"]], [], name)
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


class StrictPosition(Base):
    """The push decides "is the target slot still the open one" from SetlistPosition reads, so only
    the reply to that very READ may count: never a late reply to an earlier one."""

    def stale(self, rid, position=7):
        self.hid._reply("SetlistPosition", P.message_class("SetlistPosition")(
            action=P.ACTION["UPDATE"], request_id=rid, folder_key=fake_hid.SETLIST, position=position))

    def test_a_reply_to_another_request_is_ignored(self):
        self.dev.connect()
        self.stale(rid=9999)
        self.assertEqual(self.dev.setlist_position()["position"], self.hid.open_position)

    def test_a_late_reply_after_a_timeout_is_not_used_later(self):
        self.dev.connect()
        self.hid.position_replies = False
        with self.assertRaisesRegex(DeviceError, "carrying request_id"):
            self.dev._call("get_setlist_position", timeout_ms=200)
        self.stale(rid=self.dev._raw._req_id)  # the timed-out READ's answer finally arrives
        self.hid.position_replies = True
        self.assertEqual(self.dev.setlist_position()["position"], self.hid.open_position)

    def test_a_unit_that_doesnt_echo_ids_fails_closed(self):
        self.hid.echo_ids = False
        self.dev.connect()
        with self.assertRaisesRegex(DeviceError, "carrying request_id"):
            self.dev._call("get_setlist_position", timeout_ms=200)

    def test_push_refuses_without_a_correlated_position(self):
        self.hid.echo_ids = False
        import json
        from qc_agent import contract
        _jid, _key, data = job("push_preset")
        out = jobs.run(self.ctx(), json.loads(contract.open_envelope(config().keys, contract.TAG_JOB, data)))
        self.assertNotEqual(out.status, "succeeded")
        A = P.ACTION
        self.assertEqual([(n, o.action) for n, o in self.hid.received if n in ("Grid", "File") and o.action != A["READ"]],
                         [], "nothing may be written when the open slot can't be read reliably")
        self.assertEqual(self.hid.grid, {})


class PendingQueue(Base):
    """Upstream request() never removed the matched reply (`t is not (cmd, obj, raw, pb)` compares
    with a new tuple), so every unconsumed message doubled per matched request. With one broadcast
    per Scene read, twelve requests grew _pending to 6,153 entries; a per-scene write makes ~9."""

    def test_reply_to_a_request_without_id_is_consumed(self):
        self.dev.connect()
        msg = P.message_class("Scene")(action=P.ACTION["READ"])  # no request_id: the upstream fallback branch
        for _ in range(3):
            self.dev._raw.request("Scene", msg)
            self.assertEqual([o for c, o, *_ in self.dev._raw._pending if c == P.NAME_TO_CMD["Scene"]], [],
                             "the matched reply must not stay queued for the next request")

    def test_pending_grows_linearly_not_exponentially(self):
        orig = self.hid._handle

        def with_broadcast(name, m):
            if name == "Scene" and m.action == P.ACTION["READ"]:
                self.hid._reply("GlobalTempo", P.message_class("GlobalTempo")())
            return orig(name, m)
        self.hid._handle = with_broadcast
        self.dev.connect()
        raw = self.dev._raw
        self.assertIsInstance(raw, T.QuadCortex)
        self.assertIsNot(type(raw).request, T.QuadCortex.request, "the fixed request() overrides upstream's")
        self.assertIsNot(type(raw).get_setlist_position, T.QuadCortex.get_setlist_position)
        plan = validate(export(), catalog(), "4.1.0")
        self.dev.begin_push(152, "x", plan)
        self.dev.add_block(0, 0, 1)
        for _ in range(3):
            self.dev.set_param_scenes(0, 0, 0, [0.2, 0.8] + [0.2] * 6)
        self.assertLessEqual(len(self.dev._raw._pending), 40)  # ~9 unconsumed broadcasts per call
        for got, want in zip(self.hid.grid[0]["params"][0], [0.2, 0.8] + [0.2] * 6, strict=True):
            self.assertAlmostEqual(got, want, places=6)


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
        self.qc.set_param(0, 0, 1, 0.55)                          # the plan's own write passes

    def raw_grid(self, build, extra=b""):
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        build(g)
        full = P.encode_message("Grid", g.SerializeToString() + extra)
        guard = self.dev._guard
        n = len(self.hid.received)
        with self.assertRaises(WireRefused):
            for r in P.message_to_reports(full):
                guard.set_report(P.REPORT_HOST_TO_QC, r[1:], include_id=True)
        self.assertEqual(len(self.hid.received), n)

    def test_empty_model_entry_refused(self):
        self.dev.begin_push(152, "x", self.plan)
        self.raw_grid(lambda g: g.preset.chains.add().models.add())

    def test_unknown_fields_refused(self):
        self.dev.begin_push(152, "x", self.plan)
        self.raw_grid(lambda g: g.preset.chains.add(), extra=bytes([0xF8, 0xFF, 0x03, 0x01]))

    def test_compressed_payload_refused(self):
        self.dev.begin_push(152, "x", self.plan)
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        full = P.encode_message("Grid", P.gzip_bytes(g.SerializeToString()))
        with self.assertRaises(WireRefused):
            for r in P.message_to_reports(full):
                self.dev._guard.set_report(P.REPORT_HOST_TO_QC, r[1:], include_id=True)

    def test_scene_assign_on_bypass_column_only_past_the_params(self):
        self.dev.begin_push(152, "x", self.plan)  # the Spring (column 4) has scene-varying bypass

        def assign(idx):
            def build(g):
                m = g.preset.chains.add().models.add()
                m.column = 4
                p = m.params.add()
                p.index, p.scene_mode = idx, True
            return build
        self.raw_grid(assign(1))       # DAMPING: a real parameter, not the bypass control
        self.raw_grid(assign(6))       # nothing armed yet: the unit's own count hasn't been read
        self.dev._guard.allow_bypass_index(4, 6)   # what set_block_bypass arms from the unit's read
        self.raw_grid(assign(5))
        self.raw_grid(assign(7))
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        assign(6)(g)
        self.qc.send("Grid", g)

    def test_values_and_flags_must_be_the_plans(self):
        self.dev.begin_push(152, "x", self.plan)
        self.refused(self.qc.set_param, 0, 0, 1, 0.56)  # TREBLE is 0.55 in the plan
        self.qc.set_param(0, 0, 1, 0.55)
        self.dev._guard._push["flags"][4] = {False}     # a plan where the Spring is never bypassed
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        b = g.preset.bypass.add()
        cb = b.colBypass.add()
        cb.column = 4
        cb.sceneBypass.add().bypass = True
        n = len(self.hid.received)
        with self.assertRaises(WireRefused):
            self.qc.send("Grid", g)
        self.assertEqual(len(self.hid.received), n)

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
        with self.assertRaisesRegex(DeviceError, "save target differs from the armed push"):
            self.dev.save_to_slot(153, "Texas Blues")

    def test_save_authorisation_is_single_use_and_ends_with_the_push(self):
        self.dev.begin_push(152, "Texas Blues", self.plan)
        self.dev.save_to_slot(152, "Texas Blues")
        with self.assertRaisesRegex(DeviceError, "save target differs from the armed push"):
            self.dev.save_to_slot(152, "Texas Blues")
        self.dev.begin_push(152, "Texas Blues", self.plan)
        self.dev.end_push()
        with self.assertRaisesRegex(DeviceError, "save target differs from the armed push"):
            self.dev.save_to_slot(152, "Texas Blues")

    def test_end_push_disarms(self):
        self.dev.begin_push(152, "x", self.plan)
        self.dev.end_push()
        self.refused(self.qc.set_param, 0, 0, 1, 0.3)


class CapturesOverTheWire(Base):
    """A capture block through the real vendored set_capture and the wire guard (plan 4.2)."""

    def capture_job(self):
        import json
        from helpers import go_golden
        from qc_agent import contract
        args = {"slot": "20A", "name": "British Invasion", "source": {"preset_id": "p-9", "guitar": "ES-339"},
                "export": go_golden("03_British_Invasion")}
        _jid, _key, data = job("push_preset", args=args)
        return json.loads(contract.open_envelope(config().keys, contract.TAG_JOB, data))

    def test_capture_push(self):
        from fake_device import KEY_USER
        out = jobs.run(self.ctx(), self.capture_job())
        self.assertEqual((out.status, out.reason_code), ("succeeded", ""), out.message)
        self.assertEqual(out.push["mismatches"], 0)
        self.assertEqual(self.dev._guard.refused, [])
        A = P.ACTION
        grids = [o for n, o in self.hid.received if n == "Grid" and o.action == A["UPDATE"]]
        placed = [(mm.column, mm.hash) for g in grids for ch in g.preset.chains for mm in ch.models if mm.hash]
        self.assertIn((3, 14000), placed)
        strings = [(mm.column, p.index, p.param_values[0].string_value) for g in grids for ch in g.preset.chains
                   for mm in ch.models for p in mm.params
                   if p.param_values and p.param_values[0].WhichOneof("value") == "string_value"]
        self.assertEqual(strings, [(3, 5, KEY_USER + "66 AC30 JMP OD")])  # exactly one string write
        sent = {n for n, _o in self.hid.received}
        for never in ("ProductForward", "NeuralCapture", "NeuralCapture2"):
            self.assertNotIn(never, sent)
        creates = [o for n, o in self.hid.received if n == "File" and o.action == A["CREATE"]]
        self.assertEqual(len(creates), 1)
        self.assertEqual(self.hid.grid[3]["strings"][5], KEY_USER + "66 AC30 JMP OD")
        self.assertIn("V1 (user)", out.message)

    def test_snapshot_parses_capture_fields(self):
        self.hid.preset_deps = {9: ["AB" * 32]}
        self.hid.slots = {9: "Saved"}
        self.dev.connect()
        snap = self.dev.read_directory()
        self.assertEqual(snap.my_presets[9], "Saved")
        self.assertEqual(snap.preset_deps[9], frozenset({"ab" * 32}))
        names = {(e.name, e.folder_name, e.is_factory, e.version_field) for e in snap.captures}
        self.assertIn(("CA John's 2", "Factory Captures V2", True, "2"), names)
        self.assertIn(("66 AC30 JMP OD", "My Captures", False, "1"), names)


class CaptureGuard(Base):
    def setUp(self):
        super().setUp()
        self.dev.connect()
        self.qc = self.dev._raw
        from dataclasses import replace
        from helpers import go_golden
        from qc_agent import captures
        from qc_agent.export_validator import CAPTURE_BLOCK_IDS
        plan = validate(go_golden("03_British_Invasion"), catalog(), "4.1.0")
        res = captures.resolve(plan.blocks, self.dev.read_directory())
        self.plan = captures.with_resolution(plan, res, CAPTURE_BLOCK_IDS)
        self.cap = next(b for b in self.plan.blocks if b.capture)
        self.replace = replace

    def string_write(self, col, index, value):
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"], request_id=self.qc.next_request_id())
        ch = g.preset.chains.add()
        m = ch.models.add()
        m.column = col
        p = m.params.add()
        p.index = index
        p.param_values.add().string_value = value
        self.qc.send("Grid", g)

    def refused(self, fn, *a):
        n = len(self.hid.received)
        with self.assertRaises(WireRefused):
            fn(*a)
        self.assertEqual(len(self.hid.received), n, "nothing reached the device")

    def test_only_the_armed_string_once(self):
        self.dev.begin_push(152, "x", self.plan)
        c, fn = self.cap.col, self.cap.capture.file_name
        self.refused(self.string_write, c, 5, fn + "x")          # another string
        self.refused(self.string_write, c - 1, 5, fn)            # another column
        self.refused(self.string_write, c, 4, fn)                # another index
        self.string_write(c, 5, fn)                              # the armed one passes
        self.refused(self.string_write, c, 5, fn)                # but only once

    def test_the_other_capture_block_type_is_refused(self):
        self.dev.begin_push(152, "x", self.plan)
        self.refused(self.qc.add_block, 14001, 0, self.cap.col)

    def test_place_capture_needs_the_armed_capture(self):
        self.dev.begin_push(152, "x", self.plan)
        with self.assertRaisesRegex(DeviceError, "not the armed one"):
            self.dev.place_capture(0, self.cap.col, 14000, "ab" * 32, self.cap.capture.name)
        with self.assertRaisesRegex(DeviceError, "not a capture block"):
            self.dev.place_capture(0, self.cap.col, 1001, self.cap.capture.key, self.cap.capture.name)

    def test_an_unresolved_capture_cannot_be_armed(self):
        blocks = [self.replace(b, model_id=0) if b.capture else b for b in self.plan.blocks]
        with self.assertRaisesRegex(DeviceError, "not resolved"):
            self.dev.begin_push(152, "x", self.replace(self.plan, blocks=blocks))


class MethodAllowlist(unittest.TestCase):
    def test_set_capture_is_granted_typed_writes_are_not(self):
        self.assertIn("set_capture", qcmcp_backend.GRANT)
        for name in ("set_param_typed", "set_ir", "list_directory"):
            self.assertNotIn(name, qcmcp_backend.GRANT, name)

    def test_vendored_capture_constants_are_pinned(self):
        QC = fake_hid.M["transport"].QuadCortex
        self.assertEqual((QC.CAPTURE_V1, QC.CAPTURE_V2, QC.CAPTURE_FILE_NAME_PARAM), (14000, 14001, 5))
        with mock.patch.object(QC, "CAPTURE_V2", 14002):
            with self.assertRaisesRegex(DeviceError, "capture constants"):
                QcMcpBackend()

    def test_never_methods_raise(self):
        g = Guarded(object())
        for name in ("save_preset", "recall", "clear_grid", "send", "request", "delete_block", "set_mode",
                     "write_global_eq", "set_io_port", "load_settings_preset", "delete_model_preset",
                     "set_scene_label", "set_preset_meta", "set_param_typed", "set_ir"):
            with self.subTest(name), self.assertRaisesRegex(GuardError, "is never allowed"):
                getattr(g, name)

    def test_grant_cannot_include_never(self):
        with self.assertRaisesRegex(GuardError, "grant includes never-allowed methods"):
            Guarded(object(), grant={"open", "save_preset"})

    def test_unlisted_methods_raise(self):
        # Real vendored methods on neither list (load_model_preset, used here before, is on the
        # never-list, so this test never reached the "not granted" branch).
        for name in ("read_state", "read_global_eq", "assign_param_to_scenes", "detect_version"):
            with self.subTest(name), self.assertRaisesRegex(GuardError, "is not granted"):
                getattr(Guarded(object()), name)

    def test_read_only(self):
        with self.assertRaisesRegex(GuardError, "read-only"):
            Guarded(object()).x = 1


class VendoredLoad(unittest.TestCase):
    def test_modified_vendored_file_is_refused(self):
        with tempfile.TemporaryDirectory() as d:
            copy_dir = Path(d) / "vendor"
            shutil.copytree(VENDOR_DIR, copy_dir)
            p = copy_dir / "qc_mcp" / "transport.py"
            p.write_bytes(p.read_bytes() + b"\n# local change\n")
            # The pin check refuses before sys.modules is touched; sandbox it anyway.
            with mock.patch.object(qcmcp_backend, "_loaded", None), mock.patch.dict(sys.modules):
                with self.assertRaisesRegex(DeviceError, "differs from the reviewed f634ecd copy"):
                    qcmcp_backend.load_vendored(copy_dir)

    def test_injected_modules_must_be_the_verified_ones(self):
        mods = qcmcp_backend.load_vendored()
        self.assertIs(QcMcpBackend(mods=mods)._mods, mods)
        with mock.patch.object(qcmcp_backend, "load_vendored", return_value=mods) as lv:
            QcMcpBackend(mods={"transport": object()})  # not what load_vendored returned: ignored
            lv.assert_called_once()

    def test_imports_are_the_vendored_copy(self):
        mods = qcmcp_backend.load_vendored()
        for m in mods.values():
            self.assertTrue(Path(m.__file__).resolve().is_relative_to(VENDOR_DIR.resolve()))
        self.assertNotIn(str(VENDOR_DIR.resolve()), sys.path)
        self.assertEqual(sys.modules["qc_mcp"].__path__, [])


if __name__ == "__main__":
    unittest.main()
