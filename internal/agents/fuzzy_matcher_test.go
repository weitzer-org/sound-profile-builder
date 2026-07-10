package agents

import (
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

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
		"Parametric-3":   true,
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
	const factoryCaptureName = "Brit 2203 87"             // coros_map.json, is_capture: true

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
