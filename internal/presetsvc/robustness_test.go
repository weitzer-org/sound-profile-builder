package presetsvc

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/runs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// failingClient fails writes whose object name has failPrefix, and can stall all
// run-record writes until released.
type failingClient struct {
	*memClient
	failPrefix string
	stall      chan struct{}
	stalled    atomic.Bool
}

func (f *failingClient) WriteFile(ctx context.Context, bucket, object string, data []byte) error {
	if f.failPrefix != "" && strings.HasPrefix(object, f.failPrefix) {
		return errors.New("storage unavailable")
	}
	if f.stall != nil && f.stalled.Load() && strings.HasPrefix(object, "runs/") {
		<-f.stall
	}
	return f.memClient.WriteFile(ctx, bucket, object, data)
}

// Two refinements of one preset would each save a whole preset, and the later
// save would silently drop the earlier change.
func TestRefineOfTheSamePresetIsRejectedWhileOneIsInFlight(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	a := &storage.Preset{Name: "A"}
	b := &storage.Preset{Name: "B"}
	for _, p := range []*storage.Preset{a, b} {
		if err := h.store.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	h.orch.hold(false)
	first, err := h.svc.StartRefine(ctx, a.ID, "brighter", true)
	if err != nil {
		t.Fatalf("first StartRefine: %v", err)
	}
	if _, err := h.svc.StartRefine(ctx, a.ID, "warmer", true); !errors.Is(err, ErrPresetBusy) {
		t.Fatalf("second refine of the same preset: err = %v, want ErrPresetBusy", err)
	}
	if got := len(h.svc.sem); got != 1 {
		t.Errorf("run slots in use = %d, want 1 (the rejected start must give its slot back)", got)
	}
	// A different preset is independent.
	if _, err := h.svc.StartRefine(ctx, b.ID, "warmer", true); err != nil {
		t.Errorf("refine of another preset: %v", err)
	}

	h.orch.release()
	if rec, err := h.svc.WaitRun(ctx, first, 10*time.Second); err != nil || rec.Status != runs.StatusComplete {
		t.Fatalf("first refine = %+v, %v", rec, err)
	}
	waitIdle(t, h.svc)

	// And the guard is released afterwards.
	again, err := h.svc.StartRefine(ctx, a.ID, "later", true)
	if err != nil {
		t.Fatalf("refine after completion: %v", err)
	}
	if rec, _ := h.svc.WaitRun(ctx, again, 10*time.Second); rec == nil || rec.Status != runs.StatusComplete {
		t.Fatalf("follow-up refine = %+v", rec)
	}
	got, _ := h.store.Get(ctx, a.ID)
	if len(got.RunIDs) != 2 {
		t.Errorf("preset.RunIDs = %v, want both refinements recorded", got.RunIDs)
	}
}

// A refinement whose result couldn't be saved must not report success.
func TestRefineFailsWhenThePresetCannotBeSaved(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	p := &storage.Preset{Name: "A"}
	if err := h.store.Save(ctx, p); err != nil {
		t.Fatal(err)
	}
	// From here on, writes to presets/ fail; run records still work.
	h.svc.d.Store = storage.NewPresetStore(&failingClient{memClient: h.client, failPrefix: "presets/"}, testBucket)

	id, err := h.svc.StartRefine(ctx, p.ID, "brighter", true)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := h.svc.WaitRun(ctx, id, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != runs.StatusError || !strings.Contains(rec.Error, "Storage Error") {
		t.Errorf("run = %q / %q, want an error naming the storage failure", rec.Status, rec.Error)
	}
	res, _ := h.svc.Result(id)
	if res == nil || res.ErrKind != ErrKindStorage || res.Preset != nil {
		t.Errorf("result = %+v, want ErrKindStorage and no preset", res)
	}
}

// Shutdown must not hang on a stalled storage backend.
func TestAbandonActiveIsBoundedWhenStorageStalls(t *testing.T) {
	h := newHarness(t)
	fc := &failingClient{memClient: h.client, stall: make(chan struct{})}
	t.Cleanup(func() {
		if fc.stalled.Swap(false) {
			close(fc.stall)
		}
	})
	h.svc.runs = runs.NewStore(fc, testBucket)

	h.orch.hold(false)
	generate(t, h, GenerateOptions{Mock: true})
	fc.stalled.Store(true) // every run-record write from now on hangs

	old := abandonDeadline
	abandonDeadline = 150 * time.Millisecond
	t.Cleanup(func() { abandonDeadline = old })

	start := time.Now()
	h.svc.AbandonActive("server stopped")
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("AbandonActive took %v with storage stalled, want it bounded by the deadline", elapsed)
	}
}

// The fakes elsewhere never call the observer, so this runs the real
// orchestrator in mock mode and checks what the run log ends up holding.
func TestMockRunRecordsAgentTracesAndPromptLogs(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.svc.d.OrchMaker = func(ctx context.Context, key string) (agents.OrchestratorService, error) {
		return agents.NewOrchestrator(ctx, "fake-key", nil)
	}

	id := generate(t, h, GenerateOptions{Mock: true})
	rec, err := h.svc.WaitRun(ctx, id, 15*time.Second)
	if err != nil || rec.Status != runs.StatusComplete {
		t.Fatalf("run = %+v, %v", rec, err)
	}

	stored, err := h.svc.Runs().Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for name, r := range map[string]*runs.RunRecord{"in memory": rec, "persisted": stored} {
		if len(r.Agents) != 13 || r.Totals.Calls != 13 {
			t.Errorf("%s: %d agent traces / %d calls, want 13 / 13", name, len(r.Agents), r.Totals.Calls)
		}
		if tr := r.Agents["CorOS Librarian"]; tr == nil || tr.PromptKey != "4_coros_librarian" || tr.Status != "done" || tr.Attempts != 1 ||
			tr.SystemPromptChars == 0 || tr.InputChars == 0 || tr.OutputChars == 0 || tr.InputTokens == 0 {
			t.Errorf("%s: Librarian trace = %+v", name, tr)
		}
		for i, ev := range r.Events {
			if ev.Seq != i+1 {
				t.Errorf("%s: event %d has Seq %d", name, i, ev.Seq)
				break
			}
		}
	}
	// Prompt logs are durable by the time the run reads as complete.
	for _, obj := range []string{"4_coros_librarian.system.md", "4_coros_librarian.input.txt", "12_architect.system.md"} {
		if !h.client.has(testBucket, "logs/"+id+"/"+obj) {
			t.Errorf("missing prompt log logs/%s/%s", id, obj)
		}
	}
}
