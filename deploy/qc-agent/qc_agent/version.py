"""The agent's version string: the release number plus the commit of the checkout it started from.

`AGENT_VERSION` used to be a constant that never changed, so `device_status` and the start-up line
said the same thing for old and new code. The commit is read once, at import, from the clone's
`.git` directory with plain file reads (it never runs git and writes nothing), so it names the
code this process actually loaded: a `git pull` made while the bridge runs does not change it, and
a restart does. The result fits the contract's `agent_version` pattern `^[0-9A-Za-z.+-]{1,32}$`
(semver build metadata), so no contract change.
"""
from __future__ import annotations

import re
from pathlib import Path

BASE_VERSION = "0.1.0"
NO_GIT = "nogit"  # not a git checkout, or HEAD couldn't be read

_SHA = re.compile(r"[0-9a-f]{40}")
_REF = re.compile(r"refs/[A-Za-z0-9._/-]{1,200}")


def git_commit(repo_root: Path) -> str | None:
    """The 7-character commit that `repo_root`'s HEAD points at, or None. Reads .git/HEAD, then the
    loose ref or packed-refs. Anything unexpected (a worktree's `.git` file, a malformed ref) is None."""
    try:
        git = repo_root / ".git"
        if not git.is_dir():
            return None
        head = (git / "HEAD").read_text(encoding="ascii").strip()
        if _SHA.fullmatch(head):
            return head[:7]
        if not head.startswith("ref: "):
            return None
        ref = head[5:].strip()
        if not _REF.fullmatch(ref) or ".." in ref:
            return None
        loose = git / ref
        if loose.is_file():
            sha = loose.read_text(encoding="ascii").strip()
        else:
            sha = ""
            packed = git / "packed-refs"
            if packed.is_file():
                for line in packed.read_text(encoding="ascii").splitlines():
                    parts = line.split(" ", 1)
                    if len(parts) == 2 and parts[1].strip() == ref:
                        sha = parts[0].strip()
                        break
        return sha[:7] if _SHA.fullmatch(sha) else None
    except (OSError, UnicodeDecodeError):
        return None


def agent_version(repo_root: Path | None = None) -> str:
    """`<release>+<commit>` for the checkout this file sits in (or `repo_root`), `<release>+nogit` otherwise."""
    root = repo_root if repo_root is not None else Path(__file__).resolve().parents[3]
    return f"{BASE_VERSION}+{git_commit(root) or NO_GIT}"
