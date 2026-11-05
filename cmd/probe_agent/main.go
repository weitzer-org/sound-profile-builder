package main

// probe_agent replays ONE agent's call with production's exact generation config (temperature,
// tools, MaxOutputTokens, response schema) through a raw genai client, so the full response text
// and grounding metadata are kept even when the call is truncated -- RunAgentSplit discards the
// text of a MAX_TOKENS response. Used to diagnose (a) why grounded agents 1-3 rarely search and
// (b) what the Acoustician's runaway truncations actually contain.
//
// Variants: prod (as production), noschema (tools only, no response schema/JSON mode),
// forcesearch (prod + an explicit instruction to search first), nosearch (schema, no tools).
// -dry-run defaults to true. Live runs need EVAL_NO_FALLBACK=true for consistency with the other
// eval tools (this tool never falls back anyway: it calls exactly the model given).

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/config"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

// PipelinePrefix is what RunPipeline prepends for evalfixtures.DefaultConstraints (favor_captures
// and single_amp_mode true), copied from its log output, so a probe sees what the Phase 2 eval sent.
const PipelinePrefix = "[GLOBAL USER CONFIG FLAG: FAVOR CAPTURES. The generated preset should actively prioritize using official Factory Neural Captures (tagged with is_capture: true) over built-in algorithmic component models when possible.]\n\n[GLOBAL USER CONFIG FLAG: SINGLE AMP ONLY. The generated preset MUST route through one single amplifier block.]\n\n"

// SourcesSuffix asks for the URLs actually used; the sources variant also makes the field required.
const SourcesSuffix = "\n\nUse Google Search to research this request, and list in `sources` the URLs of the pages you actually used. Do not list a URL you did not retrieve."

// ForceSearchSuffix is appended to the user prompt in the forcesearch variant.
const ForceSearchSuffix = "\n\nBefore writing your answer, run at least one Google Search for the specific gear, artist and recording named above, and base your answer on what it returns."

var roles = map[string]string{
	"1_tone_historian": "Tone Historian", "2_sonic_profiler": "Sonic Profiler",
	"3_community_scraper": "Community Scraper", "6_acoustician": "Acoustician",
}

// UserPrompt rebuilds the agent's user prompt the way RunPipeline does. Acoustician needs the
// Sonic Profiler output, taken from a cmd/eval_architect frozen file for the same query.
func UserPrompt(key, query, sonic string) (string, error) {
	switch key {
	case "1_tone_historian", "3_community_scraper":
		return "User Request: " + query, nil
	case "2_sonic_profiler":
		return fmt.Sprintf("User Request: %s\nQC Block Parameter Vocabulary: %s", query, agents.GetQCSonicProfilerSchemaJSON()), nil
	case "6_acoustician":
		if sonic == "" {
			return "", fmt.Errorf("acoustician needs a frozen Sonic Profiler output")
		}
		return fmt.Sprintf("Sonic Profiler: %s\nConstraints: %v\nQC Block Parameter Vocabulary: %s", sonic, evalfixtures.DefaultConstraints(), agents.GetQCAmpEQSchemaJSON()), nil
	}
	return "", fmt.Errorf("unsupported agent %q", key)
}

// ApplyVariant mutates cfg/user for a variant.
func ApplyVariant(v string, cfg *genai.GenerateContentConfig, user string) (string, error) {
	switch v {
	case "prod":
	case "noschema":
		cfg.ResponseSchema, cfg.ResponseJsonSchema, cfg.ResponseMIMEType = nil, nil, ""
	case "forcesearch":
		user += ForceSearchSuffix
	case "nosearch":
		cfg.Tools = nil
	case "sources":
		if cfg.ResponseSchema == nil || cfg.ResponseSchema.Type != genai.TypeObject {
			return "", fmt.Errorf("sources variant needs a typed object schema")
		}
		sc := *cfg.ResponseSchema
		props := map[string]*genai.Schema{}
		for k, v := range sc.Properties {
			props[k] = v
		}
		props["sources"] = &genai.Schema{Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}, Description: "URLs of the pages retrieved with Google Search and used for this answer."}
		sc.Properties = props
		sc.Required = append(append([]string(nil), sc.Required...), "sources")
		cfg.ResponseSchema = &sc
		user += SourcesSuffix
	default:
		return "", fmt.Errorf("unknown variant %q", v)
	}
	return user, nil
}

type Result struct {
	Agent, Model, Variant, Query string
	PromptVersion                string
	Rep                          int
	Served, Finish               string
	Grounded                     bool
	SearchQueries                []string
	In, Out, Think               int32
	LatencyMs                    int64
	Error                        string `json:",omitempty"`
	Text                         string
}

func main() {
	agentsFlag := flag.String("agents", "1_tone_historian", "comma-separated agent keys (1_tone_historian, 2_sonic_profiler, 3_community_scraper, 6_acoustician)")
	models := flag.String("models", "gemini-4-argon", "comma-separated models")
	variants := flag.String("variants", "prod", "comma-separated: prod, noschema, forcesearch, nosearch, sources")
	queries := flag.String("queries", "07_Edge,09_BB_King,05_Clapton", "golden query names")
	reps := flag.Int("reps", 1, "repetitions")
	promptVersion := flag.String("prompt-version", "", "override config.json's prompt version for every probed agent (e.g. v4)")
	pipelinePrefix := flag.Bool("pipeline-prefix", false, "prepend the [GLOBAL USER CONFIG FLAG ...] lines RunPipeline adds for evalfixtures.DefaultConstraints (favor captures, single amp), as the eval pipeline sends them")
	frozenDir := flag.String("frozen-dir", "eval_results/phase1/architect/frozen", "eval_architect frozen files (Acoustician input)")
	dry := flag.Bool("dry-run", true, "print the grid and exit")
	out := flag.String("out", "", "output dir (default eval_results/probe_agent/<UTC>)")
	flag.Parse()

	split := func(s string) (o []string) {
		for _, x := range strings.Split(s, ",") {
			if x = strings.TrimSpace(x); x != "" {
				o = append(o, x)
			}
		}
		return
	}
	ag, ms, vs, qs := split(*agentsFlag), split(*models), split(*variants), split(*queries)
	golden := evalfixtures.GoldenQueries()
	for _, q := range qs {
		if _, ok := golden[q]; !ok {
			log.Fatalf("unknown query %q", q)
		}
	}
	for _, a := range ag {
		if roles[a] == "" {
			log.Fatalf("unsupported agent %q", a)
		}
	}
	total := len(ag) * len(ms) * len(vs) * len(qs) * (*reps)
	fmt.Printf("Plan: agents=%v models=%v variants=%v queries=%v reps=%d\nTOTAL: %d API calls\n", ag, ms, vs, qs, *reps, total)
	if *dry {
		fmt.Println("DRY RUN -- no calls made.")
		return
	}
	if !agents.EvalNoFallbackEnabled() {
		log.Fatalf("%s=true required", agents.EvalNoFallbackEnv)
	}
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY must be set")
	}
	cfgFile, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	orch, err := agents.NewOrchestrator(ctx, key, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer orch.Close()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI})
	if err != nil {
		log.Fatal(err)
	}
	if *out == "" {
		*out = filepath.Join("eval_results", "probe_agent", time.Now().UTC().Format("20060102T150405Z"))
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(filepath.Join(*out, "results.jsonl"))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	n := 0
	for _, q := range qs {
		sonic := ""
		if b, err := os.ReadFile(filepath.Join(*frozenDir, q+".json")); err == nil {
			var fz struct {
				Upstream []struct{ Role, Output string } `json:"upstream"`
			}
			if json.Unmarshal(b, &fz) == nil {
				for _, u := range fz.Upstream {
					if u.Role == "Sonic Profiler" {
						sonic = u.Output
					}
				}
			}
		}
		for r := 1; r <= *reps; r++ {
			for _, a := range ag {
				for _, m := range ms {
					for _, v := range vs {
						n++
						res := Result{Agent: a, Model: m, Variant: v, Query: q, Rep: r, PromptVersion: *promptVersion}
						orch.AgentModels = map[string]string{a: m}
						pv := cfgFile.AgentPrompts[a]
						if *promptVersion != "" {
							pv = *promptVersion
						}
						sys, err := orch.PromptForAgent(a, pv)
						qtext := golden[q]
						if *pipelinePrefix {
							qtext = PipelinePrefix + qtext
						}
						user, uerr := UserPrompt(a, qtext, sonic)
						gc := orch.GenerationConfigFor(roles[a], a, sys)
						if err == nil && uerr == nil {
							user, uerr = ApplyVariant(v, gc, user)
						}
						if err != nil || uerr != nil {
							res.Error = fmt.Sprint(err, uerr)
						} else {
							gc.SystemInstruction = &genai.Content{Parts: []*genai.Part{genai.NewPartFromText(sys)}}
							cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
							t0 := time.Now()
							resp, cerr := client.Models.GenerateContent(cctx, m, []*genai.Content{genai.NewContentFromText(user, genai.RoleUser)}, gc)
							res.LatencyMs = time.Since(t0).Milliseconds()
							cancel()
							if cerr != nil {
								res.Error = cerr.Error()
							}
							if resp != nil {
								res.Served = resp.ModelVersion
								res.Text = resp.Text()
								if resp.UsageMetadata != nil {
									res.In, res.Out, res.Think = resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount, resp.UsageMetadata.ThoughtsTokenCount
								}
								if len(resp.Candidates) > 0 && resp.Candidates[0] != nil {
									res.Finish = string(resp.Candidates[0].FinishReason)
									if gm := resp.Candidates[0].GroundingMetadata; gm != nil {
										res.SearchQueries = gm.WebSearchQueries
										res.Grounded = len(gm.WebSearchQueries) > 0 || len(gm.GroundingChunks) > 0
									}
								}
							}
						}
						b, _ := json.Marshal(res)
						fmt.Fprintln(f, string(b))
						log.Printf("[%d/%d] %s %s %s %s served=%s finish=%s grounded=%v searches=%d out=%d think=%d %s", n, total, a, m, v, q, res.Served, res.Finish, res.Grounded, len(res.SearchQueries), res.Out, res.Think, res.Error)
					}
				}
			}
		}
	}
}
