package mcpserver

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weitzer-org/sound-builder/internal/runs"
)

// jnyEvent is the subset of a run_events.json step these tests check. The file
// is the mock pipeline's replay script; the strings here are what the server
// must store verbatim in the run's prompt and output logs.
type jnyEvent struct {
	Type   string `json:"type"`
	Role   string `json:"role"`
	System string `json:"system"`
	Input  string `json:"input"`
	Output string `json:"output"`
}

// jnyLoadFixture parses testdata/e2e_mocks/run_events.json, keyed by "generate" and "refine".
func jnyLoadFixture(t *testing.T) map[string][]jnyEvent {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(findRepoRoot(t), "testdata", "e2e_mocks", "run_events.json"))
	if err != nil {
		t.Fatal(err)
	}
	var all map[string][]jnyEvent
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatalf("run_events.json: %v", err)
	}
	return all
}

// jnyAgentEvent returns the agent step for role in a fixture list.
func jnyAgentEvent(t *testing.T, events []jnyEvent, role string) jnyEvent {
	t.Helper()
	for _, ev := range events {
		if ev.Type == "agent" && ev.Role == role {
			return ev
		}
	}
	t.Fatalf("run_events.json has no agent step for %q", role)
	return jnyEvent{}
}

// jnyData returns the text framed by the generated-data markers in a paged result.
func jnyData(t *testing.T, text string) string {
	t.Helper()
	b := strings.Index(text, beginData+"\n")
	if b < 0 {
		t.Fatalf("no begin marker in result: %q", text)
	}
	body := text[b+len(beginData)+1:]
	e := strings.LastIndex(body, "\n"+endData)
	if e < 0 {
		t.Fatalf("no end marker in result: %q", text)
	}
	return body[:e]
}

// jnyCounter counts every HTTP request the MCP client makes, then passes it on.
type jnyCounter struct {
	n    *atomic.Int64
	base http.RoundTripper
}

func (c jnyCounter) RoundTrip(r *http.Request) (*http.Response, error) {
	c.n.Add(1)
	return c.base.RoundTrip(r)
}

// jnyCall calls a tool through the real client, logs its text under label, and
// returns the result with its text.
func jnyCall(t *testing.T, cs *mcp.ClientSession, label, tool string, args map[string]any) (*mcp.CallToolResult, string) {
	t.Helper()
	res := itCall(t, cs, tool, args)
	text := itText(t, res)
	t.Logf("== %s (%s, isError=%v)\n%s", label, tool, res.IsError, text)
	return res, text
}

// jnyGenerate runs a mock generate_preset to completion and returns its result.
func jnyGenerate(t *testing.T, cs *mcp.ClientSession, label, prompt string) runOutput {
	t.Helper()
	res, _ := jnyCall(t, cs, label, generateToolName, map[string]any{"prompt": prompt, "mock": true, "wait_seconds": 30})
	if res.IsError {
		t.Fatalf("generate_preset failed: %s", itText(t, res))
	}
	out := itStructured[runOutput](t, res)
	if out.Status != runs.StatusComplete || out.PresetID == "" {
		t.Fatalf("generate_preset = %+v, want a complete run with a preset", out.statusOutput)
	}
	return out
}

// jnyRead reads one resource and returns its text.
func jnyRead(t *testing.T, cs *mcp.ClientSession, uri string) string {
	t.Helper()
	res, err := cs.ReadResource(itCtx(t), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("resources/read %s: %v", uri, err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("resources/read %s: %d contents, want 1", uri, len(res.Contents))
	}
	text := res.Contents[0].Text
	t.Logf("== resources/read %s (%d chars)\n%s", uri, utf8.RuneCountInString(text), jnyHead(text, 600))
	return text
}

// jnyHead shortens long text for the log only.
func jnyHead(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "\n...(log shortened)"
}

// jnyNext renders a next_offset pointer for a log line.
func jnyNext(p *int) string {
	if p == nil {
		return "none"
	}
	return strconv.Itoa(*p)
}

// ---- TEST 1: the J1-J5 loop, one session, every step asserted and logged ----

func TestMCPJourney_J1toJ5(t *testing.T) {
	h := newMockService(t)
	fx := jnyLoadFixture(t)

	var requests atomic.Int64
	ts := itServer(t, h.svc)
	hc := &http.Client{Transport: jnyCounter{n: &requests, base: itBearer{token: testToken, base: http.DefaultTransport}}}
	cs, err := itConnect(t, ts.URL+"/mcp", hc)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	t.Cleanup(func() { t.Logf("HTTP requests made by the client for the whole journey: %d", requests.Load()) })

	// 1. initialize
	ir := cs.InitializeResult()
	t.Logf("== 1 initialize: server=%s instructions=%q", ir.ServerInfo.Name, ir.Instructions)
	if strings.TrimSpace(ir.Instructions) == "" {
		t.Error("initialize: instructions are empty")
	}
	if ir.Capabilities == nil || ir.Capabilities.Tools == nil || ir.Capabilities.Resources == nil || ir.Capabilities.Prompts == nil {
		t.Errorf("initialize: capabilities = %+v, want tools, resources and prompts", ir.Capabilities)
	}

	// 2. tools/list
	tl, err := cs.ListTools(itCtx(t), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var names []string
	for _, tool := range tl.Tools {
		names = append(names, tool.Name)
	}
	t.Logf("== 2 tools/list (%d tools)\n%s", len(tl.Tools), strings.Join(names, "\n"))
	if len(tl.Tools) != 9 {
		t.Errorf("tools/list returned %d tools, want 9", len(tl.Tools))
	}

	// 3. J1: design a tone
	genRes, genText := jnyCall(t, cs, "3 J1 generate_preset", generateToolName, map[string]any{
		"prompt": "Mayer Gravity clean-to-lead on an ES-339, no paid plugins", "mock": true,
		"allow_paid_plugins": false, "wait_seconds": 30,
	})
	if genRes.IsError {
		t.Fatalf("generate_preset failed: %s", genText)
	}
	gen := itStructured[runOutput](t, genRes)
	if gen.Status != runs.StatusComplete {
		t.Fatalf("generate status = %q, want complete", gen.Status)
	}
	if !strings.HasPrefix(gen.RunID, "gen-") || gen.PresetID == "" {
		t.Fatalf("generate run_id=%q preset_id=%q", gen.RunID, gen.PresetID)
	}
	if !strings.Contains(genText, "Content below is generated preset data") || !strings.Contains(genText, "## ") {
		t.Errorf("generate text lacks the data notice or a preset heading")
	}

	// 4. get_run_status
	stRes, _ := jnyCall(t, cs, "4 get_run_status", statusToolName, map[string]any{"run_id": gen.RunID})
	st := itStructured[statusOutput](t, stRes)
	if st.Status != runs.StatusComplete || st.PresetID != gen.PresetID {
		t.Errorf("get_run_status = status %q preset %q, want complete %q", st.Status, st.PresetID, gen.PresetID)
	}

	// 5. J4: the run log
	logRes, _ := jnyCall(t, cs, "5 J4 get_run_log", logToolName, map[string]any{"run_id": gen.RunID})
	lo := itStructured[logOutput](t, logRes)
	if len(lo.Events) == 0 {
		t.Error("run log has no events")
	}
	prevSeq := 0
	for _, e := range lo.Events {
		if e.Seq <= prevSeq {
			t.Errorf("event seq %d follows %d: not strictly increasing", e.Seq, prevSeq)
		}
		prevSeq = e.Seq
	}
	if len(lo.Agents) != 13 {
		t.Errorf("run log has %d agents, want 13", len(lo.Agents))
	}
	if v, ok := lo.Constraints["allow_paid_plugins"]; !ok || v != false {
		t.Errorf("constraints[allow_paid_plugins] = %v, want false", v)
	}
	if lo.Source != runs.SourceMCP || !lo.Mock {
		t.Errorf("run log source=%q mock=%v, want mcp and true", lo.Source, lo.Mock)
	}

	// 6. J4: the prompt index
	idxRes, _ := jnyCall(t, cs, "6 J4 get_run_prompts index", promptsToolName, map[string]any{"run_id": gen.RunID})
	pi := itStructured[promptsOutput](t, idxRes)
	if len(pi.Agents) != 13 {
		t.Errorf("prompt index has %d agents, want 13", len(pi.Agents))
	}
	if !slices.ContainsFunc(pi.Agents, func(e promptIndexEntry) bool { return e.PromptKey == "4_coros_librarian" }) {
		t.Error("prompt index lacks 4_coros_librarian")
	}

	// 7. J4: exact prompt text, by role and by key
	gl := fx["generate"]
	libr := jnyAgentEvent(t, gl, "CorOS Librarian")
	_, inText := jnyCall(t, cs, "7 J4 get_run_prompts CorOS Librarian input", promptsToolName,
		map[string]any{"run_id": gen.RunID, "agent": "CorOS Librarian", "part": "input"})
	if got := jnyData(t, inText); got != libr.Input {
		t.Errorf("input text = %q, want %q", got, libr.Input)
	}
	_, sysText := jnyCall(t, cs, "7 J4 get_run_prompts 4_coros_librarian system", promptsToolName,
		map[string]any{"run_id": gen.RunID, "agent": "4_coros_librarian", "part": "system"})
	if got := jnyData(t, sysText); got != libr.System {
		t.Errorf("system text = %q, want %q", got, libr.System)
	}

	// 8. J4: paged system prompt, max_chars 10, following next_offset to the end
	total := utf8.RuneCountInString(libr.System)
	var paged strings.Builder
	offset, pages := 0, 0
	for {
		r := itCall(t, cs, promptsToolName, map[string]any{
			"run_id": gen.RunID, "agent": "4_coros_librarian", "part": "system", "max_chars": 10, "offset": offset,
		})
		pt := itText(t, r)
		if r.IsError {
			t.Fatalf("paged page at offset %d failed: %s", offset, pt)
		}
		po := itStructured[promptsOutput](t, r)
		t.Logf("== 8 paged system page %d: offset=%d returned=%d total=%d next=%s", pages+1, po.Offset, po.ReturnedChars, po.TotalChars, jnyNext(po.NextOffset))
		if po.TotalChars != total {
			t.Errorf("page %d total_chars = %d, want %d", pages+1, po.TotalChars, total)
		}
		if po.Offset != offset {
			t.Errorf("page %d offset = %d, want %d", pages+1, po.Offset, offset)
		}
		paged.WriteString(jnyData(t, pt))
		pages++
		if po.NextOffset == nil {
			break
		}
		if *po.NextOffset != offset+po.ReturnedChars {
			t.Fatalf("page %d next_offset = %d, want %d", pages, *po.NextOffset, offset+po.ReturnedChars)
		}
		offset = *po.NextOffset
		if pages > total {
			t.Fatal("paging did not end")
		}
	}
	if paged.String() != libr.System {
		t.Errorf("concatenated pages = %q, want %q", paged.String(), libr.System)
	}
	if want := (total + 9) / 10; pages != want {
		t.Errorf("paged read took %d pages, want %d", pages, want)
	}

	// 9. the architect's output
	_, archText := jnyCall(t, cs, "9 get_agent_output 12_architect", outputToolName,
		map[string]any{"run_id": gen.RunID, "agent": "12_architect"})
	arch := jnyAgentEvent(t, gl, "Architect & Evaluator")
	if got := jnyData(t, archText); got != arch.Output {
		t.Errorf("architect output = %q, want %q", got, arch.Output)
	}

	// 10. the preset and its run links
	pRes, pText := jnyCall(t, cs, "10 get_preset", getToolName, map[string]any{"preset_id": gen.PresetID})
	if pRes.IsError {
		t.Fatalf("get_preset failed: %s", pText)
	}
	po := itStructured[getOutput](t, pRes)
	if !strings.Contains(pText, "## ") {
		t.Errorf("get_preset text has no markdown heading")
	}
	if !slices.Contains(po.RunIDs, gen.RunID) {
		t.Errorf("get_preset run_ids = %v, want to contain %s", po.RunIDs, gen.RunID)
	}

	// 11. J2: refine, then read the refinement back
	rRes, rText := jnyCall(t, cs, "11 J2 refine_preset", refineToolName, map[string]any{
		"preset_id": gen.PresetID, "message": "less gain on the lead scene", "mock": true, "wait_seconds": 30,
	})
	if rRes.IsError {
		t.Fatalf("refine_preset failed: %s", rText)
	}
	rf := itStructured[runOutput](t, rRes)
	if rf.Status != runs.StatusComplete || !strings.HasPrefix(rf.RunID, "chat-") {
		t.Fatalf("refine status=%q run_id=%q, want complete chat-*", rf.Status, rf.RunID)
	}
	p2Res, _ := jnyCall(t, cs, "11 get_preset after refine", getToolName, map[string]any{"preset_id": gen.PresetID})
	p2 := itStructured[getOutput](t, p2Res)
	if want := []string{gen.RunID, rf.RunID}; !slices.Equal(p2.RunIDs, want) {
		t.Errorf("run_ids after refine = %v, want %v", p2.RunIDs, want)
	}
	rLogRes, _ := jnyCall(t, cs, "11 get_run_log refine", logToolName, map[string]any{"run_id": rf.RunID})
	rlo := itStructured[logOutput](t, rLogRes)
	if rlo.Kind != runs.KindRefine || rlo.ParentPresetID != gen.PresetID {
		t.Errorf("refine log kind=%q parent=%q, want refine and %s", rlo.Kind, rlo.ParentPresetID, gen.PresetID)
	}
	_, refOut := jnyCall(t, cs, "11 get_agent_output Refinement Architect", outputToolName,
		map[string]any{"run_id": rf.RunID, "agent": "Refinement Architect"})
	wantRef := jnyAgentEvent(t, fx["refine"], "Refinement Architect").Output
	if got := jnyData(t, refOut); got != wantRef {
		t.Errorf("refinement output = %q, want %q", got, wantRef)
	}

	// 12. J3: keep it
	svRes, svText := jnyCall(t, cs, "12 J3 save_preset", saveToolName, map[string]any{"preset_id": gen.PresetID, "name": "Gravity Live"})
	if svRes.IsError {
		t.Fatalf("save_preset failed: %s", svText)
	}
	_, lsText := jnyCall(t, cs, "12 list_presets after save", listToolName, nil)
	if line := itLineFor(lsText, gen.PresetID); !strings.Contains(line, "Gravity Live") || strings.Contains(line, "[draft]") {
		t.Errorf("list line after save = %q, want the saved name and no draft marker", line)
	}

	// 13. resources
	tpl, err := cs.ListResourceTemplates(itCtx(t), nil)
	if err != nil {
		t.Fatalf("resources/templates/list: %v", err)
	}
	var tplURIs []string
	for _, rt := range tpl.ResourceTemplates {
		tplURIs = append(tplURIs, rt.URITemplate)
	}
	t.Logf("== 13 resources/templates/list (%d)\n%s", len(tpl.ResourceTemplates), strings.Join(tplURIs, "\n"))
	if len(tpl.ResourceTemplates) != 4 {
		t.Errorf("resource templates = %d, want 4", len(tpl.ResourceTemplates))
	}
	for _, uri := range []string{
		"qc://runs/" + gen.RunID + "/log",
		"qc://runs/" + gen.RunID + "/prompts/4_coros_librarian",
		"qc://presets/" + gen.PresetID,
		"qc://prompt-templates/4_coros_librarian/v1",
	} {
		if jnyRead(t, cs, uri) == "" {
			t.Errorf("resources/read %s: empty", uri)
		}
	}

	// 14. prompts/get design_tone
	pr, err := cs.GetPrompt(itCtx(t), &mcp.GetPromptParams{
		Name: designToneName, Arguments: map[string]string{"song": "Gravity", "guitar": "ES-339"},
	})
	if err != nil {
		t.Fatalf("prompts/get: %v", err)
	}
	if len(pr.Messages) != 1 {
		t.Fatalf("design_tone returned %d messages, want 1", len(pr.Messages))
	}
	tc, ok := pr.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("design_tone content is %T, want *mcp.TextContent", pr.Messages[0].Content)
	}
	t.Logf("== 14 prompts/get design_tone\n%s", tc.Text)
	for _, w := range []string{"Gravity", "ES-339", "generate_preset"} {
		if !strings.Contains(tc.Text, w) {
			t.Errorf("design_tone message lacks %q", w)
		}
	}

	// 15. J5: a failed run, written straight to the store, then debugged through the tools
	now := time.Now()
	failed := &runs.RunRecord{
		ID: "gen-failed-1", Kind: runs.KindGenerate, Source: runs.SourceMCP, Status: runs.StatusError,
		Phase: "Phase 2/4: Mapping to Native CorOS Effects...", UserPrompt: "Gravity clean-to-lead",
		Constraints: map[string]any{"allow_paid_plugins": false},
		Error:       "Pipeline Execution Error: all models failed",
		Agents: map[string]*runs.AgentTrace{"CorOS Librarian": {
			Role: "CorOS Librarian", PromptKey: "4_coros_librarian", Status: "error",
			Model: "gemini-3.5-flash", Attempts: 2, Error: "quota exceeded",
		}},
		Events: []runs.RunEvent{
			{Seq: 1, TS: now, Type: runs.EventPhase, Msg: "Phase 2/4: Mapping to Native CorOS Effects..."},
			{Seq: 2, TS: now, Type: runs.EventAgentStart, Agent: "CorOS Librarian", Msg: "CorOS Librarian started"},
			{Seq: 3, TS: now, Type: runs.EventAttemptFailed, Agent: "CorOS Librarian", Model: "gemini-3.1-pro-preview",
				Msg: "CorOS Librarian attempt on gemini-3.1-pro-preview failed: quota exceeded"},
			{Seq: 4, TS: now, Type: runs.EventAttemptFailed, Agent: "CorOS Librarian", Model: "gemini-3.5-flash",
				Msg: "CorOS Librarian attempt on gemini-3.5-flash failed: quota exceeded"},
			{Seq: 5, TS: now, Type: runs.EventAgentError, Agent: "CorOS Librarian", Model: "gemini-3.5-flash",
				Msg: "CorOS Librarian failed: quota exceeded"},
		},
		Mock: false, CreatedAt: now, UpdatedAt: now, CompletedAt: &now,
	}
	if err := h.svc.Runs().Put(t.Context(), failed); err != nil {
		t.Fatalf("writing the failed run: %v", err)
	}
	fsRes, fsText := jnyCall(t, cs, "15 J5 get_run_status failed run", statusToolName, map[string]any{"run_id": failed.ID})
	fs := itStructured[statusOutput](t, fsRes)
	if fs.Status != runs.StatusError || !strings.Contains(fs.Error, "all models failed") || !strings.Contains(fsText, "Pipeline Execution Error: all models failed") {
		t.Errorf("failed status = %q error=%q; text=%q", fs.Status, fs.Error, fsText)
	}
	flRes, flText := jnyCall(t, cs, "15 J5 get_run_log failed run", logToolName, map[string]any{"run_id": failed.ID})
	if flRes.IsError {
		t.Fatalf("get_run_log failed run: %s", flText)
	}
	for _, w := range []string{"CorOS Librarian", "gemini-3.1-pro-preview", "gemini-3.5-flash", "quota exceeded"} {
		if !strings.Contains(flText, w) {
			t.Errorf("failed-run log lacks %q", w)
		}
	}
}

// ---- TEST 2: paging and error edge cases on one mock run ----

func TestMCPJourney_PagingAndErrors(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	fx := jnyLoadFixture(t)
	sysText := jnyAgentEvent(t, fx["generate"], "CorOS Librarian").System
	total := utf8.RuneCountInString(sysText)
	gen := jnyGenerate(t, cs, "2 setup", "paging and errors")

	t.Run("offset past end is an error naming the total", func(t *testing.T) {
		res, text := jnyCall(t, cs, "past end", promptsToolName, map[string]any{
			"run_id": gen.RunID, "agent": "4_coros_librarian", "part": "system", "offset": total + 1,
		})
		if !res.IsError || !strings.Contains(text, strconv.Itoa(total)) {
			t.Errorf("IsError=%v, want true with the total %d in the text: %q", res.IsError, total, text)
		}
	})

	t.Run("offset exactly at end is an empty final page", func(t *testing.T) {
		res, text := jnyCall(t, cs, "at end", promptsToolName, map[string]any{
			"run_id": gen.RunID, "agent": "4_coros_librarian", "part": "system", "offset": total,
		})
		if res.IsError {
			t.Fatalf("offset at end is an error: %s", text)
		}
		po := itStructured[promptsOutput](t, res)
		if po.ReturnedChars != 0 || po.NextOffset != nil || po.TotalChars != total {
			t.Errorf("at end: returned=%d next=%s total=%d", po.ReturnedChars, jnyNext(po.NextOffset), po.TotalChars)
		}
		if got := jnyData(t, text); got != "" {
			t.Errorf("at end: data = %q, want empty", got)
		}
	})

	t.Run("max_chars 1 returns one character", func(t *testing.T) {
		res, text := jnyCall(t, cs, "max_chars 1", promptsToolName, map[string]any{
			"run_id": gen.RunID, "agent": "4_coros_librarian", "part": "system", "max_chars": 1,
		})
		if res.IsError {
			t.Fatalf("max_chars 1 failed: %s", text)
		}
		po := itStructured[promptsOutput](t, res)
		if po.ReturnedChars != 1 || po.NextOffset == nil || *po.NextOffset != 1 {
			t.Errorf("max_chars 1: returned=%d next=%s, want 1 and 1", po.ReturnedChars, jnyNext(po.NextOffset))
		}
		if got := jnyData(t, text); got != sysText[:1] {
			t.Errorf("max_chars 1: data = %q, want %q", got, sysText[:1])
		}
	})

	t.Run("max_chars above the cap is capped", func(t *testing.T) {
		big := jnyGenerate(t, cs, "3 setup big", "big log")
		body := strings.Repeat("0123456789", 13000) // 130000 characters
		if err := h.svc.Runs().WriteLog(t.Context(), big.RunID, "4_coros_librarian.system.md", []byte(body)); err != nil {
			t.Fatal(err)
		}
		res, text := jnyCall(t, cs, "max_chars 500000", promptsToolName, map[string]any{
			"run_id": big.RunID, "agent": "4_coros_librarian", "part": "system", "max_chars": 500000,
		})
		if res.IsError {
			t.Fatalf("capped page failed: %s", text)
		}
		po := itStructured[promptsOutput](t, res)
		if po.ReturnedChars != maxPageChars || po.NextOffset == nil || *po.NextOffset != maxPageChars || po.TotalChars != 130000 {
			t.Errorf("capped: returned=%d next=%s total=%d, want %d, %d, 130000",
				po.ReturnedChars, jnyNext(po.NextOffset), po.TotalChars, maxPageChars, maxPageChars)
		}
		if n := utf8.RuneCountInString(jnyData(t, text)); n != maxPageChars {
			t.Errorf("capped data has %d characters, want %d", n, maxPageChars)
		}
		// The rest comes from the next offset.
		res, text = jnyCall(t, cs, "capped tail", promptsToolName, map[string]any{
			"run_id": big.RunID, "agent": "4_coros_librarian", "part": "system", "offset": maxPageChars, "max_chars": 500000,
		})
		po = itStructured[promptsOutput](t, res)
		if po.ReturnedChars != 10000 || po.NextOffset != nil {
			t.Errorf("tail: returned=%d next=%s, want 10000 and none", po.ReturnedChars, jnyNext(po.NextOffset))
		}
	})

	t.Run("multi-byte characters page by character", func(t *testing.T) {
		mb := jnyGenerate(t, cs, "4 setup multibyte", "multibyte log")
		if err := h.svc.Runs().WriteLog(t.Context(), mb.RunID, "1_tone_historian.system.md", []byte("héllo wörld")); err != nil {
			t.Fatal(err)
		}
		res, text := jnyCall(t, cs, "multibyte", promptsToolName, map[string]any{
			"run_id": mb.RunID, "agent": "1_tone_historian", "part": "system", "offset": 1, "max_chars": 3,
		})
		if res.IsError {
			t.Fatalf("multibyte page failed: %s", text)
		}
		po := itStructured[promptsOutput](t, res)
		if got := jnyData(t, text); got != "éll" {
			t.Errorf("data = %q, want %q", got, "éll")
		}
		if po.TotalChars != 11 || po.NextOffset == nil || *po.NextOffset != 4 {
			t.Errorf("multibyte: total=%d next=%s, want 11 and 4", po.TotalChars, jnyNext(po.NextOffset))
		}
	})

	t.Run("errors", func(t *testing.T) {
		const missing = "gen-does-not-exist"
		cases := []struct {
			name string
			tool string
			args map[string]any
			want []string // substrings the error text must contain
		}{
			{"unknown run: status", statusToolName, map[string]any{"run_id": missing}, []string{"run_id", "generate_preset"}},
			{"unknown run: log", logToolName, map[string]any{"run_id": missing}, []string{"run_id", "generate_preset"}},
			{"unknown run: prompts", promptsToolName, map[string]any{"run_id": missing}, []string{"run_id", "generate_preset"}},
			{"unknown run: output", outputToolName, map[string]any{"run_id": missing, "agent": "CorOS Librarian"}, []string{"run_id", "generate_preset"}},

			{"invalid run id: status", statusToolName, map[string]any{"run_id": "../x"}, []string{"run_id"}},
			{"invalid run id: log", logToolName, map[string]any{"run_id": "a/b"}, []string{"run_id"}},
			{"invalid run id: prompts", promptsToolName, map[string]any{"run_id": "../x"}, []string{"run_id"}},
			{"invalid run id: output", outputToolName, map[string]any{"run_id": "a/b", "agent": "CorOS Librarian"}, []string{"run_id"}},

			{"unknown agent: prompts", promptsToolName, map[string]any{"run_id": gen.RunID, "agent": "Nonexistent Agent"}, []string{"4_coros_librarian"}},
			{"unknown agent: output", outputToolName, map[string]any{"run_id": gen.RunID, "agent": "Nonexistent Agent"}, []string{"4_coros_librarian"}},
			{"empty agent: output", outputToolName, map[string]any{"run_id": gen.RunID, "agent": ""}, []string{"agent"}},
			{"bogus part", promptsToolName, map[string]any{"run_id": gen.RunID, "agent": "4_coros_librarian", "part": "bogus"}, []string{"part"}},
			{"bogus until", statusToolName, map[string]any{"run_id": gen.RunID, "until": "bogus"}, []string{"until"}},
			{"refine unknown preset", refineToolName, map[string]any{"preset_id": "missing-preset", "message": "less gain", "mock": true}, []string{"list_presets"}},
			{"generate empty prompt", generateToolName, map[string]any{"prompt": "   ", "mock": true}, []string{"prompt"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				res, text := jnyCall(t, cs, tc.name, tc.tool, tc.args)
				if !res.IsError {
					t.Fatalf("IsError = false, want true; text=%q", text)
				}
				for _, w := range tc.want {
					if !strings.Contains(text, w) {
						t.Errorf("error text missing %q: %q", w, text)
					}
				}
				if strings.Contains(strings.ToLower(text), "internal error") {
					t.Errorf("error leaks a generic internal error: %q", text)
				}
			})
		}
	})
}
