package devicejobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Edit confirm and queue (preset-editing E4; contract section 12). The caller (edit_device_preset)
// rebuilds the plan from the preset and the stored read as they are now, so a changed preset or read
// changes PlanSHA256 and the token no longer matches.

// EditRequest is a confirmed edit.
type EditRequest struct {
	Claims EditClaims
	// PlanText is the plan's exact JSON text (deviceedit.Plan.Text); its hash must be Claims.PlanSHA256.
	PlanText []byte
	Source   PushSource
}

// IssueEditToken mints the dry run's confirm token.
func (b *Bridge) IssueEditToken(c EditClaims) (string, time.Time) {
	return b.Keys.IssueEditConfirm(c, b.now())
}

// ConfirmEdit checks the token against the claims and queues the edit job. A repeated confirm names the
// same job and returns it with existing = true. Only live is queued so far.
func (b *Bridge) ConfirmEdit(ctx context.Context, r EditRequest, token string) (id string, existing bool, err error) {
	now := b.now()
	if r.Claims.Mode != EditModeLive {
		return "", false, userErr("Only live edits can be applied so far; copy and overwrite are not available yet.")
	}
	if EditPlanSHA256(EditFormat, r.PlanText) != r.Claims.PlanSHA256 {
		return "", false, errors.New("the edit plan does not match its hash")
	}
	switch err := b.Keys.CheckEditConfirm(token, r.Claims, now); {
	case errors.Is(err, ErrConfirmExpired):
		return "", false, userErr("The confirm_token has expired (they last 10 minutes). Run a new dry run, show the changes to the user again, and confirm with the new token.")
	case errors.Is(err, ErrConfirmMismatch):
		return "", false, userErr("The confirm_token does not match: the preset_id, guitar or mode differ from the dry run, or the preset or the stored read of the unit changed since then. Run a new dry run and show the user the new changes.")
	case err != nil:
		return "", false, userErr("confirm_token is not valid. Run edit_device_preset without confirm_token (a dry run) to get one.")
	}
	id = EditJobIDFromToken(token)
	if v, err := b.Store.JobState(ctx, id, nil, now); err != nil {
		return "", false, err
	} else if v.State != StateNotFound {
		return id, true, nil
	}
	hv, err := b.Store.Heartbeat(ctx, now)
	if err != nil {
		return "", false, err
	}
	if !hv.Online {
		return "", false, userErr("Not queued: %s.", hv.Reason)
	}
	if !SlotAllowed(r.Claims.Slot, hv.Heartbeat.AllowedSlots) {
		return "", false, userErr("Not queued: slot %s is outside the slots the bridge may write. Pick a slot in its allowed range (see device_status).", r.Claims.Slot)
	}
	if err := b.checkQueue(ctx, ""); err != nil {
		return "", false, err
	}
	pend, err := b.Store.PendingJobs(ctx, now)
	if err != nil {
		return "", false, err
	}
	for _, j := range pend {
		if j.Kind != KindEditPreset {
			continue
		}
		var a EditArgs
		if json.Unmarshal(j.Args, &a) == nil && a.Slot == r.Claims.Slot {
			return "", false, userErr("Not queued: an edit of slot %s is already waiting (job %s). Wait for it with get_device_job.", r.Claims.Slot, j.JobID)
		}
	}
	args, err := NewEditArgs(r.Claims.Slot, r.Claims.Mode, nil, nil, r.Source, r.PlanText)
	if err != nil {
		return "", false, err
	}
	j, err := b.Store.NewJob(id, KindEditPreset, args, now)
	if err != nil {
		return "", false, err
	}
	if err := b.Store.Enqueue(ctx, j); err != nil {
		if errors.Is(err, ErrJobExists) {
			return id, true, nil
		}
		return "", false, err
	}
	return id, false, nil
}
