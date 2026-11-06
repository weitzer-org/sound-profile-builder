"""The agent's catalog copy: loading, hashing and every refusal."""
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

import helpers  # noqa: F401  (puts qc_agent on sys.path)
from qc_agent import catalog as cat
from qc_agent.catalog import Catalog, CatalogError


def raw(models):
    return json.dumps({"models": models}).encode()


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
            "param without name": raw([{**M1, "params": [{k: v for k, v in M1["params"][0].items() if k != "name"}]}]),
            "param without type": raw([{**M1, "params": [{k: v for k, v in M1["params"][0].items() if k != "type"}]}]),
            "param name not text": raw([{**M1, "params": [{**M1["params"][0], "name": 7}]}]),
        }
        for name, data in cases.items():
            with self.subTest(name), self.assertRaisesRegex(CatalogError, "malformed"):
                Catalog.from_bytes(data)


if __name__ == "__main__":
    unittest.main()
