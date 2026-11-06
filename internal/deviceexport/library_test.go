package deviceexport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
	"github.com/weitzer-org/sound-builder/internal/devicelib"
)

// The synced capture list and the seen cab mics (plan P4b) change the dry run and, only when
// a mic is written exactly or an alias maps a name, the payload. Without them nothing changes.

func libExporter(t *testing.T, snap *devicelib.Snapshot, err error) *Exporter {
	t.Helper()
	e, nerr := NewEmbedded(Options{
		CaptureSource: func(n string) string { return testCaptures[n] },
		Device:        func(context.Context) (*devicelib.Snapshot, error) { return snap, err },
	})
	if nerr != nil {
		t.Fatal(nerr)
	}
	return e
}

func lcap(name, kind string, version int, cloud bool, k string) devicelib.Capture {
	return devicelib.Capture{Name: name, Key: strings.Repeat(k, 64), Version: version, Kind: kind, Folder: "Factory Captures V2", Cloud: cloud}
}

func TestSyncedList_CaptureOnTheUnit(t *testing.T) {
	sp := preset(block("CA John's", prm("Gain", "0 dB", "+3 dB")))
	snap := &devicelib.Snapshot{Library: &devicelib.Library{Format: devicelib.LibraryFormat, SyncedAt: "2026-11-06T12:00:00Z",
		Captures: []devicelib.Capture{lcap("CA John's", "factory", 2, false, "b")}}}
	with, wp := export(t, libExporter(t, snap, nil), sp, "g")
	without, np := export(t, testExporter(t), sp, "g")
	if len(with.Unresolved) != 0 {
		t.Fatalf("unresolved: %+v", with.Unresolved)
	}
	// Resolving against the list does not change what is pushed: the agent re-resolves on the unit.
	if !bytes.Equal(with.Payload, without.Payload) {
		t.Errorf("payload changed by a synced list:\n%s\n%s", with.Payload, without.Payload)
	}
	if wp.Rows[0].Blocks[0].Capture.ResolvedOnUnit || np.Rows[0].Blocks[0].Capture.ResolvedOnUnit {
		t.Error("resolved_on_unit must stay false: the agent resolves captures itself")
	}
	ci := with.Summary[0].CaptureInfo
	if ci == nil || ci.UnitName != "CA John's" || ci.Version != 2 || ci.Kind != "factory" || ci.KeyPrefix != "bbbbbbbb" || ci.SyncedAt != "2026-11-06T12:00:00Z" || ci.AskedName != "" {
		t.Errorf("capture info: %+v", ci)
	}
	if without.Summary[0].CaptureInfo != nil {
		t.Error("no synced list: no capture info")
	}
}

func TestSyncedList_NotOnTheUnitIsUnresolved(t *testing.T) {
	sp := preset(block("CA John's", prm("Gain", "0 dB", "")))
	for name, caps := range map[string][]devicelib.Capture{
		"empty list":    nil,
		"other names":   {lcap("Brit 2203 87", "factory", 1, false, "d")},
		"near miss":     {lcap("ca john's", "factory", 2, false, "b"), lcap("CA John's 2", "factory", 2, false, "b")},
		"wrong kind":    {lcap("CA John's", "user", 1, false, "b")},
		"cloud only":    {lcap("CA John's", "factory", 2, true, "b")},
		"ambiguous":     {lcap("CA John's", "factory", 2, false, "b"), lcap("CA John's", "factory", 2, false, "c")},
		"unknown V1/V2": {lcap("CA John's", "factory", 0, false, "b")},
	} {
		snap := &devicelib.Snapshot{Library: &devicelib.Library{Format: devicelib.LibraryFormat, Captures: caps}}
		ex, p := export(t, libExporter(t, snap, nil), sp, "g")
		if len(ex.Unresolved) != 1 || len(p.Rows[0].Blocks) != 0 {
			t.Errorf("%s: unresolved %+v, blocks %d", name, ex.Unresolved, len(p.Rows[0].Blocks))
			continue
		}
		if name == "empty list" || name == "other names" || name == "near miss" {
			if !strings.Contains(ex.Unresolved[0].Reason, "not on your unit") {
				t.Errorf("%s: reason %q", name, ex.Unresolved[0].Reason)
			}
		}
	}
}

func TestSyncedList_ReviewedAliasWritesTheUnitsName(t *testing.T) {
	al, err := devicelib.NewAliases([]devicelib.Alias{{Name: "CA John's", Source: "factory", UnitName: "CA John's 2", Evidence: "screen", Reviewed: "2026-11-06"}})
	if err != nil {
		t.Fatal(err)
	}
	snap := &devicelib.Snapshot{Aliases: al, Library: &devicelib.Library{Format: devicelib.LibraryFormat, SyncedAt: "2026-11-06T12:00:00Z",
		Captures: []devicelib.Capture{lcap("CA John's 2", "factory", 2, false, "b")}}}
	ex, p := export(t, libExporter(t, snap, nil), preset(block("CA John's", prm("Gain", "0 dB", ""))), "g")
	if len(ex.Unresolved) != 0 {
		t.Fatalf("%+v", ex.Unresolved)
	}
	b := p.Rows[0].Blocks[0]
	if b.Capture.Name != "CA John's 2" || b.Model != "CA John's 2" {
		t.Errorf("the unit's name must be pushed (the validator wants model == capture.name): %+v", b)
	}
	if ci := ex.Summary[0].CaptureInfo; ci == nil || ci.AskedName != "CA John's" || ci.UnitName != "CA John's 2" {
		t.Errorf("capture info: %+v", ci)
	}
}

func TestSyncedList_ReadErrorIsAWarningNotSilence(t *testing.T) {
	ex, _ := export(t, libExporter(t, nil, errors.New("storage down")), preset(block("CA John's", prm("Gain", "0 dB", ""))), "g")
	if len(ex.Unresolved) != 0 || len(ex.Warnings) != 1 || !strings.Contains(ex.Warnings[0].Reason, "could not be read") {
		t.Errorf("unresolved %+v warnings %+v", ex.Unresolved, ex.Warnings)
	}
	if ex.Summary[0].CaptureInfo != nil {
		t.Error("an unreadable list must behave as not synced")
	}
}

const cabName = "110 US PRN C10R (M)" // model 12045: ir selectors 1 and 9, defaults ..._Dynamic 57 and ..._Ribbon 160

func micSnap(strs ...string) *devicelib.Snapshot {
	return &devicelib.Snapshot{Mics: &devicelib.Mics{Format: devicelib.MicsFormat, Cabs: map[string][]string{"12045": strs}}}
}

func TestSeenMic_IsWrittenExactly(t *testing.T) {
	sp := preset(block(cabName, prm("Mic 1", "Ribbon 121", "")))
	seen := "ME_110 US PRN C10R_Ribbon 121"
	ex, p := export(t, libExporter(t, micSnap(seen, "ME_110 US PRN C10R_Condenser 414"), nil), sp, "g")
	if len(ex.Unresolved) != 0 || len(ex.Warnings) != 0 {
		t.Fatalf("unresolved %+v warnings %+v", ex.Unresolved, ex.Warnings)
	}
	b := p.Rows[0].Blocks[0]
	if len(b.Mics) != 1 || b.Mics[0] != (Mic{Slot: 1, Index: 1, Value: seen, Evidence: MicEvidence}) {
		t.Fatalf("mics: %+v", b.Mics)
	}
	if got := ex.Summary[0].Mics; len(got) != 1 || got[0].Label != "MIC 1" || got[0].Value != seen {
		t.Errorf("plan mics: %+v", got)
	}
}

func TestSeenMic_NotSeenIsTheOldWarning(t *testing.T) {
	sp := preset(block(cabName, prm("Mic 1", "Ribbon 121", "")))
	for name, snap := range map[string]*devicelib.Snapshot{
		"no list at all":                           nil,
		"a list for another cab":                   {Mics: &devicelib.Mics{Format: devicelib.MicsFormat, Cabs: map[string][]string{"12046": {"ME_110 US PRN C10R_Ribbon 121"}}}},
		"a different mic":                          micSnap("ME_110 US PRN C10R_Condenser 414"),
		"a string of another cab (tampered store)": micSnap("ME_212 Parallax Cab_Ribbon 121"),
		"a path (tampered store)":                  micSnap(`C:\x_Ribbon 121`),
	} {
		e := testExporter(t)
		if snap != nil {
			e = libExporter(t, snap, nil)
		}
		ex, p := export(t, e, sp, "g")
		if len(ex.Unresolved) != 0 || len(ex.Warnings) != 1 || len(p.Rows[0].Blocks[0].Mics) != 0 {
			t.Errorf("%s: unresolved %+v warnings %+v mics %+v", name, ex.Unresolved, ex.Warnings, p.Rows[0].Blocks[0].Mics)
			continue
		}
		if !strings.Contains(ex.Warnings[0].Reason, "keeps its default") {
			t.Errorf("%s: warning %q", name, ex.Warnings[0].Reason)
		}
	}
}

func TestSeenMic_AmbiguousAndSceneCases(t *testing.T) {
	ambiguous := micSnap("ME_110 US PRN C10R_Ribbon 121", "ME_110 US PRN C10R_ribbon 121")
	ex, p := export(t, libExporter(t, ambiguous, nil), preset(block(cabName, prm("Mic 1", "Ribbon 121", ""))), "g")
	if len(ex.Warnings) != 1 || !strings.Contains(ex.Warnings[0].Reason, "won't guess") || len(p.Rows[0].Blocks[0].Mics) != 0 {
		t.Errorf("ambiguous: %+v mics %+v", ex.Warnings, p.Rows[0].Blocks[0].Mics)
	}
	// a cab's mic is one setting for every scene
	perScene := micSnap("ME_110 US PRN C10R_Ribbon 121")
	ex, p = export(t, libExporter(t, perScene, nil), preset(block(cabName, prm("Mic 1", "Ribbon 121", "Condenser 414"))), "g")
	if len(ex.Warnings) != 1 || !strings.Contains(ex.Warnings[0].Reason, "Scene A and Scene B") || len(p.Rows[0].Blocks[0].Mics) != 0 {
		t.Errorf("per-scene: %+v", ex.Warnings)
	}
	// the default mic stays "at default", written by nobody
	ex, p = export(t, libExporter(t, perScene, nil), preset(block(cabName, prm("Mic 1", "Dynamic 57", ""))), "g")
	if len(ex.Warnings) != 0 || len(p.Rows[0].Blocks[0].Mics) != 0 || len(p.Rows[0].Blocks[0].AtDefault) != 1 {
		t.Errorf("default mic: %+v %+v", ex.Warnings, p.Rows[0].Blocks[0])
	}
}

func TestNoLists_PayloadIsByteIdentical(t *testing.T) {
	// A hook that finds nothing must give exactly the export of a server without the feature.
	empty := &devicelib.Snapshot{}
	for _, name := range []string{"pushable_g3"} {
		sp := loadPreset(t, name)
		for g := range sp.Guitars {
			a, _ := export(t, libExporter(t, empty, nil), sp, g)
			b, _ := export(t, testExporter(t), sp, g)
			if !bytes.Equal(a.Payload, b.Payload) {
				t.Errorf("%s/%s: payload differs with empty lists", name, g)
			}
		}
	}
	var _ devicejobs.Exporter = (*Exporter)(nil)
}

// TestGoldenExport_WithSeenMic pins the payload shape of a cab mic written exactly
// (testdata/golden/pushable_g3_mic.json). The agent's tests feed every golden through its own
// validator (deploy/qc-agent/tests/test_go_goldens.py), so the two sides agree on `mics`.
func TestGoldenExport_WithSeenMic(t *testing.T) {
	sp := loadPreset(t, "pushable_g3")
	chain := sp.Guitars["Gibson ES-339 Humbuckers"]
	found := false
	for i := range chain {
		if strings.HasPrefix(chain[i].Model, "110 US PRN C10R") {
			chain[i].Parameters = append(chain[i].Parameters, prm("Mic 1", "Ribbon 121", ""))
			found = true
		}
	}
	if !found {
		t.Fatal("the cab is not in the preset")
	}
	e := libExporter(t, micSnap("ME_110 US PRN C10R_Ribbon 121"), nil)
	ex, p := export(t, e, sp, "Gibson ES-339 Humbuckers")
	if len(ex.Unresolved) != 0 {
		t.Fatalf("unresolved: %+v", ex.Unresolved)
	}
	n := 0
	for _, b := range p.Rows[0].Blocks {
		n += len(b.Mics)
	}
	if n != 1 {
		t.Fatalf("%d mic writes, want 1", n)
	}
	sum, err := ex.PlanSHA256()
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.MarshalIndent(golden{ex.Format, sum, ex.Summary, ex.Unresolved, ex.Payload}, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "golden", "pushable_g3_mic.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("export differs from %s; rerun with -update and review the diff", path)
	}
}
