package evalscore

import (
	"fmt"
	"sort"
	"strings"
)

// LibrarianRun is one Librarian call in a Librarian-only A/B.
type LibrarianRun struct {
	Query    string   `json:"query"`
	Phase1   int      `json:"phase1_sample"`
	Order    string   `json:"order"` // "new" | "old"
	Rep      int      `json:"rep"`
	Err      string   `json:"err,omitempty"`
	ParseOK  bool     `json:"parse_ok"`
	Names    []string `json:"names,omitempty"`
	Score    *Result  `json:"score,omitempty"`
	In       int32    `json:"input_tokens"`
	Cached   int32    `json:"cached_tokens"`
	Out      int32    `json:"output_tokens"`
	Thinking int32    `json:"thinking_tokens"`
	CostUSD  float64  `json:"cost_usd"`
	Latency  int64    `json:"latency_ms"`
}

// OrderStats summarises one condition.
type OrderStats struct {
	Order                    string
	Runs, Errors, ParseFail  int
	HardFrac, SoftFrac       float64 // mean fraction of hard / soft rules passed
	AllHard                  float64 // fraction of runs passing every hard rule and G3
	OneAmp                   float64
	NotInCatalog, RangeRverb float64 // mean per run
	CachedShare              float64
	CostUSD                  float64
	SelfConsistency          float64 // mean pairwise Jaccard of chosen names within a unit
}

// LibrarianComparison is the full A/B readout.
type LibrarianComparison struct {
	New, Old OrderStats
	Units    int // (query, phase1 sample) units with usable runs in both orders

	// Paired per-unit differences (new - old), each over the unit's mean.
	AllHardVerdict                       Verdict
	AllHardDiff, AllHardLo, AllHardHi    float64
	HardFracVerdict                      Verdict
	HardFracDiff, HardFracLo, HardFracHi float64
	SoftFracDiff, SoftFracLo, SoftFracHi float64
	SignPos, SignNeg, SignTies           int
	SignP                                float64

	// Noise floor: the same statistic computed between two halves of the SAME condition.
	NoiseFloorAbsDiff float64
	Margin            float64
	PerQuery          []QueryRow
}

// QueryRow is a per-query readout so a drop can be traced to a query.
type QueryRow struct {
	Query                  string
	NewAllHard, OldAllHard float64
	NewSoft, OldSoft       float64
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}

// CompareLibrarian computes the pre-registered readout. margin is the non-inferiority
// margin on the hard-rule pass fraction (0.05 = new may be at most 5 points worse).
func CompareLibrarian(runs []LibrarianRun, margin float64) LibrarianComparison {
	type key struct {
		q string
		p int
	}
	byUnit := map[key]map[string][]LibrarianRun{}
	for _, r := range runs {
		k := key{r.Query, r.Phase1}
		if byUnit[k] == nil {
			byUnit[k] = map[string][]LibrarianRun{}
		}
		byUnit[k][r.Order] = append(byUnit[k][r.Order], r)
	}
	usable := func(rs []LibrarianRun) []LibrarianRun {
		var out []LibrarianRun
		for _, r := range rs {
			if r.Err == "" && r.ParseOK && r.Score != nil {
				out = append(out, r)
			}
		}
		return out
	}
	frac := func(rs []LibrarianRun, f func(LibrarianRun) float64) float64 {
		var xs []float64
		for _, r := range rs {
			xs = append(xs, f(r))
		}
		return Mean(xs)
	}
	hardFrac := func(r LibrarianRun) float64 {
		if r.Score.HardTotal == 0 {
			return 1
		}
		return float64(r.Score.HardPass) / float64(r.Score.HardTotal)
	}
	softFrac := func(r LibrarianRun) float64 {
		if r.Score.SoftTotal == 0 {
			return 1
		}
		return float64(r.Score.SoftPass) / float64(r.Score.SoftTotal)
	}
	allHard := func(r LibrarianRun) float64 {
		if r.Score.AllHardPass {
			return 1
		}
		return 0
	}

	var c LibrarianComparison
	c.Margin = margin
	var dAll, dHard, dSoft, noise []float64
	qAgg := map[string]*[4][]float64{}
	var keys []key
	for k := range byUnit {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].q != keys[j].q {
			return keys[i].q < keys[j].q
		}
		return keys[i].p < keys[j].p
	})
	for _, k := range keys {
		nu, ol := usable(byUnit[k]["new"]), usable(byUnit[k]["old"])
		if len(nu) == 0 || len(ol) == 0 {
			continue
		}
		c.Units++
		dAll = append(dAll, frac(nu, allHard)-frac(ol, allHard))
		dHard = append(dHard, frac(nu, hardFrac)-frac(ol, hardFrac))
		dSoft = append(dSoft, frac(nu, softFrac)-frac(ol, softFrac))
		switch d := frac(nu, allHard) - frac(ol, allHard); {
		case d > 0:
			c.SignPos++
		case d < 0:
			c.SignNeg++
		default:
			c.SignTies++
		}
		if qAgg[k.q] == nil {
			qAgg[k.q] = &[4][]float64{}
		}
		qAgg[k.q][0] = append(qAgg[k.q][0], frac(nu, allHard))
		qAgg[k.q][1] = append(qAgg[k.q][1], frac(ol, allHard))
		qAgg[k.q][2] = append(qAgg[k.q][2], frac(nu, softFrac))
		qAgg[k.q][3] = append(qAgg[k.q][3], frac(ol, softFrac))
		// Noise floor: even-rep vs odd-rep halves inside each condition.
		for _, rs := range [][]LibrarianRun{nu, ol} {
			var even, odd []LibrarianRun
			for _, r := range rs {
				if r.Rep%2 == 0 {
					even = append(even, r)
				} else {
					odd = append(odd, r)
				}
			}
			if len(even) > 0 && len(odd) > 0 {
				d := frac(even, hardFrac) - frac(odd, hardFrac)
				if d < 0 {
					d = -d
				}
				noise = append(noise, d)
			}
		}
	}
	c.AllHardVerdict, c.AllHardDiff, c.AllHardLo, c.AllHardHi = NonInferiority(dAll, margin, 1)
	c.HardFracVerdict, c.HardFracDiff, c.HardFracLo, c.HardFracHi = NonInferiority(dHard, margin, 2)
	c.SoftFracDiff = Mean(dSoft)
	c.SoftFracLo, c.SoftFracHi = BootstrapMeanCI(dSoft, 10000, 0.95, 3)
	c.SignP = SignTestP(c.SignPos, c.SignNeg)
	c.NoiseFloorAbsDiff = Mean(noise)

	for _, order := range []string{"new", "old"} {
		st := OrderStats{Order: order}
		var all []LibrarianRun
		for _, k := range keys {
			all = append(all, byUnit[k][order]...)
		}
		st.Runs = len(all)
		for _, r := range all {
			if r.Err != "" {
				st.Errors++
			} else if !r.ParseOK {
				st.ParseFail++
			}
			st.CostUSD += r.CostUSD
		}
		u := usable(all)
		if len(u) > 0 {
			st.HardFrac = frac(u, hardFrac)
			st.SoftFrac = frac(u, softFrac)
			st.AllHard = frac(u, allHard)
			st.OneAmp = frac(u, func(r LibrarianRun) float64 {
				if r.Score.OneAmp {
					return 1
				}
				return 0
			})
			st.NotInCatalog = frac(u, func(r LibrarianRun) float64 { return float64(r.Score.NotInCatalog) })
			st.RangeRverb = frac(u, func(r LibrarianRun) float64 { return float64(r.Score.RangeRverb) })
			var in, ca int64
			for _, r := range u {
				in += int64(r.In)
				ca += int64(r.Cached)
			}
			if in > 0 {
				st.CachedShare = float64(ca) / float64(in)
			}
			// Within-unit self-consistency of the chosen names.
			var js []float64
			for _, k := range keys {
				rs := usable(byUnit[k][order])
				for i := 0; i < len(rs); i++ {
					for j := i + 1; j < len(rs); j++ {
						js = append(js, jaccard(toSet(rs[i].Names), toSet(rs[j].Names)))
					}
				}
			}
			st.SelfConsistency = Mean(js)
		}
		if order == "new" {
			c.New = st
		} else {
			c.Old = st
		}
	}
	var qs []string
	for q := range qAgg {
		qs = append(qs, q)
	}
	sort.Strings(qs)
	for _, q := range qs {
		a := qAgg[q]
		c.PerQuery = append(c.PerQuery, QueryRow{Query: q, NewAllHard: Mean(a[0]), OldAllHard: Mean(a[1]), NewSoft: Mean(a[2]), OldSoft: Mean(a[3])})
	}
	return c
}

func toSet(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// Markdown renders the comparison as a report.
func (c LibrarianComparison) Markdown() string {
	var b strings.Builder
	p := func(f string, a ...interface{}) { fmt.Fprintf(&b, f, a...) }
	p("# Librarian-only A/B: new order (static catalog first) vs old order\n\n")
	p("Units (query x Phase 1 sample) with usable runs in both orders: **%d**\n\n", c.Units)
	p("| | new order | old order |\n|---|---|---|\n")
	p("| runs (errors / unparseable) | %d (%d / %d) | %d (%d / %d) |\n", c.New.Runs, c.New.Errors, c.New.ParseFail, c.Old.Runs, c.Old.Errors, c.Old.ParseFail)
	p("| runs passing every hard rule + single amp | %.1f%% | %.1f%% |\n", 100*c.New.AllHard, 100*c.Old.AllHard)
	p("| mean hard-rule pass fraction | %.3f | %.3f |\n", c.New.HardFrac, c.Old.HardFrac)
	p("| mean soft-rule pass fraction | %.3f | %.3f |\n", c.New.SoftFrac, c.Old.SoftFrac)
	p("| exactly one amp (G3) | %.1f%% | %.1f%% |\n", 100*c.New.OneAmp, 100*c.Old.OneAmp)
	p("| names not in catalog / run (G1) | %.2f | %.2f |\n", c.New.NotInCatalog, c.Old.NotInCatalog)
	p("| Range Rverb uses / run (G2) | %.2f | %.2f |\n", c.New.RangeRverb, c.Old.RangeRverb)
	p("| self-consistency (mean Jaccard of picks between repeats) | %.3f | %.3f |\n", c.New.SelfConsistency, c.Old.SelfConsistency)
	p("| cached input share (NOT representative of production, see note) | %.0f%% | %.0f%% |\n", 100*c.New.CachedShare, 100*c.Old.CachedShare)
	p("| cost (est., no cache discount applied where rate unverified) | $%.2f | $%.2f |\n\n", c.New.CostUSD, c.Old.CostUSD)
	p("Note: repeats within a unit share identical Phase 1 text, so the old order gets prefix-cache hits here that it would not get in production (where per-run text differs). Use the full-pipeline run for realistic cache numbers.\n\n")
	p("## Pre-registered decision (non-inferiority, margin %.0f points)\n\n", 100*c.Margin)
	p("Paired per-unit difference, new - old, 95%% bootstrap CI:\n\n")
	p("| metric | mean diff | 95%% CI | verdict |\n|---|---|---|---|\n")
	p("| runs passing every hard rule | %+.3f | [%+.3f, %+.3f] | **%s** |\n", c.AllHardDiff, c.AllHardLo, c.AllHardHi, c.AllHardVerdict)
	p("| hard-rule pass fraction | %+.3f | [%+.3f, %+.3f] | **%s** |\n", c.HardFracDiff, c.HardFracLo, c.HardFracHi, c.HardFracVerdict)
	p("| soft-rule pass fraction | %+.3f | [%+.3f, %+.3f] | (reported only) |\n\n", c.SoftFracDiff, c.SoftFracLo, c.SoftFracHi)
	p("Sign test on units (all-hard-pass rate): new better %d, old better %d, ties %d, two-sided p = %.3f\n\n", c.SignPos, c.SignNeg, c.SignTies, c.SignP)
	p("Noise floor (mean |difference| in hard-pass fraction between two halves of the SAME condition): %.3f. A new-vs-old difference smaller than this is indistinguishable from run-to-run noise.\n\n", c.NoiseFloorAbsDiff)
	p("## Per query (fraction of runs passing all hard rules / mean soft fraction)\n\n| query | new hard | old hard | new soft | old soft |\n|---|---|---|---|---|\n")
	for _, r := range c.PerQuery {
		p("| %s | %.2f | %.2f | %.2f | %.2f |\n", r.Query, r.NewAllHard, r.OldAllHard, r.NewSoft, r.OldSoft)
	}
	return b.String()
}
