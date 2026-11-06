package deviceexport

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// The inverse of the export (plan P5): the preset open on the unit, as the agent reads it
// back (devicejobs.ReadbackPayload, format qc-device-readback/1), becomes a QC-2
// StructuredPreset draft. It shares this package's tables and rules on purpose
// (pushableCategories, taperRefusal, isPan, indexEvidence, VerifiedCloneBases ...), so a
// rule changed for the export changes here too.
//
// Everything read back is untrusted. The importer refuses rather than guesses: a control it
// can't convert exactly is listed under Unconverted with a reason, never approximated.
// Text from the unit (preset and capture names) passes through Options.Sanitize before it
// reaches a Value, a name or a note.

// ReadbackBasis is the Basis of every value read from the unit. Any non-empty basis also
// tells the exporter the preset is post-v4, so a re-push accepts its Bypass values.
const ReadbackBasis = "device_readback"

// ReadbackGuitar is the guitar key of an imported draft: the unit has no guitar notion.
const ReadbackGuitar = "From the unit"

// Capture blocks (plan 4.2): model 14000 is V1, 14001 is V2; parameter 5 is file_name.
const (
	captureV1        = 14000
	captureV2        = 14001
	captureFileIndex = 5
)

// maxCaptureName bounds a capture name taken from the unit.
const maxCaptureName = 64

var hex64 = regexp.MustCompile(`^[0-9a-fA-F]{64}`)

var _ devicejobs.ReadbackImporter = (*Exporter)(nil)

type importer struct {
	e    *Exporter
	res  *devicejobs.ImportedPreset
	san  func(string, int) string
	rows map[int]bool
}

// ImportReadback implements devicejobs.ReadbackImporter.
func (e *Exporter) ImportReadback(_ context.Context, rb *devicejobs.ReadbackPayload, meta devicejobs.ReadbackMeta) (*devicejobs.ImportedPreset, error) {
	if e.cat == nil {
		return nil, errors.New("device catalog unavailable")
	}
	if e.opts.Sanitize == nil {
		return nil, errors.New("read-back import needs a text sanitizer (Options.Sanitize)")
	}
	if rb == nil {
		return nil, errors.New("no read-back payload")
	}
	im := &importer{e: e, san: e.opts.Sanitize, res: &devicejobs.ImportedPreset{
		Guitar: ReadbackGuitar, Unconverted: []devicejobs.Unconverted{}, Notes: []string{},
	}}
	im.res.Name = im.san(meta.Name, 64)

	var row0 *devicejobs.ReadbackChain
	chains := append([]devicejobs.ReadbackChain(nil), rb.Chains...)
	sort.Slice(chains, func(i, j int) bool { return chains[i].Row < chains[j].Row })
	for i := range chains {
		ch := &chains[i]
		switch {
		case ch.Row == GridRow:
			row0 = ch
		case len(ch.Blocks) > 0:
			im.unconv(fmt.Sprintf("row %d", ch.Row), "", fmt.Sprintf("row %d has %d block(s); a QC-2 preset is one row, so rows other than 0 are not imported", ch.Row, len(ch.Blocks)))
		}
	}
	chain := []storage.EffectBlock{}
	if row0 == nil {
		im.note("the unit's row 0 is empty or missing; the draft has no blocks")
	} else {
		if row0.InPort == nil || row0.OutPort == nil || *row0.InPort != InPort || *row0.OutPort != OutPort {
			im.note(fmt.Sprintf("row 0 is routed %s to %s, not the standard %d to %d the export writes; routing is not part of a QC-2 preset",
				portText(row0.InPort), portText(row0.OutPort), InPort, OutPort))
		}
		blocks := append([]devicejobs.ReadbackPlaced(nil), row0.Blocks...)
		sort.Slice(blocks, func(i, j int) bool { return blocks[i].Col < blocks[j].Col })
		for _, pb := range blocks {
			var scenes *[Scenes]bool
			if rb.Bypass != nil && GridRow < len(rb.Bypass) && pb.Col < len(rb.Bypass[GridRow]) {
				var a [Scenes]bool
				copy(a[:], rb.Bypass[GridRow][pb.Col])
				scenes = &a
			}
			blk, err := im.block(pb, scenes)
			if err != nil {
				return nil, err
			}
			if blk != nil {
				chain = append(chain, *blk)
			}
		}
	}
	if rb.Bypass == nil {
		im.note("the unit sent no bypass states; every block is assumed active in all scenes")
	}
	if rb.Setlist.Folder != "My Presets" {
		im.note("the open preset is not in My Presets (a factory or other folder preset)")
	}
	im.res.Preset = storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{ReadbackGuitar: chain}}
	im.res.Blocks = len(chain)
	return im.res, nil
}

func portText(p *int) string {
	if p == nil {
		return "an unreported port"
	}
	return "port " + strconv.Itoa(*p)
}

func (im *importer) unconv(block, param, reason string) {
	im.res.UnconvertedCount++
	if len(im.res.Unconverted) >= maxImportItems {
		return
	}
	im.res.Unconverted = append(im.res.Unconverted, devicejobs.Unconverted{
		Block: truncate(block, 100), Param: truncate(param, 80), Reason: truncate(reason, 300)})
}

// maxImportItems bounds the notes and the unconverted list, so a hostile or odd read can't
// produce an unbounded report.
const maxImportItems = 200

func (im *importer) note(s string) {
	if len(im.res.Notes) < maxImportItems {
		im.res.Notes = append(im.res.Notes, truncate(s, 400))
	}
}

// ---- model names ----

// importName is the name the Architect would write for a model: the family name when the
// export would resolve that family to this very model (a mono member with an unambiguous
// family), else the exact catalog name. Re-exporting the draft therefore resolves to the
// same device model.
func (im *importer) importName(m *catalog.Model) string {
	if strings.HasSuffix(m.Name, " (M)") {
		fam := catalog.FamilyName(m.Name)
		b := &builder{e: im.e}
		if got, why := b.resolveModel(fam); why == "" && got != nil && got.ID == m.ID {
			return fam
		}
	}
	return m.Name
}

// effectType is EffectBlock.Type for a catalog category (the words the Architect uses).
func effectType(category string) string {
	switch {
	case strings.Contains(category, "Amplifier"):
		return "Amplifier"
	case strings.Contains(category, "Overdrive"):
		return "Overdrive"
	case strings.HasPrefix(category, "Cabsim"):
		return "Cab"
	case category == "Equalizer":
		return "EQ"
	case category == "Neural Capture":
		return "Capture"
	}
	return category
}

// ---- one block ----

func (im *importer) block(pb devicejobs.ReadbackPlaced, bypass *[Scenes]bool) (*storage.EffectBlock, error) {
	m := im.e.byID[pb.ModelID]
	if m == nil {
		im.unconv(fmt.Sprintf("col %d (model id %d)", pb.Col+1, pb.ModelID), "",
			"unknown model id: not in the catalog (a newer CorOS model, or a read fault); the block is left out of the draft")
		return nil, nil
	}
	isCapture := pb.ModelID == captureV1 || pb.ModelID == captureV2
	ref := fmt.Sprintf("#%d %s", pb.Col+1, truncate(m.Name, 60))
	blk := &storage.EffectBlock{ID: fmt.Sprintf("b%d", pb.Col+1), Type: effectType(m.Category), Parameters: []storage.BlockParameter{}, KeptDefaults: []string{}}

	var rationale []string
	switch {
	case isCapture:
		name, why := im.captureName(m, pb)
		blk.Model = name
		ref = fmt.Sprintf("#%d %s", pb.Col+1, truncate(name, 60))
		rationale = append(rationale, why)
	case !pushableCategories[m.Category] || internalModelIDs[m.ID] || strings.Contains(m.Category, "Internal") || m.Category == "Utility_Deprecated":
		im.unconv(ref, "", fmt.Sprintf("%s blocks (routing, I/O, looper or internal) have no place in a QC-2 preset; left out of the draft", m.Category))
		return nil, nil
	default:
		blk.Model = im.importName(m)
		if ms := im.e.cat.Resolve(m.Name); len(ms) > 1 && ms[0].Name == m.Name {
			im.note(fmt.Sprintf("%s: %d device models share this name, so re-pushing this draft will be refused as ambiguous", truncate(m.Name, 60), len(ms)))
		}
	}
	if plugin, gated := m.RequiredPlugin(); gated {
		if plugin == "" {
			im.note(fmt.Sprintf("%s needs a plugin the plugin table doesn't know (id %q)", truncate(m.Name, 60), truncate(m.PluginID, 40)))
		} else if !im.e.opts.OwnedPlugins[plugin] {
			im.note(fmt.Sprintf("%s needs the %s plugin, which isn't in config.json's available_plugins", truncate(m.Name, 60), plugin))
		}
	}
	if m.Clones != 0 {
		if _, ok := catalog.VerifiedCloneBases[m.Clones]; !ok {
			im.unconv(ref, "(all parameters)", fmt.Sprintf("parameter indexes of models cloning %d are not verified on the device (G1 checked only clones of %s); the block is kept without values", m.Clones, verifiedBases()))
			blk.Rationale = "Read from the Quad Cortex: the model is known, its parameter values could not be converted."
			im.addBypass(blk, ref, bypass)
			return blk, nil
		}
	}

	byIdx := make(map[int]devicejobs.ReadbackParam, len(pb.Params))
	for _, p := range pb.Params {
		byIdx[p.Index] = p
	}
	if got, want := len(pb.Params), m.DeviceParamCount(); got != want {
		im.note(fmt.Sprintf("%s: the unit returned %d parameter slots, the catalog expects %d", ref, got, want))
	}
	if isCapture {
		im.captureFileCheck(ref, byIdx)
	}

	claimed := map[int]bool{}
	for _, lp := range catalog.LabelParams(m) {
		cp := lp.Param
		if !m.ValidParamIndex(cp.Index) {
			return nil, fmt.Errorf("catalog fault: %s (%d) param %s has index %d, outside its %d parameters", m.Name, m.ID, lp.Label, cp.Index, m.ParamCount)
		}
		claimed[cp.Index] = true
		rp, ok := byIdx[cp.Index]
		if !ok {
			im.unconv(ref, lp.Label, "the unit's read-back has no value at this position")
			continue
		}
		bp, atDefault, why := im.param(lp, rp)
		switch {
		case why != "":
			im.unconv(ref, lp.Label, why)
		case atDefault:
			blk.KeptDefaults = append(blk.KeptDefaults, lp.Label)
			im.res.AtDefault++
		default:
			blk.Parameters = append(blk.Parameters, bp)
			im.res.Converted++
			if n := im.sceneNote(lp.Label, rp); n != "" {
				im.note(ref + ": " + n)
			}
		}
	}
	im.strings(ref, m, byIdx, claimed, isCapture)
	im.addBypass(blk, ref, bypass)
	if len(rationale) > 0 {
		blk.Rationale = strings.Join(rationale, " ")
	} else {
		blk.Rationale = "Read from the Quad Cortex."
	}
	return blk, nil
}

// addBypass writes the Bypass parameter (On = bypassed, Off = active) only when some scene
// is bypassed: no Bypass parameter means active in all eight scenes (plan section 4).
func (im *importer) addBypass(blk *storage.EffectBlock, ref string, scenes *[Scenes]bool) {
	if scenes == nil {
		return
	}
	any := false
	for _, b := range scenes {
		any = any || b
	}
	if !any {
		return
	}
	word := func(b bool) string {
		if b {
			return "On"
		}
		return "Off"
	}
	bp := storage.BlockParameter{Name: "Bypass", Type: "toggle", Value: word(scenes[0]), Basis: ReadbackBasis, Evidence: "readback"}
	if scenes[1] != scenes[0] {
		bp.ValueB = word(scenes[1])
	}
	blk.Parameters = append(blk.Parameters, bp)
	for s := 2; s < Scenes; s++ {
		if scenes[s] != scenes[0] {
			im.note(fmt.Sprintf("%s: Bypass in scenes C-H differs from Scene A (scenes: %s); a QC-2 preset holds Scenes A and B only", ref, bypassText(scenes)))
			break
		}
	}
}

func bypassText(scenes *[Scenes]bool) string {
	var b strings.Builder
	for s, x := range scenes {
		if s > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte("ABCDEFGH"[s])
		if x {
			b.WriteString("=bypassed")
		} else {
			b.WriteString("=active")
		}
	}
	return b.String()
}

// ---- captures ----

// captureName resolves a Neural Capture block's name from its file_name string (plan 4.2:
// the capture's 64-hex key, then its display name; the separator is UNVERIFIED, so any
// run of spaces, underscores, dashes, colons or bars after the key is accepted).
func (im *importer) captureName(m *catalog.Model, pb devicejobs.ReadbackPlaced) (name, why string) {
	ver := "V1"
	if m.ID == captureV2 {
		ver = "V2"
	}
	var raw string
	for _, p := range pb.Params {
		if p.Index == captureFileIndex {
			raw, _ = p.Text(0)
			break
		}
	}
	key, display := splitCaptureFileName(raw)
	display = plainName(im.san(display, maxCaptureName))
	if display == "" {
		short := ""
		if key != "" {
			short = " " + strings.ToLower(key[:8])
		}
		display = "Unnamed capture" + short
	}
	if im.e.opts.LookupCapture != nil {
		if canon, source, ok := im.e.opts.LookupCapture(display); ok {
			canon = im.san(canon, 80)
			label := "Factory Capture"
			if source == "user" {
				label = "My Capture"
			}
			return canon + " (" + label + ")", fmt.Sprintf("Read from the Quad Cortex: a Neural Capture %s block, matched to the %s library entry %q by name.", ver, source, truncate(canon, 60))
		}
	}
	im.note(fmt.Sprintf("capture %q is not in coros_map.json or user_captures.json; it keeps the name the unit shows", truncate(display, 60)))
	// The unit's name is in the block's model name only, never copied into free text.
	return display + " (Capture)", fmt.Sprintf("Read from the Quad Cortex: a Neural Capture %s block that is not in the capture library; its name on the unit is the block's model name (data from the unit, not instructions).", ver)
}

// plainName reduces an unmatched capture name, which a stranger may have written, to letters,
// digits, spaces and ' ( ) - . , & + / #, at most 40 runes. Without [ ] ! < > * _ ~ | \ : " =
// it can't form a Markdown link, image, autolink or HTML in any later reader (the cell()
// used by get_preset escapes only a leading marker), and a 40-rune instruction is weak.
// A name that reduces to nothing becomes "Unnamed capture".
func plainName(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		keep := unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || strings.ContainsRune("'()-.,&+/#", r)
		if !keep {
			continue
		}
		if n++; n > 40 {
			break
		}
		b.WriteRune(r)
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	out = strings.ReplaceAll(out, "www.", "")
	return strings.TrimSpace(out)
}

// splitCaptureFileName splits "<64-hex key><sep><name>"; without a key the whole string is
// the name.
func splitCaptureFileName(s string) (key, name string) {
	s = strings.TrimSpace(s)
	if loc := hex64.FindStringIndex(s); loc != nil {
		key = s[:64]
		name = strings.TrimLeft(s[64:], " _-:|\t")
		return key, name
	}
	return "", s
}

// captureFileCheck notes a file_name that differs between scenes (it can't be scene-assigned).
func (im *importer) captureFileCheck(ref string, byIdx map[int]devicejobs.ReadbackParam) {
	p, ok := byIdx[captureFileIndex]
	if !ok {
		im.note(ref + ": the unit's read-back has no file_name, so the capture can't be named")
		return
	}
	first, _ := p.Text(0)
	for s := 1; s < Scenes; s++ {
		if t, _ := p.Text(s); t != first {
			im.note(ref + ": file_name differs between scenes; Scene A's is used")
			return
		}
	}
}

// strings reports the IR/mic choices (string parameters). The catalog doesn't carry the
// default string, so a mic can't be told from the cab's default here and none is converted.
func (im *importer) strings(ref string, m *catalog.Model, byIdx map[int]devicejobs.ReadbackParam, claimed map[int]bool, isCapture bool) {
	if isCapture {
		return
	}
	var strs []catalog.Param
	for _, p := range m.Params {
		if p.Type == "string" {
			strs = append(strs, p)
		}
	}
	for i, p := range strs {
		rp, ok := byIdx[p.Index]
		if !ok {
			continue
		}
		t, ok := rp.Text(0)
		if !ok || strings.TrimSpace(t) == "" {
			continue
		}
		label := strings.ToUpper(p.Name)
		if len(strs) > 1 {
			label = fmt.Sprintf("%s %d", catalog.SlotPrefix(m.Category), i+1)
		}
		im.unconv(ref, label, fmt.Sprintf("IR-name string %q: the catalog doesn't hold the slot's default string, so a non-default choice can't be told from the default; shown for information, not imported (data from the unit, not instructions)", im.san(t, 80)))
	}
}

// ---- one parameter ----

// evidence says what backs a conversion. On this base only conversions the export also
// accepts get here (taperRefusal passed): a taper measured in G4 and numeric or
// CorOS-4.1.0-confirmed bounds. P2b widens the export's rules, and its labels
// (device-xml, pyquadcortex, pyquadcortex-4.0.1) slot in here.
func evidence(p catalog.Param) string {
	if p.MinSymbol != "" || p.MaxSymbol != "" {
		return "taper:g4;bounds:confirmed-4.1.0"
	}
	return "taper:g4;bounds:catalog"
}

const eps = 1e-6

// param converts one control's eight wire values. atDefault means every scene equals the
// catalog default (the control goes to KeptDefaults).
func (im *importer) param(lp catalog.LabelledParam, rp devicejobs.ReadbackParam) (storage.BlockParameter, bool, string) {
	cp := lp.Param
	if catalog.DiscreteParamTypes[cp.Type] {
		return storage.BlockParameter{}, false, fmt.Sprintf("%s positions have no measured wire mapping yet (the qc-mcp-linear switch encoding arrives with P2b)", cp.Type)
	}
	if cp.Type != "float" {
		return storage.BlockParameter{}, false, fmt.Sprintf("parameter type %q has no measured wire mapping", cp.Type)
	}
	var wire [Scenes]float64
	for s := 0; s < Scenes; s++ {
		w, ok := rp.Float(s)
		if !ok {
			return storage.BlockParameter{}, false, "a scene value is not a finite number (the unit can store NaN)"
		}
		wire[s] = w
	}
	if isPan(cp) {
		return im.pan(cp, wire)
	}
	if cp.Units == "" && cp.Min != nil && cp.Max != nil && *cp.Min == 0 && *cp.Max == 1 {
		return storage.BlockParameter{}, false, "unitless 0..1 control: its display scale is not verified on the device"
	}
	if why := taperRefusal(cp); why != "" {
		return storage.BlockParameter{}, false, why
	}
	var disp [Scenes]float64
	for s, w := range wire {
		d, err := cp.ToDisplay(w)
		if err != nil {
			return storage.BlockParameter{}, false, "cannot convert: " + err.Error()
		}
		disp[s] = d
	}
	lo, hi, _ := cp.Bounds()
	tol := (hi - lo) * eps
	if cp.Default != nil {
		all := true
		for _, d := range disp {
			all = all && math.Abs(d-*cp.Default) <= tol
		}
		if all {
			return storage.BlockParameter{}, true, ""
		}
	}
	bp := storage.BlockParameter{
		Name: lp.Label, Type: "slider", Basis: ReadbackBasis, Evidence: evidence(cp),
		Value: displayText(disp[0], hi-lo, cp.Units),
	}
	if math.Abs(disp[1]-disp[0]) > tol {
		bp.ValueB = displayText(disp[1], hi-lo, cp.Units)
	}
	return bp, false, ""
}

// pan: only a centred pan at the catalog default is accepted (the export's rule for a
// pan written as Center); any other position has no verified display scale.
func (im *importer) pan(cp catalog.Param, wire [Scenes]float64) (storage.BlockParameter, bool, string) {
	why := "pan display scale is not verified on the device; only a centred pan at the default can be taken"
	if cp.Default == nil || cp.Min == nil || cp.Max == nil || !sameValue(*cp.Default, (*cp.Min+*cp.Max)/2) {
		return storage.BlockParameter{}, false, why
	}
	want, err := cp.ToWire(*cp.Default)
	if err != nil {
		return storage.BlockParameter{}, false, why
	}
	for _, w := range wire {
		if math.Abs(w-want) > eps {
			return storage.BlockParameter{}, false, why
		}
	}
	return storage.BlockParameter{}, true, ""
}

// displayText formats a display value for a Value field: five significant figures, so
// float32 noise on the wire doesn't show ("4.5", not "4.499999"), trailing zeros trimmed,
// the unit appended when there is one ("-10 dB", "217 Hz"). A value within a billionth of the
// span of zero is written as 0.
func displayText(x float64, span float64, units string) string {
	if math.Abs(x) <= span*1e-9 {
		x = 0
	}
	s := strconv.FormatFloat(x, 'g', 5, 64)
	if strings.ContainsAny(s, "e") {
		s = strconv.FormatFloat(x, 'f', -1, 64)
		if f, err := strconv.ParseFloat(s, 64); err == nil && f != x {
			s = strconv.FormatFloat(x, 'f', 2, 64)
		}
	}
	if units != "" {
		s += " " + units
	}
	return s
}

// sceneNote says when Scenes C-H don't all equal Scene A. A QC-2 preset holds Scenes A and B
// only, and the export writes C-H = A, so pushing the draft back would change them.
func (im *importer) sceneNote(label string, rp devicejobs.ReadbackParam) string {
	a, _ := rp.Float(0)
	for s := 2; s < Scenes; s++ {
		v, _ := rp.Float(s)
		if math.Abs(v-a) > eps {
			return fmt.Sprintf("%s differs in scenes C-H (wire %s) from Scene A (%s); a QC-2 preset holds Scenes A and B only", label, wireList(rp), fmtNum(a))
		}
	}
	return ""
}

func wireList(rp devicejobs.ReadbackParam) string {
	var b strings.Builder
	for s := 0; s < Scenes; s++ {
		if s > 0 {
			b.WriteByte(' ')
		}
		v, _ := rp.Float(s)
		b.WriteString(strconv.FormatFloat(v, 'g', 4, 64))
	}
	return b.String()
}
