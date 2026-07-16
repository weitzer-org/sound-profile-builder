// cmd/verify_user_captures is an offline batch tool, parallel to cmd/enrich_captures, that
// researches real citation-backed descriptions for user_captures.json's 87 entries. Unlike
// coros_map.json's tonal_archetype gap, user_captures.json already has a description on
// every entry -- but those descriptions were written by an earlier Claude Code session
// inferring gear identity from the (often cryptic) exported capture name alone, with no
// search grounding and no citation. This tool doesn't assume those existing descriptions
// are wrong; it independently re-derives one from the name alone (deliberately not shown
// the existing description, to avoid anchoring) and writes both side by side in the draft
// output for a human to compare. It never writes user_captures.json directly -- results go
// to a sibling draft file for review via a normal PR, same process as cmd/enrich_captures.
// Run with: GEMINI_API_KEY=... go run ./cmd/verify_user_captures
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// userCapture mirrors user_captures.json's per-entry structure.
type userCapture struct {
	Name        string `json:"name"`
	BlockType   string `json:"block_type"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

// verificationResult is the shape of Agent 15's structured JSON output, per the schema in
// internal/agents/schemas.go ("15_user_capture_verification").
type verificationResult struct {
	FoundReliableSource bool   `json:"found_reliable_source"`
	VerifiedDescription string `json:"verified_description"`
	Citation            string `json:"citation"`
}

// draftEntry pairs the existing (unverified) description with the freshly-researched one,
// so a human reviewer can see both and judge whether the original guess held up.
type draftEntry struct {
	BlockType           string `json:"block_type"`
	ExistingDescription string `json:"existing_description"`
	VerifiedDescription string `json:"verified_description"`
	Citation            string `json:"citation"`
	Source              string `json:"source"`
}

func main() {
	sourcePath := "internal/agents/user_captures.json"
	if len(os.Args) > 1 {
		sourcePath = os.Args[1]
	}
	draftPath := "internal/agents/user_captures.description.draft.json"
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

	var captures []userCapture
	if err := json.Unmarshal(raw, &captures); err != nil {
		log.Fatalf("Failed to parse %s: %v", sourcePath, err)
	}
	log.Printf("Found %d user captures to independently verify.", len(captures))

	if limitStr := os.Getenv("VERIFY_LIMIT"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			log.Fatalf("Invalid VERIFY_LIMIT %q: must be a non-negative integer", limitStr)
		}
		if limit < len(captures) {
			log.Printf("VERIFY_LIMIT=%d set -- only researching the first %d of %d captures (smoke-test mode).", limit, limit, len(captures))
			captures = captures[:limit]
		} else {
			log.Printf("VERIFY_LIMIT=%d set, but only %d captures found -- researching all of them.", limit, len(captures))
		}
	}

	ctx := context.Background()
	orch, err := agents.NewOrchestrator(ctx, apiKey, nil)
	if err != nil {
		log.Fatalf("Failed to init orchestrator: %v", err)
	}
	defer orch.Close()

	sysPrompt, err := agents.LoadPrompt("15_user_capture_verification", "")
	if err != nil {
		log.Fatalf("Failed to load User Capture Verification prompt: %v", err)
	}

	drafts := make(map[string]draftEntry) // keyed by capture name (confirmed unique)
	skipped := 0

	// Sequential, not concurrent: this is a low-frequency, one-off offline sweep with no
	// user waiting, so simplicity beats throughput here.
	for _, c := range captures {
		log.Printf("Researching %q (%s)...", c.Name, c.BlockType)

		// Deliberately does NOT include c.Description -- the model researches fresh from
		// the name alone, so it can't just rubber-stamp the existing (possibly wrong) guess.
		userPrompt := fmt.Sprintf("Capture name: %s\nBlock type: %s", c.Name, c.BlockType)
		result, err := orch.RunAgentSplit(ctx, "User Capture Verification", sysPrompt, userPrompt)
		// Pace every call the same way regardless of outcome -- see cmd/enrich_captures for
		// why this sits right after the call rather than at the bottom of the loop.
		time.Sleep(500 * time.Millisecond)
		if err != nil {
			log.Printf("  Search failed for %q: %v -- skipping", c.Name, err)
			skipped++
			continue
		}

		var parsed verificationResult
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			log.Printf("  Failed to parse response for %q: %v -- skipping. Raw: %s", c.Name, err, result)
			skipped++
			continue
		}

		if !parsed.FoundReliableSource {
			log.Printf("  No reliable source found for %q -- skipping (needs manual research).", c.Name)
			skipped++
			continue
		}

		if parsed.VerifiedDescription == "" || parsed.Citation == "" {
			log.Printf("  %q claimed a reliable source but gave an empty description/citation -- skipping (malformed response).", c.Name)
			skipped++
			continue
		}

		drafts[c.Name] = draftEntry{
			BlockType:           c.BlockType,
			ExistingDescription: c.Description,
			VerifiedDescription: parsed.VerifiedDescription,
			Citation:            parsed.Citation,
			Source:              "search_grounded",
		}
		log.Printf("  existing: %q", c.Description)
		log.Printf("  verified: %q (citation: %s)", parsed.VerifiedDescription, parsed.Citation)
	}

	out, err := json.MarshalIndent(drafts, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal draft output: %v", err)
	}
	if err := os.WriteFile(draftPath, out, 0644); err != nil {
		log.Fatalf("Failed to write %s: %v", draftPath, err)
	}

	log.Printf("Wrote %d verified descriptions to %s (%d skipped -- no reliable source or malformed response). Review and merge into %s manually via a normal PR.", len(drafts), draftPath, skipped, sourcePath)
}
