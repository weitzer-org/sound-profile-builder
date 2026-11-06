package stomplayout

import "testing"

func TestClassifyCatalogModels(t *testing.T) {
	c := testClassifier(t)
	for _, tc := range []struct {
		model string
		want  Role
	}{
		// All four boost models in today's catalog.
		{"Rage Booster", RoleBoost},
		{"Exotic Z Boost", RoleBoost},
		{"John Mayer Justa Boost", RoleBoost},
		{"Tim Henson Boost", RoleBoost},
		// The boost rule is limited to Guitar Overdrive: these amps are not boosts.
		{"UK C30 TopBoost", RoleNone},
		{"UK C15 TopBoost", RoleNone},
		{"Exotic Bass Z Boost", RoleNone}, // Bass Overdrive category
		{"Myth Drive", RoleDrive},
		{"Brit Blues", RoleDrive},
		{"Jewel", RoleComp},
		{"Circular Delay", RoleDelay},
		{"Vintage Chorus", RoleMod},
		{"Room", RoleReverb},
		// No role.
		{"Brit 2203", RoleNone},           // guitar amp
		{"110 US PRN C10R", RoleNone},     // cab
		{"Parametric-8", RoleNone},        // EQ
		{"Bubba Wah", RoleNone},           // wah
		{"Pitch Shifter", RoleNone},       // pitch
		{"Love Meat", RoleNone},           // filter
		{"Utility Gate", RoleNone},        // utility
		{"Brit Bass 50 Bright", RoleNone}, // bass amp
		{"Freeze", RoleNone},              // morph
		{"Overlord Synth", RoleNone},      // synth
		{"Looper X", RoleNone},            // looper
		{"Send 1", RoleNone},              // FX loop
		{"Splitter AB", RoleNone},         // splitter
		{"Mixer", RoleNone},               // mixer
		{"Single (M)", RoleNone},          // IR loader
	} {
		cl := c.Classify(Block{ModelID: mid(t, tc.model)})
		if cl.Role != tc.want {
			t.Errorf("%s (%s): role %q, want %q", tc.model, cl.Category, cl.Role, tc.want)
		}
		if !cl.Decided || cl.Source != SourceCatalog || cl.Capture {
			t.Errorf("%s: decided=%v source=%q capture=%v", tc.model, cl.Decided, cl.Source, cl.Capture)
		}
	}
}

func TestClassifyUnknownModelID(t *testing.T) {
	cl := testClassifier(t).Classify(Block{ModelID: 999999})
	if cl.Decided || cl.Role != RoleNone || cl.Reason == "" {
		t.Errorf("unknown id: %+v", cl)
	}
}

// Every row of the section 4.1 step 1 table, with a capture name that has no library entry
// (so the modeler label alone decides) and a neutral one.
func TestClassifyCaptureModelerTable(t *testing.T) {
	c := testClassifier(t)
	for _, tc := range []struct {
		typ, name string
		want      Role
		decided   bool // false = undecided at step 1, and nothing else decides it either
	}{
		{"Boost", "Anything", RoleBoost, true},
		{" boost ", "Anything", RoleBoost, true}, // trimmed, case-insensitive
		{"Drive", "Anything", RoleDrive, true},
		{"Overdrive", "Anything", RoleDrive, true},
		{"Distortion", "Anything", RoleDrive, true},
		{"Fuzz", "Anything", RoleDrive, true},
		{"Overdrive (Capture)", "Anything", RoleDrive, true},
		{"Overdrive Capture", "Anything", RoleDrive, true},
		{"Neural Capture (Drive)", "Anything", RoleDrive, true},
		// The drive rows become boost by the name rule.
		{"Drive", "Super BOOSTER 9", RoleBoost, true},
		{"Fuzz", "tiny boost", RoleBoost, true},
		{"Neural Capture (Drive)", "Clean Boost", RoleBoost, true},
		{"Compressor", "Anything", RoleComp, true},
		{"Delay", "Anything", RoleDelay, true},
		{"Modulation", "Anything", RoleMod, true},
		{"Reverb", "Anything", RoleReverb, true},
		{"Amplifier", "Anything", RoleNone, true},
		{"Cabinet", "Anything", RoleNone, true},
		{"Preamp", "Anything", RoleNone, true},
		// "Boost" in the name does not turn a non-drive label into a boost.
		{"Amplifier", "Boost Channel", RoleNone, true},
		// Undecided: no library entry, so unclassified.
		{"Capture", "Anything", RoleNone, false},
		{"Neural Capture", "Anything", RoleNone, false},
		{"", "Anything", RoleNone, false},
		{"Wobbly Thing", "Anything", RoleNone, false},
	} {
		cl := c.Classify(Block{ModelID: capID, ModelerType: tc.typ, CaptureName: tc.name})
		if !cl.Capture {
			t.Fatalf("%q not seen as a capture", tc.typ)
		}
		if cl.Role != tc.want || cl.Decided != tc.decided || cl.Unclassified == tc.decided {
			t.Errorf("type %q name %q: role=%q decided=%v unclassified=%v, want %q/%v",
				tc.typ, tc.name, cl.Role, cl.Decided, cl.Unclassified, tc.want, tc.decided)
		}
		if tc.decided && cl.Source != SourceModeler {
			t.Errorf("type %q: source %q", tc.typ, cl.Source)
		}
	}
}

func TestClassifyCaptureLibraryFallback(t *testing.T) {
	c := testClassifier(t)
	for _, tc := range []struct {
		typ, name string
		want      Role
		unclass   bool
	}{
		{"Capture", "Boss Blues Driver Waza", RoleDrive, false},
		{"Neural Capture", "Boss Blues Driver Waza", RoleDrive, false},
		{"", "Boss Blues Driver Waza", RoleDrive, false},
		{"Capture", "Strymon Sunset BOOST", RoleBoost, false}, // name rule applies to the library too
		{"Capture", "Fender Champ V2", RoleNone, false},       // amp
		{"Capture", "Sears Silvertone", RoleNone, false},      // full_rig
		{"Capture", "Some Cab Capture", RoleNone, false},      // cab
		{"Capture", "Odd Library Entry", RoleNone, false},     // other
		{"Capture", "strymon sunset boost", RoleNone, true},   // exact match only
		{"Capture", "Not In The Library", RoleNone, true},
		{"Capture", "Weird Type", RoleNone, true}, // a block_type this package doesn't know
		{"Capture", "", RoleNone, true},
	} {
		cl := c.Classify(Block{ModelID: capID, ModelerType: tc.typ, CaptureName: tc.name})
		if cl.Role != tc.want || cl.Unclassified != tc.unclass {
			t.Errorf("type %q name %q: role=%q unclassified=%v, want %q/%v", tc.typ, tc.name, cl.Role, cl.Unclassified, tc.want, tc.unclass)
		}
		if !tc.unclass && cl.Source != SourceLibrary {
			t.Errorf("name %q: source %q, want library", tc.name, cl.Source)
		}
		if tc.unclass && (cl.Decided || cl.Reason == "") {
			t.Errorf("name %q: unclassified result %+v", tc.name, cl)
		}
	}
}

func TestClassifyCaptureNilLibrary(t *testing.T) {
	c := NewClassifier(testCatalog(t), nil)
	if cl := c.Classify(Block{ModelID: capID, CaptureName: "Boss Blues Driver Waza"}); !cl.Unclassified {
		t.Errorf("nil library should leave it unclassified: %+v", cl)
	}
	if cl := c.Classify(Block{ModelID: capID, ModelerType: "Drive", CaptureName: "x"}); cl.Role != RoleDrive {
		t.Errorf("modeler label needs no library: %+v", cl)
	}
}

func TestClassifyCaptureDisagreement(t *testing.T) {
	c := testClassifier(t)
	// The modeler says Amplifier, the library says drive: the modeler wins.
	cl := c.Classify(Block{ModelID: capID, ModelerType: "Amplifier", CaptureName: "Boss Blues Driver Waza"})
	if cl.Role != RoleNone || cl.Source != SourceModeler || cl.Disagreement == "" {
		t.Errorf("amp vs drive: %+v", cl)
	}
	// The other way round.
	cl = c.Classify(Block{ModelID: capID, ModelerType: "Reverb", CaptureName: "Boss Blues Driver Waza"})
	if cl.Role != RoleReverb || cl.Disagreement == "" {
		t.Errorf("reverb vs drive: %+v", cl)
	}
	// Agreement is not a disagreement: Drive vs drive, Boost vs the name rule.
	for _, b := range []Block{
		{ModelID: capID, ModelerType: "Drive", CaptureName: "Boss Blues Driver Waza"},
		{ModelID: capID, ModelerType: "Boost", CaptureName: "Strymon Sunset BOOST"},
		{ModelID: capID, ModelerType: "Amplifier", CaptureName: "Fender Champ V2"},
	} {
		if cl := c.Classify(b); cl.Disagreement != "" {
			t.Errorf("%+v: unexpected disagreement %q", b, cl.Disagreement)
		}
	}
	// "Drive" vs a library boost: drive-or-boost uses the name in both, so they agree.
	if cl := c.Classify(Block{ModelID: capID, ModelerType: "Drive", CaptureName: "Strymon Sunset BOOST"}); cl.Role != RoleBoost || cl.Disagreement != "" {
		t.Errorf("Drive + boost name: %+v", cl)
	}
	// "Boost" label vs a library drive whose name has no "boost": modeler wins, disagree.
	if cl := c.Classify(Block{ModelID: capID, ModelerType: "Boost", CaptureName: "Boss Blues Driver Waza"}); cl.Role != RoleBoost || cl.Disagreement == "" {
		t.Errorf("Boost vs drive: %+v", cl)
	}
}

// A modeler type or capture name on a plain catalog block is ignored.
func TestClassifyNonCaptureIgnoresCaptureFields(t *testing.T) {
	cl := testClassifier(t).Classify(Block{ModelID: mid(t, "Brit 2203"), ModelerType: "Boost", CaptureName: "Strymon Sunset BOOST"})
	if cl.Role != RoleNone || cl.Capture {
		t.Errorf("%+v", cl)
	}
}
