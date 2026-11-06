// Package deviceexport turns one guitar's chain of a stored preset into exact Quad Cortex
// operations: catalog model ids, parameter indexes, display and wire values for all eight
// scenes, per-scene bypass and a row-0 grid placement (plan section 4, format
// qc2-device-export/1).
//
// It is deterministic and calls no LLM. It refuses rather than guesses: anything it can't
// map exactly, or that rests on device behaviour nobody has measured on the owner's unit,
// goes to Unresolved with a reason, and a push needs zero unresolved items
// (devicejobs.Bridge enforces that). What has been measured is recorded in
// docs/design/qc-device-bridge-plan.md section 7 and, as data, in
// internal/catalog/firmware_constants.go (ConfirmedSkews, VerifiedCloneBases,
// FirmwareBounds evidence).
package deviceexport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Format is the payload schema this package writes.
const Format = "qc2-device-export/1"

// Grid placement in v1 (plan section 4): one row, the chain left to right, routed from
// input 1 to output 19 (qc-mcp clear_grid's default routing; G3 read back in=1 out=19).
const (
	GridRow   = 0
	MaxBlocks = 8
	InPort    = 1
	OutPort   = 19
	Scenes    = 8
)

// SceneSources says which preset value each of the device's eight scenes A-H gets. G6
// measured that the device does not copy Scene A into C-H (an unwritten scene keeps the
// parameter's earlier value), so every scene is written explicitly; C-H take Scene A.
var SceneSources = [Scenes]string{"A", "B", "A", "A", "A", "A", "A", "A"}

// Options configures an Exporter.
type Options struct {
	// IsCapture reports block names coros_map.json or user_captures.json mark as captures.
	// A capture wins over a catalog model of the same name, as in the pipeline.
	IsCapture func(name string) bool
	// OwnedPlugins holds the plugins the user owns, by catalog.PluginNames spelling
	// (config.json's available_plugins when allow_paid_plugins is on). Models of any other
	// plugin are refused.
	OwnedPlugins map[string]bool
}

// Exporter implements devicejobs.Exporter.
type Exporter struct {
	cat       *catalog.Catalog
	catalogSH string
	opts      Options
}

var _ devicejobs.Exporter = (*Exporter)(nil)

// New builds an Exporter over a catalog. catalogSHA256 is the hash recorded in the export
// header: the SHA-256 of the qc_catalog.json the catalog was decoded from.
func New(cat *catalog.Catalog, catalogSHA256 string, o Options) *Exporter {
	return &Exporter{cat: cat, catalogSH: catalogSHA256, opts: o}
}

// NewEmbedded builds an Exporter over the catalog compiled into the binary.
func NewEmbedded(o Options) (*Exporter, error) {
	cat, err := catalog.Embedded()
	if err != nil {
		return nil, err
	}
	return New(cat, catalog.EmbeddedSHA256(), o), nil
}

// OwnedPluginSet builds Options.OwnedPlugins from config.json's allow_paid_plugins and
// available_plugins. A name the plugin table doesn't know is dropped: it can't match a
// catalog model's plugin.
func OwnedPluginSet(allowPaid bool, available []string) map[string]bool {
	out := map[string]bool{}
	if !allowPaid {
		return out
	}
	for _, n := range available {
		if c, ok := catalog.CanonicalPluginName(n); ok {
			out[c] = true
		}
	}
	return out
}

// Payload is the qc2-device-export/1 document. Field order is fixed (structs, never maps),
// so the bytes, and the plan hash the confirm token binds, are deterministic.
type Payload struct {
	Format       string                  `json:"format"`
	Catalog      CatalogRef              `json:"catalog"`
	Guitar       string                  `json:"guitar"`
	SceneSources [Scenes]string          `json:"scene_sources"`
	Rows         []Row                   `json:"rows"`
	Unresolved   []devicejobs.Unresolved `json:"unresolved"`
}

// CatalogRef names the catalog the ids and indexes come from.
type CatalogRef struct {
	CorOS  string `json:"coros"`
	Origin string `json:"origin"`
	SHA256 string `json:"sha256"`
}

// Row is one grid row with its routing.
type Row struct {
	Row     int     `json:"row"`
	InPort  int     `json:"in_port"`
	OutPort int     `json:"out_port"`
	Blocks  []Block `json:"blocks"`
}

// Block is one block placed on the grid.
type Block struct {
	Col      int    `json:"col"`
	ModelID  int    `json:"model_id"`
	Model    string `json:"model"`
	Category string `json:"category"`
	// IndexEvidence says what backs this model's parameter indexes: "plain" (the model's
	// own ModelRepo.xml list) or "clone-<base>-g1" (a clone whose expanded layout was
	// checked against the device in G1).
	IndexEvidence string `json:"index_evidence"`
	// Plugin is the paid plugin the model needs, when it is one (owned, or it would be
	// unresolved).
	Plugin string `json:"plugin,omitempty"`
	// Bypass is the block's bypass state in scenes A-H (true = bypassed). Always all eight.
	Bypass [Scenes]bool `json:"bypass"`
	Params []Param      `json:"params"`
	// AtDefault lists controls the preset sets to the catalog default in every scene. They
	// are not written: a block added to an empty slot starts at its defaults.
	AtDefault []string `json:"at_default"`
}

// Param is one parameter write.
type Param struct {
	Index int    `json:"index"`
	Label string `json:"label"`
	Units string `json:"units,omitempty"`
	// SceneMode is true when the scenes differ; the agent then writes all eight values.
	SceneMode bool            `json:"scene_mode"`
	Display   [Scenes]float64 `json:"display"`
	Wire      [Scenes]float64 `json:"wire"`
}

// Export builds the export for one guitar. It returns an error only for faults (the guitar
// isn't in the preset, an index fails the catalog's own check); every mapping problem is
// an Unresolved item.
func (e *Exporter) Export(_ context.Context, sp storage.StructuredPreset, guitar string) (*devicejobs.Export, error) {
	chain, ok := sp.Guitars[guitar]
	if !ok {
		return nil, fmt.Errorf("guitar %q is not in this preset", truncate(guitar, 60))
	}
	if e.cat == nil {
		return nil, errors.New("device catalog unavailable")
	}
	b := &builder{e: e, legacy: !usesBasis(sp)}
	p := Payload{
		Format:       Format,
		Catalog:      CatalogRef{CorOS: e.cat.Source.CorOSVersion, Origin: e.cat.Source.Origin, SHA256: e.catalogSH},
		Guitar:       guitar,
		SceneSources: SceneSources,
		Rows:         []Row{{Row: GridRow, InPort: InPort, OutPort: OutPort, Blocks: []Block{}}},
	}
	col := 0
	for i := range chain {
		blk := &chain[i]
		ref := fmt.Sprintf("#%d %s", i+1, truncate(strings.TrimSpace(blk.Model), 80))
		if isDeviceElement(blk.Model) {
			b.unresolvedElement(ref, blk.Model)
			continue
		}
		if col >= MaxBlocks {
			col++
			continue
		}
		placed, err := b.block(ref, blk, col)
		if err != nil {
			return nil, err
		}
		if placed != nil {
			p.Rows[0].Blocks = append(p.Rows[0].Blocks, *placed)
		}
		col++
	}
	if col > MaxBlocks {
		b.unresolved = append([]devicejobs.Unresolved{{
			Block:  "(chain)",
			Reason: fmt.Sprintf("the chain has %d grid blocks; v1 places at most %d on row 0 (blocks after the 8th were not mapped)", col, MaxBlocks),
		}}, b.unresolved...)
	}
	p.Unresolved = b.unresolved
	if p.Unresolved == nil {
		p.Unresolved = []devicejobs.Unresolved{}
	}
	raw, err := marshal(p)
	if err != nil {
		return nil, err
	}
	return &devicejobs.Export{Format: Format, Payload: raw, Summary: summary(p), Unresolved: p.Unresolved}, nil
}

// marshal encodes compactly with Go's default HTML escaping, so preset text such as a
// block name never carries a raw "<", ">" or "&" into a Markdown or HTML context that
// shows the payload. The escapes decode to the same strings.
func marshal(p Payload) (json.RawMessage, error) {
	return json.Marshal(p)
}

// summary is the readable plan devicejobs shows in a dry run.
func summary(p Payload) []devicejobs.PlanBlock {
	out := []devicejobs.PlanBlock{}
	for _, r := range p.Rows {
		for _, bl := range r.Blocks {
			pb := devicejobs.PlanBlock{Row: r.Row, Col: bl.Col, Model: bl.Model, ModelID: bl.ModelID,
				Params: []devicejobs.PlanParam{}, BypassA: bl.Bypass[0], BypassB: bl.Bypass[1], AtDefault: bl.AtDefault}
			for _, pr := range bl.Params {
				pp := devicejobs.PlanParam{Name: pr.Label, A: displayString(pr.Display[0], pr.Units)}
				if pr.Display[1] != pr.Display[0] {
					pp.B = displayString(pr.Display[1], pr.Units)
				}
				pb.Params = append(pb.Params, pp)
			}
			out = append(out, pb)
		}
	}
	return out
}

// usesBasis reports whether any parameter of the preset carries a basis field. Architect
// prompts v4 and later require one on every parameter, and v4 is also where "Bypass: On"
// was defined as bypassed (Rule 13), so a preset with none predates that definition.
func usesBasis(sp storage.StructuredPreset) bool {
	for _, chain := range sp.Guitars {
		for _, b := range chain {
			for _, p := range b.Parameters {
				if strings.TrimSpace(p.Basis) != "" {
					return true
				}
			}
		}
	}
	return false
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
