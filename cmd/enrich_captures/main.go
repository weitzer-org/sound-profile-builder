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

	missing := missingKeys(mappings)
	if len(missing) == 0 {
		log.Printf("Every is_capture entry already has a tonal_archetype. Nothing to research.")
		return
	}
	log.Printf("Found %d capture entries missing tonal_archetype.", len(missing))

	// Resumable: if a draft file already exists at draftPath (e.g. from a prior run that
	// hit truncation/parse failures on some names), load it and skip any real-gear name
	// it already covers, rather than re-paying for names that already succeeded.
	drafts := make(map[string]draftEntry) // keyed by coros_map.json map key
	if existing, err := os.ReadFile(draftPath); err == nil {
		if err := json.Unmarshal(existing, &drafts); err != nil {
			log.Fatalf("Existing draft file %s is not valid JSON, refusing to overwrite blindly: %v", draftPath, err)
		}
		log.Printf("Loaded existing draft %s: %d entries already covered -- skipping those.", draftPath, len(drafts))
	}

	names := missing
	remaining := names[:0]
	for _, key := range names {
		if _, ok := drafts[key]; !ok {
			remaining = append(remaining, key)
		}
	}
	names = remaining

	if limitStr := os.Getenv("ENRICH_LIMIT"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			log.Fatalf("Invalid ENRICH_LIMIT %q: must be a non-negative integer", limitStr)
		}
		if limit < len(names) {
			log.Printf("ENRICH_LIMIT=%d set -- only researching the first %d of %d remaining names (smoke-test mode).", limit, limit, len(names))
			names = names[:limit]
		} else {
			log.Printf("ENRICH_LIMIT=%d set, but only %d remaining names found -- researching all of them.", limit, len(names))
		}
	}

	if len(names) == 0 {
		log.Printf("Nothing left to research -- every actionable name is already covered by the existing draft.")
		return
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

	skipped := 0

	// Sequential, not concurrent: this is a low-frequency, one-off (or periodic) offline
	// sweep with no user waiting, so simplicity beats throughput here.
	for _, key := range names {
		gear := mappings[key]
		log.Printf("Researching %q...", key)

		userPrompt := fmt.Sprintf("Research the real-world tonal character of: %s", key)
		if gear.CorosEquivalent != "" {
			userPrompt += fmt.Sprintf("\n(For context only, not the research subject: this capture's on-device native block is labeled %q -- that label may be a generic or obfuscated placeholder name and is not itself a real product.)", gear.CorosEquivalent)
		}
		result, err := orch.RunAgentSplit(ctx, "Capture Enrichment", sysPrompt, userPrompt)
		// Pace every call the same way regardless of outcome -- placed right after the
		// call (not at the bottom of the loop) so an error path's `continue` can't skip
		// it. Backoff matters most exactly when calls are failing, e.g. a degraded or
		// rate-limited API; a sleep only the success path hits would remove it right
		// when it's needed most.
		time.Sleep(500 * time.Millisecond)
		if err != nil {
			log.Printf("  Search failed for %q: %v -- skipping", key, err)
			skipped++
			continue
		}

		var parsed enrichmentResult
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			log.Printf("  Failed to parse response for %q: %v -- skipping. Raw: %s", key, err, result)
			skipped++
			continue
		}

		if !parsed.FoundReliableSource {
			log.Printf("  No reliable source found for %q -- skipping (needs manual research).", key)
			skipped++
			continue
		}

		// tonal_archetype/citation are schema-optional (see schemas.go) precisely so the
		// model never has to fabricate either just to produce valid JSON when it has
		// genuinely found nothing -- so a true found_reliable_source claim paired with an
		// empty value here is a malformed response, not a legitimate "no data" case.
		if parsed.TonalArchetype == "" || parsed.Citation == "" {
			log.Printf("  %q claimed a reliable source but gave an empty tonal_archetype/citation -- skipping (malformed response).", key)
			skipped++
			continue
		}

		label := parsed.TonalArchetype
		isNew := label == newCategoryValue
		if isNew {
			if parsed.NewCategoryLabel == "" {
				log.Printf("  %q proposed a new category but gave no label -- skipping.", key)
				skipped++
				continue
			}
			label = parsed.NewCategoryLabel
		} else {
			// Scrub a field that should only ever be meaningful in the New Category case --
			// don't trust that the model left it empty just because tonal_archetype wasn't
			// New Category.
			parsed.NewCategoryJustification = ""
		}

		drafts[key] = draftEntry{
			CorosEquivalent:          gear.CorosEquivalent,
			ProposedTonalArchetype:   label,
			IsNewCategory:            isNew,
			NewCategoryJustification: parsed.NewCategoryJustification,
			Citation:                 parsed.Citation,
			Source:                   "search_grounded",
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

// missingKeys returns every coros_map.json map key (the real-world gear name --
// coros_map.json's schema is real-world name -> QC on-device block name in CorosEquivalent)
// whose is_capture entry has no tonal_archetype yet, sorted for deterministic runs. This used
// to dedup and research by CorosEquivalent on the assumption that entries sharing an
// on-device block name are the same physical gear -- that assumption was wrong: the QC's
// native block names are often obfuscated/generic (e.g. "Chief Bass Overdrive" for a Boss
// ODB-3, "Love Drive 11" shared by three unrelated captures), so researching THAT string
// produced wrong-gear citations and collapsed distinct captures into one shared (wrong)
// answer. Each map key is already a unique real-world gear name, so no deduping is needed.
func missingKeys(mappings map[string]capturedGear) []string {
	var missing []string
	for key, v := range mappings {
		if v.IsCapture && v.TonalArchetype == "" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}
