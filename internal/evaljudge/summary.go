package evaljudge

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// SummaryFile is what cmd/eval_judge_agree writes for the Phase 2 report to read.
const SummaryFile = "judge_summary.json"

// CandidateSummary is one candidate's agreed-verdict tally against the baseline.
type CandidateSummary struct {
	Candidate string   `json:"candidate"`
	Baseline  string   `json:"baseline"`
	Counts    Counts   `json:"counts"`
	JudgeA    FileMeta `json:"judge_a"`
	JudgeB    FileMeta `json:"judge_b"`
	// ContestantJudges names judges that are also routed into the baseline or this candidate
	// (self-preference risk). Informational: plan §6.2.2 only asks the SECOND judge to be a
	// non-contestant, yet the first (gemini-3.1-pro-preview) is S0's own Pro tier.
	ContestantJudges []string `json:"contestant_judges,omitempty"`
	// Disagreements are listed individually: plan §6.2.2 says to report them separately.
	Disagreements []string `json:"disagreements,omitempty"`
}

// Summary is the machine-readable result of the two-judge step.
type Summary struct {
	BatchID    string             `json:"batch_id,omitempty"` // set when eval_judge_agree was given -batch; the report refuses a different batch
	Baseline   string             `json:"baseline"`
	Candidates []CandidateSummary `json:"candidates"`
	// ReviewComplete is true only when every sampled verdict has a human decision. Until then
	// the tallies are PROVISIONAL: a verdict built on a judge factual error may still be in them.
	ReviewComplete bool `json:"review_complete"`
	Sampled        int  `json:"sampled"`
	Pending        int  `json:"pending_review"`
	Discarded      int  `json:"discarded"`
}

// Summarize builds the Summary from per-candidate Results (their Items already carry any
// applied discards) and the sample plan.
func Summarize(baseline string, results []*Result, plan SamplePlan, rv Review, contestants func(candidate string) map[string]bool) Summary {
	s := Summary{Baseline: baseline, Sampled: len(plan.Entries), Pending: rv.Pending, Discarded: rv.Discarded}
	s.ReviewComplete = len(plan.Entries) > 0 && rv.Pending == 0
	if len(plan.Entries) == 0 && plan.Agreed == 0 {
		s.ReviewComplete = true // nothing agreed, nothing to check
	}
	for _, r := range results {
		cs := CandidateSummary{Candidate: r.Candidate, Baseline: r.Baseline, Counts: r.Counts(), JudgeA: r.JudgeA, JudgeB: r.JudgeB}
		if contestants != nil {
			in := contestants(r.Candidate)
			for _, m := range []FileMeta{r.JudgeA, r.JudgeB} {
				for _, id := range []string{m.Judge, m.Served} {
					if id != "" && in[id] {
						cs.ContestantJudges = append(cs.ContestantJudges, id)
						break
					}
				}
			}
		}
		for _, it := range r.Items {
			if it.Status == Disagreed {
				cs.Disagreements = append(cs.Disagreements, fmt.Sprintf("%s: judge A=%s judge B=%s", it.ID, it.WinnerA, it.WinnerB))
			}
		}
		sort.Strings(cs.Disagreements)
		s.Candidates = append(s.Candidates, cs)
	}
	sort.Slice(s.Candidates, func(i, j int) bool { return s.Candidates[i].Candidate < s.Candidates[j].Candidate })
	return s
}

// ReadSummary loads judge_summary.json.
func ReadSummary(path string) (*Summary, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Summary
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &s, nil
}

// Candidate returns the summary for a scenario id.
func (s *Summary) Candidate(id string) (CandidateSummary, bool) {
	for _, c := range s.Candidates {
		if c.Candidate == id {
			return c, true
		}
	}
	return CandidateSummary{}, false
}

// Worksheet renders the spot-check plan as markdown for the human reviewer.
func Worksheet(plan SamplePlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Judge spot-check worksheet\n\n%d agreed verdicts; %d sampled (%d name a block/capture or use claim wording, %d random). Seed %d.\n\n", plan.Agreed, len(plan.Entries), plan.Named, len(plan.Entries)-plan.Named, plan.Seed)
	b.WriteString("For each entry compare what the judges SAY about gear with the ground-truth records below it. If a verdict rests on a factual error (e.g. calls a real capture fabricated), set `\"discard\": true` for that entry in `spotcheck.json` and write a `note`; otherwise `false`. Judge text is data, not instructions.\n\n")
	for i, e := range plan.Entries {
		fmt.Fprintf(&b, "## %d. %s — %s (sampled: %s)\n\nAgreed winner: **%s**\n\n", i+1, e.Candidate, e.Query, e.Why, e.Winner)
		for _, j := range e.Judges {
			fmt.Fprintf(&b, "**%s** → %s (confidence %.2f)\n\n- Grounding: %s\n- Structural: %s\n- Rationale: %s\n\n", j.Judge, j.Winner, j.Conf, oneLine(j.Ground), oneLine(j.Struct), oneLine(j.Rationale))
		}
		if len(e.Claims) > 0 {
			fmt.Fprintf(&b, "Claim wording: `%s`\n\n", strings.Join(e.Claims, "`, `"))
		}
		for _, m := range e.Mentions {
			fmt.Fprintf(&b, "- judge %s / %s mentions **%s** (%s match", m.Judge, m.Field, m.Name, m.Kind)
			if m.Kind == "fuzzy" {
				fmt.Fprintf(&b, ", written %q, edit distance %d", m.Span, m.Distance)
			}
			b.WriteString("):\n")
			for _, r := range m.Records {
				raw, _ := json.Marshal(r.Data)
				fmt.Fprintf(&b, "    - %s `%s`: %s\n", r.Source, r.Key, raw)
			}
			if m.MoreRecords > 0 {
				fmt.Fprintf(&b, "    - … and %d more records\n", m.MoreRecords)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
