"""Contract primitives for the QC bridge home agent.

Copied from testdata/devicejobs/reference_check.py (the contract's Python reference,
which says the agent "may import or copy it"), minus its command-line vector runner.
The agent's tests (tests/test_contract_vectors.py) run every entry of
testdata/devicejobs/vectors.json through THIS copy and through the reference, so the
two cannot drift apart unnoticed. Changing behaviour here is a contract change
(docs/design/device-job-contract.md section 1).
"""

import hashlib
import hmac
import json
import math
import re
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
TAG_JOB_V2 = "qc2-device-job/v2"  # edit_preset (contract section 7.1)
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

def _envelope(keys, data):
    """Steps 1-3: the strict envelope and its key. Returns (env, payload_bytes, key)."""
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
    return env, payload_bytes, key


def _mac_ok(key, tag, payload_bytes, sig):
    want = hmac.new(key, tag.encode("ascii") + b"\n" + payload_bytes, hashlib.sha256).hexdigest()
    return hmac.compare_digest(want, sig)


def open_envelope(keys, tag, data):
    """Verify envelope bytes under tag; return the payload str."""
    env, payload_bytes, key = _envelope(keys, data)
    if not _mac_ok(key, tag, payload_bytes, env["sig"]):
        raise Reject("bad_signature")
    return env["payload"]


def open_job(keys, data):
    """Steps 3-6 for a job: verify under both job tags (both MACs computed and compared in constant
    time), then parse the payload strictly at the version of the tag that verified (v1 tag: v 1 and
    the v1 kinds; v2 tag: v 2 and edit_preset). Neither verifying is bad_signature."""
    env, payload_bytes, key = _envelope(keys, data)
    v1 = _mac_ok(key, TAG_JOB, payload_bytes, env["sig"])
    v2 = _mac_ok(key, TAG_JOB_V2, payload_bytes, env["sig"])
    if v1:
        return parse_job(env["payload"], 1)
    if v2:
        return parse_job(env["payload"], 2)
    raise Reject("bad_signature")


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


# ---- edit_preset (contract sections 7.1 and 7.2) ----

EDIT_FORMAT = "qc2-device-edit/1"
MAX_EDIT_PLAN_BYTES = 256 * 1024
MAX_EDIT_PARAM_OPS = 512
MAX_EDIT_BYPASS_OPS = 8
MAX_EDIT_LAYOUT = 32
MAX_EDIT_EVIDENCE = 8
MAX_EDIT_TEXT = 64
MAX_MODEL_ID = 2 ** 31 - 1
SHA256_RE = re.compile(r"[0-9a-f]{64}")
COROS_RE = re.compile(r"[0-9]{1,3}(?:\.[0-9]{1,3}){0,3}")
EVIDENCE_RE = re.compile(r"[a-z0-9][a-z0-9.:+-]{0,63}")
EDIT_KINDS = {1: {"push_preset", "list_slots", "read_preset", "sync_device_library"}, 2: {"edit_preset"}}


class BadPlan(ValueError):
    pass


def _need(cond, what):
    if not cond:
        raise BadPlan(what)


def _int(x, lo, hi):
    """An integer token (not a bool, not 1.0 or 1e0) in [lo, hi]."""
    return type(x) is int and lo <= x <= hi


def _unit(x):
    """A number in [0, 1] (strict_loads already refused non-finite ones)."""
    return type(x) in (int, float) and 0 <= x <= 1


def slot_label(p):
    return "%d%s" % (p // 8 + 1, "ABCDEFGH"[p % 8])


def valid_folder_key(k):
    return (isinstance(k, str) and 1 <= len(k) <= 128 and k[0] != " " and k[-1] != " " and "  " not in k
            and all(0x20 <= ord(c) <= 0x7E and c != "`" for c in k))


def edit_text_ok(s):
    """expect.name and an op's label: 1-64 code points, no C0/C1 control, no U+2028/U+2029."""
    return (isinstance(s, str) and 1 <= len(s) <= MAX_EDIT_TEXT
            and not any(ord(c) <= 0x1F or 0x7F <= ord(c) <= 0x9F or c in "\u2028\u2029" for c in s))


def parse_edit_plan(text):
    """Strict parse of a qc2-device-edit/1 plan text (contract section 7.2). Returns the plan dict
    or raises BadPlan."""
    _need(len(text.encode("utf-8")) <= MAX_EDIT_PLAN_BYTES, "plan too large")
    try:
        p = strict_loads(text)
    except ValueError:
        raise BadPlan("plan is not strict JSON")
    _need(_exact(p, "format", "catalog", "snapshot", "expect", "param_ops", "bypass_ops"), "plan keys")
    _need(p["format"] == EDIT_FORMAT, "plan format")
    c = p["catalog"]
    _need(_exact(c, "coros", "sha256") and isinstance(c["coros"], str) and COROS_RE.fullmatch(c["coros"])
          and isinstance(c["sha256"], str) and SHA256_RE.fullmatch(c["sha256"]), "catalog")
    _need(isinstance(p["snapshot"], str) and UUID_RE.fullmatch(p["snapshot"]), "snapshot")
    e = p["expect"]
    _need(_exact(e, "folder_key", "position", "slot", "name", "layout"), "expect keys")
    _need(valid_folder_key(e["folder_key"]), "folder_key")
    _need(_int(e["position"], 0, 255) and e["slot"] == slot_label(e["position"]), "position/slot")
    _need(edit_text_ok(e["name"]), "name")
    lay = e["layout"]
    _need(isinstance(lay, list) and 1 <= len(lay) <= MAX_EDIT_LAYOUT, "layout")
    cells, prev = {}, -1
    for t in lay:
        _need(isinstance(t, list) and len(t) == 3 and _int(t[0], 0, 3) and _int(t[1], 0, 7)
              and _int(t[2], 0, MAX_MODEL_ID), "layout entry")
        k = t[0] * 8 + t[1]
        _need(k > prev, "layout order")
        prev = k
        cells[(t[0], t[1])] = t[2]

    def on_cell(op):
        _need(_int(op["row"], 0, 0) and _int(op["col"], 0, 7) and _int(op["model_id"], 0, MAX_MODEL_ID)
              and cells.get((op["row"], op["col"])) == op["model_id"], "op cell")

    ops = p["param_ops"]
    _need(isinstance(ops, list) and len(ops) <= MAX_EDIT_PARAM_OPS, "param_ops")
    prev = -1
    for op in ops:
        _need(_exact(op, "row", "col", "model_id", "index", "label", "evidence", "scene_mode", "expect", "wire"),
              "param op keys")
        on_cell(op)
        _need(_int(op["index"], 0, 255), "index")
        k = op["col"] * 256 + op["index"]
        _need(k > prev, "param op order")
        prev = k
        _need(edit_text_ok(op["label"]), "label")
        ev = op["evidence"]
        _need(isinstance(ev, list) and len(ev) <= MAX_EDIT_EVIDENCE
              and all(isinstance(x, str) and EVIDENCE_RE.fullmatch(x) for x in ev), "evidence")
        _need(type(op["scene_mode"]) is bool, "scene_mode")
        ex, wi = op["expect"], op["wire"]
        _need(isinstance(ex, list) and isinstance(wi, list) and len(ex) == 8 and len(wi) == 8
              and all(_unit(x) for x in ex + wi), "values")
        _need(ex != wi, "op changes nothing")
        _need(op["scene_mode"] or all(x == wi[0] for x in wi), "scene_mode false needs one value")
    bops = p["bypass_ops"]
    _need(isinstance(bops, list) and len(bops) <= MAX_EDIT_BYPASS_OPS, "bypass_ops")
    prev = -1
    for op in bops:
        _need(_exact(op, "row", "col", "model_id", "why", "expect", "bypass"), "bypass op keys")
        on_cell(op)
        _need(op["col"] > prev, "bypass op order")
        prev = op["col"]
        _need(op["why"] in ("target", "removed"), "why")
        ex, by = op["expect"], op["bypass"]
        _need(isinstance(ex, list) and isinstance(by, list) and len(ex) == 8 and len(by) == 8
              and all(type(x) is bool for x in ex + by), "flags")
        _need(ex != by, "op changes nothing")
        _need(op["why"] != "removed" or all(by), "removed means bypassed everywhere")
    _need(len(ops) + len(bops) >= 1, "no ops")
    return p


def plan_sha256(fmt, text):
    return hashlib.sha256(fmt.encode("utf-8") + b"\n" + text.encode("utf-8")).hexdigest()


def _check_edit_args(a):
    try:
        _need(_exact(a, "slot", "mode", "target_slot", "name", "source", "plan"), "args keys")
        _need(isinstance(a["slot"], str) and SLOT_RE.fullmatch(a["slot"]), "slot")
        mode, tgt, name = a["mode"], a["target_slot"], a["name"]
        if mode == "copy":
            _need(isinstance(tgt, str) and SLOT_RE.fullmatch(tgt) and tgt != a["slot"], "target_slot")
            _need(_valid_name(name), "name")
        else:
            _need(mode in ("live", "overwrite") and tgt is None and name is None, "mode")
        s = a["source"]
        _need(_exact(s, "preset_id", "guitar") and isinstance(s["preset_id"], str)
              and isinstance(s["guitar"], str), "source")
        pl = a["plan"]
        _need(_exact(pl, "format", "sha256", "payload") and pl["format"] == EDIT_FORMAT
              and isinstance(pl["sha256"], str) and SHA256_RE.fullmatch(pl["sha256"])
              and isinstance(pl["payload"], str), "plan record")
        _need(len(pl["payload"].encode("utf-8")) <= MAX_EDIT_PLAN_BYTES, "plan too large")
        _need(plan_sha256(pl["format"], pl["payload"]) == pl["sha256"], "plan hash")
        p = parse_edit_plan(pl["payload"])
        _need(p["expect"]["slot"] == a["slot"], "plan pins another slot")
        return True
    except BadPlan:
        return False


def _check_args(kind, a):
    if kind in ("list_slots", "sync_device_library"):
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
    if kind == "edit_preset":
        return _check_edit_args(a)
    return False


def parse_job(payload, version=1):
    """Step 6 at the version of the tag that verified (1 or 2)."""
    try:
        j = strict_loads(payload)
    except ValueError:
        raise Reject("bad_payload")
    if not _exact(j, "v", "type", "job_id", "bucket", "kind", "created_at", "expires_at", "args"):
        raise Reject("bad_payload", "key set")
    if type(j["v"]) is not int or j["v"] != version or j["type"] != "qc2-device-job":
        raise Reject("bad_payload", "version/type")
    if j["kind"] not in EDIT_KINDS[version]:
        raise Reject("bad_payload", "kind is not a version %d kind" % version)
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
    j = open_job(keys, data)                                  # 3-6, under either job tag
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


# ---- agent-side additions (not in the reference) ----

TAG_OWNED_SLOTS = "qc2-device-owned-slots/v1"
MAX_RESULT_BYTES = 2 * 1024 * 1024  # 2 MiB: a sync of up to 5000 captures (internal/devicejobs.MaxResultBytes)
MAX_READBACK_BYTES = 512 * 1024    # internal/devicejobs.MaxReadbackBytes: Go refuses a larger read-back payload
MAX_HEARTBEAT_BYTES = 8 * 1024
DONE_PREFIX = "jobs/done/"
PENDING_PREFIX = "jobs/pending/"
HEARTBEAT_KEY = "agent/heartbeat.json"
OWNED_SLOTS_KEY = "jobs/owned_slots.json"


def pending_key(job_id):
    """Contract section 3 layout; job_id must already be a validated lowercase UUID."""
    return PENDING_PREFIX + job_id + ".json"


def done_key(job_id):
    return DONE_PREFIX + job_id + ".json"


def format_time(dt):
    """Contract section 6: UTC, whole seconds, literal Z."""
    return dt.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def seal_payload(key, tag, payload_obj):
    """Envelope bytes for a payload object. The payload string is produced once and
    carried verbatim, so nothing is ever re-serialized to check a signature."""
    payload = json.dumps(payload_obj, separators=(",", ":"), ensure_ascii=False, allow_nan=False)
    return seal(key, tag, payload)
