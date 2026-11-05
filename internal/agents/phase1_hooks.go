package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Eval-only hooks into RunPipeline. They exist so quality evals can (1) run the
// Phase 1 agents once and feed byte-identical output to several pipeline
// variants (a paired comparison, which removes Phase 1's sampling noise), and
// (2) exercise the CorOS Librarian alone on frozen inputs. All of them are inert
// unless the caller puts the matching value in the context, so production
// behavior is unchanged.

// Phase1Result is what the Phase 1 agents produced for one run, plus the static
// catalog text the Librarian is given for the same run.
type Phase1Result struct {
	Tone            string `json:"tone"`
	Sonic           string `json:"sonic"`
	Scrape          string `json:"scrape"`
	LibrarianStatic string `json:"librarian_static,omitempty"`
}

// ErrStoppedAfterPhase1 is returned by RunPipeline when WithStopAfterPhase1 is set.
// It is the expected outcome in that mode, not a failure.
var ErrStoppedAfterPhase1 = errors.New("pipeline stopped after Phase 1 (eval hook)")

type phase1OverrideKey struct{}
type phase1CaptureKey struct{}
type stopAfterPhase1Key struct{}

// WithPhase1Override makes RunPipeline skip the Phase 1 agent calls and use p instead.
func WithPhase1Override(ctx context.Context, p Phase1Result) context.Context {
	return context.WithValue(ctx, phase1OverrideKey{}, &p)
}

// WithPhase1Capture makes RunPipeline write its Phase 1 results into dst once Phase 1 completes.
func WithPhase1Capture(ctx context.Context, dst *Phase1Result) context.Context {
	return context.WithValue(ctx, phase1CaptureKey{}, dst)
}

// WithStopAfterPhase1 makes RunPipeline return ErrStoppedAfterPhase1 right after Phase 1.
func WithStopAfterPhase1(ctx context.Context) context.Context {
	return context.WithValue(ctx, stopAfterPhase1Key{}, true)
}

func phase1OverrideFrom(ctx context.Context) *Phase1Result {
	p, _ := ctx.Value(phase1OverrideKey{}).(*Phase1Result)
	return p
}

func phase1CaptureFrom(ctx context.Context) *Phase1Result {
	p, _ := ctx.Value(phase1CaptureKey{}).(*Phase1Result)
	return p
}

func stopAfterPhase1(ctx context.Context) bool {
	b, _ := ctx.Value(stopAfterPhase1Key{}).(bool)
	return b
}

// LibrarianContextOldOrder is the pre-caching Librarian user turn (per-run text
// first, catalog after). It is used ONLY by evals as the comparison baseline;
// production uses buildLibrarianContext.
func LibrarianContextOldOrder(staticCatalog, toneResult, scrapeResult string, constraints map[string]interface{}) string {
	return "Tone History: " + toneResult + "\nScraper: " + scrapeResult + "\n" + staticCatalog + "Constraints: " + fmtConstraints(constraints)
}

// LibrarianContextNewOrder is the production Librarian user turn (see buildLibrarianContext).
func LibrarianContextNewOrder(staticCatalog, toneResult, scrapeResult string, constraints map[string]interface{}) string {
	return buildLibrarianContext(staticCatalog, toneResult, scrapeResult, constraints)
}

// RunLibrarian runs the CorOS Librarian alone on a prepared user turn, loading the
// prompt exactly as RunPipeline does (version from agentConfig, "_flash" variant by routing).
func (o *Orchestrator) RunLibrarian(ctx context.Context, agentConfig map[string]string, userContext string) (string, error) {
	sys, _ := o.loadPromptForAgent("4_coros_librarian", agentConfig["4_coros_librarian"])
	return o.RunAgentSplit(ctx, "CorOS Librarian", sys, userContext)
}

func fmtConstraints(c map[string]interface{}) string { return fmt.Sprintf("%v", c) }

// LibrarianReminder is the fixed sentence the production Librarian user turn ends with (see
// buildLibrarianContext). It names the rule by title, not number, because rule numbers differ
// between prompt versions.
const LibrarianReminder = "Reminder: before choosing any amp, cab or pedal, check the User Capture Library above and prefer an entry there that closely matches the tonal target (the \"Prefer User Captures\" rule)."

// LibrarianContextVariant builds the Librarian user turn for an EVAL-ONLY prompt-layout
// variant, used to test fixes for the catalog-first order's lower user-capture use (see
// docs/plans/2026-11-librarian-quality-results.md). Production uses buildLibrarianContext,
// which is the "reminder" layout.
//
//	"reminder":    the production layout (catalog first, LibrarianReminder at the end).
//	"no_reminder": catalog first without the reminder (the first cache-friendly layout).
//	"ucl_last":    Dictionary and Amp Menu first (cached), then Tone History and Scraper, then
//	            the User Capture Library, then Constraints -- restoring the library's position
//	            right before Constraints, at the cost of not caching the library itself.
func LibrarianContextVariant(variant, staticCatalog, toneResult, scrapeResult string, constraints map[string]interface{}) (string, error) {
	switch variant {
	case "reminder":
		return buildLibrarianContext(staticCatalog, toneResult, scrapeResult, constraints), nil
	case "no_reminder":
		return buildLibrarianContextNoReminder(staticCatalog, toneResult, scrapeResult, constraints), nil
	case "ucl_last":
		i := strings.LastIndex(staticCatalog, "User Capture Library: ")
		if i < 0 {
			return "", fmt.Errorf("static catalog has no User Capture Library section")
		}
		head, ucl := staticCatalog[:i], staticCatalog[i:]
		return head + "Tone History: " + toneResult + "\nScraper: " + scrapeResult + "\n" + ucl + "Constraints: " + fmtConstraints(constraints), nil
	}
	return "", fmt.Errorf("unknown Librarian layout variant %q", variant)
}
