package evaljudge

import (
	"math"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evaltestutil"
)

const (
	base = "S0"
	cand = "S2"
)

// v builds a verdict as judge_compare writes it: pref is the judge's raw answer, swapped says
// the preset order shown was [cand as A, base as B].
func v(name, judge, served, pref string, swapped bool) Verdict {
	a, b := base, cand
	if swapped {
		a, b = cand, base
	}
	w := Equal
	switch pref {
	case "A":
		w = a
	case "B":
		w = b
	}
	return Verdict{Name: name, Winner: w, Conf: 0.9, Judge: judge, JudgeServed: served, SwappedAB: swapped,
		Preference: pref, PresentedA: a, PresentedB: b, DirA: "r/S0/rep1", DirB: "r/S2/rep1",
		Ground: "g", Struct: "s", Rationale: "r"}
}

func TestCompare_SwappedPositionsMapBackToScenarioLabels(t *testing.T) {
	// Judge 1 sees [S0,S2] and prefers A (= S0). Judge 2 sees [S2,S0] and prefers B (= S0). They AGREE on S0
	// even though their raw letters differ.
	a := []Verdict{v("q1", "gemini-3.1-pro-preview", "gemini-3.1-pro-preview", "A", false)}
	b := []Verdict{v("q1", "gemini-3.8-flash", "gemini-3.8-flash", "B", true)}
	res, err := Compare(a, b, "a", "b", Options{Baseline: base, Candidate: cand})
	if err != nil {
		t.Fatal(err)
	}
	if it := res.Items[0]; it.Status != Agreed || it.Winner != base {
		t.Fatalf("item = %+v", it)
	}
	// Same letter, opposite meaning: both say "A", but A is S0 for one judge and S2 for the other.
	a = []Verdict{v("q1", "j1", "j1", "A", false)}
	b = []Verdict{v("q1", "j2", "j2", "A", true)}
	res, _ = Compare(a, b, "a", "b", Options{Baseline: base, Candidate: cand})
	if res.Items[0].Status != Disagreed {
		t.Fatalf("same letter under opposite swaps is a DISAGREEMENT: %+v", res.Items[0])
	}
	if c := res.Counts(); c.Disagreed != 1 || c.Agreed != 0 || c.CandWins != 0 || c.BaseWins != 0 {
		t.Errorf("disagreements must not feed wins/losses: %+v", c)
	}
}

func TestCompare_InconsistentStoredWinnerIsUnusable(t *testing.T) {
	bad := v("q1", "j1", "j1", "A", false)
	bad.Winner = cand // contradicts preference A with A=S0
	res, _ := Compare([]Verdict{bad}, []Verdict{v("q1", "j2", "j2", "A", false)}, "a", "b", Options{Baseline: base, Candidate: cand})
	if it := res.Items[0]; it.Status != Unusable || !strings.Contains(it.Reason, "inconsistent") {
		t.Fatalf("%+v", it)
	}
}

func TestCompare_UnknownLabelAndUnpairedAndEqual(t *testing.T) {
	odd := v("q1", "j1", "j1", "A", false)
	odd.Preference, odd.Winner = "", "something-else" // legacy row with a label outside the set
	rows1 := []Verdict{odd, v("q2", "j1", "j1", "Equal", false), v("q3", "j1", "j1", "A", false)}
	rows2 := []Verdict{v("q1", "j2", "j2", "A", false), v("q2", "j2", "j2", "Equal", false)}
	res, _ := Compare(rows1, rows2, "a", "b", Options{Baseline: base, Candidate: cand})
	got := map[string]Status{}
	for _, it := range res.Items {
		got[it.Pair.Name] = it.Status
	}
	if got["q1"] != Unusable || got["q2"] != Agreed || got["q3"] != Unpaired {
		t.Errorf("statuses = %v", got)
	}
	if c := res.Counts(); c.AgreedEqual != 1 {
		t.Errorf("agreed Equal must be counted as a tie: %+v", c)
	}
}

func TestCompare_RefusesOneJudgeTwiceOrSameServedModel(t *testing.T) {
	a := []Verdict{v("q1", "j1", "served-x", "A", false)}
	if _, err := Compare(a, []Verdict{v("q1", "j1", "served-y", "A", false)}, "a", "b", Options{Baseline: base, Candidate: cand}); err == nil {
		t.Error("same JUDGE_MODEL twice must be refused")
	}
	if _, err := Compare(a, []Verdict{v("q1", "j2", "served-x", "A", false)}, "a", "b", Options{Baseline: base, Candidate: cand}); err == nil {
		t.Error("two requests served by the same model must be refused")
	}
	if _, err := Compare(a, a, "a", "b", Options{Baseline: base, Candidate: base}); err == nil {
		t.Error("equal labels must be refused")
	}
}

func TestCompare_ExcludeDropsPairsInvolvingUnscoreableRuns(t *testing.T) {
	a := []Verdict{v("q1", "j1", "j1", "A", false), v("q2", "j1", "j1", "A", false)}
	b := []Verdict{v("q1", "j2", "j2", "A", false), v("q2", "j2", "j2", "A", false)}
	res, _ := Compare(a, b, "a", "b", Options{Baseline: base, Candidate: cand, Exclude: func(_, _, name string) string {
		if name == "q2" {
			return "S2 run failed integrity"
		}
		return ""
	}})
	if c := res.Counts(); c.Agreed != 1 || c.Excluded != 1 {
		t.Errorf("counts = %+v", c)
	}
}

func TestPositionBiasRate(t *testing.T) {
	var rows []Verdict
	for i, p := range []string{"A", "A", "A", "B", "Equal"} {
		rows = append(rows, v(string(rune('a'+i)), "j1", "j1", p, false))
	}
	m := analyseFile("x", rows)
	if math.Abs(m.FirstPresentedRate-0.75) > 1e-9 {
		t.Errorf("rate = %v, want 0.75 (3 of 4 decisive verdicts went to the preset shown first)", m.FirstPresentedRate)
	}
	if m := analyseFile("x", []Verdict{{Name: "q", Winner: "S0"}}); m.FirstPresentedRate != -1 || len(m.Warnings) == 0 {
		t.Errorf("legacy file: %+v", m)
	}
}

func TestScenarioRep(t *testing.T) {
	if s, r, ok := ScenarioRep("eval/x/S2/rep3"); !ok || s != "S2" || r != "rep3" {
		t.Errorf("%s %s %v", s, r, ok)
	}
	if _, _, ok := ScenarioRep("baseline"); ok {
		t.Error("legacy dir has no rep")
	}
}

func TestExcludeUnscoreable(t *testing.T) {
	dir := t.TempDir()
	r := evaltestutil.Routing("gemini-3.1-pro-preview", "gemini-3.5-flash", "gemini-3.5-flash")
	pins := map[string]string{"gemini-3.5-flash": "gemini-3.6-flash"}
	ms := []evalcheck.ManifestScenario{{ID: "S0", AgentModels: r, Pins: pins}, {ID: "S2", AgentModels: r, Pins: pins}}
	mk := func(id string, rep int, ok bool) evaltestutil.Run {
		calls := evaltestutil.Calls(r, pins, 1, 1, 0, 1)
		if !ok {
			calls[0].ServedModel = "gemini-2.5-pro"
		}
		return evaltestutil.Run{Scenario: id, Query: "q1", Rep: rep, Success: true, IntegrityOK: true, Calls: calls, Output: evaltestutil.CleanOutput()}
	}
	evaltestutil.WriteBatch(t, dir, evaltestutil.Manifest("B", false, 2, []string{"q1"}, ms),
		[]evaltestutil.Run{mk("S0", 1, true), mk("S2", 1, true), mk("S0", 2, true), mk("S2", 2, false)})
	b, err := evalcheck.LoadBatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	ex := ExcludeUnscoreable(b)
	if why := ex("x/S0/rep1", "x/S2/rep1", "q1"); why != "" {
		t.Errorf("clean pair excluded: %s", why)
	}
	if why := ex("x/S0/rep2", "x/S2/rep2", "q1"); !strings.Contains(why, "S2/rep2/q1") {
		t.Errorf("a pair containing a fallback-served run must be excluded, got %q", why)
	}
	if why := ex("x/S0/rep1", "x/S2/rep1", "nope"); !strings.Contains(why, "not in the batch") {
		t.Errorf("unknown query: %q", why)
	}
	if why := ex("baseline", "pro-tier", "q1"); why != "" {
		t.Errorf("legacy dirs cannot be mapped and must not be excluded: %q", why)
	}
}

func TestSummarize_FlagsContestantJudgesAndReviewState(t *testing.T) {
	a := []Verdict{v("q1", "gemini-3.1-pro-preview", "gemini-3.1-pro-preview", "A", false)}
	b := []Verdict{v("q1", "gemini-4-argon", "gemini-4-argon", "A", false)}
	res, err := Compare(a, b, "a", "b", Options{Baseline: base, Candidate: cand})
	if err != nil {
		t.Fatal(err)
	}
	plan := Sample(NewIndex(), res.Items, 0.2, 1)
	rv, _ := ApplyReview(res.Items, plan)
	contestants := func(string) map[string]bool { return map[string]bool{"gemini-3.1-pro-preview": true} }
	s := Summarize(base, []*Result{res}, plan, rv, contestants)
	c, ok := s.Candidate(cand)
	if !ok || len(c.ContestantJudges) != 1 || c.ContestantJudges[0] != "gemini-3.1-pro-preview" {
		t.Errorf("contestants = %+v", c.ContestantJudges)
	}
	if s.ReviewComplete || s.Pending != len(plan.Entries) {
		t.Errorf("unreviewed sample must keep the summary provisional: %+v", s)
	}
	yes := false
	for i := range plan.Entries {
		plan.Entries[i].Discard = &yes
	}
	rv, _ = ApplyReview(res.Items, plan)
	if s := Summarize(base, []*Result{res}, plan, rv, nil); !s.ReviewComplete {
		t.Errorf("fully reviewed: %+v", s)
	}
}
