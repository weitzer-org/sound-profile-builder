"""The agent's authoritative local records (contract sections 2 and 8).

executed_jobs.jsonl: one JSON object per line, appended and fsynced, never pruned.
  {"event": "taken", "job_id", "kind", "at"}         before any device contact (step 10)
  {"event": "finished", "job_id", "at", "result"}    the signed result envelope text (step 12)
  {"event": "uploaded", "job_id", "at"}              the result reached jobs/done/ (so a restart
                                                     needn't HEAD every job the agent ever ran)

owned_slots.jsonl: one line per save the agent issued:
  {"slot", "position", "name", "job_id", "at"}
A slot counts as "owned" while the unit still shows the name the agent saved there.
Nothing here is decided from the bucket: the bucket's jobs/owned_slots.json is an
informational copy that anyone with the bucket token could overwrite.
"""
from __future__ import annotations

import json
import os
from pathlib import Path


class LedgerError(RuntimeError):
    pass


TORN = {"event": "torn"}  # written after a torn line, so the next read knows it was a crash, not corruption


def _parse(line: bytes):
    try:
        obj = json.loads(line.decode("utf-8"))
    except (ValueError, UnicodeDecodeError):
        return None
    return obj if isinstance(obj, dict) else None


def _read_lines(path: Path) -> list[dict]:
    """Entries in order. A torn line (an append cut short by a crash before its fsync) is
    skipped when it is the last line, or when the agent has since marked it with a TORN
    line; anything else unparseable is corruption and stops the agent."""
    if not path.exists():
        return []
    lines = [ln for ln in path.read_bytes().split(b"\n") if ln.strip()]
    out = []
    for n, line in enumerate(lines):
        obj = _parse(line)
        if obj is None:
            nxt = _parse(lines[n + 1]) if n + 1 < len(lines) else TORN
            if nxt == TORN:
                continue
            raise LedgerError(f"{path}: entry {n + 1} is corrupt; fix or move it aside by hand "
                              "(the agent never rewrites this file)")
        if obj != TORN:
            out.append(obj)
    return out


def _append(path: Path, obj: dict) -> None:
    data = json.dumps(obj, separators=(",", ":"), ensure_ascii=True).encode("ascii") + b"\n"
    path.parent.mkdir(parents=True, exist_ok=True)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_APPEND | getattr(os, "O_BINARY", 0), 0o600)
    try:
        size = os.fstat(fd).st_size
        if size:
            # If the previous append was torn, start on a fresh line.
            with open(path, "rb") as f:
                f.seek(size - 1)
                if f.read(1) != b"\n":
                    data = b"\n" + json.dumps(TORN).encode("ascii") + b"\n" + data
        os.write(fd, data)
        os.fsync(fd)
    finally:
        os.close(fd)


class JobLedger:
    def __init__(self, path: Path):
        self.path = path
        self.taken: dict[str, dict] = {}
        self.finished: dict[str, dict] = {}
        self.uploaded: set[str] = set()
        self.order: list[str] = []
        for e in _read_lines(path):
            jid = e.get("job_id")
            if not isinstance(jid, str):
                raise LedgerError(f"{path}: an entry has no job_id")
            if e.get("event") == "taken":
                if jid not in self.taken:
                    self.order.append(jid)
                self.taken[jid] = e
            elif e.get("event") == "finished":
                if not isinstance(e.get("result"), str):
                    raise LedgerError(f"{path}: finished entry for {jid} has no result")
                self.finished[jid] = e
            elif e.get("event") == "uploaded":
                self.uploaded.add(jid)
            else:
                raise LedgerError(f"{path}: unknown event {e.get('event')!r}")

    def contains(self, job_id: str) -> bool:
        return job_id in self.taken or job_id in self.finished

    def take(self, job_id: str, kind: str, at: str) -> None:
        e = {"event": "taken", "job_id": job_id, "kind": kind, "at": at}
        _append(self.path, e)
        self.taken[job_id] = e
        self.order.append(job_id)

    def finish(self, job_id: str, at: str, result: bytes) -> None:
        e = {"event": "finished", "job_id": job_id, "at": at, "result": result.decode("ascii")}
        _append(self.path, e)
        self.finished[job_id] = e

    def mark_uploaded(self, job_id: str, at: str) -> None:
        if job_id in self.uploaded:
            return
        _append(self.path, {"event": "uploaded", "job_id": job_id, "at": at})
        self.uploaded.add(job_id)

    def not_uploaded(self) -> list[str]:
        return [j for j in self.order if j in self.finished and j not in self.uploaded]

    def unfinished(self) -> list[str]:
        return [j for j in self.order if j not in self.finished]


class OwnedSlots:
    def __init__(self, path: Path):
        self.path = path
        self.entries = _read_lines(path)

    def record(self, slot: str, position: int, name: str, job_id: str, at: str) -> None:
        e = {"slot": slot, "position": position, "name": name, "job_id": job_id, "at": at}
        _append(self.path, e)
        self.entries.append(e)

    def owned(self, position: int, current_name: str) -> bool:
        latest = None
        for e in self.entries:
            if e.get("position") == position:
                latest = e
        return bool(latest) and bool(current_name) and latest.get("name") == current_name

    def current(self) -> list[dict]:
        latest: dict[int, dict] = {}
        for e in self.entries:
            if isinstance(e.get("position"), int):
                latest[e["position"]] = e
        return [latest[p] for p in sorted(latest)]
