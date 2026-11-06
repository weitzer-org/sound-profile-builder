// Package stomplayout decides which Quad Cortex footswitch each block of a preset goes on,
// so the same kind of effect is always on the same switch (docs/design/qc-stomp-layout.md).
//
// It is a pure function from blocks and the current stomp assignments to a plan. It has no
// storage, device, LLM or network dependency, and it does not import internal/agents: the
// caller passes the capture library in as a CaptureLookup.
//
// Everything this package says about the device (switch indexes, assignment semantics, how a
// capture appears on a block) is UNVERIFIED until gate G7 of the design doc has run.
package stomplayout

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

//go:embed stomp_layout.json
var embeddedLayout []byte

// Role is the kind of effect a block is, as far as footswitch assignment goes.
type Role string

const (
	RoleNone   Role = ""
	RoleComp   Role = "comp"
	RoleBoost  Role = "boost"
	RoleDrive  Role = "drive"
	RoleDelay  Role = "delay"
	RoleMod    Role = "mod"
	RoleReverb Role = "reverb"
)

// slotDrive2 is a layout slot, not a block role: the second and later drives share it.
const slotDrive2 = "drive2"

// Switches are the eight footswitches in device order: A-D the top row, E-H the bottom row.
const Switches = "ABCDEFGH"

// layoutSlots are the names a layout may assign to a switch.
var layoutSlots = map[string]bool{
	string(RoleComp): true, string(RoleBoost): true, string(RoleDrive): true, slotDrive2: true,
	string(RoleDelay): true, string(RoleMod): true, string(RoleReverb): true,
}

// Layout is a validated stomp_layout.json.
type Layout struct {
	Version  int
	Reserved []string
	// bySlot maps a layout slot (a Role name, or "drive2") to its switch letter.
	bySlot map[string]string
	// switches keeps the file's switch -> slot assignments for display.
	switches map[string]string
}

type layoutFile struct {
	Version  int             `json:"version"`
	Switches json.RawMessage `json:"switches"`
	Reserved []string        `json:"reserved"`
}

// Embedded returns the layout compiled into the binary.
func Embedded() (*Layout, error) { return ParseLayout(embeddedLayout) }

// ParseLayout validates and parses a stomp_layout.json. It refuses: a switch outside A-H; a
// role assigned to two switches; a reserved switch that also has a role; an unknown role.
// It also refuses an unknown version, unknown or duplicate keys, a layout with no roles, a
// repeated or non-A-H reserved switch, and "drive2" without "drive".
func ParseLayout(data []byte) (*Layout, error) {
	var f layoutFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("stomp layout: %w", err)
	}
	if dec.More() {
		return nil, errors.New("stomp layout: trailing data after the JSON object")
	}
	if f.Version != 1 {
		return nil, fmt.Errorf("stomp layout: unsupported version %d", f.Version)
	}
	pairs, err := orderedStringPairs(f.Switches)
	if err != nil {
		return nil, fmt.Errorf("stomp layout: switches: %w", err)
	}
	l := &Layout{Version: f.Version, bySlot: map[string]string{}, switches: map[string]string{}}
	for _, p := range pairs {
		sw, slot := p[0], p[1]
		if !validSwitch(sw) {
			return nil, fmt.Errorf("stomp layout: switch %q is outside A-H", sw)
		}
		if _, dup := l.switches[sw]; dup {
			return nil, fmt.Errorf("stomp layout: switch %q appears twice", sw)
		}
		if !layoutSlots[slot] {
			return nil, fmt.Errorf("stomp layout: switch %s has unknown role %q", sw, slot)
		}
		if prev, dup := l.bySlot[slot]; dup {
			return nil, fmt.Errorf("stomp layout: role %q is assigned to two switches (%s and %s)", slot, prev, sw)
		}
		l.switches[sw] = slot
		l.bySlot[slot] = sw
	}
	if len(l.switches) == 0 {
		return nil, errors.New("stomp layout: no switch has a role")
	}
	if _, hasD2 := l.bySlot[slotDrive2]; hasD2 {
		if _, hasD := l.bySlot[string(RoleDrive)]; !hasD {
			return nil, errors.New("stomp layout: role \"drive2\" needs a \"drive\" switch")
		}
	}
	seen := map[string]bool{}
	for _, r := range f.Reserved {
		if !validSwitch(r) {
			return nil, fmt.Errorf("stomp layout: reserved switch %q is outside A-H", r)
		}
		if seen[r] {
			return nil, fmt.Errorf("stomp layout: reserved switch %q is listed twice", r)
		}
		seen[r] = true
		if slot, has := l.switches[r]; has {
			return nil, fmt.Errorf("stomp layout: reserved switch %s also has role %q", r, slot)
		}
		l.Reserved = append(l.Reserved, r)
	}
	return l, nil
}

// SwitchFor returns the switch for a role, with drive2 reachable as "drive2".
func (l *Layout) SwitchFor(slot string) (string, bool) {
	sw, ok := l.bySlot[slot]
	return sw, ok
}

// Slot returns the layout slot assigned to a switch ("" when none).
func (l *Layout) Slot(sw string) string { return l.switches[sw] }

// IsReserved reports whether a switch is reserved for manual use.
func (l *Layout) IsReserved(sw string) bool {
	for _, r := range l.Reserved {
		if r == sw {
			return true
		}
	}
	return false
}

// protectedH is the switch rule 5 never touches. The design fixes it as H whatever the file
// says, and the planner enforces it on top of the file's reserved list.
const protectedH = "H"

// Summary lists "A=comp B=boost ..." in switch order, for dry runs and test failures.
func (l *Layout) Summary() string {
	var parts []string
	for _, c := range Switches {
		sw := string(c)
		if slot, ok := l.switches[sw]; ok {
			parts = append(parts, sw+"="+slot)
		}
	}
	return strings.Join(parts, " ")
}

func validSwitch(s string) bool { return len(s) == 1 && strings.Contains(Switches, s) }

// orderedStringPairs reads a JSON object of string values in file order, refusing
// duplicate keys (encoding/json would silently keep the last one).
func orderedStringPairs(raw json.RawMessage) ([][2]string, error) {
	if len(raw) == 0 {
		return nil, errors.New("missing")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errors.New("not an object")
	}
	var out [][2]string
	seen := map[string]bool{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := kt.(string)
		if seen[key] {
			return nil, fmt.Errorf("key %q appears twice", key)
		}
		seen[key] = true
		vt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		val, ok := vt.(string)
		if !ok {
			return nil, fmt.Errorf("value for %q is not a string", key)
		}
		out = append(out, [2]string{key, val})
	}
	if _, err := dec.Token(); err != nil && err != io.EOF {
		return nil, err
	}
	return out, nil
}
