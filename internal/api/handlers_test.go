package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// mockClient implements storage.Client for api tests
type mockClient struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMockClient() *mockClient {
	return &mockClient{data: make(map[string][]byte)}
}

func (m *mockClient) ReadFile(ctx context.Context, bucket, object string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := bucket + "/" + object
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockClient) WriteFile(ctx context.Context, bucket, object string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := bucket + "/" + object
	// Must copy so we don't hold pointers
	m.data[key] = make([]byte, len(data))
	copy(m.data[key], data)
	return nil
}

func (m *mockClient) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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
	m.mu.Lock()
	defer m.mu.Unlock()
	key := bucket + "/" + object
	delete(m.data, key)
	return nil
}

func (m *mockClient) Close() {}

// mockSecretFetcher implements storage.SecretFetcher
type mockSecretFetcher struct {
	err error
}
func (m *mockSecretFetcher) GetPassword(ctx context.Context, projectID, secretName string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "mock-secret", nil
}
func (m *mockSecretFetcher) Close() {}

// mockOrchestrator implements agents.OrchestratorService
type mockOrchestrator struct {
	err error
}
func (m *mockOrchestrator) RunPipeline(ctx context.Context, prompt string, constraints map[string]interface{}) (string, *agents.TokenUsage, error) {
	if m.err != nil {
		return "", nil, m.err
	}
	return `{"final_html_payload":{"Gibson ES-339 Humbuckers":"mock"},"agent_impact":[]}`, &agents.TokenUsage{}, nil
}
func (m *mockOrchestrator) RefineChat(ctx context.Context, p *storage.Preset, userMessage string) (string, *agents.TokenUsage, error) {
	if m.err != nil {
		return "", nil, m.err
	}
	return `{"final_html_payload":{"Gibson ES-339 Humbuckers":"mock"},"agent_impact":[]}`, &agents.TokenUsage{}, nil
}
func (m *mockOrchestrator) Close() {}

func setupTestServer() (*Server, *mockClient, *mockSecretFetcher, *mockOrchestrator) {
	client := newMockClient()
	store := storage.NewPresetStore(client, "test-bucket")
	sf := &mockSecretFetcher{}
	orch := &mockOrchestrator{}
	om := func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		if orch.err != nil && orch.err.Error() == "orch factory error" {
			return nil, orch.err
		}
		return orch, nil
	}
	return NewServer(store, nil, client, sf, om, nil), client, sf, orch
}

func TestHandleGetPresets_Empty(t *testing.T) {
	s, _, _, _ := setupTestServer()

	req, _ := http.NewRequest(http.MethodGet, "/api/presets", nil)
	rr := httptest.NewRecorder()
	
	s.handleGetPresets().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "No presets saved yet") {
		t.Errorf("Expected empty state message, got %s", rr.Body.String())
	}
}

func TestHandleSavePreset(t *testing.T) {
	s, storeClient, _, _ := setupTestServer()

	formData := url.Values{}
	formData.Set("preset_name", "Test Tone")
	formData.Set("payload", "<span>Cool Tone</span>")

	req, _ := http.NewRequest(http.MethodPost, "/api/preset/save", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	s.handleSavePreset().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Should render the new preset
	if !strings.Contains(rr.Body.String(), "Test Tone") {
		t.Errorf("Expected rendered HTML to contain preset name")
	}

	// Internal verify in mock
	found := false
	storeClient.mu.Lock()
	for k := range storeClient.data {
		if strings.Contains(k, "presets/") {
			found = true
			break
		}
	}
	storeClient.mu.Unlock()

	if !found {
		t.Errorf("Failed to actually save to underlying mock client")
	}
}

func TestHandleDeletePreset(t *testing.T) {
	s, storeClient, _, _ := setupTestServer()
	
	// Pre-seed a preset
	store := storage.NewPresetStore(storeClient, "test-bucket")
	p := &storage.Preset{Name: "To Be Deleted", Payload: "none"}
	store.Save(context.Background(), p) // This will generate an ID

	formData := url.Values{}
	formData.Set("id", p.ID)

	req, _ := http.NewRequest(http.MethodPost, "/api/preset/delete", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	s.handleDeletePreset().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Ensure the preset was deleted
	fetched, _ := store.List(context.Background())
	if len(fetched) != 0 {
		t.Errorf("Expected 0 presets after deletion, got %d", len(fetched))
	}
}
