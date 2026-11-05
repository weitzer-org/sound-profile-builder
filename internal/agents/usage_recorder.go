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

	// InputTokens is the FULL prompt size, cached tokens included (Gemini's
	// PromptTokenCount semantics). CachedInputTokens is the subset of it served
	// from Gemini's prompt cache; it is billed at the cheaper cached rate.
	InputTokens       int32 `json:"inputTokens"`
	CachedInputTokens int32 `json:"cachedInputTokens,omitempty"`
	OutputTokens      int32 `json:"outputTokens"`
	ThinkingTokens    int32 `json:"thinkingTokens,omitempty"`

	LatencyMS int64   `json:"latencyMs"`
	CostUSD   float64 `json:"costUsd"`

	// Success/ErrorKind: a failed call still carries a real LatencyMS and
	// Model (which fallback-chain member was attempted), zero tokens/cost.
	Success   bool   `json:"success"`
	ErrorKind string `json:"errorKind,omitempty"`

	// ServedModel is the API-reported GenerateContentResponse.ModelVersion and FinishReason the
	// candidate's finish reason; both empty when no response came back (or for openllm).
	ServedModel  string `json:"servedModel,omitempty"`
	FinishReason string `json:"finishReason,omitempty"`

	// GroundingQueries is the number of Google Search queries the model issued for this call
	// (GroundingMetadata.WebSearchQueries), billed separately from tokens: see geminiPriceSchedule.
	GroundingQueries int `json:"groundingQueries,omitempty"`

	// CostUnknown is true when the model has no entry in geminiPriceSchedule, so CostUSD=0 means
	// "not priced", not "free". Aggregations must not add these calls into a dollar total
	// without saying so.
	CostUnknown bool `json:"costUnknown,omitempty"`
}

// geminiPrice is USD per 1,000,000 tokens. Thinking tokens bill at the output rate (Google lists
// output as "response and reasoning"). cachedInput is the rate for prompt tokens served from
// Gemini's implicit prompt cache. Where the real cached rate is NOT verified it is set EQUAL to
// input (no discount), so an unknown rate never understates cost.
type geminiPrice struct{ input, cachedInput, output float64 }

// pricePeriod is a price in effect until (exclusive) Until; a zero Until means open-ended.
type pricePeriod struct {
	Until time.Time
	geminiPrice
}

// day returns 00:00 UTC on the given date. Google states promo windows as calendar dates ("through
// November 30, 2026") without a time zone, so a boundary may be off by up to a day.
func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// geminiPriceSchedule is USD per 1M tokens, standard tier, global endpoint, <=200K-token prompts.
//
// SOURCE (fetched 2026-11-05): Google Cloud's official generative AI pricing page,
// https://cloud.google.com/gemini-enterprise-agent-platform/generative-ai/pricing, plus the Google
// Cloud blog launch post for Argon (2026-10-13: "introductory rate of $2 per million input tokens
// and $10 per million output tokens through November 30, 2026 ... standard pricing of $4 ... and
// $20"). The Gemini Developer API pricing page (ai.google.dev) is blocked from the eval sessions and
// was NOT read directly; a Google-domain search summary of it reported the same Argon figures.
// Non-global (regional) endpoints are ~10% higher; prompts >200K tokens cost more on 3.1 Pro
// ($4/$18). Neither applies to this app today.
//
// Mirrors job_tracker's internal/scoring/pricing.go for the models both projects share -- keep
// them in sync when prices change (that repo is separate and was not updated in this change).
var geminiPriceSchedule = map[string][]pricePeriod{
	"gemini-4-argon": {
		{Until: day(2026, time.December, 1), geminiPrice: geminiPrice{2.00, 0.10, 10.00}}, // introductory
		{geminiPrice: geminiPrice{4.00, 0.20, 20.00}},
	},
	"gemini-3.8-flash": {
		{Until: day(2027, time.January, 1), geminiPrice: geminiPrice{0.75, 0.075, 3.75}}, // introductory
		{geminiPrice: geminiPrice{1.50, 0.15, 7.50}},
	},
	"gemini-3.6-flash": {
		{Until: day(2027, time.January, 1), geminiPrice: geminiPrice{0.75, 0.075, 3.75}}, // introductory
		{geminiPrice: geminiPrice{1.50, 0.15, 7.50}},
	},
	"gemini-3.5-flash":       {{geminiPrice: geminiPrice{1.50, 1.50, 9.00}}},
	"gemini-3.5-flash-lite":  {{geminiPrice: geminiPrice{0.30, 0.30, 2.50}}},
	"gemini-3.1-pro-preview": {{geminiPrice: geminiPrice{2.00, 0.20, 12.00}}},
	// Not on the 2026-11-05 page; kept for historical usage records only (no longer in any
	// fallback chain). VERIFY before relying on them; 3-flash-preview's figure was a placeholder.
	"gemini-3-flash-preview": {{geminiPrice: geminiPrice{1.50, 1.50, 9.00}}},
	"gemini-2.5-pro":         {{geminiPrice: geminiPrice{1.25, 1.25, 10.00}}},
}

// CACHED-INPUT RATES (fetched 2026-11-05, rendered in a headless browser): the same Google Cloud
// pricing page's "Price (/1M tokens) Cached input" column, global endpoint: gemini-4-argon $0.10
// (through 2026-11-30) then $0.20; gemini-3.8-flash and gemini-3.6-flash $0.075 (through
// 2026-12-31) then $0.15; gemini-3.1-pro-preview $0.20 (<=200K prompts). Two caveats, both
// UNVERIFIED: (1) that page is Google Cloud's; the Gemini Developer API page this app is billed
// under (ai.google.dev) is blocked from these sessions, so identical rates there are assumed, as
// for the input/output rates above; (2) the page does not say whether the column also covers
// IMPLICIT (automatic prefix) caching, which is what this app relies on, or only explicit caches.
// Models without a listed cached rate keep cachedInput == input, so cost is never understated.
//
// Deliberately unpriced: gemini-pro-latest (an alias; served as gemini-3.1-pro-preview, Phase 0.3)
// and anything unrecognized. Their calls record CostUnknown=true rather than a guessed figure.
//
// Grounding with Google Search is billed per search query, not per token: 5,000 queries/month
// free across all Gemini 3 and 4 models, then $14 per 1,000 (same source). It depends on the
// monthly total, so it is NOT folded into CostUSD; UsageRecord.GroundingQueries records the count.

// nowFunc is time.Now, swappable in tests.
var nowFunc = time.Now

// priceAt returns the price in effect for model at t.
func priceAt(model string, t time.Time) (geminiPrice, bool) {
	for _, p := range geminiPriceSchedule[model] {
		if p.Until.IsZero() || t.Before(p.Until) {
			return p.geminiPrice, true
		}
	}
	return geminiPrice{}, false
}

// computeCostUSD prices one call at the rate in effect now. It returns 0 for an unknown/openllm
// model rather than throwing -- a signal to add the model to the schedule, not "this call was
// free" (see isPriceKnown / UsageRecord.CostUnknown). Thinking tokens bill at the output rate.
//
// inputTokens is the full prompt size INCLUDING cachedTokens (Gemini's PromptTokenCount semantics);
// cachedTokens is clamped to [0, inputTokens] and billed at the cached rate, the rest at the input rate.
func computeCostUSD(model string, inputTokens, cachedTokens, outputTokens, thinkingTokens int32) float64 {
	return computeCostUSDAt(model, nowFunc(), inputTokens, cachedTokens, outputTokens, thinkingTokens)
}

func computeCostUSDAt(model string, at time.Time, inputTokens, cachedTokens, outputTokens, thinkingTokens int32) float64 {
	price, ok := priceAt(model, at)
	if !ok {
		return 0
	}
	const perM = 1_000_000.0
	if cachedTokens < 0 {
		cachedTokens = 0
	}
	if cachedTokens > inputTokens {
		cachedTokens = inputTokens
	}
	billedOutput := float64(outputTokens) + float64(thinkingTokens)
	return float64(inputTokens-cachedTokens)*price.input/perM +
		float64(cachedTokens)*price.cachedInput/perM +
		billedOutput*price.output/perM
}

// isPriceKnown reports whether model has a price schedule.
func isPriceKnown(model string) bool {
	_, ok := geminiPriceSchedule[model]
	return ok
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
	if obs := runObserverFrom(ctx); obs != nil {
		obs.OnAttempt(agentRole, model, latencyMs, err)
	}
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
		rec.CachedInputTokens = resp.UsageMetadata.CachedContentTokenCount
		rec.ThinkingTokens = resp.UsageMetadata.ThoughtsTokenCount
		rec.CostUSD = computeCostUSD(model, rec.InputTokens, rec.CachedInputTokens, rec.OutputTokens, rec.ThinkingTokens)
	}
	rec.ServedModel = responseServedModel(resp)
	rec.FinishReason = responseFinishReason(resp)
	_, rec.GroundingQueries = responseGrounding(resp)
	rec.CostUnknown = !isPriceKnown(model)
	o.Usage.appendCall(NewCallRecord(agentRole, model, latencyMs, resp, err))
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
