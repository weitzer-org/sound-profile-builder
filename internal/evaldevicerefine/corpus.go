// Package evaldevicerefine is the scorer and bookkeeping for the E2 A/B eval of
// docs/design/qc-preset-editing.md section 8.1: does the edit-aware refine context
// (arm B) produce changes the bridge can apply, compared with today's refine (arm A), on
// drafts imported from the unit?
//
// Everything here is deterministic and makes no Gemini call. The one exception to "metrics
// come from deviceedit.Plan" is block preservation, which compares the output chain with the
// draft directly. cmd/eval_device_refine is the CLI around it.
package evaldevicerefine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicelib"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Draft is one corpus entry: a read of the unit's preset (the snapshot every plan is
// computed against) and the draft the import makes from it.
type Draft struct {
	Name     string
	Snapshot *devicelib.ReadSnapshot
	Chain    []storage.EffectBlock
	// Unconverted counts the controls and blocks the import left out (Origin.Unconverted).
	Unconverted int
	// JobID names the snapshot; it is the draft's Origin.JobID.
	JobID string
}

// Preset builds the stored preset a refine run starts from, shaped as ImportDraft stores it
// (the {structured, legacy_html} envelope, Origin set). A fresh value on every call.
func (d *Draft) Preset() *storage.Preset {
	chain := append([]storage.EffectBlock(nil), d.Chain...)
	sp := storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{deviceexport.ReadbackGuitar: chain}}
	payload, _ := json.Marshal(map[string]any{"structured": sp, "legacy_html": map[string]string{}})
	return &storage.Preset{
		ID: "device-" + d.JobID, Name: "Draft Preset", Payload: string(payload),
		BuilderStatement: "Imported from the preset open on the Quad Cortex (read back from the unit, not generated).",
		Origin: &storage.PresetOrigin{Kind: storage.OriginDeviceReadback, JobID: d.JobID, Unconverted: d.Unconverted,
			DeviceName: d.Snapshot.Name, Slot: d.Snapshot.Slot, Firmware: d.Snapshot.Firmware},
	}
}

// fixtureJobID makes a stable, valid job id for a corpus file that has none.
func fixtureJobID(name string) string {
	h := sha256.Sum256([]byte("evaldevicerefine/" + name))
	x := hex.EncodeToString(h[:])
	return fmt.Sprintf("%s-%s-4%s-8%s-%s", x[0:8], x[8:12], x[13:16], x[17:20], x[20:32])
}

// LoadCorpus reads every *.json in dirs (not recursive). A file is either a round-trip
// fixture (testdata/devicejobs/readback_roundtrip: {"name","position","expect_payload"}) or a
// stored snapshot (qc2-device-snapshot/1, as device/snapshots/<job_id>.json holds it: the
// owner's own reads). Draft names are the file names; duplicates are an error.
func LoadCorpus(ctx context.Context, dirs []string, imp *deviceexport.Exporter) ([]*Draft, error) {
	var files []string
	for _, dir := range dirs {
		m, err := filepath.Glob(filepath.Join(dir, "*.json"))
		if err != nil {
			return nil, err
		}
		files = append(files, m...)
	}
	sort.Strings(files)
	seen := map[string]string{}
	var out []*Draft
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		if prev, dup := seen[name]; dup {
			return nil, fmt.Errorf("draft name %q appears twice (%s and %s)", name, prev, f)
		}
		seen[name] = f
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		snap, err := snapshotFrom(name, raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		d, err := NewDraft(ctx, name, snap, imp)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		out = append(out, d)
	}
	return out, nil
}

func snapshotFrom(name string, raw []byte) (*devicelib.ReadSnapshot, error) {
	var probe struct {
		Format string          `json:"format"`
		Expect json.RawMessage `json:"expect_payload"`
		Name   string          `json:"name"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	if probe.Format == devicelib.SnapshotFormat {
		var s devicelib.ReadSnapshot
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if err := s.Validate(s.JobID); err != nil {
			return nil, err
		}
		return &s, nil
	}
	if len(probe.Expect) == 0 {
		return nil, fmt.Errorf("neither a snapshot (%s) nor a round-trip fixture (expect_payload)", devicelib.SnapshotFormat)
	}
	rb, err := devicejobs.ParseReadbackPayload(probe.Expect)
	if err != nil {
		return nil, err
	}
	_ = rb
	title := probe.Name
	if title == "" {
		title = name
	}
	return &devicelib.ReadSnapshot{Format: devicelib.SnapshotFormat, JobID: fixtureJobID(name), Slot: "1A", Name: title,
		Firmware: "4.1.0", ReadAt: "2026-11-07T10:00:00Z", ReadbackFormat: devicejobs.ReadbackFormat, Payload: probe.Expect}, nil
}

// NewDraft imports a snapshot the way read_device_preset(import=true) does.
func NewDraft(ctx context.Context, name string, snap *devicelib.ReadSnapshot, imp *deviceexport.Exporter) (*Draft, error) {
	rb, err := snap.Readback()
	if err != nil {
		return nil, err
	}
	res, err := imp.ImportReadback(ctx, rb, devicejobs.ReadbackMeta{JobID: snap.JobID, Slot: snap.Slot, Name: snap.Name, Firmware: snap.Firmware})
	if err != nil {
		return nil, err
	}
	chain := res.Preset.Guitars[deviceexport.ReadbackGuitar]
	if len(chain) == 0 {
		return nil, fmt.Errorf("the import has no blocks")
	}
	return &Draft{Name: name, Snapshot: snap, Chain: chain, Unconverted: res.UnconvertedCount, JobID: snap.JobID}, nil
}
