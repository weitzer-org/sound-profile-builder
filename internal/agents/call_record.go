package agents

import (
	"os"
	"regexp"
	"strings"

	"google.golang.org/genai"
)

// EvalNoFallbackEnv is the eval-only switch that disables getFallbackChain (and the
// "retry without grounding" degrade path in runAgentSplit). Production never sets it.
//
// Why it exists: getFallbackChain routes any unrecognized model to gemini-2.5-pro, and the
// gemini-3.x chains end in it too, so an eval of a new candidate that fails (or truncates)
// can silently be scored on a completely different model. With this set to "true" a failed
// candidate call fails visibly instead. See docs/eval/gemini-model-upgrade-test-plan.md §3 0.1.
const EvalNoFallbackEnv = "EVAL_NO_FALLBACK"

// evalNoFallback reports whether EVAL_NO_FALLBACK=true. Read per call (not cached) so tests
// can flip it with t.Setenv and eval tools can set it before constructing anything.
func evalNoFallback() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EvalNoFallbackEnv)), "true")
}

// CallRecord is one real Gemini attempt as observed on the wire: which model we asked for,
// which model the API says answered (GenerateContentResponse.ModelVersion), and why it
// stopped (Candidate.FinishReason). TokenUsage.ModelsUsed only holds requested-name counts,
// which cannot reveal a server-side alias/reroute; this can.
//
// Every attempt is recorded, including failed and truncated ones, so a retry is never hidden.
type CallRecord struct {
	Agent          string `json:"agent"`
	AttemptModel   string `json:"attempt_model"`          // the model name sent in this request
	ServedModel    string `json:"served_model,omitempty"` // resp.ModelVersion; empty if the API returned no response
	FinishReason   string `json:"finish_reason,omitempty"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
	InputTokens    int32  `json:"input_tokens,omitempty"`
	OutputTokens   int32  `json:"output_tokens,omitempty"`
	ThinkingTokens int32  `json:"thinking_tokens,omitempty"`
	LatencyMs      int64  `json:"latency_ms"`
}

// responseServedModel / responseFinishReason pull the two fields out of a response, nil-safe.
func responseServedModel(resp *genai.GenerateContentResponse) string {
	if resp == nil {
		return ""
	}
	return resp.ModelVersion
}

func responseFinishReason(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0] == nil {
		return ""
	}
	return string(resp.Candidates[0].FinishReason)
}

// NewCallRecord builds a CallRecord from one attempt's outcome.
func NewCallRecord(agent, attemptModel string, latencyMs int64, resp *genai.GenerateContentResponse, err error) CallRecord {
	rec := CallRecord{
		Agent: agent, AttemptModel: attemptModel, LatencyMs: latencyMs,
		ServedModel: responseServedModel(resp), FinishReason: responseFinishReason(resp),
		Success: err == nil,
	}
	if err != nil {
		rec.Error = err.Error()
	}
	if resp != nil && resp.UsageMetadata != nil {
		rec.InputTokens = resp.UsageMetadata.PromptTokenCount
		rec.OutputTokens = resp.UsageMetadata.CandidatesTokenCount
		rec.ThinkingTokens = resp.UsageMetadata.ThoughtsTokenCount
	}
	return rec
}

// appendCall records one attempt on u. Safe on a nil receiver.
func (u *TokenUsage) appendCall(rec CallRecord) {
	if u == nil {
		return
	}
	u.mu.Lock()
	u.Calls = append(u.Calls, rec)
	u.mu.Unlock()
}

// CallsSnapshot returns a copy of the recorded calls, for eval tooling.
func (u *TokenUsage) CallsSnapshot() []CallRecord {
	if u == nil {
		return nil
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]CallRecord(nil), u.Calls...)
}

// versionSuffix matches the trailing "-<digit...>" Google appends to a stable model id for a
// dated/numbered build, e.g. "gemini-3.5-flash-05-2026" or "gemini-2.5-pro-001".
var versionSuffix = regexp.MustCompile(`^-\d[\w.-]*$`)

// ServedModelMatches reports whether served (ModelVersion) is the requested model, allowing a
// trailing dated/numbered build suffix but NOT a different variant: requesting
// "gemini-3.5-flash" and being served "gemini-3.5-flash-lite" is a mismatch. A leading
// "models/" prefix is ignored. An empty served value never matches -- an answer that does not
// say who gave it cannot be proven to be the requested model.
//
// Aliases (e.g. "gemini-pro-latest") are expected to mismatch by design; eval scenarios must
// pin the resolved id instead (plan item 0.3).
func ServedModelMatches(requested, served string) bool {
	requested = strings.TrimPrefix(strings.TrimSpace(requested), "models/")
	served = strings.TrimPrefix(strings.TrimSpace(served), "models/")
	if requested == "" || served == "" {
		return false
	}
	if served == requested {
		return true
	}
	return strings.HasPrefix(served, requested) && versionSuffix.MatchString(strings.TrimPrefix(served, requested))
}
