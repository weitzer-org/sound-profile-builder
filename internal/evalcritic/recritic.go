// Package evalcritic implements the plan §6.2.1 uniform re-critic: ONE fixed Critic model
// (S0's, with its served-id pin) re-reads every scoreable output of a results directory, so
// scenarios whose own Critic differs (S5, A1: Argon) are comparable with the rest.
//
// It is deliberately separate from the pipeline's Critic (agent 13 inside RunPipeline):
//   - the input is the output with the pipeline Critic's notes REMOVED, so a scenario cannot be
//     scored on another model's opinion baked into its own rationale;
//   - results go to recritic.jsonl, never into the output files or calls.json;
//   - a failed, unparseable or wrongly-served re-critic call is recorded as such. It is never
//     counted as "0 defects".
//
// The Critic is advisory, not ground truth (July: it disputed a real capture). A defect count is
// "how many things this one fixed reviewer flagged", comparable across scenarios; it is not a
// count of true errors.
package evalcritic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

// ResultsFile is where the re-critic writes, one JSON line per output.
const ResultsFile = "recritic.jsonl"

// Result is one re-critic outcome.
type Result struct {
	BatchID  string `json:"batch_id,omitempty"` // the results directory's manifest batch; the report refuses foreign lines
	Scenario string `json:"scenario"`
	Query    string `json:"query"`
	Rep      int    `json:"rep"`

	// OK is true only when the call succeeded, was served by the expected model, and its
	// response parsed. Only OK results carry a meaningful Defects count.
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`

	Defects    int                  `json:"defects"`   // findings that point at a real guitar/block of the output
	Unmatched  int                  `json:"unmatched"` // findings whose guitar/block_id exist nowhere (the pipeline silently drops these)
	BySeverity map[string]int       `json:"by_severity,omitempty"`
	Issues     []agents.CriticIssue `json:"issues,omitempty"`

	CriticModel  string `json:"critic_model"`
	ServedModel  string `json:"served_model,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
	InputTokens  int32  `json:"input_tokens,omitempty"`
	OutputTokens int32  `json:"output_tokens,omitempty"`
	LatencyMs    int64  `json:"latency_ms,omitempty"`
	At           string `json:"at"`
}

// Key returns the run this result belongs to.
func (r Result) Key() evalcheck.RunKey {
	return evalcheck.RunKey{Scenario: r.Scenario, Query: r.Query, Rep: r.Rep}
}

// Output is one file the re-critic will review.
type Output struct {
	Key  evalcheck.RunKey
	Path string
}

// Skipped is an output that is not re-criticized, with the reason.
type Skipped struct {
	Key    evalcheck.RunKey
	Reason string
}

// Plan lists the outputs to re-critic: every SCOREABLE run (successful, integrity-clean) that has
// an output file. scenarios, if non-empty, restricts to those IDs. includeUnscoreable also
// re-criticizes runs that failed integrity (their output may come from a fallback model).
func Plan(b *evalcheck.Batch, scenarios map[string]bool, includeUnscoreable bool) (outs []Output, skipped []Skipped) {
	for _, r := range b.Records {
		if len(scenarios) > 0 && !scenarios[r.Scenario] {
			continue
		}
		ok, why := b.Scoreable(r)
		if !ok && !includeUnscoreable {
			skipped = append(skipped, Skipped{r.Key(), why})
			continue
		}
		p := b.OutputFile(r.Key())
		if _, err := os.Stat(p); err != nil {
			skipped = append(skipped, Skipped{r.Key(), "no output file"})
			continue
		}
		outs = append(outs, Output{r.Key(), p})
	}
	sort.Slice(outs, func(i, j int) bool { return outs[i].Key.String() < outs[j].Key.String() })
	return outs, skipped
}

// LoadDone reads an existing recritic.jsonl so a resumed run does not pay for an output twice.
// Only OK results count as done: a failed call is retried on resume.
func LoadDone(path string) (map[evalcheck.RunKey]Result, error) {
	rs, err := ReadResults(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[evalcheck.RunKey]Result{}, nil
		}
		return nil, err
	}
	done := map[evalcheck.RunKey]Result{}
	for _, r := range rs {
		if r.OK {
			done[r.Key()] = r
		}
	}
	return done, nil
}

// ReadResults loads recritic.jsonl. When an output has several lines (a retry after failure)
// the LAST one wins, so a later success supersedes an earlier failure.
func ReadResults(path string) ([]Result, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	last := map[evalcheck.RunKey]Result{}
	var order []evalcheck.RunKey
	for i, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r Result
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+1, err)
		}
		if _, seen := last[r.Key()]; !seen {
			order = append(order, r.Key())
		}
		last[r.Key()] = r
	}
	out := make([]Result, 0, len(order))
	for _, k := range order {
		out = append(out, last[k])
	}
	return out, nil
}

// CriticInput turns a saved output envelope back into what the pipeline's Critic originally
// saw: the Architect's JSON without the rendered table (final_html_payload) and without any
// "⚠️ Critic: ..." note appended to a rationale. It also returns the set of "guitar\x00block_id"
// pairs, used to tell real findings from ones that point at nothing.
func CriticInput(envelopeJSON string) (input string, blockIDs map[string]bool, err error) {
	var env map[string]json.RawMessage
	if err := json.Unmarshal([]byte(agents.StripJSONFences(envelopeJSON)), &env); err != nil {
		return "", nil, fmt.Errorf("envelope: %w", err)
	}
	delete(env, "final_html_payload")
	blockIDs = map[string]bool{}
	if sp, ok := env["structured_payload"]; ok {
		var payload map[string]any
		if err := json.Unmarshal(sp, &payload); err != nil {
			return "", nil, fmt.Errorf("structured_payload: %w", err)
		}
		if guitars, ok := payload["guitars"].(map[string]any); ok {
			for g, v := range guitars {
				blocks, _ := v.([]any)
				for _, bi := range blocks {
					bm, ok := bi.(map[string]any)
					if !ok {
						continue
					}
					if id, ok := bm["id"].(string); ok {
						blockIDs[g+"\x00"+id] = true
					}
					if rat, ok := bm["rationale"].(string); ok {
						bm["rationale"] = evalcheck.StripCriticNotes(rat)
					}
				}
			}
		}
		nb, err := json.Marshal(payload)
		if err != nil {
			return "", nil, err
		}
		env["structured_payload"] = nb
	}
	out, err := json.Marshal(env)
	return string(out), blockIDs, err
}

// Parse reads a Critic response. A response without an "issues" array is a parse failure, NOT
// zero issues: {} and garbage must not be able to score as a perfect output.
func Parse(raw string) ([]agents.CriticIssue, error) {
	clean := agents.StripJSONFences(raw)
	if clean == "" || clean[0] != '{' {
		return nil, fmt.Errorf("critic response is not a JSON object")
	}
	var resp struct {
		Issues *[]agents.CriticIssue `json:"issues"`
	}
	if err := json.Unmarshal([]byte(clean), &resp); err != nil {
		return nil, err
	}
	if resp.Issues == nil {
		return nil, fmt.Errorf(`critic response has no "issues" array`)
	}
	return *resp.Issues, nil
}

// Caller makes ONE Critic call on input and returns the raw response and the attempt records
// (the orchestrator-backed implementation reads them from TokenUsage.Calls).
type Caller func(ctx context.Context, input string) (raw string, calls []agents.CallRecord, err error)

// Config pins the reviewer.
type Config struct {
	Model       string // requested model id, e.g. gemini-3.5-flash
	ExpectServe string // served id it must report, e.g. gemini-3.6-flash (the alias pin)
}

// Review re-critics one output. It never panics on bad input; every failure becomes a Result
// with OK=false.
func Review(ctx context.Context, cfg Config, out Output, call Caller) Result {
	res := Result{Scenario: out.Key.Scenario, Query: out.Key.Query, Rep: out.Key.Rep, CriticModel: cfg.Model, At: time.Now().UTC().Format(time.RFC3339)}
	raw, err := os.ReadFile(out.Path)
	if err != nil {
		res.Error = "read output: " + err.Error()
		return res
	}
	input, ids, err := CriticInput(string(raw))
	if err != nil {
		res.Error = "prepare input: " + err.Error()
		return res
	}
	resp, calls, err := call(ctx, input)
	for _, c := range calls { // the last attempt is the one that answered
		res.ServedModel, res.FinishReason = c.ServedModel, c.FinishReason
		res.InputTokens, res.OutputTokens, res.LatencyMs = c.InputTokens, c.OutputTokens, c.LatencyMs
	}
	if err != nil {
		res.Error = "critic call: " + err.Error()
		return res
	}
	// Uniformity: every attempt must have been the pinned model, served as the pinned id.
	for _, c := range calls {
		switch {
		case c.AttemptModel != cfg.Model:
			res.Error = fmt.Sprintf("attempt on %s, not the pinned %s (fallback fired)", c.AttemptModel, cfg.Model)
			return res
		case !c.Success:
			res.Error = "attempt failed: " + c.Error
			return res
		case !agents.ServedModelMatches(cfg.ExpectServe, c.ServedModel):
			res.Error = fmt.Sprintf("asked %s, served %q (expected %s)", cfg.Model, c.ServedModel, cfg.ExpectServe)
			return res
		}
	}
	if len(calls) == 0 {
		res.Error = "no call record: served model unverified"
		return res
	}
	issues, err := Parse(resp)
	if err != nil {
		res.Error = "parse: " + err.Error()
		return res
	}
	res.OK = true
	res.BySeverity = map[string]int{}
	for _, is := range issues {
		if !ids[is.Guitar+"\x00"+is.BlockID] {
			res.Unmatched++
			continue
		}
		res.Defects++
		res.BySeverity[strings.ToLower(is.Severity)]++
		res.Issues = append(res.Issues, is)
	}
	return res
}

// AppendResult appends one line to path.
func AppendResult(path string, r Result) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// ResultsPath is root/recritic.jsonl.
func ResultsPath(root string) string { return filepath.Join(root, ResultsFile) }

// Resolve picks the Critic model and served-id pin: explicit flags win, then the batch
// manifest's S0, then evalcheck's constants.
func Resolve(m *evalcheck.Manifest, model, served string) (Config, string) {
	cfg := Config{Model: model, ExpectServe: served}
	src := "flags"
	if cfg.Model == "" {
		cfg.Model, src = evalcheck.S0CriticModel, "built-in S0 default (no manifest)"
		if m != nil {
			if s0, ok := m.Scenario("S0"); ok && s0.AgentModels["13_critic"] != "" {
				cfg.Model, src = s0.AgentModels["13_critic"], "manifest S0"
				if p, ok := s0.Pins[cfg.Model]; ok && served == "" {
					cfg.ExpectServe = p
				}
			}
		}
	}
	if cfg.ExpectServe == "" {
		if cfg.Model == evalcheck.S0CriticModel {
			cfg.ExpectServe = evalcheck.S0CriticServed
		} else {
			cfg.ExpectServe = cfg.Model
		}
	}
	return cfg, src
}
