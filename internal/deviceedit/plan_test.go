package deviceedit

import "testing"

func TestUnchangedDraftGivesZeroOps(t *testing.T) {
	e := newExporter(t)
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			f := loadFixture(t, fx.name)
			p := plan(t, e, f, draft(t, e, f), true)
			if p.Ops() != 0 || p.NotAppliedCount != 0 {
				t.Fatalf("ops=%d param=%+v bypass=%+v items=%+v", p.Ops(), p.Payload.ParamOps, p.Payload.BypassOps, p.NotApplied)
			}
		})
	}
}

func TestOriginalAgainstItsOwnReadbackGivesZeroOps(t *testing.T) {
	e := newExporter(t)
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			f := loadFixture(t, fx.name)
			p := plan(t, e, f, original(t, fx.name, fx.guitar), false)
			if p.Ops() != 0 {
				t.Fatalf("ops=%d param=%+v bypass=%+v items=%+v", p.Ops(), p.Payload.ParamOps, p.Payload.BypassOps, p.NotApplied)
			}
			t.Logf("items: %+v", p.NotApplied)
		})
	}
}

// A /2 read pins the collection by its key and the slot by its position; a /1 read can't, so its
// plan is shown but never confirmable.
func TestCollectionPin(t *testing.T) {
	e := newExporter(t)
	f := loadFixture(t, "pushable_g3")
	tgt := draft(t, e, f)
	old := plan(t, e, f, tgt, true)
	if old.Confirmable || old.Payload.Expect.FolderKey != "" {
		t.Fatalf("a /1 read: confirmable=%v key=%q", old.Confirmable, old.Payload.Expect.FolderKey)
	}
	g := v2(t, f, "/media/p4/Presets/Sounds profiles", 24)
	p := plan(t, e, g, tgt, true)
	ex := p.Payload.Expect
	if !p.Confirmable || ex.FolderKey != "/media/p4/Presets/Sounds profiles" || ex.Position != 24 || ex.Slot != "4A" {
		t.Fatalf("a /2 read: confirmable=%v expect=%+v", p.Confirmable, ex)
	}
	if p.SHA256 == old.SHA256 {
		t.Fatal("the collection key is not in the hash")
	}
	h := v2(t, f, "/media/p4/Presets/My Presets", 24)
	if q := plan(t, e, h, tgt, true); q.SHA256 == p.SHA256 {
		t.Fatal("two collections hash the same plan")
	}
}
