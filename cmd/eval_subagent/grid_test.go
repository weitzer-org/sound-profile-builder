package main

import (
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

func TestDefaultGridMatchesPlan(t *testing.T) {
	specs, err := SelectAgents(Specs(), "1,2,3")
	if err != nil {
		t.Fatal(err)
	}
	jobs := BuildGrid(specs, "default", evalfixtures.GoldenQueryOrder(), 2)
	per, total := GridCounts(jobs)
	// agent 1: baseline + argon + 3.8 = 3 models x 26; agents 2,3: baseline + 3 candidates = 4 x 26.
	if per["1_tone_historian"] != 78 || per["2_sonic_profiler"] != 104 || per["3_community_scraper"] != 104 || total != 286 {
		t.Fatalf("grid = %v total %d, want 78/104/104 = 286", per, total)
	}
}

func TestBaselinesAndCandidates(t *testing.T) {
	sp := Specs()
	if got := ModelsFor(sp[0], ""); strings.Join(got, ",") != "gemini-3.1-pro-preview,gemini-4-argon,gemini-3.8-flash" {
		t.Fatalf("agent1 models %v", got)
	}
	if got := ModelsFor(sp[1], "default"); got[0] != "gemini-3.5-flash" || len(got) != 4 {
		t.Fatalf("agent2 models %v", got)
	}
	// override replaces candidates, baseline kept once even if listed.
	if got := ModelsFor(sp[2], "gemini-4-argon, gemini-3.5-flash"); strings.Join(got, ",") != "gemini-3.5-flash,gemini-4-argon" {
		t.Fatalf("override models %v", got)
	}
}

func TestSelectAgentsRejectsDependentAgents(t *testing.T) {
	if _, err := SelectAgents(Specs(), "4"); err == nil {
		t.Fatal("agent 4 depends on upstream output; must be rejected")
	}
	got, err := SelectAgents(Specs(), "3,3,1")
	if err != nil || len(got) != 2 || got[0].Num != 3 {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestExpectedServedPin(t *testing.T) {
	if ExpectedServed("gemini-3.5-flash") != "gemini-3.6-flash" || ExpectedServed("gemini-4-argon") != "gemini-4-argon" {
		t.Fatal("pin mapping wrong")
	}
}

func TestSelectQueries(t *testing.T) {
	g := evalfixtures.GoldenQueries()
	if q, err := SelectQueries(evalfixtures.GoldenQueryOrder(), g, "07_Edge,07_Edge"); err != nil || len(q) != 1 {
		t.Fatalf("%v %v", q, err)
	}
	if _, err := SelectQueries(evalfixtures.GoldenQueryOrder(), g, "nope"); err == nil {
		t.Fatal("unknown query accepted")
	}
}

func TestAggregateAndVerdict(t *testing.T) {
	mk := func(model string, base bool, valid, trunc bool, grounded bool) CallResult {
		return CallResult{AgentKey: "3_community_scraper", Agent: "Community Scraper", Model: model, Baseline: base,
			Success: !trunc, SchemaValid: valid, Truncated: trunc, Grounded: grounded, ServedOK: true, CallCount: 1, LatencyMs: 1000}
	}
	rs := []CallResult{
		mk("b", true, true, false, true), mk("b", true, true, false, true),
		mk("good", false, true, false, false), mk("good", false, true, false, false),
		mk("badschema", false, false, false, true), mk("badschema", false, true, false, true),
		mk("trunc", false, true, false, true), mk("trunc", false, false, true, true),
	}
	stats := Aggregate(rs)
	by := map[string]Stat{}
	for _, s := range stats {
		by[s.Model] = s
	}
	if d, _ := Verdict(by["b"], by["good"]); d {
		t.Error("good candidate dropped (grounding rate alone is not part of the exit rule)")
	}
	if d, _ := Verdict(by["b"], by["badschema"]); !d {
		t.Error("lower schema validity must drop")
	}
	if d, why := Verdict(by["b"], by["trunc"]); !d || len(why) < 2 {
		t.Errorf("truncating candidate: drop=%v why=%v", d, why)
	}
	bad := by["good"]
	bad.ServedBad = 1
	if d, _ := Verdict(by["b"], bad); !d {
		t.Error("served-model issue must drop")
	}
	if !strings.Contains(Markdown(stats), "**DROP**") {
		t.Error("markdown missing DROP lines")
	}
}
