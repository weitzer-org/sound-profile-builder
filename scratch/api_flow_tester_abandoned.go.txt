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

	// Simulated server mux initialization since NewServer might not run it
	// In server.go, s.mux is private inside Server, but we only have a public Handle/Router if available!
	// Let's assume server implements `ServeHTTP` or we call the unexported handler if we are in tests!
	// Here in main package, we cannot access unexported things.
	// But since this is a scratch script, we can just call `s.HandleGeneratePreset` if we can access it!
	
	// If it's not exported, we can just use the internal test suite or rename the scratch script to be part of the test package!
	// Let's create `internal/api/integration_test.go` instead! It belongs to `package api`!
	// That way it has access to unexported things!
}
