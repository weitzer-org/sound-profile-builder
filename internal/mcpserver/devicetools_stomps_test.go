package mcpserver

import (
	"strings"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
)

// The dry run shows the stomp layout next to the blocks. Text here stands in for anything
// the export carries, so it must go through cell(). The switch behaviour is UNVERIFIED on
// the unit (gate G7); this checks rendering only.
func TestSendPreset_DryRunShowsStompLayout(t *testing.T) {
	fixed := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	r := newDeviceRig(t, func() time.Time { return fixed })
	seedPreset(t, r.h, "p-dev", "Dev Preset")
	r.online(t)
	ex := devicejobstest.FixtureExport()
	ex.Stomps = &devicejobs.PlanStomps{
		Assigned: []devicejobs.PlanStomp{{Row: 0, Col: 0, Model: "Opto Comp (M)", Role: "comp", Switch: "A"}},
		Skipped:  []devicejobs.PlanSkipped{{Row: 0, Col: 1, Model: "Brit 2203", Reason: "no role for category Guitar Amplifier"}},
		Warnings: []string{"hostile\n# injected <script>\u202e | x"},
	}
	r.exp.Exp = ex

	res := itCall(t, r.cs, devicePushToolName, pushArgs(nil))
	got := okText(t, res)
	out := itStructured[devicePushOutput](t, res)
	if out.Plan.Stomps == nil || len(out.Plan.Stomps.Assigned) != 1 {
		t.Fatalf("structured plan lacks the layout: %+v", out.Plan)
	}
	for _, want := range []string{"Footswitch layout", "UNVERIFIED", "- A: Opto Comp (M) (row 0 col 0, comp)", "No switch:", "Brit 2203 (row 0 col 1): no role for category Guitar Amplifier"} {
		if !strings.Contains(got, want) {
			t.Errorf("dry run lacks %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, `⚠️ hostile # injected &lt;script&gt; \| x`) {
		t.Errorf("warning text was not sanitized (newline joined, bidi dropped, HTML and pipe escaped):\n%s", got)
	}
	if strings.ContainsAny(got, "\u202e") || strings.Contains(got, "<script>") {
		t.Errorf("control characters or raw HTML in the dry run:\n%s", got)
	}
}

// With no layout in the export (older exporters, fixtures) the dry run is unchanged.
func TestSendPreset_DryRunWithoutStomps(t *testing.T) {
	r := newDeviceRig(t, nil)
	seedPreset(t, r.h, "p-dev", "Dev Preset")
	r.online(t)
	got := okText(t, itCall(t, r.cs, devicePushToolName, pushArgs(nil)))
	if strings.Contains(got, "Footswitch") {
		t.Errorf("layout section without a layout:\n%s", got)
	}
}
