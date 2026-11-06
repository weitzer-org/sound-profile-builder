package stomplayout

import (
	"fmt"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/catalog"
)

// CaptureLookup returns the capture library's block_type for a capture name, matched
// exactly (user_captures.json). The caller supplies it so this package stays free of
// internal/agents. A nil lookup means "no library".
type CaptureLookup func(name string) (blockType string, ok bool)

// Source says which step of the design's section 4 / 4.1 decided a block's role.
type Source string

const (
	SourceCatalog Source = "catalog" // the model id's category and name
	SourceModeler Source = "modeler" // 4.1 step 1: the Architect's block type
	SourceLibrary Source = "library" // 4.1 step 2: user_captures.json
	SourceNone    Source = ""        // nothing decided (unclassified capture, unknown id)
)

// Block is one placed block as the planner sees it.
type Block struct {
	Row     int `json:"row"`
	Col     int `json:"col"`
	ModelID int `json:"model_id"`
	// ModelerType is the Architect's free-text block type. Only a capture reads it.
	ModelerType string `json:"modeler_type,omitempty"`
	// CaptureName is the capture's display name. Only a capture reads it.
	CaptureName string `json:"capture_name,omitempty"`
}

// Classification is the result for one block.
type Classification struct {
	Role   Role
	Source Source
	// Model and Category are the catalog's text for the id ("" when the id is unknown).
	Model    string
	Category string
	Capture  bool
	// Decided is false for a block whose role could not be determined: an unknown model id,
	// or a capture neither the modeler label nor the library settles.
	Decided bool
	// Unclassified marks a capture with no decision (4.1 step 3).
	Unclassified bool
	// Disagreement is set when the modeler label and the library both decide and differ;
	// the modeler's answer is the Role.
	Disagreement string
	// Reason explains a no-switch result.
	Reason string
}

// Classifier assigns roles from the device catalog.
type Classifier struct {
	byID    map[int]*catalog.Model
	library CaptureLookup
}

// NewClassifier builds a classifier over cat. library may be nil.
func NewClassifier(cat *catalog.Catalog, library CaptureLookup) *Classifier {
	c := &Classifier{byID: make(map[int]*catalog.Model, len(cat.Models)), library: library}
	for i := range cat.Models {
		c.byID[cat.Models[i].ID] = &cat.Models[i]
	}
	return c
}

// NewEmbeddedClassifier uses the embedded catalog.
func NewEmbeddedClassifier(library CaptureLookup) (*Classifier, error) {
	cat, err := catalog.Embedded()
	if err != nil {
		return nil, err
	}
	return NewClassifier(cat, library), nil
}

const captureCategory = "Neural Capture"

// roleFromCategory is the section 4 table for categories that map straight to a role.
var roleFromCategory = map[string]Role{
	"Compressor": RoleComp,
	"Delay":      RoleDelay,
	"Modulation": RoleMod,
	"Reverb":     RoleReverb,
}

// driveOrBoost applies the one name rule: contains "boost", any case.
func driveOrBoost(name string) Role {
	if strings.Contains(strings.ToLower(name), "boost") {
		return RoleBoost
	}
	return RoleDrive
}

// modelerRole is the fixed table of section 4.1 step 1. decided=false means "undecided: go
// to the library". The key is the type lower-cased and trimmed, nothing fuzzier.
var modelerRole = map[string]struct {
	role         Role
	driveOrBoost bool
}{
	"boost":                  {role: RoleBoost},
	"drive":                  {driveOrBoost: true},
	"overdrive":              {driveOrBoost: true},
	"distortion":             {driveOrBoost: true},
	"fuzz":                   {driveOrBoost: true},
	"overdrive (capture)":    {driveOrBoost: true},
	"overdrive capture":      {driveOrBoost: true},
	"neural capture (drive)": {driveOrBoost: true},
	"compressor":             {role: RoleComp},
	"delay":                  {role: RoleDelay},
	"modulation":             {role: RoleMod},
	"reverb":                 {role: RoleReverb},
	"amplifier":              {role: RoleNone},
	"cabinet":                {role: RoleNone},
	"preamp":                 {role: RoleNone},
}

// libraryRole maps a user_captures.json block_type.
func libraryRole(blockType, captureName string) (Role, bool) {
	switch blockType {
	case "drive":
		return driveOrBoost(captureName), true
	case "amp", "full_rig", "cab", "other":
		return RoleNone, true
	}
	return RoleNone, false
}

// Classify decides one block's role.
func (c *Classifier) Classify(b Block) Classification {
	m, ok := c.byID[b.ModelID]
	if !ok {
		return Classification{Reason: fmt.Sprintf("unknown model id %d", b.ModelID)}
	}
	cl := Classification{Model: m.Name, Category: m.Category, Decided: true, Source: SourceCatalog}
	if m.Category == captureCategory {
		return c.classifyCapture(b, cl)
	}
	switch {
	case m.Category == "Guitar Overdrive":
		cl.Role = driveOrBoost(m.Name)
	case roleFromCategory[m.Category] != RoleNone:
		cl.Role = roleFromCategory[m.Category]
	default:
		cl.Reason = "no role for category " + m.Category
	}
	return cl
}

func (c *Classifier) classifyCapture(b Block, cl Classification) Classification {
	cl.Capture = true
	name := strings.TrimSpace(b.CaptureName)
	if name != "" {
		// The catalog row behind a capture block is only a slot ("Eltron 30" is id 14000, the V1 block), not the
		// capture: label it with the capture's own name (F4, owner's dry run 2026-11-06).
		cl.Model = name
	}

	// Step 1: the modeler's label, through the fixed table.
	var mRole Role
	mDecided := false
	if e, hit := modelerRole[strings.ToLower(strings.TrimSpace(b.ModelerType))]; hit {
		mDecided = true
		mRole = e.role
		if e.driveOrBoost {
			mRole = driveOrBoost(name)
		}
	}

	// Step 2: the capture library, by exact name.
	var lRole Role
	lDecided := false
	lType := ""
	if c.library != nil && name != "" {
		if bt, hit := c.library(name); hit {
			lType = bt
			lRole, lDecided = libraryRole(bt, name)
		}
	}

	switch {
	case mDecided:
		cl.Role, cl.Source = mRole, SourceModeler
		if lDecided && lRole != mRole {
			cl.Disagreement = fmt.Sprintf("modeler type %q gives %s but the capture library (%s) gives %s; the modeler wins",
				b.ModelerType, roleText(mRole), lType, roleText(lRole))
		}
	case lDecided:
		cl.Role, cl.Source = lRole, SourceLibrary
	default:
		cl.Decided, cl.Unclassified, cl.Source = false, true, SourceNone
		cl.Reason = "unclassified capture (no usable modeler type, not in the capture library)"
		return cl
	}
	if cl.Role == RoleNone {
		cl.Reason = "capture is not a switchable effect (" + string(cl.Source) + ")"
	}
	return cl
}

func roleText(r Role) string {
	if r == RoleNone {
		return "no switch"
	}
	return string(r)
}
