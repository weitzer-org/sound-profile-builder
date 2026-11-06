package presetsvc

import (
	"context"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func importFixture() storage.StructuredPreset {
	return storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"From the unit": {
		{ID: "b1", Type: "Overdrive", Model: "Myth Drive", Parameters: []storage.BlockParameter{
			{Name: "GAIN", Type: "slider", Value: "2", ValueB: "8", Basis: "device_readback", Evidence: "taper:g4;bounds:catalog"}}},
	}}}
}

func TestImportDraft(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	origin := storage.PresetOrigin{Kind: storage.OriginDeviceReadback, JobID: "j1", Slot: "3A", DeviceName: "Texas Blues"}

	p, created, err := h.svc.ImportDraft(ctx, "device-j1", importFixture(), origin, "Imported.")
	if err != nil || !created {
		t.Fatalf("ImportDraft: created=%v err=%v", created, err)
	}
	if p.Name != DraftName || p.Origin == nil || p.Origin.Kind != storage.OriginDeviceReadback {
		t.Fatalf("draft %+v origin %+v", p, p.Origin)
	}

	// It is in the store, listed as a draft, and decodes like a generated preset.
	got, err := h.svc.GetPreset(ctx, "device-j1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin == nil || got.Origin.DeviceName != "Texas Blues" || got.BuilderStatement != "Imported." {
		t.Fatalf("stored %+v origin %+v", got, got.Origin)
	}
	sp, ok := DecodeStructured(got.Payload)
	if !ok || sp.Guitars["From the unit"][0].Parameters[0].Evidence == "" {
		t.Fatalf("payload %s", got.Payload)
	}
	if drafts, _ := h.svc.ListPresets(ctx, false); len(drafts) != 0 {
		t.Errorf("a draft showed in the saved list: %+v", drafts)
	}
	if all, _ := h.svc.ListPresets(ctx, true); len(all) != 1 {
		t.Errorf("list with drafts: %d", len(all))
	}

	// A repeat finds it and never overwrites: a rename survives.
	if _, err := h.svc.Save(ctx, "device-j1", "Texas Blues (unit)"); err != nil {
		t.Fatal(err)
	}
	again, created, err := h.svc.ImportDraft(ctx, "device-j1", importFixture(), origin, "Imported.")
	if err != nil || created || again.Name != "Texas Blues (unit)" {
		t.Fatalf("repeat: created=%v name=%q err=%v", created, again.Name, err)
	}

	// Refused inputs.
	if _, _, err := h.svc.ImportDraft(ctx, "", importFixture(), origin, ""); err == nil {
		t.Error("an empty id was accepted")
	}
	if _, _, err := h.svc.ImportDraft(ctx, "device-j2", storage.StructuredPreset{}, origin, ""); err == nil {
		t.Error("an empty preset was accepted")
	}
}
