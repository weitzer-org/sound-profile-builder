package devicelib

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
)

// EditPlanRecord (preset-editing E4): the plan QC-2 sent in an edit job, kept as device/edits/<job_id>.json
// in the main bucket when the job is queued. The pending job is deleted once the agent has answered, so
// this is what get_device_job checks the agent's per-op read-back against ("the plan QC-2 sent"). It is
// QC-2's own record, re-parsed strictly on every load: the plan text must parse with the contract parser
// and hash to PlanSHA256.
const (
	EditPlanPrefix = "device/edits/"
	EditPlanFormat = "qc2-device-edit-record/1"
)

// EditPlanRecord is one stored plan.
type EditPlanRecord struct {
	Format     string `json:"format"`
	JobID      string `json:"job_id"`
	Mode       string `json:"mode"`
	Slot       string `json:"slot"`
	PlanSHA256 string `json:"plan_sha256"`
	Plan       string `json:"plan"` // the plan's exact JSON text
	QueuedAt   string `json:"queued_at"`
}

func editPlanObject(jobID string) string { return EditPlanPrefix + jobID + ".json" }

// Validate checks a record loaded from storage.
func (r *EditPlanRecord) Validate(wantJobID string) error {
	bad := errors.New("the stored edit plan is malformed")
	if r.Format != EditPlanFormat || r.JobID != wantJobID || !devicejobs.ValidJobID(r.JobID) {
		return bad
	}
	if r.Mode != devicejobs.EditModeLive && r.Mode != devicejobs.EditModeCopy && r.Mode != devicejobs.EditModeOverwrite {
		return bad
	}
	if devicejobs.EditPlanSHA256(devicejobs.EditFormat, []byte(r.Plan)) != r.PlanSHA256 {
		return bad
	}
	if _, err := devicejobs.ParseEditPayload([]byte(r.Plan)); err != nil {
		return bad
	}
	return nil
}

// SaveEditPlan stores the plan of a queued edit job (write once per job id).
func (s *Store) SaveEditPlan(ctx context.Context, r *EditPlanRecord) error {
	if err := r.Validate(r.JobID); err != nil {
		return err
	}
	var existing EditPlanRecord
	ok, err := s.readJSON(ctx, editPlanObject(r.JobID), &existing)
	if err != nil || ok {
		return err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return s.client.WriteFile(ctx, s.bucket, editPlanObject(r.JobID), data)
}

// ReadEditPlan returns the stored plan of an edit job, or nil when none is stored.
func (s *Store) ReadEditPlan(ctx context.Context, jobID string) (*EditPlanRecord, error) {
	if !devicejobs.ValidJobID(jobID) || strings.ContainsAny(jobID, "/.") {
		return nil, errors.New("not a job id")
	}
	var r EditPlanRecord
	ok, err := s.readJSON(ctx, editPlanObject(jobID), &r)
	if err != nil || !ok {
		return nil, err
	}
	if err := r.Validate(jobID); err != nil {
		return nil, err
	}
	return &r, nil
}
