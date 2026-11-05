package agents

import (
	"encoding/json"
	"os"
	"testing"
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
