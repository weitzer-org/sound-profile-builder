package devicejobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

const (
	jobsBucket = "qc2-device-jobs-test"
	keyA       = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	keyB       = "202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
)

var t0 = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

type rig struct {
	client *devicejobstest.MemClient
	bridge *devicejobs.Bridge
	agent  *devicejobstest.FakeAgent
	exp    *devicejobstest.StaticExporter
	clock  *time.Time
}

func newRig(t *testing.T) *rig {
	t.Helper()
	kr, err := devicejobs.NewKeyring(keyA, "")
	if err != nil {
		t.Fatal(err)
	}
	c := devicejobstest.NewMemClient()
	exp := &devicejobstest.StaticExporter{Exp: devicejobstest.FixtureExport()}
	clock := t0
	b := devicejobs.NewBridge(c, &devicejobs.Config{Bucket: jobsBucket, Keys: kr}, exp)
	b.Now = func() time.Time { return clock }
	b.PollGap = 5 * time.Millisecond
	a := devicejobstest.NewFakeAgent(c, jobsBucket, kr)
	a.Now = func() time.Time { return clock }
	return &rig{client: c, bridge: b, agent: a, exp: exp, clock: &clock}
}

func (r *rig) online(t *testing.T) {
	t.Helper()
	if err := r.agent.Heartbeat(-10*time.Second, "idle", nil); err != nil {
		t.Fatal(err)
	}
}

var req = devicejobs.PushRequest{PresetID: "p-1", Guitar: "Telecaster", Slot: "20A", Name: "Texas Blues"}

func sp() storage.StructuredPreset {
	return storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"Telecaster": {}}}
}

func userMsg(t *testing.T, err error) string {
	t.Helper()
	var ue *devicejobs.UserError
	if !errors.As(err, &ue) {
		t.Fatalf("want a UserError, got %v", err)
	}
	return ue.Msg
}

// ---- config ----

func TestConfigFromEnv(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		enabled bool
		errSub  string
	}{
		{"unset", nil, false, ""},
		{"enabled", map[string]string{"DEVICE_JOBS_BUCKET": jobsBucket, "DEVICE_JOB_KEY": keyA}, true, ""},
		{"with next", map[string]string{"DEVICE_JOBS_BUCKET": jobsBucket, "DEVICE_JOB_KEY": keyA, "DEVICE_JOB_KEY_NEXT": keyB}, true, ""},
		{"bucket only", map[string]string{"DEVICE_JOBS_BUCKET": jobsBucket}, false, "DEVICE_JOB_KEY"},
		{"key only", map[string]string{"DEVICE_JOB_KEY": keyA}, false, "DEVICE_JOBS_BUCKET"},
		{"next only", map[string]string{"DEVICE_JOB_KEY_NEXT": keyB}, false, "DEVICE_JOB_KEY_NEXT"},
		{"short key", map[string]string{"DEVICE_JOBS_BUCKET": jobsBucket, "DEVICE_JOB_KEY": "abc"}, false, "64 hex"},
		{"bad next", map[string]string{"DEVICE_JOBS_BUCKET": jobsBucket, "DEVICE_JOB_KEY": keyA, "DEVICE_JOB_KEY_NEXT": "x"}, false, "NEXT"},
		{"bad bucket", map[string]string{"DEVICE_JOBS_BUCKET": "Bad_Bucket", "DEVICE_JOB_KEY": keyA}, false, "bucket name"},
		{"same as main", map[string]string{"DEVICE_JOBS_BUCKET": "main-bucket", "DEVICE_JOB_KEY": keyA}, false, "separate bucket"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, k := range []string{"DEVICE_JOBS_BUCKET", "DEVICE_JOB_KEY", "DEVICE_JOB_KEY_NEXT"} {
				t.Setenv(k, c.env[k])
			}
			cfg, err := devicejobs.ConfigFromEnv("main-bucket")
			if (cfg != nil) != c.enabled {
				t.Fatalf("enabled = %v, want %v (err %v)", cfg != nil, c.enabled, err)
			}
			if c.errSub == "" && err != nil || c.errSub != "" && (err == nil || !strings.Contains(err.Error(), c.errSub)) {
				t.Fatalf("err = %v, want containing %q", err, c.errSub)
			}
			if err != nil && strings.Contains(err.Error(), keyA[:16]) {
				t.Fatal("error message contains key material")
			}
			if cfg != nil && strings.Contains(cfg.String()+cfg.GoString(), keyA[:16]) {
				t.Fatal("Config.String prints key material")
			}
		})
	}
}

// ---- confirm token ----

func TestConfirmToken(t *testing.T) {
	kr, _ := devicejobs.NewKeyring(keyA, "")
	c := devicejobs.ConfirmClaims{PresetID: "p-1", Guitar: "Tele", Slot: "20A", Name: "N", PlanSHA256: strings.Repeat("a", 64)}
	tok, exp := kr.IssueConfirm(c, t0)
	if !strings.HasPrefix(tok, "c1.") || exp != t0.Add(devicejobs.ConfirmTTL) {
		t.Fatalf("token %q exp %v", tok, exp)
	}
	if err := kr.CheckConfirm(tok, c, t0.Add(time.Minute)); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for name, mut := range map[string]func(*devicejobs.ConfirmClaims){
		"preset": func(c *devicejobs.ConfirmClaims) { c.PresetID = "p-2" },
		"guitar": func(c *devicejobs.ConfirmClaims) { c.Guitar = "LP" },
		"slot":   func(c *devicejobs.ConfirmClaims) { c.Slot = "20B" },
		"name":   func(c *devicejobs.ConfirmClaims) { c.Name = "M" },
		"plan":   func(c *devicejobs.ConfirmClaims) { c.PlanSHA256 = strings.Repeat("b", 64) },
		// A field boundary shift must not collide.
		"shift": func(c *devicejobs.ConfirmClaims) { c.Guitar, c.Slot = "Tele\",\"20A", "" },
	} {
		d := c
		mut(&d)
		if err := kr.CheckConfirm(tok, d, t0); !errors.Is(err, devicejobs.ErrConfirmMismatch) {
			t.Errorf("%s changed: err = %v, want mismatch", name, err)
		}
	}
	if err := kr.CheckConfirm(tok, c, exp); !errors.Is(err, devicejobs.ErrConfirmExpired) {
		t.Errorf("at expiry: %v", err)
	}
	// Non-canonical spellings of a valid token (which ParseInt would accept)
	// are refused, so one approval can't yield several job ids.
	for _, bad := range []string{"", "c1", "c2.1.00", tok + "0", strings.Replace(tok, "c1.", "c1.-", 1), strings.ToUpper(tok),
		strings.Replace(tok, "c1.", "c1.+", 1), strings.Replace(tok, "c1.", "c1.0", 1), strings.Replace(tok, "c1.", "c1.00", 1)} {
		if err := kr.CheckConfirm(bad, c, t0); err == nil {
			t.Errorf("CheckConfirm(%q) accepted", bad)
		}
	}
	// Far-future expiry (a token minted with a wrong clock) is refused even
	// with a valid MAC.
	far, _ := kr.IssueConfirm(c, t0.Add(time.Hour))
	if err := kr.CheckConfirm(far, c, t0); !errors.Is(err, devicejobs.ErrConfirmInvalid) {
		t.Errorf("far-future token: %v", err)
	}
	// Rotation: a token from the old key verifies once it is NEXT.
	rotated, _ := devicejobs.NewKeyring(keyB, keyA)
	if err := rotated.CheckConfirm(tok, c, t0); err != nil {
		t.Errorf("old-key token after rotation: %v", err)
	}
	other, _ := devicejobs.NewKeyring(keyB, "")
	if err := other.CheckConfirm(tok, c, t0); err == nil {
		t.Error("token verified under an unrelated key")
	}
	id := devicejobs.JobIDFromToken(tok)
	if !devicejobs.ValidJobID(id) || id != devicejobs.JobIDFromToken(tok) || id == devicejobs.JobIDFromToken(far) || id[14] != '8' {
		t.Errorf("JobIDFromToken = %s", id)
	}
}

// ---- store ----

func TestEnqueueAndExistsUseS3ListingSemantics(t *testing.T) {
	r := newRig(t)
	j, err := r.bridge.Store.NewJob("6f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e01", devicejobs.KindListSlots, struct{}{}, t0)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := r.bridge.Store.Enqueue(ctx, j); err != nil {
		t.Fatal(err)
	}
	if keys := r.client.Keys(jobsBucket); len(keys) != 1 || keys[0] != devicejobs.PendingKey(j.JobID) {
		t.Fatalf("keys = %v", keys)
	}
	// The MemClient skips key==prefix like S3; Enqueue must still see the object.
	if err := r.bridge.Store.Enqueue(ctx, j); !errors.Is(err, devicejobs.ErrJobExists) {
		t.Fatalf("second Enqueue: %v", err)
	}
	// What was written is exactly what the agent check accepts.
	data, _ := r.client.ReadFile(ctx, jobsBucket, devicejobs.PendingKey(j.JobID))
	if _, err := devicejobs.CheckPendingJob(r.bridge.Keys, devicejobs.PendingKey(j.JobID), jobsBucket, t0, data, nil); err != nil {
		t.Fatalf("agent check rejects QC-2's own job: %v", err)
	}
	// A listing failure is an error, not "absent".
	r.client.SetFailList(true)
	j2, _ := r.bridge.Store.NewJob("6f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e02", devicejobs.KindListSlots, struct{}{}, t0)
	if err := r.bridge.Store.Enqueue(ctx, j2); err == nil {
		t.Fatal("Enqueue succeeded while listing fails")
	}
	if _, err := r.bridge.Store.JobState(ctx, j2.JobID, nil, t0); err == nil {
		t.Fatal("JobState reported a state while storage fails")
	}
}

func TestHeartbeatJudgement(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		setup  func(r *rig)
		online bool
		reason string
	}{
		{"missing", func(*rig) {}, false, "never reported"},
		{"fresh", func(r *rig) { _ = r.agent.Heartbeat(-30*time.Second, "idle", nil) }, true, ""},
		{"exactly 2 minutes", func(r *rig) { _ = r.agent.Heartbeat(-2*time.Minute, "idle", nil) }, true, ""},
		{"stale", func(r *rig) { _ = r.agent.Heartbeat(-121*time.Second, "idle", nil) }, false, "not running"},
		{"clock ahead", func(r *rig) { _ = r.agent.Heartbeat(2*time.Minute, "idle", nil) }, false, "clock is ahead"},
		{"slightly ahead", func(r *rig) { _ = r.agent.Heartbeat(30*time.Second, "idle", nil) }, true, ""},
		{"exiting", func(r *rig) { _ = r.agent.Heartbeat(0, "exiting", nil) }, false, "shut down"},
		{"other bucket", func(r *rig) { r.agent.ClaimBucket = "other-bucket"; _ = r.agent.Heartbeat(0, "idle", nil) }, false, "different jobs bucket"},
		{"forged", func(r *rig) {
			_ = r.client.WriteFile(ctx, jobsBucket, devicejobs.HeartbeatKey, []byte(`{"v":1,"kid":"72eb33c6","payload":"{}","sig":"`+strings.Repeat("0", 64)+`"}`))
		}, false, "failed verification"},
		{"unknown key", func(r *rig) {
			kr, _ := devicejobs.NewKeyring(keyB, "")
			r.agent.Keys = kr
			_ = r.agent.Heartbeat(0, "idle", nil)
		}, false, "key this server does not know"},
		{"too large", func(r *rig) {
			_ = r.client.WriteFile(ctx, jobsBucket, devicejobs.HeartbeatKey, make([]byte, devicejobs.MaxHeartbeatBytes+1))
		}, false, "could not be read"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			c.setup(r)
			hv, err := r.bridge.Store.Heartbeat(ctx, t0)
			if err != nil {
				t.Fatal(err)
			}
			if hv.Online != c.online || !strings.Contains(hv.Reason, c.reason) {
				t.Fatalf("online=%v reason=%q, want %v %q", hv.Online, hv.Reason, c.online, c.reason)
			}
		})
	}
}

// ---- push ----

func TestPushDryRunThenConfirm(t *testing.T) {
	r := newRig(t)
	r.online(t)
	ctx := context.Background()
	plan, err := r.bridge.PlanPush(ctx, sp(), req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Token == "" || plan.SlotNote != "" || !plan.Bridge.Online {
		t.Fatalf("plan = %+v", plan)
	}
	if len(r.client.Keys(jobsBucket)) != 1 { // heartbeat only: a dry run writes nothing
		t.Fatalf("dry run wrote objects: %v", r.client.Keys(jobsBucket))
	}
	id, existing, err := r.bridge.ConfirmPush(ctx, sp(), req, plan.Token)
	if err != nil || existing || id != devicejobs.JobIDFromToken(plan.Token) {
		t.Fatalf("confirm: id=%s existing=%v err=%v", id, existing, err)
	}
	// A repeat confirm is idempotent: same job, nothing new written.
	before := r.client.Keys(jobsBucket)
	id2, existing, err := r.bridge.ConfirmPush(ctx, sp(), req, plan.Token)
	if err != nil || !existing || id2 != id || len(r.client.Keys(jobsBucket)) != len(before) {
		t.Fatalf("repeat confirm: id=%s existing=%v err=%v", id2, existing, err)
	}
	if v, _ := r.bridge.Job(ctx, id); v.State != devicejobs.StateQueued {
		t.Fatalf("state = %s", v.State)
	}
	cur := id
	_ = r.agent.Heartbeat(0, "busy", &cur)
	if v, _ := r.bridge.Job(ctx, id); v.State != devicejobs.StateRunning {
		t.Fatalf("state with busy heartbeat = %s", v.State)
	}
	if n, err := r.agent.RunOnce(); err != nil || n != 1 {
		t.Fatalf("agent ran %d jobs: %v (outcomes %v)", n, err, r.agent.Outcomes)
	}
	var args devicejobs.PushArgs
	if err := json.Unmarshal(r.agent.Executed[0].Args, &args); err != nil || args.Slot != "20A" || args.Export.Format != "qc2-device-export/1" {
		t.Fatalf("executed args %+v (%v)", args, err)
	}
	v, err := r.bridge.Wait(ctx, id, time.Second)
	if err != nil || v.State != devicejobs.StatusSucceeded || v.Result.Push == nil {
		t.Fatalf("final view %+v err %v", v, err)
	}
	// Confirming again after it ran still names the same job and queues nothing.
	if id3, existing, err := r.bridge.ConfirmPush(ctx, sp(), req, plan.Token); err != nil || !existing || id3 != id {
		t.Fatalf("confirm after done: %s %v %v", id3, existing, err)
	}
	if r.client.Keys(jobsBucket)[0] == devicejobs.PendingKey(id) {
		t.Fatal("re-queued after completion")
	}
}

func TestConfirmRefusals(t *testing.T) {
	ctx := context.Background()
	t.Run("preset changed since the dry run", func(t *testing.T) {
		r := newRig(t)
		r.online(t)
		plan, _ := r.bridge.PlanPush(ctx, sp(), req)
		e := devicejobstest.FixtureExport()
		e.Payload = json.RawMessage(strings.Replace(string(e.Payload), `"wire":0.45`, `"wire":0.55`, 1))
		r.exp.Set(e)
		_, _, err := r.bridge.ConfirmPush(ctx, sp(), req, plan.Token)
		if !strings.Contains(userMsg(t, err), "changed") {
			t.Fatal(err)
		}
	})
	t.Run("different slot", func(t *testing.T) {
		r := newRig(t)
		r.online(t)
		plan, _ := r.bridge.PlanPush(ctx, sp(), req)
		other := req
		other.Slot = "20B"
		_, _, err := r.bridge.ConfirmPush(ctx, sp(), other, plan.Token)
		if !strings.Contains(userMsg(t, err), "do not match") && !strings.Contains(userMsg(t, err), "does not match") {
			t.Fatal(err)
		}
	})
	t.Run("expired token", func(t *testing.T) {
		r := newRig(t)
		r.online(t)
		plan, _ := r.bridge.PlanPush(ctx, sp(), req)
		*r.clock = t0.Add(11 * time.Minute)
		_ = r.agent.Heartbeat(0, "idle", nil)
		_, _, err := r.bridge.ConfirmPush(ctx, sp(), req, plan.Token)
		if !strings.Contains(userMsg(t, err), "expired") {
			t.Fatal(err)
		}
	})
	t.Run("bridge offline", func(t *testing.T) {
		r := newRig(t)
		plan, err := r.bridge.PlanPush(ctx, sp(), req)
		if err != nil || plan.Token == "" || plan.Bridge.Online {
			t.Fatalf("dry run offline: %+v %v", plan, err)
		}
		_, _, err = r.bridge.ConfirmPush(ctx, sp(), req, plan.Token)
		if !strings.Contains(userMsg(t, err), "start the bridge") && !strings.Contains(userMsg(t, err), "never reported") {
			t.Fatal(err)
		}
		if len(r.client.Keys(jobsBucket)) != 0 {
			t.Fatal("queued while offline")
		}
	})
	t.Run("slot outside the bridge's range", func(t *testing.T) {
		r := newRig(t)
		r.online(t)
		far := req
		far.Slot = "1A"
		plan, _ := r.bridge.PlanPush(ctx, sp(), far)
		if !strings.Contains(plan.SlotNote, "outside") {
			t.Fatalf("note = %q", plan.SlotNote)
		}
		_, _, err := r.bridge.ConfirmPush(ctx, sp(), far, plan.Token)
		if !strings.Contains(userMsg(t, err), "outside") {
			t.Fatal(err)
		}
	})
	t.Run("unresolved items", func(t *testing.T) {
		r := newRig(t)
		r.online(t)
		e := devicejobstest.FixtureExport()
		e.Unresolved = []devicejobs.Unresolved{{Block: "Spring", Reason: "unmeasured bound"}}
		r.exp.Set(e)
		plan, err := r.bridge.PlanPush(ctx, sp(), req)
		if err != nil || plan.Token != "" {
			t.Fatalf("plan with unresolved: token %q err %v", plan.Token, err)
		}
	})
	t.Run("export unavailable", func(t *testing.T) {
		r := newRig(t)
		r.bridge.Exporter = devicejobs.UnavailableExporter{}
		_, err := r.bridge.PlanPush(ctx, sp(), req)
		if !strings.Contains(userMsg(t, err), "not available yet") {
			t.Fatal(err)
		}
	})
	t.Run("invalid export", func(t *testing.T) {
		r := newRig(t)
		e := devicejobstest.FixtureExport()
		e.Payload = json.RawMessage(`[1,2]`)
		r.exp.Set(e)
		if _, err := r.bridge.PlanPush(ctx, sp(), req); err == nil {
			t.Fatal("array payload accepted")
		}
	})
	t.Run("queue caps", func(t *testing.T) {
		r := newRig(t)
		r.online(t)
		plan, _ := r.bridge.PlanPush(ctx, sp(), req)
		if _, _, err := r.bridge.ConfirmPush(ctx, sp(), req, plan.Token); err != nil {
			t.Fatal(err)
		}
		// A second push (different name, so a different token) to the same slot.
		again := req
		again.Name = "Texas Blues 2"
		plan2, _ := r.bridge.PlanPush(ctx, sp(), again)
		_, _, err := r.bridge.ConfirmPush(ctx, sp(), again, plan2.Token)
		if !strings.Contains(userMsg(t, err), "already waiting") {
			t.Fatal(err)
		}
		for i := 0; i < devicejobs.MaxPendingJobs-1; i++ {
			if _, err := r.bridge.QueueRead(ctx, devicejobs.KindListSlots); err != nil {
				t.Fatal(err)
			}
		}
		_, err = r.bridge.QueueRead(ctx, devicejobs.KindListSlots)
		if !strings.Contains(userMsg(t, err), "already waiting for the bridge") {
			t.Fatal(err)
		}
	})
}

// ---- reads and job states ----

func TestReadJobsAndStates(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	if _, err := r.bridge.QueueRead(ctx, devicejobs.KindReadPreset); err == nil {
		t.Fatal("read queued while offline")
	}
	r.online(t)
	id, err := r.bridge.QueueRead(ctx, devicejobs.KindReadPreset)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := r.bridge.Wait(ctx, id, 20*time.Millisecond); v.State != devicejobs.StateQueued {
		t.Fatalf("before the agent runs: %s", v.State)
	}
	if _, err := r.agent.RunOnce(); err != nil {
		t.Fatal(err)
	}
	v, err := r.bridge.Job(ctx, id)
	if err != nil || v.State != devicejobs.StatusSucceeded || v.Result.Readback == nil {
		t.Fatalf("%+v %v", v, err)
	}

	// A forged result (signed with another key) is unverified.
	forged := "11111111-2222-4333-8444-555555555555"
	kr, _ := devicejobs.NewKeyring(keyB, "")
	p, _ := json.Marshal(devicejobs.Result{V: 1, Type: devicejobs.TypeResult, JobID: forged, Bucket: jobsBucket, Status: "succeeded"})
	env, _ := kr.Seal(devicejobs.TagResult, p)
	_ = r.client.WriteFile(ctx, jobsBucket, devicejobs.DoneKey(forged), env)
	if v, _ := r.bridge.Job(ctx, forged); v.State != devicejobs.StateUnverified || v.Result != nil {
		t.Fatalf("forged result: %+v", v)
	}
	// A correctly signed result for a different job id is unverified too.
	moved := "11111111-2222-4333-8444-555555555556"
	data, _ := r.client.ReadFile(ctx, jobsBucket, devicejobs.DoneKey(id))
	_ = r.client.WriteFile(ctx, jobsBucket, devicejobs.DoneKey(moved), data)
	if v, _ := r.bridge.Job(ctx, moved); v.State != devicejobs.StateUnverified {
		t.Fatalf("moved result: %s", v.State)
	}
	// Unknown id.
	if v, _ := r.bridge.Job(ctx, "11111111-2222-4333-8444-555555555557"); v.State != devicejobs.StateNotFound {
		t.Fatalf("unknown: %s", v.State)
	}
	if _, err := r.bridge.Job(ctx, "../heartbeat"); err == nil {
		t.Fatal("path-like id accepted")
	}

	// A job nobody picked up is reported as expired once past expiry + grace,
	// and the agent answers a late pick-up with a signed "expired" result.
	id2, err := r.bridge.QueueRead(ctx, devicejobs.KindListSlots)
	if err != nil {
		t.Fatal(err)
	}
	*r.clock = t0.Add(devicejobs.DefaultJobTTL + 3*time.Minute)
	if v, _ := r.bridge.Job(ctx, id2); v.State != devicejobs.StateUnclaimed {
		t.Fatalf("unclaimed: %s", v.State)
	}
	if _, err := r.agent.RunOnce(); err != nil {
		t.Fatal(err)
	}
	if v, _ := r.bridge.Job(ctx, id2); v.State != devicejobs.StatusExpired {
		t.Fatalf("after late pick-up: %s", v.State)
	}
}

func TestSlotAllowed(t *testing.T) {
	cases := []struct {
		slot   string
		ranges []string
		want   bool
	}{
		{"20A", []string{"20A-25H"}, true},
		{"25H", []string{"20A-25H"}, true},
		{"26A", []string{"20A-25H"}, false},
		{"19H", []string{"20A-25H"}, false},
		{"3C", []string{"3C"}, true},
		{"1A", nil, true},                 // no ranges: no early check
		{"1A", []string{"garbage"}, true}, // unparseable: no early check
		{"33A", nil, false},
	}
	for _, c := range cases {
		if got := devicejobs.SlotAllowed(c.slot, c.ranges); got != c.want {
			t.Errorf("SlotAllowed(%s, %v) = %v", c.slot, c.ranges, got)
		}
	}
}

func TestResultKindAndPendingFlood(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	r.online(t)
	// A correctly signed result whose kind is not a contract value is unverified.
	id := "11111111-2222-4333-8444-555555555555"
	kr, _ := devicejobs.NewKeyring(keyA, "")
	p, _ := json.Marshal(devicejobs.Result{V: 1, Type: devicejobs.TypeResult, JobID: id, Bucket: jobsBucket,
		Kind: "x\n# injected <img>", Status: devicejobs.StatusSucceeded})
	env, _ := kr.Seal(devicejobs.TagResult, p)
	_ = r.client.WriteFile(ctx, jobsBucket, devicejobs.DoneKey(id), env)
	if v, _ := r.bridge.Job(ctx, id); v.State != devicejobs.StateUnverified || v.Result != nil {
		t.Fatalf("bad kind: %+v", v)
	}
	// A flood of junk pending objects is scanned only up to a bound, and does
	// not block real work.
	for i := 0; i < 500; i++ {
		_ = r.client.WriteFile(ctx, jobsBucket, devicejobs.PendingKey(fmt.Sprintf("00000000-0000-4000-8000-%012d", i)), []byte("junk"))
	}
	if _, err := r.bridge.QueueRead(ctx, devicejobs.KindListSlots); err != nil {
		t.Fatalf("queue with junk pending objects: %v", err)
	}
}

// Old signed jobs copied back into jobs/pending/ (expired) don't count against
// the queue, and an oversized object is refused without being returned.
func TestPendingIgnoresExpiredCopiesAndOversize(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	r.online(t)
	var copies [][]byte
	for i := 0; i < devicejobs.MaxPendingJobs; i++ {
		id, err := r.bridge.QueueRead(ctx, devicejobs.KindListSlots)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := r.client.ReadFile(ctx, jobsBucket, devicejobs.PendingKey(id))
		copies = append(copies, data)
	}
	if _, err := r.bridge.QueueRead(ctx, devicejobs.KindListSlots); err == nil {
		t.Fatal("queue cap not enforced")
	}
	*r.clock = t0.Add(devicejobs.DefaultJobTTL + time.Minute)
	_ = r.agent.Heartbeat(0, "idle", nil)
	if _, err := r.bridge.QueueRead(ctx, devicejobs.KindListSlots); err != nil {
		t.Fatalf("expired jobs still fill the queue: %v", err)
	}
	_ = copies
	big := make([]byte, devicejobs.MaxHeartbeatBytes+10)
	_ = r.client.WriteFile(ctx, jobsBucket, devicejobs.HeartbeatKey, big)
	if hv, err := r.bridge.Store.Heartbeat(ctx, *r.clock); err != nil || hv.Online || !strings.Contains(hv.Reason, "could not be read") {
		t.Fatalf("oversized heartbeat: %+v %v", hv, err)
	}
	if r.client.LimitedReads() == 0 {
		t.Fatal("the store did not use the bounded read")
	}
}

// A failed read of an existing result or pending job is a storage error, not
// a terminal state: Job returns it, and Wait keeps polling and recovers.
func TestTransientReadErrorsAreNotTerminal(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	r.online(t)
	id, err := r.bridge.QueueRead(ctx, devicejobs.KindListSlots)
	if err != nil {
		t.Fatal(err)
	}
	r.client.SetFailRead(true)
	if v, err := r.bridge.Store.JobState(ctx, id, nil, t0); err == nil {
		t.Fatalf("pending job with a failing read: state %s, want an error", v.State)
	}
	r.client.SetFailRead(false)
	if _, err := r.agent.RunOnce(); err != nil {
		t.Fatal(err)
	}
	r.client.SetFailRead(true)
	if v, err := r.bridge.Store.JobState(ctx, id, nil, t0); err == nil {
		t.Fatalf("result with a failing read: state %s, want an error", v.State)
	}
	// Wait rides out the hiccup: the first poll's reads fail, later ones work.
	r.client.SetFailRead(false)
	r.client.FailNextReads(1)
	v, err := r.bridge.Wait(ctx, id, 2*time.Second)
	if err != nil || v.State != devicejobs.StatusSucceeded {
		t.Fatalf("Wait across a transient failure: %+v %v", v, err)
	}
	// Oversized is permanent: unverified, not an error.
	_ = r.client.WriteFile(ctx, jobsBucket, devicejobs.DoneKey(id), make([]byte, devicejobs.MaxResultBytes+1))
	if v, err := r.bridge.Store.JobState(ctx, id, nil, t0); err != nil || v.State != devicejobs.StateUnverified {
		t.Fatalf("oversized result: %+v %v", v, err)
	}
}

// Wait lasts the full max even when max is shorter than the poll gap, and
// catches a result that lands in the last partial gap.
func TestWaitUsesTheWholeWindow(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	r.online(t)
	r.bridge.PollGap = time.Second
	id, err := r.bridge.QueueRead(ctx, devicejobs.KindListSlots)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	v, err := r.bridge.Wait(ctx, id, 80*time.Millisecond)
	if el := time.Since(start); err != nil || v.State != devicejobs.StateQueued || el < 80*time.Millisecond || el > 5*time.Second {
		t.Fatalf("short wait: state %v err %v after %v, want queued after >=80ms (and well under the 1s gap's multiples)", v, err, el)
	}
	// The agent answers 50ms in; with a 1s gap only the final poll at the
	// 150ms deadline can see it.
	timer := time.AfterFunc(50*time.Millisecond, func() { _, _ = r.agent.RunOnce() })
	defer timer.Stop()
	v, err = r.bridge.Wait(ctx, id, 150*time.Millisecond)
	if err != nil || v.State != devicejobs.StatusSucceeded {
		t.Fatalf("result in the last partial gap: %+v %v", v, err)
	}
}
