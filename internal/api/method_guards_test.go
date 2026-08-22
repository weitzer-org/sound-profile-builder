package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// These handlers used to accept any HTTP method (GET included), which meant
// e.g. GET /api/preset/delete?id=X worked -- undermining any CSRF defense,
// since SameSite/CSRF-token checks are conventionally only meaningful on
// state-changing methods. Issue #53.
func TestHandlers_RejectNonPostMethod(t *testing.T) {
	cases := []struct {
		name    string
		handler func(*Server) http.HandlerFunc
	}{
		{"handleSavePreset", (*Server).handleSavePreset},
		{"handleDeletePreset", (*Server).handleDeletePreset},
		{"handleCopyPreset", (*Server).handleCopyPreset},
		{"handleDeleteDraftPreset", (*Server).handleDeleteDraftPreset},
		{"handleRenamePreset", (*Server).handleRenamePreset},
		{"handleChatPreset", (*Server).handleChatPreset},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _, _ := setupTestServer()
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			rr := httptest.NewRecorder()
			tc.handler(s).ServeHTTP(rr, req)
			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405 for GET, got %d", rr.Code)
			}
		})
	}
}

func TestHandleDeleteMemory_RejectsNonDeleteMethod(t *testing.T) {
	s, _, _, _ := setupTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/memory/delete?id=x", nil)
	rr := httptest.NewRecorder()
	s.handleDeleteMemory().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", rr.Code)
	}
}

func TestHandleDeleteMemory_AcceptsDelete(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	memStore := storage.NewMemoryStore(mockStorage, "b")
	s := NewServer(nil, memStore, mockStorage, &mockSecretFetcher{}, nil, nil)

	m := &storage.Memory{Artist: "a", Critique: "c", Action: "act"}
	if err := memStore.Save(context.Background(), m); err != nil {
		t.Fatalf("failed to seed memory: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/memory/delete?id="+m.ID, nil)
	rr := httptest.NewRecorder()
	s.handleDeleteMemory().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for DELETE, got %d: %s", rr.Code, rr.Body.String())
	}
}

// Issue #90: preset names were interpolated into the save/rename toast
// without HTML-escaping, unlike every other rendering of a preset name in
// this file.
func TestHandleSavePreset_EscapesNameInToast(t *testing.T) {
	s, _, _, _ := setupTestServer()

	payload := `<script>alert(1)</script>`
	req := httptest.NewRequest(http.MethodPost, "/api/preset/save", strings.NewReader(
		"preset_name="+payload+"&payload=ok"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.handleSavePreset().ServeHTTP(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, payload) {
		t.Errorf("expected preset name to be HTML-escaped in response, got raw payload in body: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("expected escaped preset name in response body, got: %s", body)
	}
}

func TestHandleRenamePreset_EscapesNameInToast(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	s := NewServer(store, nil, mockStorage, &mockSecretFetcher{}, nil, nil)

	p := &storage.Preset{Name: "Old Name", Payload: "none"}
	store.Save(context.Background(), p)

	payload := `<script>alert(1)</script>`
	req := httptest.NewRequest(http.MethodPost, "/api/preset/rename", strings.NewReader(
		"id="+p.ID+"&preset_name="+payload))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.handleRenamePreset().ServeHTTP(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, payload) {
		t.Errorf("expected preset name to be HTML-escaped in response, got raw payload in body: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("expected escaped preset name in response body, got: %s", body)
	}
}

// Issue #58: two workspace renders on the same page (Generator draft +
// Library editor) used to share the literal id "workspace-wrapper", so
// htmx's document.querySelector resolved to whichever came first in the
// DOM -- silently misdirecting a Library chat-refinement response into the
// hidden Generator draft. The id is now keyed by preset ID, and the chat
// form targets "closest .workspace-wrapper" instead of the id.
func TestRenderTweakingWorkspaceHTML_UniqueWorkspaceWrapperID(t *testing.T) {
	p1 := &storage.Preset{ID: "preset-one", Name: "Preset One", Payload: "none"}
	p2 := &storage.Preset{ID: "preset-two", Name: "Preset Two", Payload: "none"}

	html1 := renderTweakingWorkspaceHTML(p1, false, false)
	html2 := renderTweakingWorkspaceHTML(p2, false, false)

	if !strings.Contains(html1, `id="workspace-wrapper-preset-one"`) {
		t.Errorf("expected id keyed by preset ID, got: %s", html1)
	}
	if !strings.Contains(html2, `id="workspace-wrapper-preset-two"`) {
		t.Errorf("expected id keyed by preset ID, got: %s", html2)
	}
	if strings.Contains(html1, `id="workspace-wrapper"`) {
		t.Errorf("did not expect the old unkeyed id to still be present")
	}

	if !strings.Contains(html1, `hx-target="closest .workspace-wrapper"`) {
		t.Errorf("expected chat form to target the closest .workspace-wrapper, got: %s", html1)
	}
	if strings.Contains(html1, `hx-target="#workspace-wrapper"`) {
		t.Errorf("did not expect the chat form to still target the shared #workspace-wrapper id")
	}
}
