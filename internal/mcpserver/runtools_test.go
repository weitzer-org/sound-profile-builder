package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/presetsvc"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

type staticSecrets struct{}

func (staticSecrets) GetPassword(context.Context, string, string) (string, error) {
	return "placeholder-key", nil
}
func (staticSecrets) Close() {}

// newMockService is newTestService with the real orchestrator, which in mock
// mode replays testdata/e2e_mocks instead of calling any model.
func newMockService(t *testing.T) *svcHarness {
	t.Helper()
	client := newMemClient()
	store := storage.NewPresetStore(client, testBucket)
	svc := presetsvc.New(presetsvc.Deps{
		Store: store, Memories: storage.NewMemoryStore(client, testBucket), Client: client, Bucket: testBucket,
		Secrets: func() storage.SecretFetcher { return staticSecrets{} },
		OrchMaker: func(ctx context.Context, key string) (agents.OrchestratorService, error) {
			return agents.NewOrchestrator(ctx, key, client)
		},
		Config: func() *config.AppConfig { return nil },
	})
	return &svcHarness{svc: svc, store: store, client: client}
}

func TestRunTools_MockGenerateAndLogs(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)

	res := itCall(t, cs, generateToolName, map[string]any{"prompt": "SRV clean Strat tone", "mock": true, "wait_seconds": 30})
	if res.IsError {
		t.Fatalf("generate: %s", itText(t, res))
	}
	out := itStructured[runOutput](t, res)
	if out.Status != "complete" || out.PresetID == "" || !strings.HasPrefix(out.RunID, "gen-") {
		t.Fatalf("generate output = %+v", out.statusOutput)
	}
	text := itText(t, res)
	if !strings.Contains(text, dataNotice) || !strings.Contains(text, "unsaved draft") {
		t.Errorf("generate text missing notice/draft hint:\n%.400s", text)
	}

	// index
	idx := itCall(t, cs, promptsToolName, map[string]any{"run_id": out.RunID})
	pi := itStructured[promptsOutput](t, idx)
	if len(pi.Agents) != 13 {
		t.Fatalf("index agents = %d, want 13: %s", len(pi.Agents), itText(t, idx))
	}
	t.Logf("index text:\n%s", itText(t, idx))

	// paged by role and by key give the same data
	for _, who := range []string{"CorOS Librarian", "4_coros_librarian", "coros librarian"} {
		r := itCall(t, cs, promptsToolName, map[string]any{"run_id": out.RunID, "agent": who, "part": "both"})
		if r.IsError || !strings.Contains(itText(t, r), "=== USER INPUT ===") {
			t.Errorf("agent %q: %s", who, itText(t, r))
		}
	}

	// output
	o := itCall(t, cs, outputToolName, map[string]any{"run_id": out.RunID, "agent": "Architect & Evaluator"})
	if o.IsError || !strings.Contains(itText(t, o), "[mock] canned output from Architect") {
		t.Errorf("architect output: %s", itText(t, o))
	}
	// log
	l := itCall(t, cs, logToolName, map[string]any{"run_id": out.RunID, "since_seq": 38})
	lo := itStructured[logOutput](t, l)
	for _, e := range lo.Events {
		if e.Seq <= 38 {
			t.Errorf("since_seq not applied: %+v", e)
		}
	}
}

// TestSDKResultSize pins the open item from PR 2: a handler-supplied text block
// is not duplicated into structuredContent, so a max-size page is ~120k bytes.
func TestSDKResultSize(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	res := itCall(t, cs, generateToolName, map[string]any{"prompt": "x", "mock": true, "wait_seconds": 30})
	runID := itStructured[runOutput](t, res).RunID
	big := strings.Repeat("0123456789", 20000) // 200k chars
	if err := h.svc.Runs().WriteLog(context.Background(), runID, "4_coros_librarian.system.md", []byte(big)); err != nil {
		t.Fatal(err)
	}
	r := itCall(t, cs, promptsToolName, map[string]any{"run_id": runID, "agent": "4_coros_librarian", "max_chars": 500000})
	raw, _ := json.Marshal(r)
	sc, _ := json.Marshal(r.StructuredContent)
	t.Logf("content blocks=%d structuredBytes=%d totalResultBytes=%d", len(r.Content), len(sc), len(raw))
	if len(r.Content) != 1 || len(raw) > 121000 {
		t.Errorf("max page should be one text block of ~120k bytes; got %d blocks, %d bytes", len(r.Content), len(raw))
	}
	out := itStructured[promptsOutput](t, r)
	if out.ReturnedChars != maxPageChars || out.NextOffset == nil || *out.NextOffset != maxPageChars || out.TotalChars != 200000 {
		t.Errorf("paging metadata = %+v", out)
	}
}

func TestPageText(t *testing.T) {
	cases := []struct {
		name     string
		s        string
		off, max int
		want     string
		next     int // -1: none
		wantErr  bool
	}{
		{"all", "hello", 0, 0, "hello", -1, false},
		{"one char", "hello", 0, 1, "h", 1, false},
		{"mid", "hello", 2, 2, "ll", 4, false},
		{"to end", "hello", 3, 10, "lo", -1, false},
		{"at end", "hello", 5, 10, "", -1, false},
		{"past end", "hello", 6, 10, "", -1, true},
		{"negative", "hello", -1, 10, "", -1, true},
		{"multibyte", "héllo wörld", 1, 3, "éll", 4, false},
		{"empty", "", 0, 5, "", -1, false},
	}
	for _, c := range cases {
		pg, err := pageText(c.s, c.off, c.max)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v", c.name, err)
			continue
		}
		if c.wantErr {
			continue
		}
		if pg.Text != c.want || (pg.NextOffset == nil) != (c.next < 0) || (pg.NextOffset != nil && *pg.NextOffset != c.next) {
			t.Errorf("%s: got %q next=%v", c.name, pg.Text, pg.NextOffset)
		}
	}
	if normalizePageSize(0) != defaultPageChars || normalizePageSize(1) != 1 || normalizePageSize(1<<30) != maxPageChars {
		t.Error("normalizePageSize bounds")
	}
}

// gatedOrch is an orchestrator whose pipeline reports a phase and then waits
// for the test to let it continue, so the test controls the run's pace.
type gatedOrch struct {
	arch   []byte
	phases []string
	step   chan struct{} // one receive per phase boundary
}

func (g *gatedOrch) RunPipeline(ctx context.Context, _ string, _ map[string]interface{}, _ map[string]string, onProgress func(string)) (string, *agents.TokenUsage, error) {
	for _, p := range g.phases {
		onProgress(p)
		select {
		case <-g.step:
		case <-ctx.Done():
			return "", nil, ctx.Err()
		}
	}
	return string(g.arch), nil, nil
}
func (g *gatedOrch) RefineChat(context.Context, *storage.Preset, string, bool, bool) (string, *agents.TokenUsage, error) {
	return "", nil, nil
}
func (g *gatedOrch) Close() {}

func newGatedService(t *testing.T, g *gatedOrch) *svcHarness {
	t.Helper()
	h := newMockService(t)
	client := newMemClient()
	store := storage.NewPresetStore(client, testBucket)
	h.svc = presetsvc.New(presetsvc.Deps{
		Store: store, Memories: storage.NewMemoryStore(client, testBucket), Client: client, Bucket: testBucket,
		Secrets:   func() storage.SecretFetcher { return staticSecrets{} },
		OrchMaker: func(context.Context, string) (agents.OrchestratorService, error) { return g, nil },
		Config:    func() *config.AppConfig { return nil },
	})
	h.store = store
	return h
}

func readArch(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(findRepoRoot(t), "testdata", "e2e_mocks", "architect_generate.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRunTools_LongPollWakesAndProgress(t *testing.T) {
	g := &gatedOrch{arch: readArch(t), phases: []string{"Phase A", "Phase B"}, step: make(chan struct{})}
	h := newGatedService(t, g)

	var mu sync.Mutex
	var got []mcp.ProgressNotificationParams
	ts := itServer(t, h.svc)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
			mu.Lock()
			got = append(got, *r.Params)
			mu.Unlock()
		},
	})
	hc := &http.Client{Transport: itBearer{token: testToken, base: http.DefaultTransport}}
	cs, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp", HTTPClient: hc, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// wait_seconds=0 returns at once with the "poll in the same turn" instruction.
	params := &mcp.CallToolParams{Name: generateToolName, Arguments: map[string]any{"prompt": "slow tone", "wait_seconds": 0}}
	res, err := cs.CallTool(itCtx(t), params)
	if err != nil || res.IsError {
		t.Fatalf("generate: %v %v", err, res)
	}
	first := itStructured[runOutput](t, res)
	if first.Status != "running" || !strings.Contains(itText(t, res), "get_run_status with run_id="+first.RunID) {
		t.Fatalf("expected running + poll hint, got %s / %s", first.Status, itText(t, res))
	}

	// generate returns before the pipeline goroutine has necessarily reported Phase A. Wait
	// for it, or the status call below starts from an empty phase and takes Phase A as the
	// change it is waiting for.
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		if rec, err := h.svc.Run(t.Context(), first.RunID); err == nil && rec.Phase == "Phase A" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("run never reached Phase A")
		}
	}

	// Concurrent second run is refused with the actionable message.
	t.Setenv("MCP_MAX_CONCURRENT_RUNS", "1")

	// until=change returns when the phase moves.
	go func() { time.Sleep(150 * time.Millisecond); g.step <- struct{}{} }()
	start := time.Now()
	st := &mcp.CallToolParams{Name: statusToolName, Arguments: map[string]any{"run_id": first.RunID, "wait_seconds": 20, "until": "change"}}
	st.SetProgressToken("tok-1")
	res, err = cs.CallTool(itCtx(t), st)
	if err != nil {
		t.Fatal(err)
	}
	s1 := itStructured[statusOutput](t, res)
	if s1.Status != "running" || s1.Phase != "Phase B" || time.Since(start) > 5*time.Second {
		t.Fatalf("until=change: %+v after %s", s1, time.Since(start))
	}

	// until=done (default) wakes the moment the run finishes, not at the deadline.
	go func() { time.Sleep(150 * time.Millisecond); g.step <- struct{}{} }()
	start = time.Now()
	st2 := &mcp.CallToolParams{Name: statusToolName, Arguments: map[string]any{"run_id": first.RunID, "wait_seconds": 60}}
	st2.SetProgressToken("tok-2")
	res, err = cs.CallTool(itCtx(t), st2)
	if err != nil {
		t.Fatal(err)
	}
	s2 := itStructured[statusOutput](t, res)
	if s2.Status != "complete" || s2.PresetID == "" || time.Since(start) > 5*time.Second {
		t.Fatalf("until=done: %+v after %s", s2, time.Since(start))
	}
	if !strings.Contains(itText(t, res), "## ") {
		t.Errorf("completed status should include the rendered preset:\n%.300s", itText(t, res))
	}

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	tokens := map[any]int{}
	var last float64 = -1
	for _, p := range got {
		tokens[p.ProgressToken]++
		t.Logf("progress token=%v progress=%v message=%q", p.ProgressToken, p.Progress, p.Message)
		_ = last
	}
	if tokens["tok-1"] == 0 || tokens["tok-2"] == 0 {
		t.Errorf("progress notifications were not delivered (stateless mode): %v", tokens)
	}
}

func TestRunTools_BusyIsActionable(t *testing.T) {
	t.Setenv("MCP_MAX_CONCURRENT_RUNS", "1")
	g := &gatedOrch{arch: readArch(t), phases: []string{"Phase A"}, step: make(chan struct{})}
	h := newGatedService(t, g)
	cs := newMCPSession(t, h.svc)
	r1 := itCall(t, cs, generateToolName, map[string]any{"prompt": "one", "wait_seconds": 0})
	if r1.IsError {
		t.Fatal(itText(t, r1))
	}
	r2 := itCall(t, cs, generateToolName, map[string]any{"prompt": "two", "wait_seconds": 0})
	if !r2.IsError || !strings.Contains(itText(t, r2), "already in progress") {
		t.Errorf("second run should be refused as busy: %s", itText(t, r2))
	}
	g.step <- struct{}{}
}
