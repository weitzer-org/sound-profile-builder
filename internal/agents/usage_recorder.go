package agents

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/storage"
	"google.golang.org/genai"
)

// UsageRecord is one durable per-call token/latency/cost/success record,
// mirroring the identical schema used by the sibling job_tracker and gsr
// projects (internal/usage.Record, adk/backend/src/usage.ts) — all three
// deliberately share the same storage layout and field names so query
// recipes are interchangeable across projects. See RunAgentSplit for where
// this is populated: every real attempt against a fallback-chain model, or
// against the Open-LLM backend, gets one record — success or failure.
type UsageRecord struct {
	Timestamp string `json:"timestamp"` // RFC3339Nano, UTC
	Provider  string `json:"provider"`  // "gemini" | "openllm"
	CallType  string `json:"callType"`  // agent role, e.g. "Tone Historian"
	RefID     string `json:"refId,omitempty"`
	Model     string `json:"model"`

	InputTokens    int32 `json:"inputTokens"`
	OutputTokens   int32 `json:"outputTokens"`
	ThinkingTokens int32 `json:"thinkingTokens,omitempty"`

	LatencyMS int64   `json:"latencyMs"`
	CostUSD   float64 `json:"costUsd"`

	// Success/ErrorKind: a failed call still carries a real LatencyMS and
	// Model (which fallback-chain member was attempted), zero tokens/cost.
	Success   bool   `json:"success"`
	ErrorKind string `json:"errorKind,omitempty"`
}

// geminiPriceTable is USD per 1,000,000 tokens. Mirrors the pricing table
// used by the sibling job_tracker project (internal/scoring/pricing.go) for
// the models both projects share — keep them in sync when prices change.
// gemini-2.5-pro/gemini-3-flash-preview are this project's own
// fallback-chain models (see getFallbackChain) that job_tracker doesn't use;
// VERIFY these two specifically before relying on them for a real cost
// report — 3-flash-preview's figure is a placeholder pending an official one.
var geminiPriceTable = map[string]struct{ input, output float64 }{
	"gemini-3.1-pro-preview": {input: 2.00, output: 12.00},
	"gemini-3.5-flash":       {input: 1.50, output: 9.00},
	"gemini-3.6-flash":       {input: 1.50, output: 7.50},
	"gemini-3-flash-preview": {input: 1.50, output: 9.00}, // placeholder, matches 3.5-flash pending an official figure
	"gemini-2.5-pro":         {input: 1.25, output: 10.00},
}

// computeCostUSD returns 0 for an unknown/openllm model rather than
// throwing — a signal to add the model to the table, not "this call was
// free." Thinking tokens are billed at the output rate, matching Google's
// documented behavior (same convention job_tracker's CostUSD uses).
func computeCostUSD(model string, inputTokens, outputTokens, thinkingTokens int32) float64 {
	price, ok := geminiPriceTable[model]
	if !ok {
		return 0
	}
	const perM = 1_000_000.0
	billedOutput := float64(outputTokens) + float64(thinkingTokens)
	return float64(inputTokens)*price.input/perM + billedOutput*price.output/perM
}

// classifyError gives a coarse, stable label for a failed call so usage
// records can be aggregated by failure reason. Neither the genai SDK nor
// OpenLLMClient exposes a single typed error taxonomy the way job_tracker's
// scoring.ProviderError does, so this inspects the error text for the
// common signals instead.
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") || strings.Contains(msg, "resource_exhausted"):
		return "rate_limit"
	case strings.Contains(msg, "401") || strings.Contains(msg, "403") || strings.Contains(msg, "permission_denied") || strings.Contains(msg, "unauthenticated"):
		return "auth"
	case strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "context deadline") || strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "unavailable") || strings.Contains(msg, "internal error") || strings.Contains(msg, "server error"):
		return "unavailable"
	case strings.Contains(msg, "maxoutputtokens"):
		return "max_tokens_truncated"
	default:
		return "api_error"
	}
}

// usageObjectKey builds usage/<YYYY-MM-DD>/<HHMMSSmmm>-<8 hex chars>.json.
// The random suffix guarantees uniqueness even for two calls completing
// within the same millisecond — an unconditional write would otherwise
// silently overwrite the earlier record instead of erroring.
func usageObjectKey(t time.Time) string {
	utc := t.UTC()
	day := utc.Format("2006-01-02")
	millis := utc.Format("150405.000")
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return fmt.Sprintf("usage/%s/%s-%s.json", day, strings.ReplaceAll(millis, ".", ""), hex.EncodeToString(buf[:]))
}

// recordAttemptUsage builds and persists a UsageRecord for one real Gemini
// attempt inside RunAgentSplit's fallback-chain retry loop — success (resp
// non-nil, err nil), a hard failure (resp nil, err non-nil), or a truncated
// response now treated as a retryable failure (resp AND err both non-nil --
// the call completed and burned real tokens, it just isn't a usable result).
// A small helper rather than inlining at each of RunAgentSplit's call sites
// to keep the token/cost extraction logic in one place.
func recordAttemptUsage(ctx context.Context, o *Orchestrator, agentRole, model string, latencyMs int64, resp *genai.GenerateContentResponse, err error) {
	rec := UsageRecord{
		Provider:  "gemini",
		CallType:  agentRole,
		Model:     model,
		LatencyMS: latencyMs,
		Success:   err == nil,
	}
	if err != nil {
		rec.ErrorKind = classifyError(err)
	}
	// Tokens/cost are real whenever the API actually returned a response, even
	// a truncated one being treated as a failure above -- a burned 8000-token
	// runaway generation must show up as spend, not silently zero out.
	if resp != nil && resp.UsageMetadata != nil {
		rec.InputTokens = resp.UsageMetadata.PromptTokenCount
		rec.OutputTokens = resp.UsageMetadata.CandidatesTokenCount
		rec.ThinkingTokens = resp.UsageMetadata.ThoughtsTokenCount
		rec.CostUSD = computeCostUSD(model, rec.InputTokens, rec.OutputTokens, rec.ThinkingTokens)
	}
	recordUsage(ctx, o.gcs, o.UsageBucket, o.Reporter, rec)
}

// recordUsage best-effort persists rec to gcs under bucket, and — when
// reporter is non-nil — pushes it to GSR's hosted usage dashboard too (see
// usage_reporter.go's UsageReporter.Report, itself nil-safe so callers never
// need to nil-check reporter). Neither path returns an error: this is
// nil-safe on gcs/an empty bucket too (most cmd/eval_*/cmd/enrich_captures
// tools construct an Orchestrator with gcs=nil and never set UsageBucket,
// since they have no durability requirement for this data) — a broken/
// unreachable/absent store must never turn a dropped analytics record into
// a failed agent call.
func recordUsage(ctx context.Context, gcs storage.Client, bucket string, reporter *UsageReporter, rec UsageRecord) {
	rec.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	reporter.Report(rec)

	if gcs == nil || bucket == "" {
		return
	}
	data, err := json.Marshal(rec)
	if err != nil {
		log.Printf("usage: marshaling record: %v", err)
		return
	}
	if err := gcs.WriteFile(ctx, bucket, usageObjectKey(time.Now()), data); err != nil {
		log.Printf("usage: writing record: %v", err)
	}
}
