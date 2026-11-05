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
	lastArchitectRequestBody = architectBody
	return requestPromptText(t, architectBody)
}

// lastArchitectRequestBody is the raw body of the last Architect request
// architectRequestForPipeline captured (system prompt, contents and generation config).
var lastArchitectRequestBody string

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
	if !strings.Contains(lastArchitectRequestBody, "kept_defaults") {
		t.Error("v5 Architect request schema should carry kept_defaults")
	}

	for _, cfg := range []map[string]string{{"4_coros_librarian": "v2", "12_architect": "v4"}, nil} {
		if got := architectRequestForPipeline(t, cfg); strings.Contains(got, "Device Block Parameters") || strings.Contains(got, "Device Effect Blocks") {
			t.Errorf("Architect %v context must be unchanged (no catalog sections)", cfg)
		}
		if strings.Contains(lastArchitectRequestBody, "kept_defaults") {
			t.Errorf("Architect %v request must not carry the kept_defaults schema field", cfg)
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
	for v, want := range map[string]bool{"v5": true, "v4": false, "v3": false} {
		prompt, err := LoadPrompt("12_architect", v)
		if err != nil {
			t.Fatalf("load %s: %v", v, err)
		}
		if got := promptUsesCatalogParams(prompt); got != want {
			t.Errorf("promptUsesCatalogParams(12_architect %s) = %v, want %v", v, got, want)
		}
	}
}

// The kept_defaults field is only in the schema of a catalog-aware prompt, where it is
// required; older prompts keep exactly their previous schema.
func TestArchitectSchemaKeptDefaults(t *testing.T) {
	for _, schema := range []map[string]any{architectGenerationJSONSchema(false), architectRefineJSONSchema(false)} {
		if b, _ := json.Marshal(schema); strings.Contains(string(b), "kept_defaults") {
			t.Error("a schema without keptDefaults must not mention kept_defaults")
		}
	}
	block := effectBlockJSONSchema(true)
	if _, ok := block["properties"].(map[string]any)["kept_defaults"]; !ok {
		t.Fatal("kept_defaults property missing")
	}
	req := block["required"].([]string)
	if req[len(req)-1] != "kept_defaults" {
		t.Errorf("kept_defaults should be required, got %v", req)
	}
	if base := effectBlockJSONSchema(false)["required"].([]string); len(base) != len(req)-1 {
		t.Errorf("adding kept_defaults changed the base schema's required list: %v", base)
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

// v5 is the prompt that explains the catalog sections; v4 stays as it was.
func TestArchitectV5PromptDescribesCatalogSections(t *testing.T) {
	v5, err := LoadPrompt("12_architect", "v5")
	if err != nil {
		t.Fatalf("load v5: %v", err)
	}
	for _, want := range []string{"# Device Block Parameters", "Device Effect Blocks", "MIC 1 LEVEL", "20*log10"} {
		if !strings.Contains(v5, want) {
			t.Errorf("v5 prompt is missing %q", want)
		}
	}
	if strings.Contains(v5, "750Hz Slider: -5.0 dB") || strings.Contains(v5, "Use this rarely and honestly") {
		t.Error("v5 still carries v4's EQ-summary or rare-confirmed_range instruction")
	}
	v4, err := LoadPrompt("12_architect", "v4")
	if err != nil {
		t.Fatalf("load v4: %v", err)
	}
	if strings.Contains(v4, "Device Block Parameters") {
		t.Error("v4 must stay unchanged")
	}
}
