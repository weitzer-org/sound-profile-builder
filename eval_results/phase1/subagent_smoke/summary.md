# Phase 1A: independent-agent screen

Rates are over all calls of the cell (failed and truncated calls count as schema-invalid). `grounded` = response carried search queries or grounding chunks. Latency is the single API attempt.

| Agent | Model | role | n | schema valid | truncated | other errors | grounded | served-model issues | p50 / p95 latency (s) | mean in / out / think tokens |
| :-- | :-- | :-- | --: | --: | --: | --: | --: | --: | --: | :-- |
| Tone Historian | `gemini-3.1-pro-preview` | baseline | 1 | 1 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 12.9 / 12.9 | 711 / 98 / 884 |
| Tone Historian | `gemini-4-argon` | candidate | 1 | 1 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 10.8 / 10.8 | 789 / 241 / 0 |
| Tone Historian | `gemini-3.8-flash` | candidate | 1 | 1 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 10.4 / 10.4 | 821 / 105 / 554 |
| Sonic Profiler | `gemini-3.5-flash` | baseline | 1 | 1 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 6.0 / 6.0 | 1608 / 96 / 791 |
| Sonic Profiler | `gemini-4-argon` | candidate | 1 | 1 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 12.6 / 12.6 | 2275 / 159 / 308 |
| Sonic Profiler | `gemini-3.8-flash` | candidate | 1 | 1 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 8.3 / 8.3 | 1608 / 96 / 776 |
| Sonic Profiler | `gemini-3.5-flash-lite` | candidate | 1 | 0 (0%) | 1 (100%) | 0 | 0 (0%) | 0 | 9.2 / 9.2 | 1608 / 3993 / 0 |
| Community Scraper | `gemini-3.5-flash` | baseline | 1 | 1 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 10.5 / 10.5 | 456 / 257 / 1047 |
| Community Scraper | `gemini-4-argon` | candidate | 1 | 1 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 7.3 / 7.3 | 681 / 291 / 0 |
| Community Scraper | `gemini-3.8-flash` | candidate | 1 | 1 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 16.9 / 16.9 | 809 / 401 / 1110 |
| Community Scraper | `gemini-3.5-flash-lite` | candidate | 1 | 1 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 2.6 / 2.6 | 221 / 202 / 0 |

## Exit rule (candidate dropped from the agent if schema validity is below baseline, truncation above baseline, or any served-model issue)

- keep Tone Historian / `gemini-4-argon`
- keep Tone Historian / `gemini-3.8-flash`
- keep Sonic Profiler / `gemini-4-argon`
- keep Sonic Profiler / `gemini-3.8-flash`
- **DROP** Sonic Profiler / `gemini-3.5-flash-lite`: schema validity 0/1 below baseline 1/1; truncation 1/1 above baseline 0/1
- keep Community Scraper / `gemini-4-argon`
- keep Community Scraper / `gemini-3.8-flash`
- keep Community Scraper / `gemini-3.5-flash-lite`
