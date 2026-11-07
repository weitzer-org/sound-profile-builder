package evaldevicerefine

import (
	"fmt"

	"github.com/weitzer-org/sound-builder/internal/presetsvc"
)

// Request is one of the four request kinds of section 8.1.
type Request struct {
	ID      string
	Message string
	// AsksStructural is true when the request itself asks for a swap, add or removal. None of
	// the four does, so every structural change counts as unasked; the field exists so a
	// hand-made request set can say otherwise.
	AsksStructural bool
}

// Requests are the same four for every draft.
var Requests = []Request{
	{ID: "tone", Message: "Less fizz on the lead tone."},
	{ID: "scene", Message: "More gain and level in scene B."},
	{ID: "review", Message: presetsvc.ReviewMessage("a cleaner blues rhythm")},
	{ID: "structural", Message: "Make it sound like a Plexi."},
}

// Arms: A is today's refine, B is the edit-aware context.
const (
	ArmA = "A"
	ArmB = "B"
)

// Arms lists both, in table order.
var Arms = []string{ArmA, ArmB}

// Triple is one (draft, request, rep): the unit the two arms are compared on.
type Triple struct {
	Draft, Request string
	Rep            int
}

// Key is the run-record query string for a triple.
func (t Triple) Key() string { return fmt.Sprintf("%s/%s", t.Draft, t.Request) }

// Run is one refine call: a triple under an arm.
type Run struct {
	Triple
	Arm string
}

// Matrix lists the runs for the given drafts, requests, arms and reps, in a stable order.
func Matrix(drafts, requests []string, arms []string, reps int) []Run {
	var out []Run
	for _, d := range drafts {
		for _, r := range requests {
			for rep := 1; rep <= reps; rep++ {
				for _, a := range arms {
					out = append(out, Run{Triple: Triple{Draft: d, Request: r, Rep: rep}, Arm: a})
				}
			}
		}
	}
	return out
}

// RequestByID finds a request.
func RequestByID(id string) (Request, bool) {
	for _, r := range Requests {
		if r.ID == id {
			return r, true
		}
	}
	return Request{}, false
}

// RequestIDs returns the ids, sorted by table order.
func RequestIDs() []string {
	ids := make([]string, len(Requests))
	for i, r := range Requests {
		ids[i] = r.ID
	}
	return ids
}
