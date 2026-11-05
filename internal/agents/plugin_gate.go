package agents

import (
	"log"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/catalog"
)

// PluginGate says which paid Neural DSP plugin devices (Archetype plugins running on the
// Quad Cortex) a run may use. It is applied everywhere a plugin device could reach the
// pipeline: the Librarian's Dictionary and amp menu, and the valid-block set used to flag
// unverified names in the Architect's output.
type PluginGate struct {
	// Active is false when the run carried no allow_paid_plugins constraint at all (older
	// callers and tests); nothing is filtered then.
	Active bool
	// Owned holds the plugins the user owns, by catalog.PluginNames spelling.
	Owned map[string]bool
}

// NewPluginGate builds an active gate. With allowPaidPlugins false no plugin is owned,
// whatever available lists. Names in available are matched ignoring case; a name the
// plugin table doesn't know is kept as written (it can still match a coros_map.json
// required_plugin spelled the same way) and logged, since it is probably a typo.
func NewPluginGate(allowPaidPlugins bool, available []string) PluginGate {
	g := PluginGate{Active: true, Owned: map[string]bool{}}
	if !allowPaidPlugins {
		return g
	}
	for _, p := range available {
		if canon, ok := catalog.CanonicalPluginName(p); ok {
			g.Owned[canon] = true
			continue
		}
		p = strings.TrimSpace(p)
		if p != "" {
			log.Printf("available_plugins: %q is not a known plugin (known: %s)", p, strings.Join(catalog.PluginNames(), ", "))
			g.Owned[p] = true
		}
	}
	return g
}

// PluginGateFromConstraints reads allow_paid_plugins/available_plugins from a pipeline
// constraints map, as built by presetsvc. No allow_paid_plugins key means an inactive gate.
func PluginGateFromConstraints(constraints map[string]interface{}) PluginGate {
	allow, ok := constraints["allow_paid_plugins"].(bool)
	if !ok {
		return PluginGate{}
	}
	available, _ := constraints["available_plugins"].([]string)
	return NewPluginGate(allow, available)
}

// Allows reports whether a device needing plugin may be used. An empty plugin means the
// device needs none.
func (g PluginGate) Allows(plugin string) bool {
	if plugin == "" || !g.Active {
		return true
	}
	return g.Owned[plugin]
}

// ownsAny reports whether the user owns at least one of plugins. An empty name (a
// plugin_id the plugin table doesn't know) is never owned.
func (g PluginGate) ownsAny(plugins []string) bool {
	for _, p := range plugins {
		if p != "" && g.Owned[p] {
			return true
		}
	}
	return false
}
