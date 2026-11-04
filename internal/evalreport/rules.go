package evalreport

import (
	"fmt"
	"math"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/evalcost"
)

// Verdict of a §7 rule.
type Verdict string

const (
	Adopt        Verdict = "ADOPT"
	NotAdopt     Verdict = "DO NOT ADOPT"
	Undetermined Verdict = "UNDETERMINED"
	Provisional  Verdict = "ADOPT (PROVISIONAL)"
	Manual       Verdict = "MANUAL"
)

// RuleResult is one tier rule applied to one scenario.
type RuleResult struct {
	Scenario string
	Tier     string
	Verdict  Verdict
	Lines    []Check // the conditions and the numbers behind them
	Note     string
}

// Quality is the §6.2 comparison of a scenario with the baseline.
type Quality struct {
	DefectMean, BaseDefectMean float64
	DefectsStatus              Status
	DefectDetail               string
	JudgeStatus                Status
	JudgeDetail                string
	Wins, Losses               int
	JudgeProvisional           bool
}

// EvaluateQuality computes the two quality conditions the §7 rules use.
func (r *Report) EvaluateQuality(s *ScenarioStats) Quality {
	q := Quality{}
	base := r.Scenario(r.BaselineID)
	switch {
	case s.DefectSource != "re-critic" || base == nil || base.DefectSource != "re-critic":
		q.DefectsStatus, q.DefectDetail = Unmeasured, "no recritic.jsonl: run cmd/eval_recritic (the pipeline's own Critic is not comparable across scenarios)"
	case s.DefectMissing > 0 || base.DefectMissing > 0 || len(s.Defects) == 0 || len(base.Defects) == 0:
		q.DefectsStatus = Unmeasured
		q.DefectDetail = fmt.Sprintf("re-critic incomplete: %d scoreable runs without a result in %s, %d in %s", s.DefectMissing, s.ID, base.DefectMissing, base.ID)
	default:
		q.DefectMean, _, _ = s.DefectSpread()
		q.BaseDefectMean, _, _ = base.DefectSpread()
		q.DefectsStatus = Pass
		if q.DefectMean > q.BaseDefectMean+eps {
			q.DefectsStatus = Fail
		}
		q.DefectDetail = fmt.Sprintf("re-critic defects per run %.2f vs S0 %.2f (lower is better)", q.DefectMean, q.BaseDefectMean)
	}
	switch {
	case r.Judge == nil:
		q.JudgeStatus, q.JudgeDetail = Unmeasured, "no judge_summary.json: run cmd/judge_compare twice per pair, then cmd/eval_judge_agree"
	default:
		c, ok := r.Judge.Candidate(s.ID)
		if !ok {
			q.JudgeStatus, q.JudgeDetail = Unmeasured, "judge summary has no entry for "+s.ID
			break
		}
		q.Wins, q.Losses = c.Counts.CandWins, c.Counts.BaseWins
		q.JudgeStatus = Pass
		if q.Wins < q.Losses {
			q.JudgeStatus = Fail
		}
		q.JudgeProvisional = !r.Judge.ReviewComplete
		q.JudgeDetail = fmt.Sprintf("agreed verdicts: %s wins %d, S0 wins %d, ties %d; disagreements %d (reported separately); discarded by spot-check %d", s.ID, q.Wins, q.Losses, c.Counts.AgreedEqual, c.Counts.Disagreed, c.Counts.Discarded)
		if q.JudgeProvisional {
			q.JudgeDetail += fmt.Sprintf(" -- PROVISIONAL: %d sampled verdicts still unreviewed", r.Judge.Pending)
		}
	}
	return q
}

// EvaluateRules applies the §7 table to a scenario, once per tier it changes.
func (r *Report) EvaluateRules(s *ScenarioStats, gates Gates) []RuleResult {
	if s.ID == r.BaselineID {
		return nil
	}
	if s.Skipped != "" {
		return nil
	}
	q := r.EvaluateQuality(s)
	var out []RuleResult
	for _, tier := range s.Tiers {
		out = append(out, r.applyRule(s, tier, gates, q))
	}
	// "Argon anywhere in production" caps every adopt verdict for scenarios that use a provisional model.
	if prov := r.provisionalModels(s); len(prov) > 0 {
		note := fmt.Sprintf("uses provisional model(s) %s. §7: until Google publishes pricing and a non-provisional id this is a PREVIEW SIGNAL and does not change routing.", strings.Join(prov, ", "))
		for i := range out {
			if out[i].Verdict == Adopt || out[i].Verdict == Provisional {
				out[i].Verdict = Undetermined
				out[i].Note = "CAPPED: " + note
			} else if out[i].Note == "" {
				out[i].Note = "Note: " + note
			} else {
				out[i].Note += " Note: " + note
			}
		}
		if len(out) == 0 {
			out = append(out, RuleResult{Scenario: s.ID, Tier: "provisional-model", Verdict: Undetermined, Note: "Note: " + note})
		}
	}
	return out
}

func (r *Report) provisionalModels(s *ScenarioStats) []string {
	seen := map[string]bool{}
	var out []string
	b := r.manifestModels(s.ID)
	for _, m := range b {
		if r.Options.Provisional[m] && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

func (r *Report) applyRule(s *ScenarioStats, tier string, gates Gates, q Quality) RuleResult {
	res := RuleResult{Scenario: s.ID, Tier: tier}
	add := func(name string, st Status, detail string) { res.Lines = append(res.Lines, Check{name, st, detail}) }

	if tier == "critic" {
		res.Verdict = Manual
		res.Note = "§7 Critic rule is not mechanizable: it asks whether the candidate Critic's findings are MORE ACCURATE. A flag count alone does not count. Sample 30 of its flags (eval_report -critic-sample 30), mark each a true or false contradiction against coros_map.json/user_captures.json, and compare with S0's sample."
		return res
	}

	g := gates.Overall()
	add("passes every §6.1 gate", g, "see gate table")
	add("critic defect count <= S0 (re-critic)", q.DefectsStatus, q.DefectDetail)
	add("agreed judge verdicts: wins >= losses", q.JudgeStatus, q.JudgeDetail)

	switch tier {
	case "pro":
		// all three conditions
	case "flash", "mechanical":
		// "quality is not worse (tie is acceptable)" == defect <= S0 AND judge wins >= losses.
		if tier == "flash" {
			base := r.Scenario(r.BaselineID)
			cheaper, faster := r.cheaperAndFaster(s, base)
			add("cheaper than S0 ($/run, priced tokens)", cheaper.st, cheaper.detail)
			add("faster than S0 (p50 pipeline latency)", faster.st, faster.detail)
			switch r.Options.BaselineFlashDeprecated {
			case Yes:
				add("S0's Flash model is deprecated (later deprecation date)", Pass, "supplied via -baseline-flash-deprecated=yes; NOT verified here (plan §10)")
			case No:
				add("S0's Flash model is deprecated (later deprecation date)", Fail, "supplied via -baseline-flash-deprecated=no")
			default:
				add("S0's Flash model is deprecated (later deprecation date)", Unmeasured, "external fact, unverified (plan §10); pass -baseline-flash-deprecated=yes|no once confirmed on Google's deprecations page")
			}
		}
	}
	res.Verdict = decide(res.Lines, tier)
	if res.Verdict == Adopt && q.JudgeProvisional {
		res.Verdict = Provisional
		res.Note = "judge verdicts are provisional until the spot-check review is complete"
	}
	return res
}

// decide: pro/mechanical need every line PASS; flash needs the first three PASS plus at least one
// of {cheaper, faster, deprecated} PASS. Any FAIL in a required line is DO NOT ADOPT; a required
// line that is UNMEASURED makes the verdict UNDETERMINED.
func decide(lines []Check, tier string) Verdict {
	required := lines
	var alt []Check
	if tier == "flash" && len(lines) > 3 {
		required, alt = lines[:3], lines[3:]
	}
	undetermined := false
	for _, l := range required {
		switch l.Status {
		case Fail:
			return NotAdopt
		case Unmeasured:
			undetermined = true
		}
	}
	if len(alt) > 0 {
		anyPass, anyUnmeasured := false, false
		for _, l := range alt {
			switch l.Status {
			case Pass:
				anyPass = true
			case Unmeasured:
				anyUnmeasured = true
			}
		}
		switch {
		case anyPass:
		case anyUnmeasured:
			undetermined = true
		default:
			return NotAdopt // not cheaper, not faster, not deprecated: no reason to switch
		}
	}
	if undetermined {
		return Undetermined
	}
	return Adopt
}

type cmp struct {
	st     Status
	detail string
}

func (r *Report) cheaperAndFaster(s, base *ScenarioStats) (cheaper, faster cmp) {
	if base == nil {
		return cmp{Unmeasured, "no baseline"}, cmp{Unmeasured, "no baseline"}
	}
	sc, bc := meanCost(s.Cost), meanCost(base.Cost)
	switch {
	case len(s.Cost) == 0 || len(base.Cost) == 0:
		cheaper = cmp{Unmeasured, "no token records"}
	case sc.Status() != "KNOWN" || bc.Status() != "KNOWN":
		cheaper = cmp{Unmeasured, fmt.Sprintf("cost is %s for %s and %s for S0 (an unpriced model is UNKNOWN, never $0): not comparable", costWord(sc), s.ID, costWord(bc))}
	case sc.KnownUSD < bc.KnownUSD-eps:
		cheaper = cmp{Pass, fmt.Sprintf("$%.4f vs S0 $%.4f per run", sc.KnownUSD, bc.KnownUSD)}
	default:
		cheaper = cmp{Fail, fmt.Sprintf("$%.4f vs S0 $%.4f per run", sc.KnownUSD, bc.KnownUSD)}
	}
	sp, bp := Percentile(s.Latency, 50), Percentile(base.Latency, 50)
	switch {
	case math.IsNaN(sp) || math.IsNaN(bp):
		faster = cmp{Unmeasured, "no latency data"}
	case sp < bp-eps:
		faster = cmp{Pass, fmt.Sprintf("p50 %.0fs vs S0 %.0fs", sp, bp)}
	default:
		faster = cmp{Fail, fmt.Sprintf("p50 %.0fs vs S0 %.0fs", sp, bp)}
	}
	return
}

func costWord(c evalcost.RunCost) string { return strings.ToLower(c.Status()) }

// meanCost averages per-run costs (known part and unpriced volumes).
func meanCost(cs []evalcost.RunCost) evalcost.RunCost {
	out := evalcost.RunCost{Unpriced: map[string]evalcost.ModelTokens{}}
	if len(cs) == 0 {
		return out
	}
	for _, c := range cs {
		out.KnownUSD += c.KnownUSD
		out.Calls += c.Calls
		for k, v := range c.Unpriced {
			m := out.Unpriced[k]
			m.Calls += v.Calls
			out.Unpriced[k] = m
		}
	}
	out.KnownUSD /= float64(len(cs))
	return out
}

// Attribution implements the §7 A1/A2/A3 reading. "Wins" is an interpretation (see the plan
// doc): all gates pass, defects <= S0, and agreed judge wins STRICTLY exceed losses.
func (r *Report) Attribution(gates map[string]Gates) []string {
	a1, a2, a3 := r.Scenario("A1"), r.Scenario("A2"), r.Scenario("A3")
	if a1 == nil || a2 == nil || a3 == nil || a1.Skipped != "" {
		return nil
	}
	wins := func(s *ScenarioStats) (bool, string) {
		g := gates[s.ID].Overall()
		q := r.EvaluateQuality(s)
		if g == Unmeasured || q.DefectsStatus == Unmeasured || q.JudgeStatus == Unmeasured {
			return false, "UNDETERMINED (a gate or quality input is unmeasured)"
		}
		ok := g == Pass && q.DefectsStatus == Pass && q.Wins > q.Losses
		return ok, fmt.Sprintf("gates %s, defects %s, judge %d-%d", g, q.DefectsStatus, q.Wins, q.Losses)
	}
	w1, d1 := wins(a1)
	w2, d2 := wins(a2)
	w3, d3 := wins(a3)
	lines := []string{"A1 " + d1, "A2 " + d2, "A3 " + d3}
	switch {
	case strings.Contains(d1, "UNDETERMINED") || strings.Contains(d2, "UNDETERMINED") || strings.Contains(d3, "UNDETERMINED"):
		lines = append(lines, "=> cannot attribute until A1, A2 and A3 are all measured")
	case !w1:
		lines = append(lines, "=> A1 does not win; no attribution needed")
	case w2 && !w3:
		lines = append(lines, "=> A1 wins and A2 (Pro tier) wins: adopt at the Pro tier level")
	case w3 && !w2:
		lines = append(lines, "=> A1 wins and A3 (Flash tier) wins: adopt at the Flash tier level")
	case w2 && w3:
		lines = append(lines, "=> A1 wins and both tiers win alone: adopt at tier level; compare the two margins")
	default:
		lines = append(lines, "=> A1 wins but neither tier alone does: the gain likely comes from shared context between agents. Run Phase 3A before deciding anything.")
	}
	return lines
}

// manifestModels is the set of models the scenario routes to (empty in legacy mode).
func (r *Report) manifestModels(id string) []string {
	if r.models == nil {
		return nil
	}
	return r.models[id]
}
