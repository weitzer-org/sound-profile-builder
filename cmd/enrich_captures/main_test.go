package main

import (
	"reflect"
	"sort"
	"testing"
)

func TestGroupMissingByEquivalent(t *testing.T) {
	mappings := map[string]capturedGear{
		// Two keys share "US DLX 58"; one already has an archetype, so both should be
		// treated as fully known -- no search needed for either.
		"Fender Deluxe Reverb 64/65": {CorosEquivalent: "US DLX 58", IsCapture: true, TonalArchetype: "Other / Unique"},
		"Fender Deluxe 5E2 1958":     {CorosEquivalent: "US DLX 58", IsCapture: true, TonalArchetype: ""},

		// Two keys share "Iba Green"; neither has an archetype, so both should come back
		// grouped under one missing entry.
		"Ibanez Tube Screamer TS9": {CorosEquivalent: "Iba Green", IsCapture: true, TonalArchetype: ""},
		"Ibanez Tube Screamer 9":   {CorosEquivalent: "Iba Green", IsCapture: true, TonalArchetype: ""},

		// Not a capture at all -- must be excluded regardless of missing archetype.
		"Some Algorithmic Amp": {CorosEquivalent: "Plexi 100 Patch", IsCapture: false, TonalArchetype: ""},

		// A capture with no coros_equivalent recorded -- must be excluded, nothing to search for.
		"Malformed Entry": {CorosEquivalent: "", IsCapture: true, TonalArchetype: ""},
	}

	missing := groupMissingByEquivalent(mappings)

	if _, ok := missing["US DLX 58"]; ok {
		t.Errorf("Expected US DLX 58 to be excluded (already known via a sibling entry), got: %v", missing["US DLX 58"])
	}

	gotIbaGreen := missing["Iba Green"]
	sort.Strings(gotIbaGreen)
	wantIbaGreen := []string{"Ibanez Tube Screamer 9", "Ibanez Tube Screamer TS9"}
	if !reflect.DeepEqual(gotIbaGreen, wantIbaGreen) {
		t.Errorf("Expected Iba Green to group both sibling keys, got %v, want %v", gotIbaGreen, wantIbaGreen)
	}

	if _, ok := missing["Plexi 100 Patch"]; ok {
		t.Errorf("Expected non-capture entry to be excluded")
	}
	if _, ok := missing[""]; ok {
		t.Errorf("Expected entry with no coros_equivalent to be excluded")
	}

	if len(missing) != 1 {
		t.Errorf("Expected exactly 1 unique real-gear name needing research, got %d: %v", len(missing), missing)
	}
}

func TestCountMissingEntries(t *testing.T) {
	mappings := map[string]capturedGear{
		"a": {IsCapture: true, TonalArchetype: ""},
		"b": {IsCapture: true, TonalArchetype: "British Crunch"},
		"c": {IsCapture: false, TonalArchetype: ""}, // not a capture -- must not count
		"d": {IsCapture: true, TonalArchetype: ""},
	}

	if got := countMissingEntries(mappings); got != 2 {
		t.Errorf("Expected 2 missing capture entries, got %d", got)
	}
}
