package agents

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"google.golang.org/genai"
)

// EvalNoFallbackEnv is the eval-only switch that disables getFallbackChain (and the
// "retry without grounding" degrade path in runAgentSplit). Production never sets it.
//
// Why it exists: getFallbackChain routes a failing model to a different one, so an eval of a
// candidate that fails (or truncates) could silently be scored on a completely different model.
// With this set to "true" a failed
// candidate call fails visibly instead. See docs/eval/gemini-model-upgrade-test-plan.md §3 0.1.
const EvalNoFallbackEnv = "EVAL_NO_FALLBACK"

// AblationEnvSet returns the ABLATE_AGENT_* variables set to "true" (runAgentSplit then returns a
// stub for that agent without calling any model and without recording a call). Live eval tools
// refuse to run with any set, so a leftover shell variable cannot leak a stub into results.
func AblationEnvSet() []string {
	var set []string
	for i := 1; i <= 13; i++ {
		k := fmt.Sprintf("ABLATE_AGENT_%d", i)
		if os.Getenv(k) == "true" {
			set = append(set, k)
		}
	}
	return set
}

// EvalNoFallbackEnabled is the exported form of evalNoFallback, so eval tools gate on exactly
// the same parsing (trimmed, case-insensitive "true") the orchestrator uses.
func EvalNoFallbackEnabled() bool { return evalNoFallback() }

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
	// Grounded is true when the response carries Google Search grounding evidence: at least one
	// web search query or grounding chunk in Candidate.GroundingMetadata. A request can ask for
	// the GoogleSearch tool and still get Grounded=false (the model chose not to search, or
	// skips search when a response schema is set -- see the Phase 0 capability matrix), so this
	// is the only honest signal that an agent was actually grounded.
	Grounded      bool `json:"grounded"`
	SearchQueries int  `json:"search_queries,omitempty"` // len(GroundingMetadata.WebSearchQueries)
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

// responseGrounding reports whether the first candidate carries grounding evidence, and how many
// search queries the model issued. nil-safe.
func responseGrounding(resp *genai.GenerateContentResponse) (grounded bool, queries int) {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0] == nil {
		return false, 0
	}
	gm := resp.Candidates[0].GroundingMetadata
	if gm == nil {
		return false, 0
	}
	return len(gm.WebSearchQueries) > 0 || len(gm.GroundingChunks) > 0, len(gm.WebSearchQueries)
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
	} else if resp != nil && len(resp.Candidates) == 0 {
		// The SDK returned no error but there is nothing to use (e.g. a safety block). Callers such
		// as the advisory Preset Critic swallow this, so it must not read as a clean call.
		rec.Success, rec.Error = false, "response has no candidates"
	}
	rec.Grounded, rec.SearchQueries = responseGrounding(resp)
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
