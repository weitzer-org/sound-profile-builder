package evalscore

import (
	"math"
	"math/rand"
	"sort"
)

// Mean returns the arithmetic mean (0 for empty input).
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// BootstrapMeanCI returns a two-sided percentile bootstrap confidence interval for the
// mean of xs. Deterministic for a given seed. conf is e.g. 0.95.
func BootstrapMeanCI(xs []float64, iters int, conf float64, seed int64) (lo, hi float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	rng := rand.New(rand.NewSource(seed))
	means := make([]float64, iters)
	for i := range means {
		s := 0.0
		for range xs {
			s += xs[rng.Intn(len(xs))]
		}
		means[i] = s / float64(len(xs))
	}
	sort.Float64s(means)
	a := (1 - conf) / 2
	return means[int(a*float64(iters))], means[int(math.Min(float64(iters-1), (1-a)*float64(iters)))]
}

// SignTestP is the exact two-sided sign-test p-value for pos wins vs neg wins (ties excluded).
func SignTestP(pos, neg int) float64 {
	n := pos + neg
	if n == 0 {
		return 1
	}
	k := pos
	if neg > k {
		k = neg
	}
	// P(X >= k) for X ~ Binomial(n, 0.5), doubled.
	tail := 0.0
	for i := k; i <= n; i++ {
		tail += math.Exp(logChoose(n, i) - float64(n)*math.Ln2)
	}
	return math.Min(1, 2*tail)
}

func logChoose(n, k int) float64 {
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}

// Verdict is the pre-registered non-inferiority decision for "new is not worse than old".
type Verdict string

const (
	NotWorse     Verdict = "NOT_WORSE"    // new - old CI lower bound >= -margin
	Worse        Verdict = "WORSE"        // new - old CI upper bound < 0
	Inconclusive Verdict = "INCONCLUSIVE" // neither
)

// NonInferiority decides from per-unit paired differences (new - old) of a higher-is-better
// score. margin is the largest drop considered acceptable (e.g. 0.05 = 5 points).
func NonInferiority(diffs []float64, margin float64, seed int64) (Verdict, float64, float64, float64) {
	lo, hi := BootstrapMeanCI(diffs, 10000, 0.95, seed)
	m := Mean(diffs)
	switch {
	case hi < 0:
		return Worse, m, lo, hi
	case lo >= -margin:
		return NotWorse, m, lo, hi
	}
	return Inconclusive, m, lo, hi
}
