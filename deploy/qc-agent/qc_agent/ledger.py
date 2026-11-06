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
TORN_TEXT = json.dumps(TORN).encode("ascii")


def _parse(line: bytes):
    try:
        obj = json.loads(line.decode("utf-8"))
    except (ValueError, UnicodeDecodeError):
        return None
    return obj if isinstance(obj, dict) else None


def _read_lines(path: Path) -> tuple[list[dict], bool]:
    """(entries in order, whether the file ends in a torn tail).

    A torn line is an append cut short by a crash before its fsync, so it never counted. It is
    accepted only as the tail of the file, optionally followed by a cut-short marker (a prefix of
    TORN_TEXT: a crash while marking it), or when a full TORN marker follows it. Anything else
    unparseable, or a real entry after a bad line, is corruption and stops the agent."""
    if not path.exists():
        return [], False
    out = []
    bad_at = None  # entry number of a torn line not yet followed by a TORN marker
    n = 0
    with open(path, "rb") as f:  # streamed: only the parsed entries are kept
        for line in f:
            text = line.strip()
            if not text:
                continue
            n += 1
            obj = _parse(text)
            if bad_at is not None:
                if obj == TORN:
                    bad_at = None
                    continue
                if obj is None and TORN_TEXT.startswith(text):
                    continue  # a marker that was itself cut short
                raise LedgerError(f"{path}: entry {bad_at} is corrupt; fix or move it aside by hand "
                                  "(the agent never rewrites this file)")
            if obj is None:
                bad_at = n
            elif obj != TORN:
                out.append(obj)
    return out, bad_at is not None


def _append(path: Path, obj: dict, torn_tail: bool = False) -> None:
    """Append one fsynced line. After a torn tail (known from the load, or a file not ending in a
    newline) a TORN marker goes first, so the torn line is never followed by an unmarked entry."""
    data = json.dumps(obj, separators=(",", ":"), ensure_ascii=True).encode("ascii") + b"\n"
    path.parent.mkdir(parents=True, exist_ok=True)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_APPEND | getattr(os, "O_BINARY", 0), 0o600)
    try:
        size = os.fstat(fd).st_size
        lead = b""
        if size:
            with open(path, "rb") as f:
                f.seek(size - 1)
                if f.read(1) != b"\n":
                    lead, torn_tail = b"\n", True
        if torn_tail:
            data = lead + TORN_TEXT + b"\n" + data
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
        entries, self._torn_tail = _read_lines(path)
        for e in entries:
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
        for jid in self.uploaded:  # an uploaded result is never needed again; keep the line, drop the body
            if jid in self.finished:
                self.finished[jid] = {**self.finished[jid], "result": None}

    def contains(self, job_id: str) -> bool:
        return job_id in self.taken or job_id in self.finished

    def take(self, job_id: str, kind: str, at: str) -> None:
        e = {"event": "taken", "job_id": job_id, "kind": kind, "at": at}
        _append(self.path, e, self._torn_tail)
        self._torn_tail = False
        self.taken[job_id] = e
        self.order.append(job_id)

    def finish(self, job_id: str, at: str, result: bytes) -> None:
        e = {"event": "finished", "job_id": job_id, "at": at, "result": result.decode("ascii")}
        _append(self.path, e, self._torn_tail)
        self._torn_tail = False
        self.finished[job_id] = e

    def mark_uploaded(self, job_id: str, at: str) -> None:
        if job_id in self.uploaded:
            return
        _append(self.path, {"event": "uploaded", "job_id": job_id, "at": at}, self._torn_tail)
        self._torn_tail = False
        self.uploaded.add(job_id)
        if job_id in self.finished:
            self.finished[job_id] = {**self.finished[job_id], "result": None}

    def not_uploaded(self) -> list[str]:
        return [j for j in self.order if j in self.finished and j not in self.uploaded]

    def unfinished(self) -> list[str]:
        return [j for j in self.order if j not in self.finished]


class OwnedSlots:
    def __init__(self, path: Path):
        self.path = path
        self._latest: dict[int, dict] = {}  # position -> the most recent save there
        entries, self._torn_tail = _read_lines(path)
        for e in entries:
            if isinstance(e.get("position"), int):
                self._latest[e["position"]] = e

    def record(self, slot: str, position: int, name: str, job_id: str, at: str) -> None:
        e = {"slot": slot, "position": position, "name": name, "job_id": job_id, "at": at}
        _append(self.path, e, self._torn_tail)
        self._torn_tail = False
        self._latest[position] = e

    def owned(self, position: int, current_name: str) -> bool:
        latest = self._latest.get(position)
        return bool(latest) and bool(current_name) and latest.get("name") == current_name

    def current(self) -> list[dict]:
        return [self._latest[p] for p in sorted(self._latest)]
