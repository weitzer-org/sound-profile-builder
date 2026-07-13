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

// capturedGear pairs a capture's resolved name with whatever descriptive color exists for
// it -- a user capture's free-text Description, or a factory capture's TonalArchetype.
type capturedGear struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

var factoryCaptureColors []capturedGear
var parseFactoryCaptureColorsOnce sync.Once

// getFactoryCaptureColors parses the embedded coros_map.json once, caching every
// is_capture=true entry's name+tonal_archetype -- the same static-embedded-data-gets-
// parsed-once pattern GetValidNativeBlocks/GetCategorizedAmplifiers already use in this
// file. SelectedCaptureContext originally re-unmarshaled the full file on every call.
func getFactoryCaptureColors() []capturedGear {
	parseFactoryCaptureColorsOnce.Do(func() {
		var fullMap map[string]map[string]interface{}
		if err := json.Unmarshal(embeddedCorosMap, &fullMap); err != nil {
			return
		}
		for _, props := range fullMap {
			isCap, _ := props["is_capture"].(bool)
			if !isCap {
				continue
			}
			equiv, _ := props["coros_equivalent"].(string)
			archetype, _ := props["tonal_archetype"].(string)
			if equiv != "" && archetype != "" {
				factoryCaptureColors = append(factoryCaptureColors, capturedGear{Name: equiv, Color: archetype})
			}
		}
	})
	return factoryCaptureColors
}

// SelectedCaptureContext scans the Librarian's and Navigator's raw output text (their names
// are never run through ApplyFuzzyCorrection/resolveBlockName -- only the Architect's final
// output is) for any known capture name and returns a compact JSON list of just the captures
// actually selected this run, each paired with its real descriptive color. Because the scan
// is a bare substring match against un-normalized text, a non-canonical spelling/casing in
// Agent 4/5's raw output will simply not match and this returns less than it ideally could --
// a silent miss, not a wrong answer (the Architect's own Rule 9 fallback covers the gap). The
// Architect is the one agent that formats final parameters for every block type per its
// Capture Parameters Mandate (Rule 9) -- amp, cab, drive, fuzz, boost, distortion, preamp,
// anything is_capture can tag -- but until this existed it only knew a given block WAS a
// capture, never anything about what made that specific capture distinctive. Scoped to
// whatever the upstream agents actually picked (rather than the full ~100-entry combined
// library) to keep this cheap: a handful of matches at most, not the whole catalog on
// every run.
//
// allowFactoryCaptures/allowUserCaptures mirror the same toggles already threaded through
// dictJSON/userCapturesJSON earlier in RunPipeline: when a source is disabled for this run,
// its captures were never available for the Librarian/Navigator to select in the first
// place, so scanning for them here too is both wasted work and a real (if narrow) leak --
// a disabled user capture's private Description could otherwise still reach the Architect's
// context via a coincidental substring match against unrelated text.
func SelectedCaptureContext(librarianResult, navigatorResult string, allowFactoryCaptures, allowUserCaptures bool) string {
	haystack := librarianResult + "\n" + navigatorResult
	var found []capturedGear

	if allowUserCaptures {
		for _, c := range GetUserCaptures() {
			if c.Name != "" && c.Description != "" && strings.Contains(haystack, c.Name) {
				found = append(found, capturedGear{Name: c.Name, Color: c.Description})
			}
		}
	}

	if allowFactoryCaptures {
		for _, c := range getFactoryCaptureColors() {
			if strings.Contains(haystack, c.Name) {
				found = append(found, c)
			}
		}
	}

	if len(found) == 0 {
		return ""
	}
	b, err := json.Marshal(found)
	if err != nil {
		return ""
	}
	return string(b)
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

const captureFormattingMismatchNote = "⚠️ Verify: %s formatted as relative dB, but this block isn't a confirmed capture (expected a plain 0-10 dial value)."

var relativeDBPattern = regexp.MustCompile(`(?i)^[+-]?\d+(\.\d+)?\s*dB$`)

// captureOnlyParamNames are the exact five parameter names Architect Rule 9 (Capture
// Parameters Mandate) restricts relative-dB formatting to: "Neural Captures natively
// feature a standardized set of parameters: Gain, Bass, Mid, Treble, and Volume." Checking
// against this list (rather than flagging any dB-shaped value on any gear-block parameter)
// avoids false-flagging a control that's legitimately dB-native regardless of capture
// status -- e.g. a Cabinet block's output Level trim -- which isn't one of the five names
// the rule actually governs.
var captureOnlyParamNames = map[string]bool{
	"gain": true, "bass": true, "mid": true, "treble": true, "volume": true,
}

// captureEligibleBlockTypes is gearBlockTypes plus "boost" -- Rule 9 explicitly names Boost
// alongside Amplifier/Cab/Drive/Fuzz/Distortion/Preamp as a block type captures commonly
// appear as, but coros_map.json has zero entries with a "boost" dictionary type (boost
// pedals are classified under "drive" there), so adding it to the shared gearBlockTypes
// would risk flagging every real Boost block as "Unverified" in resolveBlockName's
// name-matching path -- a different, unrelated check with its own established tradeoff.
// This set only feeds FlagCaptureFormattingMismatches's capture-vs-formatting check, which
// looks up the resolved name in validBlocks directly rather than requiring dictionary-type
// coverage, so the false-positive risk that keeps gearBlockTypes narrow doesn't apply here.
var captureEligibleBlockTypes = func() map[string]bool {
	m := make(map[string]bool, len(gearBlockTypes)+1)
	for k := range gearBlockTypes {
		m[k] = true
	}
	m["boost"] = true
	return m
}()

// FlagCaptureFormattingMismatches walks a StructuredPreset's gear blocks and flags any
// block where one of the five capture-only parameter names (see captureOnlyParamNames) is
// formatted as a relative dB adjustment but the block's resolved name is confirmed -- via
// validBlocks, the same ground-truth lookup FlagUnverifiedStructuredBlocks already uses --
// to be a real, verified NATIVE algorithmic model, not a capture. This is deterministic
// *detection*, not correction: a relative-dB offset and an absolute 0-10 dial position are
// different units with no formula between them, so there's no safe way to compute what the
// "corrected" number should be -- prompt wording alone (Architect Rule 9) has needed two
// rounds of tightening and still occasionally misapplies capture formatting to well-known
// algorithmic amps.
//
// The warning is appended to the block's Rationale, never to Parameter.Value/ValueB.
// Value/ValueB are the field the editable Tweaking Workspace UI treats as the live,
// re-saveable source of truth (handleUpdateParameter writes it back with no shape
// validation) -- annotating it risked a user silently persisting the warning text itself as
// the permanent parameter value on their next unrelated edit. Rationale is purely
// descriptive, already rendered in the read-only preview table, and never round-trips
// through numeric parsing anywhere.
//
// Must run BEFORE FlagUnverifiedStructuredBlocks in caller order for the capture-status
// lookup to be meaningful -- but does not depend on that order for correctness: it strips
// any capture-source annotation via stripCaptureAnnotation first, so a name already
// rewritten to "X (Factory Capture)"/"X (My Capture)" by a prior FlagUnverifiedStructuredBlocks
// pass still resolves back to a lookup-able key. Skips blocks whose (stripped) name isn't in
// validBlocks at all (unverified/fabricated names are FlagUnverifiedStructuredBlocks's
// concern, not this function's) and blocks confirmed to genuinely be captures (dB formatting
// is correct there).
func FlagCaptureFormattingMismatches(sp *storage.StructuredPreset, validBlocks map[string]bool) {
	if sp == nil {
		return
	}
	for _, blocks := range sp.Guitars {
		for i := range blocks {
			blockType := strings.ToLower(strings.TrimSpace(blocks[i].Type))
			if !captureEligibleBlockTypes[blockType] {
				continue
			}
			resolvedName := stripCaptureAnnotation(blocks[i].Model)
			isCapture, known := validBlocks[resolvedName]
			if !known || isCapture {
				continue
			}
			var mismatchedParams []string
			for j := range blocks[i].Parameters {
				p := blocks[i].Parameters[j]
				if !captureOnlyParamNames[strings.ToLower(strings.TrimSpace(p.Name))] {
					continue
				}
				if relativeDBPattern.MatchString(strings.TrimSpace(p.Value)) || relativeDBPattern.MatchString(strings.TrimSpace(p.ValueB)) {
					mismatchedParams = append(mismatchedParams, p.Name)
				}
			}
			if len(mismatchedParams) == 0 {
				continue
			}
			note := fmt.Sprintf(captureFormattingMismatchNote, strings.Join(mismatchedParams, "/"))
			if !strings.Contains(blocks[i].Rationale, note) {
				if blocks[i].Rationale != "" {
					blocks[i].Rationale += " " + note
				} else {
					blocks[i].Rationale = note
				}
			}
		}
	}
}

// IgnoreList contains structural block names that shouldn't be snapped to amplifiers/effects.
var IgnoreList = map[string]bool{
	"Noise Gate":              true,
	"Adaptive Gate":           true,
	"Global Gate":             true,
	"Global Input":            true,
	"Input / Impedance":       true,
	"Input: Global Impedance": true,
	"Gate: Noise Gate":        true,
	"Lane 1 Output":           true,
	"Lane Output":             true,
	"Input":                   true,
	"Gate":                    true,
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
				d[j]+1,      // deletion
				prev+1,      // insertion
				d[j-1]+cost, // substitution
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
