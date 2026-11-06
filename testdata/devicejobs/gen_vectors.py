"""Generates testdata/devicejobs/vectors.json (stdlib only).

Written independently of the Go code, from docs/design/device-job-contract.md.
The expected outcomes are the contract's; internal/devicejobs (Go) and
reference_check.py (Python) must both reach every one of them.

    python3 -I testdata/devicejobs/gen_vectors.py > testdata/devicejobs/vectors.json

Regenerating with a change is a contract change (contract section 1).
"""
import hashlib, hmac, json, sys

K0 = bytes(range(32)); K1 = bytes(range(32, 64))
def kid(k): return hashlib.sha256(b"qc2-device-kid/v1\n" + k).hexdigest()[:8]
def sign(k, tag, p): return hmac.new(k, tag.encode() + b"\n" + p.encode("utf-8"), hashlib.sha256).hexdigest()
def env(k, tag, p, **over):
    e = {"v": 1, "kid": kid(k), "payload": p, "sig": sign(k, tag, p)}
    e.update(over)
    return json.dumps(e, separators=(",", ":"))
def compact(o): return json.dumps(o, separators=(",", ":"), ensure_ascii=False)

JOB, RES = "qc2-device-job/v1", "qc2-device-result/v1"
ID = "6f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e01"
B = "qc2-device-jobs-test"
NOW = "2026-11-05T12:05:00Z"
def job(**over):
    j = {"v":1,"type":"qc2-device-job","job_id":ID,"bucket":B,"kind":"list_slots",
         "created_at":"2026-11-05T12:00:00Z","expires_at":"2026-11-05T12:15:00Z","args":{}}
    j.update(over); return j
P1 = compact(job())
assert P1 == '{"v":1,"type":"qc2-device-job","job_id":"6f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e01","bucket":"qc2-device-jobs-test","kind":"list_slots","created_at":"2026-11-05T12:00:00Z","expires_at":"2026-11-05T12:15:00Z","args":{}}'
P2 = '{"name":"Café <Lead>"}'
push = job(job_id="0b6d5f3e-9a41-8c2d-b7e5-1f0a2c3d4e5f", kind="push_preset",
           args={"slot":"20A","name":"Texas Blues","source":{"preset_id":"p-123","guitar":"Telecaster"},
                 "export":{"format":"qc2-device-export/1","payload":{"rows":[{"row":0,"blocks":[]}]}}})
PP = compact(push)

sigs = [
 {"name":"V1 job payload, current key","tag":JOB,"key":"current","payload":P1,"sig":sign(K0,JOB,P1)},
 {"name":"V1 payload under the result tag (must differ)","tag":RES,"key":"current","payload":P1,"sig":sign(K0,RES,P1)},
 {"name":"V2 non-ASCII and <","tag":JOB,"key":"current","payload":P2,"sig":sign(K0,JOB,P2)},
 {"name":"V1 payload, next key","tag":JOB,"key":"next","payload":P1,"sig":sign(K1,JOB,P1)},
]

# Envelope-level vectors: Open(tag, envelope) with the named keyring.
s2 = sign(K0, JOB, P2)
go_style = '{"v":1,"kid":"%s","payload":%s,"sig":"%s"}' % (kid(K0), json.dumps(P2, ensure_ascii=False).replace("<","\\u003c"), s2)
envs = [
 {"name":"python-style envelope (ensure_ascii escapes)","tag":JOB,"keyring":"current","envelope":env(K0,JOB,P2),"expect":"ok","payload":P2},
 {"name":"go-style envelope (\\u003c escape, raw UTF-8)","tag":JOB,"keyring":"current","envelope":go_style,"expect":"ok","payload":P2},
 {"name":"whitespace and key order do not matter","tag":JOB,"keyring":"current",
  "envelope":'{ "sig" : "%s", "payload" : %s, "kid" : "%s", "v" : 1 }' % (s2, json.dumps(P2), kid(K0)),"expect":"ok","payload":P2},
 {"name":"wrong tag","tag":RES,"keyring":"current","envelope":env(K0,JOB,P1),"expect":"bad_signature"},
 {"name":"tampered payload","tag":JOB,"keyring":"current","envelope":env(K0,JOB,P1,payload=P1.replace("list_slots","read_preset")),"expect":"bad_signature"},
 {"name":"signed with the next key, keyring has only current","tag":JOB,"keyring":"current","envelope":env(K1,JOB,P1),"expect":"unknown_kid"},
 {"name":"signed with the next key during rotation","tag":JOB,"keyring":"current+next","envelope":env(K1,JOB,P1),"expect":"ok","payload":P1},
 {"name":"signed with the old key after rotation (old is next)","tag":JOB,"keyring":"next-as-current","envelope":env(K0,JOB,P1),"expect":"ok","payload":P1},
 {"name":"uppercase sig","tag":JOB,"keyring":"current","envelope":env(K0,JOB,P1,sig=sign(K0,JOB,P1).upper()),"expect":"bad_envelope"},
 {"name":"uppercase kid","tag":JOB,"keyring":"current","envelope":env(K0,JOB,P1,kid=kid(K0).upper()),"expect":"bad_envelope"},
 {"name":"extra envelope key","tag":JOB,"keyring":"current","envelope":env(K0,JOB,P1,extra=1),"expect":"bad_envelope"},
 {"name":"v is 2","tag":JOB,"keyring":"current","envelope":env(K0,JOB,P1,v=2),"expect":"bad_envelope"},
 {"name":"v is 1.0","tag":JOB,"keyring":"current","envelope":env(K0,JOB,P1).replace('"v":1,','"v":1.0,'),"expect":"bad_envelope"},
 {"name":"v is the string 1","tag":JOB,"keyring":"current","envelope":env(K0,JOB,P1,v="1"),"expect":"bad_envelope"},
 {"name":"payload is an object, not a string","tag":JOB,"keyring":"current",
  "envelope":'{"v":1,"kid":"%s","payload":%s,"sig":"%s"}' % (kid(K0), P1, sign(K0,JOB,P1)),"expect":"bad_envelope"},
 {"name":"duplicate envelope key","tag":JOB,"keyring":"current",
  "envelope":'{"v":1,"kid":"%s","payload":%s,"sig":"%s","sig":"%s"}' % (kid(K0), json.dumps(P1), "0"*64, sign(K0,JOB,P1)),"expect":"bad_envelope"},
 {"name":"lone surrogate in payload","tag":JOB,"keyring":"current",
  "envelope":'{"v":1,"kid":"%s","payload":"{\\"name\\":\\"\\ud800\\"}","sig":"%s"}' % (kid(K0), "0"*64),"expect":"bad_envelope"},
 {"name":"trailing data","tag":JOB,"keyring":"current","envelope":env(K0,JOB,P1)+" {}","expect":"bad_envelope"},
 {"name":"not JSON","tag":JOB,"keyring":"current","envelope":"hello","expect":"bad_envelope"},
 {"name":"nested 3000 deep (must not crash the parser)","tag":JOB,"keyring":"current","envelope":"["*3000 + "]"*3000,"expect":"bad_envelope"},
 {"name":"v is 1e400","tag":JOB,"keyring":"current","envelope":env(K0,JOB,P1).replace('"v":1,','"v":1e400,'),"expect":"bad_envelope"},
]

# Pending-job vectors: the agent's full check order (contract section 8).
def pj(name, envelope, expect, object_key=None, agent_bucket=B, now=NOW, ledger=None, keyring="current"):
    return {"name":name,"keyring":keyring,"object_key":object_key or "jobs/pending/%s.json" % ID,
            "agent_bucket":agent_bucket,"now":now,"ledger":ledger or [],"envelope":envelope,"expect":expect}
def sj(j): return env(K0, JOB, compact(j))
PK = "jobs/pending/0b6d5f3e-9a41-8c2d-b7e5-1f0a2c3d4e5f.json"
pend = [
 pj("valid list_slots job", env(K0,JOB,P1), "ok"),
 pj("valid push_preset job", env(K0,JOB,PP), "ok", object_key="jobs/pending/0b6d5f3e-9a41-8c2d-b7e5-1f0a2c3d4e5f.json"),
 pj("valid sync_device_library job", sj(job(kind="sync_device_library")), "ok"),
 pj("sync_device_library with args", sj(job(kind="sync_device_library", args={"slot":"1A"})), "bad_payload"),
 pj("sync_device_library with a case-variant kind", sj(job(kind="Sync_Device_Library")), "bad_payload"),
 pj("valid read_preset job", sj(job(kind="read_preset", args={"slot":None})), "ok"),
 pj("object key not under jobs/pending/", env(K0,JOB,P1), "bad_object_key", object_key="jobs/done/%s.json" % ID),
 pj("object key with uppercase uuid", env(K0,JOB,P1), "bad_object_key", object_key="jobs/pending/%s.json" % ID.upper()),
 pj("object key with traversal", env(K0,JOB,P1), "bad_object_key", object_key="jobs/pending/../%s.json" % ID),
 pj("bad signature", env(K0,JOB,P1,sig="0"*64), "bad_signature"),
 pj("unknown kid", env(K1,JOB,P1), "unknown_kid"),
 pj("NaN in payload", env(K0,JOB,P1.replace('"v":1','"v":NaN')), "bad_payload"),
 pj("duplicate key in payload", env(K0,JOB,P1.replace('"kind":"list_slots"','"kind":"push_preset","kind":"list_slots"')), "bad_payload"),
 pj("unknown payload field", sj(job(priority=1)), "bad_payload"),
 pj("missing payload field", env(K0,JOB,compact({k:v for k,v in job().items() if k!="args"})), "bad_payload"),
 pj("payload v is 2", sj(job(v=2)), "bad_payload"),
 pj("unknown kind", sj(job(kind="delete_preset")), "bad_payload"),
 pj("list_slots with args", sj(job(args={"slot":"1A"})), "bad_payload"),
 pj("read_preset without slot key", sj(job(kind="read_preset", args={})), "bad_payload"),
 pj("push_preset with lowercase slot", env(K0,JOB,PP.replace('"20A"','"20a"')), "bad_payload", object_key="jobs/pending/0b6d5f3e-9a41-8c2d-b7e5-1f0a2c3d4e5f.json"),
 pj("push_preset with slot out of range", env(K0,JOB,PP.replace('"20A"','"33A"')), "bad_payload", object_key="jobs/pending/0b6d5f3e-9a41-8c2d-b7e5-1f0a2c3d4e5f.json"),
 pj("push_preset with control character in name", env(K0,JOB,PP.replace('Texas Blues','Texas\\u0007Blues')), "bad_payload", object_key="jobs/pending/0b6d5f3e-9a41-8c2d-b7e5-1f0a2c3d4e5f.json"),
 pj("timestamp with fractional seconds", sj(job(created_at="2026-11-05T12:00:00.000Z")), "bad_payload"),
 pj("timestamp with offset", sj(job(created_at="2026-11-05T12:00:00+00:00")), "bad_payload"),
 pj("job_id does not match the object key", sj(job(job_id="11111111-2222-4333-8444-555555555555")), "misrouted"),
 pj("bucket mismatch", env(K0,JOB,P1), "misrouted", agent_bucket="someone-elses-bucket"),
 pj("replay: id already in the ledger", env(K0,JOB,P1), "replay", ledger=[ID]),
 pj("replay wins over expiry", env(K0,JOB,P1), "replay", ledger=[ID], now="2026-11-05T13:00:00Z"),
 pj("expired at exactly expires_at", env(K0,JOB,P1), "expired", now="2026-11-05T12:15:00Z"),
 pj("one second before expiry", env(K0,JOB,P1), "ok", now="2026-11-05T12:14:59Z"),
 pj("created_at 6 minutes in the future", env(K0,JOB,P1), "bad_payload", now="2026-11-05T11:54:00Z"),
 pj("created_at 5 minutes in the future is allowed", env(K0,JOB,P1), "ok", now="2026-11-05T11:55:00Z"),
 pj("created_at not before expires_at", sj(job(expires_at="2026-11-05T12:00:00Z")), "bad_payload"),
 pj("lifetime over 24h", sj(job(expires_at="2026-11-06T12:00:01Z")), "bad_payload"),
 pj("rotation: job signed with the next key", env(K1,JOB,P1), "ok", keyring="current+next"),
 # Parser-differential and strictness cases (security review of the contract).
 pj("object key with a trailing newline", env(K0,JOB,P1), "bad_object_key", object_key="jobs/pending/%s.json\n" % ID),
 pj("push slot with a trailing newline", env(K0,JOB,PP.replace('"20A"','"20A\\n"')), "bad_payload", object_key=PK),
 pj("timestamp with a trailing newline", env(K0,JOB,P1.replace('12:00:00Z"','12:00:00Z\\n"',1)), "bad_payload"),
 pj("timestamp with full-width digits", sj(job(created_at="\uff12\uff10\uff12\uff16-11-05T12:00:00Z")), "bad_payload"),
 pj("timestamp in year 0000", sj(job(created_at="0000-01-01T00:00:00Z")), "bad_payload"),
 pj("case-variant duplicate key", env(K0,JOB,P1.replace('"kind":"list_slots"','"Kind":"push_preset","kind":"list_slots"')), "bad_payload"),
 pj("case-variant key alone", env(K0,JOB,P1.replace('"kind":','"KIND":')), "bad_payload"),
 pj("v is true", env(K0,JOB,P1.replace('"v":1','"v":true')), "bad_payload"),
 pj("v is 1.0", env(K0,JOB,P1.replace('"v":1','"v":1.0')), "bad_payload"),
 pj("push source is null", env(K0,JOB,compact(dict(push, args=dict(push["args"], source=None)))), "bad_payload", object_key=PK),
 pj("push source is empty", env(K0,JOB,compact(dict(push, args=dict(push["args"], source={})))), "bad_payload", object_key=PK),
 pj("push source.preset_id is null", env(K0,JOB,compact(dict(push, args=dict(push["args"], source={"preset_id":None,"guitar":"T"})))), "bad_payload", object_key=PK),
 pj("push source has an extra key", env(K0,JOB,compact(dict(push, args=dict(push["args"], source={"preset_id":"p","guitar":"T","x":1})))), "bad_payload", object_key=PK),
 pj("push export payload is an array", env(K0,JOB,compact(dict(push, args=dict(push["args"], export={"format":"qc2-device-export/1","payload":[]})))), "bad_payload", object_key=PK),
 pj("push export has an unknown format shape", env(K0,JOB,compact(dict(push, args=dict(push["args"], export={"format":"EXPORT","payload":{}})))), "bad_payload", object_key=PK),
 pj("unpaired surrogate inside the payload", env(K0,JOB,PP.replace('"Telecaster"','"Tele\\ud800"')), "bad_payload", object_key=PK),
 pj("1e400 inside the export payload", env(K0,JOB,PP.replace('"row":0','"row":1e400')), "bad_payload", object_key=PK),
 pj("400-digit integer inside the export payload", env(K0,JOB,PP.replace('"row":0','"row":'+"9"*400)), "bad_payload", object_key=PK),
 pj("payload nested exactly 32 deep", env(K0,JOB,PP.replace('"rows":[{"row":0,"blocks":[]}]','"deep":'+"["*28+"]"*28)), "ok", object_key=PK),
 pj("payload nested 33 deep", env(K0,JOB,PP.replace('"rows":[{"row":0,"blocks":[]}]','"deep":'+"["*29+"]"*29)), "bad_payload", object_key=PK),
 pj("read_preset with a padded slot", sj(job(kind="read_preset", args={"slot":" 20A\n"})), "bad_payload"),
 pj("read_preset with a slot (shape-valid, agent refuses later)", sj(job(kind="read_preset", args={"slot":"20A"})), "ok"),
]

out = {
 "contract":"qc2-device-job-contract/1",
 "note":"Shared by internal/devicejobs (Go) and the home agent (Python). Changing any vector is a contract change.",
 "keys":{"current_hex":K0.hex(),"current_kid":kid(K0),"next_hex":K1.hex(),"next_kid":kid(K1)},
 "keyrings":{"current":{"current":"current_hex"},"current+next":{"current":"current_hex","next":"next_hex"},
             "next-as-current":{"current":"next_hex","next":"current_hex"}},
 "signatures":sigs,"envelopes":envs,"pending_jobs":pend,
}
json.dump(out, sys.stdout, indent=1, ensure_ascii=False); print()
