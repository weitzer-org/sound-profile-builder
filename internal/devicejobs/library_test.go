package devicejobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// library.json is shared with the agent's tests (deploy/qc-agent/tests/test_library.py).
type libraryFixture struct {
	Snapshots []struct {
		Name          string          `json:"name"`
		ExpectPayload json.RawMessage `json:"expect_payload"`
	} `json:"snapshots"`
	Invalid []struct {
		Name    string          `json:"name"`
		Payload json.RawMessage `json:"payload"`
		Error   string          `json:"error"`
	} `json:"invalid"`
}

func loadLibraryFixture(t *testing.T) libraryFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "devicejobs", "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f libraryFixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestParseLibraryPayload_AgentOutputParses(t *testing.T) {
	f := loadLibraryFixture(t)
	if len(f.Snapshots) == 0 {
		t.Fatal("no snapshots in the fixture")
	}
	for _, s := range f.Snapshots {
		p, err := ParseLibraryPayload(s.ExpectPayload)
		if err != nil {
			t.Errorf("%s: %v", s.Name, err)
			continue
		}
		if len(p.Captures) == 0 {
			t.Errorf("%s: no captures parsed", s.Name)
		}
	}
}

func TestParseLibraryPayload_Invalid(t *testing.T) {
	f := loadLibraryFixture(t)
	if len(f.Invalid) < 15 {
		t.Fatalf("only %d invalid cases", len(f.Invalid))
	}
	for _, c := range f.Invalid {
		_, err := ParseLibraryPayload(c.Payload)
		if err == nil {
			t.Errorf("%s: accepted", c.Name)
			continue
		}
		if !strings.Contains(err.Error(), c.Error) {
			t.Errorf("%s: error %q does not contain %q", c.Name, err, c.Error)
		}
	}
}

func TestParseLibraryPayload_Bounds(t *testing.T) {
	cap1 := `{"name":"A","key":"` + strings.Repeat("0", 64) + `","version":null,"kind":"user","folder":"F","cloud":false}`
	if _, err := ParseLibraryPayload(json.RawMessage(`{"captures":[` + cap1 + `],"skipped":0}`)); err != nil {
		t.Fatalf("a null version is valid: %v", err)
	}
	many := strings.TrimSuffix(strings.Repeat(cap1+",", MaxLibraryCaptures+1), ",")
	if _, err := ParseLibraryPayload(json.RawMessage(`{"captures":[` + many + `],"skipped":0}`)); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Errorf("over MaxLibraryCaptures accepted: %v", err)
	}
	if _, err := ParseLibraryPayload(json.RawMessage(`{"captures":[],"skipped":0} {}`)); err == nil {
		t.Error("trailing data accepted")
	}
	if _, err := ParseLibraryPayload(nil); err == nil {
		t.Error("empty payload accepted")
	}
	huge := json.RawMessage(`{"captures":[],"skipped":0,"x":"` + strings.Repeat("a", MaxLibraryBytes) + `"}`)
	if _, err := ParseLibraryPayload(huge); err == nil || !strings.Contains(err.Error(), "larger") {
		t.Errorf("oversized payload: %v", err)
	}
	// the error never echoes payload text
	_, err := ParseLibraryPayload(json.RawMessage(`{"captures":[{"name":"SECRET\u0007","key":"` + strings.Repeat("0", 64) + `","version":1,"kind":"user","folder":"F","cloud":false}],"skipped":0}`))
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Errorf("error echoes device text: %v", err)
	}
}

func TestSyncKindIsAKnownJobKind(t *testing.T) {
	good := `{"v":1,"type":"qc2-device-job","job_id":"6f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e01","bucket":"qc2-device-jobs-test","kind":"sync_device_library","created_at":"2026-11-05T12:00:00Z","expires_at":"2026-11-05T12:15:00Z","args":{}}`
	if _, err := ParseJob([]byte(good)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(good, `"args":{}`, `"args":{"x":1}`, 1),
		strings.Replace(good, `"args":{}`, `"args":null`, 1),
		strings.Replace(good, "sync_device_library", "Sync_Device_Library", 1),
	} {
		if _, err := ParseJob([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
