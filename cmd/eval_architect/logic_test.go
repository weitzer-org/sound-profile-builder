package main

import (
	"context"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

func TestGridCountsMatchExpectation(t *testing.T) {
	models := []string{modelProBaseline, modelArgon, modelFlash38}
	jobs := BuildJobs(models, RuleQueries(), 5)
	if len(RuleQueries()) != 3 || len(jobs) != 45 {
		t.Fatalf("queries=%v jobs=%d, want 3 queries and 45 Architect runs", RuleQueries(), len(jobs))
	}
	if jobs[0].Model != modelProBaseline || jobs[14].Model != modelProBaseline || jobs[15].Model != modelArgon {
		t.Fatal("control must run first")
	}
}

func TestBaselineRouting(t *testing.T) {
	r := BaselineRouting()
	if r["1_tone_historian"] != modelProBaseline || r["12_architect"] != modelProBaseline {
		t.Fatalf("pro tier wrong: %v", r)
	}
	for _, k := range []string{"2_sonic_profiler", "6_acoustician", "11_dsp_dispatcher", "13_critic"} {
		if r[k] != modelFlashBaseline {
			t.Fatalf("%s -> %s", k, r[k])
		}
	}
}

func run(model, q string, scoreable, defect bool) RunResult {
	return RunResult{Model: model, Query: q, Scoreable: scoreable, CaptureDefect: defect, ArchSuccess: scoreable}
}

func TestJudge(t *testing.T) {
	var rs []RunResult
	for _, q := range RuleQueries() {
		for i := 0; i < 2; i++ {
			rs = append(rs, run("ctl", q, true, false))
			rs = append(rs, run("bad", q, true, q == "07_Edge" && i == 0))
			rs = append(rs, run("flaky", q, !(q == "09_BB_King" && i == 1), false))
		}
	}
	got := map[string]string{}
	for _, v := range Judge(rs, []string{"ctl", "bad", "flaky"}, 2) {
		got[v.Model+"/"+v.Rule] = v.Verdict
	}
	want := map[string]string{"ctl/R1": Pass, "ctl/R2": Pass, "bad/R1": Fail, "bad/R2": Pass, "flaky/R1": Pass, "flaky/R2": Inconclusive}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %s, want %s", k, got[k], w)
		}
	}
	// A model with fewer runs than expected is never a pass.
	if v := Judge(rs[:3], []string{"ctl"}, 2); v[0].Verdict == Pass {
		t.Error("missing runs counted as PASS")
	}
}

func TestControlOK(t *testing.T) {
	vs := []RuleVerdict{{Model: "c", Rule: "R1", Verdict: Pass}, {Model: "c", Rule: "R2", Verdict: Fail}, {Model: "x", Rule: "R1", Verdict: Fail}}
	if ok, why := ControlOK(vs, "c"); ok || len(why) != 1 {
		t.Fatalf("ok=%v why=%v", ok, why)
	}
	if ok, _ := ControlOK(vs[:1], "c"); !ok {
		t.Fatal("passing control rejected")
	}
}

func TestDefect(t *testing.T) {
	if Defect(evalcheck.Report{}) || !Defect(evalcheck.Report{CaptureClaimOnModeled: 1}) || !Defect(evalcheck.Report{CaptureDBOnModeled: 2}) {
		t.Fatal("Defect wrong")
	}
	// ModeledStyleOnCapture and critic notes are reported but are not the R1/R2 condition.
	if Defect(evalcheck.Report{ModeledStyleOnCapture: 3, CriticFlags: 4}) {
		t.Fatal("non-R fields must not count")
	}
}

func TestFreezeObserverCapturesAndCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	o := &freezeObserver{cancel: cancel}
	o.OnAgentDone("Tone Historian", "m", "out1", agents.AgentUsage{}, nil)
	o.OnAgentDone("Acoustician", "m", "bad", agents.AgentUsage{}, context.Canceled) // failed upstream agents are not frozen
	if o.Captured() || ctx.Err() != nil {
		t.Fatal("must not trigger before the Architect")
	}
	o.OnAgentStart("Mix Engineer", "s", "u")
	if o.Captured() {
		t.Fatal("non-architect start captured")
	}
	o.OnAgentStart(architectRole, "SYS", "USER")
	if !o.Captured() || o.ArchSystem != "SYS" || o.ArchUser != "USER" || ctx.Err() == nil {
		t.Fatalf("captured=%v ctxErr=%v", o.Captured(), ctx.Err())
	}
	if len(o.Upstream) != 1 || o.Upstream[0].Output != "out1" {
		t.Fatalf("upstream %v", o.Upstream)
	}
	var _ agents.RunObserver = o
}

func TestMissingUpstream(t *testing.T) {
	var got []UpstreamOutput
	for _, r := range requiredUpstream {
		got = append(got, UpstreamOutput{Role: r})
	}
	if m := missingUpstream(got); len(m) != 0 {
		t.Fatalf("complete set flagged: %v", m)
	}
	if m := missingUpstream(got[1:]); len(m) != 1 || m[0] != "Tone Historian" {
		t.Fatalf("got %v", m)
	}
}

func TestJudgeForSubset(t *testing.T) {
	rs := []RunResult{run("c", "07_Edge", true, false)}
	vs := JudgeFor(rs, []string{"c"}, 1, []string{"07_Edge"})
	if len(vs) != 1 || vs[0].Rule != "R1" || vs[0].Verdict != Pass {
		t.Fatalf("%+v", vs)
	}
	if ok, _ := ControlOK(vs, "c"); !ok {
		t.Fatal("subset control should pass")
	}
}
