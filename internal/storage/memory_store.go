package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Memory represents a learned rule from user alterations
type Memory struct {
	ID         string `json:"id"`
	Artist     string `json:"artist"` // Topic/Concept
	Critique   string `json:"critique"`
	Action     string `json:"action"`
	BasePreset string `json:"base_preset"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// MemoryStore manages persistence of learned rules in GCS
type MemoryStore struct {
	client Client
	bucket string
	prefix string
	cache  []*Memory
	// epoch increments inside the lock on every Save/Delete, whether or not
	// s.cache is currently populated. List's cold load runs its network I/O
	// with the lock released (see List), so a Save/Delete can land while a
	// load is in flight; without this, that mutation would go unrecorded (it
	// only touches s.cache when s.cache != nil) and the load would then
	// overwrite s.cache == nil with a now-stale snapshot that's permanently
	// missing it. Comparing the epoch captured before the fetch against the
	// current epoch before committing the result detects that and skips the
	// write instead, so the next List() retries the fetch.
	epoch uint64
	mu    sync.RWMutex
}

// NewMemoryStore creates a new memory store scoped to 'memories/' prefix
func NewMemoryStore(client Client, bucket string) *MemoryStore {
	return &MemoryStore{
		client: client,
		bucket: bucket,
		prefix: "memories/",
	}
}

// cloneMemories returns an independent copy of ms, both the slice and each Memory
// it points to. s.cache must never be handed out directly: the RWMutex only
// guards the read of the s.cache field itself, and once a slice/pointer escapes
// past RUnlock a concurrent Save/Delete mutating that same backing array (or the
// pointed-to Memory) races with a caller still reading it.
func cloneMemories(ms []*Memory) []*Memory {
	out := make([]*Memory, len(ms))
	for i, m := range ms {
		clone := *m
		out[i] = &clone
	}
	return out
}

// Save creates or updates a memory rule
func (s *MemoryStore) Save(ctx context.Context, m *Memory) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.CreatedAt == "" {
		m.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}

	objectName := fmt.Sprintf("%s%s.json", s.prefix, m.ID)
	if err := s.client.WriteFile(ctx, s.bucket, objectName, data); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.epoch++
	if s.cache != nil {
		// Store our own copy, not the caller's pointer -- m is caller-owned and
		// mutating it after Save returns must not silently corrupt the cache
		// (see cloneMemories).
		cached := *m
		found := false
		for i, existing := range s.cache {
			if existing.ID == m.ID {
				s.cache[i] = &cached
				found = true
				break
			}
		}
		if !found {
			s.cache = append(s.cache, &cached)
		}
	}

	return nil
}

// Get retrieves a specific memory rule by ID
func (s *MemoryStore) Get(ctx context.Context, id string) (*Memory, error) {
	objectName := fmt.Sprintf("%s%s.json", s.prefix, id)
	data, err := s.client.ReadFile(ctx, s.bucket, objectName)
	if err != nil {
		return nil, err
	}

	var m Memory
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// List retrieves all memory rules. Results are cached after the first successful
// load -- unlike presets, memories are read on every dashboard tab switch but
// change rarely, and List's per-file ListFiles+GetObject cost (no batch-get API)
// made the uncached path reliably blow a tight request deadline in practice.
func (s *MemoryStore) List(ctx context.Context) ([]*Memory, error) {
	s.mu.RLock()
	if s.cache != nil {
		defer s.mu.RUnlock()
		return cloneMemories(s.cache), nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	// Check again after acquiring the lock -- a concurrent cold load may have
	// already finished while we were waiting for it.
	if s.cache != nil {
		res := cloneMemories(s.cache)
		s.mu.Unlock()
		return res, nil
	}
	// Release the lock before the network I/O below: ListFiles+N sequential
	// GetObjects can run for seconds, and holding s.mu across that would block
	// every other Save/Delete/List for the duration -- the mutex must only
	// protect the s.cache field itself, not the fetch that populates it. A cold
	// start can now run this fetch more than once concurrently (no coalescing);
	// accepted as a one-time cost on process boot rather than added complexity
	// (e.g. singleflight) this single-tenant dashboard doesn't need.
	loadEpoch := s.epoch
	s.mu.Unlock()

	files, err := s.client.ListFiles(ctx, s.bucket, s.prefix)
	if err != nil {
		return nil, err
	}

	var memories []*Memory
	complete := true // every object under the prefix loaded and parsed cleanly
	for _, f := range files {
		if strings.HasSuffix(f, ".json") {
			data, err := s.client.ReadFile(ctx, s.bucket, f)
			if err != nil {
				complete = false
				continue
			}
			var m Memory
			if err := json.Unmarshal(data, &m); err == nil {
				memories = append(memories, &m)
			} else {
				complete = false
			}
		}
	}

	// Only cache a load that fetched every object cleanly. A transient failure
	// on a single GetObject (network blip, eventual-consistency gap right after
	// a write) would otherwise get cached as if it were the true state and keep
	// serving that same incomplete list from every subsequent request until the
	// process restarts, even after storage recovers. The single-call return
	// still surfaces the best-effort partial list rather than an error, same as
	// before this cache was added.
	if complete {
		s.mu.Lock()
		// Only set the cache if nothing else has: a concurrent load may have
		// already populated it (s.cache != nil), or a concurrent Save/Delete may
		// have landed while we were fetching (s.epoch != loadEpoch) -- in the
		// latter case our snapshot is missing that mutation, so leave the cache
		// unset and let the next List() retry the fetch rather than caching a
		// permanently-stale result.
		if s.cache == nil && s.epoch == loadEpoch {
			s.cache = memories
		}
		s.mu.Unlock()
	}
	return cloneMemories(memories), nil
}

// Delete removes a memory rule from storage
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	objectName := fmt.Sprintf("%s%s.json", s.prefix, id)
	if err := s.client.DeleteFile(ctx, s.bucket, objectName); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.epoch++
	if s.cache != nil {
		for i, m := range s.cache {
			if m.ID == id {
				// Zero the vacated tail slot before truncating -- shifting left
				// with append alone shrinks the length but leaves the backing
				// array's now-unreachable-by-index last slot still pointing at
				// the deleted *Memory, keeping it alive for the GC as long as
				// this backing array is (i.e. until the next append reuses it).
				copy(s.cache[i:], s.cache[i+1:])
				s.cache[len(s.cache)-1] = nil
				s.cache = s.cache[:len(s.cache)-1]
				break
			}
		}
	}

	return nil
}
