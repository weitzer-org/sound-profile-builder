package evalscore

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Judge compares two rendered presets for one golden query and says which is better.
// verdict is "A", "B" or "Equal". Implementations wrap an LLM call; tests inject a fake.
type Judge func(ctx context.Context, query, presetA, presetB string) (verdict string, confidence float64, rationale string, err error)

// PairVerdict is the outcome of judging one pair BOTH ways round (positions swapped), which
// removes the judge's position bias from a single verdict and exposes its self-inconsistency.
type PairVerdict struct {
	Query  string `json:"query"`
	Sample int    `json:"sample"`
	// Winner is "first", "second", "equal", "inconsistent" or "error". It is a winner only
	// when both orderings agree; any disagreement (including Equal vs a winner) is
	// "inconsistent" rather than being coerced into a preference.
	Winner  string    `json:"winner"`
	Verdict [2]string `json:"verdicts"` // raw: [first-as-A, first-as-B]
	Err     string    `json:"err,omitempty"`
}

// JudgeBothWays judges (first, second) twice: first as A, then first as B.
func JudgeBothWays(ctx context.Context, j Judge, query string, sample int, first, second string) PairVerdict {
	pv := PairVerdict{Query: query, Sample: sample}
	v1, _, _, err := j(ctx, query, first, second)
	if err != nil {
		pv.Winner, pv.Err = "error", err.Error()
		return pv
	}
	v2, _, _, err := j(ctx, query, second, first)
	if err != nil {
		pv.Winner, pv.Err = "error", err.Error()
		return pv
	}
	pv.Verdict = [2]string{v1, v2}
	// Translate each verdict into "first" / "second" / "equal".
	t1 := map[string]string{"A": "first", "B": "second", "Equal": "equal"}[v1]
	t2 := map[string]string{"A": "second", "B": "first", "Equal": "equal"}[v2]
	switch {
	case t1 == "" || t2 == "":
		pv.Winner = "error"
		pv.Err = fmt.Sprintf("unrecognised verdicts %q, %q", v1, v2)
	case t1 == t2:
		pv.Winner = t1
	default:
		pv.Winner = "inconsistent"
	}
	return pv
}

// JudgeSummary aggregates PairVerdicts.
type JudgeSummary struct {
	N                                          int
	First, Second, Equal, Inconsistent, Errors int
	InconsistentRate                           float64 // of non-error pairs: the judge's own noise
	SignP                                      float64 // two-sided, consistent winners only
	AShare                                     float64 // share of raw verdicts that picked position A (0.5 = no position bias)
}

// Summarise aggregates verdicts.
func Summarise(pvs []PairVerdict) JudgeSummary {
	var s JudgeSummary
	var aPicks, raw int
	for _, p := range pvs {
		s.N++
		switch p.Winner {
		case "first":
			s.First++
		case "second":
			s.Second++
		case "equal":
			s.Equal++
		case "inconsistent":
			s.Inconsistent++
		default:
			s.Errors++
		}
		if p.Winner != "error" {
			for _, v := range p.Verdict {
				raw++
				if v == "A" {
					aPicks++
				}
			}
		}
	}
	if ok := s.N - s.Errors; ok > 0 {
		s.InconsistentRate = float64(s.Inconsistent) / float64(ok)
	}
	if raw > 0 {
		s.AShare = float64(aPicks) / float64(raw)
	}
	s.SignP = SignTestP(s.First, s.Second)
	return s
}

// JudgeMarkdown renders a head-to-head and its calibration side by side. firstLabel/secondLabel
// name the conditions (e.g. new/old); aa may be empty if no calibration was run.
func JudgeMarkdown(firstLabel, secondLabel string, ab, aa []PairVerdict) string {
	var b strings.Builder
	row := func(name string, pvs []PairVerdict, l1, l2 string) {
		s := Summarise(pvs)
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %.0f%% | %.3f | %.2f |\n", name, s.N, s.First, s.Second, s.Equal, s.Inconsistent, 100*s.InconsistentRate, s.SignP, s.AShare)
		_ = l1
		_ = l2
	}
	fmt.Fprintf(&b, "# Blind LLM judge, every pair judged twice with positions swapped\n\n")
	fmt.Fprintf(&b, "'first' = %s in the head-to-head; in the same-condition calibration it is the lower-numbered sample. A winner counts only when BOTH orderings agree.\n\n", firstLabel)
	b.WriteString("| comparison | pairs | first wins | second wins | equal | inconsistent | inconsistent rate | sign-test p (consistent winners) | share of raw verdicts picking position A |\n|---|---|---|---|---|---|---|---|---|\n")
	row(firstLabel+" vs "+secondLabel, ab, firstLabel, secondLabel)
	if len(aa) > 0 {
		row("CALIBRATION: same condition, two samples", aa, "s_lo", "s_hi")
	}
	b.WriteString("\nHow to read it: the calibration row is what pure run-to-run variation looks like to this judge. A head-to-head only means something to the extent it is more lopsided than the calibration row; a high inconsistent rate means the judge cannot be trusted on these pairs.\n\n")
	sorted := append([]PairVerdict(nil), ab...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Query < sorted[j].Query })
	b.WriteString("## Head-to-head per pair\n\n| query | sample | winner | raw [first as A, first as B] |\n|---|---|---|---|\n")
	for _, p := range sorted {
		w := p.Winner
		switch w {
		case "first":
			w = firstLabel
		case "second":
			w = secondLabel
		}
		fmt.Fprintf(&b, "| %s | %d | %s | %v |\n", p.Query, p.Sample, w, p.Verdict)
	}
	return b.String()
}
