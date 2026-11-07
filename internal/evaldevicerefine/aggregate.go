package evaldevicerefine

import (
	"fmt"
	"io"
	"sort"
)

// RunScore is one attempted run with its score. Attempted runs that failed (a Gemini error)
// or failed the served-model integrity check are kept so the report can say what failed in
// which arm; only Usable ones can enter a comparison.
type RunScore struct {
	Run
	Success     bool        `json:"success"`
	IntegrityOK bool        `json:"integrity_ok"`
	Error       string      `json:"error,omitempty"`
	Metrics     Metrics     `json:"metrics"`
	Judge       *JudgeScore `json:"judge,omitempty"`
}

// Usable: the model answered as asked and the output parsed and planned.
func (r RunScore) Usable() bool { return r.Success && r.IntegrityOK && r.Metrics.Valid() }

// JudgeScore is the blind judge's verdict on one run (see judge.go).
type JudgeScore struct {
	Addressed int  `json:"addressed"` // 0 no, 1 partly, 2 fully
	ToneOK    bool `json:"tone_ok"`
}

// Pair is one triple with both arms' scores.
type Pair struct {
	Triple
	A, B RunScore
}

// Pairs returns the triples usable in BOTH arms, sorted, and, for the report, the triples
// that were attempted in either arm but are not in the comparison.
func Pairs(runs []RunScore) (pairs []Pair, dropped []Triple) {
	by := map[Triple]map[string]RunScore{}
	for _, r := range runs {
		if by[r.Triple] == nil {
			by[r.Triple] = map[string]RunScore{}
		}
		by[r.Triple][r.Arm] = r
	}
	for t, arms := range by {
		a, okA := arms[ArmA]
		b, okB := arms[ArmB]
		if okA && okB && a.Usable() && b.Usable() {
			pairs = append(pairs, Pair{Triple: t, A: a, B: b})
		} else {
			dropped = append(dropped, t)
		}
	}
	less := func(x, y Triple) bool {
		if x.Draft != y.Draft {
			return x.Draft < y.Draft
		}
		if x.Request != y.Request {
			return x.Request < y.Request
		}
		return x.Rep < y.Rep
	}
	sort.Slice(pairs, func(i, j int) bool { return less(pairs[i].Triple, pairs[j].Triple) })
	sort.Slice(dropped, func(i, j int) bool { return less(dropped[i], dropped[j]) })
	return pairs, dropped
}

// Summary is one arm's numbers over a set of runs. Ratios are pooled (sums over runs), not
// means of per-run ratios, so a run with one change does not weigh like a run with forty.
type Summary struct {
	Runs         int
	NoChange     int
	Ops          int
	NotApplied   int
	Kept         int
	DraftBlocks  int
	FullyKept    int // runs that kept every draft block
	Locked       int
	LockedRuns   int
	Structural   int
	Unasked      int
	UnaskedRuns  int
	MechRuns     int
	Defects      int
	DeviceDefect int
	JudgeRuns    int
	Addressed    int
	ToneOK       int
	Truncated    int // runs whose not-applied list was capped
}

// Summarize pools the scores of one arm.
func Summarize(rs []RunScore) Summary {
	var s Summary
	for _, r := range rs {
		m := r.Metrics
		s.Runs++
		if m.NoChange {
			s.NoChange++
		}
		s.Ops += m.Ops
		s.NotApplied += m.NotApplied
		s.Kept += m.Kept
		s.DraftBlocks += m.DraftBlocks
		if m.Kept == m.DraftBlocks && m.OutBlocks == m.DraftBlocks {
			s.FullyKept++
		}
		s.Locked += m.Locked
		if m.Locked > 0 {
			s.LockedRuns++
		}
		s.Structural += m.Structural
		s.Unasked += m.Unasked
		if m.Unasked > 0 {
			s.UnaskedRuns++
		}
		if m.CodesTruncated {
			s.Truncated++
		}
		if m.MechChecked {
			s.MechRuns++
			s.Defects += m.Defects
			s.DeviceDefect += m.DeviceDefects
		}
		if r.Judge != nil {
			s.JudgeRuns++
			s.Addressed += r.Judge.Addressed
			if r.Judge.ToneOK {
				s.ToneOK++
			}
		}
	}
	return s
}

// ApplicableShare is ops / (ops + not-applied), pooled; ok is false when there were no changes.
func (s Summary) ApplicableShare() (float64, bool) {
	if s.Ops+s.NotApplied == 0 {
		return 0, false
	}
	return float64(s.Ops) / float64(s.Ops+s.NotApplied), true
}

// Preservation is kept / draft blocks, pooled.
func (s Summary) Preservation() float64 {
	if s.DraftBlocks == 0 {
		return 0
	}
	return float64(s.Kept) / float64(s.DraftBlocks)
}

func per(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

// Criterion is one row of section 8.1's "Pass for arm B" column, with the numbers behind it.
type Criterion struct {
	Metric string
	Rule   string
	A, B   string
	Pass   bool
	// Note says what the rule operationalises, when the design doc's wording is not a number.
	Note string
}

// Criteria evaluates the section 8.1 pass column over the paired triples. The thresholds
// the doc gives as numbers are used as given (share >= 80%, preservation >= 95%, locked 0).
// Where it says "clearly above", "below" or "no worse" the operationalisation is stated in Note.
func Criteria(pairs []Pair) []Criterion {
	var as, bs []RunScore
	for _, p := range pairs {
		as, bs = append(as, p.A), append(bs, p.B)
	}
	a, b := Summarize(as), Summarize(bs)
	pct := func(v float64, ok bool) string {
		if !ok {
			return "n/a"
		}
		return fmt.Sprintf("%.1f%%", 100*v)
	}
	shA, okA := a.ApplicableShare()
	shB, okB := b.ApplicableShare()
	var out []Criterion
	out = append(out, Criterion{Metric: "Applicable share", Rule: "B >= 80% and B at least 5 points above A",
		A: pct(shA, okA), B: pct(shB, okB), Pass: okA && okB && shB >= 0.80 && shB-shA >= 0.05,
		Note: "\"clearly above A\" is operationalised as +5 points; the doc gives only the 80% target"})
	out = append(out, Criterion{Metric: "Block preservation", Rule: "B >= 95%",
		A: pct(a.Preservation(), a.DraftBlocks > 0), B: pct(b.Preservation(), b.DraftBlocks > 0), Pass: b.DraftBlocks > 0 && b.Preservation() >= 0.95})
	out = append(out, Criterion{Metric: "Locked violations", Rule: "B = 0",
		A: fmt.Sprint(a.Locked), B: fmt.Sprint(b.Locked), Pass: b.Locked == 0 && b.Truncated == 0,
		Note: "a run whose not-applied list was capped cannot show 0, so it fails this row"})
	out = append(out, Criterion{Metric: "Unasked structural changes", Rule: "B below A",
		A: fmt.Sprint(a.Unasked), B: fmt.Sprint(b.Unasked), Pass: b.Unasked < a.Unasked,
		Note: "if A has none, B cannot be below it: the row then fails (not demonstrable)"})
	mA, mB := per(a.Defects, a.MechRuns), per(b.Defects, b.MechRuns)
	out = append(out, Criterion{Metric: "Mechanical checks (defects per run)", Rule: "B <= A",
		A: fmt.Sprintf("%.2f", mA), B: fmt.Sprintf("%.2f", mB), Pass: a.MechRuns > 0 && b.MechRuns > 0 && mB <= mA,
		Note: "means over runs that changed the matrix, so a no-change run does not look clean"})
	jA, jB := per(a.Addressed, a.JudgeRuns), per(b.Addressed, b.JudgeRuns)
	out = append(out, Criterion{Metric: "Request addressed (judge, 0-2)", Rule: "B >= A",
		A: fmt.Sprintf("%.2f (n=%d)", jA, a.JudgeRuns), B: fmt.Sprintf("%.2f (n=%d)", jB, b.JudgeRuns),
		Pass: a.JudgeRuns > 0 && b.JudgeRuns > 0 && jB >= jA,
		Note: "same model family as the Architect; blind and order-randomised, but not independent"})
	return out
}

// AllPass reports whether every criterion passed.
func AllPass(cs []Criterion) bool {
	for _, c := range cs {
		if !c.Pass {
			return false
		}
	}
	return len(cs) > 0
}

// Report writes the markdown report: per-arm attempt counts (what failed where), the paired
// comparison, the criteria, then per-request tables.
func Report(w io.Writer, runs []RunScore) {
	fmt.Fprint(w, "# E2 device-refine eval, scored\n\n")
	fmt.Fprint(w, "## Runs attempted and how they ended\n\n| Arm | Attempted | Gemini/transport failure | Failed integrity | Output did not parse | No plan built | Usable |\n|---|--:|--:|--:|--:|--:|--:|\n")
	for _, arm := range Arms {
		var n, failed, integ, parse, plan, usable int
		for _, r := range runs {
			if r.Arm != arm {
				continue
			}
			n++
			switch {
			case !r.Success:
				failed++
			case !r.IntegrityOK:
				integ++
			case !r.Metrics.Parsed:
				parse++
			case !r.Metrics.PlanOK:
				plan++
			default:
				usable++
			}
		}
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %d | %d |\n", arm, n, failed, integ, parse, plan, usable)
	}
	pairs, dropped := Pairs(runs)
	fmt.Fprintf(w, "\n## Paired comparison: %d (draft, request, rep) triples usable in both arms; %d triples left out\n\n", len(pairs), len(dropped))
	if len(dropped) > 0 {
		fmt.Fprint(w, "Left out (not usable in both arms): ")
		for i, t := range dropped {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprintf(w, "%s#%d", t.Key(), t.Rep)
		}
		fmt.Fprint(w, "\n\n")
	}
	fmt.Fprint(w, "| Metric | Rule for B | A | B | Pass |\n|---|---|--:|--:|:-:|\n")
	cs := Criteria(pairs)
	for _, c := range cs {
		mark := "FAIL"
		if c.Pass {
			mark = "pass"
		}
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n", c.Metric, c.Rule, c.A, c.B, mark)
	}
	for _, c := range cs {
		if c.Note != "" {
			fmt.Fprintf(w, "\n- %s: %s", c.Metric, c.Note)
		}
	}
	fmt.Fprintf(w, "\n\nAll section 8.1 rows pass for arm B: **%v**\n", AllPass(cs))

	fmt.Fprint(w, "\n## Per request (paired triples only)\n\n| Request | n | Share A | Share B | Kept A | Kept B | Unasked A | Unasked B | Locked A | Locked B |\n|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|\n")
	for _, id := range RequestIDs() {
		var as, bs []RunScore
		for _, p := range pairs {
			if p.Request == id {
				as, bs = append(as, p.A), append(bs, p.B)
			}
		}
		if len(as) == 0 {
			continue
		}
		a, b := Summarize(as), Summarize(bs)
		sa, oka := a.ApplicableShare()
		sb, okb := b.ApplicableShare()
		share := func(v float64, ok bool) string {
			if !ok {
				return "n/a"
			}
			return fmt.Sprintf("%.0f%%", 100*v)
		}
		fmt.Fprintf(w, "| %s | %d | %s | %s | %.0f%% | %.0f%% | %d | %d | %d | %d |\n", id, len(as), share(sa, oka), share(sb, okb),
			100*a.Preservation(), 100*b.Preservation(), a.Unasked, b.Unasked, a.Locked, b.Locked)
	}

	fmt.Fprint(w, "\n## Counts behind the rows (paired triples)\n\n| Arm | Runs | No-change | Ops | Not applied | Kept/draft blocks | Runs with locked | Structural | Unasked | Runs with unasked | Mech runs | Defects | Capped lists |\n|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|\n")
	var as, bs []RunScore
	for _, p := range pairs {
		as, bs = append(as, p.A), append(bs, p.B)
	}
	for _, x := range []struct {
		name string
		s    Summary
	}{{ArmA, Summarize(as)}, {ArmB, Summarize(bs)}} {
		s := x.s
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %d/%d | %d | %d | %d | %d | %d | %d | %d |\n", x.name, s.Runs, s.NoChange, s.Ops, s.NotApplied,
			s.Kept, s.DraftBlocks, s.LockedRuns, s.Structural, s.Unasked, s.UnaskedRuns, s.MechRuns, s.Defects, s.Truncated)
	}
}
