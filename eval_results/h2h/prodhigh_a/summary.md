# Full-pipeline eval summary

Score only runs with integrity_ok=true. Quality: `go run ./cmd/eval_checks`.

| Scenario | Runs | Success | Served-model integrity OK | Avg latency (s) | MAX_TOKENS by agent |
| :-- | --: | --: | --: | --: | :-- |
| PROD | 14 | 5 | 5 | 318 | map[Architect & Evaluator:1 Control Mapper:1 CorOS Librarian:1] |
