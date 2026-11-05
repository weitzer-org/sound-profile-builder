package presetsvc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/runs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// The device-catalog parameter check reaches the saved preset through both service paths.
// The architect fixtures' "Plexi" amp, which carries a "Drive" parameter, is renamed to the
// device's "Brit 2203", whose controls are GAIN/BASS/MID/TREBLE/MASTER/PRESENCE/OUTPUT: the
// saved block must carry the device-check note.
func TestCatalogParamCheckAnnotatesSavedPresets(t *testing.T) {
	const device = "Brit 2203"
	const wantNote = `Drive isn't a control on the device's Brit 2203`
	cfg := &config.AppConfig{AllowFactoryCaptures: true, AllowUserCaptures: true}

	t.Run("generate", func(t *testing.T) {
		h := newHarnessWithConfig(t, cfg)
		h.orch.gen = withAmpModel(t, h.orch.gen, device)
		rec := waitComplete(t, h, generate(t, h, GenerateOptions{Mock: true, Source: runs.SourceMCP}))
		p, err := h.store.Get(context.Background(), rec.PresetID)
		if err != nil {
			t.Fatalf("store.Get: %v", err)
		}
		if got := deviceRationale(t, p.Payload, device); !strings.Contains(got, wantNote) {
			t.Errorf("saved rationale = %q, want it to contain %q", got, wantNote)
		}
	})

	t.Run("refine", func(t *testing.T) {
		h := newHarnessWithConfig(t, cfg)
		h.orch.refine = withAmpModel(t, h.orch.refine, device)
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
		if r := deviceRationale(t, got.Payload, device); !strings.Contains(r, wantNote) {
			t.Errorf("saved rationale = %q, want it to contain %q", r, wantNote)
		}
	})
}

// deviceRationale returns the saved rationale of the block whose model is device.
func deviceRationale(t *testing.T, payload, device string) string {
	t.Helper()
	var saved struct {
		Structured storage.StructuredPreset `json:"structured"`
	}
	if err := json.Unmarshal([]byte(payload), &saved); err != nil {
		t.Fatalf("preset payload is not JSON: %v", err)
	}
	for _, blocks := range saved.Structured.Guitars {
		for _, b := range blocks {
			if b.Model == device {
				return b.Rationale
			}
		}
	}
	t.Fatalf("no %q block in the saved preset", device)
	return ""
}
