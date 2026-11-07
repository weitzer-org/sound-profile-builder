package mcpserver

import (
	"context"
	"strings"
	"testing"

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
	for _, args := range []map[string]any{
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
