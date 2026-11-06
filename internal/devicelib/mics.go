package devicelib

import (
	"strings"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
)

// MaxMicString bounds one mic string: "<prefix>_<internal cab name>_<mic>".
const MaxMicString = 128

// Harvester finds the cab mic strings in a read-back preset. A cab's mic is the string
// value of its "ir selector" parameters. The strings are untrusted device text, so only
// one that is printable ASCII, unchanged by clean-up and a variation of the cab's OWN
// default string (same "<prefix>_<internal cab name>", different mic) is kept: an IR the
// owner loaded from somewhere else, or anything else a preset can carry in that slot, is
// not recorded.
type Harvester struct {
	cabs map[int]*cabInfo
}

type cabInfo struct {
	selectors []int // ir selector indexes, in order (slot 1, slot 2)
	cabParts  map[string]bool
}

// NewHarvester indexes the catalog's cab models.
func NewHarvester(cat *catalog.Catalog) *Harvester {
	h := &Harvester{cabs: map[int]*cabInfo{}}
	if cat == nil {
		return h
	}
	for i := range cat.Models {
		m := &cat.Models[i]
		if !strings.HasPrefix(m.Category, "Cabsim") {
			continue
		}
		ci := &cabInfo{cabParts: map[string]bool{}}
		for _, p := range m.Params {
			if p.Name == "ir selector" {
				ci.selectors = append(ci.selectors, p.Index)
				if cp := CabPart(p.DefaultString); cp != "" {
					ci.cabParts[cp] = true
				}
			}
		}
		if len(ci.selectors) > 0 && len(ci.cabParts) > 0 {
			h.cabs[m.ID] = ci
		}
	}
	return h
}

// CabPart is a mic string without its mic: everything before the last underscore
// ("ME_110 US PRN C10R" for "ME_110 US PRN C10R_Dynamic 57"), or "" when there is none.
func CabPart(s string) string {
	i := strings.LastIndex(s, "_")
	if i <= 0 {
		return ""
	}
	return s[:i]
}

// MicPart is the mic after the last underscore.
func MicPart(s string) string {
	i := strings.LastIndex(s, "_")
	if i < 0 || i == len(s)-1 {
		return ""
	}
	return s[i+1:]
}

// ValidMic reports whether s is a mic string this package would keep for a cab whose
// default strings have the given cab parts.
func validMic(s string, cabParts map[string]bool) bool {
	if len(s) == 0 || len(s) > MaxMicString || strings.TrimSpace(s) != s || strings.Contains(s, "  ") || strings.Contains(s, "`") {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return cabParts[CabPart(s)] && MicPart(s) != ""
}

// Harvest returns, per cab model id, the distinct mic strings in the read-back, and how
// many string values it looked at and left out.
func (h *Harvester) Harvest(p *devicejobs.ReadbackPayload) (seen map[int][]string, ignored int) {
	seen = map[int][]string{}
	if p == nil {
		return seen, 0
	}
	for _, ch := range p.Chains {
		for _, b := range ch.Blocks {
			ci := h.cabs[b.ModelID]
			if ci == nil {
				continue
			}
			byIndex := map[int]devicejobs.ReadbackParam{}
			for _, pr := range b.Params {
				byIndex[pr.Index] = pr
			}
			for _, idx := range ci.selectors {
				pr, ok := byIndex[idx]
				if !ok {
					continue
				}
				v, ok := pr.Text(0)
				if !ok {
					continue
				}
				same := true // a mic string is one value for all eight scenes
				for s := 1; s < ReadbackScenes; s++ {
					if t, ok := pr.Text(s); !ok || t != v {
						same = false
					}
				}
				if !same || !validMic(v, ci.cabParts) {
					ignored++
					continue
				}
				dup := false
				for _, e := range seen[b.ModelID] {
					dup = dup || e == v
				}
				if !dup {
					seen[b.ModelID] = append(seen[b.ModelID], v)
				}
			}
		}
	}
	return seen, ignored
}

// ReadbackScenes is the number of scene values a read-back parameter carries.
const ReadbackScenes = devicejobs.ReadbackScenes

// ValidMic is validMic for a cab model, used by the exporter before it writes a string.
func (h *Harvester) ValidMic(modelID int, s string) bool {
	ci := h.cabs[modelID]
	return ci != nil && validMic(s, ci.cabParts)
}
