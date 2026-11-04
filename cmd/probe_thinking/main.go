package main

// probe_thinking is the Gemini capability probe (Phase 0.0 of
// docs/eval/gemini-model-upgrade-test-plan.md; it began life as the thinking-budget probe and
// kept its name so existing references still resolve).
//
// Before a model is scored in any eval, establish what it can actually do on THIS project's
// request shapes. Checks (per model):
//
//	resolve     0.3/0.4  one plain call; reports the served ModelVersion (what an id/alias really is)
//	schematools (a)+(b)+(c)  ResponseSchema + GoogleSearch in ONE call -- agents 1/2/3 need exactly this.
//	            Passing proves (a) schema and (b) tools (tool engagement is checked from
//	            GroundingMetadata, not assumed). If it fails, or the model chose not to search,
//	            -followup runs the schema-only / tools-only calls to localise the failure.
//	tools, forced  extra checks: search alone; schema+search with a prompt that forces a live lookup
//	thinking    (d)  ThinkingBudget 512 / 2048 / 0 (the "default" cell is schematools' own call,
//	            whose ThoughtsTokenCount is reported).
//	maxtokens   (e)  MaxOutputTokens=64 against a prompt that asks for far more; expect MAX_TOKENS.
//
// Safeguards, all deliberate:
//   - Raw genai client, never Orchestrator: there is no fallback chain to substitute a model.
//   - Every cell records resp.ModelVersion and is scored against the expected served model
//     (ServedModelMatches). A served-model mismatch is its own FAIL column, never a pass.
//     Aliases ("...-latest") must be pinned with -pin alias=resolved-id.
//   - -dry-run defaults true and prints the exact call list and count. Nothing is called until
//     you pass -dry-run=false. No retries: a transient 429/503 is reported, not re-spent.
//   - The API key is read from GEMINI_API_KEY and never printed.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"google.golang.org/genai"
)

var defaultModels = []string{
	"gemini-4-argon", "gemini-3.8-flash", "gemini-pro-latest", "gemini-3.5-flash-lite", // candidates
	"gemini-3.1-pro-preview", "gemini-3.5-flash", // baselines
}

// Cell is one planned call.
type Cell struct {
	Model    string
	Check    string // resolve | schematools | schema | tools | thinking | maxtokens
	Budget   *int32 // thinking only
	Followup bool   // only run if an earlier cell did not prove its capability
}

func (c Cell) Label() string {
	if c.Budget != nil {
		return fmt.Sprintf("%s/budget=%d", c.Check, *c.Budget)
	}
	return c.Check
}

// BuildGrid returns the planned cells for the selected models and checks. Follow-ups are
// included (flagged) so dry-run can state the maximum count honestly.
func BuildGrid(models, checks []string, followup bool) []Cell {
	want := map[string]bool{}
	for _, c := range checks {
		want[strings.TrimSpace(c)] = true
	}
	var grid []Cell
	for _, m := range models {
		if want["resolve"] {
			grid = append(grid, Cell{Model: m, Check: "resolve"})
		}
		if want["schematools"] {
			grid = append(grid, Cell{Model: m, Check: "schematools"})
			if followup {
				grid = append(grid, Cell{Model: m, Check: "schema", Followup: true}, Cell{Model: m, Check: "tools", Followup: true})
			}
		}
		if want["tools"] {
			grid = append(grid, Cell{Model: m, Check: "tools"})
		}
		if want["forced"] {
			grid = append(grid, Cell{Model: m, Check: "forced"})
		}
		if want["thinking"] {
			for _, b := range []int32{512, 2048, 0} {
				b := b
				grid = append(grid, Cell{Model: m, Check: "thinking", Budget: &b})
			}
		}
		if want["maxtokens"] {
			grid = append(grid, Cell{Model: m, Check: "maxtokens"})
		}
	}
	// Follow-ups go last: they are conditional and must never starve the main grid under -max-calls.
	var main, follow []Cell
	for _, c := range grid {
		if c.Followup {
			follow = append(follow, c)
		} else {
			main = append(main, c)
		}
	}
	return append(main, follow...)
}

// CountCalls returns (guaranteed, maximum) call counts for a grid.
func CountCalls(grid []Cell) (guaranteed, max int) {
	for _, c := range grid {
		max++
		if !c.Followup {
			guaranteed++
		}
	}
	return
}

// Result is one cell's outcome. Pass is only ever true when the call succeeded, the served
// model matched expectations AND the check-specific criterion held.
type Result struct {
	Model          string  `json:"model"`
	Check          string  `json:"check"`
	Budget         *int32  `json:"budget,omitempty"`
	Pass           bool    `json:"pass"`
	Verdict        string  `json:"verdict"`
	ServedModel    string  `json:"served_model"`
	ExpectedServed string  `json:"expected_served"`
	ServedOK       bool    `json:"served_ok"`
	FinishReason   string  `json:"finish_reason"`
	InputTokens    int32   `json:"input_tokens"`
	OutputTokens   int32   `json:"output_tokens"`
	ThinkingTokens int32   `json:"thinking_tokens"`
	LatencySec     float64 `json:"latency_sec"`
	ToolEngaged    bool    `json:"tool_engaged,omitempty"`
	SchemaValid    bool    `json:"schema_valid,omitempty"`
	Error          string  `json:"error,omitempty"` // raw, untruncated
	TextSnippet    string  `json:"text_snippet,omitempty"`
}

// Evaluate scores a response against the cell's criterion. Pure function (no I/O) so the
// scoring rules are unit-tested without calling Gemini.
func Evaluate(cell Cell, expectedServed string, resp *genai.GenerateContentResponse, callErr error) Result {
	r := Result{Model: cell.Model, Check: cell.Check, Budget: cell.Budget, ExpectedServed: expectedServed}
	if callErr != nil {
		r.Error = callErr.Error()
		r.Verdict = "FAIL: API error"
		return r
	}
	if resp == nil {
		r.Verdict = "FAIL: nil response"
		return r
	}
	r.ServedModel = resp.ModelVersion
	r.ServedOK = agents.ServedModelMatches(expectedServed, resp.ModelVersion)
	if len(resp.Candidates) > 0 && resp.Candidates[0] != nil {
		r.FinishReason = string(resp.Candidates[0].FinishReason)
		if gm := resp.Candidates[0].GroundingMetadata; gm != nil {
			r.ToolEngaged = len(gm.WebSearchQueries) > 0 || len(gm.GroundingChunks) > 0
		}
	}
	if u := resp.UsageMetadata; u != nil {
		r.InputTokens, r.OutputTokens, r.ThinkingTokens = u.PromptTokenCount, u.CandidatesTokenCount, u.ThoughtsTokenCount
	}
	text := resp.Text()
	r.TextSnippet = snippet(text, 160)

	if !r.ServedOK {
		r.Verdict = fmt.Sprintf("FAIL: served model %q != expected %q (NOT scored)", resp.ModelVersion, expectedServed)
		return r
	}

	switch cell.Check {
	case "resolve":
		r.Pass = true
		r.Verdict = "OK (served " + resp.ModelVersion + ")"
	case "schema", "schematools", "forced":
		var v struct {
			Answer string `json:"answer"`
			Source string `json:"source"`
		}
		r.SchemaValid = json.Unmarshal([]byte(text), &v) == nil && v.Answer != "" && v.Source != ""
		switch {
		case !r.SchemaValid:
			r.Verdict = "FAIL: response did not parse into the requested schema"
		case (cell.Check == "schematools" || cell.Check == "forced") && !r.ToolEngaged:
			// Schema honored, but search not shown to have run: (b) is unproven here.
			r.Verdict = "PARTIAL: schema ok, tool accepted but no grounding metadata (search not shown to run)"
		default:
			r.Pass = true
			r.Verdict = "OK"
		}
	case "tools":
		if !r.ToolEngaged {
			r.Verdict = "FAIL: call succeeded but no grounding metadata (search not shown to run)"
		} else {
			r.Pass, r.Verdict = true, "OK"
		}
	case "thinking":
		b := *cell.Budget
		switch {
		case b == 0 && r.ThinkingTokens != 0:
			r.Verdict = fmt.Sprintf("FAIL: budget=0 accepted but model still used %d thinking tokens", r.ThinkingTokens)
		case b > 0 && r.ThinkingTokens > b:
			r.Verdict = fmt.Sprintf("FAIL: used %d thinking tokens > budget %d", r.ThinkingTokens, b)
		default:
			r.Pass, r.Verdict = true, fmt.Sprintf("OK (thinking tokens %d)", r.ThinkingTokens)
		}
	case "maxtokens":
		switch {
		case r.FinishReason != string(genai.FinishReasonMaxTokens):
			r.Verdict = fmt.Sprintf("FAIL: finish_reason=%s, expected MAX_TOKENS (cap not honored / prompt too short)", r.FinishReason)
		case r.OutputTokens > maxTokensCap:
			r.Verdict = fmt.Sprintf("FAIL: MAX_TOKENS but %d output tokens > cap %d", r.OutputTokens, maxTokensCap)
		default:
			r.Pass, r.Verdict = true, fmt.Sprintf("OK (out=%d thinking=%d)", r.OutputTokens, r.ThinkingTokens)
		}
	}
	return r
}

const maxTokensCap = 64

func requestFor(cell Cell) (string, *genai.GenerateContentConfig) {
	cfg := &genai.GenerateContentConfig{}
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"answer": {Type: genai.TypeString},
			"source": {Type: genai.TypeString},
		},
		Required: []string{"answer", "source"},
	}
	search := []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}}
	const searchPrompt = "Use Google Search to find the current stable version of the Go programming language. Reply as JSON with 'answer' (the version) and 'source' (the site you used)."
	switch cell.Check {
	case "resolve":
		return "Reply with the single word: ok", cfg
	case "schematools":
		cfg.ResponseMIMEType, cfg.ResponseSchema, cfg.Tools = "application/json", schema, search
		return searchPrompt, cfg
	case "forced":
		// Same shape as schematools, but the prompt demands a live lookup of something a model cannot know.
		cfg.ResponseMIMEType, cfg.ResponseSchema, cfg.Tools = "application/json", schema, search
		return "You MUST use Google Search for this; do not answer from memory. Find the title of the most recent post on the official Go blog (go.dev/blog) and its publication date. Reply as JSON with 'answer' (title and date) and 'source' (the URL you used).", cfg
	case "schema":
		cfg.ResponseMIMEType, cfg.ResponseSchema = "application/json", schema
		return "What is the capital of France? Reply as JSON with 'answer' and 'source' (any reference you would cite).", cfg
	case "tools":
		cfg.Tools = search
		return "Use Google Search to find the current stable version of the Go programming language. State the version.", cfg
	case "thinking":
		cfg.ThinkingConfig = &genai.ThinkingConfig{ThinkingBudget: cell.Budget}
		return "How many prime numbers are below 100? Reply with just the number.", cfg
	case "maxtokens":
		cfg.MaxOutputTokens = maxTokensCap
		return "Write a detailed 600-word essay on how vacuum tube amplifiers compress and saturate.", cfg
	}
	return "", cfg
}

func main() {
	modelsFlag := flag.String("models", strings.Join(defaultModels, ","), "comma-separated model ids to probe")
	checksFlag := flag.String("checks", "schematools,thinking,maxtokens", "comma-separated: resolve,schematools,thinking,maxtokens")
	followup := flag.Bool("followup", true, "if schematools does not prove schema and tools, also run the schema-only / tools-only calls")
	pin := flag.String("pin", "", "alias=resolved-id[,alias=resolved-id] -- expected served model for aliases such as gemini-pro-latest")
	maxCalls := flag.Int("max-calls", 30, "HARD cap on live calls in this invocation; the probe stops (and says so) when reached")
	dryRun := flag.Bool("dry-run", true, "print the exact call list and count, call nothing; pass -dry-run=false to spend API calls")
	outPath := flag.String("out", "", "write full JSON results here (default eval_results/capability_probe/<timestamp>.json)")
	flag.Parse()

	models := splitList(*modelsFlag)
	grid := BuildGrid(models, splitList(*checksFlag), *followup)
	pins := map[string]string{}
	for _, kv := range splitList(*pin) {
		if a, b, ok := strings.Cut(kv, "="); ok {
			pins[a] = b
		}
	}
	for _, m := range models {
		if strings.HasSuffix(m, "-latest") && pins[m] == "" && !onlyResolve(*checksFlag) {
			log.Fatalf("%s is an alias; resolve it first (-checks resolve) and pass -pin %s=<resolved id> so its served model can be verified", m, m)
		}
	}

	guaranteed, max := CountCalls(grid)
	fmt.Printf("Probe plan: %d models, %d guaranteed calls, up to %d with follow-ups\n", len(models), guaranteed, max)
	for i, c := range grid {
		tag := ""
		if c.Followup {
			tag = "  (only if schematools did not prove it)"
		}
		fmt.Printf("  %2d. %-24s %s%s\n", i+1, c.Model, c.Label(), tag)
	}
	if *dryRun {
		fmt.Println("\nDRY RUN -- no Gemini calls made. Pass -dry-run=false to execute.")
		return
	}

	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY must be set")
	}
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI})
	if err != nil {
		log.Fatalf("genai client: %v", err)
	}

	var results []Result
	proven := map[string]map[string]bool{} // model -> capability proven by schematools
	calls := 0
	for _, cell := range grid {
		if cell.Followup {
			p := proven[cell.Model]
			if p != nil && p[cell.Check] {
				continue
			}
		}
		expected := cell.Model
		if p, ok := pins[cell.Model]; ok {
			expected = p
		}
		if calls >= *maxCalls {
			log.Printf("-max-calls=%d reached: NOT running %s %s (and anything after it)", *maxCalls, cell.Model, cell.Label())
			break
		}
		prompt, cfg := requestFor(cell)
		cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		start := time.Now()
		resp, callErr := client.Models.GenerateContent(cctx, cell.Model, []*genai.Content{genai.NewContentFromText(prompt, genai.RoleUser)}, cfg)
		lat := time.Since(start).Seconds()
		cancel()
		calls++
		r := Evaluate(cell, expected, resp, callErr)
		r.LatencySec = lat
		results = append(results, r)
		if cell.Check == "schematools" {
			proven[cell.Model] = map[string]bool{"schema": r.SchemaValid && r.ServedOK, "tools": r.ToolEngaged && r.ServedOK && r.SchemaValid}
		}
		log.Printf("%-24s %-18s %s", cell.Model, cell.Label(), r.Verdict)
	}

	printMatrix(results, calls)
	path := *outPath
	if path == "" {
		path = filepath.Join("eval_results", "capability_probe", time.Now().UTC().Format("20060102T150405Z")+".json")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
		if b, err := json.MarshalIndent(results, "", "  "); err == nil {
			_ = os.WriteFile(path, b, 0o644)
			fmt.Println("Full results (raw errors included):", path)
		}
	}
}

func printMatrix(results []Result, calls int) {
	fmt.Printf("\n=== CAPABILITY PROBE (%d live calls) ===\n", calls)
	fmt.Printf("%-24s %-20s %-5s %-26s %-12s %7s %s\n", "Model", "Check", "Pass", "ServedModel", "Finish", "Think", "Verdict / raw error")
	for _, r := range results {
		label := r.Check
		if r.Budget != nil {
			label = fmt.Sprintf("%s/%d", r.Check, *r.Budget)
		}
		detail := r.Verdict
		if r.Error != "" {
			detail += " :: " + truncate(r.Error, 220)
		}
		fmt.Printf("%-24s %-20s %-5v %-26s %-12s %7d %s\n", r.Model, label, r.Pass, truncate(r.ServedModel, 26), r.FinishReason, r.ThinkingTokens, detail)
	}
}

func onlyResolve(checks string) bool { return strings.TrimSpace(checks) == "resolve" }

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func snippet(s string, n int) string { return truncate(s, n) }

// truncate flattens s to one line and shortens it to at most n runes including the ellipsis
// (runes, not bytes: error text can contain multi-byte UTF-8).
func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	return string(runes[:n-1]) + "…"
}
