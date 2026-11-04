package evalcheck

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// RunFailure is one failed pipeline run recovered from an eval report.
type RunFailure struct {
	Scenario  string `json:"scenario"`
	Query     string `json:"query"`
	Agent     string `json:"agent,omitempty"`
	Truncated bool   `json:"truncated"` // MaxOutputTokens (MAX_TOKENS) truncation
	Error     string `json:"error"`
}

var (
	reQueryRow = regexp.MustCompile(`^\|\s*\*\*(.+?)\*\*\s*\|\s*Pipeline Latency`)
	reAgent    = regexp.MustCompile(`\[([A-Za-z &]+)\]\s*(?i:response truncated)`)
)

// ParseBenchmarkReport extracts failed runs from a markdown report written by
// cmd/eval_full_pipeline (the July 2026 format: one table, one column per scenario, a
// "Fallback Events / Notes" row per query carrying "Error: ..." for failures).
func ParseBenchmarkReport(path string) ([]RunFailure, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var scenarios []string
	var query string
	var out []RunFailure
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "| Scenario | Metric") {
			cols := strings.Split(line, "|")
			scenarios = nil
			if len(cols) >= 4 { // malformed header rows are skipped, not a panic
				for _, c := range cols[3 : len(cols)-1] {
					scenarios = append(scenarios, strings.TrimSpace(c))
				}
			}
			continue
		}
		if m := reQueryRow.FindStringSubmatch(line); m != nil {
			query = strings.ReplaceAll(strings.TrimSpace(m[1]), " ", "_")
			continue
		}
		if !strings.Contains(line, "Fallback Events / Notes") || len(scenarios) == 0 {
			continue
		}
		cols := strings.Split(line, "|")
		if len(cols) < 4+len(scenarios)-1 {
			continue
		}
		for i, sname := range scenarios {
			cell := strings.TrimSpace(cols[3+i])
			if !strings.HasPrefix(cell, "Error:") {
				continue
			}
			rf := RunFailure{Scenario: sname, Query: query, Error: strings.TrimSpace(strings.TrimPrefix(cell, "Error:"))}
			low := strings.ToLower(rf.Error)
			rf.Truncated = strings.Contains(low, "truncated at the maxoutputtokens")
			if m := reAgent.FindStringSubmatch(rf.Error); m != nil {
				rf.Agent = m[1]
			}
			out = append(out, rf)
		}
	}
	return out, sc.Err()
}

// RunRecord is one line of runs.jsonl as written by cmd/eval_full_pipeline.
type RunRecord struct {
	Scenario         string         `json:"scenario"`
	Query            string         `json:"query"`
	Rep              int            `json:"rep"`
	Success          bool           `json:"success"`
	Error            string         `json:"error,omitempty"`
	OutputFile       string         `json:"output_file,omitempty"`
	LatencySec       float64        `json:"latency_sec"`
	IntegrityOK      bool           `json:"integrity_ok"`                  // every successful call served by its requested model
	IntegrityIssues  []string       `json:"integrity_issues,omitempty"`    // human-readable mismatches
	MaxTokensByAgent map[string]int `json:"max_tokens_by_agent,omitempty"` // MAX_TOKENS finish reasons per agent (incl. retried attempts)
	CallsFile        string         `json:"calls_file,omitempty"`
}

// ReadRunRecords loads runs.jsonl.
func ReadRunRecords(path string) ([]RunRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []RunRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var r RunRecord
		if err := json.Unmarshal([]byte(sc.Text()), &r); err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, sc.Err()
}

// AssessCalls applies plan principle 3 ("never score a model that didn't answer") to one run's
// per-attempt records. expectedByKey maps an agent prompt key (e.g. "12_architect") to the model
// id the scenario routes it to; pins maps an alias to the id it was resolved to (a model whose
// alias is pinned is expected to be SERVED as the pinned id).
//
// A run is clean only if EVERY attempt it made -- including attempts inside RunPipeline paths
// that swallow errors, such as the advisory Preset Critic -- succeeded and was served by the
// expected model. Any MAX_TOKENS finish reason is counted per agent (retried attempts included).
func AssessCalls(calls []agents.CallRecord, expectedByKey, pins map[string]string) (ok bool, issues []string, maxTokens map[string]int) {
	maxTokens = map[string]int{}
	ok = true
	for _, c := range calls {
		if c.FinishReason == "MAX_TOKENS" {
			maxTokens[c.Agent]++
		}
		key, known := agents.AgentRoleKeys[c.Agent]
		want, routed := expectedByKey[key]
		if !known || !routed {
			ok = false
			issues = append(issues, fmt.Sprintf("%s: no expected model for this agent", c.Agent))
			continue
		}
		expectedServed := want
		if p, has := pins[want]; has {
			expectedServed = p
		}
		switch {
		case c.AttemptModel != want:
			ok = false
			issues = append(issues, fmt.Sprintf("%s: attempted %s but scenario routes it to %s (fallback fired)", c.Agent, c.AttemptModel, want))
		case !c.Success:
			ok = false
			issues = append(issues, fmt.Sprintf("%s: attempt on %s failed (finish=%s): %s", c.Agent, c.AttemptModel, c.FinishReason, firstLine(c.Error)))
		case !agents.ServedModelMatches(expectedServed, c.ServedModel):
			ok = false
			issues = append(issues, fmt.Sprintf("%s: asked %s, served %q", c.Agent, want, c.ServedModel))
		}
	}
	return ok, issues, maxTokens
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	if r := []rune(s); len(r) > 160 {
		return string(r[:159]) + "…"
	}
	return s
}
