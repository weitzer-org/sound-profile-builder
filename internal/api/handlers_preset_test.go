package api

import (
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestRenderTweakingWorkspaceHTML(t *testing.T) {
	// 1. Test Draft Preset forces legacy mode
	draftPreset := &storage.Preset{
		Name: "Draft Preset",
		Payload: `{"structured": {"guitars": {"Strat": []}}, "legacy_html": {"Strat": "<table class='grid-matrix'></table>"}}`,
	}
	html := renderTweakingWorkspaceHTML(draftPreset, false, false)
	// The sanitizer (bluemonday) normalizes attribute quoting to double
	// quotes, so this doesn't match the single-quoted source payload verbatim.
	if !strings.Contains(html, `<table class="grid-matrix">`) {
		t.Errorf("Expected draft preset to render legacy table, got: %s", html)
	}
	if strings.Contains(html, "param-group") {
		t.Errorf("Expected draft preset to NOT render editable controls")
	}

	// 2. Test forceStatic forces legacy mode
	savedPreset := &storage.Preset{
		Name: "My Preset",
		Payload: `{"structured": {"guitars": {"Strat": []}}, "legacy_html": {"Strat": "<table class='grid-matrix'></table>"}}`,
	}
	html = renderTweakingWorkspaceHTML(savedPreset, false, true) // forceStatic = true
	if !strings.Contains(html, `<table class="grid-matrix">`) {
		t.Errorf("Expected forceStatic to render legacy table")
	}

	// 3. Test Saved Preset with structured data renders editable view
	structuredPreset := &storage.Preset{
		Name: "My Preset",
		Payload: `{
			"guitars": {
				"Strat": [
					{
						"id": "block-1",
						"type": "Amplifier",
						"model": "Plexi",
						"parameters": [
							{"name": "Gain", "type": "slider", "value": "7.0"}
						]
					}
				]
			}
		}`,
	}
	html = renderTweakingWorkspaceHTML(structuredPreset, false, false)
	if !strings.Contains(html, "param-group") {
		t.Errorf("Expected saved structured preset to render editable controls, got: %s", html)
	}
	if strings.Contains(html, "<table class='grid-matrix'>") {
		t.Errorf("Expected saved structured preset to NOT render legacy table")
	}

	// 4. Test Fallback Table Generation when forced static but only have structured data
	fallbackPreset := &storage.Preset{
		Name: "Draft Preset",
		Payload: `{
			"guitars": {
				"Strat": [
					{
						"id": "block-1",
						"type": "Amplifier",
						"model": "Plexi",
						"parameters": [
							{"name": "Gain", "type": "slider", "value": "7.0"}
						]
					}
				]
			}
		}`,
	}
	html = renderTweakingWorkspaceHTML(fallbackPreset, false, false) // Draft forces static
	if !strings.Contains(html, "<table class=\"grid-matrix\"") {
		t.Errorf("Expected fallback to generate table, got: %s", html)
	}
	if !strings.Contains(html, "Amplifier") || !strings.Contains(html, "Plexi") || !strings.Contains(html, "Gain: 7.0") {
		t.Errorf("Expected generated table to contain block details")
	}
	if !strings.Contains(html, "SCENE A (RHYTHM)") || !strings.Contains(html, "SCENE B (LEAD)") {
		t.Errorf("Expected generated table to contain scene columns")
	}
}

// TestRenderTweakingWorkspaceHTML_SliderUnitFix is the Phase 0 contract for
// fixing the range-slider bug: today every "slider" param renders
// min="0" max="10" regardless of its Unit (storage.BlockParameter already
// carries Unit, e.g. "dB"/"Hz"/"ms"/"%", it's just ignored at render time),
// so a "-65.0 dB" threshold or "8000 Hz" cutoff is an unusable slider. A
// unit-less slider (true 0-10 dimensionless value, e.g. Gain/Tone/Volume)
// must keep rendering as a range input; a unit-bearing one must render as a
// labeled text input instead, the same way "Mic 1"/"Mic 2" text params
// already correctly do.
func TestRenderTweakingWorkspaceHTML_SliderUnitFix(t *testing.T) {
	preset := &storage.Preset{
		Name: "My Preset",
		Payload: `{
			"guitars": {
				"Strat": [
					{
						"id": "block-1",
						"type": "Noise Gate",
						"model": "Gate",
						"parameters": [
							{"name": "Threshold", "type": "slider", "value": "-65.0", "unit": "dB"},
							{"name": "Gain", "type": "slider", "value": "7.0"}
						]
					}
				]
			}
		}`,
	}
	html := renderTweakingWorkspaceHTML(preset, false, false)

	if !strings.Contains(html, `value="7.0"`) || !strings.Contains(html, `type="range"`) {
		t.Errorf("Expected unit-less slider param (Gain) to render as a range input, got: %s", html)
	}
	if !strings.Contains(html, `value="-65.0 dB"`) {
		t.Errorf("Expected unit-bearing param (Threshold) to render its value with the unit, got: %s", html)
	}
	// The unit-bearing param must NOT be a 0-10 range input (that's the bug).
	thresholdIdx := strings.Index(html, "Threshold")
	if thresholdIdx == -1 {
		t.Fatalf("Threshold param not found in output")
	}
	nextRange := strings.Index(html[thresholdIdx:], `type="range"`)
	nextText := strings.Index(html[thresholdIdx:], `type="text"`)
	if nextRange != -1 && (nextText == -1 || nextRange < nextText) {
		t.Errorf("Expected Threshold (unit=dB) to render as type=\"text\", not a 0-10 range slider")
	}
}

// TestRenderTweakingWorkspaceHTML_SliderUnitFix_EmbeddedUnit covers the shape
// actually produced by the live agent pipeline (found by running a real
// generation, not just the hand-built fixture above): the Architect &
// Evaluator agent often doesn't populate the separate Unit field at all --
// it puts the whole "+2.5 dB" / "8000 Hz" string directly in Value. Checking
// param.Unit alone missed this case entirely and still rendered an invalid
// 0-10 slider for it.
func TestRenderTweakingWorkspaceHTML_SliderUnitFix_EmbeddedUnit(t *testing.T) {
	preset := &storage.Preset{
		Name: "My Preset",
		Payload: `{
			"guitars": {
				"Strat": [
					{
						"id": "block-1",
						"type": "Utility",
						"model": "Input Gate",
						"parameters": [
							{"name": "Input Gain", "type": "slider", "value": "+2.5 dB"},
							{"name": "High Cut", "type": "slider", "value": "8000 Hz"},
							{"name": "Drive", "type": "slider", "value": "1.5"}
						]
					}
				]
			}
		}`,
	}
	html := renderTweakingWorkspaceHTML(preset, false, false)

	if !strings.Contains(html, `value="1.5"`) {
		t.Errorf("Expected bare-number param (Drive) to still render as a range input, got: %s", html)
	}
	for _, label := range []string{"Input Gain", "High Cut"} {
		idx := strings.Index(html, label)
		if idx == -1 {
			t.Fatalf("%s param not found in output", label)
		}
		nextRange := strings.Index(html[idx:], `type="range"`)
		nextText := strings.Index(html[idx:], `type="text"`)
		if nextRange != -1 && (nextText == -1 || nextRange < nextText) {
			t.Errorf("Expected %s (unit embedded in value, no separate Unit field) to render as type=\"text\", not a 0-10 range slider", label)
		}
	}
	if !strings.Contains(html, `value="+2.5 dB"`) || !strings.Contains(html, `value="8000 Hz"`) {
		t.Errorf("Expected embedded-unit values to render unchanged (no unit field to append), got: %s", html)
	}
}

// TestRenderTweakingWorkspaceHTML_SliderRejectsNonFiniteValue guards against
// strconv.ParseFloat accepting "NaN"/"Inf" as valid floats: a slider param
// carrying one of those strings must fall into the text-input branch instead
// of an <input type="range" value="NaN">, which has no meaningful position.
func TestRenderTweakingWorkspaceHTML_SliderRejectsNonFiniteValue(t *testing.T) {
	preset := &storage.Preset{
		Name: "My Preset",
		Payload: `{
			"guitars": {
				"Strat": [
					{
						"id": "block-1",
						"type": "Overdrive",
						"model": "Test",
						"parameters": [
							{"name": "Drive", "type": "slider", "value": "NaN"}
						]
					}
				]
			}
		}`,
	}
	html := renderTweakingWorkspaceHTML(preset, false, false)

	idx := strings.Index(html, "Drive")
	if idx == -1 {
		t.Fatalf("Drive param not found in output")
	}
	nextRange := strings.Index(html[idx:], `type="range"`)
	nextText := strings.Index(html[idx:], `type="text"`)
	if nextRange != -1 && (nextText == -1 || nextRange < nextText) {
		t.Errorf(`Expected a "NaN" value to render as type="text", not an invalid 0-10 range slider, got: %s`, html)
	}
}

// TestRenderTweakingWorkspaceHTML_CopyOfFlatLegacyPreset is the regression
// contract for a code-review finding: presets saved via the older
// map[string]string Payload shape (no structured/legacy_html envelope, e.g.
// cmd/save_presets) have no structured guitar data. Copying one and clicking
// "Edit" (isCopyMode=false) used to unmarshal that flat map as a technically
// valid but empty StructuredPreset, producing a blank workspace with zero
// param groups instead of falling back to the original table content.
func TestRenderTweakingWorkspaceHTML_CopyOfFlatLegacyPreset(t *testing.T) {
	flatLegacyPayload := `{"Strat": "<table class='grid-matrix'><tr><td>Overdrive: Green 808</td></tr></table>"}`
	presetCopy := &storage.Preset{
		Name:    "1940s muddy waters (Copy)",
		Payload: flatLegacyPayload,
	}

	html := renderTweakingWorkspaceHTML(presetCopy, false, false)

	if !strings.Contains(html, "Green 808") {
		t.Errorf("Expected the copy to fall back to the original legacy table content instead of a blank workspace, got: %s", html)
	}
	if strings.Contains(html, "hx-get=\"/api/preset/view") {
		t.Errorf("Expected no View/Edit toggle for a preset with no structured data (both states would render identically), got: %s", html)
	}
}

func TestSanitizeAgentHTML(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantGone []string
		wantKept []string
	}{
		{
			name:     "strips script tags",
			input:    `Overdrive: Green 808<script>alert(1)</script><br/>Drive: 5.0`,
			wantGone: []string{"script", "alert(1)"},
			wantKept: []string{"Overdrive: Green 808", "Drive: 5.0"},
		},
		{
			name:     "strips a script tag with whitespace before the closing bracket",
			input:    `before<script >alert(1)</script >after`,
			wantGone: []string{"script", "alert(1)"},
			wantKept: []string{"before", "after"},
		},
		{
			name:     "strips inline event handlers",
			input:    `<div onclick="alert(1)">Rationale text</div>`,
			wantGone: []string{"onclick", "alert"},
			wantKept: []string{"<div>", "Rationale text"},
		},
		{
			name: "strips event handlers introduced without leading whitespace (svg/onload)",
			// A hand-rolled regex requiring "\s+on[a-z]+=" misses this --
			// "/" separates the attribute instead of a space.
			input:    `<svg/onload=alert(1)>`,
			wantGone: []string{"onload", "alert", "<svg"},
		},
		{
			name:     "neutralizes javascript: URIs",
			input:    `<a href="javascript:alert(1)">click</a>`,
			wantGone: []string{"javascript:alert(1)", "<a"},
		},
		{
			name: "neutralizes HTML-entity-encoded javascript: schemes",
			// Browsers decode HTML entities in attribute values before
			// resolving the URL scheme, so a regex matching the literal
			// string "javascript:" can be bypassed this way.
			input:    `<a href="jav&#x61;script:alert(1)">click</a>`,
			wantGone: []string{"alert(1)", "<a"},
		},
		{
			name: "strips dangerous attributes beyond href/src",
			// <object data="..."> and <form action="..."> can also execute
			// script; a regex only checking href/src misses these entirely.
			input:    `<object data="javascript:alert(1)"></object>`,
			wantGone: []string{"javascript:alert(1)", "<object", "data="},
		},
		{
			name:     "strips inline style attributes entirely",
			input:    `<div style='background:url(javascript:alert(1))'>Rationale text</div>`,
			wantGone: []string{"style", "javascript:alert(1)"},
			wantKept: []string{"<div>", "Rationale text"},
		},
		{
			name:     "preserves legitimate formatting tags and table structure",
			input:    `<table class='grid-matrix'><tr><td><div><em>Rationale: Boosts the sweet spot</em></div></td></tr></table>`,
			wantKept: []string{`<table class="grid-matrix">`, "<tr>", "<td>", "<div>", "<em>Rationale: Boosts the sweet spot</em>"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := sanitizeAgentHTML(tc.input)
			for _, s := range tc.wantGone {
				if strings.Contains(out, s) {
					t.Errorf("Expected %q to be stripped, got: %s", s, out)
				}
			}
			for _, s := range tc.wantKept {
				if !strings.Contains(out, s) {
					t.Errorf("Expected %q to survive sanitization, got: %s", s, out)
				}
			}
		})
	}
}
