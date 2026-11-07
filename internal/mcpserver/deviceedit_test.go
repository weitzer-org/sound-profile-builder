package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/deviceedit"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
	"github.com/weitzer-org/sound-builder/internal/devicelib"
	"github.com/weitzer-org/sound-builder/internal/presetsvc"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func editRig(t *testing.T) (*deviceRig, *devicelib.Store) {
	t.Helper()
	exp := importerForTest(t)
	main := devicejobstest.NewMemClient()
	st := devicelib.NewStore(main, "main-bucket")
	cat, err := catalog.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	lib := &DeviceLibrary{Store: st, Harvester: devicelib.NewHarvester(cat)}
	r := newDeviceRigWith(t, nil, WithImporter(exp), WithDeviceLibrary(lib), WithDeviceEditor(deviceedit.New(exp)))
	r.agent.ReadbackPayload = importPayload(t, "CA John's")
	r.agent.ReadbackName = "Texas Blues"
	r.online(t)
	r.runAgent(t)
	return r, st
}

func TestEditDevicePreset_RegisteredOnlyWithTheEditor(t *testing.T) {
	r := newDeviceRig(t, nil)
	res, err := r.cs.ListTools(itCtx(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == deviceEditToolName {
			t.Fatalf("%s is registered without the editor", deviceEditToolName)
		}
	}
	r2, _ := editRig(t)
	res, _ = r2.cs.ListTools(itCtx(t), nil)
	found := false
	for _, tool := range res.Tools {
		if tool.Name == deviceEditToolName {
			found = true
			if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
				t.Errorf("annotations %+v: want DestructiveHint true", tool.Annotations)
			}
		}
	}
	if !found {
		t.Fatal("edit_device_preset not registered")
	}
}

func TestEditDevicePreset_SnapshotStoredWithoutImportAndDryRunShowsChanges(t *testing.T) {
	r, st := editRig(t)
	ctx := context.Background()

	// A plain read (no import) still stores the snapshot.
	read := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceReadToolName, map[string]any{"wait_seconds": 5}))
	snap, err := st.ReadSnapshot(ctx, read.JobID)
	if err != nil || snap == nil || snap.Name != "Texas Blues" || snap.Slot != "1A" {
		t.Fatalf("snapshot %+v err %v", snap, err)
	}

	// Import a draft, unchanged: nothing to do.
	imp := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceJobToolName, map[string]any{"job_id": read.JobID, "import": true}))
	if imp.Import == nil || imp.Import.PresetID == "" {
		t.Fatalf("import %+v", imp.Import)
	}
	same := okText(t, itCall(t, r.cs, deviceEditToolName, map[string]any{"preset_id": imp.Import.PresetID}))
	if !strings.Contains(same, "No changes") || !strings.Contains(same, "Dry run, nothing was sent") || !strings.Contains(same, "copy:") {
		t.Fatalf("unchanged:\n%s", same)
	}

	// An edited copy of the draft: Myth Drive GAIN 2 -> 3 in Scene A, the capture removed.
	p, err := r.h.svc.GetPreset(ctx, imp.Import.PresetID)
	if err != nil {
		t.Fatal(err)
	}
	sp, _ := presetsvc.DecodeStructured(p.Payload)
	chain := sp.Guitars["From the unit"]
	chain[0].Parameters[0].Value = "3"
	sp.Guitars["From the unit"] = append(chain[:1], chain[2:]...)
	if _, _, err := r.h.svc.ImportDraft(ctx, "device-edited", sp, *p.Origin, "edited"); err != nil {
		t.Fatal(err)
	}
	res := itCall(t, r.cs, deviceEditToolName, map[string]any{"preset_id": "device-edited", "mode": "live"})
	text := okText(t, res)
	for _, want := range []string{"GAIN", "→", "Bypassed", "live:", "Scenes C-H keep", "qc2-device-edit/1"} {
		if !strings.Contains(text, want) {
			t.Errorf("lacks %q:\n%s", want, text)
		}
	}
	out := itStructured[deviceEditOutput](t, res)
	if out.ParamOps != 1 || out.BypassOps != 1 || !out.Derived || len(out.PlanSHA256) != 64 {
		t.Fatalf("out %+v", out)
	}
	_ = storage.OriginDeviceReadback

	// A confirm attempt is refused, and nothing is queued.
	// E3 can mint edit tokens, but the tool still never takes one (applying an edit is phase E4).
	kr, _ := devicejobs.NewKeyring(strings.Repeat("ab", 32), "")
	etok, _ := kr.IssueEditConfirm(devicejobs.EditClaims{PresetID: "device-edited", Mode: "live", PlanSHA256: out.PlanSHA256}, time.Now())
	for _, args := range []map[string]any{
		{"preset_id": "device-edited", "dry_run": false, "confirm_token": etok},
		{"preset_id": "device-edited", "mode": "live", "confirm_token": etok},
		{"preset_id": "device-edited", "dry_run": false, "confirm_token": "x"},
		{"preset_id": "device-edited", "confirm_token": "x"},
		{"preset_id": "device-edited", "dry_run": false},
	} {
		if msg := errText(t, itCall(t, r.cs, deviceEditToolName, args)); !strings.Contains(msg, "not available yet") {
			t.Errorf("confirm attempt %v: %s", args, msg)
		}
	}
}

func TestEditDevicePreset_FromReadNeededForANonDraftAndMissingSnapshot(t *testing.T) {
	r, _ := editRig(t)
	ctx := context.Background()
	sp := storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"G": {{ID: "b1", Model: "Myth Drive", Type: "Overdrive"}}}}
	if _, _, err := r.h.svc.ImportDraft(ctx, "plain-one", sp, storage.PresetOrigin{Kind: "other"}, "x"); err != nil {
		t.Fatal(err)
	}
	if msg := errText(t, itCall(t, r.cs, deviceEditToolName, map[string]any{"preset_id": "plain-one"})); !strings.Contains(msg, "from_read") {
		t.Errorf("no origin: %s", msg)
	}
	missing := "22222222-2222-4222-8222-222222222222"
	if !devicejobs.ValidJobID(missing) {
		t.Fatal("test id invalid")
	}
	if msg := errText(t, itCall(t, r.cs, deviceEditToolName, map[string]any{"preset_id": "plain-one", "from_read": missing})); !strings.Contains(msg, "Read the preset again") {
		t.Errorf("missing snapshot: %s", msg)
	}
}

// An old draft (imported before snapshots existed) is backfilled from the signed job result.
func TestEditDevicePreset_BackfillsAnOldDraftsSnapshot(t *testing.T) {
	r, st := editRig(t)
	ctx := context.Background()
	// Read with the importer only: store the draft but delete nothing; then use a fresh store to simulate the gap.
	read := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceReadToolName, map[string]any{"wait_seconds": 5, "import": true}))
	// Remove the snapshot by pointing the library at an empty store.
	empty := devicelib.NewStore(devicejobstest.NewMemClient(), "other")
	exp := importerForTest(t)
	lib := &DeviceLibrary{Store: empty}
	_ = st
	text, err := func() (string, error) {
		// Call the handler directly with the empty-store library.
		h := editDeviceTool(r.h.svc, r.bridge, lib, deviceedit.New(exp))
		_, out, err := h(ctx, nil, deviceEditInput{PresetID: read.Import.PresetID})
		return out.SnapshotJob, err
	}()
	if err != nil || text != read.JobID {
		t.Fatalf("backfill: %q %v", text, err)
	}
	if s, err := empty.ReadSnapshot(ctx, read.JobID); err != nil || s == nil {
		t.Fatalf("backfilled snapshot %v %v", s, err)
	}
}

// The copy name is model-supplied printable ASCII: it must not turn the dry run into a link.
func TestEditDevicePreset_NameCannotFormMarkdown(t *testing.T) {
	r, _ := editRig(t)
	read := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceReadToolName, map[string]any{"wait_seconds": 5, "import": true}))
	text := okText(t, itCall(t, r.cs, deviceEditToolName, map[string]any{
		"preset_id": read.Import.PresetID, "mode": "copy", "name": "[x](http://e.co/a)", "target_slot": "20A"}))
	if strings.Contains(text, "[x](http://e.co/a)") && !strings.Contains(text, `\[x]`) {
		t.Fatalf("name rendered as a live link:\n%s", text)
	}
}

// GSR #183: a failed cache write must not discard a snapshot that was read fine.
func TestEditDevicePreset_BackfillSurvivesAStorageWriteError(t *testing.T) {
	r, _ := editRig(t)
	ctx := context.Background()
	read := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceReadToolName, map[string]any{"wait_seconds": 5, "import": true}))
	main := devicejobstest.NewMemClient()
	main.SetFailWrite(true)
	lib := &DeviceLibrary{Store: devicelib.NewStore(main, "other")}
	h := editDeviceTool(r.h.svc, r.bridge, lib, deviceedit.New(importerForTest(t)))
	_, out, err := h(ctx, nil, deviceEditInput{PresetID: read.Import.PresetID})
	if err != nil || out.SnapshotJob != read.JobID {
		t.Fatalf("%v %+v", err, out)
	}
}

// A qc-device-readback/2 read in one of the owner's own collections: the snapshot keeps its
// format, the dry run names the collection, and the plan pins the key. A /1 read gets the
// "can't be pinned" note instead.
func TestEditDevicePreset_ReadbackV2PinsTheCollection(t *testing.T) {
	r, st := editRig(t)
	ctx := context.Background()
	old := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceReadToolName, map[string]any{"wait_seconds": 5}))
	imp := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceJobToolName, map[string]any{"job_id": old.JobID, "import": true}))
	oldText := okText(t, itCall(t, r.cs, deviceEditToolName, map[string]any{"preset_id": imp.Import.PresetID}))
	if !strings.Contains(oldText, "can't be pinned to its collection") {
		t.Fatalf("a /1 read lacks the pin note:\n%s", oldText)
	}

	var m map[string]any
	_ = json.Unmarshal(r.agent.ReadbackPayload, &m)
	m["setlist"] = map[string]any{"folder": "other", "folder_key": "/media/p4/Presets/Sounds profiles", "position": 24, "is_factory": false}
	r.agent.ReadbackPayload, _ = json.Marshal(m)
	r.agent.ReadbackFormat = devicejobs.ReadbackFormatV2
	r.agent.ReadbackSlot = "4A"
	read := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceReadToolName, map[string]any{"wait_seconds": 5}))
	snap, err := st.ReadSnapshot(ctx, read.JobID)
	if err != nil || snap == nil || snap.ReadbackFormat != devicejobs.ReadbackFormatV2 {
		t.Fatalf("snapshot %+v err %v", snap, err)
	}
	imp2 := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceJobToolName, map[string]any{"job_id": read.JobID, "import": true}))
	if imp2.Import == nil || imp2.Import.PresetID == "" {
		t.Fatalf("import of a /2 read: %+v", imp2)
	}
	res := itCall(t, r.cs, deviceEditToolName, map[string]any{"preset_id": imp2.Import.PresetID})
	text := okText(t, res)
	out := itStructured[deviceEditOutput](t, res)
	if !out.Pinned || out.Collection != "/media/p4/Presets/Sounds profiles" || strings.Contains(text, "can't be pinned") ||
		!strings.Contains(text, "collection `/media/p4/Presets/Sounds profiles`") {
		t.Fatalf("out %+v\n%s", out, text)
	}
}

// renderEdit's three states for a plan that can't be applied: not pinned (a /1 read), pinned with
// ops but failing the contract's strict checks, and no ops (nothing to say).
func TestRenderEdit_NotConfirmableNotes(t *testing.T) {
	const pinNote, strictNote = "can't be pinned to its collection", "doesn't pass the bridge's strict checks"
	for _, c := range []struct {
		name      string
		out       deviceEditOutput
		pin, strk bool
	}{
		{"a /1 read", deviceEditOutput{Pinned: false, ParamOps: 1}, true, false},
		{"pinned, ops, fails the strict parse", deviceEditOutput{Pinned: true, Confirmable: false, ParamOps: 1, BypassOps: 1}, false, true},
		{"pinned, no ops", deviceEditOutput{Pinned: true, Confirmable: false}, false, false},
		{"confirmable", deviceEditOutput{Pinned: true, Confirmable: true, ParamOps: 2}, false, false},
	} {
		var b strings.Builder
		renderEdit(&b, c.out)
		text := b.String()
		if strings.Contains(text, pinNote) != c.pin || strings.Contains(text, strictNote) != c.strk {
			t.Errorf("%s: pin note %v (want %v), strict note %v (want %v)\n%s", c.name,
				strings.Contains(text, pinNote), c.pin, strings.Contains(text, strictNote), c.strk, text)
		}
	}
}
