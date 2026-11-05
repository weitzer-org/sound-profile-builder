package evalcheck_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/evalcheck"
	"github.com/weitzer-org/sound-builder/internal/evaltestutil"
)

const (
	pro   = "gemini-3.1-pro-preview"
	flash = "gemini-3.5-flash"
)

var pins = map[string]string{flash: "gemini-3.6-flash"}

func twoScenarioBatch(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	r := evaltestutil.Routing(pro, flash, flash)
	ms := []evalcheck.ManifestScenario{{ID: "S0", AgentModels: r, Pins: pins}}
	run := func(rep int) evaltestutil.Run {
		return evaltestutil.Run{Scenario: "S0", Query: "07_Edge", Rep: rep, Success: true, IntegrityOK: true,
			Calls: evaltestutil.Calls(r, pins, 1, 1, 0, 1), Output: evaltestutil.CleanOutput()}
	}
	evaltestutil.WriteBatch(t, dir, evaltestutil.Manifest("B1", false, 2, []string{"07_Edge"}, ms), []evaltestutil.Run{run(1), run(2)})
	return dir
}

func TestLoadBatch_CleanBatchIsScoreable(t *testing.T) {
	b, err := evalcheck.LoadBatch(twoScenarioBatch(t))
	if err != nil || len(b.Problems) != 0 || len(b.Records) != 2 {
		t.Fatalf("%v %+v", err, b.Problems)
	}
	for _, r := range b.Records {
		if ok, why := b.Scoreable(r); !ok {
			t.Errorf("clean run not scoreable: %s", why)
		}
	}
	if b.Manifest.Planned("S0") != 2 {
		t.Errorf("planned = %d", b.Manifest.Planned("S0"))
	}
}

func TestLoadBatch_MixedBatchIDsAreAProblem(t *testing.T) {
	dir := twoScenarioBatch(t)
	p := filepath.Join(dir, "runs.jsonl")
	raw, _ := os.ReadFile(p)
	// a record from some other invocation pasted into this directory
	other := strings.Replace(strings.Split(string(raw), "\n")[0], `"batch_id":"B1"`, `"batch_id":"OTHER"`, 1)
	other = strings.Replace(other, `"rep":1`, `"rep":3`, 1)
	if err := os.WriteFile(p, append(raw, []byte(other+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := evalcheck.LoadBatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(b.Problems, "|"), "different batches") {
		t.Errorf("mixed batch not detected: %v", b.Problems)
	}
}

func TestLoadBatch_DuplicateRunsAndUnsafeNames(t *testing.T) {
	dir := twoScenarioBatch(t)
	p := filepath.Join(dir, "runs.jsonl")
	raw, _ := os.ReadFile(p)
	first := strings.Split(string(raw), "\n")[0]
	evil := strings.Replace(first, `"scenario":"S0"`, `"scenario":"../../etc"`, 1)
	if err := os.WriteFile(p, append(raw, []byte(first+"\n"+evil+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _ := evalcheck.LoadBatch(dir)
	joined := strings.Join(b.Problems, "|")
	if !strings.Contains(joined, "duplicate run") || !strings.Contains(joined, "unsafe") {
		t.Errorf("problems = %v", b.Problems)
	}
}

func TestScoreable_RederivesIntegrityFromCalls(t *testing.T) {
	dir := t.TempDir()
	r := evaltestutil.Routing(pro, flash, flash)
	ms := []evalcheck.ManifestScenario{{ID: "S0", AgentModels: r, Pins: pins}}
	fb := evaltestutil.Calls(r, pins, 1, 1, 0, 1)
	fb[11].AttemptModel, fb[11].ServedModel = "gemini-2.5-pro", "gemini-2.5-pro" // Architect fell back
	noCalls := evaltestutil.Run{Scenario: "S0", Query: "b", Rep: 1, Success: true, IntegrityOK: true, Output: evaltestutil.CleanOutput()}
	evaltestutil.WriteBatch(t, dir, evaltestutil.Manifest("B1", false, 1, []string{"a", "b"}, ms), []evaltestutil.Run{
		{Scenario: "S0", Query: "a", Rep: 1, Success: true, IntegrityOK: true, Calls: fb, Output: evaltestutil.CleanOutput()}, noCalls})
	b, _ := evalcheck.LoadBatch(dir)
	for _, rec := range b.Records {
		if ok, _ := b.Scoreable(rec); ok {
			t.Errorf("%s: runs.jsonl claims integrity_ok but the calls file contradicts it / is missing", rec.Key())
		}
	}
	if len(b.CallsMissing) != 1 {
		t.Errorf("CallsMissing = %v", b.CallsMissing)
	}
}

func TestLoadLegacy_JulyLayout(t *testing.T) {
	b, err := evalcheck.LoadLegacy("../../eval_results/full_pipeline")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]int{}
	for _, r := range b.Records {
		by[r.Scenario]++
		if ok, _ := b.Scoreable(r); !ok {
			t.Error("legacy outputs are listed as scoreable-with-caveat")
		}
		if _, err := os.Stat(b.OutputFile(r.Key())); err != nil {
			t.Errorf("legacy output path wrong: %v", err)
		}
	}
	if !b.Legacy || by["baseline"] != 12 || by["flash-tier-candidate"] != 11 || len(b.Records) != 35 {
		t.Errorf("legacy load: %v total=%d", by, len(b.Records))
	}
}
