package agents

import (
	"encoding/json"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// This file exists because cmd/judge_compare's blind pairwise LLM judge -- the only
// existing quality signal for comparing candidate models/configs -- is demonstrably
// unstable (see TODO.md's Pipeline Quality Work section, issue #68: re-judging identical
// files produced different verdicts, at least one verdict was internally self-contradictory,
// and the judge has twice hallucinated "fabricated" against real, verified data). Rather than
// only fixing the judge, this adds a judge-free, deterministic quality signal built entirely
// from checks this repo already trusts enough to run in production (fuzzy_matcher.go's
// Flag* functions, called from internal/api on every real generation). None of this requires
// an LLM call, ground-truth labels, or a rubric -- it answers "is this structurally/factually
// broken" as a cheap first pass, leaving "does this read well" to a judge.
//
// A `FlagUnitsEmbeddedInNumericFields` check (flagging e.g. "-3.0 dB" instead of -3.0) was
// tried and removed here: a live diagnostic run showed it firing on normal, expected
// formatting this pipeline already produces everywhere -- graphic-EQ dB sliders, Mix/Blend
// percentages, reverb Decay in seconds -- none of which are the actual regression TODO.md
// describes. That regression is specifically dB-relative formatting on one of the five
// capture-only parameter names (Gain/Bass/Mid/Treble/Volume) on a block confirmed to be
// algorithmic, not a capture -- which `FlagCaptureFormattingMismatches` below already checks,
// correctly gated on capture status. A blanket "no unit text anywhere" check has no such
// gate and mostly measures this pipeline's ordinary parameter formatting, not a defect.

// MechanicalQualityReport is a deterministic, judge-free quality signal for one generation's
// raw (pre-HTML) Architect+Critic JSON output (Orchestrator.LastArchitectJSON after a
// RunPipeline call). Every field except ParseError and TotalBlocks is a defect count -- lower
// is better, 0 across the board is a clean run.
type MechanicalQualityReport struct {
	// ParseError is non-empty if structured_payload didn't even parse as valid JSON in the
	// expected shape -- every other field is zero when this is set, since there was nothing
	// to check. A non-empty ParseError is itself the most serious possible finding.
	ParseError string

	TotalBlocks int // total effect blocks across all guitars, for normalizing rates

	UnverifiedBlocks            int // FlagUnverifiedStructuredBlocks: block name not found in coros_map.json/user_captures.json's combined ground truth
	CaptureFormattingMismatches int // FlagCaptureFormattingMismatches: relative-dB formatting on a confirmed-algorithmic (non-capture) block
	IncompleteCabinetBlocks     int // FlagIncompleteCabinetBlocks: Cabinet block missing Mic 1/Mic 2/Blend
	LeftoverValueRanges         int // FlagLeftoverValueRanges: a range ("10-15ms") left in place of one decisive value
	CriticIssues                int // count of "⚠️ Critic: " notes already baked into Rationale by applyCriticFindings before this ran
}

// TotalDefects sums every defect count into one headline number for a quick per-config
// comparison; the individual fields remain available for root-causing which check drove it.
func (r MechanicalQualityReport) TotalDefects() int {
	return r.UnverifiedBlocks + r.CaptureFormattingMismatches + r.IncompleteCabinetBlocks +
		r.LeftoverValueRanges + r.CriticIssues
}

// RunMechanicalQualityChecks parses rawArchitectJSON (Orchestrator.LastArchitectJSON) and runs
// every deterministic Flag* check against it, using validBlocks as the ground truth (typically
// BuildEffectiveValidBlocks's output, sourced from coros_map.json + user_captures.json).
//
// Check order matches the production call order in internal/api/handlers_preset.go and
// server.go (FlagCaptureFormattingMismatches, FlagIncompleteCabinetBlocks,
// FlagLeftoverValueRanges, FlagUnverifiedStructuredBlocks last) since
// FlagCaptureFormattingMismatches's doc comment requires running before
// FlagUnverifiedStructuredBlocks rewrites block names with capture-source annotations.
//
// Operates on a local copy of the parsed StructuredPreset -- unlike the production call sites,
// callers here are scoring a finished generation, not preparing one for display/save, so there
// is no reason to mutate anything the caller holds.
func RunMechanicalQualityChecks(rawArchitectJSON string, validBlocks map[string]bool) MechanicalQualityReport {
	var report MechanicalQualityReport

	clean := StripJSONFences(rawArchitectJSON)
	var envelope struct {
		StructuredPayload storage.StructuredPreset `json:"structured_payload"`
	}
	if err := json.Unmarshal([]byte(clean), &envelope); err != nil {
		report.ParseError = err.Error()
		return report
	}
	sp := envelope.StructuredPayload

	for _, blocks := range sp.Guitars {
		report.TotalBlocks += len(blocks)
		for i := range blocks {
			report.CriticIssues += strings.Count(blocks[i].Rationale, criticIssueNotePrefix)
		}
	}

	report.CaptureFormattingMismatches = FlagCaptureFormattingMismatches(&sp, validBlocks)
	report.IncompleteCabinetBlocks = FlagIncompleteCabinetBlocks(&sp)
	report.LeftoverValueRanges = FlagLeftoverValueRanges(&sp)
	report.UnverifiedBlocks = FlagUnverifiedStructuredBlocks(&sp, validBlocks)

	return report
}
