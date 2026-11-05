# Librarian prompt caching: findings, data, and how to get a clear quality signal

Status (2026-11-05): **on hold.** PR #110's prompt-layout change is not ready to merge, because
we cannot yet show that preset quality is equal or better. This document is the single place
that records what was learned. Raw data: `eval_results/librarian_order/` (see its README).
The chronological lab notebook is `docs/plans/2026-11-librarian-quality-results.md`.

## 1. The change and why it matters

The CorOS Librarian (agent 4) gets ~30K tokens of static catalog on every run (Dictionary, Amp
Menu, User Capture Library). Gemini's implicit cache only hits an identical prompt prefix, so
PR #110 moves that static text before the per-run text (Tone History, Scraper, Constraints).

| | caches? | cost per pipeline after Phase 1 (gemini-4-argon, re-priced at cached rates) |
|---|---|---|
| old order (per-run text first) | ~0% of input | $0.225 |
| new order (catalog first) | 38% of pipeline input; ~92% of the Librarian's | $0.169 (-24%); Librarian call $0.013 vs $0.067 (-81%) |

Cached-input rates are from Google Cloud's pricing page, read 2026-11-05 (Argon $0.10/M cached
vs $2.00/M input through 2026-11-30, then $0.20 vs $4.00). **Unverified:** whether the Gemini
Developer API (`ai.google.dev`, which this app is billed under and which this environment's proxy
blocks) charges the same, and whether the rate applies to implicit caching.

## 2. What was measured

All runs used the 13 golden queries and the owner-approved checklist
(`internal/evalfixtures/librarian_checklist.json`): hard rules = right amp class, query-named
pedals, exactly one amp; soft rules = typical extras. Decision rules were fixed before each run.
Paired designs freeze Phase 1 (Tone Historian, Sonic Profiler, Community Scraper) so both
conditions see byte-identical inputs.

| round | model routing | design | spend (est.) |
|---|---|---|---|
| 0 (exploratory) | 3.5-flash era | 12 vs 13 full pipelines, 1 sample, judge run 3 times | ~$15-20 (not measured) |
| 1 | 3.5-flash era | Librarian-only 260 calls; 39+39 paired pipelines; swapped-position judge | ~$48 |
| 2 | gemini-4-argon (after merging main) | same as round 1 | ~$50 |
| fixes | gemini-4-argon | 2 layouts x 130 Librarian calls vs round-2 old-order runs | $3.31 |

## 3. Findings

### 3.1 Caching works; Argon caches the prefix
Round 2 shakedown: the second Librarian call had 24,576 of 31,609 input tokens cached.
Full pipelines: 38.4% vs 0.2% cached input share (39 pairs).

### 3.2 The checklist sees no quality difference, but it is near its ceiling
| | new | old |
|---|---|---|
| R1 Librarian-only, all hard rules (after 2 scorer fixes) | 96.2% | 96.2% |
| R1 pipeline: final preset, all hard rules | 94.1% | 91.2% |
| R2 pipeline: final preset, all hard rules | 92.3% | 92.3% |
| R2 pipeline: Librarian output, all hard rules | 92.3% | 89.7% |
| R2 Librarian-only, all hard rules | 85.3% | 92.2% (gap = one query, see 3.6) |

With pass rates of 92-96% the checklist can only detect large drops; at 39 pairs its 95% interval
on "all hard rules" is about +/-8 points.

### 3.3 The blind LLM judge is too noisy to decide this
Every pair judged twice with positions swapped; a winner counts only if both orderings agree.

| | new wins | old wins | equal | inconsistent | position-A share |
|---|---|---|---|---|---|
| round 0 (one ordering only, 3 reruns pooled) | 11 | 22 | 2 | n/a | n/a |
| round 1 | 4 | 12 | 0 | 18 of 34 (53%) | 75% |
| round 1 calibration (same condition vs itself) | 2 | 5 | 0 | 4 of 11 | |
| round 2 | 10 | 9 | 1 | 19 of 39 (49%) | 60% |
| round 2 calibration | 4 | 4 | 0 | 5 of 13 | |

Round 1's old-order lean was explained by a catalog bug (3.5): when the new-order preset used
`Range Rverb` and the old one didn't, old won 6 of 6 decisive pairs; elsewhere the judge was a
wash. With the bug fixed (round 2) the lean disappeared.

### 3.4 Real behavior change: the new order uses the user's own captures less (on Argon)
Pick-distribution test (`scripts/eval/librarian_pick_distribution.py`): the two orders' picks
differ more than repeats of one order do. Round 2: +0.098, 95% CI [+0.043, +0.158] (order effect);
round 1 (3.5-flash): +0.005 [-0.025, +0.036] (none). The change is almost entirely user captures
replaced by factory/native equivalents (e.g. `Fender 57 Deluxe` -> `US DLX 58`, `ODS Joe Lead` ->
`Dumbbell ODS`, `1964 Fender Super Reverb` -> `US SPR Vibrato`).

| runs choosing >=1 user capture | new | old | paired diff, 95% CI | units old-more / new-more |
|---|---|---|---|---|
| R2 Librarian-only | 23% | 57% | -0.33 [-0.48, -0.19] | 13 / 0 |
| R2 pipeline Librarian output | 13% | 49% | -0.36 [-0.51, -0.21] | 14 / 0 |
| R1 Librarian-only (3.5-flash) | 11% | 12% | -0.02 [-0.05, +0.02] | 3 / 1 |

The Librarian prompt's "Prefer User Captures" rule asks for them, so this is a regression by the
prompt's own policy. The checklist (amp class only) and the judge (doesn't know what the user
owns) could not see it.

**Fix tested** (rule: user-capture use within 10 points of old AND checklist NOT_WORSE):

| layout | user captures (old 57%) | verdict | all hard rules (old 92.2%) | verdict |
|---|---|---|---|---|
| catalog first, no reminder | 23% | WORSE | 85.4% | INCONCLUSIVE |
| **catalog first + one fixed reminder sentence** | **73%** (+0.17 [+0.06, +0.30]) | NOT_WORSE | **92.3%** | NOT_WORSE |
| User Capture Library after the per-run text | 45% (-0.12 [-0.27, +0.03]) | INCONCLUSIVE | 85.4% | INCONCLUSIVE |

The branch now uses the reminder layout. Its extra captures look like good fits (`ODS Joe Lead` +
`1981 TS808 1` + `Klon Centaur 1` for Bonamassa; `66 AC30 JMP OD` for British Invasion) but nobody
has listened to them, and it has not been run in full pipelines.

### 3.5 Bugs found along the way
- **Catalog:** `Generic Spring Reverb` mapped to `Range Rverb` (the Orange Rockerverb amp capture's
  name) and was flagged `is_capture`, which also hid it from `TestCorosMapNamesExistInCatalog`.
  Fixed on the branch (native `Spring`), with guard tests that fail on the old file.
- **Catalog, open:** `US DLX 58` is mapped from both "Fender Deluxe 5E2 1958" (tweed) and "Fender
  Deluxe Reverb 64/65" (blackface). Factory captures aren't in the device catalog, so it can't be
  settled from the repo. `Brit 2555*` and `Watt Custom*` captures are typed `fx`, not `amp`.
- **Scorer gaps found by the data:** Hiwatt Custom captures typed `fx`; "BD-2"/"TS808" not in the
  drive patterns (both fixed with owner approval); "DLX 58" not in the tweed pattern and
  `Brit 2555*` not counted as amps (fix awaiting owner approval).
- **Unrelated, on main:** 9 Playwright specs fail on main itself (hidden `#favor-captures`, 1-day
  session cookie where the specs expect 30 days). One `internal/mcpserver` journey test failed
  once under full-suite load and passed 3/3 alone.
- **Branch history:** commit `d2e1d5e` does not compile (a piped test hid the failure); the next
  commit fixes it.

## 4. Why we still don't have a clear quality signal

1. **Ceiling:** the checklist's hard rules pass 92-96% in every condition.
2. **Judge noise:** about half of judge verdicts flip when positions swap, including on
   same-condition calibration pairs, so the judge cannot separate these presets.
3. **Distance:** the change is in one upstream agent; eight or more later agents add their own
   randomness before the final preset.
4. **No ground truth:** nothing measures what the owner would hear or prefer.

## 5. Plan: a clear quality signal with minimal manual work

Goal: an automated, repeatable quality score that is (a) sensitive above the current ceiling,
(b) stable on same-condition calibration, and (c) checked once against the owner's judgment.
Owner time target: ~20 minutes of review, not hours of authoring.

**Step 1. Model-drafted, citation-backed reference rigs (owner reviews, doesn't write).**
A tool in the style of `cmd/enrich_captures`: a grounded model drafts, for each golden query, a
graded reference (ideal amp/cab/pedals/effects with acceptable alternatives and partial credit,
plus known mistakes), every entry restricted to real catalog/user-capture names and backed by a
cited source. The owner approves or edits the draft file (~15 min). Output: a 0-10 graded score per
preset instead of pass/fail, which moves the measurement off the ceiling.

**Step 2. Expand the deterministic, free metrics.** All judge-free and computed from saved outputs:
- the device-catalog checks already in `RunMechanicalQualityChecks` (unknown parameters, out-of-range
  values, `UnconsideredParams`) via `cmd/eval_device_checks`;
- Librarian-rule compliance: user-capture preference, single amp, no time-based capture, spring
  reverb for vintage American briefs, no overdrive on pure-headroom briefs (the prompt's own rules);
- the pick-distribution test, to say whether a change alters behavior at all.

**Step 3. A judge that is gated by its own reliability.** Keep the LLM judge only as one input:
reference-guided (it sees the step-1 rig), scoring per criterion on 1-5 scales over a compact block
list rather than a preference over full HTML; two judge models with `cmd/eval_judge_agree`; and a
gate: its verdicts count only if same-condition calibration inconsistency is under ~20%.

**Step 4. One small human calibration, reused.** main's `internal/evaljudge` spot-check flow:
the owner rules on ~10 sampled items once to confirm the graded score and judge track their taste.
After that, the automated score is trusted for future changes (prompt versions, model swaps).

**Step 5. Re-run this comparison.** Paired, frozen Phase 1, 13 queries x 5 samples per condition
(old, catalog-first + reminder), scored by steps 1-3. Estimated ~$40-60 on Argon (unmeasured).
Decision rule to fix in advance: ship only if the graded score is non-inferior and user-capture
use is non-inferior.

Cost of steps 1-4: the reference-rig drafting is a few dollars of grounded calls; steps 2 and 4
are free of API cost; step 3 roughly doubles judge spend (two judges).

## 6. Reproduce

```bash
# re-score saved round-2 pipelines (no API calls)
go run ./cmd/compare_pair -new eval_results/librarian_order/round2_argon/pipeline/new -old eval_results/librarian_order/round2_argon/pipeline/old
# pick-distribution test (no API calls)
python3 -I scripts/eval/librarian_pick_distribution.py eval_results/librarian_order/round2_argon/librarian_ab
```
Live runs: `cmd/eval_librarian`, `cmd/eval_pair_pipeline` (run `new` first; it freezes Phase 1),
`cmd/judge_pairs`. All have `-dry-run` and `-max-usd`; set `EVAL_NO_FALLBACK=true`.
