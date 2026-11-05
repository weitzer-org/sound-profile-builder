// Package evalcost turns recorded tokens (CallRecord) into dollars using the repo's own
// geminiPriceSchedule, and extrapolates a smoke pass to the full matrix (plan §5 cost gate).
//
// Two rules are enforced everywhere here:
//  1. A model with no official price is UNKNOWN, never $0. A partly-priced run is a LOWER BOUND
//     and says how many calls and tokens it leaves out.
//  2. Cost comes from recorded tokens x geminiPriceSchedule only. Nothing is estimated from list
//     prices, and no price is invented.
//
// What is NOT included in any figure (the report must say so): Google Search grounding fees (the
// agents 1-3 tool calls; whether/how they are billed is an unverified assumption), cached-token
// discounts, tool-use prompt tokens beyond what UsageMetadata folds into PromptTokenCount, and
// any price change after the table was last edited.
package evalcost

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

// Basis selects which model id prices a call.
type Basis string

const (
	// BasisRequested prices by the model id we asked for (CallRecord.AttemptModel). This is what
	// the production usage/ records do, so figures reconcile with cmd/usage_report.
	BasisRequested Basis = "requested"
	// BasisServed prices by the API-reported ModelVersion. They differ for gemini-3.5-flash,
	// which is SERVED as gemini-3.6-flash (Phase 0.4) and has a different table output price.
	// Which one Google bills is an unverified assumption.
	BasisServed Basis = "served"
)

// ParseBasis validates a flag value.
func ParseBasis(s string) (Basis, error) {
	switch Basis(s) {
	case BasisRequested, BasisServed:
		return Basis(s), nil
	}
	return "", fmt.Errorf("price basis %q must be %q or %q", s, BasisRequested, BasisServed)
}

// ModelTokens is the token volume attributed to one priced-or-not model id.
type ModelTokens struct {
	Calls    int   `json:"calls"`
	Input    int64 `json:"input_tokens"`
	Output   int64 `json:"output_tokens"`
	Thinking int64 `json:"thinking_tokens"`
}

// RunCost is the cost of one run (all its attempts, failed ones included: they burned tokens).
type RunCost struct {
	KnownUSD float64 `json:"known_usd"`
	Calls    int     `json:"calls"`
	// Unpriced is the token volume per model id with no official price. KnownUSD excludes it.
	Unpriced map[string]ModelTokens `json:"unpriced,omitempty"`
	// Total token volume of the run (priced and unpriced).
	Input, Output, Thinking int64
}

// UnknownCalls is the number of calls whose cost is not in KnownUSD.
func (c RunCost) UnknownCalls() int {
	n := 0
	for _, m := range c.Unpriced {
		n += m.Calls
	}
	return n
}

// Status is "KNOWN" (every call priced), "PARTIAL" (lower bound) or "UNKNOWN" (nothing priced).
func (c RunCost) Status() string {
	switch u := c.UnknownCalls(); {
	case c.Calls == 0 || u == c.Calls:
		return "UNKNOWN"
	case u > 0:
		return "PARTIAL"
	}
	return "KNOWN"
}

// pricedModel picks the id a call is priced by under basis. A served basis falls back to the
// requested id when the API reported no ModelVersion (a failed attempt), because a failed call
// can still have burned tokens only if it returned usage -- and then it also returned a version.
func pricedModel(c agents.CallRecord, b Basis) string {
	if b == BasisServed && c.ServedModel != "" {
		return strings.TrimPrefix(c.ServedModel, "models/")
	}
	return c.AttemptModel
}

// CostOfCalls prices one run's call records.
func CostOfCalls(calls []agents.CallRecord, b Basis) RunCost {
	rc := RunCost{Calls: len(calls), Unpriced: map[string]ModelTokens{}}
	for _, c := range calls {
		rc.Input += int64(c.InputTokens)
		rc.Output += int64(c.OutputTokens)
		rc.Thinking += int64(c.ThinkingTokens)
		model := pricedModel(c, b)
		in, out, ok := agents.PriceFor(model)
		if !ok {
			m := rc.Unpriced[model]
			m.Calls++
			m.Input += int64(c.InputTokens)
			m.Output += int64(c.OutputTokens)
			m.Thinking += int64(c.ThinkingTokens)
			rc.Unpriced[model] = m
			continue
		}
		// Thinking tokens bill at the output rate, same convention as agents.computeCostUSD.
		rc.KnownUSD += float64(c.InputTokens)*in/1e6 + float64(c.OutputTokens+c.ThinkingTokens)*out/1e6
	}
	return rc
}

// SortedModels returns the unpriced model ids in stable order.
func (c RunCost) SortedModels() []string {
	var ks []string
	for k := range c.Unpriced {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// Hypothetical is a clearly-labelled what-if price (USD per 1M tokens) supplied by the operator.
// It is never merged into KnownUSD.
type Hypothetical struct{ Input, Output float64 }

// ParseAssume parses "model=in/out" (e.g. gemini-3.8-flash=0.75/3.75).
func ParseAssume(spec string) (string, Hypothetical, error) {
	model, rest, ok := strings.Cut(spec, "=")
	in, out, ok2 := strings.Cut(rest, "/")
	var h Hypothetical
	if !ok || !ok2 || strings.TrimSpace(model) == "" {
		return "", h, fmt.Errorf("-assume %q: want model=inputPerM/outputPerM", spec)
	}
	var err error
	if h.Input, err = strconv.ParseFloat(strings.TrimSpace(in), 64); err != nil {
		return "", h, fmt.Errorf("-assume %q: bad input price: %v", spec, err)
	}
	if h.Output, err = strconv.ParseFloat(strings.TrimSpace(out), 64); err != nil {
		return "", h, fmt.Errorf("-assume %q: bad output price: %v", spec, err)
	}
	if h.Input <= 0 || h.Output <= 0 {
		return "", h, fmt.Errorf("-assume %q: prices must be positive", spec)
	}
	return strings.TrimSpace(model), h, nil
}

// WhatIf prices the unpriced tokens of c at the supplied hypothetical prices. missing lists
// unpriced models that have no hypothetical, so the what-if total is itself still a lower bound.
func (c RunCost) WhatIf(assume map[string]Hypothetical) (usd float64, missing []string) {
	for _, m := range c.SortedModels() {
		h, ok := assume[m]
		if !ok {
			missing = append(missing, m)
			continue
		}
		t := c.Unpriced[m]
		usd += float64(t.Input)*h.Input/1e6 + float64(t.Output+t.Thinking)*h.Output/1e6
	}
	return usd, missing
}

// ScenarioCost is the extrapolation for one scenario.
type ScenarioCost struct {
	Scenario string
	// Measured runs, by role.
	Representative int // successful, integrity-clean runs: the basis of the per-run figure
	Excluded       []string
	SpentSmoke     RunCost // everything the measured runs burned, representative or not
	PerRun         RunCost // mean over representative runs (zero Calls when there are none)
	FullRuns       int     // runs in the full matrix for this scenario
	Projected      RunCost // PerRun x FullRuns
	Status         string  // KNOWN | PARTIAL | UNKNOWN | NO_REPRESENTATIVE_RUN
}

// Extrapolation is the whole matrix.
type Extrapolation struct {
	Scenarios         []ScenarioCost
	FullRuns          int
	KnownLowerBound   float64
	Complete          bool // false when any scenario's cost is partial/unknown
	IncompleteBecause []string
	SpentSmokeKnown   float64
	Basis             Basis
}

// Extrapolate prices each scenario of a smoke batch and scales the per-run mean to
// fullRunsPerScenario (queries x reps of the full matrix). Only scenarios that were planned (not
// skipped in the manifest) are extrapolated. Runs that failed or were not integrity-clean are not
// representative of a normal run's cost (a failed run stopped early; a fallback run was billed
// at another model's rate), so they are excluded from the mean but their spend is reported.
func Extrapolate(b *evalcheck.Batch, fullRunsPerScenario int, basis Basis) (*Extrapolation, error) {
	if b.Manifest == nil {
		return nil, fmt.Errorf("%s has no manifest.json: the scenario list (and which are skipped) is unknown, so a full-matrix total cannot be formed", b.Root)
	}
	if len(b.Problems) > 0 {
		return nil, fmt.Errorf("batch has structural problems, refusing to extrapolate: %s", strings.Join(b.Problems, "; "))
	}
	ex := &Extrapolation{Basis: basis, Complete: true}
	for _, ms := range b.Manifest.Scenarios {
		if ms.Skip != "" {
			continue
		}
		sc := ScenarioCost{Scenario: ms.ID, FullRuns: fullRunsPerScenario}
		ex.FullRuns += fullRunsPerScenario
		sc.SpentSmoke.Unpriced = map[string]ModelTokens{}
		var repCosts []RunCost
		for _, r := range b.Records {
			if r.Scenario != ms.ID {
				continue
			}
			calls, ok := b.Calls[r.Key()]
			if !ok {
				sc.Excluded = append(sc.Excluded, fmt.Sprintf("%s: calls file missing", r.Key()))
				continue
			}
			rc := CostOfCalls(calls, basis)
			sc.SpentSmoke = addCost(sc.SpentSmoke, rc)
			if okRun, why := b.Scoreable(r); !okRun {
				sc.Excluded = append(sc.Excluded, fmt.Sprintf("%s: not representative (%s)", r.Key(), why))
				continue
			}
			repCosts = append(repCosts, rc)
		}
		ex.SpentSmokeKnown += sc.SpentSmoke.KnownUSD
		sc.Representative = len(repCosts)
		if len(repCosts) == 0 {
			sc.Status = "NO_REPRESENTATIVE_RUN"
			ex.Complete = false
			ex.IncompleteBecause = append(ex.IncompleteBecause, ms.ID+": no successful integrity-clean smoke run to measure (its cost is UNKNOWN, not $0)")
			ex.Scenarios = append(ex.Scenarios, sc)
			continue
		}
		sum := RunCost{Unpriced: map[string]ModelTokens{}}
		for _, rc := range repCosts {
			sum = addCost(sum, rc)
		}
		n := float64(len(repCosts))
		sc.PerRun = scaleCost(sum, 1/n)
		sc.Projected = scaleCost(sum, float64(fullRunsPerScenario)/n)
		sc.Status = sum.Status() // from the unscaled sum: scaling rounds call counts
		if sc.Status != "KNOWN" {
			ex.Complete = false
			why := ms.ID + ": " + sc.Status + " (excludes " + strings.Join(sc.PerRun.SortedModels(), ", ") + ")"
			ex.IncompleteBecause = append(ex.IncompleteBecause, why)
		}
		ex.KnownLowerBound += sc.Projected.KnownUSD
		ex.Scenarios = append(ex.Scenarios, sc)
	}
	return ex, nil
}

func addCost(a, b RunCost) RunCost {
	out := RunCost{KnownUSD: a.KnownUSD + b.KnownUSD, Calls: a.Calls + b.Calls, Input: a.Input + b.Input, Output: a.Output + b.Output, Thinking: a.Thinking + b.Thinking, Unpriced: map[string]ModelTokens{}}
	for _, src := range []map[string]ModelTokens{a.Unpriced, b.Unpriced} {
		for k, v := range src {
			m := out.Unpriced[k]
			m.Calls += v.Calls
			m.Input += v.Input
			m.Output += v.Output
			m.Thinking += v.Thinking
			out.Unpriced[k] = m
		}
	}
	return out
}

// scaleCost multiplies a cost by f. Call/token counts are rounded to whole numbers.
func scaleCost(a RunCost, f float64) RunCost {
	r := func(x int64) int64 { return int64(float64(x)*f + 0.5) }
	out := RunCost{KnownUSD: a.KnownUSD * f, Calls: int(float64(a.Calls)*f + 0.5), Input: r(a.Input), Output: r(a.Output), Thinking: r(a.Thinking), Unpriced: map[string]ModelTokens{}}
	for k, v := range a.Unpriced {
		out.Unpriced[k] = ModelTokens{Calls: int(float64(v.Calls)*f + 0.5), Input: r(v.Input), Output: r(v.Output), Thinking: r(v.Thinking)}
	}
	return out
}
