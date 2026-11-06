package stomplayout

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

var update = flag.Bool("update", false, "rewrite testdata/golden/*.json from current output")

// goldenCase is one saved preset (or synthetic grid) laid out on row 0, col = chain index,
// as the v1 device export does, plus the assignments already on it.
type goldenCase struct {
	name    string
	preset  string // file under ../deviceexport/testdata/presets, "" for a synthetic grid
	guitar  string
	names   []string // synthetic chain, model names
	blocks  []Block  // synthetic blocks given directly (captures)
	current []Assignment
}

func TestGoldenPlans(t *testing.T) {
	cases := []goldenCase{
		// Compressor, a boost, a plain drive, a capture (labelled Amplifier), delay, reverb.
		{name: "03_British_Invasion", preset: "03_British_Invasion", guitar: "Gibson ES-339 Humbuckers"},
		// A drive (fuzz) and a reverb; nothing else switchable.
		{name: "05_Clapton", preset: "05_Clapton", guitar: "Gibson ES-339 Humbuckers"},
		// A drive, a capture (labelled Amplifier), a delay and a reverb.
		{name: "01_SRV_Clean", preset: "01_SRV_Clean", guitar: "Fender Telecaster Single Coil"},
		// Synthetic: three drives and a boost, mod and two delays, with stale assignments,
		// one block on H and one momentary layout switch.
		{name: "synthetic_stale", guitar: "synthetic", names: []string{
			"Jewel", "Rage Booster", "Myth Drive", "Brit Blues", "Obsessive Drive", "Brit 2203",
			"110 US PRN C10R", "Circular Delay", "Analog Delay (M)", "Vintage Chorus", "Room"},
			current: []Assignment{
				{Row: 0, Col: 0, Switch: "A"},                  // comp: correct
				{Row: 0, Col: 1, Switch: "B", Momentary: true}, // boost: correct but momentary
				{Row: 0, Col: 2, Switch: "F"},                  // drive on the mod switch
				{Row: 0, Col: 5, Switch: "C"},                  // amp: stale
				{Row: 0, Col: 7, Switch: "H"},                  // delay on H: stays
				{Row: 0, Col: 10, Switch: "G"},                 // reverb: correct
			}},
		// Synthetic captures on two rows: a library boost, a modeler/library disagreement, an
		// unclassified capture, and a modeler-labelled drive.
		{name: "synthetic_captures", guitar: "synthetic", blocks: []Block{
			{Row: 0, Col: 0, ModelID: capID, ModelerType: "Capture", CaptureName: "Strymon Sunset BOOST"},
			{Row: 0, Col: 1, ModelID: capID, ModelerType: "Amplifier", CaptureName: "Boss Blues Driver Waza"},
			{Row: 0, Col: 2, ModelID: capID, ModelerType: "Neural Capture", CaptureName: "Mystery Pedal"},
			{Row: 1, Col: 0, ModelID: capID, ModelerType: "Fuzz", CaptureName: "Unlisted Fuzz"},
			{Row: 1, Col: 1, ModelID: capID, ModelerType: "Capture", CaptureName: "Boss Blues Driver Waza"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := tc.blocksFor(t)
			p := build(t, blocks, tc.current)
			out := struct {
				Layout  string       `json:"layout"`
				Blocks  []Block      `json:"blocks"`
				Current []Assignment `json:"current"`
				Plan    Plan         `json:"plan"`
			}{testLayout(t).Summary(), blocks, append([]Assignment{}, tc.current...), p}
			got, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", "golden", tc.name+".json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run: go test ./internal/stomplayout -run Golden -update)", err)
			}
			if string(got) != string(want) {
				t.Errorf("golden %s differs (run with -update to rewrite)\n--- got ---\n%s", path, got)
			}
			// Every golden plan must be stable when applied.
			if again := build(t, blocks, apply(tc.current, p)); !again.Empty() {
				t.Errorf("not idempotent: %+v", again)
			}
		})
	}
}

func (tc goldenCase) blocksFor(t *testing.T) []Block {
	if tc.blocks != nil {
		return tc.blocks
	}
	if tc.preset == "" {
		return row0(t, tc.names...)
	}
	data, err := os.ReadFile(filepath.Join("..", "deviceexport", "testdata", "presets", tc.preset+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var sp storage.StructuredPreset
	if err := json.Unmarshal(data, &sp); err != nil {
		t.Fatal(err)
	}
	chain, ok := sp.Guitars[tc.guitar]
	if !ok {
		t.Fatalf("no guitar %q in %s", tc.guitar, tc.preset)
	}
	var blocks []Block
	for i, eb := range chain {
		b := Block{Row: 0, Col: i}
		if _, isCap := testLibrary[eb.Model]; isCap {
			// A capture: the catalog id is the Neural Capture slot, the model text is the
			// capture's name, the Architect's type is the modeler label.
			b.ModelID, b.ModelerType, b.CaptureName = capID, eb.Type, eb.Model
		} else {
			b.ModelID = mid(t, eb.Model)
		}
		blocks = append(blocks, b)
	}
	return blocks
}
