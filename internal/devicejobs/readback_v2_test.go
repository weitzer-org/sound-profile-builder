package devicejobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testdata/devicejobs/readback_setlist.json is shared with the agent's test_readback.py: every
// setlist the agent must send parses inside a /2 payload here, and every invalid /2 payload is
// refused with the expected reason.
func TestReadbackSetlistFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "devicejobs", "readback_setlist.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Format string `json:"format"`
		Cases  []struct {
			Name   string          `json:"name"`
			Expect json.RawMessage `json:"expect"`
			Slot   *string         `json:"slot"`
		} `json:"cases"`
		Invalid []struct {
			Name    string          `json:"name"`
			Payload json.RawMessage `json:"payload"`
			Error   string          `json:"error"`
		} `json:"invalid"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	if fx.Format != ReadbackFormatV2 || len(fx.Cases) < 20 || len(fx.Invalid) < 12 {
		t.Fatalf("fixture: format %q, %d cases, %d invalid", fx.Format, len(fx.Cases), len(fx.Invalid))
	}
	for _, c := range fx.Cases {
		t.Run(c.Name, func(t *testing.T) {
			payload := json.RawMessage(`{"setlist":` + string(c.Expect) + `,"chains":[],"bypass":null}`)
			p, err := ParseReadback(ReadbackFormatV2, payload)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			label := ""
			if p.Setlist.Position != nil {
				label = SlotLabel(*p.Setlist.Position)
			}
			want := ""
			if c.Slot != nil {
				want = *c.Slot
			}
			if label != want {
				t.Fatalf("slot label %q, want %q", label, want)
			}
			if _, err := ParseReadback(ReadbackFormat, payload); err == nil {
				t.Fatal("a /2 setlist parsed as /1")
			}
		})
	}
	for _, c := range fx.Invalid {
		t.Run("invalid/"+c.Name, func(t *testing.T) {
			_, err := ParseReadback(ReadbackFormatV2, c.Payload)
			if err == nil || !strings.Contains(err.Error(), c.Error) {
				t.Fatalf("err = %v, want it to contain %q", err, c.Error)
			}
		})
	}
}

func TestReadbackFormats(t *testing.T) {
	v1 := json.RawMessage(`{"setlist":{"folder":"My Presets","position":3},"chains":[],"bypass":null}`)
	if _, err := ParseReadback(ReadbackFormat, v1); err != nil {
		t.Fatalf("/1: %v", err)
	}
	if _, err := ParseReadback(ReadbackFormatV2, v1); err == nil {
		t.Fatal("a /1 setlist parsed as /2")
	}
	for _, f := range []string{"", "qc-device-readback/3", "QC-DEVICE-READBACK/1"} {
		if IsReadbackFormat(f) {
			t.Errorf("IsReadbackFormat(%q)", f)
		}
		if _, err := ParseReadback(f, v1); err == nil || strings.Contains(err.Error(), f) && f != "" {
			t.Errorf("ParseReadback(%q): %v", f, err)
		}
	}
	// A hostile key never reaches an error message.
	bad := json.RawMessage(`{"setlist":{"folder":"other","folder_key":"evil` + "`" + `![x](http://e/)","position":null,"is_factory":false},"chains":[],"bypass":null}`)
	if _, err := ParseReadback(ReadbackFormatV2, bad); err == nil || strings.Contains(err.Error(), "evil") {
		t.Fatalf("hostile key: %v", err)
	}
}

func TestSlotLabel(t *testing.T) {
	for p, want := range map[int]string{0: "1A", 7: "1H", 8: "2A", 24: "4A", 255: "32H", -1: "", 256: ""} {
		if got := SlotLabel(p); got != want {
			t.Errorf("SlotLabel(%d) = %q, want %q", p, got, want)
		}
		if want != "" {
			if i, ok := slotIndex(want); !ok || i != p {
				t.Errorf("slotIndex(%q) = %d, %v", want, i, ok)
			}
		}
	}
}

func TestValidFolderKey(t *testing.T) {
	good := []string{"/media/p4/Presets/My Presets", "a", strings.Repeat("k", 128), "x y"}
	bad := []string{"", " a", "a ", "a  b", "a`b", "a\tb", "café", strings.Repeat("k", 129), "a\x7fb"}
	for _, k := range good {
		if !ValidFolderKey(k) || SanitizeText(k, 200) != k {
			t.Errorf("ValidFolderKey(%q) should hold and survive SanitizeText", k)
		}
	}
	for _, k := range bad {
		if ValidFolderKey(k) {
			t.Errorf("ValidFolderKey(%q) accepted", k)
		}
	}
}
