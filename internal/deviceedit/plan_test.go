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
