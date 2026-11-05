package agents

import (
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestGetCategorizedAmplifiers_PluginFiltering(t *testing.T) {
	// "John Mayer Smooth Operator" (coros_map.json, type "amp", required_plugin "John
	// Mayer") only belongs on the menu when the caller owns that plugin. "Brit 2203" is a
	// plain non-plugin amp and must never be affected by plugin filtering.
	withoutOwnership := GetCategorizedAmplifiers(true, true, map[string]bool{})
	if strings.Contains(withoutOwnership, "John Mayer Smooth Operator") {
		t.Errorf("expected John Mayer's amp hidden when the plugin isn't in allowedPlugins")
	}
	if !strings.Contains(withoutOwnership, "Brit 2203") {
		t.Errorf("expected a plain non-plugin amp to survive plugin filtering")
	}

	withOwnership := GetCategorizedAmplifiers(true, true, map[string]bool{"John Mayer": true})
	if !strings.Contains(withOwnership, "John Mayer Smooth Operator") {
		t.Errorf("expected John Mayer's amp present once the plugin is in allowedPlugins")
	}

	unfiltered := GetCategorizedAmplifiers(true, false, nil)
	if !strings.Contains(unfiltered, "John Mayer Smooth Operator") {
		t.Errorf("expected plugin entries present when filterPlugins is false")
	}
}

func TestContainsAsToken(t *testing.T) {
	tests := []struct {
		haystack string
		needle   string
		expected bool
	}{
		{"The Librarian selected CA 400 for this run.", "CA 400", true},
		{"The Librarian selected CA 4000X for this run.", "CA 400", false}, // substring of a longer token, not a real match
		{"CA 400", "CA 400", true},                                         // exact match, no surrounding text
		{"Options: CA 400, CA 401", "CA 400", true},                        // punctuation boundary
		{"XCA 400", "CA 400", false},                                       // needle glued to a preceding word char
		{"CA 400X", "CA 400", false},                                       // needle glued to a following word char
		{"nothing relevant here", "CA 400", false},
	}
	for _, tc := range tests {
		got := containsAsToken(tc.haystack, tc.needle)
		if got != tc.expected {
			t.Errorf("containsAsToken(%q, %q) = %v; want %v", tc.haystack, tc.needle, got, tc.expected)
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s        string
		target   string
		expected int
	}{
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"abc", "bc", 1},
		{"abc", "adc", 1},
		{"abc", "cab", 2},
		{"", "abc", 3},
		{"abc", "", 3},
		{"Hello", "hello", 1}, // Case sensitive in rune comparison
	}

	for _, tc := range tests {
		sRunes := []rune(tc.s)
		tRunes := []rune(tc.target)
		dist := LevenshteinDistance(sRunes, tRunes)
		if dist != tc.expected {
			t.Errorf("LevenshteinDistance(%q, %q) = %d; want %d", tc.s, tc.target, dist, tc.expected)
		}
	}
}

func TestSnapToClosestBlock(t *testing.T) {
	// Initialize validBlocksCache
	_ = GetValidNativeBlocks()

	validBlocks := map[string]bool{
		"US TWN Vibrato":  true,
		"UK C30 TopBoost": true,
		"Parametric-3":    true,
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"US TWN Vibrato", "US TWN Vibrato"},
		{"us twn vibrato", "US TWN Vibrato"}, // case insensitive match
		{"US Twin V", "US Twin V"},           // distance to "US Prince" is 4, which is > threshold of 3, so it should not snap (no match)
		{"UK C30 TopBoost", "UK C30 TopBoost"},
		{"Bypassed", "Bypassed"},
		{"-3.5dB", "-3.5dB"},
		{"Active", "Active"},
	}

	for _, tc := range tests {
		got := SnapToClosestBlock(tc.input, validBlocks)
		if got != tc.expected {
			t.Errorf("SnapToClosestBlock(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestApplyFuzzyCorrection(t *testing.T) {
	validBlocks := map[string]bool{
		"US TWN Vibrato":  true,
		"UK C30 TopBoost": false,
	}

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `<td>Amplifier: US TWN Vibrato</td>`,
			expected: `<td>Amplifier: US TWN Vibrato (Factory Capture)</td>`, // valid + capture, not in the user's library
		},
		{
			input:    `<td>Cabinet: UK C30 TopBoost</td>`,
			expected: `<td>Cabinet: UK C30 TopBoost</td>`, // valid, not capture
		},
		{
			input:    `<td>Cabinet: UK C30 TopBst</td>`,
			expected: `<td>Cabinet: UK C30 TopBoost</td>`, // fuzzy match
		},
		{
			input:    `<td>Level: -3.5dB</td>`,
			expected: `<td>Level: -3.5dB</td>`, // ignored category
		},
		{
			input:    `<td>Amplifier: 57 Tweed Champ Bright</td>`,
			expected: `<td>Amplifier: 57 Tweed Champ Bright ⚠️ (Unverified — not in Dictionary or your Capture Library)</td>`, // fabricated name, too far from anything real to snap
		},
		{
			// Native algorithmic block categories (reverb, gate, EQ, ...) are never
			// catalogued in coros_map.json/user_captures.json by design, so a legitimate
			// native name in one of these categories must never be flagged as unverified.
			input:    `<td>Reverb: Spring Reverb</td>`,
			expected: `<td>Reverb: Spring Reverb</td>`,
		},
		{
			// The model sometimes writes "(Capture)" into the name itself, redundant with
			// the suffix this file adds programmatically. A real, valid name pre-annotated
			// this way must still match its bare dictionary entry, not get flagged.
			input:    `<td>Amplifier: US TWN Vibrato (Capture)</td>`,
			expected: `<td>Amplifier: US TWN Vibrato (Factory Capture)</td>`,
		},
		{
			input:    `<td>Utility: Input & Noise Gate</td>`,
			expected: `<td>Utility: Input & Noise Gate</td>`,
		},
		{
			// A name present in the caller's valid-blocks map but not in the real
			// user_captures.json library must be recognized as a real capture (never
			// flagged), and labeled as a factory capture since it isn't the user's own.
			input:    `<td>Amplifier: My Personal Capture</td>`,
			expected: `<td>Amplifier: My Personal Capture (Factory Capture)</td>`,
		},
		{
			// A real entry from user_captures.json must be labeled "(My Capture)", not
			// the generic "(Capture)" — this is what lets the UI tell factory and
			// personal-library captures apart.
			input:    `<td>Cabinet: FNDR TWDLX IN Edge BAL CAB</td>`,
			expected: `<td>Cabinet: FNDR TWDLX IN Edge BAL CAB (My Capture)</td>`,
		},
		{
			// A real capture name that legitimately ends in the bare word "Capture" (no
			// delimiter) must not be truncated by stripCaptureAnnotation.
			input:    `<td>Utility: JM Default Capture</td>`,
			expected: `<td>Utility: JM Default Capture</td>`,
		},
		{
			// Regression: SnapToClosestBlock always corrects against the global dictionary
			// regardless of what's in the caller's validBlocks, so a typo of a real but
			// disallowed/filtered-out factory capture ("Brit 2203 87" absent from this
			// test's validBlocks, simulating allow_factory_captures=false) snaps to its
			// exact real name. A plain `validBlocks[snapped]` read can't distinguish that
			// from a known, legitimate non-capture block (both zero-value to false), so it
			// must use the comma-ok idiom and flag it instead of passing the real name
			// through silently unflagged and unlabeled.
			input:    `<td>Amplifier: Brit 2203 8</td>`,
			expected: `<td>Amplifier: Brit 2203 87 ⚠️ (Unverified — not in Dictionary or your Capture Library)</td>`,
		},
		{
			// Regression: a chat-refinement turn re-injects the prior structured/HTML
			// payload into the next prompt and asks the LLM to preserve unchanged blocks,
			// so an already-flagged fabricated name commonly gets echoed back verbatim.
			// stripCaptureAnnotation doesn't recognize this suffix (it's not a capture
			// annotation), so without idempotency in FlagUnverifiedBlock the warning text
			// would compound on every subsequent turn instead of staying stable.
			input:    `<td>Amplifier: 57 Tweed Champ Bright ⚠️ (Unverified — not in Dictionary or your Capture Library)</td>`,
			expected: `<td>Amplifier: 57 Tweed Champ Bright ⚠️ (Unverified — not in Dictionary or your Capture Library)</td>`,
		},
		{
			// Regression: "drive:" was missing from validCategories (the HTML draft-view
			// path's category gate), separate from gearBlockTypes (the structured-payload
			// path's gate, fixed earlier). A fabricated drive-pedal name in the HTML draft
			// view must be flagged too, not silently skipped before reaching resolveBlockName.
			input:    `<td>Drive: Fake Boutique Fuzz XYZ</td>`,
			expected: `<td>Drive: Fake Boutique Fuzz XYZ ⚠️ (Unverified — not in Dictionary or your Capture Library)</td>`,
		},
	}

	// Simulate a user-owned capture being part of the recognized set for this generation.
	validBlocks["My Personal Capture"] = true
	validBlocks["FNDR TWDLX IN Edge BAL CAB"] = true

	for _, tc := range tests {
		got := ApplyFuzzyCorrection(tc.input, validBlocks)
		if got != tc.expected {
			t.Errorf("ApplyFuzzyCorrection(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestGetUserCaptures(t *testing.T) {
	caps := GetUserCaptures()
	validTypes := map[string]bool{"amp": true, "cab": true, "full_rig": true, "drive": true, "other": true}
	for _, c := range caps {
		if c.Name == "" {
			t.Errorf("UserCapture with empty Name: %+v", c)
		}
		if !validTypes[c.BlockType] {
			t.Errorf("UserCapture %q has unexpected block_type %q", c.Name, c.BlockType)
		}
	}
}

func TestBuildEffectiveValidBlocks(t *testing.T) {
	// A real user capture and a real coros_map.json factory capture, so this exercises
	// the actual merged/embedded data rather than a synthetic fixture.
	const userCaptureName = "FNDR TWDLX IN Edge BAL CAB" // user_captures.json, source: amalgamaudio
	const factoryCaptureName = "Brit 2203 87"            // coros_map.json, is_capture: true

	all := GetValidNativeBlocks()
	if !all[userCaptureName] {
		t.Fatalf("test fixture assumption failed: %q not found as a capture in the merged dictionary", userCaptureName)
	}
	if isCap, ok := all[factoryCaptureName]; !ok || !isCap {
		t.Fatalf("test fixture assumption failed: %q not found as a factory capture in coros_map.json", factoryCaptureName)
	}

	// Regression: disallowing factory captures must not also strip user captures, even
	// though both share the isCapture=true flag in GetValidNativeBlocks(). A naive
	// "keep only !isCap" filter drops both together, causing a correctly-chosen user
	// capture to be flagged "Unverified" whenever the user unchecks factory captures.
	blocks := BuildEffectiveValidBlocks(false, true)
	if _, ok := blocks[userCaptureName]; !ok {
		t.Errorf("expected user capture %q to remain valid when factory captures are disallowed but user captures are allowed", userCaptureName)
	}
	if _, ok := blocks[factoryCaptureName]; ok {
		t.Errorf("expected factory capture %q to be excluded when factory captures are disallowed", factoryCaptureName)
	}

	// And the inverse: disallowing user captures must not strip factory captures.
	blocks = BuildEffectiveValidBlocks(true, false)
	if _, ok := blocks[userCaptureName]; ok {
		t.Errorf("expected user capture %q to be excluded when user captures are disallowed", userCaptureName)
	}
	if _, ok := blocks[factoryCaptureName]; !ok {
		t.Errorf("expected factory capture %q to remain valid when user captures are disallowed but factory captures are allowed", factoryCaptureName)
	}
}

func TestFlagUnverifiedStructuredBlocks(t *testing.T) {
	validBlocks := map[string]bool{
		"US TWN Vibrato":             true,
		"FNDR TWDLX IN Edge BAL CAB": true, // real user_captures.json entry
		"Boss Blues Driver Waza":     true, // real user_captures.json entry, block_type "drive"
	}

	sp := &storage.StructuredPreset{
		Guitars: map[string][]storage.EffectBlock{
			"Guitar 1": {
				{ID: "1", Type: "Amplifier", Model: "US TWN Vibrato"},
				{ID: "2", Type: "Amplifier", Model: "57 Tweed Champ Bright"},
				{ID: "3", Type: "Utility", Model: "Bypassed"},
				{ID: "4", Type: "Reverb", Model: "Spring Reverb"},
				{ID: "5", Type: "Cabinet", Model: "FNDR TWDLX IN Edge BAL CAB"},
				{ID: "6", Type: "drive", Model: "Boss Blues Driver Waza"},
				{ID: "7", Type: "drive", Model: "Fake Boutique Fuzz XYZ"},
			},
		},
	}

	FlagUnverifiedStructuredBlocks(sp, validBlocks)

	blocks := sp.Guitars["Guitar 1"]
	if blocks[0].Model != "US TWN Vibrato (Factory Capture)" {
		t.Errorf("expected valid factory capture to be labeled, got %q", blocks[0].Model)
	}
	if blocks[1].Model != FlagUnverifiedBlock("57 Tweed Champ Bright") {
		t.Errorf("expected fabricated block to be flagged, got %q", blocks[1].Model)
	}
	if blocks[2].Model != "Bypassed" {
		t.Errorf("expected status value to be left alone, got %q", blocks[2].Model)
	}
	if blocks[4].Model != "FNDR TWDLX IN Edge BAL CAB (My Capture)" {
		t.Errorf("expected real user capture to be labeled as My Capture, got %q", blocks[4].Model)
	}
	if blocks[3].Model != "Spring Reverb" {
		t.Errorf("expected native reverb block (not a gear category) to be left unflagged, got %q", blocks[3].Model)
	}
	if blocks[5].Model != "Boss Blues Driver Waza (My Capture)" {
		t.Errorf("expected real drive-type user capture to be labeled My Capture, got %q", blocks[5].Model)
	}
	if blocks[6].Model != FlagUnverifiedBlock("Fake Boutique Fuzz XYZ") {
		t.Errorf("expected fabricated drive-type block to be flagged (regression: \"drive\" type was missing from gearBlockTypes), got %q", blocks[6].Model)
	}
}

func TestFlagCaptureFormattingMismatches(t *testing.T) {
	validBlocks := map[string]bool{
		"Brit Plexi 100 Patch": false, // real, verified, algorithmic -- NOT a capture
		"64 Fender Vibroverb":  true,  // real, verified capture -- dB formatting is correct here
		"Strymon Sunset BOOST": false, // real, verified, algorithmic Boost-type block
	}

	sp := &storage.StructuredPreset{
		Guitars: map[string][]storage.EffectBlock{
			"Guitar 1": {
				{ID: "1", Type: "Amplifier", Model: "Brit Plexi 100 Patch", Rationale: "Classic British crunch.", Parameters: []storage.BlockParameter{
					{Name: "Gain", Value: "+2.0 dB"},                  // mismatch: algorithmic block, dB formatting
					{Name: "Bass", Value: "4.2"},                      // fine: plain 0-10 value
					{Name: "Volume", Value: "7.0", ValueB: "-1.5 dB"}, // mismatch only on ValueB
				}},
				{ID: "2", Type: "Amplifier", Model: "64 Fender Vibroverb", Parameters: []storage.BlockParameter{
					{Name: "Gain", Value: "+2.0 dB"}, // correct: this block really is a capture
				}},
				{ID: "3", Type: "Reverb", Model: "Spring Reverb", Parameters: []storage.BlockParameter{
					{Name: "Decay", Value: "+2.0 dB"}, // not a gear-block category -- must be left alone
				}},
				{ID: "4", Type: "Amplifier", Model: "Some Unverified Amp", Parameters: []storage.BlockParameter{
					{Name: "Gain", Value: "+2.0 dB"}, // unknown name -- can't determine capture status, must be left alone
				}},
				// A prior FlagUnverifiedStructuredBlocks pass (or any caller) may have already
				// annotated a genuine capture's Model with its source suffix -- must still
				// resolve back to the bare dictionary key, not be treated as an unknown name.
				{ID: "5", Type: "Amplifier", Model: "64 Fender Vibroverb (My Capture)", Parameters: []storage.BlockParameter{
					{Name: "Gain", Value: "+2.0 dB"}, // correct: genuinely a capture, even though annotated
				}},
				// Boost isn't in gearBlockTypes (no coros_map.json dictionary coverage for that
				// category), but Rule 9 explicitly calls it out as capture-eligible.
				{ID: "6", Type: "Boost", Model: "Strymon Sunset BOOST", Parameters: []storage.BlockParameter{
					{Name: "Volume", Value: "+3.0 dB"}, // mismatch: algorithmic Boost block, dB formatting
				}},
			},
		},
	}

	FlagCaptureFormattingMismatches(sp, validBlocks)

	blocks := sp.Guitars["Guitar 1"]
	if !strings.Contains(blocks[0].Rationale, "⚠️") {
		t.Errorf("expected a block with a dB-formatted capture-only param on a confirmed-algorithmic block to get a Rationale warning, got %q", blocks[0].Rationale)
	}
	if !strings.Contains(blocks[0].Rationale, "Classic British crunch.") {
		t.Errorf("expected the warning to be appended to the existing rationale, not replace it, got %q", blocks[0].Rationale)
	}
	if blocks[0].Parameters[0].Value != "+2.0 dB" || blocks[0].Parameters[2].ValueB != "-1.5 dB" {
		t.Errorf("expected parameter Value/ValueB to be left completely untouched -- the warning must never mutate the editable source of truth, got %q / %q", blocks[0].Parameters[0].Value, blocks[0].Parameters[2].ValueB)
	}
	if strings.Contains(blocks[1].Rationale, "⚠️") {
		t.Errorf("expected a confirmed real capture to get no warning, got %q", blocks[1].Rationale)
	}
	if strings.Contains(blocks[2].Rationale, "⚠️") {
		t.Errorf("expected non-gear-category block (Reverb) to get no warning even with a dB-shaped value, got %q", blocks[2].Rationale)
	}
	if strings.Contains(blocks[3].Rationale, "⚠️") {
		t.Errorf("expected block with unverified/unknown name to get no warning (capture status unknown), got %q", blocks[3].Rationale)
	}
	if strings.Contains(blocks[4].Rationale, "⚠️") {
		t.Errorf("expected an already-annotated genuine capture name to still resolve correctly and get no warning, got %q", blocks[4].Rationale)
	}
	if !strings.Contains(blocks[5].Rationale, "⚠️") {
		t.Errorf("expected a Boost-type block (not in gearBlockTypes, but capture-eligible per Rule 9) to get a warning, got %q", blocks[5].Rationale)
	}
}

func TestFlagIncompleteCabinetBlocks(t *testing.T) {
	sp := &storage.StructuredPreset{
		Guitars: map[string][]storage.EffectBlock{
			"Guitar 1": {
				{ID: "1", Type: "Cabinet", Model: "212 UK C30 '65", Parameters: []storage.BlockParameter{
					{Name: "Mic 1", Value: "Cap Edge"},
					{Name: "Mic 2", Value: "Cone"},
					{Name: "Blend", Value: "60%"},
					{Name: "High Cut", Value: "6.5 kHz"},
				}}, // complete -- must not be flagged
				{ID: "2", Type: "Cabinet", Model: "412 Brit 60B", Parameters: []storage.BlockParameter{
					{Name: "High Cut", Value: "6.5 kHz"},
					{Name: "Low Cut", Value: "80 Hz"},
				}}, // missing all three mic-placement params -- the real regression this check exists for
				{ID: "3", Type: "Cabinet", Model: "Some Cab", Parameters: []storage.BlockParameter{
					{Name: "Mic 1", Value: "Cap Edge"},
					{Name: "Mic 2", Value: "Cone"},
				}}, // missing Blend only
				{ID: "4", Type: "Amplifier", Model: "Brit Plexi 100 Patch", Parameters: []storage.BlockParameter{
					{Name: "Gain", Value: "5.0"},
				}}, // not a Cabinet block -- must never be flagged by this check
			},
		},
	}

	FlagIncompleteCabinetBlocks(sp)

	blocks := sp.Guitars["Guitar 1"]
	if strings.Contains(blocks[0].Rationale, "⚠️") {
		t.Errorf("expected complete Cabinet block to get no warning, got %q", blocks[0].Rationale)
	}
	if !strings.Contains(blocks[1].Rationale, "⚠️") {
		t.Errorf("expected Cabinet block missing all mic-placement params to be flagged, got %q", blocks[1].Rationale)
	}
	if !strings.Contains(blocks[2].Rationale, "⚠️") {
		t.Errorf("expected Cabinet block missing just Blend to still be flagged, got %q", blocks[2].Rationale)
	}
	if strings.Contains(blocks[3].Rationale, "⚠️") {
		t.Errorf("expected non-Cabinet block to never be checked, got %q", blocks[3].Rationale)
	}
}

func TestFlagLeftoverValueRanges(t *testing.T) {
	sp := &storage.StructuredPreset{
		Guitars: map[string][]storage.EffectBlock{
			"Guitar 1": {
				{ID: "1", Type: "Delay", Model: "Digital Delay", Parameters: []storage.BlockParameter{
					{Name: "Time", Type: "slider", Value: "10-15ms"},             // a genuine leftover range
					{Name: "Feedback", Type: "slider", Value: "80 - 120 Hz"},     // range with spaces and a unit
					{Name: "LowCut", Type: "slider", Value: "80Hz-120Hz"},        // unit repeated on BOTH numbers
					{Name: "Mix", Type: "slider", Value: "45%"},                  // fine: single decisive value
					{Name: "Gain", Type: "slider", Value: "-3.5", ValueB: "5-8"}, // ValueB is a range, Value is fine
					{Name: "Level", Type: "slider", Value: "-2.0 dB"},            // single negative dB value -- must NOT false-positive as a range
					{Name: "Mode", Type: "dropdown", Value: "10-15"},             // range-shaped, but not a slider -- must not be flagged
				}},
			},
		},
	}

	FlagLeftoverValueRanges(sp)

	block := sp.Guitars["Guitar 1"][0]
	if !strings.Contains(block.Rationale, "Time") {
		t.Errorf("expected the Time range to be flagged, got %q", block.Rationale)
	}
	if !strings.Contains(block.Rationale, "Feedback") {
		t.Errorf("expected the Feedback range (with spaces/unit) to be flagged, got %q", block.Rationale)
	}
	if !strings.Contains(block.Rationale, "LowCut") {
		t.Errorf("expected a range with the unit repeated on both numbers to be flagged, got %q", block.Rationale)
	}
	if strings.Contains(block.Rationale, "Mix") {
		t.Errorf("expected the single-value Mix param to not be flagged, got %q", block.Rationale)
	}
	if !strings.Contains(block.Rationale, "Gain (Scene B)") {
		t.Errorf("expected only Gain's ValueB (not its Value) to be flagged as a range, got %q", block.Rationale)
	}
	if strings.Contains(block.Rationale, "Level") {
		t.Errorf("expected a single negative dB value to never false-positive as a range, got %q", block.Rationale)
	}
	if strings.Contains(block.Rationale, "Mode") {
		t.Errorf("expected a non-slider parameter to never be checked even if range-shaped, got %q", block.Rationale)
	}
}

// Plugin-gated device names are valid on an exact (case-insensitive) match but are never
// fuzzy-snap targets: "Spring Reverb" must not become the paid "Plini Reverb".
func TestSnapToClosestBlock_PluginNamesExactOnly(t *testing.T) {
	_ = GetValidNativeBlocks()
	for _, tc := range []struct{ in, want string }{
		{"plini reverb", "Plini Reverb"}, // exact, any case
		{"Plini Reverb", "Plini Reverb"},
		{"Spring Reverb", "Spring Reverb"}, // 3 edits from "Plini Reverb": not snapped
		{"Plini Reverbb", "Plini Reverbb"}, // near miss of a plugin name: not snapped
	} {
		if got := SnapToClosestBlock(tc.in, nil); got != tc.want {
			t.Errorf("SnapToClosestBlock(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// Ungated names stay normal fuzzy candidates. ("Exotic Z Boost" is the only valid
	// name within 4 edits of the probe, so map iteration order can't pick another.)
	if got := SnapToClosestBlock("Exotic Z Boos", nil); got != "Exotic Z Boost" {
		t.Errorf("SnapToClosestBlock(%q) = %q, want ungated %q", "Exotic Z Boos", got, "Exotic Z Boost")
	}
}

// Ties between equally close candidates must not depend on map iteration order.
// "Klon Centaur" is one edit from both "Klon Centaur 1" and "Klon Centaur 2"; the sorted
// pool means the alphabetically first one wins on every call.
func TestSnapToClosestBlock_TiesAreDeterministic(t *testing.T) {
	valid := GetValidNativeBlocks()
	for _, n := range []string{"Klon Centaur 1", "Klon Centaur 2"} {
		if _, ok := valid[n]; !ok {
			t.Skipf("fixture name %q no longer in the valid set", n)
		}
	}
	for i := 0; i < 200; i++ {
		if got := SnapToClosestBlock("Klon Centaur", valid); got != "Klon Centaur 1" {
			t.Fatalf("call %d: SnapToClosestBlock(%q) = %q, want %q", i, "Klon Centaur", got, "Klon Centaur 1")
		}
	}
}

func TestPluginGate(t *testing.T) {
	inactive := PluginGateFromConstraints(map[string]interface{}{})
	if inactive.Active || !inactive.Allows("Plini") {
		t.Errorf("no allow_paid_plugins constraint: want an inactive gate that allows everything, got %+v", inactive)
	}

	g := PluginGateFromConstraints(map[string]interface{}{
		"allow_paid_plugins": true,
		"available_plugins":  []string{"cory wong", "John Mayer"},
	})
	for plugin, want := range map[string]bool{"": true, "Cory Wong": true, "John Mayer": true, "Plini": false, "Tim Henson": false} {
		if got := g.Allows(plugin); got != want {
			t.Errorf("Allows(%q) = %v, want %v (owned: %v)", plugin, got, want, g.Owned)
		}
	}

	none := PluginGateFromConstraints(map[string]interface{}{
		"allow_paid_plugins": false,
		"available_plugins":  []string{"Cory Wong"},
	})
	if !none.Active || none.Allows("Cory Wong") {
		t.Errorf("allow_paid_plugins=false must own nothing, got %+v", none)
	}

	// A catalog addition whose plugin_id is missing from the plugin table (gated, no
	// plugin name) is refused even by an inactive gate.
	unknown := catalogAddition{Family: "Future Device", Type: "amp", Gated: true}
	if unknown.allowedBy(inactive) || unknown.allowedBy(g) {
		t.Errorf("a model with an unknown plugin_id must never be offered")
	}
	if !(catalogAddition{Family: "Brit 2203", Type: "amp"}).allowedBy(g) {
		t.Errorf("a non-plugin model must always be offered")
	}
}

// Device-catalog names join the valid-block set as native (non-capture) blocks: exact
// model names, mono/stereo family names, and models coros_map.json never mentions.
func TestGetValidNativeBlocks_Catalog(t *testing.T) {
	valid := GetValidNativeBlocks()
	for _, name := range []string{
		"PV-505 Lead",             // catalog amp no coros_map.json entry names
		"412 Brit 60B V30 95 (M)", // exact cab model
		"412 Brit 60B V30 95",     // its family name
		"Room",                    // non-gear catalog model (exact-match only)
		"Tim Henson Lead",         // plugin model no coros_map.json entry names
		"Love Meat",               // native Filter model, formerly flagged as a capture
	} {
		isCap, ok := valid[name]
		if !ok {
			t.Errorf("%q: want a valid block", name)
		} else if isCap {
			t.Errorf("%q: want a native block, got a capture", name)
		}
	}
	// The catalog's generic Neural Capture slots are not gear and must not be offered.
	for _, name := range []string{"Capture 2", "Eltron 30"} {
		if _, ok := valid[name]; ok {
			t.Errorf("%q (catalog Neural Capture category) must not be a valid block", name)
		}
	}
	// User captures keep their capture flag.
	for _, c := range GetUserCaptures() {
		if isCap, ok := valid[c.Name]; !ok || !isCap {
			t.Errorf("user capture %q: valid=%v isCapture=%v, want a valid capture", c.Name, ok, isCap)
		}
		break
	}
}

// Catalog names outside the gear categories, and every plugin model, are exact-match only.
// Gear-category catalog names are normal fuzzy candidates. Each probe below has a single
// nearest candidate, so the result can't depend on tie-breaking.
func TestSnapToClosestBlock_CatalogNames(t *testing.T) {
	valid := GetValidNativeBlocks()
	for _, tc := range []struct{ in, want string }{
		{"room", "Room"},                       // exact, any case
		{"Double RVB", "Double RVB"},           // not snapped to the "Doubler" utility
		{"Tim Henson Leed", "Tim Henson Leed"}, // near miss of a plugin model: not snapped
		{"tim henson lead", "Tim Henson Lead"}, // plugin model, exact match
	} {
		if got := SnapToClosestBlock(tc.in, valid); got != tc.want {
			t.Errorf("SnapToClosestBlock(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// A gear-category catalog model that no coros_map.json entry names still snaps.
	for _, tc := range []struct{ in, want string }{
		{"Bogna Uber Clen", "Bogna Uber Clean"},
		{"Freeman 100 Clea", "Freeman 100 Clean"},
	} {
		requireUniqueNearest(t, tc.in, tc.want)
		if got := SnapToClosestBlock(tc.in, valid); got != tc.want {
			t.Errorf("SnapToClosestBlock(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// requireUniqueNearest fails the test if want isn't the single closest snap candidate to
// probe, so a snapping test can't pass or fail on tie-breaking alone.
func requireUniqueNearest(t *testing.T, probe, want string) {
	t.Helper()
	GetValidNativeBlocks()
	in := []rune(strings.ToLower(probe))
	best, count, bestName := -1, 0, ""
	for _, c := range snapPool {
		d := LevenshteinDistance(in, c.runes)
		switch {
		case best < 0 || d < best:
			best, count, bestName = d, 1, c.name
		case d == best:
			count++
		}
	}
	if bestName != want || count != 1 {
		t.Fatalf("probe %q: nearest candidate is %q (%d at distance %d), want %q alone", probe, bestName, count, best, want)
	}
}

// With an active plugin gate, plugin devices the user doesn't own leave the valid set,
// whether coros_map.json names them (John Mayer Smooth Operator) or only the catalog does
// (Tim Henson Lead). An inactive gate, as the eval tools use, keeps them all.
func TestBuildEffectiveValidBlocksFor_Plugins(t *testing.T) {
	const mayer, henson, native = "John Mayer Smooth Operator", "Tim Henson Lead", "Brit 2203"
	for _, tc := range []struct {
		name   string
		gate   PluginGate
		mayer  bool
		henson bool
	}{
		{"inactive gate", PluginGate{}, true, true},
		{"owns John Mayer", NewPluginGate(true, []string{"John Mayer"}), true, false},
		{"owns Tim Henson", NewPluginGate(true, []string{"tim henson"}), false, true},
		{"paid plugins off", NewPluginGate(false, []string{"John Mayer", "Tim Henson"}), false, false},
	} {
		valid := BuildEffectiveValidBlocksFor(ValidBlockFilter{AllowFactoryCaptures: true, AllowUserCaptures: true, Plugins: tc.gate})
		if _, ok := valid[mayer]; ok != tc.mayer {
			t.Errorf("%s: %q valid=%v, want %v", tc.name, mayer, ok, tc.mayer)
		}
		if _, ok := valid[henson]; ok != tc.henson {
			t.Errorf("%s: %q valid=%v, want %v", tc.name, henson, ok, tc.henson)
		}
		if _, ok := valid[native]; !ok {
			t.Errorf("%s: native %q must always be valid", tc.name, native)
		}
	}
}

// End to end through the flagger: an amp block naming an unowned plugin device is flagged
// unverified; the same block passes once the plugin is owned.
func TestFlagUnverifiedStructuredBlocks_UnownedPluginAmp(t *testing.T) {
	for _, tc := range []struct {
		owned       []string
		wantFlagged bool
	}{
		{nil, true},
		{[]string{"Tim Henson"}, false},
	} {
		valid := BuildEffectiveValidBlocksFor(ValidBlockFilter{
			AllowFactoryCaptures: true, AllowUserCaptures: true,
			Plugins: NewPluginGate(true, tc.owned),
		})
		sp := &storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{
			"Guitar 1": {{ID: "1", Type: "Amplifier", Model: "Tim Henson Lead"}},
		}}
		n := FlagUnverifiedStructuredBlocks(sp, valid)
		if got := n == 1; got != tc.wantFlagged {
			t.Errorf("owned %v: flagged=%v (model now %q), want %v", tc.owned, got, sp.Guitars["Guitar 1"][0].Model, tc.wantFlagged)
		}
	}
}

func TestExcludeUnownedPluginBlocks(t *testing.T) {
	all := BuildEffectiveValidBlocks(true, true)
	for _, name := range []string{"Plini Drive", "John Mayer Halfman OD", "Brit 2203"} {
		if _, ok := all[name]; !ok {
			t.Fatalf("fixture assumption: %q should be a valid block name", name)
		}
	}
	cases := []struct {
		name        string
		constraints map[string]interface{}
		want        map[string]bool // block name -> kept?
	}{
		{"no plugin constraint: unfiltered", map[string]interface{}{},
			map[string]bool{"Plini Drive": true, "John Mayer Halfman OD": true, "Brit 2203": true}},
		{"paid plugins off", map[string]interface{}{"allow_paid_plugins": false, "available_plugins": []string{"John Mayer"}},
			map[string]bool{"Plini Drive": false, "John Mayer Halfman OD": false, "Brit 2203": true}},
		{"owns John Mayer only", map[string]interface{}{"allow_paid_plugins": true, "available_plugins": []string{"John Mayer"}},
			map[string]bool{"Plini Drive": false, "John Mayer Halfman OD": true, "Brit 2203": true}},
	}
	for _, tc := range cases {
		got := ExcludeUnownedPluginBlocks(all, tc.constraints)
		for name, kept := range tc.want {
			if _, ok := got[name]; ok != kept {
				t.Errorf("%s: %q kept=%v, want %v", tc.name, name, ok, kept)
			}
		}
	}

	owned := ExcludeUnownedPluginBlocks(all, map[string]interface{}{"allow_paid_plugins": true, "available_plugins": []string{"John Mayer"}})
	sp := &storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"Guitar 1": {
		{ID: "1", Type: "Overdrive", Model: "Plini Drive"},
		{ID: "2", Type: "Overdrive", Model: "John Mayer Halfman OD"},
	}}}
	if n := FlagUnverifiedStructuredBlocks(sp, owned); n != 1 {
		t.Errorf("flagged %d blocks, want 1 (the unowned Plini Drive)", n)
	}
	blocks := sp.Guitars["Guitar 1"]
	if blocks[0].Model != FlagUnverifiedBlock("Plini Drive") || blocks[1].Model != "John Mayer Halfman OD" {
		t.Errorf("got %q / %q", blocks[0].Model, blocks[1].Model)
	}
}

// An unowned plugin device is flagged in any block category, not only amp/cab/drive:
// "Tim Henson Delay Modern" in a delay block passes once the plugin is owned.
func TestFlagUnverifiedStructuredBlocks_UnownedPluginEffect(t *testing.T) {
	for _, tc := range []struct {
		owned       []string
		wantFlagged bool
	}{
		{nil, true},
		{[]string{"Tim Henson"}, false},
	} {
		valid := BuildEffectiveValidBlocksFor(ValidBlockFilter{
			AllowFactoryCaptures: true, AllowUserCaptures: true,
			Plugins: NewPluginGate(true, tc.owned),
		})
		sp := &storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{
			"Guitar 1": {
				{ID: "1", Type: "Delay", Model: "Tim Henson Delay Modern"},
				{ID: "2", Type: "Delay", Model: "Not A Real Delay XYZ"}, // non-gear unknown: still unflagged
			},
		}}
		n := FlagUnverifiedStructuredBlocks(sp, valid)
		blocks := sp.Guitars["Guitar 1"]
		if got := strings.HasSuffix(blocks[0].Model, unverifiedSuffix); got != tc.wantFlagged {
			t.Errorf("owned %v: plugin delay flagged=%v (model now %q), want %v", tc.owned, got, blocks[0].Model, tc.wantFlagged)
		}
		if strings.HasSuffix(blocks[1].Model, unverifiedSuffix) {
			t.Errorf("owned %v: unknown non-gear name %q was flagged; only plugin devices widen the check", tc.owned, blocks[1].Model)
		}
		if want := map[bool]int{true: 1, false: 0}[tc.wantFlagged]; n != want {
			t.Errorf("owned %v: flagged %d blocks, want %d", tc.owned, n, want)
		}
	}
}
