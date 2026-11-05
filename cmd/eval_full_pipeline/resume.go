package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

func runKey(scenario, query string, rep int) string {
	return fmt.Sprintf("%s|%s|%d", scenario, query, rep)
}

// isUnavailable reports a run that failed because a model was over capacity (HTTP 503
// UNAVAILABLE, "high demand"), which is worth retrying on the same model.
func isUnavailable(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Error 503") || strings.Contains(msg, "UNAVAILABLE")
}

// prepareResume readies an interrupted batch in dir for -resume: it returns the runs that
// already succeeded with integrity (to skip) and the batch id from manifest.json. Failed
// records move from runs.jsonl to failed_runs.jsonl, so runs.jsonl ends with one record per
// run that produced a scoreable output, and the audit trail is kept.
func prepareResume(dir string) (map[string]bool, string, error) {
	var m struct {
		BatchID string `json:"batch_id"`
	}
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, "", err
	}
	if err := json.Unmarshal(b, &m); err != nil || m.BatchID == "" {
		return nil, "", fmt.Errorf("manifest.json has no batch_id: %v", err)
	}
	runsPath := filepath.Join(dir, "runs.jsonl")
	recs, err := evalcheck.ReadRunRecords(runsPath)
	if err != nil {
		return nil, "", err
	}
	done := map[string]bool{}
	var keep, failed []string
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			return nil, "", err
		}
		if r.Success && r.IntegrityOK && !done[runKey(r.Scenario, r.Query, r.Rep)] {
			done[runKey(r.Scenario, r.Query, r.Rep)] = true
			keep = append(keep, string(line))
		} else {
			failed = append(failed, string(line))
		}
	}
	if len(failed) > 0 {
		f, err := os.OpenFile(filepath.Join(dir, "failed_runs.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, "", err
		}
		defer f.Close()
		if _, err := f.WriteString(strings.Join(failed, "\n") + "\n"); err != nil {
			return nil, "", err
		}
	}
	content := strings.Join(keep, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(runsPath, []byte(content), 0o644); err != nil {
		return nil, "", err
	}
	return done, m.BatchID, nil
}
