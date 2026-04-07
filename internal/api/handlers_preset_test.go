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
