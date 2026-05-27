package agents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"google.golang.org/api/option"
)

func TestOpenLLMClient_Generate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Request Headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got: %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Expected Authorization Bearer test-api-key, got: %s", r.Header.Get("Authorization"))
		}

		// Verify Request Body
		var reqPayload ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if reqPayload.Model != "override-model" {
			t.Errorf("Expected model: override-model, got: %s", reqPayload.Model)
		}
		if len(reqPayload.Messages) != 2 {
			t.Fatalf("Expected 2 messages, got: %d", len(reqPayload.Messages))
		}
		if reqPayload.Messages[0].Role != "system" || reqPayload.Messages[0].Content != "Sys Prompt" {
			t.Errorf("Unexpected system message: %+v", reqPayload.Messages[0])
		}
		if reqPayload.Messages[1].Role != "user" || reqPayload.Messages[1].Content != "User Prompt" {
			t.Errorf("Unexpected user message: %+v", reqPayload.Messages[1])
		}

		// Write Mock Response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := ChatCompletionResponse{
			Usage: struct {
				PromptTokens     int32 `json:"prompt_tokens"`
				CompletionTokens int32 `json:"completion_tokens"`
			}{
				PromptTokens:     100,
				CompletionTokens: 50,
			},
		}
		resp.Choices = append(resp.Choices, struct {
			Message ChatMessage `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			Message: ChatMessage{
				Role:    "assistant",
				Content: "Hello from Open-LLM!",
			},
			FinishReason: "stop",
		})

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenLLMClient(server.URL, "test-api-key", "default-model", 2*time.Second)
	content, promptTokens, completionTokens, err := client.Generate(context.Background(), "Sys Prompt", "User Prompt", "override-model")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if content != "Hello from Open-LLM!" {
		t.Errorf("Expected content: 'Hello from Open-LLM!', got: '%s'", content)
	}
	if promptTokens != 100 {
		t.Errorf("Expected prompt tokens: 100, got: %d", promptTokens)
	}
	if completionTokens != 50 {
		t.Errorf("Expected completion tokens: 50, got: %d", completionTokens)
	}
}

func TestOpenLLMClient_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Starting Up"))
	}))
	defer server.Close()

	client := NewOpenLLMClient(server.URL, "key", "model", 2*time.Second)
	_, _, _, err := client.Generate(context.Background(), "Sys", "User", "")
	if err == nil {
		t.Fatal("Expected error on non-200 HTTP status, got nil")
	}

	expectedSubstr := "Open-LLM gateway returned non-OK status: 503"
	if !contains(err.Error(), expectedSubstr) {
		t.Errorf("Expected error to contain '%s', got: '%v'", expectedSubstr, err)
	}
}

func TestOrchestrator_OpenLLM_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := ChatCompletionResponse{
			Usage: struct {
				PromptTokens     int32 `json:"prompt_tokens"`
				CompletionTokens int32 `json:"completion_tokens"`
			}{
				PromptTokens:     20,
				CompletionTokens: 10,
			},
		}
		resp.Choices = append(resp.Choices, struct {
			Message ChatMessage `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			Message: ChatMessage{
				Role:    "assistant",
				Content: "Orchestrator Mock Success",
			},
			FinishReason: "stop",
		})
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Set Environment Toggles
	os.Setenv("USE_OPENLLM", "true")
	os.Setenv("OPENLLM_API_URL", server.URL)
	os.Setenv("OPENLLM_API_KEY", "dummy-secret-token")
	os.Setenv("OPENLLM_MODEL", "qwen-2.5-7b")
	os.Setenv("OPENLLM_TIMEOUT", "1s")

	defer func() {
		os.Unsetenv("USE_OPENLLM")
		os.Unsetenv("OPENLLM_API_URL")
		os.Unsetenv("OPENLLM_API_KEY")
		os.Unsetenv("OPENLLM_MODEL")
		os.Unsetenv("OPENLLM_TIMEOUT")
	}()

	ctx := context.Background()
	// Pass empty apiKey to verify Gemini client is skipped without failures
	orch, err := NewOrchestrator(ctx, "", nil)
	if err != nil {
		t.Fatalf("Failed to instantiate Orchestrator with Open-LLM active: %v", err)
	}
	defer orch.Close()

	if !orch.useOpenLLM {
		t.Fatal("Expected Orchestrator useOpenLLM to be true")
	}
	if orch.openLLMClient == nil {
		t.Fatal("Expected openLLMClient to be non-nil")
	}
	if orch.client != nil {
		t.Fatal("Expected Gemini client to be nil when Open-LLM is active")
	}

	res, err := orch.RunAgentSplit(ctx, "Tone Historian", "System prompt instruction", "User dynamic prompt")
	if err != nil {
		t.Fatalf("RunAgentSplit failed: %v", err)
	}

	if res != "Orchestrator Mock Success" {
		t.Errorf("Expected 'Orchestrator Mock Success', got: '%s'", res)
	}

	// Verify Telemetry Accounting
	orch.Usage.mu.Lock()
	inputTokens := orch.Usage.InputTokens
	outputTokens := orch.Usage.OutputTokens
	modelsUsedCount := orch.Usage.ModelsUsed["qwen-2.5-7b"]
	orch.Usage.mu.Unlock()

	if inputTokens != 20 {
		t.Errorf("Expected input tokens: 20, got: %d", inputTokens)
	}
	if outputTokens != 10 {
		t.Errorf("Expected output tokens: 10, got: %d", outputTokens)
	}
	if modelsUsedCount != 1 {
		t.Errorf("Expected model qwen-2.5-7b usage count: 1, got: %d", modelsUsedCount)
	}
}

func TestOrchestrator_Hybrid_Prefix_Routing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqPayload ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if reqPayload.Model != "custom-openllm-model" {
			t.Errorf("Expected model: custom-openllm-model, got: %s", reqPayload.Model)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := ChatCompletionResponse{
			Usage: struct {
				PromptTokens     int32 `json:"prompt_tokens"`
				CompletionTokens int32 `json:"completion_tokens"`
			}{
				PromptTokens:     40,
				CompletionTokens: 20,
			},
		}
		resp.Choices = append(resp.Choices, struct {
			Message ChatMessage `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			Message: ChatMessage{
				Role:    "assistant",
				Content: "Hybrid Route Response Successful",
			},
			FinishReason: "stop",
		})
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Keep USE_OPENLLM = false to prove prefix drives routing dynamically
	os.Setenv("USE_OPENLLM", "false")
	os.Setenv("OPENLLM_API_URL", server.URL)
	os.Setenv("OPENLLM_API_KEY", "key-here")

	defer func() {
		os.Unsetenv("USE_OPENLLM")
		os.Unsetenv("OPENLLM_API_URL")
		os.Unsetenv("OPENLLM_API_KEY")
	}()

	ctx := context.Background()
	// Initialize with fake-key/mock context settings to prevent standard gRPC startup failures
	orch, err := NewOrchestrator(ctx, "mock-api-key", nil)
	if err != nil {
		t.Fatalf("Failed to instantiate Orchestrator: %v", err)
	}
	defer orch.Close()

	if orch.useOpenLLM {
		t.Fatal("Expected useOpenLLM to be false")
	}

	// Map Tone Historian specifically to Open-LLM via the "openllm:" tag prefix
	orch.AgentModels = map[string]string{
		"1_tone_historian": "openllm:custom-openllm-model",
	}

	// Call RunAgentSplit
	res, err := orch.RunAgentSplit(ctx, "Tone Historian", "Sys prompt", "User prompt")
	if err != nil {
		t.Fatalf("RunAgentSplit with openllm prefix failed: %v", err)
	}

	if res != "Hybrid Route Response Successful" {
		t.Errorf("Expected 'Hybrid Route Response Successful', got: '%s'", res)
	}

	// Verify Telemetry
	orch.Usage.mu.Lock()
	inputTokens := orch.Usage.InputTokens
	outputTokens := orch.Usage.OutputTokens
	modelsUsedCount := orch.Usage.ModelsUsed["openllm:custom-openllm-model"]
	orch.Usage.mu.Unlock()

	if inputTokens != 40 {
		t.Errorf("Expected input tokens: 40, got: %d", inputTokens)
	}
	if outputTokens != 20 {
		t.Errorf("Expected output tokens: 20, got: %d", outputTokens)
	}
	if modelsUsedCount != 1 {
		t.Errorf("Expected model openllm:custom-openllm-model usage count: 1, got: %d", modelsUsedCount)
	}
}

func TestOrchestrator_OpenLLM_ContextLimit_Fallback(t *testing.T) {
	// 1. Mock Open-LLM Server returning 400 Bad Request with "context length exceeded" error
	openLLMServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"context length exceeded: maximum context length is 2048 tokens","type":"invalid_request_error"}}`))
	}))
	defer openLLMServer.Close()

	// 2. Mock Gemini Server returning a standard successful response
	geminiResponse := `{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"text": "Fallback Gemini Pro Response Success"
						}
					],
					"role": "model"
				}
			}
		],
		"usageMetadata": {
			"promptTokenCount": 50,
			"candidatesTokenCount": 25
		}
	}`
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(geminiResponse))
	}))
	defer geminiServer.Close()

	// Setup Environment Toggles
	os.Setenv("USE_OPENLLM", "false")
	os.Setenv("OPENLLM_API_URL", openLLMServer.URL)
	os.Setenv("OPENLLM_API_KEY", "dummy-secret-token")

	defer func() {
		os.Unsetenv("USE_OPENLLM")
		os.Unsetenv("OPENLLM_API_URL")
		os.Unsetenv("OPENLLM_API_KEY")
	}()

	ctx := context.Background()
	// Build main orchestrator in hybrid mode pointing to our mock Gemini server endpoint options
	orch, err := NewOrchestrator(ctx, "fake-gemini-key", nil, 
		option.WithEndpoint(geminiServer.URL), 
		option.WithHTTPClient(geminiServer.Client()))
	if err != nil {
		t.Fatalf("Failed to instantiate Orchestrator: %v", err)
	}
	defer orch.Close()

	// Override specific agent model to target Open-LLM
	orch.AgentModels = map[string]string{
		"1_tone_historian": "openllm:active-model",
	}

	// Call RunAgentSplit (should hit Open-LLM, receive context limit error, and fall back to Gemini Pro!)
	res, err := orch.RunAgentSplit(ctx, "Tone Historian", "System prompt", "User prompt")
	if err != nil {
		t.Fatalf("RunAgentSplit failed: %v", err)
	}

	if res != "Fallback Gemini Pro Response Success" {
		t.Errorf("Expected 'Fallback Gemini Pro Response Success', got: '%s'", res)
	}

	// Verify Telemetry Accounting (should capture the Gemini Pro usage!)
	orch.Usage.mu.Lock()
	inputTokens := orch.Usage.InputTokens
	outputTokens := orch.Usage.OutputTokens
	modelsUsedCount := orch.Usage.ModelsUsed["gemini-3.1-pro-preview"]
	orch.Usage.mu.Unlock()

	if inputTokens != 50 {
		t.Errorf("Expected input tokens: 50, got: %d", inputTokens)
	}
	if outputTokens != 25 {
		t.Errorf("Expected output tokens: 25, got: %d", outputTokens)
	}
	if modelsUsedCount != 1 {
		t.Errorf("Expected model gemini-3.1-pro-preview usage count: 1, got: %d", modelsUsedCount)
	}
}

// Simple helper function since standard strings.Contains isn't imported to keep imports minimal
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(s)] != "" && (s == substr || len(substr) == 0 || (len(s) > len(substr) && (s[0:len(substr)] == substr || contains(s[1:], substr))))
}
