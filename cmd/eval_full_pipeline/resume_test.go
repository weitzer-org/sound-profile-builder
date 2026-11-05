package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareResume(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"batch_id":"20261105T170000Z"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "runs.jsonl"), []byte(strings.Join([]string{
		`{"scenario":"PROD","query":"01","rep":1,"success":true,"integrity_ok":true}`,
		`{"scenario":"PROD","query":"02","rep":1,"success":false,"error":"Error 503"}`,
		`{"scenario":"PROD","query":"03","rep":1,"success":true,"integrity_ok":false}`,
	}, "\n")+"\n"), 0o644)
	done, batch, err := prepareResume(dir)
	if err != nil {
		t.Fatal(err)
	}
	if batch != "20261105T170000Z" || len(done) != 1 || !done[runKey("PROD", "01", 1)] {
		t.Errorf("batch=%q done=%v, want only PROD|01|1", batch, done)
	}
	runs, _ := os.ReadFile(filepath.Join(dir, "runs.jsonl"))
	failed, _ := os.ReadFile(filepath.Join(dir, "failed_runs.jsonl"))
	if strings.Count(string(runs), "\n") != 1 || strings.Count(string(failed), "\n") != 2 {
		t.Errorf("runs.jsonl:\n%s\nfailed_runs.jsonl:\n%s", runs, failed)
	}
}

func TestIsUnavailable(t *testing.T) {
	if !isUnavailable(errors.New("[Tone Historian] All fallback models failed: Error 503, Message: high demand, Status: UNAVAILABLE")) {
		t.Error("a 503 should be retryable")
	}
	if isUnavailable(errors.New("[Architect] response truncated at the MaxOutputTokens limit")) {
		t.Error("a truncation is not a capacity error")
	}
}
