# Librarian prompt-order evals: raw data

Supporting data for `docs/plans/2026-11-librarian-order-findings.md`. Every number in that
document can be recomputed from these files with the tools named below (no API calls needed
for re-scoring: `cmd/compare_pair`, `scripts/eval/librarian_pick_distribution.py`, and
`cmd/eval_librarian` with every run already present).

"new" = static catalog first (cache-friendly). "old" = per-run text first (production before
PR #110). The Librarian prompt version is v2 throughout except where noted.

| directory | what | model routing | tool |
|---|---|---|---|
| `round0_initial/` | blind-judge verdicts from the first exploratory full-pipeline eval (one sample per query; the presets themselves are not kept) | gemini-3.5-flash era | `cmd/judge_compare` |
| `round1_flash35/librarian_ab/` | Librarian-only A/B: 13 queries x 2 frozen Phase 1 samples x 5 repeats x 2 orders | gemini-3.5-flash | `cmd/eval_librarian` |
| `round1_flash35/pipeline/` | paired full pipelines, 13 queries x 3 samples per order, shared frozen Phase 1 (`phase1/`), judge verdicts (`judge/`) | gemini-3.5-flash era | `cmd/eval_pair_pipeline`, `cmd/judge_pairs`, `cmd/compare_pair` |
| `round2_argon/librarian_ab/` | as round 1, after merging main | gemini-4-argon | `cmd/eval_librarian` |
| `round2_argon/pipeline/` | as round 1, after merging main and fixing `Generic Spring Reverb`; `EVAL_NO_FALLBACK=true` | gemini-4-argon (Sonic Profiler on 3.8-flash) | as round 1 |
| `round2_argon/fix_reminder/` | Librarian-only: "reminder" layout (catalog first + one fixed sentence) vs the round-2 old-order runs (copied in, same frozen Phase 1) | gemini-4-argon | `cmd/eval_librarian -variant reminder -seed-old-from ...` |
| `round2_argon/fix_ucl_last/` | same, "ucl_last" layout (User Capture Library after the per-run text) | gemini-4-argon | `-variant ucl_last` |

File formats:
- `phase1/*.json`: frozen Phase 1 output (`agents.Phase1Result`: tone, sonic, scrape, and the
  Librarian's static catalog text as it was at run time).
- `librarian_ab*/runs.jsonl`, `fix_*/runs.jsonl`: one `evalscore.LibrarianRun` per call; the raw
  Librarian JSON is in `outputs/<query>_p<phase1>_<order>_r<rep>.json`. Scores stored in runs.jsonl
  are from the scorer at run time; `cmd/eval_librarian` re-scores from `outputs/` when it reports.
- `pipeline/{new,old}/manifest.jsonl`: one `evalscore.PipelineRun` per pipeline (latest record per
  query/sample wins); presets in `s<k>/<query>.html` (JSON envelopes), Librarian output in
  `s<k>/<query>.librarian.json`.
- `judge/verdicts.json`: every pair judged twice with positions swapped (`evalscore.PairVerdict`).

Costs in these files are estimates from token counts and `geminiPriceSchedule` as it was when each
run happened (round 1 and round 2 pipelines were priced with cached input at the full input rate;
see the findings document for re-priced figures).
