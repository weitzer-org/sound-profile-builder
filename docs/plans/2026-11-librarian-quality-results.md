# Results: does the cache-friendly Librarian order change quality? (2026-11-05)

Branch: `claude/stoic-hawking-kwbe44` (PR #110). Tools: `cmd/eval_librarian`, `cmd/eval_pair_pipeline`,
`cmd/compare_pair`, `cmd/judge_pairs`; scorer in `internal/evalscore`; checklist in
`internal/evalfixtures/librarian_checklist.json` (owner-approved). Raw outputs were kept in the
session scratchpad and are not committed.

## Bottom line
- **Caching:** new order caches ~55% of input in the full pipeline, old order ~2% (34 paired runs).
- **Quality:** no regression found by the deterministic checklist (Librarian-only and full pipeline).
  The blind LLM judge leans old-order (12 vs 4 consistent wins, 18 of 34 pairs inconsistent, p=0.077),
  but that lean is concentrated in pairs where the new-order preset used `Range Rverb`, a known catalog bug.
  That is an exploratory, post-hoc finding, not a pre-registered one.
- **Not shown:** that the reorder is free of any quality effect. The catalog bug confounds the judge, and
  hard-rule pass rates are near a ceiling (~94-96%), so the checklist only detects large drops.

## Librarian-only A/B (260 calls, 13 queries x 2 frozen Phase 1 samples x 5 repeats x 2 orders)
| | new | old |
|---|---|---|
| all hard rules + single amp | 93.8% | 96.2% |
| same, after fixing 2 scorer false negatives* | 96.2% | 96.2% |
| soft-rule pass fraction | 0.904 | 0.888 |
| self-consistency of picks between repeats | 0.751 | 0.770 |

*Hiwatt Custom captures are typed `fx` in `coros_map.json`; "BD-2"/"TS808" weren't matched by the drive
pattern. Fixed with owner approval; applies equally to both orders. Pre-fix verdict INCONCLUSIVE, post-fix NOT_WORSE.
Est. spend $20.37 (no cache discount applied).

## Full pipeline, paired (13 queries x 3 samples, shared frozen Phase 1, production routing)
36/39 pipelines succeeded in each condition; all failures were the Acoustician hitting MaxOutputTokens.

| metric (diff = new - old, + = new better) | new | old | verdict / note |
|---|---|---|---|
| preset: all hard rules + single amp (34 pairs) | 0.941 | 0.912 | NOT_WORSE |
| librarian: all hard rules + single amp | 0.941 | 0.941 | NOT_WORSE |
| mechanical defects per run (lower better) | 0.235 | 0.294 | INCONCLUSIVE |
| `Range Rverb` uses per preset | 0.206 | 0.029 | new worse; 7 pairs vs 1, p=0.07 |
| cached input share | 54.8% | 1.9% | caching works |
| cost excl. Phase 1 (USD) | 0.267 | 0.267 | no change *by construction* (cached rate = input rate where unverified) |

Latency (134s vs 109s) is NOT comparable: the old condition reused frozen Phase 1, so it skipped those agents.
If `gemini-3.5-flash` cached input were billed at 0.075/M (UNVERIFIED; that is the 3.6-flash figure), the
saving would be ~$0.044 of $0.265 per pipeline (~17%).
Spend: new $11.33, old $10.43.

## Blind judge (every pair judged twice, positions swapped; judge = gemini-3.1-pro-preview)
- new vs old: new 4, old 12, inconsistent 18 (53%); sign test p=0.077. 75% of raw verdicts picked position A (strong position bias).
- Same-condition calibration: 2 vs 5, 4 of 11 inconsistent (36%).
- Exploratory cross-tab on `Range Rverb` (first=new):

| pair type | new wins | old wins | inconsistent |
|---|---|---|---|
| new used Range Rverb, old didn't | 0 | 6 | 1 |
| old used it, new didn't | 1 | 0 | 0 |
| neither used it | 3 | 6 | 17 |

Where neither preset used it, the judge shows no clear preference (3 vs 6, 17 inconsistent).
Judge spend: $5.83.

## Open questions
1. Why do new-order presets use `Range Rverb` more in the full pipeline (0.206 vs 0.029) when the Librarian alone does not (0.18 vs 0.17)? Could be chance (p=0.07).
2. Fix the catalog (`Range Rverb` mapped to both an amp and spring reverbs) and re-measure; this removes the confound.
3. Verify the cached-input rate for `gemini-3.5-flash` before quoting any dollar saving.

---

# Re-evaluation after merging main (2026-11-05, second run)

What changed since the first run: main merged in (production routing is now gemini-4-argon for
almost every agent, gemini-3.8-flash for Sonic Profiler; new prompt versions; catalog renames);
`Generic Spring Reverb` re-mapped from `Range Rverb` (the Orange amp capture's name) to the native
`Spring` block; runs used `EVAL_NO_FALLBACK=true`. Same design: 13 queries x 3 samples, shared
frozen Phase 1, same approved checklist.

## Full pipeline, paired (39 pairs; 39/39 succeeded in both conditions)
| metric | new | old | verdict / note |
|---|---|---|---|
| preset: all hard rules + single amp | 0.923 | 0.923 | INCONCLUSIVE (identical means; 39 pairs give a wide CI) |
| preset: hard-rule pass fraction | 0.966 | 0.979 | NOT_WORSE |
| librarian: all hard rules + single amp | 0.923 | 0.897 | INCONCLUSIVE (new slightly ahead) |
| mechanical defects per run | 0.54 | 0.26 | INCONCLUSIVE: 34 vs 35 runs had zero; one new run with 8 drives the mean; sign test 4 vs 5 |
| `Range Rverb` uses per preset | 0 | 0 | catalog fix removed it (was 0.21 vs 0.03) |
| cached input share | 38% | 0.2% | |

Cost re-priced from the recorded token counts at the Google Cloud cached-input rates (Argon $0.10/M
cached vs $2.00 input during the promo; see usage_recorder.go for the source and its two caveats):
new order $0.169 vs $0.223 per pipeline excluding Phase 1 (24% saved); Librarian $0.013 vs $0.067
per call (81% saved). Old order: no saving.

## Blind judge (swapped positions)
| | new wins | old wins | equal | inconsistent |
|---|---|---|---|---|
| new vs old (39 pairs) | 10 | 9 | 1 | 19 (49%) |
| calibration, same condition (13 pairs) | 4 | 4 | 0 | 5 (38%) |

The old-order lean from the first run (12 vs 4) is gone once `Range Rverb` is gone, consistent with
that lean having been the catalog bug. Raw position-A share fell from 75% to 60%.

## Librarian-only A/B (260 calls)
Raw: new 85.3% vs old 92.2% of runs pass every hard rule (INCONCLUSIVE). The entire gap is one
unit, 04_Southern_Rock Phase 1 sample 0/1: the new order picked `US DLX 58` (with the Deluxe
"Tweed" cab) in 9 of 10 runs; the old order picked the `Fender 57 Deluxe` user capture. The
checklist's AMERICAN_TWEED pattern matches "Fender 57" but not "DLX 58". coros_map.json maps
`US DLX 58` from BOTH "Fender Deluxe 5E2 1958" (tweed) and "Fender Deluxe Reverb 64/65"
(blackface) -- an unresolved catalog contradiction (captures are not in the device catalog). If
`US DLX 58` is the 1958 tweed capture, both orders score 92.2%. Other failures are identical in
both orders (01 SRV picks a non-Fender amp in one Phase 1 sample; 10 Slash picks `Brit 2555 Lead`,
a Silver Jubilee capture typed `fx`, so it also fails the single-amp check).

## Spend (estimates)
Pipelines $18.04, judge $14.20, Librarian-only $17.11, shakedown $0.49: about $50.

## Follow-up: what the Librarian picks, and user-capture use (2026-11-05, no new API spend)

Pick-distribution test (`scripts/eval/librarian_pick_distribution.py`, rule fixed before computing):
how differently do the two orders pick block names, versus how differently repeats of the SAME
order pick (equal-size halves)? Argon (2nd run): within 0.131, between 0.229, difference +0.098,
95% CI [+0.043, +0.158] -> ORDER EFFECT. 3.5-flash (1st run): +0.005 [-0.025, +0.036] -> none.

The changed picks are almost all user captures being replaced by factory/native equivalents:

| data (Argon) | runs with >=1 user capture: old | new | per-unit diff, 95% CI | units old-more / new-more |
|---|---|---|---|---|
| Librarian-only | 57% | 23% | -0.33 [-0.48, -0.19] | 13 / 0 |
| Librarian output in full pipelines | 49% | 13% | -0.36 [-0.51, -0.21] | 14 / 0 |

On 3.5-flash both orders rarely used user captures (11% vs 12%). The Librarian prompt's rule 13
says to prefer user captures when they fit, so by its own policy the new order regresses. The
checklist (amp class only) and the judge (doesn't know what the user owns) could not see this.
Hypothesis (untested): in the old order the User Capture Library sat last before Constraints;
in the new order the per-run Tone/Scraper text follows it.
