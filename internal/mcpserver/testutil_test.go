package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/presetsvc"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

const testBucket = "b"

// testToken is a placeholder, not a secret (64 chars, like `openssl rand -hex 32`).
const testToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// memClient is a mutex-guarded in-memory storage.Client.
type memClient struct {
	mu   sync.Mutex
	objs map[string]map[string][]byte
}

func newMemClient() *memClient { return &memClient{objs: map[string]map[string][]byte{}} }

func (c *memClient) ReadFile(_ context.Context, bucket, object string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.objs[bucket][object]
	if !ok {
		return nil, fmt.Errorf("object not found: %s/%s", bucket, object)
	}
	return append([]byte(nil), d...), nil
}

func (c *memClient) WriteFile(_ context.Context, bucket, object string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.objs[bucket] == nil {
		c.objs[bucket] = map[string][]byte{}
	}
	c.objs[bucket][object] = append([]byte(nil), data...)
	return nil
}

func (c *memClient) ListFiles(_ context.Context, bucket, prefix string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var names []string
	for n := range c.objs[bucket] {
		if strings.HasPrefix(n, prefix) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (c *memClient) DeleteFile(_ context.Context, bucket, object string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.objs[bucket], object)
	return nil
}

func (c *memClient) Close() {}

type svcHarness struct {
	svc   *presetsvc.Service
	store *storage.PresetStore
}

// newTestService builds a presetsvc.Service over an in-memory store. Its
// orchestrator factory always fails: PR 2's tools never start a run.
func newTestService(t *testing.T) *svcHarness {
	t.Helper()
	client := newMemClient()
	store := storage.NewPresetStore(client, testBucket)
	svc := presetsvc.New(presetsvc.Deps{
		Store:    store,
		Memories: storage.NewMemoryStore(client, testBucket),
		Client:   client,
		Bucket:   testBucket,
		Secrets:  func() storage.SecretFetcher { return nil },
		OrchMaker: func(context.Context, string) (agents.OrchestratorService, error) {
			return nil, fmt.Errorf("no orchestrator in this test")
		},
		Config: func() *config.AppConfig { return nil },
	})
	return &svcHarness{svc: svc, store: store}
}

// seedPreset stores a preset whose Payload is the architect mock's structured_payload.
func seedPreset(t *testing.T, h *svcHarness, id, name string) *storage.Preset {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(findRepoRoot(t), "testdata", "e2e_mocks", "architect_generate.json"))
	if err != nil {
		t.Fatal(err)
	}
	payload := extractStructured(t, raw)
	p := &storage.Preset{ID: id, Name: name, Payload: payload, BuilderStatement: "Seeded builder statement."}
	if err := h.store.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return p
}

// extractStructured returns the mock's structured_payload object as the JSON
// string a stored Preset.Payload holds.
func extractStructured(t *testing.T, raw []byte) string {
	t.Helper()
	var m struct {
		SP json.RawMessage `json:"structured_payload"`
	}
	if err := json.Unmarshal(raw, &m); err != nil || len(m.SP) == 0 {
		t.Fatalf("no structured_payload in mock: %v", err)
	}
	return string(m.SP)
}
