package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"
)

// slowClient mimics R2 round trips: reads honour ctx, listing is lexical, and a delete
// returns only after its latency (the object is already gone by then).
type slowClient struct {
	*mockStorageClient
	read, del time.Duration
}

func (s slowClient) ReadFile(ctx context.Context, b, o string) ([]byte, error) {
	select {
	case <-time.After(s.read):
		return s.mockStorageClient.ReadFile(ctx, b, o)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s slowClient) ListFiles(ctx context.Context, b, p string) ([]string, error) {
	f, err := s.mockStorageClient.ListFiles(ctx, b, p)
	sort.Strings(f)
	return f, err
}

func (s slowClient) DeleteFile(ctx context.Context, b, o string) error {
	err := s.mockStorageClient.DeleteFile(ctx, b, o)
	time.Sleep(s.del)
	return err
}

// A request deadline shorter than the first load must not leave a shortened list cached
// (a saved preset vanished from list_presets and the UI while get-by-id still worked).
func TestPresetStore_ListDeadlineDoesNotPoisonCache(t *testing.T) {
	bg := context.Background()
	st := NewPresetStore(slowClient{newMockStorageClient(), 40 * time.Millisecond, 0}, "b")
	for i := 0; i < 24; i++ {
		_ = st.Save(bg, &Preset{ID: fmt.Sprintf("p%02d", i), Name: "old"})
	}
	_ = st.Save(bg, &Preset{ID: "p18-saved", Name: "Saved"}) // cache still nil

	ctx, cancel := context.WithTimeout(bg, 50*time.Millisecond) // far less than the load
	l, err := st.List(ctx)
	cancel()
	if err != nil || len(l) != 25 {
		t.Fatalf("List under a short deadline: %d presets, err=%v; want all 25", len(l), err)
	}
	_ = st.Save(bg, &Preset{ID: "p11", Name: "Draft Preset"})
	l, _ = st.List(bg)
	found := false
	for _, p := range l {
		found = found || p.ID == "p18-saved"
	}
	if !found || len(l) != 25 {
		t.Fatalf("later List: %d presets, saved listed=%v", len(l), found)
	}
}

// A failed read during the load must not be cached.
type flakyRead struct {
	*mockStorageClient
	mu    sync.Mutex
	fails int
}

func (f *flakyRead) ReadFile(ctx context.Context, b, o string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fails > 0 {
		f.fails--
		return nil, fmt.Errorf("transient")
	}
	return f.mockStorageClient.ReadFile(ctx, b, o)
}

func TestPresetStore_ListReadErrorIsNotCached(t *testing.T) {
	bg := context.Background()
	c := &flakyRead{mockStorageClient: newMockStorageClient()}
	st := NewPresetStore(c, "b")
	for i := 0; i < 3; i++ {
		_ = st.Save(bg, &Preset{ID: fmt.Sprintf("p%d", i), Name: "x"})
	}
	c.fails = 1
	if _, err := st.List(bg); err == nil {
		t.Fatal("expected the transient read error to surface")
	}
	l, err := st.List(bg)
	if err != nil || len(l) != 3 {
		t.Fatalf("retry: %d presets, err=%v; want 3", len(l), err)
	}
}

// Delete racing a Save (the draft cleanup vs. save_preset) must leave cache and storage
// in agreement.
func TestPresetStore_DeleteRacingSaveStaysConsistent(t *testing.T) {
	bg := context.Background()
	for i := 0; i < 50; i++ {
		st := NewPresetStore(slowClient{newMockStorageClient(), 0, 5 * time.Millisecond}, "b")
		_ = st.Save(bg, &Preset{ID: "x", Name: "Draft Preset"})
		_, _ = st.List(bg)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _ = st.Delete(bg, "x") }()
		go func() {
			defer wg.Done()
			time.Sleep(2 * time.Millisecond)
			_ = st.Save(bg, &Preset{ID: "x", Name: "Muddy"})
		}()
		wg.Wait()
		ex, _ := st.Exists(bg, "x")
		l, _ := st.List(bg)
		if ex != (len(l) == 1) {
			t.Fatalf("run %d: object exists=%v but List has %d", i, ex, len(l))
		}
	}
}

// DeleteIf re-checks the stored name, so a draft that has been named since is kept.
func TestPresetStore_DeleteIfSkipsRenamed(t *testing.T) {
	bg := context.Background()
	st := NewPresetStore(newMockStorageClient(), "b")
	_ = st.Save(bg, &Preset{ID: "a", Name: "Muddy"})
	isDraft := func(p *Preset) bool { return p.Name == "Draft Preset" }
	if ok, err := st.DeleteIf(bg, "a", isDraft); ok || err != nil {
		t.Fatalf("deleted a renamed preset: ok=%v err=%v", ok, err)
	}
	if ex, _ := st.Exists(bg, "a"); !ex {
		t.Fatal("renamed preset was removed")
	}
	_ = st.Save(bg, &Preset{ID: "b", Name: "Draft Preset"})
	if ok, _ := st.DeleteIf(bg, "b", isDraft); !ok {
		t.Fatal("draft not deleted")
	}
	if ok, err := st.DeleteIf(bg, "gone", isDraft); ok || err != nil {
		t.Fatalf("missing preset: ok=%v err=%v", ok, err)
	}
}
