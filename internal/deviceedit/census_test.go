package deviceedit

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

var update = flag.Bool("update", false, "rewrite testdata/census.golden.txt")

var leadNumber = regexp.MustCompile(`^(-?\d+(?:\.\d+)?)(.*)$`)

// nudge changes the first numeric parameter of a block by half a unit (down if it would go negative).
func nudge(blk *storage.EffectBlock) bool {
	for i := range blk.Parameters {
		m := leadNumber.FindStringSubmatch(blk.Parameters[i].Value)
		if m == nil || blk.Parameters[i].Name == "Bypass" {
			continue
		}
		x, _ := strconv.ParseFloat(m[1], 64)
		if x >= 0.5 {
			x -= 0.5
		} else {
			x += 0.5
		}
		blk.Parameters[i].Value = strconv.FormatFloat(x, 'f', -1, 64) + m[2]
		return true
	}
	return false
}

type mutation struct {
	name    string
	derived bool
	apply   func(t *testing.T, c []storage.EffectBlock) []storage.EffectBlock
}

var mutations = []mutation{
	{"draft, unchanged", true, func(t *testing.T, c []storage.EffectBlock) []storage.EffectBlock { return c }},
	{"nudge one value per block", true, func(t *testing.T, c []storage.EffectBlock) []storage.EffectBlock {
		for i := range c {
			nudge(&c[i])
		}
		return c
	}},
	{"bypass the first block in A and B", true, func(t *testing.T, c []storage.EffectBlock) []storage.EffectBlock {
		setParam(&c[0], "Bypass", "On", "On")
		return c
	}},
	{"drop the last block (draft of this read)", true, func(t *testing.T, c []storage.EffectBlock) []storage.EffectBlock { return c[:len(c)-1] }},
	{"drop the last block (another preset)", false, func(t *testing.T, c []storage.EffectBlock) []storage.EffectBlock { return c[:len(c)-1] }},
	{"append a new block", true, func(t *testing.T, c []storage.EffectBlock) []storage.EffectBlock {
		return append(c, storage.EffectBlock{ID: "b9", Model: "Circular Delay", Type: "Delay"})
	}},
	{"swap the first block's model", true, func(t *testing.T, c []storage.EffectBlock) []storage.EffectBlock {
		c[0] = storage.EffectBlock{ID: c[0].ID, Model: "Rage Booster", Type: "Overdrive"}
		return c
	}},
	{"move the first block to the end (ids kept)", true, func(t *testing.T, c []storage.EffectBlock) []storage.EffectBlock {
		return append(c[1:], c[0])
	}},
	{"move the first block to the end, ids dropped", false, func(t *testing.T, c []storage.EffectBlock) []storage.EffectBlock {
		out := append(c[1:], c[0])
		for i := range out {
			out[i].ID = fmt.Sprintf("block-%d", i+1)
		}
		return out
	}},
}

// TestCensus tabulates, for each round-trip fixture and a set of hand edits, how many changes
// become ops and how many are not applied, by reason. The table is a golden file.
func TestCensus(t *testing.T) {
	e := newExporter(t)
	var b strings.Builder
	b.WriteString("fixture | target | param ops | bypass ops | not applied\n")
	totalOps, totalItems := 0, 0
	for _, fx := range fixtures {
		f := loadFixture(t, fx.name)
		rows := []struct {
			label string
			p     *Plan
		}{
			{"QC-2 original, not derived", plan(t, e, f, original(t, fx.name, fx.guitar), false)},
		}
		for _, m := range mutations {
			c := m.apply(t, clone(draft(t, e, f)))
			rows = append(rows, struct {
				label string
				p     *Plan
			}{m.name, plan(t, e, f, c, m.derived)})
		}
		for _, r := range rows {
			codes := codesOf(r.p)
			keys := make([]string, 0, len(codes))
			for k := range codes {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var parts []string
			for _, k := range keys {
				parts = append(parts, fmt.Sprintf("%s=%d", k, codes[k]))
			}
			na := strings.Join(parts, ", ")
			if na == "" {
				na = "-"
			}
			fmt.Fprintf(&b, "%s | %s | %d | %d | %s\n", fx.name, r.label, len(r.p.Payload.ParamOps), len(r.p.Payload.BypassOps), na)
			totalOps += r.p.Ops()
			totalItems += r.p.NotAppliedCount
		}
	}
	fmt.Fprintf(&b, "total | | | %d ops | %d not applied\n", totalOps, totalItems)
	got := b.String()
	path := filepath.Join("testdata", "census.golden.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if string(want) != got {
		t.Errorf("census changed; rerun with -update and review:\n%s", got)
	}
}
