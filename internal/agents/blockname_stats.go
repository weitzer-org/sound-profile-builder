package agents

import (
	"strings"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// This file exposes read-only views of fuzzy_matcher.go / critic.go internals to the eval
// checker (internal/evalcheck) without changing any production behaviour.

// CriticNotePrefix is the marker applyCriticFindings puts in front of each Critic note it
// appends to a block's Rationale.
const CriticNotePrefix = criticIssueNotePrefix

// IsCaptureEligibleBlockType reports whether a block type (any case) is one where Architect
// Rule 9 allows capture-style relative-dB formatting: amp/cab/drive-family plus boost.
func IsCaptureEligibleBlockType(blockType string) bool {
	return captureEligibleBlockTypes[strings.ToLower(strings.TrimSpace(blockType))]
}

// IsCaptureOnlyParamName reports whether name is one of Rule 9's five capture parameters.
func IsCaptureOnlyParamName(name string) bool {
	return captureOnlyParamNames[strings.ToLower(strings.TrimSpace(name))]
}

// IsRelativeDB reports whether v looks like a relative-dB value such as "+2.0 dB".
func IsRelativeDB(v string) bool { return relativeDBPattern.MatchString(strings.TrimSpace(v)) }

// StripCaptureAnnotation is the exported form of stripCaptureAnnotation.
func StripCaptureAnnotation(s string) string { return stripCaptureAnnotation(s) }

// BlockNameStats counts, over gear-category blocks only (see gearBlockTypes for why other
// categories are out of scope), how the pipeline's verification policy treats each block name:
//
//	Exact       the name is already a key in validBlocks
//	Snapped     not exact, but SnapToClosestBlock would rewrite it to a real entry -- a proxy
//	            for "the model invented or misspelled a block name"
//	Unverified  nothing close enough exists; FlagUnverifiedBlock would mark it
//
// This exists because the plan's original proxy -- "names changed by ApplyFuzzyCorrection" --
// is always 0 on real pipeline output: ApplyFuzzyCorrection's regex needs a raw "<td", but the
// output envelope is JSON-marshalled with "<" escaped as <, so it never matches. Counting
// over structured_payload (the source of truth the HTML is rendered from) does not have that
// problem.
type BlockNameStats struct {
	GearBlocks      int
	Exact           int
	Snapped         int
	Unverified      int
	Snaps           []string // "typed -> resolved", for the report
	UnverifiedNames []string // names nothing came close to
}

// BlockNameStatsFor computes BlockNameStats for sp against validBlocks.
func BlockNameStatsFor(sp *storage.StructuredPreset, validBlocks map[string]bool) BlockNameStats {
	var st BlockNameStats
	if sp == nil {
		return st
	}
	for _, blocks := range sp.Guitars {
		for i := range blocks {
			if !gearBlockTypes[strings.ToLower(strings.TrimSpace(blocks[i].Type))] {
				continue
			}
			name := strings.TrimSpace(strings.TrimSuffix(stripCaptureAnnotation(blocks[i].Model), unverifiedSuffix))
			if name == "" || isSkippableValue(name) {
				continue
			}
			st.GearBlocks++
			if _, ok := validBlocks[name]; ok {
				st.Exact++
				continue
			}
			snapped := SnapToClosestBlock(name, validBlocks)
			if _, ok := validBlocks[snapped]; ok {
				st.Snapped++
				st.Snaps = append(st.Snaps, name+" -> "+snapped)
			} else {
				st.Unverified++
				st.UnverifiedNames = append(st.UnverifiedNames, name)
			}
		}
	}
	return st
}
