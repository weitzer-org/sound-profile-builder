package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
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
	reqPost.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})

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
	reqErr.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})
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
	reqOrchGen.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})
	rrOrchGen := httptest.NewRecorder()
	s.mux.ServeHTTP(rrOrchGen, reqOrchGen)
	if !strings.Contains(rrOrchGen.Body.String(), "Initializing ADK Pipeline") {
		t.Errorf("Expected polling button for orch init fail async")
	}
	
	// 5. Orchestrator Execution Pipeline Error
	mockOrch.err = fmt.Errorf("pipeline execution fail") // Caught internally, rendered as grid-matrix
	reqPipe, _ := http.NewRequest(http.MethodPost, "/api/preset/generate", strings.NewReader(formData.Encode()))
	reqPipe.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqPipe.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})
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
	reqBadJson.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})
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
func (m *badJsonOrchestrator) RunPipeline(ctx context.Context, prompt string, constraints map[string]interface{}, onProgress func(string)) (string, *agents.TokenUsage, error) {
	return `{"bad json"}`, nil, nil
}
func (m *badJsonOrchestrator) RefineChat(ctx context.Context, p *storage.Preset, userMessage string) (string, *agents.TokenUsage, error) {
	return `{"bad json"}`, nil, nil
}
func (m *badJsonOrchestrator) Close() {}
