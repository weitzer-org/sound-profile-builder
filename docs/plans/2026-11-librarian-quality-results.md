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
