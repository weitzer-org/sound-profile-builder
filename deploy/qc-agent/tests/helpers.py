"""Shared test helpers: paths, keys, a fixed clock, signed jobs and P2's golden exports."""
from __future__ import annotations

import copy
import datetime as dt
import json
import sys
import uuid
from pathlib import Path

AGENT_DIR = Path(__file__).resolve().parents[1]
REPO = AGENT_DIR.parents[1]
if str(AGENT_DIR) not in sys.path:
    sys.path.insert(0, str(AGENT_DIR))

from qc_agent import contract  # noqa: E402
from qc_agent.catalog import Catalog  # noqa: E402
from qc_agent.config import from_mapping  # noqa: E402

VECTORS = REPO / "testdata" / "devicejobs" / "vectors.json"
KEY_HEX = bytes(range(32)).hex()          # the vectors' current key (kid 72eb33c6)
NEXT_HEX = bytes(range(32, 64)).hex()
BUCKET = "sound-profile-builder-device-jobs"
T0 = dt.datetime(2026, 11, 5, 20, 0, 0, tzinfo=dt.timezone.utc)

_catalog = None


def catalog():
    global _catalog
    if _catalog is None:
        _catalog = Catalog.load()
    return _catalog


def env(**over):
    e = {
        "QC_AGENT_R2_ENDPOINT": "https://0123456789abcdef.r2.cloudflarestorage.com",
        "QC_AGENT_R2_BUCKET": BUCKET,
        "QC_AGENT_R2_ACCESS_KEY_ID": "TEST-ACCESS-KEY-ID-NOT-REAL",
        "QC_AGENT_R2_SECRET_ACCESS_KEY": "TEST-SECRET-ACCESS-KEY-NOT-REAL",
        "DEVICE_JOB_KEY": KEY_HEX,
        "QC_AGENT_ALLOWED_SLOTS": "1A-32H",
        "QC_AGENT_EXPECTED_FIRMWARE": "4.1.0",
        "QC_AGENT_IDLE_MINUTES": "30",
    }
    e.update(over)
    return {k: v for k, v in e.items() if v is not None}


def config(**over):
    return from_mapping(env(**over))


class Clock:
    def __init__(self, t=T0):
        self.t = t

    def __call__(self):
        return self.t

    def advance(self, seconds):
        self.t += dt.timedelta(seconds=seconds)


GOLDEN = REPO / "internal" / "deviceexport" / "testdata" / "golden"


def golden(name="pushable_g3"):
    """One of P2's golden exports (internal/deviceexport, regenerated with `go test
    ./internal/deviceexport -run Golden -update`), read as written: a P2 change that the agent
    doesn't accept fails these tests."""
    return json.loads((GOLDEN / f"{name}.json").read_text(encoding="utf-8"))


def export(name="pushable_g3"):
    """The job's args.export for a golden: {"format", "payload"}, exactly what QC-2 queues."""
    g = golden(name)
    return {"format": g["format"], "payload": g["payload"]}


def job(kind="list_slots", args=None, job_id=None, created=T0, ttl_s=900, bucket=BUCKET, key_hex=KEY_HEX):
    jid = job_id or str(uuid.uuid4())
    if args is None:
        args = {"list_slots": {}, "read_preset": {"slot": None},
                "push_preset": {"slot": "20A", "name": "Texas Blues",
                                "source": {"preset_id": "p-123", "guitar": "Telecaster"},
                                "export": export()}}[kind]
    payload = {"v": 1, "type": "qc2-device-job", "job_id": jid, "bucket": bucket, "kind": kind,
               "created_at": contract.format_time(created),
               "expires_at": contract.format_time(created + dt.timedelta(seconds=ttl_s)),
               "args": copy.deepcopy(args)}
    data = contract.seal(bytes.fromhex(key_hex), contract.TAG_JOB, json.dumps(payload))
    return jid, f"jobs/pending/{jid}.json", data


def open_result(store, jid, key_hex=KEY_HEX):
    data = store.objects[f"jobs/done/{jid}.json"]
    keys = contract.keyring(key_hex)
    return json.loads(contract.open_envelope(keys, contract.TAG_RESULT, data))


import logging  # noqa: E402

_log = logging.getLogger("qc_agent")
_log.addHandler(logging.NullHandler())
_log.propagate = False
