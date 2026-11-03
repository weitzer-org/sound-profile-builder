package agents

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// recObserver records every hook call; safe for the concurrent phases.
type recObserver struct {
	mu       sync.Mutex
	phases   []string
	starts   []string // role
	attempts []obsAttempt
	dones    []obsDone
	infos    []string
	eff      string
}

type obsAttempt struct {
	role, model string
	err         error
}

type obsDone struct {
	role, model, output string
	usage               AgentUsage
	err                 error
}

func (o *recObserver) OnPhase(p string) { o.mu.Lock(); o.phases = append(o.phases, p); o.mu.Unlock() }
func (o *recObserver) OnAgentStart(role, sys, user string) {
	o.mu.Lock()
	o.starts = append(o.starts, role+"|"+sys+"|"+user)
	o.mu.Unlock()
}
func (o *recObserver) OnAttempt(role, model string, _ int64, err error) {
	o.mu.Lock()
	o.attempts = append(o.attempts, obsAttempt{role, model, err})
	o.mu.Unlock()
}
func (o *recObserver) OnAgentDone(role, model, output string, u AgentUsage, err error) {
	o.mu.Lock()
	o.dones = append(o.dones, obsDone{role, model, output, u, err})
	o.mu.Unlock()
}
func (o *recObserver) OnInfo(m string)             { o.mu.Lock(); o.infos = append(o.infos, m); o.mu.Unlock() }
func (o *recObserver) SetEffectivePrompt(p string) { o.mu.Lock(); o.eff = p; o.mu.Unlock() }

func mockModeCtx(obs RunObserver) context.Context {
	ctx := context.WithValue(context.Background(), MockModeKey, true)
	ctx = WithRunID(ctx, "gen-test-run")
	return WithRunObserver(ctx, obs)
}

// Mock mode calls no agents, so the observer sees the canned replay; the output
// must be exactly what a run without an observer returns.
func TestRunPipelineMockReplaysThroughObserver(t *testing.T) {
	orch, err := NewOrchestrator(context.Background(), "fake-key", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	obs := &recObserver{}
	got, _, err := orch.RunPipeline(mockModeCtx(obs), "p", map[string]interface{}{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := orch.RunPipeline(context.WithValue(context.Background(), MockModeKey, true), "p", map[string]interface{}{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Error("an attached observer changed RunPipeline's output")
	}

	if len(obs.phases) != 6 {
		t.Errorf("phases = %d (%v), want the 6 canned phases", len(obs.phases), obs.phases)
	}
	if len(obs.dones) != 13 {
		t.Errorf("agent completions = %d, want 13 (12 generation agents + the critic)", len(obs.dones))
	}
	roles := map[string]bool{}
	for _, d := range obs.dones {
		roles[d.role] = true
		if d.err != nil || d.model == "" || d.output == "" {
			t.Errorf("incomplete completion %+v", d)
		}
		if _, ok := AgentRoleKeys[d.role]; !ok {
			t.Errorf("role %q has no AgentRoleKeys entry, so the run log couldn't name its prompt files", d.role)
		}
	}
	for _, must := range []string{"Tone Historian", "CorOS Librarian", "Architect & Evaluator", "Preset Critic"} {
		if !roles[must] {
			t.Errorf("replay is missing %q", must)
		}
	}
	if len(obs.infos) == 0 {
		t.Error("expected the critic info event")
	}
}

func TestRefineChatMockReplaysThroughObserver(t *testing.T) {
	orch, err := NewOrchestrator(context.Background(), "fake-key", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	obs := &recObserver{}
	if _, _, err := orch.RefineChat(mockModeCtx(obs), nil, "warmer", true, true); err != nil {
		t.Fatal(err)
	}
	if len(obs.dones) != 1 || obs.dones[0].role != "Refinement Architect" {
		t.Errorf("completions = %+v, want exactly the Refinement Architect", obs.dones)
	}
}

// The real call path: RunAgentSplit must report start, each attempt, and the
// winning model plus token usage.
func TestRunAgentSplitReportsToObserver(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockGeminiResponse))
	}))
	defer srv.Close()
	orch, err := NewOrchestrator(context.Background(), "fake-key", nil, WithEndpoint(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	obs := &recObserver{}
	ctx := WithRunObserver(context.Background(), obs)
	out, err := orch.RunAgentSplit(ctx, "Tone Historian", "SYS", "USER")
	if err != nil || out == "" {
		t.Fatalf("RunAgentSplit = %q, %v", out, err)
	}
	if len(obs.starts) != 1 || obs.starts[0] != "Tone Historian|SYS|USER" {
		t.Errorf("starts = %v, want one start carrying the exact prompts", obs.starts)
	}
	if len(obs.attempts) != 1 || obs.attempts[0].err != nil {
		t.Errorf("attempts = %+v, want one successful attempt", obs.attempts)
	}
	if len(obs.dones) != 1 {
		t.Fatalf("dones = %+v, want exactly one", obs.dones)
	}
	d := obs.dones[0]
	if d.err != nil || d.output != out || d.model == "" || d.model != obs.attempts[0].model {
		t.Errorf("done = %+v; want the winning model (= the successful attempt's) and the output", d)
	}
	if d.usage.InputTokens <= 0 || d.usage.OutputTokens <= 0 || d.usage.Model != d.model {
		t.Errorf("usage = %+v, want token counts and the model", d.usage)
	}
}

func TestRunAgentSplitReportsFailedAttempts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":400,"message":"bad","status":"INVALID_ARGUMENT"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()
	orch, err := NewOrchestrator(context.Background(), "fake-key", nil, WithEndpoint(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	obs := &recObserver{}
	_, err = orch.RunAgentSplit(WithRunObserver(context.Background(), obs), "Tone Historian", "SYS", "USER")
	if err == nil {
		t.Fatal("expected failure")
	}
	if len(obs.attempts) < 2 {
		t.Fatalf("attempts = %d, want the primary plus its fallbacks", len(obs.attempts))
	}
	for _, a := range obs.attempts {
		if a.err == nil {
			t.Errorf("attempt %+v should carry its error", a)
		}
	}
	if len(obs.dones) != 1 || obs.dones[0].err == nil || obs.dones[0].model != "" {
		t.Errorf("dones = %+v, want one failed completion with no winning model", obs.dones)
	}
	if !strings.Contains(obs.dones[0].err.Error(), "Tone Historian") {
		t.Errorf("error %q should name the agent", obs.dones[0].err)
	}
}

func TestRunIDRoundTrip(t *testing.T) {
	if got := RunIDFrom(context.Background()); got != "" {
		t.Errorf("RunIDFrom(empty ctx) = %q, want empty", got)
	}
	if got := RunIDFrom(WithRunID(context.Background(), "gen-1")); got != "gen-1" {
		t.Errorf("RunIDFrom = %q, want gen-1", got)
	}
	if ctx := WithRunObserver(context.Background(), nil); runObserverFrom(ctx) != nil {
		t.Error("a nil observer must leave the context observer-free")
	}
}
