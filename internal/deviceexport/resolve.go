package deviceexport

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// builder collects one export's unresolved items.
type builder struct {
	e          *Exporter
	legacy     bool // the preset predates Architect v4's Bypass definition (see usesBasis)
	unresolved []devicejobs.Unresolved
}

func (b *builder) refuse(block, param, reason string) {
	b.unresolved = append(b.unresolved, devicejobs.Unresolved{Block: block, Param: truncate(param, 80), Reason: reason})
}

// ---- model resolution (plan section 4, rule 1) ----

// deviceElements are real parts of the device that aren't grid blocks: the input block
// (with its built-in gate), the output block and the global EQ. Compared lower-cased; the
// same list the pipeline's parameter check exempts.
var deviceElements = map[string]string{
	"input gate": "input and output block settings have no write path in v1",
	"input":      "input and output block settings have no write path in v1",
	"output":     "input and output block settings have no write path in v1",
	"global eq":  "the global EQ is a global setting; the bridge never writes global settings",
}

func isDeviceElement(model string) bool {
	_, ok := deviceElements[strings.ToLower(strings.TrimSpace(model))]
	return ok
}

func (b *builder) unresolvedElement(ref, model string) {
	b.refuse(ref, "", deviceElements[strings.ToLower(strings.TrimSpace(model))])
}

// captureAnnotation matches the "(capture)" / "[my capture]" / "- factory capture" suffix
// the pipeline puts on capture names (internal/agents' captureAnnotationSuffix).
var captureAnnotation = regexp.MustCompile(`(?i)\s*[(\[-]\s*(?:my |factory )?capture\s*[)\]]?\s*$`)

// internalModelIDs are the device's own capture-engine models (category "Neural Capture
// Internal": NC_Recorder 20000, NC_Trainer 20001, NC_Refiner 20002). The importer already
// drops that category; this is a second guard in case a future catalog keeps them.
var internalModelIDs = map[int]bool{20000: true, 20001: true, 20002: true}

// pushableCategories are the catalog categories v1 places. Anything else, including a
// category a future CorOS adds, is refused.
var pushableCategories = map[string]bool{
	"Guitar Amplifier": true, "Bass Amplifier": true, "Guitar Overdrive": true, "Bass Overdrive": true,
	"Cabsim Guitar (M)": true, "Cabsim Guitar (ST)": true, "Cabsim Bass (M)": true, "Cabsim Bass (ST)": true,
	"Compressor": true, "Equalizer": true, "Delay": true, "Reverb": true, "Modulation": true, "Pitch": true,
	"Wah": true, "Filter": true, "Utility": true, "Morph": true, "Synth": true,
}

// resolveModel picks the one device model a block name means, or says why it can't.
func (b *builder) resolveModel(name string) (*catalog.Model, string) {
	if name == "" {
		return nil, "the block has no model name"
	}
	if captureAnnotation.MatchString(name) || (b.e.opts.IsCapture != nil && b.e.opts.IsCapture(name)) {
		return nil, "captures and IRs are not pushed in v1"
	}
	ms := b.e.cat.Resolve(name)
	if len(ms) == 0 {
		return nil, "not a block on the device (no catalog model or mono/stereo family has this name)"
	}
	var m *catalog.Model
	if ms[0].Name == name {
		if len(ms) > 1 {
			ids := make([]string, len(ms))
			for i, x := range ms {
				ids[i] = strconv.Itoa(x.ID)
			}
			return nil, fmt.Sprintf("ambiguous: %d device models share this name (ids %s); use a name that picks one", len(ms), strings.Join(ids, ", "))
		}
		m = ms[0]
	} else {
		// A mono/stereo family name resolves to its mono member in v1.
		var mono []*catalog.Model
		for _, x := range ms {
			if strings.HasSuffix(x.Name, " (M)") {
				mono = append(mono, x)
			}
		}
		if len(mono) != 1 {
			return nil, fmt.Sprintf("family %q has %d mono models; name the model exactly", name, len(mono))
		}
		m = mono[0]
	}
	return m, b.modelRefusal(m)
}

// modelRefusal is why a resolved model can't be pushed, or "".
func (b *builder) modelRefusal(m *catalog.Model) string {
	switch {
	case internalModelIDs[m.ID] || strings.Contains(m.Category, "Internal") || m.Category == "Utility_Deprecated":
		return "internal device model; never placed"
	case m.Category == "Neural Capture" || m.Category == "IRLoaders":
		return "captures and IRs are not pushed in v1"
	case !pushableCategories[m.Category]:
		return fmt.Sprintf("%s blocks are not pushed in v1 (routing, I/O or looper blocks)", m.Category)
	case strings.HasSuffix(m.Name, "(S/C)"):
		return "side-chain models need routing; not pushed in v1"
	}
	if plugin, gated := m.RequiredPlugin(); gated {
		if plugin == "" {
			return fmt.Sprintf("plugin model with plugin id %q, which the plugin table doesn't know", m.PluginID)
		}
		if !b.e.opts.OwnedPlugins[plugin] {
			return fmt.Sprintf("needs the %s plugin, which isn't in config.json's available_plugins", plugin)
		}
	}
	if m.Clones != 0 {
		if _, ok := catalog.VerifiedCloneBases[m.Clones]; !ok {
			return fmt.Sprintf("parameter indexes of models cloning %d are not verified on the device (G1 checked only clones of %s)", m.Clones, verifiedBases())
		}
	}
	return ""
}

func verifiedBases() string {
	ids := make([]int, 0, len(catalog.VerifiedCloneBases))
	for id := range catalog.VerifiedCloneBases {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = strconv.Itoa(id)
	}
	return strings.Join(s, ", ")
}

func indexEvidence(m *catalog.Model) string {
	if m.Clones == 0 {
		return "plain"
	}
	return fmt.Sprintf("clone-%d-g1", m.Clones)
}

// ---- one block ----

// block maps one grid block. A block whose model can't be resolved is not placed (nil),
// but still takes its column.
func (b *builder) block(ref string, blk *storage.EffectBlock, col int) (*Block, error) {
	name := strings.TrimSpace(blk.Model)
	m, why := b.resolveModel(name)
	if why != "" {
		b.refuse(ref, "", why)
		return nil, nil
	}
	out := &Block{Col: col, ModelID: m.ID, Model: m.Name, Category: m.Category,
		IndexEvidence: indexEvidence(m), Params: []Param{}, AtDefault: []string{}}
	if plugin, gated := m.RequiredPlugin(); gated {
		out.Plugin = plugin
	}
	params := catalog.LabelParams(m)
	byKey := make(map[string]catalog.LabelledParam, len(params))
	hasMics := false
	for _, lp := range params {
		byKey[lp.Key] = lp
		if lp.Key == "MIC1LEVEL" {
			hasMics = true
		}
	}
	internal := map[string]bool{}
	for _, p := range m.Params {
		if catalog.IsInternalParam(p) {
			internal[catalog.ParamNameKey(p.Name)] = true
		}
	}
	written := map[int]string{}
	cab := strings.HasPrefix(m.Category, "Cabsim")
	for _, bp := range blk.Parameters {
		key := catalog.ParamNameKey(bp.Name)
		switch {
		case key == "BYPASS":
			b.bypass(ref, bp, &out.Bypass)
			continue
		case hasMics && (key == "MIC1" || key == "MIC2"):
			b.refuse(ref, bp.Name, "the mic choice is the cab's IR-name string parameter; writing it has not been measured")
			continue
		case cab && key == "BLEND":
			b.refuse(ref, bp.Name, "device cabs have no Blend control; set MIC 1 LEVEL and MIC 2 LEVEL instead")
			continue
		}
		lp, ok := lookup(byKey, key)
		if !ok {
			if internal[key] {
				b.refuse(ref, bp.Name, "internal device parameter, never written")
			} else {
				b.refuse(ref, bp.Name, fmt.Sprintf("not a control of %s", m.Name))
			}
			continue
		}
		if prev, dup := written[lp.Param.Index]; dup {
			b.refuse(ref, bp.Name, fmt.Sprintf("%s is written twice (as %q and %q)", lp.Label, truncate(prev, 40), truncate(bp.Name, 40)))
			continue
		}
		written[lp.Param.Index] = bp.Name
		if !m.ValidParamIndex(lp.Param.Index) {
			return nil, fmt.Errorf("catalog fault: %s (%d) param %s has index %d, outside its %d parameters",
				m.Name, m.ID, lp.Label, lp.Param.Index, m.ParamCount)
		}
		p, atDefault, why := b.param(lp, bp)
		switch {
		case why != "":
			b.refuse(ref, bp.Name, why)
		case atDefault:
			out.AtDefault = append(out.AtDefault, lp.Label)
		default:
			out.Params = append(out.Params, p)
		}
	}
	sort.Slice(out.Params, func(i, j int) bool { return out.Params[i].Index < out.Params[j].Index })
	return out, nil
}

func lookup(byKey map[string]catalog.LabelledParam, key string) (catalog.LabelledParam, bool) {
	if lp, ok := byKey[key]; ok {
		return lp, true
	}
	for _, alias := range catalog.ParamNameAliases[key] {
		if lp, ok := byKey[alias]; ok {
			return lp, true
		}
	}
	return catalog.LabelledParam{}, false
}

// bypass reads the block's Bypass parameter: "On" = bypassed, "Off" = active (Architect
// prompts v4-v6, Rule 13). Older presets never defined it, so theirs is refused.
func (b *builder) bypass(ref string, bp storage.BlockParameter, out *[Scenes]bool) {
	if b.legacy {
		b.refuse(ref, bp.Name, "this preset predates the rule that Bypass On means bypassed (Architect v4); refine it to push it")
		return
	}
	var ab [2]bool
	for i, v := range sceneStrings(bp) {
		switch strings.ToLower(cleanValue(v)) {
		case "on":
			ab[i] = true
		case "off":
			ab[i] = false
		default:
			b.refuse(ref, bp.Name, fmt.Sprintf("Bypass must be On (bypassed) or Off (active), got %q", truncate(v, 40)))
			return
		}
	}
	for s := range out {
		out[s] = ab[sceneIndex(s)]
	}
}

// sceneIndex maps a device scene (0-7) to the preset value it takes: 0 = A, 1 = B.
func sceneIndex(s int) int {
	if SceneSources[s] == "B" {
		return 1
	}
	return 0
}

// sceneStrings is the preset's Scene A and Scene B text; an empty ValueB means "same as A".
func sceneStrings(bp storage.BlockParameter) [2]string {
	b := bp.ValueB
	if strings.TrimSpace(b) == "" {
		b = bp.Value
	}
	return [2]string{bp.Value, b}
}

// cleanValue trims a written value and strips the "*" estimate marker (and a Markdown-
// escaped "\*") that renders put after estimated values.
func cleanValue(v string) string {
	v = strings.TrimSpace(v)
	for {
		switch {
		case strings.HasSuffix(v, `\*`):
			v = strings.TrimSpace(strings.TrimSuffix(v, `\*`))
		case strings.HasSuffix(v, "*"):
			v = strings.TrimSpace(strings.TrimSuffix(v, "*"))
		default:
			return v
		}
	}
}

// ---- one parameter (rules 2-5) ----

// param maps one written parameter. atDefault means every scene is the catalog default, so
// nothing needs writing.
func (b *builder) param(lp catalog.LabelledParam, bp storage.BlockParameter) (Param, bool, string) {
	cp := lp.Param
	vals := sceneStrings(bp)
	if strings.TrimSpace(vals[0]) == "" {
		return Param{}, false, "no value"
	}
	if catalog.DiscreteParamTypes[cp.Type] {
		return discrete(cp, vals)
	}
	if cp.Type != "float" {
		return Param{}, false, fmt.Sprintf("parameter type %q has no measured wire mapping", cp.Type)
	}
	var disp [2]float64
	for i, v := range vals {
		x, why := parseFloatValue(cp, v, bp.Unit)
		if why != "" {
			return Param{}, false, why
		}
		disp[i] = x
	}
	if cp.Default != nil && sameValue(disp[0], *cp.Default) && sameValue(disp[1], *cp.Default) {
		return Param{}, true, ""
	}
	if why := taperRefusal(cp); why != "" {
		return Param{}, false, why
	}
	p := Param{Index: cp.Index, Label: lp.Label, Units: cp.Units, SceneMode: disp[0] != disp[1]}
	var wire [2]float64
	for i, x := range disp {
		w, err := cp.ToWire(x)
		if err != nil {
			return Param{}, false, "cannot convert: " + err.Error()
		}
		wire[i] = w
	}
	for s := 0; s < Scenes; s++ {
		p.Display[s] = disp[sceneIndex(s)]
		p.Wire[s] = wire[sceneIndex(s)]
	}
	return p, false, ""
}

// taperRefusal checks that the parameter's display-to-wire mapping rests on device
// evidence: a measured skew and numeric or unit-confirmed bounds.
func taperRefusal(p catalog.Param) string {
	if p.SkewSymbol == "LOG_SKEW" {
		return "LOG_SKEW taper (0.3, pyquadcortex data) is not confirmed on CorOS 4.1.0"
	}
	skew := 1.0
	if p.Skew != nil {
		skew = *p.Skew
	}
	if _, ok := catalog.ConfirmedSkews[skew]; !ok {
		return fmt.Sprintf("taper skew %s is not confirmed on CorOS 4.1.0 (measured: %s)", fmtNum(skew), confirmedSkewList())
	}
	for _, sym := range []string{p.MinSymbol, p.MaxSymbol} {
		if sym == "" {
			continue
		}
		if why, ok := catalog.UnmeasuredBounds[sym]; ok {
			return fmt.Sprintf("bound %s is %s", sym, why)
		}
		fb, ok := catalog.FirmwareBounds[sym]
		if !ok {
			return fmt.Sprintf("bound %s is unknown", sym)
		}
		if fb.Evidence != catalog.EvidenceConfirmed410 {
			return fmt.Sprintf("bound %s = %s is %s", sym, fmtNum(fb.Value), fb.Evidence)
		}
	}
	return ""
}

func confirmedSkewList() string {
	ks := make([]float64, 0, len(catalog.ConfirmedSkews))
	for k := range catalog.ConfirmedSkews {
		ks = append(ks, k)
	}
	sort.Float64s(ks)
	s := make([]string, len(ks))
	for i, k := range ks {
		s[i] = fmtNum(k)
	}
	return strings.Join(s, ", ")
}

// numberUnit is a single number with an optional unit: "5", "-6.5 dB", "250ms", ".5", "18 %".
var numberUnit = regexp.MustCompile(`^([+-]?(?:\d+(?:\.\d*)?|\.\d+))\s*([A-Za-z%/]*)$`)

// parseFloatValue reads a written value as a display number in the catalog's own unit.
func parseFloatValue(p catalog.Param, raw, unitField string) (float64, string) {
	v := cleanValue(raw)
	switch {
	case isPan(p):
		if isCentre(v) && p.Default != nil && p.Min != nil && p.Max != nil && sameValue(*p.Default, (*p.Min+*p.Max)/2) {
			return *p.Default, ""
		}
		return 0, "pan display scale is not verified on the device; only Center (the default) can be taken"
	case p.Units == "" && p.Min != nil && p.Max != nil && *p.Min == 0 && *p.Max == 1:
		return 0, "unitless 0..1 control: its display scale is not verified on the device"
	}
	if strings.EqualFold(v, "off") {
		minOff, maxOff := strings.EqualFold(p.MinLabel, "OFF"), strings.EqualFold(p.MaxLabel, "OFF")
		switch {
		case minOff && !maxOff && p.Min != nil:
			return *p.Min, ""
		case maxOff && !minOff && p.Max != nil:
			return *p.Max, ""
		}
		return 0, fmt.Sprintf("%q isn't a value of this control", truncate(v, 40))
	}
	m := numberUnit.FindStringSubmatch(v)
	if m == nil {
		return 0, fmt.Sprintf("unparseable value %q", truncate(v, 40))
	}
	x, err := strconv.ParseFloat(m[1], 64)
	if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
		return 0, fmt.Sprintf("unparseable value %q", truncate(v, 40))
	}
	written := m[2]
	if written == "" {
		written = strings.TrimSpace(unitField)
	}
	if written == "" {
		return x, "" // a bare number is in the control's own unit
	}
	if strings.EqualFold(written, p.Units) {
		return x, ""
	}
	if p.Units == "" {
		return 0, fmt.Sprintf("written in %s, but the control is unitless", truncate(written, 12))
	}
	wDim, wFactor, wOK := catalog.UnitDimension(written)
	cDim, cFactor, cOK := catalog.UnitDimension(p.Units)
	if !wOK || !cOK || wDim != cDim {
		return 0, fmt.Sprintf("written in %s, but the control is in %s", truncate(written, 12), p.Units)
	}
	return x * wFactor / cFactor, ""
}

func isPan(p catalog.Param) bool {
	l := func(s string) bool { return s == "L" || s == "R" }
	return l(p.MinLabel) || l(p.MaxLabel)
}

func isCentre(v string) bool {
	switch strings.ToLower(v) {
	case "c", "center", "centre", "centered", "centred":
		return true
	}
	return false
}

// discrete handles switches, rotary switches, toggles and combo boxes. Their wire mapping
// has not been measured, so only a value equal to the catalog default in both scenes is
// accepted (and then not written).
func discrete(p catalog.Param, vals [2]string) (Param, bool, string) {
	if p.Default == nil || p.Min == nil {
		return Param{}, false, fmt.Sprintf("%s positions have no measured wire mapping", p.Type)
	}
	def := int(math.Round(*p.Default - *p.Min))
	for _, v := range vals {
		pos, ok := position(p, cleanValue(v))
		if !ok {
			return Param{}, false, fmt.Sprintf("%q isn't a position of this control (%s)", truncate(cleanValue(v), 40), positions(p))
		}
		if pos != def {
			return Param{}, false, fmt.Sprintf("%s positions have no measured wire mapping; only the default (%s) can be taken", p.Type, positionName(p, def))
		}
	}
	return Param{}, true, ""
}

// position reads a step name, or a whole number in the control's range.
func position(p catalog.Param, v string) (int, bool) {
	for i, s := range p.StepNames {
		if strings.EqualFold(strings.TrimSpace(s), v) {
			return i, true
		}
	}
	n, err := strconv.Atoi(v)
	if err != nil || p.Max == nil {
		return 0, false
	}
	if float64(n) < *p.Min || float64(n) > *p.Max {
		return 0, false
	}
	return int(math.Round(float64(n) - *p.Min)), true
}

func positions(p catalog.Param) string {
	if len(p.StepNames) > 0 {
		names := make([]string, len(p.StepNames))
		for i, s := range p.StepNames {
			names[i] = strings.TrimSpace(s)
		}
		return strings.Join(names, "|")
	}
	if p.Max != nil {
		return fmtNum(*p.Min) + ".." + fmtNum(*p.Max)
	}
	return "unknown"
}

func positionName(p catalog.Param, pos int) string {
	if pos >= 0 && pos < len(p.StepNames) {
		return strings.TrimSpace(p.StepNames[pos])
	}
	return fmtNum(*p.Min + float64(pos))
}

func sameValue(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

func fmtNum(x float64) string { return strconv.FormatFloat(x, 'g', -1, 64) }

func displayString(x float64, units string) string {
	s := fmtNum(x)
	if units != "" {
		s += " " + units
	}
	return s
}
