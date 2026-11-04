package evalreport

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const eps = 1e-9

// Gate results for one scenario (plan §6.1). A scenario fails if any gate fails; it cannot pass
// with an UNMEASURED gate.
type Gates struct {
	Scenario string
	Checks   []Check
}

// Overall is FAIL if any gate FAILs, UNMEASURED if none fail but one is unmeasured, else PASS.
func (g Gates) Overall() Status {
	out := Pass
	for _, c := range g.Checks {
		switch c.Status {
		case Fail:
			return Fail
		case Unmeasured:
			out = Unmeasured
		}
	}
	return out
}

func rate(n, d int) float64 {
	if d == 0 {
		return math.NaN()
	}
	return float64(n) / float64(d)
}

func pct(x float64) string {
	if math.IsNaN(x) {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", 100*x)
}

// EvaluateGates applies §6.1 to s against base (the S0 of the same batch). For the baseline
// itself the relative gates are N/A and only the absolute ones are evaluated.
func (r *Report) EvaluateGates(s *ScenarioStats) Gates {
	base := r.Scenario(r.BaselineID)
	isBase := s.ID == r.BaselineID
	g := Gates{Scenario: s.ID}
	add := func(name string, st Status, detail string) { g.Checks = append(g.Checks, Check{name, st, detail}) }

	// 1. Served-model integrity: 100% of calls served by the requested model.
	switch {
	case r.Legacy:
		add("served-model integrity", Unmeasured, "legacy directory has no calls files")
	case s.CallsMissingRuns > 0:
		add("served-model integrity", Unmeasured, fmt.Sprintf("%d recorded runs have no calls file", s.CallsMissingRuns))
	case s.CallsViolating == 0:
		add("served-model integrity", Pass, fmt.Sprintf("%d/%d calls served by the requested model", s.CallsChecked, s.CallsChecked))
	default:
		first := s.Violations
		if len(first) > 3 {
			first = first[:3]
		}
		add("served-model integrity", Fail, fmt.Sprintf("%d of %d calls NOT served by the requested model; e.g. %s", s.CallsViolating, s.CallsChecked, strings.Join(first, " | ")))
	}

	// 2. Pipeline success rate >= S0's. A run that did not get a clean answer from the requested
	// model counts as a failure (principle 3), so the numerator is scoreable runs.
	switch {
	case isBase:
		add("pipeline success rate >= S0", NA, fmt.Sprintf("baseline: %d/%d = %s", s.Scoreable, s.Planned, pct(rate(s.Scoreable, s.Planned))))
	case r.Legacy:
		add("pipeline success rate >= S0", Unmeasured, "legacy: failed runs are indistinguishable from runs that were never made")
	case base == nil || s.Planned == 0 || base.Planned == 0:
		add("pipeline success rate >= S0", Unmeasured, "no planned runs to form a rate")
	default:
		sr, br := rate(s.Scoreable, s.Planned), rate(base.Scoreable, base.Planned)
		st := Pass
		if sr+eps < br {
			st = Fail
		}
		add("pipeline success rate >= S0", st, fmt.Sprintf("%s (%d/%d scoreable) vs S0 %s (%d/%d)", pct(sr), s.Scoreable, s.Planned, pct(br), base.Scoreable, base.Planned))
	}

	// 3. Schema validity: every Architect output parses; capture_db_on_modeled and
	// modeled_style_on_capture are both 0.
	switch {
	case s.Outputs == 0:
		add("schema validity", Unmeasured, "no scoreable output")
	case s.ParseErrors == 0 && s.R2 == 0 && s.ModeledStyleOnCapture == 0:
		add("schema validity", Pass, fmt.Sprintf("%d/%d outputs parse; capture_db_on_modeled=0; modeled_style_on_capture=0", s.Outputs, s.Outputs))
	default:
		add("schema validity", Fail, fmt.Sprintf("%d/%d outputs parse; capture_db_on_modeled=%d; modeled_style_on_capture=%d", s.Outputs-s.ParseErrors, s.Outputs, s.R2, s.ModeledStyleOnCapture))
	}

	// 4. Truncation <= S0, per agent (rate per recorded run, so unequal run counts stay fair).
	switch {
	case r.Legacy || s.TruncRuns == 0:
		add("MAX_TOKENS <= S0 per agent", Unmeasured, "needs calls files for this scenario and S0")
	case isBase:
		add("MAX_TOKENS <= S0 per agent", NA, "baseline: "+truncText(s))
	case r.Legacy || s.TruncRuns == 0 || base == nil || base.TruncRuns == 0:
		add("MAX_TOKENS <= S0 per agent", Unmeasured, "needs calls files for this scenario and S0")
	case s.TruncRuns < s.Recorded || base.TruncRuns < base.Recorded:
		add("MAX_TOKENS <= S0 per agent", Unmeasured, "some recorded runs lack a calls file")
	default:
		var worse []string
		roles := map[string]bool{}
		for a := range s.Trunc {
			roles[a] = true
		}
		for a := range base.Trunc {
			roles[a] = true
		}
		var names []string
		for a := range roles {
			names = append(names, a)
		}
		sort.Strings(names)
		for _, a := range names {
			sr, br := float64(s.Trunc[a])/float64(s.TruncRuns), float64(base.Trunc[a])/float64(base.TruncRuns)
			if sr > br+eps {
				worse = append(worse, fmt.Sprintf("%s %d/%d runs vs S0 %d/%d", a, s.Trunc[a], s.TruncRuns, base.Trunc[a], base.TruncRuns))
			}
		}
		if len(worse) == 0 {
			add("MAX_TOKENS <= S0 per agent", Pass, "candidate "+truncText(s)+"; S0 "+truncText(base))
		} else {
			add("MAX_TOKENS <= S0 per agent", Fail, strings.Join(worse, "; "))
		}
	}

	// 5. R1 capture-mislabel: 0 occurrences across all (scoreable) runs.
	switch {
	case s.Outputs-s.ParseErrors == 0:
		add("R1 capture_claim_on_modeled = 0", Unmeasured, "no parseable output")
	case s.R1 == 0:
		add("R1 capture_claim_on_modeled = 0", Pass, fmt.Sprintf("0 across %d outputs", s.Outputs))
	default:
		add("R1 capture_claim_on_modeled = 0", Fail, fmt.Sprintf("%d across %d outputs", s.R1, s.Outputs))
	}

	// 6. Invented block names <= S0 (snapped + unverified, mean per scoreable output).
	inv := s.Snapped + s.Unverified
	switch {
	case isBase:
		add("invented block names <= S0", NA, fmt.Sprintf("baseline: snapped=%d unverified=%d over %d outputs", s.Snapped, s.Unverified, s.Outputs))
	case s.Outputs == 0 || base == nil || base.Outputs == 0:
		add("invented block names <= S0", Unmeasured, "no scoreable output")
	default:
		sm, bm := float64(inv)/float64(s.Outputs), float64(base.Snapped+base.Unverified)/float64(base.Outputs)
		st := Pass
		if sm > bm+eps {
			st = Fail
		}
		add("invented block names <= S0", st, fmt.Sprintf("%.2f per output (snapped %d + unverified %d over %d) vs S0 %.2f (snapped %d + unverified %d over %d)", sm, s.Snapped, s.Unverified, s.Outputs, bm, base.Snapped, base.Unverified, base.Outputs))
	}
	return g
}

func truncText(s *ScenarioStats) string {
	if len(s.Trunc) == 0 {
		return fmt.Sprintf("none in %d runs", s.TruncRuns)
	}
	var parts []string
	for a, n := range s.Trunc {
		parts = append(parts, fmt.Sprintf("%s %d", a, n))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ") + fmt.Sprintf(" (of %d runs)", s.TruncRuns)
}
