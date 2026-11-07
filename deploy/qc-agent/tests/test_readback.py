"""The read-back payload (contract section 9.1) against the shared fixtures.

internal/deviceexport/readback_test.go writes testdata/devicejobs/readback_roundtrip/*.json (the unit's
state in the agent's positional form, and the payload the agent must produce from it) and reads
testdata/devicejobs/readback.json (hand-written edge cases). This test feeds the same files to
qc_agent.jobs.readback_payload. If either side changes the shape, one of the two suites fails until the
other follows. The Go parser rejects unknown fields, so a field added here without a format bump fails there.
"""
import json
import math
import unittest

from helpers import REPO

from qc_agent import jobs, slots, text

FIXTURES = REPO / "testdata" / "devicejobs"


def _revive(x):
    """{"$float": "nan"|"inf"|"-inf"} -> the float (JSON cannot hold a non-finite number)."""
    if isinstance(x, dict) and set(x) == {"$float"}:
        return {"nan": math.nan, "inf": math.inf, "-inf": -math.inf}[x["$float"]]
    if isinstance(x, dict):
        return {k: _revive(v) for k, v in x.items()}
    if isinstance(x, list):
        return [_revive(v) for v in x]
    return x


def _load(path):
    return json.loads(path.read_text(encoding="utf-8"))


class SharedFixtures(unittest.TestCase):
    def test_edge_cases_produce_the_expected_payload(self):
        fx = _load(FIXTURES / "readback.json")
        self.assertEqual(fx["format"], "qc-device-readback/1")
        self.assertGreaterEqual(len(fx["cases"]), 3)
        for case in fx["cases"]:
            with self.subTest(case["name"]):
                got = jobs.readback_payload(_revive(case["device_preset"]), case["position"])
                self.assertEqual(got, case["expect_payload"])

    def test_every_go_roundtrip_fixture_is_reproduced(self):
        paths = sorted((FIXTURES / "readback_roundtrip").glob("*.json"))
        self.assertGreaterEqual(len(paths), 4, paths)
        for path in paths:
            fx = _load(path)
            with self.subTest(fx["name"]):
                got = jobs.readback_payload(fx["device_preset"], fx["position"])
                self.assertEqual(got, fx["expect_payload"])
                # Every block carries the device's full slot list, all eight scenes (G1, G6).
                for ch in got["chains"]:
                    for b in ch["blocks"]:
                        self.assertTrue(b["params"])
                        for p in b["params"]:
                            self.assertEqual(len(p["values"]), 8)

    def test_a_capture_name_survives_the_text_limit(self):
        # The old 64-character limit would have cut a capture's file_name to its 64-hex key.
        key = "0f" * 32
        preset = {"chains": [{"row": 0, "in_port": 1, "out_port": 19, "blocks": [
            {"col": 0, "model_id": 14001,
             "params": [{"index": 5, "values": [key + " My Capture"] * 8, "scene_mode": False}]}]}], "bypass": None}
        got = jobs.readback_payload(preset, {"folder": "My Presets", "position": 0})
        self.assertEqual(got["chains"][0]["blocks"][0]["params"][0]["values"][0], key + " My Capture")
        self.assertEqual(jobs.VALUE_STRING_LIMIT, 192)

    def test_no_unknown_keys_are_added(self):
        # The Go parser rejects unknown fields; pin the agent's key sets so a new key is a deliberate act.
        preset = {"chains": [{"row": 0, "in_port": 1, "out_port": 19, "blocks": [
            {"col": 0, "model_id": 1, "params": [{"index": 0, "values": [0.5] * 8, "scene_mode": False}]}]}],
            "bypass": [[[False] * 8 for _ in range(8)] for _ in range(4)]}
        got = jobs.readback_payload(preset, jobs.setlist_info({"folder_key": "k", "position": 1, "is_factory": False}))
        self.assertEqual(set(got), {"setlist", "chains", "bypass"})
        self.assertEqual(set(got["setlist"]), {"folder", "folder_key", "position", "is_factory"})
        self.assertEqual(set(got["chains"][0]), {"row", "in_port", "out_port", "blocks"})
        self.assertEqual(set(got["chains"][0]["blocks"][0]), {"col", "model_id", "params"})
        self.assertEqual(set(got["chains"][0]["blocks"][0]["params"][0]), {"index", "scene_mode", "values"})


if __name__ == "__main__":
    unittest.main()


class SetlistV2(unittest.TestCase):
    """testdata/devicejobs/readback_setlist.json, shared with Go's TestReadbackSetlistFixture: the unit's
    SetlistPosition reply becomes exactly the /2 setlist, and the read's slot is its position's label."""

    def test_every_reply_gives_the_expected_setlist(self):
        fx = _load(FIXTURES / "readback_setlist.json")
        self.assertEqual(fx["format"], jobs.READBACK_FORMAT)
        self.assertGreaterEqual(len(fx["cases"]), 20)
        for case in fx["cases"]:
            with self.subTest(case["name"]):
                got = jobs.setlist_info(case["reply"])
                self.assertEqual(got, case["expect"])
                slot = slots.label(got["position"]) if got["position"] is not None else None
                self.assertEqual(slot, case["slot"])
                # A key the contract carries is already what the agent's clean-up would leave.
                if got["folder_key"] is not None:
                    self.assertEqual(text.clean(got["folder_key"], 200), got["folder_key"])

    def test_not_a_dict(self):
        self.assertEqual(jobs.setlist_info(None),
                         {"folder": "other", "folder_key": None, "position": None, "is_factory": True})
