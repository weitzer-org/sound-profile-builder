"""sync_device_library (plan P4b): the capture list payload, the sync job, and the cab mic write.

The library payload is pinned against testdata/devicejobs/library.json, which the Go tests parse too
(internal/devicejobs, ParseLibraryPayload), so the two sides cannot drift. Everything here runs against a
fake unit: it proves the agent's logic and guards, not that a real unit accepts a mic string."""
from __future__ import annotations

import copy
import json
import tempfile
import unittest
from pathlib import Path

from helpers import REPO, Clock, config, catalog, go_golden, job, open_result, export
from qc_agent import captures, contract, jobs
from qc_agent.agent import Agent
from qc_agent.captures import CaptureEntry, DirectorySnapshot
from qc_agent.export_validator import ExportRefused, validate
from qc_agent.s3 import MemoryStore
from fake_device import FakeDevice, capture

FIXTURE = json.loads((REPO / "testdata" / "devicejobs" / "library.json").read_text())


def setUpModule():
    from unittest import mock
    for name in ("SAVE_LIST_INTERVAL_S", "PACE_S"):
        p = mock.patch.object(jobs, name, 0)
        p.start()
        unittest.addModuleCleanup(p.stop)


def snapshot(s):
    return DirectorySnapshot(
        my_presets={}, capture_folders=[tuple(f) for f in s["folders"]],
        captures=[CaptureEntry(name=e["name"], key=e["key"], folder_key="k-" + e["folder"], folder_name=e["folder"],
                               is_factory=e["factory"], is_cloud=e["cloud"], is_favorites=e["favorites"],
                               version_field=e["version_field"], factory_version=e.get("factory_version"))
                  for e in s["entries"]])


class Payload(unittest.TestCase):
    def test_shared_fixture(self):
        for s in FIXTURE["snapshots"]:
            with self.subTest(s["name"]):
                self.assertEqual(captures.library_payload(snapshot(s)), s["expect_payload"])

    def test_too_many_fails_instead_of_truncating(self):
        entries = [capture(f"C{i}", f"{i:064x}", "Factory Captures V1", factory=True, version="1")
                   for i in range(captures.MAX_LIBRARY_CAPTURES + 1)]
        with self.assertRaises(captures.LibraryTooLarge):
            captures.library_payload(DirectorySnapshot(my_presets={}, captures=entries,
                                                       capture_folders=[("Factory Captures V1", True, len(entries))]))

    def test_worst_case_library_fits_a_sealed_result(self):
        # the limits were sized together (owner, 2026-11-06): MAX_LIBRARY_CAPTURES entries with 64-character
        # names, 40-character folders and the escaping the envelope adds must fit MAX_RESULT_BYTES
        entries = [capture(f"{i:04d}" + "N" * 60, f"{i:064x}", "F" * 40, factory=True, version="",
                           above=("Factory Captures V1",)) for i in range(captures.MAX_LIBRARY_CAPTURES)]
        lib = captures.library_payload(DirectorySnapshot(my_presets={}, captures=entries,
                                                         capture_folders=[("F" * 40, True, len(entries))]))
        self.assertEqual(len(lib["captures"]), captures.MAX_LIBRARY_CAPTURES)
        result = {"v": 1, "type": "result", "job_id": "0" * 36, "kind": "sync_device_library", "status": "succeeded",
                  "message": "x" * 500, "readback": {"format": captures.LIBRARY_FORMAT, "payload": lib}}
        sealed = contract.seal_payload(b"k" * 32, contract.TAG_RESULT, result)
        self.assertLess(len(sealed), contract.MAX_RESULT_BYTES)

    def test_version_is_the_pushes_own(self):
        # library_payload uses version_of: a push and the sync can never disagree about V1/V2
        for s in FIXTURE["snapshots"]:
            snap = snapshot(s)
            by_name = {c["name"]: c["version"] for c in captures.library_payload(snap)["captures"]}
            for e in snap.captures:
                if e.name in by_name:
                    self.assertEqual(by_name[e.name], captures.version_of(e, snap), e.name)


class Base(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.store = MemoryStore("sound-profile-builder-device-jobs")
        self.dev = FakeDevice()
        self.clock = Clock()
        self.agent = Agent(config(), self.store, self.dev, catalog(), Path(self.tmp.name), now_fn=self.clock,
                           sleep=lambda s: None)
        self.agent.ctx.sleep = lambda s: None

    def run_job(self, kind, **kw):
        jid, key, data = job(kind, **kw)
        self.store.objects[key] = data
        for k, d in self.agent.scan():
            self.agent.run_job(k, d)
        return open_result(self.store, jid)


class SyncJob(Base):
    def test_lists_the_unit_and_writes_nothing(self):
        r = self.run_job("sync_device_library", args={})
        self.assertEqual((r["status"], r["reason_code"], r["kind"]), ("succeeded", "", "sync_device_library"))
        self.assertEqual(r["library"]["format"], "qc-device-library/1")
        caps = r["library"]["payload"]["captures"]
        self.assertEqual([c["name"] for c in caps], [c.name for c in self.dev.captures])
        self.assertEqual({c["name"]: c["version"] for c in caps}["CA John's 2"], 2)
        self.assertEqual(self.dev.write_calls(), [])
        names = {c[0] for c in self.dev.calls}
        self.assertNotIn("begin_push", names)
        self.assertEqual([c for c in self.dev.calls if c[0] == "read_directory"].__len__(), 1)  # one listing, P4c's
        self.assertIn("Listed 4 capture files", r["message"])
        self.assertIsNone(r["push"])
        self.assertIsNone(r["readback"])

    def test_other_kinds_carry_no_library(self):
        r = self.run_job("list_slots")
        self.assertIsNone(r["library"])

    def test_wrong_firmware_is_refused(self):
        self.dev.fw = "4.0.9"
        r = self.run_job("sync_device_library", args={})
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "firmware_mismatch"))
        self.assertIsNone(r["library"])

    def test_no_capture_folders_fails(self):
        self.dev.captures = []
        r = self.run_job("sync_device_library", args={})
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "device_error"))
        self.assertIn("no capture folders", r["message"])

    def test_device_text_is_bounded_in_the_message(self):
        self.dev.captures = [capture("X" * 5000, "ab" * 32, "F" * 5000, factory=True, version="1")] + self.dev.captures
        r = self.run_job("sync_device_library", args={})
        self.assertEqual(r["status"], "succeeded")
        self.assertEqual(len(r["library"]["payload"]["captures"]), 4)       # the 5000-character one is skipped
        self.assertEqual(r["library"]["payload"]["skipped"], 1)

    def test_a_push_is_unchanged_by_the_new_kind(self):
        r = self.run_job("push_preset")
        self.assertEqual(r["status"], "succeeded", r["message"])
        self.assertIsNone(r["library"])


def cab_export(value=None, slot=1, **over):
    """The G3 golden with a mic write on its cab (110 US PRN C10R (M), column 2)."""
    e = export()
    cab = e["payload"]["rows"][0]["blocks"][2]
    idx = {1: 1, 2: 9}.get(slot, 1)
    m = {"slot": slot, "index": idx, "value": value or "ME_110 US PRN C10R_Ribbon 121", "evidence": "seen-on-unit"}
    m.update(over)
    cab["mics"] = [m]
    return e


class MicValidation(unittest.TestCase):
    def refused(self, e, needle):
        with self.assertRaisesRegex(ExportRefused, needle):
            validate(e, catalog(), "4.1.0")

    def test_a_variation_of_the_cabs_own_default_passes(self):
        plan = validate(cab_export(), catalog(), "4.1.0")
        m = plan.blocks[2].mics
        self.assertEqual([(x.slot, x.index, x.value) for x in m], [(1, 1, "ME_110 US PRN C10R_Ribbon 121")])
        plan = validate(cab_export("ME_110 US PRN C10R_Condenser 414", slot=2), catalog(), "4.1.0")
        self.assertEqual(plan.blocks[2].mics[0].index, 9)

    def test_an_export_without_mics_is_as_before(self):
        self.assertEqual(validate(export(), catalog(), "4.1.0").blocks[2].mics, [])

    def test_another_cab_or_another_ir_is_refused(self):
        self.refused(cab_export("ME_212 Parallax Cab_Dynamic 57"), "not a mic of this cab")
        self.refused(cab_export("C:\\IRs\\evil.wav"), "not a mic of this cab")
        self.refused(cab_export("ME_110 US PRN C10R_"), "not a mic of this cab")
        self.refused(cab_export("ME_110 US PRN C10R"), "not a mic of this cab")
        self.refused(cab_export("ME_110 US PRN C10R_Rib`bon"), "not a mic of this cab")
        self.refused(cab_export("ME_110 US PRN C10R_Ribbon 121 (junk)"), "not a mic of this cab")
        self.refused(cab_export("ME_110 US PRN C10R_Ribbon 121;x"), "not a mic of this cab")

    def test_wrong_index_slot_or_evidence(self):
        self.refused(cab_export(index=2), "ir selector")
        self.refused(cab_export(slot=3), "not one of this cab's mic slots")
        self.refused(cab_export(evidence="guess"), "evidence")
        self.refused(cab_export(index=999), "ir selector")

    def test_bounds_and_shape(self):
        self.refused(cab_export("ME_110 US PRN C10R_" + "x" * 200), "at most 128")
        self.refused(cab_export("ME_110 US PRN C10R_Rib\u00e9"), "printable ASCII")
        e = cab_export()
        e["payload"]["rows"][0]["blocks"][2]["mics"][0]["extra"] = 1
        self.refused(e, "has keys")
        e = cab_export()
        e["payload"]["rows"][0]["blocks"][2]["mics"] = e["payload"]["rows"][0]["blocks"][2]["mics"] * 2
        self.refused(e, "appears twice")

    def test_only_a_cab_has_mics(self):
        e = export()
        e["payload"]["rows"][0]["blocks"][1]["mics"] = [{"slot": 1, "index": 1, "value": "ME_x_y", "evidence": "seen-on-unit"}]
        self.refused(e, "only a cab")


class MicPush(Base):
    def push(self, e):
        return self.run_job("push_preset", args={"slot": "20A", "name": "Texas Blues",
                                                 "source": {"preset_id": "p", "guitar": "g"}, "export": e})

    def test_the_mic_is_written_once_exactly_before_the_blocks_values(self):
        r = self.push(cab_export())
        self.assertEqual((r["status"], r["reason_code"]), ("succeeded", ""), r["message"])
        calls = self.dev.calls
        mic = [c for c in calls if c[0] == "set_mic"]
        self.assertEqual(mic, [("set_mic", 0, 2, 1, "ME_110 US PRN C10R_Ribbon 121")])
        i = calls.index(mic[0])
        cab_values = [k for k, c in enumerate(calls) if c[0].startswith("set_param") and c[2] == 2]
        self.assertTrue(cab_values and all(k > i for k in cab_values))
        self.assertEqual(r["push"]["mismatches"], 0)

    def test_the_read_back_checks_the_string(self):
        orig = self.dev.set_mic
        self.dev.set_mic = lambda row, col, index, value: orig(row, col, index, value + "X")
        r = self.push(cab_export())
        self.assertEqual((r["status"], r["reason_code"]), ("failed", "readback_mismatch"))
        self.assertNotIn("save_to_slot", [c[0] for c in self.dev.calls])

    def test_no_mics_no_string_write(self):
        self.push(export())
        self.assertNotIn("set_mic", [c[0] for c in self.dev.calls])

    def test_a_bad_mic_refuses_before_the_unit_is_touched(self):
        r = self.push(cab_export("ME_212 Parallax Cab_Dynamic 57"))
        self.assertEqual((r["status"], r["reason_code"]), ("refused", "validation"))
        self.assertEqual(self.dev.write_calls(), [])


if __name__ == "__main__":
    unittest.main()
