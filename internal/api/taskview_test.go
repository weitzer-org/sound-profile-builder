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
	"github.com/weitzer-org/sound-builder/internal/presetsvc"
	"github.com/weitzer-org/sound-builder/internal/runs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestViewFromRecord(t *testing.T) {
	s := &Server{}
	preset := &storage.Preset{ID: "p1", Name: "Draft Preset", Payload: "none"}

	tests := []struct {
		name       string
		rec        runs.RunRecord
		preset     *storage.Preset
		isChat     bool
		wantStatus string
		wantPhase  string
		wantErr    string
		wantResult bool
	}{
		{name: "running", rec: runs.RunRecord{Status: runs.StatusRunning, Phase: "Phase 2/4"}, wantStatus: "running", wantPhase: "Phase 2/4"},
		{name: "error", rec: runs.RunRecord{Status: runs.StatusError, Error: "boom"}, wantStatus: "error", wantErr: "boom"},
		{name: "abandoned", rec: runs.RunRecord{Status: runs.StatusAbandoned, Error: "server stopped"}, wantStatus: "error", wantErr: "server stopped"},
		{name: "complete without preset", rec: runs.RunRecord{Status: runs.StatusComplete}, wantStatus: "error", wantErr: "no longer available"},
		{name: "complete generate", rec: runs.RunRecord{Status: runs.StatusComplete}, preset: preset, wantStatus: "complete", wantResult: true},
		{name: "complete refine", rec: runs.RunRecord{Status: runs.StatusComplete}, preset: preset, isChat: true, wantStatus: "complete", wantResult: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.viewFromRecord(tt.rec, tt.preset, tt.isChat)
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.Phase != tt.wantPhase {
				t.Errorf("Phase = %q, want %q", got.Phase, tt.wantPhase)
			}
			if !strings.Contains(got.Error, tt.wantErr) {
				t.Errorf("Error = %q, want it to contain %q", got.Error, tt.wantErr)
			}
			if (got.Result != "") != tt.wantResult {
				t.Errorf("Result set = %v, want %v", got.Result != "", tt.wantResult)
			}
		})
	}
}

func TestTaskView(t *testing.T) {
	ctx := context.Background()

	t.Run("override map wins", func(t *testing.T) {
		s, _, _, _ := setupTestServer()
		s.tasksMu.Lock()
		s.tasks["x"] = &TaskState{Status: "running", Phase: "from override"}
		s.tasksMu.Unlock()
		v, ok := s.taskView(ctx, "x")
		if !ok || v.Phase != "from override" {
			t.Errorf("got (%+v, %v), want the override entry", v, ok)
		}
	})

	t.Run("unknown and invalid ids are not found", func(t *testing.T) {
		s, _, _, _ := setupTestServer()
		for _, id := range []string{"missing", "../presets/x", "gen-does-not-exist"} {
			if _, ok := s.taskView(ctx, id); ok {
				t.Errorf("taskView(%q) found something, want not found", id)
			}
		}
	})

	t.Run("persisted run is served after eviction or restart", func(t *testing.T) {
		s, _, _, _ := setupTestServer()
		now := time.Now().UTC()
		rec := &runs.RunRecord{ID: "gen-persisted-1", Kind: runs.KindGenerate, Status: runs.StatusError, Error: "pipeline broke", CreatedAt: now, UpdatedAt: now}
		if err := s.svc.Runs().Put(ctx, rec); err != nil {
			t.Fatal(err)
		}
		v, ok := s.taskView(ctx, rec.ID)
		if !ok || v.Status != "error" || v.Error != "pipeline broke" {
			t.Errorf("got (%+v, %v), want the persisted error", v, ok)
		}
	})

	t.Run("live run completes and renders the draft", func(t *testing.T) {
		s, _, _, _ := setupTestServer()
		t.Setenv("MCP_MAX_CONCURRENT_RUNS", "10")
		s.orchMaker = func(ctx context.Context, key string) (agents.OrchestratorService, error) {
			return &mockOrchestratorCapturesConstraints{constraintsCh: make(chan map[string]interface{}, 1)}, nil
		}
		id, err := s.svc.StartGenerate(ctx, "warm tone", presetsvc.GenerateOptions{Mock: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.svc.WaitRun(ctx, id, 10*time.Second); err != nil {
			t.Fatal(err)
		}
		v, ok := s.taskView(ctx, id)
		if !ok || v.Status != "complete" || v.Result == "" {
			t.Errorf("got (status=%q, result set=%v, ok=%v), want a complete run with rendered HTML", v.Status, v.Result != "", ok)
		}
	})
}

// Error text reaches the chat UI as HTML; it can carry LLM output or the
// request's own preset ID, so it must be escaped (the status endpoint already
// does this for generate errors).
func TestHandleChatPreset_EscapesErrorText(t *testing.T) {
	const payload = `<img src=x onerror=alert(1)>`

	t.Run("execution error", func(t *testing.T) {
		s, _, _, orch := setupTestServer()
		t.Setenv("MCP_MAX_CONCURRENT_RUNS", "10")
		s.store.Save(context.Background(), &storage.Preset{ID: "esc-1", Name: "n", Payload: "none"})
		orch.err = fmt.Errorf("model said %s", payload)

		body := postChat(t, s, url.Values{"id": {"esc-1"}, "message": {"hi"}})
		if strings.Contains(body, payload) {
			t.Errorf("raw markup reflected into the response: %s", body)
		}
		if !strings.Contains(body, "Execution Error: model said &lt;img") {
			t.Errorf("expected the escaped error text, got: %s", body)
		}
	})

	t.Run("lookup error", func(t *testing.T) {
		s, _, _, _ := setupTestServer()
		body := postChat(t, s, url.Values{"id": {payload}, "message": {"hi"}})
		if strings.Contains(body, payload) {
			t.Errorf("raw markup reflected into the response: %s", body)
		}
		if !strings.Contains(body, "Lookup Error") {
			t.Errorf("expected a lookup error, got: %s", body)
		}
	})
}

func postChat(t *testing.T, s *Server, form url.Values) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "/api/preset/chat", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.handleChatPreset().ServeHTTP(rr, req)
	return rr.Body.String()
}
