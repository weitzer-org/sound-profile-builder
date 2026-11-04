package main

import (
	"testing"

	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

func TestPlan_FullMatrixIs234(t *testing.T) {
	all := BuildScenarios(false)
	if len(all) != 9 {
		t.Fatalf("want 9 scenarios (S0-S5, A1-A3), got %d", len(all))
	}
	n := len(evalfixtures.GoldenQueryOrder())
	p := MakePlan(all, n, 2, false)
	if p.Total != 234 {
		t.Fatalf("full matrix = %d runs, want 234 (9 x 13 x 2)", p.Total)
	}
	if p.Executable != 208 { // S3 skipped: pro-latest resolves to the baseline Pro model
		t.Fatalf("executable = %d, want 208", p.Executable)
	}
	if got := MakePlan(all, n, 2, true).Executable; got != 234 {
		t.Fatalf("-include-skipped executable = %d, want 234", got)
	}
}

func TestScenarios_ChangeExactlyWhatThePlanSays(t *testing.T) {
	all := BuildScenarios(false)
	byID := map[string]Scenario{}
	for _, s := range all {
		byID[s.ID] = s
		if len(s.AgentModels) != 13 {
			t.Errorf("%s routes %d agents, want all 13", s.ID, len(s.AgentModels))
		}
	}
	diff := func(id string) map[string]string {
		out := map[string]string{}
		for k, v := range byID[id].AgentModels {
			if v != byID["S0"].AgentModels[k] {
				out[k] = v
			}
		}
		return out
	}
	expect := map[string]int{"S0": 0, "S1": 10, "S2": 2, "S3": 2, "S4": 3, "S5": 1, "A1": 13, "A2": 2, "A3": 10}
	for id, n := range expect {
		if got := len(diff(id)); got != n {
			t.Errorf("%s differs from S0 on %d agents, want %d: %v", id, got, n, diff(id))
		}
	}
	// One variable at a time: S1 must leave the Critic and Pro tier on baseline.
	if byID["S1"].AgentModels["13_critic"] != modelFlashBaseline || byID["S1"].AgentModels["12_architect"] != modelProBaseline {
		t.Error("S1 changed more than the Flash tier")
	}
	if byID["S3"].Skip == "" || byID["S3"].Pins[modelProLatest] != proLatestResolved {
		t.Error("S3 must be skipped and pinned to the resolved id")
	}
}

func TestScenarios_ArgonNoSearchKeepsGroundedAgentsOnBaseline(t *testing.T) {
	for _, s := range BuildScenarios(true) {
		if s.ID != "A1" && s.ID != "A2" && s.ID != "A3" {
			continue
		}
		for _, k := range groundedKeys {
			if got := s.AgentModels[k]; got == modelArgon {
				t.Errorf("%s: grounded agent %s on Argon despite -argon-no-search", s.ID, k)
			}
		}
	}
	// ...and the toggle only affects Argon scenarios.
	a, b := BuildScenarios(false), BuildScenarios(true)
	for i := range a {
		if a[i].ID[0] == 'A' {
			continue
		}
		for k, v := range a[i].AgentModels {
			if b[i].AgentModels[k] != v {
				t.Errorf("%s changed by -argon-no-search", a[i].ID)
			}
		}
	}
}

func TestSelect(t *testing.T) {
	all := BuildScenarios(false)
	got, err := SelectScenarios(all, "s0, A1")
	if err != nil || len(got) != 2 || got[0].ID != "S0" || got[1].ID != "A1" {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := SelectScenarios(all, "S9"); err == nil {
		t.Fatal("unknown scenario must error")
	}
	q, err := SelectQueries(evalfixtures.GoldenQueryOrder(), evalfixtures.GoldenQueries(), "07_Edge,09_BB_King")
	if err != nil || len(q) != 2 {
		t.Fatalf("got %v, %v", q, err)
	}
	if _, err := SelectQueries(evalfixtures.GoldenQueryOrder(), evalfixtures.GoldenQueries(), "nope"); err == nil {
		t.Fatal("unknown query must error")
	}
}

func TestScenarios_BaselineFlashPinnedToServedModel(t *testing.T) {
	for _, s := range BuildScenarios(false) {
		if s.Pins[modelFlashBaseline] != flashBaselineServed {
			t.Errorf("%s: gemini-3.5-flash must be pinned to its served id", s.ID)
		}
	}
	for _, s := range BuildScenarios(false) {
		if s.ID == "S3" && s.Pins[modelProLatest] != proLatestResolved {
			t.Error("S3 lost its pro-latest pin")
		}
	}
}
