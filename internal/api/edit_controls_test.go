package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
