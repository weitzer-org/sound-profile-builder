package agents

import (
	"encoding/json"
	"os"
	"testing"

	"google.golang.org/genai"
)

// The Refinement Architect does not receive config.json's agent_prompts, so its version is a
// constant. It must match generation's, or a refinement reintroduces what generation fixed
// (the Phase 1B dB-offset-on-modeled-amp defect was fixed in 12_architect_v5).
func TestRefineArchitectPromptMatchesConfig(t *testing.T) {
	b, err := os.ReadFile("../../config.json")
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		AgentPrompts map[string]string `json:"agent_prompts"`
	}
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	if got := c.AgentPrompts["12_architect"]; got != refineArchitectPromptVersion {
		t.Fatalf("config.json 12_architect=%q but RefineChat uses %q", got, refineArchitectPromptVersion)
	}
	for k, v := range c.AgentPrompts {
		if _, err := PromptFS.ReadFile("prompts/" + k + "_" + v + ".md"); err != nil {
			t.Errorf("config.json routes %s to %s but prompts/%s_%s.md does not exist (LoadPrompt would silently fall back to the base prompt)", k, v, k, v)
		}
	}
}

// Whole-number fields must be TypeInteger: as TypeNumber the models looped on "85.0000..." until
// MAX_TOKENS (Sonic Profiler, 2026-11-05 probe).
func TestWholeNumberFieldsAreIntegers(t *testing.T) {
	sp := agentResponseSchema("2_sonic_profiler")
	for _, f := range []string{"suggested_low_cut_hz", "suggested_high_cut_hz", "noise_gate_target_db", "peak_freq_hz"} {
		if sp.Properties[f].Type != genai.TypeInteger {
			t.Errorf("2_sonic_profiler.%s is %s, want INTEGER", f, sp.Properties[f].Type)
		}
	}
	foh := agentResponseSchema("8_foh_optimizer").Properties["global_eq_cuts"]
	for _, f := range []string{"hz_start", "hz_end"} {
		if foh.Properties[f].Type != genai.TypeInteger {
			t.Errorf("8_foh_optimizer.global_eq_cuts.%s is %s, want INTEGER", f, foh.Properties[f].Type)
		}
	}
	ac := agentResponseSchema("6_acoustician")
	if got := ac.Required; len(got) != 2 {
		t.Errorf("6_acoustician must require humbucker and single_coil, got %v", got)
	}
}
