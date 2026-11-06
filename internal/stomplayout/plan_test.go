package stomplayout

import (
	"reflect"
	"strings"
	"testing"
)

func st(row, col int, sw string) Stomp { return Stomp{Row: row, Col: col, Switch: sw} }

func wantAssign(t *testing.T, p Plan, want ...Stomp) {
	t.Helper()
	if want == nil {
		want = []Stomp{}
	}
	if !reflect.DeepEqual(p.Assign, want) {
		t.Errorf("assign = %v, want %v", p.Assign, want)
	}
}

func wantRemove(t *testing.T, p Plan, want ...Stomp) {
	t.Helper()
	if want == nil {
		want = []Stomp{}
	}
	if !reflect.DeepEqual(p.Remove, want) {
		t.Errorf("remove = %v, want %v", p.Remove, want)
	}
}

func hasWarning(p Plan, sub string) bool {
	for _, w := range p.Warnings {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

func hasSkip(p Plan, row, col int, sub string) bool {
	for _, s := range p.Skipped {
		if s.Row == row && s.Col == col && strings.Contains(s.Reason, sub) {
			return true
		}
	}
	return false
}

// Rule 2 and the layout itself: one block of each role lands on its own switch.
func TestPlanFullChain(t *testing.T) {
	p := build(t, row0(t, "Jewel", "Rage Booster", "Brit Blues", "Myth Drive", "Brit 2203", "110 US PRN C10R",
		"Vintage Chorus", "Circular Delay"), nil)
	wantAssign(t, p, st(0, 0, "A"), st(0, 1, "B"), st(0, 2, "C"), st(0, 3, "D"), st(0, 6, "F"), st(0, 7, "E"))
	wantRemove(t, p)
	if !hasSkip(p, 0, 4, "Guitar Amplifier") || !hasSkip(p, 0, 5, "Cabsim") {
		t.Errorf("amp and cab should be listed as skipped: %+v", p.Skipped)
	}
	if len(p.Roles) != 8 || p.Roles[1].Role != RoleBoost || p.Roles[1].Switch != "B" {
		t.Errorf("roles: %+v", p.Roles)
	}
}

// Rule 1: signal order is row-major, whatever order the blocks arrive in.
func TestPlanSignalOrderIsRowMajor(t *testing.T) {
	blocks := []Block{
		{Row: 1, Col: 0, ModelID: mid(t, "Myth Drive")},      // third in order
		{Row: 0, Col: 5, ModelID: mid(t, "Brit Blues")},      // second
		{Row: 0, Col: 2, ModelID: mid(t, "Green 808")},       // first
		{Row: 1, Col: 4, ModelID: mid(t, "Obsessive Drive")}, // fourth
	}
	p := build(t, blocks, nil)
	wantAssign(t, p, st(0, 2, "C"), st(0, 5, "D"), st(1, 0, "D"), st(1, 4, "D"))
	// Roles come back in signal order too.
	var got [][2]int
	for _, r := range p.Roles {
		got = append(got, [2]int{r.Row, r.Col})
	}
	if !reflect.DeepEqual(got, [][2]int{{0, 2}, {0, 5}, {1, 0}, {1, 4}}) {
		t.Errorf("roles order %v", got)
	}
}

// Rule 3: first drive on C, second and later on D.
func TestPlanThreeDrivesCDD(t *testing.T) {
	p := build(t, row0(t, "Myth Drive", "Brit Blues", "Obsessive Drive"), nil)
	wantAssign(t, p, st(0, 0, "C"), st(0, 1, "D"), st(0, 2, "D"))
}

// A boost is not a drive for the C/D count.
func TestPlanBoostDoesNotCountAsDrive(t *testing.T) {
	p := build(t, row0(t, "Rage Booster", "Myth Drive", "Brit Blues"), nil)
	wantAssign(t, p, st(0, 0, "B"), st(0, 1, "C"), st(0, 2, "D"))
}

// Rule 2: several blocks of one role share the switch.
func TestPlanTwoDelaysGangOnE(t *testing.T) {
	p := build(t, row0(t, "Circular Delay", "Brit 2203", "Analog Delay (M)", "Room", "Spring (ST)"), nil)
	wantAssign(t, p, st(0, 0, "E"), st(0, 2, "E"), st(0, 3, "G"), st(0, 4, "G"))
}

// Rule 4: a preset with no mod leaves F empty, and nothing shifts into it.
func TestPlanMissingModLeavesFEmpty(t *testing.T) {
	p := build(t, row0(t, "Jewel", "Myth Drive", "Circular Delay", "Room"), nil)
	wantAssign(t, p, st(0, 0, "A"), st(0, 1, "C"), st(0, 2, "E"), st(0, 3, "G"))
	for _, s := range p.Assign {
		if s.Switch == "F" {
			t.Errorf("F assigned: %v", s)
		}
	}
	// A reverb alone stays on G, not on the first free switch.
	wantAssign(t, build(t, row0(t, "Room"), nil), st(0, 0, "G"))
}

// Rule 5: H is never assigned, never removed, and a block on H stays there.
func TestPlanHIsUntouched(t *testing.T) {
	blocks := row0(t, "Jewel", "Circular Delay", "Brit 2203")
	cur := []Assignment{
		{Row: 0, Col: 1, Switch: "H"}, // delay: has its own switch (E), but sits on H
		{Row: 0, Col: 2, Switch: "H"}, // amp: no role
	}
	p := build(t, blocks, cur)
	wantAssign(t, p, st(0, 0, "A"))
	wantRemove(t, p)
	if !hasSkip(p, 0, 1, "already on H") {
		t.Errorf("delay on H should be skipped: %+v", p.Skipped)
	}
	if p.Roles[1].Role != RoleDelay || p.Roles[1].Switch != "" {
		t.Errorf("roles: %+v", p.Roles[1])
	}
	for _, s := range append(append([]Stomp{}, p.Assign...), p.Remove...) {
		if s.Switch == "H" {
			t.Errorf("plan touches H: %v", s)
		}
	}
}

// Rule 5 with rule 6: a block on H that also has a stale A-G assignment loses only the A-G one.
func TestPlanHBlockLosesStaleAG(t *testing.T) {
	p := build(t, row0(t, "Circular Delay"), []Assignment{{0, 0, "H", false}, {0, 0, "A", false}})
	wantAssign(t, p)
	wantRemove(t, p, st(0, 0, "A"))
}

// Rule 3 counts a drive parked on H as the first drive, so the next one still goes to D.
// (Literal reading of rules 1, 3 and 5; the doc is silent on this case.)
func TestPlanDriveOnHStillCountsAsFirst(t *testing.T) {
	p := build(t, row0(t, "Myth Drive", "Brit Blues"), []Assignment{{0, 0, "H", false}})
	wantAssign(t, p, st(0, 1, "D"))
}

// Rule 6: stale A-G assignments go, wrong ones are replaced, matching ones stay.
func TestPlanReplacesAG(t *testing.T) {
	blocks := row0(t, "Jewel", "Myth Drive", "Brit 2203", "Room")
	cur := []Assignment{
		{Row: 0, Col: 0, Switch: "A"}, // comp on A: correct, stays
		{Row: 0, Col: 1, Switch: "F"}, // drive on F: wrong, becomes C
		{Row: 0, Col: 2, Switch: "B"}, // amp on B: stale, removed
		{Row: 0, Col: 3, Switch: "G"}, // reverb on G: correct
	}
	p := build(t, blocks, cur)
	wantRemove(t, p, st(0, 1, "F"), st(0, 2, "B"))
	wantAssign(t, p, st(0, 1, "C"))
}

// A block with a role the layout has no switch for is skipped, and its stale assignment goes.
func TestPlanLayoutWithoutRole(t *testing.T) {
	l, err := ParseLayout([]byte(`{"version":1,"switches":{"A":"comp"},"reserved":["H"]}`))
	if err != nil {
		t.Fatal(err)
	}
	blocks := row0(t, "Jewel", "Room")
	p, err := Build(l, testClassifier(t), blocks, []Assignment{{0, 1, "G", false}})
	if err != nil {
		t.Fatal(err)
	}
	wantAssign(t, p, st(0, 0, "A"))
	wantRemove(t, p, st(0, 1, "G"))
	if !hasSkip(p, 0, 1, "no switch for reverb") {
		t.Errorf("skipped: %+v", p.Skipped)
	}
}

// Without a drive2 slot, later drives gang on the drive switch (not in the doc's layout).
func TestPlanNoDrive2GangsOnDrive(t *testing.T) {
	l, err := ParseLayout([]byte(`{"version":1,"switches":{"C":"drive"},"reserved":["H"]}`))
	if err != nil {
		t.Fatal(err)
	}
	p, _ := Build(l, testClassifier(t), row0(t, "Myth Drive", "Brit Blues"), nil)
	wantAssign(t, p, st(0, 0, "C"), st(0, 1, "C"))
}

// Running the layout on its own output gives an empty plan.
func TestPlanIsIdempotent(t *testing.T) {
	blocks := row0(t, "Jewel", "Rage Booster", "Myth Drive", "Brit Blues", "Obsessive Drive", "Brit 2203",
		"Circular Delay", "Vintage Chorus", "Room")
	stale := []Assignment{{0, 5, "A", false}, {0, 2, "G", false}, {0, 6, "H", false}}
	first := build(t, blocks, stale)
	after := apply(stale, first)
	second := build(t, blocks, after)
	if !second.Empty() {
		t.Errorf("second run not a no-op: assign=%v remove=%v", second.Assign, second.Remove)
	}
	// And from nothing.
	cur := apply(nil, build(t, blocks, nil))
	if p := build(t, blocks, cur); !p.Empty() {
		t.Errorf("run from empty not idempotent: %+v", p)
	}
}

// Rule 7: a momentary layout switch gives a warning, not a write.
func TestPlanMomentaryLayoutSwitchWarns(t *testing.T) {
	p := build(t, row0(t, "Rage Booster"), []Assignment{{0, 0, "B", true}})
	wantAssign(t, p)
	wantRemove(t, p)
	if !hasWarning(p, "momentary") {
		t.Errorf("warnings: %v", p.Warnings)
	}
	// A momentary assignment on the wrong switch is removed (and replaced by a latching one).
	p = build(t, row0(t, "Rage Booster"), []Assignment{{0, 0, "A", true}})
	wantRemove(t, p, st(0, 0, "A"))
	wantAssign(t, p, st(0, 0, "B"))
	// A momentary assignment on H is left alone, with no warning.
	p = build(t, row0(t, "Rage Booster"), []Assignment{{0, 0, "H", true}})
	if len(p.Warnings) != 0 || !p.Empty() {
		t.Errorf("momentary on H: %+v", p)
	}
}

// Rule 7, structurally: a plan entry has no momentary field and no secondary assignment.
func TestStompHasNoMomentaryField(t *testing.T) {
	if _, ok := reflect.TypeOf(Stomp{}).FieldByName("Momentary"); ok {
		t.Error("Stomp must not carry a momentary flag")
	}
}

// Section 4.1 end to end: capture handling shows up in the plan.
func TestPlanCaptures(t *testing.T) {
	blocks := []Block{
		{Row: 0, Col: 0, ModelID: capID, ModelerType: "Boost", CaptureName: "Anything"},
		{Row: 0, Col: 1, ModelID: capID, ModelerType: "Capture", CaptureName: "Strymon Sunset BOOST"},
		{Row: 0, Col: 2, ModelID: capID, ModelerType: "Amplifier", CaptureName: "Boss Blues Driver Waza"},
		{Row: 0, Col: 3, ModelID: capID, ModelerType: "Capture", CaptureName: "Mystery Pedal"},
		{Row: 0, Col: 4, ModelID: capID, ModelerType: "Drive", CaptureName: "Boss Blues Driver Waza"},
	}
	p := build(t, blocks, nil)
	wantAssign(t, p, st(0, 0, "B"), st(0, 1, "B"), st(0, 4, "C"))
	if !hasWarning(p, "modeler wins") || !hasWarning(p, `unclassified capture "Mystery Pedal"`) {
		t.Errorf("warnings: %v", p.Warnings)
	}
	if !hasSkip(p, 0, 3, "unclassified capture") || !hasSkip(p, 0, 2, "not a switchable") {
		t.Errorf("skipped: %+v", p.Skipped)
	}
	if p.Roles[1].Source != SourceLibrary || p.Roles[0].Source != SourceModeler {
		t.Errorf("sources: %+v", p.Roles)
	}
}

// An assignment whose cell has no block is left alone, with a warning.
func TestPlanAssignmentWithoutBlock(t *testing.T) {
	p := build(t, row0(t, "Room"), []Assignment{{0, 0, "G", false}, {2, 5, "C", false}})
	if !p.Empty() || !hasWarning(p, "no block") {
		t.Errorf("%+v", p)
	}
}

func TestPlanEmptyInput(t *testing.T) {
	p := build(t, nil, nil)
	if !p.Empty() || p.Assign == nil || p.Remove == nil || p.Skipped == nil || p.Warnings == nil || p.Roles == nil {
		t.Errorf("empty input should give empty non-nil lists: %+v", p)
	}
}

func TestPlanUnknownModelIsSkipped(t *testing.T) {
	p := build(t, []Block{{Row: 0, Col: 0, ModelID: 999999}}, nil)
	if !hasSkip(p, 0, 0, "unknown model id") {
		t.Errorf("%+v", p.Skipped)
	}
}

func TestBuildRejectsBadInput(t *testing.T) {
	l, c := testLayout(t), testClassifier(t)
	room := mid(t, "Room")
	for name, fn := range map[string]func() error{
		"duplicate cell": func() error {
			_, err := Build(l, c, []Block{{0, 1, room, "", ""}, {0, 1, room, "", ""}}, nil)
			return err
		},
		"negative row": func() error { _, err := Build(l, c, []Block{{-1, 0, room, "", ""}}, nil); return err },
		"negative col": func() error { _, err := Build(l, c, []Block{{0, -1, room, "", ""}}, nil); return err },
		"bad switch": func() error {
			_, err := Build(l, c, []Block{{0, 0, room, "", ""}}, []Assignment{{0, 0, "I", false}})
			return err
		},
		"lower-case switch": func() error {
			_, err := Build(l, c, []Block{{0, 0, room, "", ""}}, []Assignment{{0, 0, "g", false}})
			return err
		},
	} {
		if fn() == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// The planner must not change its inputs.
func TestBuildDoesNotMutateInput(t *testing.T) {
	blocks := []Block{{Row: 1, Col: 0, ModelID: mid(t, "Room")}, {Row: 0, Col: 0, ModelID: mid(t, "Jewel")}}
	cur := []Assignment{{Row: 1, Col: 0, Switch: "A"}, {Row: 0, Col: 0, Switch: "G"}}
	b0, c0 := append([]Block(nil), blocks...), append([]Assignment(nil), cur...)
	build(t, blocks, cur)
	if !reflect.DeepEqual(blocks, b0) || !reflect.DeepEqual(cur, c0) {
		t.Error("input was modified")
	}
}
