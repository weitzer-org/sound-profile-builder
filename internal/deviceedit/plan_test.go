package deviceedit

import (
	"testing"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

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
	if old.Confirmable || old.Pinned || old.Payload.Expect.FolderKey != "" {
		t.Fatalf("a /1 read: confirmable=%v key=%q", old.Confirmable, old.Payload.Expect.FolderKey)
	}
	g := v2(t, f, "/media/p4/Presets/Sounds profiles", 24)
	p := plan(t, e, g, tgt, true)
	ex := p.Payload.Expect
	if !p.Pinned || ex.FolderKey != "/media/p4/Presets/Sounds profiles" || ex.Position != 24 || ex.Slot != "4A" {
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

// Every plan with ops that the census builds, computed from a /2 read, passes the contract's strict
// parser (the agent runs the same rules), is Confirmable, and hashes to devicejobs.EditPlanSHA256 of
// its own text. A plan the parser refuses would be shown but never queued; this test says none of
// the census plans is one.
func TestCensusPlansPassTheContractParser(t *testing.T) {
	e := newExporter(t)
	checked := 0
	for _, fx := range fixtures {
		f := v2(t, loadFixture(t, fx.name), "/media/p4/Presets/Sounds profiles", 24)
		targets := []struct {
			c       []storage.EffectBlock
			derived bool
		}{{original(t, fx.name, fx.guitar), false}}
		for _, m := range mutations {
			targets = append(targets, struct {
				c       []storage.EffectBlock
				derived bool
			}{m.apply(t, clone(draft(t, e, f))), m.derived})
		}
		for i, tg := range targets {
			p := plan(t, e, f, tg.c, tg.derived)
			if devicejobs.EditPlanSHA256(Format, p.Text) != p.SHA256 {
				t.Fatalf("%s #%d: hash is not over the plan's text", fx.name, i)
			}
			if p.Ops() == 0 {
				if p.Confirmable {
					t.Errorf("%s #%d: a plan with no ops is confirmable", fx.name, i)
				}
				continue
			}
			if _, err := devicejobs.ParseEditPayload(p.Text); err != nil || !p.Confirmable {
				t.Errorf("%s #%d: confirmable=%v parse: %v", fx.name, i, p.Confirmable, err)
			}
			// The job's args wrap it exactly as the agent will check it.
			name := "Edited copy"
			target := "20A"
			if _, err := devicejobs.NewEditArgs(p.Payload.Expect.Slot, devicejobs.EditModeCopy, &target, &name,
				devicejobs.PushSource{PresetID: "p", Guitar: "g"}, p.Text); err != nil {
				t.Errorf("%s #%d: args: %v", fx.name, i, err)
			}
			checked++
		}
	}
	if checked < 16 {
		t.Fatalf("only %d plans with ops checked", checked)
	}
	t.Logf("%d plans with ops pass the contract parser", checked)
}
