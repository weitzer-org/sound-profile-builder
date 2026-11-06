package mcpserver

import (
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
)

func TestPushLayout(t *testing.T) {
	l, err := pushLayout(devicePushInput{Guitar: "ES"})
	if l != nil || err != nil {
		t.Fatalf("no scene fields should keep the original push: %v %v", l, err)
	}
	l, err = pushLayout(devicePushInput{Guitar: "ES", SecondGuitar: "Tele", SecondRhythm: "e", SecondLead: "F"})
	if err != nil || l == nil || len(l.Guitars) != 2 || l.Guitars[0].Rhythm != 0 || l.Guitars[0].Lead != 1 ||
		l.Guitars[1].Rhythm != 4 || l.Guitars[1].Lead != 5 {
		t.Fatalf("layout %+v err %v", l, err)
	}
	for name, in := range map[string]devicePushInput{
		"second without scenes": {Guitar: "ES", SecondGuitar: "Tele"},
		"scenes without second": {Guitar: "ES", SecondRhythm: "E", SecondLead: "F"},
		"bad letter":            {Guitar: "ES", RhythmScene: "Z"},
		"shared scene":          {Guitar: "ES", SecondGuitar: "Tele", SecondRhythm: "B", SecondLead: "C"},
		"same guitar":           {Guitar: "ES", SecondGuitar: "ES", SecondRhythm: "C", SecondLead: "D"},
	} {
		if _, err := pushLayout(in); err == nil {
			t.Errorf("%s should be refused", name)
		}
	}
}

func TestRenderLayoutTable(t *testing.T) {
	scenes := func(a, b, e, f string) []string { return []string{a, b, a, a, e, f, a, a} }
	var pl devicejobs.PlanLayout
	for i := range pl.Scenes {
		pl.Scenes[i] = devicejobs.PlanScene{Letter: devicejobs.Letter(i), Role: "unused"}
	}
	pl.Scenes[0] = devicejobs.PlanScene{Letter: "A", Guitar: "ES", Role: "rhythm"}
	pl.Scenes[1] = devicejobs.PlanScene{Letter: "B", Guitar: "ES", Role: "lead"}
	pl.Scenes[4] = devicejobs.PlanScene{Letter: "E", Guitar: "Tele", Role: "rhythm"}
	pl.Scenes[5] = devicejobs.PlanScene{Letter: "F", Guitar: "Tele", Role: "lead"}
	var b strings.Builder
	renderExportTable(&b, []devicejobs.PlanBlock{{Model: "Myth Drive", ModelID: 1, BypassScenes: []bool{false, false, false, false, true, false, false, false},
		Params: []devicejobs.PlanParam{{Name: "GAIN", A: "2", Scenes: scenes("2", "8", "4", "6")}}}}, nil, nil, nil, &pl)
	out := b.String()
	for _, want := range []string{"Scene A: ES rhythm", "Scene F: Tele lead", "| 2 | 8 | 4 | 6 |", "Active/Active/Bypassed/Active",
		"Scenes C, D, G, H are unused"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Scenes C-H get Scene A") {
		t.Errorf("layout table must not claim C-H copy A:\n%s", out)
	}
}
