package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/api"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// mockClient implements storage.Client for scratch tests
type mockClient struct {
	data map[string][]byte
}

func (m *mockClient) ReadFile(ctx context.Context, bucket, object string) ([]byte, error) {
	key := bucket + "/" + object
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockClient) WriteFile(ctx context.Context, bucket, object string, data []byte) error {
	key := bucket + "/" + object
	m.data[key] = data
	return nil
}

func (m *mockClient) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	var matches []string
	prefixKey := bucket + "/" + prefix
	for key := range m.data {
		if strings.HasPrefix(key, prefixKey) {
			parts := strings.SplitN(key, "/", 2)
			matches = append(matches, parts[1])
		}
	}
	return matches, nil
}

func (m *mockClient) DeleteFile(ctx context.Context, bucket, object string) error {
	key := bucket + "/" + object
	delete(m.data, key)
	return nil
}

func (m *mockClient) Close() {}

// mockOrchestrator implements agents.OrchestratorService
type mockOrchestrator struct{}

func (m *mockOrchestrator) RunPipeline(ctx context.Context, prompt string, constraints map[string]interface{}) (string, *agents.TokenUsage, error) {
	// Return a combined payload JSON containing both structured and legacy HTML for testing parser
	return `{"final_html_payload":{"Fender Telecaster":"<table>Legacy Table</table>"},"structured_payload":{"guitars":{"Fender Telecaster":[]}}}`, &agents.TokenUsage{}, nil
}

func (m *mockOrchestrator) RefineChat(ctx context.Context, p *storage.Preset, userMessage string) (string, *agents.TokenUsage, error) {
	return `{"final_html_payload":{"Fender Telecaster":"<table>Legacy Table Refined</table>"},"structured_payload":{"guitars":{"Fender Telecaster":[]}}}`, &agents.TokenUsage{}, nil
}

func (m *mockOrchestrator) Close() {}

func main() {
	client := &mockClient{data: make(map[string][]byte)}
	store := storage.NewPresetStore(client, "test-bucket")
	
	om := func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		return &mockOrchestrator{}, nil
	}

	s := api.NewServer(store, nil, client, nil, om, nil)

	// Test 1: Generate Preset
	fmt.Println("--- Running E2E API Test 1: Generate Preset ---")
	form := url.Values{}
	form.Set("prompt", "bb king")
	req := httptest.NewRequest(http.MethodPost, "/api/preset/generate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	s.HandleGeneratePreset(rr, req) // Wait, HandleGeneratePreset is not exported as public function usually if it's unexported! Let's assume it's unexported or we use ServeHTTP!
	// In server.go, s.mux.Handle is used! So we should use ServeHTTP on the mux!
	// But since we can't access s.mux directly, let's see if NewServer returns a mux or a struct with mux!
	// In main.go, NewServer is used and then it runs!
	// Let's check api.NewServer definition! It returns *Server! And *Server has a ServeHTTP method or similar!
	// It has a Router!
	
	// Let's use the actual handler function since we are within the package or we call ServeHTTP on the server if it implements Handler!
	// If it doesn't implement Handler, we can check how it's used in main.go!
	// Let's view NewServer signature!
}
