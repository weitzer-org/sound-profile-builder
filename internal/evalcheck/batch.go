package evalcheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// ManifestFile is written by cmd/eval_full_pipeline at the start of a live run. It is the batch's
// identity: plan §2 principle 5 says a scenario may only be compared with an S0 run from the SAME
// batch, so the Phase 2 tools need something that says which batch a results directory is.
const ManifestFile = "manifest.json"

// ManifestScenario freezes one scenario's routing at run time, so later tools do not depend on
// the (changing) scenario table in cmd/eval_full_pipeline.
type ManifestScenario struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	AgentModels map[string]string `json:"agent_models"`
	Pins        map[string]string `json:"pins,omitempty"`
	Skip        string            `json:"skip,omitempty"`
}

// Manifest describes one eval_full_pipeline invocation.
type Manifest struct {
	BatchID   string             `json:"batch_id"`
	CreatedAt string             `json:"created_at"`
	Smoke     bool               `json:"smoke"`
	Reps      int                `json:"reps"`
	Queries   []string           `json:"queries"`
	Scenarios []ManifestScenario `json:"scenarios"`
	Git       string             `json:"git,omitempty"`
}

// Scenario returns the manifest entry for id.
func (m *Manifest) Scenario(id string) (ManifestScenario, bool) {
	for _, s := range m.Scenarios {
		if s.ID == id {
			return s, true
		}
	}
	return ManifestScenario{}, false
}

// Planned is the number of runs the batch intended for scenario id (0 for skipped/unknown ones).
func (m *Manifest) Planned(id string) int {
	s, ok := m.Scenario(id)
	if !ok || s.Skip != "" {
		return 0
	}
	return len(m.Queries) * m.Reps
}

// WriteManifest writes dir/manifest.json. It refuses to overwrite one.
func WriteManifest(dir string, m Manifest) error {
	p := filepath.Join(dir, ManifestFile)
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("%s already exists", p)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

// SafeComponent reports whether s can be used as one path element: record fields come from a
// file that is data, and must not be able to point outside the results directory.
func SafeComponent(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, `/\`+"\x00")
}

// RunKey identifies one run inside a batch.
type RunKey struct {
	Scenario string
	Query    string
	Rep      int
}

func (k RunKey) String() string { return fmt.Sprintf("%s/rep%d/%s", k.Scenario, k.Rep, k.Query) }

// OutputPath is where eval_full_pipeline put the run's output, derived from the LAYOUT rather
// than from RunRecord.OutputFile (which is relative to the cwd of the original run and breaks
// when a results directory is moved).
func OutputPath(root string, k RunKey) string {
	return filepath.Join(root, k.Scenario, fmt.Sprintf("rep%d", k.Rep), k.Query+".html")
}

// CallsPath is the run's per-attempt record file.
func CallsPath(root string, k RunKey) string {
	return filepath.Join(root, k.Scenario, fmt.Sprintf("rep%d", k.Rep), k.Query+".calls.json")
}

// Batch is one results directory loaded for the Phase 2 tools.
type Batch struct {
	Root string
	// Legacy is true for a pre-manifest directory (the July 2026 eval_results/full_pipeline
	// layout: <scenario>/<query>.html, no runs.jsonl, no calls files). Served-model integrity,
	// latency, tokens and batch identity are UNVERIFIABLE there; tools must say so.
	Legacy   bool
	Manifest *Manifest // nil for legacy (pre-manifest) directories
	Records  []RunRecord
	Calls    map[RunKey][]agents.CallRecord
	// CallsMissing lists runs whose calls file was absent or unreadable.
	CallsMissing []RunKey
	// Problems are structural faults (mixed batches, duplicate runs, unsafe names...). A batch
	// with problems must not be compared; the tools refuse.
	Problems []string
}

// OutputFile is where the run's output lives in this batch (legacy layouts have no rep dir).
func (b *Batch) OutputFile(k RunKey) string {
	if b.Legacy {
		return filepath.Join(b.Root, k.Scenario, k.Query+".html")
	}
	return OutputPath(b.Root, k)
}

// LoadLegacy loads a pre-manifest results directory by listing <scenario>/<query>.html. Every
// file is treated as a successful rep-1 run whose integrity is UNKNOWN. Whether a missing query
// failed or was never run cannot be told from the files alone.
func LoadLegacy(root string) (*Batch, error) {
	b := &Batch{Root: root, Legacy: true, Calls: map[RunKey][]agents.CallRecord{}}
	scenDirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, sd := range scenDirs {
		if !sd.IsDir() || !SafeComponent(sd.Name()) {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, sd.Name()))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			name := f.Name()
			if f.IsDir() || !strings.HasSuffix(name, ".html") {
				continue
			}
			q := strings.TrimSuffix(name, ".html")
			if !SafeComponent(q) {
				continue
			}
			b.Records = append(b.Records, RunRecord{Scenario: sd.Name(), Query: q, Rep: 1, Success: true, IntegrityOK: true})
		}
	}
	sort.Slice(b.Records, func(i, j int) bool { return b.Records[i].Key().String() < b.Records[j].Key().String() })
	if len(b.Records) == 0 {
		return nil, fmt.Errorf("%s: no <scenario>/<query>.html files found", root)
	}
	return b, nil
}

// Key returns the RunKey of a record.
func (r RunRecord) Key() RunKey { return RunKey{r.Scenario, r.Query, r.Rep} }

// LoadBatch reads root/manifest.json, root/runs.jsonl and every run's calls file. It does not
// score anything. Structural faults are collected in Batch.Problems instead of failing, so a
// caller can print all of them.
func LoadBatch(root string) (*Batch, error) {
	b := &Batch{Root: root, Calls: map[RunKey][]agents.CallRecord{}}
	if mb, err := os.ReadFile(filepath.Join(root, ManifestFile)); err == nil {
		var m Manifest
		if err := json.Unmarshal(mb, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", ManifestFile, err)
		}
		b.Manifest = &m
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	recs, err := ReadRunRecords(filepath.Join(root, "runs.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("runs.jsonl: %w", err)
	}
	b.Records = recs

	seen := map[RunKey]bool{}
	batchIDs := map[string]int{}
	for _, r := range recs {
		if !SafeComponent(r.Scenario) || !SafeComponent(r.Query) || r.Rep < 1 {
			b.Problems = append(b.Problems, fmt.Sprintf("unsafe or invalid run identity in runs.jsonl: scenario=%q query=%q rep=%d", r.Scenario, r.Query, r.Rep))
			continue
		}
		if seen[r.Key()] {
			b.Problems = append(b.Problems, "duplicate run in runs.jsonl: "+r.Key().String())
		}
		seen[r.Key()] = true
		batchIDs[r.BatchID]++
		if b.Manifest != nil {
			if _, ok := b.Manifest.Scenario(r.Scenario); !ok {
				b.Problems = append(b.Problems, fmt.Sprintf("run %s names a scenario that is not in the manifest", r.Key()))
			}
		}
		raw, err := os.ReadFile(CallsPath(root, r.Key()))
		if err != nil {
			b.CallsMissing = append(b.CallsMissing, r.Key())
			continue
		}
		var calls []agents.CallRecord
		if err := json.Unmarshal(raw, &calls); err != nil {
			b.CallsMissing = append(b.CallsMissing, r.Key())
			b.Problems = append(b.Problems, fmt.Sprintf("%s: unreadable calls file: %v", r.Key(), err))
			continue
		}
		b.Calls[r.Key()] = calls
	}
	// Batch identity: every record must carry the manifest's batch id.
	switch {
	case b.Manifest != nil:
		for id, n := range batchIDs {
			if id != b.Manifest.BatchID {
				b.Problems = append(b.Problems, fmt.Sprintf("%d run record(s) carry batch_id %q but the manifest says %q: results from different batches are mixed in this directory", n, id, b.Manifest.BatchID))
			}
		}
	case len(batchIDs) > 1:
		b.Problems = append(b.Problems, "run records carry more than one batch_id: mixed batches")
	}
	sort.Slice(b.CallsMissing, func(i, j int) bool { return b.CallsMissing[i].String() < b.CallsMissing[j].String() })
	return b, nil
}

// Scoreable reports whether a run may be scored: it produced an output and every attempt was
// served by the expected model (plan principle 3). The second return is the reason when not.
// Integrity is RE-DERIVED from the calls file when the manifest provides the routing, so a
// runs.jsonl that says integrity_ok=true cannot launder a fallback-served call.
func (b *Batch) Scoreable(r RunRecord) (bool, string) {
	if b.Legacy {
		// Output exists; served-model integrity cannot be checked (no calls files). Callers
		// must report that, not silently treat it as verified.
		return true, ""
	}
	if !r.Success {
		return false, "pipeline run failed: " + firstLine(r.Error)
	}
	if !r.IntegrityOK {
		return false, "integrity_ok=false in runs.jsonl: " + strings.Join(r.IntegrityIssues, "; ")
	}
	calls, ok := b.Calls[r.Key()]
	if !ok {
		return false, "calls file missing: served model cannot be verified"
	}
	if b.Manifest != nil {
		if sc, ok := b.Manifest.Scenario(r.Scenario); ok {
			if clean, issues, _ := AssessCalls(calls, sc.AgentModels, sc.Pins); !clean {
				return false, "re-derived from calls file: " + strings.Join(issues, "; ")
			}
		}
	}
	return true, ""
}

// Fallback S0 Critic identity for directories without a manifest (kept in sync with
// cmd/eval_full_pipeline by TestS0CriticDefaultsMatchScenarioTable).
const (
	S0CriticModel  = "gemini-3.5-flash"
	S0CriticServed = "gemini-3.6-flash"
)
