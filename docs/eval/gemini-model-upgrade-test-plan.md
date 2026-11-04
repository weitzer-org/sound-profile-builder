# Gemini Model Upgrade — Test Plan

Status: **Phase 0 complete; Phase 2 readiness tooling built and unit-tested, no live Phase 2 run yet (see "Phase 2 readiness" in §5)** · Written 2026-11-04 · Owner: bweitzer

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
| 1 Tone Historian | `gemini-3.1-pro-preview` | `gemini-4-argon`, `gemini-3.8-flash`, `gemini-pro-latest`* |
| 2 Sonic Profiler | `gemini-3.5-flash` | `gemini-4-argon`, `gemini-3.8-flash`, `gemini-3.5-flash-lite` |
| 3 Community Scraper | `gemini-3.5-flash` | `gemini-4-argon`, `gemini-3.8-flash`, `gemini-3.5-flash-lite` |

\* only if 0.3 shows it resolves to something newer than 3.1-pro-preview.

**1B. Known-regression repros** on the Architect (agent 12). Upstream agents 1–11
stay on baseline; only the Architect model changes. 5 reps each.

| Repro | Query | Pass condition |
|---|---|---|
| R1 Iba Green described as a Neural Capture | `07_Edge` | 0/5 runs flagged (3.6 Flash: reproduced 4×) |
| R2 Rule 9 dB formatting on a non-capture block (the checker's `capture_db_on_modeled`; replaces "units inside numeric fields", see Phase 0 correction 1) | `09_BB_King`, `05_Clapton` | 0/5 runs with `capture_db_on_modeled > 0` |

Run for `gemini-4-argon`, `gemini-3.8-flash`, `gemini-pro-latest`*, and the baseline
as a control. The baseline must pass both, or the check itself is broken.

**Phase 1 exit:** any candidate that fails R1 or R2, or a Phase 0.5 must-have
capability, is dropped from the Pro tier. Any candidate whose 1A schema-validity
or truncation rate is worse than baseline is dropped from that agent.

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
9 runs planned, **8 executable** (S3 is skipped), `eval_full_pipeline -smoke`. Then
`eval_cost` prices the measured per-run cost from recorded tokens × `geminiPriceTable` and
multiplies by the full matrix: **208 runs = 8 executable scenarios × 13 queries × 2 reps (26
per scenario)**, not 234. Models with no official price are UNKNOWN, never $0, so until Google
publishes prices the projected total is a *lower bound* and the tool prints exactly what it
leaves out. Get explicit sign-off before the full run. Don't estimate from list prices alone;
Argon has no published price yet.

**Argon note:** Argon's 32,768 output limit is above every current
`agentMaxOutputTokens` cap (Architect = 16,000), so the caps bind first. Don't
raise any cap during this eval (principle 5).

### Phase 2 readiness (built 2026-11-04; zero live Gemini calls were made to build or validate it)

| # | Prerequisite | Status | Where |
|---|---|---|---|
| 2.1 | Smoke pass + cost extrapolation | **Exists** | `eval_full_pipeline -smoke`; `cmd/eval_cost`; `internal/evalcost` |
| 2.2 | Uniform re-critic | **Exists** | `cmd/eval_recritic`; `internal/evalcritic` |
| 2.3 | Two-judge agreement + spot-check sampler | **Exists** (`judge_compare` gained audit fields) | `cmd/eval_judge_agree`; `internal/evaljudge` |
| 2.4 | Report: gates, §7 rules, refuse cross-batch | **Exists** | `cmd/eval_report`; `internal/evalreport` |
| – | Batch identity | **New**: `eval_full_pipeline` now writes `manifest.json` (batch id, frozen per-scenario routing and pins, queries, reps) and stamps `batch_id` on every `runs.jsonl` line | `internal/evalcheck/batch.go` |

None of the tools make a Gemini call except `eval_recritic -dry-run=false` and `judge_compare`
(both are opt-in; `eval_recritic` defaults to a dry run that prints the call count).

**Run order.** `B` is the results directory. Every command below is a dry run or offline unless
it says LIVE.

```bash
# 0. Plan only (no calls): 8 executable runs
go run ./cmd/eval_full_pipeline -smoke
# 1. LIVE smoke pass (~104 agent calls) -> eval_results/full_pipeline_gemini_upgrade/smoke-<ts>
EVAL_NO_FALLBACK=true go run ./cmd/eval_full_pipeline -smoke -dry-run=false
# 2. Cost gate (offline). -assume is a labelled what-if, never merged into the figures.
go run ./cmd/eval_cost -root <smoke dir> -assume gemini-3.8-flash=0.75/3.75
#    --> get sign-off here
# 3. LIVE full matrix (208 runs, ~2,700 agent calls), baseline in the SAME batch
EVAL_NO_FALLBACK=true go run ./cmd/eval_full_pipeline -dry-run=false -out B
# 4. Uniform re-critic: dry run prints the call count (one per scoreable output, <= 208)
go run ./cmd/eval_recritic -root B
EVAL_NO_FALLBACK=true go run ./cmd/eval_recritic -root B -dry-run=false     # LIVE, resumable
# 5. Pairwise judging: per candidate C in {S1,S2,S4,S5,A1,A2,A3} and rep R in {1,2}, twice (<= 364 LIVE calls)
JUDGE_MODEL=gemini-3.1-pro-preview DIR_A=B/S0/rep$R DIR_B=B/$C/rep$R LABEL_A=S0 LABEL_B=$C OUT_FILE=B/judge/${C}_rep${R}_a.json go run ./cmd/judge_compare
JUDGE_MODEL=<second judge>         DIR_A=B/S0/rep$R DIR_B=B/$C/rep$R LABEL_A=S0 LABEL_B=$C OUT_FILE=B/judge/${C}_rep${R}_b.json go run ./cmd/judge_compare
# 6. Agreement + spot-check sample (offline); then a human fills "discard" in B/judge/spotcheck.json
go run ./cmd/eval_judge_agree -batch B -out B -pair S1=B/judge/S1_rep1_a.json,B/judge/S1_rep1_b.json -pair S1=... (repeat)
go run ./cmd/eval_judge_agree -batch B -out B <same -pair flags> -review B/spotcheck.json
# 7. The report (offline). Reads manifest, runs.jsonl, calls files, outputs, recritic.jsonl, judge_summary.json
go run ./cmd/eval_report -root B -baseline-flash-deprecated unknown -critic-sample 30
```

**Call budget for Phase 2:** 208 pipeline runs (~2,700 agent calls; the 234/~3,000 in the
original estimate included S3) + ≤ 208 re-critic calls + ≤ 364 judge calls
(7 candidates × 26 pairs × 2 judges) + 8 smoke runs first.

**How each tool was validated (all offline):**

- `eval_full_pipeline -smoke`: `-dry-run` prints `9 planned, 8 executable`; `-smoke` with
  `-queries`/`-reps` is refused; manifest contents are unit-tested.
- `eval_cost`: unit tests on synthetic smoke batches with hand-computed expected dollars
  (fully priced S0, partially priced S1/S4, an all-unpriced scenario, a failed run, a run whose
  `runs.jsonl` says clean but whose calls file shows a fallback). There is no live smoke output
  to run it on yet, and the July report only has aggregate tokens, not per-model tokens.
  Caveat on the extrapolation: in the July runs, 07_Edge's input tokens were within 4% of
  the all-query mean in all three scenarios but its output tokens were 8-13% below the mean
  (per-query output range 4.6k to 7.0k baseline, 3.2k to 10.4k flash-tier; computed from the
  July report's per-query token totals), so a
  single-query smoke figure probably under-reads the output share of the cost.
- `eval_recritic`: unit tests with a fake caller (input has the pipeline Critic's notes and the
  rendered table stripped; `{}`/garbage/missing `issues` is a failure, never 0; a wrong served
  id, fallback attempt or API error is recorded as not scored; resume skips only successes).
  Dry run on the July outputs: `eval_recritic -root eval_results/full_pipeline -legacy` = 35 calls.
- `eval_judge_agree`: unit tests on synthetic judge files, plus the two July files
  (`judge_flash_vs_baseline_v2.json`, `judge_pro_vs_baseline_v2.json`) used as real judge text.
  The sampler puts `ODS Joe Lead`'s `user_captures.json` record (a Dumble ODS, the thing the judge
  called "fabricated") next to the flash 12_Bonamassa verdict, and the `Circular Delay` `coros_map.json`
  records next to the pro 12_Bonamassa verdict ("Axe-Fx exclusive"): the two July judge errors.
  The July files have one judge only, so the second judge in those tests is synthesized.
- `eval_report`: unit tests on synthetic 9-scenario batches (including an `integrity_ok=false`
  run, a fallback-served call that `runs.jsonl` calls clean, a truncation, R1/R2, invented names,
  a missing run, mixed and foreign batch ids). On July:
  `eval_report -legacy -baseline-id baseline` reproduces `eval_checks` exactly (baseline
  R1/R2 0/0, 4 unverified names, 3 own-Critic flags; flash-tier 0/0, 2 unverified, 0 flags;
  pro-tier R1 2, R2 16, 0 unverified, 8 flags), and marks served-model integrity, success
  rate, MAX_TOKENS, latency, tokens, cost, the batch and every §7 rule UNMEASURED because that
  layout has none of them. Deleting the integrity re-derivation, or the success-rate gate's
  comparison, makes the corresponding unit tests fail.

**Not validated (cannot be without live calls):** the live `eval_recritic`/`judge_compare` paths
against the real API, the orchestrator-backed `Caller`, and real `calls.json`/`manifest.json`
files from a live run. The first smoke pass is their first real test; `eval_report` on the smoke
batch is the check that the files are shaped as the tools expect.

### Corrections and ambiguities found while building Phase 2

Where the plan could not be implemented literally, this is what the tools do. Each item marked
**DECISION** wants an explicit yes/no from the owner.

1. **234 → 208.** §5's cost-gate "multiply by 234" includes S3, which is skipped. The matrix is 208.
2. **Which model id prices a call? DECISION.** `gemini-3.5-flash` is *served* as `gemini-3.6-flash`
   (Phase 0.4) and the table prices them differently ($9.00 vs $7.50 per M output). Which one Google
   bills is unverified. `eval_cost` and `eval_report` price by the *requested* id by default
   (matches the `usage/` records) and accept `-price-basis served`.
3. **§6.1 pipeline success rate.** Numerator = scoreable runs (success *and* integrity-clean),
   denominator = planned runs, so an excluded run counts as a failure (principle 3).
4. **§6.1 truncation "≤ S0, per agent".** Compared as rate per recorded run, counting only the
   *routed* model's own attempts (a fallback's truncation is not the candidate's), over all
   recorded runs including failed ones (truncation is why they failed).
5. **§6.1 "R1 = 0".** Implemented as an absolute 0 on every scenario *including S0*. Under the
   redefinition (Phase 0 correction 3) R1 = `capture_claim_on_modeled`, R2 = `capture_db_on_modeled`.
   If S0 itself shows R1 > 0, S0 fails its own gate and the tool says so but does not block the batch.
6. **§6.1 "fuzzy-corrected names ≤ S0". DECISION.** `ApplyFuzzyCorrection` is a no-op (Phase 0
   correction 2), so the gate uses *snapped + unverified* gear names per scoreable output. The
   literal "snapped only" would let a candidate invent names too far from any real block to snap.
7. **§6.2.1 "Critic defect count".** = number of re-critic findings that point at a real
   guitar/block id of the output (findings pointing at nothing are counted separately as
   `unmatched`, as the pipeline silently drops them), mean per run; spread = sample sd and the
   mean within-query range across reps. It is how much one fixed reviewer flags, not how many
   true errors exist (the Critic is not ground truth). Fidelity limit, unverified: the re-critic
   input is rebuilt from the saved envelope (rendered table and Critic notes removed), which should
   equal what the pipeline's Critic saw but was not byte-compared. `cmd/critic_probe` was left
   alone: it hard-codes `gemini-3.1-pro-preview` (S0's Critic is 3.5-flash, served 3.6) and does no
   served-model check.
8. **§6.2.2 spot-check size. DECISION.** The sampler includes *every* agreed verdict that names an
   indexed block/capture (exact or fuzzy) or uses claim wording ("fabricated", "Axe-Fx",
   "does not exist", …), **plus** a seeded random ≥ 20% of the *remaining* agreed verdicts. In
   practice nearly every verdict names gear: on the two July files, 17 of 19 agreed verdicts
   qualify, so "≥ 20% plus every citing verdict" is ≈ "review everything". Narrowing options
   (claim wording only; or mentions only when the judge asserts something negative) trade
   review effort for missed judge errors.
9. **§6.2.2 "both judges agree".** An agreed `Equal` is a tie (neither a win nor a loss).
   Unpaired, unusable (winner outside the label set, or A/B mapping inconsistent with the raw
   preference), and runs excluded for integrity are reported, never counted. Judging the same
   model twice, or two requests served by the same model, is refused.
10. **§6.2.2 judge "non-contestant". DECISION.** The plan's judge #1, `gemini-3.1-pro-preview`, is
    itself S0's Pro tier, so it is a contestant in every comparison; only judge #2 is
    non-contestant. `eval_judge_agree` prints a self-preference note when a judge is routed into S0
    or the candidate; it does not refuse.
11. **Judges also judge unscoreable runs.** `judge_compare` reads each golden query's `.html` from both
    directories whatever the run's integrity, so the money is spent on integrity-failed outputs too; `eval_judge_agree -batch` only drops them
    afterward. Pre-filtering would need `judge_compare` to read `runs.jsonl`.
12. **§7 "Critic defect count ≤ baseline" and "wins ≥ losses".** Plain comparison of means and
    counts, no significance test. With 26 runs per scenario small differences are noise; the report
    prints the spread so a human can see that.
13. **§7 Flash "quality not worse (tie acceptable)" and Mechanical "ties on quality".** Both are
    implemented as *not worse* (defects ≤ S0 and agreed wins ≥ losses). A literal "tie" for the
    Mechanical tier would reject an improvement.
14. **§7 Flash "cheaper, faster, or later deprecation".** Cheaper needs both costs fully priced
    (an unpriced model makes it UNMEASURED); faster = lower p50 pipeline latency; deprecation is an
    external, unverified fact (§10) supplied as `-baseline-flash-deprecated yes|no|unknown`
    (default `unknown`, which leaves the clause UNMEASURED). Phase 0 correction 4 notes
    "3.5 Flash is deprecated" may be moot if it aliases 3.6.
15. **§7 Mechanical (S4) has no cost clause**, although §5 asks "cheaper mechanical agents?".
    Implemented literally: an S4 that ties on quality is ADOPT even if Lite saves nothing, and
    Lite has no official price so savings cannot be measured today. **DECISION:** add a cost clause?
16. **§7 Critic rule** ("of 30 sampled flags, how many are true contradictions") is manual:
    `eval_report -critic-sample 30` prints a seeded sample of the pipeline Critic's flags (S0 and
    Critic-tier scenarios) with the `coros_map.json`/`user_captures.json` ground truth next to each.
17. **§7 "Argon anywhere in production".** The tool cannot know whether Google has published
    pricing or a non-provisional id; it treats the ids in `-provisional-models` (default
    `gemini-4-argon`) as provisional and caps any would-be ADOPT to UNDETERMINED ("preview signal
    only"). Argon is also absent from `geminiPriceTable`, so its cost is UNKNOWN.
18. **§7 A1/A2/A3 "wins". DECISION.** Interpreted as: all gates pass, defects ≤ S0, and agreed
    judge wins strictly exceed losses. Attribution then follows the plan's text.
19. **p50/p95 pipeline latency** are nearest-rank over *scoreable* runs. With 26 runs p95 is the
    second-slowest run, so it is noisy.
20. **Refusing to compare across batches** means: mixed `batch_id`s in `runs.jsonl`, a `recritic.jsonl`
    line or `judge_summary.json` stamped with a different batch, or a second results directory
    (`-baseline-root`) from another invocation. A directory without `manifest.json` (the July
    layout) is only accepted with `-legacy`, and then everything that needs batch identity,
    calls files, latency or tokens is UNMEASURED and no decision is produced.
21. **Preconditions block verdicts.** If the baseline is missing/empty, the matrix is incomplete
    (a planned run has no record), or S0 itself has served-model violations, the gates still print
    but every ADOPT/DO NOT ADOPT is shown as UNDETERMINED.
22. **Pre-existing, unrelated:** `TestOrchestrator_OpenLLM_Integration` fails when `GEMINI_API_KEY`
    is set in the environment (`NewOrchestrator` creates the Gemini client whenever the env var is set, even
    for an empty key argument, so the test's "client must be nil" assertion breaks); it passes with `env -u GEMINI_API_KEY go test ./...`. Not changed here.

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
   re-run one fixed Critic over all outputs (`cmd/eval_recritic`, pinned to the S0
   Critic model with its served-id check; `cmd/critic_probe` is not suitable, see Phase 2 readiness item 7). This keeps S5 and A1, whose own Critic is Argon, comparable with
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
