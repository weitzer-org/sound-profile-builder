// e2e_eval_fixtures turns saved pipeline outputs (eval_full_pipeline's <query>.html JSON
// envelopes) into cmd/e2e_seed fixtures, so real presets can be loaded into a local store and
// checked in the UI by tests/e2e/eval_presets.spec.js. Each fixture also records, per guitar and
// block, the name every device control must appear under in the read-only View: the preset's
// own parameter name when it lists the control with a non-default value, else the device's
// control name in the "At device default" line. It makes no Gemini calls.
//
//	go run ./cmd/e2e_eval_fixtures -out <dir> "Eval v6: 06_Gilmour=eval_results/.../06_Gilmour.html" ...
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// expectedBlock is one catalog block and the names its controls must appear under.
type expectedBlock struct {
	Type     string   `json:"type"`
	Model    string   `json:"model"`
	Controls []string `json:"controls"`
}

type fixture struct {
	Name             string `json:"name"`
	BuilderStatement string `json:"builder_statement"`
	Payload          struct {
		Structured storage.StructuredPreset `json:"structured"`
	} `json:"payload"`
	// Expected is keyed by guitar; blocks in table order.
	Expected map[string][]expectedBlock `json:"expected_controls"`
}

var unsafe = regexp.MustCompile(`[^A-Za-z0-9]+`)

func main() {
	out := flag.String("out", "", "directory to write the fixtures to")
	flag.Parse()
	if *out == "" || flag.NArg() == 0 {
		log.Fatal(`usage: e2e_eval_fixtures -out <dir> "name=path/to/output.html" ...`)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, arg := range flag.Args() {
		name, path, ok := strings.Cut(arg, "=")
		if !ok {
			log.Fatalf("bad argument %q: want name=path", arg)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		var env struct {
			BuilderStatement string                    `json:"builder_statement"`
			Structured       *storage.StructuredPreset `json:"structured_payload"`
		}
		if err := json.Unmarshal([]byte(agents.StripJSONFences(string(raw))), &env); err != nil || env.Structured == nil {
			log.Fatalf("%s: not a pipeline output envelope: %v", path, err)
		}
		fx := fixture{Name: name, BuilderStatement: env.BuilderStatement, Expected: map[string][]expectedBlock{}}
		fx.Payload.Structured = *env.Structured
		total := 0
		for guitar, blocks := range env.Structured.Guitars {
			for _, b := range blocks {
				d, ok := agents.DeviceControlsFor(b.Model)
				if !ok {
					continue
				}
				shown, atDefault := agents.SplitDefaults(b)
				names := make([]string, len(d.Controls))
				for _, p := range shown {
					if i := d.Index(p.Name); i >= 0 {
						names[i] = p.Name
					}
				}
				for _, c := range atDefault {
					if i := d.Index(c); i >= 0 {
						names[i] = c
					}
				}
				for i, n := range names {
					if n == "" {
						log.Fatalf("%s: %s control %s is neither shown nor at default", name, b.Model, d.Controls[i].Name)
					}
				}
				fx.Expected[guitar] = append(fx.Expected[guitar], expectedBlock{Type: b.Type, Model: b.Model, Controls: names})
				total += len(names)
			}
		}
		b, _ := json.MarshalIndent(fx, "", "  ")
		file := filepath.Join(*out, strings.Trim(unsafe.ReplaceAllString(name, "_"), "_")+".json")
		if err := os.WriteFile(file, b, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %d device controls across %d guitars -> %s\n", name, total, len(fx.Expected), file)
	}
}
