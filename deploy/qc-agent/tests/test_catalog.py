"""The agent's catalog copy: loading, hashing, every refusal, and the confirmed-taper constants
mirrored from Go (a drift between the two fails here)."""
import hashlib
import json
import re
import tempfile
import unittest
from pathlib import Path

import helpers  # noqa: F401  (puts qc_agent on sys.path)
from qc_agent import catalog as cat
from qc_agent.catalog import Catalog, CatalogError


def raw(models, source=None):
    return json.dumps({"source": source or {"coros_version": "4.1", "origin": "test"}, "models": models}).encode()


M1 = {"id": 1, "name": "Myth Drive", "category": "Guitar Overdrive",
      "params": [{"index": 0, "name": "GAIN", "type": "float", "min": 0, "max": 10}]}


class Load(unittest.TestCase):
    def test_repo_catalog_loads(self):
        c = Catalog.load()
        self.assertGreater(len(c.models), 500)
        self.assertEqual(c.models[1]["params_by_index"][0]["name"], "GAIN")
        self.assertRegex(c.sha256, r"^[0-9a-f]{64}$")

    def test_hash_ignores_crlf_checkouts(self):
        lf = raw([M1]).replace(b",", b",\n")
        a, b = Catalog.from_bytes(lf), Catalog.from_bytes(lf.replace(b"\n", b"\r\n"))
        self.assertEqual(a.sha256, b.sha256)
        self.assertEqual(a.sha256, hashlib.sha256(lf).hexdigest())

    def test_missing_file(self):
        with self.assertRaisesRegex(CatalogError, "cannot read"):
            Catalog.load(Path(tempfile.gettempdir()) / "no-such-catalog.json")

    def test_oversized_file(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "c.json"
            p.write_bytes(b" " * (cat.MAX_CATALOG_BYTES + 1))
            with self.assertRaisesRegex(CatalogError, "unexpectedly large"):
                Catalog.load(p)

    def test_malformed(self):
        cases = {
            "not json": b"{",
            "no models": b"{}",
            "duplicate model": raw([M1, M1]),
            "bool id": raw([{**M1, "id": True}]),
            "duplicate param": raw([{**M1, "params": M1["params"] * 2}]),
            "missing params": raw([{k: v for k, v in M1.items() if k != "params"}]),
            "no source header": json.dumps({"models": [M1]}).encode(),
        }
        for name, data in cases.items():
            with self.subTest(name), self.assertRaisesRegex(CatalogError, "malformed"):
                Catalog.from_bytes(data)


GO_CATALOG = helpers.REPO / "internal" / "catalog"
GO_EXPORT = helpers.REPO / "internal" / "deviceexport"


def _go_block(path, var):
    text = path.read_text(encoding="utf-8")
    m = re.search(r"var " + var + r" = map\[[^\]]+\][^{]*\{(.*?)\n\}", text, re.S)
    assert m, f"{var} not found in {path.name}"
    return m.group(1)


class MirrorsGo(unittest.TestCase):
    """The agent re-implements a few of P2's rules; these fail if the Go side moves."""

    def test_confirmed_skews(self):
        body = _go_block(GO_CATALOG / "firmware_constants.go", "ConfirmedSkews")
        keys = {float(k) for k in re.findall(r"^\s*([0-9.]+):", body, re.M)}
        self.assertEqual(keys, set(cat.CONFIRMED_SKEWS))

    def test_verified_clone_bases(self):
        body = _go_block(GO_CATALOG / "firmware_constants.go", "VerifiedCloneBases")
        self.assertEqual({int(k) for k in re.findall(r"^\s*([0-9]+):", body, re.M)}, set(cat.VERIFIED_CLONE_BASES))

    def test_confirmed_bounds(self):
        body = _go_block(GO_CATALOG / "firmware_constants.go", "FirmwareBounds")
        confirmed = {name: float(v) for name, v in
                     re.findall(r'"([A-Z_]+)":\s*\{(-?[0-9.]+),\s*EvidenceConfirmed410', body)}
        self.assertEqual(confirmed, cat.CONFIRMED_BOUNDS)

    def test_internal_ids_and_categories(self):
        from qc_agent.export_validator import ALLOWED_CATEGORIES
        text = (GO_EXPORT / "resolve.go").read_text(encoding="utf-8")
        ids = re.search(r"var internalModelIDs = map\[int\]bool\{([^}]*)\}", text).group(1)
        self.assertEqual({int(x) for x in re.findall(r"([0-9]+):", ids)}, set(cat.INTERNAL_MODEL_IDS))
        cats = _go_block(GO_EXPORT / "resolve.go", "pushableCategories")
        self.assertEqual(set(re.findall(r'"([^"]+)":', cats)), set(ALLOWED_CATEGORIES))

    def test_internal_param_rules(self):
        text = (GO_CATALOG / "labels.go").read_text(encoding="utf-8")
        names = re.search(r"var internalParamNames = map\[string\]bool\{([^}]*)\}", text).group(1)
        types = re.search(r"var internalParamTypes = map\[string\]bool\{([^}]*)\}", text).group(1)
        self.assertEqual(set(re.findall(r'"([^"]+)":', names)), set(cat._INTERNAL_PARAM_NAMES))
        self.assertEqual(set(re.findall(r'"([^"]+)":', types)), set(cat._INTERNAL_PARAM_TYPES))


class Labels(unittest.TestCase):
    def test_dual_mic_cab(self):
        lab = cat.labels(Catalog.load().models[12045])
        self.assertEqual((lab[2], lab[10], lab[16], lab[18]), ("MIC 1 LEVEL", "MIC 2 LEVEL", "HPF", "OUTPUT VOLUME"))
        self.assertNotIn(0, lab)   # the per-mic bypass switch
        self.assertNotIn(1, lab)   # ir selector

    def test_plain_model(self):
        self.assertEqual(cat.labels(Catalog.load().models[1]), {0: "GAIN", 1: "TREBLE", 2: "LEVEL"})


class Taper(unittest.TestCase):
    def test_g4_points(self):
        c = Catalog.load()
        # plan 7.1 G4, read on the owner's screen: wire -> display
        cases = [((1001, 6), -10.0, 0.25), ((12045, 2), 6.0, 1.0), ((4003, 1), 217.0, 0.25), ((1001, 0), 4.5, 0.45)]
        for (mid, idx), display, wire in cases:
            with self.subTest(mid=mid, idx=idx):
                self.assertAlmostEqual(cat.to_wire(c.models[mid]["params_by_index"][idx], display), wire, places=3)

    def test_refusals(self):
        c = Catalog.load()
        with self.assertRaisesRegex(ValueError, "outside"):
            cat.to_wire(c.models[1001]["params_by_index"][6], -61)
        with self.assertRaisesRegex(ValueError, "pan"):
            cat.to_wire(c.models[12045]["params_by_index"][3], 5)
        with self.assertRaisesRegex(ValueError, "switch"):
            cat.to_wire(c.models[8006]["params_by_index"][4], 1)
        self.assertIsNone(cat.default_wire(c.models[8006]["params_by_index"][4]))
        self.assertAlmostEqual(cat.default_wire(c.models[1001]["params_by_index"][4]), 0.3)


if __name__ == "__main__":
    unittest.main()
