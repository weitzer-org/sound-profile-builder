// Package deviceedit plans an edit of the preset open on the Quad Cortex (preset-editing plan,
// docs/design/qc-preset-editing.md, phase E1). Given a stored snapshot S of the unit's
// preset (the raw read-back) and a target T (one guitar's chain of a QC-2 preset), it produces
// a qc2-device-edit/1 plan: compare-and-swap set_param and set_bypass ops on row 0, a list of
// the changes it did not apply, each with a fixed reason, an untouched summary and a plan hash.
//
// It is pure and deterministic: no storage, no device, no LLM. It is built on internal/
// deviceexport's tables (its editapi.go) so a conversion rule can't drift between a push, the
// read-back import and an edit.
//
// Rules (settled in the E1 plan):
//
//   - Only row 0 is edited; other rows are never written. Cells the import never exposes
//     (unknown ids, routing, loopers, internal models) are locked: never matched, removed or
//     written.
//   - A target block is matched to a cell first by block id (the import writes b<col+1>) and the
//     same model, then by the unique maximum order-preserving alignment of model names between
//     those anchors. A tie matches nothing in its segment. A block matched by id that sits out of
//     order is a move: reported as reorder, but its parameter edits still apply, to the cell it
//     is matched with.
//   - A value is a no-op when the unit's value is within H of the target's in display space
//     (deviceexport.ToleranceH), which is a band in wire space because ToWire is monotonic.
//     Discrete controls compare whole positions.
//   - A op writes all eight scenes: A and B from the target, C-H exactly as the snapshot has them.
//     The one exception is a removed block, which is bypassed in all eight scenes (decision D4).
//   - kept_defaults entries are not an instruction (decision 3, owner discussion): they never
//     write. Controls the target omits are never written.
//   - Nothing from the device is put in an op except the preset name pinned in expect.
package deviceedit

// Codes of items that could not be applied. Each has one fixed reason text below; no device or
// preset text is ever interpolated into a reason.
const (
	CodeNewBlock         = "new_block"
	CodeModelSwap        = "model_swap"
	CodeCaptureSwap      = "capture_swap"
	CodeReorder          = "reorder"
	CodeMatchAmbiguous   = "match_ambiguous"
	CodeUnknownModel     = "unknown_model"
	CodeNotEditableBlock = "not_editable_block"
	CodeDeviceElement    = "device_element"
	CodeCellNotInTarget  = "cell_not_in_target"
	CodeBypassUnreadable = "bypass_unreadable"

	CodeNotAControl      = "not_a_control"
	CodeInternalControl  = "internal_control"
	CodeDuplicateControl = "duplicate_control"
	CodeCabBlend         = "cab_blend"
	CodeMicNotWritten    = "mic_not_written"
	CodeValueUnparseable = "value_unparseable"
	CodeValueWrongUnit   = "value_wrong_unit"
	CodeValueOutOfRange  = "value_out_of_range"
	CodeValueNotPosition = "value_not_a_position"
	CodeBypassWord       = "bypass_word"
	CodeBypassLegacy     = "bypass_legacy"

	CodeLockedClone      = "locked_clone"
	CodeLockedUnmeasured = "locked_unmeasured_bound"
	CodeLockedNoMapping  = "locked_no_wire_mapping"
	CodeLockedNotWhole   = "locked_not_whole_position"
	CodeLockedUnreadable = "locked_unreadable_value"
)

const pushNew = "Push as a new preset instead (send_preset_to_device, into an empty slot)."

// reasons is the fixed list. Tests assert every code the planner can emit is a key here.
var reasons = map[string]string{
	CodeNewBlock:         "A new block can't be added to a preset on the unit yet. " + pushNew,
	CodeModelSwap:        "Swapping a block's model isn't available yet (phase E6); the unit's block is left as it is. " + pushNew,
	CodeCaptureSwap:      "Swapping a capture isn't available yet (phase E6); the unit's block is left as it is. " + pushNew,
	CodeReorder:          "Blocks can't be moved on the unit. The block stays where it is; parameter edits to a block that could be matched still apply. " + pushNew,
	CodeMatchAmbiguous:   "More than one way to line this block up with the unit's blocks, so nothing was changed for it or for the unit blocks it could be.",
	CodeUnknownModel:     "This name is neither a Quad Cortex block nor a capture in the libraries, so there is nothing to edit.",
	CodeNotEditableBlock: "This kind of block (IR loader, routing, internal or side-chain model) is never edited.",
	CodeDeviceElement:    "The input gate, the output and the global EQ aren't grid blocks and are never edited.",
	CodeCellNotInTarget:  "The unit has a block the target doesn't mention. It is left as it is.",
	CodeBypassUnreadable: "The unit sent no bypass states, so bypass can't be compared or written.",

	CodeNotAControl:      "Not a control of the block the unit has in this cell.",
	CodeInternalControl:  "An internal device parameter; it is never written.",
	CodeDuplicateControl: "This control is written twice in the target.",
	CodeCabBlend:         "Device cabs have no Blend control; set MIC 1 LEVEL and MIC 2 LEVEL instead.",
	CodeMicNotWritten:    "A cab's mic can't be changed by an edit; the unit's mic is left as it is.",
	CodeValueUnparseable: "The target's value can't be read as a number for this control.",
	CodeValueWrongUnit:   "The target's value is written in a unit that doesn't fit this control.",
	CodeValueOutOfRange:  "The target's value is outside this control's range.",
	CodeValueNotPosition: "The target's value is not a position of this switch.",
	CodeBypassWord:       "Bypass must be On (bypassed) or Off (active).",
	CodeBypassLegacy:     "This preset predates the rule that Bypass On means bypassed; refine it first.",

	CodeLockedClone:      "This block's model clones a base whose parameter indexes aren't verified on the unit, so its controls are left as they are (bypass still works).",
	CodeLockedUnmeasured: "This control's range isn't measured, so the unit's value can't be read exactly; it is left as it is.",
	CodeLockedNoMapping:  "This kind of control (combo box, LED fader, other) has no measured wire mapping; it is left as it is.",
	CodeLockedNotWhole:   "The unit's value is not a whole position of this switch, so the encoding is wrong for it; it is left as it is.",
	CodeLockedUnreadable: "The unit's value for this control can't be read as a number in range; it is left as it is.",
}

// Reason returns the fixed text for a code ("" for an unknown code).
func Reason(code string) string { return reasons[code] }

// Codes lists every item code, sorted, for tests and the census.
func Codes() []string {
	out := make([]string, 0, len(reasons))
	for c := range reasons {
		out = append(out, c)
	}
	sortStrings(out)
	return out
}

// Whole-plan failures: no plan is returned.
const (
	ErrSnapshotMissing  = "snapshot_missing"
	ErrSnapshotInvalid  = "snapshot_invalid"
	ErrNoPosition       = "no_position"
	ErrNameUnpinnable   = "name_unpinnable"
	ErrFirmwareMismatch = "firmware_mismatch"
	ErrCatalogUnavail   = "catalog_unavailable"
	ErrPlanTooLarge     = "plan_too_large"
)

// PlanError is a failure that stops the whole plan.
type PlanError struct {
	Code string
	Msg  string
}

func (e *PlanError) Error() string { return e.Msg }

func planErr(code, msg string) *PlanError { return &PlanError{Code: code, Msg: msg} }
