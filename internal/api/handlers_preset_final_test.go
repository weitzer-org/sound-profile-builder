package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestHandlers_FillCoverageGaps(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	orchMaker := func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		return &mockOrchestrator{}, nil
	}
	s := NewServer(store, mockStorage, &mockSecretFetcher{}, orchMaker)

	// Save Preset: empty name
	formEmptyName := url.Values{}
	formEmptyName.Set("preset_name", "")
	reqSn, _ := http.NewRequest(http.MethodPost, "/api/preset/save", strings.NewReader(formEmptyName.Encode()))
	reqSn.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrSn := httptest.NewRecorder()
	s.handleSavePreset().ServeHTTP(rrSn, reqSn)
	if rrSn.Code != http.StatusOK {
		t.Errorf("expected OK request for missing save name, but got %d", rrSn.Code)
	}

	// Save Preset: mock list failure after saving
	mockStorage.failList = true
	formValid := url.Values{}
	formValid.Set("preset_name", "ok")
	formValid.Set("payload", "ok")
	reqSaveListFail, _ := http.NewRequest(http.MethodPost, "/api/preset/save", strings.NewReader(formValid.Encode()))
	reqSaveListFail.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrSaveListFail := httptest.NewRecorder()
	s.handleSavePreset().ServeHTTP(rrSaveListFail, reqSaveListFail)
	mockStorage.failList = false

	// Delete Preset: empty id
	formEmptyId := url.Values{}
	formEmptyId.Set("id", "")
	reqDi, _ := http.NewRequest(http.MethodPost, "/api/preset/delete", strings.NewReader(formEmptyId.Encode()))
	reqDi.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrDi := httptest.NewRecorder()
	s.handleDeletePreset().ServeHTTP(rrDi, reqDi)
	if rrDi.Code != http.StatusBadRequest {
		t.Errorf("expected bad request for missing delete id")
	}

	// Copy Preset: empty id
	reqCi, _ := http.NewRequest(http.MethodPost, "/api/preset/copy", strings.NewReader(formEmptyId.Encode()))
	reqCi.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrCi := httptest.NewRecorder()
	s.handleCopyPreset().ServeHTTP(rrCi, reqCi)
	if rrCi.Code != http.StatusNotFound {
		t.Errorf("expected Not Found request for missing copy id, got %d", rrCi.Code)
	}

	// Chat Preset: simulate successful agent impact block output logic
	store.Save(context.Background(), &storage.Preset{ID: "testing_adk", Name: "name", Payload: "none"})
	goodOrchMaker := func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		// Mock a response that triggers DSPMatrixUpdated and AgentImpact inclusion
		return &mockOrchestratorSuccessComplex{}, nil
	}
	s2 := NewServer(store, mockStorage, &mockSecretFetcher{}, goodOrchMaker)
	formChat := url.Values{}
	formChat.Set("id", "testing_adk")
	formChat.Set("message", "do something")
	reqChatSuccess, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(formChat.Encode()))
	reqChatSuccess.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrChatSuccess := httptest.NewRecorder()
	s2.handleChatPreset().ServeHTTP(rrChatSuccess, reqChatSuccess)

	// Simulate GenerateParse form failure by providing bad content type string
	reqGenParseFail, _ := http.NewRequest(http.MethodPost, "/api/preset/generate", strings.NewReader("bad%bad"))
	// Not setting content-type application/x-www-form... usually triggers ParseForm failure nicely. Wait, it might just ignore.
	// In Go, sending a malformed query with a specific length triggers ParseForm fail
	reqGenParseFail.URL.RawQuery = "%;"
	rrGenParseFail := httptest.NewRecorder()
	s.mux.ServeHTTP(rrGenParseFail, reqGenParseFail)
	
	// Server Parse Form failures for view, delete, copy, chat, save
	badReq, _ := http.NewRequest(http.MethodPost, "/api/preset/save", strings.NewReader(""))
	badReq.URL.RawQuery = "%;"
	rrBadReq := httptest.NewRecorder()
	s.handleSavePreset().ServeHTTP(rrBadReq, badReq)

	badReq2, _ := http.NewRequest(http.MethodPost, "/api/preset/delete", strings.NewReader(""))
	badReq2.URL.RawQuery = "%;"
	rrBadReq2 := httptest.NewRecorder()
	s.handleDeletePreset().ServeHTTP(rrBadReq2, badReq2)

	badReq3, _ := http.NewRequest(http.MethodPost, "/api/preset/copy", strings.NewReader(""))
	badReq3.URL.RawQuery = "%;"
	rrBadReq3 := httptest.NewRecorder()
	s.handleCopyPreset().ServeHTTP(rrBadReq3, badReq3)
	
	badReq4, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(""))
	badReq4.URL.RawQuery = "%;"
	rrBadReq4 := httptest.NewRecorder()
	s.handleChatPreset().ServeHTTP(rrBadReq4, badReq4)
}

type mockOrchestratorSuccessComplex struct {
	err error
}
func (m *mockOrchestratorSuccessComplex) RunPipeline(ctx context.Context, prompt string, constraints map[string]interface{}) (string, *agents.TokenUsage, error) {
	return `{"final_html_payload":{"Gibson ES-339 Humbuckers":"mock"},"agent_impact":["changed eq"], "dsp_matrix_updated": true}`, &agents.TokenUsage{}, nil
}
func (m *mockOrchestratorSuccessComplex) RefineChat(ctx context.Context, p *storage.Preset, userMessage string) (string, *agents.TokenUsage, error) {
	return `{"conversational_response": "done", "final_html_payload":{"Gibson ES-339 Humbuckers":"mock"},"agent_impact":["changed eq"], "dsp_matrix_updated": true}`, &agents.TokenUsage{}, nil
}
func (m *mockOrchestratorSuccessComplex) Close() {}

func TestHandleRenamePreset(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	s := NewServer(store, mockStorage, &mockSecretFetcher{}, nil)

	// Save dummy
	store.Save(context.Background(), &storage.Preset{ID: "testing_id_rename", Name: "name", Payload: "none"})

	// Parse form error
	badReq, _ := http.NewRequest(http.MethodPost, "/api/preset/rename", strings.NewReader(""))
	badReq.URL.RawQuery = "%;"
	rrBadReq := httptest.NewRecorder()
	s.handleRenamePreset().ServeHTTP(rrBadReq, badReq)
	if rrBadReq.Code != http.StatusBadRequest {
		t.Errorf("Expected ParseForm error")
	}

	// Missing ID or name
	formData := url.Values{}
	formData.Set("id", "testing_id_rename") // no name
	reqMiss, _ := http.NewRequest(http.MethodPost, "/api/preset/rename", strings.NewReader(formData.Encode()))
	reqMiss.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrMiss := httptest.NewRecorder()
	s.handleRenamePreset().ServeHTTP(rrMiss, reqMiss)
	if rrMiss.Code != http.StatusBadRequest {
		t.Errorf("Expected missing name error")
	}

	// Valid Rename via FormValue
	formDataValid := url.Values{}
	formDataValid.Set("id", "testing_id_rename")
	formDataValid.Set("preset_name", "new_name_form")
	reqValid, _ := http.NewRequest(http.MethodPost, "/api/preset/rename", strings.NewReader(formDataValid.Encode()))
	reqValid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrValid := httptest.NewRecorder()
	s.handleRenamePreset().ServeHTTP(rrValid, reqValid)
	if rrValid.Code != http.StatusOK {
		t.Errorf("Expected OK for valid rename")
	}

	// Valid Rename via HX-Prompt
	formDataPrompt := url.Values{}
	formDataPrompt.Set("id", "testing_id_rename")
	reqPrompt, _ := http.NewRequest(http.MethodPost, "/api/preset/rename", strings.NewReader(formDataPrompt.Encode()))
	reqPrompt.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqPrompt.Header.Set("HX-Prompt", "new_name_prompt")
	rrPrompt := httptest.NewRecorder()
	s.handleRenamePreset().ServeHTTP(rrPrompt, reqPrompt)
	if rrPrompt.Code != http.StatusOK {
		t.Errorf("Expected OK for valid prompt rename")
	}

	// Preset not found
	formDataMissing := url.Values{}
	formDataMissing.Set("id", "nonexistent")
	formDataMissing.Set("preset_name", "new_name")
	reqNotFound, _ := http.NewRequest(http.MethodPost, "/api/preset/rename", strings.NewReader(formDataMissing.Encode()))
	reqNotFound.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrNotFound := httptest.NewRecorder()
	s.handleRenamePreset().ServeHTTP(rrNotFound, reqNotFound)
	if rrNotFound.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for missing preset")
	}

	// Save error
	mockStorage.failWrite = true
	reqWriteFail, _ := http.NewRequest(http.MethodPost, "/api/preset/rename", strings.NewReader(formDataValid.Encode()))
	reqWriteFail.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrWriteFail := httptest.NewRecorder()
	s.handleRenamePreset().ServeHTTP(rrWriteFail, reqWriteFail)
	if rrWriteFail.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for write error")
	}
	mockStorage.failWrite = false
}

func TestHandleDeleteDraftPreset(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	s := NewServer(store, mockStorage, &mockSecretFetcher{}, nil)

	// Save dummy
	store.Save(context.Background(), &storage.Preset{ID: "testing_id_draft", Name: "name", Payload: "none"})

	// Parse form error
	badReq, _ := http.NewRequest(http.MethodPost, "/api/preset/delete_draft", strings.NewReader(""))
	badReq.URL.RawQuery = "%;"
	rrBadReq := httptest.NewRecorder()
	s.handleDeleteDraftPreset().ServeHTTP(rrBadReq, badReq)
	if rrBadReq.Code != http.StatusBadRequest {
		t.Errorf("Expected ParseForm error")
	}

	// Missing ID
	reqMiss, _ := http.NewRequest(http.MethodPost, "/api/preset/delete_draft", strings.NewReader(url.Values{}.Encode()))
	reqMiss.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrMiss := httptest.NewRecorder()
	s.handleDeleteDraftPreset().ServeHTTP(rrMiss, reqMiss)
	if rrMiss.Code != http.StatusBadRequest {
		t.Errorf("Expected missing ID error")
	}

	// Delete error
	mockStorage.failDelete = true
	formDataValid := url.Values{}
	formDataValid.Set("id", "testing_id_draft")
	reqDelFail, _ := http.NewRequest(http.MethodPost, "/api/preset/delete_draft", strings.NewReader(formDataValid.Encode()))
	reqDelFail.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrDelFail := httptest.NewRecorder()
	s.handleDeleteDraftPreset().ServeHTTP(rrDelFail, reqDelFail)
	if rrDelFail.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for delete error")
	}
	mockStorage.failDelete = false

	// Success
	reqOk, _ := http.NewRequest(http.MethodPost, "/api/preset/delete_draft", strings.NewReader(formDataValid.Encode()))
	reqOk.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrOk := httptest.NewRecorder()
	s.handleDeleteDraftPreset().ServeHTTP(rrOk, reqOk)
	if rrOk.Code != http.StatusOK {
		t.Errorf("Expected 200 for successful draft deletion")
	}
	if rrOk.Header().Get("HX-Redirect") != "/" {
		t.Errorf("Expected HX-Redirect to /")
	}
}
