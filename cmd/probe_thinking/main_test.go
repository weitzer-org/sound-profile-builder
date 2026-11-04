package main

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/genai"
)

func resp(served string, fr genai.FinishReason, text string, think, out int32, grounded bool) *genai.GenerateContentResponse {
	c := &genai.Candidate{FinishReason: fr, Content: &genai.Content{Parts: []*genai.Part{{Text: text}}}}
	if grounded {
		c.GroundingMetadata = &genai.GroundingMetadata{WebSearchQueries: []string{"go version"}}
	}
	return &genai.GenerateContentResponse{ModelVersion: served, Candidates: []*genai.Candidate{c},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{ThoughtsTokenCount: think, CandidatesTokenCount: out}}
}

func zero() *int32 { v := int32(0); return &v }

func TestGridCount_FullSixModelGrid(t *testing.T) {
	grid := BuildGrid(defaultModels, []string{"schematools", "thinking", "maxtokens"}, true)
	g, m := CountCalls(grid)
	// per model: schematools(1) + thinking(3) + maxtokens(1) = 5 guaranteed; +2 follow-ups max.
	if g != 30 || m != 42 {
		t.Fatalf("guaranteed=%d max=%d, want 30/42", g, m)
	}
}

func TestEvaluate_ServedModelMismatchNeverPasses(t *testing.T) {
	c := Cell{Model: "gemini-4-argon", Check: "schematools"}
	r := Evaluate(c, "gemini-4-argon", resp("gemini-2.5-pro", genai.FinishReasonStop, `{"answer":"1.26","source":"go.dev"}`, 0, 5, true), nil)
	if r.Pass || !strings.Contains(r.Verdict, "NOT scored") {
		t.Fatalf("fallback served model must not pass: %+v", r)
	}
}

func TestEvaluate_SchemaTools(t *testing.T) {
	c := Cell{Model: "m", Check: "schematools"}
	good := `{"answer":"1.26","source":"go.dev"}`
	if r := Evaluate(c, "m", resp("m", genai.FinishReasonStop, good, 0, 5, true), nil); !r.Pass {
		t.Fatalf("want pass: %+v", r)
	}
	if r := Evaluate(c, "m", resp("m", genai.FinishReasonStop, good, 0, 5, false), nil); r.Pass || !strings.HasPrefix(r.Verdict, "PARTIAL") {
		t.Fatalf("schema ok but no grounding must be PARTIAL, not pass: %+v", r)
	}
	if r := Evaluate(c, "m", resp("m", genai.FinishReasonStop, "not json", 0, 5, true), nil); r.Pass {
		t.Fatalf("bad JSON must fail: %+v", r)
	}
	r := Evaluate(c, "m", nil, errors.New("400 tool use with a response mime type"))
	if r.Pass || r.Error == "" {
		t.Fatalf("API error must fail and keep raw text: %+v", r)
	}
}

func TestEvaluate_Thinking(t *testing.T) {
	c0 := Cell{Model: "m", Check: "thinking", Budget: zero()}
	if r := Evaluate(c0, "m", resp("m", genai.FinishReasonStop, "25", 0, 2, false), nil); !r.Pass {
		t.Fatalf("budget 0 with 0 thinking tokens should pass: %+v", r)
	}
	if r := Evaluate(c0, "m", resp("m", genai.FinishReasonStop, "25", 90, 2, false), nil); r.Pass {
		t.Fatalf("budget 0 but thinking tokens used must fail: %+v", r)
	}
	b := int32(512)
	if r := Evaluate(Cell{Model: "m", Check: "thinking", Budget: &b}, "m", resp("m", genai.FinishReasonStop, "25", 900, 2, false), nil); r.Pass {
		t.Fatalf("exceeding budget must fail: %+v", r)
	}
}

func TestEvaluate_MaxTokens(t *testing.T) {
	c := Cell{Model: "m", Check: "maxtokens"}
	if r := Evaluate(c, "m", resp("m", genai.FinishReasonMaxTokens, "x", 0, 64, false), nil); !r.Pass {
		t.Fatalf("MAX_TOKENS within cap should pass: %+v", r)
	}
	if r := Evaluate(c, "m", resp("m", genai.FinishReasonStop, "long", 0, 900, false), nil); r.Pass {
		t.Fatalf("ignored cap must fail: %+v", r)
	}
}

func TestBuildGrid_FollowupsRunLast(t *testing.T) {
	grid := BuildGrid([]string{"a", "b"}, []string{"schematools", "thinking"}, true)
	seenFollow := false
	for _, c := range grid {
		if c.Followup {
			seenFollow = true
		} else if seenFollow {
			t.Fatalf("non-follow-up cell %s/%s after a follow-up", c.Model, c.Label())
		}
	}
}

func TestGridCount_ProposedPhase05(t *testing.T) {
	// pro-latest resolved to gemini-3.1-pro-preview (0.3), so it is not probed separately.
	models := []string{"gemini-4-argon", "gemini-3.8-flash", "gemini-3.5-flash-lite", "gemini-3.1-pro-preview", "gemini-3.5-flash"}
	g, m := CountCalls(BuildGrid(models, []string{"schematools", "thinking", "maxtokens"}, true))
	if g != 25 || m != 35 {
		t.Fatalf("guaranteed=%d max=%d, want 25/35", g, m)
	}
}
