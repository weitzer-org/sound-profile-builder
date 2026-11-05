package agents

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// geminiTextResponse wraps text in the GenAI response shape the mock servers return.
func geminiTextResponse(t *testing.T, text string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"candidates": []any{map[string]any{"content": map[string]any{
			"role": "model", "parts": []any{map[string]any{"text": text}},
		}}},
		"usageMetadata": map[string]any{"promptTokenCount": 5, "candidatesTokenCount": 10, "totalTokenCount": 15},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// requestPromptText is the user-turn text of an outgoing GenAI request body.
func requestPromptText(t *testing.T, body string) string {
	t.Helper()
	var req struct {
		Contents []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
	}
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	var parts []string
	for _, c := range req.Contents {
		for _, p := range c.Parts {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// architectRequestForPipeline runs RunPipeline against a mock Gemini whose Librarian picks
// fixed blocks, and returns the Architect's outgoing prompt text.
func architectRequestForPipeline(t *testing.T, agentConfig map[string]string) string {
	t.Helper()
	librarianJSON := `{"mandatory_blocks": ["Green 808", "Made Up Amp 9000"], "suggested_blocks": ["Spring"], "tactical_hints": [], "dropped_gear": []}`
	var mu sync.Mutex
	var architectBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s := string(body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(s, "Evaluate the impact of the pipeline"):
			mu.Lock()
			architectBody = s
			mu.Unlock()
			w.Write([]byte(mockGeminiResponse))
		case strings.Contains(s, "Prefer User Captures"): // the Librarian's system prompt
			w.Write(geminiTextResponse(t, librarianJSON))
		default:
			w.Write([]byte(mockGeminiResponse))
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	orch, err := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	constraints := map[string]interface{}{
		"guitars": []string{"Strat"}, "allow_paid_plugins": true, "available_plugins": []string{"John Mayer"},
	}
	if _, _, err := orch.RunPipeline(ctx, "test prompt", constraints, agentConfig, nil); err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if architectBody == "" {
		t.Fatal("no Architect request was captured")
	}
	return requestPromptText(t, architectBody)
}

func TestRunPipelineInjectsCatalogParamsForV5Only(t *testing.T) {
	v5 := architectRequestForPipeline(t, map[string]string{"4_coros_librarian": "v2", "12_architect": "v5"})
	for _, want := range []string{
		"Device Block Parameters",
		"Green 808 [Guitar Overdrive]: OVERDRIVE 0..10",
		"Spring [Reverb]: MIX 0..100%",
		"Device Effect Blocks",
		"John Mayer Spring Reverb", // owned plugin effect
	} {
		if !strings.Contains(v5, want) {
			t.Errorf("v5 Architect context is missing %q", want)
		}
	}
	for _, unwanted := range []string{"Made Up Amp 9000 [", "Petrucci Wah"} {
		if strings.Contains(v5, unwanted) {
			t.Errorf("v5 Architect context should not contain %q", unwanted)
		}
	}

	for _, cfg := range []map[string]string{{"4_coros_librarian": "v2", "12_architect": "v4"}, nil} {
		if got := architectRequestForPipeline(t, cfg); strings.Contains(got, "Device Block Parameters") || strings.Contains(got, "Device Effect Blocks") {
			t.Errorf("Architect %v context must be unchanged (no catalog sections)", cfg)
		}
	}
}

func TestRefineChatUsesConfiguredArchitectVersionAndPayloadParams(t *testing.T) {
	var mu sync.Mutex
	var lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		lastBody = string(body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockGeminiResponse))
	}))
	defer srv.Close()

	ctx := context.Background()
	orch, err := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	sp := storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"Strat": {
		{ID: "b1", Type: "Drive", Model: "Green 808"},
		{ID: "b2", Type: "Cabinet", Model: "412 Brit 60B V30 95"},
	}}}
	payload, _ := json.Marshal(map[string]any{"structured": sp, "legacy_html": map[string]string{"Strat": "<table></table>"}})
	p := &storage.Preset{Payload: string(payload)}

	refine := func(ctx context.Context) string {
		t.Helper()
		if _, _, err := orch.RefineChat(ctx, p, "brighter", true, true); err != nil {
			t.Fatalf("RefineChat: %v", err)
		}
		mu.Lock()
		defer mu.Unlock()
		return requestPromptText(t, lastBody)
	}

	v5 := refine(WithPromptVersions(ctx, map[string]string{"12_architect": "v5"}))
	for _, want := range []string{"Green 808 [Guitar Overdrive]: OVERDRIVE", "412 Brit 60B V30 95 [Cabsim", "MIC 1 LEVEL", "Device Effect Blocks", "Spring"} {
		if !strings.Contains(v5, want) {
			t.Errorf("v5 refine context is missing %q", want)
		}
	}
	// RefineChat doesn't know which plugins the user owns, so it offers no plugin effects.
	if strings.Contains(v5, "John Mayer Spring Reverb") {
		t.Error("refine's effects list should offer no plugin effects")
	}

	for name, c := range map[string]context.Context{
		"no versions": ctx,
		"v4":          WithPromptVersions(ctx, map[string]string{"12_architect": "v4"}),
	} {
		if got := refine(c); strings.Contains(got, "Device Block Parameters") {
			t.Errorf("%s: refine context must not carry the catalog sections", name)
		}
	}
}

func TestRefineArchitectVersion(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		ctx  context.Context
		want string
	}{
		{ctx, "v4"},
		{WithPromptVersions(ctx, map[string]string{"12_architect": "v5"}), "v5"},
		{WithPromptVersions(ctx, map[string]string{"12_architect": "off"}), "v4"},
		{WithPromptVersions(ctx, map[string]string{"4_coros_librarian": "v2"}), "v4"},
	} {
		if got := refineArchitectVersion(tc.ctx); got != tc.want {
			t.Errorf("refineArchitectVersion = %q, want %q", got, tc.want)
		}
	}
	for v, want := range map[string]bool{"v5": true, "v12": true, "V6": true, "v4": false, "v1": false, "": false, "off": false, "v5_flash": false} {
		if got := architectUsesCatalogParams(v); got != want {
			t.Errorf("architectUsesCatalogParams(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestPayloadBlockModels(t *testing.T) {
	bare := `{"guitars":{"B":[{"model":"Spring"}],"A":[{"model":"Green 808"}]}}`
	if got := strings.Join(payloadBlockModels(bare), "|"); got != "Green 808|Spring" {
		t.Errorf("bare payload = %q", got)
	}
	if got := payloadBlockModels("<table>Room Gain Volume</table>"); len(got) != 0 {
		t.Errorf("legacy HTML payload should yield no names, got %v", got)
	}
}
