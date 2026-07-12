package agents

import (
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestRenderPresetHTML_NoDoubleEscapedBreaks(t *testing.T) {
	blocks := []storage.EffectBlock{
		{
			ID: "b1", Type: "Amplifier", Model: "Brit 2203", Rationale: "Classic British crunch.",
			Parameters: []storage.BlockParameter{
				{Name: "Gain", Type: "slider", Value: "5.0", ValueB: "7.5"},
				{Name: "Bass", Type: "slider", Value: "5.0"},
			},
		},
	}

	out := renderPresetHTML(blocks)

	// The bug this guards against: escaping the whole joined string (instead of each
	// parameter individually) turns the literal "<br/>" separator into "&lt;br/&gt;",
	// which renders as visible raw text in the browser instead of a line break.
	if strings.Contains(out, "&lt;br") {
		t.Errorf("found double-escaped <br/> tag in rendered HTML: %s", out)
	}
	if !strings.Contains(out, "Gain: 5.0<br/>Bass: 5.0") {
		t.Errorf("expected a real <br/> separator between parameters, got: %s", out)
	}
}

func TestRenderPresetHTML_ValueBFallsBackToValue(t *testing.T) {
	blocks := []storage.EffectBlock{
		{
			ID: "b1", Type: "Reverb", Model: "Spring", Rationale: "Ambience.",
			Parameters: []storage.BlockParameter{
				{Name: "Mix", Type: "slider", Value: "15%"}, // no ValueB set
			},
		},
	}

	out := renderPresetHTML(blocks)
	sceneA := "Mix: 15%"
	if strings.Count(out, sceneA) != 2 {
		t.Errorf("expected Scene A and Scene B to both show %q when ValueB is unset, got: %s", sceneA, out)
	}
}

func TestRenderPresetHTML_EscapesUserContent(t *testing.T) {
	blocks := []storage.EffectBlock{
		{
			ID: "b1", Type: "Amplifier", Model: "<script>alert(1)</script>", Rationale: "Test",
			Parameters: []storage.BlockParameter{{Name: "Gain", Type: "slider", Value: "5.0"}},
		},
	}

	out := renderPresetHTML(blocks)
	if strings.Contains(out, "<script>") {
		t.Errorf("expected model name to be HTML-escaped, got raw script tag: %s", out)
	}
}

func TestInjectRenderedHTML_SkipsWhenNoStructuredPayload(t *testing.T) {
	raw := `{"conversational_response": "hi", "dsp_matrix_updated": false}`
	out, err := injectRenderedHTML(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "final_html_payload") {
		t.Errorf("expected no final_html_payload injected when structured_payload is absent, got: %s", out)
	}
}

func TestInjectRenderedHTML_RendersFromStructuredPayload(t *testing.T) {
	raw := `{"builder_statement": "test", "structured_payload": {"guitars": {"Tele": [{"id":"b1","type":"Amplifier","model":"Plexi","rationale":"r","parameters":[{"name":"Gain","type":"slider","value":"5.0"}]}]}}, "agent_impact": []}`
	out, err := injectRenderedHTML(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "final_html_payload") {
		t.Errorf("expected final_html_payload to be injected, got: %s", out)
	}
	if !strings.Contains(out, "grid-matrix") {
		t.Errorf("expected rendered table to be present, got: %s", out)
	}
}
