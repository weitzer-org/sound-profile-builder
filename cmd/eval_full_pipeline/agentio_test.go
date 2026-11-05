package main

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

func TestAgentIORecorder(t *testing.T) {
	r := newAgentIORecorder()
	var wg sync.WaitGroup
	// Phase-1 agents run in parallel; none of them keeps its prompts.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			role := fmt.Sprintf("Agent %d", i)
			r.OnAgentStart(role, "sys", "user")
			r.OnAgentDone(role, "gemini-4-argon", "out", agents.AgentUsage{}, nil)
		}(i)
	}
	wg.Wait()
	r.OnAgentStart("CorOS Librarian", "lib sys", "lib user")
	r.OnAgentDone("CorOS Librarian", "gemini-4-argon", `{"suggested_blocks":["Spring"]}`, agents.AgentUsage{}, nil)
	r.OnAgentStart("Architect & Evaluator", "arch sys", "arch user")
	r.OnAgentDone("Architect & Evaluator", "", "", agents.AgentUsage{}, errors.New("boom"))

	calls := r.Calls()
	if len(calls) != 5 {
		t.Fatalf("got %d calls, want 5", len(calls))
	}
	for _, c := range calls[:3] {
		if c.SystemPrompt != "" || c.UserPrompt != "" || c.Output != "out" {
			t.Errorf("%s: only the output should be kept, got %+v", c.Role, c)
		}
	}
	if lib := calls[3]; lib.SystemPrompt != "lib sys" || lib.UserPrompt != "lib user" || lib.Output == "" {
		t.Errorf("Librarian call = %+v, want its prompts and output", lib)
	}
	if arch := calls[4]; arch.Error != "boom" || arch.UserPrompt != "arch user" {
		t.Errorf("Architect call = %+v, want its prompts and the error", arch)
	}
}
