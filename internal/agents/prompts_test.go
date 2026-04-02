package agents

import (
	"testing"
)

func TestLoadPrompt_Success(t *testing.T) {
	// The directory `prompts/1_tone_historian.md` should exist in the repository
	p, err := LoadPrompt("1_tone_historian", "")
	if err != nil {
		t.Fatalf("Failed to load tone historian prompt: %v", err)
	}
	if len(p) == 0 {
		t.Errorf("Prompt content was empty")
	}
}

func TestLoadPrompt_Error(t *testing.T) {
	// Missing file
	_, err := LoadPrompt("does_not_exist", "")
	if err == nil {
		t.Errorf("Expected an error when loading non-existent prompt")
	}
}
