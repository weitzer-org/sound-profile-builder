package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// probe_thinking is Phase 0 of the gemini-3.5-flash/gemini-3.6-flash/gemini-3.1-pro-preview
// thinking-level comparison (see TODO.md's Pipeline Quality Work section): before spending
// budget on a full model x thinking-budget eval matrix, empirically confirm which of the
// three target models actually accept and honor Orchestrator.ThinkingBudget (wired into
// buildGenerationConfig via genai.ThinkingConfig) rather than assuming uniform API support
// across models. Runs one cheap, isolable agent (Sonic Profiler, same choice
// cmd/eval_subagent makes for the same reason) against every {model, budget} pair and reports
// success/failure, latency, and reported ThoughtsTokenCount for each.
func main() {
	// Bounded so a single stalled Gemini call can't hang the whole probe indefinitely --
	// 12 cells at up to 3 minutes each (RunAgentSplit's own per-attempt timeout) is a
	// generous but finite ceiling.
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()

	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Fatalf("GEMINI_API_KEY must be set to run this probe")
	}

	models := []string{"gemini-3.5-flash", "gemini-3.6-flash", "gemini-3.1-pro-preview"}

	// nil = provider/model default (ThinkingConfig omitted entirely); the rest are explicit
	// budgets, including 0 (thinking disabled) and a value bracketing job_tracker's own sweep
	// range (256/1024/2048/4096) without repeating its full 4-point sweep here -- this probe
	// only needs to establish whether the knob is honored at all per model, not tune it yet.
	budgets := []*int32{nil, int32Ptr(0), int32Ptr(512), int32Ptr(2048)}

	const query = "Clean funk blues tone. Stevie Ray Vaughan style with high headroom."
	sysPrompt, err := agents.LoadPrompt("2_sonic_profiler", "")
	if err != nil {
		log.Fatalf("Failed to load Sonic Profiler prompt: %v", err)
	}
	userPrompt := fmt.Sprintf("User Request: %s\nQC Block Parameter Vocabulary: %s", query, agents.GetQCSonicProfilerSchemaJSON())

	type result struct {
		Model          string
		Budget         string
		LatencySec     float64
		InputTokens    int32
		OutputTokens   int32
		ThinkingTokens int32
		ServedByOther  string // non-empty if RunAgentSplit's fallback chain silently substituted a different model
		Error          string
	}
	var results []result

	for _, model := range models {
		for _, budget := range budgets {
			label := "default (unset)"
			if budget != nil {
				label = fmt.Sprintf("%d", *budget)
			}
			log.Printf("Probing model=%s thinking_budget=%s ...", model, label)

			orch, err := agents.NewOrchestrator(ctx, geminiKey, nil)
			if err != nil {
				log.Fatalf("Failed to init orchestrator: %v", err)
			}
			orch.AgentModels = map[string]string{"2_sonic_profiler": model}
			orch.ThinkingBudget = budget

			start := time.Now()
			_, runErr := orch.RunAgentSplit(ctx, "Sonic Profiler", sysPrompt, userPrompt)
			latency := time.Since(start).Seconds()
			orch.Close()

			r := result{Model: model, Budget: label, LatencySec: latency}
			if runErr != nil {
				r.Error = runErr.Error()
				log.Printf("  ❌ %v", runErr)
			} else {
				r.InputTokens = orch.Usage.InputTokens
				r.OutputTokens = orch.Usage.OutputTokens
				r.ThinkingTokens = orch.Usage.ThinkingTokens
				// RunAgentSplit's fallback chain (getFallbackChain) can silently substitute a
				// different model if the requested one rejects this budget -- ModelsUsed records
				// whichever model actually served the call, which is not necessarily the one this
				// probe requested. A "success" that's actually a substitution is a materially
				// different finding (see the gemini-3.1-pro-preview/budget=0 case below) than the
				// requested model genuinely honoring the budget, so it must be surfaced, not hidden
				// inside a green checkmark.
				for servedModel := range orch.Usage.ModelsUsed {
					if servedModel != model {
						r.ServedByOther = servedModel
					}
				}
				if r.ServedByOther != "" {
					log.Printf("  ⚠️  %.2fs | in=%d out=%d thinking=%d (SILENTLY FELL BACK TO %s)", latency, r.InputTokens, r.OutputTokens, r.ThinkingTokens, r.ServedByOther)
				} else {
					log.Printf("  ✅ %.2fs | in=%d out=%d thinking=%d", latency, r.InputTokens, r.OutputTokens, r.ThinkingTokens)
				}
			}
			results = append(results, r)
		}
	}

	fmt.Println("\n=== THINKING-BUDGET PROBE RESULTS ===")
	fmt.Printf("%-24s %-16s %10s %8s %8s %10s %-22s %s\n", "Model", "Budget", "Latency", "In", "Out", "Thinking", "ServedBy(if fallback)", "Error")
	for _, r := range results {
		errCol := "-"
		if r.Error != "" {
			errCol = truncate(r.Error, 80)
		}
		servedCol := "-"
		if r.ServedByOther != "" {
			servedCol = "⚠️ " + r.ServedByOther
		}
		fmt.Printf("%-24s %-16s %9.2fs %8d %8d %10d %-22s %s\n", r.Model, r.Budget, r.LatencySec, r.InputTokens, r.OutputTokens, r.ThinkingTokens, servedCol, errCol)
	}
}

func int32Ptr(v int32) *int32 { return &v }

// truncate flattens s to a single line and shortens it to at most n runes (not bytes -- error
// text can carry multi-byte UTF-8 like curly quotes or em-dashes, and a byte-index slice can
// split one and corrupt the output).
func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
