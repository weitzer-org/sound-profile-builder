"""Generates testdata/devicejobs/vectors.json (stdlib only).

Written independently of the Go code, from docs/design/device-job-contract.md.
Contract v2 (edit_preset) only appends entries: the v1 ones are unchanged.
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

# ---- contract v2: edit_preset (sections 7.1, 7.2 and 8). Appended only: every v1 entry above is
# unchanged, byte for byte (Go's TestV1VectorsUnchanged pins them).
JOB2 = "qc2-device-job/v2"
EID = "7a2d9c41-5e6f-4a1b-8c3d-9e0f1a2b3c4d"
EK = "jobs/pending/%s.json" % EID
SNAP = "3f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e02"
def plan(**over):
    p = {"format":"qc2-device-edit/1","catalog":{"coros":"4.1","sha256":"ab"*32},"snapshot":SNAP,
         "expect":{"folder_key":"/media/p4/Presets/Sounds profiles","position":24,"slot":"4A","name":"Texas Blues",
                   "layout":[[0,0,1120],[0,1,12045],[0,2,8006],[1,0,5]]},
         "param_ops":[{"row":0,"col":0,"model_id":1120,"index":0,"label":"GAIN","evidence":["taper:g4"],"scene_mode":True,
                       "expect":[0.45]*8,"wire":[0.5,0.62,0.45,0.45,0.45,0.45,0.45,0.45]}],
         "bypass_ops":[{"row":0,"col":2,"model_id":8006,"why":"target","expect":[False]*8,"bypass":[False,True]+[False]*6}]}
    p.update(over); return p
def expect(**over):
    e = dict(plan()["expect"]); e.update(over); return e
def pop(**over):
    o = dict(plan()["param_ops"][0]); o.update(over); return o
def bop(**over):
    o = dict(plan()["bypass_ops"][0]); o.update(over); return o
def sha(t, fmt="qc2-device-edit/1"): return hashlib.sha256(fmt.encode() + b"\n" + t.encode("utf-8")).hexdigest()
def ejob(p=None, text=None, digest=None, **args_over):
    """An edit_preset job. text overrides the plan text (default: compact(p)); digest overrides the hash."""
    t = text if text is not None else compact(p if p is not None else plan())
    a = {"slot":"4A","mode":"copy","target_slot":"20A","name":"Texas Blues edit",
         "source":{"preset_id":"device-1","guitar":"From the unit"},
         "plan":{"format":"qc2-device-edit/1","sha256":digest if digest is not None else sha(t),"payload":t}}
    a.update(args_over)
    return job(v=2, job_id=EID, kind="edit_preset", args=a)
def ej(name, j, expect, tag=JOB2, key=K0, keyring="current"):
    return pj(name, env(key, tag, compact(j)), expect, object_key=EK, keyring=keyring)
OK_TEXT = compact(plan())
edits = [
 ej("valid edit_preset: copy", ejob(), "ok"),
 ej("valid edit_preset: live", ejob(mode="live", target_slot=None, name=None), "ok"),
 ej("valid edit_preset: overwrite", ejob(mode="overwrite", target_slot=None, name=None), "ok"),
 ej("valid edit_preset: only a param op", ejob(plan(bypass_ops=[])), "ok"),
 ej("valid edit_preset: only a bypass op (removed, all eight)", ejob(plan(param_ops=[], bypass_ops=[bop(why="removed", bypass=[True]*8)])), "ok"),
 ej("valid edit_preset: non-ASCII name and label, '<' (hash over UTF-8)", ejob(plan(expect=expect(name="Caf\u00e9 <Lead>"), param_ops=[pop(label="\u00d8 SHIFT <")])), "ok"),
 ej("valid edit_preset: plan text with escapes and whitespace (hashed as given)",
    ejob(text=OK_TEXT.replace('{"format":', ' {"format" : ', 1).replace("Texas Blues", "Texas \\u0042lues").replace("GAIN", "G\\u0041IN") + "\n"), "ok"),
 ej("valid edit_preset: integer wire values 0 and 1", ejob(plan(param_ops=[pop(wire=[1,0,0.45,0.45,0.45,0.45,0.45,0.45])])), "ok"),
 ej("valid edit_preset: scene_mode false with one value", ejob(plan(param_ops=[pop(scene_mode=False, wire=[0.5]*8)])), "ok"),
 ej("valid edit_preset: signed with the next key during rotation", ejob(), "ok", key=K1, keyring="current+next"),
 # Version and tag binding.
 ej("edit_preset (v 2) signed under the v1 tag", ejob(), "bad_payload", tag=JOB),
 ej("edit_preset with v 1 under the v2 tag", dict(ejob(), v=1), "bad_payload"),
 ej("edit_preset with v 1 under the v1 tag", dict(ejob(), v=1), "bad_payload", tag=JOB),
 pj("list_slots (v 1) signed under the v2 tag", env(K0, JOB2, P1), "bad_payload"),
 pj("list_slots with v 2 under the v2 tag", env(K0, JOB2, compact(job(v=2))), "bad_payload"),
 pj("push_preset with v 2 under the v2 tag", env(K0, JOB2, compact(dict(push, v=2))), "bad_payload", object_key=PK),
 ej("edit_preset signed under the result tag", ejob(), "bad_signature", tag=RES),
 pj("edit_preset envelope with the v1 payload swapped in", env(K0, JOB2, compact(ejob()), payload=compact(dict(ejob(), v=1))), "bad_signature", object_key=EK),
 pj("edit_preset with v 2.0", env(K0, JOB2, compact(ejob()).replace('"v":2,', '"v":2.0,', 1)), "bad_payload", object_key=EK),
 ej("edit_preset with a case-variant kind", dict(ejob(), kind="Edit_Preset"), "bad_payload"),
 # The plan record and its hash.
 ej("plan hash does not match the text", ejob(digest=sha(OK_TEXT + " ")), "bad_payload"),
 ej("plan hash in upper case", ejob(digest=sha(OK_TEXT).upper()), "bad_payload"),
 ej("plan hash over a re-serialised text", ejob(text=OK_TEXT.replace(",", ", "), digest=sha(OK_TEXT)), "bad_payload"),
 ej("plan format is qc2-device-edit/2", dict(ejob(), args=dict(ejob()["args"], plan={"format":"qc2-device-edit/2","sha256":sha(OK_TEXT,"qc2-device-edit/2"),"payload":OK_TEXT})), "bad_payload"),
 ej("plan payload is an object, not a string", dict(ejob(), args=dict(ejob()["args"], plan={"format":"qc2-device-edit/1","sha256":sha(OK_TEXT),"payload":plan()})), "bad_payload"),
 ej("plan record has an extra key", dict(ejob(), args=dict(ejob()["args"], plan=dict(ejob()["args"]["plan"], v=1))), "bad_payload"),
 ej("plan text is not JSON", ejob(text="{not json"), "bad_payload"),
 ej("plan text has trailing data", ejob(text=OK_TEXT + " {}"), "bad_payload"),
 ej("plan text has a duplicate key", ejob(text=OK_TEXT.replace('{"format":"qc2-device-edit/1"', '{"format":"x","format":"qc2-device-edit/1"', 1)), "bad_payload"),
 ej("plan text has NaN", ejob(text=OK_TEXT.replace("0.62", "NaN", 1)), "bad_payload"),
 ej("plan text has 1e400", ejob(text=OK_TEXT.replace("0.62", "1e400", 1)), "bad_payload"),
 ej("plan text has an unpaired surrogate escape", ejob(text=OK_TEXT.replace('"GAIN"', '"G\\ud800"', 1)), "bad_payload"),
 ej("plan text over 256 KiB (whitespace)", ejob(text=OK_TEXT + " " * (262144 - len(OK_TEXT) + 1)), "bad_payload"),
 ej("plan text exactly 256 KiB (whitespace)", ejob(text=OK_TEXT + " " * (262144 - len(OK_TEXT))), "ok"),
 # Mode, slots and name.
 ej("copy without target_slot", ejob(target_slot=None), "bad_payload"),
 ej("copy to the open slot", ejob(target_slot="4A"), "bad_payload"),
 ej("copy with a lowercase target_slot", ejob(target_slot="20a"), "bad_payload"),
 ej("copy without a name", ejob(name=None), "bad_payload"),
 ej("copy with a 33-character name", ejob(name="x" * 33), "bad_payload"),
 ej("live with a name", ejob(mode="live", target_slot=None), "bad_payload"),
 ej("live with a target_slot", ejob(mode="live", name=None), "bad_payload"),
 ej("overwrite with a name (an overwrite never takes a name from a job)", ejob(mode="overwrite", target_slot=None, name="Texas Blues"), "bad_payload"),
 ej("mode is Live", ejob(mode="Live", target_slot=None, name=None), "bad_payload"),
 ej("mode is delete", ejob(mode="delete", target_slot=None, name=None), "bad_payload"),
 ej("args slot differs from the plan's", ejob(slot="4B"), "bad_payload"),
 ej("args missing source", dict(ejob(), args={k: v for k, v in ejob()["args"].items() if k != "source"}), "bad_payload"),
 ej("args with an extra key", ejob(snapshot=SNAP), "bad_payload"),
 # expect.
 ej("plan slot is not its position's label", ejob(plan(expect=expect(position=25))), "bad_payload"),
 ej("plan position 256", ejob(plan(expect=expect(position=256, slot="33A")), slot="4A"), "bad_payload"),
 ej("plan position 24.0", ejob(text=OK_TEXT.replace('"position":24', '"position":24.0')), "bad_payload"),
 ej("plan position as a string", ejob(plan(expect=expect(position="24"))), "bad_payload"),
 ej("folder_key empty (a plan from a /1 read)", ejob(plan(expect=expect(folder_key=""))), "bad_payload"),
 ej("folder_key with a backtick", ejob(plan(expect=expect(folder_key="/media/p4/a`b"))), "bad_payload"),
 ej("folder_key null", ejob(plan(expect=expect(folder_key=None))), "bad_payload"),
 ej("expect.name empty", ejob(plan(expect=expect(name=""))), "bad_payload"),
 ej("expect.name with a control character", ejob(plan(expect=expect(name="Texas\u0007Blues"))), "bad_payload"),
 ej("expect.name with U+2028", ejob(plan(expect=expect(name="Texas\u2028Blues"))), "bad_payload"),
 ej("expect.name of 65 characters", ejob(plan(expect=expect(name="x" * 65))), "bad_payload"),
 ej("expect.name of 64 characters", ejob(plan(expect=expect(name="\u00e9" * 64))), "ok"),
 ej("expect has an extra key (folder)", ejob(plan(expect=expect(folder="other"))), "bad_payload"),
 ej("layout empty", ejob(plan(expect=expect(layout=[]))), "bad_payload"),
 ej("layout out of order", ejob(plan(expect=expect(layout=[[0,1,12045],[0,0,1120],[0,2,8006]]))), "bad_payload"),
 ej("layout with a duplicate cell", ejob(plan(expect=expect(layout=[[0,0,1120],[0,0,1120],[0,1,12045],[0,2,8006]]))), "bad_payload"),
 ej("layout entry of four numbers", ejob(plan(expect=expect(layout=[[0,0,1120,1],[0,1,12045],[0,2,8006]]))), "bad_payload"),
 ej("layout row 4", ejob(plan(expect=expect(layout=[[0,0,1120],[0,1,12045],[0,2,8006],[4,0,5]]))), "bad_payload"),
 ej("layout model_id negative", ejob(plan(expect=expect(layout=[[0,0,1120],[0,1,12045],[0,2,8006],[1,0,-5]]))), "bad_payload"),
 ej("catalog coros not a version", ejob(plan(catalog={"coros":"v4","sha256":"ab"*32})), "bad_payload"),
 ej("snapshot in upper case", ejob(plan(snapshot=SNAP.upper())), "bad_payload"),
 # Ops: the hostile plans the contract can express.
 ej("param op on a cell not in the layout", ejob(plan(param_ops=[pop(col=5)])), "bad_payload"),
 ej("param op with another model than the layout's", ejob(plan(param_ops=[pop(model_id=1121)])), "bad_payload"),
 ej("param op on row 1 (a cell the layout has)", ejob(plan(param_ops=[pop(row=1, model_id=5)])), "bad_payload"),
 ej("bypass op on row 1", ejob(plan(bypass_ops=[bop(row=1, col=0, model_id=5)])), "bad_payload"),
 ej("the same param op twice", ejob(plan(param_ops=[pop(), pop()])), "bad_payload"),
 ej("param ops out of order", ejob(plan(param_ops=[pop(index=3), pop(index=1)])), "bad_payload"),
 ej("param op that changes nothing", ejob(plan(param_ops=[pop(wire=[0.45]*8)])), "bad_payload"),
 ej("param op with nine values", ejob(plan(param_ops=[pop(wire=[0.5]*9)])), "bad_payload"),
 ej("param op with seven expect values", ejob(plan(param_ops=[pop(expect=[0.45]*7)])), "bad_payload"),
 ej("param op value 1.5", ejob(plan(param_ops=[pop(wire=[1.5]+[0.45]*7)])), "bad_payload"),
 ej("param op value -0.1", ejob(plan(param_ops=[pop(wire=[-0.1]+[0.45]*7)])), "bad_payload"),
 ej("param op value as a string", ejob(plan(param_ops=[pop(wire=["0.5"]+[0.45]*7)])), "bad_payload"),
 ej("param op value true", ejob(plan(param_ops=[pop(wire=[True]+[0.45]*7)])), "bad_payload"),
 ej("param op index 256", ejob(plan(param_ops=[pop(index=256)])), "bad_payload"),
 ej("param op col 0.0", ejob(text=OK_TEXT.replace('"col":0,"model_id":1120', '"col":0.0,"model_id":1120')), "bad_payload"),
 ej("param op col 0e0", ejob(text=OK_TEXT.replace('"col":0,"model_id":1120', '"col":0e0,"model_id":1120')), "bad_payload"),
 ej("param op value 1e-400 (underflows to 0 in both languages)", ejob(text=OK_TEXT.replace("0.62", "1e-400", 1)), "ok"),
 ej("param op value -0.0", ejob(text=OK_TEXT.replace("0.62", "-0.0", 1)), "ok"),
 ej("param op col -0 (an integer token)", ejob(text=OK_TEXT.replace('"col":0,"model_id":1120', '"col":-0,"model_id":1120')), "ok"),
 ej("scene_mode false with different values", ejob(plan(param_ops=[pop(scene_mode=False)])), "bad_payload"),
 ej("scene_mode as 1", ejob(plan(param_ops=[pop(scene_mode=1)])), "bad_payload"),
 ej("param op label empty", ejob(plan(param_ops=[pop(label="")])), "bad_payload"),
 ej("param op evidence in upper case", ejob(plan(param_ops=[pop(evidence=["Taper:G4"])])), "bad_payload"),
 ej("param op evidence with a space", ejob(plan(param_ops=[pop(evidence=["taper g4"])])), "bad_payload"),
 ej("param op with nine evidence entries", ejob(plan(param_ops=[pop(evidence=["a"] * 9)])), "bad_payload"),
 ej("param op with an extra key", ejob(plan(param_ops=[dict(pop(), string="x")])), "bad_payload"),
 ej("513 param ops", ejob(plan(expect=expect(layout=[[0, c, 1120] for c in range(8)]), bypass_ops=[],
                             param_ops=[pop(col=i // 65, index=i % 65) for i in range(513)])), "bad_payload"),
 ej("512 param ops", ejob(plan(expect=expect(layout=[[0, c, 1120] for c in range(8)]), bypass_ops=[],
                             param_ops=[pop(col=i // 64, index=i % 64) for i in range(512)])), "ok"),
 ej("bypass op why moved", ejob(plan(bypass_ops=[bop(why="moved")])), "bad_payload"),
 ej("removed block not bypassed in every scene", ejob(plan(bypass_ops=[bop(why="removed")])), "bad_payload"),
 ej("bypass op that changes nothing", ejob(plan(bypass_ops=[bop(bypass=[False]*8)])), "bad_payload"),
 ej("bypass flags as numbers", ejob(plan(bypass_ops=[bop(bypass=[0,1,0,0,0,0,0,0])])), "bad_payload"),
 ej("two bypass ops on one cell", ejob(plan(bypass_ops=[bop(), bop()])), "bad_payload"),
 ej("no ops", ejob(plan(param_ops=[], bypass_ops=[])), "bad_payload"),
 ej("plan missing bypass_ops", ejob({k: v for k, v in plan().items() if k != "bypass_ops"}), "bad_payload"),
 ej("plan with an extra top-level key (not_applied)", ejob(plan(not_applied=[])), "bad_payload"),
]
pend += edits

out = {
 "contract":"qc2-device-job-contract/2",
 "note":"Shared by internal/devicejobs (Go) and the home agent (Python). Changing any vector is a contract change.",
 "keys":{"current_hex":K0.hex(),"current_kid":kid(K0),"next_hex":K1.hex(),"next_kid":kid(K1)},
 "keyrings":{"current":{"current":"current_hex"},"current+next":{"current":"current_hex","next":"next_hex"},
             "next-as-current":{"current":"next_hex","next":"current_hex"}},
 "signatures":sigs,"envelopes":envs,"pending_jobs":pend,
}
json.dump(out, sys.stdout, indent=1, ensure_ascii=False); print()
