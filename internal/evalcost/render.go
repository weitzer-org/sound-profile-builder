package evalcost

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Money formats a known figure; never used for UNKNOWN ones.
func Money(usd float64) string { return fmt.Sprintf("$%.3f", usd) }

// CostCell renders a RunCost as a table cell: exact when fully priced, ">= $X (+N calls unpriced)"
// when partial, and "UNKNOWN" when nothing was priced. It never prints $0 for an unpriced call.
func CostCell(c RunCost) string {
	switch c.Status() {
	case "KNOWN":
		return Money(c.KnownUSD)
	case "PARTIAL":
		return fmt.Sprintf(">= %s (+%d unpriced calls)", Money(c.KnownUSD), c.UnknownCalls())
	}
	return "UNKNOWN"
}

// Render writes the cost-gate report.
func Render(w io.Writer, ex *Extrapolation, batchID string, smoke bool, assume map[string]Hypothetical) {
	fmt.Fprintf(w, "=== Cost gate (plan §5): smoke batch %s ===\n", batchID)
	if !smoke {
		fmt.Fprintln(w, "NOTE: this batch is not a smoke batch; per-run costs average over whatever it contains.")
	}
	fmt.Fprintf(w, "Prices: repo geminiPriceTable (internal/agents/usage_recorder.go) -- NOT independently verified here. Pricing basis: %s model id.\n", ex.Basis)
	fmt.Fprintln(w, "Cost = recorded input + output + thinking tokens x table price (thinking billed at the output rate).")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-4s %-9s %-34s %-28s %-9s %s\n", "ID", "runs used", "measured $/run", "projected for full matrix", "full runs", "unpriced models (calls, in/out+think tokens per run)")
	for _, s := range ex.Scenarios {
		if s.Status == "NO_REPRESENTATIVE_RUN" {
			fmt.Fprintf(w, "%-4s %-9d %-34s %-28s %-9d smoke spend so far: %s\n", s.Scenario, 0, "UNKNOWN (no usable run)", "UNKNOWN", s.FullRuns, CostCell(s.SpentSmoke))
			continue
		}
		var un []string
		for _, m := range s.PerRun.SortedModels() {
			t := s.PerRun.Unpriced[m]
			un = append(un, fmt.Sprintf("%s (%d calls, %d/%d)", m, t.Calls, t.Input, t.Output+t.Thinking))
		}
		fmt.Fprintf(w, "%-4s %-9d %-34s %-28s %-9d %s\n", s.Scenario, s.Representative, CostCell(s.PerRun), CostCell(s.Projected), s.FullRuns, strings.Join(un, "; "))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "FULL MATRIX: %d runs.\n", ex.FullRuns)
	if ex.Complete {
		fmt.Fprintf(w, "  Projected total: %s (every scenario fully priced).\n", Money(ex.KnownLowerBound))
	} else {
		fmt.Fprintf(w, "  Projected total: UNKNOWN. Priced part only (LOWER BOUND, not a budget): >= %s\n", Money(ex.KnownLowerBound))
		fmt.Fprintln(w, "  Left out of that figure:")
		for _, why := range ex.IncompleteBecaus {
			fmt.Fprintln(w, "    -", why)
		}
	}
	if len(assume) > 0 {
		fmt.Fprintln(w, "\n  WHAT-IF (HYPOTHETICAL operator-supplied prices; NOT official, never added to the figures above):")
		var models []string
		for m := range assume {
			models = append(models, m)
		}
		sort.Strings(models)
		for _, m := range models {
			fmt.Fprintf(w, "    assumed %s = $%g in / $%g out per 1M tokens\n", m, assume[m].Input, assume[m].Output)
		}
		total := ex.KnownLowerBound
		var still []string
		for _, s := range ex.Scenarios {
			if s.Status == "NO_REPRESENTATIVE_RUN" {
				still = append(still, s.Scenario+" (no usable smoke run)")
				continue
			}
			add, missing := s.Projected.WhatIf(assume)
			total += add
			if len(missing) > 0 {
				still = append(still, s.Scenario+" (no assumed price for "+strings.Join(missing, ", ")+")")
			}
		}
		if len(still) == 0 {
			fmt.Fprintf(w, "    what-if total: %s\n", Money(total))
		} else {
			fmt.Fprintf(w, "    what-if total: >= %s (still lower bound; unpriced: %s)\n", Money(total), strings.Join(still, "; "))
		}
	}
	fmt.Fprintf(w, "\n  Smoke pass actually spent (priced calls only): %s\n", Money(ex.SpentSmokeKnown))
	fmt.Fprintln(w, "\nEXCLUDED from every figure above:")
	fmt.Fprintln(w, "  - calls to models with no entry in geminiPriceTable (listed per scenario; never counted as $0)")
	fmt.Fprintln(w, "  - Google Search grounding fees for agents 1-3 (billing of grounded calls is an UNVERIFIED assumption)")
	fmt.Fprintln(w, "  - cached-token discounts, and any price change since the table was last edited")
	fmt.Fprintln(w, "  - the re-critic (one extra Critic call per output, Phase 2.2) and the judge calls (Phase 2.3): priced separately")
	fmt.Fprintln(w, "  - skipped scenarios (S3)")
	fmt.Fprintln(w, "  - smoke runs that failed or were not integrity-clean (not representative; their spend is in the line above):")
	any := false
	for _, s := range ex.Scenarios {
		for _, e := range s.Excluded {
			fmt.Fprintf(w, "      %s: %s\n", s.Scenario, e)
			any = true
		}
	}
	if !any {
		fmt.Fprintln(w, "      (none)")
	}
	fmt.Fprintln(w, "\nCAVEAT: each scenario's per-run figure is ONE run of one query. Cost varies by query; multiplying by 26 assumes that query is typical.")
}
