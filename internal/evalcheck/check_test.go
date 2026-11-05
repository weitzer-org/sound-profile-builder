package evalcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

func envelope(t *testing.T, model, typ, rationale string, params ...map[string]string) string {
	t.Helper()
	var ps []map[string]string
	for _, p := range params {
		p["type"] = "slider"
		ps = append(ps, p)
	}
	b, err := json.Marshal(map[string]any{"structured_payload": map[string]any{"guitars": map[string]any{
		"G": []map[string]any{{"id": "b1", "type": typ, "model": model, "rationale": rationale, "parameters": ps}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCheck_GroundTruthGating(t *testing.T) {
	valid := agents.BuildEffectiveValidBlocks(true, true)
	// UK C30 TopBoost: non-capture per coros_map.json; Iba Green: capture.
	if valid["UK C30 TopBoost"] || !valid["Iba Green"] {
		t.Fatalf("test premise changed: UK C30 TopBoost=%v Iba Green=%v", valid["UK C30 TopBoost"], valid["Iba Green"])
	}

	r := Check(envelope(t, "UK C30 TopBoost", "Amplifier", "Factory Neural Capture of the Vox AC30.", map[string]string{"name": "Gain", "value": "+1.0 dB"}), valid)
	if r.CaptureDBOnModeled != 1 || r.CaptureClaimOnModeled != 1 {
		t.Fatalf("modeled amp in capture clothing must be flagged twice: %+v", r)
	}

	r = Check(envelope(t, "Iba Green", "Overdrive", "Selected as a pre-trained Neural Capture.", map[string]string{"name": "Gain", "value": "+2.0 dB"}), valid)
	if r.Defects() != 0 {
		t.Fatalf("a real capture described and formatted as one is NOT a defect: %+v", r)
	}

	r = Check(envelope(t, "Iba Green", "Overdrive", "ok", map[string]string{"name": "Gain", "value": "5.0"}), valid)
	if r.ModeledStyleOnCapture != 1 {
		t.Fatalf("capture with 0-10 style value must be flagged: %+v", r)
	}
}

func TestCheck_UnitsAreReferenceOnly(t *testing.T) {
	valid := agents.BuildEffectiveValidBlocks(true, true)
	r := Check(envelope(t, "Spring Reverb", "Reverb", "ok",
		map[string]string{"name": "Decay", "value": "2.2 s"}, map[string]string{"name": "Mix", "value": "35.0%"}, map[string]string{"name": "Level", "value": "5.0"}), valid)
	if r.NonNumericValues != 2 || r.UnitBearingValues != 2 || r.Defects() != 0 {
		t.Fatalf("ordinary unit-bearing values must count as reference only, never as defects: %+v", r)
	}
}

func TestCheck_CaptureClaimNeedsRealClaim(t *testing.T) {
	valid := agents.BuildEffectiveValidBlocks(true, true)
	for _, ok := range []string{
		"Uses off-axis mics to capture warmth.",                    // verb
		"Pairs naturally with the Hiwatt capture.",                 // another block
		"Algorithmic model, not a Neural Capture.",                 // denial
		"A classic algorithmic amp rather than a factory capture.", // denial
	} {
		if r := Check(envelope(t, "UK C30 TopBoost", "Amplifier", ok), valid); r.CaptureClaimOnModeled != 0 {
			t.Errorf("false positive on %q", ok)
		}
	}
	for _, bad := range []string{"Factory Neural Capture of the Vox.", "Selected as a pre-trained Neural Capture.", "This is a neural capture."} {
		if r := Check(envelope(t, "UK C30 TopBoost", "Amplifier", bad), valid); r.CaptureClaimOnModeled != 1 {
			t.Errorf("missed claim in %q", bad)
		}
	}
}

func TestCheck_CriticNotesAreNotArchitectProse(t *testing.T) {
	valid := agents.BuildEffectiveValidBlocks(true, true)
	// The Critic's own note quotes the claim; it must not be double-counted as an Architect claim,
	// and a Critic dispute of a REAL capture is surfaced as probably-wrong.
	rat := "Solid drive. ⚠️ Critic: The rationale describes the block as a 'Neural capture', but it is a native pedal."
	r := Check(envelope(t, "Iba Green", "Overdrive", rat), valid)
	if r.CriticFlags != 1 || r.CriticByType["capture_claim"] != 1 || r.CriticCaptureContradictsMap != 1 || r.CaptureClaimOnModeled != 0 {
		t.Fatalf("unexpected: %+v", r)
	}
}

func TestCheck_ParseFailureIsNotCleanRun(t *testing.T) {
	valid := agents.BuildEffectiveValidBlocks(true, true)
	for _, in := range []string{"not json", `{"structured_payload":null}`, `{"structured_payload":{"guitars":{}}}`} {
		if r := Check(in, valid); r.ParseError == "" {
			t.Errorf("%q must report a parse error, not 0 defects", in)
		}
	}
}

func TestClassifyCriticNote(t *testing.T) {
	cases := map[string]string{
		"describes the block as a Neural capture":  "capture_claim",
		"Bypass parameter is set to On":            "bypass_state",
		"contradicts the Gibson ES-339 Humbuckers": "guitar_mismatch",
		"is a digital delay not tape":              "device_type",
		"something else entirely":                  "other",
	}
	for in, want := range cases {
		if got := ClassifyCriticNote(in); got != want {
			t.Errorf("ClassifyCriticNote(%q) = %s, want %s", in, got, want)
		}
	}
}

// TestValidationAgainstJulyOutputs is plan item 0.8's exit criterion: the checker, run on the
// committed July 2026 outputs, must see the defects July recorded. Each assertion below is a
// finding reproduced from data in the repo, not from memory of TODO.md.
func TestValidationAgainstJulyOutputs(t *testing.T) {
	root := filepath.Join("..", "..", "eval_results", "full_pipeline")
	if _, err := os.Stat(root); err != nil {
		t.Skip("July outputs not present")
	}
	valid := agents.BuildEffectiveValidBlocks(true, true)
	load := func(scen, q string) Report {
		b, err := os.ReadFile(filepath.Join(root, scen, q+".html"))
		if err != nil {
			t.Fatal(err)
		}
		return Check(string(b), valid)
	}

	// 07_Edge, 3.6 Flash as Pro tier: a modeled amp (UK C30 TopBoost) described AND formatted as a
	// Factory Neural Capture, on both guitars. The Critic flagged the same thing (twice).
	edge := load("pro-tier-candidate", "07_Edge")
	if edge.CaptureClaimOnModeled != 2 || edge.CaptureDBOnModeled != 12 || edge.CriticByType["capture_claim"] != 2 {
		t.Errorf("07_Edge pro-tier: want capClaim=2 capDB=12 criticCapture=2, got %+v", edge)
	}
	// 05_Clapton, same scenario: Brit Plexi 100 Patch (Rule 9's own named example) in dB.
	if c := load("pro-tier-candidate", "05_Clapton"); c.CaptureDBOnModeled != 4 {
		t.Errorf("05_Clapton pro-tier: want capDB=4, got %+v", c)
	}
	// Baseline 07_Edge is clean of capture-vs-modeled defects.
	if b := load("baseline", "07_Edge"); b.CaptureClaimOnModeled+b.CaptureDBOnModeled != 0 {
		t.Errorf("baseline 07_Edge should have no capture contradictions: %+v", b)
	}
	// Iba Green on 07_Edge (flash tier) is described as a Neural Capture -- and coros_map.json says
	// it IS one, so a ground-truth checker must NOT flag it. If this assertion ever fails because
	// someone flipped is_capture, the July "Iba Green mislabel" premise needs revisiting.
	if f := load("flash-tier-candidate", "07_Edge"); f.CaptureClaimOnModeled != 0 || f.CaptureDBOnModeled != 0 || f.ModeledStyleOnCapture != 0 {
		t.Errorf("flash-tier 07_Edge Iba Green must not be a capture-claim defect: %+v", f)
	}
	// ...but the Critic's dispute of a real capture on 13_Hard_Rock_Blues is surfaced.
	if h := load("pro-tier-candidate", "13_Hard_Rock_Blues"); h.CriticCaptureContradictsMap != 2 {
		t.Errorf("13_Hard_Rock_Blues: want 2 Critic flags that contradict the map, got %+v", h)
	}
}

func TestValidationJulyTruncations(t *testing.T) {
	path := filepath.Join("..", "..", "eval_results", "full_pipeline", "full_pipeline_gemini36_benchmark_report.md")
	if _, err := os.Stat(path); err != nil {
		t.Skip("July report not present")
	}
	fails, err := ParseBenchmarkReport(path)
	if err != nil {
		t.Fatalf("committed July report failed to parse: %v", err)
	}
	if len(fails) != 4 {
		t.Fatalf("want 4 failed runs, got %d: %+v", len(fails), fails)
	}
	for _, f := range fails {
		if !f.Truncated || f.Agent != "Acoustician" {
			t.Errorf("want Acoustician MAX_TOKENS truncation, got %+v", f)
		}
	}
	if !strings.Contains(fails[0].Scenario, "Flash-Tier") {
		t.Errorf("scenario column mapping looks wrong: %+v", fails[0])
	}
}

func TestAssessCalls(t *testing.T) {
	expected := map[string]string{"12_architect": "gemini-4-argon", "13_critic": "gemini-3.5-flash"}
	good := agents.CallRecord{Agent: "Architect & Evaluator", AttemptModel: "gemini-4-argon", ServedModel: "gemini-4-argon", Success: true, FinishReason: "STOP"}
	if ok, issues, _ := AssessCalls([]agents.CallRecord{good}, expected, nil); !ok {
		t.Fatalf("clean call flagged: %v", issues)
	}
	// Served by a different model than requested: not scoreable.
	bad := good
	bad.ServedModel = "gemini-2.5-pro"
	if ok, _, _ := AssessCalls([]agents.CallRecord{bad}, expected, nil); ok {
		t.Fatal("served-model mismatch must fail")
	}
	// A swallowed Critic failure (RunPipeline continues without it) must still fail the run.
	critFail := agents.CallRecord{Agent: "Preset Critic", AttemptModel: "gemini-3.5-flash", Success: false, Error: "429"}
	if ok, issues, _ := AssessCalls([]agents.CallRecord{good, critFail}, expected, nil); ok || len(issues) != 1 {
		t.Fatalf("failed attempt must fail the run: ok=%v %v", ok, issues)
	}
	// Fallback attempt on a different model.
	fb := agents.CallRecord{Agent: "Preset Critic", AttemptModel: "gemini-2.5-pro", ServedModel: "gemini-2.5-pro", Success: true}
	if ok, _, _ := AssessCalls([]agents.CallRecord{fb}, expected, nil); ok {
		t.Fatal("fallback model must fail the run")
	}
	// Pinned alias: requested alias, served the pinned id.
	exp2 := map[string]string{"12_architect": "gemini-pro-latest"}
	pinned := agents.CallRecord{Agent: "Architect & Evaluator", AttemptModel: "gemini-pro-latest", ServedModel: "gemini-3.1-pro-preview", Success: true}
	if ok, issues, _ := AssessCalls([]agents.CallRecord{pinned}, exp2, map[string]string{"gemini-pro-latest": "gemini-3.1-pro-preview"}); !ok {
		t.Fatalf("pinned alias should pass: %v", issues)
	}
	if ok, _, _ := AssessCalls([]agents.CallRecord{pinned}, exp2, nil); ok {
		t.Fatal("unpinned alias must not pass")
	}
	// MAX_TOKENS counted per agent, even on a retried (failed) attempt.
	trunc := agents.CallRecord{Agent: "Acoustician", AttemptModel: "gemini-3.5-flash", FinishReason: "MAX_TOKENS", Success: false, Error: "truncated"}
	_, _, mt := AssessCalls([]agents.CallRecord{trunc}, map[string]string{"6_acoustician": "gemini-3.5-flash"}, nil)
	if mt["Acoustician"] != 1 {
		t.Fatalf("MAX_TOKENS not counted: %v", mt)
	}
}

func TestSplitCriticNotes_KeepsOrganicWarningEmoji(t *testing.T) {
	prose, notes := splitCriticNotes("Great lead boost ⚠️ watch clipping. ⚠️ Verify: x ⚠️ Critic: bad claim")
	if !strings.Contains(prose, "watch clipping") || strings.Contains(prose, "Verify") || len(notes) != 1 || notes[0] != "bad claim" {
		t.Fatalf("prose=%q notes=%v", prose, notes)
	}
}

func TestClaimsCapture_MultibytePrefix(t *testing.T) {
	// Curly quotes/accents right before the claim must not corrupt the negation look-back.
	if !claimsCapture("Réglé “à l’ancienne” — a Factory Neural Capture of the amp.") {
		t.Error("missed claim after multibyte text")
	}
	if claimsCapture("Réglé “à l’ancienne” — not a Factory Neural Capture.") {
		t.Error("negation lost after multibyte text")
	}
}

func TestCheck_NormalizesUnverifiedSuffixAndWhitespace(t *testing.T) {
	valid := agents.BuildEffectiveValidBlocks(true, true)
	name := "  UK C30 TopBoost  "
	r := Check(envelope(t, name, "Amplifier", "Factory Neural Capture.", map[string]string{"name": "Gain", "value": "+1.0 dB"}), valid)
	if r.CaptureClaimOnModeled != 1 || r.CaptureDBOnModeled != 1 {
		t.Fatalf("whitespace must not mask defects: %+v", r)
	}
}

func TestClaimsCapture_ContractionsAndNotOnly(t *testing.T) {
	if claimsCapture("This isn't a Neural Capture, it is algorithmic.") || claimsCapture("It doesn't behave like a Factory Capture.") {
		t.Error("contractions must count as denial")
	}
	if !claimsCapture("Not only an amp, this is a Neural Capture of the original.") {
		t.Error(`"not only" must not read as a denial`)
	}
}

func TestCheck_EmptyBlockListsAreParseErrors(t *testing.T) {
	valid := agents.BuildEffectiveValidBlocks(true, true)
	for _, in := range []string{`{"structured_payload":{"guitars":{"G":[]}}}`, `{"structured_payload":{"guitars":{"G":null}}}`} {
		if r := Check(in, valid); r.ParseError == "" {
			t.Errorf("%s must be a parse error, not a clean 0-defect run", in)
		}
	}
}

func TestClassifyCriticNote_ParameterNotesAreNotIdentityDisputes(t *testing.T) {
	if got := ClassifyCriticNote("Capture Gain must use relative dB, but the value is a plain number"); got != "capture_other" {
		t.Errorf("got %s", got)
	}
	valid := agents.BuildEffectiveValidBlocks(true, true)
	r := Check(envelope(t, "Iba Green", "Overdrive", "ok ⚠️ Critic: Capture Gain must use relative dB."), valid)
	if r.CriticCaptureContradictsMap != 0 || r.CriticByType["capture_other"] != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestAssessCalls_ZeroCallsIsNotClean(t *testing.T) {
	if ok, issues, _ := AssessCalls(nil, map[string]string{"12_architect": "m"}, nil); ok || len(issues) == 0 {
		t.Fatal("a run with no recorded model calls (mock mode) must not be integrity_ok")
	}
}

func TestDialOffsetOnModeled(t *testing.T) {
	valid := agents.BuildEffectiveValidBlocks(true, true)
	env := `{"structured_payload":{"guitars":{"G":[
	 {"id":"a","type":"Amplifier","model":"UK C30 TopBoost","rationale":"x","parameters":[
	   {"name":"Volume","type":"slider","value":"-2.0","value_b":"6.0"},
	   {"name":"Level","type":"slider","value":"-2.0 dB"},
	   {"name":"Gain","type":"slider","value":"4.5"}]},
	 {"id":"b","type":"Drive","model":"Brit Blues","rationale":"x","parameters":[{"name":"Level","type":"slider","value":"+3.5 dB"}]},
	 {"id":"c","type":"Drive","model":"Iba Green","rationale":"x","parameters":[{"name":"Volume","type":"slider","value":"+1.0 dB"}]}]}}}`
	r := Check(env, valid)
	if r.ParseError != "" {
		t.Fatal(r.ParseError)
	}
	// UK C30 Volume "-2.0" + Level "-2.0 dB" + Brit Blues Level "+3.5 dB" = 3; the plain 6.0/4.5 and
	// Iba Green (a real capture) are not counted.
	if r.DialOffsetOnModeled != 3 {
		t.Fatalf("DialOffsetOnModeled = %d, want 3 (%v)", r.DialOffsetOnModeled, r.Examples)
	}
	if r.CaptureDBOnModeled != 0 {
		t.Fatalf("narrow counter changed: %d", r.CaptureDBOnModeled)
	}
}
