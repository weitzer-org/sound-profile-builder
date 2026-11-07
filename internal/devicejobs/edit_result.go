package devicejobs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
)

// The qc-device-edit-result/1 payload (contract section 9.3): what an edit_preset job did on the
// unit. QC-2 reads results leniently, but this payload strictly, like the read-back: unknown
// fields, trailing data, wrong types and counts or values outside the geometry are refused, and
// error text never quotes the payload. Matching each entry to the plan QC-2 sent (same cells,
// same indexes) is phase E4's.

// EditResultFormat is the format string of Result.Edit.Format.
const EditResultFormat = "qc-device-edit-result/1"

// Bounds of the edit result.
const (
	// MaxEditResultBytes bounds the payload before decoding.
	MaxEditResultBytes = 256 << 10
	// MaxCollateralItems bounds the collateral mismatches listed (the count is complete).
	MaxCollateralItems = 64
	// maxCollateralParams: every parameter of every cell (4 x 8 cells x 256 slots).
	maxCollateralParams = ReadbackRows * ReadbackCols * MaxReadbackParams
	// maxCollateralBypass: every bypass flag (4 x 8 cells x 8 scenes).
	maxCollateralBypass = ReadbackRows * ReadbackCols * ReadbackScenes
	// maxRollbackOps: every op of a plan.
	maxRollbackOps = MaxEditParamOps + MaxEditBypassOps
)

// CAS failure names, in the order the contract lists them.
var casPins = map[string]bool{"folder_key": true, "position": true, "name": true, "layout": true,
	"param_expect": true, "bypass_expect": true}

// EditResult is the payload.
type EditResult struct {
	PlanSHA256 string           `json:"plan_sha256"`
	Snapshot   string           `json:"snapshot"`
	Mode       string           `json:"mode"`
	CAS        EditCAS          `json:"cas"`
	Params     []EditParamRead  `json:"params"`
	Bypass     []EditBypassRead `json:"bypass"`
	Collateral *EditCollateral  `json:"collateral"`
	Rollback   EditRollback     `json:"rollback"`
	Save       EditSave         `json:"save"`
}

// EditCAS is the compare-and-swap against a fresh read before the first write. State is
// "matched", "mismatch" or "not_reached" (the job stopped before it); Failed names the pins that
// differed.
type EditCAS struct {
	State  string   `json:"state"`
	Failed []string `json:"failed"`
}

// EditParamRead is one param op's read-back after the write: the eight scene values (null for a
// non-finite or non-numeric value) and whether they equal the op's wire values.
type EditParamRead struct {
	Row      int        `json:"row"`
	Col      int        `json:"col"`
	Index    int        `json:"index"`
	Readback []*float64 `json:"readback"`
	OK       bool       `json:"ok"`
}

// EditBypassRead is one bypass op's read-back (null when the unit sent no bypass states).
type EditBypassRead struct {
	Row      int     `json:"row"`
	Col      int     `json:"col"`
	Readback *[]bool `json:"readback"`
	OK       bool    `json:"ok"`
}

// EditCollateral is the check that nothing outside the plan changed, on all rows: every parameter
// value (all eight scenes) and bypass flag not in the plan, compared with the pre-write read, and
// the layout. Items lists up to MaxCollateralItems differences: Index null = a bypass flag, Scene
// null = the cell's model changed.
type EditCollateral struct {
	ParamsCompared int                  `json:"params_compared"`
	BypassCompared int                  `json:"bypass_compared"`
	LayoutSame     bool                 `json:"layout_same"`
	Mismatches     int                  `json:"mismatches"`
	Items          []EditCollateralItem `json:"items"`
}

// EditCollateralItem is one difference the collateral check found.
type EditCollateralItem struct {
	Row   int  `json:"row"`
	Col   int  `json:"col"`
	Index *int `json:"index"`
	Scene *int `json:"scene"`
}

// EditRollback is the automatic rollback after a failed verify. State is "not_needed", "restored",
// "partial" or "failed".
type EditRollback struct {
	State    string `json:"state"`
	Restored int    `json:"restored"`
	Failed   int    `json:"failed"`
}

// EditSave is what was saved. Backup is null until phase E5's backup copy.
type EditSave struct {
	Saved  bool        `json:"saved"`
	Slot   *string     `json:"slot"`
	Name   *string     `json:"name"`
	Listed bool        `json:"listed"`
	Backup *EditBackup `json:"backup"`
}

// EditBackup is where an overwrite's original went (phase E5).
type EditBackup struct {
	Slot       string `json:"slot"`
	Collection string `json:"collection"`
}

// ParseEditResultPayload decodes and checks an edit result payload.
func ParseEditResultPayload(raw json.RawMessage) (*EditResult, error) {
	if len(raw) == 0 {
		return nil, errors.New("edit result is empty")
	}
	if len(raw) > MaxEditResultBytes {
		return nil, fmt.Errorf("edit result is larger than %d bytes", MaxEditResultBytes)
	}
	if err := checkStrictJSON(raw); err != nil {
		return nil, errors.New("edit result is not strict JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var r EditResult
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("edit result does not match %s: %s", EditResultFormat, shortErr(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("edit result has data after the object")
	}
	if err := requireKeys(raw); err != nil {
		return nil, err
	}
	if err := r.validate(); err != nil {
		return nil, errors.New("edit result: " + err.Error())
	}
	return &r, nil
}

// requireKeys: the struct decoder accepts missing keys (zero values), matches keys without regard
// to case, and decodes null into a non-pointer field as its zero value (a null bypass flag would
// read as "not bypassed"). So check every object's exact key set, spelled exactly, and that only
// the keys the contract lets be null are null. A missing, case-variant or wrongly null key is as
// wrong as an unknown one. The shape walk is requireKeys; the two primitives are requireObj and
// requireList.
func requireKeys(raw json.RawMessage) error {
	top, err := requireObj(raw, "the payload", []string{"collateral"},
		"plan_sha256", "snapshot", "mode", "cas", "params", "bypass", "collateral", "rollback", "save")
	if err != nil {
		return err
	}
	cas, err := requireObj(top["cas"], "cas", nil, "state", "failed")
	if err != nil {
		return err
	}
	if _, err := requireList(cas["failed"], "cas.failed", false); err != nil {
		return err
	}
	params, err := requireList(top["params"], "params", false)
	if err != nil {
		return err
	}
	for _, e := range params {
		p, err := requireObj(e, "a param read-back", nil, "row", "col", "index", "readback", "ok")
		if err != nil {
			return err
		}
		if _, err := requireList(p["readback"], "a param read-back", true); err != nil { // a value may be null
			return err
		}
	}
	bypass, err := requireList(top["bypass"], "bypass", false)
	if err != nil {
		return err
	}
	for _, e := range bypass {
		b, err := requireObj(e, "a bypass read-back", []string{"readback"}, "row", "col", "readback", "ok")
		if err != nil {
			return err
		}
		if !isJSONNull(b["readback"]) {
			if _, err := requireList(b["readback"], "a bypass read-back", false); err != nil { // a flag is never null
				return err
			}
		}
	}
	if !isJSONNull(top["collateral"]) {
		c, err := requireObj(top["collateral"], "collateral", nil, "params_compared", "bypass_compared", "layout_same", "mismatches", "items")
		if err != nil {
			return err
		}
		items, err := requireList(c["items"], "collateral.items", false)
		if err != nil {
			return err
		}
		for _, e := range items {
			if _, err := requireObj(e, "a collateral item", []string{"index", "scene"}, "row", "col", "index", "scene"); err != nil {
				return err
			}
		}
	}
	if _, err := requireObj(top["rollback"], "rollback", nil, "state", "restored", "failed"); err != nil {
		return err
	}
	save, err := requireObj(top["save"], "save", []string{"slot", "name", "backup"}, "saved", "slot", "name", "listed", "backup")
	if err != nil {
		return err
	}
	if !isJSONNull(save["backup"]) {
		if _, err := requireObj(save["backup"], "save.backup", nil, "slot", "collection"); err != nil {
			return err
		}
	}
	return nil
}

func shapeErr(where string) error {
	return fmt.Errorf("edit result: %s must have exactly its fields, non-null where required", where)
}

func isJSONNull(v json.RawMessage) bool { return string(bytes.TrimSpace(v)) == "null" }

// requireObj decodes v as an object with exactly keys (spelled exactly); a key not in nullable
// must not be null.
func requireObj(v json.RawMessage, where string, nullable []string, keys ...string) (map[string]json.RawMessage, error) {
	var o map[string]json.RawMessage
	if json.Unmarshal(v, &o) != nil || o == nil || len(o) != len(keys) {
		return nil, shapeErr(where)
	}
	for _, k := range keys {
		val, ok := o[k]
		if !ok || (isJSONNull(val) && !slices.Contains(nullable, k)) {
			return nil, shapeErr(where)
		}
	}
	return o, nil
}

// requireList decodes v as an array (never null) whose elements are non-null unless elemNullable.
func requireList(v json.RawMessage, where string, elemNullable bool) ([]json.RawMessage, error) {
	var a []json.RawMessage
	if isJSONNull(v) || json.Unmarshal(v, &a) != nil {
		return nil, shapeErr(where)
	}
	if !elemNullable && slices.ContainsFunc(a, isJSONNull) {
		return nil, shapeErr(where)
	}
	return a, nil
}

func inRange(x, lo, hi int) bool { return x >= lo && x <= hi }

func (r *EditResult) validate() error {
	if !sha256Re.MatchString(r.PlanSHA256) {
		return errors.New("plan_sha256 must be 64 lowercase hex characters")
	}
	if !ValidJobID(r.Snapshot) {
		return errors.New("snapshot must be a lowercase UUID")
	}
	switch r.Mode {
	case EditModeLive, EditModeCopy, EditModeOverwrite:
	default:
		return errors.New("mode must be live, copy or overwrite")
	}
	switch r.CAS.State {
	case "matched", "mismatch", "not_reached":
	default:
		return errors.New("cas.state is not a known state")
	}
	if r.CAS.Failed == nil || len(r.CAS.Failed) > len(casPins) {
		return errors.New("cas.failed must be a list of at most 6 pins")
	}
	seen := map[string]bool{}
	for _, f := range r.CAS.Failed {
		if !casPins[f] || seen[f] {
			return errors.New("cas.failed holds an unknown or repeated pin")
		}
		seen[f] = true
	}
	if (r.CAS.State == "mismatch") != (len(r.CAS.Failed) > 0) {
		return errors.New("cas.failed must be non-empty exactly when cas.state is mismatch")
	}
	if r.Params == nil || len(r.Params) > MaxEditParamOps {
		return fmt.Errorf("params must be a list of at most %d", MaxEditParamOps)
	}
	for _, p := range r.Params {
		if p.Row != EditRow || !inRange(p.Col, 0, ReadbackCols-1) || !inRange(p.Index, 0, MaxReadbackParams-1) {
			return errors.New("a param read-back is outside the grid")
		}
		if len(p.Readback) != ReadbackScenes {
			return errors.New("a param read-back must have 8 values")
		}
		for _, v := range p.Readback {
			if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0)) {
				return errors.New("a param read-back value is not finite")
			}
		}
	}
	if r.Bypass == nil || len(r.Bypass) > MaxEditBypassOps {
		return fmt.Errorf("bypass must be a list of at most %d", MaxEditBypassOps)
	}
	for _, b := range r.Bypass {
		if b.Row != EditRow || !inRange(b.Col, 0, ReadbackCols-1) {
			return errors.New("a bypass read-back is outside the grid")
		}
		if b.Readback != nil && len(*b.Readback) != ReadbackScenes {
			return errors.New("a bypass read-back must have 8 flags")
		}
	}
	if c := r.Collateral; c != nil {
		if !inRange(c.ParamsCompared, 0, maxCollateralParams) || !inRange(c.BypassCompared, 0, maxCollateralBypass) ||
			!inRange(c.Mismatches, 0, maxCollateralParams+maxCollateralBypass+MaxEditLayout) {
			return errors.New("a collateral count is out of range")
		}
		if c.Items == nil || len(c.Items) > MaxCollateralItems || len(c.Items) > c.Mismatches {
			return fmt.Errorf("collateral.items must be a list of at most %d, and no more than mismatches", MaxCollateralItems)
		}
		if c.Mismatches == 0 && !c.LayoutSame {
			return errors.New("a changed layout is a mismatch")
		}
		for _, it := range c.Items {
			if !inRange(it.Row, 0, ReadbackRows-1) || !inRange(it.Col, 0, ReadbackCols-1) ||
				(it.Index != nil && !inRange(*it.Index, 0, MaxReadbackParams-1)) ||
				(it.Scene != nil && !inRange(*it.Scene, 0, ReadbackScenes-1)) {
				return errors.New("a collateral item is outside the grid")
			}
		}
	}
	switch r.Rollback.State {
	case "not_needed", "restored", "partial", "failed":
	default:
		return errors.New("rollback.state is not a known state")
	}
	if !inRange(r.Rollback.Restored, 0, maxRollbackOps) || !inRange(r.Rollback.Failed, 0, maxRollbackOps) {
		return errors.New("a rollback count is out of range")
	}
	rb := r.Rollback
	if !((rb.State == "not_needed" && rb.Restored == 0 && rb.Failed == 0) ||
		(rb.State == "restored" && rb.Failed == 0) ||
		(rb.State == "failed" && rb.Restored == 0 && rb.Failed > 0) ||
		(rb.State == "partial" && rb.Restored > 0 && rb.Failed > 0)) {
		return errors.New("rollback counts do not fit its state")
	}
	s := r.Save
	if s.Slot != nil && !slotRe.MatchString(*s.Slot) {
		return errors.New("save.slot is not a slot")
	}
	if s.Name != nil && !ValidDeviceName(*s.Name) {
		return errors.New("save.name is not a valid preset name")
	}
	if s.Saved && (s.Slot == nil || s.Name == nil) {
		return errors.New("a saved edit names its slot and name")
	}
	if s.Saved && r.Mode == EditModeLive {
		return errors.New("a live edit is never saved")
	}
	if s.Saved && r.Rollback.State != "not_needed" {
		return errors.New("an edit that was rolled back is never saved")
	}
	if b := s.Backup; b != nil {
		if !slotRe.MatchString(b.Slot) || b.Collection == "" || len([]rune(b.Collection)) > 40 ||
			SanitizeText(b.Collection, 40) != b.Collection || strings.ContainsRune(b.Collection, '`') {
			return errors.New("save.backup is not a slot and a clean collection name")
		}
	}
	return nil
}
