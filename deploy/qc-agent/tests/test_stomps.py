"""Footswitches on a push (stomp doc section 6.1, entry point 1; owner decision #147).

Two levels, like the rest of the suite:
* PushStomps: the job logic against fake_device.FakeDevice. The stomps are assigned only after the
  values read back clean, only through op() (the open slot re-read before each), and read back; any
  difference fails the job before the save.
* StompWire / StompWindow: the real vendored transport against fake_hid.FakeHID, through QcMcpBackend.
  assign_stomp stays on the NEVER list and opens for one call only in the push's stomp step; the wire
  guard passes only the armed PRIMARY assignments, each once, and never a momentary flag, a secondary
  assignment, a delete, H, or a block the push didn't place and confirm.

Proves the agent's rules offline. Which physical switch an index is, and that a switch toggles its
block, is UNVERIFIED until README step 15 has been run on the unit.
"""
import json
import unittest
from unittest import mock

import fake_hid
import test_agent
import test_qcmcp_backend
from helpers import catalog, config, export, job
from qc_agent import contract, jobs
from qc_agent.device import DeviceError
from qc_agent.export_validator import STOMP_INDEX, validate
from qc_agent.qcmcp_backend import NEVER, PUSH_STEP_ONLY, GuardError, Guarded, QcMcpBackend, WireRefused, preset_to_plain

P = fake_hid.P


def setUpModule():
    for name in ("SAVE_LIST_INTERVAL_S", "PACE_S"):
        p = mock.patch.object(jobs, name, 0)
        p.start()
        unittest.addModuleCleanup(p.stop)
    p = mock.patch.object(QcMcpBackend, "STOMP_SETTLE_S", 0)
    p.start()
    unittest.addModuleCleanup(p.stop)


# pushable_g3's layout: Myth Drive (col 0) on C, Spring (col 4) on G
PLANNED = [("assign_stomp", 0, 0, 2), ("assign_stomp", 0, 4, 6)]


class PushStomps(test_agent.Base):
    def push(self, export_obj=None):
        args = None
        if export_obj is not None:
            args = {"slot": "20A", "name": "Texas Blues", "source": {"preset_id": "p-123", "guitar": "Telecaster"},
                    "export": export_obj}
        jid, _ = self.queue("push_preset", args=args)
        self.poll()
        return self.result(jid)

    def names(self):
        return [c[0] for c in self.dev.calls]

    def assert_not_saved(self, r, code="readback_mismatch"):
        self.assertEqual((r["status"], r["reason_code"]), ("failed", code), r["message"])
        self.assertNotIn("save_to_slot", self.names())
        self.assertNotIn(152, self.dev.slots)

    def test_stomps_are_assigned_after_the_readback_and_before_the_save(self):
        r = self.push()
        self.assertEqual((r["status"], r["reason_code"]), ("succeeded", ""), r["message"])
        self.assertIn("2 footswitch(es) assigned and read back", r["message"])
        self.assertEqual([c for c in self.dev.calls if c[0] == "assign_stomp"], PLANNED)
        self.assertEqual(self.dev.stomps, {(0, 0): 2, (0, 4): 6})
        names = self.names()
        last_value = max(i for i, n in enumerate(names) if n in ("set_param", "set_param_scenes", "set_block_bypass"))
        verify_read = names.index("read_current_preset", last_value)
        arm = names.index("arm_stomps")
        assigns = [i for i, n in enumerate(names) if n == "assign_stomp"]
        self.assertLess(verify_read, arm)                     # the values read back first
        self.assertLess(arm, assigns[0])
        self.assertLess(assigns[-1], names.index("save_to_slot"))
        self.assertIn("read_current_preset", names[assigns[-1]:names.index("save_to_slot")])  # the stomp read-back
        for i in assigns:                                     # the open slot re-read before each, like every write
            self.assertEqual(names[i - 1], "setlist_position")
        self.assertNotIn(7, [c[3] for c in self.dev.calls if c[0] == "assign_stomp"])  # never H
        self.assertEqual(set(r["push"]), {"slot", "name", "mismatches", "cpu_load", "diff"})  # no contract change

    def test_no_stomps_means_no_stomp_step(self):
        e = export()
        e["payload"]["stomps"] = []
        self.dev.omit_stomps = True  # and the read-back isn't consulted for them
        r = self.push(e)
        self.assertEqual(r["status"], "succeeded", r["message"])
        self.assertNotIn("arm_stomps", self.names())
        self.assertNotIn("assign_stomp", self.names())
        self.assertNotIn("footswitch", r["message"])

    def test_a_value_mismatch_stops_before_any_stomp(self):
        self.dev.corrupt_readback = True
        r = self.push()
        self.assert_not_saved(r)
        self.assertNotIn("arm_stomps", self.names())
        self.assertNotIn("assign_stomp", self.names())

    def test_a_switch_that_reads_back_differently_fails_before_the_save(self):
        self.dev.stomp_index_skew = 1
        r = self.push()
        self.assert_not_saved(r)
        self.assertIn("footswitch assignments differ", r["message"])
        self.assertEqual(r["push"]["mismatches"], 4)  # two unexpected, two missing

    def test_an_extra_assignment_fails(self):
        self.dev.extra_stomps = [{"row": 0, "col": 1, "index": 7, "type": 0}]  # something on H
        self.assert_not_saved(self.push())

    def test_missing_assignments_fail(self):
        self.dev.omit_stomps = True
        r = self.push()
        self.assert_not_saved(r)
        self.assertIn("footswitch assignments differ", r["message"])

    def test_a_secondary_assignment_fails(self):
        self.dev.extra_stomps = [{"row": 0, "col": 0, "index": 2, "type": 1}]
        self.assert_not_saved(self.push())

    def test_a_momentary_switch_fails(self):
        self.dev.momentary = {6: True}
        r = self.push()
        self.assert_not_saved(r)
        self.assertIn("footswitch assignments differ", r["message"])

    def test_values_that_change_during_the_stomps_fail(self):
        def hook(name):
            if name == "assign_stomp":
                self.dev.grid[0]["params"][0] = [0.5] * 8
        self.dev.on_call = hook
        r = self.push()
        self.assert_not_saved(r)
        self.assertIn("values differ", r["message"])

    def test_another_preset_selected_during_the_stomps_stops_them(self):
        def hook(name):
            if name == "assign_stomp":
                self.dev.open_position = 0
        self.dev.on_call = hook
        r = self.push()
        self.assert_not_saved(r, code="device_error")
        self.assertEqual([c for c in self.dev.calls if c[0] == "assign_stomp"], PLANNED[:1])

    def test_stomps_that_change_before_the_save_fail(self):
        def hook(fake, n):
            if any(c[0] == "assign_stomp" for c in fake.calls):
                fake.stomps[(0, 0)] = 3  # moved to D on the unit while the listing ran
        self.dev.on_listing = hook
        r = self.push()
        self.assert_not_saved(r)
        self.assertIn("changed after it was verified", r["message"])

    def test_the_index_map_never_reaches_h(self):
        self.assertEqual(STOMP_INDEX, {"A": 0, "B": 1, "C": 2, "D": 3, "E": 4, "F": 5, "G": 6})


class StompWire(test_qcmcp_backend.Base):
    def setUp(self):
        super().setUp()
        self.dev.connect()
        self.qc = self.dev._raw  # the unguarded transport, to try forbidden sends directly
        self.plan = validate(export(), catalog(), "4.1.0")

    def armed(self, confirm=(0, 4)):
        self.dev.begin_push(152, "x", self.plan)
        for b in self.plan.blocks:
            if b.col in confirm:
                self.dev.confirm_layout(b.col, b.param_count + 1)

    def refused(self, fn, *a, **kw):
        n = len(self.hid.received)
        with self.assertRaises(WireRefused):
            fn(*a, **kw)
        self.assertEqual(len(self.hid.received), n, "nothing reached the device")

    def raw_assign(self, *a, **kw):
        return self.qc.assign_stomp(*a, settle_s=0, **kw)

    def assignments(self):
        return [a for n, o in self.hid.received if n == "Grid" for a in o.preset.stomp_mode_assignments]

    def test_full_push_assigns_the_planned_stomps(self):
        _jid, _key, data = job("push_preset")
        j = json.loads(contract.open_envelope(config().keys, contract.TAG_JOB, data))
        out = jobs.run(self.ctx(), j)
        self.assertEqual((out.status, out.reason_code), ("succeeded", ""), out.message)
        self.assertEqual(self.hid.stomps, {(0, 0): (2, 0), (0, 4): (6, 0)})
        self.assertEqual(len(self.assignments()), 2)
        A = P.ACTION
        grids = [o for n, o in self.hid.received if n == "Grid"]
        self.assertNotIn(A["DELETE"], [g.action for g in grids])                         # never unassign
        self.assertEqual([g for g in grids if len(g.preset.stomp_is_momentary)], [])    # latching: no flag sent
        self.assertEqual(self.dev._guard.refused, [])
        # the stomp messages come after every value write and before the save
        idx = [i for i, (n, o) in enumerate(self.hid.received) if n == "Grid" and len(o.preset.stomp_mode_assignments)]
        values = [i for i, (n, o) in enumerate(self.hid.received) if n == "Grid" and (len(o.preset.chains) or len(o.preset.bypass))]
        save = next(i for i, (n, o) in enumerate(self.hid.received) if n == "File" and o.action == A["CREATE"])
        self.assertLess(max(values), min(idx))
        self.assertLess(max(idx), save)

    def test_the_read_carries_the_assignments(self):
        self.hid.stomps = {(0, 1): (3, 0)}
        self.hid.momentary = {3: True}
        self.hid.grid[1] = {"hash": 1, "params": {}, "scene": set(), "bypass": [False] * 8, "bypass_scene": False}
        plain = self.dev.read_current_preset()
        self.assertEqual(plain["stomps"], [{"row": 0, "col": 1, "index": 3, "type": 0}])
        self.assertEqual(plain["momentary"], {3: True})

    def test_preset_to_plain_with_no_assignments(self):
        bp = P.message_class("RecallPreset")().preset
        bp.chains.add()
        plain = preset_to_plain(bp)
        self.assertEqual((plain["stomps"], plain["momentary"]), ([], {}))

    def test_never_outside_the_stomp_step(self):
        with self.assertRaisesRegex(GuardError, "never allowed"):
            self.dev._qc.assign_stomp                          # the guarded transport: still on NEVER
        self.refused(self.raw_assign, 0, 0, 2)                 # no push armed
        self.armed()
        self.refused(self.raw_assign, 0, 0, 2)                 # a push, but not its stomp step
        with self.assertRaisesRegex(DeviceError, "not an armed stomp"):
            self.dev.assign_stomp(0, 0, 2)
        self.dev.arm_stomps(self.plan.stomps)
        with self.assertRaisesRegex(GuardError, "never allowed"):
            self.dev._qc.assign_stomp                          # the window is per call, not per step

    def test_only_the_armed_assignments_once_each(self):
        self.armed()
        self.dev.arm_stomps(self.plan.stomps)
        self.refused(self.raw_assign, 0, 0, 3)                 # another switch for column 0
        self.refused(self.raw_assign, 0, 1, 2)                 # a block without a planned stomp
        self.refused(self.raw_assign, 1, 0, 2)                 # another row
        self.refused(self.raw_assign, 0, 0, 7)                 # H
        self.refused(self.raw_assign, 0, 0, 2, kind="secondary")
        self.refused(self.qc.unassign_stomp, 0, 0, 2, settle_s=0)  # a Grid DELETE
        self.dev.assign_stomp(0, 0, 2)                         # the plan's own passes
        self.refused(self.raw_assign, 0, 0, 2)                 # but only once
        with self.assertRaisesRegex(DeviceError, "not an armed stomp"):
            self.dev.assign_stomp(0, 0, 2)
        self.dev.assign_stomp(0, 4, 6)
        self.assertEqual(self.hid.stomps, {(0, 0): (2, 0), (0, 4): (6, 0)})

    def test_momentary_never_passes(self):
        self.armed()
        self.dev.arm_stomps(self.plan.stomps)
        with self.assertRaises(WireRefused):
            self.raw_assign(0, 0, 2, momentary=True)  # the assignment passes; the momentary message is refused
        self.assertEqual(self.hid.momentary, {})
        m = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        m.preset.stomp_is_momentary[6] = False
        self.refused(self.qc.send, "Grid", m)                  # not even "latching" written explicitly

    def test_two_assignments_in_one_message_are_refused(self):
        self.armed()
        self.dev.arm_stomps(self.plan.stomps)
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        for col, idx in ((0, 2), (4, 6)):
            a = g.preset.stomp_mode_assignments.add()
            a.row, a.column, a.stomp_index, a.type = 0, col, idx, 0
        self.refused(self.qc.send, "Grid", g)

    def test_the_stomp_step_ends_the_build(self):
        self.armed()
        self.qc.set_param(0, 0, 1, 0.55)                       # a planned value passes during the build
        self.dev.arm_stomps(self.plan.stomps)
        self.refused(self.qc.set_param, 0, 0, 1, 0.55)         # but not once the stomp step began
        self.refused(self.qc.add_block, 1, row=0, column=0)
        self.refused(self.qc.set_routing, 0, in_portid=1, out_portid=19)
        self.refused(self.qc.set_block_bypass, 0, 0, scenes=[False] * 8, bypass_param=4)
        s = P.message_class("Scene")(action=P.ACTION["UPDATE"], selected_scene=1)
        self.refused(self.qc.send, "Scene", s)

    def test_a_stomp_assignment_never_rides_a_build_message(self):
        self.armed()
        g = P.message_class("Grid")(action=P.ACTION["UPDATE"])
        a = g.preset.stomp_mode_assignments.add()
        a.row, a.column, a.stomp_index = 0, 0, 2
        self.refused(self.qc.send, "Grid", g)

    def test_arming_rules(self):
        with self.assertRaisesRegex(DeviceError, "not armed"):
            self.dev.arm_stomps(self.plan.stomps)              # no push
        self.armed(confirm=(0,))
        with self.assertRaisesRegex(WireRefused, "column 4: not a block this push placed and confirmed"):
            self.dev.arm_stomps(self.plan.stomps)              # column 4's layout wasn't confirmed on the unit
        g = self.dev._guard
        self.dev.confirm_layout(4, self.plan.blocks[4].param_count + 1)
        for bad in ([(0, 6, 2)], [(1, 0, 2)], [(0, 0, 7)], [(0, 0, -1)], [(0, 0, 2), (0, 0, 3)], []):
            with self.subTest(bad), self.assertRaises(WireRefused):
                g.arm_stomps(bad)
        self.dev.arm_stomps(self.plan.stomps)
        with self.assertRaisesRegex(WireRefused, "only be armed once"):
            self.dev.arm_stomps(self.plan.stomps)
        self.dev.end_push()
        self.assertFalse(g.stomp_armed(0, 0, 2))
        self.refused(self.raw_assign, 0, 0, 2)

    def test_not_after_the_save_is_armed(self):
        self.armed()
        self.dev._guard.arm_save(fake_hid.SETLIST, 152, "x")
        with self.assertRaisesRegex(WireRefused, "before the save"):
            self.dev.arm_stomps(self.plan.stomps)


class StompWindow(unittest.TestCase):
    def test_the_window_is_only_for_push_step_methods(self):
        self.assertTrue(PUSH_STEP_ONLY <= NEVER)
        self.assertIn("unassign_stomp", NEVER)
        self.assertNotIn("unassign_stomp", PUSH_STEP_ONLY)
        g = Guarded(mock.Mock())
        for name in ("unassign_stomp", "save_preset", "set_mode", "set_param"):
            with self.subTest(name), self.assertRaisesRegex(GuardError, "can't be opened"):
                with g.window(name):
                    pass

    def test_the_window_closes_and_does_not_nest(self):
        g = Guarded(mock.Mock())
        with g.window("assign_stomp"):
            g.assign_stomp
            with self.assertRaisesRegex(GuardError, "already open"):
                with g.window("assign_stomp"):
                    pass
            with self.assertRaisesRegex(GuardError, "never allowed"):
                g.unassign_stomp
        with self.assertRaisesRegex(GuardError, "never allowed"):
            g.assign_stomp
        with self.assertRaises(RuntimeError):
            with g.window("assign_stomp"):
                raise RuntimeError("boom")
        with self.assertRaisesRegex(GuardError, "never allowed"):
            g.assign_stomp

    def test_the_vendored_method_is_the_one_cited(self):
        # jobs/backend cite vendor/qc_mcp/transport.py:1172 for assign_stomp; pin its signature
        import inspect
        sig = inspect.signature(fake_hid.M["transport"].QuadCortex.assign_stomp)
        self.assertEqual(list(sig.parameters), ["self", "row", "column", "stomp_index", "kind", "momentary", "settle_s"])
        self.assertEqual(inspect.getsourcelines(fake_hid.M["transport"].QuadCortex.assign_stomp)[1], 1172)


if __name__ == "__main__":
    unittest.main()
