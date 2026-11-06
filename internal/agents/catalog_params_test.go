package agents

import (
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestParamNameKeyHasNoCollisions(t *testing.T) {
	cat, err := catalog.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range buildDeviceBlocks(cat) {
		seen := map[string]string{}
		for _, p := range b.Params {
			if prev, dup := seen[p.Key]; dup && prev != p.Label {
				t.Errorf("%s: %q and %q share key %q", b.Name, prev, p.Label, p.Key)
			}
			seen[p.Key] = p.Label
		}
	}
	for written, want := range map[string]string{
		"Pre-delay": "PREDELAY", "PRE DELAY": "PREDELAY", "65 Hz": "65HZ", "OCT +1": "OCT+1", "OCT -1": "OCT-1",
	} {
		if got := paramNameKey(written); got != want {
			t.Errorf("paramNameKey(%q) = %q, want %q", written, got, want)
		}
	}
}

func mustDeviceBlock(t *testing.T, name string) *deviceBlock {
	t.Helper()
	b, ok := deviceBlockFor(name)
	if !ok {
		t.Fatalf("%q not in the device catalog", name)
	}
	return b
}

func TestDeviceBlockDropsInternalParamsAndNumbersMicSlots(t *testing.T) {
	cab := mustDeviceBlock(t, "412 Brit 60B V30 95")
	got := strings.Join(cab.labels(), ",")
	for _, internal := range []string{"bypass", "phi", "ir selector"} {
		if strings.Contains(got, internal) {
			t.Errorf("cab labels include internal %q: %s", internal, got)
		}
	}
	for _, want := range []string{"MIC 1 LEVEL", "MIC 2 LEVEL", "MIC 1 POSITION", "MIC 2 DISTANCE", "HPF", "LPF", "OUTPUT VOLUME"} {
		if _, ok := cab.byKey[paramNameKey(want)]; !ok {
			t.Errorf("cab is missing %q: %s", want, got)
		}
	}
	// The family merges (M) and (ST): the stereo cab has BALANCE where the mono one has PAN.
	if _, ok := cab.byKey["MIC1BALANCE"]; !ok {
		t.Errorf("family should include the stereo model's MIC 1 BALANCE: %s", got)
	}
	// A meter is not a control.
	comp := mustDeviceBlock(t, "VCA Comp")
	if _, ok := comp.byKey["GAINREDUCTION"]; ok {
		t.Error("the gain-reduction meter should not be listed as a parameter")
	}
}

func TestDeviceBlockMergesDuplicateNamesWithWidestRange(t *testing.T) {
	// The device lists two "Graphic-9" models; their HPF bounds differ (19..250 and 19..500)
	// and only one has an LPF.
	eq := mustDeviceBlock(t, "Graphic-9")
	hpf, ok := eq.byKey["HPF"]
	if !ok || !hpf.numericRange() {
		t.Fatalf("Graphic-9 HPF = %+v", hpf)
	}
	if *hpf.Max != 500 {
		t.Errorf("merged HPF max = %v, want the wider 500", *hpf.Max)
	}
	if _, ok := eq.byKey["LPF"]; !ok {
		t.Error("merged Graphic-9 should include the LPF only one of the models has")
	}
}

func TestRenderDeviceParamForms(t *testing.T) {
	amp := mustDeviceBlock(t, "Brit 2203")
	line := renderDeviceBlock(amp)
	for _, want := range []string{"Brit 2203 [Guitar Amplifier]: ", "GAIN 0..10 d5", "OUTPUT -60..12dB min=OFF d0"} {
		if !strings.Contains(line, want) {
			t.Errorf("render missing %q: %s", want, line)
		}
	}
	cab := renderDeviceBlock(mustDeviceBlock(t, "412 Brit 60B V30 95 (M)"))
	for _, want := range []string{"MIC 1 LEVEL dB range?", "MIC 1 PAN 0=L..10=R d5", "HPF 20..500Hz min=OFF d20", "LPF 500..20000Hz max=OFF", "MIC 1 GRID MODE switch 0..1 d1"} {
		if !strings.Contains(cab, want) {
			t.Errorf("cab render missing %q: %s", want, cab)
		}
	}
	delay := renderDeviceBlock(mustDeviceBlock(t, "Analog Delay"))
	if !strings.Contains(delay, "TRAILS {") {
		t.Errorf("delay render should list TRAILS positions: %s", delay)
	}
}

// TestCabFamilyMarksModelOnlyControls: a family name merges the mono and stereo cab; the
// export places the mono one, so the Architect is told PAN is its to set and BALANCE is not.
func TestCabFamilyMarksModelOnlyControls(t *testing.T) {
	cab := mustDeviceBlock(t, "212 UK C30 65")
	line := renderDeviceBlock(cab)
	for _, want := range []string{"MIC 1 PAN 0=L..10=R d5 [mono cab only]", "MIC 1 BALANCE -1=L..1=R d0 [stereo cab only]"} {
		if !strings.Contains(line, want) {
			t.Errorf("render missing %q: %s", want, line)
		}
	}
	if strings.Contains(renderDeviceParam(cab.byKey["MIC1LEVEL"]), "only]") {
		t.Error("a control both cabs have must not be marked")
	}
	// A single model, and non-family merges (two Graphic-9 models), are not marked.
	if strings.Contains(renderDeviceBlock(mustDeviceBlock(t, "212 UK C30 65 (M)")), "only]") ||
		strings.Contains(renderDeviceBlock(mustDeviceBlock(t, "Graphic-9")), "only]") {
		t.Error("only a mono/stereo family marks model-only controls")
	}
}

// TestPanScaleRendering: a pan is shown with its catalog range, which end is L and R, and the
// default; a name whose models disagree on the scale stays "L..R" (nothing is guessed).
func TestPanScaleRendering(t *testing.T) {
	if got := renderDeviceParam(mustDeviceBlock(t, "412 Brit 60B V30 95 (M)").byKey["MIC1PAN"]); got != "MIC 1 PAN 0=L..10=R d5" {
		t.Errorf("mono cab pan = %q", got)
	}
	if got := renderDeviceParam(&deviceParam{Label: "PAN", Pan: true, PanScale: true, Min: fp(-1), Max: fp(1), Default: fp(0)}); got != "PAN -1=L..1=R d0" {
		t.Errorf("-1..1 pan = %q", got)
	}
	if got := renderDeviceParam(&deviceParam{Label: "PAN", Pan: true, Min: fp(0), Max: fp(10)}); got != "PAN L..R" {
		t.Errorf("a pan with no agreed scale = %q", got)
	}
	// Merging two models with different pan scales drops the scale.
	b := &deviceBlock{byKey: map[string]*deviceParam{}}
	b.merge("PAN", catalog.Param{MinLabel: "L", MaxLabel: "R", Min: fp(0), Max: fp(10), Default: fp(5)})
	b.merge("PAN", catalog.Param{MinLabel: "L", MaxLabel: "R", Min: fp(0), Max: fp(10), Default: fp(5)})
	if !b.byKey["PAN"].PanScale {
		t.Error("two models with the same pan scale should keep it")
	}
	b.merge("PAN", catalog.Param{MinLabel: "L", MaxLabel: "R", Min: fp(-1), Max: fp(1), Default: fp(0)})
	if b.byKey["PAN"].PanScale || renderDeviceParam(b.byKey["PAN"]) != "PAN L..R" {
		t.Errorf("disagreeing pan scales should render L..R, got %q", renderDeviceParam(b.byKey["PAN"]))
	}
}

// TestPanRangeCheckOnlyForCatalogAwareOutput: a pan outside its catalog range is noted only
// when the preset uses kept_defaults (the catalog-aware prompt, which showed the scale);
// v5-and-older output is unchanged.
func TestPanRangeCheckOnlyForCatalogAwareOutput(t *testing.T) {
	mk := func(kept []string) *storage.StructuredPreset {
		return catalogCheckFixture(storage.EffectBlock{Type: "Cabinet", Model: "412 Brit 60B V30 95", KeptDefaults: kept,
			Parameters: []storage.BlockParameter{{Name: "Mic 1 Pan", Value: "-40"}, {Name: "Mic 2 Pan", Value: "3.5"}}})
	}
	valid := map[string]bool{"412 Brit 60B V30 95": false}
	if got := FlagCatalogParamMismatches(mk(nil), valid); got.OutOfRange != 0 {
		t.Errorf("v5-style output: OutOfRange = %d, want 0", got.OutOfRange)
	}
	sp := mk([]string{})
	if got := FlagCatalogParamMismatches(sp, valid); got.OutOfRange != 1 {
		t.Errorf("catalog-aware output: OutOfRange = %d, want 1 (only -40; 3.5 is on the 0..10 scale)", got.OutOfRange)
	}
	if r := sp.Guitars["Guitar"][0].Rationale; !strings.Contains(r, "Mic 1 Pan -40 is outside the device's 0..10 range") {
		t.Errorf("rationale = %q", r)
	}
}

func TestCatalogParamContextSkipsCapturesUnknownAndUnownedPlugins(t *testing.T) {
	gate := NewPluginGate(true, []string{"John Mayer"})
	valid := map[string]bool{"Green 808": false, "Some Capture": true}
	got := CatalogParamContext([]string{
		"Green 808", "Green 808", // listed once
		"Some Capture",        // a capture: no catalog params
		"Made Up Amp 9000",    // not on the device
		"Tim Henson Lead",     // plugin the user doesn't own
		"412 Brit 60B V30 95", // family name
	}, valid, gate)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines (Green 808, the cab family), got %d:\n%s", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "Green 808 [Guitar Overdrive]: OVERDRIVE 0..10") {
		t.Errorf("line 0 = %s", lines[0])
	}
	if !strings.HasPrefix(lines[1], "412 Brit 60B V30 95 [") {
		t.Errorf("line 1 = %s", lines[1])
	}
	if CatalogParamContext([]string{"Tim Henson Lead"}, nil, NewPluginGate(true, []string{"Tim Henson"})) == "" {
		t.Error("an owned plugin model should be listed")
	}
}

func TestCatalogEffectNamesGatesPlugins(t *testing.T) {
	got := CatalogEffectNames(NewPluginGate(true, []string{"John Mayer"}))
	for _, want := range []string{"Delay: ", "Reverb: ", "Spring", "Adaptive Gate", "John Mayer Spring Reverb"} {
		if !strings.Contains(got, want) {
			t.Errorf("effects list missing %q", want)
		}
	}
	if strings.Contains(got, "Petrucci Wah") {
		t.Error("an unowned plugin's effect is listed")
	}
	if strings.Contains(got, "Brit 2203") {
		t.Error("amps belong to the Librarian, not the effects list")
	}
}

func TestLibrarianBlockNames(t *testing.T) {
	out := "```json\n" + `{"mandatory_blocks": ["Brit 2203", "412 Brit 60B V30 95"],
	  "suggested_blocks": ["Green 808 as a lead boost", "Spring (ST)", "Some Capture (My Capture)"],
	  "tactical_hints": ["Place Analog Delay before the amp"], "dropped_gear": []}` + "\n```"
	got := strings.Join(LibrarianBlockNames(out), "|")
	want := "Brit 2203|412 Brit 60B V30 95|Green 808|Spring (ST)"
	if got != want {
		t.Errorf("LibrarianBlockNames = %q, want %q (hints are not block picks)", got, want)
	}
	// Not JSON: the whole text is scanned, and a longer name wins over its prefix.
	got = strings.Join(LibrarianBlockNames("Use the Spring (ST) reverb after the Brit 2203."), "|")
	if got != "Spring (ST)|Brit 2203" {
		t.Errorf("free-text scan = %q", got)
	}
	// A shorter name whose first occurrence sits inside a longer one is still found later.
	got = strings.Join(LibrarianBlockNames("Try Plate Lush on leads; for rhythm a plain Plate."), "|")
	if got != "Plate Lush|Plate" {
		t.Errorf("shadowed first occurrence: %q, want Plate Lush|Plate", got)
	}
	if tokenIndex("Green 8080", "Green 808") != -1 || tokenIndex("a Green 808.", "Green 808") != 2 {
		t.Error("tokenIndex must match whole tokens only")
	}
}

// TestArchitectCatalogContextSize logs what the catalog sections add to the Architect's
// context for a typical 10-block preset, and fails if the per-model listing grows past a
// ceiling. Tokens are estimated as bytes/4, not Gemini's tokenizer. Run with -v.
func TestArchitectCatalogContextSize(t *testing.T) {
	names := []string{
		"US DLX 65 Reissue", "412 Brit 60B V30 95", "Green 808", "Analog Delay", "Spring",
		"Tremolo", "Adaptive Gate", "Graphic-9", "Opto Comp", "Volume",
	}
	gate := NewPluginGate(true, []string{"Cory Wong", "John Mayer"})
	params := CatalogParamContext(names, nil, gate)
	if n := strings.Count(params, "\n") + 1; n != len(names) {
		t.Fatalf("rendered %d models, want %d:\n%s", n, len(names), params)
	}
	effects := CatalogEffectNames(gate)
	t.Logf("%-34s %6d bytes  ~%5d tokens", "Device Block Parameters (10 models)", len(params), len(params)/4)
	t.Logf("%-34s %6d bytes  ~%5d tokens", "Effects list", len(effects), len(effects)/4)
	t.Logf("%-34s %6d bytes  ~%5d tokens", "qc_block_schema.json (unchanged)", len(GetQCBlockSchemaJSON()), len(GetQCBlockSchemaJSON())/4)
	const paramsCeiling, effectsCeiling = 4096, 3072
	if len(params) > paramsCeiling {
		t.Errorf("10-model parameter listing is %d bytes, over the %d-byte ceiling", len(params), paramsCeiling)
	}
	if len(effects) > effectsCeiling {
		t.Errorf("effects list is %d bytes, over the %d-byte ceiling", len(effects), effectsCeiling)
	}
}

// catalogCheckFixture builds a one-guitar preset from blocks.
func catalogCheckFixture(blocks ...storage.EffectBlock) *storage.StructuredPreset {
	return &storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"Guitar": blocks}}
}

func param(name, value string) storage.BlockParameter {
	return storage.BlockParameter{Name: name, Type: "slider", Value: value}
}

func TestFlagCatalogParamMismatches(t *testing.T) {
	valid := map[string]bool{
		"Green 808": false, "Rodent Drive": false, "Analog Delay": false, "US TWN Vibrato": false,
		"Plate": false, "412 Brit 60B V30 95": false, "Some Capture": true,
		"Plugin Graphic-9": false, "CA John's 2C Ch1": false,
	}
	cases := []struct {
		name  string
		block storage.EffectBlock
		want  CatalogParamCounts
		note  string // a substring the Rationale must contain ("" = no note at all)
	}{
		{"wrong knob name", storage.EffectBlock{Type: "Drive", Model: "Green 808", Parameters: []storage.BlockParameter{param("Drive", "4"), param("Overdrive", "4"), param("Level", "6")}},
			CatalogParamCounts{UnknownNames: 1, CheckedBlocks: 1}, `Drive isn't a control on the device's Green 808 (its controls: OVERDRIVE, TONE, LEVEL)`},
		{"case and spelling", storage.EffectBlock{Type: "Delay", Model: "Analog Delay", Parameters: []storage.BlockParameter{param("Time", "250 ms"), param("delay time", "0.25 s"), param("Mix", "20 %")}},
			CatalogParamCounts{CheckedBlocks: 1}, ""},
		{"out of range", storage.EffectBlock{Type: "Drive", Model: "Rodent Drive", Parameters: []storage.BlockParameter{param("Filter", "55.0"), param("Volume", "5.0")}},
			CatalogParamCounts{OutOfRange: 1, CheckedBlocks: 1}, `Filter 55.0 is outside the device's 0..10 range`},
		{"out of range after unit conversion", storage.EffectBlock{Type: "Delay", Model: "Analog Delay", Parameters: []storage.BlockParameter{param("Delay Time", "2 s")}},
			CatalogParamCounts{OutOfRange: 1, CheckedBlocks: 1}, `outside the device's 200..1100ms range`},
		{"switch position", storage.EffectBlock{Type: "Delay", Model: "Analog Delay", Parameters: []storage.BlockParameter{param("Trails", "Maybe"), param("Ping Pong", "on")}},
			CatalogParamCounts{BadSwitchValues: 1, CheckedBlocks: 1}, `Trails "Maybe" isn't a device position (Off|On)`},
		{"dB on a 0..10 dial", storage.EffectBlock{Type: "Drive", Model: "Green 808", Parameters: []storage.BlockParameter{param("Tone", "3 dB")}},
			CatalogParamCounts{UnitMismatches: 1, CheckedBlocks: 1}, `Tone is written in dB, but the device control is a unitless 0..10 dial`},
		{"capture-style dB is the capture check's finding", storage.EffectBlock{Type: "Amplifier", Model: "US TWN Vibrato", Parameters: []storage.BlockParameter{param("Bass", "+3 dB")}},
			CatalogParamCounts{CheckedBlocks: 1}, ""},
		{"wrong dimension", storage.EffectBlock{Type: "Reverb", Model: "Plate", Parameters: []storage.BlockParameter{param("Decay", "2.5 s")}},
			CatalogParamCounts{UnitMismatches: 1, CheckedBlocks: 1}, `Decay is written in s, but the device control is in %`},
		{"cab: mics, aliases, OFF, pan and Blend", storage.EffectBlock{Type: "Cabinet", Model: "412 Brit 60B V30 95", Parameters: []storage.BlockParameter{
			param("Bypass", "Off"), param("Mic 1", "Dyn 57 On-Axis"), param("Mic 2", "Ribbon 121"),
			param("Mic 1 Position", "3"), param("Mic 2 Distance", "2.5"), param("Mic 1 Pan", "-40"), param("Mic 1 Level", "-3 dB"),
			param("Low Cut", "OFF"), param("High Cut", "6500 Hz"), param("Blend", "60")}},
			CatalogParamCounts{CabConvention: 1, CheckedBlocks: 1}, `no Blend control on a cab`},
		{"cab filter out of range", storage.EffectBlock{Type: "Cabinet", Model: "412 Brit 60B V30 95", Parameters: []storage.BlockParameter{param("High Cut", "25 kHz")}},
			CatalogParamCounts{OutOfRange: 1, CheckedBlocks: 1}, `High Cut 25 kHz is outside the device's 500..20000Hz range`},
		{"confirmed_range on an unknown range", storage.EffectBlock{Type: "Cabinet", Model: "412 Brit 60B V30 95", Parameters: []storage.BlockParameter{
			{Name: "Mic 1 Level", Value: "-3 dB", Basis: "confirmed_range"}, {Name: "HPF", Value: "80 Hz", Basis: "confirmed_range"}}},
			CatalogParamCounts{BasisOverclaims: 1, CheckedBlocks: 1}, `Mic 1 Level is tagged confirmed_range, but the device's range for it isn't known`},
		// Faders (Graphic-9 bands -12..12 dB, an amp's EQ faders 0..10) are checked on their range like any
		// other dB / 0..10 control, so an export that later converts them as a guess never sees one the
		// Architect wrote outside it.
		{"fader in range", storage.EffectBlock{Type: "EQ", Model: "Plugin Graphic-9", Parameters: []storage.BlockParameter{param("65Hz", "-12 dB"), param("125Hz", "12 dB"), param("250Hz", "0")}},
			CatalogParamCounts{CheckedBlocks: 1}, ""},
		{"dB fader out of range", storage.EffectBlock{Type: "EQ", Model: "Plugin Graphic-9", Parameters: []storage.BlockParameter{param("65Hz", "+15 dB")}},
			CatalogParamCounts{OutOfRange: 1, CheckedBlocks: 1}, `65Hz +15 dB is outside the device's -12..12dB range`},
		{"dB fader in the wrong unit", storage.EffectBlock{Type: "EQ", Model: "Plugin Graphic-9", Parameters: []storage.BlockParameter{param("500Hz", "3 Hz")}},
			CatalogParamCounts{UnitMismatches: 1, CheckedBlocks: 1}, `500Hz is written in Hz, but the device control is in dB`},
		{"0..10 fader out of range", storage.EffectBlock{Type: "Amplifier", Model: "CA John's 2C Ch1", Parameters: []storage.BlockParameter{param("80 Hz", "11")}},
			CatalogParamCounts{OutOfRange: 1, CheckedBlocks: 1}, `80 Hz 11 is outside the device's 0..10 range`},
		{"0..10 fader in dB", storage.EffectBlock{Type: "Amplifier", Model: "CA John's 2C Ch1", Parameters: []storage.BlockParameter{param("240 Hz", "3 dB")}},
			CatalogParamCounts{UnitMismatches: 1, CheckedBlocks: 1}, `240 Hz is written in dB, but the device control is a unitless 0..10 dial`},
		{"capture is skipped", storage.EffectBlock{Type: "Amplifier", Model: "Some Capture (My Capture)", Parameters: []storage.BlockParameter{param("Drive", "99")}},
			CatalogParamCounts{UncheckedBlocks: 1}, ""},
		{"unverified is skipped", storage.EffectBlock{Type: "Amplifier", Model: FlagUnverifiedBlock("Made Up Amp"), Parameters: []storage.BlockParameter{param("Drive", "99")}},
			CatalogParamCounts{UncheckedBlocks: 1}, ""},
		{"the input block's own gate isn't a grid block but is real", storage.EffectBlock{Type: "Noise Gate", Model: "Input Gate", Parameters: []storage.BlockParameter{param("Threshold", "-60 dB")}},
			CatalogParamCounts{UncheckedBlocks: 1}, ""},
		{"an invented effect name is counted, not silently skipped", storage.EffectBlock{Type: "Reverb", Model: "Plate Reverb", Parameters: []storage.BlockParameter{param("Nonsense", "1")}},
			CatalogParamCounts{UnrecognizedBlocks: 1, UncheckedBlocks: 1}, `"Plate Reverb" isn't a block on the device or in your libraries`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sp := catalogCheckFixture(tc.block)
			before := append([]storage.BlockParameter(nil), tc.block.Parameters...)
			got := FlagCatalogParamMismatches(sp, valid)
			if got != tc.want {
				t.Errorf("counts = %+v, want %+v", got, tc.want)
			}
			b := sp.Guitars["Guitar"][0]
			if tc.note == "" && strings.Contains(b.Rationale, deviceCheckPrefix) {
				t.Errorf("unexpected note: %s", b.Rationale)
			}
			if tc.note != "" && !strings.Contains(b.Rationale, tc.note) {
				t.Errorf("rationale %q does not contain %q", b.Rationale, tc.note)
			}
			for i := range before {
				if b.Parameters[i] != before[i] {
					t.Errorf("parameter %d changed: %+v -> %+v", i, before[i], b.Parameters[i])
				}
			}
			// Idempotent: a second pass (e.g. a refine echoing the block back) adds nothing.
			rationale := b.Rationale
			FlagCatalogParamMismatches(sp, valid)
			if sp.Guitars["Guitar"][0].Rationale != rationale {
				t.Errorf("second pass changed the rationale:\n%s\n%s", rationale, sp.Guitars["Guitar"][0].Rationale)
			}
		})
	}
}

// The dB-on-amp-knob case belongs to FlagCaptureFormattingMismatches; between the two
// checks it must be reported exactly once.
func TestCatalogCheckAndCaptureCheckReportDBOnceBetweenThem(t *testing.T) {
	valid := map[string]bool{"US TWN Vibrato": false}
	sp := catalogCheckFixture(storage.EffectBlock{Type: "Amplifier", Model: "US TWN Vibrato", Parameters: []storage.BlockParameter{param("Bass", "+3 dB")}})
	capture := FlagCaptureFormattingMismatches(sp, valid)
	catalogCounts := FlagCatalogParamMismatches(sp, valid)
	if capture != 1 || catalogCounts.Defects() != 0 {
		t.Errorf("capture check = %d, catalog defects = %d; want 1 and 0", capture, catalogCounts.Defects())
	}
}

func TestFlagCatalogParamMismatchesNilValidBlocksUsesCatalogOnly(t *testing.T) {
	sp := catalogCheckFixture(storage.EffectBlock{Type: "Drive", Model: "Green 808", Parameters: []storage.BlockParameter{param("Drive", "4")}})
	if got := FlagCatalogParamMismatches(sp, nil); got.UnknownNames != 1 {
		t.Errorf("nil validBlocks: counts = %+v", got)
	}
	if got := FlagCatalogParamMismatches(nil, nil); got != (CatalogParamCounts{}) {
		t.Errorf("nil preset: %+v", got)
	}
}

// A synthetic preset in the style v4 produced on real runs (the classes measured in this
// PR: invented knob names, Blend on cabs, a percent-style value on a 0..10 knob), and the
// same preset written with the device's names. Exact counts, so a change to the aliases or
// rules that shifts what gets flagged shows up here.
func TestCatalogCheckOnV4StyleVsV5StylePreset(t *testing.T) {
	valid := map[string]bool{"Adaptive Gate": false, "Green 808": false, "Rodent Drive": false, "US DLX 65 Reissue": false,
		"412 Brit 60B V30 95": false, "Circular Delay": false, "Spring": false}
	v4 := catalogCheckFixture(
		storage.EffectBlock{Type: "Noise Gate", Model: "Adaptive Gate", Parameters: []storage.BlockParameter{param("Threshold", "-65.0 dB"), param("Release", "250 ms")}},
		storage.EffectBlock{Type: "Drive", Model: "Green 808", Parameters: []storage.BlockParameter{param("Bypass", "Off"), param("Drive", "2.0"), param("Tone", "6.0"), param("Level", "7.0")}},
		storage.EffectBlock{Type: "Drive", Model: "Rodent Drive", Parameters: []storage.BlockParameter{param("Drive", "3.0"), param("Filter", "55.0"), param("Level", "60.0")}},
		storage.EffectBlock{Type: "Amplifier", Model: "US DLX 65 Reissue", Parameters: []storage.BlockParameter{param("Gain", "4.5"), param("Volume", "4.5"), param("Treble", "7.0"), param("Output", "0.0 dB")}},
		storage.EffectBlock{Type: "Cabinet", Model: "412 Brit 60B V30 95", Parameters: []storage.BlockParameter{param("Mic 1", "Dyn 57 On-Axis 1.0 in"), param("Mic 2", "Ribbon 121 Off-Axis 2.0 in"), param("Blend", "60"), param("Low Cut", "80 Hz"), param("High Cut", "6500 Hz")}},
		storage.EffectBlock{Type: "Delay", Model: "Circular Delay", Parameters: []storage.BlockParameter{param("Time", "140 ms"), param("Feedback", "10 %"), param("Mix", "18 %")}},
		storage.EffectBlock{Type: "Reverb", Model: "Spring", Parameters: []storage.BlockParameter{param("Bypass", "Off"), param("Decay", "2.5 s"), param("Mix", "15 %")}},
	)
	got := FlagCatalogParamMismatches(v4, valid)
	// Unknown: Threshold, Release, Drive (808), Drive + Level (Rodent), Gain (US DLX), Decay (Spring).
	// Out of range: Rodent FILTER 55. Cab: Blend.
	want := CatalogParamCounts{UnknownNames: 7, OutOfRange: 1, CabConvention: 1, CheckedBlocks: 7}
	if got != want {
		t.Errorf("v4-style counts = %+v, want %+v", got, want)
	}

	v5 := catalogCheckFixture(
		storage.EffectBlock{Type: "Noise Gate", Model: "Adaptive Gate", Parameters: []storage.BlockParameter{param("NOISE REDUCTION", "30 %")}, KeptDefaults: []string{}},
		storage.EffectBlock{Type: "Drive", Model: "Green 808", Parameters: []storage.BlockParameter{param("Bypass", "Off"), param("OVERDRIVE", "2.0"), param("TONE", "6.0"), param("LEVEL", "7.0")}, KeptDefaults: []string{}},
		storage.EffectBlock{Type: "Drive", Model: "Rodent Drive", Parameters: []storage.BlockParameter{param("DISTORTION", "3.0"), param("FILTER", "5.5"), param("VOLUME", "6.0")}, KeptDefaults: []string{}},
		storage.EffectBlock{Type: "Amplifier", Model: "US DLX 65 Reissue", Parameters: []storage.BlockParameter{param("VOLUME", "4.5"), param("TREBLE", "7.0"), param("MODE", "Vibrato"), param("OUTPUT", "0.0 dB")},
			KeptDefaults: []string{"INPUT", "BASS"}},
		storage.EffectBlock{Type: "Cabinet", Model: "412 Brit 60B V30 95", Parameters: []storage.BlockParameter{param("Mic 1", "Dyn 57"), param("Mic 2", "Ribbon 121"),
			param("MIC 1 POSITION", "2"), param("MIC 1 DISTANCE", "1"), param("MIC 1 LEVEL", "0 dB"), param("MIC 2 POSITION", "5"), param("MIC 2 DISTANCE", "3"), param("MIC 2 LEVEL", "-4 dB"),
			param("HPF", "80 Hz"), param("LPF", "6500 Hz")}, KeptDefaults: []string{"MIC 1 GRID MODE", "MIC 2 GRID MODE", "OUTPUT VOLUME"}},
		storage.EffectBlock{Type: "Delay", Model: "Circular Delay", Parameters: []storage.BlockParameter{param("DELAY TIME", "140 ms"), param("FEEDBACK", "10 %"), param("MIX", "18 %")},
			KeptDefaults: []string{"TAP PRESET", "SYNC", "SYNC NOTE", "DIFFUSION", "HIGH PASS", "LOW PASS", "MOD RATE", "MOD DEPTH", "VINTAGE MODE", "TRAILS"}},
		storage.EffectBlock{Type: "Reverb", Model: "Spring", Parameters: []storage.BlockParameter{param("Bypass", "Off"), param("DAMPING", "40 %"), param("MIX", "15 %")},
			KeptDefaults: []string{"TONE", "BOING", "TRAILS"}},
	)
	if got := FlagCatalogParamMismatches(v5, valid); got.Defects() != 0 || got.CheckedBlocks != 7 {
		t.Errorf("v5-style counts = %+v, want no defects on 7 checked blocks", got)
	}
}

func TestKeptDefaultsCoverage(t *testing.T) {
	valid := map[string]bool{"Green 808": false, "412 Brit 60B V30 95": false, "Spring": false}
	g808 := func(params []storage.BlockParameter, kept []string) storage.EffectBlock {
		return storage.EffectBlock{Type: "Drive", Model: "Green 808", Parameters: params, KeptDefaults: kept}
	}
	for _, tc := range []struct {
		name   string
		blocks []storage.EffectBlock
		want   CatalogParamCounts
		note   string
	}{
		{"every control set or kept", []storage.EffectBlock{g808([]storage.BlockParameter{param("OVERDRIVE", "6")}, []string{"TONE", "level"})},
			CatalogParamCounts{CheckedBlocks: 1}, ""},
		{"a control neither set nor kept", []storage.EffectBlock{g808([]storage.BlockParameter{param("OVERDRIVE", "6")}, []string{"TONE"})},
			CatalogParamCounts{UnconsideredParams: 1, CheckedBlocks: 1}, "not set and not marked as kept at the device default: LEVEL."},
		{"kept_defaults names a control the model lacks", []storage.EffectBlock{g808([]storage.BlockParameter{param("OVERDRIVE", "6")}, []string{"TONE", "LEVEL", "GAIN"})},
			CatalogParamCounts{UnknownNames: 1, CheckedBlocks: 1}, "kept_defaults names GAIN, which the device's Green 808 doesn't have."},
		{"no block uses the convention (v4 output): coverage isn't checked", []storage.EffectBlock{g808([]storage.BlockParameter{param("OVERDRIVE", "6")}, nil)},
			CatalogParamCounts{CheckedBlocks: 1}, ""},
		{"the convention is preset-wide: a block that omits the list is checked too", []storage.EffectBlock{
			g808([]storage.BlockParameter{param("OVERDRIVE", "6")}, []string{"TONE", "LEVEL"}),
			{Type: "Reverb", Model: "Spring", Parameters: []storage.BlockParameter{param("MIX", "20 %")}}},
			CatalogParamCounts{UnconsideredParams: 4, CheckedBlocks: 2}, ""},
		{"a family name doesn't require controls only one member has", []storage.EffectBlock{{Type: "Cabinet", Model: "412 Brit 60B V30 95",
			Parameters:   []storage.BlockParameter{param("Mic 1", "Dyn 57"), param("Mic 2", "Ribbon 121"), param("MIC 1 LEVEL", "0 dB"), param("MIC 2 LEVEL", "-3 dB")},
			KeptDefaults: []string{"MIC 1 DISTANCE", "MIC 1 POSITION", "MIC 1 GRID MODE", "MIC 2 DISTANCE", "MIC 2 POSITION", "MIC 2 GRID MODE", "HPF", "LPF", "OUTPUT VOLUME"}}},
			CatalogParamCounts{CheckedBlocks: 1}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sp := catalogCheckFixture(tc.blocks...)
			if got := FlagCatalogParamMismatches(sp, valid); got != tc.want {
				t.Errorf("counts = %+v, want %+v", got, tc.want)
			}
			if tc.note != "" && !strings.Contains(sp.Guitars["Guitar"][0].Rationale, tc.note) {
				t.Errorf("rationale %q lacks %q", sp.Guitars["Guitar"][0].Rationale, tc.note)
			}
		})
	}
	// The family's pan controls differ by member (PAN on mono, BALANCE on stereo).
	cab := mustDeviceBlock(t, "412 Brit 60B V30 95")
	if cab.byKey["MIC1PAN"].Required || cab.byKey["MIC1BALANCE"].Required || !cab.byKey["MIC1LEVEL"].Required {
		t.Error("only controls every family member has should be required")
	}
}

// The delays and reverbs the Architect adds or swaps in get their controls listed: every model
// in those categories, minus the models already listed for the chosen blocks, with plugin
// models only when owned.
func TestCatalogEffectParamContext(t *testing.T) {
	valid := BuildEffectiveValidBlocks(true, true)
	none := NewPluginGate(false, nil)
	got := CatalogEffectParamContext([]string{"Analog Delay", "Green 808", "Chief DC2W"}, valid, none)
	for _, want := range []string{"Spring [Reverb]: MIX", "Slapback Delay [Delay]:", "Adaptive Gate [Utility]: NOISE REDUCTION", "[Compressor]:"} {
		if !strings.Contains(got, want) {
			t.Errorf("effect parameters missing %q", want)
		}
	}
	if strings.Contains(got, "\nAnalog Delay [") || strings.HasPrefix(got, "Analog Delay [") {
		t.Error("a chosen block's controls are already listed and shouldn't repeat")
	}
	for _, unwanted := range []string{"[Modulation]:", "[Equalizer]:", "[Wah]:", "[Pitch]:", "John Mayer"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("effect parameters should not contain %q (only delays, reverbs, compressors and utility; plugins need ownership)", unwanted)
		}
	}
	owned := CatalogEffectParamContext(nil, valid, NewPluginGate(true, []string{"John Mayer"}))
	if !strings.Contains(owned, "John Mayer Spring Reverb [Reverb]:") {
		t.Error("an owned plugin's reverb should be listed")
	}
	all := CatalogEffectParamContext(nil, valid, PluginGate{}) // inactive gate: every plugin allowed
	t.Logf("delays, reverbs, compressors, utility: no plugins %d bytes (~%d tokens), shipped config's plugins %d bytes, every plugin %d bytes",
		len(got), len(got)/4, len(owned), len(all))
	// 24000 before compressors and utility joined delays and reverbs (26486 with them).
	if len(all) > 30000 {
		t.Errorf("effect parameter section is %d bytes with every plugin, over the 30000-byte ceiling", len(all))
	}
}

// tokenIndex is the first of tokenIndices, or -1 (a test helper).
func tokenIndex(haystack, needle string) int {
	if hits := tokenIndices(haystack, needle); len(hits) > 0 {
		return hits[0]
	}
	return -1
}

func fp(x float64) *float64 { return &x }
