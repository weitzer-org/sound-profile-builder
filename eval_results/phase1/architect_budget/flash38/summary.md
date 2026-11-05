# Phase 1B: Architect known-regression repros

Upstream agents 1-11 are frozen once per query on baseline models; only the Architect (agent 12) changes, with the Critic on baseline. Defect = `capture_claim_on_modeled > 0` or `capture_db_on_modeled > 0` (internal/evalcheck). A run is scoreable only if the Architect call succeeded, was served by the requested model in one attempt, and parsed.

| Model | Rule | defect runs / scoreable (expected) | per query (defect/scoreable) | Verdict |
| :-- | :-- | --: | :-- | :-- |
| `gemini-3.8-flash` | R1 | 0 / 5 (5) | 07_Edge 0/5 | **PASS** |
| `gemini-3.8-flash` | R2 | 0 / 9 (10) | 09_BB_King 0/4; 05_Clapton 0/5 | **INCONCLUSIVE** |

## Secondary metrics (reported, not gated by 1B)

| Model | runs | Architect ok | scoreable | MAX_TOKENS (Architect) | gear blocks | fuzzy-snapped names | unverified names | modeled-style-on-capture | Critic flags | mean latency (s) | mean out / think tokens | served as |
| :-- | --: | --: | --: | --: | --: | --: | --: | --: | --: | --: | :-- | :-- |
| `gemini-3.8-flash` | 15 | 14 | 14 | 1 | 86 | 0 | 0 | 0 | 0 | 60.9 | 3874 / 3216 | gemini-3.8-flash x15 |
