package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func deviceExportServer(t *testing.T, guitars map[string][]storage.EffectBlock) *Server {
	t.Helper()
	client := newMockClient()
	store := storage.NewPresetStore(client, "b")
	s := NewServer(store, nil, client, &mockSecretFetcher{}, nil, &config.AppConfig{})
	raw, err := json.Marshal(storage.StructuredPreset{Guitars: guitars})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), &storage.Preset{ID: "p-dl", Name: "DL", Payload: string(raw)}); err != nil {
		t.Fatal(err)
	}
	return s
}

func dlChain() []storage.EffectBlock {
	return []storage.EffectBlock{{Model: "Myth Drive", Parameters: []storage.BlockParameter{{Name: "Gain", Value: "3", Basis: "estimate"}}}}
}

func getDL(s *Server, query string, authed bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/preset/device_export?"+query, nil)
	if authed {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: generateCookieValue("mock-secret")})
	}
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	return rr
}

func TestDeviceExportDownload(t *testing.T) {
	const hostile = "Tele \"evil\"\r\nX-Injected: 1; <script>"
	s := deviceExportServer(t, map[string][]storage.EffectBlock{hostile: dlChain(), "ES-339": dlChain()})

	rr := getDL(s, "id=p-dl&guitar="+url.QueryEscape(hostile), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type %q", ct)
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff")
	}
	cd := rr.Header().Get("Content-Disposition")
	if !regexp.MustCompile(`^attachment; filename="qc2-device-[a-z0-9-]+\.json"$`).MatchString(cd) {
		t.Errorf("Content-Disposition %q", cd)
	}
	if rr.Header().Get("X-Injected") != "" {
		t.Error("guitar name injected a header")
	}
	var p map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if p["format"] != "qc2-device-export/1" || p["guitar"] != hostile {
		t.Errorf("payload format %v guitar %v", p["format"], p["guitar"])
	}
	if strings.Contains(rr.Body.String(), "<script>") {
		t.Error("raw '<' in the download")
	}
}

func TestDeviceExportDownloadErrors(t *testing.T) {
	s := deviceExportServer(t, map[string][]storage.EffectBlock{"Tele": dlChain(), "ES-339": dlChain()})
	cases := []struct {
		query  string
		authed bool
		code   int
	}{
		{"id=p-dl&guitar=Tele", false, http.StatusFound}, // login redirect
		{"guitar=Tele", true, http.StatusBadRequest},
		{"id=nope&guitar=Tele", true, http.StatusNotFound},
		{"id=p-dl&guitar=Strat", true, http.StatusNotFound},
		{"id=p-dl", true, http.StatusBadRequest}, // two guitars, none chosen
	}
	for _, c := range cases {
		if rr := getDL(s, c.query, c.authed); rr.Code != c.code {
			t.Errorf("%s (authed %v): status %d, want %d", c.query, c.authed, rr.Code, c.code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/preset/device_export?id=p-dl&guitar=Tele", nil)
	withAuthAndCSRF(req)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d", rr.Code)
	}
	// One guitar: guitar may be omitted.
	one := deviceExportServer(t, map[string][]storage.EffectBlock{"Tele": dlChain()})
	if rr := getDL(one, "id=p-dl", true); rr.Code != http.StatusOK {
		t.Errorf("single guitar without guitar=: %d", rr.Code)
	}
}

func TestDeviceExportLinksHTML(t *testing.T) {
	out := deviceExportLinksHTML("p-1", []string{"B <b>", "A&A"})
	if strings.Contains(out, "<b>") {
		t.Errorf("guitar name not escaped: %s", out)
	}
	if !strings.Contains(out, `href="/api/preset/device_export?id=p-1&amp;guitar=A%26A"`) ||
		strings.Index(out, "A&amp;A") > strings.Index(out, "B &lt;b&gt;") {
		t.Errorf("links: %s", out)
	}
	if deviceExportLinksHTML("p-1", nil) != "" {
		t.Error("no guitars should render nothing")
	}
}

func TestWorkspaceShowsDeviceExportLinks(t *testing.T) {
	raw, _ := json.Marshal(storage.StructuredPreset{Guitars: map[string][]storage.EffectBlock{"Tele": dlChain()}})
	p := &storage.Preset{ID: "p-1", Name: "Saved", Payload: string(raw)}
	if out := renderTweakingWorkspaceHTML(p, false, false); !strings.Contains(out, "Download device JSON (Tele)") {
		t.Error("edit view lacks the download link")
	}
	if out := renderTweakingWorkspaceHTML(p, true, false); strings.Contains(out, "Download device JSON") {
		t.Error("duplicate view should not offer the download")
	}
}
