**STOPPED: the control did not pass; no candidate conclusions.**

# Phase 1B: Architect known-regression repros

Upstream agents 1-11 are frozen once per query on baseline models; only the Architect (agent 12) changes, with the Critic on baseline. Defect = `capture_claim_on_modeled > 0` or `capture_db_on_modeled > 0` (internal/evalcheck). A run is scoreable only if the Architect call succeeded, was served by the requested model in one attempt, and parsed.

| Model | Rule | defect runs / scoreable (expected) | per query (defect/scoreable) | Verdict |
| :-- | :-- | --: | :-- | :-- |
| `gemini-3.1-pro-preview` | R1 | 2 / 5 (5) | 07_Edge 2/5 | **FAIL** |
| `gemini-3.1-pro-preview` | R2 | 0 / 10 (10) | 09_BB_King 0/5; 05_Clapton 0/5 | **PASS** |
| `gemini-4-argon` | R1 | 0 / 0 (5) | 07_Edge 0/0 | **INCONCLUSIVE** |
| `gemini-4-argon` | R2 | 0 / 0 (10) | 09_BB_King 0/0; 05_Clapton 0/0 | **INCONCLUSIVE** |
| `gemini-3.8-flash` | R1 | 0 / 0 (5) | 07_Edge 0/0 | **INCONCLUSIVE** |
| `gemini-3.8-flash` | R2 | 0 / 0 (10) | 09_BB_King 0/0; 05_Clapton 0/0 | **INCONCLUSIVE** |

## Secondary metrics (reported, not gated by 1B)

| Model | runs | Architect ok | scoreable | MAX_TOKENS (Architect) | gear blocks | fuzzy-snapped names | unverified names | modeled-style-on-capture | Critic flags | mean latency (s) | mean out / think tokens | served as |
| :-- | --: | --: | --: | --: | --: | --: | --: | --: | --: | --: | :-- | :-- |
| `gemini-3.1-pro-preview` | 15 | 15 | 15 | 0 | 82 | 0 | 0 | 0 | 1 | 50.2 | 3836 / 4309 | gemini-3.1-pro-preview x15 |
| `gemini-4-argon` | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0.0 | 0 / 0 |  |
| `gemini-3.8-flash` | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0.0 | 0 / 0 |  |
