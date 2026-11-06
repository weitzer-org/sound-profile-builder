package deviceexport

import (
	"fmt"
	"sync"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/stomplayout"
)

// embeddedLayout parses the embedded stomp_layout.json once; Build only reads it.
var embeddedLayout = sync.OnceValues(stomplayout.Embedded)

type stompResult struct {
	assign []stomplayout.Stomp
	view   *devicejobs.PlanStomps
}

// planStomps runs the stomp planner (design doc qc-stomp-layout.md, S2) over the blocks
// the export actually places. A fresh slot has no assignments, so none are passed. Blocks
// the export refused or left unresolved are not placed and never appear here. The rules
// live in internal/stomplayout; this only adapts the placed blocks to it.
func (e *Exporter) planStomps(rows []Row) (stompResult, error) {
	res := stompResult{assign: []stomplayout.Stomp{}, view: &devicejobs.PlanStomps{
		Assigned: []devicejobs.PlanStomp{}, Skipped: []devicejobs.PlanSkipped{}, Warnings: []string{}}}
	layout, err := embeddedLayout()
	if err != nil {
		return res, fmt.Errorf("stomp layout: %w", err)
	}
	var blocks []stomplayout.Block
	for _, r := range rows {
		for _, b := range r.Blocks {
			blocks = append(blocks, stomplayout.Block{Row: r.Row, Col: b.Col, ModelID: b.ModelID})
		}
	}
	plan, err := stomplayout.Build(layout, e.stompCls, blocks, nil)
	if err != nil {
		return res, fmt.Errorf("stomp layout: %w", err)
	}
	res.assign = append(res.assign, plan.Assign...)
	for _, r := range plan.Roles {
		if r.Switch != "" {
			res.view.Assigned = append(res.view.Assigned, devicejobs.PlanStomp{
				Row: r.Row, Col: r.Col, Model: r.Model, Role: string(r.Role), Switch: r.Switch})
		}
	}
	for _, s := range plan.Skipped {
		res.view.Skipped = append(res.view.Skipped, devicejobs.PlanSkipped{Row: s.Row, Col: s.Col, Model: s.Model, Reason: s.Reason})
	}
	res.view.Warnings = append(res.view.Warnings, plan.Warnings...)
	return res, nil
}
