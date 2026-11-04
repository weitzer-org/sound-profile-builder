package main

import (
	"fmt"
	"sort"
	"strings"
)

// CallResult is one executed call, written as one JSON line to calls.jsonl.
type CallResult struct {
	Agent         string   `json:"agent"`
	AgentKey      string   `json:"agent_key"`
	Model         string   `json:"model"` // requested
	Baseline      bool     `json:"baseline"`
	Query         string   `json:"query"`
	Rep           int      `json:"rep"`
	PromptVersion string   `json:"prompt_version,omitempty"`
	ServedModel   string   `json:"served_model"`
	ServedOK      bool     `json:"served_ok"` // answered by the requested model (3.5-flash pinned to 3.6-flash)
	FinishReason  string   `json:"finish_reason"`
	Success       bool     `json:"success"`
	Error         string   `json:"error,omitempty"`
	Truncated     bool     `json:"truncated"` // finish_reason == MAX_TOKENS
	Grounded      bool     `json:"grounded"`
	SearchQueries int      `json:"search_queries"`
	SchemaValid   bool     `json:"schema_valid"`
	SchemaIssues  []string `json:"schema_issues,omitempty"`
	CallCount     int      `json:"call_count"` // attempts recorded for this job; anything but 1 means a retry/fallback happened
	LatencyMs     int64    `json:"latency_ms"`
	InputTokens   int32    `json:"input_tokens"`
	OutputTokens  int32    `json:"output_tokens"`
	ThinkTokens   int32    `json:"thinking_tokens"`
	Output        string   `json:"output,omitempty"`
}

// Stat aggregates every call of one (agent, model).
type Stat struct {
	Agent, Key, Model string
	Baseline          bool
	N                 int
	Success           int
	SchemaValid       int
	Truncated         int
	Grounded          int
	ServedBad         int // served by a different model than requested, or >1 attempt
	Errors            int // failed for a reason other than truncation
	LatencyP50Ms      int64
	LatencyP95Ms      int64
	MeanIn, MeanOut   float64
	MeanThink         float64
}

func (s Stat) rate(n int) float64 {
	if s.N == 0 {
		return 0
	}
	return float64(n) / float64(s.N)
}

// Aggregate groups results by (agent, model), preserving first-seen order.
func Aggregate(rs []CallResult) []Stat {
	type k struct{ key, model string }
	idx := map[k]int{}
	var stats []Stat
	lat := map[k][]int64{}
	for _, r := range rs {
		kk := k{r.AgentKey, r.Model}
		i, ok := idx[kk]
		if !ok {
			i = len(stats)
			idx[kk] = i
			stats = append(stats, Stat{Agent: r.Agent, Key: r.AgentKey, Model: r.Model, Baseline: r.Baseline})
		}
		s := &stats[i]
		s.N++
		if r.Success {
			s.Success++
		}
		if r.SchemaValid {
			s.SchemaValid++
		}
		if r.Truncated {
			s.Truncated++
		} else if !r.Success {
			s.Errors++
		}
		if r.Grounded {
			s.Grounded++
		}
		if !r.ServedOK || r.CallCount != 1 {
			s.ServedBad++
		}
		s.MeanIn += float64(r.InputTokens)
		s.MeanOut += float64(r.OutputTokens)
		s.MeanThink += float64(r.ThinkTokens)
		lat[kk] = append(lat[kk], r.LatencyMs)
	}
	for kk, i := range idx {
		s := &stats[i]
		if s.N > 0 {
			s.MeanIn /= float64(s.N)
			s.MeanOut /= float64(s.N)
			s.MeanThink /= float64(s.N)
		}
		l := lat[kk]
		sort.Slice(l, func(a, b int) bool { return l[a] < l[b] })
		s.LatencyP50Ms = pct(l, 0.50)
		s.LatencyP95Ms = pct(l, 0.95)
	}
	return stats
}

func pct(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(p*float64(len(sorted)) + 0.5)
	if i < 1 {
		i = 1
	}
	if i > len(sorted) {
		i = len(sorted)
	}
	return sorted[i-1]
}

// Verdict applies the plan's Phase 1 exit rule to one candidate against its agent's baseline:
// dropped if its schema-validity rate is lower than baseline's, its truncation rate is higher, or
// any call was not served by the requested model. Strict comparison, no tolerance: with n=26 per
// cell a single call is ~4 points, so the report also prints the raw counts.
func Verdict(base, cand Stat) (drop bool, reasons []string) {
	if cand.SchemaValid*base.N < base.SchemaValid*cand.N { // cand rate < base rate, cross-multiplied
		reasons = append(reasons, fmt.Sprintf("schema validity %d/%d below baseline %d/%d", cand.SchemaValid, cand.N, base.SchemaValid, base.N))
	}
	if cand.Truncated*base.N > base.Truncated*cand.N {
		reasons = append(reasons, fmt.Sprintf("truncation %d/%d above baseline %d/%d", cand.Truncated, cand.N, base.Truncated, base.N))
	}
	if cand.ServedBad > 0 {
		reasons = append(reasons, fmt.Sprintf("%d call(s) not served by the requested model or retried", cand.ServedBad))
	}
	return len(reasons) > 0, reasons
}

// Markdown renders the per-(agent, model) table and the exit-rule verdicts.
func Markdown(stats []Stat) string {
	var sb strings.Builder
	sb.WriteString("# Phase 1A: independent-agent screen\n\n")
	sb.WriteString("Rates are over all calls of the cell (failed and truncated calls count as schema-invalid). `grounded` = response carried search queries or grounding chunks. Latency is the single API attempt.\n\n")
	sb.WriteString("| Agent | Model | role | n | schema valid | truncated | other errors | grounded | served-model issues | p50 / p95 latency (s) | mean in / out / think tokens |\n| :-- | :-- | :-- | --: | --: | --: | --: | --: | --: | --: | :-- |\n")
	for _, s := range stats {
		role := "candidate"
		if s.Baseline {
			role = "baseline"
		}
		sb.WriteString(fmt.Sprintf("| %s | `%s` | %s | %d | %d (%.0f%%) | %d (%.0f%%) | %d | %d (%.0f%%) | %d | %.1f / %.1f | %.0f / %.0f / %.0f |\n",
			s.Agent, s.Model, role, s.N,
			s.SchemaValid, 100*s.rate(s.SchemaValid), s.Truncated, 100*s.rate(s.Truncated), s.Errors,
			s.Grounded, 100*s.rate(s.Grounded), s.ServedBad,
			float64(s.LatencyP50Ms)/1000, float64(s.LatencyP95Ms)/1000, s.MeanIn, s.MeanOut, s.MeanThink))
	}
	sb.WriteString("\n## Exit rule (candidate dropped from the agent if schema validity is below baseline, truncation above baseline, or any served-model issue)\n\n")
	base := map[string]Stat{}
	for _, s := range stats {
		if s.Baseline {
			base[s.Key] = s
		}
	}
	for _, s := range stats {
		if s.Baseline {
			continue
		}
		b, ok := base[s.Key]
		if !ok {
			sb.WriteString(fmt.Sprintf("- %s / `%s`: no baseline in this run, cannot apply the rule\n", s.Agent, s.Model))
			continue
		}
		if drop, why := Verdict(b, s); drop {
			sb.WriteString(fmt.Sprintf("- **DROP** %s / `%s`: %s\n", s.Agent, s.Model, strings.Join(why, "; ")))
		} else {
			sb.WriteString(fmt.Sprintf("- keep %s / `%s`\n", s.Agent, s.Model))
		}
	}
	return sb.String()
}
