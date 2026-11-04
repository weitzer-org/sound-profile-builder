package evalcost

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evaltestutil"
)

const (
	pro   = "gemini-3.1-pro-preview"
	flash = "gemini-3.5-flash"
	f38   = "gemini-3.8-flash"
	lite  = "gemini-3.5-flash-lite"
)

var pins = map[string]string{flash: "gemini-3.6-flash"}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// smokeFixture: S0 fully priced; S1 all-unpriced Flash tier; S2 failed run; S4 partially priced
// (lite on 8,10,11); S5 has a fallback-served call (integrity); S3 skipped.
func smokeFixture(t *testing.T) *evalcheck.Batch {
	t.Helper()
	dir := t.TempDir()
	route := map[string]map[string]string{
		"S0": evaltestutil.Routing(pro, flash, flash),
		"S1": evaltestutil.Routing(pro, f38, flash),
		"S2": evaltestutil.Routing(f38, flash, flash),
		"S4": evaltestutil.Routing(pro, flash, flash),
		"S5": evaltestutil.Routing(pro, flash, "gemini-4-argon"),
	}
	for _, k := range []string{"8_foh_optimizer", "10_control_mapper", "11_dsp_dispatcher"} {
		route["S4"][k] = lite
	}
	var ms []evalcheck.ManifestScenario
	for _, id := range []string{"S0", "S1", "S2", "S3", "S4", "S5"} {
		sc := evalcheck.ManifestScenario{ID: id, AgentModels: route[id], Pins: pins}
		if id == "S3" {
			sc.Skip = "pro-latest == baseline pro"
		}
		ms = append(ms, sc)
	}
	var runs []evaltestutil.Run
	for _, id := range []string{"S0", "S1", "S2", "S4", "S5"} {
		r := evaltestutil.Run{Scenario: id, Query: "07_Edge", Rep: 1, Success: true, IntegrityOK: true, LatencySec: 100,
			Calls: evaltestutil.Calls(route[id], pins, 1000, 100, 50, 1000), Output: evaltestutil.CleanOutput()}
		switch id {
		case "S2":
			r.Success, r.Output = false, ""
			r.Calls = r.Calls[:5] // stopped early
		case "S5":
			r.Calls[12].ServedModel = "gemini-3.5-flash-lite" // wrong model answered, runs.jsonl still says integrity_ok=true
		}
		runs = append(runs, r)
	}
	evaltestutil.WriteBatch(t, dir, evaltestutil.Manifest("B1", true, 1, []string{"07_Edge"}, ms), runs)
	b, err := evalcheck.LoadBatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestExtrapolate_KnownPartialUnknownAndExclusions(t *testing.T) {
	b := smokeFixture(t)
	ex, err := Extrapolate(b, 26, BasisRequested)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]ScenarioCost{}
	for _, s := range ex.Scenarios {
		by[s.Scenario] = s
	}
	if _, ok := by["S3"]; ok {
		t.Error("skipped scenario S3 must not be extrapolated")
	}
	// S0: 2 pro calls (1000 in, 150 out-billed) + 11 flash calls.
	wantRun := 2*(1000*2.0/1e6+150*12.0/1e6) + 11*(1000*1.5/1e6+150*9.0/1e6)
	s0 := by["S0"]
	if s0.Status != "KNOWN" || !near(s0.PerRun.KnownUSD, wantRun) || !near(s0.Projected.KnownUSD, wantRun*26) {
		t.Errorf("S0 = %+v, want KNOWN %.6f/run, %.6f projected", s0, wantRun, wantRun*26)
	}
	// S1: Flash tier (10 agents) on an unpriced model -> PARTIAL: only agents 1, 12 and the Critic priced.
	s1 := by["S1"]
	wantS1 := 2*(1000*2.0/1e6+150*12.0/1e6) + (1000*1.5/1e6 + 150*9.0/1e6)
	if s1.Status != "PARTIAL" || !near(s1.PerRun.KnownUSD, wantS1) || s1.PerRun.Unpriced[f38].Calls != 10 {
		t.Errorf("S1 = %+v, want PARTIAL with 10 unpriced %s calls and known %.6f", s1, f38, wantS1)
	}
	if s1.PerRun.Unpriced[f38].Input != 10000 || s1.PerRun.Unpriced[f38].Output != 1000 || s1.PerRun.Unpriced[f38].Thinking != 500 {
		t.Errorf("unpriced token volume not carried: %+v", s1.PerRun.Unpriced[f38])
	}
	// S4: lite on 3 agents -> PARTIAL, 3 unpriced calls.
	if s4 := by["S4"]; s4.Status != "PARTIAL" || s4.PerRun.Unpriced[lite].Calls != 3 {
		t.Errorf("S4 = %+v", s4)
	}
	// S2: failed run -> no representative run, UNKNOWN (never $0), but its spend is counted.
	s2 := by["S2"]
	if s2.Status != "NO_REPRESENTATIVE_RUN" || s2.SpentSmoke.Calls != 5 || len(s2.Excluded) != 1 {
		t.Errorf("S2 = %+v", s2)
	}
	// S5: runs.jsonl says integrity_ok=true but the calls file shows a wrong served model -> excluded.
	s5 := by["S5"]
	if s5.Status != "NO_REPRESENTATIVE_RUN" || !strings.Contains(strings.Join(s5.Excluded, " "), "re-derived from calls file") {
		t.Errorf("S5 fallback-served run must be excluded by re-deriving integrity from calls: %+v", s5)
	}
	if ex.Complete {
		t.Error("extrapolation with unpriced scenarios must not be Complete")
	}
	// Lower bound is exactly S0 + S1's priced part + S4's priced part, times 26.
	wantLB := (wantRun + wantS1 + by["S4"].PerRun.KnownUSD) * 26
	if !near(ex.KnownLowerBound, wantLB) {
		t.Errorf("lower bound %.6f, want %.6f", ex.KnownLowerBound, wantLB)
	}
}

func TestExtrapolate_PriceBasisServedUsesThe36Rate(t *testing.T) {
	b := smokeFixture(t)
	ex, _ := Extrapolate(b, 26, BasisServed)
	var s0 ScenarioCost
	for _, s := range ex.Scenarios {
		if s.Scenario == "S0" {
			s0 = s
		}
	}
	// 3.5-flash is SERVED as 3.6-flash (output $7.50/M in the table, vs $9.00 for 3.5-flash).
	want := 2*(1000*2.0/1e6+150*12.0/1e6) + 11*(1000*1.5/1e6+150*7.5/1e6)
	if !near(s0.PerRun.KnownUSD, want) {
		t.Errorf("served-basis S0 = %.6f, want %.6f", s0.PerRun.KnownUSD, want)
	}
}

func TestRender_NeverPrintsZeroForUnpricedAndListsExclusions(t *testing.T) {
	b := smokeFixture(t)
	ex, _ := Extrapolate(b, 26, BasisRequested)
	var buf bytes.Buffer
	Render(&buf, ex, "B1", true, nil)
	out := buf.String()
	for _, want := range []string{"UNKNOWN", "LOWER BOUND", "gemini-3.8-flash", "gemini-3.5-flash-lite", "Google Search grounding", "S3", "re-derived from calls file", "skipped scenarios"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "$0.00") {
		t.Errorf("an unpriced scenario must never render as $0:\n%s", out)
	}
	if !strings.Contains(out, "Projected total: UNKNOWN") {
		t.Errorf("total must be UNKNOWN when scenarios are unpriced:\n%s", out)
	}
}

func TestRender_WhatIfIsSeparateAndLabelled(t *testing.T) {
	b := smokeFixture(t)
	ex, _ := Extrapolate(b, 26, BasisRequested)
	m, h, err := ParseAssume("gemini-3.8-flash=0.75/3.75")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	Render(&buf, ex, "B1", true, map[string]Hypothetical{m: h})
	out := buf.String()
	if !strings.Contains(out, "HYPOTHETICAL") || !strings.Contains(out, "still lower bound") {
		t.Errorf("what-if must be labelled and remain a lower bound while other models are unpriced:\n%s", out)
	}
	if !strings.Contains(out, "Projected total: UNKNOWN") {
		t.Error("what-if must not change the official total")
	}
}

func TestParseAssume(t *testing.T) {
	for _, bad := range []string{"", "m", "m=1", "m=a/b", "=1/2", "m=0/1", "m=-1/2", "m=0.75abc/3.75", "m=1/2x"} {
		if _, _, err := ParseAssume(bad); err == nil {
			t.Errorf("ParseAssume(%q) should fail", bad)
		}
	}
}

func TestExtrapolate_RefusesBatchWithoutManifestOrWithProblems(t *testing.T) {
	if _, err := Extrapolate(&evalcheck.Batch{Root: "x"}, 26, BasisRequested); err == nil {
		t.Error("no manifest must be refused")
	}
	b := smokeFixture(t)
	b.Problems = []string{"mixed batches"}
	if _, err := Extrapolate(b, 26, BasisRequested); err == nil {
		t.Error("structural problems must be refused")
	}
}

func TestExtrapolate_FullMatrixCountsEveryPlannedScenarioEvenUnmeasured(t *testing.T) {
	ex, _ := Extrapolate(smokeFixture(t), 26, BasisRequested)
	// S0,S1,S2,S4,S5 are planned (S3 skipped), including S2/S5 which have no usable smoke run.
	if ex.FullRuns != 5*26 {
		t.Errorf("FullRuns = %d, want %d", ex.FullRuns, 5*26)
	}
	// plan §5: 9 scenarios x 13 x 2 = 234, minus skipped S3 (26) = 208 = 8 scenarios x 26.
	if 8*13*2 != 208 {
		t.Fatal("arithmetic")
	}
}
