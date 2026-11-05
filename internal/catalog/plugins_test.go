package catalog

import "testing"

// Every plugin model in the catalog needs a known plugin name, or plugin gating has no
// name to check ownership against and the model is never offered. A CorOS update that
// adds a plugin fails here: add its ID to pluginNames, spelled as config.json's
// available_plugins would spell it.
func TestPluginNamesCoverCatalog(t *testing.T) {
	c, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	missing := map[string]string{}
	for i := range c.Models {
		m := &c.Models[i]
		if name, gated := m.RequiredPlugin(); gated && name == "" {
			missing[m.PluginID] = m.Name
		}
	}
	for id, example := range missing {
		t.Errorf("plugin_id %q (e.g. %q) has no entry in pluginNames", id, example)
	}
}

// TestRequiredPlugin checks the three cases: a non-plugin model, a plugin model with a
// known plugin_id, and one whose plugin_id the table doesn't know (gated, no name).
func TestRequiredPlugin(t *testing.T) {
	for _, tc := range []struct {
		m         Model
		wantName  string
		wantGated bool
	}{
		{Model{Name: "Brit 2203"}, "", false},
		{Model{Name: "Plini Clean", PluginID: "3"}, "Plini", true},
		{Model{Name: "Future Device", PluginID: "999"}, "", true}, // unknown: gated, no name
	} {
		name, gated := tc.m.RequiredPlugin()
		if name != tc.wantName || gated != tc.wantGated {
			t.Errorf("%s: RequiredPlugin() = (%q, %v), want (%q, %v)", tc.m.Name, name, gated, tc.wantName, tc.wantGated)
		}
	}
}

// TestCanonicalPluginName checks that plugin names match ignoring case and surrounding
// space, and that an unknown name is rejected.
func TestCanonicalPluginName(t *testing.T) {
	for in, want := range map[string]string{"cory wong": "Cory Wong", " John Mayer ": "John Mayer", "SLO-100": "SLO-100"} {
		if got, ok := CanonicalPluginName(in); !ok || got != want {
			t.Errorf("CanonicalPluginName(%q) = (%q, %v), want %q", in, got, ok, want)
		}
	}
	if got, ok := CanonicalPluginName("Not A Plugin"); ok {
		t.Errorf("CanonicalPluginName(unknown) = %q, want not ok", got)
	}
}
