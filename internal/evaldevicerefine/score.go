package evaldevicerefine

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/deviceedit"
	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/presetsvc"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Codes (deviceedit's not-applied codes) the metrics group on.
var (
	// structuralCodes are changes to the chain itself: a block added, a model or capture
	// swapped, a block moved. A block the target dropped is the "removed" bypass op, counted
	// separately in Score.
	structuralCodes = map[string]bool{
		deviceedit.CodeNewBlock: true, deviceedit.CodeModelSwap: true,
		deviceedit.CodeCaptureSwap: true, deviceedit.CodeReorder: true,
	}
	// lockedCodes are changes aimed at what the import left out or the edit never touches: a
	// control the unit's block does not have in the draft, an internal control, a clone's
	// unverified controls, a cab mic, a block kind that is never edited, a device element.
	lockedCodes = map[string]bool{
		deviceedit.CodeNotAControl: true, deviceedit.CodeInternalControl: true,
		deviceedit.CodeMicNotWritten: true, deviceedit.CodeNotEditableBlock: true,
		deviceedit.CodeDeviceElement: true, deviceedit.CodeLockedClone: true,
		deviceedit.CodeLockedUnmeasured: true, deviceedit.CodeLockedNoMapping: true,
		deviceedit.CodeLockedNotWhole: true, deviceedit.CodeLockedUnreadable: true,
	}
)

// Metrics is everything deterministic about one refine output, scored against its draft.
type Metrics struct {
	// Parsed: the output is JSON in the refine envelope. PlanOK: a plan was built, or the
	// output changed nothing. A run counts in a comparison only when both hold.
	Parsed     bool   `json:"parsed"`
	ParseError string `json:"parse_error,omitempty"`
	NoChange   bool   `json:"no_change"`
	PlanOK     bool   `json:"plan_ok"`
	PlanError  string `json:"plan_error,omitempty"`

	Ops        int `json:"ops"`
	NotApplied int `json:"not_applied"`
	// Locked counts not-applied items with a locked code; Structural counts adds, swaps and
	// moves plus removed blocks; Unasked is Structural for a request that did not ask for it.
	Locked     int `json:"locked_violations"`
	Structural int `json:"structural"`
	Unasked    int `json:"unasked_structural"`
	Removed    int `json:"removed"`
	Ambiguous  int `json:"match_ambiguous"`

	// Kept of DraftBlocks draft blocks survive with the same id and model in the same order.
	Kept        int `json:"kept"`
	DraftBlocks int `json:"draft_blocks"`
	OutBlocks   int `json:"out_blocks"`

	CodeCounts map[string]int `json:"code_counts,omitempty"`
	// CodesTruncated: the plan listed fewer not-applied items than it counted (its list is
	// capped), so Locked, Structural and CodeCounts are lower bounds for this run.
	CodesTruncated bool `json:"codes_truncated,omitempty"`

	// Mechanical checks (agents.RunMechanicalQualityChecks, as cmd/eval_device_checks).
	MechChecked   bool   `json:"mech_checked"`
	Defects       int    `json:"defects"`
	DeviceDefects int    `json:"device_defects"`
	CheckedBlocks int    `json:"checked_blocks"`
	Reply         string `json:"reply,omitempty"`
}

// Valid reports whether the run can enter a comparison.
func (m Metrics) Valid() bool { return m.Parsed && m.PlanOK }

// Scorer scores refine outputs against drafts. The zero value is not usable; use NewScorer.
type Scorer struct {
	Planner *deviceedit.Planner
	// Valid is the block set the mechanical checks use (agents.BuildEffectiveValidBlocks).
	ValidBlocks map[string]bool
}

// NewScorer builds a scorer over an exporter, with the same block set the pipeline checks use.
func NewScorer(e *deviceexport.Exporter) *Scorer {
	return &Scorer{Planner: deviceedit.New(e), ValidBlocks: agents.BuildEffectiveValidBlocks(true, true)}
}

// refineEnvelope is the part of the refine answer presetsvc reads.
type refineEnvelope struct {
	ConversationalResponse string                   `json:"conversational_response"`
	DSPMatrixUpdated       bool                     `json:"dsp_matrix_updated"`
	StructuredPayload      storage.StructuredPreset `json:"structured_payload"`
}

// Score scores one saved refine output (the string RefineChat returned) for a draft and request.
func (s *Scorer) Score(d *Draft, req Request, raw string) Metrics {
	var m Metrics
	m.DraftBlocks = len(d.Chain)

	// The same cleanup presetsvc.runRefine applies before it unmarshals.
	clean := strings.TrimSpace(agents.StripJSONFences(raw))
	clean = presetsvc.SanitizeJSONQuotes(clean)
	var env refineEnvelope
	if err := json.Unmarshal([]byte(clean), &env); err != nil {
		m.ParseError = err.Error()
		return m
	}
	m.Parsed = true
	m.Reply = env.ConversationalResponse

	// Like production, a reply that does not update the matrix (or has no guitars) leaves the
	// draft exactly as it was.
	if !env.DSPMatrixUpdated || len(env.StructuredPayload.Guitars) == 0 {
		m.NoChange, m.PlanOK = true, true
		m.Kept, m.OutBlocks = m.DraftBlocks, m.DraftBlocks
		return m
	}

	m.MechChecked = true
	rep := agents.RunMechanicalQualityChecks(raw, s.ValidBlocks)
	if rep.ParseError != "" {
		m.MechChecked = false
	} else {
		m.Defects, m.DeviceDefects, m.CheckedBlocks = rep.TotalDefects(), rep.Catalog.Defects(), rep.Catalog.CheckedBlocks
	}

	chain, err := chainOf(env.StructuredPayload)
	if err != nil {
		m.PlanError = err.Error()
		return m
	}
	m.OutBlocks = len(chain)
	m.Kept = preserved(d.Chain, chain)

	plan, perr := s.Planner.Plan(deviceedit.Input{
		Snapshot: d.Snapshot, Target: chain, Legacy: deviceedit.LegacyPreset(env.StructuredPayload), Derived: true})
	if perr != nil {
		m.PlanError = perr.Code + ": " + perr.Msg
		return m
	}
	m.PlanOK = true
	m.Ops = plan.Ops()
	m.NotApplied = plan.NotAppliedCount
	m.CodeCounts = map[string]int{}
	m.CodesTruncated = len(plan.NotApplied) < plan.NotAppliedCount
	// plan.NotApplied is capped; NotAppliedCount is complete. If the list was cut, the
	// per-code counts below are a lower bound and CodeCountsTruncated says so via the gap.
	for _, na := range plan.NotApplied {
		m.CodeCounts[na.Code]++
		switch {
		case lockedCodes[na.Code]:
			m.Locked++
		case structuralCodes[na.Code]:
			m.Structural++
		case na.Code == deviceedit.CodeMatchAmbiguous:
			m.Ambiguous++
		}
	}
	for _, op := range plan.Payload.BypassOps {
		if op.Why == "removed" {
			m.Removed++
			m.Structural++
		}
	}
	if !req.AsksStructural {
		m.Unasked = m.Structural
	}
	return m
}

// chainOf picks the guitar's chain: the import's guitar, else the only one.
func chainOf(sp storage.StructuredPreset) ([]storage.EffectBlock, error) {
	if c, ok := sp.Guitars[deviceexport.ReadbackGuitar]; ok {
		return c, nil
	}
	if len(sp.Guitars) == 1 {
		for _, c := range sp.Guitars {
			return c, nil
		}
	}
	return nil, fmt.Errorf("the output has %d guitars and none is %q", len(sp.Guitars), deviceexport.ReadbackGuitar)
}

// preserved is the number of draft blocks that appear in out with the same id and model, in
// the same relative order: the longest common subsequence of (id, model) pairs.
func preserved(draft, out []storage.EffectBlock) int {
	same := func(a, b storage.EffectBlock) bool { return a.ID == b.ID && a.Model == b.Model }
	n, m := len(draft), len(out)
	prev := make([]int, m+1)
	cur := make([]int, m+1)
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			switch {
			case same(draft[i-1], out[j-1]):
				cur[j] = prev[j-1] + 1
			case prev[j] >= cur[j-1]:
				cur[j] = prev[j]
			default:
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	return prev[m]
}
