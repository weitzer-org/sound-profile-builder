"""The command line (qc_agent/__main__.py): routing and the startup refusals, without a unit or a bucket."""
import contextlib
import io
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from helpers import KEY_HEX
from qc_agent import __main__ as cli
from qc_agent.lock import InstanceLock
from qc_agent.vendor_pins import COMMIT

ENV = f"""QC_AGENT_R2_ENDPOINT=https://abc.r2.cloudflarestorage.com
QC_AGENT_R2_BUCKET=sound-profile-builder-device-jobs
QC_AGENT_R2_ACCESS_KEY_ID=AKID
QC_AGENT_R2_SECRET_ACCESS_KEY=TEST-SECRET-NOT-REAL-7f3a
DEVICE_JOB_KEY={KEY_HEX}
QC_AGENT_ALLOWED_SLOTS=1A-32H
QC_AGENT_EXPECTED_FIRMWARE=4.1.0
QC_AGENT_IDLE_MINUTES=30
"""


def run(argv):
    out = io.StringIO()
    with contextlib.redirect_stdout(out), mock.patch.object(cli, "_pause", lambda msg="", *a, **k: print(msg)):
        code = cli.main(argv)
    return code, out.getvalue()


class Main(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.home = Path(self.tmp.name)

    def test_routing(self):
        for argv, fn in ((["run"], "cmd_run"), (["check"], "cmd_check"),
                         (["install-shortcut"], "cmd_install"), (["uninstall-shortcut"], "cmd_uninstall")):
            with self.subTest(argv[0]), mock.patch.object(cli, fn, return_value=7) as m:
                with mock.patch.dict(cli.__dict__):
                    self.assertEqual(cli.main(argv), 7)
                    m.assert_called_once()

    def test_unknown_command_exits(self):
        with self.assertRaises(SystemExit), contextlib.redirect_stderr(io.StringIO()):
            cli.main(["serve"])

    def test_check_reports_config_failure(self):
        code, out = run(["check", "--home", str(self.home)])
        self.assertEqual(code, 1)
        self.assertIn("config: FAILED", out)
        self.assertIn(f"vendored qc-mcp {COMMIT[:7]}: OK", out)

    def test_check_passes_without_echoing_secrets(self):
        (self.home / "agent.env").write_text(ENV)
        code, out = run(["check", "--home", str(self.home)])
        self.assertEqual(code, 0, out)
        self.assertIn("all checks passed", out)
        self.assertNotIn("TEST-SECRET-NOT-REAL-7f3a", out)
        self.assertNotIn(KEY_HEX, out)

    def test_run_refuses_bad_config(self):
        (self.home / "agent.env").write_text("QC_AGENT_IDLE_MINUTES=0\n")
        code, out = run(["run", "--home", str(self.home)])
        self.assertEqual(code, cli.EXIT_CONFIG)
        self.assertIn("configuration problem", out)

    def test_run_refuses_a_second_instance(self):
        (self.home / "agent.env").write_text(ENV)
        with InstanceLock(self.home / "agent.lock"):
            code, out = run(["run", "--home", str(self.home)])
        self.assertEqual(code, cli.EXIT_RUNNING)
        self.assertIn("already running", out)

    def test_run_wires_the_agent(self):
        (self.home / "agent.env").write_text(ENV)
        with mock.patch("qc_agent.qcmcp_backend.QcMcpBackend") as backend, \
                mock.patch("qc_agent.agent.Agent.run", return_value=0) as agent_run, \
                mock.patch.object(cli, "setup_logging"):
            code, _ = run(["run", "--home", str(self.home)])
        self.assertEqual(code, 0)
        backend.assert_called_once()
        agent_run.assert_called_once()


if __name__ == "__main__":
    unittest.main()
