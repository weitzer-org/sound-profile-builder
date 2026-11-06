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
	warnings   []devicejobs.Unresolved
}

func (b *builder) warn(block, param, reason string) {
	b.warnings = append(b.warnings, devicejobs.Unresolved{Block: block, Param: truncate(param, 80), Reason: reason})
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

// captureBlockID is the catalog model whose parameter layout a capture block uses. The
// V1 (14000) and V2 (14001) blocks list the same five controls (TestCaptureBlockLayouts).
const captureBlockID = 14000

// resolveCapture reports whether a block name is a capture, and if so which. A name with
// the pipeline's "(capture)" annotation but no library entry is refused: there is nothing
// to look up on the unit.
func (b *builder) resolveCapture(name string) (c *Capture, isCapture bool, why string) {
	bare := strings.TrimSpace(captureAnnotation.ReplaceAllString(name, ""))
	annotated := bare != name
	src := ""
	if b.e.opts.CaptureSource != nil {
		src = b.e.opts.CaptureSource(bare)
	}
	switch {
	case src == "ir":
		return nil, true, "this library entry is a cab IR, and IR loaders are not pushed in v1"
	case src != "":
		return &Capture{Name: bare, Source: src}, true, ""
	case annotated:
		return nil, true, "marked as a capture, but it isn't in coros_map.json or user_captures.json"
	}
	return nil, false, ""
}

// resolveModel picks the one device model a block name means, or says why it can't.
func (b *builder) resolveModel(name string) (*catalog.Model, string) {
	if name == "" {
		return nil, "the block has no model name"
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
	case m.Category == "Neural Capture":
		return "a Neural Capture block needs a specific capture: name the capture, not the block type"
	case m.Category == "IRLoaders":
		return "IR loaders are not pushed in v1"
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
	var out *Block
	var m *catalog.Model
	switch capt, isCapture, why := b.resolveCapture(name); {
	case why != "":
		b.refuse(ref, "", why)
		return nil, nil
	case isCapture:
		m = b.captureModel()
		if m == nil {
			return nil, fmt.Errorf("catalog fault: no Neural Capture block (id %d)", captureBlockID)
		}
		out = &Block{Col: col, ModelID: 0, Model: capt.Name, Category: m.Category,
			IndexEvidence: "capture-block", Capture: capt, Params: []Param{}, AtDefault: []string{},
			modelerType: blk.Type}
	default:
		var why string
		m, why = b.resolveModel(name)
		if why != "" {
			b.refuse(ref, "", why)
			return nil, nil
		}
		out = &Block{Col: col, ModelID: m.ID, Model: m.Name, Category: m.Category,
			IndexEvidence: indexEvidence(m), Params: []Param{}, AtDefault: []string{}}
		if plugin, gated := m.RequiredPlugin(); gated {
			out.Plugin = plugin
		}
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
			label, note := micChoice(m, key, bp)
			if note != "" {
				// Owner decision (plan 4.1): the cab keeps its default mic, visibly.
				b.warn(ref, bp.Name, note)
			} else {
				out.AtDefault = append(out.AtDefault, label)
			}
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

// captureModel is the catalog's V1 Neural Capture block, whose controls a capture block
// exports.
func (b *builder) captureModel() *catalog.Model {
	for i := range b.e.cat.Models {
		if b.e.cat.Models[i].ID == captureBlockID {
			return &b.e.cat.Models[i]
		}
	}
	return nil
}

// micChoice checks a cab's "Mic 1"/"Mic 2" against the mic that slot loads by default (the
// XML's default IR name, "<prefix>_<cab>_<mic>"). The default isn't written. Any other mic
// isn't written either, because which mics a cab offers isn't in ModelRepo.xml: the cab
// keeps its default and note says so (a warning, not a refusal; plan 4.1).
func micChoice(m *catalog.Model, key string, bp storage.BlockParameter) (label, note string) {
	slot := 1
	if key == "MIC2" {
		slot = 2
	}
	label = fmt.Sprintf("MIC %d", slot)
	def := ""
	n := 0
	for _, p := range m.Params {
		if p.Name == "ir selector" {
			n++
			if n == slot {
				def = p.DefaultString
			}
		}
	}
	defMic := def
	if i := strings.LastIndex(def, "_"); i >= 0 {
		defMic = def[i+1:]
	}
	if defMic == "" {
		return "", "not applied: this cab's default mic isn't known, so the cab keeps whatever mic it loads"
	}
	for _, v := range sceneStrings(bp) {
		if micKey(cleanValue(v)) != micKey(defMic) {
			return "", fmt.Sprintf("not applied: %q can't be written yet (which mics this cab offers isn't known), so the cab keeps its default %s",
				truncate(cleanValue(v), 40), defMic)
		}
	}
	return label, ""
}

// micKey normalizes a mic name: case, spacing, the usual abbreviations ("Dyn 57" =
// "Dynamic 57"), and trailing placement notes ("Dynamic 57 - Cap Edge", "Ribbon 160 (off-axis)").
func micKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, sep := range []string{" - ", " (", ",", ";"} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	f := strings.Fields(s)
	if len(f) > 0 {
		switch f[0] {
		case "dyn":
			f[0] = "dynamic"
		case "cond":
			f[0] = "condenser"
		case "rib":
			f[0] = "ribbon"
		}
	}
	return strings.Join(f, " ")
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
		return discrete(lp, vals)
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
	evidence, why := taperEvidence(cp)
	if why != "" {
		return Param{}, false, why
	}
	p := Param{Index: cp.Index, Label: lp.Label, Units: cp.Units, SceneMode: disp[0] != disp[1], Evidence: evidence}
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

// taperEvidence says what a float parameter's display-to-wire mapping rests on (plan 4.1),
// or why it can't be converted at all. Measured on the unit (G4) is labelled ":g4"; the
// device's own XML skew, pyquadcortex's LOG_SKEW and pyquadcortex's symbolic bounds are
// allowed and labelled. A bound nobody has measured, or an unknown one, is refused.
func taperEvidence(p catalog.Param) ([]string, string) {
	var ev []string
	switch skew := skewOf(p); {
	case p.SkewSymbol == "LOG_SKEW":
		ev = append(ev, "taper:pyquadcortex")
	case catalog.ConfirmedSkews[skew] != "":
		ev = append(ev, "taper:g4")
	default:
		ev = append(ev, "taper:device-xml")
	}
	if p.MinSymbol == "" && p.MaxSymbol == "" {
		return ev, ""
	}
	bound := "bounds:g4"
	for _, sym := range []string{p.MinSymbol, p.MaxSymbol} {
		if sym == "" {
			continue
		}
		if why, ok := catalog.UnmeasuredBounds[sym]; ok {
			return nil, fmt.Sprintf("bound %s is %s", sym, why)
		}
		fb, ok := catalog.FirmwareBounds[sym]
		if !ok {
			return nil, fmt.Sprintf("bound %s is unknown", sym)
		}
		if fb.Evidence != catalog.EvidenceConfirmed410 {
			bound = "bounds:pyquadcortex-4.0.1"
		}
	}
	return append(ev, bound), ""
}

func skewOf(p catalog.Param) float64 {
	if p.Skew != nil {
		return *p.Skew
	}
	return 1
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

// linearSwitchTypes are the discrete types written with qc-mcp's position encoding,
// wire = (value - min) / (max - min), where value = min + position (plan 4.1). Combo boxes
// (and faders) have no such data and are only taken at their default.
var linearSwitchTypes = map[string]bool{"switch": true, "rotarySwitch": true, "toggleButton": true}

// discrete handles switches, rotary switches, toggles and combo boxes. A value equal to the
// catalog default in both scenes is not written.
func discrete(lp catalog.LabelledParam, vals [2]string) (Param, bool, string) {
	p := lp.Param
	if p.Min == nil || p.Max == nil || !(*p.Max > *p.Min) {
		return Param{}, false, fmt.Sprintf("%s has no numeric range", p.Type)
	}
	var pos [2]int
	for i, v := range vals {
		n, ok := position(p, cleanValue(v))
		if !ok {
			return Param{}, false, fmt.Sprintf("%q isn't a position of this control (%s)", truncate(cleanValue(v), 40), positions(p))
		}
		pos[i] = n
	}
	if p.Default != nil {
		def := int(math.Round(*p.Default - *p.Min))
		if pos[0] == def && pos[1] == def {
			return Param{}, true, ""
		}
	}
	if !linearSwitchTypes[p.Type] {
		return Param{}, false, fmt.Sprintf("%s positions have no known wire mapping; only the default can be taken", p.Type)
	}
	out := Param{Index: p.Index, Label: lp.Label, Units: p.Units, SceneMode: pos[0] != pos[1],
		Evidence: []string{"encoding:qc-mcp-linear"}}
	var disp, wire [2]float64
	for i, n := range pos {
		disp[i] = *p.Min + float64(n)
		wire[i] = (disp[i] - *p.Min) / (*p.Max - *p.Min)
		if wire[i] < 0 || wire[i] > 1 {
			return Param{}, false, fmt.Sprintf("position %d is outside %s..%s", n, fmtNum(*p.Min), fmtNum(*p.Max))
		}
		if n < len(p.StepNames) {
			out.names[i] = strings.TrimSpace(p.StepNames[n])
		}
	}
	for s := 0; s < Scenes; s++ {
		out.Display[s] = disp[sceneIndex(s)]
		out.Wire[s] = wire[sceneIndex(s)]
	}
	return out, false, ""
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
