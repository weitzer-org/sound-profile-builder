package devicejobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
)

// editPlanText is a valid qc2-device-edit/1 plan, as json.Marshal of the Go types writes it.
func editPlanText(t *testing.T) []byte {
	t.Helper()
	p := devicejobs.EditPayload{
		Format:   devicejobs.EditFormat,
		Catalog:  devicejobs.EditCatalogRef{CorOS: "4.1", SHA256: strings.Repeat("ab", 32)},
		Snapshot: "3f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e02",
		Expect: devicejobs.EditExpect{FolderKey: "/media/p4/Presets/Sounds profiles", Position: 24, Slot: "4A",
			Name: "Café <Lead>", Layout: [][3]int{{0, 0, 1120}, {0, 1, 12045}, {0, 2, 8006}, {1, 0, 5}}},
		ParamOps: []devicejobs.EditParamOp{{Row: 0, Col: 0, ModelID: 1120, Index: 0, Label: "Ø GAIN", Evidence: []string{"taper:g4"},
			SceneMode: true, Expect: [8]float64{0.45, 0.45, 0.45, 0.45, 0.45, 0.45, 0.45, 0.45},
			Wire: [8]float64{1, 0.62, 0.45, 0.45, 0.45, 0.45, 0.45, 0.45}}},
		BypassOps: []devicejobs.EditBypassOp{{Row: 0, Col: 2, ModelID: 8006, Why: "removed",
			Bypass: [8]bool{true, true, true, true, true, true, true, true}}},
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "\\u003c") {
		t.Fatalf("expected Go's HTML-safe escaping in the plan text: %s", raw)
	}
	return raw
}

func editArgs(t *testing.T, text []byte) *devicejobs.EditArgs {
	t.Helper()
	tgt, name := "20A", "Texas Blues edit"
	a, err := devicejobs.NewEditArgs("4A", devicejobs.EditModeCopy, &tgt, &name, devicejobs.PushSource{PresetID: "device-1", Guitar: "From the unit"}, text)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// The Go types' own JSON passes the strict parser, round-trips, and hashes over the exact text.
func TestEditPayloadFromGoTypes(t *testing.T) {
	text := editPlanText(t)
	p, err := devicejobs.ParseEditPayload(text)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := json.Marshal(p); string(again) != string(text) {
		t.Fatalf("round trip changed the text:\n%s\n%s", text, again)
	}
	a := editArgs(t, text)
	if a.Plan.SHA256 != devicejobs.EditPlanSHA256(devicejobs.EditFormat, text) || a.Plan.Payload != string(text) {
		t.Fatalf("args %+v", a.Plan)
	}
	// The mode rules, from the Go side.
	src := devicejobs.PushSource{}
	tgt, name := "20A", "N"
	same := "4A"
	for _, c := range []struct {
		mode       string
		tgt, name  *string
		wantReject bool
	}{
		{devicejobs.EditModeLive, nil, nil, false},
		{devicejobs.EditModeOverwrite, nil, nil, false},
		{devicejobs.EditModeCopy, &tgt, &name, false},
		{devicejobs.EditModeCopy, nil, &name, true},
		{devicejobs.EditModeCopy, &same, &name, true},
		{devicejobs.EditModeLive, nil, &name, true},
		{devicejobs.EditModeOverwrite, &tgt, nil, true},
		{"delete", nil, nil, true},
	} {
		_, err := devicejobs.NewEditArgs("4A", c.mode, c.tgt, c.name, src, text)
		if (err != nil) != c.wantReject {
			t.Errorf("mode %s tgt=%v name=%v: err %v", c.mode, c.tgt != nil, c.name != nil, err)
		}
	}
	if _, err := devicejobs.NewEditArgs("4B", devicejobs.EditModeLive, nil, nil, src, text); err == nil {
		t.Error("a plan for 4A accepted for slot 4B")
	}
	// Errors never quote the plan.
	hostile := strings.Replace(string(text), `"taper:g4"`, `"EVIL![x](http://e/)"`, 1)
	if _, err := devicejobs.ParseEditPayload([]byte(hostile)); err == nil || strings.Contains(err.Error(), "EVIL") {
		t.Fatalf("hostile evidence: %v", err)
	}
	for _, bad := range []string{`{"EVILKEY":1,"EVILKEY":2}`, `{"a":1e999999}`, string(text) + ` "EVIL"`} {
		if _, err := devicejobs.ParseEditPayload([]byte(bad)); err == nil || strings.Contains(err.Error(), "EVIL") || strings.Contains(err.Error(), "999") {
			t.Errorf("strict-JSON error quotes the plan: %v", err)
		}
	}
}

// A job signed under one tag never opens as the other version.
func TestOpenJobBindsTagAndVersion(t *testing.T) {
	kr, _ := devicejobs.NewKeyring(keyA, "")
	text := editPlanText(t)
	args, _ := json.Marshal(editArgs(t, text))
	j := devicejobs.Job{V: 2, Type: devicejobs.TypeJob, JobID: "7a2d9c41-5e6f-4a1b-8c3d-9e0f1a2b3c4d", Bucket: jobsBucket,
		Kind: devicejobs.KindEditPreset, CreatedAt: "2026-11-05T12:00:00Z", ExpiresAt: "2026-11-05T12:15:00Z", Args: args}
	p, _ := json.Marshal(j)
	for _, c := range []struct {
		tag  string
		want devicejobs.Outcome
	}{{devicejobs.TagJobV2, devicejobs.OutcomeOK}, {devicejobs.TagJob, devicejobs.OutcomeBadPayload},
		{devicejobs.TagResult, devicejobs.OutcomeBadSignature}, {devicejobs.TagHeartbeat, devicejobs.OutcomeBadSignature}} {
		env, _ := kr.Seal(c.tag, p)
		got, err := kr.OpenJob(env)
		if devicejobs.OutcomeOf(err) != c.want {
			t.Errorf("sealed under %s: %v, want %s", c.tag, err, c.want)
		}
		if err == nil && (got.V != 2 || got.Kind != devicejobs.KindEditPreset) {
			t.Errorf("parsed %+v", got)
		}
	}
	// A v1 job still opens exactly as before, and not under the v2 tag.
	v1 := []byte(`{"v":1,"type":"qc2-device-job","job_id":"6f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e01","bucket":"qc2-device-jobs-test","kind":"list_slots","created_at":"2026-11-05T12:00:00Z","expires_at":"2026-11-05T12:15:00Z","args":{}}`)
	env, _ := kr.Seal(devicejobs.TagJob, v1)
	if got, err := kr.OpenJob(env); err != nil || got.V != 1 {
		t.Fatalf("v1 job: %+v %v", got, err)
	}
	env, _ = kr.Seal(devicejobs.TagJobV2, v1)
	if _, err := kr.OpenJob(env); devicejobs.OutcomeOf(err) != devicejobs.OutcomeBadPayload {
		t.Fatalf("v1 job under the v2 tag: %v", err)
	}
}

// QC-2 queues an edit job under the v2 tag, sees it as pending, and reads the agent's answer
// (refused/edit_not_enabled) instead of dropping an unknown kind.
func TestEditJobQueuedAndAnswered(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	r.online(t)
	text := editPlanText(t)
	id := "7a2d9c41-5e6f-4a1b-8c3d-9e0f1a2b3c4d"
	j, err := r.bridge.Store.NewJob(id, devicejobs.KindEditPreset, editArgs(t, text), t0)
	if err != nil || j.V != 2 {
		t.Fatalf("NewJob: %+v %v", j, err)
	}
	if err := r.bridge.Store.Enqueue(ctx, j); err != nil {
		t.Fatal(err)
	}
	pend, err := r.bridge.Store.PendingJobs(ctx, t0)
	if err != nil || len(pend) != 1 || pend[0].Kind != devicejobs.KindEditPreset {
		t.Fatalf("pending: %+v %v", pend, err)
	}
	if v, _ := r.bridge.Job(ctx, id); v.State != devicejobs.StateQueued {
		t.Fatalf("before the agent runs: %s", v.State)
	}
	if _, err := r.agent.RunOnce(); err != nil {
		t.Fatal(err)
	}
	v, err := r.bridge.Job(ctx, id)
	if err != nil || v.State != devicejobs.StatusRefused || v.Result == nil || v.Result.Kind != devicejobs.KindEditPreset ||
		v.Result.ReasonCode != "edit_not_enabled" || v.Result.Edit != nil {
		t.Fatalf("%+v %v", v, err)
	}
	if o := r.agent.Outcomes[devicejobs.PendingKey(id)]; o != devicejobs.OutcomeOK {
		t.Fatalf("the agent's check: %s", o)
	}
}

func TestEnqueueRefusesBadEditJobs(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	text := editPlanText(t)
	id := "7a2d9c41-5e6f-4a1b-8c3d-9e0f1a2b3c4d"
	// A hash that doesn't match its text.
	a := editArgs(t, text)
	a.Plan.SHA256 = strings.Repeat("0", 64)
	j, _ := r.bridge.Store.NewJob(id, devicejobs.KindEditPreset, a, t0)
	if err := r.bridge.Store.Enqueue(ctx, j); err == nil {
		t.Error("enqueued an edit whose hash does not match")
	}
	// An edit job carrying payload v 1.
	j, _ = r.bridge.Store.NewJob(id, devicejobs.KindEditPreset, editArgs(t, text), t0)
	j.V = 1
	if err := r.bridge.Store.Enqueue(ctx, j); err == nil {
		t.Error("enqueued an edit job as v1")
	}
	// A plan under the 256 KiB plan cap whose sealed job is over the 512 KiB object cap: JSON
	// whitespace newlines in the plan are escaped once in the args and again in the envelope.
	padded := append(append([]byte{}, text...), []byte(strings.Repeat("\n", 200_000))...)
	big, err := r.bridge.Store.NewJob(id, devicejobs.KindEditPreset, editArgs(t, padded), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.bridge.Store.Enqueue(ctx, big); err == nil || !strings.Contains(err.Error(), "sealed job") {
		t.Errorf("oversized sealed job: %v", err)
	}
	if names, _ := r.client.ListFiles(ctx, jobsBucket, devicejobs.PendingPrefix); len(names) != 0 {
		t.Fatalf("something was written: %v", names)
	}
}

func TestEditConfirmToken(t *testing.T) {
	kr, _ := devicejobs.NewKeyring(keyA, "")
	c := devicejobs.EditClaims{PresetID: "device-1", Guitar: "From the unit", Mode: "copy", Slot: "4A", TargetSlot: "20A",
		Name: "Edited", DeviceName: "Texas Blues", Snapshot: "3f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e02", PlanSHA256: strings.Repeat("a", 64)}
	tok, exp := kr.IssueEditConfirm(c, t0)
	if !strings.HasPrefix(tok, "e1.") || exp != t0.Add(devicejobs.ConfirmTTL) {
		t.Fatalf("token %q exp %v", tok, exp)
	}
	if err := kr.CheckEditConfirm(tok, c, t0.Add(time.Minute)); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for name, mut := range map[string]func(*devicejobs.EditClaims){
		"preset":      func(c *devicejobs.EditClaims) { c.PresetID = "p-2" },
		"guitar":      func(c *devicejobs.EditClaims) { c.Guitar = "LP" },
		"mode":        func(c *devicejobs.EditClaims) { c.Mode = "overwrite" },
		"slot":        func(c *devicejobs.EditClaims) { c.Slot = "4B" },
		"target":      func(c *devicejobs.EditClaims) { c.TargetSlot = "20B" },
		"name":        func(c *devicejobs.EditClaims) { c.Name = "Other" },
		"device name": func(c *devicejobs.EditClaims) { c.DeviceName = "Texas Blues 2" },
		"snapshot":    func(c *devicejobs.EditClaims) { c.Snapshot = "3f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e03" },
		"plan":        func(c *devicejobs.EditClaims) { c.PlanSHA256 = strings.Repeat("b", 64) },
		"shift":       func(c *devicejobs.EditClaims) { c.Guitar, c.Mode = "From the unit\",\"copy", "" },
	} {
		d := c
		mut(&d)
		if err := kr.CheckEditConfirm(tok, d, t0); !errors.Is(err, devicejobs.ErrConfirmMismatch) {
			t.Errorf("%s changed: err = %v, want mismatch", name, err)
		}
	}
	if err := kr.CheckEditConfirm(tok, c, exp); !errors.Is(err, devicejobs.ErrConfirmExpired) {
		t.Errorf("at expiry: %v", err)
	}
	far, _ := kr.IssueEditConfirm(c, t0.Add(time.Hour))
	if err := kr.CheckEditConfirm(far, c, t0); !errors.Is(err, devicejobs.ErrConfirmInvalid) {
		t.Errorf("far-future token: %v", err)
	}
	// A push token is never an edit token, and the reverse, even with the same MAC bytes.
	push, _ := kr.IssueConfirm(devicejobs.ConfirmClaims{PresetID: c.PresetID, Guitar: c.Guitar, Slot: c.Slot, Name: c.Name, PlanSHA256: c.PlanSHA256}, t0)
	if err := kr.CheckEditConfirm(push, c, t0); !errors.Is(err, devicejobs.ErrConfirmInvalid) {
		t.Errorf("push token as an edit token: %v", err)
	}
	if err := kr.CheckEditConfirm("e1"+strings.TrimPrefix(push, "c1"), c, t0); !errors.Is(err, devicejobs.ErrConfirmMismatch) {
		t.Errorf("push MAC under the edit prefix: %v", err)
	}
	pc := devicejobs.ConfirmClaims{PresetID: c.PresetID, Guitar: c.Guitar, Slot: c.Slot, Name: c.Name, PlanSHA256: c.PlanSHA256}
	if err := kr.CheckConfirm(tok, pc, t0); !errors.Is(err, devicejobs.ErrConfirmInvalid) {
		t.Errorf("edit token as a push token: %v", err)
	}
	if err := kr.CheckConfirm("c1"+strings.TrimPrefix(tok, "e1"), pc, t0); !errors.Is(err, devicejobs.ErrConfirmMismatch) {
		t.Errorf("edit MAC under the push prefix: %v", err)
	}
	for _, bad := range []string{"", "e1", "e2.1.00", tok + "0", strings.ToUpper(tok), strings.Replace(tok, "e1.", "e1.+", 1), strings.Replace(tok, "e1.", "e1.0", 1)} {
		if err := kr.CheckEditConfirm(bad, c, t0); err == nil {
			t.Errorf("CheckEditConfirm(%q) accepted", bad)
		}
	}
	rotated, _ := devicejobs.NewKeyring(keyB, keyA)
	if err := rotated.CheckEditConfirm(tok, c, t0); err != nil {
		t.Errorf("old-key token after rotation: %v", err)
	}
	id := devicejobs.EditJobIDFromToken(tok)
	if !devicejobs.ValidJobID(id) || id[14] != '8' || id == devicejobs.JobIDFromToken(tok) {
		t.Errorf("EditJobIDFromToken = %s (push derivation %s)", id, devicejobs.JobIDFromToken(tok))
	}
}

// testdata/devicejobs/edit_result.json: every valid result parses, every invalid one is refused
// with the expected reason, and no error quotes the payload.
func TestEditResultFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "devicejobs", "edit_result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Format string `json:"format"`
		Valid  []struct {
			Name    string          `json:"name"`
			Payload json.RawMessage `json:"payload"`
		} `json:"valid"`
		Invalid []struct {
			Name    string          `json:"name"`
			Payload json.RawMessage `json:"payload"`
			Raw     string          `json:"raw"`
			Error   string          `json:"error"`
		} `json:"invalid"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	if fx.Format != devicejobs.EditResultFormat || len(fx.Valid) < 8 || len(fx.Invalid) < 40 {
		t.Fatalf("fixture: %q %d %d", fx.Format, len(fx.Valid), len(fx.Invalid))
	}
	for _, c := range fx.Valid {
		t.Run(c.Name, func(t *testing.T) {
			if _, err := devicejobs.ParseEditResultPayload(c.Payload); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, c := range fx.Invalid {
		t.Run("invalid/"+c.Name, func(t *testing.T) {
			p := c.Payload
			if c.Raw != "" {
				p = json.RawMessage(c.Raw)
			}
			_, err := devicejobs.ParseEditResultPayload(p)
			if err == nil || !strings.Contains(err.Error(), c.Error) {
				t.Fatalf("err = %v, want it to contain %q", err, c.Error)
			}
			if strings.Contains(err.Error(), "http://") || strings.Contains(err.Error(), "Texas") {
				t.Fatalf("error quotes the payload: %v", err)
			}
		})
	}
	// Over the cap: refused for its size, before any decoding (a valid result padded with whitespace).
	valid := string(fx.Valid[0].Payload)
	big := valid + strings.Repeat(" ", devicejobs.MaxEditResultBytes+1-len(valid))
	if _, err := devicejobs.ParseEditResultPayload(json.RawMessage(big)); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("oversized result: %v", err)
	}
	if _, err := devicejobs.ParseEditResultPayload(json.RawMessage(big[:devicejobs.MaxEditResultBytes])); err != nil {
		t.Errorf("a result exactly at the cap: %v", err)
	}
}

// testdata/devicejobs/edit_agent_results.json: every result the Python agent's tests build (E4) parses, and its
// rollback, CAS and save fields fit the plan-independent rules of contract 9.3. The file is rewritten by
// deploy/qc-agent's tests (UPDATE_GOLDENS=1), so the agent cannot drift from this parser.
func TestAgentEditResultsParse(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "devicejobs", "edit_agent_results.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Results map[string]json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	if len(fx.Results) < 25 {
		t.Fatalf("only %d agent results in the fixture", len(fx.Results))
	}
	states := map[string]int{}
	for name, p := range fx.Results {
		r, err := devicejobs.ParseEditResultPayload(p)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if r.Save.Saved {
			t.Errorf("%s: a live edit result says it saved", name)
		}
		states[r.Rollback.State]++
	}
	for _, st := range []string{"not_needed", "restored", "partial", "failed"} {
		if states[st] == 0 {
			t.Errorf("no agent result with rollback state %q", st)
		}
	}
}
