// Package evalreport is the Phase 2 report generator (plan §6.1 hard gates, §6.2-6.3 quality,
// cost and latency, §7 decision rules). It reads ONE results directory -- one eval_full_pipeline
// batch -- and compares every scenario with S0 from that same batch. It makes no Gemini calls.
//
// Principles enforced here:
//   - never compare across batches: mixed or mismatching batch ids are a refusal, not a warning;
//   - a run that failed, was not served by its requested model, or lacks a calls file is EXCLUDED
//     and listed, never scored;
//   - a number that was not measured is UNMEASURED, and a gate or rule that needs it cannot PASS.
package evalreport

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evalcost"
	"github.com/weitzer-org/sound-builder/internal/evalcritic"
	"github.com/weitzer-org/sound-builder/internal/evaljudge"
)

// Tri is a yes/no/unknown input the data cannot supply.
type Tri int

const (
	Unknown Tri = iota
	Yes
	No
)

// ParseTri parses yes|no|unknown.
func ParseTri(s string) (Tri, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "true":
		return Yes, nil
	case "no", "false":
		return No, nil
	case "unknown", "":
		return Unknown, nil
	}
	return Unknown, fmt.Errorf("%q: want yes|no|unknown", s)
}

// Options configure Build.
type Options struct {
	Root       string
	Legacy     bool   // pre-manifest July layout: output-derived metrics only
	BaselineID string // default "S0"
	// BaselineFlashDeprecated feeds the §7 Flash rule's "later deprecation date" clause. Plan §10
	// says deprecation dates are unverified, so this defaults to Unknown and the clause is then
	// UNMEASURED rather than assumed.
	BaselineFlashDeprecated Tri
	// Provisional is the set of model ids whose use in production §7 forbids until Google has
	// published pricing and a non-provisional id (default: gemini-4-argon).
	Provisional map[string]bool
	Basis       evalcost.Basis
	// OtherRoot, if set, is a second results directory the caller wants compared. It is accepted
	// only when it is the SAME batch; otherwise Build refuses (plan §2 principle 5).
	OtherRoot string
	// JudgeSummaryPath overrides root/judge_summary.json.
	JudgeSummaryPath string
}

// ExcludedRun is a run that was not scored.
type ExcludedRun struct {
	Key    evalcheck.RunKey
	Reason string
}

// DefectPoint is one run's re-critic defect count.
type DefectPoint struct {
	Query string
	Rep   int
	N     int
}

// ScenarioStats are the raw measurements for one scenario (no thresholds applied).
type ScenarioStats struct {
	ID, Label string
	Skipped   string
	Changed   []string // agent keys routed differently from the baseline
	Tiers     []string // pro | flash | mechanical | critic

	Planned, Recorded, Succeeded, Scoreable int
	Excluded                                []ExcludedRun

	// Served-model integrity, from the calls files.
	CallsChecked, CallsViolating int
	CallsMissingRuns             int
	Violations                   []string

	// Outputs of scoreable runs.
	Outputs, ParseErrors            int
	R1, R2, ModeledStyleOnCapture   int
	Snapped, Unverified, GearBlocks int
	NonNumericRef                   int
	OwnCriticFlags                  int

	// MAX_TOKENS finish reasons on the routed model's own attempts, per agent role, over all
	// recorded runs that have a calls file (failed runs included: truncation is why they fail).
	Trunc     map[string]int
	TruncRuns int

	// Re-critic.
	DefectSource  string // "re-critic" | "none"
	Defects       []DefectPoint
	DefectMissing int // scoreable runs with no OK re-critic result

	// Latency / tokens / cost over scoreable runs.
	Latency  []float64
	AgentLat map[string][]float64 // seconds, summed over a run's attempts of that agent
	InTok    []int64
	OutTok   []int64
	ThinkTok []int64
	Cost     []evalcost.RunCost
}

// Report is the whole analysis.
type Report struct {
	Root       string
	Legacy     bool
	BatchID    string
	Smoke      bool
	BaselineID string
	Scenarios  []*ScenarioStats
	Judge      *evaljudge.Summary
	Options    Options

	// Preconditions that must hold before any comparison means anything.
	Pre []Check

	models map[string][]string // scenario -> distinct routed model ids (from the manifest)
}

// Check is a named PASS/FAIL/UNMEASURED line with the numbers behind it.
type Check struct {
	Name   string
	Status Status
	Detail string
}

// Status of a gate or precondition.
type Status string

const (
	Pass       Status = "PASS"
	Fail       Status = "FAIL"
	Unmeasured Status = "UNMEASURED"
	NA         Status = "N/A"
)

// Percentile is the nearest-rank percentile of xs (p in (0,100]); NaN for no data.
func Percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	rank := int(math.Ceil(p / 100 * float64(len(s))))
	if rank < 1 {
		rank = 1
	}
	return s[rank-1]
}

// Mean of xs; NaN for none.
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	t := 0.0
	for _, x := range xs {
		t += x
	}
	return t / float64(len(xs))
}

// StdDev is the sample standard deviation (0 for fewer than two points).
func StdDev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := Mean(xs)
	t := 0.0
	for _, x := range xs {
		t += (x - m) * (x - m)
	}
	return math.Sqrt(t / float64(len(xs)-1))
}

// DefectSpread returns the mean, the sample sd across all runs, and the mean per-query range
// (max-min across reps) over queries that have at least two reps.
func (s *ScenarioStats) DefectSpread() (mean, sd, repRange float64) {
	var all []float64
	byQ := map[string][]float64{}
	for _, d := range s.Defects {
		all = append(all, float64(d.N))
		byQ[d.Query] = append(byQ[d.Query], float64(d.N))
	}
	mean, sd = Mean(all), StdDev(all)
	var ranges []float64
	for _, v := range byQ {
		if len(v) < 2 {
			continue
		}
		lo, hi := v[0], v[0]
		for _, x := range v {
			lo, hi = math.Min(lo, x), math.Max(hi, x)
		}
		ranges = append(ranges, hi-lo)
	}
	return mean, sd, Mean(ranges)
}

// Build loads root and computes every ScenarioStats. It returns an error (a REFUSAL) when the
// directory mixes batches, has structural problems, lacks the baseline, or when OtherRoot is a
// different batch.
func Build(opt Options) (*Report, error) {
	if opt.BaselineID == "" {
		opt.BaselineID = "S0"
	}
	if opt.Basis == "" {
		opt.Basis = evalcost.BasisRequested
	}
	if opt.Provisional == nil {
		opt.Provisional = map[string]bool{"gemini-4-argon": true}
	}
	load := evalcheck.LoadBatch
	if opt.Legacy {
		load = evalcheck.LoadLegacy
	}
	b, err := load(opt.Root)
	if err != nil {
		return nil, err
	}
	rep := &Report{Root: opt.Root, Legacy: b.Legacy, BaselineID: opt.BaselineID, Options: opt}
	if b.Manifest != nil {
		rep.BatchID, rep.Smoke = b.Manifest.BatchID, b.Manifest.Smoke
	}
	if len(b.Problems) > 0 {
		return nil, fmt.Errorf("REFUSING TO REPORT: %s has structural problems: %s", opt.Root, strings.Join(b.Problems, "; "))
	}
	if opt.OtherRoot != "" {
		ob, err := evalcheck.LoadBatch(opt.OtherRoot)
		if err != nil {
			return nil, fmt.Errorf("-baseline-root: %w", err)
		}
		if b.Manifest == nil || ob.Manifest == nil || b.Manifest.BatchID != ob.Manifest.BatchID {
			return nil, fmt.Errorf("REFUSING TO COMPARE ACROSS BATCHES: %s and %s are different eval_full_pipeline invocations (plan §2 principle 5: a scenario is only comparable with an S0 run from the same batch). Run the baseline and the candidates in one invocation", opt.Root, opt.OtherRoot)
		}
	}

	valid := agents.BuildEffectiveValidBlocks(true, true)
	re, reErr := loadRecritic(b, opt.Root)
	if reErr != nil {
		return nil, reErr
	}
	if sp := judgePath(opt); sp != "" {
		js, err := evaljudge.ReadSummary(sp)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil {
			if js.BatchID != "" && b.Manifest != nil && js.BatchID != b.Manifest.BatchID {
				return nil, fmt.Errorf("REFUSING: %s was produced for batch %q, not %q", sp, js.BatchID, b.Manifest.BatchID)
			}
			rep.Judge = js
		}
	}

	rep.models = map[string][]string{}
	for _, ms := range scenarioList(b) {
		seenM := map[string]bool{}
		for _, m := range ms.AgentModels {
			if !seenM[m] {
				seenM[m] = true
				rep.models[ms.ID] = append(rep.models[ms.ID], m)
			}
		}
		sort.Strings(rep.models[ms.ID])
		st := &ScenarioStats{ID: ms.ID, Label: ms.Label, Skipped: ms.Skip, Trunc: map[string]int{}, AgentLat: map[string][]float64{}, DefectSource: "none"}
		if b.Manifest != nil {
			st.Planned = b.Manifest.Planned(ms.ID)
		}
		var baselineModels map[string]string
		if b.Manifest != nil {
			if s0, ok := b.Manifest.Scenario(opt.BaselineID); ok {
				baselineModels = s0.AgentModels
			}
		}
		st.Changed, st.Tiers = tiersOf(ms.AgentModels, baselineModels)
		for _, r := range b.Records {
			if r.Scenario != ms.ID {
				continue
			}
			st.Recorded++
			if r.Success {
				st.Succeeded++
			}
			calls, haveCalls := b.Calls[r.Key()]
			if haveCalls && b.Manifest != nil {
				accumulateCalls(st, r, calls, ms)
			} else if !b.Legacy {
				st.CallsMissingRuns++
			}
			ok, why := b.Scoreable(r)
			if !ok {
				st.Excluded = append(st.Excluded, ExcludedRun{r.Key(), why})
				continue
			}
			st.Scoreable++
			addScoreable(st, b, r, calls, haveCalls, valid, re, opt.Basis)
		}
		sort.Slice(st.Excluded, func(i, j int) bool { return st.Excluded[i].Key.String() < st.Excluded[j].Key.String() })
		if b.Legacy {
			st.Planned = st.Recorded
		}
		rep.Scenarios = append(rep.Scenarios, st)
	}
	rep.Pre = preconditions(rep, b)
	return rep, nil
}

func judgePath(opt Options) string {
	if opt.JudgeSummaryPath != "" {
		return opt.JudgeSummaryPath
	}
	return opt.Root + "/" + evaljudge.SummaryFile
}

func scenarioList(b *evalcheck.Batch) []evalcheck.ManifestScenario {
	if b.Manifest != nil {
		return b.Manifest.Scenarios
	}
	// legacy: scenarios from the directory names, "baseline" first.
	seen := map[string]bool{}
	var ids []string
	for _, r := range b.Records {
		if !seen[r.Scenario] {
			seen[r.Scenario] = true
			ids = append(ids, r.Scenario)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if (ids[i] == "baseline") != (ids[j] == "baseline") {
			return ids[i] == "baseline"
		}
		return ids[i] < ids[j]
	})
	out := make([]evalcheck.ManifestScenario, 0, len(ids))
	for _, id := range ids {
		out = append(out, evalcheck.ManifestScenario{ID: id, Label: id})
	}
	return out
}

func loadRecritic(b *evalcheck.Batch, root string) (map[evalcheck.RunKey]evalcritic.Result, error) {
	rs, err := evalcritic.ReadResults(evalcritic.ResultsPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := map[evalcheck.RunKey]evalcritic.Result{}
	for _, r := range rs {
		if b.Manifest != nil && r.BatchID != "" && r.BatchID != b.Manifest.BatchID {
			return nil, fmt.Errorf("REFUSING: %s has a re-critic result for batch %q, not %q", evalcritic.ResultsFile, r.BatchID, b.Manifest.BatchID)
		}
		out[r.Key()] = r
	}
	return out, nil
}

var (
	proKeys  = map[string]bool{"1_tone_historian": true, "12_architect": true}
	mechKeys = map[string]bool{"8_foh_optimizer": true, "10_control_mapper": true, "11_dsp_dispatcher": true}
)

// tiersOf says which agent keys a scenario routes differently from the baseline and which §7
// tiers that touches. The mechanical tier is reported only when the Flash-tier change is limited
// to agents 8, 10 and 11; any other Flash-tier change is the Flash tier.
func tiersOf(models, baseline map[string]string) (changed, tiers []string) {
	if models == nil || baseline == nil {
		return nil, nil
	}
	var pro, flash, mech, other bool
	for k, v := range models {
		if baseline[k] == v {
			continue
		}
		changed = append(changed, k)
		switch {
		case proKeys[k]:
			pro = true
		case k == "13_critic":
			other = true
		case mechKeys[k]:
			mech = true
		default:
			flash = true
		}
	}
	sort.Strings(changed)
	if pro {
		tiers = append(tiers, "pro")
	}
	if flash {
		tiers = append(tiers, "flash")
	} else if mech {
		tiers = append(tiers, "mechanical")
	}
	if other {
		tiers = append(tiers, "critic")
	}
	return changed, tiers
}

// accumulateCalls does the served-model and truncation accounting from a run's calls file.
func accumulateCalls(st *ScenarioStats, r evalcheck.RunRecord, calls []agents.CallRecord, ms evalcheck.ManifestScenario) {
	st.TruncRuns++
	for _, c := range calls {
		key := agents.AgentRoleKeys[c.Agent]
		want, routed := ms.AgentModels[key]
		expectServed := want
		if p, ok := ms.Pins[want]; ok {
			expectServed = p
		}
		st.CallsChecked++
		switch {
		case !routed:
			st.CallsViolating++
			st.Violations = append(st.Violations, fmt.Sprintf("%s %s: agent has no routed model", r.Key(), c.Agent))
		case c.AttemptModel != want:
			st.CallsViolating++
			st.Violations = append(st.Violations, fmt.Sprintf("%s %s: attempted %s, scenario routes %s (fallback)", r.Key(), c.Agent, c.AttemptModel, want))
		case c.Success && !agents.ServedModelMatches(expectServed, c.ServedModel):
			st.CallsViolating++
			st.Violations = append(st.Violations, fmt.Sprintf("%s %s: asked %s, served %q (expected %s)", r.Key(), c.Agent, want, c.ServedModel, expectServed))
		}
		// Truncation counts only the routed model's own attempts; a fallback's truncation is not the candidate's.
		if routed && c.AttemptModel == want && c.FinishReason == "MAX_TOKENS" {
			st.Trunc[c.Agent]++
		}
	}
}

func addScoreable(st *ScenarioStats, b *evalcheck.Batch, r evalcheck.RunRecord, calls []agents.CallRecord, haveCalls bool, valid map[string]bool, re map[evalcheck.RunKey]evalcritic.Result, basis evalcost.Basis) {
	// Output checks.
	if raw, err := os.ReadFile(b.OutputFile(r.Key())); err != nil {
		st.Outputs++
		st.ParseErrors++
	} else {
		rp := evalcheck.Check(string(raw), valid)
		st.Outputs++
		if rp.ParseError != "" {
			st.ParseErrors++
		} else {
			st.R1 += rp.CaptureClaimOnModeled
			st.R2 += rp.CaptureDBOnModeled
			st.ModeledStyleOnCapture += rp.ModeledStyleOnCapture
			st.Snapped += rp.SnappedNames
			st.Unverified += rp.UnverifiedNames
			st.GearBlocks += rp.GearBlocks
			st.NonNumericRef += rp.NonNumericValues
			st.OwnCriticFlags += rp.CriticFlags
		}
	}
	// Re-critic.
	if re != nil {
		st.DefectSource = "re-critic"
		if res, ok := re[r.Key()]; ok && res.OK {
			st.Defects = append(st.Defects, DefectPoint{r.Query, r.Rep, res.Defects})
		} else {
			st.DefectMissing++
		}
	}
	// Latency, tokens, cost.
	if !b.Legacy {
		st.Latency = append(st.Latency, r.LatencySec)
	}
	if haveCalls {
		perAgent := map[string]float64{}
		var in, out, think int64
		for _, c := range calls {
			perAgent[c.Agent] += float64(c.LatencyMs) / 1000
			in += int64(c.InputTokens)
			out += int64(c.OutputTokens)
			think += int64(c.ThinkingTokens)
		}
		for a, v := range perAgent {
			st.AgentLat[a] = append(st.AgentLat[a], v)
		}
		st.InTok, st.OutTok, st.ThinkTok = append(st.InTok, in), append(st.OutTok, out), append(st.ThinkTok, think)
		st.Cost = append(st.Cost, evalcost.CostOfCalls(calls, basis))
	}
}

func preconditions(rep *Report, b *evalcheck.Batch) []Check {
	var pre []Check
	switch {
	case rep.Legacy:
		pre = append(pre, Check{"single batch", Unmeasured, "LEGACY directory: no manifest.json / batch ids, so \"same batch\" cannot be verified; July's own report says the three scenarios were run together but this tool cannot prove it"})
	default:
		pre = append(pre, Check{"single batch", Pass, fmt.Sprintf("all %d run records carry batch_id %s", len(b.Records), rep.BatchID)})
	}
	base := rep.Scenario(rep.BaselineID)
	if base == nil {
		pre = append(pre, Check{"baseline " + rep.BaselineID + " present", Fail, "not in this batch: every comparison is relative to it"})
		return pre
	}
	pre = append(pre, Check{"baseline " + rep.BaselineID + " present", Pass, fmt.Sprintf("%d scoreable of %d recorded runs", base.Scoreable, base.Recorded)})
	if base.Scoreable == 0 {
		pre = append(pre, Check{"baseline has scoreable runs", Fail, "S0 has no scoreable run, so nothing can be compared with it"})
	}
	// Completeness: every planned (non-skipped) scenario has all planned runs recorded.
	var gaps []string
	for _, s := range rep.Scenarios {
		if s.Skipped != "" {
			continue
		}
		if s.Recorded != s.Planned {
			gaps = append(gaps, fmt.Sprintf("%s recorded %d of %d planned", s.ID, s.Recorded, s.Planned))
		}
	}
	if rep.Legacy {
		pre = append(pre, Check{"complete matrix", Unmeasured, "legacy directory has no plan; missing queries cannot be told apart from failures"})
	} else if len(gaps) > 0 {
		pre = append(pre, Check{"complete matrix", Fail, strings.Join(gaps, "; ") + " (the run was interrupted or runs.jsonl is incomplete)"})
	} else {
		pre = append(pre, Check{"complete matrix", Pass, "every planned run has a record"})
	}
	if base.CallsViolating > 0 || base.CallsMissingRuns > 0 {
		pre = append(pre, Check{"baseline is clean", Fail, fmt.Sprintf("S0 has %d served-model violations and %d runs without a calls file: the control itself is compromised", base.CallsViolating, base.CallsMissingRuns)})
	}
	return pre
}

// Scenario returns the stats for id.
func (r *Report) Scenario(id string) *ScenarioStats {
	for _, s := range r.Scenarios {
		if s.ID == id {
			return s
		}
	}
	return nil
}
