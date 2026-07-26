package agents

import (
	"testing"
)

func TestRunMechanicalQualityChecks_ParseError(t *testing.T) {
	report := RunMechanicalQualityChecks("not json at all", map[string]bool{})
	if report.ParseError == "" {
		t.Fatal("expected ParseError to be set for unparseable input")
	}
	if report.TotalDefects() != 0 {
		t.Errorf("expected zero defect counts alongside a parse error, got %+v", report)
	}
}

// A missing or explicitly-null structured_payload must be flagged as a ParseError, not scored
// as a flawless 0-defect run -- see the doc comment in RunMechanicalQualityChecks for why a
// value-typed field can't distinguish these cases (PR #84 review finding).
func TestRunMechanicalQualityChecks_MissingStructuredPayload(t *testing.T) {
	for name, raw := range map[string]string{
		"key absent entirely": `{"builder_statement": "..."}`,
		"explicit JSON null":  `{"structured_payload": null}`,
	} {
		t.Run(name, func(t *testing.T) {
			report := RunMechanicalQualityChecks(raw, map[string]bool{})
			if report.ParseError == "" {
				t.Fatalf("expected ParseError for %s, got a scored report: %+v", name, report)
			}
			if report.TotalDefects() != 0 {
				t.Errorf("expected zero defect counts alongside a parse error, got %+v", report)
			}
		})
	}
}

func TestRunMechanicalQualityChecks_Clean(t *testing.T) {
	validBlocks := map[string]bool{"US Twin Vibrato": true, "Any Cab": false}
	raw := `{"structured_payload": {"guitars": {"Guitar 1": [
		{"id": "1", "type": "Amplifier", "model": "US Twin Vibrato", "parameters": [
			{"name": "Gain", "type": "slider", "value": "5.0"},
			{"name": "Bass", "type": "slider", "value": "4.2"}
		]},
		{"id": "2", "type": "Cabinet", "model": "Any Cab", "parameters": [
			{"name": "Mic 1 Position", "type": "slider", "value": "0.5"},
			{"name": "Mic 2 Position", "type": "slider", "value": "0.8"},
			{"name": "Blend", "type": "slider", "value": "50"}
		]}
	]}}}`

	report := RunMechanicalQualityChecks(raw, validBlocks)
	if report.ParseError != "" {
		t.Fatalf("unexpected parse error: %s", report.ParseError)
	}
	if report.TotalBlocks != 2 {
		t.Errorf("expected TotalBlocks=2, got %d", report.TotalBlocks)
	}
	if got := report.TotalDefects(); got != 0 {
		t.Errorf("expected a clean preset to have zero defects, got %d (%+v)", got, report)
	}
}

func TestRunMechanicalQualityChecks_CountsEachDefectType(t *testing.T) {
	// "Brit Plexi 100 Patch" is a known, verified, algorithmic (non-capture) block;
	// "Fake Boutique Fuzz XYZ" and "Some Cab" match nothing in validBlocks, and both
	// "Amplifier"/"drive"/"Cabinet" are gear-block categories subject to name verification
	// (see TestFlagUnverifiedStructuredBlocks), so both get flagged unverified.
	validBlocks := map[string]bool{"Brit Plexi 100 Patch": false}
	raw := `{"structured_payload": {"guitars": {"Guitar 1": [
		{"id": "1", "type": "Amplifier", "model": "Brit Plexi 100 Patch", "rationale": "⚠️ Critic: contradicts builder statement.", "parameters": [
			{"name": "Gain", "type": "slider", "value": "+2.0 dB"},
			{"name": "Bass", "type": "slider", "value": "4.0"}
		]},
		{"id": "2", "type": "drive", "model": "Fake Boutique Fuzz XYZ", "parameters": [
			{"name": "Level", "type": "slider", "value": "10-15"}
		]},
		{"id": "3", "type": "Cabinet", "model": "Some Cab", "parameters": [
			{"name": "High Cut", "type": "slider", "value": "5000"}
		]}
	]}}}`

	report := RunMechanicalQualityChecks(raw, validBlocks)
	if report.ParseError != "" {
		t.Fatalf("unexpected parse error: %s", report.ParseError)
	}
	if report.CaptureFormattingMismatches != 1 {
		t.Errorf("expected 1 capture-formatting mismatch (Gain on a confirmed-algorithmic block), got %d", report.CaptureFormattingMismatches)
	}
	if report.LeftoverValueRanges != 1 {
		t.Errorf("expected 1 leftover-range flag (Level=\"10-15\"), got %d", report.LeftoverValueRanges)
	}
	if report.IncompleteCabinetBlocks != 1 {
		t.Errorf("expected 1 incomplete-cabinet flag (missing Mic 1/Mic 2/Blend), got %d", report.IncompleteCabinetBlocks)
	}
	if report.UnverifiedBlocks != 2 {
		t.Errorf("expected 2 unverified-block flags (Fake Boutique Fuzz XYZ, Some Cab), got %d", report.UnverifiedBlocks)
	}
	if report.CriticIssues != 1 {
		t.Errorf("expected 1 pre-baked critic issue, got %d", report.CriticIssues)
	}
	if report.TotalBlocks != 3 {
		t.Errorf("expected TotalBlocks=3, got %d", report.TotalBlocks)
	}
	if got, want := report.TotalDefects(), 1+1+1+2+1; got != want {
		t.Errorf("expected TotalDefects()=%d, got %d", want, got)
	}
}
