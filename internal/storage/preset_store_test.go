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

type mockCacheStorageClient struct {
	*mockStorageClient
	listCalls int
}

func (m *mockCacheStorageClient) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	m.listCalls++
	return m.mockStorageClient.ListFiles(ctx, bucket, prefix)
}

func TestPresetStore_Cache(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockCacheStorageClient{
		mockStorageClient: newMockStorageClient(),
	}
	store := NewPresetStore(mockClient, "test-bucket")

	// Save a preset
	p := &Preset{Name: "Cached Preset"}
	err := store.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// First List should call ListFiles
	presets, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list: %v", err)
	}
	if mockClient.listCalls != 1 {
		t.Errorf("Expected 1 list call, got %d", mockClient.listCalls)
	}
	if len(presets) != 1 {
		t.Errorf("Expected 1 preset, got %d", len(presets))
	}

	// Second List should NOT call ListFiles
	presets2, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list 2: %v", err)
	}
	if mockClient.listCalls != 1 {
		t.Errorf("Expected still 1 list call, got %d", mockClient.listCalls)
	}
	if len(presets2) != 1 {
		t.Errorf("Expected 1 preset, got %d", len(presets2))
	}

	// Save another preset (should update cache)
	p2 := &Preset{Name: "Cached Preset 2"}
	err = store.Save(ctx, p2)
	if err != nil {
		t.Fatalf("Failed to save 2: %v", err)
	}

	// Third List should STILL NOT call ListFiles (served from updated cache)
	presets3, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list 3: %v", err)
	}
	if mockClient.listCalls != 1 {
		t.Errorf("Expected still 1 list call, got %d", mockClient.listCalls)
	}
	if len(presets3) != 2 {
		t.Errorf("Expected 2 presets in cache, got %d", len(presets3))
	}

	// Update an existing preset (should update cache in place)
	p.Name = "Updated Cached Preset"
	err = store.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	// Fourth List should STILL NOT call ListFiles
	presets4, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list 4: %v", err)
	}
	if mockClient.listCalls != 1 {
		t.Errorf("Expected still 1 list call, got %d", mockClient.listCalls)
	}
	
	// Verify that the name was updated in the cached list!
	found := false
	for _, cached := range presets4 {
		if cached.ID == p.ID && cached.Name == "Updated Cached Preset" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find updated preset in cache")
	}
}
