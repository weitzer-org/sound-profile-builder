package deviceedit

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

var unreachable = map[string]bool{}
var seen = map[string]bool{} // every item code any test case produced

func codesOf(p *Plan) map[string]int {
	m := map[string]int{}
	for _, n := range p.NotApplied {
		m[n.Code]++
		seen[n.Code] = true
		if n.Reason != Reason(n.Code) || n.Reason == "" {
			panic("reason text not the fixed one for " + n.Code)
		}
	}
	return m
}

func blockNamed(chain []storage.EffectBlock, model string) *storage.EffectBlock {
	for i := range chain {
		if chain[i].Model == model {
			return &chain[i]
		}
	}
	return nil
}

func setParam(blk *storage.EffectBlock, name, a, b string) {
	for i := range blk.Parameters {
		if blk.Parameters[i].Name == name {
			blk.Parameters[i].Value, blk.Parameters[i].ValueB = a, b
			return
		}
	}
	blk.Parameters = append(blk.Parameters, storage.BlockParameter{Name: name, Value: a, ValueB: b, Basis: "device_readback"})
}

func mutateS(t *testing.T, f fixture, fn func(m map[string]any)) fixture {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(f.payload, &m); err != nil {
		t.Fatal(err)
	}
	fn(m)
	raw, _ := json.Marshal(m)
	rb, err := devicejobs.ParseReadbackPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{payload: raw, rb: rb}
}

func wantCode(t *testing.T, p *Plan, code string) {
	t.Helper()
	if codesOf(p)[code] == 0 {
		t.Fatalf("want an item %q, got %+v (ops %d)", code, p.NotApplied, p.Ops())
	}
}

func TestEditOpsAndScenes(t *testing.T) {
	e := newExporter(t)
	f := loadFixture(t, "pushable_g3")
	base := draft(t, e, f)

	// GAIN 2/8 -> 3/8 on Myth Drive: one op; A from the target, B and C-H from the snapshot.
	tgt := clone(base)
	setParam(blockNamed(tgt, "Myth Drive"), "GAIN", "3", "8")
	p := plan(t, e, f, tgt, true)
	if len(p.Payload.ParamOps) != 1 || len(p.Payload.BypassOps) != 0 || p.NotAppliedCount != 0 {
		t.Fatalf("%+v %+v", p.Payload.ParamOps, p.NotApplied)
	}
	op := p.Payload.ParamOps[0]
	if op.Label != "GAIN" || op.Col != 0 || op.Wire[0] != 0.3 || op.Wire[1] != op.Expect[1] || len(op.Evidence) == 0 {
		t.Fatalf("op %+v", op)
	}
	for s := 2; s < 8; s++ {
		if op.Wire[s] != op.Expect[s] {
			t.Errorf("scene %d: wire %v, snapshot %v", s, op.Wire[s], op.Expect[s])
		}
	}

	// C-H of the unit differ from A: the edit must keep them exactly.
	f2 := mutateS(t, f, func(m map[string]any) {
		prm := m["chains"].([]any)[0].(map[string]any)["blocks"].([]any)[0].(map[string]any)["params"].([]any)[0].(map[string]any)
		prm["values"] = []any{0.2, 0.8, 0.31, 0.32, 0.33, 0.34, 0.35, 0.36}
	})
	p2 := plan(t, e, f2, tgt, true)
	if len(p2.Payload.ParamOps) != 1 {
		t.Fatalf("%+v", p2.Payload.ParamOps)
	}
	o2 := p2.Payload.ParamOps[0]
	want := [8]float64{0.3, 0.8, 0.31, 0.32, 0.33, 0.34, 0.35, 0.36}
	if o2.Wire != want || !o2.SceneMode {
		t.Fatalf("wire %v scene_mode %v, want %v", o2.Wire, o2.SceneMode, want)
	}
	// A difference in C-H alone is not a change.
	if p3 := plan(t, e, f2, base, true); p3.Ops() != 0 {
		t.Fatalf("C-H-only difference produced %d ops", p3.Ops())
	}

	// A small edit is not swallowed: 4.5 -> 4.6 on a 0..10 dial.
	small := clone(base)
	setParam(blockNamed(small, "Brit 2203"), "GAIN", "4.6", "6")
	if p4 := plan(t, e, f, small, true); len(p4.Payload.ParamOps) != 1 {
		t.Fatalf("4.5 -> 4.6 gave %+v", p4.Payload.ParamOps)
	}

	// Bypass: Spring is active in A and bypassed in B; the target bypasses it in A too.
	byp := clone(base)
	setParam(blockNamed(byp, "Spring"), "Bypass", "On", "On")
	p5 := plan(t, e, f, byp, true)
	if len(p5.Payload.BypassOps) != 1 || p5.Payload.BypassOps[0].Why != "target" || !p5.Payload.BypassOps[0].Bypass[0] {
		t.Fatalf("%+v", p5.Payload.BypassOps)
	}
	for s := 2; s < 8; s++ {
		if p5.Payload.BypassOps[0].Bypass[s] != p5.Payload.BypassOps[0].Expect[s] {
			t.Errorf("bypass scene %d not preserved", s)
		}
	}
	// A target without Bypass means active: Spring's B bypass is lifted.
	nb := clone(base)
	sp := blockNamed(nb, "Spring")
	sp.Parameters = sp.Parameters[:len(sp.Parameters)-1]
	if p6 := plan(t, e, f, nb, true); len(p6.Payload.BypassOps) != 1 || p6.Payload.BypassOps[0].Bypass[1] {
		t.Fatalf("%+v", p6.Payload.BypassOps)
	}
}

func TestNotAppliedReasons(t *testing.T) {
	e := newExporter(t)
	g3 := loadFixture(t, "pushable_g3")
	british := loadFixture(t, "03_British_Invasion")
	base := draft(t, e, g3)

	t.Run("new_block", func(t *testing.T) {
		tgt := append(clone(base), storage.EffectBlock{ID: "b9", Model: "Circular Delay", Type: "Delay"})
		wantCode(t, plan(t, e, g3, tgt, true), CodeNewBlock)
	})
	t.Run("model_swap", func(t *testing.T) {
		tgt := clone(base)
		tgt[0] = storage.EffectBlock{ID: "b1", Model: "Rage Booster", Type: "Overdrive"}
		p := plan(t, e, g3, tgt, true)
		wantCode(t, p, CodeModelSwap)
		if p.Ops() != 0 {
			t.Errorf("a swap must not bypass the old block: %+v", p.Payload.BypassOps)
		}
	})
	t.Run("capture_swap", func(t *testing.T) {
		bt := draft(t, e, british)
		bt[3] = storage.EffectBlock{ID: "b4", Model: "CA John's (Factory Capture)", Type: "Capture"}
		wantCode(t, plan(t, e, british, bt, true), CodeCaptureSwap)
	})
	t.Run("reorder_still_edits_the_moved_block", func(t *testing.T) {
		tgt := clone(base)
		tgt[0], tgt[1] = tgt[1], tgt[0] // Brit 2203 first, Myth Drive second; ids kept
		setParam(blockNamed(tgt, "Myth Drive"), "GAIN", "3", "8")
		p := plan(t, e, g3, tgt, true)
		wantCode(t, p, CodeReorder)
		if len(p.Payload.ParamOps) != 1 || p.Payload.ParamOps[0].Col != 0 {
			t.Fatalf("the moved block's edit must still land on its own cell: %+v", p.Payload.ParamOps)
		}
	})
	t.Run("match_ambiguous", func(t *testing.T) {
		tgt := clone(base)
		for i := range tgt {
			tgt[i].ID = fmt.Sprintf("block-%d", i+1)
		}
		dup := tgt[4]
		dup.ID = "block-9"
		wantCode(t, plan(t, e, g3, append(tgt, dup), false), CodeMatchAmbiguous)
	})
	t.Run("unknown_and_not_editable_and_device_element", func(t *testing.T) {
		tgt := append(clone(base),
			storage.EffectBlock{ID: "b9", Model: "Plate Reverb"},
			storage.EffectBlock{ID: "b10", Model: "FNDR TWDLX IN Edge BAL CAB"},
			storage.EffectBlock{ID: "b11", Model: "Output"})
		p := plan(t, e, g3, tgt, true)
		wantCode(t, p, CodeUnknownModel)
		wantCode(t, p, CodeNotEditableBlock)
		wantCode(t, p, CodeDeviceElement)
	})
	t.Run("cell_not_in_target_vs_removed", func(t *testing.T) {
		tgt := clone(base)
		tgt = tgt[:4] // drop Spring
		p := plan(t, e, g3, tgt, false)
		wantCode(t, p, CodeCellNotInTarget)
		if p.Ops() != 0 {
			t.Errorf("a non-derived target must not bypass: %+v", p.Payload.BypassOps)
		}
		d := plan(t, e, g3, tgt, true)
		if len(d.Payload.BypassOps) != 1 || d.Payload.BypassOps[0].Why != "removed" || d.Payload.BypassOps[0].Bypass != [8]bool{true, true, true, true, true, true, true, true} {
			t.Fatalf("derived removal: %+v", d.Payload.BypassOps)
		}
		if len(d.View) != 1 || !strings.Contains(d.View[0].Note, "C-H") {
			t.Errorf("the dry run must say C-H are bypassed too: %+v", d.View)
		}
	})
	t.Run("bypass_unreadable", func(t *testing.T) {
		f := mutateS(t, g3, func(m map[string]any) { m["bypass"] = nil })
		tgt := clone(base)
		setParam(blockNamed(tgt, "Myth Drive"), "Bypass", "On", "On")
		wantCode(t, plan(t, e, f, tgt, true), CodeBypassUnreadable)
	})
	t.Run("control_level", func(t *testing.T) {
		tgt := clone(base)
		md := blockNamed(tgt, "Myth Drive")
		md.Parameters = append(md.Parameters,
			storage.BlockParameter{Name: "Frobnicate", Value: "1", Basis: "x"},
			storage.BlockParameter{Name: "GAIN", Value: "3", Basis: "x"}, // written twice
		)
		setParam(md, "TREBLE", "5.5 Hz", "") // wrong unit
		setParam(md, "LEVEL", "99", "")      // out of range
		setParam(md, "GAIN", "loud", "")     // unparseable (first GAIN)
		md.Parameters = append(md.Parameters, storage.BlockParameter{Name: "Bypass", Value: "Maybe", Basis: "x"})
		cab := blockNamed(tgt, "110 US PRN C10R")
		cab.Parameters = append(cab.Parameters,
			storage.BlockParameter{Name: "Blend", Value: "5", Basis: "x"},
			storage.BlockParameter{Name: "Mic 1", Value: "Ribbon 160", Basis: "x"})
		cd := draft(t, e, british)
		_ = cd
		sp := blockNamed(tgt, "Spring")
		sp.Parameters = append(sp.Parameters, storage.BlockParameter{Name: "Trails", Value: "Sideways", Basis: "x"})
		p := plan(t, e, g3, tgt, true)
		for _, c := range []string{CodeNotAControl, CodeDuplicateControl, CodeValueWrongUnit, CodeValueOutOfRange, CodeValueUnparseable, CodeBypassWord, CodeCabBlend, CodeMicNotWritten} {
			wantCode(t, p, c)
		}
	})
	t.Run("bypass_legacy", func(t *testing.T) {
		tgt := clone(base)
		setParam(blockNamed(tgt, "Myth Drive"), "Bypass", "On", "On")
		p, perr := New(e).Plan(Input{Snapshot: g3.snapshot(), Target: tgt, Legacy: true, Derived: true})
		if perr != nil {
			t.Fatal(perr)
		}
		wantCode(t, p, CodeBypassLegacy)
	})
}

// cellFixture is a read-back with one block of the given model, every parameter at wire 0.5.
func cellFixture(t *testing.T, m *catalog.Model, drop int, tweak func(index int) any) fixture {
	t.Helper()
	params := []any{}
	for i := 0; i < m.DeviceParamCount(); i++ {
		if i == drop {
			continue
		}
		var v any = 0.5
		if tweak != nil {
			if x := tweak(i); x != nil {
				v = x
			}
		}
		params = append(params, map[string]any{"index": i, "scene_mode": false, "values": []any{v, v, v, v, v, v, v, v}})
	}
	byp := make([][][]bool, 4)
	for r := range byp {
		byp[r] = make([][]bool, 8)
		for c := range byp[r] {
			byp[r][c] = make([]bool, 8)
		}
	}
	raw, _ := json.Marshal(map[string]any{
		"setlist": map[string]any{"folder": "My Presets", "position": 4},
		"chains":  []any{map[string]any{"row": 0, "in_port": 1, "out_port": 19, "blocks": []any{map[string]any{"col": 0, "model_id": m.ID, "params": params}}}},
		"bypass":  byp,
	})
	rb, err := devicejobs.ParseReadbackPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{payload: raw, rb: rb}
}

func TestLockedControlReasons(t *testing.T) {
	e := newExporter(t)
	cat, err := catalog.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	pushable := func(m *catalog.Model) bool {
		return !strings.Contains(m.Category, "Internal") && m.Category != "Utility_Deprecated" && m.ID < 14000 && len(cat.Resolve(m.Name)) == 1
	}
	find := func(pred func(*catalog.Model, catalog.LabelledParam) bool) (*catalog.Model, catalog.LabelledParam) {
		for i := range cat.Models {
			m := &cat.Models[i]
			if !pushable(m) || deviceexport.CloneLocked(m) || e.ClassifyName(m.Name) != deviceexport.NameModel {
				continue
			}
			for _, lp := range catalog.LabelParams(m) {
				if pred(m, lp) {
					return m, lp
				}
			}
		}
		return nil, catalog.LabelledParam{}
	}
	run := func(name, code string, m *catalog.Model, label string, f fixture) {
		t.Run(name, func(t *testing.T) {
			tgt := []storage.EffectBlock{{ID: "b1", Model: m.Name, Type: "x", Parameters: []storage.BlockParameter{{Name: label, Value: "1", Basis: "x"}}}}
			p, perr := New(e).Plan(Input{Snapshot: f.snapshot(), Target: tgt, Derived: true})
			if perr != nil {
				t.Fatal(perr)
			}
			wantCode(t, p, code)
			if strings.HasPrefix(code, "locked_") && p.Untouched.LockedControls == 0 {
				t.Errorf("locked control not counted: %+v", p.Untouched)
			}
		})
	}
	// unreadable: the unit's read-back has no value at the control's position.
	var m1 *catalog.Model
	for i := range cat.Models {
		if cat.Models[i].ID == 1 {
			m1 = &cat.Models[i]
		}
	}
	lpm := catalog.LabelParams(m1)[0]
	run("unreadable", CodeLockedUnreadable, m1, lpm.Label, cellFixture(t, m1, lpm.Param.Index, nil))

	m, lp := find(func(m *catalog.Model, lp catalog.LabelledParam) bool {
		return lp.Param.Type == "comboBox" || lp.Param.Type == "floatWithLed"
	})
	run("no_wire_mapping", CodeLockedNoMapping, m, lp.Label, cellFixture(t, m, -1, nil))

	m, lp = find(func(m *catalog.Model, lp catalog.LabelledParam) bool {
		_, a := catalog.UnmeasuredBounds[lp.Param.MinSymbol]
		_, b := catalog.UnmeasuredBounds[lp.Param.MaxSymbol]
		return catalog.ContinuousParamTypes[lp.Param.Type] && (a || b)
	})
	if m != nil {
		run("unmeasured_bound", CodeLockedUnmeasured, m, lp.Label, cellFixture(t, m, -1, nil))
	} else {
		unreachable[CodeLockedUnmeasured] = true // only input-trim bounds are unmeasured, and no exposed model has them
	}

	m, lp = find(func(m *catalog.Model, lp catalog.LabelledParam) bool {
		c := lp.Param
		return (c.Type == "switch" || c.Type == "rotarySwitch" || c.Type == "toggleButton") && c.Min != nil && c.Max != nil && *c.Max-*c.Min >= 3
	})
	run("not_whole_position", CodeLockedNotWhole, m, lp.Label, cellFixture(t, m, -1, nil))

	// A model cloning an unverified base.
	for i := range cat.Models {
		cm := &cat.Models[i]
		if deviceexport.CloneLocked(cm) && cm.ID < 14000 && e.ClassifyName(cm.Name) == deviceexport.NameModel && len(cat.Resolve(cm.Name)) == 1 {
			if lps := catalog.LabelParams(cm); len(lps) > 0 {
				run("locked_clone", CodeLockedClone, cm, lps[0].Label, cellFixture(t, cm, -1, nil))
				break
			}
		}
	}

	// An internal device parameter.
	for i := range cat.Models {
		cm := &cat.Models[i]
		if !pushable(cm) || deviceexport.CloneLocked(cm) || e.ClassifyName(cm.Name) != deviceexport.NameModel {
			continue
		}
		for _, p := range cm.Params {
			if catalog.IsInternalParam(p) && !strings.EqualFold(p.Name, "bypass") && !strings.EqualFold(p.Name, "ir selector") {
				run("internal_control", CodeInternalControl, cm, p.Name, cellFixture(t, cm, -1, nil))
				return
			}
		}
	}
}

// GSR #183: an unparseable Bypass word must not be read as "active".
func TestUnparseableBypassWordChangesNothing(t *testing.T) {
	e := newExporter(t)
	f := loadFixture(t, "pushable_g3") // Spring is bypassed in Scene B on the unit
	tgt := clone(draft(t, e, f))
	setParam(blockNamed(tgt, "Spring"), "Bypass", "Maybe", "Maybe")
	p := plan(t, e, f, tgt, true)
	wantCode(t, p, CodeBypassWord)
	if len(p.Payload.BypassOps) != 0 {
		t.Fatalf("an unparseable word produced a bypass op: %+v", p.Payload.BypassOps)
	}
	// With the unit's bypass unreadable the same target still says so.
	f2 := mutateS(t, f, func(m map[string]any) { m["bypass"] = nil })
	wantCode(t, plan(t, e, f2, tgt, true), CodeBypassUnreadable)
}

// GSR #183: a gap cell a swap has claimed can't be claimed again.
func TestClaimedGapCellIsNotReused(t *testing.T) {
	e := newExporter(t)
	f := loadFixture(t, "pushable_g3")
	tgt := clone(draft(t, e, f))
	tgt[0] = storage.EffectBlock{ID: "b1", Model: "Rage Booster", Type: "Overdrive"}
	tgt = append([]storage.EffectBlock{tgt[0], {ID: "b9", Model: "Brit Blues", Type: "Overdrive"}}, tgt[1:]...)
	c := codesOf(plan(t, e, f, tgt, true))
	if c[CodeModelSwap] != 1 || c[CodeNewBlock] != 1 {
		t.Fatalf("want one swap and one new block, got %v", c)
	}
}

// GSR #183: every control a locked block's target tries to write is counted.
func TestLockedControlsCountedPerControl(t *testing.T) {
	e := newExporter(t)
	cat, _ := catalog.Embedded()
	for i := range cat.Models {
		m := &cat.Models[i]
		if !deviceexport.CloneLocked(m) || m.ID >= 14000 || e.ClassifyName(m.Name) != deviceexport.NameModel || len(cat.Resolve(m.Name)) != 1 {
			continue
		}
		lps := catalog.LabelParams(m)
		if len(lps) < 2 {
			continue
		}
		tgt := []storage.EffectBlock{{ID: "b1", Model: m.Name, Type: "x", Parameters: []storage.BlockParameter{
			{Name: lps[0].Label, Value: "1", Basis: "x"}, {Name: lps[1].Label, Value: "1", Basis: "x"}}}}
		p, perr := New(e).Plan(Input{Snapshot: cellFixture(t, m, -1, nil).snapshot(), Target: tgt, Derived: true})
		if perr != nil {
			t.Fatal(perr)
		}
		c := codesOf(p)
		if c[CodeLockedClone] != 1 || p.Untouched.LockedControls != 2 {
			t.Fatalf("items %v, locked controls %d, want one item and 2", c, p.Untouched.LockedControls)
		}
		return
	}
	t.Fatal("no clone-locked model with two controls")
}
