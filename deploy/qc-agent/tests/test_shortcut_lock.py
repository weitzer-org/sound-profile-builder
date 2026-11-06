"""Desktop shortcut bytes, install/uninstall, the single-instance lock and the lockfile."""
import re
import struct
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from helpers import AGENT_DIR, REPO
from qc_agent import shortcut
from qc_agent.lock import InstanceLock

EXE = r"C:\Users\example\AppData\Local\qc-agent\venv\Scripts\python.exe"
WD = r"C:\Users\example\src\qc2\deploy\qc-agent"


class Lnk(unittest.TestCase):
    def test_round_trip(self):
        data = shortcut.build_lnk(EXE, "-E -s -m qc_agent run", WD)
        info = shortcut.parse_lnk(data)
        self.assertEqual(info["target"], EXE)
        self.assertEqual(info["env_target"], EXE)
        self.assertEqual(info["arguments"], "-E -s -m qc_agent run")
        self.assertEqual(info["working_dir"], WD)
        self.assertEqual(info["show"], 1)
        self.assertEqual(struct.unpack_from("<I", data)[0], 0x4C)

    def test_header_layout(self):
        data = shortcut.build_lnk(EXE, "a", WD)
        self.assertEqual(data[4:20].hex(), "0114020000000000c000000000000046")  # CLSID_ShellLink, little-endian
        flags = struct.unpack_from("<I", data, 20)[0]
        self.assertEqual(flags & 0x1, 0, "no IDList")
        self.assertTrue(flags & 0x80, "Unicode strings")
        self.assertTrue(data.endswith(b"\x00\x00\x00\x00"))

    def test_non_ascii_path(self):
        exe = r"C:\Users\Zoë\venv\Scripts\python.exe"
        self.assertEqual(shortcut.parse_lnk(shortcut.build_lnk(exe, "x", WD))["target"], exe)

    def test_relative_target_refused(self):
        with self.assertRaises(ValueError):
            shortcut.build_lnk("python.exe", "x", WD)

    def test_install_and_uninstall(self):
        with tempfile.TemporaryDirectory() as d:
            desk = Path(d)
            p = shortcut.install(python_exe=EXE, desktop=desk)
            self.assertEqual(p.name, "Start QC bridge.lnk")
            info = shortcut.parse_lnk(p.read_bytes())
            self.assertIn("-m qc_agent run", info["arguments"])
            self.assertEqual(info["working_dir"], str(AGENT_DIR))
            self.assertEqual(shortcut.uninstall(desktop=desk), p)
            self.assertFalse(p.exists())
            self.assertIsNone(shortcut.uninstall(desktop=desk))

    def test_uninstall_leaves_foreign_shortcuts(self):
        for args, wd in (("-m something_else", str(AGENT_DIR)), ("-E -s -m qc_agent runner", str(AGENT_DIR)),
                         (shortcut.ARGUMENTS, WD)):  # another checkout's shortcut is not ours either
            with self.subTest(args=args, wd=wd), tempfile.TemporaryDirectory() as d:
                p = Path(d) / shortcut.SHORTCUT_NAME
                p.write_bytes(shortcut.build_lnk(EXE, args, wd))
                with self.assertRaises(RuntimeError):
                    shortcut.uninstall(desktop=Path(d))
                self.assertTrue(p.exists())

    def test_install_never_replaces_a_foreign_shortcut(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / shortcut.SHORTCUT_NAME
            p.write_bytes(b"not ours")
            with self.assertRaises(RuntimeError):
                shortcut.install(python_exe=EXE, desktop=Path(d))
            self.assertEqual(p.read_bytes(), b"not ours")
            p.unlink()
            shortcut.install(python_exe=EXE, desktop=Path(d))
            shortcut.install(python_exe=EXE, desktop=Path(d))  # our own: replaced in place

    def test_utf16_length_is_what_counts(self):
        emoji_path = "C:\\" + "\U0001F3B8" * 129 + "\\p.exe"  # < 260 characters, > 260 UTF-16 units
        self.assertLess(len(emoji_path), 260)
        with self.assertRaises(ValueError):
            shortcut.build_lnk(emoji_path, "x", WD)

    def test_no_scheduler_service_or_powershell(self):
        # The agent never shells out (so no schtasks/PowerShell/sc.exe), and never writes autostart entries.
        for p in (AGENT_DIR / "qc_agent").glob("*.py"):
            src = p.read_text(encoding="utf-8")
            with self.subTest(p.name):
                self.assertNotRegex(src, r"^\s*(import|from)\s+subprocess", "no subprocess")
                self.assertNotRegex(src, r"os\.system|os\.popen|startfile|ShellExecute")
                self.assertNotIn("CurrentVersion\\Run", src)
                self.assertNotIn("Start Menu\\Programs\\Startup", src)


class Lock(unittest.TestCase):
    def test_second_instance_refused(self):
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / "agent.lock"
            with InstanceLock(path):
                code = ("import sys; sys.path.insert(0, sys.argv[1]);"
                        "from qc_agent.lock import InstanceLock, AlreadyRunning\n"
                        "try:\n InstanceLock(__import__('pathlib').Path(sys.argv[2])).acquire(); print('got')\n"
                        "except AlreadyRunning: print('busy')")
                out = subprocess.run([sys.executable, "-c", code, str(AGENT_DIR), str(path)],
                                     capture_output=True, text=True, timeout=30).stdout.strip()
                self.assertEqual(out, "busy")
            with InstanceLock(path):
                pass  # released: free again


class LockFile(unittest.TestCase):
    def test_every_requirement_is_hash_pinned(self):
        text = (AGENT_DIR / "requirements.lock.txt").read_text()
        reqs = [ln for ln in text.splitlines() if ln and not ln.startswith((" ", "#"))]
        self.assertEqual(reqs, ["protobuf==7.36.0 \\"])
        hashes = re.findall(r"--hash=sha256:([0-9a-f]{64})", text)
        self.assertEqual(len(hashes), 8)

    def test_same_pins_as_the_reviewed_qc_mcp_lock(self):
        trial = (REPO / "tools/qc-mcp-trial/qc-mcp-f634ecd.requirements.txt").read_text()
        block = re.search(r"^protobuf==7\.36\.0 \\\n((?:    --hash=sha256:[0-9a-f]{64}(?: \\)?\n)+)", trial, re.M)
        self.assertIsNotNone(block)
        ours = (AGENT_DIR / "requirements.lock.txt").read_text()
        self.assertEqual(sorted(re.findall(r"[0-9a-f]{64}", block.group(1))),
                         sorted(re.findall(r"sha256:([0-9a-f]{64})", ours)))


if __name__ == "__main__":
    unittest.main()
