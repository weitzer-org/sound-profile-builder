package devicelib

import (
	"fmt"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
)

// Status is the outcome of resolving one capture name against the synced list.
type Status string

const (
	StatusOnUnit         Status = "on_unit"
	StatusNotOnUnit      Status = "not_on_unit"
	StatusWrongKind      Status = "wrong_kind"
	StatusCloudOnly      Status = "cloud_only"
	StatusAmbiguous      Status = "ambiguous"
	StatusVersionUnknown Status = "version_unknown"
)

// Resolution is what the dry run shows for one capture block. The key is the unit's
// 64-hex key, shown shortened; the agent finds it again itself at push time.
type Resolution struct {
	Status   Status
	Reason   string // for a status other than on_unit; written for the user
	UnitName string // the exact name on the unit (differs from the asked name only by alias)
	Aliased  bool
	Key      string
	Version  int
	Kind     string
	Folder   string
}

// Resolve applies the agent's own rules (qc_agent.captures.resolve_one) to the stored
// list: an exact, case-sensitive name match first; only when there is none, one reviewed
// alias whose target must itself be an exact entry of the list; never fuzzy. The same
// kind (factory or user) must match, a cloud-only entry is not a file on the unit, two
// keys under one name are ambiguous, and the V1/V2 version must be known and single.
func Resolve(lib *Library, aliases *Aliases, name, source string) Resolution {
	find := func(n string) []Capture {
		var out []Capture
		for _, c := range lib.Captures {
			if c.Name == n {
				out = append(out, c)
			}
		}
		return out
	}
	unitName, aliased := name, false
	cands := find(name)
	if len(cands) == 0 {
		if to, ok := aliases.Lookup(name, source); ok {
			if cands = find(to); len(cands) > 0 {
				unitName, aliased = to, true
			}
		}
	}
	if len(cands) == 0 {
		return Resolution{Status: StatusNotOnUnit, Reason: "not on your unit: no capture there has exactly this name. Download it in Cortex Control, or refine the preset to use a capture you have"}
	}
	var same []Capture
	for _, c := range cands {
		if c.Kind == source {
			same = append(same, c)
		}
	}
	if len(same) == 0 {
		other := devicejobs.CaptureKindUser
		if source == devicejobs.CaptureKindUser {
			other = devicejobs.CaptureKindFactory
		}
		return Resolution{Status: StatusWrongKind, Reason: fmt.Sprintf("QC-2's library has this as a %s capture, but your unit has it only as a %s capture", source, other)}
	}
	var local []Capture
	for _, c := range same {
		if !c.Cloud {
			local = append(local, c)
		}
	}
	if len(local) == 0 {
		return Resolution{Status: StatusCloudOnly, Reason: "listed on your unit only as a Cortex Cloud item, not a file on the unit: download it in Cortex Control first"}
	}
	keys := map[string]bool{}
	versions := map[int]bool{}
	for _, c := range local {
		keys[strings.ToLower(c.Key)] = true
		versions[c.Version] = true
	}
	if len(keys) > 1 {
		return Resolution{Status: StatusAmbiguous, Reason: fmt.Sprintf("matches %d different captures on your unit; the bridge won't guess which one", len(keys))}
	}
	if len(versions) != 1 || versions[0] {
		return Resolution{Status: StatusVersionUnknown, Reason: "the bridge could not tell whether this is a V1 or V2 capture on your unit, so it would not pick a block type"}
	}
	c := local[0]
	return Resolution{Status: StatusOnUnit, UnitName: unitName, Aliased: aliased, Key: c.Key, Version: c.Version, Kind: c.Kind, Folder: c.Folder}
}
