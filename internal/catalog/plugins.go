package catalog

import (
	"sort"
	"strings"
)

// pluginNames maps a model's PluginID to the plugin's short name, spelled the way
// config.json's available_plugins and coros_map.json's required_plugin spell it.
//
// Where the names come from:
//   - 1, 3, 4, 5, 9, 10, 11, 21: confirmed against coros_map.json. Every required_plugin
//     entry pointing at a model with this PluginID uses this name, and no ID has two.
//     TestPluginTableAgreesWithCorosMap (internal/agents) keeps that true.
//   - 12, 13, 15, 20: no coros_map.json entry uses them yet, so the names are inferred
//     from the models' own names ("Tim Henson Lead", "Petrucci Clean", "Rabea Crunch",
//     "Misha Clean"/"Horizon Devices® Precision Drive™"). Neural DSP sells these as
//     Archetype: Tim Henson X, Petrucci X, Rabea X and Misha Mansoor X, and CorOS 4.1
//     added all four to the Quad Cortex (per Neural DSP's announcements, as reported by
//     the music press); the ID-to-plugin pairing itself has not been checked on a device.
//
// A PluginID missing from this table means a plugin this code doesn't know about, and
// its models are never offered (see Model.RequiredPlugin). TestPluginNamesCoverCatalog
// fails when a catalog import brings in a new ID.
var pluginNames = map[string]string{
	"1":  "Nameless",
	"3":  "Plini",
	"4":  "Parallax",
	"5":  "Nolly",
	"9":  "Cory Wong",
	"10": "Gojira",
	"11": "SLO-100",
	"12": "Tim Henson",
	"13": "Petrucci",
	"15": "Rabea",
	"20": "Misha Mansoor",
	"21": "John Mayer",
}

// PluginName returns the short plugin name for a PluginID.
func PluginName(id string) (string, bool) {
	n, ok := pluginNames[id]
	return n, ok
}

// PluginNames returns every known plugin's short name, sorted.
func PluginNames() []string {
	out := make([]string, 0, len(pluginNames))
	for _, n := range pluginNames {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// CanonicalPluginName returns the table's spelling of a plugin name, ignoring case and
// surrounding space, so "cory wong" in config.json still matches "Cory Wong".
func CanonicalPluginName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	for _, n := range pluginNames {
		if strings.EqualFold(n, name) {
			return n, true
		}
	}
	return "", false
}

// RequiredPlugin reports whether using this model needs a paid plugin, and which one.
// gated is true for every plugin model. name is empty when the PluginID isn't in the
// table; callers must then treat the model as unusable, never as free.
func (m *Model) RequiredPlugin() (name string, gated bool) {
	if m.PluginID == "" {
		return "", false
	}
	return pluginNames[m.PluginID], true
}
