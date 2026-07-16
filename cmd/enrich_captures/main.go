// cmd/enrich_captures is an offline batch tool that backfills coros_map.json's sparse
// tonal_archetype field using the same GoogleSearch grounding tool Sonic Profiler already
// uses live (see internal/agents/schemas.go, key "14_capture_enrichment"). It never writes
// coros_map.json directly -- results go to a sibling draft file for human review via a
// normal PR, per the design documented in TODO.md under "RefineChat's missing
// SelectedCaptureContext". Run with: GEMINI_API_KEY=... go run ./cmd/enrich_captures
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// newCategoryValue must match the "New Category" enum value in
// internal/agents/schemas.go's "14_capture_enrichment" schema exactly.
const newCategoryValue = "New Category"

// capturedGear mirrors just the fields this tool needs from coros_map.json's per-entry
// structure. Unlike a full round-trip rewrite of the source file, this tool only ever
// reads coros_map.json and writes to a separate draft file, so it doesn't need to
// preserve every other field on each entry.
type capturedGear struct {
	CorosEquivalent string `json:"coros_equivalent"`
	IsCapture       bool   `json:"is_capture"`
	TonalArchetype  string `json:"tonal_archetype"`
}

// enrichmentResult is the shape of Agent 14's structured JSON output, per the schema in
// internal/agents/schemas.go ("14_capture_enrichment").
type enrichmentResult struct {
	FoundReliableSource      bool   `json:"found_reliable_source"`
	TonalArchetype           string `json:"tonal_archetype"`
	NewCategoryLabel         string `json:"new_category_label"`
	NewCategoryJustification string `json:"new_category_justification"`
	Citation                 string `json:"citation"`
}

// draftEntry is one proposed label in the output draft file, keyed by the original
// coros_map.json map key(s) that share this coros_equivalent (see groupByEquivalent).
type draftEntry struct {
	CorosEquivalent          string `json:"coros_equivalent"`
	ProposedTonalArchetype   string `json:"proposed_tonal_archetype"`
	IsNewCategory            bool   `json:"is_new_category"`
	NewCategoryJustification string `json:"new_category_justification,omitempty"`
	Citation                 string `json:"citation"`
	Source                   string `json:"source"`
}

func main() {
	sourcePath := "internal/agents/coros_map.json"
	if len(os.Args) > 1 {
		sourcePath = os.Args[1]
	}
	draftPath := "internal/agents/coros_map.tonal_archetype.draft.json"
	if len(os.Args) > 2 {
		draftPath = os.Args[2]
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatalf("GEMINI_API_KEY must be set to run this tool")
	}

	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", sourcePath, err)
	}

	var mappings map[string]capturedGear
	if err := json.Unmarshal(raw, &mappings); err != nil {
		log.Fatalf("Failed to parse %s: %v", sourcePath, err)
	}

	missing := groupMissingByEquivalent(mappings)
	if len(missing) == 0 {
		log.Printf("Every is_capture entry already has a tonal_archetype. Nothing to research.")
		return
	}
	actionableEntries := 0
	for _, keys := range missing {
		actionableEntries += len(keys)
	}
	unfixable := countCapturesMissingEquivalent(mappings)
	log.Printf("Found %d unique real-gear names across %d capture entries missing tonal_archetype.", len(missing), actionableEntries)
	if unfixable > 0 {
		log.Printf("%d additional capture entries are missing tonal_archetype AND have no coros_equivalent recorded -- this tool cannot research those; they need manual attention.", unfixable)
	}

	names := sortedKeys(missing)
	if limitStr := os.Getenv("ENRICH_LIMIT"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			log.Fatalf("Invalid ENRICH_LIMIT %q: must be a non-negative integer", limitStr)
		}
		if limit < len(names) {
			log.Printf("ENRICH_LIMIT=%d set -- only researching the first %d of %d names (smoke-test mode).", limit, limit, len(names))
			names = names[:limit]
		} else {
			log.Printf("ENRICH_LIMIT=%d set, but only %d names found -- researching all of them.", limit, len(names))
		}
	}

	ctx := context.Background()
	orch, err := agents.NewOrchestrator(ctx, apiKey, nil)
	if err != nil {
		log.Fatalf("Failed to init orchestrator: %v", err)
	}
	defer orch.Close()

	sysPrompt, err := agents.LoadPrompt("14_capture_enrichment", "")
	if err != nil {
		log.Fatalf("Failed to load Capture Enrichment prompt: %v", err)
	}

	drafts := make(map[string]draftEntry) // keyed by original coros_map.json map key
	skipped := 0

	// Sequential, not concurrent: this is a low-frequency, one-off (or periodic) offline
	// sweep with no user waiting, so simplicity beats throughput here.
	for _, equiv := range names {
		keys := missing[equiv]
		log.Printf("Researching %q (%d capture entries)...", equiv, len(keys))

		userPrompt := fmt.Sprintf("Research the real-world tonal character of: %s", equiv)
		result, err := orch.RunAgentSplit(ctx, "Capture Enrichment", sysPrompt, userPrompt)
		// Pace every call the same way regardless of outcome -- placed right after the
		// call (not at the bottom of the loop) so an error path's `continue` can't skip
		// it. Backoff matters most exactly when calls are failing, e.g. a degraded or
		// rate-limited API; a sleep only the success path hits would remove it right
		// when it's needed most.
		time.Sleep(500 * time.Millisecond)
		if err != nil {
			log.Printf("  Search failed for %q: %v -- skipping", equiv, err)
			skipped += len(keys)
			continue
		}

		var parsed enrichmentResult
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			log.Printf("  Failed to parse response for %q: %v -- skipping. Raw: %s", equiv, err, result)
			skipped += len(keys)
			continue
		}

		if !parsed.FoundReliableSource {
			log.Printf("  No reliable source found for %q -- skipping (needs manual research).", equiv)
			skipped += len(keys)
			continue
		}

		// tonal_archetype/citation are schema-optional (see schemas.go) precisely so the
		// model never has to fabricate either just to produce valid JSON when it has
		// genuinely found nothing -- so a true found_reliable_source claim paired with an
		// empty value here is a malformed response, not a legitimate "no data" case.
		if parsed.TonalArchetype == "" || parsed.Citation == "" {
			log.Printf("  %q claimed a reliable source but gave an empty tonal_archetype/citation -- skipping (malformed response).", equiv)
			skipped += len(keys)
			continue
		}

		label := parsed.TonalArchetype
		isNew := label == newCategoryValue
		if isNew {
			if parsed.NewCategoryLabel == "" {
				log.Printf("  %q proposed a new category but gave no label -- skipping.", equiv)
				skipped += len(keys)
				continue
			}
			label = parsed.NewCategoryLabel
		} else {
			// Scrub a field that should only ever be meaningful in the New Category case --
			// don't trust that the model left it empty just because tonal_archetype wasn't
			// New Category.
			parsed.NewCategoryJustification = ""
		}

		entry := draftEntry{
			CorosEquivalent:          equiv,
			ProposedTonalArchetype:   label,
			IsNewCategory:            isNew,
			NewCategoryJustification: parsed.NewCategoryJustification,
			Citation:                 parsed.Citation,
			Source:                   "search_grounded",
		}
		for _, k := range keys {
			drafts[k] = entry
		}
		log.Printf("  -> %q (citation: %s)", label, parsed.Citation)
	}

	out, err := json.MarshalIndent(drafts, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal draft output: %v", err)
	}
	if err := os.WriteFile(draftPath, out, 0644); err != nil {
		log.Fatalf("Failed to write %s: %v", draftPath, err)
	}

	log.Printf("Wrote %d proposed labels to %s (%d skipped -- no reliable source or malformed response). Review and merge into %s manually via a normal PR.", len(drafts), draftPath, skipped, sourcePath)
}

// groupMissingByEquivalent maps each real-world gear name (coros_equivalent) with no known
// tonal_archetype to every coros_map.json key that shares it, so a single search covers
// every entry for the same physical gear rather than researching it once per map key.
// A coros_equivalent already labeled on ANY of its entries is treated as fully known --
// SelectedCaptureContext only needs one archetype per real-world gear name.
func groupMissingByEquivalent(mappings map[string]capturedGear) map[string][]string {
	known := make(map[string]bool)
	for _, v := range mappings {
		if v.IsCapture && v.CorosEquivalent != "" && v.TonalArchetype != "" {
			known[v.CorosEquivalent] = true
		}
	}

	missing := make(map[string][]string)
	for key, v := range mappings {
		if !v.IsCapture || v.CorosEquivalent == "" || known[v.CorosEquivalent] {
			continue
		}
		missing[v.CorosEquivalent] = append(missing[v.CorosEquivalent], key)
	}
	return missing
}

// countCapturesMissingEquivalent counts is_capture entries that are missing
// tonal_archetype AND have no coros_equivalent recorded at all -- groupMissingByEquivalent
// can't act on these (there's no real-gear name to search for), so they'd otherwise be
// silently absent from every count this tool reports.
func countCapturesMissingEquivalent(mappings map[string]capturedGear) int {
	count := 0
	for _, v := range mappings {
		if v.IsCapture && v.TonalArchetype == "" && v.CorosEquivalent == "" {
			count++
		}
	}
	return count
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
