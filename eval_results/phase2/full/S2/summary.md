# Full-pipeline eval summary

Score only runs with integrity_ok=true. Quality: `go run ./cmd/eval_checks`.

| Scenario | Runs | Success | Served-model integrity OK | Avg latency (s) | MAX_TOKENS by agent |
| :-- | --: | --: | --: | --: | :-- |
| S2 | 26 | 15 | 15 | 88 | map[Acoustician:9 Architect & Evaluator:2] |
