package storage

import (
	"context"
	"testing"
)

func TestMemoryStore_CRUD(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockStorageClient()
	store := NewMemoryStore(mockClient, "test-bucket")

	// Test Save
	mem := &Memory{
		Artist:   "Hendrix",
		Critique: "Too dark",
		Action:   "Increase treble",
	}
	
	err := store.Save(ctx, mem)
	if err != nil {
		t.Fatalf("Failed to save memory: %v", err)
	}

	if mem.ID == "" {
		t.Error("Expected memory ID to be generated")
	}

	// Test Get
	fetched, err := store.Get(ctx, mem.ID)
	if err != nil {
		t.Fatalf("Failed to get memory: %v", err)
	}

	if fetched.Artist != mem.Artist || fetched.Critique != mem.Critique {
		t.Errorf("Fetched memory does not match original. Got %+v", fetched)
	}

	// Test List
	memories, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list memories: %v", err)
	}

	if len(memories) != 1 {
		t.Errorf("Expected 1 memory in list, got %d", len(memories))
	} else if memories[0].ID != mem.ID {
		t.Errorf("Expected memory ID %s in list, got %s", mem.ID, memories[0].ID)
	}

	// Test Delete
	err = store.Delete(ctx, mem.ID)
	if err != nil {
		t.Fatalf("Failed to delete memory: %v", err)
	}

	memoriesAfterDelete, _ := store.List(ctx)
	if len(memoriesAfterDelete) != 0 {
		t.Errorf("Expected 0 memories after deletion, got %d", len(memoriesAfterDelete))
	}
}

func TestMemoryStore_Errors(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockErrorStorageClient{}
	store := NewMemoryStore(mockClient, "test-bucket")

	// Save failure
	err := store.Save(ctx, &Memory{Artist: "Fail"})
	if err == nil {
		t.Errorf("Expected Save error")
	}

	// Get missing/failure
	_, err = store.Get(ctx, "missing")
	if err == nil {
		t.Errorf("Expected Get read error")
	}

	// List failure
	mockClient.failList = true
	_, err = store.List(ctx)
	if err == nil {
		t.Errorf("Expected List error")
	}
}
