package evalcheck

import (
	"os"
	"path/filepath"
	"testing"
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
