// cmd/device_census runs the device export (internal/deviceexport) over eval outputs and
// reports how many guitar chains could be pushed and why the rest can't. It reads the
// Architect's structured_payload from every *.agents.json under the given directories.
// Offline: no LLM, no device, no storage.
//
//	go run ./cmd/device_census eval_results/v5_vs_v6_r2/v6_a/PROD
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: device_census <dir> [dir...]")
	}
	exp, err := newExporter()
	if err != nil {
		log.Fatal(err)
	}
	var files []string
	for _, dir := range os.Args[1:] {
		err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".agents.json") {
				files = append(files, p)
			}
			return err
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	sort.Strings(files)
	// Reasons are grouped with quoted text and numbers blanked, so "skew 0.6" and
	// "skew 4.93" count as one kind.
	norm := regexp.MustCompile(`"[^"]*"|[-+]?\d+(\.\d+)?|\(ids [^)]*\)`)
	counts := map[string]int{}
	chains, pushable, presets := 0, 0, 0
	for _, f := range files {
		sp, ok := structured(f)
		if !ok {
			continue
		}
		presets++
		guitars := make([]string, 0, len(sp.Guitars))
		for g := range sp.Guitars {
			guitars = append(guitars, g)
		}
		sort.Strings(guitars)
		for _, g := range guitars {
			e, err := exp.Export(context.Background(), sp, g)
			if err != nil {
				log.Printf("%s / %s: %v", f, g, err)
				continue
			}
			chains++
			if len(e.Unresolved) == 0 {
				pushable++
			}
			for _, u := range e.Unresolved {
				counts[norm.ReplaceAllString(u.Reason, "#")]++
			}
		}
	}
	fmt.Printf("%d presets, %d guitar chains, %d pushable\n\n| Count | Unresolved reason |\n|---:|---|\n", presets, chains, pushable)
	type kv struct {
		k string
		v int
	}
	var rows []kv
	for k, v := range counts {
		rows = append(rows, kv{k, v})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].v != rows[j].v {
			return rows[i].v > rows[j].v
		}
		return rows[i].k < rows[j].k
	})
	for _, r := range rows {
		fmt.Printf("| %d | %s |\n", r.v, strings.ReplaceAll(r.k, "|", `\|`))
	}
}

// structured extracts the Architect's structured_payload from an eval *.agents.json.
func structured(path string) (storage.StructuredPreset, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storage.StructuredPreset{}, false
	}
	var calls []struct {
		Role   string `json:"role"`
		Output string `json:"output"`
	}
	if json.Unmarshal(data, &calls) != nil {
		return storage.StructuredPreset{}, false
	}
	for _, c := range calls {
		if !strings.HasPrefix(c.Role, "Architect") {
			continue
		}
		var out struct {
			SP json.RawMessage `json:"structured_payload"`
		}
		if json.Unmarshal([]byte(c.Output), &out) != nil || len(out.SP) == 0 {
			return storage.StructuredPreset{}, false
		}
		var sp storage.StructuredPreset
		if json.Unmarshal(out.SP, &sp) != nil {
			var s string // some outputs carry it as a JSON string
			if json.Unmarshal(out.SP, &s) != nil || json.Unmarshal([]byte(s), &sp) != nil {
				return storage.StructuredPreset{}, false
			}
		}
		return sp, len(sp.Guitars) > 0
	}
	return storage.StructuredPreset{}, false
}

func newExporter() (*deviceexport.Exporter, error) {
	return deviceexport.NewEmbedded(deviceexport.Options{CaptureSource: agents.CaptureSource})
}
