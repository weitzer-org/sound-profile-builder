package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/presetsvc"
	"github.com/weitzer-org/sound-builder/internal/runs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// memClient is a map-backed storage.Client fake.
type memClient struct {
	mu sync.Mutex
	m  map[string][]byte
}

func newMemClient() *memClient { return &memClient{m: make(map[string][]byte)} }

func (c *memClient) ReadFile(_ context.Context, bucket, object string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, ok := c.m[bucket+"/"+object]
	if !ok {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), data...), nil
}

func (c *memClient) WriteFile(_ context.Context, bucket, object string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[bucket+"/"+object] = append([]byte(nil), data...)
	return nil
}

func (c *memClient) ListFiles(_ context.Context, bucket, prefix string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	full := bucket + "/" + prefix
	for k := range c.m {
		if strings.HasPrefix(k, full) {
			out = append(out, strings.TrimPrefix(k, bucket+"/"))
		}
	}
	return out, nil
}

func (c *memClient) DeleteFile(_ context.Context, bucket, object string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, bucket+"/"+object)
	return nil
}

func (c *memClient) Close() {}

type fakeSecrets struct{}

func (fakeSecrets) GetPassword(context.Context, string, string) (string, error) { return "key", nil }
func (fakeSecrets) Close()                                                      {}

// blockingOrch is an OrchestratorService whose RunPipeline parks until release
// is closed, so the run stays "running" for the duration of the test.
type blockingOrch struct {
	started chan<- struct{}
	release <-chan struct{}
}

func (o *blockingOrch) RunPipeline(ctx context.Context, _ string, _ map[string]interface{}, _ map[string]string, _ func(string)) (string, *agents.TokenUsage, error) {
	o.started <- struct{}{}
	<-o.release
	return "", nil, errors.New("released by test")
}

func (o *blockingOrch) RefineChat(context.Context, *storage.Preset, string, bool, bool) (string, *agents.TokenUsage, error) {
	return "", nil, errors.New("not used")
}

func (o *blockingOrch) Close() {}

func TestServeAbandonsActiveRunOnStop(t *testing.T) {
	client := newMemClient()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	orch := &blockingOrch{started: started, release: release}

	svc := presetsvc.New(presetsvc.Deps{
		Store:   storage.NewPresetStore(client, "b"),
		Client:  client,
		Bucket:  "b",
		Secrets: func() storage.SecretFetcher { return fakeSecrets{} },
		OrchMaker: func(context.Context, string) (agents.OrchestratorService, error) {
			return orch, nil
		},
		Config: func() *config.AppConfig { return nil },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runID, err := svc.StartGenerate(ctx, "p", presetsvc.GenerateOptions{Mock: true})
	if err != nil {
		t.Fatalf("StartGenerate: %v", err)
	}
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("pipeline never started")
	}

	rec, err := svc.Run(ctx, runID)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rec.Status != runs.StatusRunning {
		t.Fatalf("before stop: status = %q, want %q", rec.Status, runs.StatusRunning)
	}

	srv := &http.Server{Addr: "127.0.0.1:0", Handler: http.NotFoundHandler()}
	serveCtx, stopServe := context.WithCancel(context.Background())
	defer stopServe()
	errCh := make(chan error, 1)
	go func() { errCh <- serve(serveCtx, srv, svc) }()

	stopServe()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve returned %v, want nil", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("serve did not return after ctx cancel")
	}

	live, err := svc.Run(ctx, runID)
	if err != nil {
		t.Fatalf("Run after stop: %v", err)
	}
	if live.Status != runs.StatusAbandoned {
		t.Errorf("in-memory status after stop = %q, want %q", live.Status, runs.StatusAbandoned)
	}

	persisted, err := svc.Runs().Get(ctx, runID)
	if err != nil {
		t.Fatalf("persisted Get: %v", err)
	}
	if persisted.Status != runs.StatusAbandoned {
		t.Errorf("persisted status after stop = %q, want %q", persisted.Status, runs.StatusAbandoned)
	}
}
