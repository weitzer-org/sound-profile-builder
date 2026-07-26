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

  **Merged into `coros_map.json` (188/192, 97.9% coverage) and
  `user_captures.json` (all 87 entries `description_verified`).** An Opus
  review of both full drafts before merge caught a real bug in
  `cmd/enrich_captures` itself, not just bad individual labels: the tool
  researched each entry's `coros_equivalent` (the QC's own on-device block
  name, e.g. `"Chief Bass Overdrive"`, `"D-Cell H4 Ch2"`) instead of the
  map key (the actual real-world gear name, e.g. `"Boss ODB-3"`,
  `"Diezel VH4 Ch2"`) -- on-device names are often obfuscated/generic
  enough that this usually still worked, but it produced a wrong-pedal
  citation on `Boss ODB-3` (matched an unrelated Flattley pedal via
  "Chief") and collapsed three unrelated captures that happen to share the
  on-device block `"Love Drive 11"` into one shared (and wrong) citation.
  Fixed to research by map key, with `coros_equivalent` passed only as
  secondary disambiguating context; the fix also let the tool research 17
  entries it previously couldn't act on at all (captures with no
  `coros_equivalent` recorded). Four reverb captures (`Generic Spring
  Reverb`, `Nameless Reverb`, `Nolly Reverb`, `Plini Reverb`) were left
  without a `tonal_archetype` rather than merged -- they'd all collided
  onto the same on-device block as an Orange Rockerverb amp mapping and
  inherited its (amp-centric, wrong-for-reverb) label; real fix is a
  type-specific archetype vocabulary (see "worth deriving next" below),
  not a patched label. For `user_captures.json`, 7 of 87 re-researched
  descriptions were rejected in favor of the existing one (regressions
  that dropped a correct circuit/variant detail, or over-specific
  inferences the capture name itself didn't support) -- all 87 are still
  marked `description_verified` either way, since "verified but kept the
  original" is still a completed review, not a skipped one.

  **Worth deriving next, beyond `tonal_archetype`** (from a follow-up data
  audit, not yet started): (1) a `channel_type` field (Clean/Crunch/Lead/
  Rhythm) -- 53 `coros_map.json` entries already encode this in their key
  name (e.g. `Matchless DC30 Ch1` vs `Ch2`), and it directly explains
  mislabels like a lead channel absorbing its amp's generic archetype;
  (2) per-`type` archetype vocabularies instead of one universal enum --
  the current 3-value enum is really amp/drive-shaped, which is why a
  reverb or delay block has no good answer to give it. `confidence_score`
  was checked and ruled out as a signal: 468/469 entries are exactly
  `1.0`, a non-discriminating default, not real per-entry confidence.

  **Post-merge Opus audit of the merged files** (direct file inspection,
  not just the draft proposals): confirmed the root-cause research-subject
  fix actually took (cross-checked the obfuscation pattern between several
  real names and their `coros_equivalent` values -- consistent, no repeat
  of the substring-collision failure mode), confirmed the 4 reverb entries
  genuinely ship with no `tonal_archetype` key at all (the bad collided
  value never left the draft file), and confirmed all 7
  `user_captures.json` keep-existing decisions were sound. Two issues
  followed up on:
  - **`Diezel VH4 Ch2`** -- its re-researched `British Crunch` citation
    ("Diezel official VH4 product documentation") was too generic to
    confirm it actually verified the per-channel voicing claim rather than
    just returning the amp's landing page. Plausible on its face (the VH4
    does split into a Ch1/Ch2 lower-gain pair vs. a Ch3/Ch4 lead pair,
    so a "crunch" bucket for Ch2 isn't inherently wrong), but not
    confident enough to ship -- `tonal_archetype` cleared back to unset
    for this entry pending a real per-channel source, same treatment as
    the 4 reverb entries.
  - **Pre-existing data-quality issue, unrelated to this work** (found
    during the spot-check, not introduced by it): `"Supro Thunderbolt 15"`
    (`type: "amp"`) and `"Two Notes Supro Thunderbolt 15”"` (`type: "fx"`)
    both map to the same on-device block (`coros_equivalent: "Super Bolt"`)
    for what's plausibly the same physical amp, with an inconsistent
    `type` field between them -- and the second key uses a curly
    right-quote (`”`, U+201D) instead of a straight `"`, which would
    silently fail to match against real device-exported capture names in
    any exact-string lookup. Not fixed here (predates this PR, needs its
    own look at whether they're genuinely different captures or a data
    entry error); flagged for a follow-up pass.
- **Cabinet manual-research-vs-actual-model reconciliation** -- still an open
  question, not a wiring fix: is the real QC hardware IR-loader-only per
  `qc_block_schema.json`'s manual research, or does it have the continuous
  mic-placement controls (`Mic 1`/`Mic 2`/`Blend`) every actual eval output
  generates? Needs the Cortex Control app cross-check mentioned elsewhere
  in this doc before either model is treated as ground truth. Tracked in
  [issue #81](https://github.com/weitzer-org/sound-profile-builder/issues/81).
  Split out into its own issue,
  [#82](https://github.com/weitzer-org/sound-profile-builder/issues/82): new
  evidence that this isn't just an open modeling question but an active,
  severe (85-90%) compliance failure against the pipeline's own current
  Rule 12, independent of which hardware model eventually wins -- see the
  eval-infrastructure section above.

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

## Gemini 3.6 Flash Migration — Status (as of 2026-07-22)

`gemini-3.6-flash` is a live, callable model (confirmed via a direct API call)
and a real latency win over both models it could replace -- but two live eval
rounds via `cmd/eval_full_pipeline` + blind pairwise judging via
`cmd/judge_compare` (judge stays `gemini-3.1-pro-preview` for both
comparisons, including the Pro-tier one where it's also a candidate, to keep
the grading rubric identical across both -- a known, deliberate tradeoff, not
an oversight) found a real, reproducible quality gap. **Not adopted** in
either production tier pending the fixes below.

- **What was tested** (`feature/gemini-3.6-flash-quality-gap` branch):
  `cmd/eval_full_pipeline` was rewritten from a uniform "one model for all 12
  agents" sweep to three named routing scenarios that mirror how production
  actually splits agents -- `baseline` (Pro tier agents 1/12 on
  `gemini-3.1-pro-preview`, Flash tier agents 2-11/13 on `gemini-3.5-flash`),
  `flash-tier-candidate` (same Pro tier, Flash tier swapped to
  `gemini-3.6-flash`), and `pro-tier-candidate` (Pro tier swapped to
  `gemini-3.6-flash`, Flash tier unchanged). `cmd/eval_subagent` was
  similarly narrowed to the 3 agents (Tone Historian, Sonic Profiler,
  Community Scraper) whose input doesn't depend on another agent's output,
  so they can be isolated without running the full pipeline.
  `cmd/judge_compare` needed two compatibility fixes to work against current
  output at all: it still expected `.json` files matching a dead output
  format from an earlier pipeline version, not the `.html` files
  `RunPipeline`/`cmd/eval_full_pipeline` actually produce today, and its
  hardcoded 12-item query list was missing the 13th golden-set query.
- **Latency**: `gemini-3.6-flash` was ~11-24% faster than the model it
  replaced in both tiers, with token usage roughly flat. Not the deciding
  factor -- quality was prioritized over this win, per the decision below.
- **Quality** (combined across both eval rounds): Flash tier is close to a
  wash (10 baseline / 9 candidate wins) with no clear case either way; Pro
  tier favors baseline clearly (11 baseline / 5 candidate / 1 equal) -- do
  not move Tone Historian/Architect to `gemini-3.6-flash` without closing the
  gap below first.
- **Root cause #1 (blocking)**: `gemini-3.6-flash` recurrently describes a
  native/algorithmic overdrive block ("Iba Green") as a Neural Capture in its
  rationale text -- a description-vs-implementation contradiction serious
  enough that the existing Preset Critic agent flags it. Reproduced 4
  separate times across both eval rounds and both tiers, 3 of those on the
  exact same golden-set query (`07_Edge`) -- a specific, reproducible defect,
  not judge noise.
- **Root cause #2 (blocking)**: `gemini-3.6-flash` recurrently embeds units
  directly into numeric JSON parameter fields (e.g. `"-3.0 dB"` instead of
  `-3.0`), a schema-discipline regression that risks breaking downstream
  parsing. Reproduced independently in both eval rounds on the same query
  (`09_BB_King`).
- **Reliability caveat, partially addressed**: Acoustician was truncating at
  `MaxOutputTokens` on ~7/39 pipeline runs in the first eval round (both on
  `gemini-3.5-flash` and on the `gemini-3.6-flash` candidate). Doubling the
  caps for the 4 agents that actually failed (`agentMaxOutputTokens` in
  `internal/agents/schemas.go`: `3_community_scraper`, `4_coros_librarian`,
  `6_acoustician`, `7_transducer_tech`) cut the second round's failures to
  4/39 -- but all 4 were still Acoustician, split evenly across both models,
  even at the doubled cap. Per this same file's own comment (added during
  the prior `hotfix/max-output-tokens-caps` round), recurrence *after* a
  doubling is itself signal that this needs a different fix (e.g. detecting
  and retrying on a degenerate repetition pattern, matching the original
  32,775-token pathological-case observation the caps exist to catch) rather
  than a third blind doubling.
- **Judge-reliability caveat, with concrete verified examples (not just
  "verdicts vary" per issue #68)**: two separate judge verdicts were checked
  against this repo's own ground truth and found to rest on a judge
  hallucination, not a real quality difference -- it called `"ODS Joe Lead"`
  (a real, `description_verified: true` entry in `user_captures.json`)
  "fabricated," and separately called `"Circular Delay"` (referenced
  unremarked-upon as a normal QC block in multiple other judge rationales in
  this same eval) an "Axe-Fx exclusive model." Some fraction of any single
  judge tally rests on the judge's own factual error.

**Action items before reconsidering adoption:**

- [ ] Root-cause the Iba-Green/Neural-Capture mislabeling on `07_Edge` --
  reproducible on demand (4/4 occurrences so far), so isolate it via a
  single-agent Architect call (`cmd/eval_subagent`-style) rather than a full
  pipeline run before attempting a prompt fix. Likely a prompt-level
  ambiguity in how `gemini-3.6-flash` specifically distinguishes captures
  from modeled blocks, since neither `gemini-3.5-flash` nor
  `gemini-3.1-pro-preview` showed this pattern in either eval round.
- [ ] Fix or guard against the units-embedded-in-numeric-fields regression --
  either a schema-level guard (reject/strip non-numeric characters in
  `Value`/`ValueB` before render) or a targeted prompt instruction; needs its
  own isolated repro before deciding which.
- [ ] Investigate Acoustician's remaining `MaxOutputTokens` truncation as a
  repetition-loop bug rather than raising `agentMaxOutputTokens["6_acoustician"]`
  a third time -- e.g. capture and inspect one truncated raw response to
  confirm degenerate repetition vs. genuinely longer legitimate output.
- [ ] Once the above are addressed, re-run the same three-scenario eval
  (`cmd/eval_full_pipeline`) and re-judge (`cmd/judge_compare`, judge stays
  `gemini-3.1-pro-preview` for both comparisons per the decision above) before
  any routing change.
- [ ] Given the two verified judge-hallucination cases above, consider
  spot-checking a sample of judge verdicts against `coros_map.json`/
  `user_captures.json` ground truth before trusting a close-call tally --
  ties into the broader judge-methodology fix already tracked in issue #68.
- [ ] Only after a clean quality run: update the production routing defaults
  in `RunAgentSplit` (`internal/agents/orchestrator.go`, ~line 750) and add
  `gemini-3.6-flash` to `getFallbackChain` (currently absent as a fallback
  target for any primary model).

### Thinking-budget eval infrastructure (2026-07-25)

Precursor work for testing `gemini-3.5-flash`/`gemini-3.6-flash`/
`gemini-3.1-pro-preview` at different thinking levels, prompted by comparing
this project's eval tooling against `job_tracker`'s more mature
`internal/eval` harness (same-domain precedent: it already ran this exact
3-model + thinking-budget-sweep comparison for its own scoring feature).

- **Wired `ThinkingConfig` into the orchestrator** (`Orchestrator.ThinkingBudget
  *int32`, applied in `buildGenerationConfig`; `nil`=provider default, explicit
  `0`=disable) -- previously absent entirely, so "test at different thinking
  levels" wasn't yet a knob that existed in production code. `ThinkingTokens`
  is now tracked alongside `InputTokens`/`OutputTokens` in `TokenUsage`/
  `AgentUsage` (Gemini bills thinking tokens as output, so this is needed for
  any future cost comparison).
- **Live-probed (`cmd/probe_thinking`) which of the three models actually
  honor the budget.** Only `gemini-3.5-flash` can genuinely disable thinking
  (`budget=0`); both `gemini-3.6-flash` and `gemini-3.1-pro-preview` reject it
  outright (`400 INVALID_ARGUMENT: "This model only works in thinking mode"`).
  `gemini-3.6-flash` additionally has **no working fallback** at `budget=0`
  (its only fallback, `gemini-2.5-pro` via `getFallbackChain`'s default case,
  rejects it too). `gemini-3.1-pro-preview`'s fallback chain does include
  `gemini-3.5-flash`, which *does* accept `budget=0` -- so a naive test of
  that cell silently substitutes a different model and reports a false
  success; `cmd/probe_thinking` tracks the actually-served model
  (`TokenUsage.ModelsUsed`) specifically to catch this.
- **Rejected replicating `job_tracker`'s ground-truth/AUC approach** --
  "relevant/not_relevant" job labels have no clean analog for "is this a good
  guitar tone." Instead added a judge-free, deterministic quality signal
  (`internal/agents/qualitychecks.go`, `RunMechanicalQualityChecks`) built
  from the same `Flag*` checks the API layer already runs in production
  (`FlagUnverifiedStructuredBlocks`, `FlagCaptureFormattingMismatches`,
  `FlagIncompleteCabinetBlocks`, `FlagLeftoverValueRanges`, all now also
  returning a defect count). This exists specifically because
  `cmd/judge_compare`'s blind pairwise LLM judge is the weakest link in this
  repo's eval story (issue #68) and doesn't scale to a multi-model x
  multi-budget matrix anyway (pairwise comparisons grow combinatorially).
  `Orchestrator` exposes the raw pre-HTML-render Architect+Critic JSON as
  `LastArchitectJSON` (a side-channel, like `Usage`) so eval tooling can run
  these checks without changing `RunPipeline`'s return signature.
  - A `FlagUnitsEmbeddedInNumericFields` check (flagging e.g. `"-3.0 dB"`
    instead of `-3.0`) was added, then **removed after live evidence
    showed it was wrong**: it fired 25-45 times per generation, and a live
    diagnostic run showed why -- it was flagging normal, expected
    formatting this pipeline already produces everywhere (graphic-EQ dB
    sliders, Mix/Blend percentages, reverb Decay in seconds), not the
    actual regression. That regression is specifically dB-relative
    formatting on one of the five capture-only parameter names on a block
    confirmed to be algorithmic, not a capture -- already correctly caught,
    capture-status-gated, by `FlagCaptureFormattingMismatches`. Caught
    during the first live matrix run (which was stopped and restarted once
    this was found and fixed, rather than let ~6 hours of live runs
    complete with a contaminated headline `TotalDefects` metric).
- **Consolidated the golden query set** (`internal/evalfixtures`), previously
  copy-pasted across `cmd/eval_compare`, `cmd/eval_full_pipeline`, and
  `cmd/eval_subagent` -- `cmd/eval_compare`'s copy had silently drifted to 12
  queries (missing `13_Hard_Rock_Blues`), now fixed by construction.
- **New tool: `cmd/eval_thinking_matrix`** runs the real full pipeline across
  a `{model, thinking budget}` grid (excluding the confirmed-invalid `budget=0`
  cells above) and scores each output with `RunMechanicalQualityChecks`.
  Defaults are deliberately cost-conservative: `-dry-run=true` by default (this
  repo has no per-token pricing table the way `job_tracker` does, so call count
  is the only cost lever), and `-queries` defaults to the 3 golden-set prompts
  already tied to known regressions (`07_Edge`, `08_EVH`, `09_BB_King`) rather
  than the full 13. `-models`/`-budgets` filter the grid for a cheaper partial
  run. Validated end-to-end with 2 live smoke-test cells (1 config x 1 query x
  1 rep each) -- both ran clean, and the per-check breakdown the report prints
  correctly explained the observed defect count both times.
- **Full matrix run completed 2026-07-26** (`-all-queries -reps=1`, 130 cells,
  121 succeeded, $25.85 total per Gemini Developer API list pricing). Full
  writeup and evidence: [issue #80](https://github.com/weitzer-org/sound-profile-builder/issues/80).
  Headline findings, in priority order:
  1. **Cabinet mic-placement completeness (Mic 1/Mic 2/Blend) is missing on
     85-90% of Cabinet blocks across all three models uniformly** (avg
     1.73-1.84 incomplete out of a max 2 per run) -- not a model-comparison
     signal at all, a systemic prompt/schema bug that dominates every
     model's "defects" count and should be root-caused independent of any
     routing decision. Tracked in
     [issue #82](https://github.com/weitzer-org/sound-profile-builder/issues/82)
     (the compliance bug itself) and
     [issue #81](https://github.com/weitzer-org/sound-profile-builder/issues/81)
     (the pre-existing open question about `qc_block_schema.json`'s Cabinet
     entry contradicting this same mic-placement model -- split into its
     own issue since it's an independent question from whether the pipeline
     complies with its own current rule).
  2. After controlling for (1), **`gemini-3.1-pro-preview` -- the current
     production Pro-tier model -- had 3.8x more Critic-flagged
     self-contradictions and 6.1x more unverified/fabricated block names
     than `gemini-3.5-flash`** in this single-rep comparison. Caveat: n=1
     rep, and `3.5-flash` failed more often (survivorship bias on its own
     average) -- see issue #80 for the full caveat and a repeated-rep
     re-run recommendation before trusting the exact magnitude.
  3. **Higher thinking budgets correlate with more Critic flags for
     `gemini-3.6-flash` and `gemini-3.1-pro-preview` specifically** (clean,
     monotonic ~2x-per-budget-step increase for both) but not for
     `gemini-3.5-flash` -- thinking budget is not a free quality lever here.
  4. `gemini-3.6-flash` had **zero truncation failures across all 39 of its
     cells** (vs. `gemini-3.1-pro-preview` 5.1%, `gemini-3.5-flash` 13.5%,
     the latter matching this doc's already-tracked verbosity/truncation
     history). 3 of `gemini-3.5-flash`'s 7 failures happened even at
     `budget=0` (thinking disabled), ruling out "thinking competing for
     output-token headroom" as the mechanism -- confirms genuine response
     verbosity, not a thinking/output tradeoff artifact.
  - Does **not** confirm or refute the two specific blocking bugs from the
    original `gemini-3.6-flash` candidate-routing eval above (Iba-Green
    mislabeling, the narrower original units-embedded regression) -- those
    remain open per the action items above.
- **Eval framework overview + robustness gaps**, written up after the above:
  [issue #83](https://github.com/weitzer-org/sound-profile-builder/issues/83).
  Headline gap: no trustworthy subjective-quality signal exists (`judge_compare`'s
  instability, #68, is still unaddressed) -- the mechanical checks catch
  structural/factual defects but say nothing about whether a preset actually
  sounds right. Also flags zero test coverage on the eval tools themselves
  (two bugs in `eval_thinking_matrix`'s own report code had to be caught live
  this session, see above) and the fragmented `cmd/eval_*` family as
  concrete, prioritized follow-ups.

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
