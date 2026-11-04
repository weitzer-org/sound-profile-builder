package evalscore

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeJudge prefers the preset whose text contains "GOOD"; ties -> Equal.
func fakeJudge(ctx context.Context, q, a, b string) (string, float64, string, error) {
	ag, bg := strings.Contains(a, "GOOD"), strings.Contains(b, "GOOD")
	switch {
	case ag && !bg:
		return "A", 1, "", nil
	case bg && !ag:
		return "B", 1, "", nil
	}
	return "Equal", 1, "", nil
}

func TestJudgeBothWays_ConsistentWinnerAndPositionBias(t *testing.T) {
	pv := JudgeBothWays(context.Background(), fakeJudge, "q", 0, "GOOD", "bad")
	if pv.Winner != "first" {
		t.Errorf("want first, got %+v", pv)
	}
	pv = JudgeBothWays(context.Background(), fakeJudge, "q", 0, "bad", "GOOD")
	if pv.Winner != "second" {
		t.Errorf("want second, got %+v", pv)
	}
	// A judge that always says "A" has pure position bias: it must read as inconsistent, not as a winner.
	alwaysA := func(context.Context, string, string, string) (string, float64, string, error) { return "A", 1, "", nil }
	if pv := JudgeBothWays(context.Background(), alwaysA, "q", 0, "x", "y"); pv.Winner != "inconsistent" {
		t.Errorf("position-biased judge must be inconsistent: %+v", pv)
	}
}

func TestJudgeBothWays_EqualVsWinnerIsInconsistent(t *testing.T) {
	calls := 0
	j := func(context.Context, string, string, string) (string, float64, string, error) {
		calls++
		if calls == 1 {
			return "A", 1, "", nil // first-as-A: first wins
		}
		return "Equal", 1, "", nil
	}
	if pv := JudgeBothWays(context.Background(), j, "q", 0, "x", "y"); pv.Winner != "inconsistent" {
		t.Errorf("a win in one order and 'Equal' in the other is not a preference: %+v", pv)
	}
}

func TestJudgeBothWays_ErrorsAreNeverWinners(t *testing.T) {
	bad := func(context.Context, string, string, string) (string, float64, string, error) {
		return "", 0, "", errors.New("network")
	}
	pv := JudgeBothWays(context.Background(), bad, "q", 0, "x", "y")
	if pv.Winner != "error" || pv.Err == "" {
		t.Errorf("%+v", pv)
	}
	weird := func(context.Context, string, string, string) (string, float64, string, error) { return "C", 1, "", nil }
	if pv := JudgeBothWays(context.Background(), weird, "q", 0, "x", "y"); pv.Winner != "error" {
		t.Errorf("unrecognised verdict must be an error: %+v", pv)
	}
}

func TestSummarise(t *testing.T) {
	pvs := []PairVerdict{
		{Winner: "first", Verdict: [2]string{"A", "B"}},
		{Winner: "first", Verdict: [2]string{"A", "B"}},
		{Winner: "second", Verdict: [2]string{"B", "A"}},
		{Winner: "inconsistent", Verdict: [2]string{"A", "A"}},
		{Winner: "error"},
	}
	s := Summarise(pvs)
	if s.First != 2 || s.Second != 1 || s.Inconsistent != 1 || s.Errors != 1 || s.N != 5 {
		t.Errorf("%+v", s)
	}
	if s.InconsistentRate != 0.25 {
		t.Errorf("inconsistent rate over non-error pairs: %v", s.InconsistentRate)
	}
	// 4 judged pairs x 2 raw verdicts: A,B | A,B | B,A | A,A -> position A picked 5 of 8.
	if s.AShare != 5.0/8.0 {
		t.Errorf("AShare = %v, want 0.625", s.AShare)
	}
	if !strings.Contains(JudgeMarkdown("new", "old", pvs, pvs), "CALIBRATION") {
		t.Error("report must include the calibration row")
	}
}
