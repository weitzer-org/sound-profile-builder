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
	if !strings.Contains(html, "<table class='grid-matrix'>") {
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
	if !strings.Contains(html, "<table class='grid-matrix'>") {
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
