package stomplayout

import (
	"testing"

	"github.com/weitzer-org/sound-builder/internal/catalog"
)

// capID is the catalog's "Capture 2" slot in category Neural Capture. How a capture block
// appears on the unit is UNVERIFIED (design doc G7 h); the planner only needs the category.
const capID = 14001

// testLibrary stands in for user_captures.json so the goldens don't move when the library does.
var testLibrary = map[string]string{
	"Strymon Sunset BOOST":     "drive",
	"Boss Blues Driver Waza":   "drive",
	"Fender Champ V2":          "amp",
	"Sears Silvertone":         "full_rig",
	"66 AC30 JMP OD":           "full_rig",
	"1964 Fender Super Reverb": "full_rig",
	"Some Cab Capture":         "cab",
	"Odd Library Entry":        "other",
	"Weird Type":               "mystery",
}

func lib(name string) (string, bool) { bt, ok := testLibrary[name]; return bt, ok }

var testCat *catalog.Catalog

func testCatalog(t testing.TB) *catalog.Catalog {
	t.Helper()
	if testCat == nil {
		c, err := catalog.Embedded()
		if err != nil {
			t.Fatal(err)
		}
		testCat = c
	}
	return testCat
}

func testClassifier(t testing.TB) *Classifier { return NewClassifier(testCatalog(t), lib) }

func testLayout(t testing.TB) *Layout {
	t.Helper()
	l, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// mid resolves a catalog model name to its id (the first model with that name).
func mid(t testing.TB, name string) int {
	t.Helper()
	ms := testCatalog(t).Resolve(name)
	if len(ms) == 0 {
		t.Fatalf("model %q is not in the catalog", name)
	}
	return ms[0].ID
}

// row0 lays names out on row 0, col = index.
func row0(t testing.TB, names ...string) []Block {
	bs := make([]Block, len(names))
	for i, n := range names {
		bs[i] = Block{Row: 0, Col: i, ModelID: mid(t, n)}
	}
	return bs
}

func build(t testing.TB, blocks []Block, cur []Assignment) Plan {
	t.Helper()
	p, err := Build(testLayout(t), testClassifier(t), blocks, cur)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// apply returns cur after the plan's removals and then its assignments, as the agent would
// do it (latching only). The move-on-reassign behaviour it relies on is UNVERIFIED (G7 c),
// which is why the plan removes first.
func apply(cur []Assignment, p Plan) []Assignment {
	var out []Assignment
	for _, a := range cur {
		gone := false
		for _, r := range p.Remove {
			if r.Row == a.Row && r.Col == a.Col && r.Switch == a.Switch {
				gone = true
			}
		}
		if !gone {
			out = append(out, a)
		}
	}
	for _, s := range p.Assign {
		out = append(out, Assignment{Row: s.Row, Col: s.Col, Switch: s.Switch})
	}
	return out
}
