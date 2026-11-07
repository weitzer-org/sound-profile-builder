package deviceedit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicelib"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Format is the payload schema this package writes.
const Format = "qc2-device-edit/1"

const (
	scenes       = deviceexport.Scenes
	maxItems     = 200
	maxViewText  = 80
	maxNameRunes = 64
)

// ---- the payload: everything that reaches the unit, and the only part that is hashed ----

// Payload is the qc2-device-edit/1 document. Structs only, fields in this order, so the bytes
// and the hash are deterministic.
type Payload struct {
	Format    string     `json:"format"`
	Catalog   CatalogRef `json:"catalog"`
	Snapshot  string     `json:"snapshot"` // the read job's id
	Expect    Expect     `json:"expect"`
	ParamOps  []ParamOp  `json:"param_ops"`
	BypassOps []BypassOp `json:"bypass_ops"`
}

// CatalogRef names the catalog the ids and indexes come from.
type CatalogRef struct {
	CorOS  string `json:"coros"`
	SHA256 string `json:"sha256"`
}

// Expect is what the agent must find on the unit before the first write.
type Expect struct {
	// Folder is "My Presets" or "other": all the read-back says (a real collection key needs a
	// contract field, phase E3).
	Folder   string `json:"folder"`
	Position int    `json:"position"`
	// Slot is the slot the bridge reported ("" if none).
	Slot string `json:"slot"`
	// Name is the preset's name on the unit: device text, compared and never written.
	Name string `json:"name"`
	// Layout is [row, col, model_id] for every block on every row, sorted.
	Layout [][3]int `json:"layout"`
}

// ParamOp sets one control's eight scene values. Expect is what the snapshot has (compare-and-swap).
type ParamOp struct {
	Row       int             `json:"row"`
	Col       int             `json:"col"`
	ModelID   int             `json:"model_id"`
	Index     int             `json:"index"`
	Label     string          `json:"label"`
	Evidence  []string        `json:"evidence"`
	SceneMode bool            `json:"scene_mode"`
	Expect    [scenes]float64 `json:"expect"`
	Wire      [scenes]float64 `json:"wire"`
}

// BypassOp sets a block's bypass flag in all eight scenes. Why is "target" or "removed".
type BypassOp struct {
	Row     int          `json:"row"`
	Col     int          `json:"col"`
	ModelID int          `json:"model_id"`
	Why     string       `json:"why"`
	Expect  [scenes]bool `json:"expect"`
	Bypass  [scenes]bool `json:"bypass"`
}

// ---- everything else: shown to the user, not hashed ----

// NotApplied is one change the plan did not make. Block and Param are sanitized text from the
// preset or the unit (render in code spans); Reason is the fixed text for Code.
type NotApplied struct {
	Code   string `json:"code"`
	Block  string `json:"block"`
	Param  string `json:"param,omitempty"`
	Reason string `json:"reason"`
}

// Untouched summarises what the plan leaves alone.
type Untouched struct {
	OtherRowsBlocks     int `json:"other_rows_blocks"`
	LockedCells         int `json:"locked_cells"`
	LockedControls      int `json:"locked_controls"`
	ControlsNotInTarget int `json:"controls_not_in_target"`
	KeptDefaultsIgnored int `json:"kept_defaults_ignored"`
	BlocksLeftAsIs      int `json:"blocks_left_as_is"`
}

// ViewRow is one before/after row of the dry-run table.
type ViewRow struct {
	Col         int    `json:"col"`
	Block       string `json:"block"`
	Control     string `json:"control"`
	BeforeA     string `json:"before_a"`
	AfterA      string `json:"after_a"`
	BeforeB     string `json:"before_b"`
	AfterB      string `json:"after_b"`
	Evidence    string `json:"evidence"`
	Unconfirmed bool   `json:"unconfirmed"`
	// Note is fixed text: "removed: bypassed in all 8 scenes, C-H included" and the like.
	Note string `json:"note,omitempty"`
}

// Plan is the planner's result.
type Plan struct {
	Payload         Payload
	SHA256          string
	NotApplied      []NotApplied
	NotAppliedCount int
	Untouched       Untouched
	View            []ViewRow
	// Matched counts target blocks matched to a cell.
	Matched int
}

// Ops is the number of writes in the plan.
func (p *Plan) Ops() int { return len(p.Payload.ParamOps) + len(p.Payload.BypassOps) }

// Input is what the planner needs.
type Input struct {
	Snapshot *devicelib.ReadSnapshot
	// Target is one guitar's chain.
	Target []storage.EffectBlock
	// Legacy: the target predates the Bypass definition (no parameter carries a basis).
	Legacy bool
	// Derived: the target is a draft imported from this very snapshot (or refined from it). Only
	// then does a block the target lacks become a bypass; any other target (a QC-2 original)
	// leaves blocks it doesn't mention alone.
	Derived bool
}

// Planner computes plans over one exporter.
type Planner struct{ e *deviceexport.Exporter }

// New returns a planner over the exporter's tables.
func New(e *deviceexport.Exporter) *Planner { return &Planner{e: e} }

type builder struct {
	p   *Planner
	in  Input
	out *Plan
}

func (b *builder) item(code, block, param string) {
	b.out.NotAppliedCount++
	if len(b.out.NotApplied) >= maxItems {
		return
	}
	b.out.NotApplied = append(b.out.NotApplied, NotApplied{
		Code: code, Block: devicejobs.SanitizeText(block, maxViewText), Param: devicejobs.SanitizeText(param, maxViewText), Reason: reasons[code]})
}

func targetRef(blk *storage.EffectBlock) string {
	id := strings.TrimSpace(blk.ID)
	if id == "" {
		return strings.TrimSpace(blk.Model)
	}
	return id + " " + strings.TrimSpace(blk.Model)
}

// Plan computes the edit plan of target T against snapshot S.
func (p *Planner) Plan(in Input) (*Plan, *PlanError) {
	e := p.e
	if e == nil || !e.Ready() {
		return nil, planErr(ErrCatalogUnavail, "the device catalog is unavailable")
	}
	snap := in.Snapshot
	if snap == nil {
		return nil, planErr(ErrSnapshotMissing, "there is no stored read of the unit for this preset; read it again with read_device_preset")
	}
	rb, err := snap.Readback()
	if err != nil {
		return nil, planErr(ErrSnapshotInvalid, "the stored read of the unit is not valid; read it again")
	}
	if rb.Setlist.Position == nil {
		return nil, planErr(ErrNoPosition, "the read did not say which slot was open, so the edit can't be pinned to it")
	}
	if n := snap.Name; n != "" && (utf8.RuneCountInString(n) > maxNameRunes || devicejobs.SanitizeText(n, maxNameRunes) != n) {
		return nil, planErr(ErrNameUnpinnable, "the preset's name on the unit has characters that can't be pinned safely")
	}
	coros, sha := e.CatalogInfo()
	if fw := snap.Firmware; fw != "" && fw != coros && !strings.HasPrefix(fw, coros+".") {
		return nil, planErr(ErrFirmwareMismatch, "the read came from firmware that doesn't match this server's device catalog")
	}

	out := &Plan{NotApplied: []NotApplied{}}
	out.Payload = Payload{
		Format: Format, Catalog: CatalogRef{CorOS: coros, SHA256: sha}, Snapshot: snap.JobID,
		Expect:   Expect{Folder: rb.Setlist.Folder, Position: *rb.Setlist.Position, Slot: snap.Slot, Name: snap.Name, Layout: layoutOf(rb)},
		ParamOps: []ParamOp{}, BypassOps: []BypassOp{},
	}
	b := &builder{p: p, in: in, out: out}

	cells := e.Cells(rb)
	var xs []deviceexport.Cell
	for _, c := range cells {
		if c.Exposed {
			xs = append(xs, c)
		} else {
			out.Untouched.LockedCells++
		}
	}
	for _, ch := range rb.Chains {
		if ch.Row != deviceexport.GridRow {
			out.Untouched.OtherRowsBlocks += len(ch.Blocks)
		}
	}

	// Targets: device elements are not grid blocks.
	var ts []tgt
	for i := range in.Target {
		blk := &in.Target[i]
		if deviceexport.IsDeviceElement(blk.Model) {
			b.item(CodeDeviceElement, targetRef(blk), "")
			continue
		}
		ts = append(ts, tgt{pos: i, blk: blk})
	}
	compat := make([][]bool, len(ts))
	for ti, t := range ts {
		compat[ti] = make([]bool, len(xs))
		for ci, c := range xs {
			compat[ti][ci] = e.SameModel(c, t.blk.Model)
		}
	}
	m := match(ts, xs, compat)

	matchedT := map[int]int{}
	matchedC := map[int]bool{}
	for _, pr := range m.pairs {
		matchedT[pr.t] = pr.c
		matchedC[pr.c] = true
	}
	out.Matched = len(m.pairs)

	claimed := map[int]bool{} // unmatched cells a swap or addition accounts for
	for ti, t := range ts {
		if _, ok := matchedT[ti]; ok {
			continue
		}
		ref := targetRef(t.blk)
		if m.ambiguousT[ti] {
			b.item(CodeMatchAmbiguous, ref, "")
			continue
		}
		switch e.ClassifyName(t.blk.Model) {
		case deviceexport.NameUnknown:
			b.item(CodeUnknownModel, ref, "")
			continue
		case deviceexport.NameNotEditable:
			b.item(CodeNotEditableBlock, ref, "")
			continue
		}
		// The same model sits unmatched elsewhere: the block was moved.
		moved := false
		for ci := range xs {
			if !matchedC[ci] && !m.ambiguousC[ci] && !claimed[ci] && compat[ti][ci] {
				moved = true
				claimed[ci] = true // the cell is where the block came from, not a block the target dropped
				break
			}
		}
		if moved {
			b.item(CodeReorder, ref, "")
			continue
		}
		// The gap between the neighbouring matches: unmatched cells there are what this block replaces.
		lo, hi := -1, len(xs)
		for tj := ti - 1; tj >= 0; tj-- {
			if c, ok := matchedT[tj]; ok {
				lo = c
				break
			}
		}
		for tj := ti + 1; tj < len(ts); tj++ {
			if c, ok := matchedT[tj]; ok {
				hi = c
				break
			}
		}
		var gap []int
		for ci := lo + 1; ci < hi; ci++ {
			if ci >= 0 && ci < len(xs) && !matchedC[ci] && !m.ambiguousC[ci] && !claimed[ci] {
				gap = append(gap, ci)
			}
		}
		if len(gap) == 0 {
			b.item(CodeNewBlock, ref, "")
			continue
		}
		code := CodeModelSwap
		if e.ClassifyName(t.blk.Model) == deviceexport.NameCapture && xs[gap[0]].Capture {
			code = CodeCaptureSwap
		}
		b.item(code, ref, "")
		for _, ci := range gap {
			claimed[ci] = true
		}
	}

	for _, pr := range m.pairs {
		if pr.moved {
			b.item(CodeReorder, targetRef(ts[pr.t].blk), "")
		}
		b.pair(xs[pr.c], ts[pr.t].blk, rb)
	}

	for ci, c := range xs {
		if matchedC[ci] || m.ambiguousC[ci] || claimed[ci] {
			if !matchedC[ci] {
				out.Untouched.BlocksLeftAsIs++
			}
			continue
		}
		if in.Derived {
			b.removal(c, rb)
		} else {
			out.Untouched.BlocksLeftAsIs++
			b.item(CodeCellNotInTarget, c.Name, "")
		}
	}

	sort.SliceStable(out.Payload.ParamOps, func(i, j int) bool {
		a, c := out.Payload.ParamOps[i], out.Payload.ParamOps[j]
		if a.Col != c.Col {
			return a.Col < c.Col
		}
		return a.Index < c.Index
	})
	sort.SliceStable(out.Payload.BypassOps, func(i, j int) bool { return out.Payload.BypassOps[i].Col < out.Payload.BypassOps[j].Col })
	sort.SliceStable(out.View, func(i, j int) bool { return out.View[i].Col < out.View[j].Col })

	raw, err := json.Marshal(out.Payload)
	if err != nil {
		return nil, planErr(ErrPlanTooLarge, "the plan could not be encoded")
	}
	if len(raw) > devicejobs.MaxExportBytes {
		return nil, planErr(ErrPlanTooLarge, fmt.Sprintf("the plan is %d bytes, over the %d limit", len(raw), devicejobs.MaxExportBytes))
	}
	h := sha256.New()
	h.Write([]byte(Format))
	h.Write([]byte{'\n'})
	h.Write(raw)
	out.SHA256 = hex.EncodeToString(h.Sum(nil))
	return out, nil
}

func layoutOf(rb *devicejobs.ReadbackPayload) [][3]int {
	out := [][3]int{}
	for _, ch := range rb.Chains {
		for _, bl := range ch.Blocks {
			out = append(out, [3]int{ch.Row, bl.Col, bl.ModelID})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

// bypassOf returns the snapshot's bypass flags of a row-0 cell, or nil when the unit sent none.
func bypassOf(rb *devicejobs.ReadbackPayload, col int) *[scenes]bool {
	if rb.Bypass == nil || deviceexport.GridRow >= len(rb.Bypass) || col >= len(rb.Bypass[deviceexport.GridRow]) {
		return nil
	}
	var a [scenes]bool
	copy(a[:], rb.Bypass[deviceexport.GridRow][col])
	return &a
}

func bypassWord(b bool) string {
	if b {
		return "Bypassed"
	}
	return "Active"
}

// removal bypasses a cell the (derived) target dropped, in all eight scenes (decision D4).
func (b *builder) removal(c deviceexport.Cell, rb *devicejobs.ReadbackPayload) {
	cur := bypassOf(rb, c.Col)
	if cur == nil {
		b.item(CodeBypassUnreadable, c.Name, "")
		return
	}
	all := [scenes]bool{true, true, true, true, true, true, true, true}
	if *cur == all {
		return
	}
	b.out.Payload.BypassOps = append(b.out.Payload.BypassOps, BypassOp{Row: deviceexport.GridRow, Col: c.Col, ModelID: c.ModelID, Why: "removed", Expect: *cur, Bypass: all})
	b.out.View = append(b.out.View, ViewRow{Col: c.Col + 1, Block: c.Name, Control: "Bypass",
		BeforeA: bypassWord(cur[0]), AfterA: "Bypassed", BeforeB: bypassWord(cur[1]), AfterB: "Bypassed",
		Evidence: "readback", Note: "removed from the target: bypassed in all 8 scenes, C-H included"})
}

// pair compares one matched target block with its cell.
func (b *builder) pair(c deviceexport.Cell, blk *storage.EffectBlock, rb *devicejobs.ReadbackPayload) {
	m := c.Model
	ref := targetRef(blk)
	pc := deviceexport.NewParamClassifier(m, c.Capture, blk.Type)
	byIdx := map[int]devicejobs.ReadbackParam{}
	for _, rp := range c.Placed.Params {
		byIdx[rp.Index] = rp
	}
	b.out.Untouched.KeptDefaultsIgnored += len(blk.KeptDefaults)
	named := map[int]bool{}
	locked := deviceexport.CloneLocked(m)
	lockedSaid := false
	written := map[int]bool{}
	var tA, tB bool
	haveBypass, bypassErr := false, false
	for i := range blk.Parameters {
		bp := blk.Parameters[i]
		cl := pc.Classify(bp.Name)
		switch cl.Class {
		case deviceexport.ParamBypass:
			a, aok := deviceexport.ParseBypass(bp.Value)
			bv, bok := deviceexport.ParseBypass(deviceexport.SceneB(bp))
			switch {
			case b.in.Legacy:
				b.item(CodeBypassLegacy, ref, bp.Name)
			case !aok || !bok:
				b.item(CodeBypassWord, ref, bp.Name)
				bypassErr = true // an unreadable word is no instruction: leave the unit's bypass alone
			default:
				tA, tB, haveBypass = a, bv, true
			}
			continue
		case deviceexport.ParamMic:
			b.mic(ref, m, c, bp, cl.Slot, byIdx)
			continue
		case deviceexport.ParamNotControl:
			b.item(CodeNotAControl, ref, bp.Name)
			continue
		case deviceexport.ParamInternal:
			b.item(CodeInternalControl, ref, bp.Name)
			continue
		case deviceexport.ParamCabBlend:
			b.item(CodeCabBlend, ref, bp.Name)
			continue
		}
		lp := cl.Label
		if written[lp.Param.Index] {
			b.item(CodeDuplicateControl, ref, bp.Name)
			continue
		}
		written[lp.Param.Index] = true
		named[lp.Param.Index] = true
		if locked {
			if !lockedSaid {
				lockedSaid = true
				b.item(CodeLockedClone, ref, "(all controls)")
			}
			b.out.Untouched.LockedControls++
			continue
		}
		b.control(c, ref, lp, bp, byIdx)
	}
	for _, lp := range pc.Labels {
		if !named[lp.Param.Index] {
			b.out.Untouched.ControlsNotInTarget++
		}
	}

	// Bypass: A and B from the target (active when it has no Bypass entry), C-H from the snapshot.
	cur := bypassOf(rb, c.Col)
	if cur == nil {
		if haveBypass || bypassErr {
			b.item(CodeBypassUnreadable, ref, "Bypass")
		}
		return
	}
	if bypassErr {
		return
	}
	if !haveBypass {
		tA, tB = false, false
		if b.in.Legacy {
			// A legacy target has no usable Bypass definition: leave bypass alone.
			return
		}
	}
	if tA == cur[0] && tB == cur[1] {
		return
	}
	next := *cur
	next[0], next[1] = tA, tB
	b.out.Payload.BypassOps = append(b.out.Payload.BypassOps, BypassOp{Row: deviceexport.GridRow, Col: c.Col, ModelID: c.ModelID, Why: "target", Expect: *cur, Bypass: next})
	b.out.View = append(b.out.View, ViewRow{Col: c.Col + 1, Block: c.Name, Control: "Bypass",
		BeforeA: bypassWord(cur[0]), AfterA: bypassWord(tA), BeforeB: bypassWord(cur[1]), AfterB: bypassWord(tB), Evidence: "readback"})
}

// mic: a cab's mic is never changed by an edit. The same mic (or its default) is a no-op.
func (b *builder) mic(ref string, m *catalog.Model, c deviceexport.Cell, bp storage.BlockParameter, slot int, byIdx map[int]devicejobs.ReadbackParam) {
	idx, def := deviceexport.MicDefault(m, slot)
	want := bp.Value
	if rp, ok := byIdx[idx]; ok && idx >= 0 {
		if t, ok := rp.Text(0); ok && t != "" {
			if deviceexport.SameMic(lastPart(t), want) && deviceexport.SameMic(lastPart(t), deviceexport.SceneB(bp)) {
				return
			}
			b.item(CodeMicNotWritten, ref, bp.Name)
			return
		}
	}
	if def != "" && deviceexport.SameMic(def, want) && deviceexport.SameMic(def, deviceexport.SceneB(bp)) {
		return
	}
	b.item(CodeMicNotWritten, ref, bp.Name)
}

func lastPart(s string) string {
	if i := strings.LastIndex(s, "_"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// control compares and, when different, plans one continuous or discrete control.
func (b *builder) control(c deviceexport.Cell, ref string, lp catalog.LabelledParam, bp storage.BlockParameter, byIdx map[int]devicejobs.ReadbackParam) {
	e := b.p.e
	rp, present := byIdx[lp.Param.Index]
	rd, lock := e.ReadControl(lp, rp, present)
	if lock != deviceexport.LockNone {
		b.out.Untouched.LockedControls++
		b.item(lockCode(lock), ref, bp.Name)
		return
	}
	cv, kind := e.ConvertTarget(lp, bp)
	if kind != deviceexport.ConvOK {
		b.item(convCode(kind), ref, bp.Name)
		return
	}
	same := true
	var sceneSame [2]bool
	for s := 0; s < 2; s++ {
		switch {
		case rd.Discrete != cv.Discrete:
		case rd.Discrete:
			sceneSame[s] = rd.Pos[s] == cv.Pos[s]
		default:
			sceneSame[s] = abs(rd.Disp[s]-cv.Disp[s]) <= deviceexport.ToleranceH(lp.Param, cv.Disp[s])
		}
		same = same && sceneSame[s]
	}
	if same {
		return
	}
	wire := rd.Wire // C-H exactly as the snapshot has them
	for s := 0; s < 2; s++ {
		if !sceneSame[s] { // a scene that is already right keeps the unit's exact value
			wire[s] = cv.Wire[s]
		}
	}
	sceneMode := rd.SceneMode
	for s := 1; s < scenes; s++ {
		if wire[s] != wire[0] {
			sceneMode = true
		}
	}
	b.out.Payload.ParamOps = append(b.out.Payload.ParamOps, ParamOp{
		Row: deviceexport.GridRow, Col: c.Col, ModelID: c.ModelID, Index: lp.Param.Index, Label: lp.Label,
		Evidence: cv.Evidence, SceneMode: sceneMode, Expect: rd.Wire, Wire: wire})
	b.out.View = append(b.out.View, ViewRow{Col: c.Col + 1, Block: c.Name, Control: lp.Label,
		BeforeA: rd.Shown[0], AfterA: cv.Shown[0], BeforeB: rd.Shown[1], AfterB: cv.Shown[1],
		Evidence: strings.Join(cv.Evidence, ";"), Unconfirmed: !cv.Confirmed})
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func lockCode(k deviceexport.LockKind) string {
	switch k {
	case deviceexport.LockNoWireMapping:
		return CodeLockedNoMapping
	case deviceexport.LockUnmeasured:
		return CodeLockedUnmeasured
	case deviceexport.LockNotWholePos:
		return CodeLockedNotWhole
	}
	return CodeLockedUnreadable
}

func convCode(k deviceexport.ConvKind) string {
	switch k {
	case deviceexport.ConvWrongUnit:
		return CodeValueWrongUnit
	case deviceexport.ConvOutOfRange:
		return CodeValueOutOfRange
	case deviceexport.ConvNotAPosition:
		return CodeValueNotPosition
	case deviceexport.ConvNoWireMapping:
		return CodeLockedNoMapping
	case deviceexport.ConvUnmeasured:
		return CodeLockedUnmeasured
	}
	return CodeValueUnparseable
}

// LegacyPreset reports whether a preset predates the Bypass definition (no parameter carries a
// basis), so its Bypass values can't be read.
func LegacyPreset(sp storage.StructuredPreset) bool { return !deviceexport.UsesBasis(sp) }
