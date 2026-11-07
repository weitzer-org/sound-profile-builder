package evaldevicerefine

import (
	"strings"
	"testing"
)

func rs(arm, draft, req string, rep int, m Metrics) RunScore {
	return RunScore{Run: Run{Triple: Triple{Draft: draft, Request: req, Rep: rep}, Arm: arm}, Success: true, IntegrityOK: true, Metrics: m}
}

func ok(ops, na, kept, draft int) Metrics {
	return Metrics{Parsed: true, PlanOK: true, Ops: ops, NotApplied: na, Kept: kept, DraftBlocks: draft, OutBlocks: draft, MechChecked: true}
}

func TestPairsKeepOnlyTriplesUsableInBothArms(t *testing.T) {
	bad := rs("B", "d1", "tone", 2, Metrics{Parsed: false})
	runs := []RunScore{
		rs("A", "d1", "tone", 1, ok(1, 0, 3, 3)), rs("B", "d1", "tone", 1, ok(1, 0, 3, 3)),
		rs("A", "d1", "tone", 2, ok(1, 0, 3, 3)), bad, // B did not parse
		rs("A", "d1", "scene", 1, ok(1, 0, 3, 3)), // B never ran
	}
	failed := rs("A", "d2", "tone", 1, ok(1, 0, 3, 3))
	failed.IntegrityOK = false
	runs = append(runs, failed, rs("B", "d2", "tone", 1, ok(1, 0, 3, 3)))

	pairs, dropped := Pairs(runs)
	if len(pairs) != 1 || pairs[0].Draft != "d1" || pairs[0].Request != "tone" || pairs[0].Rep != 1 {
		t.Fatalf("pairs = %+v", pairs)
	}
	if len(dropped) != 3 {
		t.Fatalf("dropped = %+v, want d1/tone#2, d1/scene#1, d2/tone#1", dropped)
	}
}

func TestPooledRatiosAndCriteria(t *testing.T) {
	a1, a2 := ok(1, 3, 3, 3), ok(9, 0, 3, 3) // pooled 10/13, not mean(25%, 100%)
	s := Summarize([]RunScore{rs("A", "d", "r", 1, a1), rs("A", "d", "r", 2, a2)})
	if sh, _ := s.ApplicableShare(); sh < 0.769 || sh > 0.770 {
		t.Errorf("pooled share = %v, want 10/13", sh)
	}
	if _, has := Summarize(nil).ApplicableShare(); has {
		t.Error("no changes at all must be n/a, not 0 or 1")
	}

	// B wins on every row.
	a := ok(5, 5, 9, 10)
	a.Unasked, a.Locked, a.Defects = 2, 0, 3
	b := ok(9, 1, 10, 10)
	b.Defects = 2
	pa, pb := rs("A", "d", "r", 1, a), rs("B", "d", "r", 1, b)
	pa.Judge, pb.Judge = &JudgeScore{Addressed: 1}, &JudgeScore{Addressed: 2}
	cs := Criteria([]Pair{{Triple: pa.Triple, A: pa, B: pb}})
	if !AllPass(cs) {
		t.Errorf("expected all pass: %+v", cs)
	}

	// A locked violation in B fails that row only.
	b.Locked = 1
	pb = rs("B", "d", "r", 1, b)
	pb.Judge = &JudgeScore{Addressed: 2}
	cs = Criteria([]Pair{{Triple: pa.Triple, A: pa, B: pb}})
	if AllPass(cs) {
		t.Error("a locked violation must fail the verdict")
	}
	for _, c := range cs {
		if c.Metric == "Locked violations" == c.Pass {
			t.Errorf("only the locked row should fail: %+v", c)
		}
	}
	// Without judge scores the judge row cannot pass.
	pa.Judge, pb.Judge = nil, nil
	for _, c := range Criteria([]Pair{{Triple: pa.Triple, A: pa, B: pb}}) {
		if strings.HasPrefix(c.Metric, "Request addressed") && c.Pass {
			t.Error("judge row passed with no judge data")
		}
	}
}

func TestReportMentionsWhatFailedWhere(t *testing.T) {
	bad := rs("B", "d1", "tone", 1, Metrics{ParseError: "x"})
	var sb strings.Builder
	Report(&sb, []RunScore{rs("A", "d1", "tone", 1, ok(1, 0, 3, 3)), bad})
	out := sb.String()
	for _, want := range []string{"Output did not parse", "0 (draft, request, rep) triples usable in both arms", "d1/tone#1", "All section 8.1 rows pass for arm B: **false**"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
}

func TestMatrixAndJudgeOrder(t *testing.T) {
	m := Matrix([]string{"a", "b"}, RequestIDs(), Arms, 1)
	if len(m) != 16 {
		t.Fatalf("pilot matrix = %d runs, want 2 drafts x 4 requests x 2 arms x 1 rep = 16", len(m))
	}
	if got := len(Matrix([]string{"a", "b", "c", "d", "e", "f", "g", "h"}, RequestIDs(), Arms, 2)); got != 128 {
		t.Fatalf("full matrix = %d, want 128", got)
	}
	// Judge order: deterministic per triple, and not constant.
	seenA, seenB := false, false
	for rep := 1; rep <= 40; rep++ {
		tr := Triple{Draft: "d", Request: "tone", Rep: rep}
		if XIsA(tr) != XIsA(tr) {
			t.Fatal("not deterministic")
		}
		if XIsA(tr) {
			seenA = true
		} else {
			seenB = true
		}
	}
	if !seenA || !seenB {
		t.Error("the judge order never varies")
	}
}

func TestJudgeVerdictMapsBackToArms(t *testing.T) {
	v, err := ParseVerdict("```json\n{\"x\":{\"addressed\":2,\"tone_ok\":true},\"y\":{\"addressed\":0,\"tone_ok\":false},\"rationale\":\"r\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	a, b := JudgeRecord{XIsA: true, Verdict: v}.Scores()
	if a.Addressed != 2 || b.Addressed != 0 {
		t.Errorf("x is a: a=%+v b=%+v", a, b)
	}
	a, b = JudgeRecord{XIsA: false, Verdict: v}.Scores()
	if a.Addressed != 0 || b.Addressed != 2 {
		t.Errorf("x is b: a=%+v b=%+v", a, b)
	}
	if _, err := ParseVerdict(`{"x":{"addressed":3},"y":{"addressed":0}}`); err == nil {
		t.Error("score 3 must be refused")
	}
}

// GSR, PR 188: 0.85-0.80 is 0.04999999999999993 in float64, which is still the 5-point margin.
func TestApplicableShareMarginIsNotLostToFloatRounding(t *testing.T) {
	pair := func(opsA, naA, opsB, naB int) []Pair {
		a, b := rs("A", "d", "r", 1, ok(opsA, naA, 3, 3)), rs("B", "d", "r", 1, ok(opsB, naB, 3, 3))
		return []Pair{{Triple: a.Triple, A: a, B: b}}
	}
	share := func(cs []Criterion) bool {
		for _, c := range cs {
			if c.Metric == "Applicable share" {
				return c.Pass
			}
		}
		t.Fatal("no applicable share row")
		return false
	}
	// A 80% (80/100), B 85% (85/100): exactly 5 points above, and above the 80% target.
	if !share(Criteria(pair(80, 20, 85, 15))) {
		t.Error("an exact 5-point margin must pass")
	}
	// 4.9 points does not.
	if share(Criteria(pair(801, 199, 850, 150))) {
		t.Error("4.9 points must not pass")
	}
}
