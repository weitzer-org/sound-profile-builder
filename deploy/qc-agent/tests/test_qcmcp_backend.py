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
from qc_agent import captures, jobs, qcmcp_backend
from qc_agent.device import DeviceError, UnitAbsent
from qc_agent.export_validator import validate
from qc_agent.ledger import OwnedSlots
from qc_agent.qcmcp_backend import GuardError, Guarded, QcMcpBackend, WireRefused
from qc_agent.vendor_pins import VENDOR_DIR
import fake_hid
from edit_fixtures import GAIN_EXPECT, GAIN_WIRE, f32

def setUpModule():
    # Pacing off for this module only; restored afterwards so other modules see the real values.
    from unittest import mock
    for name in ("SAVE_LIST_INTERVAL_S", "PACE_S"):
        p = mock.patch.object(jobs, name, 0)
        p.start()
        unittest.addModuleCleanup(p.stop)
    p = mock.patch.object(QcMcpBackend, "STOMP_SETTLE_S", 0)
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

    def test_short_table_is_logged_bounded_and_refused_as_partial(self):
        f = P.message_class("File")(action=P.ACTION["UPDATE"], type=0)
        f.folder.key, f.folder.name = fake_hid.SETLIST, "My Presets"
        for i in range(8):
            e = f.folder.files.add()
            e.index, e.name = i + 100, "Texas\nBlues`" + "x" * 80
        with self.assertLogs(qcmcp_backend.log, "WARNING") as cm:
            with self.assertRaises(qcmcp_backend.PartialListing) as ctx:
                qcmcp_backend.directory_snapshot([f])
        self.assertEqual(ctx.exception.count, 8)
        out = "\n".join(cm.output)
        self.assertIn("8 entries, not 256", out)
        self.assertEqual(out.count("index="), 5)  # bounded
        self.assertNotIn("\n", cm.records[0].getMessage())
        self.assertNotIn("`", out)

    def test_other_complete_collections_are_listed_incomplete_ones_are_left_out(self):
        from qc_agent.qcmcp_backend import directory_snapshot
        self.dev.connect()

        def folder(key, name, n, names=None, factory=False):
            f = P.message_class("File")(action=P.ACTION["UPDATE"], type=0)
            f.folder.key, f.folder.name, f.folder.is_factory = key, name, factory
            for i in range(n):
                f.folder.files.add().name = (names or {}).get(i, "")
            return f

        sounds, odd, fact = "/media/p4/Presets/Sounds profiles", "/media/p4/Presets/Short", "/opt/factory"
        snap = directory_snapshot([
            folder(fake_hid.SETLIST, "My Presets", 256),
            folder(sounds, "Sounds profiles", 256, {24: "Chess"}),
            folder(odd, "Short", 100),
            folder(fact, "Factory", 256, factory=True),
        ])
        self.assertEqual(set(snap.preset_folders), {fake_hid.SETLIST, sounds})  # not the short or the factory one
        self.assertEqual(snap.for_folder(sounds).my_presets[24], "Chess")
        self.assertEqual(snap.for_folder(sounds).my_presets[25], "")
        self.assertIsNone(snap.for_folder(odd))
        self.assertIsNone(snap.for_folder(fact))
        self.assertEqual(snap.my_presets, snap.for_folder(fake_hid.SETLIST).my_presets)  # My Presets still the main table
        with self.assertRaisesRegex(DeviceError, "My Presets is missing"):
            directory_snapshot([folder(sounds, "Sounds profiles", 256)])

    def test_dead_reader_thread_means_gone(self):
        self.dev.connect()
        self.hid._thread.running = False
        self.assertFalse(self.dev._watchdog_sample())
        self.assertFalse(self.dev.alive())

    def test_rising_write_errors_mean_gone(self):
        self.dev.connect()
        self.dev._errs = []
        for n in (1, 2):
            self.dev._raw.write_errors = n
            self.assertTrue(self.dev._watchdog_sample())
            self.assertTrue(self.dev.alive())
        self.dev._raw.write_errors = 3  # rose on two consecutive samples
        self.assertFalse(self.dev._watchdog_sample())
        self.assertFalse(self.dev.alive())

    def test_one_rise_is_not_gone(self):
        self.dev.connect()
        self.dev._errs = []
        for n in (1, 1, 2, 2, 3):  # rises, but never on two consecutive samples
            self.dev._raw.write_errors = n
            self.assertTrue(self.dev._watchdog_sample())
        self.assertTrue(self.dev.alive())

    def test_gone_is_sticky_for_the_session(self):
        self.dev.connect()
        self.hid._thread.running = False
        self.dev._watchdog_sample()
        self.hid._thread.running = True  # a later healthy sample doesn't bring the unit back
        self.dev._watchdog_sample()
        self.assertFalse(self.dev.alive())

    def test_the_watchdog_thread_samples_by_itself(self):
        with mock.patch.object(QcMcpBackend, "WATCH_INTERVAL_S", 0.02):
            self.dev.connect()
            t = self.dev._watch_thread
            self.assertTrue(t is not None and t.is_alive() and t.daemon)
            self.hid._thread.running = False  # nobody calls alive() or the transport here
            self.assertTrue(self.dev._gone.wait(5.0), "the watchdog didn't notice the dead reader thread")
            self.assertFalse(self.dev.alive())
        self.dev.disconnect()
        self.assertIsNone(self.dev._watch_thread)
        self.assertFalse(t.is_alive())

    def test_the_vendored_transport_still_has_the_liveness_attributes(self):
        # The watchdog reads two attributes the vendored transport has no public API for (G2 c; upstream has
        # no is_alive). A re-vendoring that renames them would make getattr() quietly skip the reader-thread
        # check, so pin them here: such a re-vendoring fails this test instead.
        from qc_agent.vendor_pins import verified_bytes
        win = verified_bytes("qc_mcp/winhid.py", VENDOR_DIR).decode()
        tr = verified_bytes("qc_mcp/transport.py", VENDOR_DIR).decode()
        self.assertIn("self._thread = threading.Thread(target=self._reader", win)
        self.assertIn("self.write_errors += 1", tr)
        # the real vendored classes, not the fakes (WinHIDTransport builds without opening hid.dll)
        mods = qcmcp_backend.load_vendored()
        self.assertIn("_thread", vars(mods["winhid"].WinHIDTransport()))
        self.assertEqual(vars(mods["transport"].QuadCortex(io=fake_hid.FakeHID()))["write_errors"], 0)

    def test_a_missing_reader_thread_fails_closed(self):
        # a re-vendoring that renamed _thread (or a transport closed under us) must read as gone, not alive
        self.dev.connect()
        del self.hid._thread
        self.assertFalse(self.dev._watchdog_sample())
        self.assertFalse(self.dev.alive())

    def test_a_stale_verdict_counts_as_gone(self):
        import time
        self.dev.connect()
        self.assertTrue(self.dev.alive())
        self.dev._last_sample = time.monotonic() - QcMcpBackend.WATCH_STALE_S - 1  # the watchdog stalled
        self.assertFalse(self.dev.alive())

    def test_a_broken_sample_is_gone(self):
        with mock.patch.object(QcMcpBackend, "WATCH_INTERVAL_S", 0.02):
            self.dev.connect()
            with mock.patch.object(self.dev, "_watchdog_sample", side_effect=RuntimeError("boom")):
                self.assertTrue(self.dev._gone.wait(5.0), "a broken sample must count as gone")
            self.assertFalse(self.dev.alive())

    def test_the_watchdog_never_waits_for_the_transport(self):
        import threading
        self.dev.connect()
        done = threading.Event()
        with self.dev._raw._io_lock:  # a transport call is blocked, holding the lock (G2: seconds)
            t = threading.Thread(target=lambda: (self.dev._watchdog_sample(), self.dev.alive(), done.set()),
                                 daemon=True)
            t.start()
            finished = done.wait(2.0)
        t.join(2.0)  # the lock is released and the thread joined before anything can fail
        self.assertTrue(finished, "a liveness sample must not take the transport's lock")

    def test_reconnect_starts_with_a_clean_verdict(self):
        self.dev.connect()
        self.hid._thread.running = False
        self.dev._watchdog_sample()
        self.dev.disconnect()
        self.hid._thread.running = True
        self.dev.connect()
        self.assertTrue(self.dev.alive())

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
        # contract section 9: the push result carries exactly these fields, and diff entries exactly these
        self.assertEqual(set(out.push), {"slot", "name", "mismatches", "cpu_load", "diff"})
        for d in out.push["diff"]:
            self.assertEqual(set(d), {"row", "col", "param_index", "expected", "readback", "ok"})
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
        # all eight bypass flags on every block, through its bypass control at param_count + 1 (the
        # simulated unit marks a block's bypass scene-assigned only for an index past its own read)
        cat = catalog()
        grids = [o for n, o in self.hid.received if n == "Grid" and o.action == A["UPDATE"]]
        for col, b in self.hid.grid.items():
            self.assertTrue(b["bypass_scene"], col)
            pc = cat.models[b["hash"]]["param_count"]
            assigns = {p.index for g in grids for ch in g.preset.chains for mm in ch.models if mm.column == col
                       for p in mm.params if p.scene_mode and not p.param_values}
            self.assertIn(pc + 1, assigns, col)
            written = {p.index for g in grids for ch in g.preset.chains for mm in ch.models if mm.column == col
                       for p in mm.params if p.param_values}
            self.assertTrue(all(i < pc for i in written), (col, written, pc))  # never the trailing slot


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

    def test_scene_assign_only_at_the_bypass_control(self):
        self.dev.begin_push(152, "x", self.plan)  # the Spring (column 4): param_count 5, the unit reads 6

        def assign(idx, col=4):
            def build(g):
                m = g.preset.chains.add().models.add()
                m.column = col
                p = m.params.add()
                p.index, p.scene_mode = idx, True
            return build
        self.raw_grid(assign(1))       # DAMPING: a real parameter, not scene-varying in the plan
        self.raw_grid(assign(5))       # the trailing slot
        self.raw_grid(assign(7))
        self.raw_grid(assign(6))       # the bypass control, but no read of the unit has confirmed it yet
        with self.assertRaisesRegex(WireRefused, "reads 7 params"):
            self.dev.confirm_layout(4, 7)  # a block with more params than the catalog: 6 would be a control
        self.raw_grid(assign(6))
        for col, idx in ((4, 6), (0, 4), (2, 22)):  # once confirmed: param_count + 1 on every block
            with self.subTest(col=col, idx=idx):
                self.dev.confirm_layout(col, idx)
                g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
                assign(idx, col)(g)
                self.qc.send("Grid", g)
        with self.assertRaisesRegex(WireRefused, "no armed block in column 6"):
            self.dev.confirm_layout(6, 1)

    def test_no_value_at_or_past_param_count(self):
        self.dev.begin_push(152, "x", self.plan)
        push = self.dev._guard._push
        # even if the plan somehow held it, the trailing slot (param_count 3 on column 0) is never written
        push["params"].add((0, 3))
        push["values"][(0, 3)] = {0.5}
        with self.assertRaisesRegex(WireRefused, "at or past param_count"):
            self.qc.set_param(0, 0, 3, 0.5)
        push["strings"][(0, 3)] = "x"
        with self.assertRaisesRegex(WireRefused, "at or past param_count"):
            self.raw_string(0, 3, "x")

    def raw_string(self, col, idx, value):
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        m = g.preset.chains.add().models.add()
        m.column = col
        p = m.params.add()
        p.index = idx
        p.param_values.add().string_value = value
        self.qc.send("Grid", g)

    def test_backend_bypass_index_must_be_the_armed_one(self):
        self.dev.begin_push(152, "x", self.plan)
        with self.assertRaisesRegex(DeviceError, "not the armed one"):
            self.dev.set_block_bypass(0, 4, [False] * 8, 6)  # right index, but not confirmed from a read
        self.dev.confirm_layout(4, 6)
        with self.assertRaisesRegex(DeviceError, "not the armed one"):
            self.dev.set_block_bypass(0, 4, [False] * 8, 5)
        self.dev.end_push()
        with self.assertRaisesRegex(DeviceError, "not armed"):
            self.dev.set_block_bypass(0, 4, [False] * 8, 6)

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


class EditGuard(Base):
    """The wire guard of an armed edit (preset-editing E4): only the plan's (column, index) values, scene
    assignments and bypass flags, in two phases, and nothing else that exists on the wire."""

    def setUp(self):
        super().setUp()
        from edit_fixtures import base_plan
        from qc_agent import edit_checks
        self.GAIN_WIRE = GAIN_WIRE
        self.dev.connect()
        self.qc = self.dev._raw
        self.plan = base_plan()
        self.checked = edit_checks.catalog_check(self.plan, catalog(), "4.1.0")
        for col, mid in ((0, 6), (1, 1011)):
            self.hid.grid[col] = {"hash": mid, "params": {}, "scene": set(), "bypass": [False] * 8, "bypass_scene": False}
        self.hid.grid[0]["params"][0] = list(GAIN_EXPECT)
        self.hid.grid[1]["params"][1] = [0.0] * 8

    def arm(self, confirm=True):
        self.dev.begin_edit(self.plan, self.checked)
        if confirm:
            self.dev.confirm_layout(0, 5)   # Exotic: param_count 4, the unit reads 5
            self.dev.confirm_layout(1, 7)   # UK C30 TopBoost: param_count 6, the unit reads 7

    def refused(self, fn, *a, **kw):
        n = len(self.hid.received)
        with self.assertRaises((WireRefused, GuardError, DeviceError)):
            fn(*a, **kw)
        self.assertEqual(len(self.hid.received), n, "nothing reached the device")

    def raw(self, build, extra=b""):
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        build(g)
        full = P.encode_message("Grid", g.SerializeToString() + extra)
        n = len(self.hid.received)
        with self.assertRaises(WireRefused):
            for r in P.message_to_reports(full):
                self.dev._guard.set_report(P.REPORT_HOST_TO_QC, r[1:], include_id=True)
        self.assertEqual(len(self.hid.received), n)

    def test_nothing_passes_unarmed(self):
        self.refused(self.qc.set_param, 0, 0, 0, 0.5)
        self.refused(self.qc.set_scene, 1)

    def test_the_plans_writes_pass_and_land(self):
        self.arm()
        self.qc.set_param(0, 1, 1, 1.0)                              # BOOST, a plain write
        self.qc.set_param_scenes(0, 0, 0, self.GAIN_WIRE)           # GAIN: assign to scenes, then each scene's value
        self.qc.set_block_bypass(0, 1, scenes=[False, True] + [False] * 6, bypass_param=7)
        g0, g1 = self.hid.grid[0], self.hid.grid[1]
        self.assertEqual(g0["params"][0], [f32(x) for x in self.GAIN_WIRE])   # float32 on the wire
        self.assertEqual(g1["params"][1], [1.0] * 8)
        self.assertEqual(g1["bypass"][:2], [False, True])
        self.assertEqual(self.dev._guard.refused, [])

    def test_off_plan_writes_are_refused(self):
        self.arm()
        self.refused(self.qc.set_param, 0, 1, 1, 0.7)                # a value the plan doesn't carry
        self.refused(self.qc.set_param, 0, 1, 0, 1.0)                # index 0 of column 1 is not in the plan
        self.refused(self.qc.set_param, 0, 0, 1, 0.5)                # nor index 1 of column 0
        self.refused(self.qc.set_param, 0, 2, 0, 0.5)                # a column the plan doesn't name
        self.refused(self.qc.set_param, 1, 0, 0, 0.5)                # row 1
        self.refused(self.qc.set_param, 0, 0, 4, 0.5)                # the trailing slot of the Exotic
        self.refused(self.qc.set_param, 0, 1, 1, 1.5)                # outside 0..1
        self.refused(self.qc.set_param, 0, 0, 0, 0.9)                # GAIN: not one of the plan's scene values
        self.qc.set_param(0, 1, 1, 1.0)

    def test_nothing_that_places_deletes_or_saves_passes(self):
        self.arm()
        self.refused(self.qc.add_block, 6, row=0, column=0)
        self.refused(self.qc.add_block, 6, row=0, column=5)
        self.refused(self.qc.delete_block, 0, 0)
        self.refused(self.qc.set_routing, 0, in_portid=1, out_portid=19)
        self.refused(self.qc.set_param_typed, 0, 0, 0, [[None, None, "x"]])
        self.refused(self.qc.set_capture, 0, 3, "a" * 64, "x")
        self.refused(self.qc.set_ir, 0, 3, "CIR_x")
        self.refused(self.qc.recall, fake_hid.SETLIST, 3)
        self.refused(self.qc.save_preset)
        self.refused(self.qc.write_preset_file, fake_hid.SETLIST, 152, "Texas Blues")
        self.refused(self.qc.set_mode, 1)
        self.refused(self.qc.set_scene_label, 0, "x")
        self.refused(self.qc.assign_stomp, 0, 0, 1)
        f = P.message_class("File")(action=P.ACTION["DELETE"])
        f.folder.key = fake_hid.SETLIST
        self.refused(self.qc.send, "File", f)

    def test_the_backend_cannot_arm_a_save_during_an_edit(self):
        self.arm()
        with self.assertRaisesRegex(DeviceError, "save target differs"):
            self.dev.save_to_slot(152, "Texas Blues")
        self.assertEqual(self.dev._guard._save, None)

    def test_raw_messages_with_fields_an_edit_never_sends(self):
        self.arm()

        def chain(g):
            return g.preset.chains.add()

        def one(g, col=1, idx=1, value=1.0):
            ch = chain(g)
            m = ch.models.add()
            m.column = col
            p = m.params.add()
            p.index = idx
            p.param_values.add().float_value = value
            return ch, m, p
        self.raw(lambda g: setattr(one(g)[0], "in_portid", 3))                   # routing
        self.raw(lambda g: setattr(one(g)[1], "hash", 1011))                     # places a model
        self.raw(lambda g: one(g)[1].params.add())                               # two params in one message
        self.raw(lambda g: (one(g), chain(g)))                                   # two chains
        self.raw(lambda g: chain(g).models.add())                                # the shape of a delete
        self.raw(lambda g: chain(g))                                             # an empty chain
        self.raw(lambda g: setattr(g.preset, "name", "Hacked"))                  # a preset field
        self.raw(lambda g: g.preset.stomp_mode_assignments.add())                # a stomp
        self.raw(lambda g: one(g), extra=bytes([0xF8, 0xFF, 0x03, 0x01]))        # an unknown field

        def string(g):
            _, m, p = one(g, idx=1)
            p.param_values[0].string_value = "x"
        self.raw(string)                                                          # a string where a float belongs

        def two_values(g):
            one(g)[2].param_values.add().float_value = 1.0
        self.raw(two_values)

    def test_scene_assignment_only_where_the_plan_has_it(self):
        self.arm()

        def assign(col, idx, on=True):
            def build(g):
                m = g.preset.chains.add().models.add()
                m.column = col
                p = m.params.add()
                p.index, p.scene_mode = idx, on
            return build
        self.raw(assign(1, 1))                  # BOOST: the plan writes it plainly, not per scene
        self.raw(assign(0, 1))                  # a control the plan doesn't name
        self.raw(assign(0, 0, on=False))        # taking GAIN out of scenes
        self.raw(assign(0, 4))                  # the trailing slot / bypass slot of the Exotic (no bypass op there)
        self.raw(assign(1, 6))                  # the TopBoost's trailing slot is not its bypass control (7 is)
        self.dev._guard._edit["confirmed"].pop(1)
        self.raw(assign(1, 7))                  # the bypass control, but no read confirmed the layout
        self.dev.confirm_layout(1, 7)
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        assign(1, 7)(g)
        self.qc.send("Grid", g)                 # confirmed: passes
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        assign(0, 0)(g)
        self.qc.send("Grid", g)                 # GAIN is scene-varying in the plan: passes

    def test_a_layout_the_unit_does_not_confirm_blocks_the_column(self):
        self.arm(confirm=False)
        self.refused(self.qc.set_param, 0, 1, 1, 1.0)           # not confirmed yet
        with self.assertRaisesRegex(WireRefused, "reads 8 params"):
            self.dev.confirm_layout(1, 8)                      # a block with more params than the catalog
        self.refused(self.qc.set_param, 0, 1, 1, 1.0)
        with self.assertRaisesRegex(WireRefused, "no armed block in column 5"):
            self.dev.confirm_layout(5, 7)
        self.dev.confirm_layout(1, 7)
        self.qc.set_param(0, 1, 1, 1.0)

    def test_bypass_writes_need_a_planned_cell_and_a_confirmed_index(self):
        self.arm(confirm=False)
        with self.assertRaisesRegex(DeviceError, "not the armed one"):
            self.dev.set_block_bypass(0, 1, [False] * 8, 7)    # right index, nothing confirmed it
        self.dev.confirm_layout(1, 7)
        with self.assertRaisesRegex(DeviceError, "not the armed one"):
            self.dev.set_block_bypass(0, 1, [False] * 8, 6)    # the wrong index
        with self.assertRaisesRegex(DeviceError, "not the armed one"):
            self.dev.set_block_bypass(0, 0, [False] * 8, 5)    # a column with no bypass op
        n = len(self.hid.received)
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        cb = g.preset.bypass.add().colBypass.add()
        cb.column = 0                                          # a flag on a cell the plan doesn't bypass
        cb.sceneBypass.add().bypass = True
        with self.assertRaises(WireRefused):
            self.qc.send("Grid", g)
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        b = g.preset.bypass.add()
        b.row = 1                                              # another row
        cb = b.colBypass.add()
        cb.column = 1
        cb.sceneBypass.add().bypass = True
        with self.assertRaises(WireRefused):
            self.qc.send("Grid", g)
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        cb = g.preset.bypass.add().colBypass.add()
        cb.column = 1
        cb.sceneBypass.add().bypass = True                     # two flags in one message
        cb.sceneBypass.add().bypass = False
        with self.assertRaises(WireRefused):
            self.qc.send("Grid", g)
        self.assertEqual(len(self.hid.received), n)

    def test_rollback_allows_only_old_values_of_ops_that_were_started(self):
        self.arm()
        self.refused(self.qc.set_param, 0, 1, 1, 0.0)           # BOOST's old value, but apply phase: the new one is 1.0
        self.dev.enter_rollback()
        self.refused(self.qc.set_param, 0, 1, 1, 0.0)           # old value, but its write was never started
        self.dev.end_push()
        self.arm()
        self.qc.set_param(0, 1, 1, 1.0)                         # started
        self.dev.enter_rollback()
        self.refused(self.qc.set_param, 0, 1, 1, 1.0)           # the new value is not allowed any more
        self.qc.set_param(0, 1, 1, 0.0)                         # the old one is
        self.refused(self.qc.set_param_scenes, 0, 0, 0, list(GAIN_EXPECT))  # GAIN was never started

    def test_rollback_restores_a_scene_varying_control_and_a_bypass(self):
        self.arm()
        self.qc.set_param_scenes(0, 0, 0, self.GAIN_WIRE)
        self.qc.set_block_bypass(0, 1, scenes=[False, True] + [False] * 6, bypass_param=7)
        self.dev.enter_rollback()
        self.qc.set_param_scenes(0, 0, 0, list(GAIN_EXPECT))
        self.qc.set_block_bypass(0, 1, scenes=[False] * 8, bypass_param=7)
        self.assertEqual(self.hid.grid[0]["params"][0], list(GAIN_EXPECT))
        self.assertEqual(self.hid.grid[1]["bypass"], [False] * 8)

    def test_end_push_disarms_an_edit(self):
        self.arm()
        self.dev.end_push()
        self.refused(self.qc.set_param, 0, 1, 1, 1.0)

    def test_an_edit_cannot_be_armed_twice_or_over_a_push(self):
        self.arm()
        with self.assertRaisesRegex(WireRefused, "on its own, once"):
            self.dev.begin_edit(self.plan, self.checked)
        self.dev.end_push()
        self.dev.begin_push(152, "x", validate(export(), catalog(), "4.1.0"))
        with self.assertRaisesRegex(WireRefused, "on its own, once"):
            self.dev.begin_edit(self.plan, self.checked)

    def test_a_push_arms_nothing_of_an_edit(self):
        self.dev.begin_push(152, "x", validate(export(), catalog(), "4.1.0"))
        self.assertIsNone(self.dev._guard._edit)
        with self.assertRaisesRegex(WireRefused, "no armed edit"):
            self.dev.enter_rollback()


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
        # same G1 rules as any block: catalog param_count 6 for 14000, bypass control at 7, no write at or past 6
        self.assertTrue(self.hid.grid[3]["bypass_scene"])
        idx3 = {p.index for g in grids for ch in g.preset.chains for mm in ch.models if mm.column == 3 for p in mm.params}
        self.assertEqual(max(i for i in idx3 if i != 7), 5)
        self.assertIn(7, idx3)
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


class DirectoryListing(Base):
    def folder(self, typ, key, name, parent="", cloud=False, fav=False, plugin=False, files=(), factory=False,
               version="1"):
        f = P.message_class("File")(action=P.ACTION["UPDATE"], type=typ)
        f.folder.key, f.folder.name, f.folder.parent_key = key, name, parent
        f.folder.is_cloud, f.folder.is_favorites, f.folder.is_plugin = cloud, fav, plugin
        f.folder.is_factory = factory
        for n, k in files:
            fi = f.folder.files.add()
            fi.name, fi.key, fi.version = n, k, version
        return f

    def test_factory_version_from_the_folder_tree(self):
        # shaped like the owner's unit (P6): per-device factory folders, empty version field; whether V1/V2
        # parents exist there is unverified, so each case below is a guess at the tree, not a reading of it
        msgs = [self.my_presets(),
                self.folder(2, "v1", "Factory Captures V1", factory=True),
                self.folder(2, "v2", "Factory Captures V2", factory=True),
                self.folder(2, "b", "Brit 2210", parent="v2", factory=True, version="", files=[("Brit A", "a1" * 32)]),
                self.folder(2, "amp", "Amp", parent="v1", factory=True),
                self.folder(2, "c", "CA 3Axe", parent="amp", factory=True, version="", files=[("CA B", "a2" * 32)]),
                self.folder(2, "lost", "Aggi 700", parent="gone", factory=True, version="",
                            files=[("Aggi C", "a3" * 32)]),
                self.folder(2, "top", "ABA MPre1", factory=True, version="", files=[("ABA D", "a4" * 32)]),
                self.folder(2, "u", "", files=[("Mine", "a5" * 32)], version="2")]
        snap = qcmcp_backend.directory_snapshot(msgs)
        got = {e.name: captures.version_of(e) for e in snap.captures}
        self.assertEqual(got, {"Brit A": 2, "CA B": 1, "Aggi C": None, "ABA D": None, "Mine": 2})
        rows = {r["name"]: r for r in snap.folder_census}
        self.assertEqual(rows["CA 3Axe"]["path"], "Factory Captures V1 > Amp")
        self.assertEqual(rows["Aggi 700"]["path"], "<parent not listed>")
        self.assertEqual(rows["ABA MPre1"]["path"], "-")
        self.assertEqual((rows["Brit 2210"]["v"], rows["Brit 2210"]["files"], rows["Brit 2210"]["version"]), (2, 1, [""]))

    def test_skip_census_over_a_tree_shaped_like_the_owners(self):
        # P6 census shape: Captures Library (not factory) holds everything below it; Factory V1/V2, My Captures
        # and a top-level '' folder with downloads. Which reason actually hits the factory files is the open
        # question, so each reason gets one entry here and the log must count them per folder and bound samples.
        good = "ab" * 32
        msgs = [self.my_presets(),
                self.folder(2, "lib", "Captures Library", version="1", files=[("Lib A", good)]),
                self.folder(2, "v1", "Factory Captures V1", parent="lib", factory=True, version="",
                            files=[("Fac \u00e9", good), ("Fac B`", good), ("Fac C", "xyz")]),
                self.folder(2, "amp", "Amp", parent="v1", factory=True, version="",
                            files=[("x" * 70, good), ("", good), ("Fac D", good)]),
                self.folder(2, "v2", "Factory Captures V2", parent="lib", factory=True, version=""),
                self.folder(2, "mine", "My Captures", parent="lib", version="", files=[("Mine", good)]),
                self.folder(2, "fav", "Favs", fav=True, files=[("Lib A", good)]),
                self.folder(2, "dl", "", version="2", files=[("Down A", good)])]
        snap = qcmcp_backend.directory_snapshot(msgs)
        lines = captures.skip_census(snap)
        out = "\n".join(lines)
        self.assertIn("folder 'Factory Captures V1': name_charset 1, name_cleaned 1, key_format 1", out)
        self.assertIn("folder 'Amp': kept 1, name_length 2", out)
        self.assertIn("folder 'Favs': favorites 1", out)
        self.assertIn("folder '(unnamed folder)': kept 1", out)
        self.assertIn("totals kept 4, favorites 1, name_length 2, name_charset 1, name_cleaned 1, key_format 1", out)
        for r in captures.SKIP_REASONS:
            self.assertEqual(out.count(f"capture skip sample ({r})"), 1)
        # samples show lengths and classes only: no key or name text
        self.assertNotIn(good, out)
        self.assertIn("non_ascii=1", out)
        self.assertIn("backtick=1", out)
        # the payload and the census agree on what is kept
        self.assertEqual(len(captures.library_payload(snap)["captures"]), 4)
        self.assertEqual(captures.library_payload(snap)["skipped"], 6)

    def test_census_logs_once_per_process(self):
        msgs = [self.my_presets(),
                self.folder(2, "b", "Brit 2210", factory=True, version="", files=[("Brit A", "a1" * 32)]),
                self.folder(2, "u", "", files=[("Mine", "a5" * 32)], version="2")]
        snap = qcmcp_backend.directory_snapshot(msgs)
        with mock.patch.object(qcmcp_backend, "_CENSUS_LOGGED", False):
            with self.assertLogs(qcmcp_backend.log, "INFO") as cm:
                qcmcp_backend._log_census(snap)
                qcmcp_backend._log_census(snap)
        out = "\n".join(cm.output)
        self.assertEqual(out.count("capture census: 2 folders"), 1)
        self.assertIn("capture folder 'Brit 2210' in -: factory=True", out)
        self.assertIn("-> no V1/V2 folder", out)
        self.assertIn("factory V1 0, V2 0, unknown 1; user V1 0, V2 1, unknown 0", out)

    def my_presets(self):
        f = P.message_class("File")(action=P.ACTION["UPDATE"], type=0)
        f.folder.key, f.folder.name = fake_hid.SETLIST, "My Presets"
        for _ in range(256):
            f.folder.files.add().name = ""
        return f

    def test_flags_are_inherited_from_parent_folders(self):
        msgs = [self.my_presets(),
                self.folder(2, "cloud", "Cortex Cloud", cloud=True),
                self.folder(2, "sub", "Downloads 1", parent="cloud", files=[("Amp", "ab" * 32)]),
                self.folder(2, "favs", "Favorites", fav=True),
                self.folder(2, "favsub", "Fav amps", parent="favs", files=[("Amp", "cd" * 32)]),
                self.folder(2, "plug", "Plugin", plugin=True, files=[("Amp", "ef" * 32)]),
                self.folder(2, "loop", "Loop", parent="loop", files=[("Amp", "aa" * 32)]),
                self.folder(2, "mine", "My Captures", files=[("Amp", "bb" * 32)])]
        snap = qcmcp_backend.directory_snapshot(msgs)
        flags = {e.folder_key: (e.is_cloud, e.is_favorites) for e in snap.captures}
        self.assertEqual(flags, {"sub": (True, False), "favsub": (False, True), "plug": (True, False),
                                 "loop": (True, False), "mine": (False, False)})

    def test_slot_index_must_match_position(self):
        msgs = [self.my_presets()]
        msgs[0].folder.files[5].index = 5  # agrees with its position: fine
        self.assertEqual(len(qcmcp_backend.directory_snapshot(msgs).my_presets), 256)
        msgs[0].folder.files[7].index = 9
        with self.assertRaisesRegex(qcmcp_backend.DeviceError, "disagrees with its position"):
            qcmcp_backend.directory_snapshot(msgs)

    def test_stale_queued_folders_are_dropped(self):
        self.dev.connect()
        stale = self.my_presets()
        stale.folder.files[3].name = "STALE"
        self.dev._raw._pending.append((P.NAME_TO_CMD["File"], stale, b"", b"xx"))
        snap = self.dev.read_directory()
        self.assertEqual(snap.my_presets[3], "")

    def test_the_reads_own_folderless_reply_is_consumed(self):
        self.dev.connect()
        raw = self.dev._raw
        orig = self.hid._handle
        File = P.message_class("File")

        def with_ack(name, m):
            out = orig(name, m)
            if name == "File" and m.action == P.ACTION["READ"]:
                self.hid._reply("File", File(action=P.ACTION["READ"], request_id=m.request_id))
                self.hid._reply("File", File(action=P.ACTION["READ"], request_id=m.request_id + 1000))
            return out
        self.hid._handle = with_ack
        self.dev.read_directory()
        left = [o for c, o, *_ in raw._pending if c == P.NAME_TO_CMD["File"]]
        # its own ack is gone; another request's reply is left for request() to judge
        self.assertEqual(len(left), 1)
        self.assertFalse(left[0].HasField("folder"))

    def test_unconsumed_queue_is_bounded(self):
        self.dev.connect()
        raw = self.dev._raw
        tempo = P.message_class("GlobalTempo")()
        raw._pending.extend((P.NAME_TO_CMD["GlobalTempo"], tempo, b"", b"xx")
                            for _ in range(qcmcp_backend.MAX_PENDING + 500))
        snap = self.dev.read_directory()  # the listing itself still completes
        self.assertEqual(len(snap.my_presets), 256)
        self.assertLessEqual(len(raw._pending), qcmcp_backend.MAX_PENDING)

    def test_an_unfinished_listing_is_refused(self):
        self.dev.connect()
        orig = self.hid._handle

        def silent(name, m):
            if name == "File" and m.action == P.ACTION["READ"]:
                return None
            return orig(name, m)
        self.hid._handle = silent
        with self.assertRaisesRegex(DeviceError, "did not finish"):
            self.dev._call("list_directory_raw", timeout_ms=300, quiet_ms=100)


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


class MicGuard(Base):
    """The typed write the cab mic needs (plan P4b): granted as a method, but the wire guard lets through
    only the planned string, at the planned column and the catalog's ir selector index, once."""

    def setUp(self):
        super().setUp()
        self.dev.connect()
        self.qc = self.dev._raw
        from helpers import export
        e = export()
        e["payload"]["rows"][0]["blocks"][2]["mics"] = [
            {"slot": 1, "index": 1, "value": "ME_110 US PRN C10R_Ribbon 121", "evidence": "seen-on-unit"}]
        self.plan = validate(e, catalog(), "4.1.0")
        self.value = "ME_110 US PRN C10R_Ribbon 121"

    def refused(self, fn, *a):
        n = len(self.hid.received)
        with self.assertRaises(WireRefused):
            fn(*a)
        self.assertEqual(len(self.hid.received), n, "nothing reached the device")

    def test_the_armed_mic_passes_once_and_nothing_else_does(self):
        self.dev.begin_push(152, "x", self.plan)
        t = lambda col, idx, v: self.qc.set_param_typed(0, col, idx, [[None, None, v]])  # noqa: E731
        self.refused(t, 2, 1, "ME_110 US PRN C10R_Ribbon 160")      # another string
        self.refused(t, 2, 1, "C:\\IRs\\evil.wav")                    # another IR
        self.refused(t, 1, 1, self.value)                           # another column
        self.refused(t, 2, 9, self.value)                           # the other mic slot, not planned
        self.refused(t, 2, 21, self.value)                          # past param_count
        self.refused(self.qc.set_param_typed, 0, 2, 1, [[None, None, self.value], [None, None, self.value]])
        self.refused(self.qc.set_param_typed, 0, 2, 1, [[0.5, None, None]])   # a float where a string is planned
        t(2, 1, self.value)                                         # the plan's own write passes
        self.refused(t, 2, 1, self.value)                           # but only once

    def test_set_mic_needs_the_armed_string(self):
        with self.assertRaisesRegex(DeviceError, "not the armed one"):
            self.dev.set_mic(0, 2, 1, self.value)                   # nothing armed
        self.dev.begin_push(152, "x", self.plan)
        with self.assertRaisesRegex(DeviceError, "not the armed one"):
            self.dev.set_mic(0, 2, 1, self.value + "x")
        with self.assertRaisesRegex(DeviceError, "not the armed one"):
            self.dev.set_mic(0, 3, 1, self.value)
        self.dev.set_mic(0, 2, 1, self.value)

    def test_an_unarmed_typed_write_is_refused(self):
        self.refused(self.qc.set_param_typed, 0, 2, 1, [[None, None, self.value]])


class MethodAllowlist(unittest.TestCase):
    def test_set_capture_and_typed_writes_are_granted_other_typed_ones_are_not(self):
        # set_param_typed is granted for the planned cab mic string only (P4b); the wire guard, not the
        # method list, holds it to that (MicGuard below)
        self.assertIn("set_capture", qcmcp_backend.GRANT)
        self.assertIn("set_param_typed", qcmcp_backend.GRANT)
        for name in ("set_ir", "list_directory"):
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
                     "set_scene_label", "set_preset_meta", "set_ir"):
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
