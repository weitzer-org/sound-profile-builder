package evalreport

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evalcost"
	"github.com/weitzer-org/sound-builder/internal/evalcritic"
	"github.com/weitzer-org/sound-builder/internal/evaljudge"
	"github.com/weitzer-org/sound-builder/internal/evaltestutil"
)

const (
	pro   = "gemini-3.1-pro-preview"
	flash = "gemini-3.5-flash"
	f38   = "gemini-3.8-flash"
	lite  = "gemini-3.5-flash-lite"
	argon = "gemini-4-argon"
)

var (
	pins    = map[string]string{flash: "gemini-3.6-flash"}
	queries = []string{"q1", "q2"}
)

type scen struct {
	id    string
	route map[string]string
	skip  string
	// mutate lets a case damage specific runs (key "q1/2" = query/rep).
	mutate  func(r *evaltestutil.Run)
	latency float64
	out     func() string
}

func routes() map[string]map[string]string {
	r := map[string]map[string]string{
		"S0": evaltestutil.Routing(pro, flash, flash),
		"S1": evaltestutil.Routing(pro, f38, flash),
		"S2": evaltestutil.Routing(f38, flash, flash),
		"S3": evaltestutil.Routing(pro, flash, flash),
		"S4": evaltestutil.Routing(pro, flash, flash),
		"S5": evaltestutil.Routing(pro, flash, argon),
		"A1": evaltestutil.Routing(argon, argon, argon),
		"A2": evaltestutil.Routing(argon, flash, flash),
		"A3": evaltestutil.Routing(pro, argon, flash),
	}
	for _, k := range []string{"8_foh_optimizer", "10_control_mapper", "11_dsp_dispatcher"} {
		r["S4"][k] = lite
	}
	return r
}

// build writes a full synthetic batch and returns its directory. Defaults: every scenario is
// clean (CleanOutput, 100s) so each test damages only what it is about.
func build(t *testing.T, over map[string]func(*scen), drop map[string]bool) string {
	t.Helper()
	dir := t.TempDir()
	var ms []evalcheck.ManifestScenario
	var runs []evaltestutil.Run
	rt := routes()
	for _, id := range []string{"S0", "S1", "S2", "S3", "S4", "S5", "A1", "A2", "A3"} {
		sc := &scen{id: id, route: rt[id], latency: 100, out: evaltestutil.CleanOutput}
		if id == "S3" {
			sc.skip = "pro-latest == baseline pro"
		}
		if f := over[id]; f != nil {
			f(sc)
		}
		ms = append(ms, evalcheck.ManifestScenario{ID: id, Label: id + " label", AgentModels: sc.route, Pins: pins, Skip: sc.skip})
		if sc.skip != "" {
			continue
		}
		for rep := 1; rep <= 2; rep++ {
			for _, q := range queries {
				if drop[id+"/"+q+"/"+string(rune('0'+rep))] {
					continue
				}
				r := evaltestutil.Run{Scenario: id, Query: q, Rep: rep, Success: true, IntegrityOK: true, LatencySec: sc.latency,
					Calls: evaltestutil.Calls(sc.route, pins, 1000, 100, 50, 2000), Output: sc.out()}
				if sc.mutate != nil {
					sc.mutate(&r)
				}
				runs = append(runs, r)
			}
		}
	}
	evaltestutil.WriteBatch(t, dir, evaltestutil.Manifest("BATCH1", false, 2, queries, ms), runs)
	return dir
}

func mustBuild(t *testing.T, dir string, mod func(*Options)) *Report {
	t.Helper()
	opt := Options{Root: dir}
	if mod != nil {
		mod(&opt)
	}
	rep, err := Build(opt)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func gate(g Gates, name string) Check {
	for _, c := range g.Checks {
		if strings.HasPrefix(c.Name, name) {
			return c
		}
	}
	return Check{Name: "MISSING " + name}
}

func writeRecritic(t *testing.T, dir string, defects map[string]int, batch string) {
	t.Helper()
	b, _ := evalcheck.LoadBatch(dir)
	for _, r := range b.Records {
		if ok, _ := b.Scoreable(r); !ok {
			continue
		}
		n, has := defects[r.Scenario]
		if !has {
			continue
		}
		if err := evalcritic.AppendResult(evalcritic.ResultsPath(dir), evalcritic.Result{BatchID: batch, Scenario: r.Scenario, Query: r.Query, Rep: r.Rep, OK: true, Defects: n}); err != nil {
			t.Fatal(err)
		}
	}
}

func writeJudge(t *testing.T, dir, batch string, complete bool, per map[string][2]int) {
	t.Helper()
	s := evaljudge.Summary{BatchID: batch, Baseline: "S0", ReviewComplete: complete}
	for id, wl := range per {
		s.Candidates = append(s.Candidates, evaljudge.CandidateSummary{Candidate: id, Baseline: "S0", Counts: evaljudge.Counts{CandWins: wl[0], BaseWins: wl[1]}})
	}
	b, _ := jsonMarshal(s)
	if err := os.WriteFile(filepath.Join(dir, evaljudge.SummaryFile), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCleanBatch_AllGatesPassAndNumbersAreRight(t *testing.T) {
	rep := mustBuild(t, build(t, nil, nil), nil)
	for _, c := range rep.Pre {
		if c.Status != Pass {
			t.Fatalf("precondition %+v", c)
		}
	}
	s0 := rep.Scenario("S0")
	if s0.Planned != 4 || s0.Scoreable != 4 || s0.Outputs != 4 || s0.ParseErrors != 0 || s0.CallsChecked != 4*13 {
		t.Fatalf("S0 stats %+v", s0)
	}
	if p := Percentile(s0.Latency, 50); p != 100 {
		t.Errorf("p50 = %v", p)
	}
	if got := Mean(s0.InTok64()); got != 13000 { // 13 calls x 1000 input tokens
		t.Errorf("mean input tokens = %v", got)
	}
	// S0 per-run cost: 2 pro calls + 11 flash calls, 1000 in / 150 billed-out each.
	want := 2*(1000*2.0/1e6+150*12.0/1e6) + 11*(1000*1.5/1e6+150*9.0/1e6)
	if c := meanCost(s0.Cost); c.Status() != "KNOWN" || math.Abs(c.KnownUSD-want) > 1e-9 {
		t.Errorf("S0 cost %+v want %.6f", c, want)
	}
	// agent latency: one call per agent per run, 2s
	if p := Percentile(s0.AgentLat["Architect & Evaluator"], 50); p != 2 {
		t.Errorf("agent latency = %v", p)
	}
	g := rep.EvaluateGates(rep.Scenario("S1"))
	for _, c := range g.Checks {
		if c.Status != Pass {
			t.Errorf("clean S1 gate %+v", c)
		}
	}
}

func (s *ScenarioStats) InTok64() []float64 {
	var o []float64
	for _, x := range s.InTok {
		o = append(o, float64(x))
	}
	return o
}

func TestIntegrityFalseRunIsExcludedReportedAndNeverScored(t *testing.T) {
	dir := build(t, map[string]func(*scen){"S2": func(s *scen) {
		s.mutate = func(r *evaltestutil.Run) {
			if r.Query == "q1" && r.Rep == 1 {
				r.IntegrityOK = false                // runs.jsonl itself says so
				r.Output = evaltestutil.R1R2Output() // poison: if it were scored, R1/R2 would be > 0
			}
		}
	}}, nil)
	rep := mustBuild(t, dir, nil)
	s2 := rep.Scenario("S2")
	if s2.Scoreable != 3 || len(s2.Excluded) != 1 || !strings.Contains(s2.Excluded[0].Reason, "integrity_ok=false") {
		t.Fatalf("S2 = %+v", s2)
	}
	if s2.R1 != 0 || s2.R2 != 0 || s2.Outputs != 3 {
		t.Errorf("the excluded run's output was scored: R1=%d R2=%d outputs=%d", s2.R1, s2.R2, s2.Outputs)
	}
	g := rep.EvaluateGates(s2)
	if c := gate(g, "pipeline success"); c.Status != Fail || !strings.Contains(c.Detail, "3/4") {
		t.Errorf("an unscoreable run counts against the success rate: %+v", c)
	}
	var buf bytes.Buffer
	rep.Render(&buf)
	if !strings.Contains(buf.String(), "S2/rep1/q1") || !strings.Contains(buf.String(), "EXCLUDED RUNS") {
		t.Errorf("exclusion not reported:\n%s", buf.String())
	}
}

func TestFallbackServedCallIsExcludedEvenWhenRunsJSONLSaysClean(t *testing.T) {
	dir := build(t, map[string]func(*scen){"S4": func(s *scen) {
		s.mutate = func(r *evaltestutil.Run) {
			if r.Query == "q2" && r.Rep == 2 {
				// The Architect fell back to 2.5-pro; runs.jsonl (wrongly) still says integrity_ok=true.
				r.Calls[11].AttemptModel, r.Calls[11].ServedModel = "gemini-2.5-pro", "gemini-2.5-pro"
				r.Output = evaltestutil.R1R2Output()
			}
		}
	}}, nil)
	rep := mustBuild(t, dir, nil)
	s4 := rep.Scenario("S4")
	if s4.Scoreable != 3 || s4.R1 != 0 || s4.R2 != 0 {
		t.Fatalf("fallback-served run was scored: %+v", s4)
	}
	if !strings.Contains(s4.Excluded[0].Reason, "re-derived from calls file") {
		t.Errorf("reason = %q", s4.Excluded[0].Reason)
	}
	g := rep.EvaluateGates(s4)
	if c := gate(g, "served-model integrity"); c.Status != Fail || !strings.Contains(c.Detail, "1 of 52") {
		t.Errorf("integrity gate: %+v", c)
	}
	if g.Overall() != Fail {
		t.Errorf("overall = %s", g.Overall())
	}
}

func TestServedModelDriftOnSuccessfulCallFailsIntegrity(t *testing.T) {
	dir := build(t, map[string]func(*scen){"S1": func(s *scen) {
		s.mutate = func(r *evaltestutil.Run) { r.Calls[3].ServedModel = "gemini-3.5-flash-lite" } // same attempt model, other model answered
	}}, nil)
	rep := mustBuild(t, dir, nil)
	if c := gate(rep.EvaluateGates(rep.Scenario("S1")), "served-model integrity"); c.Status != Fail {
		t.Errorf("%+v", c)
	}
	if rep.Scenario("S1").Scoreable != 0 {
		t.Errorf("every run has the drifted call, none may be scored")
	}
}

func TestTruncationGateComparesPerAgentAgainstS0(t *testing.T) {
	dir := build(t, map[string]func(*scen){"S1": func(s *scen) {
		s.mutate = func(r *evaltestutil.Run) {
			if r.Query == "q1" && r.Rep == 1 {
				r.Success, r.IntegrityOK, r.Output = false, false, ""
				r.Calls[5].Success, r.Calls[5].FinishReason, r.Calls[5].Error = false, "MAX_TOKENS", "truncated"
			}
		}
	}}, nil)
	rep := mustBuild(t, dir, nil)
	g := rep.EvaluateGates(rep.Scenario("S1"))
	if c := gate(g, "MAX_TOKENS"); c.Status != Fail || !strings.Contains(c.Detail, "Acoustician 1/4 runs vs S0 0/4") {
		t.Errorf("%+v", c)
	}
	if c := gate(g, "pipeline success"); c.Status != Fail {
		t.Errorf("a truncation-failed run also lowers the success rate: %+v", c)
	}
	// Same truncation in S0 as well -> candidate no worse than S0.
	dir = build(t, map[string]func(*scen){
		"S0": func(s *scen) { s.mutate = truncAcoustician },
		"S1": func(s *scen) { s.mutate = truncAcoustician },
	}, nil)
	rep = mustBuild(t, dir, nil)
	if c := gate(rep.EvaluateGates(rep.Scenario("S1")), "MAX_TOKENS"); c.Status != Pass {
		t.Errorf("equal truncation is <= S0: %+v", c)
	}
}

func truncAcoustician(r *evaltestutil.Run) {
	if r.Query == "q1" && r.Rep == 1 {
		r.Success, r.IntegrityOK, r.Output = false, false, ""
		r.Calls[5].Success, r.Calls[5].FinishReason, r.Calls[5].Error = false, "MAX_TOKENS", "truncated"
	}
}

func TestR1R2AndInventedNameGates(t *testing.T) {
	invented := func() string {
		return evaltestutil.Envelope(evaltestutil.Block{Type: "Amplifier", Model: "Zorblax Quantum 9000", Rationale: "invented", Params: [][2]string{{"Gain", "5"}}})
	}
	dir := build(t, map[string]func(*scen){
		"S1": func(s *scen) { s.out = evaltestutil.R1R2Output },
		"S2": func(s *scen) { s.out = invented },
	}, nil)
	rep := mustBuild(t, dir, nil)
	g1 := rep.EvaluateGates(rep.Scenario("S1"))
	if c := gate(g1, "R1"); c.Status != Fail || !strings.Contains(c.Detail, "4 across 4") {
		t.Errorf("R1: %+v", c)
	}
	if c := gate(g1, "schema validity"); c.Status != Fail {
		t.Errorf("R2 is part of schema validity: %+v", c)
	}
	if rep.Scenario("S1").R2 == 0 {
		t.Error("R2 count missing")
	}
	g2 := rep.EvaluateGates(rep.Scenario("S2"))
	if c := gate(g2, "invented"); c.Status != Fail {
		t.Errorf("invented names must exceed S0's 0: %+v", c)
	}
	if rep.Scenario("S2").Unverified != 4 {
		t.Errorf("unverified = %d", rep.Scenario("S2").Unverified)
	}
}

func TestUnparseableOutputFailsSchemaValidity(t *testing.T) {
	dir := build(t, map[string]func(*scen){"S1": func(s *scen) { s.out = func() string { return "{not json" } }}, nil)
	rep := mustBuild(t, dir, nil)
	if c := gate(rep.EvaluateGates(rep.Scenario("S1")), "schema validity"); c.Status != Fail || !strings.Contains(c.Detail, "0/4 outputs parse") {
		t.Errorf("%+v", c)
	}
}

func TestMixedBatchesAreRefused(t *testing.T) {
	dir := build(t, nil, nil)
	p := filepath.Join(dir, "runs.jsonl")
	raw, _ := os.ReadFile(p)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	lines[0] = strings.Replace(lines[0], `"batch_id":"BATCH1"`, `"batch_id":"OLDER"`, 1)
	_ = os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	if _, err := Build(Options{Root: dir}); err == nil || !strings.Contains(err.Error(), "different batches") {
		t.Fatalf("err = %v", err)
	}
}

func TestOtherRootFromAnotherBatchIsRefused(t *testing.T) {
	a := build(t, nil, nil)
	b := build(t, nil, nil)
	// same batch id string by construction in this fixture, so make b a different batch
	mp := filepath.Join(b, evalcheck.ManifestFile)
	raw, _ := os.ReadFile(mp)
	_ = os.WriteFile(mp, []byte(strings.ReplaceAll(string(raw), "BATCH1", "BATCH2")), 0o644)
	rp := filepath.Join(b, "runs.jsonl")
	rr, _ := os.ReadFile(rp)
	_ = os.WriteFile(rp, []byte(strings.ReplaceAll(string(rr), "BATCH1", "BATCH2")), 0o644)
	_, err := Build(Options{Root: a, OtherRoot: b})
	if err == nil || !strings.Contains(err.Error(), "ACROSS BATCHES") {
		t.Fatalf("err = %v", err)
	}
	if _, err := Build(Options{Root: a, OtherRoot: a}); err != nil {
		t.Errorf("the same batch is fine: %v", err)
	}
}

func TestForeignRecriticAndJudgeFilesAreRefused(t *testing.T) {
	dir := build(t, nil, nil)
	writeRecritic(t, dir, map[string]int{"S0": 1}, "SOME-OTHER-BATCH")
	if _, err := Build(Options{Root: dir}); err == nil || !strings.Contains(err.Error(), "REFUSING") {
		t.Errorf("foreign re-critic: %v", err)
	}
	dir = build(t, nil, nil)
	writeJudge(t, dir, "SOME-OTHER-BATCH", true, map[string][2]int{"S1": {3, 1}})
	if _, err := Build(Options{Root: dir}); err == nil || !strings.Contains(err.Error(), "REFUSING") {
		t.Errorf("foreign judge summary: %v", err)
	}
}

func TestIncompleteMatrixBlocksDecisions(t *testing.T) {
	dir := build(t, nil, map[string]bool{"S1/q2/2": true})
	rep := mustBuild(t, dir, nil)
	found := false
	for _, c := range rep.Pre {
		if c.Name == "complete matrix" && c.Status == Fail && strings.Contains(c.Detail, "S1 recorded 3 of 4") {
			found = true
		}
	}
	if !found {
		t.Fatalf("pre = %+v", rep.Pre)
	}
	var buf bytes.Buffer
	rep.Render(&buf)
	if !strings.Contains(buf.String(), "NOT DECIDABLE") {
		t.Error("an incomplete batch must be marked NOT DECIDABLE")
	}
}

func TestSection7Rules(t *testing.T) {
	over := map[string]func(*scen){
		"S1": func(s *scen) { s.latency = 70 }, // faster Flash candidate
		"S2": func(s *scen) { s.latency = 100 },
		"S4": func(s *scen) { s.latency = 100 },
	}
	dir := build(t, over, nil)
	writeRecritic(t, dir, map[string]int{"S0": 2, "S1": 2, "S2": 3, "S4": 2, "S5": 1, "A1": 1, "A2": 1, "A3": 1}, "BATCH1")
	writeJudge(t, dir, "BATCH1", true, map[string][2]int{"S1": {3, 3}, "S2": {5, 1}, "S4": {2, 2}, "S5": {3, 1}, "A1": {5, 1}, "A2": {1, 3}, "A3": {2, 1}})
	rep := mustBuild(t, dir, func(o *Options) { o.BaselineFlashDeprecated = Yes })
	gates := map[string]Gates{}
	for _, s := range rep.Scenarios {
		if s.Skipped == "" {
			gates[s.ID] = rep.EvaluateGates(s)
		}
	}
	verdict := func(id, tier string) RuleResult {
		for _, rr := range rep.EvaluateRules(rep.Scenario(id), gates[id]) {
			if rr.Tier == tier {
				return rr
			}
		}
		t.Fatalf("no %s rule for %s", tier, id)
		return RuleResult{}
	}
	// Flash (S1): gates pass, defects tie (2 vs 2), judge 3-3 tie, faster, S0 deprecated -> ADOPT. Cost is PARTIAL (3.8 unpriced) so "cheaper" is UNMEASURED, not a fail.
	s1 := verdict("S1", "flash")
	if s1.Verdict != Adopt {
		t.Errorf("S1 flash = %s: %+v", s1.Verdict, s1.Lines)
	}
	cheap := lineStatus(s1, "cheaper")
	if cheap != Unmeasured {
		t.Errorf("cheaper must be UNMEASURED when the candidate has unpriced calls, got %s", cheap)
	}
	// Pro (S2): defects 3 > 2 -> DO NOT ADOPT even though the judge likes it.
	if v := verdict("S2", "pro"); v.Verdict != NotAdopt || lineStatus(v, "critic defect") != Fail {
		t.Errorf("S2 pro = %s %+v", v.Verdict, v.Lines)
	}
	// Mechanical (S4): lite is unpriced but cost is irrelevant for this rule; ties on quality -> ADOPT.
	if v := verdict("S4", "mechanical"); v.Verdict != Adopt {
		t.Errorf("S4 mechanical = %s %+v", v.Verdict, v.Lines)
	}
	// Critic (S5): not mechanizable, and Argon is provisional.
	if v := verdict("S5", "critic"); v.Verdict != Manual || !strings.Contains(v.Note, "PREVIEW SIGNAL") {
		t.Errorf("S5 critic = %s %q", v.Verdict, v.Note)
	}
	// A1 (Argon everywhere): would otherwise adopt, but is capped to UNDETERMINED.
	if v := verdict("A1", "pro"); v.Verdict != Undetermined || !strings.Contains(v.Note, "CAPPED") {
		t.Errorf("A1 pro = %s %q", v.Verdict, v.Note)
	}
	// Attribution: A1 wins (5-1), A2 does not (1-3), A3 wins (2-1) => Flash tier drives it.
	got := strings.Join(rep.Attribution(gates), "\n")
	if !strings.Contains(got, "A3 (Flash tier) wins") {
		t.Errorf("attribution:\n%s", got)
	}
}

func lineStatus(rr RuleResult, prefix string) Status {
	for _, l := range rr.Lines {
		if strings.HasPrefix(l.Name, prefix) {
			return l.Status
		}
	}
	return "MISSING"
}

func TestRulesAreUndeterminedWithoutReCriticOrJudge(t *testing.T) {
	rep := mustBuild(t, build(t, nil, nil), nil)
	for _, rr := range rep.EvaluateRules(rep.Scenario("S2"), rep.EvaluateGates(rep.Scenario("S2"))) {
		if rr.Verdict != Undetermined {
			t.Errorf("without re-critic/judge a Pro candidate cannot be adopted: %s", rr.Verdict)
		}
	}
}

func TestProvisionalJudgeReviewDowngradesAdopt(t *testing.T) {
	dir := build(t, nil, nil)
	writeRecritic(t, dir, map[string]int{"S0": 2, "S2": 1}, "BATCH1")
	writeJudge(t, dir, "BATCH1", false, map[string][2]int{"S2": {4, 1}})
	rep := mustBuild(t, dir, nil)
	rr := rep.EvaluateRules(rep.Scenario("S2"), rep.EvaluateGates(rep.Scenario("S2")))
	if len(rr) != 1 || rr[0].Verdict != Provisional {
		t.Errorf("%+v", rr)
	}
}

func TestDecideTable(t *testing.T) {
	L := func(ss ...Status) []Check {
		var out []Check
		for _, s := range ss {
			out = append(out, Check{Status: s})
		}
		return out
	}
	cases := []struct {
		tier  string
		lines []Check
		want  Verdict
	}{
		{"pro", L(Pass, Pass, Pass), Adopt},
		{"pro", L(Pass, Pass, Unmeasured), Undetermined},
		{"pro", L(Fail, Pass, Unmeasured), NotAdopt}, // a failure beats an unmeasured input
		{"flash", L(Pass, Pass, Pass, Fail, Fail, Fail), NotAdopt},
		{"flash", L(Pass, Pass, Pass, Fail, Pass, Fail), Adopt},
		{"flash", L(Pass, Pass, Pass, Fail, Unmeasured, Unmeasured), Undetermined},
		{"flash", L(Pass, Pass, Pass, Unmeasured, Fail, Pass), Adopt},
		{"mechanical", L(Pass, Pass, Pass), Adopt},
	}
	for i, c := range cases {
		if got := decide(c.lines, c.tier); got != c.want {
			t.Errorf("case %d (%s): %s, want %s", i, c.tier, got, c.want)
		}
	}
}

func TestPercentileDefectSpreadAndTri(t *testing.T) {
	xs := []float64{5, 1, 3, 2, 4, 10, 9, 8, 7, 6}
	if Percentile(xs, 50) != 5 || Percentile(xs, 95) != 10 || Percentile(xs, 10) != 1 {
		t.Errorf("nearest-rank percentiles: %v %v %v", Percentile(xs, 50), Percentile(xs, 95), Percentile(xs, 10))
	}
	if !math.IsNaN(Percentile(nil, 50)) {
		t.Error("empty must be NaN")
	}
	s := &ScenarioStats{Defects: []DefectPoint{{"a", 1, 2}, {"a", 2, 4}, {"b", 1, 3}, {"b", 2, 3}, {"c", 1, 6}}}
	m, sd, rg := s.DefectSpread()
	if math.Abs(m-3.6) > 1e-9 || math.Abs(rg-1.0) > 1e-9 || sd <= 0 { // ranges: a=2, b=0 -> mean 1; c has 1 rep and is skipped
		t.Errorf("mean=%v sd=%v range=%v", m, sd, rg)
	}
	if v, err := ParseTri("YES"); err != nil || v != Yes {
		t.Error("ParseTri")
	}
	if _, err := ParseTri("maybe"); err == nil {
		t.Error("ParseTri must reject junk")
	}
}

func TestGatesNeverPassWhenUnmeasured(t *testing.T) {
	dir := build(t, nil, nil)
	// remove one calls file: S1's served-model integrity can no longer be verified
	_ = os.Remove(evalcheck.CallsPath(dir, evalcheck.RunKey{Scenario: "S1", Query: "q1", Rep: 1}))
	rep := mustBuild(t, dir, nil)
	g := rep.EvaluateGates(rep.Scenario("S1"))
	if c := gate(g, "served-model integrity"); c.Status != Unmeasured {
		t.Errorf("%+v", c)
	}
	if g.Overall() == Pass {
		t.Error("a scenario with an unmeasured gate must not PASS")
	}
}

func TestRenderCostNeverZeroForUnpriced(t *testing.T) {
	rep := mustBuild(t, build(t, nil, nil), nil)
	var buf bytes.Buffer
	rep.Render(&buf)
	out := buf.String()
	if strings.Contains(out, "$0.0000") {
		t.Errorf("an unpriced scenario rendered as $0:\n%s", out)
	}
	for _, want := range []string{"UNPRICED(gemini-3.8-flash", "UNPRICED(gemini-4-argon", "SKIPPED", "p50/p95", "HARD GATES", "DECISION RULES"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q", want)
		}
	}
	_ = agents.PriceFor
	_ = evalcost.BasisServed
}
