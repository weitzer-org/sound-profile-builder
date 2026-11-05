// judge_pairs is the blind pairwise LLM judge for eval_pair_pipeline output. It replaces
// cmd/judge_compare's single random-position verdicts with:
//   - manifest-aware pairing: only (query, sample) pairs where BOTH runs succeeded, never a
//     stale file from an earlier run;
//   - every pair judged twice with positions swapped; a winner counts only if both agree;
//   - an optional same-condition calibration (-aa-dir): sample k vs sample k+1 of ONE
//     condition, which shows what pure run-to-run variation looks like to this judge.
//
// Makes live, PAID Gemini calls (2 per pair). -dry-run shows the plan; -max-usd is a ceiling.
//
//	go run ./cmd/judge_pairs -new <dir> -old <dir> -aa-dir <dir>
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"google.golang.org/genai"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalscore"
)

const judgeModel = "gemini-3.1-pro-preview"

func loadOK(dir string) map[string]map[int]string { // query -> sample -> html path
	f, err := os.Open(filepath.Join(dir, "manifest.jsonl"))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	out := map[string]map[int]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var r evalscore.PipelineRun
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		if out[r.Query] == nil {
			out[r.Query] = map[int]string{}
		}
		if r.OK && r.HTMLPath != "" {
			out[r.Query][r.Sample] = r.HTMLPath
		} else {
			delete(out[r.Query], r.Sample) // a later failure supersedes an earlier success
		}
	}
	return out
}

type pairSpec struct {
	query         string
	sample        int
	first, second string // file paths
}

func main() {
	var (
		newDir  = flag.String("new", "", "new-order dir")
		oldDir  = flag.String("old", "", "old-order dir")
		aaDir   = flag.String("aa-dir", "", "calibration: pair sample 0 vs 1, 2 vs 3, ... within this one dir")
		out     = flag.String("out", "", "output dir for results (default: <new>/judge)")
		workers = flag.Int("workers", 4, "parallel judge calls")
		maxUSD  = flag.Float64("max-usd", 25, "abort when estimated judge spend exceeds this")
		dryRun  = flag.Bool("dry-run", false, "print the number of pairs only")
	)
	flag.Parse()
	if *newDir == "" || *oldDir == "" {
		log.Fatal("-new and -old are required")
	}
	if *out == "" {
		*out = filepath.Join(*newDir, "judge")
	}

	var ab, aa []pairSpec
	nw, od := loadOK(*newDir), loadOK(*oldDir)
	for q, samples := range nw {
		for s, np := range samples {
			if op, ok := od[q][s]; ok {
				ab = append(ab, pairSpec{q, s, np, op})
			}
		}
	}
	if *aaDir != "" {
		for q, samples := range loadOK(*aaDir) {
			for s := 0; s+1 < 100; s += 2 {
				a, aok := samples[s]
				b, bok := samples[s+1]
				if aok && bok {
					aa = append(aa, pairSpec{q, s, a, b})
				}
			}
		}
	}
	fmt.Printf("pairs: %d head-to-head + %d calibration = %d pairs x 2 judge calls = %d calls\n", len(ab), len(aa), len(ab)+len(aa), 2*(len(ab)+len(aa)))
	if *dryRun {
		return
	}

	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY must be set")
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI})
	if err != nil {
		log.Fatal(err)
	}
	var mu sync.Mutex
	var spent float64
	judge := func(ctx context.Context, query, a, b string) (string, float64, string, error) {
		mu.Lock()
		over := spent > *maxUSD
		mu.Unlock()
		if over {
			return "", 0, "", fmt.Errorf("judge spend ceiling reached")
		}
		v, conf, why, in, outTok, err := callJudge(ctx, client, query, a, b)
		mu.Lock()
		spent += agents.EstimateCostUSD(judgeModel, in, 0, outTok, 0)
		mu.Unlock()
		return v, conf, why, err
	}

	run := func(specs []pairSpec) []evalscore.PairVerdict {
		res := make([]evalscore.PairVerdict, len(specs))
		var wg sync.WaitGroup
		ch := make(chan int)
		for w := 0; w < *workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range ch {
					sp := specs[i]
					fa, e1 := os.ReadFile(sp.first)
					fb, e2 := os.ReadFile(sp.second)
					if e1 != nil || e2 != nil {
						res[i] = evalscore.PairVerdict{Query: sp.query, Sample: sp.sample, Winner: "error", Err: "unreadable preset file"}
						continue
					}
					res[i] = evalscore.JudgeBothWays(context.Background(), judge, sp.query, sp.sample, string(fa), string(fb))
					log.Printf("judged %s s%d: %s", sp.query, sp.sample, res[i].Winner)
				}
			}()
		}
		for i := range specs {
			ch <- i
		}
		close(ch)
		wg.Wait()
		return res
	}
	abRes := run(ab)
	aaRes := run(aa)

	_ = os.MkdirAll(*out, 0o755)
	b, _ := json.MarshalIndent(map[string]interface{}{"head_to_head": abRes, "calibration": aaRes}, "", " ")
	_ = os.WriteFile(filepath.Join(*out, "verdicts.json"), b, 0o644)
	md := evalscore.JudgeMarkdown("new", "old", abRes, aaRes) + fmt.Sprintf("\nEstimated judge spend: $%.2f\n", spent)
	_ = os.WriteFile(filepath.Join(*out, "judge_summary.md"), []byte(md), 0o644)
	fmt.Println(md)
}

const rubric = `You are a master guitar tone/gear judge evaluating two AI-generated Quad Cortex presets for the same target: %q.

Both are the final rendered HTML preset pages produced by the pipeline (a builder statement in prose, followed by the full block table and per-block rationale). Parameter rows may optionally carry a "Scene B / Lead" override value and a basis tag (confirmed_range / real_gear_analog / engineering_convention / estimate). These are legitimate, intentional parts of newer presets: do NOT treat their presence as hallucinated.

Evaluate on: (1) plausibility and specificity of the tonal/historical reasoning; (2) internal consistency (do parameters, rationale and each block's Bypass/active state agree with the builder statement); (3) structural correctness (a complete, sensible block list without missing or malformed rows); (4) overall usefulness as a preset a guitarist could load and tweak.

Quote the specific evidence for any claim of fabrication or structural error. Do not infer which preset is "newer" or "baseline".

Preset A:
%s

Preset B:
%s

Respond with your structured judgement. confidence is a number from 0.0 to 1.0.`

func callJudge(ctx context.Context, client *genai.Client, query, a, b string) (verdict string, conf float64, why string, in, out int32, err error) {
	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"preference": {Type: genai.TypeString, Enum: []string{"A", "B", "Equal"}},
				"rationale":  {Type: genai.TypeString},
				"confidence": {Type: genai.TypeNumber, Description: "0.0 to 1.0"},
			},
			Required: []string{"preference", "rationale", "confidence"},
		},
	}
	resp, err := client.Models.GenerateContent(ctx, judgeModel, genai.Text(fmt.Sprintf(rubric, query, a, b)), cfg)
	if err != nil {
		return "", 0, "", 0, 0, err
	}
	if resp.UsageMetadata != nil {
		in, out = resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount+resp.UsageMetadata.ThoughtsTokenCount
	}
	var j struct {
		Preference string  `json:"preference"`
		Rationale  string  `json:"rationale"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(resp.Text()), &j); err != nil {
		return "", 0, "", in, out, err
	}
	return j.Preference, j.Confidence, j.Rationale, in, out, nil
}
