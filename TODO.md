# Project Backlog & To-Do

## Pipeline Quality Work — Status (as of 2026-07-16)

A multi-tier effort to raise generation quality without a structural rewrite,
evaluated each time against the same 12-prompt golden set + blind pairwise
LLM judge. Tiers 0-2 are merged to `main`; Tier 3 (agent wiring/context
fixes, as opposed to another detection layer) is in progress -- see below.

- **Tier 0** (PR #63) — structural fix for a real drift bug: capture-only
  relative-dB parameter formatting was leaking onto algorithmic (non-capture)
  Amplifier blocks. Root-caused and fixed at the source.
- **Tier 1** (PR #69) — `qc_block_schema.json` (manual-sourced QC parameter
  vocabulary), the `basis` confidence field on every parameter
  (confirmed_range/real_gear_analog/engineering_convention/estimate, so a
  value's provenance is always visible and hedging is never silent), search
  grounding for Sonic Profiler, `SelectedCaptureContext` (capture descriptive
  color), and `FlagCaptureFormattingMismatches` (deterministic detection of
  the Tier 0 bug class, in case it recurs).
- **Tier 2** (PR #71) — a detect-don't-silently-fix consistency layer:
  `FlagIncompleteCabinetBlocks` (Rule 12 Mic 1/Mic 2/Blend) and
  `FlagLeftoverValueRanges` (Rule 6 no ranges) deterministic checks, plus a
  13th advisory **Preset Critic** agent (`internal/agents/critic.go`,
  `prompts/13_critic.md`) that re-reads the Architect's own output for
  prose-vs-data contradictions no deterministic check can catch (e.g. a
  rationale claiming "tape echo" on a block whose model is a digital delay).
  Findings are appended to `Rationale`, never used to mutate the editable
  `Value`/`ValueB` fields. Validated cheaply pre-integration via
  `cmd/critic_probe` (runs the critic against on-disk output, no new
  generation) — this is how a Bypass-semantics false positive
  (`Bypass: On` = bypassed in this pipeline, not the everyday reading) was
  caught and fixed before it ever reached a live eval.
- Also shipped alongside this track: an urgent production reliability
  hotfix (`hotfix/max-output-tokens-caps`, PR #70) after Tier 2 development
  surfaced a ~50% `MaxOutputTokens` truncation failure rate spread across
  several previously-reliable agents (see the dedicated bullet under Known
  Bugs & Investigations below).

**Deliberate tradeoffs / open critiques worth knowing before extending this
track further:**
- The Preset Critic pattern (an agent re-reading the same model's own
  output) is architecturally shallower than Tier 0's approach of fixing the
  generation logic itself — a fair critique, kept as a conscious tradeoff
  rather than a fourth prompt-tuning round.
- The critic path has **no e2e or mock-mode test coverage** — `MOCK_MODE`
  returns before it ever runs, so it's validated only via `critic_probe` +
  live runs + Go unit tests on `applyCriticFindings`. Worth deciding whether
  that's sufficient or needs a mock-mode fixture.
- Cabinet mic-placement parameter names (`Mic 1`/`Mic 2`/`Blend` /
  `mic_1_pos`/`mic_2_pos`/`blend_ratio`) are now hardcoded independently in
  four places: `schemas.go`, `prompts/7_transducer_tech.md`,
  `prompts/12_architect_v4.md`, and `fuzzy_matcher.go`'s
  `cabinetMicParamTokens`. Low urgency (prompts and schema naturally
  hardcode vocabulary) but worth remembering before a fifth copy appears.
- `agentTemperature`, `agentSearchTool`, and `agentMaxOutputTokens`
  (`internal/agents/schemas.go`) remain three independent maps with nothing
  enforcing they stay in sync across all 13 agent keys — still not unified
  (see the fuller entry further down).

**Tier 3 — agent wiring/context fixes, not another detection layer:**

- **Sonic Profiler token-bloat parity fix** (PR #75, merged to `main`) —
  Sonic Profiler's output schema (`eq_profile`/
  `suggested_low_cut_hz`/`suggested_high_cut_hz`, `saturation_style`,
  `reverb_type`, `noise_gate_target_db`) has no field that names a specific
  gear/capture, but it was receiving the full 12-category
  `qc_block_schema.json` plus the entire user capture library on every run --
  the same token-bloat pattern already fixed for Acoustician in Tier 1.
  Added `GetQCSonicProfilerSchemaJSON()` (scoped to
  `global_eq`/`drive`/`reverb`/`noise_gate` only), dropped the capture
  library from its context, shipped as `prompts/2_sonic_profiler_v4.md` per
  the prompt-versioning convention (`v3` untouched, `config.json` pin
  bumped). Validated against the live golden set + blind pairwise judge:
  Sonic Profiler input tokens -72.8% (85,709 -> 23,353 across 11 comparable
  prompts), whole-pipeline input tokens -9.2%, output/latency flat, judge
  verdict a dead heat (5 pre-fix / 5 post-fix / 1 equal, none of the 11
  rationales citing anything Sonic Profiler's schema actually controls).
  `01_SRV_Clean` excluded from the comparison -- failed post-fix on
  Acoustician hitting Gemini's `MaxOutputTokens` cap, the same
  pre-existing flaky failure mode described below, unrelated to this
  change. GSR's deep-review (agent-swarm) pass on this PR also flagged,
  and this branch fixed: an invalid `//` comment inside the JSON schema
  block and a `"-65dB"`-style string example on a numeric field (both real
  risk only on the Open-LLM gateway route, which has no schema/
  response-format enforcement unlike Gemini's `ResponseSchema` path; both
  pre-existing text carried over from `v3` verbatim) and a partial-schema-
  drift gap in `subsetQCBlockSchema`'s fallback (now falls back to the full
  file if even one requested category is missing, not just when all are).
  Declined from that same review pass: extracting a `SchemaProvider`
  interface + DI and moving the `sync.Once` caches off package scope
  (premature abstraction for a ~15-line pure function over one static
  embedded file -- the caching pattern predates this PR, see Tier 1's
  `GetQCAmpEQSchemaJSON`), and a prompt-injection delimiter fix scoped to
  just this one agent (the underlying unguarded-interpolation pattern is
  identical at two other pre-existing call sites -- Tone Historian,
  Community Scraper -- so a one-off fix here would be inconsistent; see the
  new backlog item below if a real pipeline-wide pass is wanted). Reasoning
  for all four is posted on the PR review thread.
- **`RefineChat`'s missing `SelectedCaptureContext`** (PR #77, merged to
  `main`) -- `RefineChat` doesn't re-run Librarian/Navigator, so it had no
  raw text to scan for capture names the way `RunPipeline` does.
  `SelectedCaptureContext` just token-scans whatever text it's given, and
  `RefineChat` already has `p.Payload` (the existing structured preset)
  containing the actual selected block model names as plain JSON text --
  arguably better data than raw Librarian/Navigator output, since it's the
  real final selection. Threaded `allowFactoryCaptures`/`allowUserCaptures`
  through `RefineChat`'s signature (interface, real impl, 4 test mocks,
  `orchestrator_test.go`, `cmd/eval_refine`) and hoisted
  `handlers_preset.go`'s allow-flag computation above the `RefineChat` call.
  Validated: unit tests (including a new test that inspects the actual
  outgoing request body), a full mock-mode e2e run diffed against
  unmodified `main` as baseline (identical pre-existing failures, no
  regression), and an independent Opus review (no severe correctness
  findings).
  **Live proof-of-benefit investigation (not just safety) surfaced a real,
  separate data-coverage gap**, not a code bug: of `coros_map.json`'s 192
  `is_capture: true` entries, only 14 (7.3%) have any `tonal_archetype`
  value at all, and only 3 of those 14 carry a substantive label
  (`High-Gain Modern` x2, `British Crunch` x1) rather than the near-content-
  free catch-all `Other / Unique`. Cross-referencing every available
  golden-set fixture (~13 prompts x 6 model tiers) found the *only*
  archetype-bearing capture any golden generation ever actually selects is
  `US DLX 58` -- labeled `Other / Unique`. A live A/B (capture context on
  vs. forced empty via a temporary env toggle, since removed) on that one
  case showed only noise-level (~0.5dB) divergence, as expected for a
  content-free label. A follow-up synthetic-preset A/B forcing a
  substantively-labeled capture (`Bogna X100B Ch1` -> `High-Gain Modern`)
  showed a more structurally interesting split -- the context-on arm
  produced a scooped-mid/boosted-treble EQ shape consistent with that
  archetype, the context-off arm left Mid/Treble untouched -- but at n=1
  with no temperature pinning this is suggestive, not proof; indistinguishable
  from ordinary sampling variance without repeat runs. Closing verdict
  (from an Opus review of the full investigation): the fix is correct,
  safe, and plausibly influential, but efficacy beyond that is not cheaply
  provable given current data sparsity and is diminishing-returns to chase
  further. **Follow-up worth doing, if this is revisited**: expand
  `tonal_archetype` coverage in `coros_map.json` beyond the current 7.3% --
  that's the actual bottleneck on this fix (and any future capture-color
  work) having real-world impact, not the wiring.

  **Design session on how to close that gap (2026-07-16), not yet
  started.** First instinct was to hand this to Ben directly as content
  research, not an agent task -- reconsidered after the point was raised
  that this codebase already has a proven live-search-grounding pattern
  (Sonic Profiler's `GoogleSearch` tool, `internal/agents/orchestrator.go`)
  that could plausibly research real gear's tonal character itself. Two
  designs were worked through; the offline/PR-gated one below is the
  chosen direction, the live one is documented and deliberately deferred.

  *Chosen: offline batch tool, human review via normal PR, no new
  infrastructure beyond a `cmd/` tool and one queue key.* End-to-end:
  1. A new `cmd/enrich_captures` tool (same shape as `cmd/eval_refine`/
     `cmd/validate_mappings`) walks every `coros_map.json` entry with
     `is_capture: true` and no `tonal_archetype` (178 of 192 today).
  2. For each, one Gemini call with the `GoogleSearch` tool researches
     that specific `coros_equivalent` real-world gear name and proposes a
     label. Output is constrained to the existing 3-value enum
     (`British Crunch` / `High-Gain Modern` / `Other / Unique`) plus an
     explicit "propose a new category" escape hatch that requires extra
     justification in its own field -- prevents 178 near-synonym
     freeform phrasings fragmenting today's small, coherent vocabulary.
     Any capture the search can't find a real citeable source for is left
     blank rather than guessed.
  3. Results are written to a sibling draft file
     (`coros_map.tonal_archetype.draft.json`), never mutating the real
     file directly. Each draft entry carries the label, the citation/
     source the search surfaced, and a `basis`-style confidence tag
     (reusing this codebase's existing `manual_confirmed` /
     `engineering_convention` convention from `qc_block_schema.json`,
     e.g. `search_grounded_reviewed` once approved) -- `coros_map.json`
     has no such field on its capture entries today, so this would be the
     first time one exists there, extending a pattern proven elsewhere
     rather than inventing a new one.
  4. The tool's output becomes a normal branch + PR against
     `coros_map.json`, with each proposed label, its citation, and its
     confidence tag laid out in the PR description so a reviewer isn't
     reading opaque JSON. Ben reviews it exactly like any other PR --
     reads the citations, edits/rejects individual entries in the branch,
     lets GSR/gemini-code-assist/CodeRabbit run their usual pass -- no new
     admin UI, approve/reject button, or reviewer role needed. The
     project's existing PR habit *is* the approval gate. This also means
     the live generation pipeline is completely unaffected while review is
     pending -- nothing ships until the PR merges and a new build deploys.
  5. Re-run periodically (manually, or later via a scheduled job) to sweep
     for newly-added captures with no label -- same mechanism, just
     triggered again, not a different one.
  Roughly 178 calls, bounded one-time cost, no live latency impact on any
  real user request.

  *Considered and deferred: live, on-demand derivation during actual
  generation/refinement.* On a `SelectedCaptureContext` miss, fire one
  grounded search for just that capture (1-3 per run, not all 192 --
  meaningfully cheaper than the Acoustician grounding revert, which
  blew timeouts by searching on nearly every prompt against a much
  heavier schema). Two viable sub-designs surfaced:
  - *Use it immediately in that same request* -- simplest, zero new
    infrastructure, but means an unreviewed, possibly-hallucinated real-
    gear description steers a live Rule 9 dB-formatting decision before
    anyone has looked at it, with no visible marker distinguishing it
    from a vetted label -- exactly the "hedging must never be silent"
    failure mode Tier 1's `basis` field exists to prevent. Also
    non-reproducible (same capture could get a different, possibly
    contradictory, un-cached label in a different session) and pays the
    same search cost every time a popular capture is used, since nothing
    persists -- never actually closes the coverage gap, just repeats the
    same cost against it indefinitely.
  - *Derive live, but only ever persist as an unapproved draft; the
    current request still falls back to today's safe "no color, general
    knowledge" behavior* -- this is not a second mechanism, it's the same
    queue-and-review pipeline above with a second trigger (a real cache
    miss, in addition to a batch sweep). Doesn't cost the current request
    any correctness risk, still requires building the same overlay-store
    write path.
  Deferred for now: the offline batch sweep alone already clears nearly
  the entire backlog (178 of 192) in one pass, so the marginal value of
  also wiring a live trigger is small until real usage patterns surface
  gaps the batch pass missed (e.g. a brand-new capture added to a future
  `coros_map.json` update). Revisit the live-trigger addition then, not
  before -- it's a small additive change once the batch tool and draft
  file format exist, not a reason to delay building those.
- **Cabinet manual-research-vs-actual-model reconciliation** (not yet
  started, needs a research spike before any code change) -- still an open
  question, not a wiring fix: is the real QC hardware IR-loader-only per
  `qc_block_schema.json`'s manual research, or does it have the continuous
  mic-placement controls (`Mic 1`/`Mic 2`/`Blend`) every actual eval output
  generates? Needs the Cortex Control app cross-check mentioned elsewhere
  in this doc before either model is treated as ground truth. See issues
  #64-#68 for the full history and tradeoffs on all three candidates.

**New from this session, not yet triaged:**
- **Pipeline-wide prompt-injection hardening** -- GSR's deep-review swarm
  (see below) flagged unguarded user-tone-text interpolation
  (`"User Request: " + prompt` / `fmt.Sprintf("User Request: %s...", prompt, ...)`)
  as a prompt-injection risk on Sonic Profiler specifically, but the same
  pattern is identical at two other call sites (Tone Historian, Community
  Scraper, `orchestrator.go`) that predate any Tier work. Real impact is
  low today (Sonic Profiler's search tool is Gemini's built-in `GoogleSearch`
  grounding, not an arbitrary-fetch capability, and its output is
  schema-bounded), but if this gets hardened it should be one deliberate
  pass across all agent context-construction call sites with a single
  consistent delimiter convention, not three ad hoc fixes accumulated
  PR-by-PR. A fourth site, arguably higher-impact than the first three:
  GSR's deep-review on PR #77 (the `RefineChat` capture-context fix)
  flagged `RefineChat`'s `EXISTING STRUCTURED PAYLOAD` block
  (`internal/agents/orchestrator.go`, `refinementPrompt`), which raw-
  interpolates `p.Payload` -- this one predates PR #77 (PR #77 only added
  one more `%s` for `captureContext`, sourced from a controlled static
  lookup, not user text) but is a real, distinct injection vector: unlike
  the other three sites' LLM-generated tone-prompt text, `p.Payload` can
  contain rationale/model text the user directly edited via the Tweaking
  Workspace before saving. Declined as an ad hoc fix in PR #77 for the
  same reason as the other three -- fold it into the eventual single
  hardening pass, not a fifth one-off patch.
- **GSR deep-review (agent-swarm) mode is now live** (PR #74, merged) --
  `.github/workflows/gsr-review-deep.yml` runs the full GSR agent swarm
  (Architecture/Logic/Security/TechDebt/Testing agents + dedup pass,
  `mode: subagent`) on a PR when the `deep-review` label is applied (or on
  new commits to a PR that already has it). Opt-in only, mirrors the
  `/code-review high` escalation pattern for large/risky changes. Runs on
  GSR's own Gemini key, zero Claude quota. Also merged alongside it (PR
  #76): `/quick-review` demoted from "run on every PR" to on-demand, since
  GSR's basic-mode review (`gsr-review.yml`) already covers every PR at
  zero Claude quota -- see the updated Code Review section of this repo's
  `CLAUDE.md` for the current default review policy.

## Features & Enhancements

- **Interactive Post-Generation Tweaking**: Build a follow-up conversational thread UI/architecture where the user can chat with the Orchestrator LLM to recursively tweak the generated results, give feedback, or ask questions about the generated DSP blocks.
- **Guitar-Side Setup Recommendations**: Generation and chat adjustments should recommend pickup selector position and guitar Volume/Tone knob settings, not just Quad Cortex device parameters -- today the pipeline already reasons about pickup type internally (Acoustician's humbucker/single-coil split) but never surfaces guitar-side guidance to the player. See [issue #67](https://github.com/weitzer-org/sound-profile-builder/issues/67) for scope and open design questions (schema placement, pickup-position vocabulary risk, eval coverage). Unrelated to the Tier 0/Tier 1 quality track -- a separate feature.
- **Free Cloud Capture Mapping**: Investigate a mechanism to keep a local copy of Cloud Navigator content specifically tailored to enforce sourcing *only* free presets on the Cortex Cloud. (The initial `cloud_captures_free.json` map concept was paused pending further UX/architectural refinement).
- **Progress Indicator**: Add a progress indicator while the agent is running.
- **Multi-Guitar Run**: Run all guitars at the same time with a tabbed experience.
- **Bug Fix**: Fix bugs with save preset functionality.
- **UI Improvement**: Improve the UI around the agent log.
- **Token Optimization**: Optimize overall prompt/API token usage.
- **Token Monitoring**: `TokenUsage` (`internal/agents/orchestrator.go`) only tracks aggregate `InputTokens`/`OutputTokens` across the whole pipeline run, plus a call *count* per model (`ModelsUsed`) -- there's no per-model token breakdown and no timing at all. Per-call numbers do get logged (`AGENT_TOKEN_LOG: Agent=X In=Y Out=Z Model=W`) but aren't persisted or surfaced in the UI. Needs:
    - Per-model input/output token totals, not just an aggregate across every model combined.
    - Total wall-clock time for the run, and ideally per-agent/per-call timing (nothing is timed anywhere today).
    - Surface both (aggregate + per-model + timing) in the UI, not just the current combined "Input: X | Output: Y | Models: A (n), B (m)" line.
- **Gemini Flash Profile Generation**: Test whether using a Gemini Flash call to generate preset metadata (Profile Name, Inspired By / Goal, Core Tone Source, Key Characteristics, Good For) yields better results.
- **Default Preset Name**: Have the preset name defaulted to the tone prompt prior to saving.
- **Temporary Presets Section**: Include non-saved presets as a separate section in the saved presets for X time.
- **Preset Library**: Build out a preset library.
- **Track Adjustments**: Only the capture half of this exists. Every chat refinement already auto-saves a "Learned Rule" memory (`handlers_preset.go:1025`, via `memoryStore.Save`), and `/api/memories` displays them -- but nothing ever reads them back. Confirmed `memoryStore.List` is only ever called from the read-only display endpoint (`handleGetMemories`), never from the orchestrator or the generation path. So today Learned Rules are captured and shown to the user, but have zero influence on future generations. The actual remaining work is wiring memory retrieval into `RunPipeline`/`RefineChat` so past corrections actually inform new output. See also [issue #66](https://github.com/weitzer-org/sound-profile-builder/issues/66) (broader version: full saved presets, not just chat corrections).
- **Prior Presets Informing Generation**: Part of the quality work -- today every generation starts from zero, with no use of the user's own saved-preset history. See [issue #66](https://github.com/weitzer-org/sound-profile-builder/issues/66) for full pros/cons and a proposed (not yet implemented) path: cheap heuristic matching before embeddings, condensed summaries injected (not full preset JSON, to avoid repeating this round's token-bloat mistake), validated with the same golden-set/blind-judge methodology as Tier 0/Tier 1 before shipping.
- **(Placeholder) Second quality-related idea**: flagged mid-session (2026-07-13) as a second item to track alongside prior-presets-informing-generation, but the specifics weren't recalled at the time. Revisit with the user to fill this in -- don't invent content for it.
- **Capture Toggle**: Add a UI toggle to allow or restrict the multi-agent pipeline from sourcing Factory Captures during preset generation.
- **Paid Plugin Toggle**: Add a UI configuration to selectively enable/disable routing through Paid Plugin architectures (e.g., Nameless, Cory Wong, Plini).
- **YouTube Tone Analysis Agent**: Research integrating an agent to search for YouTube videos representing the prompt and analyze the tone from the video audio/context to inform preset generation.
- **Change Default Presets**: Add/Update standard presets to include: Rhythm, Clean Boost, Overdrive, and Comp.
- **Performance Optimization**: Improve performance of the Preset Library loading. It currently feels slow. Options include:
    - *Option A (Easy)*: Implement an in-memory TTL cache (e.g., 30 seconds) on the listing handler to eliminate repeated GCS reads.
    - *Option B (Structured)*: Refactor storage to extract thin metadata index files for lists, instead of handling flat heavy-file scans.
    - *Option C (Test-Specific)*: Hardcode dry mock returns specifically labeled for E2E speed checks in testing routes.
- **CSRF Protection**: No CSRF protection exists on any state-changing route (save/delete/copy/rename preset, generate, chat-refine) -- confirmed zero `csrf` references anywhere in `internal/api`. The HMAC session cookie alone doesn't defend against cross-site request forgery.
- **Rate Limiting**: No rate limiting exists on `/api/preset/generate` or `/api/preset/chat`, both of which call the paid Gemini API. Nothing currently stops a compromised session or a client-side bug from hammering either endpoint and running up API cost.
- **Per-User Ownership Model**: `storage.Preset` has no `owner`/`user_id` field -- confirmed via grep. Every authenticated session shares one `MOCK_PASSWORD` and can view/edit/delete any preset. Decide whether this is acceptable long-term (single-team internal tool) or needs per-user scoping.
- [x] ~~**Live-Mode Playwright Test Convention**~~: `tests/e2e/live_capture.spec.js`, `live_recheck.spec.js`, and `builder_statement_check.spec.js` (real-Gemini-data verification) are now committed, along with their `tests/e2e/live-shots/` screenshots. Still open: no explicit naming/gating convention exists yet (e.g. a `*.live.spec.js` pattern excluded from the normal mock-mode run since these cost real API calls) -- they currently rely on being manually invoked rather than being structurally separated from the mock-mode suite.

## Known Bugs & Investigations

- [ ] Add effect rationale to the fallback table renderer in `handlers_preset.go` when available in StructuredPreset.
- [ ] Fix `#workspace-wrapper` duplicate-ID bug (confirmed, not just suspected): `renderTweakingWorkspaceHTML` hardcodes `id="workspace-wrapper"` on its root div and is called for both the Generator draft workspace and the Library Adjust workspace, both of which can be present in the DOM at once (one merely `display:none`). The refinement chat form's `hx-target="#workspace-wrapper"` (`handlers_preset.go:792`) always resolves to the first match in DOM order -- the Generator's -- regardless of which workspace the user is actually refining. Refining a saved preset from the Library can silently write the response into the hidden Generator draft instead of the visible Library workspace.
- [ ] Dead code: `refinementSummaryHtml` (`handlers_preset.go` ~line 758) checks `lastMsg.Role == "architect"` to decide whether to show a prominent "Latest Refinement Result" callout, but `ChatHistory` entries are only ever written with role `"user"` or `"model"` (confirmed via grep across `internal/api`/`internal/agents`) -- that condition can never be true, so the callout never renders. Either wire a real `"architect"`-role message in, or remove the dead branch.
- [ ] `s.tasks` (in-memory generate/chat task map in `server.go`) never evicts entries -- confirmed zero `delete(s.tasks, ...)` calls anywhere. Every generation and chat-refinement task ID plus its full rendered HTML `Result` string stays in memory for the process's lifetime. Low urgency given Fly.io scale-to-zero, but unbounded growth on a longer-lived instance.
- [ ] Add a `gofmt -l` check to CI (`.github/workflows/ci.yml`) -- confirmed no gofmt check exists there and no local pre-commit hook either (`.git/hooks/pre-commit` doesn't exist). Pre-existing whitespace drift in `internal/api/handlers_preset.go` and others has been accumulating and has to be carefully avoided on every unrelated diff.
- [ ] Root-cause the vanished "Live Capture - Texas Flood (Desktop)" preset from the mobile UX redesign session -- it disappeared from the store after a container rebuild/redeploy and was never explained; worked around at the time by reusing the "(Mobile)" preset's data instead. Not confirmed as a one-off.
- [x] ~~The Architect's Capture Parameters Mandate (Rule 9) occasionally misapplies relative-dB parameter formatting to algorithmic (non-capture) Amplifier blocks.~~ Prompt-only tightening reduced but didn't eliminate this (confirmed via direct `coros_map.json` `is_capture` lookup, not judge opinion) -- `11_Mayer_Lead`'s "Dumbbell ODS" (confirmed algorithmic) still got the full treatment. Resolved with a deterministic Go-side check instead of a third prompt-tuning attempt: `FlagCaptureFormattingMismatches` (`internal/agents/fuzzy_matcher.go`) looks up each gear block's resolved name against `coros_map.json`/`user_captures.json`'s real capture status and flags (doesn't silently correct -- a relative-dB offset and an absolute 0-10 dial position are different units with no formula between them) any of Rule 9's five capture-only parameter names (`Gain`/`Bass`/`Mid`/`Treble`/`Volume`) that are dB-formatted on a confirmed-non-capture block. Wired into both save paths (`server.go`, `handlers_preset.go`) alongside the existing `FlagUnverifiedStructuredBlocks`.
- [ ] Revisit search grounding for the Acoustician agent (`internal/agents/prompts/6_acoustician_v3.md`, `agentSearchTool` in `internal/agents/schemas.go`). See [issue #64](https://github.com/weitzer-org/sound-profile-builder/issues/64) for full root cause/history and candidate approaches. Short version: adding the `GoogleSearch` tool reliably blew the 3-minute per-attempt timeout because Acoustician searches on nearly every prompt (almost all golden-set queries name identifiable real gear) and has a heavier nested `humbucker`/`single_coil` schema than Sonic Profiler, whose grounding shipped fine. Reverted rather than shipped broken; Sonic Profiler stayed grounded.
- [ ] Improve the blind LLM-judge eval methodology (`cmd/judge_compare`) -- individual verdicts are demonstrably unstable (re-judging identical files produced different scores and almost entirely different per-prompt outcomes) and at least one verdict was internally self-contradictory (rationale said one preset was "vastly superior," the structured preference field it's scored from picked the other one). See [issue #68](https://github.com/weitzer-org/sound-profile-builder/issues/68) for full evidence and candidate fixes (multi-run averaging, a self-consistency check on the judge's own output, requiring quoted evidence for structural claims). Not urgent -- the aggregate signal has stayed directionally consistent across re-runs -- but should be fixed before being relied on for a closer call.
- [ ] Investigate whether Acoustician-style specialist numeric tuning should extend beyond the Amplifier block to effect/pedal blocks (Drive, Fuzz, Boost, Delay, Reverb, Comp, Gate, Pitch, Mod) -- today the Architect tunes all of those directly itself in its single final-assembly call. See [issue #65](https://github.com/weitzer-org/sound-profile-builder/issues/65) for full pros/cons and a proposed (not yet implemented) path: isolate the highest-risk sub-problem first (gain-staging when 2+ drive/fuzz/boost blocks stack), prefer folding it into an existing agent (Mix Engineer) over adding a 13th call, and if a dedicated specialist is ever justified it must use the dynamic guitar-keyed `ResponseJsonSchema` pattern, not Acoustician's fixed humbucker/single-coil shape (effect chains have no fixed cardinality).

Found during `/code-review medium` on the Tier 1 branch (all confirmed real, deferred as lower-priority than the fixes shipped in that same PR):

- [ ] `RefineChat` was bumped to `12_architect_v4` (whose Rule 9 references a `Selected Capture Details` context block) but never computes/injects `SelectedCaptureContext` the way `RunPipeline` does (`internal/agents/orchestrator.go`, RefineChat's `refinementPrompt`) -- RefineChat doesn't re-run Librarian/Navigator, so there's no equivalent source data to build it from without a bigger RefineChat change. The prompt has a stated fallback ("absence is normal, use general knowledge") so this degrades rather than breaks. Deferred along with other RefineChat-specific quality work.
- [ ] Sonic Profiler's context always includes the full ~10KB `qc_block_schema.json` and the user's entire capture library, unscoped, on every run -- the same token-bloat pattern already found and fixed for Acoustician this session (trimmed to a 2-category subset, capture library dropped entirely), not applied symmetrically. Sonic Profiler's scope is legitimately broader than Acoustician's (touches EQ/saturation/reverb/gate, not just amp), so the fix isn't a direct copy -- at minimum the capture library (which Sonic Profiler's output schema has no fields to use) should probably drop the same way it did for Acoustician; the full vocabulary file may or may not be justified depending on how much Sonic Profiler's grounding actually leans on it.
- [ ] `agentTemperature`, `agentSearchTool`, and `agentMaxOutputTokens` (`internal/agents/schemas.go`) are three independent maps each hand-listing all 12 agent-role key strings, with nothing enforcing they stay in sync -- adding a 13th agent or fixing a key typo requires updating up to three maps by convention only. Should be unified into one per-agent config struct/map.
- [ ] Neither `FlagUnverifiedStructuredBlocks`'s nor `FlagCaptureFormattingMismatches`'s warnings ever appear in the read-only preview table (`final_html_payload`) -- `injectRenderedHTML` bakes that table inside `RunPipeline`, before the result ever reaches the API-layer code that calls either Flag function. Pre-existing limitation (not introduced by `FlagCaptureFormattingMismatches`, which just inherits it), and affects the primary UI surface users actually look at; the warning currently only surfaces in the less-visited editable Tweaking Workspace. Fixing this properly means moving (or duplicating) the flagging step to before `injectRenderedHTML` runs, inside the pipeline itself.
- [ ] `RunAgentSplit`'s Tools+JSON-mime-type retry (`internal/agents/orchestrator.go`) matches on a substring of the raw Gemini error message (`"tool use with a response mime type"`), not a documented SDK error code/type. If Gemini ever changes that wording, the match silently stops firing and the degraded-retry recovery quietly stops working with no test or alert to catch it. Same pre-existing pattern as the `isContextLimit` check elsewhere in this file, not a new category of debt, but worth a more durable dispatch (status code + reason field) if the SDK exposes one.
- [ ] `qc_block_schema.json`'s Cabinet entry (manual-researched: IR-loader-only, pick a factory/custom impulse file, no continuous mic-position controls) contradicts the pipeline's actual, pre-existing Cabinet block model -- Transducer Tech's virtual mic X/Y placement (`mic_1_pos`/`mic_2_pos`/`blend_ratio`) and Architect Rule 12, both of which predate/are independent of the Tier 1 vocabulary work and are what every real eval output (Tier 0 and Tier 1) actually generates. Confirmed, not investigated further -- `FlagIncompleteCabinetBlocks` (Tier 2) was built against the established Mic 1/Mic 2/Blend behavior, not the manual research, since reconciling which model is actually hardware-accurate is a bigger question than a validator addition. Worth a real investigation (possibly needing the same optional/opportunistic Cortex Control app cross-check discussed for range verification) before either model gets treated as ground truth.
- [ ] `MaxOutputTokens` truncation turned out to be much more widespread than the single Community Scraper/`08_EVH` case first noticed during Tier 2 development -- a full golden-set run hit a ~50% failure rate spread across CorOS Librarian, Sonic Profiler, Control Mapper, and Community Scraper, all previously reliable at their original caps, with no code change to those agents in between and no reported Gemini incident. Addressed as an urgent hotfix (`hotfix/max-output-tokens-caps`, merged independently of Tier 2): every cap roughly doubled, validated via 4 consecutive live retries of a previously-failing prompt (0/3 pre-fix, 4/4 post-fix). Most consistent explanation is `gemini-3.5-flash` (a named alias, not a pinned version) drifting toward more verbose completions server-side -- something outside this repo's control. If truncations recur at the new (doubled) values, that's real signal this needs a different fix (e.g. pinning a dated model version, or investigating whether it's specifically the flash tier / specifically grounded calls) rather than a third round of cap-raising. The original `08_EVH`-specific observations (Community Scraper occasionally returning exactly 0 output tokens even pre-truncation-check; that prompt's gear-dense content possibly needing genuinely more headroom) are still real data points worth revisiting during that investigation.
