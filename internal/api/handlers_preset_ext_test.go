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

// mockErrorClient wraps mockClient but fails on specific objects/operations
type mockErrorClient struct {
	*mockClient
	failList   bool
	failRead   bool
	failWrite  bool
	failDelete bool
}

func (m *mockErrorClient) ReadFile(ctx context.Context, bucket, object string) ([]byte, error) {
	if m.failRead {
		return nil, fmt.Errorf("mock read error")
	}
	return m.mockClient.ReadFile(ctx, bucket, object)
}

func (m *mockErrorClient) WriteFile(ctx context.Context, bucket, object string, data []byte) error {
	if m.failWrite {
		return fmt.Errorf("mock write error")
	}
	return m.mockClient.WriteFile(ctx, bucket, object, data)
}

func (m *mockErrorClient) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	if m.failList {
		return nil, fmt.Errorf("mock list error")
	}
	return m.mockClient.ListFiles(ctx, bucket, prefix)
}

func (m *mockErrorClient) DeleteFile(ctx context.Context, bucket, object string) error {
	if m.failDelete {
		return fmt.Errorf("mock delete error")
	}
	return m.mockClient.DeleteFile(ctx, bucket, object)
}

func TestHandleGetPresets_Errors(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient(), failList: true}
	store := storage.NewPresetStore(mockStorage, "b")
	s := NewServer(store, nil, mockStorage, &mockSecretFetcher{}, nil, nil)

	req, _ := http.NewRequest(http.MethodGet, "/api/presets", nil)
	rr := httptest.NewRecorder()
	s.handleGetPresets().ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 error code for list fail")
	}
}

func TestHandleSavePreset_Errors(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient(), failWrite: true}
	store := storage.NewPresetStore(mockStorage, "b")
	s := NewServer(store, nil, mockStorage, &mockSecretFetcher{}, nil, nil)

	// Form parse error
	reqParse, _ := http.NewRequest(http.MethodPost, "/api/preset/save", strings.NewReader("%%%"))
	reqParse.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrParse := httptest.NewRecorder()
	s.handleSavePreset().ServeHTTP(rrParse, reqParse)
	if rrParse.Code != http.StatusBadRequest {
		t.Errorf("Expected 400")
	}

	// Write error
	formData := url.Values{}
	formData.Set("preset_name", "Test Name")
	formData.Set("payload", "<span>Cool Tone</span>")
	reqFail, _ := http.NewRequest(http.MethodPost, "/api/preset/save", strings.NewReader(formData.Encode()))
	reqFail.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrFail := httptest.NewRecorder()
	s.handleSavePreset().ServeHTTP(rrFail, reqFail)
	if rrFail.Code != http.StatusInternalServerError {
		t.Errorf("Expected Save Error 500")
	}
}

func TestHandleDeletePreset_Errors(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	s := NewServer(store, nil, mockStorage, &mockSecretFetcher{}, nil, nil)

	// Save dummy
	store.Save(context.Background(), &storage.Preset{ID: "testing_id", Name: "name", Payload: "none"})

	// Internal Delete failure
	mockStorage.failDelete = true
	formData := url.Values{}
	formData.Set("id", "testing_id")
	reqFail, _ := http.NewRequest(http.MethodPost, "/api/preset/delete", strings.NewReader(formData.Encode()))
	reqFail.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrFail := httptest.NewRecorder()
	s.handleDeletePreset().ServeHTTP(rrFail, reqFail)
	if rrFail.Code != http.StatusInternalServerError {
		t.Errorf("Expected Delete Error 500")
	}

	// Internal List failure
	mockStorage.failDelete = false
	mockStorage.failList = true
	rrFailList := httptest.NewRecorder()
	reqFailList, _ := http.NewRequest(http.MethodPost, "/api/preset/delete", strings.NewReader(formData.Encode()))
	reqFailList.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleDeletePreset().ServeHTTP(rrFailList, reqFailList)
	if !strings.Contains(rrFailList.Body.String(), "No presets saved yet") {
		t.Errorf("Expected empty body for failed deletion reload")
	}
}

func TestHandleCopyPreset(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	s := NewServer(store, nil, mockStorage, &mockSecretFetcher{}, nil, nil)

	formData := url.Values{}
	formData.Set("id", "123")
	reqSrc, _ := http.NewRequest(http.MethodPost, "/api/preset/copy", strings.NewReader(formData.Encode()))
	reqSrc.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Missing ID Get error
	mockStorage.failRead = true
	rrMissing := httptest.NewRecorder()
	s.handleCopyPreset().ServeHTTP(rrMissing, reqSrc)
	if !strings.Contains(rrMissing.Body.String(), "Preset not found") {
		t.Errorf("Expected empty response on copy missing")
	}
	mockStorage.failRead = false

	// Valid Copy
	store.Save(context.Background(), &storage.Preset{ID: "123", Name: "name", Payload: "none"})
	rrCopy := httptest.NewRecorder()
	reqCopy, _ := http.NewRequest(http.MethodPost, "/api/preset/copy", strings.NewReader(formData.Encode()))
	reqCopy.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleCopyPreset().ServeHTTP(rrCopy, reqCopy)
	if rrCopy.Code != http.StatusOK {
		t.Errorf("Expected success copying text")
	}



	// Write fails on copy wrapper
	mockStorage.failList = false
	mockStorage.failWrite = true
	rrWriteFail := httptest.NewRecorder()
	reqWriteFail, _ := http.NewRequest(http.MethodPost, "/api/preset/copy", strings.NewReader(formData.Encode()))
	reqWriteFail.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleCopyPreset().ServeHTTP(rrWriteFail, reqWriteFail)
	if rrWriteFail.Code != http.StatusInternalServerError {
		t.Errorf("Expected duplication write failure 500")
	}
}

func TestHandleViewPreset(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	s := NewServer(store, nil, mockStorage, &mockSecretFetcher{}, nil, nil)

	// Missing id
	reqMissing, _ := http.NewRequest(http.MethodGet, "/api/preset/view", nil)
	rrMissing := httptest.NewRecorder()
	s.handleViewPreset().ServeHTTP(rrMissing, reqMissing)
	if rrMissing.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 bad request without ID")
	}

	// Get error
	reqLookup, _ := http.NewRequest(http.MethodGet, "/api/preset/view?id=123", nil)
	rrLookup := httptest.NewRecorder()
	s.handleViewPreset().ServeHTTP(rrLookup, reqLookup)
	if !strings.Contains(rrLookup.Body.String(), "Lookup Error") {
		t.Errorf("Expected lookup error in DOM")
	}

	// Valid View
	store.Save(context.Background(), &storage.Preset{ID: "123", Name: "name", Payload: "none", ChatHistory: []storage.ChatMessage{{Role:"user", Content:"hey"}}})
	reqView, _ := http.NewRequest(http.MethodGet, "/api/preset/view?id=123", nil)
	rrView := httptest.NewRecorder()
	s.handleViewPreset().ServeHTTP(rrView, reqView)
	if rrView.Code != http.StatusOK {
		t.Errorf("Expected valid preset name text")
	}
}

func TestHandleChatPreset(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	orchMaker := func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		return &mockOrchestrator{}, nil
	}
	s := NewServer(store, nil, mockStorage, &mockSecretFetcher{}, orchMaker, nil)

	formData := url.Values{}
	formData.Set("id", "123")
	formData.Set("message", "Test Chat")
	
	reqNoID, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(""))
	reqNoID.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrNoID := httptest.NewRecorder()
	s.handleChatPreset().ServeHTTP(rrNoID, reqNoID)
	if !strings.Contains(rrNoID.Body.String(), "Missing ID or message") {
		t.Errorf("Expected ID miss")
	}

	reqNoMsg, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(url.Values{"id": {"123"}}.Encode()))
	reqNoMsg.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrNoMsg := httptest.NewRecorder()
	s.handleChatPreset().ServeHTTP(rrNoMsg, reqNoMsg)
	if !strings.Contains(rrNoMsg.Body.String(), "Missing ID or message") {
		t.Errorf("Expected Msg miss")
	}

	reqMissing, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(formData.Encode()))
	reqMissing.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrMissing := httptest.NewRecorder()
	s.handleChatPreset().ServeHTTP(rrMissing, reqMissing)
	if !strings.Contains(rrMissing.Body.String(), "Lookup Error") {
		t.Errorf("Expected Preset Not found")
	}

	// Secret manager fail
	sc := &mockSecretFetcher{err: fmt.Errorf("auth-err")}
	s.smFetcher = sc
	store.Save(context.Background(), &storage.Preset{ID: "123", Name: "name", Payload: "none", ChatHistory: []storage.ChatMessage{{Role:"user", Content:"hey"}}})
	rrAuth := httptest.NewRecorder()
	reqAuth, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(formData.Encode()))
	reqAuth.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleChatPreset().ServeHTTP(rrAuth, reqAuth)
	if !strings.Contains(rrAuth.Body.String(), "Auth Error") {
		t.Errorf("Expected Auth Error")
	}
	sc.err = nil

	// Orch creation fail
	s.orchMaker = func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		return nil, fmt.Errorf("orch fail")
	}
	rrOrch := httptest.NewRecorder()
	reqOrch, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(formData.Encode()))
	reqOrch.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleChatPreset().ServeHTTP(rrOrch, reqOrch)
	if !strings.Contains(rrOrch.Body.String(), "ADK Error") {
		t.Errorf("Expected ADK Error")
	}
	
	// Valid Orch
	s.orchMaker = func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		return &mockOrchestrator{}, nil
	}
	
	// Run valid Chat
	rrValid := httptest.NewRecorder()
	reqValid, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(formData.Encode()))
	reqValid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleChatPreset().ServeHTTP(rrValid, reqValid)
	if rrValid.Code != http.StatusOK {
		t.Errorf("Expected valid chat")
	}

	// Failed JSON decode in architect payload
	s.orchMaker = func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		return &badJsonOrchestrator{}, nil
	}
	rrBad := httptest.NewRecorder()
	reqBad, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(formData.Encode()))
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleChatPreset().ServeHTTP(rrBad, reqBad)
	if !strings.Contains(rrBad.Body.String(), "Serialization Error") {
		t.Errorf("Expected Refinement Json Error")
	}

	// Refinement fail inside ADK
	s.orchMaker = func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		return &mockOrchestrator{err: fmt.Errorf("ADK Failure Runtime")}, nil
	}
	rrRunF := httptest.NewRecorder()
	reqRunF, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(formData.Encode()))
	reqRunF.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleChatPreset().ServeHTTP(rrRunF, reqRunF)
	if !strings.Contains(rrRunF.Body.String(), "Execution Error") {
		t.Errorf("Expected Execution Error fail mode")
	}
}

func TestHandleCopyPresetUI(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	s := NewServer(store, nil, mockStorage, &mockSecretFetcher{}, nil, nil)

	// Missing ID
	req, _ := http.NewRequest(http.MethodGet, "/api/preset/copy_ui", nil)
	rr := httptest.NewRecorder()
	s.handleCopyPresetUI().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing ID")
	}

	// Lookup error
	reqLookup, _ := http.NewRequest(http.MethodGet, "/api/preset/copy_ui?id=missing", nil)
	rrLookup := httptest.NewRecorder()
	s.handleCopyPresetUI().ServeHTTP(rrLookup, reqLookup)
	if !strings.Contains(rrLookup.Body.String(), "Lookup Error") {
		t.Errorf("Expected Lookup Error DOM")
	}

	// Success
	store.Save(context.Background(), &storage.Preset{ID: "exists_id", Name: "Name", Payload: "none"})
	reqSuccess, _ := http.NewRequest(http.MethodGet, "/api/preset/copy_ui?id=exists_id", nil)
	rrSuccess := httptest.NewRecorder()
	s.handleCopyPresetUI().ServeHTTP(rrSuccess, reqSuccess)
	if rrSuccess.Code != http.StatusOK {
		t.Errorf("Expected 200 for valid ID")
	}
}

func TestCleanUpOldDrafts(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")

	// Store errors out
	mockStorage.failList = true
	res := cleanUpOldDrafts(context.Background(), store)
	if len(res) != 0 {
		t.Errorf("Expected empty result on store list fail")
	}
	mockStorage.failList = false

	// Success limits
	store.Save(context.Background(), &storage.Preset{ID: "1", Name: "Draft Preset", UpdatedAt: "2026-03-31T00:00:00Z"})
	store.Save(context.Background(), &storage.Preset{ID: "2", Name: "Draft Preset", UpdatedAt: "2026-03-31T01:00:00Z"})
	store.Save(context.Background(), &storage.Preset{ID: "3", Name: "Draft Preset", UpdatedAt: "2026-03-31T02:00:00Z"})
	store.Save(context.Background(), &storage.Preset{ID: "4", Name: "Draft Preset", UpdatedAt: "2026-03-31T03:00:00Z"})

	resOk := cleanUpOldDrafts(context.Background(), store)
	if len(resOk) != 3 {
		t.Errorf("Expected 3 drafts returned, got %d", len(resOk))
	}
}
