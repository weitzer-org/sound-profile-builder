package evalscore

import (
	"math"
	"testing"
)

func TestSignTestP(t *testing.T) {
	// Values checked against exact binomial tails: 10/12 -> 0.0386; 4/6 -> 0.6875; 6/6 -> 0.03125.
	cases := []struct {
		pos, neg int
		want     float64
	}{{10, 2, 0.03857421875}, {4, 2, 0.6875}, {6, 0, 0.03125}, {0, 0, 1}, {5, 5, 1}}
	for _, c := range cases {
		if got := SignTestP(c.pos, c.neg); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("SignTestP(%d,%d) = %v, want %v", c.pos, c.neg, got, c.want)
		}
	}
}

func TestBootstrapCI_DeterministicAndBrackets(t *testing.T) {
	xs := []float64{0, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	a1, b1 := BootstrapMeanCI(xs, 2000, 0.95, 7)
	a2, b2 := BootstrapMeanCI(xs, 2000, 0.95, 7)
	if a1 != a2 || b1 != b2 {
		t.Error("bootstrap must be deterministic for a fixed seed")
	}
	if !(a1 <= 0.1 && 0.1 <= b1) {
		t.Errorf("CI [%v,%v] should bracket the mean 0.1", a1, b1)
	}
}

func TestNonInferiority(t *testing.T) {
	same := make([]float64, 30) // all zero diffs
	if v, _, _, _ := NonInferiority(same, 0.05, 1); v != NotWorse {
		t.Errorf("identical scores should be NOT_WORSE, got %s", v)
	}
	worse := make([]float64, 30)
	for i := range worse {
		worse[i] = -0.3
	}
	if v, _, _, _ := NonInferiority(worse, 0.05, 1); v != Worse {
		t.Errorf("consistently lower should be WORSE, got %s", v)
	}
	noisy := []float64{-1, 1, -1, 1, 0, 0}
	if v, _, _, _ := NonInferiority(noisy, 0.05, 1); v != Inconclusive {
		t.Errorf("noisy small sample should be INCONCLUSIVE, got %s", v)
	}
}
