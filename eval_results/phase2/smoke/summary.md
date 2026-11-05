# Full-pipeline eval summary

Score only runs with integrity_ok=true. Quality: `go run ./cmd/eval_checks`.

| Scenario | Runs | Success | Served-model integrity OK | Avg latency (s) | MAX_TOKENS by agent |
| :-- | --: | --: | --: | --: | :-- |
| A1 | 1 | 1 | 1 | 120 | map[] |
| A2 | 1 | 0 | 0 | 111 | map[Acoustician:1] |
| A3 | 1 | 1 | 1 | 115 | map[] |
| S0 | 1 | 1 | 1 | 97 | map[] |
| S1 | 1 | 1 | 1 | 142 | map[] |
| S2 | 1 | 0 | 0 | 61 | map[Acoustician:1] |
| S4 | 1 | 1 | 1 | 96 | map[] |
| S5 | 1 | 1 | 1 | 129 | map[] |
