# Gemini Model Upgrade — Test Plan

Status: **Phase 0 code done; 0.3/0.4 resolved; 0.5 capability probe awaiting sign-off** · Written 2026-11-04 · Owner: bweitzer

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
| 0.4 | **Check the baseline is really what we think.** Confirm `gemini-3.5-flash` responses report a 3.5 `ModelVersion`. A search result claimed it auto-routes to 3.6; `models.list` shows a distinct `3.5-flash-05-2026` version, but no call has confirmed it yet. | one-off | Baseline's served model confirmed |
| 0.5 | **Capability probe** (extend `cmd/probe_thinking`) for each candidate. Probe: (a) `responseSchema` structured output; (b) the `GoogleSearch` tool, needed by agents 1, 2, 3, 14, 15 (`agentSearchTool`); (c) schema + tools combined, which `gemini-2.5-pro` rejects (see orchestrator ~line 1020); (d) `ThinkingBudget` default / 512 / 2048 / 0; (e) `MaxOutputTokens` honored. | `cmd/probe_thinking` | A pass/fail matrix. A candidate that fails (a) is out. One that fails (b) is excluded from agents 1/2/3 |
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

#### Proposed 0.5 grid (awaiting sign-off)

5 models (`gemini-pro-latest` is not probed separately; it is `gemini-3.1-pro-preview`) × {schema+tools combined, thinking 512 / 2048 / 0, MaxOutputTokens=64} = **25 calls**, plus up to 10 conditional follow-ups (schema-only / tools-only, only when the combined call fails or does not show search ran), plus 2 `resolve` calls (a repeat on `gemini-3.5-flash` and a control on `gemini-3.6-flash`). 2 calls are already spent (0.3, 0.4). Hard cap via `-max-calls`.

#### Corrections to this plan found in Phase 0

1. **§3 0.8(i) / §6.1 "0 non-numeric `Value`/`ValueB`" is unsatisfiable.** `value` is a free string in the Architect schema and the prompt prescribes unit-bearing values; July's *baseline* outputs contain ~390 of them. (TODO.md already recorded that a blanket units check was removed for this reason.) The checker reports it as a reference number only; the gated defect is Rule 9 formatting vs. capture status. R2 should be redefined that way.
2. **§3 0.8(iii) "names changed by `ApplyFuzzyCorrection`" always counts 0.** The output envelope is JSON-marshalled with `<` escaped as `\u003c`, so `ApplyFuzzyCorrection`'s `<td` regex never matches (demonstrated: a typo injected into a July output survived it). The checker counts snapped/unverified gear names over `structured_payload` instead. Whether `ApplyFuzzyCorrection` being a no-op in `RunPipeline` is a production bug is outside this PR.
3. **R1 (§4) as written is probably a false premise.** See the Iba Green row above; the reproducible July defect is the `UK C30 TopBoost`/Rule 9 pattern. Redefine R1 as "capture claimed or dB-formatted on a block the map says is not a capture", on `07_Edge` and `05_Clapton`.
4. **The baseline may not be 3.5 Flash** (0.4). If confirmed, S0's Flash tier is 3.6 Flash, July's "3.5 vs 3.6" comparison was partly 3.6 against itself, and §6.1's served-model gate (served == requested) would fail S0 itself. Needs a decision before Phase 2: pin the baseline to the served id, or request `gemini-3.6-flash` explicitly.
5. The Critic is not ground truth: it disputed a real capture (Iba Green) twice in July.

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
| R2 Units inside numeric fields | `09_BB_King` | 0/5 runs with a non-numeric `Value`/`ValueB` |

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
9 runs in total. Multiply the measured cost per run by 234, using the post-introductory
3.8 Flash price if one is known. Then get explicit sign-off before the full run.
Don't estimate from list prices alone; Argon has no published price yet.

**Argon note:** Argon's 32,768 output limit is above every current
`agentMaxOutputTokens` cap (Architect = 16,000), so the caps bind first. Don't
raise any cap during this eval (principle 5).

## 6. Scoring

### 6.1 Hard gates. A scenario fails if any of these fails.

| Gate | Threshold |
|---|---|
| Served-model integrity | 100% of calls served by the requested model (any fallback = failed call) |
| Pipeline success rate | ≥ S0's rate in the same batch |
| Schema validity | 100% of Architect outputs parse; 0 non-numeric `Value`/`ValueB` |
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
