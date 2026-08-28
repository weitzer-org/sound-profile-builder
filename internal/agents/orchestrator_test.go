package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// mockGeminiResponse is the expected shape of Google's GenAI API output
var mockGeminiResponse = `{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "{\"final_html_payload\": {\"Gibson ES-339 Humbuckers\": \"<p>test</p>\"}, \"agent_impact\": [\"done\"]}"
          }
        ],
        "role": "model"
      }
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 5,
    "candidatesTokenCount": 10,
    "totalTokenCount": 15
  }
}`

func TestFilterEmbeddedCorosMap(t *testing.T) {
	raw := []byte(`{
		"Native Thing": {"coros_equivalent": "Native Thing", "type": "amp"},
		"Real Capture": {"coros_equivalent": "Real Capture", "type": "amp", "is_capture": true},
		"Plini Clean": {"coros_equivalent": "Plini Clean", "type": "amp", "required_plugin": "Plini"}
	}`)

	unmarshal := func(t *testing.T, s string) map[string]map[string]interface{} {
		t.Helper()
		var m map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			t.Fatalf("filterEmbeddedCorosMap returned invalid JSON: %v\n%s", err, s)
		}
		return m
	}

	t.Run("no filters", func(t *testing.T) {
		got := unmarshal(t, filterEmbeddedCorosMap(raw, true, false, nil))
		for _, name := range []string{"Native Thing", "Real Capture", "Plini Clean"} {
			if _, ok := got[name]; !ok {
				t.Errorf("expected %q present with no filters active", name)
			}
		}
	})

	t.Run("factory captures disallowed", func(t *testing.T) {
		got := unmarshal(t, filterEmbeddedCorosMap(raw, false, false, nil))
		if _, ok := got["Real Capture"]; ok {
			t.Errorf("expected is_capture entry dropped when allowFactoryCaptures=false")
		}
		if _, ok := got["Native Thing"]; !ok {
			t.Errorf("expected non-capture entry to survive allowFactoryCaptures=false")
		}
		if _, ok := got["Plini Clean"]; !ok {
			t.Errorf("plugin filter is off; required_plugin entry should not be affected by the capture toggle")
		}
	})

	t.Run("plugin not owned", func(t *testing.T) {
		got := unmarshal(t, filterEmbeddedCorosMap(raw, true, true, map[string]bool{}))
		if _, ok := got["Plini Clean"]; ok {
			t.Errorf("expected required_plugin entry dropped when its plugin isn't in allowedPlugins")
		}
		if _, ok := got["Native Thing"]; !ok {
			t.Errorf("expected plain entry with no required_plugin to survive plugin filtering")
		}
	})

	t.Run("plugin owned", func(t *testing.T) {
		got := unmarshal(t, filterEmbeddedCorosMap(raw, true, true, map[string]bool{"Plini": true}))
		if _, ok := got["Plini Clean"]; !ok {
			t.Errorf("expected required_plugin entry to survive when its plugin is in allowedPlugins")
		}
	})
}

func TestOrchestrator_RunPipeline_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, err := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))
	if err != nil {
		t.Fatalf("Failed to init orchestrator: %v", err)
	}
	defer orch.Close()

	constraints := map[string]interface{}{
		"single_amp_mode":      true,
		"allow_cloud_captures": false,
	}

	res, usage, err := orch.RunPipeline(ctx, "test prompt", constraints, nil, nil)
	if err != nil {
		t.Fatalf("Expected pipeline to succeed, got %v", err)
	}

	if !strings.Contains(res, "final_html_payload") {
		t.Errorf("Unexpected pipeline output: %s", res)
	}

	if usage.InputTokens <= 0 || usage.OutputTokens <= 0 {
		t.Errorf("Expected token tracking to have accrued: %+v", usage)
	}
}

// TestOrchestrator_RunPipeline_JohnMayerPluginAvailable is the real end-to-end proof that
// owning the "John Mayer" plugin (config.json's available_plugins, exactly as server.go
// forwards it into RunPipeline's constraints) actually reaches the outgoing LLM request --
// not just that the filtering helpers return the right thing in isolation, but that the
// full RunPipeline call graph wires ownership through to what the CorOS Librarian agent
// (internal/agents/prompts/4_coros_librarian_v2.md) is told it may pick from. Also proves
// the negative: a plugin NOT in available_plugins (Gojira/Plini here, standing in for
// every plugin the user hasn't purchased) never reaches that request at all, which is the
// actual bug this was written to catch -- prior to the required_plugin gating, every
// plugin's devices were present regardless of ownership.
//
// A browser/Playwright e2e run can't observe this: the dashboard has no UI that renders
// available_plugins (only the blanket "Allow Paid Plugins" checkbox), and ?mock=true
// short-circuits RunPipeline before it ever builds the Dictionary or calls Gemini. This is
// the meaningful "prove it reaches production behavior" test for this feature.
func TestOrchestrator_RunPipeline_JohnMayerPluginAvailable(t *testing.T) {
	var mu sync.Mutex
	var requestBodies []string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requestBodies = append(requestBodies, string(body))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, err := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))
	if err != nil {
		t.Fatalf("Failed to init orchestrator: %v", err)
	}
	defer orch.Close()

	// Mirrors exactly what internal/api/server.go builds from config.json for a real
	// generate request (see the "constraints := map[string]interface{}{...}" block around
	// server.go:230), not a synthetic minimal set -- allow_paid_plugins: true and
	// available_plugins holding the two plugins actually owned today.
	constraints := map[string]interface{}{
		"guitars":                []string{"Gibson ES-339 Humbuckers", "Fender Telecaster Single Coil"},
		"single_amp_mode":        true,
		"allow_cloud_captures":   false,
		"allow_factory_captures": true,
		"allow_user_captures":    true,
		"favor_captures":         false,
		"favor_cloud_captures":   false,
		"allow_paid_plugins":     true,
		"available_plugins":      []string{"Cory Wong", "John Mayer"},
	}

	if _, _, err := orch.RunPipeline(ctx, "test prompt", constraints, nil, nil); err != nil {
		t.Fatalf("Expected pipeline to succeed, got %v", err)
	}

	mu.Lock()
	combined := strings.Join(requestBodies, "\n---\n")
	mu.Unlock()

	// Positive: an owned plugin's device must reach the Dictionary text sent to the LLM.
	if !strings.Contains(combined, "John Mayer Smooth Operator") {
		t.Errorf("expected an owned John Mayer plugin device in the outgoing request(s), got none. Bodies:\n%s", combined)
	}

	// Negative: a device belonging to a plugin NOT in available_plugins must never appear.
	for _, unowned := range []string{"Gojira CLN", "Plini Clean", "Nolly Clean"} {
		if strings.Contains(combined, unowned) {
			t.Errorf("expected unowned plugin device %q to be filtered out of every outgoing request, but it was present", unowned)
		}
	}
}

func TestOrchestrator_RefineChat_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))

	p := &storage.Preset{
		Payload: "<matrix></matrix>",
		ChatHistory: []storage.ChatMessage{
			{Role: "user", Content: "hello"},
			{Role: "model", Content: "hi"},
		},
	}

	res, usage, err := orch.RefineChat(ctx, p, "change it", true, true)
	if err != nil {
		t.Fatalf("Expected RefineChat to succeed, got %v", err)
	}
	if !strings.Contains(res, "final_html_payload") {
		t.Errorf("Unexpected refine output: %s", res)
	}
	if usage.OutputTokens <= 0 {
		t.Errorf("Usage wasn't tracked properly")
	}
}

// TestOrchestrator_RefineChat_InjectsSelectedCaptureContext confirms RefineChat surfaces
// a selected capture's real descriptive color from p.Payload (the only data it has --
// unlike RunPipeline, it never re-runs Librarian/Navigator), and that the opposite
// allow-capture flag suppresses it, mirroring RunPipeline's existing leak-prevention rule.
func TestOrchestrator_RefineChat_InjectsSelectedCaptureContext(t *testing.T) {
	var mu sync.Mutex
	var lastRequestBody string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		lastRequestBody = string(body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))

	// A known factory capture from coros_map.json (is_capture: true, real tonal_archetype).
	// getFactoryCaptureColors keys on coros_equivalent (the resolved name agents actually
	// emit), not the JSON entry's own map key.
	const captureName = "US DLX 58"
	const captureColor = "Other / Unique"
	p := &storage.Preset{
		Payload: fmt.Sprintf(`{"guitars":{"Strat":[{"id":"block-1","type":"Amplifier","model":%q}]}}`, captureName),
	}

	promptText := func() string {
		t.Helper()
		mu.Lock()
		body := lastRequestBody
		mu.Unlock()

		var reqBody struct {
			Contents []struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"contents"`
		}
		if err := json.Unmarshal([]byte(body), &reqBody); err != nil {
			t.Fatalf("Failed to decode outgoing LLM request body: %v", err)
		}
		if len(reqBody.Contents) == 0 || len(reqBody.Contents[0].Parts) == 0 {
			t.Fatalf("Outgoing LLM request had no prompt text: %s", body)
		}
		return reqBody.Contents[0].Parts[0].Text
	}

	_, _, err := orch.RefineChat(ctx, p, "change it", true, true)
	if err != nil {
		t.Fatalf("Expected RefineChat to succeed, got %v", err)
	}
	if !strings.Contains(promptText(), captureColor) {
		t.Errorf("Expected outgoing prompt to carry the selected capture's descriptive color %q, got: %s", captureColor, promptText())
	}

	// allowFactoryCaptures=false must suppress the capture context entirely, same leak
	// guard SelectedCaptureContext already enforces for RunPipeline.
	_, _, err = orch.RefineChat(ctx, p, "change it", false, true)
	if err != nil {
		t.Fatalf("Expected RefineChat to succeed, got %v", err)
	}
	if strings.Contains(promptText(), captureColor) {
		t.Errorf("Expected capture context to be suppressed when allowFactoryCaptures=false, got: %s", promptText())
	}
}

func TestOrchestrator_TimeoutsAndErrors(t *testing.T) {
	ctx := context.Background()

	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint("http://127.0.0.1:0"))

	// Fast timeout ensures RunAgent errors immediately
	ctxTimeout, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()

	_, _, err := orch.RunPipeline(ctxTimeout, "test", nil, nil, nil)
	if err == nil {
		t.Errorf("Expected pipeline to fail on timeout")
	}

	_, _, err = orch.RefineChat(ctxTimeout, &storage.Preset{}, "test", true, true)
	if err == nil {
		t.Errorf("Expected refine chat to fail on timeout")
	}

	_, err = orch.RunAgent(ctxTimeout, "Agent", "prompt")
	if err == nil {
		t.Errorf("Expected agent run to fail on timeout")
	}
}

func TestOrchestrator_EmptyCandidates(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"candidates":[]}`)) // empty!
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))

	_, err := orch.RunAgent(ctx, "Agent", "prompt")
	if err == nil {
		t.Errorf("Expected error from empty candidates array")
	}
}

func TestOrchestrator_NewOrchestrator_Error(t *testing.T) {
	ctx := context.Background()
	_, err := NewOrchestrator(ctx, "key", nil, WithInvalidConfig())
	if err == nil {
		t.Errorf("Expected GenAI client to fail with an invalid client configuration")
	}
}

func TestOrchestrator_RunPipeline_PhaseErrors(t *testing.T) {
	tests := []struct {
		name       string
		failTarget int
	}{
		{"Phase1", 1},
		{"Phase2_Librarian", 4},
		{"Phase2_Navigator", 5},
		{"Phase3", 6},
		{"Phase4", 9},
		{"Architect", 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var count atomic.Int32
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c := count.Add(1)
				if int(c) >= tt.failTarget {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(mockGeminiResponse))
			}))
			defer mockServer.Close()

			ctx := context.Background()
			orch, _ := NewOrchestrator(ctx, "key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))
			defer orch.Close()

			_, _, err := orch.RunPipeline(ctx, "test", map[string]interface{}{"allow_cloud_captures": true}, nil, nil)
			if err == nil {
				t.Errorf("Expected pipeline to fail on target %d", tt.failTarget)
			}
		})
	}
}

func TestOrchestrator_RunPipeline_AblationConfig(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"mock"}]}}]}`))
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))
	defer orch.Close()

	// Turn off Architect (Agent 12)
	config := map[string]string{
		"12_architect": "off",
	}

	res, _, err := orch.RunPipeline(ctx, "test", nil, config, nil)
	if err != nil {
		t.Fatalf("Expected pipeline to succeed even with ablation, got %v", err)
	}

	if res != "Architect skipped by configuration." {
		t.Errorf("Expected skipped message, got %s", res)
	}
}

func TestOrchestrator_RunPipeline_PluginConstraints(t *testing.T) {
	var capturedBody string
	var mu sync.Mutex
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))

		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		capturedBody += string(body)
		mu.Unlock()
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))
	defer orch.Close()

	constraints := map[string]interface{}{
		"allow_paid_plugins": false,
	}

	_, _, err := orch.RunPipeline(ctx, "test prompt", constraints, nil, nil)
	if err != nil {
		t.Fatalf("Pipeline failed: %v", err)
	}

	if !strings.Contains(capturedBody, "NO PAID PLUGINS") {
		t.Errorf("Expected prompt to contain 'NO PAID PLUGINS', got captured body: %s", capturedBody)
	}
}

func TestOrchestrator_RunPipeline_FactoryCaptureConstraints(t *testing.T) {
	var capturedBody string
	var mu sync.Mutex
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))

		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		capturedBody += string(body)
		mu.Unlock()
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))
	defer orch.Close()

	constraints := map[string]interface{}{
		"allow_factory_captures": false,
	}

	_, _, err := orch.RunPipeline(ctx, "test prompt", constraints, nil, nil)
	if err != nil {
		t.Fatalf("Pipeline failed: %v", err)
	}

	if !strings.Contains(capturedBody, "NO FACTORY CAPTURES") {
		t.Errorf("Expected prompt to contain 'NO FACTORY CAPTURES', got captured body: %s", capturedBody)
	}
}

func TestOrchestrator_RunPipeline_UserCaptureConstraints_Disabled(t *testing.T) {
	var capturedBody string
	var mu sync.Mutex
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))

		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		capturedBody += string(body)
		mu.Unlock()
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))
	defer orch.Close()

	constraints := map[string]interface{}{
		"allow_user_captures": false,
	}

	_, _, err := orch.RunPipeline(ctx, "test prompt", constraints, nil, nil)
	if err != nil {
		t.Fatalf("Pipeline failed: %v", err)
	}

	if !strings.Contains(capturedBody, "NO USER CAPTURES") {
		t.Errorf("Expected prompt to contain 'NO USER CAPTURES', got captured body: %s", capturedBody)
	}
	// "ToneJunkie" is a capture source unique to user_captures.json (unlike an amp/pedal
	// name, which can coincidentally also appear in coros_map.json's own dictionary) —
	// it must not leak into the outgoing request when the library is disabled.
	if strings.Contains(capturedBody, "ToneJunkie") {
		t.Errorf("Expected user capture library to be excluded from prompt when disabled, got captured body: %s", capturedBody)
	}
}

func TestOrchestrator_RunPipeline_FavorCloudCaptureConstraints(t *testing.T) {
	var capturedBody string
	var mu sync.Mutex
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))

		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		capturedBody += string(body)
		mu.Unlock()
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))
	defer orch.Close()

	constraints := map[string]interface{}{
		"favor_cloud_captures": true,
	}

	_, _, err := orch.RunPipeline(ctx, "test prompt", constraints, nil, nil)
	if err != nil {
		t.Fatalf("Pipeline failed: %v", err)
	}

	if !strings.Contains(capturedBody, "FAVOR CLOUD CAPTURES") {
		t.Errorf("Expected prompt to contain 'FAVOR CLOUD CAPTURES', got captured body: %s", capturedBody)
	}
}

func TestOrchestrator_RunPipeline_UserCaptureLibraryInjected(t *testing.T) {
	var capturedBody string
	var mu sync.Mutex
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))

		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		capturedBody += string(body)
		mu.Unlock()
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))
	defer orch.Close()

	// allow_cloud_captures: true so Agent 5 (Cloud Navigator) actually runs and also
	// receives the injected library, in addition to Agent 4 (Librarian) which receives
	// it unconditionally.
	constraints := map[string]interface{}{
		"allow_cloud_captures": true,
	}

	_, _, err := orch.RunPipeline(ctx, "test prompt", constraints, nil, nil)
	if err != nil {
		t.Fatalf("Pipeline failed: %v", err)
	}

	if !strings.Contains(capturedBody, "User Capture Library") {
		t.Errorf("Expected Librarian/Navigator prompts to include 'User Capture Library', got captured body: %s", capturedBody)
	}
}

func TestOrchestrator_RunPipeline_FavorCaptureConstraints(t *testing.T) {
	var capturedBody string
	var mu sync.Mutex
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))

		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		capturedBody += string(body)
		mu.Unlock()
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, WithEndpoint(mockServer.URL), WithHTTPClient(mockServer.Client()))
	defer orch.Close()

	constraints := map[string]interface{}{
		"favor_captures": true,
	}

	_, _, err := orch.RunPipeline(ctx, "test prompt", constraints, nil, nil)
	if err != nil {
		t.Fatalf("Pipeline failed: %v", err)
	}

	if !strings.Contains(capturedBody, "FAVOR CAPTURES") {
		t.Errorf("Expected prompt to contain 'FAVOR CAPTURES', got captured body: %s", capturedBody)
	}
}

func TestSubsetQCBlockSchema(t *testing.T) {
	original := embeddedQCBlockSchema
	defer func() { embeddedQCBlockSchema = original }()
	embeddedQCBlockSchema = []byte(`{
		"_readme": {"version": "1.0"},
		"global_eq": {"param": "value"},
		"drive": {"param": "value"}
	}`)

	tests := []struct {
		name           string
		keys           []string
		expectFallback bool
		expectedKeys   []string
	}{
		{
			name:         "valid subset with _readme and a real category",
			keys:         []string{"_readme", "global_eq"},
			expectedKeys: []string{"_readme", "global_eq"},
		},
		{
			name:           "fallback when the only requested category doesn't exist",
			keys:           []string{"_readme", "non_existent_category"},
			expectFallback: true,
		},
		{
			name:           "fallback on partial drift -- one of two requested categories missing",
			keys:           []string{"_readme", "drive", "missing_category"},
			expectFallback: true,
		},
		{
			name:         "empty keys list has nothing to drift, so no fallback",
			keys:         []string{},
			expectedKeys: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := subsetQCBlockSchema(tc.keys)

			if tc.expectFallback {
				if result != string(embeddedQCBlockSchema) {
					t.Errorf("expected fallback to full schema, got: %s", result)
				}
				return
			}

			var parsed map[string]json.RawMessage
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("failed to parse subset JSON: %v", err)
			}
			if len(parsed) != len(tc.expectedKeys) {
				t.Errorf("expected %d keys, got %d (%v)", len(tc.expectedKeys), len(parsed), parsed)
			}
			for _, key := range tc.expectedKeys {
				if _, ok := parsed[key]; !ok {
					t.Errorf("subset missing expected key: %s", key)
				}
			}
		})
	}
}

// TestGetQCSonicProfilerSchemaJSON_Integrity is a contract test against the real, embedded
// production schema: if a category referenced here (global_eq/drive/reverb/noise_gate) ever
// gets renamed in qc_block_schema.json, this fails loudly instead of Sonic Profiler silently
// losing that category from its context with no test to catch it.
func TestGetQCSonicProfilerSchemaJSON_Integrity(t *testing.T) {
	result := GetQCSonicProfilerSchemaJSON()

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse Sonic Profiler schema: %v", err)
	}

	for _, key := range []string{"_readme", "global_eq", "drive", "reverb", "noise_gate"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("production schema is missing a category Sonic Profiler's prompt depends on: %s", key)
		}
	}
}
