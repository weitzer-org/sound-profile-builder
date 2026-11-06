"""The capture resolver (qc_agent.captures) over hand-built directory snapshots.

Proves the agent's rules for picking a capture and its V1/V2 block. Says nothing about
the unit: folder names, the version field and dependency lists there are unverified (P6)."""
import unittest
from types import SimpleNamespace as NS

from fake_device import KEY_USER, KEY_V1, KEY_V2, capture, default_captures
from qc_agent import captures as C


def snap(entries=None, deps=None):
    entries = default_captures() if entries is None else entries
    folders = {}
    for e in entries:
        n, f, c = folders.get(e.folder_name, (e.folder_name, e.is_factory, 0))
        folders[e.folder_name] = (n, f, c + 1)
    return C.DirectorySnapshot(my_presets={}, preset_deps=deps or {}, captures=entries,
                               capture_folders=list(folders.values()))


def blk(name, source, col=0):
    return NS(col=col, capture=NS(name=name, source=source))


class Resolve(unittest.TestCase):
    def refused(self, needle, name, source, s=None):
        with self.assertRaises(C.CaptureRefused) as cm:
            C.resolve_one(name, source, 3, s or snap())
        self.assertIn(needle, str(cm.exception))
        self.assertLessEqual(len(str(cm.exception)), 600)
        return str(cm.exception)

    def test_factory_v1_and_v2(self):
        r = C.resolve_one("CA John's", "factory", 3, snap())
        self.assertEqual((r.key, r.version, r.file_name), (KEY_V1, 1, KEY_V1 + "CA John's"))
        r = C.resolve_one("CA John's 2", "factory", 3, snap())
        self.assertEqual((r.key, r.version), (KEY_V2, 2))

    def test_exact_names_only(self):
        # "CA John's" never matches "CA John's 2", nor a different case or spacing
        self.assertEqual(C.resolve_one("CA John's", "factory", 0, snap()).key, KEY_V1)
        msg = self.refused("isn't on the unit", "ca john's", "factory")
        self.assertIn("Close names on the unit: \"CA John's\"", msg)
        self.refused("isn't on the unit", "CA  John's", "factory")

    def test_user_capture_version_from_calibration(self):
        r = C.resolve_one("66 AC30 JMP OD", "user", 0, snap())
        self.assertEqual((r.key, r.version), (KEY_USER, 1))
        e = default_captures() + [capture("Klon user", "e5" * 32, version="2")]
        self.assertEqual(C.resolve_one("Klon user", "user", 0, snap(e)).version, 2)

    def test_calibration_fails_closed(self):
        user = capture("66 AC30 JMP OD", KEY_USER, version="1")
        cases = {
            "mixed V1 values": [capture("A", "a1" * 32, "Factory Captures V1", True, "1"),
                                capture("B", "a2" * 32, "Factory Captures V1", True, "x"),
                                capture("C", "a3" * 32, "Factory Captures V2", True, "2"), user],
            "empty values": [capture("A", "a1" * 32, "Factory Captures V1", True, ""),
                             capture("C", "a3" * 32, "Factory Captures V2", True, "2"), user],
            "same value in both": [capture("A", "a1" * 32, "Factory Captures V1", True, "1"),
                                   capture("C", "a3" * 32, "Factory Captures V2", True, "1"), user],
            "no V2 folder": [capture("A", "a1" * 32, "Factory Captures V1", True, "1"), user],
            "user value unknown": [capture("A", "a1" * 32, "Factory Captures V1", True, "1"),
                                   capture("C", "a3" * 32, "Factory Captures V2", True, "2"),
                                   capture("66 AC30 JMP OD", KEY_USER, version="9")],
        }
        for label, entries in cases.items():
            with self.subTest(label):
                self.refused("can't tell whether capture", "66 AC30 JMP OD", "user", snap(entries))

    def test_ambiguous(self):
        e = default_captures() + [capture("66 AC30 JMP OD", "f6" * 32, "Downloads", version="1")]
        self.refused("matches 2 different captures", "66 AC30 JMP OD", "user", snap(e))

    def test_same_key_twice_and_favorites_count_once(self):
        e = default_captures() + [capture("66 AC30 JMP OD", KEY_USER.upper(), "Other", version="1"),
                                  capture("66 AC30 JMP OD", "f6" * 32, "Favorites", favorites=True)]
        self.assertEqual(C.resolve_one("66 AC30 JMP OD", "user", 0, snap(e)).version, 1)

    def test_source_mismatch(self):
        self.refused("only has it as a factory capture", "CA John's", "user")
        self.refused("only has it as a user capture", "66 AC30 JMP OD", "factory")

    def test_cloud_only(self):
        e = default_captures() + [capture("Cloud Amp", "a7" * 32, "Cloud", cloud=True)]
        self.refused("only as a Cortex Cloud", "Cloud Amp", "user", snap(e))

    def test_unknown_factory_folder(self):
        e = [capture("Odd", "a8" * 32, "Factory Captures V3", True, "3")] + default_captures()
        msg = self.refused("doesn't know as V1 or V2", "Odd", "factory", snap(e))
        self.assertIn("'Factory Captures V3'", msg)

    def test_bad_key(self):
        e = default_captures() + [capture("Bad", "not-hex", version="1")]
        self.refused("a key the bridge can't use", "Bad", "user", snap(e))

    def test_device_text_is_bounded(self):
        hostile = "x\n# injected `code` " + "y" * 300
        e = default_captures() + [capture(hostile, "a9" * 32, "F\x00" + "z" * 300, version="1"),
                                  capture(hostile.upper(), "aa" * 32, "Downloads", version="1")]
        # a different case is a different name: both show only as near-misses, cleaned and bounded
        msg = self.refused("isn't on the unit", hostile.casefold() + " ", "user", snap(e))
        self.assertIn("Close names on the unit", msg)
        for bad in ("\n", "\x00", "`"):
            self.assertNotIn(bad, msg)
        # one exact match resolves even with hostile text in it (it's only ever compared)
        self.assertEqual(C.resolve_one(hostile, "user", 0, snap(e)).key, "a9" * 32)

    def test_resolve_and_with_resolution(self):
        from qc_agent.export_validator import CAPTURE_BLOCK_IDS, PlannedBlock, PlannedCapture
        plan = NS(blocks=[PlannedBlock(0, 1, "Myth Drive", [], [False] * 8),
                          PlannedBlock(1, 0, "CA John's 2", [], [False] * 8, 4, PlannedCapture("CA John's 2", "factory"))])
        res = C.resolve(plan.blocks, snap())
        self.assertEqual(list(res), [1])
        from dataclasses import dataclass

        @dataclass
        class Plan:
            blocks: list
        out = C.with_resolution(Plan(plan.blocks), res, CAPTURE_BLOCK_IDS)
        b = out.blocks[1]
        self.assertEqual((b.model_id, b.capture.key, b.capture.version, b.capture.file_name),
                         (14001, KEY_V2, 2, KEY_V2 + "CA John's 2"))
        self.assertIs(out.blocks[0], plan.blocks[0])


class SummaryCap(unittest.TestCase):
    def test_at_most_four_named(self):
        res = {c: C.ResolvedCapture("x" * 64, "user", "ab" * 32, 1) for c in range(7)}
        s = C.summary(res)
        self.assertIn("+3 more", s)
        self.assertLess(len(s), 260)


class Reports(unittest.TestCase):
    def res(self):
        return {3: C.ResolvedCapture("66 AC30 JMP OD", "user", KEY_USER, 1)}

    def test_deps_present(self):
        s = snap(deps={152: frozenset({KEY_USER})})
        self.assertIn("names every placed capture", C.deps_report(s, 152, self.res()))

    def test_deps_absent_is_an_observation(self):
        s = snap(deps={152: frozenset({"ff" * 32}), 7: frozenset({"ee" * 32})})
        self.assertIn("Observation", C.deps_report(s, 152, self.res()))

    def test_no_dependency_lists_anywhere_is_unchecked(self):
        s = snap(deps={152: frozenset(), 7: frozenset()})
        self.assertIn("wasn't checked", C.deps_report(s, 152, self.res()))
        self.assertIn("wasn't checked", C.deps_report(snap(), 152, self.res()))

    def test_no_captures_no_report(self):
        self.assertEqual(C.deps_report(snap(), 152, {}), "")
        self.assertEqual(C.summary({}), "")

    def test_summary(self):
        self.assertEqual(C.summary(self.res()), " Captures: col 3 '66 AC30 JMP OD' V1 (user).")


if __name__ == "__main__":
    unittest.main()
