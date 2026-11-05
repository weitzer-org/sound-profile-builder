package agents

import (
	"errors"
	"reflect"
	"testing"

	"google.golang.org/genai"
)

func TestGetFallbackChain_DefaultUnchanged(t *testing.T) {
	t.Setenv(EvalNoFallbackEnv, "")
	cases := map[string][]string{
		"gemini-4-argon":         {"gemini-3.8-flash", "gemini-3.1-pro-preview"},
		"gemini-3.8-flash":       {"gemini-4-argon", "gemini-3.1-pro-preview"},
		"gemini-3.1-pro-preview": {"gemini-4-argon", "gemini-3.8-flash"},
		"gemini-3.5-flash":       {"gemini-3.8-flash", "gemini-4-argon"},
		"totally-bogus-model":    {"gemini-3.8-flash", "gemini-4-argon"},
	}
	for model, want := range cases {
		for _, m := range want {
			if m == "gemini-2.5-pro" || m == "gemini-3-flash-preview" || m == "gemini-3.5-flash" {
				t.Errorf("chain for %s ends on deprecated model %s", model, m)
			}
		}
	}
	for model, want := range cases {
		if got := getFallbackChain(model); !reflect.DeepEqual(got, want) {
			t.Errorf("getFallbackChain(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestGetFallbackChain_EvalNoFallback(t *testing.T) {
	for _, v := range []string{"true", "TRUE", " true "} {
		t.Setenv(EvalNoFallbackEnv, v)
		for _, m := range []string{"gemini-3.1-pro-preview", "gemini-3.5-flash", "gemini-4-argon", "totally-bogus-model"} {
			if got := getFallbackChain(m); got != nil {
				t.Errorf("EVAL_NO_FALLBACK=%q: getFallbackChain(%q) = %v, want nil", v, m, got)
			}
		}
	}
	// Only the literal "true" enables it; anything else keeps production behavior.
	for _, v := range []string{"false", "1", "yes", ""} {
		t.Setenv(EvalNoFallbackEnv, v)
		if getFallbackChain("gemini-4-argon") == nil {
			t.Errorf("EVAL_NO_FALLBACK=%q must not disable fallback", v)
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
	// gemini-pro-latest is an alias (served as gemini-3.1-pro-preview); it stays unpriced.
	if isPriceKnown("gemini-pro-latest") {
		t.Error("gemini-pro-latest must stay unpriced (alias)")
	}
	for _, m := range []string{"gemini-3.1-pro-preview", "gemini-3.5-flash", "gemini-4-argon", "gemini-3.8-flash", "gemini-3.5-flash-lite", "gemini-3.6-flash"} {
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

func TestNewCallRecord_Grounded(t *testing.T) {
	mk := func(gm *genai.GroundingMetadata) *genai.GenerateContentResponse {
		return &genai.GenerateContentResponse{ModelVersion: "m", Candidates: []*genai.Candidate{{FinishReason: genai.FinishReasonStop, GroundingMetadata: gm}}}
	}
	cases := []struct {
		name     string
		resp     *genai.GenerateContentResponse
		grounded bool
		queries  int
	}{
		{"nil response", nil, false, 0},
		{"no metadata (tool requested, model did not search)", mk(nil), false, 0},
		{"empty metadata", mk(&genai.GroundingMetadata{}), false, 0},
		{"search queries", mk(&genai.GroundingMetadata{WebSearchQueries: []string{"a", "b"}}), true, 2},
		{"chunks only", mk(&genai.GroundingMetadata{GroundingChunks: []*genai.GroundingChunk{{}}}), true, 0},
	}
	for _, c := range cases {
		rec := NewCallRecord("Tone Historian", "m", 1, c.resp, nil)
		if rec.Grounded != c.grounded || rec.SearchQueries != c.queries {
			t.Errorf("%s: grounded=%v queries=%d, want %v/%d", c.name, rec.Grounded, rec.SearchQueries, c.grounded, c.queries)
		}
	}
}

func TestBuildGenerationConfig_ThinkingLevel(t *testing.T) {
	o := &Orchestrator{}
	if cfg := o.buildGenerationConfig("Tone Historian", "1_tone_historian"); cfg.ThinkingConfig != nil {
		t.Fatalf("production default must not set ThinkingConfig on agents without a budget, got %+v", cfg.ThinkingConfig)
	}
	if cfg := o.buildGenerationConfig("Architect & Evaluator", "12_architect"); cfg.ThinkingConfig == nil || cfg.ThinkingConfig.ThinkingBudget == nil || *cfg.ThinkingConfig.ThinkingBudget != 8192 {
		t.Fatalf("Architect must carry its 8192 thinking budget by default, got %+v", cfg.ThinkingConfig)
	}
	o.ThinkingLevel = genai.ThinkingLevelHigh
	cfg := o.buildGenerationConfig("Architect & Evaluator", "12_architect")
	if cfg.ThinkingConfig == nil || cfg.ThinkingConfig.ThinkingLevel != genai.ThinkingLevelHigh || cfg.ThinkingConfig.ThinkingBudget != nil {
		t.Fatalf("ThinkingLevel not applied: %+v", cfg.ThinkingConfig)
	}
}
