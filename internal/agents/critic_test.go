package agents

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApplyCriticFindings_AppliesToMatchingBlock(t *testing.T) {
	raw := `{"builder_statement": "test", "structured_payload": {"guitars": {"Tele": [
		{"id":"b1","type":"Drive","model":"Facial Fuzz","rationale":"Stacked rhythm tone.","parameters":[{"name":"Bypass","type":"toggle","value":"On"}]}
	]}}, "agent_impact": []}`

	criticRaw := `{"issues": [
		{"guitar": "Tele", "block_id": "b1", "issue": "Bypass stays On in Scene B despite builder_statement claiming pedals strip back.", "severity": "high"}
	]}`

	out := applyCriticFindings(raw, criticRaw)

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if !strings.Contains(out, "Critic:") {
		t.Errorf("expected the critic's note to appear in the output, got: %s", out)
	}
	if !strings.Contains(out, "Stacked rhythm tone.") {
		t.Errorf("expected the critic note to be appended to, not replace, the existing rationale, got: %s", out)
	}
}

func TestApplyCriticFindings_HandlesMarkdownFencedInputs(t *testing.T) {
	// Both the Architect envelope and the critic response can come back wrapped in ```json
	// fences despite structured-output enforcement (notably via the un-schema-constrained
	// Open-LLM branch). Before stripJSONFences was shared, a fenced Architect envelope made
	// this step silently no-op while injectRenderedHTML (which did strip fences) parsed the
	// same string fine -- so a real critic finding never reached the rendered table.
	raw := "```json\n" + `{"builder_statement": "test", "structured_payload": {"guitars": {"Tele": [
		{"id":"b1","type":"Drive","model":"Facial Fuzz","rationale":"Original.","parameters":[]}
	]}}, "agent_impact": []}` + "\n```"
	criticRaw := "```json\n" + `{"issues": [{"guitar": "Tele", "block_id": "b1", "issue": "prose contradicts data", "severity": "high"}]}` + "\n```"

	out := applyCriticFindings(raw, criticRaw)
	if !strings.Contains(out, "Critic:") {
		t.Errorf("expected the critic note to be applied even when both inputs were markdown-fenced, got: %s", out)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("expected valid (unfenced) JSON output, got parse error %v on: %s", err, out)
	}
}

func TestApplyCriticFindings_NoIssuesLeavesInputUnchanged(t *testing.T) {
	raw := `{"builder_statement": "test", "structured_payload": {"guitars": {"Tele": [
		{"id":"b1","type":"Drive","model":"Facial Fuzz","rationale":"Stacked rhythm tone.","parameters":[]}
	]}}, "agent_impact": []}`
	criticRaw := `{"issues": []}`

	out := applyCriticFindings(raw, criticRaw)
	if out != raw {
		t.Errorf("expected input to be returned unchanged when the critic finds nothing, got: %s", out)
	}
}

func TestApplyCriticFindings_UnknownGuitarOrBlockIDIsSkippedNotFatal(t *testing.T) {
	raw := `{"builder_statement": "test", "structured_payload": {"guitars": {"Tele": [
		{"id":"b1","type":"Drive","model":"Facial Fuzz","rationale":"r","parameters":[]}
	]}}, "agent_impact": []}`
	criticRaw := `{"issues": [
		{"guitar": "Strat", "block_id": "b1", "issue": "references a guitar that doesn't exist in this preset", "severity": "high"},
		{"guitar": "Tele", "block_id": "b99", "issue": "references a block id that doesn't exist", "severity": "high"}
	]}`

	out := applyCriticFindings(raw, criticRaw)
	if out != raw {
		t.Errorf("expected hallucinated guitar/block_id references to be skipped (no match to apply), got: %s", out)
	}
}

func TestApplyCriticFindings_MalformedCriticResponseDegradesGracefully(t *testing.T) {
	raw := `{"builder_statement": "test", "structured_payload": {"guitars": {"Tele": []}}, "agent_impact": []}`

	// The exact shape a skipped/ablated agent call returns (RunAgentSplit's
	// "Ablated Output for %s." stub) -- not valid JSON at all.
	criticRaw := "Ablated Output for Preset Critic."

	out := applyCriticFindings(raw, criticRaw)
	if out != raw {
		t.Errorf("expected an unparseable critic response to degrade to the original input unchanged, got: %s", out)
	}
}

func TestApplyCriticFindings_Idempotent(t *testing.T) {
	raw := `{"builder_statement": "test", "structured_payload": {"guitars": {"Tele": [
		{"id":"b1","type":"Drive","model":"Facial Fuzz","rationale":"r","parameters":[]}
	]}}, "agent_impact": []}`
	criticRaw := `{"issues": [{"guitar": "Tele", "block_id": "b1", "issue": "same issue twice", "severity": "high"}]}`

	once := applyCriticFindings(raw, criticRaw)
	twice := applyCriticFindings(once, criticRaw)

	if strings.Count(twice, "same issue twice") != 1 {
		t.Errorf("expected applying the same finding twice to not duplicate the note, got: %s", twice)
	}
}
