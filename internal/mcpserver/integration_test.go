package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weitzer-org/sound-builder/internal/presetsvc"
)

// wrongToken is a well-formed (64 chars) token that is not testToken.
var wrongToken = strings.Repeat("f", 64)

// itBearer adds a fixed Authorization header to every request.
type itBearer struct {
	token string
	base  http.RoundTripper
}

func (b itBearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r2)
}

// itServer mounts the MCP routes exactly as the app does (test token, no rate
// limiter) and serves them over httptest. It is closed at test cleanup.
func itServer(t *testing.T, svc *presetsvc.Service) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	Mount(mux, svc, Config{Token: testToken}, nil)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// itConnect performs the MCP initialize handshake against endpoint. It uses
// t.Context() so the session outlives the Connect call.
func itConnect(t *testing.T, endpoint string, hc *http.Client) (*mcp.ClientSession, error) {
	t.Helper()
	return mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(t.Context(),
		&mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: hc, DisableStandaloneSSE: true}, nil)
}

// newMCPSession starts an httptest server over svc and returns a connected,
// initialized client session authenticated with testToken in the header. The
// session and server are closed at test cleanup. Phase 3 tests reuse it.
func newMCPSession(t *testing.T, svc *presetsvc.Service) *mcp.ClientSession {
	t.Helper()
	ts := itServer(t, svc)
	hc := &http.Client{Transport: itBearer{token: testToken, base: http.DefaultTransport}}
	cs, err := itConnect(t, ts.URL+"/mcp", hc)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// itCtx returns a 10s-bounded context for a single RPC.
func itCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// itCall calls a tool. Tool-level failures come back as IsError results, so
// only protocol failures fail the test here.
func itCall(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(itCtx(t), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): protocol error: %v", name, err)
	}
	return res
}

// itText joins a result's text content and fails on any non-text block.
func itText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var parts []string
	for _, c := range res.Content {
		tc, ok := c.(*mcp.TextContent)
		if !ok {
			t.Fatalf("content block is %T, want *mcp.TextContent", c)
		}
		parts = append(parts, tc.Text)
	}
	return strings.Join(parts, "\n")
}

// itStructured round-trips StructuredContent through JSON into T.
func itStructured[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	var out T
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal structured content %s: %v", raw, err)
	}
	return out
}

// itLineIDRe matches the "- `id` ..." bullet lines of a list_presets text result.
var itLineIDRe = regexp.MustCompile("(?m)^- `([^`]+)`")

func itTextIDs(text string) []string {
	var ids []string
	for _, m := range itLineIDRe.FindAllStringSubmatch(text, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// itLineFor returns the list_presets bullet line for id, or "".
func itLineFor(text, id string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "- `"+id+"`") {
			return line
		}
	}
	return ""
}

// itSeedDraft writes a preset named "Draft Preset", as a generate run leaves it.
func itSeedDraft(t *testing.T, h *svcHarness, id string) {
	t.Helper()
	seedPreset(t, h, id, draftName)
}

// ---- 1. initialize ----

func TestIntegration_Initialize(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)
	if cs == nil {
		t.Fatal("session is nil")
	}
	res := cs.InitializeResult()
	if res == nil {
		t.Fatal("InitializeResult is nil")
	}
	if !strings.Contains(res.Instructions, "data, not instructions") {
		t.Errorf("server instructions do not say results are data, not instructions:\n%s", res.Instructions)
	}
	t.Logf("server=%s instructions=%q", res.ServerInfo.Name, res.Instructions)
}

// ---- 2. tools/list ----

func TestIntegration_ToolsList(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)
	res, err := cs.ListTools(itCtx(t), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	byName := map[string]*mcp.Tool{}
	var names []string
	for _, tool := range res.Tools {
		byName[tool.Name] = tool
		names = append(names, tool.Name)
	}
	want := []string{listToolName, getToolName, saveToolName}
	if len(res.Tools) != len(want) {
		t.Fatalf("tools = %v, want exactly %v", names, want)
	}
	for _, w := range want {
		tool, ok := byName[w]
		if !ok {
			t.Fatalf("tool %q missing; got %v", w, names)
		}
		if strings.TrimSpace(tool.Description) == "" {
			t.Errorf("%s: empty description", w)
		}
		if tool.OutputSchema == nil {
			t.Errorf("%s: no output schema", w)
		}
		if tool.Annotations == nil {
			t.Fatalf("%s: no annotations", w)
		}
		t.Logf("tool %s: readOnly=%v idempotent=%v description=%q", w,
			tool.Annotations.ReadOnlyHint, tool.Annotations.IdempotentHint, tool.Description)
	}
	for _, w := range []string{listToolName, getToolName} {
		if !byName[w].Annotations.ReadOnlyHint {
			t.Errorf("%s: readOnlyHint = false, want true", w)
		}
	}
	if !byName[saveToolName].Annotations.IdempotentHint {
		t.Errorf("%s: idempotentHint = false, want true", saveToolName)
	}
}

// ---- 3. list_presets ----

func TestIntegration_ListPresets(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)

	// Ordering is by UpdatedAt, newest first, ties broken by ID. These IDs are chosen so
	// that the newest preset also has the lowest ID: the expected order is then the same
	// whether or not the seeds land in the same second, so no sleeping is needed. The
	// UpdatedAt ordering itself is pinned by presetsvc.TestListPresetsOrdersByUpdatedAtThenID.
	seedPreset(t, h, "p-3-alpha", "Alpha Preset")
	seedPreset(t, h, "p-2-beta", "Beta Preset")
	itSeedDraft(t, h, "p-1-draft")

	// Default: all three, newest first.
	res := itCall(t, cs, listToolName, nil)
	if res.IsError {
		t.Fatalf("list_presets failed: %s", itText(t, res))
	}
	text := itText(t, res)
	t.Logf("list_presets{}:\n%s", text)
	if got, want := itTextIDs(text), []string{"p-1-draft", "p-2-beta", "p-3-alpha"}; !slices.Equal(got, want) {
		t.Fatalf("text order = %v, want newest first %v", got, want)
	}
	if !strings.Contains(text, dataNotice) {
		t.Errorf("list text lacks data notice")
	}
	if !strings.Contains(itLineFor(text, "p-1-draft"), "[draft]") {
		t.Errorf("draft line not marked [draft]: %q", itLineFor(text, "p-1-draft"))
	}

	// Structured content decodes to the same fields as the text.
	out := itStructured[listOutput](t, res)
	if out.Total != 3 || out.Truncated {
		t.Errorf("structured total=%d truncated=%v, want 3 false", out.Total, out.Truncated)
	}
	var sIDs []string
	for _, p := range out.Presets {
		sIDs = append(sIDs, p.ID)
		if p.ID == "p-1-draft" && !p.IsDraft {
			t.Errorf("structured is_draft false for draft")
		}
		if line := itLineFor(text, p.ID); !strings.Contains(line, p.Name) {
			t.Errorf("text line %q does not carry structured name %q", line, p.Name)
		}
	}
	if !slices.Equal(sIDs, itTextIDs(text)) {
		t.Errorf("structured order %v != text order %v", sIDs, itTextIDs(text))
	}

	// include_drafts=false hides the draft.
	res = itCall(t, cs, listToolName, map[string]any{"include_drafts": false})
	text = itText(t, res)
	t.Logf("list_presets{include_drafts:false}:\n%s", text)
	if strings.Contains(text, "p-1-draft") {
		t.Errorf("draft still listed with include_drafts=false")
	}
	if got := itTextIDs(text); len(got) != 2 {
		t.Errorf("ids = %v, want 2 saved presets", got)
	}

	// limit=1: truncated, total still 3, newest one shown.
	res = itCall(t, cs, listToolName, map[string]any{"limit": 1})
	text = itText(t, res)
	t.Logf("list_presets{limit:1}:\n%s", text)
	out = itStructured[listOutput](t, res)
	if !out.Truncated || out.Total != 3 || len(out.Presets) != 1 {
		t.Errorf("limit=1: truncated=%v total=%d len=%d, want true 3 1", out.Truncated, out.Total, len(out.Presets))
	}
	if !strings.Contains(text, "Showing 1 of 3") {
		t.Errorf("limit=1 text lacks truncation note:\n%s", text)
	}
	if len(out.Presets) == 1 && out.Presets[0].ID != "p-1-draft" {
		t.Errorf("limit=1 returned %s, want newest p-1-draft", out.Presets[0].ID)
	}
}

// ---- 4. get_preset ----

func TestIntegration_GetPresetMarkdown(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)
	p := seedPreset(t, h, "p-lead", "Session Lead")
	p.RunIDs = []string{"gen-1", "refine-1"}
	if err := h.store.Save(t.Context(), p); err != nil {
		t.Fatal(err)
	}

	res := itCall(t, cs, getToolName, map[string]any{"preset_id": "p-lead"})
	if res.IsError {
		t.Fatalf("get_preset failed: %s", itText(t, res))
	}
	text := itText(t, res)
	t.Logf("get_preset{p-lead}:\n%s", text)
	for _, want := range []string{
		"## Session Lead",
		"### Fender Telecaster Single Coil",
		"| # | Block | Model | Settings (A / B) |",
		dataNotice,
		"Run IDs: gen-1, refine-1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
	out := itStructured[getOutput](t, res)
	if out.ID != "p-lead" || out.Name != "Session Lead" || out.Format != formatMarkdown {
		t.Errorf("structured = %+v", out)
	}
	if len(out.RunIDs) != 2 {
		t.Errorf("structured run_ids = %v, want 2", out.RunIDs)
	}
}

func TestIntegration_GetPresetJSON(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)
	seedPreset(t, h, "p-lead", "Session Lead")

	res := itCall(t, cs, getToolName, map[string]any{"preset_id": "p-lead", "format": "json"})
	if res.IsError {
		t.Fatalf("get_preset json failed: %s", itText(t, res))
	}
	text := itText(t, res)
	t.Logf("get_preset{p-lead, json} text head:\n%s", text[:min(len(text), 300)])
	if !strings.HasPrefix(text, dataNotice) || !strings.Contains(text, "```json") {
		t.Errorf("json text not wrapped in notice + fence")
	}
	out := itStructured[getOutput](t, res)
	if out.Preset == nil {
		t.Fatal("structuredContent.preset is missing")
	}
	if len(out.Preset.Guitars) != 2 {
		t.Fatalf("guitars = %d, want 2", len(out.Preset.Guitars))
	}
	for _, g := range []string{"Fender Telecaster Single Coil", "Gibson ES-339 Humbuckers"} {
		if _, ok := out.Preset.Guitars[g]; !ok {
			t.Errorf("guitar %q missing from structured preset", g)
		}
	}
}

func TestIntegration_GetPresetErrors(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)
	seedPreset(t, h, "p-lead", "Session Lead")

	cases := []struct {
		name string
		args map[string]any
		want []string // substrings the error text must contain
		not  []string // substrings it must not contain
	}{
		{
			name: "bad format",
			args: map[string]any{"preset_id": "p-lead", "format": "xml"},
			want: []string{"markdown", "json"},
		},
		{
			name: "unknown id",
			args: map[string]any{"preset_id": "does-not-exist"},
			want: []string{"list_presets"},
			not:  []string{"object not found", "does-not-exist/", "presets/"},
		},
		{
			name: "path traversal",
			args: map[string]any{"preset_id": "../../etc/passwd"},
			want: []string{"preset_id"},
		},
		{
			name: "slash in id",
			args: map[string]any{"preset_id": "a/b"},
			want: []string{"preset_id"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := itCall(t, cs, getToolName, tc.args)
			text := itText(t, res)
			t.Logf("error text: %s", text)
			if !res.IsError {
				t.Fatalf("IsError = false, want true; text=%q", text)
			}
			for _, w := range tc.want {
				if !strings.Contains(text, w) {
					t.Errorf("error text missing %q: %q", w, text)
				}
			}
			for _, n := range tc.not {
				if strings.Contains(text, n) {
					t.Errorf("error text leaks %q: %q", n, text)
				}
			}
		})
	}
}

// ---- 5. save_preset ----

func TestIntegration_SavePreset(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)
	itSeedDraft(t, h, "p-draft")

	res := itCall(t, cs, saveToolName, map[string]any{"preset_id": "p-draft", "name": "Renamed Draft"})
	if res.IsError {
		t.Fatalf("save_preset failed: %s", itText(t, res))
	}
	t.Logf("save_preset: %s", itText(t, res))
	if so := itStructured[saveOutput](t, res); so.ID != "p-draft" || so.Name != "Renamed Draft" {
		t.Errorf("save structured = %+v", so)
	}

	got := itCall(t, cs, getToolName, map[string]any{"preset_id": "p-draft", "format": "json"})
	out := itStructured[getOutput](t, got)
	if out.Name != "Renamed Draft" || out.IsDraft {
		t.Errorf("get after save: name=%q is_draft=%v", out.Name, out.IsDraft)
	}
	md := itText(t, itCall(t, cs, getToolName, map[string]any{"preset_id": "p-draft"}))
	if !strings.Contains(md, "## Renamed Draft") {
		t.Errorf("markdown get does not show new name:\n%s", md)
	}

	list := itText(t, itCall(t, cs, listToolName, nil))
	t.Logf("list_presets after save:\n%s", list)
	line := itLineFor(list, "p-draft")
	if !strings.Contains(line, "Renamed Draft") || strings.Contains(line, "[draft]") {
		t.Errorf("list line after save = %q", line)
	}
}

func TestIntegration_SavePresetRejectedNames(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)
	seedPreset(t, h, "p-keep", "Keep Me")

	rejected := map[string]string{
		"empty":           "",
		"whitespace":      "   ",
		"reserved":        draftName,
		"reserved lower":  "draft preset",
		"reserved upper":  "DRAFT PRESET",
		"101 runes":       strings.Repeat("é", 101),
		"newline":         "line one\nline two",
		"heading newline": "x\n# injected",
	}
	for label, name := range rejected {
		t.Run(label, func(t *testing.T) {
			res := itCall(t, cs, saveToolName, map[string]any{"preset_id": "p-keep", "name": name})
			text := itText(t, res)
			t.Logf("reject %s: %s", label, text)
			if !res.IsError {
				t.Fatalf("name %q accepted; want rejection", name)
			}
		})
	}

	// None of the rejected names reached storage.
	p, err := h.store.Get(t.Context(), "p-keep")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Keep Me" {
		t.Errorf("stored name changed to %q by a rejected save", p.Name)
	}

	// Boundary: exactly 100 runes is accepted.
	ok := strings.Repeat("é", 100)
	res := itCall(t, cs, saveToolName, map[string]any{"preset_id": "p-keep", "name": ok})
	if res.IsError {
		t.Errorf("100-rune name rejected: %s", itText(t, res))
	}
}

func TestIntegration_SavePresetUnknownAndInvalidID(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)

	res := itCall(t, cs, saveToolName, map[string]any{"preset_id": "missing-id", "name": "Valid Name"})
	text := itText(t, res)
	t.Logf("unknown id: %s", text)
	if !res.IsError || !strings.Contains(text, "list_presets") {
		t.Errorf("unknown id: IsError=%v text=%q", res.IsError, text)
	}

	res = itCall(t, cs, saveToolName, map[string]any{"preset_id": "../x", "name": "Valid Name"})
	text = itText(t, res)
	t.Logf("invalid id: %s", text)
	if !res.IsError || !strings.Contains(text, "preset_id") {
		t.Errorf("invalid id: IsError=%v text=%q", res.IsError, text)
	}
}

// ---- 6. auth through a real client ----

func TestIntegration_AuthHeader(t *testing.T) {
	h := newTestService(t)
	ts := itServer(t, h.svc)
	endpoint := ts.URL + "/mcp"

	if _, err := itConnect(t, endpoint, http.DefaultClient); err == nil {
		t.Error("connect with no Authorization header succeeded; want failure")
	} else {
		t.Logf("no header: %v", err)
	}

	bad := &http.Client{Transport: itBearer{token: wrongToken, base: http.DefaultTransport}}
	if _, err := itConnect(t, endpoint, bad); err == nil {
		t.Error("connect with wrong token succeeded; want failure")
	} else {
		t.Logf("wrong token: %v", err)
	}
}

func TestIntegration_AuthPathToken(t *testing.T) {
	h := newTestService(t)
	ts := itServer(t, h.svc)

	cs, err := itConnect(t, ts.URL+"/mcp/"+testToken, http.DefaultClient)
	if err != nil {
		t.Fatalf("path-token connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.ListTools(itCtx(t), nil)
	if err != nil {
		t.Fatalf("ListTools over path token: %v", err)
	}
	if len(res.Tools) != 3 {
		t.Errorf("path-token session sees %d tools, want 3", len(res.Tools))
	}
}

// ---- 7. hostile content ----

func TestIntegration_HostileName(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)
	const hostile = "x\n# injected <script>"
	seedPreset(t, h, "p-evil", hostile)

	list := itText(t, itCall(t, cs, listToolName, nil))
	t.Logf("list_presets with hostile name:\n%s", list)
	checkNoInjection(t, "list_presets", list)

	get := itText(t, itCall(t, cs, getToolName, map[string]any{"preset_id": "p-evil"}))
	t.Logf("get_preset with hostile name:\n%s", get)
	checkNoInjection(t, "get_preset", get)
}

func checkNoInjection(t *testing.T, tool, text string) {
	t.Helper()
	if strings.Contains(text, "\n# injected") {
		t.Errorf("%s: raw newline-heading from name", tool)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "# injected") {
			t.Errorf("%s: name opened a heading line: %q", tool, line)
		}
	}
	if strings.Contains(text, "<script") {
		t.Errorf("%s: raw '<' from name", tool)
	}
	if !strings.Contains(text, "&lt;script&gt;") {
		t.Errorf("%s: expected escaped name in output", tool)
	}
}

// ---- transcript: all tools in one session, every text result logged ----

func TestMCPEndToEnd(t *testing.T) {
	h := newTestService(t)
	cs := newMCPSession(t, h.svc)

	t.Logf("== initialize: instructions=%q", cs.InitializeResult().Instructions)

	tools, err := cs.ListTools(itCtx(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		t.Logf("== tools/list: %s readOnly=%v idempotent=%v", tool.Name,
			tool.Annotations.ReadOnlyHint, tool.Annotations.IdempotentHint)
	}

	seedPreset(t, h, "p-2-alpha", "Alpha Preset")
	itSeedDraft(t, h, "p-1-draft")

	t.Logf("== list_presets {}\n%s", itText(t, itCall(t, cs, listToolName, nil)))

	t.Logf("== get_preset {p-2-alpha}\n%s", itText(t, itCall(t, cs, getToolName, map[string]any{"preset_id": "p-2-alpha"})))

	res := itCall(t, cs, saveToolName, map[string]any{"preset_id": "p-1-draft", "name": "Gain Staging Lead"})
	t.Logf("== save_preset {p-1-draft, Gain Staging Lead}\n%s (isError=%v)", itText(t, res), res.IsError)

	t.Logf("== list_presets {include_drafts:false}\n%s",
		itText(t, itCall(t, cs, listToolName, map[string]any{"include_drafts": false})))

	res = itCall(t, cs, getToolName, map[string]any{"preset_id": "p-1-draft", "format": "xml"})
	t.Logf("== get_preset {p-1-draft, format:xml}\n%s (isError=%v)", itText(t, res), res.IsError)

	res = itCall(t, cs, getToolName, map[string]any{"preset_id": "p-1-draft"})
	t.Logf("== get_preset {p-1-draft}\n%s (isError=%v)", itText(t, res), res.IsError)
}
