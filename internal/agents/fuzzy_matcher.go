package agents

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

var validNativeBlocks = make(map[string]bool)
var validBlocksRunes = make(map[string][]rune)
var parseBlocksOnce sync.Once
var validCategories = map[string]bool{
	"amplifier:": true, "cab:": true, "cabinet:": true,
	"overdrive:": true, "distortion:": true, "fuzz:": true, "drive:": true,
	"reverb:": true, "delay:": true, "modulation:": true,
	"pitch:": true, "filter:": true, "eq:": true,
	"utility:": true, "wah:": true, "volume:": true,
	"compressor:": true, "preamp:": true,
}
var blockCorrectionRegex = regexp.MustCompile(`(?i)(<td[^>]*>)([A-Za-z0-9\s/]+:\s*)([^<]+)`)

// gearBlockTypes are the block categories checked against the Dictionary/Capture
// Library before flagging a name as unverified. Scoped to amp/cab/drive-family
// categories deliberately: coros_map.json's coverage of non-gear categories (reverb,
// delay, gate, EQ, ...) is real but sparse (e.g. only 4 distinct native reverb names
// across 12 entries) — it's a gear-translation table, not a canonical list of every
// native QC block name. Flagging against it for those categories produced false
// positives on common, legitimate native names ("Spring Reverb", "Noise Gate") that
// simply aren't covered. Known tradeoff: this also means a genuinely fabricated name
// in one of those categories won't be caught. Accepted because the fabrication risk
// this guardrail targets — inventing a fake "capture" — is structurally an amp/cab/
// drive-only problem (RULE 11 already forbids treating time-based effects as captures).
var gearBlockTypes = map[string]bool{
	"amplifier": true, "cab": true, "cabinet": true,
	"overdrive": true, "distortion": true, "fuzz": true, "drive": true,
	"preamp": true,
}

// UserCapture represents a single Cortex Cloud capture the user has personally
// downloaded, hand-curated into user_captures.json.
type UserCapture struct {
	Name        string `json:"name"`
	BlockType   string `json:"block_type"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

var userCaptures []UserCapture
var parseUserCapturesOnce sync.Once

// GetUserCaptures parses the embedded user_captures.json, returning the user's
// personal library of downloaded Cortex Cloud captures.
func GetUserCaptures() []UserCapture {
	parseUserCapturesOnce.Do(func() {
		_ = json.Unmarshal(embeddedUserCaptures, &userCaptures)
	})
	return userCaptures
}

var userCapturesJSONCache string
var marshalUserCapturesOnce sync.Once

// GetUserCapturesJSON returns GetUserCaptures() pre-marshaled to JSON, cached after the
// first call — the library is static embedded data, so there's no reason to re-encode
// the same ~KB blob on every single pipeline run.
func GetUserCapturesJSON() string {
	marshalUserCapturesOnce.Do(func() {
		userCapturesJSONCache = "[]"
		if b, err := json.Marshal(GetUserCaptures()); err == nil {
			userCapturesJSONCache = string(b)
		}
	})
	return userCapturesJSONCache
}

var userCaptureNameSet map[string]bool
var parseUserCaptureNamesOnce sync.Once

// IsUserCaptureName reports whether name is an exact entry in the user's personal
// capture library, as distinct from a coros_map.json factory capture — both are
// captures, but the UI and the allow/favor toggles need to tell them apart.
func IsUserCaptureName(name string) bool {
	parseUserCaptureNamesOnce.Do(func() {
		userCaptureNameSet = make(map[string]bool)
		for _, c := range GetUserCaptures() {
			if c.Name != "" {
				userCaptureNameSet[c.Name] = true
			}
		}
	})
	return userCaptureNameSet[name]
}

// GetValidNativeBlocks parses the embedded coros_map.json and user_captures.json,
// returning a map of [blockName]isCapture. User captures are always treated as
// captures since they are, by definition, real Cortex Cloud captures.
func GetValidNativeBlocks() map[string]bool {
	parseBlocksOnce.Do(func() {
		var corosData map[string]map[string]interface{}
		if err := json.Unmarshal(embeddedCorosMap, &corosData); err == nil {
			for _, props := range corosData {
				if equiv, ok := props["coros_equivalent"].(string); ok && equiv != "" {
					isCap, _ := props["is_capture"].(bool)
					validNativeBlocks[equiv] = isCap
					validBlocksRunes[equiv] = []rune(strings.ToLower(equiv))
				}
			}
		}
		for _, c := range GetUserCaptures() {
			if c.Name == "" {
				continue
			}
			validNativeBlocks[c.Name] = true
			validBlocksRunes[c.Name] = []rune(strings.ToLower(c.Name))
		}
	})
	return validNativeBlocks
}

// BuildEffectiveValidBlocks filters GetValidNativeBlocks() by which capture sources are
// currently allowed for this generation. Native (non-capture) blocks always pass
// through unfiltered. coros_map.json's own factory captures and the user's personal
// user_captures.json library are gated independently — allowFactoryCaptures and
// allowUserCaptures — even though GetValidNativeBlocks() represents both under the same
// is_capture=true flag. Without this distinction, disallowing factory captures would
// also strip every real user capture out of the valid set, causing a correctly-chosen
// user capture to be flagged "Unverified" purely because it shares the isCapture flag
// with the factory captures being excluded.
func BuildEffectiveValidBlocks(allowFactoryCaptures, allowUserCaptures bool) map[string]bool {
	all := GetValidNativeBlocks()
	effective := make(map[string]bool, len(all))
	for name, isCap := range all {
		if IsUserCaptureName(name) {
			if allowUserCaptures {
				effective[name] = isCap
			}
			continue
		}
		if isCap && !allowFactoryCaptures {
			continue
		}
		effective[name] = isCap
	}
	return effective
}

// ApplyFuzzyCorrection iterates over HTML table rows and corrects block names.
func ApplyFuzzyCorrection(jsonStr string, validBlocks map[string]bool) string {
	re := blockCorrectionRegex
	corrected := re.ReplaceAllStringFunc(jsonStr, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) == 4 {
			prefix := sub[1] + sub[2]
			key := strings.ToLower(strings.TrimSpace(sub[2]))

			if !validCategories[key] {
				return match
			}

			resolved := resolveBlockName(sub[3], strings.TrimSuffix(key, ":"), validBlocks)
			return prefix + resolved
		}
		return match
	})
	return corrected
}

// resolveBlockName is the single place the verification policy lives, shared by
// ApplyFuzzyCorrection (HTML draft view) and FlagUnverifiedStructuredBlocks (saved
// structured payload) so both paths always agree: snap to the closest real Dictionary/
// Capture Library entry, label it by source if it's a capture, or flag it as unverified
// if it's an unrecognized gear-category name nothing came close to.
func resolveBlockName(rawName, category string, validBlocks map[string]bool) string {
	name := stripCaptureAnnotation(rawName)
	if isSkippableValue(name) {
		return name
	}

	snapped := SnapToClosestBlock(name, validBlocks)

	// Comma-ok is required here: a missing key and a known-but-non-capture entry both
	// zero-value to `false` on a plain map read, which previously let a name that snapped
	// to a real but *disallowed* (filtered-out) capture pass through unflagged.
	if isCap, known := validBlocks[snapped]; known {
		if isCap {
			return annotateCaptureSource(snapped)
		}
		return snapped
	}

	// Nothing in the Dictionary or the user's Capture Library — as allowed by the
	// caller's validBlocks — matched closely enough. Only gear-type categories are
	// checked; native block categories (reverb, gate, EQ, ...) have no dictionary
	// entries to match against by design.
	if gearBlockTypes[category] {
		return FlagUnverifiedBlock(snapped)
	}

	return snapped
}

// annotateCaptureSource labels a verified capture name with which library it came from,
// so the UI always shows both the exact real name and whether it's a Neural DSP factory
// capture or one of the user's own downloaded Cortex Cloud captures, rather than a
// generic "(Capture)" suffix that doesn't distinguish the two.
func annotateCaptureSource(name string) string {
	if IsUserCaptureName(name) {
		return name + " (My Capture)"
	}
	return name + " (Factory Capture)"
}

// stripCaptureAnnotation removes a trailing capture-annotation suffix the model
// sometimes writes into a block name itself — "(Capture)", "(My Capture)",
// "[Factory Capture]", etc. — mirroring what this file adds programmatically once a
// name is verified. Without stripping it first, a real, valid capture name showing up
// pre-annotated would fail to match its bare dictionary entry and get incorrectly
// flagged as unverified. Requires an actual bracket/paren/dash delimiter before
// "capture", not just trailing whitespace — some real capture names legitimately end in
// the bare word "Capture" (e.g. "JM Default Capture" in this library), and a delimiter-
// free match would truncate those.
var captureAnnotationSuffix = regexp.MustCompile(`(?i)\s*[(\[-]\s*(?:my |factory )?capture\s*[)\]]?\s*$`)

func stripCaptureAnnotation(s string) string {
	trimmed := strings.TrimSpace(s)
	return strings.TrimSpace(captureAnnotationSuffix.ReplaceAllString(trimmed, ""))
}

const unverifiedSuffix = " ⚠️ (Unverified — not in Dictionary or your Capture Library)"

// FlagUnverifiedBlock marks a block name that could not be matched against the
// Dictionary or the user's Capture Library, so the UI surfaces a warning instead of
// silently presenting a possibly-fabricated name as real. Idempotent: a chat-refinement
// turn that echoes an already-flagged name back unchanged (SnapToClosestBlock can't snap
// the long annotated string to anything real, so resolveBlockName reaches here again)
// must not compound the warning text turn over turn.
func FlagUnverifiedBlock(name string) string {
	if strings.HasSuffix(name, unverifiedSuffix) {
		return name
	}
	return name + unverifiedSuffix
}

// FlagUnverifiedStructuredBlocks walks a StructuredPreset's effect blocks and applies
// the same resolveBlockName policy ApplyFuzzyCorrection applies to the HTML draft view,
// so a fabricated block name doesn't survive a preset save unflagged, and a real capture
// is always labeled with its source and shown under its exact name.
func FlagUnverifiedStructuredBlocks(sp *storage.StructuredPreset, validBlocks map[string]bool) {
	if sp == nil {
		return
	}
	for _, blocks := range sp.Guitars {
		for i := range blocks {
			if strings.TrimSpace(blocks[i].Model) == "" {
				continue
			}
			blockType := strings.ToLower(strings.TrimSpace(blocks[i].Type))
			blocks[i].Model = resolveBlockName(blocks[i].Model, blockType, validBlocks)
		}
	}
}

// IgnoreList contains structural block names that shouldn't be snapped to amplifiers/effects.
var IgnoreList = map[string]bool{
	"Noise Gate":       true,
	"Adaptive Gate":    true,
	"Global Gate":      true,
	"Global Input":     true,
	"Input / Impedance": true,
	"Input: Global Impedance": true,
	"Gate: Noise Gate": true,
	"Lane 1 Output":    true,
	"Lane Output":      true,
	"Input":            true,
	"Gate":             true,
}

// LevenshteinDistance calculates the minimum string edits to go from s to t.
func LevenshteinDistance(sRunes, tRunes []rune) int {
	m := len(sRunes)
	n := len(tRunes)

	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}

	d := make([]int, n+1)
	for j := 0; j <= n; j++ {
		d[j] = j
	}

	for i := 1; i <= m; i++ {
		prev := i
		for j := 1; j <= n; j++ {
			cost := 1
			if sRunes[i-1] == tRunes[j-1] {
				cost = 0
			}
			cur := min(
				d[j]+1,        // deletion
				prev+1,        // insertion
				d[j-1]+cost,   // substitution
			)
			d[j-1] = prev
			prev = cur
		}
		d[n] = prev
	}
	return d[n]
}

func min(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

var categorizedAmpsWithCapturesCache string
var categorizedAmpsWithoutCapturesCache string
var parseAmpsOnce sync.Once

// GetCategorizedAmplifiers reads the embedded JSON and creates a formatted Markdown menu
// grouping all available distinct amplifier names by their tonal archetype for LLM injection.
func GetCategorizedAmplifiers(allowFactoryCaptures bool) string {
	parseAmpsOnce.Do(func() {
		var corosData map[string]map[string]interface{}
		bucketsWithCaptures := make(map[string]map[string]bool)
		bucketsWithoutCaptures := make(map[string]map[string]bool)

		if err := json.Unmarshal(embeddedCorosMap, &corosData); err == nil {
			for _, props := range corosData {
				if t, ok := props["type"].(string); ok && t == "amp" {
					if equiv, ok := props["coros_equivalent"].(string); ok && equiv != "" {
						arch, _ := props["tonal_archetype"].(string)
						if arch == "" {
							arch = "Other / Unique"
						}
						isCap, _ := props["is_capture"].(bool)

						if bucketsWithCaptures[arch] == nil {
							bucketsWithCaptures[arch] = make(map[string]bool)
						}
						bucketsWithCaptures[arch][equiv] = true

						if !isCap {
							if bucketsWithoutCaptures[arch] == nil {
								bucketsWithoutCaptures[arch] = make(map[string]bool)
							}
							bucketsWithoutCaptures[arch][equiv] = true
						}
					}
				}
			}
		}

		buildMenu := func(buckets map[string]map[string]bool) string {
			var sb strings.Builder
			sb.WriteString("=== AVAILABLE AMPLIFIER ARCHETYPES ===\n")
			for archetype, amps := range buckets {
				sb.WriteString(fmt.Sprintf("\n%s:\n", strings.ToUpper(archetype)))
				for amp := range amps {
					sb.WriteString(fmt.Sprintf("- %s\n", amp))
				}
			}
			return sb.String()
		}

		categorizedAmpsWithCapturesCache = buildMenu(bucketsWithCaptures)
		categorizedAmpsWithoutCapturesCache = buildMenu(bucketsWithoutCaptures)
	})

	if allowFactoryCaptures {
		return categorizedAmpsWithCapturesCache
	}
	return categorizedAmpsWithoutCapturesCache
}

// isSkippableValue reports whether a value is a structural UI element or an obviously
// non-block input (a parameter like "-3.0dB", or a status like "Bypassed") that should
// never be snapped or flagged as an unverified block name.
func isSkippableValue(s string) bool {
	return IgnoreList[s] || len(s) < 3 || strings.Contains(s, "dB") || strings.Contains(s, "%") || strings.Contains(s, "ms") || strings.Contains(s, "Hz") || s == "Bypassed" || s == "Active" || s == "Engaged"
}

// SnapToClosestBlock checks if the input is a valid block, else returns the closest equivalent.
func SnapToClosestBlock(input string, validBlocks map[string]bool) string {
	inputStr := strings.TrimSpace(input)

	if isSkippableValue(inputStr) {
		return inputStr
	}

	// Ensure the candidate pool (validBlocksRunes) is populated regardless of whether
	// the caller already called GetValidNativeBlocks() — this must not depend on call
	// order, since validBlocksRunes is a package-level cache separate from whatever
	// validBlocks map the caller passes in.
	GetValidNativeBlocks()

	bestDistance := math.MaxInt32
	bestMatch := inputStr

	inputRunes := []rune(strings.ToLower(inputStr))

	for v, vRunes := range validBlocksRunes {
		if strings.EqualFold(inputStr, v) {
			return v // Perfect case-insensitive match
		}

		// Length pruning: if the length difference is already >= bestDistance, it can't be better.
		diff := len(inputRunes) - len(vRunes)
		if diff < 0 {
			diff = -diff
		}
		if diff >= bestDistance {
			continue
		}

		dist := LevenshteinDistance(inputRunes, vRunes)
		if dist < bestDistance {
			bestDistance = dist
			bestMatch = v
		}
	}

	// Code review: Tighter threshold (len/3) to prevent aggressive warping.
	maxAllowedEdits := len(inputRunes) / 3
	if maxAllowedEdits < 2 {
		maxAllowedEdits = 2 // Allow at least 2 edits for very short strings
	}

	if bestDistance <= maxAllowedEdits {
		return bestMatch
	}

	return inputStr
}
