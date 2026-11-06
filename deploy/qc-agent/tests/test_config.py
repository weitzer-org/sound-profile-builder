"""agent.env parsing: exactly the setup script's names, strict, and no secret in any message or repr."""
import os
import re
import tempfile
import unittest
from unittest import mock
from pathlib import Path

from helpers import AGENT_DIR, KEY_HEX, NEXT_HEX, env
from qc_agent import config as cfg
from qc_agent.slots import SlotRanges, label, position


def text(e):
    return "\n".join(f"{k}={v}" for k, v in e.items()) + "\n"


class Names(unittest.TestCase):
    def test_names_match_the_setup_script(self):
        src = (AGENT_DIR / "write_agent_env.py").read_text(encoding="utf-8")
        for name in cfg.REQUIRED + cfg.OPTIONAL:
            with self.subTest(name):
                self.assertIn(name + "=", src)

    def test_setup_script_accepts_the_same_firmware_strings(self):
        # write_agent_env.py is standalone (stdlib only), so it keeps its own copy of the pattern.
        import importlib.util
        spec = importlib.util.spec_from_file_location("write_agent_env", AGENT_DIR / "write_agent_env.py")
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)  # module level only defines things; main() is behind __main__
        self.assertEqual(mod.FIRMWARE_RE.pattern, cfg._FIRMWARE_RE.pattern)
        for v in ("4.1.0", "4.10.12", "4.1", "4.1.0b", " 4.1.0", "v4.1.0"):
            with self.subTest(v):
                self.assertEqual(bool(mod.FIRMWARE_RE.fullmatch(v)), bool(cfg._FIRMWARE_RE.fullmatch(v)))

    def test_file_written_by_the_setup_script_loads(self):
        # The exact lines write_agent_env.py writes, including its comments.
        lines = [
            "# QC bridge config. Secrets: keep this file private; never commit or paste it.",
            "QC_AGENT_R2_ENDPOINT=https://abc123.r2.cloudflarestorage.com",
            "QC_AGENT_R2_BUCKET=sound-profile-builder-device-jobs",
            "QC_AGENT_R2_ACCESS_KEY_ID=AKID0123",
            "QC_AGENT_R2_SECRET_ACCESS_KEY=s3cr3t",
            f"DEVICE_JOB_KEY={KEY_HEX}",
            "# DEVICE_JOB_KEY_NEXT=   (only during a key rotation)",
            "QC_AGENT_ALLOWED_SLOTS=1A-32H",
            "QC_AGENT_EXPECTED_FIRMWARE=4.1.0",
            "QC_AGENT_IDLE_MINUTES=30",
            "",
        ]
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "agent.env"
            p.write_text("\n".join(lines), encoding="utf-8")
            c = cfg.load(p)
        self.assertEqual((c.bucket, c.expected_firmware, c.idle_minutes, c.current_kid),
                         ("sound-profile-builder-device-jobs", "4.1.0", 30, "72eb33c6"))
        self.assertEqual(c.allowed_slots.labels(), ["1A-32H"])
        self.assertEqual(len(c.keys), 1)


class Strict(unittest.TestCase):
    def bad(self, needle, **over):
        with self.assertRaisesRegex(cfg.ConfigError, re.escape(needle)) as cm:
            cfg.from_mapping(env(**over))
        return str(cm.exception)

    def test_missing(self):
        self.bad("missing: DEVICE_JOB_KEY", DEVICE_JOB_KEY=None)

    def test_bad_values(self):
        self.bad("QC_AGENT_R2_ENDPOINT", QC_AGENT_R2_ENDPOINT="http://x.example")
        self.bad("QC_AGENT_R2_ENDPOINT", QC_AGENT_R2_ENDPOINT="https://x.example/path")
        self.bad("QC_AGENT_R2_BUCKET", QC_AGENT_R2_BUCKET="Bad_Bucket")
        self.bad("64 hex", DEVICE_JOB_KEY="ab cd")
        self.bad("64 hex", DEVICE_JOB_KEY_NEXT="zz")
        self.bad("QC_AGENT_ALLOWED_SLOTS", QC_AGENT_ALLOWED_SLOTS="33A")
        self.bad("QC_AGENT_ALLOWED_SLOTS", QC_AGENT_ALLOWED_SLOTS="5A-2A")
        self.bad("QC_AGENT_EXPECTED_FIRMWARE", QC_AGENT_EXPECTED_FIRMWARE="4.1")
        self.bad("QC_AGENT_IDLE_MINUTES", QC_AGENT_IDLE_MINUTES="0")
        self.bad("QC_AGENT_IDLE_MINUTES", QC_AGENT_IDLE_MINUTES="241")

    def test_malformed_next_key_is_named(self):
        msg = self.bad("DEVICE_JOB_KEY_NEXT must be exactly 64 hex characters", DEVICE_JOB_KEY_NEXT="zz")
        self.assertNotIn("zz", msg)

    def test_unreadable_file_is_a_config_error(self):
        with tempfile.TemporaryDirectory() as d:
            with self.assertRaisesRegex(cfg.ConfigError, "cannot read"):
                cfg.load(Path(d))  # a directory: open() fails with an OSError other than FileNotFoundError

    def test_errors_never_echo_secrets(self):
        secret = "f" * 63 + "g"
        msg = self.bad("DEVICE_JOB_KEY", DEVICE_JOB_KEY=secret)
        self.assertNotIn(secret, msg)

    def test_unknown_and_duplicate_lines(self):
        with self.assertRaisesRegex(cfg.ConfigError, "unknown setting QC_AGENT_BUCKET"):
            cfg.parse_env_text("QC_AGENT_BUCKET=x\n")
        with self.assertRaisesRegex(cfg.ConfigError, "QC_AGENT_IDLE_MINUTES is set twice"):
            cfg.parse_env_text("QC_AGENT_IDLE_MINUTES=1\nQC_AGENT_IDLE_MINUTES=2\n")
        with self.assertRaisesRegex(cfg.ConfigError, "expected NAME=value"):
            cfg.parse_env_text("not a line\n")

    def test_environment_does_not_override_the_file(self):
        with mock.patch.dict(os.environ, {"DEVICE_JOB_KEY": NEXT_HEX}):
            self.assertEqual(cfg.from_mapping(env()).current_kid, "72eb33c6")

    def test_rotation(self):
        c = cfg.from_mapping(env(DEVICE_JOB_KEY_NEXT=NEXT_HEX))
        self.assertEqual(len(c.keys), 2)
        self.assertEqual(c.signing_key, bytes(range(32)))

    def test_repr_redacts(self):
        c = cfg.from_mapping(env())
        r = repr(c)
        for secret in ("secret/EXAMPLE+value0123456789", KEY_HEX, "AKIDEXAMPLE0123456789"):
            self.assertNotIn(secret, r)


class Slots(unittest.TestCase):
    def test_geometry(self):
        self.assertEqual((position("1A"), position("2A"), position("3A"), position("20A"), position("32H")),
                         (0, 8, 16, 152, 255))
        self.assertEqual(label(255), "32H")

    def test_ranges(self):
        r = SlotRanges.parse("20a-25h,30A")
        self.assertTrue(r.contains("20A") and r.contains("25H") and r.contains("30A"))
        self.assertFalse(r.contains("19H") or r.contains("26A") or r.contains("30B"))
        self.assertFalse(r.contains("20A\n"))
        self.assertEqual(r.labels(), ["20A-25H", "30A"])
        with self.assertRaisesRegex(ValueError, "ranges such as 20A-25H"):
            SlotRanges.parse(",".join(["1A"] * 17))


if __name__ == "__main__":
    unittest.main()
