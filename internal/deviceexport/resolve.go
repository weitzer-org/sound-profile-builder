package deviceexport

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicelib"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// builder collects one export's unresolved items.
type builder struct {
	e          *Exporter
	legacy     bool // the preset predates Architect v4's Bypass definition (see usesBasis)
	unresolved []devicejobs.Unresolved
	warnings   []devicejobs.Unresolved
	dev        *devicelib.Snapshot // the synced lists, nil when none were loaded
	layout     devicejobs.SceneLayout
	src        [Scenes]devicejobs.SceneSource // layout.Sources(), cached
}

// absentValue marks a scene whose guitar doesn't list a parameter that another guitar of the
// same layout sets: that guitar's tone has the control at the device default.
const absentValue = "\x00default"

// sceneParam is one parameter as the layout sees it: the text each of the eight scenes
// takes. For the default layout, Scenes A and B hold the preset's Value and ValueB (B falls
// back to A when empty) and Scenes C-H repeat Scene A, as before.
type sceneParam struct {
	Name string
	Unit string
	Vals [Scenes]string
}

// sceneWords says which scenes a note about "the scenes" means.
func (b *builder) sceneWords() string {
	if b.layout.IsDefault() {
		return "Scene A and Scene B"
	}
	return "the scenes you assigned"
}

// sceneParamOf builds a parameter's per-scene text. per[g] is guitar g's entry for it, nil
// when that guitar doesn't list the control.
func (b *builder) sceneParamOf(per []*storage.BlockParameter) sceneParam {
	var sp sceneParam
	for _, bp := range per {
		if bp != nil && sp.Name == "" {
			sp.Name, sp.Unit = bp.Name, bp.Unit
		}
	}
	for s := 0; s < Scenes; s++ {
		so := b.src[s]
		bp := per[so.Guitar]
		switch {
		case bp == nil:
			sp.Vals[s] = absentValue
		case so.Lead:
			sp.Vals[s] = sceneStrings(*bp)[1]
		default:
			sp.Vals[s] = bp.Value
		}
	}
	return sp
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
	case src != "" && b.dev != nil && b.dev.Library != nil:
		// A synced list: resolve exactly as the agent will (plan 4.2). A capture the unit
		// doesn't list blocks the push here instead of failing at the bridge.
		r := devicelib.Resolve(b.dev.Library, b.dev.Aliases, bare, src)
		if r.Status != devicelib.StatusOnUnit {
			return nil, true, r.Reason
		}
		return &Capture{Name: r.UnitName, Source: src, res: &r, presetName: bare, syncedAt: b.dev.Library.SyncedAt}, true, ""
	case src != "":
		return &Capture{Name: bare, Source: src}, true, ""
	case annotated:
		return nil, true, "marked as a capture, but it isn't in coros_map.json or user_captures.json"
	}
	return nil, false, ""
}

// resolveModel picks the one device model a block name means, or says why it can't.
func (b *builder) resolveModel(name string) (*catalog.Model, string) {
	m, why, _ := b.resolveModelFor(name, nil)
	return m, why
}

// PickedByControls is appended to a block's index_evidence when its name was ambiguous
// (several device models, different controls) and the block's own parameters selected one.
const PickedByControls = "+picked-by-controls"

// resolveModelFor is resolveModel with the block's parameters in hand. For a name whose models
// differ, it takes the one model whose controls cover every parameter the block writes and every
// kept_defaults entry; with none or several fitting it stays ambiguous (never by id order).
// picked reports that choice so the caller can label it.
func (b *builder) resolveModelFor(name string, blk *storage.EffectBlock) (m *catalog.Model, why string, picked bool) {
	if name == "" {
		return nil, "the block has no model name", false
	}
	ms := b.e.cat.Resolve(name)
	if len(ms) == 0 {
		return nil, "not a block on the device (no catalog model or mono/stereo family has this name)", false
	}
	if ms[0].Name == name {
		if len(ms) > 1 {
			// Owner decision (2026-11-06): models that share a name and have identical
			// controls are interchangeable; take the lowest id (Green 808 is 5 and 27, and
			// G1 read a hand-placed Green 808 back as 5). Different layouts stay ambiguous.
			if !sameLayout(ms) {
				if blk != nil {
					if fit := modelsCovering(ms, blk); len(fit) == 1 {
						return fit[0], b.modelRefusal(fit[0]), true
					} else if len(fit) > 1 {
						return nil, fmt.Sprintf("ambiguous: %d device models share this name with different controls and the block's parameters fit %d of them (ids %s); use a name that picks one", len(ms), len(fit), idList(fit)), false
					}
					return nil, fmt.Sprintf("ambiguous: %d device models share this name with different controls (ids %s) and none has every control the block uses; use a name that picks one", len(ms), idList(ms)), false
				}
				return nil, fmt.Sprintf("ambiguous: %d device models share this name with different controls (ids %s); use a name that picks one", len(ms), idList(ms)), false
			}
			m = ms[0]
			for _, x := range ms[1:] {
				if x.ID < m.ID {
					m = x
				}
			}
			return m, b.modelRefusal(m), false
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
			return nil, fmt.Sprintf("family %q has %d mono models; name the model exactly", name, len(mono)), false
		}
		m = mono[0]
	}
	return m, b.modelRefusal(m), false
}

func idList(ms []*catalog.Model) string {
	ids := make([]string, len(ms))
	for i, x := range ms {
		ids[i] = strconv.Itoa(x.ID)
	}
	return strings.Join(ids, ", ")
}

// modelsCovering returns the models that have a control for every parameter the block writes
// (Bypass excepted) and every kept_defaults entry, matched as block() matches them.
func modelsCovering(ms []*catalog.Model, blk *storage.EffectBlock) []*catalog.Model {
	var keys []string
	for _, bp := range blk.Parameters {
		if k := catalog.ParamNameKey(bp.Name); k != "BYPASS" {
			keys = append(keys, k)
		}
	}
	for _, kd := range blk.KeptDefaults {
		keys = append(keys, catalog.ParamNameKey(kd))
	}
	var fit []*catalog.Model
	for _, m := range ms {
		byKey := map[string]catalog.LabelledParam{}
		for _, lp := range catalog.LabelParams(m) {
			byKey[lp.Key] = lp
		}
		ok := true
		for _, k := range keys {
			if _, found := lookup(byKey, k); !found {
				ok = false
				break
			}
		}
		if ok {
			fit = append(fit, m)
		}
	}
	return fit
}

// sameLayout reports whether every model has the same category, parameter count and
// parameter list (indexes, names, types, ranges, defaults, steps), so any of them is the
// same block on the device.
func sameLayout(ms []*catalog.Model) bool {
	for _, m := range ms[1:] {
		if !reflect.DeepEqual(ms[0].Params, m.Params) || m.ParamCount != ms[0].ParamCount ||
			m.Category != ms[0].Category || m.Clones != ms[0].Clones || m.PluginID != ms[0].PluginID {
			return false
		}
	}
	return true
}

// BridgePushesPlugins is whether the home agent accepts plugin models. It does (F3): the agent's
// validator accepts a plugin model when the block names the plugin its catalog row belongs to
// (export_validator.PLUGIN_NAMES). Setting it false makes the dry run report a plugin model as
// unresolved instead; keep it in step with the agent's check (plan 7.3 follow-up F3).
var BridgePushesPlugins = true

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
	if plugin, gated := m.RequiredPlugin(); gated && !BridgePushesPlugins {
		return fmt.Sprintf("%s is a model of the %s plugin, and the home bridge refuses plugin models in v1 (it would refuse the whole push); use a native model for this block, or build this block on the unit by hand", m.Name, plugin)
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
func (b *builder) block(ref string, blks []*storage.EffectBlock, col int) (*Block, error) {
	blk := unionBlock(blks)
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
		var picked bool
		m, why, picked = b.resolveModelFor(name, blk)
		if why != "" {
			b.refuse(ref, "", why)
			return nil, nil
		}
		ev := indexEvidence(m)
		if picked {
			ev += PickedByControls
		}
		out = &Block{Col: col, ModelID: m.ID, Model: m.Name, Category: m.Category,
			IndexEvidence: ev, Params: []Param{}, AtDefault: []string{}}
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
	canon := func(key string) string {
		if lp, ok := lookup(byKey, key); ok {
			return fmt.Sprintf("#%d", lp.Param.Index) // "Time" and "Delay Time" are one control
		}
		return key
	}
	for _, mp := range b.mergeParams(ref, blks, canon) {
		bp := mp.first
		sp := b.sceneParamOf(mp.per)
		key := catalog.ParamNameKey(bp.Name)
		switch {
		case key == "BYPASS":
			b.bypass(ref, sp, &out.Bypass)
			continue
		case hasMics && (key == "MIC1" || key == "MIC2"):
			label, note, mw := b.micChoice(m, key, sp)
			switch {
			case mw != nil:
				out.Mics = append(out.Mics, *mw)
			case note != "":
				// Owner decision (plan 4.1): the cab keeps its default mic, visibly.
				b.warn(ref, bp.Name, note)
			default:
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
		p, atDefault, why := b.param(lp, sp)
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

// unionBlock is the block the model is resolved from: the first guitar's block, carrying every
// parameter and kept default that any guitar of the layout names, so a name shared by models
// with different controls is picked by what all the tones need. One guitar returns its own block.
func unionBlock(blks []*storage.EffectBlock) *storage.EffectBlock {
	if len(blks) == 1 {
		return blks[0]
	}
	u := *blks[0]
	u.Parameters = append([]storage.BlockParameter(nil), blks[0].Parameters...)
	u.KeptDefaults = append([]string(nil), blks[0].KeptDefaults...)
	for _, o := range blks[1:] {
		u.Parameters = append(u.Parameters, o.Parameters...)
		u.KeptDefaults = append(u.KeptDefaults, o.KeptDefaults...)
	}
	return &u
}

// mergedParam is one control of a block across the layout's guitars: per[g] is guitar g's
// entry, nil when that guitar doesn't list it. first is the entry that names it.
type mergedParam struct {
	first *storage.BlockParameter
	per   []*storage.BlockParameter
}

// mergeParams lines up the guitars' parameters by control. The first guitar's order comes
// first, with every entry kept as before (a control listed twice is caught later by the
// written-twice check); controls only a later guitar sets follow. A later guitar listing one
// control twice is refused.
func (b *builder) mergeParams(ref string, blks []*storage.EffectBlock, canon func(string) string) []mergedParam {
	var out []mergedParam
	for i := range blks[0].Parameters {
		bp := &blks[0].Parameters[i]
		per := make([]*storage.BlockParameter, len(blks))
		per[0] = bp
		out = append(out, mergedParam{first: bp, per: per})
	}
	for g := 1; g < len(blks); g++ {
		seen := map[string]bool{}
		for i := range blks[g].Parameters {
			bp := &blks[g].Parameters[i]
			key := canon(catalog.ParamNameKey(bp.Name))
			if seen[key] {
				b.refuse(ref, bp.Name, fmt.Sprintf("%s is listed twice for the same guitar", truncate(bp.Name, 40)))
				continue
			}
			seen[key] = true
			placed := false
			for j := range out {
				if canon(catalog.ParamNameKey(out[j].first.Name)) == key && out[j].per[g] == nil {
					out[j].per[g] = bp
					placed = true
					break
				}
			}
			if !placed {
				per := make([]*storage.BlockParameter, len(blks))
				per[g] = bp
				out = append(out, mergedParam{first: bp, per: per})
			}
		}
	}
	return out
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
// XML's default IR name, "<prefix>_<cab>_<mic>"). The default isn't written. Any other mic is
// written only when it is exactly one of the strings seen on the unit for this same cab (plan
// P4b: strings collected from presets the user asked to read), because which mics a cab offers
// isn't in ModelRepo.xml. Otherwise the cab keeps its default and note says so (a warning,
// not a refusal; plan 4.1).
func (b *builder) micChoice(m *catalog.Model, key string, sp sceneParam) (label, note string, write *Mic) {
	slot := 1
	if key == "MIC2" {
		slot = 2
	}
	label = fmt.Sprintf("MIC %d", slot)
	def, idx := "", -1
	n := 0
	for _, p := range m.Params {
		if p.Name == "ir selector" {
			n++
			if n == slot {
				def, idx = p.DefaultString, p.Index
			}
		}
	}
	defMic := def
	if i := strings.LastIndex(def, "_"); i >= 0 {
		defMic = def[i+1:]
	}
	if defMic == "" {
		return "", "not applied: this cab's default mic isn't known, so the cab keeps whatever mic it loads", nil
	}
	var scenes [Scenes]string
	for i, v := range sp.Vals {
		scenes[i] = v
		if v == absentValue {
			scenes[i] = defMic // this guitar doesn't name a mic: the cab's default
		}
	}
	want := ""
	for _, v := range scenes {
		if micKey(cleanValue(v)) != micKey(defMic) {
			want = cleanValue(v)
			break
		}
	}
	if want == "" {
		return label, "", nil // every scene asks for the default mic
	}
	if b.dev == nil || b.dev.Mics == nil || idx < 0 || !m.ValidParamIndex(idx) {
		// No mic list yet: the note is exactly what it was before the library sync existed.
		return "", fmt.Sprintf("not applied: %q can't be written yet (which mics this cab offers isn't known), so the cab keeps its default %s",
			truncate(want, 40), defMic), nil
	}
	unseen := fmt.Sprintf("not applied: %q can't be written yet (it hasn't been seen on your unit for this cab; read a preset that uses it with read_device_preset), so the cab keeps its default %s",
		truncate(want, 40), defMic)
	for _, v := range scenes[1:] {
		if micKey(cleanValue(v)) != micKey(cleanValue(scenes[0])) {
			return "", fmt.Sprintf("not applied: the mic differs between %s, and a cab's mic is one setting for all scenes, so the cab keeps its default %s", b.sceneWords(), defMic), nil
		}
	}
	var hits []string
	for _, s := range b.dev.Mics.For(m.ID) {
		if b.e.harv != nil && b.e.harv.ValidMic(m.ID, s) && micKey(devicelib.MicPart(s)) == micKey(want) {
			hits = append(hits, s)
		}
	}
	switch len(hits) {
	case 0:
		return "", unseen, nil
	case 1:
		return label, "", &Mic{Slot: slot, Index: idx, Value: hits[0], Evidence: MicEvidence}
	}
	return "", fmt.Sprintf("not applied: %q matches %d different strings seen on your unit for this cab, and the bridge won't guess; the cab keeps its default %s",
		truncate(want, 40), len(hits), defMic), nil
}

// MicEvidence labels a mic string taken from the unit's own read-back.
const MicEvidence = "seen-on-unit"

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
func (b *builder) bypass(ref string, sp sceneParam, out *[Scenes]bool) {
	if b.legacy {
		b.refuse(ref, sp.Name, "this preset predates the rule that Bypass On means bypassed (Architect v4); refine it to push it")
		return
	}
	var flags [Scenes]bool
	for i, v := range sp.Vals {
		if v == absentValue {
			continue // this guitar's block has no Bypass entry: active, as a block with none always was
		}
		switch strings.ToLower(cleanValue(v)) {
		case "on":
			flags[i] = true
		case "off":
			flags[i] = false
		default:
			b.refuse(ref, sp.Name, fmt.Sprintf("Bypass must be On (bypassed) or Off (active), got %q", truncate(v, 40)))
			return
		}
	}
	*out = flags
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
func (b *builder) param(lp catalog.LabelledParam, sp sceneParam) (Param, bool, string) {
	cp := lp.Param
	vals := sp.Vals
	for _, v := range vals {
		if v != absentValue && strings.TrimSpace(v) == "" {
			return Param{}, false, "no value"
		}
	}
	if catalog.DiscreteParamTypes[cp.Type] {
		return discrete(lp, vals)
	}
	if !catalog.ContinuousParamTypes[cp.Type] {
		return Param{}, false, fmt.Sprintf("parameter type %q has no measured wire mapping", cp.Type)
	}
	var disp [Scenes]float64
	allDefault := cp.Default != nil
	for i, v := range vals {
		if v == absentValue {
			if cp.Default == nil {
				return Param{}, false, "one guitar doesn't set this control and it has no catalog default to fall back on"
			}
			disp[i] = *cp.Default
			continue
		}
		x, why := parseFloatValue(cp, v, sp.Unit)
		if why != "" {
			return Param{}, false, why
		}
		disp[i] = x
		if allDefault && !sameValue(x, *cp.Default) {
			allDefault = false
		}
	}
	if allDefault {
		return Param{}, true, ""
	}
	evidence, why := paramEvidence(cp)
	if why != "" {
		return Param{}, false, why
	}
	if unitless01(cp) || isPan(cp) {
		evidence = append(evidence, "scale:catalog-range")
	}
	p := Param{Index: cp.Index, Label: lp.Label, Units: cp.Units, Evidence: evidence}
	for s, x := range disp {
		w, err := cp.ToWire(x)
		if err != nil {
			return Param{}, false, "cannot convert: " + err.Error()
		}
		p.Display[s], p.Wire[s] = x, w
		if x != disp[0] {
			p.SceneMode = true
		}
	}
	return p, false, ""
}

// FaderEvidence labels a fader converted like a float on its catalog range (owner decision,
// 2026-11-06): wire = ((display - min) / (max - min)) ^ skew, linear when the catalog has no
// skew. A GUESS: qc-mcp's to_norm treats a fader like a float, but no one has read a fader
// on the unit (plan 4.1, 8.1). Never counts as confirmed, so the dry run marks it with a dagger.
const FaderEvidence = "encoding:fader-linear-guess"

// paramEvidence is taperEvidence for a float, and for a fader the same labels behind
// FaderEvidence. A fader without a skew of its own gets no taper label: "taper:g4" would
// say the unit measured a fader's taper, and it measured none.
func paramEvidence(p catalog.Param) ([]string, string) {
	ev, why := taperEvidence(p)
	if why != "" || p.Type != "fader" {
		return ev, why
	}
	out := []string{FaderEvidence}
	for _, e := range ev {
		if strings.HasPrefix(e, "taper:") && p.Skew == nil && p.SkewSymbol == "" {
			continue
		}
		out = append(out, e)
	}
	return out, ""
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
		// A plain number is read on the control's own catalog range ("scale:catalog-range",
		// marked unconfirmed in the dry run) -- the Architect is shown that range, with L
		// and R ends, in a catalog-aware prompt. What the unit's screen shows for it is
		// unverified. A unit, a word other than Center, or a number outside the range is refused
		// (ToWire refuses out-of-range), never rescaled.
		if x, err := strconv.ParseFloat(v, 64); err == nil && !math.IsNaN(x) && !math.IsInf(x, 0) {
			return x, ""
		}
		return 0, fmt.Sprintf("pan: only Center or a plain number on its catalog range can be taken, got %q", truncate(v, 40))
	case unitless01(p):
		// Owner decision (2026-11-06): a bare number on the control's own 0..1 scale is
		// taken as written ("scale:catalog-range", marked unconfirmed in the dry run). A unit
		// or another scale's number (e.g. 40 or "40%") is refused, never rescaled.
		x, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, fmt.Sprintf("unitless 0..1 control: only a plain number on its 0..1 scale can be taken, got %q", truncate(v, 40))
		}
		return x, ""
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

// unitless01 is a unitless control with a 0..1 range: the preset's number is read on that
// scale (plan 4.1).
func unitless01(p catalog.Param) bool {
	return p.Units == "" && p.Min != nil && p.Max != nil && *p.Min == 0 && *p.Max == 1 && !isPan(p)
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
// have no such data and are only taken at their default. Faders are not discrete: param
// converts them like floats (FaderEvidence).
var linearSwitchTypes = map[string]bool{"switch": true, "rotarySwitch": true, "toggleButton": true}

// discrete handles switches, rotary switches, toggles and combo boxes. A value equal to the
// catalog default in both scenes is not written.
func discrete(lp catalog.LabelledParam, vals [Scenes]string) (Param, bool, string) {
	p := lp.Param
	if p.Min == nil || p.Max == nil || !(*p.Max > *p.Min) {
		return Param{}, false, fmt.Sprintf("%s has no numeric range", p.Type)
	}
	var pos [Scenes]int
	allDefault := p.Default != nil
	for i, v := range vals {
		if v == absentValue {
			if p.Default == nil {
				return Param{}, false, "one guitar doesn't set this control and it has no catalog default to fall back on"
			}
			pos[i] = int(math.Round(*p.Default - *p.Min))
			continue
		}
		n, ok := position(p, cleanValue(v))
		if !ok {
			return Param{}, false, fmt.Sprintf("%q isn't a position of this control (%s)", truncate(cleanValue(v), 40), positions(p))
		}
		pos[i] = n
		if allDefault && n != int(math.Round(*p.Default-*p.Min)) {
			allDefault = false
		}
	}
	if allDefault {
		return Param{}, true, ""
	}
	if !linearSwitchTypes[p.Type] {
		return Param{}, false, fmt.Sprintf("%s positions have no known wire mapping; only the default can be taken", p.Type)
	}
	out := Param{Index: p.Index, Label: lp.Label, Units: p.Units, Evidence: []string{"encoding:qc-mcp-linear"}}
	for i, n := range pos {
		out.Display[i] = *p.Min + float64(n)
		out.Wire[i] = (out.Display[i] - *p.Min) / (*p.Max - *p.Min)
		if out.Wire[i] < 0 || out.Wire[i] > 1 {
			return Param{}, false, fmt.Sprintf("position %d is outside %s..%s", n, fmtNum(*p.Min), fmtNum(*p.Max))
		}
		if n < len(p.StepNames) {
			out.names[i] = strings.TrimSpace(p.StepNames[n])
		}
		if n != pos[0] {
			out.SceneMode = true
		}
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
