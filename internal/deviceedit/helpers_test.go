package deviceedit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicelib"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

var testCaptures = map[string]string{"1964 Fender Super Reverb": "user", "66 AC30 JMP OD": "user",
	"CA John's": "factory", "FNDR TWDLX IN Edge BAL CAB": "ir"}

func sanitize(s string, n int) string { return devicejobs.SanitizeText(s, n) }

var _ = unicode.IsControl

func newExporter(t *testing.T) *deviceexport.Exporter {
	t.Helper()
	e, err := deviceexport.NewEmbedded(deviceexport.Options{
		CaptureSource: func(n string) string { return testCaptures[n] },
		OwnedPlugins:  deviceexport.OwnedPluginSet(true, []string{"Cory Wong"}),
		Sanitize:      sanitize,
		LookupCapture: func(n string) (string, string, bool) {
			for name, src := range testCaptures {
				if src != "ir" && strings.EqualFold(name, strings.TrimSpace(n)) {
					return name, src, true
				}
			}
			return "", "", false
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// fixtures: the round-trip goldens and the guitar each was exported from.
var fixtures = []struct{ name, guitar string }{
	{"01_SRV_Clean", "Fender Telecaster Single Coil"},
	{"03_British_Invasion", "Gibson ES-339 Humbuckers"},
	{"05_Clapton", "Gibson ES-339 Humbuckers"},
	{"pushable_g3", "Gibson ES-339 Humbuckers"},
	{"pushable_faders", "Gibson ES-339 Humbuckers"},
	{"pushable_plugin", "Gibson ES-339 Humbuckers"},
}

type fixture struct {
	payload json.RawMessage
	rb      *devicejobs.ReadbackPayload
	format  string // "" = qc-device-readback/1
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "devicejobs", "readback_roundtrip", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Expect json.RawMessage `json:"expect_payload"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	rb, err := devicejobs.ParseReadbackPayload(f.Expect)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{payload: f.Expect, rb: rb}
}

const testJob = "11111111-1111-4111-8111-111111111111"

func (f fixture) snapshot() *devicelib.ReadSnapshot {
	format := f.format
	if format == "" {
		format = devicejobs.ReadbackFormat
	}
	return &devicelib.ReadSnapshot{Format: devicelib.SnapshotFormat, JobID: testJob, Slot: "1A", Name: "Round trip",
		Firmware: "4.1.0", ReadAt: "2026-11-07T10:00:00Z", ReadbackFormat: format, Payload: f.payload}
}

// v2 rewrites a fixture as a qc-device-readback/2 read of the given collection and position.
func v2(t *testing.T, f fixture, key string, pos int) fixture {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(f.payload, &m); err != nil {
		t.Fatal(err)
	}
	m["setlist"] = map[string]any{"folder": "other", "folder_key": key, "position": pos, "is_factory": false}
	raw, _ := json.Marshal(m)
	rb, err := devicejobs.ParseReadback(devicejobs.ReadbackFormatV2, raw)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{payload: raw, rb: rb, format: devicejobs.ReadbackFormatV2}
}

// draft imports the fixture the way read_device_preset(import=true) would and returns its chain.
func draft(t *testing.T, e *deviceexport.Exporter, f fixture) []storage.EffectBlock {
	t.Helper()
	imp, err := e.ImportReadback(context.Background(), f.rb, devicejobs.ReadbackMeta{JobID: testJob, Name: "Round trip"})
	if err != nil {
		t.Fatal(err)
	}
	return imp.Preset.Guitars[deviceexport.ReadbackGuitar]
}

func original(t *testing.T, name, guitar string) []storage.EffectBlock {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "deviceexport", "testdata", "presets", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var sp storage.StructuredPreset
	if err := json.Unmarshal(data, &sp); err != nil {
		t.Fatal(err)
	}
	c, ok := sp.Guitars[guitar]
	if !ok {
		t.Fatalf("no guitar %q in %s", guitar, name)
	}
	return c
}

func plan(t *testing.T, e *deviceexport.Exporter, f fixture, target []storage.EffectBlock, derived bool) *Plan {
	t.Helper()
	p, perr := New(e).Plan(Input{Snapshot: f.snapshot(), Target: target, Derived: derived, Legacy: LegacyPreset(storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"g": target}})})
	if perr != nil {
		t.Fatalf("plan: %v", perr)
	}
	return p
}

func clone(blocks []storage.EffectBlock) []storage.EffectBlock {
	b, _ := json.Marshal(blocks)
	var out []storage.EffectBlock
	_ = json.Unmarshal(b, &out)
	return out
}
