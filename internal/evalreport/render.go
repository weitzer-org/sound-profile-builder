package evalreport

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/weitzer-org/sound-builder/internal/evalcost"
)

func f1(x float64) string {
	if math.IsNaN(x) {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", x)
}

func f2(x float64) string {
	if math.IsNaN(x) {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", x)
}

func meanI64(xs []int64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	t := 0.0
	for _, x := range xs {
		t += float64(x)
	}
	return t / float64(len(xs))
}

// costCell renders the mean $/run of a scenario without ever printing $0 for unpriced calls.
func costCell(cs []evalcost.RunCost) string {
	if len(cs) == 0 {
		return "UNMEASURED"
	}
	m := meanCost(cs)
	n := float64(len(cs))
	switch m.Status() {
	case "KNOWN":
		return fmt.Sprintf("$%.4f", m.KnownUSD)
	case "PARTIAL":
		var ms []string
		for k := range m.Unpriced {
			ms = append(ms, k)
		}
		sort.Strings(ms)
		return fmt.Sprintf(">=$%.4f +UNPRICED(%s, %.0f calls)", m.KnownUSD, strings.Join(ms, ","), float64(m.UnknownCalls())/n)
	}
	return "UNKNOWN"
}

// Render writes the human-readable report.
func (r *Report) Render(w io.Writer) {
	fmt.Fprintf(w, "=== Phase 2 report: %s ===\n", r.Root)
	if r.Legacy {
		fmt.Fprintln(w, "LEGACY (pre-manifest) directory: batch identity, served models, latency, tokens and cost are UNVERIFIABLE; only output-derived metrics are real. No decision can be made from this.")
	} else {
		fmt.Fprintf(w, "batch %s%s; baseline %s; every scenario is compared only with this batch's %s\n", r.BatchID, map[bool]string{true: " (SMOKE: 1 query x 1 rep, not a basis for decisions)", false: ""}[r.Smoke], r.BaselineID, r.BaselineID)
	}
	fmt.Fprintf(w, "cost basis: price by %s model id; prices are the repo's geminiPriceSchedule, not independently verified\n\n", r.Options.Basis)

	fmt.Fprintln(w, "PRECONDITIONS (a comparison is meaningless unless these hold)")
	blocked := false
	for _, c := range r.Pre {
		fmt.Fprintf(w, "  [%s] %s: %s\n", c.Status, c.Name, c.Detail)
		if c.Status != Pass {
			blocked = true
		}
	}
	if blocked {
		fmt.Fprintln(w, "  => NOT DECIDABLE: gates below are printed for diagnosis, but no scenario may be adopted or rejected on this batch.")
	}

	fmt.Fprintln(w, "\nEXCLUDED RUNS (reported, never scored)")
	any := false
	for _, s := range r.Scenarios {
		for _, e := range s.Excluded {
			fmt.Fprintf(w, "  %s: %s\n", e.Key, e.Reason)
			any = true
		}
	}
	if !any {
		fmt.Fprintln(w, "  (none)")
	}

	r.renderMatrix(w)
	r.renderAgentLatency(w)

	gates := map[string]Gates{}
	fmt.Fprintln(w, "\nHARD GATES (§6.1) -- a scenario fails if any gate fails")
	for _, s := range r.Scenarios {
		if s.Skipped != "" {
			fmt.Fprintf(w, "\n  %s %s: SKIPPED (%s)\n", s.ID, s.Label, s.Skipped)
			continue
		}
		g := r.EvaluateGates(s)
		gates[s.ID] = g
		fmt.Fprintf(w, "\n  %s: %s\n", scenarioName(s), g.Overall())
		for _, c := range g.Checks {
			fmt.Fprintf(w, "    [%-10s] %s -- %s\n", c.Status, c.Name, c.Detail)
		}
	}

	fmt.Fprintln(w, "\nDECISION RULES (§7), applied mechanically; UNDETERMINED means a needed input was not measured")
	for _, s := range r.Scenarios {
		results := r.EvaluateRules(s, gates[s.ID])
		if s.ID == r.BaselineID || s.Skipped != "" {
			continue
		}
		if len(results) == 0 {
			if r.Legacy {
				fmt.Fprintf(w, "\n  %s: routing is unknown in a legacy directory (no manifest), so no §7 tier rule can be applied\n", s.ID)
			} else {
				fmt.Fprintf(w, "\n  %s: routes identically to %s; nothing to decide\n", s.ID, r.BaselineID)
			}
			continue
		}
		for _, rr := range results {
			verdict := rr.Verdict
			if blocked && (verdict == Adopt || verdict == Provisional || verdict == NotAdopt) {
				verdict = Undetermined
			}
			fmt.Fprintf(w, "\n  %s [%s tier]: %s", s.ID, rr.Tier, verdict)
			if verdict != rr.Verdict {
				fmt.Fprintf(w, "  (would be %s; batch preconditions failed)", rr.Verdict)
			}
			fmt.Fprintln(w)
			for _, l := range rr.Lines {
				fmt.Fprintf(w, "    [%-10s] %s -- %s\n", l.Status, l.Name, l.Detail)
			}
			if rr.Note != "" {
				fmt.Fprintf(w, "    %s\n", rr.Note)
			}
		}
	}
	if lines := r.Attribution(gates); len(lines) > 0 {
		fmt.Fprintln(w, "\nA1/A2/A3 ATTRIBUTION (§7)")
		for _, l := range lines {
			fmt.Fprintln(w, "  "+l)
		}
	}
	fmt.Fprintln(w, "\nNOT MEASURED BY THIS REPORT: human listen test (§6.2.3); Critic accuracy sample (§7 Critic rule); deprecation dates (plan §10); Google Search grounding fees in $/run.")
}

func (r *Report) renderMatrix(w io.Writer) {
	fmt.Fprintln(w, "\nPER-SCENARIO METRICS (scoreable runs only; S0 first)")
	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	var cols []*ScenarioStats
	for _, s := range r.Scenarios {
		if s.Skipped == "" {
			cols = append(cols, s)
		}
	}
	head := "metric"
	for _, s := range cols {
		head += "\t" + s.ID
	}
	fmt.Fprintln(tw, head)
	row := func(name string, f func(s *ScenarioStats) string) {
		line := name
		for _, s := range cols {
			line += "\t" + f(s)
		}
		fmt.Fprintln(tw, line)
	}
	row("runs scoreable/planned", func(s *ScenarioStats) string { return fmt.Sprintf("%d/%d", s.Scoreable, s.Planned) })
	row("runs excluded", func(s *ScenarioStats) string { return fmt.Sprint(len(s.Excluded)) })
	row("served-model violations", func(s *ScenarioStats) string {
		if r.Legacy || s.CallsMissingRuns > 0 {
			return "UNMEASURED"
		}
		return fmt.Sprintf("%d/%d calls", s.CallsViolating, s.CallsChecked)
	})
	row("pipeline success rate", func(s *ScenarioStats) string {
		if r.Legacy {
			return "UNMEASURED"
		}
		return pct(rate(s.Scoreable, s.Planned))
	})
	row("schema valid (parse ok)", func(s *ScenarioStats) string { return fmt.Sprintf("%d/%d", s.Outputs-s.ParseErrors, s.Outputs) })
	row("MAX_TOKENS (per agent)", func(s *ScenarioStats) string {
		if r.Legacy || s.TruncRuns == 0 {
			return "UNMEASURED"
		}
		if len(s.Trunc) == 0 {
			return "0"
		}
		var p []string
		for a, n := range s.Trunc {
			p = append(p, fmt.Sprintf("%s:%d", shortAgent(a), n))
		}
		sort.Strings(p)
		return strings.Join(p, ",")
	})
	row("R1 capture_claim_on_modeled", func(s *ScenarioStats) string { return fmt.Sprint(s.R1) })
	row("R2 capture_db_on_modeled", func(s *ScenarioStats) string { return fmt.Sprint(s.R2) })
	row("modeled_style_on_capture", func(s *ScenarioStats) string { return fmt.Sprint(s.ModeledStyleOnCapture) })
	row("snapped block names", func(s *ScenarioStats) string { return fmt.Sprint(s.Snapped) })
	row("unverified block names", func(s *ScenarioStats) string { return fmt.Sprint(s.Unverified) })
	row("re-critic defects mean (sd; rep range)", func(s *ScenarioStats) string {
		if s.DefectSource != "re-critic" {
			return fmt.Sprintf("UNMEASURED (own critic: %d flags)", s.OwnCriticFlags)
		}
		if len(s.Defects) == 0 {
			return "UNMEASURED"
		}
		m, sd, rg := s.DefectSpread()
		miss := ""
		if s.DefectMissing > 0 {
			miss = fmt.Sprintf(" [%d missing]", s.DefectMissing)
		}
		return fmt.Sprintf("%s (%s; %s)%s", f2(m), f2(sd), f2(rg), miss)
	})
	row("pipeline latency p50/p95 (s)", func(s *ScenarioStats) string {
		if len(s.Latency) == 0 {
			return "UNMEASURED"
		}
		return fmt.Sprintf("%s/%s", f1(Percentile(s.Latency, 50)), f1(Percentile(s.Latency, 95)))
	})
	row("tokens/run in | out | think", func(s *ScenarioStats) string {
		if len(s.InTok) == 0 {
			return "UNMEASURED"
		}
		return fmt.Sprintf("%.0f | %.0f | %.0f", meanI64(s.InTok), meanI64(s.OutTok), meanI64(s.ThinkTok))
	})
	row("$/run (priced tokens)", func(s *ScenarioStats) string { return costCell(s.Cost) })
	tw.Flush()
	fmt.Fprintln(w, "  (percentiles: nearest-rank over scoreable runs; sd is across all runs; rep range is the mean per-query max-min across reps)")
}

func scenarioName(s *ScenarioStats) string {
	if s.Label == "" || s.Label == s.ID {
		return s.ID
	}
	return s.ID + " " + s.Label
}

func shortAgent(a string) string {
	if len(a) > 10 {
		return a[:10]
	}
	return a
}

func (r *Report) renderAgentLatency(w io.Writer) {
	roles := map[string]bool{}
	var cols []*ScenarioStats
	for _, s := range r.Scenarios {
		if s.Skipped != "" {
			continue
		}
		cols = append(cols, s)
		for a := range s.AgentLat {
			roles[a] = true
		}
	}
	if len(roles) == 0 {
		return
	}
	var names []string
	for a := range roles {
		names = append(names, a)
	}
	sort.Strings(names)
	fmt.Fprintln(w, "\nPER-AGENT LATENCY p50 (s; sum of the agent's attempts in a run)")
	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	head := "agent"
	for _, s := range cols {
		head += "\t" + s.ID
	}
	fmt.Fprintln(tw, head)
	for _, a := range names {
		line := a
		for _, s := range cols {
			line += "\t" + f1(Percentile(s.AgentLat[a], 50))
		}
		fmt.Fprintln(tw, line)
	}
	tw.Flush()
}
