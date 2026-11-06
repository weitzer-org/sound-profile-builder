package deviceexport

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/stomplayout"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func st(row, col int, sw string) stomplayout.Stomp {
	return stomplayout.Stomp{Row: row, Col: col, Switch: sw}
}

// These tests exercise the planner's output as the exporter wraps it. Every switch index
// and assignment behaviour is UNVERIFIED on the unit until gate G7; nothing here touches a
// device.
func TestStompsInExport(t *testing.T) {
	cases := []struct {
		name       string
		chain      []storage.EffectBlock
		want       []stomplayout.Stomp
		skipped    int // placed blocks that get no switch
		unresolved int
	}{
		{"pushable chain gets the layout",
			[]storage.EffectBlock{block("Opto Comp (M)"), block("Rage Booster"), block("Myth Drive"), block("Obsessive Drive"),
				block("Brit 2203"), block("Tape Delay (ST)"), block("Vintage Chorus"), block("Hall")},
			[]stomplayout.Stomp{st(0, 0, "A"), st(0, 1, "B"), st(0, 2, "C"), st(0, 3, "D"), st(0, 5, "E"), st(0, 6, "F"), st(0, 7, "G")}, 1, 0},
		{"no switchable blocks gives an empty array",
			[]storage.EffectBlock{block("Brit 2203")}, []stomplayout.Stomp{}, 1, 0},
		{"empty chain gives an empty array", []storage.EffectBlock{}, []stomplayout.Stomp{}, 0, 0},
		{"several blocks of one role share a switch",
			[]storage.EffectBlock{block("Myth Drive"), block("Obsessive Drive"), block("Rage Booster"), block("Myth Drive"), block("Hall"), block("Spring (M)")},
			[]stomplayout.Stomp{st(0, 0, "C"), st(0, 1, "D"), st(0, 2, "B"), st(0, 3, "D"), st(0, 4, "G"), st(0, 5, "G")}, 0, 0},
		{"an unresolved block is not placed and gets no switch; columns keep their gap",
			[]storage.EffectBlock{block("Myth Drive"), block("FNDR TWDLX IN Edge BAL CAB"), block("Hall")},
			[]stomplayout.Stomp{st(0, 0, "C"), st(0, 2, "G")}, 0, 1},
		{"an unknown block is not placed",
			[]storage.EffectBlock{block("Not A Real Block"), block("Hall")},
			[]stomplayout.Stomp{st(0, 1, "G")}, 0, 1},
		{"blocks past the 8th are not placed",
			nine(), []stomplayout.Stomp{st(0, 0, "C"), st(0, 1, "D"), st(0, 2, "D"), st(0, 3, "D"), st(0, 4, "D"), st(0, 5, "D"), st(0, 6, "D"), st(0, 7, "D")}, 0, 1},
	}
	e := testExporter(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ex, p := export(t, e, preset(c.chain...), "g")
			if p.Stomps == nil {
				t.Fatal("stomps is nil; it must always be an array")
			}
			if !reflect.DeepEqual(p.Stomps, c.want) {
				t.Errorf("stomps = %+v, want %+v", p.Stomps, c.want)
			}
			if len(p.Unresolved) != c.unresolved {
				t.Errorf("unresolved = %v, want %d", p.Unresolved, c.unresolved)
			}
			// Never a switch for a cell with no placed block.
			placed := map[int]bool{}
			for _, b := range p.Rows[0].Blocks {
				placed[b.Col] = true
			}
			for _, s := range p.Stomps {
				if s.Row != GridRow || !placed[s.Col] {
					t.Errorf("stomp %+v is not on a placed block", s)
				}
				if s.Switch == "H" {
					t.Errorf("stomp %+v uses H", s)
				}
			}
			if ex.Stomps == nil || len(ex.Stomps.Assigned) != len(c.want) || len(ex.Stomps.Skipped) != c.skipped {
				t.Fatalf("dry-run view %+v: want %d assigned, %d skipped", ex.Stomps, len(c.want), c.skipped)
			}
			for i, a := range ex.Stomps.Assigned {
				if a.Switch != c.want[i].Switch || a.Col != c.want[i].Col {
					t.Errorf("view %+v disagrees with payload %+v", a, c.want[i])
				}
			}
		})
	}
}

// The raw JSON must carry "stomps": [] (not null) with row/col/switch keys, and no "column".
func TestStompsJSONShape(t *testing.T) {
	e := testExporter(t)
	ex, _ := export(t, e, preset(block("Brit 2203")), "g")
	if !bytes.Contains(ex.Payload, []byte(`"stomps":[]`)) {
		t.Errorf("empty layout must marshal as []: %s", ex.Payload)
	}
	ex, _ = export(t, e, preset(block("Hall")), "g")
	if !bytes.Contains(ex.Payload, []byte(`"stomps":[{"row":0,"col":0,"switch":"G"}]`)) {
		t.Errorf("payload %s", ex.Payload)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(ex.Payload, &raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw["stomps"]), "column") {
		t.Error("stomps must use col, never column")
	}
	if ex.Format != "qc2-device-export/1" {
		t.Errorf("format changed to %q; a bump needs the owner's decision", ex.Format)
	}
}

func TestStompsDeterministicAndHashBound(t *testing.T) {
	e := testExporter(t)
	sp := preset(block("Myth Drive"), block("Hall"))
	a, _ := export(t, e, sp, "g")
	b, _ := export(t, e, sp, "g")
	if !bytes.Equal(a.Payload, b.Payload) {
		t.Error("export is not deterministic")
	}
	other, _ := export(t, e, preset(block("Hall"), block("Myth Drive")), "g")
	h1, _ := a.PlanSHA256()
	h2, _ := other.PlanSHA256()
	if h1 == h2 {
		t.Error("plan hash did not change when the layout did")
	}
}

// A unit-test capture lookup is wired through Options without importing internal/agents.
func TestStompCapturesOption(t *testing.T) {
	called := false
	e, err := NewEmbedded(Options{StompCaptures: func(string) (string, bool) { called = true; return "", false }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Export(t.Context(), preset(block("Myth Drive")), "g"); err != nil {
		t.Fatal(err)
	}
	// v1 never places a capture, so the library is not asked yet.
	if called {
		t.Error("capture lookup consulted although no capture is placed")
	}
}

// TestStompsForCaptures: a placed capture block is classified as a Neural Capture block
// (its id is picked on the unit), so a drive capture gets a drive switch and an amp
// capture none.
func TestStompsForCaptures(t *testing.T) {
	e := testExporter(t)
	drive := block("66 AC30 JMP OD")
	drive.Type = "Overdrive (Capture)"
	amp := block("CA John's")
	amp.Type = "Amplifier"
	ex, p := export(t, e, preset(drive, amp, block("Hall")), "g")
	if len(p.Unresolved) != 0 || len(p.Rows[0].Blocks) != 3 {
		t.Fatalf("unresolved %+v, blocks %d", p.Unresolved, len(p.Rows[0].Blocks))
	}
	got := map[int]string{}
	for _, s := range p.Stomps {
		got[s.Col] = s.Switch
	}
	if got[0] == "" || got[1] != "" || got[2] != "G" {
		t.Fatalf("stomps %+v (view %+v)", p.Stomps, ex.Stomps)
	}
	for _, s := range ex.Stomps.Skipped {
		if strings.Contains(s.Reason, "unknown model id") {
			t.Errorf("capture block not classified: %+v", s)
		}
	}
}
