package main

// eval_strip_critic prepares pipeline outputs for quality scoring (plan §6.2): it removes each
// run's own Preset Critic notes from every block rationale and drops the pre-rendered
// final_html_payload (which embeds the same notes), so a fixed re-Critic (cmd/critic_probe) and
// the pairwise judge (cmd/judge_compare) see each scenario's Architect output on equal terms.
// Without this, a scenario whose Critic flags less (gemini-4-argon's Critic replied with a median
// of 20 tokens in Phase 2) would look cleaner for reasons unrelated to preset quality.
//
// Usage: go run ./cmd/eval_strip_critic -in <dir with <query>.html envelopes> -out <dir>
// Writes <query>.html (read by judge_compare) and <query>.json (read by critic_probe), same content.

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalcheck"
)

// Strip returns the envelope with critic notes removed and final_html_payload dropped.
func Strip(envelope []byte) ([]byte, int, error) {
	var env map[string]json.RawMessage
	if err := json.Unmarshal([]byte(agents.StripJSONFences(string(envelope))), &env); err != nil {
		return nil, 0, err
	}
	delete(env, "final_html_payload")
	var sp map[string]any
	if err := json.Unmarshal(env["structured_payload"], &sp); err != nil {
		return nil, 0, err
	}
	removed := 0
	if guitars, ok := sp["guitars"].(map[string]any); ok {
		for _, blocks := range guitars {
			list, _ := blocks.([]any)
			for _, b := range list {
				blk, _ := b.(map[string]any)
				if r, ok := blk["rationale"].(string); ok {
					clean := evalcheck.StripCriticNotes(r)
					if clean != strings.TrimSpace(r) {
						removed++
					}
					blk["rationale"] = clean
				}
			}
		}
	}
	b, err := json.Marshal(sp)
	if err != nil {
		return nil, 0, err
	}
	env["structured_payload"] = b
	out, err := json.MarshalIndent(env, "", "  ")
	return out, removed, err
}

func main() {
	in := flag.String("in", "", "directory of <query>.html pipeline envelopes")
	out := flag.String("out", "", "output directory")
	flag.Parse()
	if *in == "" || *out == "" {
		log.Fatal("-in and -out are required")
	}
	files, err := filepath.Glob(filepath.Join(*in, "*.html"))
	if err != nil || len(files) == 0 {
		log.Fatalf("no .html envelopes in %s (%v)", *in, err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	total := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		clean, n, err := Strip(raw)
		if err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		total += n
		base := strings.TrimSuffix(filepath.Base(f), ".html")
		for _, ext := range []string{".html", ".json"} {
			if err := os.WriteFile(filepath.Join(*out, base+ext), clean, 0o644); err != nil {
				log.Fatal(err)
			}
		}
	}
	log.Printf("%d files, %d block rationales had critic notes removed -> %s", len(files), total, *out)
}
