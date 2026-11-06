"""The agent's validator against the Go exporter's golden files.

internal/deviceexport writes these (testdata/golden/*.json; `go test ./internal/deviceexport
-run Golden -update`). If the Go side changes the export's shape, a golden changes, and this
test (or the validator tests) fails until the agent follows, and the other way round. The
validator and the exporter once drifted apart unnoticed; this is what stops that.

Also pins the two small tables the agent keeps in step with Go by value."""
import glob
import json
import re
import unittest

from helpers import GO_GOLDENS, REPO, catalog, go_golden
from qc_agent import export_validator as V

GOLDENS = sorted(p.stem for p in GO_GOLDENS.glob("*.json"))
FW_CONSTANTS = (REPO / "internal" / "catalog" / "firmware_constants.go").read_text()


class Goldens(unittest.TestCase):
    def test_goldens_exist(self):
        self.assertIn("pushable_g3", GOLDENS)
        self.assertGreaterEqual(len(GOLDENS), 4, glob.glob(str(GO_GOLDENS / "*")))

    def test_every_golden_gets_the_right_verdict(self):
        for name in GOLDENS:
            with self.subTest(name):
                g = json.loads((GO_GOLDENS / f"{name}.json").read_text())
                e = go_golden(name)
                if g["unresolved"]:
                    # The eval presets use captures, switches, unconfirmed tapers: not pushable in v1.
                    with self.assertRaisesRegex(V.ExportRefused, "unresolved item"):
                        V.validate(e, catalog(), "4.1.0")
                else:
                    V.validate(e, catalog(), "4.1.0")

    def test_the_pushable_golden_is_accepted_unmodified(self):
        # No catalog-hash override: Go's embedded catalog and the agent's copy are the same file.
        g = go_golden("pushable_g3")
        self.assertEqual(g["payload"]["catalog"]["sha256"], catalog().sha256)
        plan = V.validate(g, catalog(), "4.1.0")
        self.assertEqual(len(plan.blocks), 5)

    def test_every_golden_stomp_sits_on_a_placed_block(self):
        for name in GOLDENS:
            with self.subTest(name):
                p = go_golden(name)["payload"]
                cols = {b["col"] for b in p["rows"][0]["blocks"]}
                self.assertIsInstance(p["stomps"], list)
                for s in p["stomps"]:
                    self.assertEqual(set(s), {"row", "col", "switch"})
                    self.assertIn(s["col"], cols)
                    self.assertIn(s["switch"], V.STOMP_SWITCHES)
                self.assertEqual(len({s["col"] for s in p["stomps"]}), len(p["stomps"]))


class TablesMatchGo(unittest.TestCase):
    def test_confirmed_bounds(self):
        # Exactly the bounds Go marks EvidenceConfirmed410, with the same numbers.
        go = {m.group(1): float(m.group(2))
              for m in re.finditer(r'"(M(?:IN|AX)_[A-Z0-9_]+)":\s*\{(-?[0-9.]+),\s*EvidenceConfirmed410', FW_CONSTANTS)}
        self.assertEqual(V.CONFIRMED_BOUNDS, go)

    def test_verified_clone_bases(self):
        block = re.search(r"var VerifiedCloneBases = map\[int\]string\{(.*?)\n\}", FW_CONSTANTS, re.S).group(1)
        go = {int(m) for m in re.findall(r"^\s*(\d+):", block, re.M)}
        self.assertEqual(set(V.VERIFIED_CLONE_BASES), go)


if __name__ == "__main__":
    unittest.main()
