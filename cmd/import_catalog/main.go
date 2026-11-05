// cmd/import_catalog converts a Quad Cortex ModelRepo.xml (the device's own catalog of
// block models and their parameters) into internal/catalog/qc_catalog.json, the derived
// form the app embeds. See the internal/catalog package doc for what the catalog is.
//
// Getting a ModelRepo.xml: the device sends it over USB on request. The unofficial
// qc-mcp project (github.com/lexasoft123/qc-mcp) has tools/dump_model_repo.py, which reads
// it from a connected unit, and also bundles a snapshot at src/qc_mcp/ModelRepo.xml.
// Prefer a dump from your own unit after a CorOS update. Keep the raw XML out of the repo.
//
// Usage:
//
//	go run ./cmd/import_catalog -in ModelRepo.xml -coros 4.1 -origin "dumped from my QC"
//	go run ./cmd/import_catalog -in ModelRepo.xml -coros 4.1 -diff   # report only
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/weitzer-org/sound-builder/internal/catalog"
)

const readme = "Derived from the Quad Cortex's own device catalog (ModelRepo.xml, which the unit " +
	"sends over USB) by cmd/import_catalog -- do not edit by hand; re-run the importer after a " +
	"CorOS update. Hidden/internal models, firmware-plumbing categories and hidden parameters " +
	"are dropped. A param's index is its position in the device's parameter list (hidden ones " +
	"included), which is how the device addresses it; names repeat within a model (e.g. one " +
	"POSITION/DISTANCE/LEVEL/PAN set per mic on dual-mic cabs), so use index to tell them apart. " +
	"min/max are omitted and min_symbol/max_symbol set when the device uses a named bound " +
	"(e.g. MAX_CABSIM_DB) whose numeric value lives in firmware -- treat that range as unknown."

func main() {
	in := flag.String("in", "", "path to ModelRepo.xml (required)")
	out := flag.String("out", "internal/catalog/qc_catalog.json", "output path")
	coros := flag.String("coros", "", "CorOS version the ModelRepo.xml came from, e.g. 4.1 (required)")
	origin := flag.String("origin", "", "where the ModelRepo.xml came from (recorded in the output)")
	diff := flag.Bool("diff", false, "report changes against the existing -out file instead of writing")
	flag.Parse()
	if *in == "" || *coros == "" {
		flag.Usage()
		os.Exit(2)
	}

	f, err := os.Open(*in)
	if err != nil {
		log.Fatal(err)
	}
	models, err := catalog.ParseModelRepo(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}
	next := catalog.New(models, catalog.Source{CorOSVersion: *coros, Origin: *origin}, readme)

	if *diff {
		data, err := os.ReadFile(*out)
		if err != nil {
			log.Fatalf("read existing catalog: %v", err)
		}
		prev, err := catalog.Decode(data)
		if err != nil {
			log.Fatal(err)
		}
		report(prev, next)
		return
	}

	data, err := encode(next)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		log.Fatal(err)
	}
	params, unknown := 0, 0
	for _, m := range next.Models {
		for _, p := range m.Params {
			params++
			if p.MinSymbol != "" || p.MaxSymbol != "" {
				unknown++
			}
		}
	}
	fmt.Fprintf(os.Stderr, "wrote %s: %d models, %d params (%d with a symbolic, unknown range)\n",
		*out, len(next.Models), params, unknown)
}

// encode writes one model per line so a CorOS update shows up as a readable diff.
func encode(c *catalog.Catalog) ([]byte, error) {
	var buf bytes.Buffer
	head := func(v any) ([]byte, error) { return json.Marshal(v) }
	r, err := head(c.Readme)
	if err != nil {
		return nil, err
	}
	s, err := head(c.Source)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&buf, "{\n\"_readme\": %s,\n\"source\": %s,\n\"models\": [\n", r, s)
	for i, m := range c.Models {
		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		buf.Write(b)
		if i < len(c.Models)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("]\n}\n")
	return buf.Bytes(), nil
}

func report(prev, next *catalog.Catalog) {
	old := map[int]catalog.Model{}
	for _, m := range prev.Models {
		old[m.ID] = m
	}
	cur := map[int]catalog.Model{}
	for _, m := range next.Models {
		cur[m.ID] = m
	}
	var ids []int
	for id := range old {
		ids = append(ids, id)
	}
	for id := range cur {
		if _, ok := old[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	fmt.Printf("existing: CorOS %s, %d models; new: CorOS %s, %d models\n",
		prev.Source.CorOSVersion, len(prev.Models), next.Source.CorOSVersion, len(next.Models))
	for _, id := range ids {
		o, inOld := old[id]
		n, inNew := cur[id]
		switch {
		case !inOld:
			fmt.Printf("  + %6d  %s  [%s]\n", id, n.Name, n.Category)
		case !inNew:
			fmt.Printf("  - %6d  %s  [%s]  REMOVED\n", id, o.Name, o.Category)
		default:
			if o.Name != n.Name {
				fmt.Printf("  ~ %6d  %s -> %s\n", id, o.Name, n.Name)
			}
			ob, _ := json.Marshal(o.Params)
			nb, _ := json.Marshal(n.Params)
			if !bytes.Equal(ob, nb) {
				fmt.Printf("  * %6d  %s: parameters changed\n", id, n.Name)
			}
		}
	}
}
