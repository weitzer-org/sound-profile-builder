package evalcheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

func TestParseBenchmarkReport_MalformedHeaderDoesNotPanic(t *testing.T) {
	p := filepath.Join(t.TempDir(), "r.md")
	if err := os.WriteFile(p, []byte("| Scenario | Metric\n| Scenario |\n| x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBenchmarkReport(p); err != nil {
		t.Fatal(err)
	}
}

// Regression for a PR #114 review question: AssessCalls validates the calls it is given against
// their routed models; routing entries with no call (e.g. Architect/Critic during a freeze) are
// not failures. Missing-agent coverage is the caller's job (eval_architect's missingUpstreamCalls).
func TestAssessCalls_RoutingSupersetIsOK(t *testing.T) {
	routing := map[string]string{"1_tone_historian": "m", "12_architect": "m", "13_critic": "m"}
	calls := []agents.CallRecord{{Agent: "Tone Historian", AttemptModel: "m", ServedModel: "m", Success: true}}
	if ok, issues, _ := AssessCalls(calls, routing, nil); !ok {
		t.Fatalf("routing superset flagged: %v", issues)
	}
	calls = append(calls, agents.CallRecord{Agent: "Acoustician", AttemptModel: "m", ServedModel: "m", Success: true})
	if ok, _, _ := AssessCalls(calls, routing, nil); ok {
		t.Fatal("a call for an unrouted agent must fail")
	}
}
