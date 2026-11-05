"""Python reference for docs/design/device-job-contract.md (stdlib only).

An independent implementation of the contract's envelope rules (section 5) and
the agent's check order for a pending job (section 8, steps 1-9), written
from the document, not from the Go code. It must reach the same outcome as
internal/devicejobs for every entry in vectors.json:

    python3 -I testdata/devicejobs/reference_check.py testdata/devicejobs/vectors.json

The home agent (plan P4) may import or copy it. Changing behaviour here is a
contract change (contract section 1).
"""

import hashlib
import hmac
import json
import math
import re
import sys
from datetime import datetime, timezone

MAX_JOB_BYTES = 512 * 1024
MAX_DEPTH = 32
MAX_CREATED_AHEAD_S = 5 * 60
MAX_LIFETIME_S = 24 * 3600

# Always use .fullmatch(): with .match(), "$" also matches before a trailing
# newline. Write [0-9], not \d, which matches non-ASCII digits in Python.
KEY_RE = re.compile(r"[0-9a-fA-F]{64}")
KID_RE = re.compile(r"[0-9a-f]{8}")
SIG_RE = re.compile(r"[0-9a-f]{64}")
UUID = r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
UUID_RE = re.compile(UUID)
PENDING_RE = re.compile(r"jobs/pending/(" + UUID + r")\.json")
TIME_RE = re.compile(r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z")
SLOT_RE = re.compile(r"(?:[1-9]|[12][0-9]|3[0-2])[A-H]")
BUCKET_RE = re.compile(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]")
FORMAT_RE = re.compile(r"[a-z0-9][a-z0-9.-]{0,40}/[0-9]{1,4}")

TAG_JOB = "qc2-device-job/v1"
TAG_RESULT = "qc2-device-result/v1"
TAG_HEARTBEAT = "qc2-device-heartbeat/v1"


class Reject(Exception):
    def __init__(self, outcome, detail=""):
        super().__init__(outcome + (": " + detail if detail else ""))
        self.outcome = outcome


# ---- keys ----

def parse_key(s):
    s = s.strip()
    if not KEY_RE.fullmatch(s):  # bytes.fromhex alone accepts inner spaces
        raise ValueError("DEVICE_JOB_KEY must be exactly 64 hex characters")
    return bytes.fromhex(s)


def kid(key):
    return hashlib.sha256(b"qc2-device-kid/v1\n" + key).hexdigest()[:8]


def keyring(current_hex, next_hex=None):
    """{kid: key_bytes} for DEVICE_JOB_KEY and the optional DEVICE_JOB_KEY_NEXT.

    Same rules as Go's NewKeyring: the current key is required; a NEXT equal
    to it is ignored; a different NEXT with the same kid is refused."""
    if not current_hex or not current_hex.strip():
        raise ValueError("DEVICE_JOB_KEY must be set; DEVICE_JOB_KEY_NEXT requires a valid current key")
    cur = parse_key(current_hex)
    out = {kid(cur): cur}
    if next_hex and next_hex.strip():
        nxt = parse_key(next_hex)
        if kid(nxt) in out and nxt != cur:
            raise ValueError("DEVICE_JOB_KEY_NEXT: its key id collides with DEVICE_JOB_KEY's; generate another key")
        out.setdefault(kid(nxt), nxt)
    return out


def sign(key, tag, payload):
    return hmac.new(key, tag.encode("ascii") + b"\n" + payload.encode("utf-8"), hashlib.sha256).hexdigest()


def seal(key, tag, payload):
    """Envelope bytes for payload (a str of JSON text) under key."""
    return json.dumps({"v": 1, "kid": kid(key), "payload": payload, "sig": sign(key, tag, payload)}).encode("utf-8")


# ---- strict JSON ----

def _max_depth(text):
    """Deepest [/{ nesting outside strings, without recursion."""
    depth = deepest = 0
    in_str = esc = False
    for c in text:
        if in_str:
            if esc:
                esc = False
            elif c == "\\":
                esc = True
            elif c == '"':
                in_str = False
        elif c == '"':
            in_str = True
        elif c in "[{":
            depth += 1
            deepest = max(deepest, depth)
        elif c in "]}":
            depth -= 1
    return deepest


def _no_dupes(pairs):
    d = {}
    for k, v in pairs:
        if k in d:
            raise ValueError("duplicate key")
        d[k] = v
    return d


def _no_constant(_):
    raise ValueError("NaN/Infinity")


def _finite_float(s):
    f = float(s)
    if not math.isfinite(f):  # 1e400 parses to inf without parse_constant
        raise ValueError("number out of range")
    return f


def _finite_int(s):
    if not math.isfinite(float(s)):  # Go reads every number as a float64
        raise ValueError("number out of range")
    return int(s)


def _has_surrogate(v):
    if isinstance(v, str):
        return any("\ud800" <= c <= "\udfff" for c in v)
    if isinstance(v, dict):
        return any(_has_surrogate(k) or _has_surrogate(x) for k, x in v.items())
    if isinstance(v, list):
        return any(_has_surrogate(x) for x in v)
    return False


def strict_loads(text):
    """json.loads with the contract's rules. Raises ValueError on any violation."""
    if _max_depth(text) > MAX_DEPTH:
        raise ValueError("nested too deeply")
    try:
        v = json.loads(text, object_pairs_hook=_no_dupes, parse_constant=_no_constant,
                       parse_float=_finite_float, parse_int=_finite_int)
    except RecursionError:
        raise ValueError("nested too deeply")
    if _has_surrogate(v):
        raise ValueError("unpaired surrogate")
    return v


# ---- envelope (contract section 5) ----

def open_envelope(keys, tag, data):
    """Verify envelope bytes under tag; return the payload str."""
    try:
        env = strict_loads(data.decode("utf-8"))
    except (UnicodeDecodeError, ValueError):
        raise Reject("bad_envelope")
    if (not isinstance(env, dict) or set(env) != {"v", "kid", "payload", "sig"}
            or type(env["v"]) is not int or env["v"] != 1
            or not isinstance(env["kid"], str) or not KID_RE.fullmatch(env["kid"])
            or not isinstance(env["sig"], str) or not SIG_RE.fullmatch(env["sig"])
            or not isinstance(env["payload"], str)):
        raise Reject("bad_envelope")
    try:
        payload_bytes = env["payload"].encode("utf-8")  # unpaired surrogate -> error
    except UnicodeEncodeError:
        raise Reject("bad_envelope")
    key = keys.get(env["kid"])
    if key is None:
        raise Reject("unknown_kid")
    want = hmac.new(key, tag.encode("ascii") + b"\n" + payload_bytes, hashlib.sha256).hexdigest()
    if not hmac.compare_digest(want, env["sig"]):
        raise Reject("bad_signature")
    return env["payload"]


# ---- job (contract sections 6-8) ----

def parse_time(s):
    if not isinstance(s, str) or not TIME_RE.fullmatch(s):
        raise ValueError("bad timestamp")
    return datetime.strptime(s, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)  # year 0 -> ValueError


def _exact(obj, *keys):
    return isinstance(obj, dict) and set(obj) == set(keys)


def _valid_name(s):
    return (isinstance(s, str) and 1 <= len(s) <= 32 and s.strip() == s
            and all(0x20 <= ord(c) <= 0x7E for c in s))


def _check_args(kind, a):
    if kind == "list_slots":
        return _exact(a)
    if kind == "read_preset":
        return _exact(a, "slot") and (a["slot"] is None or (isinstance(a["slot"], str) and SLOT_RE.fullmatch(a["slot"])))
    if kind == "push_preset":
        return (_exact(a, "slot", "name", "source", "export")
                and isinstance(a["slot"], str) and bool(SLOT_RE.fullmatch(a["slot"]))
                and _valid_name(a["name"])
                and _exact(a["source"], "preset_id", "guitar")
                and isinstance(a["source"]["preset_id"], str) and isinstance(a["source"]["guitar"], str)
                and _exact(a["export"], "format", "payload")
                and isinstance(a["export"]["format"], str) and bool(FORMAT_RE.fullmatch(a["export"]["format"]))
                and isinstance(a["export"]["payload"], dict))
    return False


def parse_job(payload):
    try:
        j = strict_loads(payload)
    except ValueError:
        raise Reject("bad_payload")
    if not _exact(j, "v", "type", "job_id", "bucket", "kind", "created_at", "expires_at", "args"):
        raise Reject("bad_payload", "key set")
    if type(j["v"]) is not int or j["v"] != 1 or j["type"] != "qc2-device-job":
        raise Reject("bad_payload", "version/type")
    if not (isinstance(j["job_id"], str) and UUID_RE.fullmatch(j["job_id"])
            and isinstance(j["bucket"], str) and BUCKET_RE.fullmatch(j["bucket"])
            and isinstance(j["kind"], str)):
        raise Reject("bad_payload", "ids")
    try:
        parse_time(j["created_at"])
        parse_time(j["expires_at"])
    except ValueError:
        raise Reject("bad_payload", "timestamp")
    if not _check_args(j["kind"], j["args"]):
        raise Reject("bad_payload", "args")
    return j


def check_pending_job(keys, object_key, agent_bucket, now, data, seen):
    """Steps 1-9 of contract section 8. Returns the job dict on "ok"; raises
    Reject otherwise. On "expired" the job is attached as .job, because the
    agent writes a signed expired result for it."""
    m = PENDING_RE.fullmatch(object_key)                      # 1
    if not m:
        raise Reject("bad_object_key")
    if len(data) > MAX_JOB_BYTES:                             # 2 (also check LIST size before reading)
        raise Reject("too_large")
    payload = open_envelope(keys, TAG_JOB, data)              # 3-5
    j = parse_job(payload)                                    # 6
    if j["job_id"] != m.group(1) or j["bucket"] != agent_bucket:  # 7
        raise Reject("misrouted")
    if seen(j["job_id"]):                                     # 8
        raise Reject("replay")
    created, expires = parse_time(j["created_at"]), parse_time(j["expires_at"])  # 9
    if not created < expires:
        raise Reject("bad_payload", "created_at >= expires_at")
    if (expires - created).total_seconds() > MAX_LIFETIME_S:
        raise Reject("bad_payload", "lifetime")
    if (created - now).total_seconds() > MAX_CREATED_AHEAD_S:
        raise Reject("bad_payload", "created_at in the future")
    if not now < expires:
        r = Reject("expired")
        r.job = j
        raise r
    return j


# ---- vectors ----

def run_vectors(path):
    v = json.load(open(path, encoding="utf-8"))

    def ring(name):
        r = v["keyrings"][name]
        return keyring(v["keys"][r["current"]], v["keys"].get(r.get("next", "")))

    bad = total = 0
    for s in v["signatures"]:
        total += 1
        if sign(parse_key(v["keys"][s["key"] + "_hex"]), s["tag"], s["payload"]) != s["sig"]:
            bad += 1
            print("signature mismatch:", s["name"])
    for e in v["envelopes"]:
        total += 1
        try:
            payload = open_envelope(ring(e["keyring"]), e["tag"], e["envelope"].encode("utf-8"))
            got = "ok"
        except Reject as r:
            payload, got = None, r.outcome
        if got != e["expect"]:
            bad += 1
            print("envelope:", e["name"], "got", got, "want", e["expect"])
        elif got == "ok" and payload != e["payload"]:  # the exact signed string
            bad += 1
            print("envelope:", e["name"], "returned a different payload")
    for c in v["pending_jobs"]:
        total += 1
        ledger = set(c["ledger"])
        try:
            check_pending_job(ring(c["keyring"]), c["object_key"], c["agent_bucket"], parse_time(c["now"]),
                              c["envelope"].encode("utf-8"), lambda i: i in ledger)
            got = "ok"
        except Reject as r:
            got = r.outcome
        if got != c["expect"]:
            bad += 1
            print("pending job:", c["name"], "got", got, "want", c["expect"])
    print("python reference: %d of %d vectors mismatched" % (bad, total))
    return bad == 0


if __name__ == "__main__":
    sys.exit(0 if run_vectors(sys.argv[1]) else 1)
