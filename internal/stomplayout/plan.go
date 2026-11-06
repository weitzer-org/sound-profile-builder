package stomplayout

import (
	"fmt"
	"sort"
)

// Assignment is a stomp assignment as read back from the unit.
type Assignment struct {
	Row       int    `json:"row"`
	Col       int    `json:"col"`
	Switch    string `json:"switch"`
	Momentary bool   `json:"momentary,omitempty"`
}

// Stomp is one assignment to add or remove. Latching only: it carries no momentary flag.
type Stomp struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Switch string `json:"switch"`
}

// BlockRole records what the planner found for one block, in signal order.
type BlockRole struct {
	Row      int    `json:"row"`
	Col      int    `json:"col"`
	ModelID  int    `json:"model_id"`
	Model    string `json:"model,omitempty"`
	Category string `json:"category,omitempty"`
	Role     Role   `json:"role,omitempty"`
	Source   Source `json:"source,omitempty"`
	// Switch is where the layout puts the block ("" when it gets none).
	Switch string `json:"switch,omitempty"`
}

// Skipped is a block that gets no switch from this plan, and why.
type Skipped struct {
	Row     int    `json:"row"`
	Col     int    `json:"col"`
	ModelID int    `json:"model_id"`
	Model   string `json:"model,omitempty"`
	Reason  string `json:"reason"`
}

// Plan is the result: what to add, what to remove, and what was left alone.
type Plan struct {
	Assign   []Stomp     `json:"assign"`
	Remove   []Stomp     `json:"remove"`
	Skipped  []Skipped   `json:"skipped"`
	Warnings []string    `json:"warnings"`
	Roles    []BlockRole `json:"roles"`
}

// Empty reports whether the plan changes nothing.
func (p Plan) Empty() bool { return len(p.Assign) == 0 && len(p.Remove) == 0 }

type cell struct{ row, col int }

// Build computes the plan for blocks and the preset's current assignments (design doc
// section 5). It is deterministic and has no side effects. It returns an error for input
// that cannot be a real grid: a negative coordinate, two blocks in one cell, or a switch
// outside A-H.
func Build(l *Layout, c *Classifier, blocks []Block, current []Assignment) (Plan, error) {
	plan := Plan{Assign: []Stomp{}, Remove: []Stomp{}, Skipped: []Skipped{}, Warnings: []string{}, Roles: []BlockRole{}}

	seen := map[cell]bool{}
	for _, b := range blocks {
		if b.Row < 0 || b.Col < 0 {
			return Plan{}, fmt.Errorf("block at (%d,%d): negative coordinate", b.Row, b.Col)
		}
		k := cell{b.Row, b.Col}
		if seen[k] {
			return Plan{}, fmt.Errorf("two blocks at row %d col %d", b.Row, b.Col)
		}
		seen[k] = true
	}
	byCell := map[cell][]Assignment{}
	for _, a := range current {
		if !validSwitch(a.Switch) {
			return Plan{}, fmt.Errorf("assignment at (%d,%d): switch %q is outside A-H", a.Row, a.Col, a.Switch)
		}
		byCell[cell{a.Row, a.Col}] = append(byCell[cell{a.Row, a.Col}], a)
	}

	// Rule 1: signal order is row-major.
	ordered := append([]Block(nil), blocks...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Row != ordered[j].Row {
			return ordered[i].Row < ordered[j].Row
		}
		return ordered[i].Col < ordered[j].Col
	})

	drivesSeen := 0
	target := map[cell]string{} // cell -> switch the layout wants ("" = none)
	for _, b := range ordered {
		cl := c.Classify(b)
		k := cell{b.Row, b.Col}
		br := BlockRole{Row: b.Row, Col: b.Col, ModelID: b.ModelID, Model: cl.Model, Category: cl.Category, Role: cl.Role, Source: cl.Source}
		skip := func(reason string) {
			plan.Skipped = append(plan.Skipped, Skipped{Row: b.Row, Col: b.Col, ModelID: b.ModelID, Model: cl.Model, Reason: reason})
		}
		if cl.Disagreement != "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("block (%d,%d) %q: %s", b.Row, b.Col, b.CaptureName, cl.Disagreement))
		}
		if cl.Unclassified {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("unclassified capture %q at (%d,%d): no usable modeler type and not in the capture library; add it to internal/agents/user_captures.json", b.CaptureName, b.Row, b.Col))
		}

		// Rule 3 counts every drive in signal order, including one parked on H.
		slot := string(cl.Role)
		if cl.Role == RoleDrive {
			drivesSeen++
			if drivesSeen >= 2 {
				if _, ok := l.SwitchFor(slotDrive2); ok {
					slot = slotDrive2
				}
			}
		}

		onH := false
		for _, a := range byCell[k] {
			if a.Switch == protectedH {
				onH = true
			}
		}
		switch {
		case cl.Role == RoleNone:
			reason := cl.Reason
			if reason == "" {
				reason = "no role"
			}
			skip(reason)
		case onH:
			// Rule 5: a block the owner put on H stays there.
			skip(fmt.Sprintf("already on H (%s); H is never touched", cl.Role))
		default:
			sw, ok := l.SwitchFor(slot)
			if !ok {
				skip(fmt.Sprintf("layout has no switch for %s", slot))
			} else if l.IsReserved(sw) || sw == protectedH {
				skip(fmt.Sprintf("layout puts %s on reserved switch %s", slot, sw))
			} else {
				target[k] = sw
				br.Switch = sw
			}
		}
		plan.Roles = append(plan.Roles, br)
	}

	// Rule 6: diff against the current assignments. H is never touched (rule 5); a
	// matching assignment stays (second run is a no-op); anything else in A-G goes.
	for _, a := range sortedAssignments(current) {
		if a.Switch == protectedH {
			continue
		}
		k := cell{a.Row, a.Col}
		if !seen[k] {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("assignment %s at (%d,%d) has no block; left alone", a.Switch, a.Row, a.Col))
			continue
		}
		if target[k] == a.Switch {
			if a.Momentary {
				// Rule 7: warn, never rewrite the flag.
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("switch %s at (%d,%d) is momentary; this tool only writes latching assignments and leaves it as is", a.Switch, a.Row, a.Col))
			}
			continue
		}
		plan.Remove = append(plan.Remove, Stomp{a.Row, a.Col, a.Switch})
	}
	for _, b := range ordered {
		k := cell{b.Row, b.Col}
		sw := target[k]
		if sw == "" {
			continue
		}
		has := false
		for _, a := range byCell[k] {
			if a.Switch == sw {
				has = true
			}
		}
		if !has {
			plan.Assign = append(plan.Assign, Stomp{b.Row, b.Col, sw})
		}
	}
	sortStomps(plan.Remove)
	sortStomps(plan.Assign)
	return plan, nil
}

func sortedAssignments(in []Assignment) []Assignment {
	out := append([]Assignment(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Row != b.Row {
			return a.Row < b.Row
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return a.Switch < b.Switch
	})
	return out
}

func sortStomps(s []Stomp) {
	sort.SliceStable(s, func(i, j int) bool {
		a, b := s[i], s[j]
		if a.Row != b.Row {
			return a.Row < b.Row
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return a.Switch < b.Switch
	})
}
