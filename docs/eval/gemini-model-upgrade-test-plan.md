# Gemini Model Upgrade — Test Plan

Status: **Phase 0-1 complete; Phase 2 gates done (quality scoring §6.2 pending); Phase 1B complete (control failed R1, so candidates were compared against the control's own rate; see Phase 1 results)** · Written 2026-11-04 · Owner: bweitzer

## 1. Goal

Decide, per agent, whether to move off the current production routing onto a
newer Gemini model, using evidence that holds up better than the July 2026
`gemini-3.6-flash` eval did (see TODO.md, "Gemini 3.6 Flash Migration").

**Current production routing** (`RunAgentSplit`, `internal/agents/orchestrator.go`):

| Tier | Agents | Model |
|---|---|---|
| Pro | 1 Tone Historian, 12 Architect (+ Refinement Architect) | `gemini-3.1-pro-preview` |
| Flash | 2–11, 13 Preset Critic | `gemini-3.5-flash` |
| Fallback | `getFallbackChain` | → `gemini-3.5-flash` → `gemini-3-flash-preview` → `gemini-2.5-pro` |

**Candidates.** Each one was confirmed to appear in `models.list` for our API key
on 2026-11-04 and to advertise `generateContent`. None of them has been called yet.

| Model | Why test it | Known facts (from `models.list`) | Unknowns |
|---|---|---|---|
| `gemini-4-argon` | Only Gemini 4 entry; requested for **all agents** | 1M in / **32,768 out**; thinking: true; no `createCachedContent` | Preview vs. stable, price, rate limits, Google Search grounding support, longevity (the codename and `version: "3.0"` look provisional) |
| `gemini-3.8-flash` | Newest Flash | 1M in / 65,536 out; thinking: true | Price (third-party sites quote $0.75/$3.75 per M tokens, introductory through 2026-12-31, *unverified*) |
| `gemini-pro-latest` | Alias, possibly pointing at a Pro newer than 3.1 | 1M in / 65,536 out | **Which model it resolves to** — tested only if it isn't `gemini-3.1-pro-preview` |
| `gemini-3.5-flash-lite` | Cost/latency probe for mechanical agents | 1M in / 65,536 out | Quality on structured-output agents |

Not tested: `gemini-3.6-flash` and `gemini-3.7-flash`. Both are superseded by 3.8,
and 3.6 already failed its July eval.

## 2. Principles (lessons from the July eval)

1. **Deterministic checks gate; the judge only breaks ties.** In July the LLM judge
   produced at least two verified hallucinations ("ODS Joe Lead is fabricated",
   "Circular Delay is Axe-Fx only"). Every hard gate below is a machine check.
2. **Swap one variable at a time.** Change one tier per scenario. The Critic stays
   on the baseline model except in scenarios where the Critic is what's being tested,
   because a changed Critic changes the defect count it reports.
3. **Never score a model that didn't answer.** Any call served by a fallback model
   counts as a failure for that scenario, not a data point.
4. **n ≥ 2 reps per query.** The July Pro-tier verdict (11/5/1) rested on one rep.
5. **Same denominator across scenarios.** Same 13 golden queries
   (`internal/evalfixtures`), same reps, same plugin/capture flags, and same
   `agentMaxOutputTokens` caps. Compare a scenario only against a baseline run
   in the same eval batch, not against July's numbers.

## 3. Phase 0 — Prerequisites (code + ≤ ~20 API calls)

No quality evaluation happens until all of these are done.

| # | Task | Where | Exit criterion |
|---|---|---|---|
| 0.1 | **Eval no-fallback mode.** Add an env flag (e.g. `EVAL_NO_FALLBACK=true`) that makes `getFallbackChain` return nil, so a failed candidate call fails visibly. | `orchestrator.go` | A forced-bad model ID fails rather than silently serving `gemini-2.5-pro` |
| 0.2 | **Record the served model per call.** Capture `GenerateContentResponse.ModelVersion` plus `FinishReason` per agent call in eval output. Today only `TokenUsage.ModelsUsed` (requested-name counts) is recorded. | orchestrator + `cmd/eval_full_pipeline` | Report has a per-call `served_model` / `finish_reason` column |
| 0.3 | **Resolve `gemini-pro-latest`.** Make one call and read `ModelVersion`. If it is `gemini-3.1-pro-preview`, drop scenario S3. If it's something newer, pin the resolved ID, not the alias. | one-off | Resolved ID written into this doc |
| 0.4 | **Done: baseline is NOT 3.5** (3 calls) | `gemini-3.5-flash` reported `ModelVersion: gemini-3.6-flash` in the first call, in the repeat, and in all 5 later 0.5 cells (7 of 7); the control call to `gemini-3.6-flash` reports `gemini-3.6-flash`. `models.get` still lists 3.5-flash as version `3.5-flash-05-2026`, so the id is silently served by 3.6. Production's Flash tier is therefore 3.6 Flash. |
| 0.5 | **Done (28 calls; follow-ups for 2 models cut by the cap)** | Matrix below. All 5 probed models were served as requested (3.5-flash as 3.6, pinned). |
| 0.6 | **Pricing entries.** Add the candidates to `geminiPriceTable` (`usage_recorder.go`) and keep `job_tracker`'s `internal/scoring/pricing.go` in sync. Use official prices when available. Until then, record tokens and compute cost afterwards, and mark any figure as unverified. | `usage_recorder.go` | No candidate's cost records as $0 |
| 0.7 | **Parameterize the eval tools.** `eval_full_pipeline` gets `-scenarios`, `-queries`, `-reps`, and `-dry-run` (default true, like `eval_thinking_matrix`). `judge_compare` gets a `JUDGE_MODEL` env var and randomizes A/B position per query, if it doesn't already. | `cmd/` | `-dry-run` prints the run count for every scenario in §5 |
| 0.8 | **Deterministic checker** (new `cmd/eval_checks` or a function in `eval_full_pipeline`). Over each run's output, count: (i) non-numeric `Value`/`ValueB` (the units-in-numbers regression); (ii) Critic-flagged contradictions by type, especially "capture claimed on a modeled block" (the Iba Green regression); (iii) block names changed by `ApplyFuzzyCorrection`, a proxy for invented block names (add a counter if one isn't exposed); (iv) `MAX_TOKENS` finish reasons per agent; (v) pipeline success/failure. | new | Running it on the July `eval_results/full_pipeline` outputs reproduces the July defect counts |

### Phase 0 status (updated 2026-11-04)

| # | Status | Result |
|---|---|---|
| 0.1 | **Done** | `EVAL_NO_FALLBACK=true` makes `getFallbackChain` return nil for every model and also disables the "retry without grounding" degrade path (which would otherwise turn a failed schema+tools call into a quiet success). Unit-tested for both settings. `eval_full_pipeline` refuses live runs without it. |
| 0.2 | **Done** | genai v1.63.0 exposes `GenerateContentResponse.ModelVersion` and `Candidate.FinishReason` (checked in the module source). Every attempt (success, failure, truncation, fallback) is appended to `TokenUsage.Calls` as a `CallRecord`; `eval_full_pipeline` writes `<query>.calls.json` per run plus `integrity_ok` in `runs.jsonl`. `UsageRecord` also gains `servedModel`/`finishReason`. |
| 0.3 | **Done** (1 call) | `gemini-pro-latest` is served as **`gemini-3.1-pro-preview`**. S3 is therefore dropped (marked skipped in `eval_full_pipeline`; `-include-skipped` forces it). |
| 0.4 | **Done, surprising** (1 call) | A call to `gemini-3.5-flash` reported `ModelVersion: gemini-3.6-flash`. `models.get` still lists it as version `3.5-flash-05-2026`, so the id and the served model disagree. **One observation only**: a repeat is in the proposed 0.5 grid. If it holds, the production Flash tier and the plan's baseline are 3.6 Flash, not 3.5 (see "Corrections" below). |
| 0.5 | **Tool done, not yet run** | `cmd/probe_thinking` is now the capability probe (raw genai client, so no fallback; served-model checked on every cell; `-dry-run` default; `-max-calls` hard cap). Awaiting sign-off on the grid below. |
| 0.6 | **Done** | No official prices exist for the four candidates, so none were invented. Calls on unpriced models record `costUnknown: true`; rollups and `cmd/usage_report` show "unknown" / "EXCLUDES N calls" instead of $0. `job_tracker`'s `pricing.go` is a separate repo and needs no change until a row is added here. |
| 0.7 | **Done** | `eval_full_pipeline`: `-scenarios -queries -reps -dry-run(default true) -include-skipped -argon-no-search -allow-fallback`; all 9 scenarios defined and unit-tested (each differs from S0 on exactly the agents the plan says). `-dry-run` prints `TOTAL: 234 pipeline runs` (208 executable, S3 skipped). `judge_compare` already randomized A/B per query; it now also has `JUDGE_MODEL` and records the judge, its served model and the swap per verdict. |
| 0.8 | **Done, with corrections** | `internal/evalcheck` + `cmd/eval_checks`, validated against `eval_results/full_pipeline` (see below). |

#### 0.8 validation against the July outputs

`go run ./cmd/eval_checks -root eval_results/full_pipeline -report eval_results/full_pipeline/full_pipeline_gemini36_benchmark_report.md`
(also pinned as `TestValidationAgainstJulyOutputs`).

| July finding | Reproduced? | Evidence |
|---|---|---|
| Capture claimed on a modeled block, `07_Edge` | **Yes, but the block is `UK C30 TopBoost`, not Iba Green** | pro-tier-candidate 07_Edge: 2 rationale claims ("Factory Neural Capture") on a block `coros_map.json` marks `is_capture:false`, plus 12 relative-dB values on its Gain/Bass/Mid/Treble/Volume. The Critic flagged it too (2 notes). Baseline 07_Edge: 0. |
| Iba Green described as a Neural Capture | **No, and that is correct** | `coros_map.json` has `Iba Green` with `is_capture:true`, and Architect Rule 9 itself names Iba Green as a static-snapshot capture. The flash-tier 07_Edge output calls it a "pre-trained Neural Capture" with dB values: right per ground truth. The only Iba Green flags in the retained outputs are 2 **Critic** notes on 13_Hard_Rock_Blues calling it "a native pedal", which contradict the map (the checker reports these as `critic_capture_contradicts_map`). I did not bend the checker to flag a correct statement. |
| Relative-dB / unit formatting on algorithmic blocks | **Yes** | pro-tier 05_Clapton: 4 dB values on `Brit Plexi 100 Patch` (Rule 9's own named algorithmic example). |
| Units in numeric fields, `09_BB_King` | **Not reproducible from retained files** | The only five-name dB values on 09_BB_King are Gain on a `Parametric EQ` (+2.5 dB), where dB is a normal EQ unit, and the capture-eligible gate (amp/cab/drive family) rightly excludes it. The retained files are the second July round; first-round outputs are not in the repo. |
| Acoustician truncations | **Yes** | 4 failed runs in the report, all `[Acoustician] ... truncated at the MaxOutputTokens limit` (2 on 3.6 Flash flash-tier, 1 baseline, 1 pro-tier). |

Other checker output on July data: unverified (invented) gear names appear even in baseline and flash-tier runs (`1x15 JBL D130F IR`, `1x8 US Tweed Cab`, `Two Rock 2x12`), so §6.1's "fuzzy-corrected names ≤ S0" gate has real signal.

#### 0.5 capability matrix (run 2026-11-04; raw JSON: `eval_results/capability_probe/phase0_matrix_20261104.json`)

Call budget used: 37 (0.3: 1, 0.4: 3, 0.5: 28, grounding follow-up: 4, Argon forced-search: 1 (`phase0_argon_forced_20261104.json`); raw: `phase0_grounding_20261104.json`). `gemini-pro-latest` was not probed separately: it is `gemini-3.1-pro-preview`. Every cell records the served model; all matched except `gemini-3.5-flash`, which is served as `gemini-3.6-flash` (expected value pinned for that row).

| Model (served as) | (a) responseSchema | (b) GoogleSearch alone | (c) schema + search in one call | (d) budget 512 / 2048 / 0 | (e) MaxOutputTokens=64 |
|---|---|---|---|---|---|
| `gemini-4-argon` | pass | **pass** (grounding metadata present) | first prompt: no search shown; **forced-search prompt: search ran with schema set (pass)** | all accepted; 0 thinking tokens at every budget | pass (out=64) |
| `gemini-3.8-flash` | pass | **pass** | accepted, search not shown to run | all accepted; 0 / 114 / 0 thinking tokens | pass, but cap consumed by thinking (out=0, thinking=61) |
| `gemini-3.5-flash-lite` | pass | **pass** | accepted, search not shown to run | 512, 2048 ok; **0 rejected** | pass (out=60) |
| `gemini-3.1-pro-preview` | pass | **pass** | first prompt: no search shown; **forced-search prompt: search ran with schema set (pass)** | 512, 2048 ok; **0 rejected** | pass, cap consumed by thinking (out=0, thinking=60) |
| `gemini-3.5-flash` (served 3.6) | pass | **pass** | accepted; search not shown to run on either prompt (1 sample each) | all ok incl. 0 (0 thinking tokens) | pass, cap consumed by thinking (out=0, thinking=61) |

Raw errors for the failures:
- `gemini-3.5-flash-lite`, budget=0: `Error 400, Message: Request contains an invalid argument., Status: INVALID_ARGUMENT`
- `gemini-3.1-pro-preview`, budget=0: `Error 400, Message: Budget 0 is invalid. This model only works in thinking mode., Status: INVALID_ARGUMENT`

How to read this:
- **No candidate hard-fails a must-have capability** (a)/(b): structured output works on all, and GoogleSearch alone works on all three candidates it was run on, **including Argon**.
- **Follow-up (4 calls):** `gemini-3.1-pro-preview` DOES ground with a schema set when the prompt demands a live lookup, so the first miss was the prompt, and combined schema+search works on the Pro baseline. `gemini-3.6-flash` (as `gemini-3.5-flash`) grounds alone but showed no search in the combined call on either prompt (1 sample each, so suggestive, not proof). **Argon, with the forcing prompt, DID search with a schema set (1 call, grounding metadata present, valid JSON, served as `gemini-4-argon`), so Argon can use schema+GoogleSearch together and is not excluded from agents 1/2/3.** Caveat: single sample, and models skip search on prompts that don't demand it, which is also true of the baselines.
- **(c) as first measured (before the follow-up) was unproven for every model, baselines included.** The combined call was accepted (no 400, unlike the `gemini-2.5-pro` rejection noted in the orchestrator) and returned schema-valid JSON, but no response carried grounding metadata, including `gemini-3.1-pro-preview` and `gemini-3.5-flash`, which production uses this way for agents 1-3. Two readings fit the data and the cap prevented separating them: (1) with a JSON schema set, these models skip search silently, i.e. agents 1-3 may be effectively ungrounded in production today; or (2) the probe prompt did not trigger a search. A tools-only call on the baselines plus a search-forcing combined prompt would settle it (about 4 calls). **So "Argon can be used on agents 1/2/3" is: the tool is supported, but combined-call grounding is not demonstrated for Argon or anything else.**
- **Thinking:** `budget=0` is rejected by `gemini-3.1-pro-preview` and `gemini-3.5-flash-lite`, accepted by Argon, 3.8 Flash and 3.5/3.6 Flash (0 thinking tokens). The trivial probe prompt makes 512/2048 weak evidence (thinking stayed below budget, Argon reported 0 thinking tokens at all budgets); it shows "accepted", not "tuned".
- **MaxOutputTokens:** honored by all, but on thinking models the cap includes thinking tokens, so a tight cap can yield empty output with `MAX_TOKENS`. Relevant to Acoustician-style truncations.
- Argon's output limit and thinking behaviour need more than this probe to characterise; its 32,768 limit was not exercised.

#### Corrections to this plan found in Phase 0

1. **§3 0.8(i) / §6.1 "0 non-numeric `Value`/`ValueB`" is unsatisfiable.** `value` is a free string in the Architect schema and the prompt prescribes unit-bearing values; July's *baseline* outputs contain ~390 of them. (TODO.md already recorded that a blanket units check was removed for this reason.) The checker reports it as a reference number only; the gated defect is Rule 9 formatting vs. capture status. R2 should be redefined that way.
2. **§3 0.8(iii) "names changed by `ApplyFuzzyCorrection`" always counts 0.** The output envelope is JSON-marshalled with `<` escaped as `\u003c`, so `ApplyFuzzyCorrection`'s `<td` regex never matches (demonstrated: a typo injected into a July output survived it). The checker counts snapped/unverified gear names over `structured_payload` instead. Whether `ApplyFuzzyCorrection` being a no-op in `RunPipeline` is a production bug is outside this PR.
3. **R1 (§4) as written is probably a false premise.** See the Iba Green row above; the reproducible July defect is the `UK C30 TopBoost`/Rule 9 pattern. Redefine R1 as "capture claimed or dB-formatted on a block the map says is not a capture", on `07_Edge` and `05_Clapton`.
4. **The baseline is not 3.5 Flash** (0.4, 7 of 7 calls). S0's Flash tier is really 3.6 Flash; if the alias behaved the same in July (no July call recorded a served model, so this is unknown, not confirmed), July's "3.5 vs 3.6" Flash-tier comparison was 3.6 against itself; and §6.1's served-model gate (served == requested) would fail S0. **Decision: keep requesting `gemini-3.5-flash` in S0 (it is what production sends) and record the pinned served id `gemini-3.6-flash` as its expected value** (`Pins` in `eval_full_pipeline`), so the gate stays meaningful for every scenario. Report S0 as "production Flash tier (served: 3.6 Flash)" and treat S1 (3.8) as 3.8-vs-3.6. Also note 3.5 Flash's deprecation is moot if it already aliases 3.6.
5. §3 0.5(c): the schema+tools combination is accepted by every model but search was never shown to run with a schema set, baselines included (see matrix).
6. The Critic is not ground truth: it disputed a real capture (Iba Green) twice in July.

Run 0.8 against the July outputs **before** using it on new data. That's how we
know the checker sees the defects it's supposed to catch.

## 4. Phase 1 — Cheap screens and known-regression repros

These are single-agent or targeted runs. The point is to cut candidates before
paying for full pipelines.

**1A. Independent-agent screen** (`cmd/eval_subagent`, generalized beyond its
hard-coded `gemini-3.6-flash`). Covers the 3 agents whose input doesn't depend
on upstream output. 13 queries × 2 reps.

| Agent | Baseline | Candidates |
|---|---|---|
| 1 Tone Historian | `gemini-3.1-pro-preview` | `gemini-4-argon`, `gemini-3.8-flash` (pro-latest dropped: served as 3.1-pro-preview) |
| 2 Sonic Profiler | `gemini-3.5-flash` | `gemini-4-argon`, `gemini-3.8-flash`, `gemini-3.5-flash-lite` |
| 3 Community Scraper | `gemini-3.5-flash` | `gemini-4-argon`, `gemini-3.8-flash`, `gemini-3.5-flash-lite` |

\* only if 0.3 shows it resolves to something newer than 3.1-pro-preview.

**1B. Known-regression repros** on the Architect (agent 12). Upstream agents 1–11
stay on baseline; only the Architect model changes. 5 reps each.

| Repro | Query | Pass condition |
|---|---|---|
| R1 (redefined, see Phase 1 results) capture claimed or dB-formatted on a block the map says is not a capture | `07_Edge` | 0/5 runs with `capture_claim_on_modeled > 0` or `capture_db_on_modeled > 0` |
| R2 Rule 9 dB formatting on a non-capture block (the checker's `capture_db_on_modeled`; replaces "units inside numeric fields", see Phase 0 correction 1) | `09_BB_King`, `05_Clapton` | 0/5 runs with `capture_db_on_modeled > 0` |

Run for `gemini-4-argon`, `gemini-3.8-flash`, `gemini-pro-latest`*, and the baseline
as a control. The baseline must pass both, or the check itself is broken.

**Phase 1 exit:** any candidate that fails R1 or R2, or a Phase 0.5 must-have
capability, is dropped from the Pro tier. Any candidate whose 1A schema-validity
or truncation rate is worse than baseline is dropped from that agent.

## 4a. Phase 1 results (run 2026-11-04)

Raw data: `eval_results/phase1/subagent/` (`calls.jsonl`, one record per call incl. served model, finish reason, grounded, schema issues, output; `summary.md/json`), `eval_results/phase1/architect/` (`frozen/*.json` = the frozen upstream, `full/` = the run, stopped at the control gate), plus `*_smoke` dirs. Tools: `cmd/eval_subagent`, `cmd/eval_architect` (PR #114). Live calls spent, approximately: 1A smoke 11 + full 286; 1B 20 wasted on a freeze check bug (see below) + 10 freeze for the smoke + 6 smoke run + 20 freeze + 30 control (Architect + Critic) = about 385. Every `EVAL_NO_FALLBACK=true`; all 286 1A calls were served by the requested model in one attempt (3.5-flash served as 3.6-flash, pinned).

### 1A: independent-agent screen (13 queries x 2 reps = n 26 per cell)

| Agent | Model | schema valid | truncated | grounded | p50 / p95 latency (s) | Exit rule |
|---|---|--:|--:|--:|--:|---|
| 1 Tone Historian | `gemini-3.1-pro-preview` (baseline) | 26/26 | 0 | 0/26 | 12.6 / 18.2 | n/a |
| 1 | `gemini-4-argon` | 26/26 | 0 | 3/26 | 10.9 / 26.8 | keep |
| 1 | `gemini-3.8-flash` | 26/26 | 0 | 0/26 | 10.7 / 29.1 | keep |
| 2 Sonic Profiler | `gemini-3.5-flash` (served 3.6, baseline) | 26/26 | 0 | 0/26 | 6.9 / 10.5 | n/a |
| 2 | `gemini-4-argon` | 16/26 | **10** | 3/26 | 11.2 / 41.8 | **DROP** |
| 2 | `gemini-3.8-flash` | 26/26 | 0 | 0/26 | 9.4 / 24.8 | keep |
| 2 | `gemini-3.5-flash-lite` | 24/26 | **2** | 0/26 | 1.8 / 8.4 | **DROP** |
| 3 Community Scraper | `gemini-3.5-flash` (served 3.6, baseline) | 26/26 | 0 | 6/26 | 10.6 / 13.9 | n/a |
| 3 | `gemini-4-argon` | 26/26 | 0 | 4/26 | 9.7 / 23.7 | keep |
| 3 | `gemini-3.8-flash` | 26/26 | 0 | 4/26 | 17.8 / 45.0 | keep |
| 3 | `gemini-3.5-flash-lite` | 26/26 | 0 | 5/26 | 2.3 / 2.9 | keep |

- The Argon and Lite Sonic Profiler truncations are runaway generations, not thinking-token starvation: truncated outputs are ~3,970-4,050 output tokens against the 4,000 cap (baseline mean output 78 tokens), with 0 thinking tokens on 7 of 10 Argon cases.
- **Grounding (measured, not assumed):** the production baselines were grounded on 0/26 (agent 1), 0/26 (agent 2) and 6/26 (agent 3) calls, so agents 1 and 2 are effectively **not grounded in production today** despite `agentSearchTool`, and agent 3 only 23% of the time. Argon grounded 3/26, 3/26, 4/26, at or above baseline. So grounding does not stop Argon from staying on agents 1 and 3. Caveat: this tool sends the raw query without the pipeline's `[GLOBAL USER CONFIG FLAG ...]` prefixes, one sample per cell, and `grounded` means search queries or chunks present in the response metadata.
- Strict exit rule, no tolerance: at n=26 one call is ~4 points. No other candidate had a single schema failure, truncation or served-model issue.

### 1B: Architect repros (control gate tripped; candidates then run on the owner's approval)

Frozen upstream: agents 1-11 once per query on baseline (`frozen/`). Control = `gemini-3.1-pro-preview`, 5 reps per query, Critic on baseline, all 15 runs scoreable and served as requested.

| Rule | Queries | Control result |
|---|---|---|
| R1 | `07_Edge` | **FAIL: 2/5 runs** (`capture_db_on_modeled` = 4 in reps 3 and 5: `UK C30 TopBoost` Volume written in relative dB, e.g. `"-2.0 dB"`, on both guitars) |
| R2 | `09_BB_King`, `05_Clapton` | PASS: 0/10 |

The tool stopped before any candidate was run, as required. I inspected the evidence: the flagged values are on `UK C30 TopBoost`, a modeled amp (`is_capture: false`), so the check is correct, and the frozen prompt is the production one (the Librarian mandated the amp, and the Rule 9 capture list contained only `Iba Green`). **The control model itself has the defect at 2/5, so R1 as a zero-defect pass bar is unsatisfiable for the baseline, and the July attribution of this pattern to 3.6 Flash was never isolated from the baseline's own rate** (The retained July baseline 07_Edge output had 0 such values, but that is a single run, which says little about a rate near 2/5; unverified either way beyond that.) Smoke pass (n=1, not a conclusion): control 0/1, Argon 0/1, 3.8 Flash 1/1 defect on 07_Edge; all served as requested, no MAX_TOKENS.

**Candidate runs (run after the stop, with `-no-stop-on-control-failure`; same frozen upstream, 5 reps per query; raw: `eval_results/phase1/architect/candidates/`)**

| Model | R1 `07_Edge` defect runs / scoreable | R2 `09_BB_King` + `05_Clapton` | Architect MAX_TOKENS | Failed runs | Fuzzy-snapped / unverified names | served as requested |
|---|--:|--:|--:|--|--:|--|
| `gemini-3.1-pro-preview` (control) | **2 / 5** | 0 / 10 | 0 | none | 0 / 0 | 15/15 |
| `gemini-4-argon` | **4 / 4** | 0 / 10 | 0 | 1 transient connection reset (`07_Edge` rep2; infrastructure, not a model fault; unscoreable) | 0 / 0 | 14/14 answered |
| `gemini-3.8-flash` | **5 / 5** | 0 / 9 | **1** (`09_BB_King` rep5: 15,359 of 16,000 tokens went to thinking, 627 to output) | that truncation | 0 / 0 | 15/15 |

**Follow-up sample, run on the owner's instruction to bias toward upgrading unless a candidate is clearly worse (15 more `07_Edge` reps per model, same frozen upstream; raw: `eval_results/phase1/architect/extra_edge_*`). Pooled with the first 5, 20 runs per model:**

| Model | `07_Edge` defect runs / scoreable | rate | Architect MAX_TOKENS on `07_Edge` | unscoreable |
|---|--:|--:|--:|---|
| `gemini-3.1-pro-preview` (control) | 3 / 20 | 15% | 0 | 0 |
| `gemini-4-argon` | 18 / 19 | 95% | 0 | 1 connection reset (`07_Edge`; infrastructure) |
| `gemini-3.8-flash` | 17 / 20 | 85% | 0 | 0 |

Fisher exact, two-sided, each candidate vs the control: p < 0.001 for both (computed in this session). The earlier "not distinguishable" reading was a small-sample artifact: with 20 runs the candidates are clearly worse than the production Pro model on R1. Every defect is the same pattern: relative-dB Volume on `UK C30 TopBoost`; no capture-claim defects anywhere. R2 (`09_BB_King`, `05_Clapton`) was not extended: 0 defects for all three models in 5 reps per query (3.8 Flash: 9 scoreable because of one truncation).

### 1B follow-up: why the Architect writes dB on a modeled amp, and a prompt fix (2026-11-05)

**Broader measure.** `internal/evalcheck`'s `capture_db_on_modeled` only counts values with a `dB` unit. `docs/eval/scripts/amp_volume_audit.py` also counts bare signed offsets such as `Volume: -2.0`, which are just as unusable on a 0-10 dial. Re-scored that way, production v4 on `07_Edge` is wrong in **7/20** Pro runs (3 dB + 4 bare signed; 13 of its "clean" runs simply omit the amp `Volume` parameter), **18/19** Argon, **17/20** 3.8 Flash.

**Root cause, from the per-run logs (frozen input + raw Architect output + call records):**
1. The upstream agents hand the Architect dB values for a modeled amp. The Acoustician's prompt opens "calculate precise dB offsets", and its output says "-2.0dB volume offset" for the ES-339. The Control Mapper is told to "strictly adhere to the dB ... constants" from the Acoustician and writes `UK C30 TopBoost: ... Level = -2.0 dB` (Scene A) / `Level = 0.0 dB` (Scene B). Those are exactly the humbucker values in the defective runs.
2. The Acoustician returned only the `humbucker` profile, though the run has two guitars, so every Telecaster dB value (`+1.5 dB`, `+1.0 dB`) was extrapolated by the Architect.
3. The orchestrator's Architect user prompt says "based strictly on the following aggregated data payload", and Rule 9's only `Volume` example is the capture form `Volume: 0.0 dB`, the most common wrong value. Nothing tells the Architect how to turn an offset into a dial, and there is no final format check (there is one for Bypass, rule 13).
4. Why the newer models look worse: Pro mostly drops the amp `Volume` parameter (13/20), which hides the conflict. Argon and 3.8 Flash faithfully include every AC30 control and carry the upstream offset into it.
5. The Critic (baseline) caught 0 of 38 defective runs, and instead flagged Iba Green (a real capture) as "not a capture" again.
6. Argon used **0 thinking tokens** on all 19 default Architect calls (Pro ~4,200, 3.8 Flash ~2,500). `ThinkingConfig.ThinkingLevel=HIGH` (new `Orchestrator.ThinkingLevel`, eval-only) makes it think (~3,900-5,500 tokens).
7. `evalfixtures.DefaultConstraints` sets `favor_captures: true`, which prefixes every prompt with "prioritize Neural Captures"; a plausible nudge toward capture formatting, not tested separately.

**Experiment** (same frozen `07_Edge` upstream, 5 reps per arm, Critic on baseline; raw: `eval_results/phase1/architect_promptfix/`). `12_architect_v5.md` = v4 plus: rule 10 "translate upstream dB offsets into 0-10 dials on algorithmic blocks", rule 14 "final dial-format check", and a modeled-amp counter-example in Rule 9 (which keeps its number, because the orchestrator's user prompt cites "Rule 9").

| Arm | Wrong-format runs (audit) | `capture_db_on_modeled` runs | Iba Green (capture) still in dB | MAX_TOKENS | median thinking / latency |
|---|--:|--:|---|--:|---|
| Argon, v4, thinking default (earlier) | 18/19 | 18/19 | yes | 0 | 0 / ~49 s |
| A: Argon, v4, thinking HIGH | **4/5** | 4/5 | yes | 0 | 3,852 / 94 s |
| B: Argon, **v5**, default | **0/5** | 0/5 | yes | 0 | 0 / 59 s |
| C: Argon, **v5**, HIGH | **0/5** | 0/5 | yes | 0 | 5,534 / 104 s |
| D: 3.8 Flash, **v5** | **0/5** | 0/5 | yes | 0 | 3,222 / 53 s |
| E: Pro, **v5** | **0/5** | 0/5 | yes | 0 | 4,967 / 54 s |

- **The prompt fixes it; thinking alone does not** (arm A). Under v5, all three models write plain dials, and in every v5 run the humbucker's AC30 Volume is lower than the Telecaster's (e.g. 5.5 vs 6.5), i.e. the -2 dB intent was applied in the right direction. No new snapped/unverified names, capture claims, parse failures or truncations.
- **Consequence for 1B:** the R1 failures of Argon and 3.8 Flash on v4 were a prompt defect shared by every model (production included), not a model regression. Under v5 both pass R1 (0/5). **They should not be dropped from agent 12 on R1 if v5 is adopted.**
- **v5 regression check on the other repro queries (`09_BB_King`, `05_Clapton`, 5 reps each per model, 30 runs; raw: `eval_results/phase1/architect_promptfix/R2_v5_*`):** all 30 scoreable, 0 MAX_TOKENS, 0 on every gated counter, and 0 dial-format problems on the broader audit. That includes Argon on `05_Clapton`, which under v4 wrote `Brit Blues Level = +3.5 dB` (a modeled drive) in **4/5** runs, a defect the narrow checker missed (see below). Block counts per run are unchanged within noise (Pro 13.4 -> 13.2, Argon 13.2 -> 13.0, 3.8 Flash 11.6 -> 10.6; n = 9-10, watch in Phase 2). Critic flags: 3.8 Flash 2 (a real "Range Rverb is not a spring reverb" prose catch), others 0.
- **Checker blind spot found and partly addressed.** `capture_db_on_modeled` only looks at Rule 9's five parameter names and only at values with a `dB` unit. The broader audit (`docs/eval/scripts/dial_format_audit.py`, any amp/drive dial on a non-capture block, dB or bare signed) shows production Pro on v4 `07_Edge` is wrong in **19/20** runs, not 3/20: it mostly writes the offset into an invented `Level` control (`Level = -2.0 dB`; the AC30 has no `Level` in the QC amp vocabulary) or a bare `Volume = -2.0`. `internal/evalcheck` now reports this as `dial_offset_on_modeled` (reference only, not gated, because some algorithmic controls may be genuinely dB-native and that is not verified per block). Earlier R1/R2 numbers in this section use the narrow counter and therefore understate the v4 problem for every model.
- **Not yet tested:** v5 on the other 10 golden queries; v5 against a different upstream sample (one freeze per query); the upstream prompts themselves (Acoustician / Control Mapper emitting dB for modeled amps; Acoustician skipping the second guitar). v5 is a downstream guard, not a fix of the upstream contradiction. Production still routes `12_architect` to v4 (`config.json`); nothing was deployed.

### 1C: Phase 1 exit report

**Dropped candidates (and why)**
- Sonic Profiler (2) on `gemini-4-argon`: schema valid 16/26 vs 26/26 baseline, 10/26 truncated (exit rule).
- Sonic Profiler (2) on `gemini-3.5-flash-lite`: schema valid 24/26, 2/26 truncated (exit rule).
- `gemini-pro-latest` and S3: served as `gemini-3.1-pro-preview` (Phase 0.3).
- `gemini-4-argon` and `gemini-3.8-flash` on the Architect (agent 12): both fail R1 as the plan defines it (any defective `07_Edge` run), 4/4 and 5/5. **The control also trips R1 (3/20 = 15%), so a zero-defect bar would drop the baseline too; the decision rests on the comparison instead: both candidates are clearly worse than the control (95% and 85% vs 15%, p < 0.001), after the owner asked for a larger sample because the bias is to upgrade unless a candidate is clearly worse. It is clear.** The defect is Architect-specific, so I did NOT drop either from agent 1 (1A evidence only): the plan says "dropped from the Pro tier" (agents 1 and 12); splitting it is a judgment call for the owner.
- `gemini-3.8-flash` additionally truncated once on the Architect (1/15, thinking consumed the 16,000-token cap), which is worse than the control's 0/15 on the Phase 2 truncation gate.
- No candidate failed a Phase 0.5 must-have capability.

**Scenarios surviving into Phase 2**
- S0 baseline: always.
- S1 (Flash tier -> 3.8 Flash): survives (3.8 passed 1A on agents 2 and 3; its Architect result is irrelevant here because the Pro tier stays on baseline).
- S2 (Pro tier -> 3.8 Flash): **dropped** (3.8 fails R1 on agent 12 and truncated once). A variant with 3.8 on agent 1 only is possible but is not a plan scenario.
- S3: dropped (alias resolves to the baseline Pro model).
- S4 (Lite on 8, 10, 11): survives; Lite was only screened on agents 2 and 3 (and dropped on 2), so Phase 2 is its first test on 8/10/11.
- S5 (Critic -> Argon): survives (the Architect is on baseline there; the Critic was not screened in Phase 1).
- A1 (Argon everywhere): **dropped** (Argon on agent 12 fails R1; on agent 2 fails the 1A exit rule).
- A2 (Argon Pro tier only): **dropped** (same agent-12 failure).
- A3 (Argon Flash tier only): survives, **with a change**: Argon must stay off agent 2 (keep `gemini-3.5-flash`). `BuildScenarios`' `-argon-no-search` is all-or-nothing for agents 1-3 and cannot express this; Phase 2 needs a per-agent override first.
- Net: Phase 2 would run S0, S1, S4, S5, A3 (5 scenarios x 13 queries x 2 reps = 130 pipeline runs, not 234). Not started, as instructed.

**Can Argon stay on agents 1/2/3?** Agents 1 and 3: yes on the evidence above (valid, no truncation, grounds at or above baseline). Agent 2: no (truncation/schema failures).

**What the plan got wrong**
1. "The baseline must pass R1" is false: the production Pro model fails the redefined R1 at 2/5 under identical upstream. Candidates must be compared against the control's measured rate, not against zero.
2. The 12-agent freeze assumption: with the default constraints (`allow_cloud_captures: false`) Cloud Navigator is bypassed with no API call, so a freeze is 10 calls per query, not 11 (30 total, not 33). My first freeze check required 11 outputs and wasted 20 calls before I fixed it.
3. Production agents 1 and 2 are effectively ungrounded (0/26), contradicting the premise in the prompts and in `agentSearchTool`'s comments; a decision for the owner, outside this PR.
4. The 1A tool and the Architect freeze both resolve prompts through production code (`PromptForAgent`), so no `_flash` prompt variant exists today; if one is added, the freeze flags `prompt_matches_frozen`.

**Owner decisions (2026-11-04)**
1. More Architect samples were requested; done (above). Result on the production v4 prompt: Argon and 3.8 Flash are clearly worse than the control on R1. **Superseded by the follow-up:** the cause is a v4 prompt defect, and under `12_architect_v5` all three models pass R1 (0/5 each), so the agent-12 drop is withdrawn pending the v5 decision.
2. Split the Pro tier: agreed. Agent 1 candidates (Argon, 3.8 Flash) stay alive on the 1A evidence; agent 12 candidates are dropped.
3. The `UK C30 TopBoost` volume-in-dB defect (also present in production's own model at ~15%) will NOT be fixed as part of this work.

## 5. Phase 2 — Full-pipeline routing scenarios

`cmd/eval_full_pipeline`: all 13 golden queries × **2 reps** per scenario,
`EVAL_NO_FALLBACK=true`, run sequentially (as the tool already does), with the
baseline included in the **same batch**. Scenarios that Phase 1 eliminated are skipped.

| ID | Scenario | Agents 1, 12 | Agents 2–11 | Agent 13 Critic | Question it answers |
|---|---|---|---|---|---|
| S0 | **Baseline** | 3.1-pro-preview | 3.5-flash | 3.5-flash | Control |
| S1 | Flash tier → 3.8 Flash | 3.1-pro-preview | **3.8-flash** | 3.5-flash | Replace the deprecated Flash? |
| S2 | Pro tier → 3.8 Flash | **3.8-flash** | 3.5-flash | 3.5-flash | Can Flash now match Pro? |
| S3 | Pro tier → pro-latest* | **pro-latest (pinned)** | 3.5-flash | 3.5-flash | Is there a better Pro? |
| S4 | Lite on mechanical agents | 3.1-pro-preview | 8, 10, 11 → **3.5-flash-lite**; rest 3.5-flash | 3.5-flash | Cheaper mechanical agents? |
| S5 | Critic → Argon | 3.1-pro-preview | 3.5-flash | **4-argon** | Better advisory critic? (scored by §6 re-critic) |
| **A1** | **Argon everywhere** | **4-argon** | **4-argon** | **4-argon** | Does an all-Gemini-4 pipeline beat baseline? |
| **A2** | Argon Pro tier only | **4-argon** | 3.5-flash | 3.5-flash | Attributes A1's result to the Pro tier |
| **A3** | Argon Flash tier only | 3.1-pro-preview | **4-argon** | 3.5-flash | Attributes A1's result to the Flash tier |

\* skipped if 0.3 resolves `gemini-pro-latest` to `gemini-3.1-pro-preview`.
If 0.5 shows Argon can't use Google Search, agents 1, 2 and 3 stay on baseline in
A1/A2/A3, and the report must say so prominently.

**Size:** 9 scenarios × 13 queries × 2 reps = **234 pipeline runs (~3,000 agent calls)**.

**Cost gate.** Run a smoke pass first: 1 query (`07_Edge`) × 1 rep × every scenario,
9 runs in total. Multiply the measured cost per run by 234, using the post-introductory
3.8 Flash price if one is known. Then get explicit sign-off before the full run.
Don't estimate from list prices alone; Argon has no published price yet.

**Argon note:** Argon's 32,768 output limit is above every current
`agentMaxOutputTokens` cap (Architect = 16,000), so the caps bind first. Don't
raise any cap during this eval (principle 5).

## 5a. Phase 2 results (run 2026-11-05)

**Setup.** 8 scenarios (S0, S1, S2, S4, S5, A1, A2, A3; S3 skipped) x 13 golden queries x 2 reps = 208 pipeline runs, `EVAL_NO_FALLBACK=true`, **every scenario on `12_architect_v5`** (`-architect-prompt v5`, so the prompt is not a variable between scenarios; S0 is therefore "production models + v5", not literal production). A1/A3 keep agent 2 on `gemini-3.5-flash` (Phase 1A). Scenarios ran as 8 parallel processes (sequential within each). Raw: `eval_results/phase2/full/<ID>/` (`runs.jsonl`, per-run `.html` envelope + `.calls.json`, `run_config.json`); combined `all_runs.jsonl`. Scored with `go run ./cmd/eval_checks -root eval_results/phase2/full -runs eval_results/phase2/full/all_runs.jsonl`. Smoke pass: `eval_results/phase2/smoke/`.

**Integrity.** 0 calls served by a different model than requested, 0 fallbacks. All 55 integrity failures are failed calls: Acoustician truncations, 10 local-proxy connection resets (`read tcp 127.0.0.1...`; infrastructure, spread across scenarios), and 2 Architect truncations.

| Scenario | Scoreable runs | p50 / p95 latency | MAX_TOKENS by agent | Gated defects (capDB / capClaim / snapped / unverified) | Cost/run (priced calls) | Cost/run if Argon = 3.1 Pro and 3.8/Lite = 3.5 Flash prices (**hypothetical**) |
|---|--:|--:|---|---|--:|--:|
| S0 baseline (+v5) | 18/26 | 101 / 148 s | Acoustician 5 | 0 / 0 / 0 / 0 | $0.27 | $0.27 |
| S1 Flash tier -> 3.8 Flash | 18/26 | 138 / 173 s | Acoustician 3, **Sonic Profiler 4** | **14 / 2 / 0 / 2** (one run: Hiwatt `Watt D103 Normal` written as a capture) | $0.11 + 3.8 tokens | $0.27 |
| S2 Pro tier -> 3.8 Flash | 15/26 | 91 / 135 s | Acoustician 9, **Architect 2** | 0 / 0 / 0 / 0 | $0.18 + 3.8 tokens | $0.24 |
| S4 Lite on 8, 10, 11 | 19/26 | 100 / 133 s | Acoustician 6 | 0 / 0 / 0 / 0 | $0.25 + Lite tokens | $0.26 |
| S5 Critic -> Argon | 18/26 | 94 / 155 s | Acoustician 7 | 0 / 0 / 0 / 0 | $0.25 + Argon tokens | $0.25 |
| A1 Argon everywhere (agent 2 baseline) | **26/26** | 121 / 190 s | **none** | 0 / 0 / 0 / 0 | $0.01 + Argon tokens | $0.23 |
| A2 Argon Pro tier | 16/26 | 116 / 169 s | Acoustician 9 | 0 / 0 / **2** / 0 (Architect named a non-existent cab "Fender 5E3 Deluxe - T8 I4 M12") | $0.18 + Argon tokens | $0.23 |
| A3 Argon Flash tier (agent 2 baseline) | **25/26** (1 Critic connection reset) | 115 / 171 s | **none** | 0 / 1* / 0 / 0 | $0.18 + Argon tokens | $0.29 |

\* A3's one capture claim is a checker false positive: the Myth Drive's rationale mentions "the neural capture's input" (the amp downstream), not itself.

**Acoustician truncation by the model it ran on** (all scenarios): `gemini-3.5-flash` (served 3.6) **36 of 127 calls (28%)**, `gemini-3.8-flash` 3/22, `gemini-4-argon` **0/52**. This is the dominant cause of failed pipelines, it is about double July's rate on the same model, and it is what production runs today. The scenarios that differ from S0 only outside the Acoustician (S2, S4, S5, A2) lose runs to the same baseline Acoustician; their success-rate differences from S0 are not attributable to the scenario (e.g. S0 18/26 vs S2 15/26, Fisher two-sided p ~ 0.57, not computed in this session; treat as approximate).

**v5 holds up across all 13 queries.** Under v5, the dial-format problem on amp/drive blocks appears in **1 of 155** scoreable outputs (S1, `06_Gilmour` rep1). `dial_offset_on_modeled` initially also flagged cabinet `Level` values in dB (e.g. `212 UK C30 '65 Level -2.5`); cabinets are now excluded from it because a cab Level is plausibly a genuine dB control (unverified against the QC manual).

**§6.1 gates.** Read two ways: the literal rule, and attributable to the agents the scenario actually changed.

| Scenario | Literal §6.1 | Attributable to the change | Notes |
|---|---|---|---|
| S1 | FAIL | **FAIL** | Rule 9 defect + capture claim on a modeled Hiwatt, 4 Sonic Profiler truncations on 3.8 Flash (S0: 0), 2 invented cab names |
| S2 | FAIL | **FAIL** | 2 Architect truncations on 3.8 Flash (S0: 0; Phase 1B also saw 1); success 15 vs 18 is mostly the unchanged Acoustician |
| S4 | FAIL (Acoustician 6 > 5) | pass | the only failing gate is the unchanged baseline Acoustician |
| S5 | FAIL (Acoustician 7 > 5) | pass | same; the Critic itself is scored in §6.2, not here |
| A1 | **pass** | **pass** | 26/26, no truncations, no defects |
| A2 | FAIL | borderline | 2 snapped (invented) cab names from Argon as Architect; lower success is the unchanged Acoustician |
| A3 | **pass** | **pass** | 25/26 (one Critic connection reset), no truncations, no real defects |

**Not done yet (§6.2 quality):** the uniform re-Critic over all outputs and the two-judge pairwise comparison against S0. Gates say which scenarios are safe; they do not say which presets are better. Also, Argon cannot be adopted in production under §7 until Google publishes pricing and a non-provisional model id.

## 6. Scoring

### 6.1 Hard gates. A scenario fails if any of these fails.

| Gate | Threshold |
|---|---|
| Served-model integrity | 100% of calls served by the requested model (any fallback = failed call) |
| Pipeline success rate | ≥ S0's rate in the same batch |
| Schema validity | 100% of Architect outputs parse (no `ParseError`); `capture_db_on_modeled` and `modeled_style_on_capture` both 0. The raw non-numeric/unit-bearing count is reported for reference but is **not** a gate (Phase 0 correction 1) |
| Truncation (`MAX_TOKENS`) | ≤ S0, per agent |
| R1 capture-mislabel | 0 occurrences across all runs |
| Fuzzy-corrected block names | ≤ S0 (more corrections = more invented block names) |

### 6.2 Quality (scenarios that pass every gate)

1. **Critic defect count per run, re-scored uniformly.** After every scenario,
   re-run one fixed Critic over all outputs (`cmd/critic_probe`, pinned to the S0
   Critic model). This keeps S5 and A1, whose own Critic is Argon, comparable with
   the others. Lower is better. Report mean and spread across reps.
2. **Pairwise judge** (`cmd/judge_compare`, each scenario vs. S0):
   - Judge with **two** models, `gemini-3.1-pro-preview` plus one non-contestant
     for that comparison (e.g. 3.8 Flash for the Argon scenarios, Argon for S1/S2).
     **Count only verdicts where both judges agree;** report disagreements separately.
   - Spot-check at least 20% of the agreed verdicts, plus every verdict that cites
     a specific block or capture, against `coros_map.json` / `user_captures.json`.
     Discard verdicts built on a judge factual error, and record how many were discarded.
3. **Human listen test** (optional, final candidates only): load 3 presets per
   finalist on the QC, blind A/B against baseline.

### 6.3 Cost and latency

Per scenario: p50/p95 pipeline wall time, per-agent latency, input/output tokens,
and $/run. Data comes from `usage/<date>/` records (`cmd/usage_report`).
**Cost and latency break ties. They never override a quality loss** (July precedent).

## 7. Decision rules

| Tier | Adopt the candidate if |
|---|---|
| Pro (1, 12) | Passes every gate, has a Critic defect count ≤ baseline, and wins at least as many agreed judge verdicts as it loses. July's 11-to-5 against 3.6 Flash is the bar. |
| Flash (2–11) | Passes every gate, quality is not worse (tie is acceptable), and it is cheaper, faster, or has a later deprecation date. 3.5 Flash is deprecated, so a tie means adopt. |
| Critic (13) | Its findings are **more accurate**. Check manually: of 30 sampled flags, how many are true contradictions. A higher flag count alone doesn't count. |
| Mechanical (8, 10, 11) → Lite | Passes every gate and ties on quality. |
| Argon anywhere in production | All of the above, **and** Google has published pricing and a non-provisional model ID. Until then, Argon results are a preview signal and its findings don't change routing. |

**Interpreting A1/A2/A3:** if A1 wins and A2/A3 show which tier drives the win,
adopt at the tier level. If A1 wins but neither tier alone does, the gain likely
comes from shared context between agents. In that case run Phase 3A before
deciding anything.

## 8. Phase 3 — Optional follow-ups (only if Phase 2 warrants it)

- **3A. Per-agent Argon isolation.** Swap one agent to Argon, everything else on
  baseline, 13 queries × 2 reps × 13 agents = 338 runs. Only run this if A1 wins
  and A2/A3 are ambiguous. Needs its own cost sign-off.
- **3B. Thinking sweep on the winners** (`cmd/eval_thinking_matrix`, add candidates
  to its grid). Note `ThinkingBudget` is a single orchestrator-wide setting today.
  Per-agent thinking (e.g. high for the Architect, low for the Control Mapper)
  needs new plumbing and a `thinking_level` probe. Treat that as a separate change.
- **3C. Refinement path.** Re-run `cmd/eval_refine` with the winning Architect
  model, since Refinement Architect shares the `12_architect` routing key.

## 9. Rollout

1. Set the winners in `config.json` `agent_models` (overrides the code defaults), and
   update the code defaults in `RunAgentSplit` to match.
2. **Update `getFallbackChain` regardless of the eval outcome.** Remove
   `gemini-2.5-pro` once it's confirmed shut down or scheduled to shut down, and add
   chains for every model that can now be a primary. The default case must not route
   unknown models to a model that's about to disappear.
3. Run unit + e2e tests (CLAUDE.md requirement). Open a PR; a merge to `main`
   auto-deploys.
4. Watch `usage/` analytics for 7 days: error rate, truncations, $/run vs. eval
   figures, and fallback frequency. **Rollback:** revert the `agent_models` change.
5. Record results in TODO.md, following the format of the July 3.6 Flash section,
   including what wasn't adopted and why.

## 10. Open questions / unverified assumptions

- Argon's price, rate limits, preview terms, and whether it supports `GoogleSearch`
  grounding are all unverified. Phase 0.5 answers grounding. The rest needs Google's
  docs, which `ai.google.dev` serves; that host is blocked from the cloud dev
  sessions, so check it from a normal browser.
- 3.8 Flash pricing comes from third-party sites only.
- `gemini-pro-latest`'s target is unknown until 0.3.
- Deprecation and shutdown dates for 3.5 Flash and 2.5 Pro came from search results,
  and Gemini API and Vertex give different dates. Confirm on the official
  deprecations page before setting rollout urgency.
