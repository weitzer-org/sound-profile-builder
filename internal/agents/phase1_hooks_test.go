package agents

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func newHookTestOrch(t *testing.T) (*Orchestrator, func() string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockGeminiResponse))
	}))
	t.Cleanup(srv.Close)
	orch, err := NewOrchestrator(context.Background(), "fake-key", nil, WithEndpoint(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return orch, func() string { mu.Lock(); defer mu.Unlock(); return strings.Join(bodies, "\n") }
}

func TestPhase1Hooks_StopAfterPhase1_CapturesAndSkipsLaterAgents(t *testing.T) {
	orch, bodies := newHookTestOrch(t)
	var got Phase1Result
	ctx := WithStopAfterPhase1(WithPhase1Capture(context.Background(), &got))
	_, _, err := orch.RunPipeline(ctx, "SRV clean", map[string]interface{}{"single_amp_mode": true}, nil, nil)
	if !errors.Is(err, ErrStoppedAfterPhase1) {
		t.Fatalf("want ErrStoppedAfterPhase1, got %v", err)
	}
	if got.Tone == "" || got.Scrape == "" {
		t.Errorf("capture empty: %+v", got)
	}
	if !strings.Contains(got.LibrarianStatic, "Dictionary: ") || !strings.Contains(got.LibrarianStatic, "User Capture Library: ") {
		t.Errorf("captured Librarian static catalog is missing sections")
	}
	if strings.Contains(bodies(), "CorOS Librarian") {
		t.Errorf("Librarian must not run when stopping after Phase 1")
	}
}

func TestPhase1Hooks_OverrideSkipsPhase1AndFeedsLibrarian(t *testing.T) {
	orch, bodies := newHookTestOrch(t)
	ov := Phase1Result{Tone: "FROZEN-TONE-XYZ", Sonic: "FROZEN-SONIC", Scrape: "FROZEN-SCRAPE-XYZ"}
	ctx := WithStopAfterPhase1(WithPhase1Override(context.Background(), ov))
	var got Phase1Result
	ctx = WithPhase1Capture(ctx, &got)
	if _, _, err := orch.RunPipeline(ctx, "q", nil, nil, nil); !errors.Is(err, ErrStoppedAfterPhase1) {
		t.Fatal(err)
	}
	if got.Tone != ov.Tone || got.Scrape != ov.Scrape || got.Sonic != ov.Sonic {
		t.Errorf("override not reflected in capture: %+v", got)
	}
	if bodies() != "" {
		t.Errorf("no model call should happen when Phase 1 is overridden and the run stops after it, got %d bytes", len(bodies()))
	}
}

func TestLibrarianContextOrders(t *testing.T) {
	static := buildLibrarianStaticCatalog("D", "A", "U")
	c := map[string]interface{}{"x": 1}
	nw := LibrarianContextNewOrder(static, "T", "S", c)
	old := LibrarianContextOldOrder(static, "T", "S", c)
	if !strings.HasPrefix(nw, static) {
		t.Error("new order must start with the static catalog")
	}
	if strings.HasPrefix(old, static) || !strings.Contains(old, static) {
		t.Error("old order must contain the catalog but not start with it")
	}
	if len(nw) != len(old) {
		t.Errorf("same content, different order expected: %d vs %d", len(nw), len(old))
	}
	if nw != buildLibrarianContext(static, "T", "S", c) {
		t.Error("LibrarianContextNewOrder must equal buildLibrarianContext")
	}
}
