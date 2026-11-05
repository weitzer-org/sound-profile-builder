# Librarian quality: catalog fixes, eval fixes, and a proper re-measurement

Status: proposed, 2026-11-04. Written to hand off to a development session.
Related: PR #110 (`claude/stoic-hawking-kwbe44`, "Make Librarian context cacheable
and record cached tokens").

## Why this exists

PR #110 reorders the CorOS Librarian's context (static catalog first) so Gemini's
implicit prefix cache can hit. Measured on baseline routing, cached input went from
~0% (old order, 13 pipelines) to 51% (new order, 12 pipelines).

A blind LLM judge first preferred the old order 10-2. Re-judging the same 12 pairs
twice gave 6-5 and 6-4-2. Across the three runs, 4 queries consistently favored the
old order and 2 the new: a two-sided sign test gives p = 0.69. So there is no
demonstrated order regression. But looking at what the presets actually contain
turned up real, order-independent quality bugs in the Librarian's catalog, and
showed that the eval tooling can't reliably detect a gap of this size.

This plan has three workstreams:

1. **Catalog fixes + guard tests.** Free, order-independent quality gain.
2. **Eval tooling fixes.** Code-only, so a future comparison means something.
3. **Re-measurement.** Paid and needs the owner's approval: old vs new order, after 1 and 2 land.

Do 1 and 2 as separate PRs from `main`, not on PR #110's branch. Do 3 last.

## Ground rules for the development session

- **No live Gemini calls and no paid evals in workstreams 1 and 2.** Unit tests and
  MOCK mode only. Workstream 3 needs explicit owner approval at the time it runs, with
  a cost estimate stated up front.
- **Every catalog name change needs a citable source.** This is the same bar CLAUDE.md
  sets for `tonal_archetype`: never guess a Quad Cortex block name. If a name can't be
  sourced, leave the entry unchanged and list it as unresolved in the PR.
- `coros_map.json` and `user_captures.json` are embedded with `//go:embed`, so changing
  them needs a rebuild/redeploy.
- Follow CLAUDE.md's review policy: the bots are the default gate. Don't run
  `/code-review` automatically.
- Before claiming done: `go vet ./...` and `go test ./...` (unset `GEMINI_API_KEY`,
  because `TestOrchestrator_OpenLLM_Integration` fails in sandboxes where it's set,
  and also fails on `main`). Run Playwright e2e if Docker is available. If it isn't,
  say so.

## Evidence (from the 2026-11-04 investigation)

These are verified by running code or reading files, unless marked.

- **`Range Rverb` is mapped to both an amp and reverbs.** In `internal/agents/coros_map.json`,
  the key `"Generic Spring Reverb"` (type `reverb`, `is_capture: true`) maps to
  `"Range Rverb"`. That is also the equivalent for `"Orange Rockerverb 100 MK3"` (an
  amp capture, ~line 384), and for `"Nameless Reverb"`, `"Nolly Reverb"` and
  `"Plini Reverb"`. The catalog has no other spring-reverb target. The Librarian's
  anti-fabrication rule (v2 rule 12) therefore steers it to `Range Rverb`, which also
  breaks v2 rule 11 (never map a time-based effect to a capture).
  - Seen in presets: new order 3/12, 3/13 and 3/10 (three scenarios), old-order
    control 2/12, and committed `main` eval results 4/35.
  - The judge penalized it explicitly ("severe hallucination... 'Range Rverb'") in
    `13_Hard_Rock_Blues`.
- **Unrelated devices share one equivalent name.** 42 equivalent names are used by
  entries of more than one `type`. Most are harmless same-device duplicates (e.g.
  `"Boss BD-2 Blues Driver"`/drive and `"BOSS BD-2"`/fx both map to `Chief BD2`).
  The ones that look like real errors (unverified until sourced):
  - Default path (no plugin needed):
    - `Exotic`: Xotic BB Preamp (fx) **and** MXR UniVibe (mod).
    - `Facial Fuzz`: Dallas Arbiter Fuzz Face (drive) **and** Xotic RC Booster (fx).
    - `US TWN Vibrato` ("Fender Twin Reverb") vs `US Twin Vibrato` ("Fender 65 Twin
      Reverb"): two spellings of what is presumably one block.
  - Plugin-gated (live for this owner: `config.json` `available_plugins` =
    Cory Wong, John Mayer):
    - `Crying Wah`: Dunlop Cry Baby **and** Cory Wong Wah.
  - Plugin-gated (hidden unless the plugin is owned, but wrong):
    - Gojira CHR/CLN/RST mapped to `Chief CS3`.
    - Gojira HOT mapped to `Chief MT`; Gojira DRT to `Chief DS1`; Gojira OD to
      `Chief OD1`; Gojira OCT to `Chief OC2`.
    - Gojira PHSR mapped to `Bogna Fish`; Gojira WOW to `Bubba Wah`; Gojira REV
      Shimmer to `Rage Booster`.
    - Gojira and Nolly cabs mapped to `Solid State Comp (ST)`; SLO-100 Cab to
      `Solo 100 Crunch Bright`.
    - Nameless Grind mapped to `CA Tremo Red`; Nameless Hexdrive to `Obsessive Drive`.
    - Nolly and SLO overdrives mapped to `Love Drive 11`.
    - Plini Drive mapped to `Myth Drive`; Plini Comp to `OC-76 Comp`.

  Reproduce the full list with the script in "Workstream 1 / step 1" below.
- **The prompt names a block the Dictionary doesn't have.** Librarian v1/v2 rule 5
  says to map a Twin Reverb to `'US Twin Vibrato'` or `'US Twin Normal'`.
  `US Twin Normal` does not exist in `coros_map.json`, and rule 12 forbids any name
  that isn't in the Dictionary.
- **Common native effect blocks aren't in the catalog.** Presets use names such as
  `Spring Reverb`, `Analog Delay`, `Noise Gate`, `Room Reverb` and `Volume Pan`,
  which are not `coros_equivalent` values. `fuzzy_matcher.go:30-45` documents this
  as deliberate: the map is a gear-translation table, and native non-gear categories
  aren't validated. The cost is that the Librarian has no correct name for a generic
  spring reverb and grabs the wrong one.
- **The snap pool may ignore plugin ownership** (not yet verified end to end). A probe
  of `SnapToClosestBlock(name, GetValidNativeBlocks())` gave:
  - `"Vintage Spring Reverb"` → `"John Mayer Spring Reverb"` (a plugin-gated name).
  - `"Parametric EQ"` → `"Parametric-8"`.
  - `"Spring Reverb"` stays as is, so the snap step is **not** the source of
    `Range Rverb`.

  `resolveBlockName` (`fuzzy_matcher.go` ~line 306) returns the snapped name for
  non-gear categories even when it isn't in the caller's filtered `validBlocks`.
  That could leak a plugin block to a user who doesn't own the plugin. Verify before
  fixing.
- **Eval tooling problems:**
  - `cmd/eval_full_pipeline` writes into the **tracked** `eval_results/full_pipeline/`.
    When a pipeline fails, the old committed HTML stays in place, and
    `cmd/judge_compare` then judges it as if it were new output. This happened with
    `06_Gilmour`.
  - `judge_compare` randomizes A/B once per pair, so single verdicts flip:
    6 of 12 queries changed winner across three runs.
  - Its `confidence` field has no defined scale; values came back as 0.9 and 5.0.
  - Its query list is hard-coded (lines ~66-70) instead of
    `evalfixtures.GoldenQueryOrder()`.
  - Nothing saves the Librarian's own output during evals, although
    `LOCAL_AGENT_LOGGING=true` (`orchestrator.go` ~line 475) already writes each
    agent's output to `eval_results/diagnostics/<dir>/<agent>.json`.
  - A hand-written Librarian probe loaded the **v1** prompt
    (`loadPromptForAgent(key, "")`), while production uses **v2** (`config.json`
    `agent_prompts["4_coros_librarian"] = "v2"`), so its results don't transfer.
- **Reliability.** The Acoustician hit MaxOutputTokens on every fallback model 5 times
  across the three eval runs:
  - Aborted first attempt: `01_SRV_Clean`.
  - Full run: `06_Gilmour` (baseline), `01_SRV_Clean`, `07_Edge` and `11_Mayer_Lead`.
  - Old-order control: none.

  This is out of scope here, but it causes missing eval pairs. See "Related follow-up".

## Workstream 1: catalog fixes and guard tests

Branch suggestion: `fix/coros-map-name-collisions` from `origin/main`.

1. **Reproduce the audit.** Commit it as a test helper or a small `cmd/audit_coros_map`.
   It should:
   - Group `coros_map.json` by `coros_equivalent` and list groups whose entries have
     more than one `type`.
   - List time-based entries (`reverb`, `delay`) with `is_capture: true`.
   - List near-duplicate spellings (case- and space-insensitive, `TWN` read as `Twin`).
   - List names the Librarian prompts reference that aren't in the Dictionary
     (currently `US Twin Normal`).
2. **Find a source.** Look for an authoritative list of Quad Cortex block names. Try
   the Neural DSP Quad Cortex user manual first (`qc_block_schema.json`'s `_readme`
   already cites https://www.manualslib.com/manual/3537295/Neural-Quad-Cortex.html),
   then Neural DSP's official device/model list pages. Fetch them; don't recall names
   from memory. Record a citation for every name you change or add.
3. **Write a draft first.** Mirror the `cmd/enrich_captures` pattern: write proposed
   changes plus citations to a sibling draft file, have a human review it, then merge
   the approved changes into `coros_map.json` in the PR. At minimum, resolve:
   - `Generic Spring Reverb`: point it at the real native spring reverb block and set
     `is_capture: false`. `fuzzy_matcher.go`'s comment suggests `Spring Reverb`; verify it.
   - `Nameless Reverb`, `Nolly Reverb`, `Plini Reverb`: give each its real plugin block
     name, or remove the entry if it can't be sourced. Don't leave them pointing at
     `Range Rverb`.
   - `Orange Rockerverb 100 MK3`: confirm `Range Rverb` is the real capture name.
   - `Exotic`, `Facial Fuzz`: decide which device really owns each name, and re-map
     the other device.
   - `US TWN Vibrato` / `US Twin Vibrato`: keep the verified spelling. Also resolve
     `US Twin Normal`: add it if it's real, otherwise edit rule 5 in **both**
     `prompts/4_coros_librarian.md` and `_v2.md`. This is the only prompt edit in the
     plan; call it out in the PR.
   - The plugin-gated collisions listed in the Evidence section. Prioritize
     `Cory Wong Wah` (live for this owner). Leave anything unsourced as is, and list it.
4. **Add the missing native generic blocks.** Add a few clearly-keyed generic entries
   (e.g. `"Generic Spring Reverb"`, `"Generic Plate Reverb"`, `"Generic Room Reverb"`,
   `"Generic Analog Delay"`, `"Generic Tape Delay"`, `"Generic Noise Gate"`,
   `"Generic Parametric EQ"`, `"Generic Volume Pan"`), pointing at verified native
   names, with `is_capture: false`. Only add names you can source. Update the
   `gearBlockTypes` comment in `fuzzy_matcher.go` to match. No `tonal_archetype` on
   these: archetypes are only for amps.
5. **Snap-pool plugin leak** (verify first):
   - Write a unit test showing whether `resolveBlockName` can return a plugin-gated
     name (e.g. `John Mayer Spring Reverb`) when `allowedPlugins` doesn't include that
     plugin.
   - If it can, make the snap candidate pool respect the same filtering as
     `BuildEffectiveValidBlocks`. Pass the allowed set into the snap step instead of
     using the package-level `validBlocksRunes` cache unfiltered, or re-check the
     result against `validBlocks` and fall back to the raw name.
   - Keep `ApplyFuzzyCorrection` and `FlagUnverifiedStructuredBlocks` agreeing (both
     call `resolveBlockName`).
6. **Guard tests** (`internal/agents`, e.g. `coros_map_integrity_test.go`):
   - No `coros_equivalent` is shared by entries of different types, except an explicit
     allowlist of known same-device duplicates. Write the allowlist as data with a
     one-line reason each, so a new collision fails the test.
   - No `reverb`/`delay` entry has `is_capture: true` (matches Librarian rule 11).
   - No near-duplicate spellings among equivalents.
   - Every block name quoted in `prompts/4_coros_librarian*.md` rules exists in the
     Dictionary (catches the `US Twin Normal` class of bug).
7. **Keep existing tests passing**, including `TestGetCategorizedAmplifiers_PluginFiltering`,
   the `fuzzy_matcher_test.go` fixtures that name real entries (`"Brit 2203 87"`,
   `"John Mayer Smooth Operator"`), and `TestStaticCatalog_DeterministicAcrossCalls`
   (once PR #110 is merged).
8. **In the PR body,** list every changed entry with its citation, and every
   unresolved entry with the reason.

Acceptance:
- The audit shows no unexplained cross-type collisions.
- No time-based capture entries remain.
- The guard tests fail on the pre-fix file and pass after. Show both outputs.

## Workstream 2: eval tooling fixes

Branch suggestion: `fix/eval-tooling` from `origin/main`. Code only; nothing here
makes API calls in tests.

1. **`cmd/eval_full_pipeline`:**
   - Add flags: `-scenarios` (comma list of `baseline,flash-tier-candidate,pro-tier-candidate`,
     default all), `-samples N` (default 1), `-out DIR`.
   - Default `-out` to a fresh, untracked directory such as
     `eval_results/runs/<UTC-timestamp>/`, and add `eval_results/runs/` and
     `eval_results/diagnostics/` to `.gitignore`. Never write into the tracked
     `eval_results/full_pipeline/` again.
   - Layout: `<out>/<scenario>/s<k>/<query>.html`.
   - Write `<out>/manifest.json` with one row per (scenario, query, sample): success
     or error, latency, input, cached input, output and thinking tokens, and models used.
   - Save every agent's output for each run, so the Librarian's output can be inspected.
     `logToGCS` builds the path as `"eval_results/diagnostics/" + dirName`. Either set
     `LOCAL_AGENT_LOGGING=true` and `LOCAL_AGENT_LOGGING_DIR=<runid>/<scenario>/s<k>/<query>`
     per pipeline, or pass the directory in a cleaner way. Keep any production-code
     change minimal and covered by a test.
   - Add an "Avg Cached Input Tokens" column to the report.
2. **`cmd/judge_compare`:**
   - Read both sides' `manifest.json`. **Skip, and report, any pair where either side
     failed.** Never fall back to a file the run didn't produce.
   - Replace the hard-coded `order` with `evalfixtures.GoldenQueryOrder()`.
   - **Judge each pair twice with A/B swapped** (deterministic, not random). Record
     `consistent_winner` (`old`/`new`/`equal`), or `inconsistent` when the two verdicts
     disagree.
   - Support samples: pair `s<k>` with `s<k>` when both sides have N samples.
   - Define `confidence` as 0-1 in the response schema description.
   - Print and write a summary with consistent wins per side, inconsistent count, and
     a two-sided sign test p-value on consistent pairs (ignore ties). Pure Go, no new
     dependency.
   - Unit-test the pairing, skip, swap-consistency and sign-test logic with fixture
     directories and a fake judge function (inject the call). No live calls.
3. **Librarian-only A/B tool** (optional; only if workstream 3 wants it). If added, make
   it a gated `cmd/` tool rather than a `_test.go` file:
   - Load the prompt version from `config.json` (`cfg.AgentPrompts`).
   - Set `AgentModels` to production routing, so `loadPromptForAgent`'s `_flash` logic
     matches production.
   - Write raw outputs plus a summary. The previous probe's design (freeze the
     Tone Historian and Community Scraper outputs per query, call only the Librarian
     under each condition, compare within-condition vs across-condition overlap of
     catalog names) is fine. Its prompt-version bug was the problem.

Acceptance: `go vet ./...` and `go test ./...` are green, and new unit tests cover the
judge logic. Doc comments at the top of both tools are updated.

## Workstream 3: re-measure (paid; owner approval required)

Preconditions: workstreams 1 and 2 merged; PR #110 merged or rebased onto them.

- **Question:** with the fixed catalog, does putting the static catalog first change
  preset quality compared with the old order?
- **Conditions,** both with the sorted amp menu, which caching requires anyway:
  - `new`: current code.
  - `old`: a worktree where only `buildLibrarianContext`'s format string is reverted to
    `"Tone History: %s\nScraper: %s\n%sConstraints: %v"` (per-run text before the
    catalog). Don't add a production toggle for this.
- **Run:**
  - `eval_full_pipeline -scenarios baseline -samples 3` in each worktree, which is
    13 × 3 × 2 = 78 pipelines.
  - Then `judge_compare` with swapped judging (2 judge calls × 39 pairs).
  - Rough cost (unverified estimate, from the 2026-11-04 runs at ~56K input / ~5K
    output tokens per pipeline): **~$20-40 total**. Measure the actual cost from the
    manifest's token counts and `geminiPriceTable`, and report it.
- **Decide in advance:**
  - Call it a regression only if consistent pairs favor `old` with a sign-test
    p < 0.05.
  - Also report `Range Rverb` incidence (expected to drop to 0), cached share per
    condition, and the failed-pipeline count.
  - Report how much Librarian outputs (from diagnostics) differ between repeats of
    the same condition vs between conditions.
- **If a regression is found,** try in order:
  - (a) Move the static catalog into the system instruction (after the role prompt) so
    the user turn is just the per-run task. This is cache-compatible.
  - (b) Order the amp menu by a fixed, meaningful order (archetype family) instead of
    alphabetically.

  Re-measure after each change.

## Related follow-up (not in this plan)

- Acoustician MaxOutputTokens truncation (5 failures across the 2026-11-04 eval runs). Check its `agentMaxOutputTokens`
  value in `schemas.go`, and whether the output is degenerate.
- Mirror `cachedInput` rates into job_tracker's `internal/scoring/pricing.go` (noted
  in PR #110).
- Verify cached-input prices for `gemini-3.5-flash` / `gemini-3.6-flash` before using
  recorded cost for decisions.
