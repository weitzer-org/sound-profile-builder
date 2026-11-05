# Full-pipeline eval summary

Score only runs with integrity_ok=true. Quality: `go run ./cmd/eval_checks`.

| Scenario | Runs | Success | Served-model integrity OK | Avg latency (s) | MAX_TOKENS by agent |
| :-- | --: | --: | --: | --: | :-- |
| S1 | 26 | 18 | 18 | 114 | map[Acoustician:3 Sonic Profiler:4] |
