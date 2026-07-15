package agents

import (
	"encoding/json"
	"log"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// CriticIssue is one finding from the Preset Critic agent (see prompts/13_critic.md):
// a contradiction between a block's prose (builder_statement/rationale) and its own
// structured data, not something deterministically checkable the way FlagCaptureFormattingMismatches
// or FlagIncompleteCabinetBlocks are.
type CriticIssue struct {
	Guitar   string `json:"guitar"`
	BlockID  string `json:"block_id"`
	Issue    string `json:"issue"`
	Severity string `json:"severity"`
}

type criticResponse struct {
	Issues []CriticIssue `json:"issues"`
}

const criticIssueNotePrefix = "⚠️ Critic: "

// applyCriticFindings parses the Preset Critic's raw JSON response and appends each flagged
// issue to the matching block's Rationale in the Architect's own raw JSON, via the same
// envelope-parse/re-marshal pattern injectRenderedHTML uses. Must run BEFORE
// injectRenderedHTML: the read-only preview table renders Rationale, so applying critic
// notes first means they actually reach the surface users primarily look at -- unlike the
// API-layer FlagCaptureFormattingMismatches/FlagIncompleteCabinetBlocks warnings, which run
// after that table is already baked (a known, separately-tracked limitation, see TODO.md).
//
// Fails soft, never hard: the critic is advisory, not a gate. A malformed critic response,
// an unparseable Architect envelope, or a critic finding that doesn't match any real
// guitar/block (hallucinated block_id) is logged and skipped rather than blocking an
// otherwise-valid preset from being returned. criticRaw being empty or unparseable
// (including an ablation stub, which is not valid JSON) degrades to "no findings", not
// an error.
func applyCriticFindings(raw string, criticRaw string) string {
	var resp criticResponse
	if err := json.Unmarshal([]byte(stripJSONFences(criticRaw)), &resp); err != nil {
		log.Printf("[Preset Critic] response unparseable, proceeding without critic annotations: %v", err)
		return raw
	}
	if len(resp.Issues) == 0 {
		return raw
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stripJSONFences(raw)), &envelope); err != nil {
		log.Printf("[Preset Critic] Architect envelope unparseable, proceeding without critic annotations: %v", err)
		return raw
	}
	rawStructured, ok := envelope["structured_payload"]
	if !ok {
		return raw
	}
	var sp storage.StructuredPreset
	if err := json.Unmarshal(rawStructured, &sp); err != nil {
		log.Printf("[Preset Critic] structured_payload unparseable, proceeding without critic annotations: %v", err)
		return raw
	}

	applied := 0
	for _, issue := range resp.Issues {
		blocks, ok := sp.Guitars[issue.Guitar]
		if !ok {
			log.Printf("[Preset Critic] finding references unknown guitar %q, skipping", issue.Guitar)
			continue
		}
		for i := range blocks {
			if blocks[i].ID != issue.BlockID {
				continue
			}
			appendRationaleNote(&blocks[i], criticIssueNotePrefix+issue.Issue)
			applied++
			break // block IDs are unique within a guitar; stop scanning and don't double-apply
		}
	}
	if applied == 0 {
		return raw
	}

	updatedStructured, err := json.Marshal(sp)
	if err != nil {
		log.Printf("[Preset Critic] failed to remarshal structured_payload, proceeding without critic annotations: %v", err)
		return raw
	}
	envelope["structured_payload"] = updatedStructured

	out, err := json.Marshal(envelope)
	if err != nil {
		log.Printf("[Preset Critic] failed to remarshal envelope, proceeding without critic annotations: %v", err)
		return raw
	}
	return string(out)
}
