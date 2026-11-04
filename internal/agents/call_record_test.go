package agents

import (
	"errors"
	"reflect"
	"testing"

	"google.golang.org/genai"
)

// fallbackChainCases is the full expected routing table (EVAL_NO_FALLBACK unset). The first
// four rows are the existing production chains and must not change; the next four are the
// explicit chains for models that can now be a primary; the last two are unknown models.
var fallbackChainCases = map[string][]string{
	// existing production chains -- unchanged
	"gemini-3.1-pro-preview": {"gemini-3.5-flash", "gemini-3-flash-preview", "gemini-2.5-pro"},
	"gemini-3.5-flash":       {"gemini-3-flash-preview", "gemini-2.5-pro"},
	"gemini-3-flash-preview": {"gemini-2.5-pro"},
	"gemini-2.5-pro":         nil,
	// candidate primaries
	"gemini-4-argon":        {"gemini-3.1-pro-preview", "gemini-3.5-flash", "gemini-3-flash-preview"},
	"gemini-3.8-flash":      {"gemini-3.5-flash", "gemini-3-flash-preview"},
	"gemini-3.5-flash-lite": {"gemini-3.5-flash", "gemini-3-flash-preview"},
	"gemini-3.6-flash":      {"gemini-3-flash-preview"},
	// unknown: no fallback, never a guessed model
	"totally-bogus-model": nil,
	"":                    nil,
}

func TestGetFallbackChain_Table(t *testing.T) {
	for _, v := range []string{"", "false", "1", "yes"} { // anything but "true" is production behavior
		t.Setenv(EvalNoFallbackEnv, v)
		for model, want := range fallbackChainCases {
			if got := getFallbackChain(model); !reflect.DeepEqual(got, want) {
				t.Errorf("EVAL_NO_FALLBACK=%q: getFallbackChain(%q) = %v, want %v", v, model, got, want)
			}
		}
	}
}

func TestGetFallbackChain_EvalNoFallback(t *testing.T) {
	for _, v := range []string{"true", "TRUE", " true "} {
		t.Setenv(EvalNoFallbackEnv, v)
		for model := range fallbackChainCases {
			if got := getFallbackChain(model); got != nil {
				t.Errorf("EVAL_NO_FALLBACK=%q: getFallbackChain(%q) = %v, want nil", v, model, got)
			}
		}
	}
}

// The unknown-model default must never name a model, and no candidate primary may be
// slipped into a production chain as a fallback target (that decision belongs to the eval).
func TestGetFallbackChain_Invariants(t *testing.T) {
	t.Setenv(EvalNoFallbackEnv, "")
	for _, m := range []string{"totally-bogus-model", "", "gemini-5-unreleased", "GEMINI-3.5-FLASH"} {
		if got := getFallbackChain(m); len(got) != 0 {
			t.Errorf("unknown model %q must have no fallback, got %v", m, got)
		}
	}
	candidates := []string{"gemini-4-argon", "gemini-3.8-flash", "gemini-3.5-flash-lite", "gemini-3.6-flash"}
	production := []string{"gemini-3.1-pro-preview", "gemini-3.5-flash", "gemini-3-flash-preview", "gemini-2.5-pro"}
	for _, p := range production {
		for _, target := range getFallbackChain(p) {
			for _, c := range candidates {
				if target == c {
					t.Errorf("production chain for %q must not target candidate %q", p, c)
				}
			}
		}
	}
	for _, c := range candidates {
		for _, target := range getFallbackChain(c) {
			if target == "gemini-2.5-pro" {
				t.Errorf("candidate %q chain must not contain %q: %v", c, target, getFallbackChain(c))
				continue
			}
			// Every target must be a production-chain model (also rules out self-reference,
			// since candidates and production are disjoint).
			isProd := false
			for _, p := range production {
				if target == p {
					isProd = true
					break
				}
			}
			if !isProd {
				t.Errorf("candidate %q chain contains non-production target %q", c, target)
			}
		}
	}
}

func TestNewCallRecord_CapturesServedModelAndFinishReason(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		ModelVersion: "gemini-3.5-flash-05-2026",
		Candidates:   []*genai.Candidate{{FinishReason: genai.FinishReasonMaxTokens}},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount: 10, CandidatesTokenCount: 20, ThoughtsTokenCount: 5,
		},
	}
	rec := NewCallRecord("Acoustician", "gemini-3.5-flash", 123, resp, errors.New("truncated"))
	if rec.ServedModel != "gemini-3.5-flash-05-2026" || rec.FinishReason != "MAX_TOKENS" || rec.Success || rec.Error == "" {
		t.Fatalf("unexpected record: %+v", rec)
	}
	if rec.InputTokens != 10 || rec.OutputTokens != 20 || rec.ThinkingTokens != 5 {
		t.Fatalf("tokens not captured: %+v", rec)
	}
	// nil response (hard API error) must not panic and leaves served model empty.
	rec = NewCallRecord("Acoustician", "gemini-4-argon", 5, nil, errors.New("400"))
	if rec.ServedModel != "" || rec.FinishReason != "" || rec.Success {
		t.Fatalf("unexpected record for nil resp: %+v", rec)
	}
}

func TestRecordAttemptUsage_AppendsCallRecordAndFlagsUnpricedModel(t *testing.T) {
	o := &Orchestrator{Usage: &TokenUsage{ModelsUsed: map[string]int{}}}
	resp := &genai.GenerateContentResponse{
		ModelVersion: "gemini-4-argon",
		Candidates:   []*genai.Candidate{{FinishReason: genai.FinishReasonStop}},
	}
	recordAttemptUsage(t.Context(), o, "Tone Historian", "gemini-4-argon", 10, resp, nil)
	calls := o.Usage.CallsSnapshot()
	if len(calls) != 1 || calls[0].ServedModel != "gemini-4-argon" || calls[0].FinishReason != "STOP" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestIsPriceKnown_CandidatesAreUnpricedNotFree(t *testing.T) {
	for _, m := range []string{"gemini-4-argon", "gemini-3.8-flash", "gemini-pro-latest", "gemini-3.5-flash-lite"} {
		if isPriceKnown(m) {
			t.Errorf("%s must stay unpriced until an official price is added", m)
		}
	}
	for _, m := range []string{"gemini-3.1-pro-preview", "gemini-3.5-flash"} {
		if !isPriceKnown(m) {
			t.Errorf("%s should be priced", m)
		}
	}
	rollup := AggregateUsage("2026-11-04", []UsageRecord{
		{Provider: "gemini", CallType: "x", Model: "gemini-4-argon", Success: true, CostUnknown: true},
		{Provider: "gemini", CallType: "x", Model: "gemini-3.5-flash", Success: true, CostUSD: 1},
	})
	if rollup.UnpricedCalls != 1 || rollup.ByModel["gemini-4-argon"].UnpricedCalls != 1 {
		t.Fatalf("rollup did not surface unpriced calls: %+v", rollup)
	}
}

func TestServedModelMatches(t *testing.T) {
	yes := [][2]string{
		{"gemini-3.5-flash", "gemini-3.5-flash"},
		{"gemini-3.5-flash", "models/gemini-3.5-flash"},
		{"gemini-3.5-flash", "gemini-3.5-flash-05-2026"},
		{"gemini-2.5-pro", "gemini-2.5-pro-001"},
	}
	no := [][2]string{
		{"gemini-3.5-flash", "gemini-3.5-flash-lite"}, // different variant, not a build suffix
		{"gemini-3.5-flash", "gemini-3.6-flash"},
		{"gemini-3.1-pro-preview", "gemini-3.5-flash"},
		{"gemini-4-argon", "gemini-2.5-pro"},
		{"gemini-pro-latest", "gemini-3.1-pro-preview"}, // alias: pin the resolved id instead
		{"gemini-3.5-flash", ""},
		{"", "gemini-3.5-flash"},
	}
	for _, c := range yes {
		if !ServedModelMatches(c[0], c[1]) {
			t.Errorf("ServedModelMatches(%q,%q) = false, want true", c[0], c[1])
		}
	}
	for _, c := range no {
		if ServedModelMatches(c[0], c[1]) {
			t.Errorf("ServedModelMatches(%q,%q) = true, want false", c[0], c[1])
		}
	}
}

func TestNewCallRecord_NoCandidatesIsFailure(t *testing.T) {
	rec := NewCallRecord("Preset Critic", "gemini-3.5-flash", 5, &genai.GenerateContentResponse{ModelVersion: "gemini-3.6-flash"}, nil)
	if rec.Success || rec.Error == "" {
		t.Fatalf("a response with no candidates must not read as success: %+v", rec)
	}
}
