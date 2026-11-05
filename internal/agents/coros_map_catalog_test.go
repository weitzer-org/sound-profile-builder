package agents

import (
	"sort"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/catalog"
)

// TestCorosMapNamesExistInCatalog keeps coros_map.json honest against the device's own
// catalog (internal/catalog): every non-capture entry's coros_equivalent must be a real
// block name on the Quad Cortex, either an exact model name or a mono/stereo family name
// ("412 Brit 60B V30 95" for its (M) and (ST) models).
//
// Neural Captures are skipped because the device catalog does not list them. An empty
// coros_equivalent is skipped too: GetValidNativeBlocks already ignores those entries, so
// they never reach the pipeline as a block name.
//
// When this fails after a CorOS update or a coros_map.json edit, fix the entry to the
// catalog's spelling -- don't loosen the test.
func TestCorosMapNamesExistInCatalog(t *testing.T) {
	cat, err := catalog.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for gear, rec := range CorosMapRecords() {
		if isCap, _ := rec["is_capture"].(bool); isCap {
			continue
		}
		equiv, _ := rec["coros_equivalent"].(string)
		if equiv == "" {
			continue
		}
		if !cat.IsKnownName(equiv) {
			bad = append(bad, gear+" -> "+equiv)
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("coros_map.json entry %q: not a block name in the device catalog (CorOS %s)", b, cat.Source.CorOSVersion)
	}
}

// TestCorosMapPluginEntriesUsePluginModels: an entry gated behind a paid plugin
// (required_plugin) must map to that plugin's own device in the catalog, never a native
// stand-in. It checks is_capture entries too: plugin devices are not Neural Captures, and
// a wrong is_capture flag previously hid a dozen such entries from the name check above.
func TestCorosMapPluginEntriesUsePluginModels(t *testing.T) {
	cat, err := catalog.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for gear, rec := range CorosMapRecords() {
		plugin, _ := rec["required_plugin"].(string)
		equiv, _ := rec["coros_equivalent"].(string)
		if plugin == "" || equiv == "" {
			continue
		}
		models := cat.Family(catalog.FamilyName(equiv))
		if m, ok := cat.Lookup(equiv); ok {
			models = []*catalog.Model{m}
		}
		isPlugin := len(models) > 0
		for _, m := range models {
			if m.PluginID == "" {
				isPlugin = false
			}
		}
		if !isPlugin {
			bad = append(bad, gear+" ("+plugin+") -> "+equiv)
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("coros_map.json entry %q: required_plugin set, but the block is not a plugin model in the device catalog", b)
	}
}

// TestPluginTableAgreesWithCorosMap: catalog.PluginName (keyed by the device's plugin_id)
// and coros_map.json's hand-written required_plugin must name the same plugin for the
// same model, or one of the two gating paths would check ownership of the wrong plugin.
func TestPluginTableAgreesWithCorosMap(t *testing.T) {
	cat, err := catalog.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for gear, rec := range CorosMapRecords() {
		plugin, _ := rec["required_plugin"].(string)
		equiv, _ := rec["coros_equivalent"].(string)
		if plugin == "" || equiv == "" {
			continue
		}
		models := cat.Family(catalog.FamilyName(equiv))
		if m, ok := cat.Lookup(equiv); ok {
			models = []*catalog.Model{m}
		}
		for _, m := range models {
			if name, _ := m.RequiredPlugin(); name != plugin {
				bad = append(bad, gear+": required_plugin "+plugin+", catalog plugin_id "+m.PluginID+" is "+name)
			}
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Error(b)
	}
}

// TestCaptureNamesAreNotCatalogModels: a Neural Capture is not a block model, so no
// coros_map.json capture entry or user capture may use a name the device catalog lists as
// a model. If one did, the valid-block set would have to call the same name both a
// capture and a native block, and the capture flag would win silently.
func TestCaptureNamesAreNotCatalogModels(t *testing.T) {
	cat, err := catalog.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for gear, rec := range CorosMapRecords() {
		equiv, _ := rec["coros_equivalent"].(string)
		if isCap, _ := rec["is_capture"].(bool); isCap && cat.IsKnownName(equiv) {
			bad = append(bad, "coros_map.json "+gear+" -> "+equiv)
		}
	}
	for _, c := range GetUserCaptures() {
		if cat.IsKnownName(c.Name) {
			bad = append(bad, "user_captures.json "+c.Name)
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("%s: marked as a capture, but the device catalog lists it as a block model", b)
	}
}
