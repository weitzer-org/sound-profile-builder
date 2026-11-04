package evalcritic

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evaltestutil"
)

const (
	pro   = "gemini-3.1-pro-preview"
	flash = "gemini-3.5-flash"
)

var pins = map[string]string{flash: "gemini-3.6-flash"}
var cfg = Config{Model: flash, ExpectServe: "gemini-3.6-flash"}

func okCall(model, served string) []agents.CallRecord {
	return []agents.CallRecord{{Agent: "Preset Critic", AttemptModel: model, ServedModel: served, FinishReason: "STOP", Success: true, InputTokens: 900, OutputTokens: 40, LatencyMs: 800}}
}

func fakeCaller(resp string, calls []agents.CallRecord, err error, seen *string) Caller {
	return func(_ context.Context, in string) (string, []agents.CallRecord, error) {
		if seen != nil {
			*seen = in
		}
		return resp, calls, err
	}
}

func writeOut(t *testing.T, body string) Output {
	t.Helper()
	p := t.TempDir() + "/o.html"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return Output{Key: evalcheck.RunKey{Scenario: "S5", Query: "07_Edge", Rep: 1}, Path: p}
}

func TestCriticInput_StripsPipelineCriticNotesAndRenderedHTML(t *testing.T) {
	body := evaltestutil.Envelope(evaltestutil.Block{Type: "Amplifier", Model: "Iba Green",
		Rationale: "Pre-trained capture. ⚠️ Critic: this is a native pedal ⚠️ Verify: keep this one"})
	var env map[string]any
	_ = json.Unmarshal([]byte(body), &env)
	env["final_html_payload"] = "<table>rendered</table>"
	raw, _ := json.Marshal(env)

	in, ids, err := CriticInput(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(in, "native pedal") || strings.Contains(in, "final_html_payload") || strings.Contains(in, "rendered") {
		t.Errorf("re-critic input leaked the pipeline Critic's note or the rendered table: %s", in)
	}
	if !strings.Contains(in, "keep this one") || !strings.Contains(in, "Pre-trained capture.") {
		t.Errorf("non-Critic content must survive: %s", in)
	}
	if !ids["G\x00b1"] {
		t.Errorf("block ids = %v", ids)
	}
}

func TestParse_MissingIssuesIsAFailureNotZero(t *testing.T) {
	for _, bad := range []string{"", "{}", "not json", `{"issues": null}`, `[]`} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) must fail: a malformed response cannot score as 0 defects", bad)
		}
	}
	if is, err := Parse("```json\n{\"issues\": []}\n```"); err != nil || len(is) != 0 {
		t.Errorf("fenced empty issues: %v %v", is, err)
	}
}

func TestReview_CountsMatchedFindingsAndSeparatesUnmatched(t *testing.T) {
	out := writeOut(t, evaltestutil.CleanOutput())
	resp := `{"issues":[
	  {"guitar":"G","block_id":"b1","issue":"a","severity":"High"},
	  {"guitar":"G","block_id":"b2","issue":"b","severity":"low"},
	  {"guitar":"G","block_id":"nope","issue":"hallucinated id","severity":"high"},
	  {"guitar":"X","block_id":"b1","issue":"hallucinated guitar","severity":"high"}]}`
	var seen string
	r := Review(context.Background(), cfg, out, fakeCaller(resp, okCall(flash, "gemini-3.6-flash"), nil, &seen))
	if !r.OK || r.Defects != 2 || r.Unmatched != 2 || r.BySeverity["high"] != 1 || r.BySeverity["low"] != 1 {
		t.Fatalf("result = %+v", r)
	}
	if !strings.Contains(seen, "structured_payload") {
		t.Error("caller did not receive the prepared input")
	}
}

func TestReview_NeverScoresAFailedOrNonUniformCallAsZero(t *testing.T) {
	out := writeOut(t, evaltestutil.CleanOutput())
	good := `{"issues":[]}`
	cases := []struct {
		name  string
		resp  string
		calls []agents.CallRecord
		err   error
		want  string
	}{
		{"api error", "", nil, errors.New("boom"), "critic call"},
		{"wrong served model (alias drift)", good, okCall(flash, "gemini-3.5-flash-lite"), nil, "served"},
		{"fallback attempt", good, okCall(pro, pro), nil, "fallback"},
		{"unparseable", "garbage", okCall(flash, "gemini-3.6-flash"), nil, "parse"},
		{"no call record", good, nil, nil, "no call record"},
		{"failed attempt", good, []agents.CallRecord{{AttemptModel: flash, ServedModel: "gemini-3.6-flash", Success: false, Error: "truncated"}}, nil, "attempt failed"},
	}
	for _, c := range cases {
		r := Review(context.Background(), cfg, out, fakeCaller(c.resp, c.calls, c.err, nil))
		if r.OK || r.Defects != 0 || !strings.Contains(r.Error, c.want) {
			t.Errorf("%s: %+v, want OK=false with error containing %q", c.name, r, c.want)
		}
	}
	if r := Review(context.Background(), cfg, Output{Key: out.Key, Path: out.Path + ".missing"}, fakeCaller(good, nil, nil, nil)); r.OK {
		t.Error("unreadable output must not be OK")
	}
}

func batchFixture(t *testing.T) *evalcheck.Batch {
	t.Helper()
	dir := t.TempDir()
	r0 := evaltestutil.Routing(pro, flash, flash)
	r5 := evaltestutil.Routing(pro, flash, "gemini-4-argon")
	ms := []evalcheck.ManifestScenario{{ID: "S0", AgentModels: r0, Pins: pins}, {ID: "S5", AgentModels: r5, Pins: pins}}
	mk := func(id string, rep int, route map[string]string) evaltestutil.Run {
		return evaltestutil.Run{Scenario: id, Query: "07_Edge", Rep: rep, Success: true, IntegrityOK: true,
			Calls: evaltestutil.Calls(route, pins, 10, 10, 0, 10), Output: evaltestutil.CleanOutput()}
	}
	bad := mk("S5", 2, r5)
	bad.Calls[3].ServedModel = "gemini-2.5-pro" // fallback-served; runs.jsonl still claims integrity_ok
	failed := mk("S5", 3, r5)
	failed.Success, failed.Output = false, ""
	evaltestutil.WriteBatch(t, dir, evaltestutil.Manifest("B", false, 3, []string{"07_Edge"}, ms),
		[]evaltestutil.Run{mk("S0", 1, r0), mk("S5", 1, r5), bad, failed})
	b, err := evalcheck.LoadBatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPlan_OneCallPerScoreableOutput(t *testing.T) {
	b := batchFixture(t)
	outs, skipped := Plan(b, nil, false)
	if len(outs) != 2 || len(skipped) != 2 {
		t.Fatalf("outs=%v skipped=%v: want S0 rep1 + S5 rep1, skipping the fallback-served and the failed run", outs, skipped)
	}
	if outs, _ := Plan(b, map[string]bool{"S0": true}, false); len(outs) != 1 {
		t.Errorf("scenario filter: %v", outs)
	}
	if outs, _ := Plan(b, nil, true); len(outs) != 3 { // the failed run has no output file
		t.Errorf("include-unscoreable: %d", len(outs))
	}
}

func TestResumeSkipsDoneAndRetriesFailures(t *testing.T) {
	path := t.TempDir() + "/recritic.jsonl"
	a := Result{Scenario: "S0", Query: "q", Rep: 1, OK: true, Defects: 2}
	f := Result{Scenario: "S5", Query: "q", Rep: 1, OK: false, Error: "boom"}
	f2 := Result{Scenario: "S5", Query: "q", Rep: 1, OK: true, Defects: 1}
	for _, r := range []Result{a, f} {
		if err := AppendResult(path, r); err != nil {
			t.Fatal(err)
		}
	}
	done, err := LoadDone(path)
	if err != nil || len(done) != 1 {
		t.Fatalf("done = %v %v: a failed result must be retried, not skipped", done, err)
	}
	_ = AppendResult(path, f2)
	rs, _ := ReadResults(path)
	if len(rs) != 2 || !rs[1].OK || rs[1].Defects != 1 {
		t.Errorf("a later success must supersede the earlier failure: %+v", rs)
	}
	if d, err := LoadDone(path + ".none"); err != nil || len(d) != 0 {
		t.Error("missing file = nothing done")
	}
}

func TestResolve(t *testing.T) {
	m := &evalcheck.Manifest{Scenarios: []evalcheck.ManifestScenario{{ID: "S0", AgentModels: map[string]string{"13_critic": flash}, Pins: pins}}}
	if c, src := Resolve(m, "", ""); c.Model != flash || c.ExpectServe != "gemini-3.6-flash" || !strings.Contains(src, "manifest") {
		t.Errorf("manifest: %+v %s", c, src)
	}
	if c, _ := Resolve(nil, "", ""); c.Model != evalcheck.S0CriticModel || c.ExpectServe != evalcheck.S0CriticServed {
		t.Errorf("default: %+v", c)
	}
	if c, _ := Resolve(m, "gemini-3.8-flash", ""); c.ExpectServe != "gemini-3.8-flash" {
		t.Errorf("explicit model w/o pin must expect itself: %+v", c)
	}
}

func TestStripCriticNotes(t *testing.T) {
	if got := evalcheck.StripCriticNotes("a ⚠️ Critic: x ⚠️ Verify: y ⚠️ my own"); got != "a ⚠️ Verify: y ⚠️ my own" {
		t.Errorf("got %q", got)
	}
}
