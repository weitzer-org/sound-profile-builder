package runs

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// flakyClient fails the first `failures` writes to run records (runs/…), and can
// hold prompt-log writes (logs/…) until released.
type flakyClient struct {
	*memClient
	failures atomic.Int32
	logGate  chan struct{} // when non-nil, logs/ writes block until it is closed
}

func (f *flakyClient) WriteFile(ctx context.Context, bucket, object string, data []byte) error {
	if strings.HasPrefix(object, "runs/") && f.failures.Add(-1) >= 0 {
		return errors.New("transient storage failure")
	}
	if f.logGate != nil && strings.HasPrefix(object, "logs/") {
		<-f.logGate
	}
	return f.memClient.WriteFile(ctx, bucket, object, data)
}

func newRobustRecorder(t *testing.T, c *flakyClient) (*Recorder, *Store) {
	t.Helper()
	store := NewStore(c, "b")
	rec := NewRecorder(store, RunRecord{ID: "gen-robust-1", Kind: KindGenerate, Source: SourceUI})
	return rec, store
}

// A repeated phase (the observer and the legacy callback both report each one)
// must neither wake WaitRunChange waiters nor add an event.
func TestOnPhaseDuplicateIsInvisible(t *testing.T) {
	rec, _ := newRobustRecorder(t, &flakyClient{memClient: newMemClient()})
	var wakes atomic.Int32
	rec.SetOnChange(func() { wakes.Add(1) })

	rec.OnPhase("Phase 1/4")
	rec.OnPhase("Phase 1/4")
	rec.OnPhase("Phase 2/4")
	rec.OnPhase("Phase 2/4")

	if got := wakes.Load(); got != 2 {
		t.Errorf("onChange fired %d times, want 2 (one per real change)", got)
	}
	if n := len(rec.Snapshot().Events); n != 2 {
		t.Errorf("events = %d, want 2", n)
	}
}

// Completion is persisted before waiters are woken, and survives transient
// write failures.
func TestTerminalStateIsPersistedBeforeWakeAndRetried(t *testing.T) {
	c := &flakyClient{memClient: newMemClient()}
	c.failures.Store(2) // the first two record writes fail
	rec, store := newRobustRecorder(t, c)

	var statusAtWake atomic.Value
	rec.SetOnChange(func() {
		if got, err := store.Get(context.Background(), "gen-robust-1"); err == nil {
			statusAtWake.Store(got.Status)
		}
	})

	if !rec.Complete("preset-1") {
		t.Fatal("Complete reported no transition")
	}
	if got, _ := statusAtWake.Load().(string); got != StatusComplete {
		t.Errorf("status visible in storage when waiters woke = %q, want complete", got)
	}
	got, err := store.Get(context.Background(), "gen-robust-1")
	if err != nil || got.Status != StatusComplete || got.PresetID != "preset-1" {
		t.Errorf("stored record = %+v, %v; want complete with the preset ID despite 2 failed writes", got, err)
	}
}

// Shutdown must not wait for slow prompt-log writes.
func TestAbandonDoesNotWaitForPromptLogs(t *testing.T) {
	c := &flakyClient{memClient: newMemClient(), logGate: make(chan struct{})}
	rec, store := newRobustRecorder(t, c)
	rec.OnAgentStart("Tone Historian", "sys", "user") // its log writes now block on the gate

	done := make(chan bool, 1)
	go func() { done <- rec.Abandon("server stopped") }()
	select {
	case ok := <-done:
		if !ok {
			t.Error("Abandon reported no transition")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Abandon blocked on in-flight prompt-log writes")
	}
	if got, err := store.Get(context.Background(), "gen-robust-1"); err != nil || got.Status != StatusAbandoned {
		t.Errorf("stored record = %+v, %v; want abandoned", got, err)
	}
	close(c.logGate) // let the log goroutine finish
}

// Every attempt persists the record, so a long fallback chain keeps UpdatedAt
// fresh and a reader never judges a live run abandoned.
func TestAttemptPersistsAHeartbeat(t *testing.T) {
	c := &flakyClient{memClient: newMemClient()}
	rec, store := newRobustRecorder(t, c)
	rec.OnAgentStart("Tone Historian", "", "")
	before := rec.Snapshot().UpdatedAt

	time.Sleep(5 * time.Millisecond)
	rec.OnAttempt("Tone Historian", "gemini-3.1-pro-preview", 1000, errors.New("overloaded"))

	got, err := store.Get(context.Background(), "gen-robust-1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.After(before) || got.Agents["Tone Historian"].Attempts != 1 {
		t.Errorf("stored record not refreshed by the attempt: %+v", got)
	}
}

func TestCloneDoesNotShareConstraintSlices(t *testing.T) {
	orig := RunRecord{Constraints: map[string]any{"available_plugins": []string{"Cory Wong"}}}
	c := orig.Clone()
	c.Constraints["available_plugins"].([]string)[0] = "changed"
	if got := orig.Constraints["available_plugins"].([]string)[0]; got != "Cory Wong" {
		t.Errorf("mutating a clone changed the original: %q", got)
	}
}
