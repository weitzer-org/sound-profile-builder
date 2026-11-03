package agents

import (
	"context"
	"encoding/json"
	"log"
)

// RunObserver receives a play-by-play of one pipeline/refine run. It is
// carried on the context (WithRunObserver), never on OrchestratorService, so
// the RunPipeline/RefineChat signatures, the cmd/eval_* tools and every test
// fake stay untouched. A nil observer (the default) means every hook below is
// skipped and behaviour is byte-for-byte what it was before the hook existed.
//
// Phase 1/3/4 agents run concurrently, so implementations MUST be safe for
// concurrent use.
type RunObserver interface {
	OnPhase(phase string)
	OnAgentStart(role, systemPrompt, userPrompt string)
	// OnAttempt fires once per model attempt in the fallback chain.
	OnAttempt(role, model string, latencyMs int64, err error)
	// OnAgentDone fires once per RunAgentSplit call, success or failure. u.Model
	// is the model that produced the output (empty on failure).
	OnAgentDone(role, model, output string, u AgentUsage, err error)
	// OnInfo carries free-form notes, e.g. that the Preset Critic ran.
	OnInfo(msg string)
}

type runObserverKey struct{}
type runIDKey struct{}

// WithRunObserver returns ctx carrying o. A nil o returns ctx unchanged.
func WithRunObserver(ctx context.Context, o RunObserver) context.Context {
	if o == nil {
		return ctx
	}
	return context.WithValue(ctx, runObserverKey{}, o)
}

// WithRunID returns ctx carrying the run ID; RunPipeline uses it as the folder
// name under logs/ in place of its timestamp execID.
func WithRunID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, runIDKey{}, id)
}

// RunIDFrom returns the run ID set by WithRunID, or "".
func RunIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(runIDKey{}).(string)
	return id
}

func runObserverFrom(ctx context.Context) RunObserver {
	o, _ := ctx.Value(runObserverKey{}).(RunObserver)
	return o
}

// callInfo carries the winning model and token usage out of runAgentSplit to
// the RunAgentSplit wrapper, which hands them to the observer. runAgentSplit
// takes it as an explicit parameter; nil (no observer) skips the bookkeeping.
type callInfo struct {
	model string
	usage AgentUsage
}

// AgentRoleKeys maps an agent role (the string passed to RunAgentSplit) to its
// prompt key in config.json's agent_prompts. It is the single source of truth
// for that mapping: RunAgentSplit's model routing uses it, and the run log
// uses it to name per-agent log files and look up prompt versions.
var AgentRoleKeys = map[string]string{
	"Tone Historian":            "1_tone_historian",
	"Sonic Profiler":            "2_sonic_profiler",
	"Community Scraper":         "3_community_scraper",
	"CorOS Librarian":           "4_coros_librarian",
	"Cloud Navigator":           "5_cloud_navigator",
	"Acoustician":               "6_acoustician",
	"Transducer Tech":           "7_transducer_tech",
	"FOH Optimizer":             "8_foh_optimizer",
	"Mix Engineer":              "9_mix_engineer",
	"Control Mapper":            "10_control_mapper",
	"DSP Dispatcher":            "11_dsp_dispatcher",
	"Architect & Evaluator":     "12_architect",
	"Refinement Architect":      "12_architect",
	"Preset Critic":             "13_critic",
	"Capture Enrichment":        "14_capture_enrichment",
	"User Capture Verification": "15_user_capture_verification",
}

// mockRunEvent is one step of the canned replay in testdata/e2e_mocks/run_events.json.
type mockRunEvent struct {
	Type         string `json:"type"` // phase | agent | info
	Msg          string `json:"msg,omitempty"`
	Role         string `json:"role,omitempty"`
	Model        string `json:"model,omitempty"`
	System       string `json:"system,omitempty"`
	Input        string `json:"input,omitempty"`
	Output       string `json:"output,omitempty"`
	InputTokens  int32  `json:"input_tokens,omitempty"`
	OutputTokens int32  `json:"output_tokens,omitempty"`
	LatencyMs    int64  `json:"latency_ms,omitempty"`
}

// replayMockRun feeds the canned event sequence for kind ("generate" or
// "refine") through obs so mock-mode runs have a realistic log. Mock mode
// calls no agents, so without this the log would be empty. A missing or
// malformed file is logged and skipped: it must never fail the mock run.
func replayMockRun(obs RunObserver, kind string) {
	raw, err := readMockFile("testdata/e2e_mocks/run_events.json")
	if err != nil {
		log.Printf("Warning: mock run replay skipped, cannot read run_events.json: %v", err)
		return
	}
	var all map[string][]mockRunEvent
	if err := json.Unmarshal([]byte(raw), &all); err != nil {
		log.Printf("Warning: mock run replay skipped, bad run_events.json: %v", err)
		return
	}
	for _, ev := range all[kind] {
		switch ev.Type {
		case "phase":
			obs.OnPhase(ev.Msg)
		case "info":
			obs.OnInfo(ev.Msg)
		case "agent":
			obs.OnAgentStart(ev.Role, ev.System, ev.Input)
			obs.OnAttempt(ev.Role, ev.Model, ev.LatencyMs, nil)
			obs.OnAgentDone(ev.Role, ev.Model, ev.Output, AgentUsage{
				InputTokens:  ev.InputTokens,
				OutputTokens: ev.OutputTokens,
				Model:        ev.Model,
				Calls:        1,
				LatencyMs:    ev.LatencyMs,
			}, nil)
		}
	}
}
