package mcpserver

import (
	"context"
	"encoding/json"
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
	r, st, _ := editRigMain(t)
	return r, st
}

func editRigMain(t *testing.T) (*deviceRig, *devicelib.Store, *devicejobstest.MemClient) {
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
	return r, st, main
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

	// Without a pinned (/2) read the plan can't be confirmed, so no token is issued and a confirm is refused.
	if out.CanApply || out.ConfirmToken != "" || !strings.Contains(text, "can't be applied") {
		t.Fatalf("a /1 read must not be confirmable: %+v", out)
	}
	if msg := errText(t, itCall(t, r.cs, deviceEditToolName, map[string]any{"preset_id": "device-edited", "mode": "live", "dry_run": false, "confirm_token": "e1.1.00000000000000000000000000000000"})); !strings.Contains(msg, "can't be applied") {
		t.Errorf("confirm on a /1 read: %s", msg)
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

// ---- E4: the confirm path ----

// pinnedEdit is a /2 read in a collection of the owner's, imported as a draft and edited: GAIN 2 -> 3 and the capture
// removed (a bypass).
func pinnedEdit(t *testing.T) (*deviceRig, *devicejobstest.MemClient, deviceEditOutput, string) {
	t.Helper()
	r, _, main := editRigMain(t)
	ctx := context.Background()
	r.agent.AllowedSlots = []string{"1A-32H"}
	r.online(t)
	var m map[string]any
	_ = json.Unmarshal(r.agent.ReadbackPayload, &m)
	m["setlist"] = map[string]any{"folder": "other", "folder_key": "/media/p4/Presets/Sounds profiles", "position": 24, "is_factory": false}
	r.agent.ReadbackPayload, _ = json.Marshal(m)
	r.agent.ReadbackFormat = devicejobs.ReadbackFormatV2
	r.agent.ReadbackSlot = "4A"
	read := itStructured[deviceJobOutput](t, itCall(t, r.cs, deviceReadToolName, map[string]any{"wait_seconds": 5, "import": true}))
	p, err := r.h.svc.GetPreset(ctx, read.Import.PresetID)
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
	return r, main, itStructured[deviceEditOutput](t, res), read.JobID
}

func confirmArgs(tok string) map[string]any {
	return map[string]any{"preset_id": "device-edited", "mode": "live", "dry_run": false, "confirm_token": tok}
}

func TestEditDevicePreset_LiveDryRunIssuesATokenAndConfirmQueuesOnce(t *testing.T) {
	r, _, out, _ := pinnedEdit(t)
	if !out.CanApply || out.ConfirmToken == "" || !strings.HasPrefix(out.ConfirmToken, "e1.") || out.ConfirmExpiry == "" {
		t.Fatalf("dry run %+v", out)
	}
	res := itCall(t, r.cs, deviceEditToolName, confirmArgs(out.ConfirmToken))
	q := itStructured[deviceEditOutput](t, res)
	if q.Mode != "queued" || !devicejobs.ValidJobID(q.JobID) || q.JobID != devicejobs.EditJobIDFromToken(out.ConfirmToken) || q.Existing {
		t.Fatalf("confirm %+v", q)
	}
	if text := okText(t, res); !strings.Contains(text, "not saved") || !strings.Contains(text, "get_device_job") {
		t.Fatalf("confirm text:\n%s", text)
	}
	// the same confirm again names the same job and sends nothing new
	again := itStructured[deviceEditOutput](t, itCall(t, r.cs, deviceEditToolName, confirmArgs(out.ConfirmToken)))
	if again.JobID != q.JobID || !again.Existing {
		t.Fatalf("repeat %+v", again)
	}
	r.runAgent(t)
	var edits []*devicejobs.Job
	for _, j := range r.agent.ExecutedJobs() {
		if j.Kind == devicejobs.KindEditPreset {
			edits = append(edits, j)
		}
	}
	if len(edits) != 1 || edits[0].JobID != q.JobID || edits[0].V != devicejobs.PayloadVersionEdit {
		t.Fatalf("executed edits %+v", edits)
	}
	args, err := devicejobs.ParseEditArgs(edits[0].Args)
	if err != nil || args.Expect.Slot != "4A" {
		t.Fatalf("args %v %+v", err, args)
	}
}

func TestEditDevicePreset_ConfirmRefusals(t *testing.T) {
	r, main, out, _ := pinnedEdit(t)
	tok := out.ConfirmToken
	for name, args := range map[string]map[string]any{
		"token with a dry run":        {"preset_id": "device-edited", "mode": "live", "confirm_token": tok},
		"no token with dry_run=false": {"preset_id": "device-edited", "mode": "live", "dry_run": false},
		"another preset":              {"preset_id": "other-one", "mode": "live", "dry_run": false, "confirm_token": tok},
		"copy cannot be applied":      {"preset_id": "device-edited", "mode": "copy", "dry_run": false, "confirm_token": tok, "name": "X", "target_slot": "21A"},
		"overwrite cannot be applied": {"preset_id": "device-edited", "mode": "overwrite", "dry_run": false, "confirm_token": tok},
		"a push token":                confirmArgs("c1." + strings.Split(tok, ".")[1] + "." + strings.Split(tok, ".")[2]),
		"a garbled token":             confirmArgs("e1.1.zz"),
	} {
		if msg := errText(t, itCall(t, r.cs, deviceEditToolName, args)); msg == "" {
			t.Errorf("%s: no error", name)
		}
	}
	// a different guitar value changes the claims
	if msg := errText(t, itCall(t, r.cs, deviceEditToolName, map[string]any{"preset_id": "device-edited", "guitar": "From the unit", "mode": "live", "dry_run": false, "confirm_token": tok[:len(tok)-1] + "0"})); msg == "" {
		t.Error("a token with a changed last digit was accepted")
	}
	r.runAgent(t)
	for _, j := range r.agent.ExecutedJobs() {
		if j.Kind == devicejobs.KindEditPreset {
			t.Fatalf("a refused confirm queued %s", j.JobID)
		}
	}
	// the stored read is altered after the dry run: the token no longer matches / the plan can't be applied
	keys := main.Keys("main-bucket")
	var snapKey string
	for _, k := range keys {
		if strings.HasPrefix(k, devicelib.SnapshotPrefix) {
			snapKey = k
		}
	}
	raw, _ := main.ReadFile(context.Background(), "main-bucket", snapKey)
	var snap map[string]any
	_ = json.Unmarshal(raw, &snap)
	snap["name"] = "Texas Blues (planted)"
	planted, _ := json.Marshal(snap)
	_ = main.WriteFile(context.Background(), "main-bucket", snapKey, planted)
	res := itCall(t, r.cs, deviceEditToolName, map[string]any{"preset_id": "device-edited", "mode": "live"})
	o := itStructured[deviceEditOutput](t, res)
	if o.CanApply || o.ConfirmToken != "" || !strings.Contains(okText(t, res), "signed result") {
		t.Fatalf("a planted snapshot must not be confirmable: %+v\n%s", o, okText(t, res))
	}
	if msg := errText(t, itCall(t, r.cs, deviceEditToolName, confirmArgs(tok))); !strings.Contains(msg, "can't be applied") {
		t.Errorf("confirm after the snapshot was planted: %s", msg)
	}
}

func TestEditApplicable_RefusesACaptureBlocksControls(t *testing.T) {
	plan := &deviceedit.Plan{Pinned: true, Confirmable: true}
	plan.Payload.ParamOps = []devicejobs.EditParamOp{{ModelID: 14001, Index: 0}}
	if ok, why := editApplicable(context.Background(), nil, nil, plan, "live"); ok || !strings.Contains(why, "Neural Capture") {
		t.Fatalf("%v %q", ok, why)
	}
	if ok, why := editApplicable(context.Background(), nil, nil, plan, "copy"); ok || !strings.Contains(why, "live") {
		t.Fatalf("%v %q", ok, why)
	}
}

// get_device_job shows an applied edit only when the agent's result fits the plan QC-2 sent.
func TestEditDevicePreset_ResultIsCheckedAgainstTheSentPlan(t *testing.T) {
	for _, c := range []struct {
		name    string
		mutate  func(res *devicejobs.EditResult)
		applied bool
		want    string
	}{
		{"as planned", func(*devicejobs.EditResult) {}, true, "Edit applied"},
		{"a control did not read back", func(res *devicejobs.EditResult) { v := 0.9; res.Params[0].Readback[0] = &v }, false, "NO"},
		{"the agent claims ok but the values differ", func(res *devicejobs.EditResult) { v := 0.9; res.Params[0].Readback[1] = &v; res.Params[0].OK = true }, false, "NO"},
		{"an extra control", func(res *devicejobs.EditResult) { res.Params = append(res.Params, res.Params[0]) }, false, "does not fit the plan"},
		{"another cell", func(res *devicejobs.EditResult) { res.Params[0].Col = 5 }, false, "does not fit the plan"},
		{"collateral change", func(res *devicejobs.EditResult) { res.Collateral.Mismatches = 1; res.Collateral.LayoutSame = true }, false, "outside the plan"},
		{"rolled back", func(res *devicejobs.EditResult) {
			res.Rollback = devicejobs.EditRollback{State: "restored", Restored: 2}
		}, false, "rolled back"},
		{"half-edited", func(res *devicejobs.EditResult) {
			res.Rollback = devicejobs.EditRollback{State: "partial", Restored: 1, Failed: 1}
		}, false, "WITHOUT saving"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, _, out, _ := pinnedEdit(t)
			r.agent.EditResult = func(j *devicejobs.Job, res *devicejobs.Result) {
				args, err := devicejobs.ParseEditArgs(j.Args)
				if err != nil {
					t.Error(err)
					return
				}
				er := devicejobs.EditResult{PlanSHA256: out.PlanSHA256, Snapshot: args.Snapshot, Mode: "live",
					CAS: devicejobs.EditCAS{State: "matched", Failed: []string{}}, Params: []devicejobs.EditParamRead{}, Bypass: []devicejobs.EditBypassRead{},
					Collateral: &devicejobs.EditCollateral{ParamsCompared: 10, BypassCompared: 200, LayoutSame: true, Items: []devicejobs.EditCollateralItem{}},
					Rollback:   devicejobs.EditRollback{State: "not_needed"}}
				for _, op := range args.ParamOps {
					rb := make([]*float64, 8)
					for i := range rb {
						v := op.Wire[i]
						rb[i] = &v
					}
					er.Params = append(er.Params, devicejobs.EditParamRead{Row: op.Row, Col: op.Col, Index: op.Index, Readback: rb, OK: true})
				}
				for _, op := range args.BypassOps {
					fl := op.Bypass[:]
					er.Bypass = append(er.Bypass, devicejobs.EditBypassRead{Row: op.Row, Col: op.Col, Readback: &fl, OK: true})
				}
				c.mutate(&er)
				payload, _ := json.Marshal(er)
				res.Status, res.ReasonCode = devicejobs.StatusSucceeded, ""
				if c.name != "as planned" {
					res.Status, res.ReasonCode = devicejobs.StatusFailed, "readback_mismatch"
				}
				res.Edit = &devicejobs.EditBlock{Format: devicejobs.EditResultFormat, Payload: payload}
			}
			q := itStructured[deviceEditOutput](t, itCall(t, r.cs, deviceEditToolName, confirmArgs(out.ConfirmToken)))
			r.runAgent(t)
			res := itCall(t, r.cs, deviceJobToolName, map[string]any{"job_id": q.JobID, "wait_seconds": 10})
			text := okText(t, res)
			job := itStructured[deviceJobOutput](t, res)
			if job.Edit == nil || job.Edit.Applied != c.applied || !strings.Contains(text, c.want) {
				t.Fatalf("applied=%v want %v (%q)\n%+v\n%s", job.Edit != nil && job.Edit.Applied, c.applied, c.want, job.Edit, text)
			}
		})
	}
}
