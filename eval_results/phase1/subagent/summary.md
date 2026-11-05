# Phase 1A: independent-agent screen

Rates are over all calls of the cell (failed and truncated calls count as schema-invalid). `grounded` = response carried search queries or grounding chunks. Latency is the single API attempt.

| Agent | Model | role | n | schema valid | truncated | other errors | grounded | served-model issues | p50 / p95 latency (s) | mean in / out / think tokens |
| :-- | :-- | :-- | --: | --: | --: | --: | --: | --: | --: | :-- |
| Tone Historian | `gemini-4-argon` | candidate | 26 | 26 (100%) | 0 (0%) | 0 | 3 (12%) | 0 | 10.9 / 26.8 | 785 / 201 / 30 |
| Tone Historian | `gemini-3.8-flash` | candidate | 26 | 26 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 10.7 / 29.1 | 912 / 97 / 916 |
| Tone Historian | `gemini-3.1-pro-preview` | baseline | 26 | 26 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 12.6 / 18.2 | 587 / 92 / 1024 |
| Sonic Profiler | `gemini-3.5-flash` | baseline | 26 | 26 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 6.9 / 10.5 | 1731 / 78 / 991 |
| Sonic Profiler | `gemini-3.5-flash-lite` | candidate | 26 | 24 (92%) | 2 (8%) | 0 | 0 (0%) | 0 | 1.8 / 8.4 | 1604 / 531 / 0 |
| Community Scraper | `gemini-3.5-flash` | baseline | 26 | 26 (100%) | 0 (0%) | 0 | 6 (23%) | 0 | 10.6 / 13.9 | 416 / 309 / 1044 |
| Sonic Profiler | `gemini-3.8-flash` | candidate | 26 | 26 (100%) | 0 (0%) | 0 | 0 (0%) | 0 | 9.4 / 24.8 | 2148 / 90 / 1120 |
| Community Scraper | `gemini-4-argon` | candidate | 26 | 26 (100%) | 0 (0%) | 0 | 4 (15%) | 0 | 9.7 / 23.7 | 677 / 436 / 0 |
| Community Scraper | `gemini-3.5-flash-lite` | candidate | 26 | 26 (100%) | 0 (0%) | 0 | 5 (19%) | 0 | 2.3 / 2.9 | 217 / 221 / 0 |
| Community Scraper | `gemini-3.8-flash` | candidate | 26 | 26 (100%) | 0 (0%) | 0 | 4 (15%) | 0 | 17.8 / 45.0 | 1076 / 406 / 1387 |
| Sonic Profiler | `gemini-4-argon` | candidate | 26 | 16 (62%) | 10 (38%) | 0 | 3 (12%) | 0 | 11.2 / 41.8 | 2271 / 1616 / 107 |

## Exit rule (candidate dropped from the agent if schema validity is below baseline, truncation above baseline, or any served-model issue)

- keep Tone Historian / `gemini-4-argon`
- keep Tone Historian / `gemini-3.8-flash`
- **DROP** Sonic Profiler / `gemini-3.5-flash-lite`: schema validity 24/26 below baseline 26/26; truncation 2/26 above baseline 0/26
- keep Sonic Profiler / `gemini-3.8-flash`
- keep Community Scraper / `gemini-4-argon`
- keep Community Scraper / `gemini-3.5-flash-lite`
- keep Community Scraper / `gemini-3.8-flash`
- **DROP** Sonic Profiler / `gemini-4-argon`: schema validity 16/26 below baseline 26/26; truncation 10/26 above baseline 0/26
