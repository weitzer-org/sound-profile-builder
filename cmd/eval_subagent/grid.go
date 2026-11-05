package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// Model ids (docs/eval/gemini-model-upgrade-test-plan.md §4, 1A).
const (
	modelProBaseline   = "gemini-3.1-pro-preview"
	modelFlashBaseline = "gemini-3.5-flash"
	modelArgon         = "gemini-4-argon"
	modelFlash38       = "gemini-3.8-flash"
	modelLite          = "gemini-3.5-flash-lite"

	// flashBaselineServed: gemini-3.5-flash is served as gemini-3.6-flash (Phase 0.4, 7 of 7
	// calls). The baseline still REQUESTS 3.5-flash (what production sends) and is expected to be
	// served as this id.
	flashBaselineServed = "gemini-3.6-flash"
)

// servedPins maps a requested id to the id the API is expected to report as served.
var servedPins = map[string]string{modelFlashBaseline: flashBaselineServed}

// ExpectedServed returns the ModelVersion a call to model should report.
func ExpectedServed(model string) string {
	if p, ok := servedPins[model]; ok {
		return p
	}
	return model
}

// AgentSpec describes one independent agent (input is a direct function of the user query).
type AgentSpec struct {
	Num             int
	Role            string // RunAgentSplit agentRole
	Key             string // agentConfig / AgentModels key
	Baseline        string // current production model
	Candidates      []string
	BuildUserPrompt func(query string) string
}

// Specs returns the three independent agents with their plan-§4 baselines and candidates.
func Specs() []AgentSpec {
	plain := func(q string) string { return "User Request: " + q }
	return []AgentSpec{
		{Num: 1, Role: "Tone Historian", Key: "1_tone_historian", Baseline: modelProBaseline,
			Candidates: []string{modelArgon, modelFlash38}, BuildUserPrompt: plain},
		{Num: 2, Role: "Sonic Profiler", Key: "2_sonic_profiler", Baseline: modelFlashBaseline,
			Candidates: []string{modelArgon, modelFlash38, modelLite},
			BuildUserPrompt: func(q string) string {
				return fmt.Sprintf("User Request: %s\nQC Block Parameter Vocabulary: %s", q, agents.GetQCSonicProfilerSchemaJSON())
			}},
		{Num: 3, Role: "Community Scraper", Key: "3_community_scraper", Baseline: modelFlashBaseline,
			Candidates: []string{modelArgon, modelFlash38, modelLite}, BuildUserPrompt: plain},
	}
}

// SelectAgents parses "1,2,3" (numbers or prompt keys, or "all").
func SelectAgents(all []AgentSpec, s string) ([]AgentSpec, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "all") {
		return all, nil
	}
	var out []AgentSpec
	seen := map[int]bool{}
	for _, tok := range strings.Split(s, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		var found *AgentSpec
		for i := range all {
			if n, err := strconv.Atoi(tok); err == nil && n == all[i].Num || tok == all[i].Key {
				found = &all[i]
			}
		}
		if found == nil {
			return nil, fmt.Errorf("unknown agent %q (this tool covers 1,2,3 only: their input does not depend on upstream agents)", tok)
		}
		if !seen[found.Num] {
			seen[found.Num] = true
			out = append(out, *found)
		}
	}
	return out, nil
}

// ModelsFor returns the models to run for spec: the baseline first, then the candidates. override
// ("" or "default" = the plan's per-agent candidates) replaces the candidate list for every
// selected agent; the baseline is always included exactly once.
func ModelsFor(spec AgentSpec, override string) []string {
	cands := spec.Candidates
	if o := strings.TrimSpace(override); o != "" && !strings.EqualFold(o, "default") {
		cands = nil
		for _, m := range strings.Split(o, ",") {
			if m = strings.TrimSpace(m); m != "" {
				cands = append(cands, m)
			}
		}
	}
	out := []string{spec.Baseline}
	seen := map[string]bool{spec.Baseline: true}
	for _, m := range cands {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// Job is one planned API call.
type Job struct {
	Spec     AgentSpec
	Model    string
	Baseline bool
	Query    string
	Rep      int
}

// BuildGrid enumerates every call. Order is query -> rep -> agent -> model, so all models of an
// agent are exercised back to back on the same input and drift in API conditions hits them equally.
func BuildGrid(specs []AgentSpec, modelsOverride string, queries []string, reps int) []Job {
	var jobs []Job
	for _, q := range queries {
		for r := 1; r <= reps; r++ {
			for _, sp := range specs {
				for _, m := range ModelsFor(sp, modelsOverride) {
					jobs = append(jobs, Job{Spec: sp, Model: m, Baseline: m == sp.Baseline, Query: q, Rep: r})
				}
			}
		}
	}
	return jobs
}

// GridCounts returns calls per agent key and the total.
func GridCounts(jobs []Job) (perAgent map[string]int, total int) {
	perAgent = map[string]int{}
	for _, j := range jobs {
		perAgent[j.Spec.Key]++
	}
	return perAgent, len(jobs)
}

// SelectQueries filters the golden order by comma-separated names (empty/"all" = all).
func SelectQueries(order []string, known map[string]string, names string) ([]string, error) {
	names = strings.TrimSpace(names)
	if names == "" || strings.EqualFold(names, "all") {
		return order, nil
	}
	var out []string
	seen := map[string]bool{}
	for _, n := range strings.Split(names, ",") {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		if _, ok := known[n]; !ok {
			ks := make([]string, 0, len(known))
			for k := range known {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			return nil, fmt.Errorf("unknown query %q (known: %s)", n, strings.Join(ks, ","))
		}
		out = append(out, n)
	}
	return out, nil
}
