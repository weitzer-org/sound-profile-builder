package presetsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/runs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

const testBucket = "b"

// ---- fixtures ---------------------------------------------------------------

var (
	fixturesOnce   sync.Once
	fixtureGen     []byte
	fixtureRefine  []byte
	fixtureLoadErr error
)

// loadFixtures reads the architect mocks once; they are shared read-only.
func loadFixtures(t *testing.T) ([]byte, []byte) {
	t.Helper()
	fixturesOnce.Do(func() {
		fixtureGen, fixtureLoadErr = os.ReadFile("../../testdata/e2e_mocks/architect_generate.json")
		if fixtureLoadErr != nil {
			return
		}
		fixtureRefine, fixtureLoadErr = os.ReadFile("../../testdata/e2e_mocks/architect_refine.json")
	})
	if fixtureLoadErr != nil {
		t.Fatalf("loading e2e mocks: %v", fixtureLoadErr)
	}
	return fixtureGen, fixtureRefine
}

// ---- fake storage.Client ----------------------------------------------------

// memClient is a mutex-guarded in-memory object store: bucket -> object -> bytes.
type memClient struct {
	mu   sync.Mutex
	objs map[string]map[string][]byte
}

func newMemClient() *memClient {
	return &memClient{objs: make(map[string]map[string][]byte)}
}

func (c *memClient) ReadFile(_ context.Context, bucket, object string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, ok := c.objs[bucket][object]
	if !ok {
		return nil, fmt.Errorf("object not found: %s/%s", bucket, object)
	}
	return append([]byte(nil), data...), nil
}

func (c *memClient) WriteFile(_ context.Context, bucket, object string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.objs[bucket] == nil {
		c.objs[bucket] = make(map[string][]byte)
	}
	c.objs[bucket][object] = append([]byte(nil), data...)
	return nil
}

// ListFiles returns object names (not bucket-qualified) with the given prefix.
func (c *memClient) ListFiles(_ context.Context, bucket, prefix string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var names []string
	for name := range c.objs[bucket] {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
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

func (c *memClient) has(bucket, object string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.objs[bucket][object]
	return ok
}

// ---- fake storage.SecretFetcher --------------------------------------------

type fakeSecrets struct {
	key string
	err error // when set, GetPassword fails with it
}

func (f *fakeSecrets) GetPassword(_ context.Context, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.key, nil
}

func (f *fakeSecrets) Close() {}

// ---- fake agents.OrchestratorService ---------------------------------------

// fakeOrch returns the architect fixtures. While a gate is held, RunPipeline and
// RefineChat block until the test releases it. With emitLoop set, the pipeline
// keeps reporting distinct phases on a short ticker until released, so a
// WaitRunChange that starts at any moment still sees a change.
type fakeOrch struct {
	gen    []byte
	refine []byte

	mu   sync.Mutex
	gate chan struct{} // nil means never block
	// refineVersions is the prompt-version map the last RefineChat call's ctx carried.
	refineVersions map[string]string
	emitLoop       bool
	released       bool
}

// hold makes subsequent pipeline calls block until release.
func (f *fakeOrch) hold(emitLoop bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gate = make(chan struct{})
	f.emitLoop = emitLoop
	f.released = false
}

// release unblocks held calls. Safe to call repeatedly or with no gate.
func (f *fakeOrch) release() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gate != nil && !f.released {
		close(f.gate)
		f.released = true
	}
}

// block reports "Phase X" through onProgress (when given) and then waits on the
// gate, if one is held.
func (f *fakeOrch) block(onProgress func(string)) {
	f.mu.Lock()
	gate, loop := f.gate, f.emitLoop
	f.mu.Unlock()
	if gate == nil {
		return
	}
	if onProgress != nil {
		onProgress("Phase X")
	}
	if !loop || onProgress == nil {
		<-gate
		return
	}
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for i := 2; ; i++ {
		select {
		case <-gate:
			return
		case <-tick.C:
			onProgress(fmt.Sprintf("Phase X %d", i))
		}
	}
}

func (f *fakeOrch) RunPipeline(_ context.Context, _ string, _ map[string]interface{}, _ map[string]string, onProgress func(phase string)) (string, *agents.TokenUsage, error) {
	f.block(onProgress)
	return string(f.gen), &agents.TokenUsage{ModelsUsed: map[string]int{}}, nil
}

func (f *fakeOrch) RefineChat(ctx context.Context, _ *storage.Preset, _ string, _, _ bool) (string, *agents.TokenUsage, error) {
	f.mu.Lock()
	f.refineVersions = agents.PromptVersionsFrom(ctx)
	f.mu.Unlock()
	f.block(nil)
	return string(f.refine), &agents.TokenUsage{ModelsUsed: map[string]int{}}, nil
}

func (f *fakeOrch) Close() {}

// ---- harness ----------------------------------------------------------------

type harness struct {
	svc      *Service
	client   *memClient
	store    *storage.PresetStore
	memories *storage.MemoryStore
	orch     *fakeOrch
	secrets  *fakeSecrets
	orchErr  error // when set, OrchMaker fails with it
}

// newHarness builds a Service over fakes. Any held fake is released, and the
// Service's run slots drained, at test cleanup so no goroutine stays blocked.
func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWithConfig(t, nil)
}

// newHarnessWithConfig is newHarness with a fixed app config (nil = none).
func newHarnessWithConfig(t *testing.T, cfg *config.AppConfig) *harness {
	t.Helper()
	gen, refine := loadFixtures(t)
	h := &harness{
		client:  newMemClient(),
		orch:    &fakeOrch{gen: gen, refine: refine},
		secrets: &fakeSecrets{key: "key"},
	}
	h.store = storage.NewPresetStore(h.client, testBucket)
	h.memories = storage.NewMemoryStore(h.client, testBucket)
	h.svc = New(Deps{
		Store:    h.store,
		Memories: h.memories,
		Client:   h.client,
		Bucket:   testBucket,
		Secrets:  func() storage.SecretFetcher { return h.secrets },
		OrchMaker: func(context.Context, string) (agents.OrchestratorService, error) {
			if h.orchErr != nil {
				return nil, h.orchErr
			}
			return h.orch, nil
		},
		Config: func() *config.AppConfig { return cfg },
	})
	t.Cleanup(func() {
		h.orch.release()
		waitIdle(t, h.svc)
	})
	return h
}

// waitIdle waits until every run slot has been released, i.e. no pipeline
// goroutine is still executing.
func waitIdle(t *testing.T, s *Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for len(s.sem) > 0 {
		if time.Now().After(deadline) {
			t.Errorf("%d run slot(s) still held after 5s", len(s.sem))
			return
		}
		time.Sleep(time.Millisecond)
	}
}

func generate(t *testing.T, h *harness, o GenerateOptions) string {
	t.Helper()
	id, err := h.svc.StartGenerate(context.Background(), "warm lead tone", o)
	if err != nil {
		t.Fatalf("StartGenerate: %v", err)
	}
	return id
}

// ---- tests ------------------------------------------------------------------

func TestGenerateCompletes(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	runID := generate(t, h, GenerateOptions{Mock: true, Source: runs.SourceMCP})
	if !strings.HasPrefix(runID, "gen-") {
		t.Errorf("run ID %q does not start with gen-", runID)
	}

	rec, err := h.svc.WaitRun(ctx, runID, 10*time.Second)
	if err != nil {
		t.Fatalf("WaitRun: %v", err)
	}
	if rec.Status != runs.StatusComplete {
		t.Fatalf("status = %q (error %q), want complete", rec.Status, rec.Error)
	}
	if rec.PresetID == "" {
		t.Fatal("completed record has no PresetID")
	}
	if !rec.Mock {
		t.Error("record.Mock = false, want true")
	}

	p, err := h.store.Get(ctx, rec.PresetID)
	if err != nil {
		t.Fatalf("store.Get(%s): %v", rec.PresetID, err)
	}
	if want := []string{runID}; !reflect.DeepEqual(p.RunIDs, want) {
		t.Errorf("preset.RunIDs = %v, want %v", p.RunIDs, want)
	}

	stored, err := h.svc.Runs().Get(ctx, runID)
	if err != nil {
		t.Fatalf("Runs().Get: %v", err)
	}
	if stored.Status != runs.StatusComplete {
		t.Errorf("persisted status = %q, want complete", stored.Status)
	}
	if !h.client.has(testBucket, "runs/"+runID+".json") {
		t.Errorf("runs/%s.json was not written to storage", runID)
	}
}

func TestConcurrencyCapReturnsErrBusyAndReleasesSlots(t *testing.T) {
	t.Setenv("MCP_MAX_CONCURRENT_RUNS", "2")
	h := newHarness(t)
	ctx := context.Background()
	opts := GenerateOptions{Mock: true}

	h.orch.hold(false)
	id1 := generate(t, h, opts)
	id2 := generate(t, h, opts)

	if _, err := h.svc.StartGenerate(ctx, "third", opts); !errors.Is(err, ErrBusy) {
		t.Fatalf("third StartGenerate err = %v, want ErrBusy", err)
	}

	h.orch.release()
	for _, id := range []string{id1, id2} {
		rec, err := h.svc.WaitRun(ctx, id, 10*time.Second)
		if err != nil {
			t.Fatalf("WaitRun(%s): %v", id, err)
		}
		if rec.Status != runs.StatusComplete {
			t.Fatalf("run %s status = %q (error %q), want complete", id, rec.Status, rec.Error)
		}
	}
	waitIdle(t, h.svc)

	id3, err := h.svc.StartGenerate(ctx, "after release", opts)
	if err != nil {
		t.Fatalf("StartGenerate after release: %v", err)
	}
	rec, err := h.svc.WaitRun(ctx, id3, 10*time.Second)
	if err != nil {
		t.Fatalf("WaitRun(%s): %v", id3, err)
	}
	if rec.Status != runs.StatusComplete {
		t.Errorf("run %s status = %q, want complete", id3, rec.Status)
	}
}

// WaitRun must wake as soon as the run finishes, not sleep until max.
func TestWaitRunWakesOnCompletion(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	h.orch.hold(false)
	runID := generate(t, h, GenerateOptions{Mock: true})

	type result struct {
		rec *runs.RunRecord
		err error
	}
	done := make(chan result, 1)
	go func() {
		rec, err := h.svc.WaitRun(ctx, runID, 10*time.Second)
		done <- result{rec, err}
	}()

	h.orch.release()

	var res result
	select {
	case res = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WaitRun did not return after the run completed")
	}
	// No wall-clock bound here: the select above already fails the test if WaitRun
	// slept toward its 10s max instead of waking on completion.
	if res.err != nil {
		t.Fatalf("WaitRun: %v", res.err)
	}
	if res.rec.Status != runs.StatusComplete {
		t.Errorf("status = %q, want complete", res.rec.Status)
	}
}

// WaitRunChange must return on the first phase event while the run still runs.
// The fake emits phases on a ticker while held, so whichever phase lands after
// the wait starts wakes it promptly.
func TestWaitRunChangeWakesOnPhase(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	h.orch.hold(true)
	runID := generate(t, h, GenerateOptions{Mock: true})

	start := time.Now()
	rec, err := h.svc.WaitRunChange(ctx, runID, 10*time.Second)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("WaitRunChange: %v", err)
	}
	if elapsed >= 5*time.Second { // generous: only distinguishes "woke on change" from "slept toward the 10s max"
		t.Errorf("WaitRunChange took %v, want it to return on the first phase change", elapsed)
	}
	if rec.Status != runs.StatusRunning {
		t.Errorf("status = %q, want running", rec.Status)
	}
	if !strings.HasPrefix(rec.Phase, "Phase X") {
		t.Errorf("phase = %q, want a \"Phase X\" change", rec.Phase)
	}
}

// WaitRun with a short max on a blocked run returns the running record after
// about max.
func TestWaitRunTimesOutWhileRunning(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	h.orch.hold(false)
	runID := generate(t, h, GenerateOptions{Mock: true})

	const max = 50 * time.Millisecond
	start := time.Now()
	rec, err := h.svc.WaitRun(ctx, runID, max)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("WaitRun: %v", err)
	}
	if rec.Status != runs.StatusRunning {
		t.Errorf("status = %q, want running", rec.Status)
	}
	if elapsed < max {
		t.Errorf("WaitRun returned after %v, before max %v", elapsed, max)
	}
	if elapsed > 5*time.Second { // generous CI headroom; the lower bound above is the real check
		t.Errorf("WaitRun returned after %v, want about %v", elapsed, max)
	}
}

func TestRefineAppendsRunID(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	p := &storage.Preset{Name: "Warm Lead"}
	if err := h.store.Save(ctx, p); err != nil {
		t.Fatalf("store.Save: %v", err)
	}

	runID, err := h.svc.StartRefine(ctx, p.ID, "make it brighter", true)
	if err != nil {
		t.Fatalf("StartRefine: %v", err)
	}
	if !strings.HasPrefix(runID, "chat-") {
		t.Errorf("run ID %q does not start with chat-", runID)
	}

	rec, err := h.svc.WaitRun(ctx, runID, 10*time.Second)
	if err != nil {
		t.Fatalf("WaitRun: %v", err)
	}
	if rec.Status != runs.StatusComplete {
		t.Fatalf("status = %q (error %q), want complete", rec.Status, rec.Error)
	}
	if rec.ParentPresetID != p.ID {
		t.Errorf("ParentPresetID = %q, want %q", rec.ParentPresetID, p.ID)
	}

	got, err := h.store.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if want := []string{runID}; !reflect.DeepEqual(got.RunIDs, want) {
		t.Errorf("preset.RunIDs = %v, want %v", got.RunIDs, want)
	}
}

// RefineChat used to load Architect v4 whatever config.json said; the run must now hand it
// the configured versions.
func TestRefinePassesConfiguredPromptVersions(t *testing.T) {
	h := newHarnessWithConfig(t, &config.AppConfig{AgentPrompts: map[string]string{"12_architect": "v5"}})
	ctx := context.Background()
	p := &storage.Preset{Name: "Warm Lead"}
	if err := h.store.Save(ctx, p); err != nil {
		t.Fatalf("store.Save: %v", err)
	}
	runID, err := h.svc.StartRefine(ctx, p.ID, "make it brighter", true)
	if err != nil {
		t.Fatalf("StartRefine: %v", err)
	}
	if _, err := h.svc.WaitRun(ctx, runID, 10*time.Second); err != nil {
		t.Fatalf("WaitRun: %v", err)
	}
	h.orch.mu.Lock()
	got := h.orch.refineVersions["12_architect"]
	h.orch.mu.Unlock()
	if got != "v5" {
		t.Errorf("RefineChat saw 12_architect = %q, want the configured v5", got)
	}
}

func TestRefineUnknownPresetIsLookupError(t *testing.T) {
	h := newHarness(t)

	_, err := h.svc.StartRefine(context.Background(), "no-such-preset", "make it brighter", true)
	var lookup *LookupError
	if !errors.As(err, &lookup) {
		t.Fatalf("StartRefine err = %v (%T), want *LookupError", err, err)
	}
}

func TestMissingCredentialsIsPublicError(t *testing.T) {
	t.Setenv("USE_OPENLLM", "")
	t.Setenv("GEMINI_API_KEY_NAME", "")

	h := newHarness(t)
	h.secrets.err = errors.New("secret not found")

	_, err := h.svc.StartGenerate(context.Background(), "warm lead tone", GenerateOptions{Mock: true})
	var creds *CredentialsError
	if !errors.As(err, &creds) {
		t.Fatalf("StartGenerate err = %v (%T), want *CredentialsError", err, err)
	}
	if creds.Public != "Missing Secure AI Credentials" {
		t.Errorf("Public = %q, want %q", creds.Public, "Missing Secure AI Credentials")
	}
}

func TestOrchestratorInitFailureRecordsErrKindInit(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.orchErr = errors.New("adk unavailable")

	runID := generate(t, h, GenerateOptions{Mock: true})
	rec, err := h.svc.WaitRun(ctx, runID, 10*time.Second)
	if err != nil {
		t.Fatalf("WaitRun: %v", err)
	}
	if rec.Status != runs.StatusError {
		t.Fatalf("status = %q, want error", rec.Status)
	}

	res, ok := h.svc.Result(runID)
	if !ok {
		t.Fatal("Result: run not cached")
	}
	if res.ErrKind != ErrKindInit {
		t.Errorf("ErrKind = %q, want %q", res.ErrKind, ErrKindInit)
	}
	if res.Preset != nil {
		t.Errorf("Preset = %+v, want nil for a failed run", res.Preset)
	}
}

func TestAbandonActiveMarksBlockedRun(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	h.orch.hold(false)
	runID := generate(t, h, GenerateOptions{Mock: true})

	if n := h.svc.AbandonActive("x"); n != 1 {
		t.Fatalf("AbandonActive returned %d, want 1", n)
	}

	rec, err := h.svc.Run(ctx, runID)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rec.Status != runs.StatusAbandoned {
		t.Errorf("in-memory status = %q, want abandoned", rec.Status)
	}

	stored, err := h.svc.Runs().Get(ctx, runID)
	if err != nil {
		t.Fatalf("Runs().Get: %v", err)
	}
	if stored.Status != runs.StatusAbandoned {
		t.Errorf("persisted status = %q, want abandoned", stored.Status)
	}
}

// Save must refuse while a refinement of the same preset is in flight (the
// refinement would otherwise overwrite the rename, or the rename the refinement).
func TestSaveIsRefusedWhileThePresetIsBeingRefined(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	p := &storage.Preset{ID: "p1", Name: "Draft Preset", Payload: "{}"}
	if err := h.store.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	h.svc.mu.Lock()
	h.svc.refining[p.ID] = struct{}{}
	h.svc.mu.Unlock()
	if _, err := h.svc.Save(ctx, p.ID, "Named"); !errors.Is(err, ErrPresetBusy) {
		t.Fatalf("Save during a refine: err = %v, want ErrPresetBusy", err)
	}
	if got, _ := h.store.Get(ctx, p.ID); got.Name != "Draft Preset" {
		t.Errorf("a refused Save must not rename: name = %q", got.Name)
	}

	h.svc.mu.Lock()
	delete(h.svc.refining, p.ID)
	h.svc.mu.Unlock()
	if got, err := h.svc.Save(ctx, p.ID, "Named"); err != nil || got.Name != "Named" {
		t.Fatalf("Save after the refine ended: %v, %v", got, err)
	}
}

// ListPresets hands back copies: writing through a result must not reach the store's cache.
func TestListPresetsReturnsCopiesNewestFirstWithoutDrafts(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for _, p := range []*storage.Preset{
		{ID: "a", Name: "Alpha", Payload: "{}", RunIDs: []string{"r1"}},
		{ID: "b", Name: "Draft Preset", Payload: "{}"},
		{ID: "c", Name: "Gamma", Payload: "{}"},
	} {
		if err := h.store.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	got, err := h.svc.ListPresets(ctx, false)
	if err != nil || len(got) != 2 {
		t.Fatalf("ListPresets(false) = %d presets, err %v; want 2 (draft left out)", len(got), err)
	}
	if all, _ := h.svc.ListPresets(ctx, true); len(all) != 3 {
		t.Errorf("ListPresets(true) = %d presets, want 3", len(all))
	}
	for i := range got {
		got[i].Name = "MUTATED"
		if len(got[i].RunIDs) > 0 {
			got[i].RunIDs[0] = "MUTATED"
		}
	}
	again, _ := h.svc.ListPresets(ctx, false)
	for _, p := range again {
		if p.Name == "MUTATED" || (len(p.RunIDs) > 0 && p.RunIDs[0] == "MUTATED") {
			t.Fatalf("mutating a ListPresets result reached the store cache: %+v", p)
		}
	}
	if _, err := h.svc.GetPreset(ctx, "nope"); err == nil {
		t.Error("GetPreset of a missing ID must fail")
	} else if _, ok := err.(*LookupError); !ok {
		t.Errorf("GetPreset error = %T, want *LookupError", err)
	}
}

func TestDecodeStructuredHandlesEnvelopeBareAndLegacy(t *testing.T) {
	guitars := `{"guitars":{"Strat":[{"id":"b1","type":"Amplifier","model":"Plexi","parameters":[]}]}}`
	for name, tc := range map[string]struct {
		payload string
		ok      bool
	}{
		"envelope":    {`{"structured":` + guitars + `,"legacy_html":{"Strat":"<div>x</div>"}}`, true},
		"bare":        {guitars, true},
		"legacy only": {`{"Strat":"<div>x</div>"}`, false},
		"legacy env":  {`{"structured":{"guitars":{}},"legacy_html":{"Strat":"x"}}`, false},
		"garbage":     {`not json`, false},
		"empty":       {``, false},
	} {
		sp, ok := DecodeStructured(tc.payload)
		if ok != tc.ok || (ok && len(sp.Guitars["Strat"]) != 1) {
			t.Errorf("%s: ok=%v guitars=%v, want ok=%v", name, ok, sp.Guitars, tc.ok)
		}
	}
}

// GetPreset returns a fresh object: the store's Get decodes from storage on every
// call and never hands out its cache, so a caller may change the result freely.
func TestGetPresetReturnsAnIndependentObject(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.store.Save(ctx, &storage.Preset{ID: "g1", Name: "Orig", Payload: "{}", RunIDs: []string{"r1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.ListPresets(ctx, true); err != nil { // populate the store's cache too
		t.Fatal(err)
	}
	a, err := h.svc.GetPreset(ctx, "g1")
	if err != nil {
		t.Fatal(err)
	}
	a.Name = "MUTATED"
	a.RunIDs[0] = "MUTATED"
	b, _ := h.svc.GetPreset(ctx, "g1")
	if b.Name != "Orig" || b.RunIDs[0] != "r1" {
		t.Fatalf("mutating one GetPreset result leaked into the next: %+v", b)
	}
	listed, _ := h.svc.ListPresets(ctx, true)
	if listed[0].Name != "Orig" || listed[0].RunIDs[0] != "r1" {
		t.Fatalf("mutating a GetPreset result reached the cache: %+v", listed[0])
	}
}

// Ordering is by UpdatedAt (newest first) and only then by ID. Objects are written
// straight into the fake bucket with explicit timestamps, so no sleeping is needed.
func TestListPresetsOrdersByUpdatedAtThenID(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for _, p := range []storage.Preset{
		{ID: "a-oldest", Name: "Old", Payload: "{}", UpdatedAt: "2026-01-01T00:00:00Z"},
		{ID: "z-newest", Name: "New", Payload: "{}", UpdatedAt: "2026-03-01T00:00:00Z"},
		{ID: "m-tie-2", Name: "T2", Payload: "{}", UpdatedAt: "2026-02-01T00:00:00Z"},
		{ID: "b-tie-1", Name: "T1", Payload: "{}", UpdatedAt: "2026-02-01T00:00:00Z"},
	} {
		data, _ := json.Marshal(p)
		if err := h.client.WriteFile(ctx, testBucket, "presets/"+p.ID+".json", data); err != nil {
			t.Fatal(err)
		}
	}
	got, err := h.svc.ListPresets(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range got {
		ids = append(ids, p.ID)
	}
	want := []string{"z-newest", "b-tie-1", "m-tie-2", "a-oldest"}
	if !slices.Equal(ids, want) {
		t.Fatalf("order = %v, want %v", ids, want)
	}
}
