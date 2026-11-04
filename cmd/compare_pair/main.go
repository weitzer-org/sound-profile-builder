// compare_pair compares two eval_pair_pipeline manifests (new vs old order) run by run.
// No API calls. Pairs on (query, sample); see internal/evalscore/pipeline.go for the
// metrics and the pre-registered non-inferiority rule.
//
//	go run ./cmd/compare_pair -new <dir> -old <dir> [-margin 0.05] [-out report.md]
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/weitzer-org/sound-builder/internal/evalscore"
)

func load(dir string) []evalscore.PipelineRun {
	f, err := os.Open(filepath.Join(dir, "manifest.jsonl"))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	// Last record per (query, sample) wins, so a retried unit replaces its earlier failure.
	type k struct {
		q string
		s int
	}
	last := map[k]evalscore.PipelineRun{}
	var order []k
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var r evalscore.PipelineRun
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		key := k{r.Query, r.Sample}
		if _, seen := last[key]; !seen {
			order = append(order, key)
		}
		last[key] = r
	}
	var out []evalscore.PipelineRun
	for _, key := range order {
		out = append(out, last[key])
	}
	return out
}

func main() {
	newDir := flag.String("new", "", "new-order output dir")
	oldDir := flag.String("old", "", "old-order output dir")
	margin := flag.Float64("margin", 0.05, "non-inferiority margin for quality metrics")
	outPath := flag.String("out", "", "write the report here too")
	flag.Parse()
	if *newDir == "" || *oldDir == "" {
		log.Fatal("-new and -old are required")
	}
	n, o := load(*newDir), load(*oldDir)
	md := evalscore.PipelineMarkdown(evalscore.ComparePipelines(n, o, *margin), *margin, len(n), len(o))
	fmt.Println(md)
	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(md), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
