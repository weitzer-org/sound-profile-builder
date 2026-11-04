# Phase 1B: Architect known-regression repros

Upstream agents 1-11 are frozen once per query on baseline models; only the Architect (agent 12) changes, with the Critic on baseline. Defect = `capture_claim_on_modeled > 0` or `capture_db_on_modeled > 0` (internal/evalcheck). A run is scoreable only if the Architect call succeeded, was served by the requested model in one attempt, and parsed.

| Model | Rule | defect runs / scoreable (expected) | per query (defect/scoreable) | Verdict |
| :-- | :-- | --: | :-- | :-- |
| `gemini-3.1-pro-preview` | R1 | 0 / 1 (1) | 07_Edge 0/1 | **PASS** |
| `gemini-4-argon` | R1 | 0 / 1 (1) | 07_Edge 0/1 | **PASS** |
| `gemini-3.8-flash` | R1 | 1 / 1 (1) | 07_Edge 1/1 | **FAIL** |

## Secondary metrics (reported, not gated by 1B)

| Model | runs | Architect ok | scoreable | MAX_TOKENS (Architect) | gear blocks | fuzzy-snapped names | unverified names | modeled-style-on-capture | Critic flags | mean latency (s) | mean out / think tokens | served as |
| :-- | --: | --: | --: | --: | --: | --: | --: | --: | --: | --: | :-- | :-- |
| `gemini-3.1-pro-preview` | 1 | 1 | 1 | 0 | 8 | 0 | 0 | 0 | 0 | 49.9 | 3798 / 4032 | gemini-3.1-pro-preview x1 |
| `gemini-4-argon` | 1 | 1 | 1 | 0 | 8 | 0 | 0 | 0 | 0 | 45.4 | 4566 / 0 | gemini-4-argon x1 |
| `gemini-3.8-flash` | 1 | 1 | 1 | 0 | 8 | 0 | 0 | 0 | 0 | 46.4 | 2818 / 4228 | gemini-3.8-flash x1 |
