package storage

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
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

// blockingListStorageClient stands in for the real several-seconds-of-network-
// latency ListFiles+GetObject sequence a cold List() load makes against R2:
// ListFiles enumerates immediately (so its result reflects a snapshot taken at
// call time, same as a real ListObjectsV2), then blocks returning that result
// until the test signals proceed -- simulating a fetch that's in flight but
// hasn't yet reached the point of writing back to the cache.
type blockingListStorageClient struct {
	*mockStorageClient
	proceed chan struct{}
}

func (m *blockingListStorageClient) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	files, err := m.mockStorageClient.ListFiles(ctx, bucket, prefix)
	<-m.proceed
	return files, err
}

// TestMemoryStore_List_DoesNotBlockConcurrentSaveDuringIO guards against holding
// s.mu across List's network I/O: previously the write lock stayed held for the
// entire ListFiles+GetObject sequence, so a concurrent Save (or Delete, or
// another List) would stall for however long that fetch took -- multiple seconds
// against real object storage.
func TestMemoryStore_List_DoesNotBlockConcurrentSaveDuringIO(t *testing.T) {
	ctx := context.Background()
	client := &blockingListStorageClient{
		mockStorageClient: newMockStorageClient(),
		proceed:           make(chan struct{}),
	}
	store := NewMemoryStore(client, "test-bucket")

	listDone := make(chan error, 1)
	go func() {
		_, err := store.List(ctx)
		listDone <- err
	}()

	// Give the List goroutine time to reach ListFiles and block on `proceed`.
	time.Sleep(50 * time.Millisecond)

	saveDone := make(chan error, 1)
	go func() {
		saveDone <- store.Save(ctx, &Memory{Artist: "Concurrent"})
	}()

	select {
	case err := <-saveDone:
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Save blocked while List's ListFiles call was still in flight -- the cache lock is being held across network I/O")
	}

	close(client.proceed)
	if err := <-listDone; err != nil {
		t.Fatalf("List failed: %v", err)
	}
}

// TestMemoryStore_List_ConcurrentSaveDuringColdLoadIsNotPermanentlyLost guards
// against the race releasing the lock during I/O introduced: a Save landing
// after ListFiles enumerated (so the in-flight fetch's snapshot doesn't include
// it) but before that fetch writes back to s.cache used to go unnoticed --
// Save only touches s.cache when it's already non-nil, so the fetch would then
// see s.cache == nil and cache its now-stale snapshot, permanently hiding the
// Save until process restart. The epoch counter must catch this and skip
// caching that snapshot instead.
func TestMemoryStore_List_ConcurrentSaveDuringColdLoadIsNotPermanentlyLost(t *testing.T) {
	ctx := context.Background()
	base := newMockStorageClient()

	// Seed one pre-existing memory via a throwaway store sharing the same
	// backing data, written before the racy client below is even constructed.
	// This matters: without it, the in-flight fetch's stale snapshot would be
	// empty, and caching an empty (nil) slice is indistinguishable from not
	// caching at all -- the bug this test targets only manifests when the
	// stale cached snapshot is a real, non-empty, truthy value that a later
	// List() can wrongly keep serving instead of retrying.
	seed := NewMemoryStore(base, "test-bucket")
	if err := seed.Save(ctx, &Memory{Artist: "Pre-existing"}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	client := &blockingListStorageClient{
		mockStorageClient: base,
		proceed:           make(chan struct{}),
	}
	store := NewMemoryStore(client, "test-bucket")

	listDone := make(chan error, 1)
	go func() {
		_, err := store.List(ctx)
		listDone <- err
	}()

	// Give the List goroutine time to enumerate ListFiles (a 1-item snapshot,
	// taken before the Save below) and block before returning it.
	time.Sleep(50 * time.Millisecond)

	m := &Memory{Artist: "Landed During Cold Load"}
	if err := store.Save(ctx, m); err != nil {
		t.Fatalf("Save: %v", err)
	}

	close(client.proceed)
	if err := <-listDone; err != nil {
		t.Fatalf("first List failed: %v", err)
	}

	// The first List()'s own return value legitimately missed the concurrent
	// Save -- it's a snapshot taken before the write. What must not happen is
	// that snapshot getting cached, hiding the Save from every later request.
	memories, err := store.List(ctx)
	if err != nil {
		t.Fatalf("second List failed: %v", err)
	}
	found := false
	for _, mm := range memories {
		if mm.ID == m.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the memory saved during the in-flight cold load to be visible on a later List() (got %d memories, none matching id %s) -- the stale pre-Save snapshot was cached instead of being discarded", len(memories), m.ID)
	}
}

// TestMemoryStore_List_CachesEmptyResult guards a store with zero memories
// (a fresh install, or every rule deleted): the loop that builds `memories`
// never appends anything in this case, so it stays nil, and s.cache == nil is
// this store's "not yet loaded" sentinel -- without normalizing to a non-nil
// empty slice before publishing, a genuinely empty store would look
// indistinguishable from an unloaded one and pay the full ListFiles round
// trip on every single List() call forever, never actually caching.
// mockCacheStorageClient is defined in preset_store_test.go (same package).
func TestMemoryStore_List_CachesEmptyResult(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockCacheStorageClient{
		mockStorageClient: newMockStorageClient(),
	}
	store := NewMemoryStore(mockClient, "test-bucket")

	memories, err := store.List(ctx)
	if err != nil {
		t.Fatalf("first List: %v", err)
	}
	if len(memories) != 0 {
		t.Fatalf("expected 0 memories, got %d", len(memories))
	}
	if mockClient.listCalls != 1 {
		t.Fatalf("expected 1 ListFiles call after the first List(), got %d", mockClient.listCalls)
	}

	if _, err := store.List(ctx); err != nil {
		t.Fatalf("second List: %v", err)
	}
	if mockClient.listCalls != 1 {
		t.Errorf("expected the empty result to have been cached -- still 1 ListFiles call after a second List(), got %d", mockClient.listCalls)
	}
}
