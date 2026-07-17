// cmd/verify_user_captures is an offline batch tool, parallel to cmd/enrich_captures, that
// researches real citation-backed descriptions for user_captures.json entries. Unlike
// coros_map.json's tonal_archetype gap, user_captures.json already has a description on
// every entry -- but those descriptions were written by an earlier Claude Code session
// inferring gear identity from the (often cryptic) exported capture name alone, with no
// search grounding and no citation. This tool doesn't assume those existing descriptions
// are wrong; it independently re-derives one from the name alone (deliberately not shown
// the existing description, to avoid anchoring) and writes both side by side in the draft
// output for a human to compare. It never writes user_captures.json directly -- results go
// to a sibling draft file for review via a normal PR, same process as cmd/enrich_captures.
//
// Only researches captures where description_verified is not yet true (see userCapture) --
// so after new captures are downloaded and added to user_captures.json, re-running this
// tool only researches the new ones, not the entire library again. Set
// description_verified=true on an entry in user_captures.json when merging its approved
// verified_description from the draft file; that's what makes future runs skip it.
//
// Run with: GEMINI_API_KEY=... go run ./cmd/verify_user_captures
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// userCapture mirrors user_captures.json's per-entry structure. DescriptionVerified is the
// skip signal for already-covered entries: unlike coros_map.json (where an empty
// tonal_archetype is itself the "still needs research" marker), every user_captures.json
// entry already has *some* description by construction, so there's no natural empty-field
// signal -- this explicit marker is what lets a future run target only newly-added
// captures instead of re-researching everything each time. Set it to true in
// user_captures.json when merging an approved verified_description from the draft file.
type userCapture struct {
	Name                string `json:"name"`
	BlockType           string `json:"block_type"`
	Description         string `json:"description"`
	Source              string `json:"source"`
	DescriptionVerified bool   `json:"description_verified,omitempty"`
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

	var allCaptures []userCapture
	if err := json.Unmarshal(raw, &allCaptures); err != nil {
		log.Fatalf("Failed to parse %s: %v", sourcePath, err)
	}
	log.Printf("Found %d user captures total.", len(allCaptures))

	captures := filterUnverified(allCaptures)
	if alreadyVerified := len(allCaptures) - len(captures); alreadyVerified > 0 {
		log.Printf("%d captures already have description_verified=true -- skipping those (only new/unverified captures are researched).", alreadyVerified)
	}
	log.Printf("%d captures need research.", len(captures))

	// Resumable: if a draft file already exists at draftPath (e.g. from a prior run that
	// hit truncation/parse failures on some captures), load it and skip any capture name
	// it already covers, rather than re-paying for ones that already succeeded.
	drafts := make(map[string]draftEntry) // keyed by capture name (confirmed unique)
	existingDraft, err := os.ReadFile(draftPath)
	if err == nil {
		if err := json.Unmarshal(existingDraft, &drafts); err != nil {
			log.Fatalf("Existing draft file %s is not valid JSON, refusing to overwrite blindly: %v", draftPath, err)
		}
		log.Printf("Loaded existing draft %s: %d captures already covered -- skipping those.", draftPath, len(drafts))
	} else if !errors.Is(err, os.ErrNotExist) {
		// Anything other than "no draft yet" (permissions, I/O error, etc.) must not be
		// silently treated as a fresh start -- that would overwrite an existing,
		// merely-unreadable draft and lose whatever progress it held.
		log.Fatalf("Failed to read existing draft %s: %v", draftPath, err)
	}
	remaining := captures[:0]
	for _, c := range captures {
		if _, ok := drafts[c.Name]; !ok {
			remaining = append(remaining, c)
		}
	}
	captures = remaining

	if limitStr := os.Getenv("VERIFY_LIMIT"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			log.Fatalf("Invalid VERIFY_LIMIT %q: must be a non-negative integer", limitStr)
		}
		if limit < len(captures) {
			log.Printf("VERIFY_LIMIT=%d set -- only researching the first %d of %d remaining captures (smoke-test mode).", limit, limit, len(captures))
			captures = captures[:limit]
		} else {
			log.Printf("VERIFY_LIMIT=%d set, but only %d remaining captures found -- researching all of them.", limit, len(captures))
		}
	}

	if len(captures) == 0 {
		log.Printf("Nothing left to research -- every capture is already covered by the existing draft.")
		return
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

		// Persist after every success, not just at the end -- a long run against a live,
		// occasionally slow/rate-limited API can get killed by an external timeout well
		// before it finishes, and only ever writing once at the bottom of the loop would
		// lose the entire run's progress in that case.
		if err := writeDraftFile(draftPath, drafts); err != nil {
			log.Fatalf("Failed to persist draft progress to %s: %v", draftPath, err)
		}
	}

	log.Printf("Wrote %d verified descriptions to %s (%d skipped -- no reliable source or malformed response). Review and merge into %s manually via a normal PR.", len(drafts), draftPath, skipped, sourcePath)
}

// writeDraftFile writes drafts to path atomically (write to a temp file in the same
// directory, then rename over the target) so a crash or kill mid-write can never leave a
// truncated or corrupt draft file behind.
func writeDraftFile(path string, drafts map[string]draftEntry) error {
	out, err := json.MarshalIndent(drafts, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// filterUnverified returns only the captures that haven't already been marked
// description_verified -- so re-running this tool after new captures are added only
// researches the new ones, not the entire library again. Set description_verified=true in
// user_captures.json when merging an approved verified_description from the draft file.
func filterUnverified(captures []userCapture) []userCapture {
	unverified := make([]userCapture, 0, len(captures))
	for _, c := range captures {
		if !c.DescriptionVerified {
			unverified = append(unverified, c)
		}
	}
	return unverified
}
