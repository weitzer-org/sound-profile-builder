package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

const (
	modelProBaseline   = "gemini-3.1-pro-preview"
	modelFlashBaseline = "gemini-3.5-flash"
	modelArgon         = "gemini-4-argon"
	modelFlash38       = "gemini-3.8-flash"

	// gemini-3.5-flash is served as gemini-3.6-flash (Phase 0.4). Requested id kept, served id pinned.
	flashBaselineServed = "gemini-3.6-flash"

	architectRole = "Architect & Evaluator"
	criticRole    = "Preset Critic"
	architectKey  = "12_architect"
	criticKey     = "13_critic"
)

var pins = map[string]string{modelFlashBaseline: flashBaselineServed}

// BaselineRouting is production's routing for every agent (RunAgentSplit defaults), spelled out so
// a run never depends on a default: Pro tier = agents 1 and 12, everything else Flash.
func BaselineRouting() map[string]string {
	m := map[string]string{}
	for _, k := range agents.AgentRoleKeys {
		switch k {
		case "1_tone_historian", architectKey:
			m[k] = modelProBaseline
		default:
			m[k] = modelFlashBaseline
		}
	}
	return m
}

// RRule groups queries into the plan's regression repros (§4 1B, as redefined in the Phase 0
// corrections): both use the same machine check, capture_claim_on_modeled or capture_db_on_modeled > 0.
type RRule struct {
	ID      string
	Queries []string
}

func Rules() []RRule {
	return []RRule{
		{ID: "R1", Queries: []string{"07_Edge"}},
		{ID: "R2", Queries: []string{"09_BB_King", "05_Clapton"}},
	}
}

// RuleQueries is every query the rules need, in rule order, deduplicated.
func RuleQueries() []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range Rules() {
		for _, q := range r.Queries {
			if !seen[q] {
				seen[q] = true
				out = append(out, q)
			}
		}
	}
	return out
}

// Job is one Architect (+Critic) repro run.
type Job struct {
	Model string
	Query string
	Rep   int
}

// BuildJobs enumerates the repro runs, control model first (so the harness can stop before
// spending on candidates if the control is broken), then candidates; within a model: query, rep.
func BuildJobs(models, queries []string, reps int) []Job {
	var jobs []Job
	for _, m := range models {
		for _, q := range queries {
			for r := 1; r <= reps; r++ {
				jobs = append(jobs, Job{Model: m, Query: q, Rep: r})
			}
		}
	}
	return jobs
}

// RunResult is one executed repro run.
type RunResult struct {
	Model             string            `json:"model"`
	Query             string            `json:"query"`
	Rep               int               `json:"rep"`
	PromptMatches     bool              `json:"prompt_matches_frozen"` // production prompt selection for this model == the frozen system prompt
	ArchPromptVersion string            `json:"architect_prompt_version,omitempty"`
	ThinkingLevel     string            `json:"architect_thinking_level,omitempty"`
	ArchServed        string            `json:"architect_served_model"`
	ArchFinish        string            `json:"architect_finish_reason"`
	ArchSuccess       bool              `json:"architect_success"`
	ArchError         string            `json:"architect_error,omitempty"`
	ArchLatencyMs     int64             `json:"architect_latency_ms"`
	ArchInTokens      int32             `json:"architect_input_tokens"`
	ArchOutTokens     int32             `json:"architect_output_tokens"`
	ArchThinkTokens   int32             `json:"architect_thinking_tokens"`
	ArchIntegrityOK   bool              `json:"architect_integrity_ok"` // answered by the requested model, one attempt, success
	CriticServed      string            `json:"critic_served_model,omitempty"`
	CriticFinish      string            `json:"critic_finish_reason,omitempty"`
	CriticSuccess     bool              `json:"critic_success"`
	CriticError       string            `json:"critic_error,omitempty"`
	CriticIntegrity   bool              `json:"critic_integrity_ok"`
	MaxTokens         map[string]int    `json:"max_tokens_by_agent,omitempty"`
	IntegrityIssues   []string          `json:"integrity_issues,omitempty"`
	Scoreable         bool              `json:"scoreable"` // architect integrity ok AND output parsed; critic status does not affect the R check
	Report            *evalcheck.Report `json:"report,omitempty"`
	CaptureDefect     bool              `json:"capture_defect"` // capture_claim_on_modeled>0 || capture_db_on_modeled>0
	OutputFile        string            `json:"output_file,omitempty"`
	CallsFile         string            `json:"calls_file,omitempty"`
}

// Defect applies the redefined R1/R2 condition to a report.
func Defect(r evalcheck.Report) bool { return r.CaptureClaimOnModeled > 0 || r.CaptureDBOnModeled > 0 }

// Verdict values.
const (
	Pass         = "PASS"
	Fail         = "FAIL"
	Inconclusive = "INCONCLUSIVE"
)

// RuleVerdict is one (model, rule) outcome.
type RuleVerdict struct {
	Model, Rule         string
	Runs, Scoreable     int
	DefectRuns          int
	Verdict             string
	PerQuery            map[string][2]int // query -> {defect runs, scoreable runs}
	ExpectedRunsPerRule int
}

// Judge folds results into a verdict per (model, rule): FAIL if any scoreable run has the defect;
// otherwise INCONCLUSIVE if any run was unscoreable or fewer runs exist than wantPerQuery x queries
// (a missing run is not a pass); otherwise PASS.
func Judge(results []RunResult, models []string, wantPerQuery int) []RuleVerdict {
	return JudgeFor(results, models, wantPerQuery, nil)
}

// JudgeFor is Judge restricted to the queries actually selected for the run (nil = all rule
// queries). A rule none of whose queries were selected is omitted; one with only some selected is
// judged on those, so a subset run cannot be flagged INCONCLUSIVE for queries it never asked for.
func JudgeFor(results []RunResult, models []string, wantPerQuery int, selected []string) []RuleVerdict {
	sel := map[string]bool{}
	for _, q := range selected {
		sel[q] = true
	}
	var out []RuleVerdict
	for _, m := range models {
		for _, full := range Rules() {
			rule := RRule{ID: full.ID}
			for _, q := range full.Queries {
				if selected == nil || sel[q] {
					rule.Queries = append(rule.Queries, q)
				}
			}
			if len(rule.Queries) == 0 {
				continue
			}
			v := RuleVerdict{Model: m, Rule: rule.ID, PerQuery: map[string][2]int{}, ExpectedRunsPerRule: wantPerQuery * len(rule.Queries)}
			for _, q := range rule.Queries {
				for _, r := range results {
					if r.Model != m || r.Query != q {
						continue
					}
					v.Runs++
					pq := v.PerQuery[q]
					if r.Scoreable {
						v.Scoreable++
						pq[1]++
						if r.CaptureDefect {
							v.DefectRuns++
							pq[0]++
						}
					}
					v.PerQuery[q] = pq
				}
			}
			switch {
			case v.DefectRuns > 0:
				v.Verdict = Fail
			case v.Scoreable < v.ExpectedRunsPerRule:
				v.Verdict = Inconclusive
			default:
				v.Verdict = Pass
			}
			out = append(out, v)
		}
	}
	return out
}

// ControlOK reports whether the control model passed every rule; if not, the harness must stop.
func ControlOK(vs []RuleVerdict, control string) (bool, []string) {
	ok := true
	var why []string
	for _, v := range vs {
		if v.Model == control && v.Verdict != Pass {
			ok = false
			why = append(why, fmt.Sprintf("%s: control %s (%d defect run(s), %d/%d scoreable)", v.Rule, v.Verdict, v.DefectRuns, v.Scoreable, v.ExpectedRunsPerRule))
		}
	}
	return ok, why
}

// ModelStats aggregates secondary metrics per model.
type ModelStats struct {
	Model                                string
	Runs, Scoreable, ArchSuccess, MaxTok int
	GearBlocks, Snapped, Unverified      int
	CriticFlags, ModeledStyleOnCapture   int
	MeanArchLatencyS, MeanOut, MeanThink float64
	ServedModels                         map[string]int
}

func Stats(results []RunResult, models []string) []ModelStats {
	var out []ModelStats
	for _, m := range models {
		s := ModelStats{Model: m, ServedModels: map[string]int{}}
		for _, r := range results {
			if r.Model != m {
				continue
			}
			s.Runs++
			if r.ArchSuccess {
				s.ArchSuccess++
			}
			if r.Scoreable {
				s.Scoreable++
			}
			s.MaxTok += r.MaxTokens[architectRole]
			s.MeanArchLatencyS += float64(r.ArchLatencyMs) / 1000
			s.MeanOut += float64(r.ArchOutTokens)
			s.MeanThink += float64(r.ArchThinkTokens)
			s.ServedModels[r.ArchServed]++
			if r.Report != nil {
				s.GearBlocks += r.Report.GearBlocks
				s.Snapped += r.Report.SnappedNames
				s.Unverified += r.Report.UnverifiedNames
				s.CriticFlags += r.Report.CriticFlags
				s.ModeledStyleOnCapture += r.Report.ModeledStyleOnCapture
			}
		}
		if s.Runs > 0 {
			s.MeanArchLatencyS /= float64(s.Runs)
			s.MeanOut /= float64(s.Runs)
			s.MeanThink /= float64(s.Runs)
		}
		out = append(out, s)
	}
	return out
}

// Markdown renders the verdict and secondary tables.
func Markdown(results []RunResult, models []string, reps int, selected []string) string {
	var sb strings.Builder
	sb.WriteString("# Phase 1B: Architect known-regression repros\n\n")
	sb.WriteString("Upstream agents 1-11 are frozen once per query on baseline models; only the Architect (agent 12) changes, with the Critic on baseline. Defect = `capture_claim_on_modeled > 0` or `capture_db_on_modeled > 0` (internal/evalcheck). A run is scoreable only if the Architect call succeeded, was served by the requested model in one attempt, and parsed.\n\n")
	sb.WriteString("| Model | Rule | defect runs / scoreable (expected) | per query (defect/scoreable) | Verdict |\n| :-- | :-- | --: | :-- | :-- |\n")
	for _, v := range JudgeFor(results, models, reps, selected) {
		var qs []string
		for _, r := range Rules() {
			if r.ID != v.Rule {
				continue
			}
			for _, q := range r.Queries {
				pq := v.PerQuery[q]
				qs = append(qs, fmt.Sprintf("%s %d/%d", q, pq[0], pq[1]))
			}
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %d / %d (%d) | %s | **%s** |\n", v.Model, v.Rule, v.DefectRuns, v.Scoreable, v.ExpectedRunsPerRule, strings.Join(qs, "; "), v.Verdict))
	}
	sb.WriteString("\n## Secondary metrics (reported, not gated by 1B)\n\n")
	sb.WriteString("| Model | runs | Architect ok | scoreable | MAX_TOKENS (Architect) | gear blocks | fuzzy-snapped names | unverified names | modeled-style-on-capture | Critic flags | mean latency (s) | mean out / think tokens | served as |\n| :-- | --: | --: | --: | --: | --: | --: | --: | --: | --: | --: | :-- | :-- |\n")
	for _, s := range Stats(results, models) {
		var served []string
		for k, v := range s.ServedModels {
			served = append(served, fmt.Sprintf("%s x%d", k, v))
		}
		sort.Strings(served)
		sb.WriteString(fmt.Sprintf("| `%s` | %d | %d | %d | %d | %d | %d | %d | %d | %d | %.1f | %.0f / %.0f | %s |\n",
			s.Model, s.Runs, s.ArchSuccess, s.Scoreable, s.MaxTok, s.GearBlocks, s.Snapped, s.Unverified, s.ModeledStyleOnCapture, s.CriticFlags, s.MeanArchLatencyS, s.MeanOut, s.MeanThink, strings.Join(served, ", ")))
	}
	return sb.String()
}

// freezeObserver records the exact prompts RunPipeline builds for the Architect and every upstream
// agent's output, then cancels the run the instant the Architect is about to be called, so no
// Architect (or Critic) money is spent while freezing. It is safe for concurrent use.
type freezeObserver struct {
	cancel context.CancelFunc
	mu     sync.Mutex

	ArchSystem, ArchUser string
	Upstream             []UpstreamOutput
	captured             bool
}

// UpstreamOutput is one frozen agent 1-11 result.
type UpstreamOutput struct {
	Role   string `json:"role"`
	Model  string `json:"model"`
	Output string `json:"output"`
}

func (f *freezeObserver) OnPhase(string) {}
func (f *freezeObserver) OnInfo(string)  {}
func (f *freezeObserver) OnAttempt(string, string, int64, error) {
}
func (f *freezeObserver) OnAgentStart(role, system, user string) {
	if role != architectRole {
		return
	}
	f.mu.Lock()
	f.ArchSystem, f.ArchUser, f.captured = system, user, true
	f.mu.Unlock()
	f.cancel()
}
func (f *freezeObserver) OnAgentDone(role, model, output string, _ agents.AgentUsage, err error) {
	if role == architectRole || err != nil {
		return
	}
	f.mu.Lock()
	f.Upstream = append(f.Upstream, UpstreamOutput{Role: role, Model: model, Output: output})
	f.mu.Unlock()
}

// Captured reports whether the Architect prompt was reached.
func (f *freezeObserver) Captured() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.captured
}
