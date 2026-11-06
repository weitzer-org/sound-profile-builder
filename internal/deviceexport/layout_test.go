package deviceexport

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func twoGuitars(a, b []storage.EffectBlock) storage.StructuredPreset {
	marker := storage.EffectBlock{Model: "Myth Drive", Parameters: []storage.BlockParameter{prm("Gain", "5", "")}}
	return storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"ES": a, "Tele": b, "basis-marker": {marker}}}
}

func layoutExport(t *testing.T, sp storage.StructuredPreset, l devicejobs.SceneLayout) (*devicejobs.Export, Payload) {
	t.Helper()
	ex, err := testExporter(t).ExportLayout(context.Background(), sp, l)
	if err != nil {
		t.Fatal(err)
	}
	if err := ex.Validate(); err != nil {
		t.Fatal(err)
	}
	var p Payload
	dec := json.NewDecoder(bytes.NewReader(ex.Payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		t.Fatal(err)
	}
	return ex, p
}

func gs(g string, r, l string) devicejobs.GuitarScenes {
	ri, _ := devicejobs.ParseScene(r)
	li, _ := devicejobs.ParseScene(l)
	return devicejobs.GuitarScenes{Guitar: g, Rhythm: ri, Lead: li}
}

// The default layout is the original export, byte for byte.
func TestLayoutDefaultIsExport(t *testing.T) {
	sp := twoGuitars([]storage.EffectBlock{block("Myth Drive", prm("Gain", "2", "8"))}, nil)
	e := testExporter(t)
	a, err := e.Export(context.Background(), sp, "ES")
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.ExportLayout(context.Background(), sp, devicejobs.DefaultLayout("ES"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Payload, b.Payload) || b.Layout != nil {
		t.Fatal("default layout must equal Export")
	}
	if !strings.Contains(string(a.Payload), `"scene_sources":["A","B","A","A","A","A","A","A"]`) {
		t.Fatalf("payload: %s", a.Payload)
	}
}

// ES-339 rhythm A / lead B, Telecaster rhythm E / lead F, C D G H unused (= ES rhythm).
func TestLayoutTwoGuitarsPlaceScenes(t *testing.T) {
	es := []storage.EffectBlock{block("Myth Drive", prm("Gain", "2", "8"), prm("Bypass", "Off", "Off"))}
	tele := []storage.EffectBlock{block("Myth Drive", prm("Gain", "4", "6"), prm("Bypass", "On", "Off"))}
	ex, p := layoutExport(t, twoGuitars(es, tele), devicejobs.SceneLayout{Guitars: []devicejobs.GuitarScenes{gs("ES", "A", "B"), gs("Tele", "E", "F")}})
	if len(ex.Unresolved) != 0 {
		t.Fatalf("unresolved %+v", ex.Unresolved)
	}
	if p.Guitar != "ES + Tele" {
		t.Errorf("guitar %q", p.Guitar)
	}
	if want := [Scenes]string{"A", "B", "A", "A", "2A", "2B", "A", "A"}; p.SceneSources != want {
		t.Errorf("scene_sources %v, want %v", p.SceneSources, want)
	}
	blk := p.Rows[0].Blocks[0]
	var gain *Param
	for i := range blk.Params {
		if blk.Params[i].Label == "GAIN" || strings.EqualFold(blk.Params[i].Label, "gain") {
			gain = &blk.Params[i]
		}
	}
	if gain == nil {
		t.Fatalf("no gain: %+v", blk.Params)
	}
	if want := [Scenes]float64{2, 8, 2, 2, 4, 6, 2, 2}; gain.Display != want || !gain.SceneMode {
		t.Errorf("gain display %v mode %v, want %v", gain.Display, gain.SceneMode, want)
	}
	if want := [Scenes]bool{false, false, false, false, true, false, false, false}; blk.Bypass != want {
		t.Errorf("bypass %v, want %v", blk.Bypass, want)
	}
	if ex.Layout == nil || ex.Layout.Scenes[4].Guitar != "Tele" || ex.Layout.Scenes[4].Role != "rhythm" || ex.Layout.Scenes[2].Role != "unused" {
		t.Errorf("layout %+v", ex.Layout)
	}
	pb := ex.Summary[0]
	if len(pb.BypassScenes) != 8 || !pb.BypassScenes[4] || len(pb.Params[0].Scenes) != 8 {
		t.Errorf("summary %+v", pb)
	}
}

// One guitar alone can be moved off A/B.
func TestLayoutSingleGuitarOtherScenes(t *testing.T) {
	es := []storage.EffectBlock{block("Myth Drive", prm("Gain", "2", "8"))}
	_, p := layoutExport(t, twoGuitars(es, nil), devicejobs.SceneLayout{Guitars: []devicejobs.GuitarScenes{gs("ES", "E", "F")}})
	if want := [Scenes]string{"A", "A", "A", "A", "A", "B", "A", "A"}; p.SceneSources != want {
		t.Errorf("scene_sources %v", p.SceneSources)
	}
	g := p.Rows[0].Blocks[0].Params[0]
	if want := [Scenes]float64{2, 2, 2, 2, 2, 8, 2, 2}; g.Display != want {
		t.Errorf("display %v, want %v", g.Display, want)
	}
}

// A control only one guitar sets is at the catalog default for the other.
func TestLayoutControlOnlyOneGuitarSets(t *testing.T) {
	es := []storage.EffectBlock{block("Myth Drive", prm("Gain", "2", "2"))}
	tele := []storage.EffectBlock{block("Myth Drive", prm("Gain", "2", "2"), prm("Level", "7", "7"))}
	_, p := layoutExport(t, twoGuitars(es, tele), devicejobs.SceneLayout{Guitars: []devicejobs.GuitarScenes{gs("ES", "A", "B"), gs("Tele", "E", "F")}})
	var level *Param
	for i, pr := range p.Rows[0].Blocks[0].Params {
		if strings.EqualFold(pr.Label, "level") {
			level = &p.Rows[0].Blocks[0].Params[i]
		}
	}
	if level == nil {
		t.Fatalf("level not written: %+v", p.Rows[0].Blocks[0])
	}
	if !level.SceneMode || level.Display[4] != 7 || level.Display[5] != 7 || level.Display[0] == 7 || level.Display[0] != level.Display[1] {
		t.Errorf("level display %v", level.Display)
	}
}

func TestLayoutRefusals(t *testing.T) {
	a := []storage.EffectBlock{block("Myth Drive", prm("Gain", "2", ""))}
	cases := []struct {
		name string
		b    []storage.EffectBlock
		want string
	}{
		{"different length", []storage.EffectBlock{a[0], block("Myth Drive")}, "different chains"},
		{"different model", []storage.EffectBlock{block("Chief OD1")}, "differ at block 1"},
	}
	for _, c := range cases {
		ex, _ := layoutExport(t, twoGuitars(a, c.b), devicejobs.SceneLayout{Guitars: []devicejobs.GuitarScenes{gs("ES", "A", "B"), gs("Tele", "E", "F")}})
		if len(ex.Unresolved) == 0 || !strings.Contains(ex.Unresolved[0].Reason, c.want) {
			t.Errorf("%s: %+v", c.name, ex.Unresolved)
		}
	}
	e := testExporter(t)
	bad := []devicejobs.SceneLayout{
		{Guitars: []devicejobs.GuitarScenes{gs("ES", "A", "A")}},
		{Guitars: []devicejobs.GuitarScenes{gs("ES", "A", "B"), gs("Tele", "B", "C")}},
		{Guitars: []devicejobs.GuitarScenes{gs("ES", "A", "B"), gs("ES", "C", "D")}},
		{Guitars: []devicejobs.GuitarScenes{gs("ES", "A", "B"), gs("nope", "C", "D")}},
	}
	for i, l := range bad {
		if _, err := e.ExportLayout(context.Background(), twoGuitars(a, a), l); err == nil {
			t.Errorf("layout %d should be refused", i)
		}
	}
}

func TestParseScene(t *testing.T) {
	for in, want := range map[string]int{"A": 0, "h": 7, " e ": 4} {
		if got, ok := devicejobs.ParseScene(in); !ok || got != want {
			t.Errorf("%q = %d,%v", in, got, ok)
		}
	}
	for _, in := range []string{"", "I", "AB", "1"} {
		if _, ok := devicejobs.ParseScene(in); ok {
			t.Errorf("%q should not parse", in)
		}
	}
}

// TestGoldenExport_TwoGuitarsLayout pins the payload of two guitars in one slot
// (testdata/golden/pushable_g3_layout.json); the agent's golden test feeds it through its own validator.
func TestGoldenExport_TwoGuitarsLayout(t *testing.T) {
	sp := loadPreset(t, "pushable_g3")
	const first = "Gibson ES-339 Humbuckers"
	second := append([]storage.EffectBlock(nil), sp.Guitars[first]...)
	for i := range second {
		second[i].Parameters = append([]storage.BlockParameter(nil), second[i].Parameters...)
		for j, p := range second[i].Parameters {
			if p.Name == "Gain" || p.Name == "GAIN" {
				second[i].Parameters[j].Value, second[i].Parameters[j].ValueB = "3", "6"
			}
		}
	}
	sp.Guitars["Fender Telecaster Single Coil"] = second
	l := devicejobs.SceneLayout{Guitars: []devicejobs.GuitarScenes{gs(first, "A", "B"), gs("Fender Telecaster Single Coil", "E", "F")}}
	ex, _ := layoutExport(t, sp, l)
	if len(ex.Unresolved) != 0 {
		t.Fatalf("unresolved: %+v", ex.Unresolved)
	}
	sum, err := ex.PlanSHA256()
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.MarshalIndent(golden{ex.Format, sum, ex.Summary, ex.Unresolved, ex.Payload}, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "golden", "pushable_g3_layout.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("export differs from %s; rerun with -update and review the diff", path)
	}
}
