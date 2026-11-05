package main

import (
	"sync"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// promptRoles are the agents whose full system and user prompts are saved, besides every
// agent's output: the Librarian and Cloud Navigator (what blocks were offered) and the
// Architect (exactly what it was given, including the injected device sections). The other
// agents' prompts are fixed templates plus upstream outputs already saved here.
var promptRoles = map[string]bool{
	"CorOS Librarian":       true,
	"Cloud Navigator":       true,
	"Architect & Evaluator": true,
}

// agentCall is one agent call of a run, as saved to <query>.agents.json.
type agentCall struct {
	Role         string `json:"role"`
	Model        string `json:"model,omitempty"`
	Error        string `json:"error,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	UserPrompt   string `json:"user_prompt,omitempty"`
	Output       string `json:"output"`
}

// agentIORecorder is a RunObserver that keeps each agent call's output (and, for
// promptRoles, its prompts), so an eval can attribute what an agent produced to what it
// was given. Agents run in parallel within a phase, so it locks.
type agentIORecorder struct {
	mu      sync.Mutex
	pending map[string][2]string // role -> system, user prompt of the call in flight
	calls   []agentCall
}

func newAgentIORecorder() *agentIORecorder {
	return &agentIORecorder{pending: map[string][2]string{}}
}

func (r *agentIORecorder) OnPhase(string)                         {}
func (r *agentIORecorder) OnAttempt(string, string, int64, error) {}
func (r *agentIORecorder) OnInfo(string)                          {}
func (r *agentIORecorder) OnAgentStart(role, system, user string) {
	if !promptRoles[role] {
		return
	}
	r.mu.Lock()
	r.pending[role] = [2]string{system, user}
	r.mu.Unlock()
}

func (r *agentIORecorder) OnAgentDone(role, model, output string, _ agents.AgentUsage, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := agentCall{Role: role, Model: model, Output: output}
	if err != nil {
		c.Error = err.Error()
	}
	if p, ok := r.pending[role]; ok {
		c.SystemPrompt, c.UserPrompt = p[0], p[1]
		delete(r.pending, role)
	}
	r.calls = append(r.calls, c)
}

// Calls returns the recorded calls in completion order.
func (r *agentIORecorder) Calls() []agentCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]agentCall(nil), r.calls...)
}
