package agents

import (
	"strings"
	"testing"
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
		"UK C30 TopBoost": true,
	}

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `<td>Amplifier:</td><td>US Twin Vibrato</td>`,
			expected: `<td>Amplifier:</td><td>US Twin Vibrato</td>`, // valid
		},
		{
			input:    `<td>Cabinet:</td><td>US Twin V</td>`,
			expected: `<td>Cabinet:</td><td>US Twin V</td>`, // should allow (if it doesn't match and just returns match) or should correct if valid
		},
		{
			input:    `<td>Level:</td><td>-3.5dB</td>`,
			expected: `<td>Level:</td><td>-3.5dB</td>`, // should ignore
		},
	}

	for _, tc := range tests {
		got := ApplyFuzzyCorrection(tc.input, validBlocks)
		if !strings.Contains(got, tc.expected) {
			t.Errorf("ApplyFuzzyCorrection(%q) = %q; want it to contain %q", tc.input, got, tc.expected)
		}
	}
}
