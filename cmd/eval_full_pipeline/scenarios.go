package main

import (
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

// Model ids used by the Gemini-upgrade scenarios (docs/eval/gemini-model-upgrade-test-plan.md §5).
const (
	modelProBaseline   = "gemini-3.1-pro-preview"
	modelFlashBaseline = "gemini-3.5-flash"
	modelFlash38       = "gemini-3.8-flash"
	modelLite          = "gemini-3.5-flash-lite"
	modelArgon         = "gemini-4-argon"
	modelProLatest     = "gemini-pro-latest" // alias; PINNED below to what Phase 0.3 resolved it to

	// proLatestResolved is what gemini-pro-latest was served as on 2026-11-04 (plan item 0.3).
	proLatestResolved = "gemini-3.1-pro-preview"

	// flashBaselineServed is what gemini-3.5-flash is actually served as (Phase 0.4: 7 of 7
	// calls reported ModelVersion gemini-3.6-flash). Scenarios keep REQUESTING 3.5-flash (what
	// production sends) and expect it to be served as this id.
	flashBaselineServed = "gemini-3.6-flash"
)

// Agent prompt keys by production tier (internal/agents RunAgentSplit routing).
var (
	proTierKeys   = []string{"1_tone_historian", "12_architect"}
	flashTierKeys = []string{
		"2_sonic_profiler", "3_community_scraper", "4_coros_librarian", "5_cloud_navigator",
		"6_acoustician", "7_transducer_tech", "8_foh_optimizer", "9_mix_engineer",
		"10_control_mapper", "11_dsp_dispatcher",
	}
	criticKey = "13_critic"
	// Agents whose prompts depend on GoogleSearch grounding (agentSearchTool): 1, 2, 3.
	groundedKeys = []string{"1_tone_historian", "2_sonic_profiler", "3_community_scraper"}
	// S4's "mechanical" agents.
	mechanicalKeys = []string{"8_foh_optimizer", "10_control_mapper", "11_dsp_dispatcher"}
)

// Scenario is one complete per-agent routing map (every one of the 13 keys is always set, so a
// run never depends on a default).
type Scenario struct {
	ID          string
	Label       string
	AgentModels map[string]string
	Pins        map[string]string // alias -> resolved id the alias is expected to be served as
	Skip        string            // non-empty: not run by default, with the reason
}

type routing struct{ pro, flash, critic string }

func build(r routing, overrides map[string]string) map[string]string {
	m := map[string]string{}
	for _, k := range proTierKeys {
		m[k] = r.pro
	}
	for _, k := range flashTierKeys {
		m[k] = r.flash
	}
	m[criticKey] = r.critic
	for k, v := range overrides {
		m[k] = v
	}
	return m
}

// BuildScenarios returns S0-S5 and A1-A3 in plan order. argonNoSearch implements the plan's
// rule "if 0.5 shows Argon can't use Google Search, agents 1, 2 and 3 stay on baseline in
// A1/A2/A3".
func BuildScenarios(argonNoSearch bool) []Scenario {
	base := routing{modelProBaseline, modelFlashBaseline, modelFlashBaseline}
	keepGroundedOnBaseline := func(m map[string]string) map[string]string {
		if argonNoSearch {
			m["1_tone_historian"] = modelProBaseline
			m["2_sonic_profiler"] = modelFlashBaseline
			m["3_community_scraper"] = modelFlashBaseline
		}
		return m
	}
	argonOffAgent2 := map[string]string{"2_sonic_profiler": modelFlashBaseline}
	lite := map[string]string{}
	for _, k := range mechanicalKeys {
		lite[k] = modelLite
	}
	scenarios := []Scenario{
		{ID: "S0", Label: "Baseline", AgentModels: build(base, nil)},
		{ID: "S1", Label: "Flash tier -> 3.8 Flash", AgentModels: build(routing{modelProBaseline, modelFlash38, modelFlashBaseline}, nil)},
		{ID: "S2", Label: "Pro tier -> 3.8 Flash", AgentModels: build(routing{modelFlash38, modelFlashBaseline, modelFlashBaseline}, nil)},
		{ID: "S3", Label: "Pro tier -> pro-latest (pinned)",
			AgentModels: build(routing{modelProLatest, modelFlashBaseline, modelFlashBaseline}, nil),
			Pins:        map[string]string{modelProLatest: proLatestResolved},
			Skip:        fmt.Sprintf("Phase 0.3: %s is served as %s, identical to S0's Pro tier", modelProLatest, proLatestResolved)},
		{ID: "S4", Label: "Lite on mechanical agents 8,10,11", AgentModels: build(base, lite)},
		{ID: "S5", Label: "Critic -> Argon", AgentModels: build(routing{modelProBaseline, modelFlashBaseline, modelArgon}, nil)},
		// A1/A3 keep agent 2 (Sonic Profiler) on the Flash baseline: Phase 1A dropped Argon there
		// (schema-valid 16/26, truncated 10/26 vs 26/26 and 0 for baseline).
		{ID: "A1", Label: "Argon everywhere (agent 2 on baseline)", AgentModels: keepGroundedOnBaseline(build(routing{modelArgon, modelArgon, modelArgon}, argonOffAgent2))},
		{ID: "A2", Label: "Argon Pro tier only", AgentModels: keepGroundedOnBaseline(build(routing{modelArgon, modelFlashBaseline, modelFlashBaseline}, nil))},
		{ID: "A3", Label: "Argon Flash tier only (agent 2 on baseline)", AgentModels: keepGroundedOnBaseline(build(routing{modelProBaseline, modelArgon, modelFlashBaseline}, argonOffAgent2))},
	}
	// Every scenario that sends gemini-3.5-flash expects it to be served as 3.6 Flash.
	for i := range scenarios {
		pins := map[string]string{modelFlashBaseline: flashBaselineServed}
		for k, v := range scenarios[i].Pins {
			pins[k] = v
		}
		scenarios[i].Pins = pins
	}
	return scenarios
}

// ProductionScenario routes every agent exactly as production does by default
// (agents.DefaultModelFor), so a run validates what would deploy. Not part of the plan's S/A grid.
func ProductionScenario() Scenario {
	m := map[string]string{}
	for _, k := range append(append(append([]string{}, proTierKeys...), flashTierKeys...), criticKey) {
		m[k] = agents.DefaultModelFor(k)
	}
	return Scenario{ID: "PROD", Label: "Production defaults (agents.DefaultModelFor)", AgentModels: m, Pins: map[string]string{modelFlashBaseline: flashBaselineServed}}
}

// All38Scenario routes every agent to gemini-3.8-flash: the head-to-head counterpart of PROD.
func All38Scenario() Scenario {
	m := map[string]string{}
	for _, k := range append(append(append([]string{}, proTierKeys...), flashTierKeys...), criticKey) {
		m[k] = modelFlash38
	}
	return Scenario{ID: "ALL38", Label: "Every agent on gemini-3.8-flash", AgentModels: m, Pins: map[string]string{modelFlashBaseline: flashBaselineServed}}
}

// SelectScenarios filters by comma-separated IDs (empty = all, in plan order). Unknown IDs are an error.
func SelectScenarios(all []Scenario, ids string) ([]Scenario, error) {
	ids = strings.TrimSpace(ids)
	if ids == "" || strings.EqualFold(ids, "all") {
		return all, nil
	}
	byID := map[string]Scenario{"PROD": ProductionScenario(), "ALL38": All38Scenario()}
	for _, s := range all {
		byID[strings.ToUpper(s.ID)] = s
	}
	var out []Scenario
	seen := map[string]bool{}
	for _, id := range strings.Split(ids, ",") {
		id = strings.ToUpper(strings.TrimSpace(id))
		if id == "" || seen[id] {
			continue
		}
		s, ok := byID[id]
		if !ok {
			var known []string
			for k := range byID {
				known = append(known, k)
			}
			sort.Strings(known)
			return nil, fmt.Errorf("unknown scenario %q (known: %s)", id, strings.Join(known, ","))
		}
		seen[id] = true
		out = append(out, s)
	}
	return out, nil
}

// SelectQueries filters the golden order by comma-separated names (empty/"all" = all 13).
func SelectQueries(order []string, known map[string]string, names string) ([]string, error) {
	names = strings.TrimSpace(names)
	if names == "" || strings.EqualFold(names, "all") {
		return order, nil
	}
	var out []string
	seen := map[string]bool{}
	for _, n := range strings.Split(names, ",") {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] { // duplicates would overwrite their own output files
			continue
		}
		seen[n] = true
		if _, ok := known[n]; !ok {
			return nil, fmt.Errorf("unknown query %q (see evalfixtures.GoldenQueryOrder)", n)
		}
		out = append(out, n)
	}
	return out, nil
}

// Plan is the run count for a selection.
type Plan struct {
	Scenarios, Queries, Reps int
	Total                    int // every selected scenario, skipped ones included
	Executable               int // excluding skipped scenarios
	AgentCalls               int // Executable x 13 (one call per agent, before retries)
}

func MakePlan(sel []Scenario, queries, reps int, includeSkipped bool) Plan {
	p := Plan{Scenarios: len(sel), Queries: queries, Reps: reps}
	for _, s := range sel {
		p.Total += queries * reps
		if s.Skip == "" || includeSkipped {
			p.Executable += queries * reps
		}
	}
	p.AgentCalls = p.Executable * 13
	return p
}

// SmokeQuery is the single golden query the §5 cost-gate smoke pass runs.
const SmokeQuery = "07_Edge"

// ApplySmoke resolves -smoke: it pins queries to SmokeQuery and reps to 1, and refuses an
// explicit -queries/-reps (a "smoke" pass that silently ran something else would make the cost
// extrapolation meaningless). explicit is the set of flag names the user actually passed.
func ApplySmoke(explicit map[string]bool, queries string, reps int) (string, int, error) {
	if explicit["queries"] || explicit["reps"] {
		return queries, reps, fmt.Errorf("-smoke fixes the run to %s x 1 rep and cannot be combined with -queries/-reps", SmokeQuery)
	}
	return SmokeQuery, 1, nil
}

// BuildManifest freezes the invocation (scenario routing included) into the batch manifest.
// Skipped scenarios are listed with their reason so a reader can tell "not run" from "lost".
func BuildManifest(selected []Scenario, queries []string, reps int, smoke, includeSkipped bool, now time.Time) evalcheck.Manifest {
	m := evalcheck.Manifest{
		BatchID:   now.Format("20060102T150405Z"),
		CreatedAt: now.Format(time.RFC3339),
		Smoke:     smoke,
		Reps:      reps,
		Queries:   append([]string(nil), queries...),
		Git:       gitRevision(),
	}
	for _, s := range selected {
		skip := s.Skip
		if includeSkipped {
			skip = ""
		}
		m.Scenarios = append(m.Scenarios, evalcheck.ManifestScenario{ID: s.ID, Label: s.Label, AgentModels: s.AgentModels, Pins: s.Pins, Skip: skip})
	}
	return m
}

// gitRevision is the vcs.revision stamped into the binary by `go build` (empty under `go run`).
func gitRevision() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, kv := range bi.Settings {
			if kv.Key == "vcs.revision" {
				return kv.Value
			}
		}
	}
	return ""
}
