package deviceexport

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// The surface internal/deviceedit (preset-editing plan E1) builds on. It is deliberately thin:
// every function here calls the same tables and helpers the export and the read-back import
// use (pushableCategories, importName, captureName, importer.param/discrete, parseFloatValue,
// paramEvidence, ToWire/ToDisplay), so a conversion rule changed for a push changes for an
// edit too. Nothing here writes or reads anything outside its arguments.

// CatalogInfo is the catalog the ids and indexes come from: the CorOS version and the hash of
// qc_catalog.json, as in the export header.
func (e *Exporter) CatalogInfo() (coros, sha256 string) {
	if e.cat == nil {
		return "", e.catalogSH
	}
	return e.cat.Source.CorOSVersion, e.catalogSH
}

// Ready reports whether the exporter has a catalog.
func (e *Exporter) Ready() bool { return e.cat != nil }

// Cell is one row-0 block of a read-back, as the import sees it.
type Cell struct {
	Col     int
	ModelID int
	// Model is nil when the id is not in the catalog.
	Model *catalog.Model
	// Exposed: the import would include this cell in a draft (known id, a pushable category
	// or a capture block, not internal). Every other cell is locked: never matched, removed
	// or written by an edit.
	Exposed bool
	Capture bool
	// Name is the name the import would give the block (the family name for a mono member,
	// a capture's library or unit name). Device text, already through Options.Sanitize.
	// Empty for a cell that is not exposed.
	Name   string
	Placed devicejobs.ReadbackPlaced
}

// Cells lists the read-back's row-0 cells in column order. A payload with no row 0 has none.
func (e *Exporter) Cells(rb *devicejobs.ReadbackPayload) []Cell {
	var out []Cell
	if e.cat == nil || rb == nil || e.opts.Sanitize == nil {
		return out
	}
	im := &importer{e: e, san: e.opts.Sanitize, res: &devicejobs.ImportedPreset{}}
	for _, ch := range rb.Chains {
		if ch.Row != GridRow {
			continue
		}
		for _, pb := range ch.Blocks {
			c := Cell{Col: pb.Col, ModelID: pb.ModelID, Placed: pb}
			m := e.byID[pb.ModelID]
			if m != nil {
				c.Model = m
				c.Capture = pb.ModelID == captureV1 || pb.ModelID == captureV2
				switch {
				case c.Capture:
					c.Exposed = true
					c.Name, _ = im.captureName(m, pb)
				case !pushableCategories[m.Category] || internalModelIDs[m.ID] || strings.Contains(m.Category, "Internal") || m.Category == "Utility_Deprecated":
				default:
					c.Exposed = true
					c.Name = im.importName(m)
				}
			}
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Col < out[j].Col })
	return out
}

// bareCaptureName strips the pipeline's "(capture)" / "(Factory Capture)" / "(My Capture)"
// annotation.
func bareCaptureName(name string) (bare string, annotated bool) {
	bare = strings.TrimSpace(captureAnnotation.ReplaceAllString(name, ""))
	return bare, bare != strings.TrimSpace(name)
}

// NameKind says what a target block's model text names.
type NameKind int

const (
	// NameModel: one or more catalog models (exact name or mono/stereo family).
	NameModel NameKind = iota
	// NameCapture: a capture in the user's libraries, or a name marked as a capture.
	NameCapture
	// NameUnknown: nothing on the device or in the libraries has this name.
	NameUnknown
	// NameNotEditable: it names something an edit never touches (a cab IR, a category the
	// import and export don't place, an internal model, a side-chain model).
	NameNotEditable
)

// ClassifyName classifies a target block's model text.
func (e *Exporter) ClassifyName(name string) NameKind {
	name = strings.TrimSpace(name)
	if e.cat == nil || name == "" {
		return NameUnknown
	}
	bare, annotated := bareCaptureName(name)
	src := ""
	if e.opts.CaptureSource != nil {
		src = e.opts.CaptureSource(bare)
	}
	switch {
	case src == "ir":
		return NameNotEditable
	case src != "" || annotated:
		return NameCapture
	}
	ms := e.cat.Resolve(name)
	if len(ms) == 0 {
		return NameUnknown
	}
	for _, m := range ms {
		if pushableCategories[m.Category] && !internalModelIDs[m.ID] && !strings.Contains(m.Category, "Internal") &&
			m.Category != "Utility_Deprecated" && !strings.Contains(m.Name, "(S/C)") {
			return NameModel
		}
	}
	return NameNotEditable
}

// IsDeviceElement reports whether a target block names the input gate, the output or the
// global EQ: real parts of the device that are not grid blocks.
func IsDeviceElement(model string) bool { return isDeviceElement(model) }

// SameModel reports whether a target block's model text means this cell's model. A capture
// cell matches only a capture-like name whose bare name equals the cell's (case-folded). Any
// other cell matches when its model id is among the models the name resolves to, so a family
// name matches the (M) or the (ST) member and a name shared by several models matches each
// (resolveModelFor would call some of those ambiguous; an edit already knows the cell).
func (e *Exporter) SameModel(c Cell, targetName string) bool {
	if !c.Exposed || e.cat == nil {
		return false
	}
	targetName = strings.TrimSpace(targetName)
	if c.Capture {
		if e.ClassifyName(targetName) != NameCapture {
			return false
		}
		tb, _ := bareCaptureName(targetName)
		cb, _ := bareCaptureName(c.Name)
		return tb != "" && strings.EqualFold(tb, cb)
	}
	if e.ClassifyName(targetName) != NameModel {
		return false
	}
	for _, m := range e.cat.Resolve(targetName) {
		if m.ID == c.ModelID {
			return true
		}
	}
	return false
}

// CloneLocked reports whether the cell's model clones a base whose parameter indexes are not
// verified on the device: the import keeps such a block without values, so an edit can't
// write its controls (bypass works per cell and stays allowed).
func CloneLocked(m *catalog.Model) bool {
	if m == nil || m.Clones == 0 {
		return false
	}
	_, ok := catalog.VerifiedCloneBases[m.Clones]
	return !ok
}

// ---- classifying a target's parameter on a matched cell ----

// ParamClass is how a target parameter name lands on a model.
type ParamClass int

const (
	ParamControl    ParamClass = iota // a catalog control: Label is set
	ParamBypass                       // the Bypass entry
	ParamMic                          // Mic 1 / Mic 2 on a cab; Slot is 1 or 2
	ParamNotControl                   // not a control of this model
	ParamInternal                     // an internal device parameter
	ParamCabBlend                     // Blend on a cab: the device has none
	ParamCaptureCab                   // an IR cab control on a capture sitting in a cab slot
)

// ClassifiedParam is a ClassifyParam answer.
type ClassifiedParam struct {
	Class ParamClass
	Label catalog.LabelledParam
	Slot  int
}

// ParamClassifier classifies target parameter names for one model, exactly as the export does.
type ParamClassifier struct {
	m          *catalog.Model
	byKey      map[string]catalog.LabelledParam
	internal   map[string]bool
	hasMics    bool
	cab        bool
	captureCab bool
	Labels     []catalog.LabelledParam
}

// NewParamClassifier builds the classifier for a model. captureCab is true for a capture
// sitting in a cab slot (the target block's type is a cab).
func NewParamClassifier(m *catalog.Model, isCapture bool, blockType string) *ParamClassifier {
	pc := &ParamClassifier{m: m, byKey: map[string]catalog.LabelledParam{}, internal: map[string]bool{}}
	pc.Labels = catalog.LabelParams(m)
	for _, lp := range pc.Labels {
		pc.byKey[lp.Key] = lp
		if lp.Key == "MIC1LEVEL" {
			pc.hasMics = true
		}
	}
	for _, p := range m.Params {
		if catalog.IsInternalParam(p) {
			pc.internal[catalog.ParamNameKey(p.Name)] = true
		}
	}
	pc.cab = strings.HasPrefix(m.Category, "Cabsim")
	pc.captureCab = isCapture && isCabBlockType(blockType)
	return pc
}

// Classify mirrors the order of checks in builder.block.
func (pc *ParamClassifier) Classify(name string) ClassifiedParam {
	key := catalog.ParamNameKey(name)
	switch {
	case key == "BYPASS":
		return ClassifiedParam{Class: ParamBypass}
	case pc.hasMics && (key == "MIC1" || key == "MIC2"):
		slot := 1
		if key == "MIC2" {
			slot = 2
		}
		return ClassifiedParam{Class: ParamMic, Slot: slot}
	case pc.captureCab && catalog.IsCabIRControl(key):
		return ClassifiedParam{Class: ParamNotControl}
	case pc.cab && key == "BLEND":
		return ClassifiedParam{Class: ParamCabBlend}
	}
	lp, ok := lookup(pc.byKey, key)
	if !ok {
		if pc.internal[key] {
			return ClassifiedParam{Class: ParamInternal}
		}
		return ClassifiedParam{Class: ParamNotControl}
	}
	return ClassifiedParam{Class: ParamControl, Label: lp}
}

// MicDefault is the cab's default mic part for slot 1 or 2 ("Dynamic 57"), or "".
func MicDefault(m *catalog.Model, slot int) (idx int, defMic string) {
	n := 0
	for _, p := range m.Params {
		if p.Name == "ir selector" {
			if n++; n == slot {
				d := p.DefaultString
				if i := strings.LastIndex(d, "_"); i >= 0 {
					d = d[i+1:]
				}
				return p.Index, d
			}
		}
	}
	return -1, ""
}

// SameMic compares two mic names the way the export does (case, spacing, "Dyn 57" = "Dynamic 57",
// trailing placement notes).
func SameMic(a, b string) bool { return micKey(a) == micKey(b) && micKey(a) != "" }

// ---- reading the snapshot's value of a control ----

// LockKind says why the import (and so an edit) can't read or write a control.
type LockKind string

const (
	LockNone          LockKind = ""
	LockNoWireMapping LockKind = "no_wire_mapping"
	LockUnmeasured    LockKind = "unmeasured_bound"
	LockNotWholePos   LockKind = "not_whole_position"
	LockUnreadable    LockKind = "unreadable_value"
)

// Reading is the snapshot's value of one control, converted by the import's own rules.
type Reading struct {
	Wire      [Scenes]float64
	Discrete  bool
	Disp      [2]float64 // continuous: display value, Scenes A and B
	Pos       [2]int     // discrete: position, Scenes A and B
	Shown     [2]string  // as the import would write them
	SceneMode bool       // the unit has the control scene-assigned
}

// ReadControl reads a control's eight scene values from the snapshot with the import's own
// reader (importer.param / importer.discrete). A control the import would leave out comes back
// with a LockKind and no Reading.
func (e *Exporter) ReadControl(lp catalog.LabelledParam, rp devicejobs.ReadbackParam, present bool) (*Reading, LockKind) {
	if !present {
		return nil, LockUnreadable
	}
	cp := lp.Param
	im := &importer{e: e, san: e.opts.Sanitize, res: &devicejobs.ImportedPreset{}}
	var why string
	if catalog.DiscreteParamTypes[cp.Type] {
		_, _, why = im.discrete(lp, rp)
	} else {
		_, _, why = im.param(lp, rp)
	}
	if why != "" {
		return nil, lockOf(why)
	}
	r := &Reading{SceneMode: rp.SceneMode}
	for s := 0; s < Scenes; s++ {
		w, ok := rp.Float(s)
		if !ok {
			return nil, LockUnreadable
		}
		r.Wire[s] = w
	}
	if catalog.DiscreteParamTypes[cp.Type] {
		r.Discrete = true
		span := *cp.Max - *cp.Min
		for s := 0; s < 2; s++ {
			r.Pos[s] = int(math.Max(0, math.Min(span, math.Round(r.Wire[s]*span))))
			r.Shown[s] = positionName(cp, r.Pos[s])
		}
		return r, LockNone
	}
	lo, hi, _ := cp.Bounds()
	for s := 0; s < 2; s++ {
		d, err := cp.ToDisplay(r.Wire[s])
		if err != nil {
			return nil, LockUnreadable
		}
		r.Disp[s] = d
		r.Shown[s] = displayText(d, hi-lo, cp.Units)
		if sp := screenPan(cp, r.Wire[s]); sp != "" {
			r.Shown[s] += " (" + sp + ")"
		}
	}
	return r, LockNone
}

func positionName(cp catalog.Param, n int) string {
	if n >= 0 && n < len(cp.StepNames) && strings.TrimSpace(cp.StepNames[n]) != "" {
		return strings.TrimSpace(cp.StepNames[n])
	}
	return fmtNum(*cp.Min + float64(n))
}

// lockOf classifies the import's reason text. The fragments are the import's own; a test pins
// one control per kind so a reworded reason can't silently change a code.
func lockOf(why string) LockKind {
	switch {
	case strings.Contains(why, "no known wire mapping"), strings.Contains(why, "no measured wire mapping"), strings.Contains(why, "has no numeric range"):
		return LockNoWireMapping
	case strings.Contains(why, "is not a whole position"):
		return LockNotWholePos
	case strings.HasPrefix(why, "bound "):
		return LockUnmeasured
	}
	return LockUnreadable
}

// ---- converting the target's value ----

// ConvKind says why a target value can't be converted.
type ConvKind string

const (
	ConvOK            ConvKind = ""
	ConvUnparseable   ConvKind = "unparseable"
	ConvWrongUnit     ConvKind = "wrong_unit"
	ConvOutOfRange    ConvKind = "out_of_range"
	ConvNotAPosition  ConvKind = "not_a_position"
	ConvNoWireMapping ConvKind = "no_wire_mapping"
	ConvUnmeasured    ConvKind = "unmeasured_bound"
)

// Converted is a target value converted for Scenes A and B.
type Converted struct {
	Discrete bool
	Disp     [2]float64
	Pos      [2]int
	Wire     [2]float64
	Evidence []string
	Shown    [2]string
	// Param is the export's own Param for the two scenes, for Confirmed().
	Confirmed bool
}

// ConvertTarget converts a target parameter's Scene A and Scene B text (valueB empty means the
// same as A, the export's rule) with the export's parsers and Param.ToWire. Unlike the export it
// has no "at default, don't write" shortcut: an edit compares with what the unit holds.
func (e *Exporter) ConvertTarget(lp catalog.LabelledParam, bp storage.BlockParameter) (*Converted, ConvKind) {
	cp := lp.Param
	vals := sceneStrings(bp)
	for _, v := range vals {
		if strings.TrimSpace(v) == "" {
			return nil, ConvUnparseable
		}
	}
	c := &Converted{}
	if catalog.DiscreteParamTypes[cp.Type] {
		if cp.Min == nil || cp.Max == nil || !(*cp.Max > *cp.Min) || !linearSwitchTypes[cp.Type] {
			return nil, ConvNoWireMapping
		}
		c.Discrete = true
		c.Evidence = []string{"encoding:qc-mcp-linear"}
		for s, v := range vals {
			n, ok := position(cp, cleanValue(v))
			if !ok {
				return nil, ConvNotAPosition
			}
			c.Pos[s] = n
			c.Disp[s] = *cp.Min + float64(n)
			c.Wire[s] = (c.Disp[s] - *cp.Min) / (*cp.Max - *cp.Min)
			c.Shown[s] = positionName(cp, n)
		}
		c.Confirmed = Param{Evidence: c.Evidence}.Confirmed()
		return c, ConvOK
	}
	if !catalog.ContinuousParamTypes[cp.Type] {
		return nil, ConvNoWireMapping
	}
	ev, why := paramEvidence(cp)
	if why != "" {
		return nil, ConvUnmeasured
	}
	if unitless01(cp) || isPan(cp) {
		ev = append(ev, "scale:catalog-range")
	}
	c.Evidence = ev
	lo, hi, _ := cp.Bounds()
	for s, v := range vals {
		x, w := parseFloatValue(cp, v, bp.Unit)
		if w != "" {
			if strings.Contains(w, "written in") {
				return nil, ConvWrongUnit
			}
			return nil, ConvUnparseable
		}
		wire, err := cp.ToWire(x)
		if err != nil {
			if strings.Contains(err.Error(), "is outside") {
				return nil, ConvOutOfRange
			}
			return nil, ConvUnparseable
		}
		c.Disp[s], c.Wire[s] = x, wire
		c.Shown[s] = displayText(x, hi-lo, cp.Units)
		if sp := screenPan(cp, wire); sp != "" {
			c.Shown[s] += " (" + sp + ")"
		}
	}
	c.Confirmed = Param{Evidence: c.Evidence}.Confirmed()
	return c, ConvOK
}

// ToleranceH is the no-op tolerance for a continuous control, in display units: half of one
// ten-thousandth of the range, or half a unit in the fifth significant figure of the value
// (the import writes values with five), whichever is larger. A target within H of what the
// unit holds is the same value. See deviceedit's package comment for the reasoning.
func ToleranceH(cp catalog.Param, target float64) float64 {
	lo, hi, err := cp.Bounds()
	if err != nil {
		return 0
	}
	h := (hi - lo) * 5e-5
	if a := math.Abs(target); a > 0 {
		if sig := 0.5 * math.Pow(10, math.Floor(math.Log10(a))-4); sig > h {
			h = sig
		}
	}
	return h
}

// ParseBypass reads a Bypass value: "On" is bypassed, "Off" is active (Architect Rule 13).
func ParseBypass(v string) (bypassed, ok bool) {
	switch strings.ToLower(cleanValue(v)) {
	case "on":
		return true, true
	case "off":
		return false, true
	}
	return false, false
}

// SceneB is a parameter's Scene B text, falling back to Scene A when empty.
func SceneB(bp storage.BlockParameter) string { return sceneStrings(bp)[1] }

// UsesBasis reports whether a preset's parameters carry a basis (Architect v4 and later).
func UsesBasis(sp storage.StructuredPreset) bool { return usesBasis(sp) }

// ShownScene is the export's own rendering of a converted scene value; kept for the dry run
// of a push. (Edit tables use Converted.Shown.)
func (p Param) ShownScene(scene int) string { return p.shown(scene) }

// Truncate cuts s to n runes, like the rest of this package.
func Truncate(s string, n int) string { return truncate(s, n) }

// Sprint is a tiny helper for stable number text in plans.
func Sprint(x float64) string { return fmt.Sprint(fmtNum(x)) }
