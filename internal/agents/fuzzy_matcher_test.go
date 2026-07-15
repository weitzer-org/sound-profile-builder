package agents

import (
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

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
		"US Twin Vibrato": true,
		"UK C30 TopBoost": true,
		"Parametric-3":    true,
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"US Twin Vibrato", "US Twin Vibrato"},
		{"us twin vibrato", "US Twin Vibrato"}, // case insensitive match
		{"US Twin V", "US Twin V"},             // distance to "US Prince" is 4, which is > threshold of 3, so it should not snap (no match)
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
		"US Twin Vibrato": true,
		"UK C30 TopBoost": false,
	}

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `<td>Amplifier: US Twin Vibrato</td>`,
			expected: `<td>Amplifier: US Twin Vibrato (Factory Capture)</td>`, // valid + capture, not in the user's library
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
			input:    `<td>Amplifier: US Twin Vibrato (Capture)</td>`,
			expected: `<td>Amplifier: US Twin Vibrato (Factory Capture)</td>`,
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
		"US Twin Vibrato":            true,
		"FNDR TWDLX IN Edge BAL CAB": true, // real user_captures.json entry
		"Boss Blues Driver Waza":     true, // real user_captures.json entry, block_type "drive"
	}

	sp := &storage.StructuredPreset{
		Guitars: map[string][]storage.EffectBlock{
			"Guitar 1": {
				{ID: "1", Type: "Amplifier", Model: "US Twin Vibrato"},
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
	if blocks[0].Model != "US Twin Vibrato (Factory Capture)" {
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
