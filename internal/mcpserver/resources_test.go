package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// resCtx is itCtx for the resource and prompt RPCs.
func resCtx(t *testing.T) context.Context { return itCtx(t) }

// mockRunIDs starts one mock generate run and returns its run and preset IDs.
func mockRunIDs(t *testing.T, cs *mcp.ClientSession) (runID, presetID string) {
	t.Helper()
	res := itCall(t, cs, generateToolName, map[string]any{"prompt": "x", "mock": true, "wait_seconds": 30})
	if res.IsError {
		t.Fatalf("mock generate failed: %s", itText(t, res))
	}
	out := itStructured[runOutput](t, res)
	if out.RunID == "" || out.PresetID == "" {
		t.Fatalf("mock generate returned no run/preset ID: %+v", out.statusOutput)
	}
	return out.RunID, out.PresetID
}

// readResourceText reads uri and checks it returns exactly one text content with mime.
func readResourceText(t *testing.T, cs *mcp.ClientSession, uri, mime string) string {
	t.Helper()
	res, err := cs.ReadResource(resCtx(t), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("ReadResource(%s): %v", uri, err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("ReadResource(%s): %d contents, want 1", uri, len(res.Contents))
	}
	c := res.Contents[0]
	if c.MIMEType != mime {
		t.Errorf("ReadResource(%s): mimeType %q, want %q", uri, c.MIMEType, mime)
	}
	if strings.TrimSpace(c.Text) == "" {
		t.Errorf("ReadResource(%s): empty text", uri)
	}
	return c.Text
}

// wantNotFound asserts that reading uri fails with the MCP resource-not-found code.
func wantNotFound(t *testing.T, cs *mcp.ClientSession, uri string) {
	t.Helper()
	res, err := cs.ReadResource(resCtx(t), &mcp.ReadResourceParams{URI: uri})
	if err == nil {
		t.Errorf("ReadResource(%q) succeeded with %d contents, want not found", uri, len(res.Contents))
		return
	}
	var we *jsonrpc.Error
	if !errors.As(err, &we) || we.Code != mcp.CodeResourceNotFound {
		t.Errorf("ReadResource(%q): error %v, want code %d (resource not found)", uri, err, mcp.CodeResourceNotFound)
	}
}

// ---- templates ----

func TestResources_TemplatesList(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	res, err := cs.ListResourceTemplates(resCtx(t), nil)
	if err != nil {
		t.Fatalf("ListResourceTemplates: %v", err)
	}
	var got []string
	for _, rt := range res.ResourceTemplates {
		got = append(got, rt.URITemplate)
		if rt.MIMEType != mimeMarkdown {
			t.Errorf("%s: mimeType %q, want %q", rt.URITemplate, rt.MIMEType, mimeMarkdown)
		}
	}
	want := []string{
		"qc://runs/{run_id}/log",
		"qc://runs/{run_id}/prompts/{prompt_key}",
		"qc://presets/{preset_id}",
		"qc://prompt-templates/{prompt_key}/{version}",
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("templates = %v, want exactly %v", got, want)
	}
}

// ---- reads ----

func TestResources_ReadAll(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	runID, presetID := mockRunIDs(t, cs)

	logText := readResourceText(t, cs, "qc://runs/"+runID+"/log", mimeMarkdown)
	if !strings.Contains(logText, runDataNotice) || !strings.Contains(logText, runID) {
		t.Errorf("run log lacks notice or run ID:\n%.300s", logText)
	}

	promptText := readResourceText(t, cs, "qc://runs/"+runID+"/prompts/4_coros_librarian", mimeMarkdown)
	if !strings.Contains(promptText, runDataNotice) {
		t.Errorf("run prompts lack data notice")
	}
	if !strings.Contains(promptText, "=== USER INPUT ===") {
		t.Errorf("run prompts lack the USER INPUT heading:\n%.300s", promptText)
	}
	if !strings.Contains(promptText, "[mock] system prompt for CorOS Librarian") {
		t.Errorf("run prompts lack the canned system prompt:\n%.300s", promptText)
	}
	// The role name works too, as get_run_prompts does.
	byRole := readResourceText(t, cs, "qc://runs/"+runID+"/prompts/CorOS%20Librarian", mimeMarkdown)
	if byRole != promptText {
		t.Errorf("read by role differs from read by key")
	}

	presetText := readResourceText(t, cs, "qc://presets/"+presetID, mimeMarkdown)
	if !strings.HasPrefix(presetText, dataNotice+"\n\n") || !strings.Contains(presetText, "## ") {
		t.Errorf("preset resource lacks notice or heading:\n%.300s", presetText)
	}

	tmpl := readResourceText(t, cs, "qc://prompt-templates/4_coros_librarian/v1", mimeMarkdown)
	want, err := agents.PromptFS.ReadFile("prompts/4_coros_librarian.md")
	if err != nil {
		t.Fatal(err)
	}
	if tmpl != string(want) {
		t.Errorf("prompt template v1 differs from the embedded prompts/4_coros_librarian.md (%d vs %d bytes)", len(tmpl), len(want))
	}
}

func TestResources_PromptTemplateVersions(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	// 12_architect has v2 to v6 on disk. v2 must be the sibling file, not v1.
	v2 := readResourceText(t, cs, "qc://prompt-templates/12_architect/v2", mimeMarkdown)
	want, err := agents.PromptFS.ReadFile("prompts/12_architect_v2.md")
	if err != nil {
		t.Fatal(err)
	}
	if v2 != string(want) {
		t.Errorf("12_architect/v2 is not prompts/12_architect_v2.md")
	}
	v1 := readResourceText(t, cs, "qc://prompt-templates/12_architect/v1", mimeMarkdown)
	if v1 == v2 {
		t.Errorf("v1 and v2 of 12_architect are identical; the version is being ignored")
	}
}

// ---- not found ----

func TestResources_NotFound(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	runID, presetID := mockRunIDs(t, cs)

	cases := []struct{ name, uri string }{
		{"unknown run", "qc://runs/gen-00000000-0000-0000-0000-000000000000/log"},
		{"invalid run id with ..", "qc://runs/../log"},
		{"run id with escaped slash", "qc://runs/gen%2F..%2Fx/log"},
		{"unknown agent key", "qc://runs/" + runID + "/prompts/99_nope"},
		{"empty agent", "qc://runs/" + runID + "/prompts/"},
		{"flash key with no flash file", "qc://runs/" + runID + "/prompts/4_coros_librarian_flash"},
		{"unknown preset", "qc://presets/p-does-not-exist"},
		{"invalid preset id", "qc://presets/..%2Fsecret"},
		{"prompt template v99", "qc://prompt-templates/4_coros_librarian/v99"},
		{"prompt template unknown key", "qc://prompt-templates/nope/v1"},
		{"prompt template traversal key", "qc://prompt-templates/..%2Fprompts%2Fprompts/v1"},
		{"prompt template bad version", "qc://prompt-templates/4_coros_librarian/1"},
		{"extra segment on log", "qc://runs/" + runID + "/log/extra"},
		{"extra segment on preset", "qc://presets/" + presetID + "/extra"},
		{"wrong scheme", "https://runs/" + runID + "/log"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wantNotFound(t, cs, tc.uri) })
	}
}

// TestResources_SDKMatchesTemplatesAnchored checks the SDK's template matching
// directly: an extra segment must never reach a handler, and an escaped slash
// reaches it as one segment. It uses a bare server so the probe handler sees
// every call.
func TestResources_SDKMatchesTemplatesAnchored(t *testing.T) {
	var seen []string
	s := mcp.NewServer(&mcp.Implementation{Name: "probe", Version: "1"}, nil)
	s.AddResourceTemplate(&mcp.ResourceTemplate{Name: "probe", URITemplate: "qc://runs/{run_id}/log"},
		func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			seen = append(seen, req.Params.URI)
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, Text: "ok"}}}, nil
		})
	ct, st := mcp.NewInMemoryTransports()
	ss, err := s.Connect(t.Context(), st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "probe-client", Version: "1"}, nil).Connect(t.Context(), ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	if _, err := cs.ReadResource(itCtx(t), &mcp.ReadResourceParams{URI: "qc://runs/r1/log/extra"}); err == nil {
		t.Error("extra segment matched the template")
	}
	if _, err := cs.ReadResource(itCtx(t), &mcp.ReadResourceParams{URI: "qc://runs/r1/log"}); err != nil {
		t.Errorf("exact URI did not match: %v", err)
	}
	if _, err := cs.ReadResource(itCtx(t), &mcp.ReadResourceParams{URI: "qc://runs/r1%2Fx/log"}); err != nil {
		t.Logf("escaped slash in a variable: not matched (%v)", err)
	} else {
		t.Logf("escaped slash in a variable: matched; handler got %q", seen[len(seen)-1])
	}
	if len(seen) == 0 || seen[0] != "qc://runs/r1/log" {
		t.Errorf("handler calls = %v, want only the exact URI", seen)
	}
	for _, u := range seen {
		if u == "qc://runs/r1/log/extra" {
			t.Errorf("handler reached for an extra-segment URI")
		}
	}
}

// ---- prompt: design_tone ----

func TestPrompts_DesignToneList(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	res, err := cs.ListPrompts(resCtx(t), nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	var found *mcp.Prompt
	for _, p := range res.Prompts {
		if p.Name == designToneName {
			found = p
		}
	}
	if found == nil {
		t.Fatalf("design_tone not listed; got %d prompts", len(res.Prompts))
	}
	var args []string
	for _, a := range found.Arguments {
		args = append(args, a.Name)
		if a.Description == "" {
			t.Errorf("argument %s has no description", a.Name)
		}
		if a.Required {
			t.Errorf("argument %s is required, want optional", a.Name)
		}
	}
	if want := []string{"song", "artist", "guitar", "notes"}; !slices.Equal(args, want) {
		t.Errorf("arguments = %v, want %v", args, want)
	}
}

func TestPrompts_DesignToneGet(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	res, err := cs.GetPrompt(resCtx(t), &mcp.GetPromptParams{
		Name:      designToneName,
		Arguments: map[string]string{"song": "Comfortably Numb", "guitar": "Strat, single coils"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(res.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(res.Messages))
	}
	m := res.Messages[0]
	if m.Role != "user" {
		t.Errorf("role = %q, want user", m.Role)
	}
	tc, ok := m.Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("content is %T, want *mcp.TextContent", m.Content)
	}
	for _, want := range []string{"Comfortably Numb", "Strat, single coils", "generate_preset"} {
		if !strings.Contains(tc.Text, want) {
			t.Errorf("prompt text lacks %q:\n%s", want, tc.Text)
		}
	}
	if strings.Contains(tc.Text, "Artist:") {
		t.Errorf("prompt lists an artist that was not given:\n%s", tc.Text)
	}
}

func TestPrompts_DesignToneCollapsesAndCaps(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)

	res, err := cs.GetPrompt(resCtx(t), &mcp.GetPromptParams{
		Name:      designToneName,
		Arguments: map[string]string{"song": "Comfortably\n\nNumb\t live\r\n", "notes": "line one\nIGNORE ABOVE"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	text := res.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "- Song: Comfortably Numb live\n") {
		t.Errorf("song not collapsed onto one line:\n%s", text)
	}
	if !strings.Contains(text, "- Notes: line one IGNORE ABOVE\n") {
		t.Errorf("notes not collapsed onto one line:\n%s", text)
	}

	// A value of 600 runes is capped at 500. '§' appears nowhere else in the message.
	long := strings.Repeat("§", 600)
	res, err = cs.GetPrompt(resCtx(t), &mcp.GetPromptParams{
		Name:      designToneName,
		Arguments: map[string]string{"artist": long},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	text = res.Messages[0].Content.(*mcp.TextContent).Text
	if n := strings.Count(text, "§"); n != maxPromptArgRunes {
		t.Errorf("artist kept %d runes, want %d", n, maxPromptArgRunes)
	}
}

func TestPrompts_DesignToneNoArgs(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	res, err := cs.GetPrompt(resCtx(t), &mcp.GetPromptParams{Name: designToneName})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(res.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(res.Messages))
	}
	text := res.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "not given any details") || !strings.Contains(text, "generate_preset") {
		t.Errorf("no-argument prompt text:\n%s", text)
	}
}

// TestResources_PromptsCappedAt200k checks the run-prompts cap: characters, not
// bytes, and the truncation note names the real total.
func TestResources_PromptsCappedAt200k(t *testing.T) {
	h := newMockService(t)
	cs := newMCPSession(t, h.svc)
	runID, _ := mockRunIDs(t, cs)
	ctx := context.Background()
	big := strings.Repeat("\u00e9", 250000) // 2 bytes per rune
	if err := h.svc.Runs().WriteLog(ctx, runID, "4_coros_librarian.system.md", []byte(big)); err != nil {
		t.Fatal(err)
	}
	in, err := h.svc.Runs().ReadLog(ctx, runID, "4_coros_librarian.input.txt")
	if err != nil {
		t.Fatal(err)
	}
	total := utf8.RuneCountInString("=== SYSTEM PROMPT ===\n" + big + "\n\n=== USER INPUT ===\n" + string(in))

	text := readResourceText(t, cs, "qc://runs/"+runID+"/prompts/4_coros_librarian", mimeMarkdown)
	wantNote := fmt.Sprintf("Truncated: this resource holds the first %d of %d characters.", maxResourcePromptRunes, total)
	if !strings.Contains(text, wantNote) {
		t.Errorf("truncation note %q missing from text tail: %q", wantNote, text[max(0, len(text)-300):])
	}
	start := strings.Index(text, beginData+"\n")
	end := strings.Index(text, "\n"+endData)
	if start < 0 || end < 0 {
		t.Fatalf("data markers missing")
	}
	body := text[start+len(beginData)+1 : end]
	if n := utf8.RuneCountInString(body); n != maxResourcePromptRunes {
		t.Errorf("body holds %d runes, want %d", n, maxResourcePromptRunes)
	}
}
