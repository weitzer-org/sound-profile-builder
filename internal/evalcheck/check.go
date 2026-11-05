// Package evalcheck is the deterministic, LLM-free checker for pipeline outputs
// (docs/eval/gemini-model-upgrade-test-plan.md §3 item 0.8). It reads the JSON envelope that
// RunPipeline returns (structured_payload + final_html_payload) and counts defects against the
// repo's own ground truth (coros_map.json / user_captures.json) -- never against an LLM's opinion.
//
// Design rule learned the hard way (TODO.md, "Thinking-budget eval infrastructure"): a blanket
// "no units in value fields" check is wrong. BlockParameter.Value is a free string by schema and
// the Architect prompt prescribes unit-bearing values (graphic-EQ dB, Mix %, Decay s ...), so
// ~300 unit-bearing values appear even in a clean baseline run. NonNumericValues is therefore
// reported as a REFERENCE number only; the defect counters are gated on ground truth.
package evalcheck

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Report is the per-output result.
type Report struct {
	ParseError string `json:"parse_error,omitempty"` // nothing below is meaningful if set
	Blocks     int    `json:"blocks"`

	// (i) Numeric-field discipline.
	NonNumericValues      int `json:"non_numeric_values"`       // REFERENCE ONLY: slider Value/ValueB that is not a bare number
	UnitBearingValues     int `json:"unit_bearing_values"`      // REFERENCE ONLY: subset: number followed by a unit (dB, Hz, ms, s, %)
	CaptureDBOnModeled    int `json:"capture_db_on_modeled"`    // DEFECT: Rule 9 five params in relative dB on a block the map says is NOT a capture (gear types)
	ModeledStyleOnCapture int `json:"modeled_style_on_capture"` // DEFECT: Rule 9 five params NOT in relative dB on a block the map says IS a capture (gear types)
	// REFERENCE ONLY (broader than CaptureDBOnModeled): any amp/drive-family dial (dialParamNames) on
	// a known non-capture block written as relative dB OR as a bare signed offset ("-2.0"). Added
	// after Phase 1B: the narrow counter missed "Level = +3.5 dB" on Brit Blues and "Volume = -2.0" /
	// invented "Level = -2.0 dB" on UK C30 TopBoost. Not gated, because some algorithmic controls may
	// be genuinely dB-native (see captureOnlyParamNames' comment) and that is not verified per block.
	DialOffsetOnModeled int `json:"dial_offset_on_modeled"`

	// (ii) Capture-vs-modeled contradictions.
	CaptureClaimOnModeled       int            `json:"capture_claim_on_modeled"`       // DEFECT (ground truth): rationale calls a known non-capture block a Neural/Factory Capture
	CriticFlags                 int            `json:"critic_flags"`                   // Critic notes already baked into rationales
	CriticByType                map[string]int `json:"critic_by_type"`                 // classified by keyword; see ClassifyCriticNote
	CriticCaptureContradictsMap int            `json:"critic_capture_contradicts_map"` // Critic said "not a capture" but the map says the block IS one (Critic probably wrong)

	// (iii) Invented block names (see agents.BlockNameStats for why not ApplyFuzzyCorrection).
	GearBlocks      int      `json:"gear_blocks"`
	SnappedNames    int      `json:"snapped_names"`
	UnverifiedNames int      `json:"unverified_names"`
	Snaps           []string `json:"snaps,omitempty"`
	UnverifiedList  []string `json:"unverified_list,omitempty"`

	// Examples is human-readable evidence for each defect, so a count can be audited.
	Examples []string `json:"examples,omitempty"`
}

// Defects is the sum of the ground-truth-gated defect counters (not the reference numbers).
func (r Report) Defects() int {
	return r.CaptureDBOnModeled + r.ModeledStyleOnCapture + r.CaptureClaimOnModeled + r.SnappedNames + r.UnverifiedNames
}

var (
	plainNumber = regexp.MustCompile(`^[+-]?\d+(\.\d+)?$`)
	numberUnit  = regexp.MustCompile(`^[+-]?\d+(\.\d+)?\s*[A-Za-z%°]+$`)
	// A deliberate claim that THIS block is a capture. Bare "capture" is not enough: it is also
	// a verb ("to capture warmth") and is used for other blocks ("pairs with the Hiwatt capture").
	captureClaim = regexp.MustCompile(`(?i)\b(?:factory|neural|pre-?trained|snapshot)(?:[ -]+(?:neural|factory|pre-?trained))*[ -]+capture\b`)
	negation     = regexp.MustCompile(`(?i)(?:\bnot\b|\bnor\b|n't\b|rather than|instead of|\bnon-?|unlike|\bno\b)[^.;]{0,40}$`)
	notOnly      = regexp.MustCompile(`(?i)\bnot only\b`)
	signedNumber = regexp.MustCompile(`^[+-]\d+(\.\d+)?$`)
	// dialParamNames are the 0-10 dial controls of the QC amp and drive vocabularies
	// (internal/agents/qc_block_schema.json "amplifier" and "drive").
	dialParamNames = map[string]bool{"gain": true, "volume": true, "bass": true, "mid": true, "treble": true,
		"presence": true, "master volume": true, "drive": true, "tone": true, "level": true}
)

// Check parses one output envelope and runs every check. validBlocks is typically
// agents.BuildEffectiveValidBlocks(true, true) -- used here purely as a lookup of ground truth,
// not as a gate on what the run was allowed to pick.
func Check(envelopeJSON string, validBlocks map[string]bool) Report {
	var r Report
	r.CriticByType = map[string]int{}
	var env struct {
		StructuredPayload *storage.StructuredPreset `json:"structured_payload"`
	}
	if err := json.Unmarshal([]byte(agents.StripJSONFences(envelopeJSON)), &env); err != nil {
		r.ParseError = err.Error()
		return r
	}
	if env.StructuredPayload == nil {
		r.ParseError = "structured_payload missing or null"
		return r
	}
	total := 0
	for _, blocks := range env.StructuredPayload.Guitars {
		total += len(blocks)
	}
	if total == 0 { // guitars absent, or present with only empty/null block lists
		r.ParseError = "structured_payload has no blocks"
		return r
	}
	sp := env.StructuredPayload

	for guitar, blocks := range sp.Guitars {
		for _, b := range blocks {
			r.Blocks++
			name := agents.NormalizeBlockName(b.Model)
			isCap, known := validBlocks[name]
			prose, notes := splitCriticNotes(b.Rationale)

			// (i)
			for _, p := range b.Parameters {
				for _, v := range []string{p.Value, p.ValueB} {
					v = strings.TrimSpace(v)
					if v == "" || p.Type != "slider" {
						continue
					}
					if !plainNumber.MatchString(v) {
						r.NonNumericValues++
						if numberUnit.MatchString(v) {
							r.UnitBearingValues++
						}
					}
				}
				// Cabinets are excluded: a cab's Level is plausibly a genuine dB output control
				// (unverified against the QC manual), and every cab hit in Phase 2 was that pattern.
				if known && !isCap && agents.IsCaptureEligibleBlockType(b.Type) && !strings.Contains(strings.ToLower(b.Type), "cab") && dialParamNames[strings.ToLower(strings.TrimSpace(p.Name))] && p.Type == "slider" {
					for _, v := range []string{p.Value, p.ValueB} {
						if v = strings.TrimSpace(v); v != "" && (agents.IsRelativeDB(v) || signedNumber.MatchString(v)) {
							r.DialOffsetOnModeled++
						}
					}
				}
				if known && agents.IsCaptureEligibleBlockType(b.Type) && agents.IsCaptureOnlyParamName(p.Name) && p.Type == "slider" {
					for _, v := range []string{p.Value, p.ValueB} {
						v = strings.TrimSpace(v)
						if v == "" {
							continue
						}
						if !isCap && agents.IsRelativeDB(v) {
							r.CaptureDBOnModeled++
							r.example("%s / %s: %s=%q is relative dB on a non-capture block", guitar, b.Model, p.Name, v)
						}
						if isCap && !agents.IsRelativeDB(v) {
							r.ModeledStyleOnCapture++
							r.example("%s / %s: %s=%q is not relative dB on a capture block", guitar, b.Model, p.Name, v)
						}
					}
				}
			}

			// (ii) ground-truth capture claim
			if known && !isCap && claimsCapture(prose) {
				r.CaptureClaimOnModeled++
				r.example("%s / %s: rationale calls it a capture but the map says it is not (%q)", guitar, b.Model, snippet(prose))
			}
			for _, n := range notes {
				r.CriticFlags++
				typ := ClassifyCriticNote(n)
				r.CriticByType[typ]++
				if typ == "capture_claim" && known && isCap {
					r.CriticCaptureContradictsMap++
					r.example("%s / %s: Critic disputes a capture claim but the map says this block IS a capture (%q)", guitar, b.Model, snippet(n))
				}
			}
		}
	}

	bn := agents.BlockNameStatsFor(sp, validBlocks)
	r.GearBlocks, r.SnappedNames, r.UnverifiedNames, r.Snaps = bn.GearBlocks, bn.Snapped, bn.Unverified, bn.Snaps
	r.UnverifiedList = bn.UnverifiedNames
	return r
}

func (r *Report) example(format string, args ...any) {
	if len(r.Examples) < 12 {
		r.Examples = append(r.Examples, sprintf(format, args...))
	}
}

// claimsCapture reports whether prose asserts (not denies) that the block is a Neural/Factory
// Capture.
func claimsCapture(prose string) bool {
	for _, loc := range captureClaim.FindAllStringIndex(prose, -1) {
		// negation is anchored at the end of the prefix and bounds its own look-back
		// ([^.;]{0,40}), so the whole prefix is passed: no byte-offset slicing that could split a
		// UTF-8 rune or chop a negation word in half.
		// "Not only an amp, this is a Neural Capture" asserts the claim: don't let "not only" read as a denial.
		if !negation.MatchString(notOnly.ReplaceAllString(prose[:loc[0]], "also")) {
			return true
		}
	}
	return false
}

// splitCriticNotes separates a rationale into the Architect's own prose and the appended
// "⚠️ Critic: ..." notes. Other "⚠️ Verify" warnings are dropped from both.
func splitCriticNotes(rationale string) (prose string, criticNotes []string) {
	parts := strings.Split(rationale, "⚠️")
	var pb strings.Builder
	pb.WriteString(parts[0])
	for _, p := range parts[1:] {
		t := strings.TrimSpace(p)
		switch {
		case strings.HasPrefix(t, "Critic:"):
			criticNotes = append(criticNotes, strings.TrimSpace(strings.TrimPrefix(t, "Critic:")))
		case strings.HasPrefix(t, "Verify"):
			// injected Verify warning: dropped from prose
		default:
			// the model's own use of the warning emoji: keep it in the prose
			pb.WriteString("⚠️")
			pb.WriteString(p)
		}
	}
	return strings.TrimSpace(pb.String()), criticNotes
}

// ClassifyCriticNote buckets a Critic finding by keyword. Order matters: the first rule that
// matches wins. It is deliberately simple and auditable; unmatched notes land in "other".
func ClassifyCriticNote(note string) string {
	n := strings.ToLower(note)
	switch {
	case strings.Contains(n, "capture") && disputesIdentity(n):
		return "capture_claim"
	case strings.Contains(n, "capture"):
		return "capture_other" // e.g. a parameter-format note on a capture; not a dispute of its identity
	case strings.Contains(n, "bypass"):
		return "bypass_state"
	case strings.Contains(n, "single coil") || strings.Contains(n, "single-coil") || strings.Contains(n, "humbucker") || strings.Contains(n, "guitar"):
		return "guitar_mismatch"
	case strings.Contains(n, "spring") || strings.Contains(n, "tape") || strings.Contains(n, "digital") || strings.Contains(n, "analog") || strings.Contains(n, "hall") || strings.Contains(n, "plate"):
		return "device_type"
	}
	return "other"
}

func sprintf(format string, args ...any) string { return fmt.Sprintf(format, args...) }

func snippet(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 110 {
		return string(r[:109]) + "…"
	}
	return s
}

// disputesIdentity reports whether a (lowercased) Critic note argues about WHAT the block is
// (native vs capture), as opposed to commenting on a capture's parameters.
func disputesIdentity(n string) bool {
	for _, w := range []string{"native", "algorithmic", "modeled", "modelled", "not a ", "isn't", "is not", "rather than", "instead of", "describes the block", "described as", "claims", "labeled", "labelled", "actually"} {
		if strings.Contains(n, w) {
			return true
		}
	}
	return false
}

// StripCriticNotes removes the "⚠️ Critic: ..." notes the pipeline's own Critic appended to a
// rationale, keeping the Architect's prose and any other warnings. The uniform re-critic (plan
// §6.2.1) feeds outputs back to a Critic with these removed, so it never sees another model's
// opinion.
func StripCriticNotes(rationale string) string {
	parts := strings.Split(rationale, "⚠️")
	var b strings.Builder
	b.WriteString(parts[0])
	for _, p := range parts[1:] {
		if strings.HasPrefix(strings.TrimSpace(p), "Critic:") {
			continue
		}
		b.WriteString("⚠️")
		b.WriteString(p)
	}
	return strings.TrimSpace(b.String())
}

// CriticNote is one "⚠️ Critic: ..." finding baked into an output, with the ground truth a human
// needs to rule on it (plan §7 Critic rule: of 30 sampled flags, how many are true contradictions).
type CriticNote struct {
	Guitar, BlockID, Model, Type string
	Note                         string
	Class                        string // ClassifyCriticNote
	KnownBlock                   bool   // the block name is in coros_map.json / user_captures.json
	MapSaysCapture               bool   // ground truth is_capture (meaningful only when KnownBlock)
}

// CriticNotes lists the pipeline Critic's notes in one output envelope.
func CriticNotes(envelopeJSON string, validBlocks map[string]bool) []CriticNote {
	var env struct {
		StructuredPayload *storage.StructuredPreset `json:"structured_payload"`
	}
	if err := json.Unmarshal([]byte(agents.StripJSONFences(envelopeJSON)), &env); err != nil || env.StructuredPayload == nil {
		return nil
	}
	var out []CriticNote
	for guitar, blocks := range env.StructuredPayload.Guitars {
		for _, b := range blocks {
			isCap, known := validBlocks[agents.NormalizeBlockName(b.Model)]
			_, notes := splitCriticNotes(b.Rationale)
			for _, n := range notes {
				out = append(out, CriticNote{Guitar: guitar, BlockID: b.ID, Model: b.Model, Type: b.Type, Note: n, Class: ClassifyCriticNote(n), KnownBlock: known, MapSaysCapture: isCap})
			}
		}
	}
	return out
}
