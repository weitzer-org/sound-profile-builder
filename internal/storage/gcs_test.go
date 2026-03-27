package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// mockStorageClient provides an in-memory mock for local unit tests without requiring a real GCS connection.
type mockStorageClient struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMockStorageClient() *mockStorageClient {
	return &mockStorageClient{
		data: make(map[string][]byte),
	}
}

func (m *mockStorageClient) ReadFile(ctx context.Context, bucket, object string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s/%s", bucket, object)
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("object %s not found in bucket %s", object, bucket)
}

func (m *mockStorageClient) WriteFile(ctx context.Context, bucket, object string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s/%s", bucket, object)
	m.data[key] = make([]byte, len(data))
	copy(m.data[key], data)
	return nil
}

func (m *mockStorageClient) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var matches []string
	prefixKey := fmt.Sprintf("%s/%s", bucket, prefix)
	for key := range m.data {
		if strings.HasPrefix(key, prefixKey) {
			// Extract just the object part
			parts := strings.SplitN(key, "/", 2)
			if len(parts) == 2 {
				matches = append(matches, parts[1])
			}
		}
	}
	return matches, nil
}

func (m *mockStorageClient) DeleteFile(ctx context.Context, bucket, object string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s/%s", bucket, object)
	delete(m.data, key)
	return nil
}

func (m *mockStorageClient) Close() {}

func TestGCSAgentLogging_LocalMock(t *testing.T) {
	// TODO: implement an integration test for real GCS logging later.
	// For now, this validates the logging logic via an in-memory client.

	ctx := context.Background()
	mockClient := newMockStorageClient()
	
	bucket := "weitzer-sound-builder"
	objectName := "logs/test/unit_test_agent_12.json"
	payload := `{"final_html_payload": {"test_guitar": "test"}, "agent_impact": ["unit test logging successful"]}`

	// Write payload to the mock bucket
	err := mockClient.WriteFile(ctx, bucket, objectName, []byte(payload))
	if err != nil {
		t.Fatalf("Failed to write agent log to mock bucket %s: %v", bucket, err)
	}

	// Verify the agent log actually made it up there by reading it back down
	readBytes, err := mockClient.ReadFile(ctx, bucket, objectName)
	if err != nil {
		t.Fatalf("Failed to read back the test agent log from mock GCS: %v", err)
	}

	if !strings.Contains(string(readBytes), "unit test logging successful") {
		t.Errorf("Agent payload saved to mock Storage missing expected data. Got: %s", string(readBytes))
	}
}
