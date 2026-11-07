package devicelib

import (
	"context"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
)

// A stored edit plan is QC-2's own record, but anyone who can write the bucket could change it: a record whose plan no
// longer hashes to its PlanSHA256, or has another mode, must not load.
func TestEditPlanRecord_RoundTripAndTamper(t *testing.T) {
	plan := `{"format":"qc2-device-edit/1","catalog":{"coros":"4.1","sha256":"0000000000000000000000000000000000000000000000000000000000000000"},"snapshot":"11111111-2222-4333-8444-555555555555","expect":{"folder_key":"/k","position":24,"slot":"4A","name":"A","layout":[[0,0,6]]},"param_ops":[{"row":0,"col":0,"model_id":6,"index":0,"label":"GAIN","evidence":[],"scene_mode":false,"expect":[0.1,0.1,0.1,0.1,0.1,0.1,0.1,0.1],"wire":[0.2,0.2,0.2,0.2,0.2,0.2,0.2,0.2]}],"bypass_ops":[]}`
	id := "e1111111-2222-4333-8444-555555555555"
	good := &EditPlanRecord{Format: EditPlanFormat, JobID: id, Mode: devicejobs.EditModeLive, Slot: "4A",
		PlanSHA256: devicejobs.EditPlanSHA256(devicejobs.EditFormat, []byte(plan)), Plan: plan}
	if err := good.Validate(id); err != nil {
		t.Fatalf("a good record: %v", err)
	}
	for name, mut := range map[string]func(r *EditPlanRecord){
		"another mode":  func(r *EditPlanRecord) { r.Mode = "sideways" },
		"hash mismatch": func(r *EditPlanRecord) { r.PlanSHA256 = "0" + r.PlanSHA256[1:] },
		"another job":   func(r *EditPlanRecord) { r.JobID = "f1111111-2222-4333-8444-555555555555" },
		"plan changed":  func(r *EditPlanRecord) { r.Plan = r.Plan + " " },
	} {
		bad := *good
		mut(&bad)
		if err := bad.Validate(id); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Errorf("%s: a tampered record: err = %v", name, err)
		}
	}
	st := NewStore(devicejobstest.NewMemClient(), "main")
	ctx := context.Background()
	if err := st.SaveEditPlan(ctx, good); err != nil {
		t.Fatal(err)
	}
	if r, err := st.ReadEditPlan(ctx, id); err != nil || r == nil || r.PlanSHA256 != good.PlanSHA256 {
		t.Fatalf("read back: %v %v", r, err)
	}
	if r, err := st.ReadEditPlan(ctx, "11111111-2222-4333-8444-555555555556"); err != nil || r != nil {
		t.Fatalf("an unknown job must read as nil: %v %v", r, err)
	}
}
