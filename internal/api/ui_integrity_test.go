package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestUIRuleDeletionTarget(t *testing.T) {
	// 1. Setup mock storage with sample rules
	mc := newMockClient()
	
	ruleJSON := `{"id":"test-rule-123","artist":"Slash","critique":"too harsh","action":"less treble"}`
	mc.WriteFile(context.Background(), "test-bucket", "memories/test-rule-123.json", []byte(ruleJSON))

	memStore := storage.NewMemoryStore(mc, "test-bucket")

	server := &Server{
		memoryStore: memStore,
	}

	// 2. Hit the endpoint
	req := httptest.NewRequest("GET", "/api/memories", nil)
	rr := httptest.NewRecorder()

	handler := server.handleGetMemories()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	// 3. Verify it contains the EXPECTED target
	if !strings.Contains(body, `hx-target="#rules-list-container"`) {
		t.Errorf("UI Regression: Response missing hx-target=\"#rules-list-container\"")
	}

	// 4. Verify it does NOT contain the BAD target (sidebar regression)
	if strings.Contains(body, `hx-target="#sidebar-content"`) {
		t.Errorf("UI Error Detected: Response mistakenly contains hx-target=\"#sidebar-content\"")
	}

	// 5. Audit all hx-targets via Regex to prevent stray targets
	targetRegex := regexp.MustCompile(`hx-target="([^"]+)"`)
	matches := targetRegex.FindAllStringSubmatch(body, -1)

	for _, match := range matches {
		target := match[1]
		if target != "#rules-list-container" {
			t.Errorf("UI Warning: Found unexpected hx-target definition: %s", target)
		}
	}
}

func TestUIGlobalTemplateIntegrity(t *testing.T) {
	// Need to import "os"
	var htmlData []byte
	var err error
	
	// Running tests from CWD of internal/api gives us ../../web/templates/index.html
	htmlData, err = os.ReadFile("../../web/templates/index.html")
	if err != nil {
		t.Fatalf("Failed to read templates: %v", err)
	}
	body := string(htmlData)

	idRegex := regexp.MustCompile(`\sid="([^"]+)"`)
	idMatches := idRegex.FindAllStringSubmatch(body, -1)

	definedIDs := make(map[string]bool)
	for _, match := range idMatches {
		definedIDs[match[1]] = true
	}

	targetRegex := regexp.MustCompile(`\shx-target="#([^"]+)"`)
	targetMatches := targetRegex.FindAllStringSubmatch(body, -1)

	for _, match := range targetMatches {
		target := match[1]
		if !definedIDs[target] {
			t.Errorf("UI Regression: hx-target=\"#%s\" points to a non-existent container in index.html", target)
		}
	}
}

func TestUILibraryIntegrity(t *testing.T) {
	mc := newMockClient()
	
	p := &storage.Preset{ID: "preset-123", Name: "Fender Clean", UpdatedAt: "2026-03-31T20:00:00Z"}
	pData, _ := json.Marshal(p)
	mc.WriteFile(context.Background(), presetBucketName, "presets/preset-123.json", pData)

	store := storage.NewPresetStore(mc, presetBucketName)
	server := &Server{
		store: store,
	}

	req := httptest.NewRequest("GET", "/api/presets", nil)
	rr := httptest.NewRecorder()

	handler := server.handleGetPresets()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	htmlData, _ := os.ReadFile("../../web/templates/index.html")
	bodyIndex := string(htmlData)
	idRegex := regexp.MustCompile(`\sid="([^"]+)"`)
	idMatches := idRegex.FindAllStringSubmatch(bodyIndex, -1)
	definedIDs := make(map[string]bool)
	for _, match := range idMatches {
		definedIDs[match[1]] = true
	}

	targetRegex := regexp.MustCompile(`\shx-target="#([^"]+)"`)
	matches := targetRegex.FindAllStringSubmatch(body, -1)

	for _, match := range matches {
		target := match[1]
		if !definedIDs[target] {
			t.Errorf("Library Regression: hx-target=\"#%s\" emitted by API points to a non-existent UI container", target)
		}
	}
}

func TestUIGoldenTweakingWorkspace(t *testing.T) {
	p := &storage.Preset{
		ID:   "test-preset",
		Name: "Test Preset",
		Payload: `{"guitars":{"Les Paul":[{"id":"b1","type":"drive","model":"Overdrive","parameters":[{"name":"Gain","value":"5.0","type":"slider","unit":""}]}]}}`,
	}

	// Read-Only mode
	htmlReadOnly := renderTweakingWorkspaceHTML(p, false, true)
	verifyGolden(t, "workspace_readonly", htmlReadOnly)

	// Edit mode (Note: renderTweakingWorkspaceHTML now determines edit mode based on p.Name or payload, so passing false for isCopyMode still applies)
	htmlEdit := renderTweakingWorkspaceHTML(p, false, false)
	verifyGolden(t, "workspace_edit", htmlEdit)
}

func TestUIGoldenPresetList(t *testing.T) {
	presets := []*storage.Preset{
		{ID: "preset-aaa", Name: "Texas Flood - SRV", UpdatedAt: "2026-07-11T12:45:09Z"},
		{ID: "preset-bbb", Name: "1940s muddy waters", UpdatedAt: "2026-07-09T23:11:30Z"},
	}
	html := renderPresetList(presets, false)
	verifyGolden(t, "preset_list", html)
}

func TestRenderPresetListEmpty(t *testing.T) {
	html := renderPresetList(nil, false)
	if !strings.Contains(html, "No presets saved yet.") {
		t.Errorf("expected empty-state message, got: %s", html)
	}
}

// TestUIWiringContract pins the hx-target/id and form-field-name contracts that
// JS and the mobile UX redesign depend on, so a pure visual restyle (checkbox ->
// toggle switch, top nav -> bottom tab bar, etc.) can't silently break wiring.
func TestUIWiringContract(t *testing.T) {
	htmlData, err := os.ReadFile("../../web/templates/index.html")
	if err != nil {
		t.Fatalf("Failed to read templates: %v", err)
	}
	body := string(htmlData)

	requiredIDs := []string{
		"gen-workspace-wrapper",
		"library-editor-workspace",
		"library-list-container",
		"gen-progress-area",
		"rules-list-container",
	}
	for _, id := range requiredIDs {
		if !strings.Contains(body, `id="`+id+`"`) {
			t.Errorf("UI Wiring Regression: expected id=%q in index.html, not found", id)
		}
	}

	requiredCheckboxNames := []string{
		"allow_factory_captures",
		"allow_paid_plugins",
		"favor_captures",
		"allow_user_captures",
		"favor_cloud_captures",
	}
	for _, name := range requiredCheckboxNames {
		if !strings.Contains(body, `name="`+name+`"`) {
			t.Errorf("UI Wiring Regression: expected form field name=%q in index.html, not found", name)
		}
	}
}

func verifyGolden(t *testing.T, name string, got string) {
	t.Helper()
	goldenFile := "testdata/" + name + ".golden"

	if os.Getenv("UPDATE_GOLDEN") == "true" {
		os.MkdirAll("testdata", 0755)
		err := os.WriteFile(goldenFile, []byte(got), 0644)
		if err != nil {
			t.Fatalf("Failed to update golden file: %v", err)
		}
		t.Logf("Updated golden file %s", goldenFile)
		return
	}

	wantBytes, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("Failed to read golden file %s: %v. Run with UPDATE_GOLDEN=true to create it.", goldenFile, err)
	}
	want := string(wantBytes)

	if got != want {
		t.Errorf("Golden mismatch for %s. Run with UPDATE_GOLDEN=true to update if this change is intended.", name)
	}
}
