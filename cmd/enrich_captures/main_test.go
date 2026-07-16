package main

import (
	"reflect"
	"sort"
	"testing"
)

func TestMissingKeys(t *testing.T) {
	mappings := map[string]capturedGear{
		// Already has an archetype -- must be excluded.
		"Fender Deluxe Reverb 64/65": {CorosEquivalent: "US DLX 58", IsCapture: true, TonalArchetype: "Other / Unique"},

		// Two different keys share the same on-device block name ("Iba Green") -- both must
		// still be researched individually, not deduped, since a shared on-device block
		// name does not mean the same real gear (see missingKeys' doc comment).
		"Ibanez Tube Screamer TS9": {CorosEquivalent: "Iba Green", IsCapture: true, TonalArchetype: ""},
		"Ibanez Tube Screamer 9":   {CorosEquivalent: "Iba Green", IsCapture: true, TonalArchetype: ""},

		// Not a capture at all -- must be excluded regardless of missing archetype.
		"Some Algorithmic Amp": {CorosEquivalent: "Plexi 100 Patch", IsCapture: false, TonalArchetype: ""},

		// A capture with no coros_equivalent recorded -- still researchable by key alone.
		"Malformed Entry": {CorosEquivalent: "", IsCapture: true, TonalArchetype: ""},
	}

	got := missingKeys(mappings)
	sort.Strings(got)
	want := []string{"Ibanez Tube Screamer 9", "Ibanez Tube Screamer TS9", "Malformed Entry"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("missingKeys() = %v, want %v", got, want)
	}
}
