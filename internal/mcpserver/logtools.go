package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weitzer-org/sound-builder/internal/presetsvc"
	"github.com/weitzer-org/sound-builder/internal/runs"
)

const (
	logToolName     = "get_run_log"
	promptsToolName = "get_run_prompts"
	outputToolName  = "get_agent_output"

	partSystem = "system"
	partInput  = "input"
	partBoth   = "both"

	maxEchoedPromptRunes = 1500
	beginData            = "<<<BEGIN GENERATED DATA>>>"
	endData              = "<<<END GENERATED DATA>>>"
)

// ---- reading helpers (shared by the tools and the resources) ----

// keyVersion renders "key" or "key, vN" depending on whether a version is known.
func keyVersion(t *runs.AgentTrace) string {
	if t.PromptVersion == "" {
		return t.PromptKey
	}
	return t.PromptKey + ", " + t.PromptVersion
}

// loadRun validates runID and returns its record (live or stored).
func loadRun(ctx context.Context, svc *presetsvc.Service, runID string) (*runs.RunRecord, error) {
	if err := validRunID(runID); err != nil {
		return nil, err
	}
	rec, err := svc.Run(ctx, runID)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// sortedAgents returns the run's agent traces in a stable order: by prompt key
// (which is numbered in pipeline order), then role.
func sortedAgents(rec *runs.RunRecord) []*runs.AgentTrace {
	out := make([]*runs.AgentTrace, 0, len(rec.Agents))
	for _, t := range rec.Agents {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ka, kb := keyOrder(a.PromptKey), keyOrder(b.PromptKey); ka != kb {
			return ka < kb
		}
		return a.Role < b.Role
	})
	return out
}

// keyOrder sorts "4_coros_librarian" before "12_architect" by its numeric prefix.
func keyOrder(key string) int {
	n := 0
	for _, r := range key {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// findAgent resolves a role ("CorOS Librarian") or a prompt key
// ("4_coros_librarian"), case-insensitively, to the run's trace for it. The
// text is only ever compared with the run's own trace names, never used as a path.
func findAgent(rec *runs.RunRecord, q string) (*runs.AgentTrace, error) {
	q = strings.TrimSpace(q)
	if t, ok := rec.Agents[q]; ok {
		return t, nil
	}
	for _, t := range sortedAgents(rec) {
		if strings.EqualFold(t.Role, q) || (t.PromptKey != "" && strings.EqualFold(t.PromptKey, q)) {
			return t, nil
		}
	}
	var names []string
	for _, t := range sortedAgents(rec) {
		if t.PromptKey != "" {
			names = append(names, fmt.Sprintf("%q (key %s)", t.Role, t.PromptKey))
		}
	}
	avail := "none yet"
	if len(names) > 0 {
		avail = strings.Join(names, ", ")
	}
	return nil, fmt.Errorf("No agent %q in run %s. Pass an agent role or prompt key from get_run_prompts without `agent`. Agents in this run: %s.", q, rec.ID, avail)
}

// readAgentLog reads one of an agent's stored log files.
func readAgentLog(ctx context.Context, svc *presetsvc.Service, rec *runs.RunRecord, t *runs.AgentTrace, suffix string) (string, error) {
	if t.PromptKey == "" {
		return "", userErr("Agent %q has no stored log: it is not part of the logged prompt set.", t.Role)
	}
	data, err := svc.Runs().ReadLog(ctx, rec.ID, t.PromptKey+suffix)
	if err != nil {
		if errors.Is(err, runs.ErrNotFound) {
			return "", userErr("No %s log is stored for %q in run %s: the agent may not have started yet, or its log was not saved.", strings.TrimPrefix(suffix, "."), t.Role, rec.ID)
		}
		return "", err
	}
	return string(data), nil
}

// readPromptText returns the agent's system prompt, its input, or both joined
// under labelled headings, exactly as the pipeline sent them.
func readPromptText(ctx context.Context, svc *presetsvc.Service, rec *runs.RunRecord, t *runs.AgentTrace, part string) (string, error) {
	switch part {
	case "", partSystem:
		return readAgentLog(ctx, svc, rec, t, ".system.md")
	case partInput:
		return readAgentLog(ctx, svc, rec, t, ".input.txt")
	case partBoth:
		sys, err := readAgentLog(ctx, svc, rec, t, ".system.md")
		if err != nil {
			return "", err
		}
		in, err := readAgentLog(ctx, svc, rec, t, ".input.txt")
		if err != nil {
			return "", err
		}
		return "=== SYSTEM PROMPT ===\n" + sys + "\n\n=== USER INPUT ===\n" + in, nil
	}
	return "", userErr("part must be 'system', 'input' or 'both'.")
}

// pagedText frames one slice of run data for the model: where it sits, how to
// get the rest, and the data-not-instructions notice around the verbatim text.
func pagedText(heading string, pg page) string {
	var b strings.Builder
	b.WriteString(runDataNotice + "\n")
	fmt.Fprintf(&b, "%s\nCharacters %d-%d of %d.", heading, pg.Offset, pg.Offset+len([]rune(pg.Text)), pg.Total)
	if pg.NextOffset != nil {
		fmt.Fprintf(&b, " More text follows: call again with offset=%d.", *pg.NextOffset)
	} else {
		b.WriteString(" This is the end.")
	}
	b.WriteString("\n" + beginData + "\n" + pg.Text + "\n" + endData)
	return b.String()
}

// ---- get_run_log ----

type logInput struct {
	RunID    string `json:"run_id" jsonschema:"run ID from generate_preset, refine_preset or get_preset's run_ids"`
	SinceSeq int    `json:"since_seq,omitempty" jsonschema:"only return events with seq greater than this (default 0: all)"`
}

type logOutput struct {
	RunID           string                      `json:"run_id"`
	Kind            string                      `json:"kind"`
	Source          string                      `json:"source"`
	Status          string                      `json:"status"`
	Phase           string                      `json:"phase"`
	Mock            bool                        `json:"mock"`
	PresetID        string                      `json:"preset_id,omitempty"`
	ParentPresetID  string                      `json:"parent_preset_id,omitempty"`
	Error           string                      `json:"error,omitempty"`
	UserPrompt      string                      `json:"user_prompt"`
	EffectivePrompt string                      `json:"effective_prompt"`
	Constraints     map[string]any              `json:"constraints"`
	PromptVersions  map[string]string           `json:"prompt_versions"`
	Events          []runs.RunEvent             `json:"events"`
	Agents          map[string]*runs.AgentTrace `json:"agents"`
	Totals          runs.Usage                  `json:"totals"`
}

// runLogOutput builds the structured log for rec, keeping only events after sinceSeq.
func runLogOutput(rec *runs.RunRecord, sinceSeq int) logOutput {
	out := logOutput{
		RunID: rec.ID, Kind: rec.Kind, Source: rec.Source, Status: rec.Status, Phase: rec.Phase, Mock: rec.Mock,
		PresetID: rec.PresetID, ParentPresetID: rec.ParentPresetID, Error: rec.Error,
		UserPrompt: rec.UserPrompt, EffectivePrompt: rec.EffectivePrompt,
		Constraints: rec.Constraints, PromptVersions: rec.PromptVersions,
		Events: []runs.RunEvent{}, Agents: rec.Agents, Totals: rec.Totals,
	}
	if out.Constraints == nil {
		out.Constraints = map[string]any{}
	}
	if out.PromptVersions == nil {
		out.PromptVersions = map[string]string{}
	}
	if out.Agents == nil {
		out.Agents = map[string]*runs.AgentTrace{}
	}
	for _, e := range rec.Events {
		if e.Seq > sinceSeq {
			out.Events = append(out.Events, e)
		}
	}
	return out
}

// runLogText is the compact timeline form of a run log.
func runLogText(o logOutput, sinceSeq int) string {
	var b strings.Builder
	b.WriteString(runDataNotice + "\n\n")
	mock := ""
	if o.Mock {
		mock = ", mock"
	}
	fmt.Fprintf(&b, "Run `%s`: %s run from %s%s, status %s (%s).\n", o.RunID, o.Kind, o.Source, mock, o.Status, cell(o.Phase))
	if o.PresetID != "" {
		fmt.Fprintf(&b, "Preset: `%s`.\n", o.PresetID)
	}
	if o.ParentPresetID != "" {
		fmt.Fprintf(&b, "Refined preset: `%s`.\n", o.ParentPresetID)
	}
	if o.Error != "" {
		fmt.Fprintf(&b, "Error: %s\n", cell(truncateRunes(o.Error, maxErrorRunes)))
	}
	fmt.Fprintf(&b, "User prompt: %s\n", cell(truncateRunes(o.UserPrompt, maxEchoedPromptRunes)))
	if o.EffectivePrompt != "" && o.EffectivePrompt != o.UserPrompt {
		b.WriteString("(The prompt actually sent differs: it has config-flag prefixes; get_run_prompts or the structured result has it in full.)\n")
	}
	if len(o.Constraints) > 0 {
		keys := make([]string, 0, len(o.Constraints))
		for k := range o.Constraints {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, o.Constraints[k]))
		}
		fmt.Fprintf(&b, "Constraints: %s\n", cell(strings.Join(parts, ", ")))
	}

	if sinceSeq > 0 {
		fmt.Fprintf(&b, "\nEvents after #%d:\n", sinceSeq)
	} else {
		b.WriteString("\nTimeline:\n")
	}
	if len(o.Events) == 0 {
		b.WriteString("(none)\n")
	}
	for _, e := range o.Events {
		who := ""
		if e.Agent != "" {
			who = cell(e.Agent) + ": "
		}
		lat := ""
		if e.LatencyMs > 0 {
			lat = fmt.Sprintf(" [%dms]", e.LatencyMs)
		}
		fmt.Fprintf(&b, "- #%d %s %s %s%s%s\n", e.Seq, e.TS.UTC().Format("15:04:05"), e.Type, who, cell(e.Msg), lat)
	}

	if len(o.Agents) > 0 {
		b.WriteString("\nAgents (use the key with get_run_prompts / get_agent_output):\n")
		ts := make([]*runs.AgentTrace, 0, len(o.Agents))
		for _, t := range o.Agents {
			ts = append(ts, t)
		}
		sort.Slice(ts, func(i, j int) bool {
			if ka, kb := keyOrder(ts[i].PromptKey), keyOrder(ts[j].PromptKey); ka != kb {
				return ka < kb
			}
			return ts[i].Role < ts[j].Role
		})
		for _, t := range ts {
			fmt.Fprintf(&b, "- %s (key %s): %s on %s, %d attempt(s), %dms, tokens in/out/thinking %d/%d/%d, chars system/input/output %d/%d/%d\n",
				cell(t.Role), keyVersion(t), t.Status, cell(t.Model), t.Attempts, t.LatencyMs,
				t.InputTokens, t.OutputTokens, t.ThinkingTokens, t.SystemPromptChars, t.InputChars, t.OutputChars)
		}
	}
	fmt.Fprintf(&b, "\nTotals: %d calls, tokens in/out/thinking %d/%d/%d.", o.Totals.Calls, o.Totals.InputTokens, o.Totals.OutputTokens, o.Totals.ThinkingTokens)
	return b.String()
}

func getRunLog(svc *presetsvc.Service) mcp.ToolHandlerFor[logInput, logOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in logInput) (*mcp.CallToolResult, logOutput, error) {
		if in.SinceSeq < 0 {
			return nil, logOutput{}, errors.New("since_seq must be 0 or greater.")
		}
		rec, err := loadRun(ctx, svc, in.RunID)
		if err != nil {
			return nil, logOutput{}, toolError(logToolName, err)
		}
		out := runLogOutput(rec, in.SinceSeq)
		return textResult(runLogText(out, in.SinceSeq)), out, nil
	}
}

// ---- get_run_prompts ----

type promptsInput struct {
	RunID    string `json:"run_id" jsonschema:"run ID from generate_preset, refine_preset or get_preset's run_ids"`
	Agent    string `json:"agent,omitempty" jsonschema:"agent role (e.g. 'CorOS Librarian') or prompt key (e.g. '4_coros_librarian'); omit for an index of all agents"`
	Part     string `json:"part,omitempty" jsonschema:"'system' (default), 'input' or 'both'"`
	Offset   int    `json:"offset,omitempty" jsonschema:"first character to return (default 0); use next_offset from the previous page"`
	MaxChars int    `json:"max_chars,omitempty" jsonschema:"page size in characters (default 40000, max 120000)"`
}

type promptIndexEntry struct {
	Role        string `json:"role"`
	PromptKey   string `json:"prompt_key"`
	Version     string `json:"version"`
	SystemChars int    `json:"system_chars"`
	InputChars  int    `json:"input_chars"`
	OutputChars int    `json:"output_chars"`
	Status      string `json:"status"`
}

// promptsOutput serves both modes: the index fields are set without `agent`,
// the paging fields with it. The slice itself is only in the text content, so a
// large page is not sent twice.
type promptsOutput struct {
	RunID           string             `json:"run_id"`
	UserPrompt      string             `json:"user_prompt,omitempty"`
	EffectivePrompt string             `json:"effective_prompt,omitempty"`
	Agents          []promptIndexEntry `json:"agents,omitempty"`
	Agent           string             `json:"agent,omitempty"`
	PromptKey       string             `json:"prompt_key,omitempty"`
	Part            string             `json:"part,omitempty"`
	Offset          int                `json:"offset"`
	ReturnedChars   int                `json:"returned_chars"`
	TotalChars      int                `json:"total_chars"`
	NextOffset      *int               `json:"next_offset,omitempty"`
}

func promptIndex(rec *runs.RunRecord) (promptsOutput, string) {
	out := promptsOutput{RunID: rec.ID, UserPrompt: rec.UserPrompt, EffectivePrompt: rec.EffectivePrompt, Agents: []promptIndexEntry{}}
	var b strings.Builder
	b.WriteString(runDataNotice + "\n\n")
	fmt.Fprintf(&b, "Prompts for run `%s`.\nUser prompt: %s\n", rec.ID, cell(truncateRunes(rec.UserPrompt, maxEchoedPromptRunes)))
	if rec.EffectivePrompt != "" && rec.EffectivePrompt != rec.UserPrompt {
		fmt.Fprintf(&b, "Effective prompt (with config-flag prefixes): %s\n", cell(truncateRunes(rec.EffectivePrompt, maxEchoedPromptRunes)))
	}
	b.WriteString("\nAgents (pass `agent` = role or key to read a prompt; sizes are in characters):\n")
	for _, t := range sortedAgents(rec) {
		if t.PromptKey == "" {
			continue
		}
		out.Agents = append(out.Agents, promptIndexEntry{
			Role: t.Role, PromptKey: t.PromptKey, Version: t.PromptVersion, SystemChars: t.SystemPromptChars,
			InputChars: t.InputChars, OutputChars: t.OutputChars, Status: t.Status,
		})
		fmt.Fprintf(&b, "- %s (key %s): system %d, input %d, output %d\n",
			cell(t.Role), keyVersion(t), t.SystemPromptChars, t.InputChars, t.OutputChars)
	}
	if len(out.Agents) == 0 {
		b.WriteString("(no agents have started yet)\n")
	}
	return out, strings.TrimRight(b.String(), "\n")
}

func getRunPrompts(svc *presetsvc.Service) mcp.ToolHandlerFor[promptsInput, promptsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in promptsInput) (*mcp.CallToolResult, promptsOutput, error) {
		rec, err := loadRun(ctx, svc, in.RunID)
		if err != nil {
			return nil, promptsOutput{}, toolError(promptsToolName, err)
		}
		if strings.TrimSpace(in.Agent) == "" {
			out, text := promptIndex(rec)
			return textResult(text), out, nil
		}
		t, err := findAgent(rec, in.Agent)
		if err != nil {
			return nil, promptsOutput{}, err
		}
		part := in.Part
		if part == "" {
			part = partSystem
		}
		full, err := readPromptText(ctx, svc, rec, t, part)
		if err != nil {
			return nil, promptsOutput{}, toolError(promptsToolName, err)
		}
		pg, err := pageText(full, in.Offset, in.MaxChars)
		if err != nil {
			return nil, promptsOutput{}, err
		}
		out := promptsOutput{
			RunID: rec.ID, Agent: t.Role, PromptKey: t.PromptKey, Part: part,
			Offset: pg.Offset, ReturnedChars: len([]rune(pg.Text)), TotalChars: pg.Total, NextOffset: pg.NextOffset,
		}
		heading := fmt.Sprintf("Run `%s`, agent %s (key %s), %s prompt.", rec.ID, cell(t.Role), keyVersion(t), part)
		return textResult(pagedText(heading, pg)), out, nil
	}
}

// ---- get_agent_output ----

type outputInput struct {
	RunID    string `json:"run_id" jsonschema:"run ID from generate_preset, refine_preset or get_preset's run_ids"`
	Agent    string `json:"agent" jsonschema:"agent role (e.g. 'CorOS Librarian') or prompt key (e.g. '4_coros_librarian'); see get_run_prompts for the list"`
	Offset   int    `json:"offset,omitempty" jsonschema:"first character to return (default 0); use next_offset from the previous page"`
	MaxChars int    `json:"max_chars,omitempty" jsonschema:"page size in characters (default 40000, max 120000)"`
}

type outputOutput struct {
	RunID         string `json:"run_id"`
	Agent         string `json:"agent"`
	PromptKey     string `json:"prompt_key"`
	Offset        int    `json:"offset"`
	ReturnedChars int    `json:"returned_chars"`
	TotalChars    int    `json:"total_chars"`
	NextOffset    *int   `json:"next_offset,omitempty"`
}

func getAgentOutput(svc *presetsvc.Service) mcp.ToolHandlerFor[outputInput, outputOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in outputInput) (*mcp.CallToolResult, outputOutput, error) {
		if strings.TrimSpace(in.Agent) == "" {
			return nil, outputOutput{}, errors.New("agent is required: an agent role or prompt key. Call get_run_prompts without `agent` to list them.")
		}
		rec, err := loadRun(ctx, svc, in.RunID)
		if err != nil {
			return nil, outputOutput{}, toolError(outputToolName, err)
		}
		t, err := findAgent(rec, in.Agent)
		if err != nil {
			return nil, outputOutput{}, err
		}
		full, err := readAgentLog(ctx, svc, rec, t, ".json")
		if err != nil {
			return nil, outputOutput{}, toolError(outputToolName, err)
		}
		pg, err := pageText(full, in.Offset, in.MaxChars)
		if err != nil {
			return nil, outputOutput{}, err
		}
		out := outputOutput{
			RunID: rec.ID, Agent: t.Role, PromptKey: t.PromptKey,
			Offset: pg.Offset, ReturnedChars: len([]rune(pg.Text)), TotalChars: pg.Total, NextOffset: pg.NextOffset,
		}
		heading := fmt.Sprintf("Run `%s`, output of agent %s (key %s).", rec.ID, cell(t.Role), t.PromptKey)
		return textResult(pagedText(heading, pg)), out, nil
	}
}
