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
// raw (pre-HTML) Architect+Critic JSON output (Orchestrator.LastArchitectJSON() after a
// RunPipeline call). Every field except ParseError and TotalBlocks is a defect count -- lower
// is better, 0 across the board is a clean run.
//
// A non-empty ParseError means none of the checks ran at all -- there was nothing to check,
// not "nothing wrong was found" -- so every defect count is also 0 in that case. Callers MUST
// check ParseError before trusting TotalDefects()/IsClean() as a quality signal: a broken or
// truncated generation and a genuinely flawless one are otherwise indistinguishable by count
// alone (GSR finding on PR #84). IsClean() checks both for exactly this reason -- prefer it
// over a bare TotalDefects()==0 comparison.
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
// Meaningless on its own when ParseError is set -- see the struct's doc comment -- callers
// wanting a single pass/fail read should use IsClean() instead.
func (r MechanicalQualityReport) TotalDefects() int {
	return r.UnverifiedBlocks + r.CaptureFormattingMismatches + r.IncompleteCabinetBlocks +
		r.LeftoverValueRanges + r.CriticIssues
}

// IsClean reports whether this generation both parsed successfully and had zero defects --
// the safe, single-call replacement for a bare TotalDefects()==0 check, which would otherwise
// treat a ParseError (nothing was checked) the same as a genuinely flawless run.
func (r MechanicalQualityReport) IsClean() bool {
	return r.ParseError == "" && r.TotalDefects() == 0
}

// RunMechanicalQualityChecks parses rawArchitectJSON (Orchestrator.LastArchitectJSON()) and runs
// every deterministic Flag* check against it, using validBlocks as the ground truth (typically
// BuildEffectiveValidBlocks's output, sourced from coros_map.json + user_captures.json).
//
// Check order matches the production call order in internal/api/handlers_preset.go and
// server.go (FlagCaptureFormattingMismatches, FlagIncompleteCabinetBlocks,
// FlagLeftoverValueRanges, FlagUnverifiedStructuredBlocks last) since
// FlagCaptureFormattingMismatches's doc comment requires running before
// FlagUnverifiedStructuredBlocks rewrites block names with capture-source annotations.
//
// Operates on the freshly-unmarshaled StructuredPreset -- unlike the production call sites,
// which mutate a preset already held by a caller, this one owns the only reference to it
// (nothing else has seen rawArchitectJSON's parsed form yet), so there's no copy to make and
// no caller state to protect from the Flag* functions' in-place mutation.
func RunMechanicalQualityChecks(rawArchitectJSON string, validBlocks map[string]bool) MechanicalQualityReport {
	var report MechanicalQualityReport

	clean := StripJSONFences(rawArchitectJSON)
	var envelope struct {
		StructuredPayload *storage.StructuredPreset `json:"structured_payload"`
	}
	if err := json.Unmarshal([]byte(clean), &envelope); err != nil {
		report.ParseError = err.Error()
		return report
	}
	// A pointer (not a value) so a response missing structured_payload entirely, or
	// carrying it as an explicit JSON null, is distinguishable from a genuinely present-but-
	// empty payload -- both unmarshal a value-typed field to the same zero value, which would
	// otherwise report a broken/truncated generation as a flawless "0 defects" run (GSR/
	// CodeRabbit finding on PR #84: a value type here can't tell "nothing to check" apart
	// from "there was nothing here to check in the first place").
	if envelope.StructuredPayload == nil {
		report.ParseError = "structured_payload missing or null"
		return report
	}
	sp := envelope.StructuredPayload

	for _, blocks := range sp.Guitars {
		report.TotalBlocks += len(blocks)
		for i := range blocks {
			report.CriticIssues += strings.Count(blocks[i].Rationale, criticIssueNotePrefix)
		}
	}

	report.CaptureFormattingMismatches = FlagCaptureFormattingMismatches(sp, validBlocks)
	report.IncompleteCabinetBlocks = FlagIncompleteCabinetBlocks(sp)
	report.LeftoverValueRanges = FlagLeftoverValueRanges(sp)
	report.UnverifiedBlocks = FlagUnverifiedStructuredBlocks(sp, validBlocks)

	return report
}
