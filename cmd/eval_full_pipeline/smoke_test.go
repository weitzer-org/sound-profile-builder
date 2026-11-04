package main

import (
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

func TestSmoke_IsNineRunsEightExecutable(t *testing.T) {
	q, reps, err := ApplySmoke(map[string]bool{}, "all", 2)
	if err != nil || q != "07_Edge" || reps != 1 {
		t.Fatalf("ApplySmoke = %q,%d,%v", q, reps, err)
	}
	all := BuildScenarios(false)
	p := MakePlan(all, 1, reps, false)
	if p.Total != 9 || p.Executable != 8 {
		t.Fatalf("smoke plan = %+v, want 9 planned / 8 executable (S3 skipped)", p)
	}
}

func TestSmoke_RefusesExplicitQueriesOrReps(t *testing.T) {
	for _, name := range []string{"queries", "reps"} {
		if _, _, err := ApplySmoke(map[string]bool{name: true}, "all", 2); err == nil {
			t.Errorf("-smoke with explicit -%s must be refused", name)
		}
	}
}

func TestManifest_FreezesRoutingAndPlannedRuns(t *testing.T) {
	m := BuildManifest(BuildScenarios(false), []string{"07_Edge"}, 1, true, false, time.Date(2026, 11, 5, 1, 2, 3, 0, time.UTC))
	if m.BatchID != "20261105T010203Z" || !m.Smoke || len(m.Scenarios) != 9 {
		t.Fatalf("manifest = %+v", m)
	}
	if m.Planned("S3") != 0 || m.Planned("S0") != 1 {
		t.Errorf("planned S3=%d S0=%d, want 0 / 1", m.Planned("S3"), m.Planned("S0"))
	}
	s5, _ := m.Scenario("S5")
	if s5.AgentModels["13_critic"] != "gemini-4-argon" {
		t.Errorf("manifest lost S5 routing: %v", s5.AgentModels)
	}
	if s0, _ := m.Scenario("S0"); s0.Pins["gemini-3.5-flash"] != "gemini-3.6-flash" {
		t.Errorf("S0 pin not recorded: %v", s0.Pins)
	}
	if mi := BuildManifest(BuildScenarios(false), []string{"07_Edge"}, 1, true, true, time.Now()); mi.Planned("S3") != 1 {
		t.Error("-include-skipped must make S3 plannable")
	}
}

// The re-critic tool defaults to the S0 Critic model it reads from the manifest; without a
// manifest it falls back to evalcheck's constants. They must agree with the scenario table.
func TestS0CriticDefaultsMatchScenarioTable(t *testing.T) {
	s0 := BuildScenarios(false)[0]
	if got := s0.AgentModels["13_critic"]; got != evalcheck.S0CriticModel {
		t.Errorf("S0 critic = %s, evalcheck.S0CriticModel = %s", got, evalcheck.S0CriticModel)
	}
	if got := s0.Pins[evalcheck.S0CriticModel]; got != evalcheck.S0CriticServed {
		t.Errorf("S0 critic pin = %s, evalcheck.S0CriticServed = %s", got, evalcheck.S0CriticServed)
	}
}
