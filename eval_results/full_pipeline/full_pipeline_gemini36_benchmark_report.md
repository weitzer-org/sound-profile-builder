# 🪐 Full Pipeline - gemini-3.6-flash Candidate Routing Benchmark Report

An end-to-end performance and qualitative benchmark comparing production routing (Pro tier: gemini-3.1-pro-preview on agents 1/12, Flash tier: gemini-3.5-flash on agents 2-11/13) against two gemini-3.6-flash candidate routings, one tier swapped at a time.

## 📊 Telemetry Summary Matrix

| Scenario | Metric | Baseline (Prod Routing) | Flash-Tier Candidate (gemini-3.6-flash) | Pro-Tier Candidate (gemini-3.6-flash) |
| :--- | :--- | :--- | :--- | :--- |
| **01 SRV Clean** | Pipeline Latency | 144.58s | 112.79s | 89.85s |
| | Accrued Tokens (In/Out) | 55264 / 5156 | 53444 / 4573 | 54281 / 4880 |
| | Fallback Events / Notes | None | None | None |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/01_SRV_Clean.html) | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/01_SRV_Clean.html) | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/01_SRV_Clean.html) |
| **02 Chicago Blues** | Pipeline Latency | 132.48s | 135.72s | 103.03s |
| | Accrued Tokens (In/Out) | 54114 / 4568 | 60645 / 8199 | 55706 / 5812 |
| | Fallback Events / Notes | None | None | None |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/02_Chicago_Blues.html) | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/02_Chicago_Blues.html) | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/02_Chicago_Blues.html) |
| **03 British Invasion** | Pipeline Latency | 129.85s | *Failed*  | 131.00s |
| | Accrued Tokens (In/Out) | 53850 / 4745 | -  | 56922 / 4746 |
| | Fallback Events / Notes | None | Error: Phase 3 failures: [Acoustician] Response truncated at the MaxOutputTokens limit (model gemini-3.6-flash) -- output is incomplete/invalid JSON, not a usable result, <nil>, <nil> | None |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/03_British_Invasion.html) | N/A  | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/03_British_Invasion.html) |
| **04 Southern Rock** | Pipeline Latency | 132.17s | 103.69s | 118.61s |
| | Accrued Tokens (In/Out) | 56534 / 6977 | 53730 / 5678 | 56744 / 6429 |
| | Fallback Events / Notes | None | None | None |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/04_Southern_Rock.html) | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/04_Southern_Rock.html) | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/04_Southern_Rock.html) |
| **05 Clapton** | Pipeline Latency | *Failed*  | 115.35s | 126.52s |
| | Accrued Tokens (In/Out) | -  | 53763 / 5413 | 57659 / 6012 |
| | Fallback Events / Notes | Error: Phase 3 failures: [Acoustician] Response truncated at the MaxOutputTokens limit (model gemini-3.5-flash) -- output is incomplete/invalid JSON, not a usable result, <nil>, <nil> | None | None |
| | Local HTML Matrix Page | N/A  | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/05_Clapton.html) | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/05_Clapton.html) |
| **06 Gilmour** | Pipeline Latency | 137.72s | 101.92s | 113.40s |
| | Accrued Tokens (In/Out) | 53813 / 5222 | 52338 / 3233 | 56034 / 5195 |
| | Fallback Events / Notes | None | None | None |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/06_Gilmour.html) | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/06_Gilmour.html) | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/06_Gilmour.html) |
| **07 Edge** | Pipeline Latency | 149.17s | 96.30s | 99.86s |
| | Accrued Tokens (In/Out) | 55595 / 4997 | 53749 / 5284 | 54760 / 4930 |
| | Fallback Events / Notes | None | None | None |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/07_Edge.html) | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/07_Edge.html) | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/07_Edge.html) |
| **08 EVH** | Pipeline Latency | 127.63s | 125.15s | 111.60s |
| | Accrued Tokens (In/Out) | 56523 / 6866 | 60753 / 9270 | 54437 / 5266 |
| | Fallback Events / Notes | None | None | None |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/08_EVH.html) | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/08_EVH.html) | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/08_EVH.html) |
| **09 BB King** | Pipeline Latency | 166.30s | 91.79s | 97.52s |
| | Accrued Tokens (In/Out) | 55526 / 5064 | 53042 / 4288 | 54787 / 4498 |
| | Fallback Events / Notes | None | None | ⚠️ Yes (1 agent calls fell back to a non-target model) |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/09_BB_King.html) | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/09_BB_King.html) | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/09_BB_King.html) |
| **10 Slash** | Pipeline Latency | 116.69s | 97.76s | 93.71s |
| | Accrued Tokens (In/Out) | 54446 / 5664 | 54071 / 5208 | 54686 / 5216 |
| | Fallback Events / Notes | None | None | None |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/10_Slash.html) | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/10_Slash.html) | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/10_Slash.html) |
| **11 Mayer Lead** | Pipeline Latency | 155.22s | 132.46s | *Failed*  |
| | Accrued Tokens (In/Out) | 56356 / 6462 | 64300 / 10381 | -  |
| | Fallback Events / Notes | None | None | Error: Phase 3 failures: [Acoustician] Response truncated at the MaxOutputTokens limit (model gemini-3.5-flash) -- output is incomplete/invalid JSON, not a usable result, <nil>, <nil> |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/11_Mayer_Lead.html) | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/11_Mayer_Lead.html) | N/A  |
| **12 Bonamassa** | Pipeline Latency | 138.46s | 114.14s | 96.00s |
| | Accrued Tokens (In/Out) | 54701 / 5401 | 54029 / 5283 | 54009 / 5262 |
| | Fallback Events / Notes | None | None | None |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/12_Bonamassa.html) | [View Flash-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/flash-tier-candidate/12_Bonamassa.html) | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/12_Bonamassa.html) |
| **13 Hard Rock Blues** | Pipeline Latency | 121.94s | *Failed*  | 104.85s |
| | Accrued Tokens (In/Out) | 54706 / 6130 | -  | 54208 / 5953 |
| | Fallback Events / Notes | None | Error: Phase 3 failures: [Acoustician] Response truncated at the MaxOutputTokens limit (model gemini-3.6-flash) -- output is incomplete/invalid JSON, not a usable result, <nil>, <nil> | None |
| | Local HTML Matrix Page | [View Baseline (Prod Routing) Output](file://eval_results/full_pipeline/baseline/13_Hard_Rock_Blues.html) | N/A  | [View Pro-Tier Candidate (gemini-3.6-flash) Output](file://eval_results/full_pipeline/pro-tier-candidate/13_Hard_Rock_Blues.html) |

---

## 📉 Performance & Resource Averages Summary

| Scenario | Avg Pipeline Latency | Avg Input Tokens | Avg Output Tokens | Queries With Fallback Events |
| :--- | :--- | :--- | :--- | :--- |
| **Baseline (Prod Routing)** | 137.69s | 55119 | 5604 | 0 |
| **Flash-Tier Candidate (gemini-3.6-flash)** | 111.55s | 55806 | 6074 | 0 |
| **Pro-Tier Candidate (gemini-3.6-flash)** | 107.16s | 55353 | 5350 | 1 |


> [!NOTE]
> **Understanding Fallback Events:** a scenario's routing map only targets its own tier's candidate/production models; if any agent call in `ModelsUsed` shows a model outside that set, `getFallbackChain` (orchestrator.go) kicked in after the primary target failed or was rate-limited for that step.

> **Judging quality:** run `cmd/judge_compare` with `DIR_A`/`DIR_B` pointed at `baseline` vs each candidate folder here, `LABEL_A`/`LABEL_B` set accordingly. The judge model stays `gemini-3.1-pro-preview` for both comparisons -- including the Pro-Tier one, where it is itself one of the two candidates -- to keep the grading rubric identical across both runs; that conflict of interest is a known, accepted tradeoff, not an oversight.
