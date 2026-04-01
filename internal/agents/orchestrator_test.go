package agents

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/storage"
	"google.golang.org/api/option"
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

func TestOrchestrator_RunPipeline_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, err := NewOrchestrator(ctx, "fake-key", nil, option.WithEndpoint(mockServer.URL), option.WithHTTPClient(mockServer.Client()))
	if err != nil {
		t.Fatalf("Failed to init orchestrator: %v", err)
	}
	defer orch.Close()

	constraints := map[string]interface{}{
		"single_amp_mode":      true,
		"allow_cloud_captures": false,
	}

	res, usage, err := orch.RunPipeline(ctx, "test prompt", constraints, nil)
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

func TestOrchestrator_RefineChat_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse))
	}))
	defer mockServer.Close()

	ctx := context.Background()
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, option.WithEndpoint(mockServer.URL), option.WithHTTPClient(mockServer.Client()))

	p := &storage.Preset{
		Payload: "<matrix></matrix>",
		ChatHistory: []storage.ChatMessage{
			{Role: "user", Content: "hello"},
			{Role: "model", Content: "hi"},
		},
	}

	res, usage, err := orch.RefineChat(ctx, p, "change it")
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

func TestOrchestrator_TimeoutsAndErrors(t *testing.T) {
	ctx := context.Background()

	orch, _ := NewOrchestrator(ctx, "fake-key", nil, option.WithEndpoint("http://127.0.0.1:0"))

	// Fast timeout ensures RunAgent errors immediately
	ctxTimeout, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()

	_, _, err := orch.RunPipeline(ctxTimeout, "test", nil, nil)
	if err == nil {
		t.Errorf("Expected pipeline to fail on timeout")
	}

	_, _, err = orch.RefineChat(ctxTimeout, &storage.Preset{}, "test")
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
	orch, _ := NewOrchestrator(ctx, "fake-key", nil, option.WithEndpoint(mockServer.URL), option.WithHTTPClient(mockServer.Client()))

	_, err := orch.RunAgent(ctx, "Agent", "prompt")
	if err == nil {
		t.Errorf("Expected error from empty candidates array")
	}
}

func TestOrchestrator_NewOrchestrator_Error(t *testing.T) {
	ctx := context.Background()
	_, err := NewOrchestrator(ctx, "key", nil, option.WithCredentialsFile("doesnotexist.json"))
	if err == nil {
		t.Errorf("Expected GenAI client to fail with nonexistent credentials injection")
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
			orch, _ := NewOrchestrator(ctx, "key", nil, option.WithEndpoint(mockServer.URL), option.WithHTTPClient(mockServer.Client()))
			defer orch.Close()

			_, _, err := orch.RunPipeline(ctx, "test", map[string]interface{}{"allow_cloud_captures": true}, nil)
			if err == nil {
				t.Errorf("Expected pipeline to fail on target %d", tt.failTarget)
			}
		})
	}
}
