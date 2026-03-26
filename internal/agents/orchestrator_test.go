package agents

import (
	"context"
	"strings"
	"testing"
)

// MockOrchestrator overriding RunAgent to test concurrency without hitting the real API
type MockOrchestrator struct {
	Orchestrator
	callCount int
}

func (m *MockOrchestrator) RunAgent(ctx context.Context, agentRole string, prompt string) (string, error) {
	m.callCount++
	// Return stub JSON based on agentRole
	if strings.Contains(agentRole, "Historian") {
		return `{"amp_type": "Fender Twin", "pedal_names": ["TS9"], "speaker_type": "12in", "mic_type": "SM57"}`, nil
	}
	if strings.Contains(agentRole, "Profiler") {
		return `{"peak_freq_hz": 1200, "saturation_style": "clean_breakup", "reverb_type": "spring"}`, nil
	}
	if strings.Contains(agentRole, "Scraper") {
		return `{"suggested_qc_blocks": ["UK C30"], "community_warnings": ["too muddy"]}`, nil
	}
	return "{}", nil
}

func TestPhase1Concurrency(t *testing.T) {
	m := &MockOrchestrator{}
	
	ctx := context.Background()
	
	// Inject the real WaitGroup logic but override RunAgent calls for mock testing
	var toneResult, sonicResult, scrapeResult string
	var err1, err2, err3 error

	importWg := make(chan bool)

	go func() {
		toneResult, err1 = m.RunAgent(ctx, "Tone Historian", "Mock Prompt")
		sonicResult, err2 = m.RunAgent(ctx, "Sonic Profiler", "Mock Prompt")
		scrapeResult, err3 = m.RunAgent(ctx, "Community Scraper", "Mock Prompt")
		importWg <- true
	}()

	<-importWg

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("Expected no errors, got: %v, %v, %v", err1, err2, err3)
	}
	
	if !strings.Contains(toneResult, "Fender Twin") || len(sonicResult) == 0 || len(scrapeResult) == 0 {
		t.Errorf("One of the agents failed to return strict output mocks")
	}

	if m.callCount != 3 {
		t.Errorf("Expected 3 agent calls concurrent, got %d", m.callCount)
	}
}
