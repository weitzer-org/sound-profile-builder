// Package runs records what happened during one pipeline or refine run: its
// status and phase, per-agent traces (model, tokens, latency, attempts) and an
// ordered event log. Records persist as runs/<run_id>.json through
// storage.Client, so they survive restarts and work on every backend.
package runs

import "time"

// Run statuses.
const (
	StatusRunning   = "running"
	StatusComplete  = "complete"
	StatusError     = "error"
	StatusAbandoned = "abandoned"
)

// Run kinds and sources.
const (
	KindGenerate = "generate"
	KindRefine   = "refine"

	SourceUI  = "ui"
	SourceMCP = "mcp"
)

// RunEvent types.
const (
	EventPhase         = "phase"
	EventAgentStart    = "agent_start"
	EventAgentDone     = "agent_done"
	EventAgentError    = "agent_error"
	EventAttemptFailed = "attempt_failed"
	EventInfo          = "info"
)

// Usage is an aggregate of token and call counts. Like agents.TokenUsage it counts
// each agent call's winning attempt; tokens burned by failed fallback attempts are
// recorded per attempt in the usage/ analytics records (recordAttemptUsage), and
// AgentTrace.Attempts shows how many attempts a call took.
type Usage struct {
	InputTokens    int32 `json:"input_tokens"`
	OutputTokens   int32 `json:"output_tokens"`
	ThinkingTokens int32 `json:"thinking_tokens"`
	Calls          int   `json:"calls"`
}

// RunRecord is the persisted manifest of one run.
type RunRecord struct {
	ID              string                 `json:"id"`     // gen-<uuid> | chat-<uuid>
	Kind            string                 `json:"kind"`   // generate | refine
	Source          string                 `json:"source"` // ui | mcp
	Status          string                 `json:"status"` // running | complete | error | abandoned
	Phase           string                 `json:"phase"`  // last progress string
	UserPrompt      string                 `json:"user_prompt"`
	EffectivePrompt string                 `json:"effective_prompt"` // after GLOBAL USER CONFIG FLAG prefixes; empty until known
	Constraints     map[string]any         `json:"constraints"`
	PromptVersions  map[string]string      `json:"prompt_versions"` // cfg.AgentPrompts snapshot
	PresetID        string                 `json:"preset_id,omitempty"`
	ParentPresetID  string                 `json:"parent_preset_id,omitempty"` // refine runs
	Agents          map[string]*AgentTrace `json:"agents"`                     // key = role, e.g. "CorOS Librarian"
	Events          []RunEvent             `json:"events"`
	Totals          Usage                  `json:"totals"`
	Error           string                 `json:"error,omitempty"`
	Mock            bool                   `json:"mock"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
}

// RunEvent is one entry in a run's ordered log. Seq is 1-based and strictly
// increasing within a run.
type RunEvent struct {
	Seq       int       `json:"seq"`
	TS        time.Time `json:"ts"`
	Type      string    `json:"type"`
	Agent     string    `json:"agent,omitempty"`
	Msg       string    `json:"msg"`
	Model     string    `json:"model,omitempty"`
	LatencyMs int64     `json:"latency_ms,omitempty"`
}

// AgentTrace summarises one agent's call within a run.
type AgentTrace struct {
	Role              string `json:"role"`
	Model             string `json:"model,omitempty"`
	Status            string `json:"status"` // running | done | error
	PromptKey         string `json:"prompt_key,omitempty"`
	PromptVersion     string `json:"prompt_version,omitempty"`
	InputTokens       int32  `json:"input_tokens"`
	OutputTokens      int32  `json:"output_tokens"`
	ThinkingTokens    int32  `json:"thinking_tokens"`
	LatencyMs         int64  `json:"latency_ms"`
	Attempts          int    `json:"attempts"`
	SystemPromptChars int    `json:"system_prompt_chars"` // so clients know how much to page
	InputChars        int    `json:"input_chars"`
	OutputChars       int    `json:"output_chars"`
	Error             string `json:"error,omitempty"`
}

// Clone returns a deep copy, safe to hand out while the recorder keeps mutating.
func (r *RunRecord) Clone() RunRecord {
	c := *r
	if r.Constraints != nil {
		c.Constraints = make(map[string]any, len(r.Constraints))
		for k, v := range r.Constraints {
			// Slices would otherwise stay shared with the live constraints (and the
			// config's plugin list), so a reader mutating a snapshot could race.
			if ss, ok := v.([]string); ok {
				v = append([]string(nil), ss...)
			}
			c.Constraints[k] = v
		}
	}
	if r.PromptVersions != nil {
		c.PromptVersions = make(map[string]string, len(r.PromptVersions))
		for k, v := range r.PromptVersions {
			c.PromptVersions[k] = v
		}
	}
	if r.Agents != nil {
		c.Agents = make(map[string]*AgentTrace, len(r.Agents))
		for k, v := range r.Agents {
			t := *v
			c.Agents[k] = &t
		}
	}
	if r.Events != nil {
		c.Events = append([]RunEvent(nil), r.Events...)
	}
	if r.CompletedAt != nil {
		t := *r.CompletedAt
		c.CompletedAt = &t
	}
	return c
}
