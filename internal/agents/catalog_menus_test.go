package agents

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/catalog"
)

// menuBucketOf returns the bucket heading a name is listed under in an amp menu, or "".
func menuBucketOf(menu, name string) string {
	bucket := ""
	for _, line := range strings.Split(menu, "\n") {
		if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "- ") {
			bucket = strings.TrimSuffix(line, ":")
			continue
		}
		if line == "- "+name {
			return bucket
		}
	}
	return ""
}

func TestLibrarianDictionary_CatalogAdditions(t *testing.T) {
	covered := map[string]bool{}
	for _, rec := range CorosMapRecords() {
		if e, _ := rec["coros_equivalent"].(string); e != "" {
			covered[e] = true
		}
	}

	dictFor := func(owned ...string) map[string]map[string]any {
		t.Helper()
		dict, _ := librarianInputs(true, NewPluginGate(true, owned))
		var m map[string]map[string]any
		if err := json.Unmarshal([]byte(dict), &m); err != nil {
			t.Fatalf("Dictionary is not JSON: %v", err)
		}
		return m
	}

	m := dictFor("Cory Wong", "John Mayer")
	added := 0
	for key, rec := range m {
		if rec["source"] != "device_catalog" {
			continue
		}
		added++
		equiv, _ := rec["coros_equivalent"].(string)
		if covered[equiv] {
			t.Errorf("%q: catalog addition duplicates coros_map.json's %q", key, equiv)
		}
		if _, ok := rec["tonal_archetype"]; ok {
			t.Errorf("%q: catalog additions must not carry a tonal_archetype", key)
		}
		if p, _ := rec["required_plugin"].(string); p != "" && p != "Cory Wong" && p != "John Mayer" {
			t.Errorf("%q: plugin %q offered without being owned", key, p)
		}
	}
	if added == 0 {
		t.Fatal("no device-catalog entries were added to the Dictionary")
	}

	pv, ok := m["PV-505 Lead"]
	if !ok {
		t.Fatal(`"PV-505 Lead" (catalog amp with no coros_map.json entry) missing from the Dictionary`)
	}
	if got := pv["coros_equivalent"]; got != "PV-505 Lead" {
		t.Errorf(`"PV-505 Lead" coros_equivalent = %v, want itself`, got)
	}
	if got := pv["type"]; got != "amp" {
		t.Errorf(`"PV-505 Lead" type = %v, want "amp"`, got)
	}
	if basedOn, _ := pv["based_on"].(string); !strings.HasPrefix(basedOn, "Based on ") {
		t.Errorf(`"PV-505 Lead" based_on = %v, want the device's "Based on ..." text`, pv["based_on"])
	}

	if _, ok := m["Tim Henson Lead"]; ok {
		t.Error(`"Tim Henson Lead" offered although the Tim Henson plugin isn't owned`)
	}
	if rec, ok := dictFor("Tim Henson")["Tim Henson Lead"]; !ok || rec["required_plugin"] != "Tim Henson" {
		t.Errorf(`"Tim Henson Lead" with the plugin owned = %v, want an entry with required_plugin "Tim Henson"`, rec)
	}

	a, _ := librarianInputs(true, NewPluginGate(true, []string{"Cory Wong", "John Mayer"}))
	b, _ := librarianInputs(true, NewPluginGate(true, []string{"Cory Wong", "John Mayer"}))
	if a != b {
		t.Error("the Dictionary differs between two identical calls")
	}
}

func TestGetCategorizedAmplifiers_Catalog(t *testing.T) {
	const unclassified = "UNCLASSIFIED (NO TONAL LABEL YET)"
	menu := GetCategorizedAmplifiers(true, true, map[string]bool{"Cory Wong": true, "John Mayer": true})

	for name, want := range map[string]string{
		"PV-505 Lead":         unclassified, // catalog-only guitar amp
		"Brit Bass 50 Bright": "BASS AMPS",  // catalog-only bass amp
		"Brit 2203":           "BRITISH CRUNCH",
	} {
		if got := menuBucketOf(menu, name); got != want {
			t.Errorf("%q is under %q, want %q", name, got, want)
		}
	}
	if got := menuBucketOf(menu, "Tim Henson Lead"); got != "" {
		t.Errorf(`"Tim Henson Lead" listed under %q although the plugin isn't owned`, got)
	}
	owned := GetCategorizedAmplifiers(true, true, map[string]bool{"Tim Henson": true})
	if got := menuBucketOf(owned, "Tim Henson Lead"); got != unclassified {
		t.Errorf(`"Tim Henson Lead" with the plugin owned is under %q, want %q`, got, unclassified)
	}
	if again := GetCategorizedAmplifiers(true, true, map[string]bool{"Cory Wong": true, "John Mayer": true}); again != menu {
		t.Error("the amp menu differs between two identical calls")
	}
	// The repeat-call check above can't catch a lost sort (the buckets are cached once
	// per process), so check the order directly.
	prev := ""
	for _, line := range strings.Split(menu, "\n") {
		if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "- ") {
			prev = ""
			continue
		}
		if name, ok := strings.CutPrefix(line, "- "); ok {
			if name < prev {
				t.Errorf("amp menu not sorted within a bucket: %q listed after %q", name, prev)
			}
			prev = name
		}
	}
}

// buildCatalogAdditions against a small fixture catalog: coverage by exact or family
// name, one entry per mono/stereo family, non-gear categories ignored, plugin metadata
// (known and unknown plugin_id) carried through, sorted output.
func TestBuildCatalogAdditions_Fixture(t *testing.T) {
	cat := catalog.New([]catalog.Model{
		{Name: "Brit 2203", Category: "Guitar Amplifier", BasedOn: "Based on Marshall JCM800"},
		{Name: "Zed Amp", Category: "Guitar Amplifier", BasedOn: "Based on Something"},
		{Name: "412 Test Cab (M)", Category: "Cabsim Guitar (M)"},
		{Name: "412 Test Cab (ST)", Category: "Cabsim Guitar (ST)", BasedOn: "Based on A Cab"},
		{Name: "412 Covered (M)", Category: "Cabsim Guitar (M)"},
		{Name: "412 Sibling (M)", Category: "Cabsim Guitar (M)"},
		{Name: "412 Sibling (ST)", Category: "Cabsim Guitar (ST)"}, // covered via its (M) sibling's family
		{Name: "Bass Thing", Category: "Bass Amplifier"},
		{Name: "Plini Clean", Category: "Guitar Amplifier", PluginID: "3"},
		{Name: "Future Amp", Category: "Guitar Amplifier", PluginID: "999"},
		{Name: "Room", Category: "Reverb"},
	}, catalog.Source{}, "")
	records := map[string]map[string]any{
		"Marshall JCM800": {"coros_equivalent": "Brit 2203"},
		"Covered Cab":     {"coros_equivalent": "412 Covered", "is_capture": true},
		"Sibling Cab":     {"coros_equivalent": "412 Sibling (M)"},
	}
	got := buildCatalogAdditions(cat, records)
	want := []catalogAddition{
		{Family: "412 Test Cab", Type: "cab", BasedOn: "Based on A Cab"},
		{Family: "Bass Thing", Type: "amp", Bass: true},
		{Family: "Future Amp", Type: "amp", Gated: true},
		{Family: "Plini Clean", Type: "amp", Plugin: "Plini", Gated: true},
		{Family: "Zed Amp", Type: "amp", BasedOn: "Based on Something"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildCatalogAdditions =\n%+v\nwant\n%+v", got, want)
	}
}
