package devicelib

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/catalog"
	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicejobs/devicejobstest"
)

func lib(cs ...Capture) *Library { return &Library{Format: LibraryFormat, Captures: cs} }

func cp(name, kind string, version int, cloud bool, keyByte string) Capture {
	return Capture{Name: name, Key: strings.Repeat(keyByte, 64), Version: version, Kind: kind, Folder: "F", Cloud: cloud}
}

func TestResolve(t *testing.T) {
	al, err := NewAliases([]Alias{{Name: "CA John's", Source: "factory", UnitName: "CA John's 2", Evidence: "the unit's own screen", Reviewed: "2026-11-06"}})
	if err != nil {
		t.Fatal(err)
	}
	l := lib(
		cp("CA John's 2", "factory", 2, false, "b"),
		cp("Brit 2203 87", "factory", 1, false, "d"),
		cp("Mine", "user", 1, false, "c"),
		cp("Cloudy", "user", 1, true, "e"),
		cp("Twice", "user", 1, false, "1"), cp("Twice", "user", 1, false, "2"),
		cp("Same Key", "user", 1, false, "3"), cp("Same Key", "user", 1, false, "3"),
		cp("Unversioned", "user", 0, false, "4"),
		cp("Mixed", "user", 1, false, "5"),
	)
	cases := []struct {
		name, source string
		aliases      *Aliases
		want         Status
		unit         string
	}{
		{"Brit 2203 87", "factory", nil, StatusOnUnit, "Brit 2203 87"},
		{"CA John's 2", "factory", nil, StatusOnUnit, "CA John's 2"},
		{"Mine", "user", nil, StatusOnUnit, "Mine"},
		{"Same Key", "user", nil, StatusOnUnit, "Same Key"}, // one key listed twice is one capture
		{"CA John's", "factory", nil, StatusNotOnUnit, ""},  // no alias table: no match
		{"CA John's", "factory", al, StatusOnUnit, "CA John's 2"},
		{"CA John's", "user", al, StatusNotOnUnit, ""},       // the alias is for the factory capture only
		{"ca john's 2", "factory", nil, StatusNotOnUnit, ""}, // never case-insensitive
		{"CA John's 2 ", "factory", nil, StatusNotOnUnit, ""},
		{"Brit 2203", "factory", nil, StatusNotOnUnit, ""}, // never a prefix
		{"Mine", "factory", nil, StatusWrongKind, ""},
		{"Brit 2203 87", "user", nil, StatusWrongKind, ""},
		{"Cloudy", "user", nil, StatusCloudOnly, ""},
		{"Twice", "user", nil, StatusAmbiguous, ""},
		{"Unversioned", "user", nil, StatusVersionUnknown, ""},
	}
	for _, c := range cases {
		r := Resolve(l, c.aliases, c.name, c.source)
		if r.Status != c.want || r.UnitName != c.unit {
			t.Errorf("Resolve(%q,%s) = %s %q (%s), want %s %q", c.name, c.source, r.Status, r.UnitName, r.Reason, c.want, c.unit)
		}
		if r.Status != StatusOnUnit && r.Reason == "" {
			t.Errorf("Resolve(%q): no reason", c.name)
		}
	}
	if r := Resolve(l, al, "CA John's", "factory"); !r.Aliased || r.Version != 2 || r.Key != strings.Repeat("b", 64) {
		t.Errorf("alias resolution: %+v", r)
	}
	// An alias never makes a capture appear: its target must be in the list.
	if r := Resolve(lib(cp("Other", "factory", 1, false, "a")), al, "CA John's", "factory"); r.Status != StatusNotOnUnit {
		t.Errorf("alias to a capture that is not on the unit resolved: %+v", r)
	}
	// An exact match wins over an alias.
	both := lib(cp("CA John's", "factory", 1, false, "a"), cp("CA John's 2", "factory", 2, false, "b"))
	if r := Resolve(both, al, "CA John's", "factory"); r.UnitName != "CA John's" || r.Aliased {
		t.Errorf("exact match did not win: %+v", r)
	}
}

func TestAliasTable(t *testing.T) {
	a, err := EmbeddedAliases()
	if err != nil {
		t.Fatal(err)
	}
	if a.Len() != 0 {
		t.Errorf("the alias table starts empty (filled from the first real sync), has %d", a.Len())
	}
	good := Alias{Name: "A", Source: "factory", UnitName: "B", Evidence: "seen", Reviewed: "2026-11-06"}
	for name, mod := range map[string]func(*Alias){
		"no evidence": func(a *Alias) { a.Evidence = " " }, "no date": func(a *Alias) { a.Reviewed = "" },
		"self alias": func(a *Alias) { a.UnitName = "A" }, "bad source": func(a *Alias) { a.Source = "cloud" },
		"non-ASCII": func(a *Alias) { a.Name = "Caf\u00e9" }, "padded": func(a *Alias) { a.Name = " A" },
		"empty": func(a *Alias) { a.UnitName = "" },
	} {
		c := good
		mod(&c)
		if _, err := NewAliases([]Alias{c}); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := NewAliases([]Alias{good, good}); err == nil {
		t.Error("duplicate accepted")
	}
	if _, err := NewAliases([]Alias{good}); err != nil {
		t.Error(err)
	}
}

func TestNewLibraryAndStore(t *testing.T) {
	v1 := 1
	p := &devicejobs.LibraryPayload{Skipped: 2, Captures: []devicejobs.LibraryCapture{
		{Name: "A", Key: strings.Repeat("a", 64), Version: &v1, Kind: "factory", Folder: "Factory Captures V1"},
		{Name: "B", Key: strings.Repeat("b", 64), Kind: "user", Folder: "Mine", Cloud: true},
	}}
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	l := NewLibrary(p, "6f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e01", "4.1.0\x00", at)
	if l.SyncedAt != "2026-11-06T12:00:00Z" || l.Skipped != 2 || len(l.Captures) != 2 || l.Firmware != "4.1.0" {
		t.Fatalf("%+v", l)
	}
	if c := l.Count(); c.Total != 2 || c.Factory != 1 || c.User != 1 || c.V1 != 1 || c.UnknownVersion != 1 || c.Cloud != 1 {
		t.Errorf("counts %+v", c)
	}
	mem := devicejobstest.NewMemClient()
	s := NewStore(mem, "main")
	ctx := context.Background()
	if got, err := s.Library(ctx); err != nil || got != nil {
		t.Fatalf("empty store: %v %v", got, err)
	}
	if err := s.SaveLibrary(ctx, l); err != nil {
		t.Fatal(err)
	}
	got, err := s.Library(ctx)
	if err != nil || got == nil || len(got.Captures) != 2 || got.JobID != l.JobID {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	// a stored file of another format is refused, not trusted
	if err := mem.WriteFile(ctx, "main", LibraryObject, []byte(`{"format":"other/9","captures":[]}`)); err != nil {
		t.Fatalf("test setup failed: %v", err)
	}
	if _, err := s.Library(ctx); err == nil {
		t.Error("unknown format accepted")
	}
	// a storage failure is an error, not "not synced"
	mem.SetFailList(true)
	if _, err := s.Library(ctx); err == nil {
		t.Error("a listing failure was read as 'no list'")
	}
}

func TestAddMics(t *testing.T) {
	mem := devicejobstest.NewMemClient()
	s := NewStore(mem, "main")
	ctx := context.Background()
	at := time.Now()
	n, err := s.AddMics(ctx, map[int][]string{12045: {"ME_110 US PRN C10R_Ribbon 121", "ME_110 US PRN C10R_Dynamic 57"}}, at)
	if err != nil || n != 2 {
		t.Fatalf("%d %v", n, err)
	}
	if n, err := s.AddMics(ctx, map[int][]string{12045: {"ME_110 US PRN C10R_Ribbon 121"}}, at); err != nil || n != 0 {
		t.Errorf("a repeat recorded %d, err %v (want 0, nil)", n, err)
	}
	n, err = s.AddMics(ctx, map[int][]string{12045: {"ME_110 US PRN C10R_Condenser 414"}, 12046: {"x"}}, at)
	m, merr := s.Mics(ctx)
	if err != nil || merr != nil || n != 2 || len(m.For(12045)) != 3 || m.Total() != 4 {
		t.Errorf("union: %d %+v", n, m)
	}
	// bounded
	big := map[int][]string{}
	for i := 0; i < MaxMicCabs+50; i++ {
		big[100000+i] = []string{"a"}
	}
	_, _ = s.AddMics(ctx, big, at)
	m, _ = s.Mics(ctx)
	if len(m.Cabs) > MaxMicCabs {
		t.Errorf("%d cabs stored, limit %d", len(m.Cabs), MaxMicCabs)
	}
	var per []string
	for i := 0; i < MaxMicsPerCab+10; i++ {
		per = append(per, "m"+strings.Repeat("x", i))
	}
	_, _ = s.AddMics(ctx, map[int][]string{12045: per}, at)
	m, _ = s.Mics(ctx)
	if len(m.For(12045)) > MaxMicsPerCab {
		t.Errorf("%d mics for one cab, limit %d", len(m.For(12045)), MaxMicsPerCab)
	}
	if nilM := (*Mics)(nil); nilM.For(1) != nil || nilM.Total() != 0 {
		t.Error("nil Mics")
	}
}

func readback(t *testing.T, modelID int, idx int, values []any) *devicejobs.ReadbackPayload {
	t.Helper()
	vs, _ := json.Marshal(values)
	raw := `{"setlist":{"folder":"other","position":null},"chains":[{"row":0,"in_port":1,"out_port":19,"blocks":[{"col":2,"model_id":` +
		itoa(modelID) + `,"params":[{"index":` + itoa(idx) + `,"scene_mode":false,"values":` + string(vs) + `}]}]}],"bypass":null}`
	p, err := devicejobs.ParseReadbackPayload(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func eight(v any) []any { return []any{v, v, v, v, v, v, v, v} }

func TestHarvest(t *testing.T) {
	cat, err := catalog.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	h := NewHarvester(cat)
	const cab, sel = 12045, 1 // 110 US PRN C10R (M): ir selector at 1 and 9, default ME_110 US PRN C10R_Dynamic 57
	ok := "ME_110 US PRN C10R_Ribbon 121"
	seen, ign := h.Harvest(readback(t, cab, sel, eight(ok)))
	if len(seen[cab]) != 1 || seen[cab][0] != ok || ign != 0 {
		t.Fatalf("%v %d", seen, ign)
	}
	bad := map[string][]any{
		"another cab's IR":    eight("ME_212 Parallax Cab_Dynamic 57"),
		"a path":              eight(`C:\IRs\x.wav`),
		"no mic":              eight("ME_110 US PRN C10R_"),
		"a backtick":          eight("ME_110 US PRN C10R_Rib`bon"),
		"non-ASCII":           eight("ME_110 US PRN C10R_Rib\u00e9"),
		"a control character": eight("ME_110 US PRN C10R_Rib\u0007bon"),
		"a zero-width space":  eight("ME_110 US PRN C10R_Rib\u200bbon"),
		"too long":            eight("ME_110 US PRN C10R_" + strings.Repeat("x", 150)),
		"leading space":       eight(" ME_110 US PRN C10R_Ribbon 121"),
		"scenes that differ":  {ok, "ME_110 US PRN C10R_Dynamic 57", ok, ok, ok, ok, ok, ok},
		"a number":            eight(0.5),
		"a trailing note":     eight("ME_110 US PRN C10R_Ribbon 121 (junk)"),
		"a semicolon":         eight("ME_110 US PRN C10R_Ribbon 121;x"),
	}
	for name, v := range bad {
		seen, ign := h.Harvest(readback(t, cab, sel, v))
		if len(seen) != 0 {
			t.Errorf("%s: recorded %v", name, seen)
		}
		if _, isNum := v[0].(float64); !isNum && ign != 1 {
			t.Errorf("%s: ignored %d, want 1", name, ign)
		}
	}
	// a string at an index that is not an ir selector, and a model that is not a cab
	if seen, _ := h.Harvest(readback(t, cab, 2, eight(ok))); len(seen) != 0 {
		t.Errorf("a non-selector index: %v", seen)
	}
	if seen, _ := h.Harvest(readback(t, 1001, sel, eight(ok))); len(seen) != 0 {
		t.Errorf("a non-cab: %v", seen)
	}
	if seen, ign := h.Harvest(nil); len(seen) != 0 || ign != 0 {
		t.Error("nil payload")
	}
	if !h.ValidMic(cab, ok) || h.ValidMic(cab, "ME_212 Parallax Cab_Dynamic 57") || h.ValidMic(1001, ok) {
		t.Error("ValidMic")
	}
}

func TestStoredLibraryIsRevalidatedOnRead(t *testing.T) {
	mem := devicejobstest.NewMemClient()
	s := NewStore(mem, "main")
	ctx := context.Background()
	k := strings.Repeat("a", 64)
	for name, body := range map[string]string{
		"markdown folder": `{"format":"qc2-device-library/1","synced_at":"2026-11-06T12:00:00Z","captures":[{"name":"A","key":"` + k + `","version":1,"kind":"user","folder":"[x](https://evil.example)‮","cloud":false}]}`,
		"bad timestamp":   `{"format":"qc2-device-library/1","synced_at":"later","captures":[]}`,
		"bad version":     `{"format":"qc2-device-library/1","synced_at":"2026-11-06T12:00:00Z","captures":[{"name":"A","key":"` + k + `","version":7,"kind":"user","folder":"F","cloud":false}]}`,
	} {
		if err := mem.WriteFile(ctx, "main", LibraryObject, []byte(body)); err != nil {
			t.Fatalf("%s: test setup failed: %v", name, err)
		}
		if _, err := s.Library(ctx); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if err := mem.WriteFile(ctx, "main", LibraryObject, []byte(`{"format":"qc2-device-library/1","synced_at":"2026-11-06T12:00:00Z","captures":[{"name":"A","key":"`+k+`","version":0,"kind":"user","folder":"F","cloud":true}]}`)); err != nil {
		t.Fatalf("test setup failed: %v", err)
	}
	if l, err := s.Library(ctx); err != nil || len(l.Captures) != 1 {
		t.Errorf("a valid list was refused: %v", err)
	}
}

func TestAddMics_StoredFileWithoutCabsDoesNotPanic(t *testing.T) {
	mem := devicejobstest.NewMemClient()
	s := NewStore(mem, "main")
	ctx := context.Background()
	if err := mem.WriteFile(ctx, "main", MicsObject, []byte(`{"format":"qc2-cab-mics/1"}`)); err != nil {
		t.Fatal(err)
	}
	n, err := s.AddMics(ctx, map[int][]string{12045: {"ME_110 US PRN C10R_Ribbon 121"}}, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("n %d err %v", n, err)
	}
}
