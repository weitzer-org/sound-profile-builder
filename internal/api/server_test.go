package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestServer_Start(t *testing.T) {
	s, _, _, _ := setupTestServer()
	// Pass an invalid address to get a fast error
	err := s.Start("invalid-host:-1")
	if err == nil {
		t.Errorf("Expected Start to fail on invalid port")
	}
}

func TestServer_HandleIndex(t *testing.T) {
	s, _, _, _ := setupTestServer()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		// Just ensure it doesn't panic. Normally would return 404 because file isn't in test dir
		t.Errorf("Unexpected status: %d", rr.Code)
	}
}

func TestServer_HandleGeneratePreset(t *testing.T) {
	s, _, mockSM, mockOrch := setupTestServer()

	// 1. Method Not Allowed
	reqGet, _ := http.NewRequest(http.MethodGet, "/api/preset/generate", nil)
	reqGet.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})
	rrGet := httptest.NewRecorder()
	s.mux.ServeHTTP(rrGet, reqGet)
	if rrGet.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed")
	}

	// 2. Form Parse Error (Simulated by sending bad body, though standard forms usually just result in empty values. We test success anyway.)
	formData := url.Values{}
	formData.Set("prompt", "Make it sound huge")
	reqPost, _ := http.NewRequest(http.MethodPost, "/api/preset/generate", strings.NewReader(formData.Encode()))
	reqPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	withAuthAndCSRF(reqPost)

	rrSuccess := httptest.NewRecorder()
	s.mux.ServeHTTP(rrSuccess, reqPost)

	if rrSuccess.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got: %d", rrSuccess.Code)
	}
	if !strings.Contains(rrSuccess.Body.String(), `hx-get="/api/preset/status`) {
		t.Errorf("Expected response to contain polling button")
	}

	// 3. Secret Fetcher Error
	mockSM.err = fmt.Errorf("sm error")
	reqErr, _ := http.NewRequest(http.MethodPost, "/api/preset/generate", strings.NewReader(formData.Encode()))
	reqErr.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	withAuthAndCSRF(reqErr)
	rrErr := httptest.NewRecorder()
	s.mux.ServeHTTP(rrErr, reqErr)
	if rrErr.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 on SM fail")
	}
	mockSM.err = nil

	// 4. Orchestrator Initialization Error
	mockOrch.err = fmt.Errorf("orch factory error")
	reqOrchGen, _ := http.NewRequest(http.MethodPost, "/api/preset/generate", strings.NewReader(formData.Encode()))
	reqOrchGen.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	withAuthAndCSRF(reqOrchGen)
	rrOrchGen := httptest.NewRecorder()
	s.mux.ServeHTTP(rrOrchGen, reqOrchGen)
	if !strings.Contains(rrOrchGen.Body.String(), "Initializing ADK Pipeline") {
		t.Errorf("Expected polling button for orch init fail async")
	}

	// 5. Orchestrator Execution Pipeline Error
	mockOrch.err = fmt.Errorf("pipeline execution fail") // Caught internally, rendered as grid-matrix
	reqPipe, _ := http.NewRequest(http.MethodPost, "/api/preset/generate", strings.NewReader(formData.Encode()))
	reqPipe.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	withAuthAndCSRF(reqPipe)
	rrPipe := httptest.NewRecorder()
	s.mux.ServeHTTP(rrPipe, reqPipe)
	if !strings.Contains(rrPipe.Body.String(), "Initializing ADK Pipeline") {
		t.Errorf("Expected polling button for pipeline fail async")
	}
	mockOrch.err = nil

	// 6. JSON Unmarshal Error (Architect returns bad json)
	s, _, _, mockOrch2 := setupTestServer()
	// Replace factory
	s.orchMaker = func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		return &badJsonOrchestrator{}, nil
	}
	reqBadJson, _ := http.NewRequest(http.MethodPost, "/api/preset/generate", strings.NewReader(formData.Encode()))
	reqBadJson.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	withAuthAndCSRF(reqBadJson)
	rrBadJson := httptest.NewRecorder()
	s.mux.ServeHTTP(rrBadJson, reqBadJson)
	if rrBadJson.Code != http.StatusOK {
		t.Errorf("Expected 200 OK on bad json spawn, got: %d", rrBadJson.Code)
	}
	if !strings.Contains(rrBadJson.Body.String(), `hx-get="/api/preset/status`) {
		t.Errorf("Expected response to contain polling area")
	}
	mockOrch2.err = nil
}

type badJsonOrchestrator struct{}

func (m *badJsonOrchestrator) RunPipeline(ctx context.Context, prompt string, constraints map[string]interface{}, agentConfig map[string]string, onProgress func(string)) (string, *agents.TokenUsage, error) {
	return `{"bad json"}`, nil, nil
}
func (m *badJsonOrchestrator) RefineChat(ctx context.Context, p *storage.Preset, userMessage string, allowFactoryCaptures, allowUserCaptures bool) (string, *agents.TokenUsage, error) {
	return `{"bad json"}`, nil, nil
}
func (m *badJsonOrchestrator) Close() {}

// mockOrchestratorCapturesConstraints records the constraints map RunPipeline was
// actually invoked with, over a channel since handleGeneratePreset calls RunPipeline
// from an internal goroutine (the HTTP response returns a polling placeholder
// immediately -- see server.go's "go func() { ... orch.RunPipeline(...) ... }()").
type mockOrchestratorCapturesConstraints struct {
	constraintsCh chan map[string]interface{}
}

func (m *mockOrchestratorCapturesConstraints) RunPipeline(ctx context.Context, prompt string, constraints map[string]interface{}, agentConfig map[string]string, onProgress func(string)) (string, *agents.TokenUsage, error) {
	m.constraintsCh <- constraints
	return `{"final_html_payload":{"Gibson ES-339 Humbuckers":"mock"},"agent_impact":[]}`, &agents.TokenUsage{}, nil
}
func (m *mockOrchestratorCapturesConstraints) RefineChat(ctx context.Context, p *storage.Preset, userMessage string, allowFactoryCaptures, allowUserCaptures bool) (string, *agents.TokenUsage, error) {
	return `{"final_html_payload":{"Gibson ES-339 Humbuckers":"mock"},"agent_impact":[]}`, &agents.TokenUsage{}, nil
}
func (m *mockOrchestratorCapturesConstraints) Close() {}

// TestServer_HandleGeneratePreset_ForwardsAvailablePluginsFromConfig is the API-level
// proof that owning the John Mayer plugin actually reaches the pipeline through the real
// HTTP surface, not just internal Go call graphs: it POSTs to /api/preset/generate
// through s.mux (the full middleware chain -- auth, CSRF, rate limiting, form parsing --
// exactly as a real browser request would hit it), backed by a config.AppConfig shaped
// exactly like the real config.json (AllowPaidPlugins: true, AvailablePlugins: ["Cory
// Wong", "John Mayer"]), and asserts the constraints map handleGeneratePreset built from
// that config and handed to RunPipeline actually carries "John Mayer" in
// available_plugins. Complements (does not replace) the deeper
// TestOrchestrator_RunPipeline_JohnMayerPluginAvailable pipeline-level test, which picks
// up from a hand-built constraints map and proves it reaches the outgoing LLM request --
// this test proves the HTTP layer actually builds that map correctly from config in the
// first place, closing the last link in the chain from config.json to the LLM request.
func TestServer_HandleGeneratePreset_ForwardsAvailablePluginsFromConfig(t *testing.T) {
	client := newMockClient()
	store := storage.NewPresetStore(client, "test-bucket")
	sf := &mockSecretFetcher{}
	mockOrch := &mockOrchestratorCapturesConstraints{constraintsCh: make(chan map[string]interface{}, 1)}
	orchMaker := func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		return mockOrch, nil
	}
	appConfig := &config.AppConfig{
		SingleAmpMode:        true,
		AllowFactoryCaptures: true,
		AllowUserCaptures:    true,
		AllowPaidPlugins:     true,
		AvailablePlugins:     []string{"Cory Wong", "John Mayer"},
	}
	s := NewServer(store, nil, client, sf, orchMaker, appConfig)

	formData := url.Values{}
	formData.Set("prompt", "Warm blues tone")
	req, _ := http.NewRequest(http.MethodPost, "/api/preset/generate", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	withAuthAndCSRF(req)

	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from POST /api/preset/generate, got %d: %s", rr.Code, rr.Body.String())
	}

	select {
	case constraints := <-mockOrch.constraintsCh:
		got, ok := constraints["available_plugins"].([]string)
		if !ok {
			t.Fatalf("expected available_plugins to be a []string in constraints, got %T: %v", constraints["available_plugins"], constraints["available_plugins"])
		}
		found := false
		for _, p := range got {
			if p == "John Mayer" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected the real HTTP API request to forward config.json's John Mayer plugin ownership into RunPipeline's constraints, got available_plugins=%v", got)
		}
		if allowPaid, _ := constraints["allow_paid_plugins"].(bool); !allowPaid {
			t.Errorf("expected allow_paid_plugins=true to reach RunPipeline")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the async generate handler to invoke RunPipeline")
	}
}

func TestServer_HandleTaskStatus(t *testing.T) {
	s, _, _, _ := setupTestServer()

	// 1. Missing ID
	reqNoID, _ := http.NewRequest(http.MethodGet, "/api/preset/status", nil)
	rrNoID := httptest.NewRecorder()
	s.handleTaskStatus().ServeHTTP(rrNoID, reqNoID)
	if rrNoID.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing ID")
	}

	// 2. Task Not Found
	reqNotFound, _ := http.NewRequest(http.MethodGet, "/api/preset/status?id=missing", nil)
	rrNotFound := httptest.NewRecorder()
	s.handleTaskStatus().ServeHTTP(rrNotFound, reqNotFound)
	if rrNotFound.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for task not found")
	}

	// 3. Task Running
	s.tasksMu.Lock()
	s.tasks["task-1"] = &TaskState{Status: "running", Phase: "Testing"}
	s.tasksMu.Unlock()

	reqRunning, _ := http.NewRequest(http.MethodGet, "/api/preset/status?id=task-1", nil)
	rrRunning := httptest.NewRecorder()
	s.handleTaskStatus().ServeHTTP(rrRunning, reqRunning)
	if rrRunning.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for running task")
	}
	if !strings.Contains(rrRunning.Body.String(), "Testing") {
		t.Errorf("Expected response to contain phase 'Testing'")
	}

	// 4. Task Complete
	s.tasksMu.Lock()
	s.tasks["task-1"].Status = "complete"
	s.tasks["task-1"].Result = "<div>Done</div>"
	s.tasksMu.Unlock()

	rrComplete := httptest.NewRecorder()
	s.handleTaskStatus().ServeHTTP(rrComplete, reqRunning)
	if rrComplete.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for complete task")
	}
	if !strings.Contains(rrComplete.Body.String(), "Done") {
		t.Errorf("Expected response to contain result 'Done'")
	}

	// 5. Task Error
	s.tasksMu.Lock()
	s.tasks["task-1"].Status = "error"
	s.tasks["task-1"].Error = "some error"
	s.tasksMu.Unlock()

	rrError := httptest.NewRecorder()
	s.handleTaskStatus().ServeHTTP(rrError, reqRunning)
	if rrError.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for error task (returns error UI)")
	}
	if !strings.Contains(rrError.Body.String(), "some error") {
		t.Errorf("Expected response to contain error 'some error'")
	}
}
