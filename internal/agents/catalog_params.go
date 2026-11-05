package agents

// Device-catalog parameters for the Architect, and the check of its output against them.
//
// The device catalog (internal/catalog) lists every block model's real parameters. This
// file turns that into two things:
//
//   - CatalogParamContext: a compact, per-model listing of the parameters of the blocks
//     chosen in a run, injected into the Architect's context (catalog-aware prompts, v6, only; see
//     architectUsesCatalogParams), so it can use the device's real names and ranges.
//   - FlagCatalogParamMismatches: a post-generation check that annotates a block's
//     Rationale when a parameter name, value, switch position or unit contradicts the
//     catalog. Like the other Flag* checks in fuzzy_matcher.go it never touches
//     Value/ValueB (see FlagCaptureFormattingMismatches for why).
//
// What the catalog says vs what the device displays: the catalog gives the raw bounds from
// the device's ModelRepo.xml. For 0..10 amp/pedal dials and for dB/Hz/ms/s/% parameters
// those match the convention presets already use, so they are checked. Where the display
// convention is unverified, the check is skipped instead of guessed: pans (L..R labels),
// unitless 0..1 floats, and bounds the catalog only gives as firmware symbols.

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// deviceParam is one user-facing parameter of a block, merged across every catalog model
// the block name can refer to (see catalog.Resolve).
type deviceParam struct {
	// Label is the name shown to the Architect: the catalog name, prefixed with its slot
	// ("MIC 1 LEVEL") when a model repeats the name once per mic or IR slot.
	Label string
	Key   string
	Units string
	// Min/Max are the widest numeric bounds across the merged models; nil when any of them
	// gives a symbolic (firmware) bound.
	Min, Max  *float64
	Default   *float64
	StepNames []string
	// MinOff/MaxOff: the device labels that end of the range "OFF".
	MinOff, MaxOff bool
	// Pan: the range ends are labelled L/R. How the device displays these is unverified.
	Pan bool
	// Switch: a discrete control (switch, rotary switch, toggle, combo box), with or
	// without named positions.
	Switch bool
	// Required: every model behind the block name has this control, so a catalog-aware
	// Architect must account for it (set it or keep its default). A control only some
	// members have -- a stereo cab's BALANCE where the mono one has PAN -- isn't required.
	Required bool
}

func (p *deviceParam) numericRange() bool { return p.Min != nil && p.Max != nil }

// unitless01 reports a unitless 0..1 float whose display convention is unverified.
func (p *deviceParam) unitless01() bool {
	return p.Units == "" && !p.Switch && p.numericRange() && *p.Min == 0 && *p.Max == 1
}

// rangeCheckable reports whether a numeric value can be compared with Min/Max.
func (p *deviceParam) rangeCheckable() bool {
	return p.numericRange() && !p.Pan && !p.unitless01()
}

// zeroToTenDial is the 0..10 unitless knob amps and pedals use.
func (p *deviceParam) zeroToTenDial() bool {
	return p.Units == "" && !p.Switch && p.numericRange() && *p.Min == 0 && *p.Max == 10 && !p.Pan
}

// deviceBlock is the merged parameter list for one block name.
type deviceBlock struct {
	Name     string
	Category string
	// Plugin/Gated mirror catalog.Model.RequiredPlugin for gating (any gated member gates).
	Plugin string
	Gated  bool
	Params []*deviceParam
	byKey  map[string]*deviceParam
}

// cab reports a cabinet-style block with per-mic slots.
func (b *deviceBlock) cab() bool { return strings.HasPrefix(b.Category, "Cabsim") }

// hasMicSlots reports whether the block has "MIC n" slot parameters (cabs, and a cab-sim
// inside a pedal such as Parallax).
func (b *deviceBlock) hasMicSlots() bool {
	_, ok := b.byKey["MIC1LEVEL"]
	return ok
}

func (b *deviceBlock) labels() []string {
	out := make([]string, len(b.Params))
	for i, p := range b.Params {
		out[i] = p.Label
	}
	return out
}

// internalParamNames are catalog parameters that aren't user controls: the per-slot
// bypass/phase switches and the IR file index. Lower-case in the device's own XML.
var internalParamNames = map[string]bool{"bypass": true, "phi": true, "ir selector": true}

// internalParamTypes are read-only meters and placeholders.
var internalParamTypes = map[string]bool{"string": true, "grMeter": true, "empty": true}

func isInternalParam(p catalog.Param) bool {
	return internalParamTypes[p.Type] || internalParamNames[p.Name]
}

var interLetterHyphen = regexp.MustCompile(`([A-Z])-([A-Z])`)

// paramNameKey normalizes a parameter name for comparison: upper case, a hyphen between
// two letters dropped ("PRE-DELAY" = "PRE DELAY"), spaces and underscores dropped ("65 Hz"
// = "65Hz"). Other punctuation is kept, so "OCT +1" and "OCT -1" stay distinct.
// TestParamNameKeyHasNoCollisions checks that no two parameters of one model share a key.
func paramNameKey(name string) string {
	s := strings.ToUpper(strings.TrimSpace(name))
	s = interLetterHyphen.ReplaceAllString(s, "$1$2")
	return paramKeyStripper.Replace(s)
}

// paramKeyStripper drops the separators paramNameKey ignores. Built once: a Replacer
// compiles its lookup table at construction.
var paramKeyStripper = strings.NewReplacer(" ", "", "_", "", "\t", "")

// slotPrefix is how a repeated per-slot parameter is labelled: "MIC 1 LEVEL" on cabs,
// "IR 1 LEVEL" on dual IR loaders.
func slotPrefix(category string) string {
	if category == "IRLoaders" {
		return "IR"
	}
	return "MIC"
}

func normalizeUnits(u string) string {
	u = strings.TrimSpace(u)
	switch strings.ToLower(u) {
	case "cents":
		return "cents"
	}
	return u
}

// buildDeviceBlock merges the parameters of every model a name resolves to.
func buildDeviceBlock(name string, models []*catalog.Model) *deviceBlock {
	b := &deviceBlock{Name: name, Category: models[0].Category, byKey: map[string]*deviceParam{}}
	modelsWith := map[string]int{}
	for _, m := range models {
		if plugin, gated := m.RequiredPlugin(); gated {
			b.Gated = true
			if b.Plugin == "" {
				b.Plugin = plugin
			}
		}
		total := map[string]int{}
		for _, p := range m.Params {
			if !isInternalParam(p) {
				total[p.Name]++
			}
		}
		seen := map[string]int{}
		inModel := map[string]bool{}
		for _, p := range m.Params {
			if isInternalParam(p) {
				continue
			}
			seen[p.Name]++
			label := p.Name
			if total[p.Name] > 1 {
				label = fmt.Sprintf("%s %d %s", slotPrefix(m.Category), seen[p.Name], p.Name)
			}
			b.merge(label, p)
			inModel[paramNameKey(label)] = true
		}
		for k := range inModel {
			modelsWith[k]++
		}
	}
	for _, p := range b.Params {
		p.Required = modelsWith[p.Key] == len(models)
	}
	return b
}

func (b *deviceBlock) merge(label string, p catalog.Param) {
	key := paramNameKey(label)
	steps := make([]string, 0, len(p.StepNames))
	for _, s := range p.StepNames {
		steps = append(steps, strings.TrimSpace(s))
	}
	dp, ok := b.byKey[key]
	if !ok {
		dp = &deviceParam{
			Label: label, Key: key, Units: normalizeUnits(p.Units),
			Min: p.Min, Max: p.Max, Default: p.Default, StepNames: steps,
			MinOff: strings.EqualFold(p.MinLabel, "OFF"), MaxOff: strings.EqualFold(p.MaxLabel, "OFF"),
			Pan:    isPanLabel(p.MinLabel) || isPanLabel(p.MaxLabel),
			Switch: discreteParamTypes[p.Type],
		}
		b.byKey[key] = dp
		b.Params = append(b.Params, dp)
		return
	}
	// Widest range: a value is only out of range if it's out for every model.
	if dp.Min == nil || p.Min == nil {
		dp.Min = nil
	} else if *p.Min < *dp.Min {
		v := *p.Min
		dp.Min = &v
	}
	if dp.Max == nil || p.Max == nil {
		dp.Max = nil
	} else if *p.Max > *dp.Max {
		v := *p.Max
		dp.Max = &v
	}
	for _, s := range steps {
		if !containsFold(dp.StepNames, s) {
			dp.StepNames = append(dp.StepNames, s)
		}
	}
	dp.MinOff = dp.MinOff || strings.EqualFold(p.MinLabel, "OFF")
	dp.MaxOff = dp.MaxOff || strings.EqualFold(p.MaxLabel, "OFF")
	dp.Pan = dp.Pan || isPanLabel(p.MinLabel) || isPanLabel(p.MaxLabel)
}

// discreteParamTypes are the catalog parameter types with discrete positions.
var discreteParamTypes = map[string]bool{"switch": true, "rotarySwitch": true, "toggleButton": true, "comboBox": true}

func isPanLabel(s string) bool { return s == "L" || s == "R" }

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(s)) {
			return true
		}
	}
	return false
}

var (
	deviceBlocksOnce sync.Once
	deviceBlocks     map[string]*deviceBlock
)

// deviceBlockFor returns the merged parameters for a block name that is a catalog model
// name or mono/stereo family name. The catalog's generic "Neural Capture" slots are left
// out: captures have no catalog parameters.
func deviceBlockFor(name string) (*deviceBlock, bool) {
	deviceBlocksOnce.Do(func() {
		deviceBlocks = map[string]*deviceBlock{}
		cat, err := catalog.Embedded()
		if err != nil {
			log.Printf("device catalog unavailable, no catalog parameter checks: %v", err)
			return
		}
		deviceBlocks = buildDeviceBlocks(cat)
	})
	b, ok := deviceBlocks[strings.TrimSpace(name)]
	return b, ok
}

func buildDeviceBlocks(cat *catalog.Catalog) map[string]*deviceBlock {
	out := map[string]*deviceBlock{}
	for i := range cat.Models {
		m := &cat.Models[i]
		if m.Category == catalogCaptureCategory {
			continue
		}
		for _, name := range []string{m.Name, catalog.FamilyName(m.Name)} {
			if _, done := out[name]; done {
				continue
			}
			if models := cat.Resolve(name); len(models) > 0 {
				out[name] = buildDeviceBlock(name, models)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------------------
// Architect context

// maxCatalogContextModels bounds how many models' parameters are injected, so an
// unexpectedly long Librarian list can't blow up the Architect's context.
const maxCatalogContextModels = 16

func formatNum(x float64) string { return strconv.FormatFloat(x, 'g', -1, 64) }

// renderDeviceParam is one parameter in the compact form documented in the v6 prompt:
//
//	GAIN 0..10 d5            numeric range, default 5
//	OUTPUT -60..12dB min=OFF d0
//	TRAILS {Off|On} d=Off    switch positions
//	LEVEL dB range?          the device's bounds aren't known (firmware symbols)
//	PAN L..R                 a pan; its display scale isn't known
//	GRID MODE switch 0..1 d1 a switch the device gives no position names for
//	DISTANCE 0..1?           unitless 0..1; display scale isn't known
func renderDeviceParam(p *deviceParam) string {
	var s strings.Builder
	s.WriteString(p.Label)
	switch {
	case len(p.StepNames) > 0:
		s.WriteString(" {" + strings.Join(p.StepNames, "|") + "}")
		if p.Default != nil && p.Min != nil {
			if i := int(math.Round(*p.Default - *p.Min)); i >= 0 && i < len(p.StepNames) {
				s.WriteString(" d=" + p.StepNames[i])
			}
		}
		return s.String()
	case p.Pan:
		s.WriteString(" L..R")
		return s.String()
	case p.Switch && p.numericRange():
		s.WriteString(" switch " + formatNum(*p.Min) + ".." + formatNum(*p.Max))
		if p.Default != nil {
			s.WriteString(" d" + formatNum(*p.Default))
		}
		return s.String()
	case !p.numericRange():
		if p.Units != "" {
			s.WriteString(" " + p.Units)
		}
		s.WriteString(" range?")
		return s.String()
	case p.unitless01():
		s.WriteString(" 0..1?")
		return s.String()
	}
	s.WriteString(" " + formatNum(*p.Min) + ".." + formatNum(*p.Max) + p.Units)
	if p.MinOff {
		s.WriteString(" min=OFF")
	}
	if p.MaxOff {
		s.WriteString(" max=OFF")
	}
	if p.Default != nil {
		s.WriteString(" d" + formatNum(*p.Default))
	}
	return s.String()
}

func renderDeviceBlock(b *deviceBlock) string {
	parts := make([]string, len(b.Params))
	for i, p := range b.Params {
		parts[i] = renderDeviceParam(p)
	}
	return b.Name + " [" + b.Category + "]: " + strings.Join(parts, "; ")
}

// CatalogParamContext renders the device parameters of the given block names, one line per
// model, for the Architect. Names that aren't catalog models (captures, unknown or invented
// names), captures that share a catalog name (validBlocks marks them; a coros_map.json
// capture always wins over a catalog name), and plugin devices the gate doesn't allow are
// skipped. Duplicates are dropped, order is kept, and at most maxCatalogContextModels
// models are listed. Returns "" when nothing resolves.
func CatalogParamContext(names []string, validBlocks map[string]bool, gate PluginGate) string {
	return renderDeviceBlocks(catalogParamBlocks(names, validBlocks, gate))
}

// catalogParamBlocks resolves CatalogParamContext's block list.
func catalogParamBlocks(names []string, validBlocks map[string]bool, gate PluginGate) []*deviceBlock {
	var blocks []*deviceBlock
	seen := map[string]bool{}
	for _, raw := range names {
		name := stripCaptureAnnotation(raw)
		if isCap, known := validBlocks[name]; known && isCap {
			continue
		}
		b, ok := deviceBlockFor(name)
		if !ok || seen[b.Name] {
			continue
		}
		if b.Gated && (b.Plugin == "" || !gate.Allows(b.Plugin)) {
			continue
		}
		seen[b.Name] = true
		blocks = append(blocks, b)
		if len(blocks) == maxCatalogContextModels {
			break
		}
	}
	return blocks
}

func renderDeviceBlocks(blocks []*deviceBlock) string {
	lines := make([]string, len(blocks))
	for i, b := range blocks {
		lines[i] = renderDeviceBlock(b)
	}
	return strings.Join(lines, "\n")
}

// effectParamCategories are the effect categories whose every model's controls are given to
// the Architect, chosen or not, so a delay or reverb it adds or swaps in (Rule 3) has its real
// controls listed too. In the 2026-11-05 v5-vs-v6 eval, 79% of v6's leftover device notes were
// a reverb or delay it added without having been shown its controls (a Spring, a Slapback
// Delay); the chosen blocks' own controls are listed already. Kept to these two categories
// because each one costs tokens on every run (~8 KB for delays, ~4.5 KB for reverbs with the
// shipped plugin config).
var effectParamCategories = []string{"Delay", "Reverb"}

// CatalogEffectParamContext renders the controls of every model, one line per family, in the
// effectParamCategories, for the plugins the gate allows. Models CatalogParamContext already
// lists for the same names are left out. Returns "" when nothing is left.
func CatalogEffectParamContext(names []string, validBlocks map[string]bool, gate PluginGate) string {
	listed := map[string]bool{}
	for _, b := range catalogParamBlocks(names, validBlocks, gate) {
		listed[b.Name] = true
	}
	cat, err := catalog.Embedded()
	if err != nil {
		return ""
	}
	var blocks []*deviceBlock
	for _, c := range effectParamCategories {
		families := map[string]bool{}
		for i := range cat.Models {
			if m := &cat.Models[i]; m.Category == c {
				families[catalog.FamilyName(m.Name)] = true
			}
		}
		names := make([]string, 0, len(families))
		for f := range families {
			names = append(names, f)
		}
		sort.Strings(names)
		for _, f := range names {
			b, ok := deviceBlockFor(f)
			if !ok || listed[b.Name] {
				continue
			}
			if b.Gated && (b.Plugin == "" || !gate.Allows(b.Plugin)) {
				continue
			}
			listed[b.Name] = true
			blocks = append(blocks, b)
		}
	}
	return renderDeviceBlocks(blocks)
}

// effectListCategories are the catalog categories offered by name in the effects list
// (Architect Rule 3 lets it swap a suggested effect for another). Amps, cabs and drives are
// the Librarian's job; FX loops, IR loaders, loopers, mixers, splitters and the synth aren't
// tone choices.
var effectListCategories = []string{"Delay", "Reverb", "Modulation", "Compressor", "Pitch", "Wah", "Filter", "Equalizer", "Utility", "Morph"}

// CatalogEffectNames lists the real effect block names by category, one line each, for
// plugins the gate allows. Family names only (the "(M)"/"(ST)" choice is the device's).
func CatalogEffectNames(gate PluginGate) string {
	cat, err := catalog.Embedded()
	if err != nil {
		return ""
	}
	byCat := map[string]map[string]bool{}
	for i := range cat.Models {
		m := &cat.Models[i]
		if plugin, gated := m.RequiredPlugin(); gated && (plugin == "" || !gate.Allows(plugin)) {
			continue
		}
		if byCat[m.Category] == nil {
			byCat[m.Category] = map[string]bool{}
		}
		byCat[m.Category][catalog.FamilyName(m.Name)] = true
	}
	var lines []string
	for _, c := range effectListCategories {
		names := make([]string, 0, len(byCat[c]))
		for n := range byCat[c] {
			names = append(names, n)
		}
		if len(names) == 0 {
			continue
		}
		sort.Strings(names)
		lines = append(lines, c+": "+strings.Join(names, ", "))
	}
	return strings.Join(lines, "\n")
}

// LibrarianBlockNames returns the block names the Librarian chose, for CatalogParamContext.
// It reads mandatory_blocks, suggested_blocks and (v1 prompt) verified_blocks from its JSON.
// An entry that isn't itself a catalog name ("Green 808 for the lead boost") is scanned for
// catalog names as whole tokens; if the output isn't JSON at all, the whole text is.
func LibrarianBlockNames(librarianResult string) []string {
	var out struct {
		Mandatory []string `json:"mandatory_blocks"`
		Suggested []string `json:"suggested_blocks"`
		Verified  []string `json:"verified_blocks"`
	}
	entries := []string{librarianResult}
	if err := json.Unmarshal([]byte(StripJSONFences(librarianResult)), &out); err == nil {
		entries = append(append(append([]string{}, out.Mandatory...), out.Suggested...), out.Verified...)
	}
	var names []string
	for _, e := range entries {
		e = stripCaptureAnnotation(e)
		if _, ok := deviceBlockFor(e); ok {
			names = append(names, e)
			continue
		}
		names = append(names, catalogNamesIn(e)...)
	}
	return names
}

// payloadBlockModels returns the block model names in a saved preset payload, for
// RefineChat's CatalogParamContext. It reads the {structured, legacy_html} envelope or a
// bare StructuredPreset; an older HTML-only payload yields nothing (scanning HTML prose for
// short names like "Room" or "Gain" would list blocks the preset doesn't have). Guitars are
// visited in name order so the listing is stable.
func payloadBlockModels(payload string) []string {
	var envelope struct {
		Structured storage.StructuredPreset `json:"structured"`
	}
	sp := storage.StructuredPreset{}
	if err := json.Unmarshal([]byte(payload), &envelope); err == nil && len(envelope.Structured.Guitars) > 0 {
		sp = envelope.Structured
	} else if err := json.Unmarshal([]byte(payload), &sp); err != nil {
		return nil
	}
	guitars := make([]string, 0, len(sp.Guitars))
	for g := range sp.Guitars {
		guitars = append(guitars, g)
	}
	sort.Strings(guitars)
	var names []string
	for _, g := range guitars {
		for _, b := range sp.Guitars[g] {
			names = append(names, b.Model)
		}
	}
	return names
}

// catalogNamesIn finds catalog block names in free text as whole tokens, longest first, so
// "Spring (ST)" wins over "Spring" at the same place. Order of appearance is kept.
func catalogNamesIn(text string) []string {
	deviceBlockFor("") // populate deviceBlocks
	type hit struct {
		name string
		at   int
	}
	var hits []hit
	for name := range deviceBlocks {
		if name == "" {
			continue
		}
		// Every occurrence, not just the first: the first one may sit inside a longer name
		// ("Plate" in "Plate Lush") while a later one stands alone.
		for _, i := range tokenIndices(text, name) {
			hits = append(hits, hit{name, i})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].at != hits[j].at {
			return hits[i].at < hits[j].at
		}
		if len(hits[i].name) != len(hits[j].name) {
			return len(hits[i].name) > len(hits[j].name)
		}
		return hits[i].name < hits[j].name
	})
	var out []string
	seen := map[string]bool{}
	coveredUntil := -1
	for _, h := range hits {
		if h.at < coveredUntil {
			continue // inside a longer name already taken
		}
		coveredUntil = h.at + len(h.name)
		if !seen[h.name] {
			seen[h.name] = true
			out = append(out, h.name)
		}
	}
	return out
}

// tokenIndices returns every position where needle appears in haystack as a whole token
// (no letter or digit right before or after it), in order.
func tokenIndices(haystack, needle string) []int {
	isWordChar := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
	var out []int
	for start := 0; needle != "" && start <= len(haystack)-len(needle); {
		idx := strings.Index(haystack[start:], needle)
		if idx < 0 {
			break
		}
		at, end := start+idx, start+idx+len(needle)
		before, after := true, true
		if at > 0 {
			r, _ := utf8.DecodeLastRuneInString(haystack[:at])
			before = !isWordChar(r)
		}
		if end < len(haystack) {
			r, _ := utf8.DecodeRuneInString(haystack[end:])
			after = !isWordChar(r)
		}
		if before && after {
			out = append(out, at)
		}
		start = at + 1
	}
	return out
}

// tokenIndex is the first of tokenIndices, or -1.
func tokenIndex(haystack, needle string) int {
	if hits := tokenIndices(haystack, needle); len(hits) > 0 {
		return hits[0]
	}
	return -1
}

// ---------------------------------------------------------------------------------------
// Validation

// CatalogParamCounts is what FlagCatalogParamMismatches found. Each count is parameters
// (or parameter values), not blocks, except CheckedBlocks/UncheckedBlocks.
type CatalogParamCounts struct {
	UnknownNames    int // a parameter name the model doesn't have
	OutOfRange      int // a numeric value outside the device's known range
	BadSwitchValues int // a switch value that isn't one of the device's positions
	UnitMismatches  int // a unit that contradicts the device control
	BasisOverclaims int // basis "confirmed_range" where the device's range isn't known
	// CabConvention counts cab parameters from the old Mic 1/Mic 2/Blend convention the
	// device has no control for (Blend). Separate from UnknownNames so an old-vs-v6
	// comparison can tell a convention change from a real wrong knob.
	CabConvention int
	// UnrecognizedBlocks counts non-capture blocks whose name is neither a device block nor
	// in the user's libraries and wasn't already flagged unverified -- in practice invented
	// effect names, which the gear-only unverified check doesn't cover.
	UnrecognizedBlocks int
	// UnconsideredParams counts required device controls a block neither sets in
	// parameters nor names in kept_defaults. Only checked in a preset that uses the
	// kept_defaults convention (any block carries the list), i.e. catalog-aware output.
	UnconsideredParams int
	// CheckedBlocks resolved in the catalog and were checked; UncheckedBlocks didn't
	// (captures, unverified or invented names). Information, not defects.
	CheckedBlocks   int
	UncheckedBlocks int
}

// Defects is the total of the defect counts (not Checked/UncheckedBlocks).
func (c CatalogParamCounts) Defects() int {
	return c.UnknownNames + c.OutOfRange + c.BadSwitchValues + c.UnitMismatches + c.BasisOverclaims + c.CabConvention + c.UnrecognizedBlocks + c.UnconsideredParams
}

const deviceCheckPrefix = "⚠️ Device check: "

// deviceElementNames are real parts of the device that aren't grid blocks, so the block
// catalog doesn't list them: the input block (with its built-in noise gate), the output
// block and the global EQ. Compared lower-cased.
var deviceElementNames = map[string]bool{"input gate": true, "input": true, "output": true, "global eq": true}

// paramNameAliases are spellings the Architect uses for a control the device names
// differently. An alias only applies when its target exists on that model.
var paramNameAliases = map[string][]string{
	"TIME":    {"DELAYTIME"},
	"LOWCUT":  {"HPF", "HIGHPASS", "HIPASS", "HPFFREQ"},
	"HIGHCUT": {"LPF", "LOWPASS", "LPFFREQ"},
}

// lookupParam finds the device parameter a written name refers to.
func (b *deviceBlock) lookupParam(written string) (*deviceParam, bool) {
	key := paramNameKey(written)
	if p, ok := b.byKey[key]; ok {
		return p, true
	}
	for _, alias := range paramNameAliases[key] {
		if p, ok := b.byKey[alias]; ok {
			return p, true
		}
	}
	return nil, false
}

// valuePattern is a single number with an optional unit: "5", "-6.5 dB", "250ms", "18 %".
var valuePattern = regexp.MustCompile(`^\s*([+-]?\d+(?:\.\d+)?)\s*([A-Za-z%]*)\s*$`)

// unitDimension maps a written or catalog unit to its dimension and the factor to its base
// unit (Hz, s), so "0.25 s" compares with a "ms" range. Unknown units return ok=false.
func unitDimension(u string) (dim string, factor float64, ok bool) {
	switch strings.ToLower(strings.TrimSpace(u)) {
	case "db":
		return "level", 1, true
	case "hz":
		return "frequency", 1, true
	case "khz":
		return "frequency", 1000, true
	case "ms":
		return "time", 0.001, true
	case "s", "sec", "secs", "seconds":
		return "time", 1, true
	case "%":
		return "percent", 1, true
	}
	return "", 0, false
}

// FlagCatalogParamMismatches checks every block whose model resolves in the device catalog
// against that model's real parameters, appends a "⚠️ Device check:" note to the block's
// Rationale for each problem, and returns the counts. Value/ValueB are never changed.
//
// Skipped blocks: those already flagged unverified, captures (validBlocks marks them; they
// have no catalog parameters), names not in validBlocks (when validBlocks is non-nil), and
// names the catalog doesn't know. A skipped name that isn't in validBlocks and wasn't
// flagged unverified is counted (and noted) as UnrecognizedBlocks.
//
// Always accepted names: "Bypass" on any block (a block on/off state, not a catalog
// parameter), and "Mic 1"/"Mic 2" on blocks with mic slots (the mic model choice, which
// the device makes through its internal IR selector). "Blend" on a cab is counted as
// CabConvention: the device sets each mic's LEVEL instead.
//
// Run it after FlagUnverifiedStructuredBlocks, so it sees snapped model names and skips
// unverified ones. A dB value on Gain/Bass/Mid/Treble/Volume of a gear block is
// FlagCaptureFormattingMismatches's finding; this check skips that parameter so it isn't
// reported twice.
func FlagCatalogParamMismatches(sp *storage.StructuredPreset, validBlocks map[string]bool) CatalogParamCounts {
	var counts CatalogParamCounts
	if sp == nil {
		return counts
	}
	keptDefaultsConvention := false
	for _, blocks := range sp.Guitars {
		for i := range blocks {
			if blocks[i].KeptDefaults != nil {
				keptDefaultsConvention = true
			}
		}
	}
	for _, blocks := range sp.Guitars {
		for i := range blocks {
			b := &blocks[i]
			// An unverified block's name carries unverifiedSuffix, so it never resolves below.
			if strings.TrimSpace(b.Model) == "" {
				counts.UncheckedBlocks++
				continue
			}
			name := stripCaptureAnnotation(b.Model)
			if validBlocks != nil {
				isCap, known := validBlocks[name]
				if !known && !strings.HasSuffix(b.Model, unverifiedSuffix) && !isSkippableValue(name) &&
					!deviceElementNames[strings.ToLower(name)] {
					// resolveBlockName only flags unknown names in gear categories, so an
					// invented effect name ("Plate Reverb") would otherwise pass unseen while
					// a real one ("Plate") gets its parameters checked.
					counts.UnrecognizedBlocks++
					appendRationaleNote(b, fmt.Sprintf(deviceCheckPrefix+"%q isn't a block on the device or in your libraries; its settings can't be checked.", name))
				}
				if !known || isCap {
					counts.UncheckedBlocks++
					continue
				}
			}
			dev, ok := deviceBlockFor(name)
			if !ok {
				counts.UncheckedBlocks++
				continue
			}
			counts.CheckedBlocks++
			checkBlockParams(b, dev, &counts)
			if keptDefaultsConvention {
				checkKeptDefaults(b, dev, &counts)
			}
		}
	}
	return counts
}

func checkBlockParams(b *storage.EffectBlock, dev *deviceBlock, counts *CatalogParamCounts) {
	blockType := strings.ToLower(strings.TrimSpace(b.Type))
	var unknown []string
	for _, p := range b.Parameters {
		key := paramNameKey(p.Name)
		switch {
		case key == "BYPASS":
			continue
		case dev.hasMicSlots() && (key == "MIC1" || key == "MIC2"):
			if p.Basis == "confirmed_range" {
				counts.BasisOverclaims++
				appendRationaleNote(b, fmt.Sprintf(deviceCheckPrefix+"%s is tagged confirmed_range, but the device's mic list isn't in its catalog.", p.Name))
			}
			continue
		case dev.cab() && key == "BLEND":
			counts.CabConvention++
			appendRationaleNote(b, deviceCheckPrefix+"the device has no Blend control on a cab; set MIC 1 LEVEL and MIC 2 LEVEL instead.")
			continue
		}
		dp, ok := dev.lookupParam(p.Name)
		if !ok {
			counts.UnknownNames++
			unknown = append(unknown, p.Name)
			continue
		}
		if p.Basis == "confirmed_range" && !dp.rangeCheckable() && len(dp.StepNames) == 0 {
			counts.BasisOverclaims++
			appendRationaleNote(b, fmt.Sprintf(deviceCheckPrefix+"%s is tagged confirmed_range, but the device's range for it isn't known.", p.Name))
		}
		if captureEligibleBlockTypes[blockType] && captureOnlyParamNames[strings.ToLower(strings.TrimSpace(p.Name))] &&
			(relativeDBPattern.MatchString(strings.TrimSpace(p.Value)) || relativeDBPattern.MatchString(strings.TrimSpace(p.ValueB))) {
			continue // FlagCaptureFormattingMismatches reports this one
		}
		for _, v := range []string{p.Value, p.ValueB} {
			checkParamValue(b, p, dp, v, counts)
		}
	}
	if len(unknown) > 0 {
		labels := dev.labels()
		if len(labels) > 12 {
			labels = append(labels[:12:12], "…")
		}
		verb := "isn't a control"
		if len(unknown) > 1 {
			verb = "aren't controls"
		}
		appendRationaleNote(b, fmt.Sprintf(deviceCheckPrefix+"%s %s on the device's %s (its controls: %s).",
			strings.Join(unknown, ", "), verb, dev.Name, strings.Join(labels, ", ")))
	}
}

// checkKeptDefaults checks a block's kept_defaults against the device: every name must be
// one of the block's controls, and every required control must be either set in
// parameters or kept at its default, so leaving a control alone is a recorded choice.
func checkKeptDefaults(b *storage.EffectBlock, dev *deviceBlock, counts *CatalogParamCounts) {
	covered := map[string]bool{}
	for _, p := range b.Parameters {
		if dp, ok := dev.lookupParam(p.Name); ok {
			covered[dp.Key] = true
		}
	}
	var unknown []string
	for _, name := range b.KeptDefaults {
		dp, ok := dev.lookupParam(name)
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		covered[dp.Key] = true
	}
	if len(unknown) > 0 {
		counts.UnknownNames += len(unknown)
		appendRationaleNote(b, fmt.Sprintf(deviceCheckPrefix+"kept_defaults names %s, which the device's %s doesn't have.", strings.Join(unknown, ", "), dev.Name))
	}
	var missing []string
	for _, p := range dev.Params {
		if p.Required && !covered[p.Key] {
			missing = append(missing, p.Label)
		}
	}
	if len(missing) == 0 {
		return
	}
	counts.UnconsideredParams += len(missing)
	shown := missing
	if len(shown) > 12 {
		shown = append(shown[:12:12], "…")
	}
	appendRationaleNote(b, fmt.Sprintf(deviceCheckPrefix+"not set and not marked as kept at the device default: %s.", strings.Join(shown, ", ")))
}

func checkParamValue(b *storage.EffectBlock, p storage.BlockParameter, dp *deviceParam, value string, counts *CatalogParamCounts) {
	v := strings.TrimSpace(value)
	if v == "" {
		return
	}
	m := valuePattern.FindStringSubmatch(v)
	if len(dp.StepNames) > 0 {
		if m != nil {
			return // a numeric switch value may be a position index; not checked
		}
		if !containsFold(dp.StepNames, v) {
			counts.BadSwitchValues++
			appendRationaleNote(b, fmt.Sprintf(deviceCheckPrefix+"%s %q isn't a device position (%s).", p.Name, v, strings.Join(dp.StepNames, "|")))
		}
		return
	}
	if m == nil {
		return // text (a dropdown choice, "OFF", a range left for FlagLeftoverValueRanges): not checked
	}
	x, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return
	}
	written := m[2]
	if written == "" {
		written = strings.TrimSpace(p.Unit)
	}
	if written != "" {
		wDim, wFactor, wOK := unitDimension(written)
		if !wOK {
			return // a unit this check doesn't know: neither unit nor range is checked
		}
		if dp.Units == "" {
			if dp.zeroToTenDial() && wDim != "percent" {
				counts.UnitMismatches++
				appendRationaleNote(b, fmt.Sprintf(deviceCheckPrefix+"%s is written in %s, but the device control is a unitless 0..10 dial.", p.Name, written))
			}
			return // % on a unitless control, or a unit on another unitless range: not compared
		}
		cDim, cFactor, cOK := unitDimension(dp.Units)
		if !cOK {
			return
		}
		if cDim != wDim {
			counts.UnitMismatches++
			appendRationaleNote(b, fmt.Sprintf(deviceCheckPrefix+"%s is written in %s, but the device control is in %s.", p.Name, written, dp.Units))
			return
		}
		x = x * wFactor / cFactor
	} else if _, _, known := unitDimension(dp.Units); dp.Units != "" && !known {
		return // a bare number on a Semitones/BPM/... control: its convention isn't checked
	}
	if !dp.rangeCheckable() {
		return
	}
	const eps = 1e-9
	if x < *dp.Min-eps || x > *dp.Max+eps {
		counts.OutOfRange++
		appendRationaleNote(b, fmt.Sprintf(deviceCheckPrefix+"%s %s is outside the device's %s..%s%s range.", p.Name, v, formatNum(*dp.Min), formatNum(*dp.Max), dp.Units))
	}
}
