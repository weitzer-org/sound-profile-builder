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
	"github.com/weitzer-org/sound-builder/internal/devicelib"
	"github.com/weitzer-org/sound-builder/internal/stomplayout"
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

// sceneTokens says which preset value each of the device's eight scenes A-H gets: "A" and "B"
// are the first guitar's Scene A (rhythm) and Scene B (lead) values, "2A" and "2B" the second
// guitar's. G6 measured that the device does not copy Scene A into C-H (an unwritten scene
// keeps the parameter's earlier value), so every scene is written explicitly; a scene no tone
// is assigned to takes the first guitar's rhythm value, "A". The default layout (one guitar on
// A and B) gives A, B, A, A, A, A, A, A.
func sceneTokens(l devicejobs.SceneLayout) [Scenes]string {
	var out [Scenes]string
	for s, so := range l.Sources() {
		tok := "A"
		if so.Lead {
			tok = "B"
		}
		if so.Guitar > 0 {
			tok = fmt.Sprintf("%d%s", so.Guitar+1, tok)
		}
		out[s] = tok
	}
	return out
}

// Options configures an Exporter.
type Options struct {
	// CaptureSource reports whether a block name is a capture the user's libraries know:
	// "factory" (coros_map.json) or "user" (user_captures.json), "ir" for a library entry
	// that is a cab IR, "" when it isn't one. A capture wins over a catalog model of the
	// same name, as in the pipeline.
	CaptureSource func(name string) string
	// OwnedPlugins holds the plugins the user owns, by catalog.PluginNames spelling
	// (config.json's available_plugins when allow_paid_plugins is on). Models of any other
	// plugin are refused.
	OwnedPlugins map[string]bool
	// StompCaptures is the capture library lookup for the stomp layout (design doc
	// qc-stomp-layout.md section 4.1 step 2); the caller supplies it so this package
	// stays free of internal/agents. Nil means no library. In v1 captures are never
	// placed, so it is not consulted yet.
	StompCaptures stomplayout.CaptureLookup
	// LookupCapture resolves a capture name read from the unit to its entry in
	// coros_map.json or user_captures.json (exact name, case-insensitive; source is
	// "factory" or "user"). The read-back import uses it; nil means every capture keeps the
	// name the unit shows. The caller supplies it so this package stays free of
	// internal/agents.
	LookupCapture func(deviceName string) (name, source string, ok bool)
	// Sanitize cleans a string read from the unit (control and format characters, backticks,
	// whitespace runs) and cuts it to n runes. The read-back import requires it:
	// cmd/server passes devicejobs.SanitizeText, the function mcpserver's sanitizeDeviceText is, so import and display share one definition.
	Sanitize func(s string, n int) string
	// Device loads what QC-2 knows about the unit's contents (plan P4b): the synced capture
	// list, the reviewed alias table and the cab mics seen in presets the user asked to read.
	// Nil, or a snapshot with no library, leaves the export exactly as it was before the
	// sync existed: a capture is resolved on the unit at push time and a non-default cab
	// mic is a warning.
	Device func(ctx context.Context) (*devicelib.Snapshot, error)
}

// Exporter implements devicejobs.Exporter.
type Exporter struct {
	cat       *catalog.Catalog
	catalogSH string
	opts      Options
	stompCls  *stomplayout.Classifier
	byID      map[int]*catalog.Model // model id -> model, for the read-back import
	harv      *devicelib.Harvester   // validates a seen cab mic string before it is written
}

var _ devicejobs.Exporter = (*Exporter)(nil)

// New builds an Exporter over a catalog. catalogSHA256 is the hash recorded in the export
// header: the SHA-256 of the qc_catalog.json the catalog was decoded from.
func New(cat *catalog.Catalog, catalogSHA256 string, o Options) *Exporter {
	e := &Exporter{cat: cat, catalogSH: catalogSHA256, opts: o}
	if cat != nil {
		e.byID = make(map[int]*catalog.Model, len(cat.Models))
		for i := range cat.Models {
			e.byID[cat.Models[i].ID] = &cat.Models[i]
		}
		e.stompCls = stomplayout.NewClassifier(cat, o.StompCaptures)
		e.harv = devicelib.NewHarvester(cat)
	}
	return e
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
	Format       string         `json:"format"`
	Catalog      CatalogRef     `json:"catalog"`
	Guitar       string         `json:"guitar"`
	SceneSources [Scenes]string `json:"scene_sources"`
	Rows         []Row          `json:"rows"`
	// Stomps is the footswitch layout for the placed blocks (design doc qc-stomp-layout.md
	// section 6.1, entry point 1). Always an array, never null. UNVERIFIED on the unit
	// until gate G7.
	Stomps     []stomplayout.Stomp     `json:"stomps"`
	Unresolved []devicejobs.Unresolved `json:"unresolved"`
	// Warnings are settings the push leaves out on purpose (plan 4.1: a non-default cab
	// mic keeps the cab's default). They don't block a push.
	Warnings []devicejobs.Unresolved `json:"warnings"`
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
	// Capture is set on a Neural Capture block (plan 4.2). Its ModelID is 0: the agent picks
	// 14000 (V1) or 14001 (V2) from the capture's version on the unit, where it also finds
	// the capture's key. Params use the capture block's own layout (GAIN..VOLUME, 0-4),
	// which both block types share.
	Capture *Capture `json:"capture,omitempty"`

	modelerType string // the preset's block type, for the stomp layout of a capture
	// Bypass is the block's bypass state in scenes A-H (true = bypassed). Always all eight.
	Bypass [Scenes]bool `json:"bypass"`
	Params []Param      `json:"params"`
	// AtDefault lists controls the preset sets to the catalog default in every scene. They
	// are not written: a block added to an empty slot starts at its defaults.
	AtDefault []string `json:"at_default"`
	// Mics are cab mic strings written exactly (plan P4b): a mic the preset asks for that
	// was seen on the unit, in a preset the user asked to read, for this same cab. Absent
	// unless used, so an export without them is byte-identical to one before they existed.
	Mics []Mic `json:"mics,omitempty"`
}

// Mic is one cab mic string write: the cab's "ir selector" parameter for the mic slot.
type Mic struct {
	Slot  int    `json:"slot"`  // 1 or 2: the cab's MIC 1 / MIC 2
	Index int    `json:"index"` // the catalog's ir selector parameter index for that slot
	Value string `json:"value"` // the exact string seen on the unit, "<prefix>_<cab>_<mic>"
	// Evidence is "seen-on-unit". Whether the unit accepts the string is UNVERIFIED on
	// hardware until the README's library-sync step has been run.
	Evidence string `json:"evidence"`
}

// Capture names the capture a Neural Capture block loads.
type Capture struct {
	// Name is the capture's name as the preset gives it (the library entry's name).
	Name string `json:"name"`
	// Source is "factory" (coros_map.json) or "user" (user_captures.json).
	Source string `json:"source"`
	// ResolvedOnUnit is false until the unit's capture list is synced to QC-2 (plan P4b):
	// the agent then resolves the name against a fresh read of the unit's captures and
	// fails the job, writing nothing, when it isn't there.
	ResolvedOnUnit bool `json:"resolved_on_unit"`

	// res is the synced list's answer, for the readable plan only. It is not in the payload:
	// the agent always resolves against its own fresh listing.
	res        *devicelib.Resolution
	presetName string // the name the preset used, when an alias replaced Name
	syncedAt   string
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
	// Evidence says what the display-to-wire mapping rests on (plan 4.1), e.g.
	// "taper:g4" (read on the owner's unit), "taper:device-xml" (the device's own skew,
	// formula confirmed at other exponents), "taper:pyquadcortex", "bounds:g4",
	// "bounds:pyquadcortex-4.0.1", "encoding:qc-mcp-linear" (switch positions),
	// "encoding:fader-linear-guess" (a fader converted like a float: UNVERIFIED, FaderEvidence).
	Evidence []string `json:"evidence"`

	names [Scenes]string // each scene's position name of a switch, for the readable plan
	// screen is the unit's on-screen reading of a 0..10 pan per scene, inferred (screenPan); shown beside the
	// number in the readable plan only. Never part of the payload or the plan hash.
	screen [Scenes]string
}

// Confirmed reports whether every part of the mapping was read on the owner's unit.
func (p Param) Confirmed() bool {
	for _, e := range p.Evidence {
		if !strings.HasSuffix(e, ":g4") {
			return false
		}
	}
	return true
}

// Export builds the export for one guitar. It returns an error only for faults (the guitar
// isn't in the preset, an index fails the catalog's own check); every mapping problem is
// an Unresolved item.
func (e *Exporter) Export(ctx context.Context, sp storage.StructuredPreset, guitar string) (*devicejobs.Export, error) {
	return e.ExportLayout(ctx, sp, devicejobs.DefaultLayout(guitar))
}

var _ devicejobs.LayoutExporter = (*Exporter)(nil)

// ExportLayout builds the export for one or two guitars of a preset in one slot. Each guitar's
// Scene A values go on its Rhythm scene and its Scene B values on its Lead scene; scenes
// nobody takes carry the first guitar's rhythm values. Two guitars must have the same blocks
// in the same order (their parameter values may differ, and a control only one of them sets
// is at the device default for the other). With the default layout the result is exactly
// Export's.
func (e *Exporter) ExportLayout(ctx context.Context, sp storage.StructuredPreset, layout devicejobs.SceneLayout) (*devicejobs.Export, error) {
	if err := layout.Validate(); err != nil {
		return nil, err
	}
	chains := make([][]storage.EffectBlock, len(layout.Guitars))
	for i, g := range layout.Guitars {
		c, ok := sp.Guitars[g.Guitar]
		if !ok {
			return nil, fmt.Errorf("guitar %q is not in this preset", truncate(g.Guitar, 60))
		}
		chains[i] = c
	}
	chain := chains[0]
	if e.cat == nil {
		return nil, errors.New("device catalog unavailable")
	}
	b := &builder{e: e, legacy: !usesBasis(sp), layout: layout, src: layout.Sources()}
	if e.opts.Device != nil {
		snap, err := e.opts.Device(ctx)
		if err != nil {
			// Not "not synced": the list exists but could not be read. Say so, in the plan.
			b.warn("(device lists)", "", "the synced capture and cab mic lists could not be read, so captures are checked on the unit at push time and non-default cab mics are left out")
		} else {
			b.dev = snap
		}
	}
	p := Payload{
		Format:       Format,
		Catalog:      CatalogRef{CorOS: e.cat.Source.CorOSVersion, Origin: e.cat.Source.Origin, SHA256: e.catalogSH},
		Guitar:       truncate(layout.Label(), 200),
		SceneSources: sceneTokens(layout),
		Rows:         []Row{{Row: GridRow, InPort: InPort, OutPort: OutPort, Blocks: []Block{}}},
	}
	if !sameChains(b, layout, chains) {
		// Nothing is placed: the blocks of two tones that don't line up can't share one slot.
		chain = nil
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
		blks := make([]*storage.EffectBlock, len(chains))
		for g := range chains {
			blks[g] = &chains[g][i]
		}
		placed, err := b.block(ref, blks, col)
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
	p.Warnings = b.warnings
	if p.Warnings == nil {
		p.Warnings = []devicejobs.Unresolved{}
	}
	stomps, err := e.planStomps(p.Rows)
	if err != nil {
		return nil, err
	}
	p.Stomps = stomps.assign
	raw, err := marshal(p)
	if err != nil {
		return nil, err
	}
	out := &devicejobs.Export{Format: Format, Payload: raw, Summary: summary(p, layout), Unresolved: p.Unresolved, Warnings: p.Warnings, Stomps: stomps.view}
	if !layout.IsDefault() {
		out.Layout = planLayout(layout)
	}
	return out, nil
}

// sameChains reports whether the layout's guitars have the same blocks in the same order,
// and records an unresolved item when they don't. One guitar always matches.
func sameChains(b *builder, layout devicejobs.SceneLayout, chains [][]storage.EffectBlock) bool {
	ok := true
	for g := 1; g < len(chains); g++ {
		a, o := chains[0], chains[g]
		who := fmt.Sprintf("%s and %s", truncate(layout.Guitars[0].Guitar, 40), truncate(layout.Guitars[g].Guitar, 40))
		if len(a) != len(o) {
			b.refuse("(guitars)", "", fmt.Sprintf("%s have different chains (%d and %d blocks): two guitars share one slot only with the same blocks in the same order", who, len(a), len(o)))
			ok = false
			continue
		}
		for i := range a {
			ma, mo := strings.TrimSpace(a[i].Model), strings.TrimSpace(o[i].Model)
			if !strings.EqualFold(ma, mo) {
				b.refuse(fmt.Sprintf("#%d", i+1), "", fmt.Sprintf("%s differ at block %d (%s and %s): two guitars share one slot only with the same blocks in the same order", who, i+1, truncate(ma, 40), truncate(mo, 40)))
				ok = false
			}
		}
	}
	return ok
}

// planLayout is the readable scene assignment of a non-default layout.
func planLayout(l devicejobs.SceneLayout) *devicejobs.PlanLayout {
	pl := &devicejobs.PlanLayout{}
	for s := range pl.Scenes {
		pl.Scenes[s] = devicejobs.PlanScene{Letter: devicejobs.Letter(s), Role: "unused"}
	}
	for _, g := range l.Guitars {
		pl.Scenes[g.Rhythm] = devicejobs.PlanScene{Letter: devicejobs.Letter(g.Rhythm), Guitar: g.Guitar, Role: "rhythm"}
		pl.Scenes[g.Lead] = devicejobs.PlanScene{Letter: devicejobs.Letter(g.Lead), Guitar: g.Guitar, Role: "lead"}
	}
	return pl
}

// marshal encodes compactly with Go's default HTML escaping, so preset text such as a
// block name never carries a raw "<", ">" or "&" into a Markdown or HTML context that
// shows the payload. The escapes decode to the same strings.
func marshal(p Payload) (json.RawMessage, error) {
	return json.Marshal(p)
}

// summary is the readable plan devicejobs shows in a dry run.
func summary(p Payload, layout devicejobs.SceneLayout) []devicejobs.PlanBlock {
	out := []devicejobs.PlanBlock{}
	for _, r := range p.Rows {
		for _, bl := range r.Blocks {
			pb := devicejobs.PlanBlock{Row: r.Row, Col: bl.Col, Model: bl.Model, ModelID: bl.ModelID,
				Params: []devicejobs.PlanParam{}, BypassA: bl.Bypass[0], BypassB: bl.Bypass[1], AtDefault: bl.AtDefault}
			if !layout.IsDefault() {
				pb.BypassScenes = append([]bool(nil), bl.Bypass[:]...)
			}
			if bl.Capture != nil {
				pb.Capture = bl.Capture.Name
				if r := bl.Capture.res; r != nil {
					ci := &devicejobs.PlanCapture{UnitName: r.UnitName, Version: r.Version, Kind: r.Kind, Folder: r.Folder,
						KeyPrefix: keyPrefix(r.Key), SyncedAt: bl.Capture.syncedAt}
					if r.Aliased {
						ci.AskedName = bl.Capture.presetName
					}
					pb.CaptureInfo = ci
				}
			}
			for _, m := range bl.Mics {
				pb.Mics = append(pb.Mics, devicejobs.PlanMic{Label: fmt.Sprintf("MIC %d", m.Slot), Value: m.Value})
			}
			for _, pr := range bl.Params {
				pp := devicejobs.PlanParam{Name: pr.Label, A: pr.shown(0), Unconfirmed: !pr.Confirmed()}
				if pr.Display[1] != pr.Display[0] {
					pp.B = pr.shown(1)
				}
				if !layout.IsDefault() {
					for sc := 0; sc < Scenes; sc++ {
						pp.Scenes = append(pp.Scenes, pr.shown(sc))
					}
				}
				pb.Params = append(pb.Params, pp)
			}
			out = append(out, pb)
		}
	}
	return out
}

func keyPrefix(k string) string {
	if len(k) > 8 {
		return k[:8]
	}
	return k
}

// shown is a scene's (0 = A .. 7 = H) value as the plan shows it: a switch's position name,
// else the number and unit.
func (p Param) shown(scene int) string {
	if p.names[scene] != "" {
		return p.names[scene]
	}
	if p.screen[scene] != "" {
		return displayString(p.Display[scene], p.Units) + " (" + p.screen[scene] + ")"
	}
	return displayString(p.Display[scene], p.Units)
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
