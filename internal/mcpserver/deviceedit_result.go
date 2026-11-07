package mcpserver

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
)

// The result of an edit_preset job, as get_device_job shows it (preset-editing E4). The agent's result is
// signed but its content is the agent's word, so every entry is checked against the plan QC-2 sent (stored
// when the job was queued): the same cells and indexes, nothing extra, and the values recomputed here
// against the plan's wire values. Anything that doesn't fit is shown as unverified, never as applied.

type deviceEditOp struct {
	Col   int    `json:"col"`
	Index *int   `json:"index,omitempty"`
	Label string `json:"label"`
	OK    bool   `json:"ok"` // recomputed by QC-2 from the read-back and the plan, not copied from the agent
}

type deviceEditResultOut struct {
	Mode            string         `json:"mode"`
	CheckedAgainst  bool           `json:"checked_against_plan"`
	Applied         bool           `json:"applied"`
	Saved           bool           `json:"saved"`
	Verified        bool           `json:"verified_on_open_preset"` // every control read back, nothing else changed, no rollback
	CopySlot        string         `json:"copy_slot,omitempty"`
	CopyName        string         `json:"copy_name,omitempty"`
	CopyListed      bool           `json:"copy_listed,omitempty"`
	CAS             string         `json:"compare_and_swap"`
	CASFailed       []string       `json:"compare_and_swap_failed,omitempty"`
	Ops             []deviceEditOp `json:"ops,omitempty"`
	CollateralOK    *bool          `json:"nothing_else_changed,omitempty"`
	CollateralCount int            `json:"collateral_mismatches,omitempty"`
	Rollback        string         `json:"rollback"`
	RollbackRestore int            `json:"rollback_restored,omitempty"`
	RollbackFailed  int            `json:"rollback_failed,omitempty"`
	Note            string         `json:"note,omitempty"`
}

const tolWire = 1e-4

// editResultOutput builds the output for an edit job's result, or nil when v has none.
func editResultOutput(ctx context.Context, lib *DeviceLibrary, v *devicejobs.JobView) *deviceEditResultOut {
	if v == nil || v.Result == nil || v.Result.Kind != devicejobs.KindEditPreset || v.Result.Edit == nil {
		return nil
	}
	res, err := devicejobs.ParseEditResultPayload(v.Result.Edit.Payload)
	if err != nil || v.Result.Edit.Format != devicejobs.EditResultFormat {
		return &deviceEditResultOut{CAS: "unknown", Rollback: "unknown", Note: "The bridge's edit result could not be read, so nothing in it is trusted. Read the preset on the unit again."}
	}
	out := &deviceEditResultOut{Mode: res.Mode, CAS: res.CAS.State, Rollback: res.Rollback.State,
		RollbackRestore: res.Rollback.Restored, RollbackFailed: res.Rollback.Failed, Saved: res.Save.Saved}
	for _, f := range res.CAS.Failed {
		out.CASFailed = append(out.CASFailed, sanitizeDeviceText(f, 20))
	}
	if c := res.Collateral; c != nil {
		ok := c.Mismatches == 0 && c.LayoutSame
		out.CollateralOK, out.CollateralCount = &ok, c.Mismatches
	}
	var rec *planRecord
	if lib != nil && lib.Store != nil {
		r, err := lib.Store.ReadEditPlan(ctx, v.JobID)
		if err == nil && r != nil && r.PlanSHA256 == res.PlanSHA256 {
			if p, perr := devicejobs.ParseEditPayload([]byte(r.Plan)); perr == nil && p.Snapshot == res.Snapshot && r.Mode == res.Mode {
				rec = &planRecord{p: p, target: r.TargetSlot, name: r.Name}
			}
		}
	}
	if rec == nil {
		out.Note = "QC-2 has no record of the plan this result answers, so the bridge's per-control read-back could not be checked. Treat it as unverified and read the preset on the unit."
		return out
	}
	out.CheckedAgainst = true
	p := rec.p
	fits := true
	if res.CAS.State != "matched" {
		fits = len(res.Params) == 0 && len(res.Bypass) == 0 // nothing was written, so nothing is read back
	} else {
		if len(res.Params) != len(p.ParamOps) || len(res.Bypass) != len(p.BypassOps) {
			fits = false
		}
		for i := 0; fits && i < len(p.ParamOps); i++ {
			op, got := p.ParamOps[i], res.Params[i]
			ok := got.Row == op.Row && got.Col == op.Col && got.Index == op.Index
			for s := 0; ok && s < devicejobs.ReadbackScenes; s++ {
				if got.Readback[s] == nil || math.Abs(*got.Readback[s]-op.Wire[s]) > tolWire {
					ok = false
				}
			}
			idx := op.Index
			out.Ops = append(out.Ops, deviceEditOp{Col: op.Col + 1, Index: &idx, Label: sanitizeDeviceText(op.Label, 64), OK: ok})
			if got.Row != op.Row || got.Col != op.Col || got.Index != op.Index {
				fits = false
			}
		}
		for i := 0; fits && i < len(p.BypassOps); i++ {
			op, got := p.BypassOps[i], res.Bypass[i]
			if got.Row != op.Row || got.Col != op.Col {
				fits = false
				break
			}
			ok := got.Readback != nil && len(*got.Readback) == devicejobs.ReadbackScenes
			for s := 0; ok && s < devicejobs.ReadbackScenes; s++ {
				ok = (*got.Readback)[s] == op.Bypass[s]
			}
			out.Ops = append(out.Ops, deviceEditOp{Col: op.Col + 1, Label: "Bypass", OK: ok})
		}
	}
	if !fits {
		out.CheckedAgainst = false
		out.Ops = nil
		out.Note = "The bridge's result does not fit the plan QC-2 sent (other cells or controls than planned), so it is not trusted. Read the preset on the unit."
		return out
	}
	// A save block must be the copy QC-2 queued, in the slot and under the name it asked for; a live edit has none.
	if res.Mode == devicejobs.EditModeCopy {
		if res.Save.Saved && (res.Save.Slot == nil || res.Save.Name == nil || *res.Save.Slot != rec.target || *res.Save.Name != rec.name) {
			out.CheckedAgainst = false
			out.Ops = nil
			out.Note = "The bridge's result says it saved somewhere other than the copy QC-2 asked for, so it is not trusted. Check the unit before doing anything else."
			return out
		}
		if res.Save.Saved {
			out.CopySlot, out.CopyName, out.CopyListed = rec.target, rec.name, res.Save.Listed
		}
	}
	allOK := res.CAS.State == "matched" && len(out.Ops) > 0
	for _, o := range out.Ops {
		allOK = allOK && o.OK
	}
	out.Verified = allOK && out.CollateralOK != nil && *out.CollateralOK && res.Rollback.State == "not_needed"
	switch res.Mode {
	case devicejobs.EditModeCopy:
		out.Applied = out.Verified && v.Result.Status == devicejobs.StatusSucceeded && res.Save.Saved && res.Save.Listed
	default:
		out.Applied = out.Verified && v.Result.Status == devicejobs.StatusSucceeded && !res.Save.Saved
	}
	return out
}

type planRecord struct {
	p            *devicejobs.EditPayload
	target, name string // a copy's target slot and name, as QC-2 queued them
}

func renderEditResult(b *strings.Builder, e *deviceEditResultOut) {
	if e == nil {
		return
	}
	switch {
	case e.Applied && e.Mode == devicejobs.EditModeCopy:
		fmt.Fprintf(b, "\n**Edit saved as a copy: slot %s, \"%s\".** Every control read back as planned before the save, nothing else changed, and the bridge saw the copy in a fresh listing. The original slot was never written by the bridge, but the preset still open on the unit holds the edit unsaved: the user re-selects it WITHOUT saving. Saving a copy at a slot other than the open one is **not yet verified on the unit** (gate E-G3): ask the user to open the copy and check it, and which slot is open now.\n", e.CopySlot, dcell(e.CopyName))
	case e.Mode == devicejobs.EditModeCopy && e.CheckedAgainst && e.Saved:
		fmt.Fprintf(b, "\n**The copy save to slot %s was issued but not confirmed** (the bridge's listing never showed it, or something else was off). It may still land: the user should check slot %s and the original slot on the unit before using them. Do not say the copy exists.\n", e.CopySlot, e.CopySlot)
	case e.Mode == devicejobs.EditModeCopy && e.CheckedAgainst && e.Verified:
		b.WriteString("\n**The edit verified on the open preset, but the copy was NOT saved and the edit was NOT rolled back.** The open preset holds the edit unsaved: tell the user to re-select that slot on the unit WITHOUT saving to discard it. Nothing new exists in any other slot.\n")
	case e.Applied:
		b.WriteString("\n**Edit applied to the open preset on the unit, NOT saved.** Every control read back as planned and nothing else changed. The user listens, then saves on the unit, or re-selects the slot to discard the edit.\n")
	case e.Rollback == "restored":
		b.WriteString("\n**The edit did not verify and was rolled back.** The original values were put back and checked. Nothing was saved; the slot can be re-selected on the unit without saving.\n")
	case e.Rollback == "partial" || e.Rollback == "failed":
		b.WriteString("\n**The edit failed and could not be fully rolled back.** The open preset on the unit may be half-edited and was NOT saved: tell the user to re-select that slot on the unit WITHOUT saving.\n")
	case e.CAS == "mismatch":
		fmt.Fprintf(b, "\n**Refused, nothing was changed:** the preset on the unit differs from the read the plan was built on (%s). Read it again with read_device_preset and run a new dry run.\n", strings.Join(e.CASFailed, ", "))
	}
	if e.Note != "" {
		fmt.Fprintf(b, "\n%s\n", dcell(e.Note))
	}
	if len(e.Ops) > 0 {
		b.WriteString("\n| Grid | Control | Read back as planned |\n|---|---|---|\n")
		for _, o := range e.Ops {
			yes := "yes"
			if !o.OK {
				yes = "NO"
			}
			fmt.Fprintf(b, "| col %d | %s | %s |\n", o.Col, tcell(o.Label), yes)
		}
	}
	if e.CollateralOK != nil && !*e.CollateralOK {
		fmt.Fprintf(b, "\nSomething outside the plan changed on the unit (%d difference(s)).\n", e.CollateralCount)
	}
	if e.Rollback != "not_needed" && e.Rollback != "unknown" {
		fmt.Fprintf(b, "Rollback: %s (%d put back, %d not).\n", e.Rollback, e.RollbackRestore, e.RollbackFailed)
	}
}
