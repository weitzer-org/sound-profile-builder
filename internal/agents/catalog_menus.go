package agents

import (
	"encoding/json"
	"log"
	"sort"
	"sync"

	"github.com/weitzer-org/sound-builder/internal/catalog"
)

// catalogAddition is a device-catalog gear model (one mono/stereo family) that no
// coros_map.json entry names yet, offered to the Librarian alongside the hand-built
// Dictionary so it can pick any amp, cab or drive the unit actually has.
//
// These are derived at runtime rather than written into coros_map.json: the catalog's
// based_on text isn't unique per model (one real amp covers several channel models), a
// dozen based_on strings are already coros_map.json keys pointing at another block, and
// a bulk-added copy would go stale after every CorOS update. As coros_map.json gains a
// real entry for a model, that model drops out of this list on its own.
//
// They carry no tonal_archetype: CLAUDE.md forbids guessed labels, and cmd/enrich_captures
// only researches coros_map.json entries. The amp menu puts these amps in the
// unclassifiedBucket instead.
type catalogAddition struct {
	Family  string // the family name, which is also the block name offered
	Type    string // coros_map.json-style type: "amp", "cab" or "drive"
	Bass    bool   // a bass amp, bass cab or bass drive
	BasedOn string // the device's own real-gear reference, e.g. "Based on Marshall® JCM800®"
	Plugin  string // required plugin's name; "" when none, or when Gated with an unknown plugin_id
	Gated   bool   // a plugin device
}

// catalogDictionaryType maps the catalog's gear categories to coros_map.json types. Its
// keys are also the categories whose models are fuzzy-snap targets (addCatalogBlocks).
var catalogDictionaryType = map[string]string{
	"Guitar Amplifier":   "amp",
	"Bass Amplifier":     "amp",
	"Cabsim Guitar (M)":  "cab",
	"Cabsim Guitar (ST)": "cab",
	"Cabsim Bass (M)":    "cab",
	"Cabsim Bass (ST)":   "cab",
	"Guitar Overdrive":   "drive",
	"Bass Overdrive":     "drive",
}

var catalogBassCategories = map[string]bool{
	"Bass Amplifier": true, "Cabsim Bass (M)": true, "Cabsim Bass (ST)": true, "Bass Overdrive": true,
}

// unclassifiedBucket is the amp-menu bucket for amps with no researched tonal_archetype:
// catalog-only guitar amps, and coros_map.json amps whose tonal_archetype is still empty.
// It is deliberately not "Other / Unique", which is a real researched label.
const unclassifiedBucket = "Unclassified (no tonal label yet)"

var (
	catalogAdditionsOnce sync.Once
	catalogAdditionsList []catalogAddition
)

// catalogGearAdditions returns the embedded catalog's gear additions (see
// buildCatalogAdditions), computed once per process.
func catalogGearAdditions() []catalogAddition {
	catalogAdditionsOnce.Do(func() {
		cat, err := catalog.Embedded()
		if err != nil {
			log.Printf("device catalog unavailable, the Librarian sees coros_map.json only: %v", err)
			return
		}
		catalogAdditionsList = buildCatalogAdditions(cat, CorosMapRecords())
	})
	return catalogAdditionsList
}

// buildCatalogAdditions returns every gear-category family in cat that the coros_map.json
// records don't cover, sorted by family name. A family counts as covered when a record
// names the family or any of its models, whatever that record's capture or plugin flags:
// an entry filtered out for this run must not come back through the catalog.
func buildCatalogAdditions(cat *catalog.Catalog, corosRecords map[string]map[string]any) []catalogAddition {
	covered := make(map[string]bool)
	for _, rec := range corosRecords {
		if equiv, _ := rec["coros_equivalent"].(string); equiv != "" {
			covered[equiv] = true
			covered[catalog.FamilyName(equiv)] = true
		}
	}
	byFamily := make(map[string]*catalogAddition)
	var order []string
	for i := range cat.Models {
		m := &cat.Models[i]
		typ, ok := catalogDictionaryType[m.Category]
		if !ok {
			continue
		}
		fam := catalog.FamilyName(m.Name)
		if covered[fam] || covered[m.Name] {
			continue
		}
		a, seen := byFamily[fam]
		if !seen {
			plugin, gated := m.RequiredPlugin()
			a = &catalogAddition{
				Family: fam, Type: typ, Bass: catalogBassCategories[m.Category],
				Plugin: plugin, Gated: gated,
			}
			byFamily[fam] = a
			order = append(order, fam)
		}
		if a.BasedOn == "" {
			a.BasedOn = m.BasedOn
		}
	}
	sort.Strings(order)
	out := make([]catalogAddition, 0, len(order))
	for _, fam := range order {
		out = append(out, *byFamily[fam])
	}
	return out
}

// allowedBy reports whether the run's plugin gate lets the Librarian see this addition.
// A plugin device whose plugin_id the plugin table doesn't know is never offered.
func (a catalogAddition) allowedBy(g PluginGate) bool {
	if !a.Gated {
		return true
	}
	return a.Plugin != "" && g.Allows(a.Plugin)
}

// withCatalogDictionaryEntries adds the catalog gear models coros_map.json doesn't cover
// to the Librarian's Dictionary JSON, keyed by family name, filtered by the run's plugin
// gate. Each entry has coros_equivalent, type, based_on (when the device gives one),
// source "device_catalog" and, for plugin devices, required_plugin. The result is
// re-marshalled with sorted keys, so it is stable from run to run. On any parse error the
// Dictionary is returned unchanged.
func withCatalogDictionaryEntries(dictJSON string, plugins PluginGate) string {
	additions := catalogGearAdditions()
	if len(additions) == 0 {
		return dictJSON
	}
	var dict map[string]json.RawMessage
	if err := json.Unmarshal([]byte(dictJSON), &dict); err != nil {
		log.Printf("Librarian Dictionary not extended with device-catalog models: %v", err)
		return dictJSON
	}
	added := 0
	for _, a := range additions {
		if !a.allowedBy(plugins) {
			continue
		}
		key := a.Family
		if _, clash := dict[key]; clash {
			key += " (device catalog)"
		}
		entry := map[string]string{
			"coros_equivalent": a.Family,
			"type":             a.Type,
			"source":           "device_catalog",
		}
		if a.BasedOn != "" {
			entry["based_on"] = a.BasedOn
		}
		if a.Plugin != "" {
			entry["required_plugin"] = a.Plugin
		}
		b, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		dict[key] = b
		added++
	}
	if added == 0 {
		return dictJSON
	}
	out, err := json.Marshal(dict)
	if err != nil {
		return dictJSON
	}
	return string(out)
}
