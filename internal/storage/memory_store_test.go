package storage

import (
	"context"
	"fmt"
	"sync"
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

// flakyReadStorageClient fails ReadFile for one specific object exactly once,
// then delegates to the wrapped client -- simulates a transient GetObject
// failure (network blip, eventual-consistency gap) on an otherwise-healthy store.
type flakyReadStorageClient struct {
	*mockStorageClient
	failObject string
	failed     bool
}

func (m *flakyReadStorageClient) ReadFile(ctx context.Context, bucket, object string) ([]byte, error) {
	if !m.failed && object == m.failObject {
		m.failed = true
		return nil, fmt.Errorf("simulated transient read failure")
	}
	return m.mockStorageClient.ReadFile(ctx, bucket, object)
}

// TestMemoryStore_List_TransientReadFailureDoesNotPoisonCache guards against a
// real regression the caching change introduced: List's per-file ReadFile
// errors were silently skipped (so a partial result could still be returned
// without an error), but the resulting incomplete list used to get cached
// unconditionally -- one flaky GetObject during a cold load would then serve
// that same incomplete list forever, even after storage recovered.
func TestMemoryStore_List_TransientReadFailureDoesNotPoisonCache(t *testing.T) {
	ctx := context.Background()
	base := newMockStorageClient()

	seed := NewMemoryStore(base, "test-bucket")
	m1 := &Memory{Artist: "Hendrix", Critique: "c1", Action: "a1"}
	m2 := &Memory{Artist: "Clapton", Critique: "c2", Action: "a2"}
	if err := seed.Save(ctx, m1); err != nil {
		t.Fatalf("seeding m1: %v", err)
	}
	if err := seed.Save(ctx, m2); err != nil {
		t.Fatalf("seeding m2: %v", err)
	}

	flaky := &flakyReadStorageClient{
		mockStorageClient: base,
		failObject:        fmt.Sprintf("memories/%s.json", m2.ID),
	}
	store := NewMemoryStore(flaky, "test-bucket")

	memories, err := store.List(ctx)
	if err != nil {
		t.Fatalf("expected a transient single-object read failure to yield a partial result, not an error: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 memory in the partial result, got %d", len(memories))
	}

	memories2, err := store.List(ctx)
	if err != nil {
		t.Fatalf("second List failed: %v", err)
	}
	if len(memories2) != 2 {
		t.Errorf("expected the transient failure to NOT be cached -- retry should see both memories, got %d", len(memories2))
	}
}

// TestMemoryStore_ConcurrentListAndMutation exercises List, Save, and Delete
// concurrently under `go test -race`: List used to return the cache's own
// backing slice/Memory pointers directly, so a concurrent Delete compacting
// that array (or a concurrent Save overwriting an entry in place) raced with a
// caller still reading the previously-returned slice.
func TestMemoryStore_ConcurrentListAndMutation(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(newMockStorageClient(), "test-bucket")

	for i := 0; i < 5; i++ {
		if err := store.Save(ctx, &Memory{Artist: fmt.Sprintf("seed-%d", i)}); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				memories, err := store.List(ctx)
				if err != nil {
					t.Errorf("List: %v", err)
					return
				}
				for _, m := range memories {
					_ = m.ID // read fields the way handleGetMemories does while iterating
				}
			}
		}()
	}

	for i := 0; i < 50; i++ {
		m := &Memory{Artist: fmt.Sprintf("writer-%d", i)}
		if err := store.Save(ctx, m); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := store.Delete(ctx, m.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
	}
	close(stop)
	wg.Wait()
}
