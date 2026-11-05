"""Vendored qc-mcp files are byte-identical to upstream f634ecd.

The pins in qc_agent/vendor_pins.py were taken with `git show f634ecd:<path> | sha256sum`
on a clone of lexasoft123/qc-mcp; PROVENANCE.md lists the same values and the git
blob ids. A local change to any vendored file fails this test (and the agent refuses
to load the transport), so any change is deliberate and visible."""
import hashlib
import re
import unittest

from helpers import AGENT_DIR
from qc_agent.vendor_pins import COMMIT, PINS, VENDOR_DIR, mismatches


class VendorPins(unittest.TestCase):
    def test_each_file_matches_f634ecd(self):
        for rel, (_up, want) in PINS.items():
            with self.subTest(rel):
                self.assertEqual(hashlib.sha256((VENDOR_DIR / rel).read_bytes()).hexdigest(), want)

    def test_no_unpinned_files(self):
        self.assertEqual(mismatches(), [])

    def test_the_requested_set_is_vendored(self):
        want = {"transport.py", "protocol.py", "backend.py", "winhid.py", "directory.py", "LICENSE",
                "descriptors/qc_descriptors-4.0.pb", "descriptors/qc_descriptors-4.1.pb"}
        have = {rel.split("qc_mcp/", 1)[1] for rel in PINS}
        self.assertTrue(want <= have, want - have)

    def test_provenance_lists_every_pin(self):
        text = (VENDOR_DIR / "PROVENANCE.md").read_text(encoding="utf-8")
        self.assertIn(COMMIT, text)
        for rel, (up, sha) in PINS.items():
            with self.subTest(rel):
                self.assertIn(sha, text)
                self.assertIn(up, text)

    def test_no_line_ending_conversion(self):
        attrs = (VENDOR_DIR / ".gitattributes").read_text()
        self.assertTrue(re.search(r"^\* -text$", attrs, re.M), "vendored bytes must not be CRLF-converted on Windows")

    def test_descriptors_present_so_no_fallback_scan(self):
        # protocol._recovered_fdps falls back to importing tools/extract_protos when the
        # bundled descriptor sets are missing; with both present that path never runs.
        for gen in ("4.0", "4.1"):
            self.assertTrue((VENDOR_DIR / "qc_mcp" / "descriptors" / f"qc_descriptors-{gen}.pb").is_file())

    def test_vendored_code_never_imported_from_elsewhere(self):
        self.assertEqual(VENDOR_DIR, AGENT_DIR / "vendor")


if __name__ == "__main__":
    unittest.main()
