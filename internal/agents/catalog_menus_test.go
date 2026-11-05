package agents

import (
	"encoding/json"
	"strings"
	"testing"
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
	if pv["coros_equivalent"] != "PV-505 Lead" || pv["type"] != "amp" || !strings.HasPrefix(pv["based_on"].(string), "Based on ") {
		t.Errorf(`"PV-505 Lead" entry = %v, want coros_equivalent itself, type amp, and the device's based_on`, pv)
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
		"Brit Bass 50 Bright": "BASS AMPS",   // catalog-only bass amp
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
}
