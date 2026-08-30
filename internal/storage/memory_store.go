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
	mu     sync.RWMutex
}

// NewMemoryStore creates a new memory store scoped to 'memories/' prefix
func NewMemoryStore(client Client, bucket string) *MemoryStore {
	return &MemoryStore{
		client: client,
		bucket: bucket,
		prefix: "memories/",
	}
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

	if s.cache != nil {
		found := false
		for i, cached := range s.cache {
			if cached.ID == m.ID {
				s.cache[i] = m
				found = true
				break
			}
		}
		if !found {
			s.cache = append(s.cache, m)
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
		return s.cache, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check again after acquiring write lock
	if s.cache != nil {
		return s.cache, nil
	}

	files, err := s.client.ListFiles(ctx, s.bucket, s.prefix)
	if err != nil {
		return nil, err
	}

	var memories []*Memory
	for _, f := range files {
		if strings.HasSuffix(f, ".json") {
			data, err := s.client.ReadFile(ctx, s.bucket, f)
			if err != nil {
				continue
			}
			var m Memory
			if err := json.Unmarshal(data, &m); err == nil {
				memories = append(memories, &m)
			}
		}
	}

	s.cache = memories
	return memories, nil
}

// Delete removes a memory rule from storage
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	objectName := fmt.Sprintf("%s%s.json", s.prefix, id)
	if err := s.client.DeleteFile(ctx, s.bucket, objectName); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache != nil {
		for i, m := range s.cache {
			if m.ID == id {
				s.cache = append(s.cache[:i], s.cache[i+1:]...)
				break
			}
		}
	}

	return nil
}
