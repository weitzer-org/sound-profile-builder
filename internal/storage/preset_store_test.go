package storage

import (
	"context"
	"fmt"
	"testing"
)

func TestPresetStore_CRUD(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockStorageClient()
	store := NewPresetStore(mockClient, "test-bucket")

	// Test Save
	preset := &Preset{
		Name:    "Test Preset",
		Payload: `{"foo":"bar"}`,
	}
	
	err := store.Save(ctx, preset)
	if err != nil {
		t.Fatalf("Failed to save preset: %v", err)
	}

	if preset.ID == "" {
		t.Error("Expected preset ID to be generated")
	}

	// Test Get
	fetched, err := store.Get(ctx, preset.ID)
	if err != nil {
		t.Fatalf("Failed to get preset: %v", err)
	}

	if fetched.Name != preset.Name || fetched.Payload != preset.Payload {
		t.Errorf("Fetched preset does not match original. Got %+v", fetched)
	}

	// Test List
	presets, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list presets: %v", err)
	}

	if len(presets) != 1 {
		t.Errorf("Expected 1 preset in list, got %d", len(presets))
	} else if presets[0].ID != preset.ID {
		t.Errorf("Expected preset ID %s in list, got %s", preset.ID, presets[0].ID)
	}

	// Test Delete
	err = store.Delete(ctx, preset.ID)
	if err != nil {
		t.Fatalf("Failed to delete preset: %v", err)
	}

	presetsAfterDelete, _ := store.List(ctx)
	if len(presetsAfterDelete) != 0 {
		t.Errorf("Expected 0 presets after deletion, got %d", len(presetsAfterDelete))
	}
}

// mockErrorStorageClient simulates forced failures
type mockErrorStorageClient struct {
	failList bool
}

func (m *mockErrorStorageClient) ReadFile(ctx context.Context, bucket, object string) ([]byte, error) {
	if object == "presets/bad-json.json" {
		return []byte(`{bad json`), nil
	}
	return nil, fmt.Errorf("read error")
}
func (m *mockErrorStorageClient) WriteFile(ctx context.Context, bucket, object string, data []byte) error {
	return fmt.Errorf("write error")
}
func (m *mockErrorStorageClient) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	if m.failList {
		return nil, fmt.Errorf("list error")
	}
	return []string{"presets/bad-json.json", "presets/read-err.json"}, nil
}
func (m *mockErrorStorageClient) DeleteFile(ctx context.Context, bucket, object string) error {
	return fmt.Errorf("delete error")
}
func (m *mockErrorStorageClient) Close() {}

func TestPresetStore_Errors(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockErrorStorageClient{}
	store := NewPresetStore(mockClient, "test-bucket")

	// Save failure
	err := store.Save(ctx, &Preset{Name: "Fail"})
	if err == nil {
		t.Errorf("Expected Save error")
	}

	// Get missing/failure
	_, err = store.Get(ctx, "missing")
	if err == nil {
		t.Errorf("Expected Get read error")
	}

	// Get unmarshal failure
	_, err = store.Get(ctx, "bad-json")
	if err == nil {
		t.Errorf("Expected Get unmarshal error")
	}

	// List bad files
	presets, err := store.List(ctx)
	if err != nil {
		t.Errorf("Expected List to ignore individual file errors: %v", err)
	}
	if len(presets) != 0 {
		t.Errorf("Expected 0 valid presets")
	}

	// List failure
	mockClient.failList = true
	_, err = store.List(ctx)
	if err == nil {
		t.Errorf("Expected List array error")
	}
}
