package devicejobs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"unicode/utf8"
)

// Contract v2 (docs/design/device-job-contract.md sections 7.1-7.2): the edit_preset job.
//
// An edit job is signed under TagJobV2 and carries payload v = 2; the v1 kinds stay exactly as
// they were (v = 1 under TagJob). The edit plan travels as an exact JSON string plus its SHA-256,
// so Go and Python hash the same bytes, and the agent parses it strictly with the rules below.
// Nothing here talks to the unit: checks that need the catalog or the unit (is the index a real,
// writable control; is the cell locked; compare-and-swap against a fresh read) are the agent's,
// phase E4.

// Edit job constants.
const (
	// PayloadVersionEdit is the job payload v of an edit_preset job.
	PayloadVersionEdit = 2
	KindEditPreset     = "edit_preset"
	// EditFormat is the only plan format a v2 job may carry.
	EditFormat = "qc2-device-edit/1"
	// MaxEditPlanBytes bounds the plan string (UTF-8 bytes), the same cap as a push's export.
	MaxEditPlanBytes = MaxExportBytes
	// MaxEditParamOps bounds param_ops. The largest model has 56 parameters, so row 0 holds at
	// most 8 x 56 = 448 controls.
	MaxEditParamOps = 512
	// MaxEditBypassOps: at most one bypass op per row-0 column.
	MaxEditBypassOps = ReadbackCols
	// MaxEditLayout: every cell of the 4 x 8 grid.
	MaxEditLayout   = ReadbackRows * ReadbackCols
	MaxEditEvidence = 8
	// MaxEditText bounds expect.name and an op's label, in code points.
	MaxEditText = 64
	// EditRow is the only row an edit writes (decision D7).
	EditRow = 0
)

// Save modes (design section 5).
const (
	EditModeLive      = "live"
	EditModeCopy      = "copy"
	EditModeOverwrite = "overwrite"
)

var (
	sha256Re   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	corosRe    = regexp.MustCompile(`^[0-9]{1,3}(?:\.[0-9]{1,3}){0,3}$`)
	evidenceRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.:+-]{0,63}$`)
	integerRe  = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)$`)
)

// ---- the plan: qc2-device-edit/1 ----

// EditPayload is the qc2-device-edit/1 document (internal/deviceedit emits it; it uses type
// aliases to these types, so the two can't drift). Fields in this order: the JSON bytes, and so
// the plan hash, are deterministic.
type EditPayload struct {
	Format    string         `json:"format"`
	Catalog   EditCatalogRef `json:"catalog"`
	Snapshot  string         `json:"snapshot"` // the read job's id
	Expect    EditExpect     `json:"expect"`
	ParamOps  []EditParamOp  `json:"param_ops"`
	BypassOps []EditBypassOp `json:"bypass_ops"`
}

// EditCatalogRef names the catalog the ids and indexes come from.
type EditCatalogRef struct {
	CorOS  string `json:"coros"`
	SHA256 string `json:"sha256"`
}

// EditExpect is what the agent must find on the unit before the first write.
type EditExpect struct {
	// FolderKey is the open collection's key (qc-device-readback/2). "" only in a plan computed
	// from a /1 read, which no parser accepts: such a plan is never confirmable.
	FolderKey string `json:"folder_key"`
	Position  int    `json:"position"`
	// Slot is SlotLabel(Position).
	Slot string `json:"slot"`
	// Name is the preset's name on the unit: device text, compared and never written.
	Name string `json:"name"`
	// Layout is [row, col, model_id] for every block on every row, sorted by (row, col).
	Layout [][3]int `json:"layout"`
}

// EditParamOp sets one control's eight scene values. Expect is what the snapshot has (compare-
// and-swap); Wire is what is written.
type EditParamOp struct {
	Row       int                     `json:"row"`
	Col       int                     `json:"col"`
	ModelID   int                     `json:"model_id"`
	Index     int                     `json:"index"`
	Label     string                  `json:"label"`
	Evidence  []string                `json:"evidence"`
	SceneMode bool                    `json:"scene_mode"`
	Expect    [ReadbackScenes]float64 `json:"expect"`
	Wire      [ReadbackScenes]float64 `json:"wire"`
}

// EditBypassOp sets a block's bypass flag in all eight scenes. Why is "target" or "removed".
type EditBypassOp struct {
	Row     int                  `json:"row"`
	Col     int                  `json:"col"`
	ModelID int                  `json:"model_id"`
	Why     string               `json:"why"`
	Expect  [ReadbackScenes]bool `json:"expect"`
	Bypass  [ReadbackScenes]bool `json:"bypass"`
}

// EditPlanSHA256 is the plan hash: lowercase hex SHA-256 of UTF-8(format) || 0x0A || UTF-8(text).
// text is the plan's exact JSON text, as the job carries it.
func EditPlanSHA256(format string, text []byte) string {
	h := sha256.New()
	h.Write([]byte(format))
	h.Write([]byte{'\n'})
	h.Write(text)
	return hex.EncodeToString(h.Sum(nil))
}

// ---- the job's args ----

// EditArgs are an edit_preset job's args (contract section 7.1).
type EditArgs struct {
	Slot string `json:"slot"`
	Mode string `json:"mode"`
	// TargetSlot and Name are set for copy only (null otherwise).
	TargetSlot *string        `json:"target_slot"`
	Name       *string        `json:"name"`
	Source     PushSource     `json:"source"`
	Plan       EditPlanRecord `json:"plan"`
}

// EditPlanRecord carries the plan as its exact JSON text and that text's hash.
type EditPlanRecord struct {
	Format  string `json:"format"`
	SHA256  string `json:"sha256"`
	Payload string `json:"payload"`
}

// NewEditArgs builds an edit job's args from a plan's JSON text (as json.Marshal of the plan's
// EditPayload wrote it) and checks them exactly as the agent will.
func NewEditArgs(slot, mode string, targetSlot, name *string, src PushSource, planText []byte) (*EditArgs, error) {
	a := &EditArgs{Slot: slot, Mode: mode, TargetSlot: targetSlot, Name: name, Source: src,
		Plan: EditPlanRecord{Format: EditFormat, SHA256: EditPlanSHA256(EditFormat, planText), Payload: string(planText)}}
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	if _, err := ParseEditArgs(raw); err != nil {
		return nil, err
	}
	return a, nil
}

// ParseEditArgs checks an edit_preset job's args (contract section 7.1): the exact key set, slot
// and mode, target_slot and name as the mode requires, source, the plan record, the plan's hash,
// and the plan itself (ParseEditPayload); and that the plan pins the job's slot.
func ParseEditArgs(raw json.RawMessage) (*EditPayload, error) {
	m, err := object(raw, "slot", "mode", "target_slot", "name", "source", "plan")
	if err != nil {
		return nil, err
	}
	slot, err := str(m["slot"], "slot")
	if err != nil {
		return nil, err
	}
	if !slotRe.MatchString(slot) {
		return nil, errors.New("bad slot")
	}
	mode, err := str(m["mode"], "mode")
	if err != nil {
		return nil, err
	}
	target, hasTarget, err := nullableStr(m["target_slot"], "target_slot")
	if err != nil {
		return nil, err
	}
	name, hasName, err := nullableStr(m["name"], "name")
	if err != nil {
		return nil, err
	}
	switch mode {
	case EditModeCopy:
		if !hasTarget || !slotRe.MatchString(target) || target == slot {
			return nil, errors.New("copy needs a target_slot other than slot")
		}
		if !hasName || !ValidDeviceName(name) {
			return nil, errors.New("copy needs a valid name")
		}
	case EditModeLive, EditModeOverwrite:
		if hasTarget || hasName {
			return nil, errors.New("target_slot and name must be null unless mode is copy")
		}
	default:
		return nil, errors.New("mode must be live, copy or overwrite")
	}
	src, err := object(m["source"], "preset_id", "guitar")
	if err != nil {
		return nil, fmt.Errorf("source: %v", err)
	}
	for _, f := range []string{"preset_id", "guitar"} {
		if _, err := str(src[f], "source."+f); err != nil {
			return nil, err
		}
	}
	pl, err := object(m["plan"], "format", "sha256", "payload")
	if err != nil {
		return nil, fmt.Errorf("plan: %v", err)
	}
	format, err := str(pl["format"], "plan.format")
	if err != nil {
		return nil, err
	}
	if format != EditFormat {
		return nil, errors.New("plan.format is not " + EditFormat)
	}
	sum, err := str(pl["sha256"], "plan.sha256")
	if err != nil {
		return nil, err
	}
	if !sha256Re.MatchString(sum) {
		return nil, errors.New("plan.sha256 must be 64 lowercase hex characters")
	}
	text, err := str(pl["payload"], "plan.payload")
	if err != nil {
		return nil, err
	}
	if len(text) > MaxEditPlanBytes {
		return nil, fmt.Errorf("plan.payload is over %d bytes", MaxEditPlanBytes)
	}
	if EditPlanSHA256(format, []byte(text)) != sum {
		return nil, errors.New("plan.sha256 does not match plan.payload")
	}
	p, err := ParseEditPayload([]byte(text))
	if err != nil {
		return nil, err
	}
	if p.Expect.Slot != slot {
		return nil, errors.New("the plan pins another slot")
	}
	return p, nil
}

func nullableStr(raw json.RawMessage, field string) (string, bool, error) {
	if string(bytes.TrimSpace(raw)) == "null" {
		return "", false, nil
	}
	s, err := str(raw, field)
	return s, err == nil, err
}

// ---- the plan's strict parse (contract section 7.2) ----

// ParseEditPayload strictly parses a qc2-device-edit/1 plan text: the section 5 JSON rules,
// exact key sets at every level, types and bounds, and the cross-field rules (every op's cell
// is in the layout with the same model, ops strictly ordered, no op that changes nothing, at
// least one op). Error text never quotes the plan.
func ParseEditPayload(text []byte) (*EditPayload, error) {
	if len(text) > MaxEditPlanBytes {
		return nil, fmt.Errorf("plan: over %d bytes", MaxEditPlanBytes)
	}
	if !utf8.Valid(text) {
		return nil, errors.New("plan: not valid UTF-8")
	}
	if checkStrictJSON(text) != nil { // its error can quote a key or a number: keep it out
		return nil, errors.New("plan: not strict JSON (section 5: one value, no duplicate keys, finite numbers, depth 32)")
	}
	if hasLoneSurrogate(text) {
		return nil, errors.New("plan: unpaired surrogate escape")
	}
	dec := json.NewDecoder(bytes.NewReader(text))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, errors.New("plan: invalid JSON")
	}
	w := walker{}
	top := w.obj(v, "plan", "format", "catalog", "snapshot", "expect", "param_ops", "bypass_ops")
	if w.err != nil {
		return nil, w.err
	}
	if w.str(top["format"], "format") != EditFormat {
		w.fail("plan.format is not " + EditFormat)
	}
	cat := w.obj(top["catalog"], "catalog", "coros", "sha256")
	if cat != nil {
		if !corosRe.MatchString(w.str(cat["coros"], "catalog.coros")) {
			w.fail("catalog.coros is not a version")
		}
		if !sha256Re.MatchString(w.str(cat["sha256"], "catalog.sha256")) {
			w.fail("catalog.sha256 must be 64 lowercase hex characters")
		}
	}
	if !ValidJobID(w.str(top["snapshot"], "snapshot")) {
		w.fail("snapshot must be a lowercase UUID")
	}
	cells := w.expect(top["expect"])
	w.paramOps(top["param_ops"], cells)
	w.bypassOps(top["bypass_ops"], cells)
	if w.err != nil {
		return nil, w.err
	}
	if w.ops == 0 {
		return nil, errors.New("plan: no ops")
	}
	var p EditPayload
	if err := json.Unmarshal(text, &p); err != nil { // the shape is already exact
		return nil, errors.New("plan: does not decode")
	}
	return &p, nil
}

// walker checks a decoded (UseNumber) JSON value; the first failure sticks.
type walker struct {
	err error
	ops int
}

func (w *walker) fail(msg string) {
	if w.err == nil {
		w.err = errors.New("plan: " + msg)
	}
}

func (w *walker) obj(v any, where string, keys ...string) map[string]any {
	m, ok := v.(map[string]any)
	if !ok || len(m) != len(keys) {
		w.fail(where + " must be an object with exactly the keys " + fmt.Sprint(keys))
		return nil
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			w.fail(where + " must be an object with exactly the keys " + fmt.Sprint(keys))
			return nil
		}
	}
	return m
}

func (w *walker) str(v any, where string) string {
	s, ok := v.(string)
	if !ok {
		w.fail(where + " must be a string")
	}
	return s
}

// integer: a JSON number with no fraction or exponent, in [lo, hi].
func (w *walker) integer(v any, where string, lo, hi int) int {
	n, ok := v.(json.Number)
	if !ok || !integerRe.MatchString(string(n)) {
		w.fail(where + " must be an integer")
		return lo
	}
	i, err := strconv.Atoi(string(n))
	if err != nil || i < lo || i > hi {
		w.fail(fmt.Sprintf("%s must be %d-%d", where, lo, hi))
		return lo
	}
	return i
}

func (w *walker) arr(v any, where string, lo, hi int) []any {
	a, ok := v.([]any)
	if !ok || len(a) < lo || len(a) > hi {
		w.fail(fmt.Sprintf("%s must be an array of %d-%d", where, lo, hi))
		return nil
	}
	return a
}

// unit: a finite number in [0, 1] (every value an edit writes or expects is a wire value).
func (w *walker) unit(v any, where string) float64 {
	n, ok := v.(json.Number)
	if !ok {
		w.fail(where + " must be numbers")
		return 0
	}
	f, err := strconv.ParseFloat(string(n), 64)
	if err != nil || f != f || f < 0 || f > 1 {
		w.fail(where + " must be numbers in 0..1")
		return 0
	}
	return f
}

// EditTextOK is the rule for expect.name and an op's label: 1 to MaxEditText code points, none
// of them a C0 or C1 control or U+2028/U+2029. It uses code point ranges only, so Go and Python
// can't disagree.
func EditTextOK(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	n := 0
	for _, r := range s {
		n++
		if r <= 0x1F || (r >= 0x7F && r <= 0x9F) || r == 0x2028 || r == 0x2029 {
			return false
		}
	}
	return n >= 1 && n <= MaxEditText
}

type cellKey struct{ row, col int }

func (w *walker) expect(v any) map[cellKey]int {
	ex := w.obj(v, "expect", "folder_key", "position", "slot", "name", "layout")
	if ex == nil {
		return nil
	}
	if !ValidFolderKey(w.str(ex["folder_key"], "expect.folder_key")) {
		w.fail("expect.folder_key is not a valid collection key")
	}
	pos := w.integer(ex["position"], "expect.position", 0, SlotCount-1)
	if w.str(ex["slot"], "expect.slot") != SlotLabel(pos) {
		w.fail("expect.slot is not the position's label")
	}
	if !EditTextOK(w.str(ex["name"], "expect.name")) {
		w.fail("expect.name must be 1-64 characters with no control characters")
	}
	cells := map[cellKey]int{}
	prev := -1
	for _, e := range w.arr(ex["layout"], "expect.layout", 1, MaxEditLayout) {
		t := w.arr(e, "expect.layout entry", 3, 3)
		if t == nil {
			return nil
		}
		row := w.integer(t[0], "layout row", 0, ReadbackRows-1)
		col := w.integer(t[1], "layout col", 0, ReadbackCols-1)
		id := w.integer(t[2], "layout model_id", 0, MaxReadbackModelID)
		if k := row*ReadbackCols + col; k <= prev {
			w.fail("expect.layout must be strictly ascending by (row, col)")
		} else {
			prev = k
		}
		cells[cellKey{row, col}] = id
	}
	return cells
}

// cell checks an op's row, col and model_id against the layout.
func (w *walker) cell(op map[string]any, where string, cells map[cellKey]int) int {
	row := w.integer(op["row"], where+".row", EditRow, EditRow)
	col := w.integer(op["col"], where+".col", 0, ReadbackCols-1)
	id := w.integer(op["model_id"], where+".model_id", 0, MaxReadbackModelID)
	if got, ok := cells[cellKey{row, col}]; !ok || got != id {
		w.fail(where + " is not on a cell of expect.layout with that model_id")
	}
	return col
}

func (w *walker) paramOps(v any, cells map[cellKey]int) {
	prev := -1
	for _, e := range w.arr(v, "param_ops", 0, MaxEditParamOps) {
		op := w.obj(e, "param op", "row", "col", "model_id", "index", "label", "evidence", "scene_mode", "expect", "wire")
		if op == nil {
			return
		}
		col := w.cell(op, "param op", cells)
		idx := w.integer(op["index"], "param op index", 0, MaxReadbackParams-1)
		if k := col*MaxReadbackParams + idx; k <= prev {
			w.fail("param_ops must be strictly ascending by (col, index)")
		} else {
			prev = k
		}
		if !EditTextOK(w.str(op["label"], "param op label")) {
			w.fail("param op label must be 1-64 characters with no control characters")
		}
		for _, ev := range w.arr(op["evidence"], "param op evidence", 0, MaxEditEvidence) {
			if !evidenceRe.MatchString(w.str(ev, "param op evidence")) {
				w.fail("param op evidence entries must match " + evidenceRe.String())
			}
		}
		sceneMode, ok := op["scene_mode"].(bool)
		if !ok {
			w.fail("param op scene_mode must be a boolean")
		}
		exp := w.arr(op["expect"], "param op expect", ReadbackScenes, ReadbackScenes)
		wire := w.arr(op["wire"], "param op wire", ReadbackScenes, ReadbackScenes)
		if exp == nil || wire == nil {
			return
		}
		changed, uniform := false, true
		first := w.unit(wire[0], "param op wire")
		for s := 0; s < ReadbackScenes; s++ {
			e, x := w.unit(exp[s], "param op expect"), w.unit(wire[s], "param op wire")
			changed = changed || e != x
			uniform = uniform && x == first
		}
		if !changed {
			w.fail("a param op must change at least one scene")
		}
		if !sceneMode && !uniform {
			w.fail("a param op with scene_mode false must write one value to all eight scenes")
		}
		w.ops++
	}
}

func (w *walker) bypassOps(v any, cells map[cellKey]int) {
	prev := -1
	for _, e := range w.arr(v, "bypass_ops", 0, MaxEditBypassOps) {
		op := w.obj(e, "bypass op", "row", "col", "model_id", "why", "expect", "bypass")
		if op == nil {
			return
		}
		col := w.cell(op, "bypass op", cells)
		if col <= prev {
			w.fail("bypass_ops must be strictly ascending by col")
		}
		prev = col
		why := w.str(op["why"], "bypass op why")
		if why != "target" && why != "removed" {
			w.fail("bypass op why must be target or removed")
		}
		exp := w.arr(op["expect"], "bypass op expect", ReadbackScenes, ReadbackScenes)
		set := w.arr(op["bypass"], "bypass op bypass", ReadbackScenes, ReadbackScenes)
		if exp == nil || set == nil {
			return
		}
		changed, all := false, true
		for s := 0; s < ReadbackScenes; s++ {
			e, ok1 := exp[s].(bool)
			b, ok2 := set[s].(bool)
			if !ok1 || !ok2 {
				w.fail("bypass op expect and bypass must be booleans")
				return
			}
			changed = changed || e != b
			all = all && b
		}
		if !changed {
			w.fail("a bypass op must change at least one scene")
		}
		if why == "removed" && !all {
			w.fail("a removed block is bypassed in all eight scenes")
		}
		w.ops++
	}
}
