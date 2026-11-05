package presetsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/runs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// The plugin gate reaches the saved preset through both service paths: generate
// (gate from the run's constraints) and refine (gate from config). The architect
// fixtures are rewritten so an amp block names the paid "Tim Henson Lead" device; it
// must come back flagged unverified unless config.json says the plugin is owned.
func TestPluginGateFlagsUnownedDevices(t *testing.T) {
	const device = "Tim Henson Lead"
	for _, tc := range []struct {
		name     string
		owned    []string
		wantFlag bool
	}{
		{"unowned", nil, true},
		{"owned", []string{"Tim Henson"}, false},
	} {
		cfg := &config.AppConfig{AllowFactoryCaptures: true, AllowUserCaptures: true, AllowPaidPlugins: true, AvailablePlugins: tc.owned}
		want := device
		if tc.wantFlag {
			want = agents.FlagUnverifiedBlock(device)
		}

		t.Run(tc.name+"/generate", func(t *testing.T) {
			h := newHarnessWithConfig(t, cfg)
			h.orch.gen = withAmpModel(t, h.orch.gen, device)
			ctx := context.Background()
			rec := waitComplete(t, h, generate(t, h, GenerateOptions{Mock: true, Source: runs.SourceMCP}))
			p, err := h.store.Get(ctx, rec.PresetID)
			if err != nil {
				t.Fatalf("store.Get: %v", err)
			}
			if got := deviceModel(t, p.Payload, device); got != want {
				t.Errorf("saved amp model = %q, want %q", got, want)
			}
		})

		t.Run(tc.name+"/refine", func(t *testing.T) {
			h := newHarnessWithConfig(t, cfg)
			h.orch.refine = withAmpModel(t, h.orch.refine, device)
			// The rendered table names a paid effect too: the saved HTML must be
			// flagged the same way as the structured payload.
			const htmlDevice = "Tim Henson Delay Modern"
			oldCell := []byte("Delay: Digital")
			if !bytes.Contains(h.orch.refine, oldCell) {
				t.Fatal(`refine fixture has no "Delay: Digital" table cell to rewrite`)
			}
			h.orch.refine = bytes.Replace(h.orch.refine, oldCell, []byte("Delay: "+htmlDevice), 1)
			ctx := context.Background()
			p := &storage.Preset{Name: "Warm Lead"}
			if err := h.store.Save(ctx, p); err != nil {
				t.Fatalf("store.Save: %v", err)
			}
			runID, err := h.svc.StartRefine(ctx, p.ID, "make it brighter", true)
			if err != nil {
				t.Fatalf("StartRefine: %v", err)
			}
			waitComplete(t, h, runID)
			got, err := h.store.Get(ctx, p.ID)
			if err != nil {
				t.Fatalf("store.Get: %v", err)
			}
			if m := deviceModel(t, got.Payload, device); m != want {
				t.Errorf("saved amp model = %q, want %q", m, want)
			}
			var saved struct {
				LegacyHTML map[string]string `json:"legacy_html"`
			}
			if err := json.Unmarshal([]byte(got.Payload), &saved); err != nil {
				t.Fatalf("preset payload is not JSON: %v", err)
			}
			var all string
			for _, html := range saved.LegacyHTML {
				all += html + "\n"
			}
			if !strings.Contains(all, htmlDevice) {
				t.Fatalf("saved HTML lost the %q cell", htmlDevice)
			}
			if flagged := strings.Contains(all, agents.FlagUnverifiedBlock(htmlDevice)); flagged != tc.wantFlag {
				t.Errorf("saved HTML flags %q = %v, want %v", htmlDevice, flagged, tc.wantFlag)
			}
		})
	}
}

// waitComplete waits for a run to finish and fails the test unless it completed.
func waitComplete(t *testing.T, h *harness, runID string) *runs.RunRecord {
	t.Helper()
	rec, err := h.svc.WaitRun(context.Background(), runID, 10*time.Second)
	if err != nil {
		t.Fatalf("WaitRun: %v", err)
	}
	if rec.Status != runs.StatusComplete {
		t.Fatalf("status = %q (error %q), want complete", rec.Status, rec.Error)
	}
	return rec
}

// withAmpModel returns a copy of an architect fixture whose first "Plexi" amp names
// model instead.
func withAmpModel(t *testing.T, fixture []byte, model string) []byte {
	t.Helper()
	old := []byte(`"model": "Plexi"`)
	if !bytes.Contains(fixture, old) {
		t.Fatal(`fixture has no "model": "Plexi" block to rewrite`)
	}
	return bytes.Replace(fixture, old, []byte(`"model": "`+model+`"`), 1)
}

// deviceModel returns the saved model of the block naming device (flagged or not),
// from a saved preset's payload ({"structured": StructuredPreset, "legacy_html": ...}).
func deviceModel(t *testing.T, payload, device string) string {
	t.Helper()
	var saved struct {
		Structured storage.StructuredPreset `json:"structured"`
	}
	if err := json.Unmarshal([]byte(payload), &saved); err != nil {
		t.Fatalf("preset payload is not JSON: %v", err)
	}
	for _, blocks := range saved.Structured.Guitars {
		for _, b := range blocks {
			if strings.HasPrefix(b.Model, device) {
				return b.Model
			}
		}
	}
	t.Fatalf("no %q block in the saved preset", device)
	return ""
}
