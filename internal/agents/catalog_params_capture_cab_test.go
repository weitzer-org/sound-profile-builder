package agents

import (
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestFlagCatalogParamMismatchesCaptureCab(t *testing.T) {
	blk := storage.EffectBlock{
		Type: "Cab", Model: "64 Fender Vibroverb (My Capture)",
		Parameters: []storage.BlockParameter{{Name: "Mic 1", Value: "x"}, {Name: "MIC 1 POSITION", Value: "3"}, {Name: "HPF", Value: "80 Hz"}},
	}
	sp := &storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"g": {blk}}}
	c := FlagCatalogParamMismatches(sp, map[string]bool{"64 Fender Vibroverb": true})
	got := sp.Guitars["g"][0].Rationale
	if c.UnknownNames != 3 || !strings.Contains(got, "is a capture, which has no cab controls") {
		t.Fatalf("counts %+v rationale %q", c, got)
	}
}
