"""The agent's version string names the commit it started from (qc_agent/version.py)."""
import re
import tempfile
import unittest
from pathlib import Path

import qc_agent
from qc_agent.version import NO_GIT, agent_version, git_commit

SHA = "be0f0a691413be618f697393df305be3386202de"
PATTERN = re.compile(r"^[0-9A-Za-z.+-]{1,32}$")  # contract: agent_version


def repo(files: dict[str, str]) -> Path:
    root = Path(tempfile.mkdtemp())
    for name, body in files.items():
        p = root / ".git" / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(body, encoding="ascii")
    return root


class Version(unittest.TestCase):
    def test_this_checkout_fits_the_contract(self):
        self.assertTrue(PATTERN.match(qc_agent.AGENT_VERSION), qc_agent.AGENT_VERSION)
        self.assertTrue(qc_agent.AGENT_VERSION.startswith("0.1.0+"))

    def test_loose_ref(self):
        r = repo({"HEAD": "ref: refs/heads/main\n", "refs/heads/main": SHA + "\n"})
        self.assertEqual(git_commit(r), "be0f0a6")
        self.assertEqual(agent_version(r), "0.1.0+be0f0a6")

    def test_packed_ref(self):
        r = repo({"HEAD": "ref: refs/heads/main\n",
                  "packed-refs": f"# pack-refs\n{'a' * 40} refs/heads/other\n{SHA} refs/heads/main\n"})
        self.assertEqual(git_commit(r), "be0f0a6")

    def test_detached_head(self):
        self.assertEqual(git_commit(repo({"HEAD": SHA + "\n"})), "be0f0a6")

    def test_not_a_checkout_or_unreadable(self):
        cases = {
            "no .git": Path(tempfile.mkdtemp()),
            "unborn branch": repo({"HEAD": "ref: refs/heads/main\n"}),
            "garbage head": repo({"HEAD": "nonsense\n"}),
            "traversal ref": repo({"HEAD": "ref: refs/../../etc/passwd\n"}),
            "short sha": repo({"HEAD": "ref: refs/heads/main\n", "refs/heads/main": "abc123\n"}),
            "upper-case sha": repo({"HEAD": SHA.upper() + "\n"}),
        }
        for name, root in cases.items():
            with self.subTest(name):
                self.assertIsNone(git_commit(root))
                self.assertEqual(agent_version(root), f"0.1.0+{NO_GIT}")
                self.assertTrue(PATTERN.match(agent_version(root)))


if __name__ == "__main__":
    unittest.main()
