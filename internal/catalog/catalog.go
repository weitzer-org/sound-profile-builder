// Package catalog is the Quad Cortex's own device catalog: every block model the unit
// offers (amps, cabs, drives, delays, ...) and the exact parameters each one exposes, with
// real names, ranges, defaults, units and switch positions.
//
// The data comes from the unit's ModelRepo.xml, which the device sends over USB when an
// editor such as Cortex Control asks for it (ModelRepo message, type 51). That makes it the
// ground truth for what exists on the device, as opposed to coros_map.json (a hand-built
// real-gear -> block mapping) or qc_block_schema.json (generic per-category parameter
// names researched from the manual, with no ranges).
//
// qc_catalog.json is derived from ModelRepo.xml by cmd/import_catalog. The raw XML is
// deliberately not committed; only the fields this app uses are kept. Re-run the importer
// after a CorOS update (it has a -diff mode) rather than editing qc_catalog.json by hand.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed qc_catalog.json
var embeddedCatalog []byte

// Catalog is the derived, committed form of the device's ModelRepo.xml.
type Catalog struct {
	Readme string  `json:"_readme"`
	Source Source  `json:"source"`
	Models []Model `json:"models"`

	byName   map[string]*Model
	byFamily map[string][]*Model
	// allByName keeps every model with a given name. A handful of names appear twice in
	// the device's catalog (e.g. two "Graphic-9" equalizers with different parameter sets);
	// byName keeps only the last one for Lookup's single-result API.
	allByName map[string][]*Model
}

// Source records where a catalog snapshot came from, so a stale one is recognizable.
type Source struct {
	CorOSVersion string `json:"coros_version"`
	Origin       string `json:"origin"`
	ModelCount   int    `json:"model_count"`
}

// Model is one block model the device offers. ID is the catalog id, which is also the
// block's model hash on the wire.
type Model struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	// BasedOn is the device's own real-gear reference text, e.g.
	// "Based on Marshall® JCM800®". Empty for many plugin and generic models.
	BasedOn string `json:"based_on,omitempty"`
	// PluginID is set for models that belong to a paid Neural DSP plugin. The model
	// appears in every unit's catalog, but using it needs that plugin's licence.
	PluginID string  `json:"plugin_id,omitempty"`
	Params   []Param `json:"params"`
}

// Param is one user-visible parameter of a model.
//
// Names are not unique within a model: dual-mic cabs, for example, list POSITION,
// DISTANCE, LEVEL and PAN once per mic. Index (the parameter's position among all of the
// model's <Parameter> elements in ModelRepo.xml, hidden ones included) is what tells them
// apart, and is how the device itself addresses a parameter.
type Param struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	// Min/Max are nil when the XML gives a symbolic bound instead of a number (see
	// MinSymbol/MaxSymbol). The device resolves those from a table compiled into its
	// firmware, so the real range is unknown here and must not be guessed.
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	MinSymbol string   `json:"min_symbol,omitempty"`
	MaxSymbol string   `json:"max_symbol,omitempty"`
	Default   *float64 `json:"default,omitempty"`
	Units     string   `json:"units,omitempty"`
	// Steps is the number of discrete positions for switch-like parameters.
	Steps     int      `json:"steps,omitempty"`
	StepNames []string `json:"step_names,omitempty"`
	// MinLabel/MaxLabel are what the device displays at the ends of the range,
	// e.g. "OFF", or "L"/"R" for a pan control.
	MinLabel   string `json:"min_label,omitempty"`
	MaxLabel   string `json:"max_label,omitempty"`
	DisplayPos *int   `json:"display_pos,omitempty"`
}

// RangeKnown reports whether both bounds are numeric.
func (p Param) RangeKnown() bool { return p.Min != nil && p.Max != nil }

var monoStereoSuffix = regexp.MustCompile(`\s*\((M|ST)\)$`)

// FamilyName strips a trailing " (M)" or " (ST)" mono/stereo suffix. The device lists the
// mono and stereo versions of a cab or effect as separate models ("412 Brit 60B V30 95 (M)"
// and "... (ST)"); coros_map.json maps real gear to the family, not to one channel mode.
func FamilyName(name string) string {
	return monoStereoSuffix.ReplaceAllString(strings.TrimSpace(name), "")
}

func (c *Catalog) index() {
	c.byName = make(map[string]*Model, len(c.Models))
	c.byFamily = make(map[string][]*Model)
	c.allByName = make(map[string][]*Model, len(c.Models))
	for i := range c.Models {
		m := &c.Models[i]
		c.byName[m.Name] = m
		c.allByName[m.Name] = append(c.allByName[m.Name], m)
		fam := FamilyName(m.Name)
		c.byFamily[fam] = append(c.byFamily[fam], m)
	}
}

// Lookup returns the model with exactly this name.
func (c *Catalog) Lookup(name string) (*Model, bool) {
	m, ok := c.byName[name]
	return m, ok
}

// Family returns every model in a mono/stereo family, e.g. "412 Brit 60B V30 95" returns
// its (M) and (ST) models. A model with no suffix is a family of one.
func (c *Catalog) Family(family string) []*Model {
	return c.byFamily[family]
}

// Resolve returns every model a block name can refer to: all models with exactly this
// name (some names appear more than once, with different parameters), or failing that every
// member of the mono/stereo family it names. Nil when the name isn't on the device. The
// result is shared; callers must not modify it.
func (c *Catalog) Resolve(name string) []*Model {
	if ms := c.allByName[name]; len(ms) > 0 {
		return ms
	}
	return c.byFamily[name]
}

// IsKnownName reports whether name is either an exact model name or a mono/stereo family
// name on the device.
func (c *Catalog) IsKnownName(name string) bool {
	if _, ok := c.byName[name]; ok {
		return true
	}
	return len(c.byFamily[name]) > 0
}

// Names returns every exact model name, sorted.
func (c *Catalog) Names() []string {
	out := make([]string, 0, len(c.byName))
	for n := range c.byName {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Decode parses a qc_catalog.json document.
func Decode(data []byte) (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	c.index()
	return &c, nil
}

var (
	embeddedOnce sync.Once
	embedded     *Catalog
	embeddedErr  error
)

// Embedded returns the catalog compiled into the binary. The result is shared; callers
// must not modify it.
func Embedded() (*Catalog, error) {
	embeddedOnce.Do(func() {
		embedded, embeddedErr = Decode(embeddedCatalog)
	})
	return embedded, embeddedErr
}
