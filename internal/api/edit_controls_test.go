package api

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// A catalog model's Edit view shows every device control as the control the device has:
// a slider over its real range, a dropdown of its named positions, and the controls the
// preset leaves alone at their default, tagged as such.
func TestBlockControlsHTMLForCatalogModel(t *testing.T) {
	tgt := editTarget{presetID: "p1", guitar: "Strat", blockID: "b1", tabID: "strat"}
	delay := blockControlsHTML(tgt, storage.EffectBlock{ID: "b1", Type: "Delay", Model: "Analog Delay",
		Parameters: []storage.BlockParameter{
			{Name: "Bypass", Type: "toggle", Value: "On"},
			{Name: "MIX", Type: "slider", Value: "30 %"},
			{Name: "TRAILS", Type: "toggle", Value: "on"},
		},
		KeptDefaults: []string{"FEEDBACK"},
	})
	for _, want := range []string{
		// Every control is present, untouched ones tagged "default".
		`>WIDTH <span class="param-default-tag"`,
		`>FEEDBACK <span class="param-default-tag"`,
		// A % control is a slider over 0..100 at the preset's value, posting its unit.
		`min="0" max="100" step="1" value="30"`,
		`&#34;unit&#34;:&#34;%&#34;`,
		// A named switch is a dropdown with the preset's position selected, matched
		// regardless of case.
		`<option value="On" selected>On</option>`,
		// SYNC NOTE's positions come from the device.
		`<option value="1/4" selected>1/4</option>`,
	} {
		if !strings.Contains(delay, want) {
			t.Errorf("Analog Delay controls missing %q", want)
		}
	}
	if strings.Contains(delay, `type="checkbox"`) {
		t.Error("switches should be dropdowns, not checkboxes")
	}
	if strings.Index(delay, "Bypass") > strings.Index(delay, "MIX") {
		t.Error("Bypass should come first")
	}

	// A control whose range isn't 0..10 gets its own range, not the 0..10 dial slider.
	comp := blockControlsHTML(tgt, storage.EffectBlock{ID: "b1", Model: "Opto Comp", Parameters: []storage.BlockParameter{{Name: "RATIO", Type: "slider", Value: "12"}}})
	if !strings.Contains(comp, `min="2" max="20" step="0.1" value="12"`) {
		t.Errorf("Opto Comp RATIO should be a 2..20 slider at 12")
	}

	// A cab's microphone choice comes before its per-mic controls.
	cab := blockControlsHTML(tgt, storage.EffectBlock{ID: "b1", Model: "412 Brit 60B V30 95 (M)", Parameters: []storage.BlockParameter{
		{Name: "MIC 1 POSITION", Type: "slider", Value: "2"}, {Name: "Mic 1", Type: "dropdown", Value: "Dyn 57"}}})
	if strings.Index(cab, ">Mic 1<") > strings.Index(cab, "MIC 1 LEVEL") {
		t.Error("Mic 1 should come before the per-mic controls")
	}

	// A model outside the catalog keeps the generic controls; a toggle is a dropdown too.
	generic := blockControlsHTML(tgt, storage.EffectBlock{ID: "b1", Model: "Made Up Pedal", Parameters: []storage.BlockParameter{
		{Name: "Bypass", Type: "toggle", Value: "On"}, {Name: "Drive", Type: "slider", Value: "4"}}})
	if !strings.Contains(generic, `<option value="On" selected>On</option>`) || !strings.Contains(generic, `min="0" max="10"`) {
		t.Errorf("generic controls: %s", generic)
	}
	if strings.Contains(generic, "param-default-tag") {
		t.Error("a model outside the catalog has no defaults to show")
	}
}

func postUpdate(t *testing.T, s *Server, fields map[string]string) int {
	t.Helper()
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	req, _ := http.NewRequest(http.MethodPost, "/api/preset/update_parameter", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.handleUpdateParameter().ServeHTTP(rr, req)
	return rr.Code
}

// Editing a control the preset left at its default records it as a parameter and drops
// it from kept_defaults; a slider's bare number gets the control's unit back.
func TestUpdateParameterRecordsDefaultControl(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	s := NewServer(store, nil, mockStorage, &mockSecretFetcher{}, nil, nil)
	payload := `{"guitars":{"Strat":[{"id":"b1","type":"Delay","model":"Analog Delay","parameters":[{"name":"MIX","type":"slider","value":"30 %"}],"kept_defaults":["FEEDBACK","TRAILS"]}]}}`
	store.Save(context.Background(), &storage.Preset{ID: "p1", Name: "Test", Payload: payload})

	base := map[string]string{"preset_id": "p1", "guitar": "Strat", "block_id": "b1"}
	with := func(kv ...string) map[string]string {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}
	if code := postUpdate(t, s, with("param_name", "FEEDBACK", "value", "40", "unit", "%")); code != http.StatusNoContent {
		t.Fatalf("edit of a kept default: status %d", code)
	}
	if code := postUpdate(t, s, with("param_name", "MIX", "value", "45", "unit", "%")); code != http.StatusNoContent {
		t.Fatalf("edit of a set control: status %d", code)
	}
	if code := postUpdate(t, s, with("param_name", "NOT A CONTROL", "value", "1")); code != http.StatusNotFound {
		t.Errorf("an unknown name should still be 404, got %d", code)
	}

	p, _ := store.Get(context.Background(), "p1")
	var sp storage.StructuredPreset
	if err := json.Unmarshal([]byte(p.Payload), &sp); err != nil {
		t.Fatal(err)
	}
	b := sp.Guitars["Strat"][0]
	got := map[string]string{}
	for _, prm := range b.Parameters {
		got[prm.Name] = prm.Value
	}
	if got["FEEDBACK"] != "40 %" || got["MIX"] != "45 %" {
		t.Errorf("parameters = %v, want FEEDBACK 40 %% and MIX 45 %%", got)
	}
	if len(b.KeptDefaults) != 1 || b.KeptDefaults[0] != "TRAILS" {
		t.Errorf("kept_defaults = %v, want only TRAILS", b.KeptDefaults)
	}
}

// Parameter names and block ids are LLM-written. HTML escaping doesn't protect a JS string
// inside an event attribute (the browser decodes &#39; back to ' before running it), so
// every slider's oninput script must be the one constant script, with no preset data in it.
func TestSliderScriptHasNoPresetData(t *testing.T) {
	tgt := editTarget{presetID: "p1", guitar: "Strat", blockID: "b1');alert(2);//", tabID: "strat"}
	for _, block := range []storage.EffectBlock{
		{ID: tgt.blockID, Model: "Made Up Pedal", Parameters: []storage.BlockParameter{{Name: "x');alert(1);//", Type: "slider", Value: "5"}}},
		{ID: tgt.blockID, Model: "Green 808", Parameters: []storage.BlockParameter{{Name: "OVERDRIVE", Type: "slider", Value: "5"}}},
		{ID: tgt.blockID, Model: "Analog Delay", Parameters: []storage.BlockParameter{{Name: "MIX", Type: "slider", Value: "30 %"}}},
	} {
		out := blockControlsHTML(tgt, block)
		scripts := regexp.MustCompile(`oninput="([^"]*)"`).FindAllStringSubmatch(out, -1)
		if len(scripts) == 0 {
			t.Fatalf("no slider rendered for %s", block.Model)
		}
		for _, m := range scripts {
			if html.UnescapeString(m[1]) != sliderLiveScript {
				t.Errorf("slider script for %s isn't the constant script: %s", block.Model, m[1])
			}
		}
	}
}

func TestApplyParamEdit(t *testing.T) {
	b := storage.EffectBlock{Model: "Analog Delay", KeptDefaults: []string{"TRAILS", "WIDTH"},
		Parameters: []storage.BlockParameter{{Name: "Level", Type: "slider", Value: "-3", Unit: "dB"}}}
	// A unit typed back into a parameter that stores it separately is stripped.
	if !applyParamEdit(&b, "Level", "-6 DB") || b.Parameters[0].Value != "-6" {
		t.Errorf("Level = %q, want -6", b.Parameters[0].Value)
	}
	// A control at its device default becomes a parameter and leaves kept_defaults.
	if !applyParamEdit(&b, "width", "75 %") {
		t.Fatal("WIDTH edit not applied")
	}
	if last := b.Parameters[len(b.Parameters)-1]; last.Name != "WIDTH" || last.Value != "75 %" {
		t.Errorf("added parameter = %+v, want WIDTH 75 %%", last)
	}
	if len(b.KeptDefaults) != 1 || b.KeptDefaults[0] != "TRAILS" {
		t.Errorf("kept_defaults = %v, want [TRAILS]", b.KeptDefaults)
	}
	// A name that's neither a parameter nor a device control is not found.
	if applyParamEdit(&b, "Made Up", "1") {
		t.Error("unknown parameter reported as updated")
	}
}

// A parameter that keeps its unit in Unit: the posted value's unit decides what's stored.
func TestStoredValueUnit(t *testing.T) {
	for _, c := range []struct{ value, unit, wantV, wantU string }{
		{"-6 DB", "dB", "-6", "dB"},
		{"-65.0dB", "dB", "-65.0", "dB"},
		{"300 ms", "s", "300 ms", ""}, // a slider over a ms control, on a value stored in s
		{"4", "dB", "4", "dB"},
		{"OFF", "dB", "OFF", "dB"},
		{"5", "", "5", ""},
	} {
		if v, u := storedValueUnit(c.value, c.unit); v != c.wantV || u != c.wantU {
			t.Errorf("storedValueUnit(%q, %q) = %q, %q; want %q, %q", c.value, c.unit, v, u, c.wantV, c.wantU)
		}
	}
}

// A device control whose preset value keeps its unit in Unit is shown in the right units.
func TestDeviceControlUsesSeparateUnit(t *testing.T) {
	tgt := editTarget{presetID: "p1", guitar: "Strat", blockID: "b1", tabID: "strat"}
	out := blockControlsHTML(tgt, storage.EffectBlock{ID: "b1", Model: "Analog Delay",
		Parameters: []storage.BlockParameter{{Name: "DELAY TIME", Type: "slider", Value: "0.38", Unit: "s"}}})
	if !strings.Contains(out, `value="380" data-unit=" ms"`) {
		t.Errorf("DELAY TIME 0.38 s should be a slider at 380 ms:\n%s", out)
	}
}
