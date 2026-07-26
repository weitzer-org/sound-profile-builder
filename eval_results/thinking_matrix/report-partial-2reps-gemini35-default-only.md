# Thinking-Budget Matrix Eval Report

Queries: 01_SRV_Clean, 02_Chicago_Blues, 03_British_Invasion, 04_Southern_Rock, 05_Clapton, 06_Gilmour, 07_Edge, 08_EVH, 09_BB_King, 10_Slash, 11_Mayer_Lead, 12_Bonamassa, 13_Hard_Rock_Blues | Reps per cell: 2

Scored with RunMechanicalQualityChecks (deterministic, judge-free -- see package doc comment). Lower TotalDefects is better; 0 is clean.

Written incrementally -- if this run is interrupted, every row up to the last one present already reflects a completed cell.

| Model | Budget | Query | Reps OK | Avg Latency(s) | Avg In | Avg Out | Avg Thinking | Avg Defects | Unverified | CaptureFmt | Cabinet | Ranges | Critic | Fallback/Parse Issues |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| gemini-3.5-flash | default | 01_SRV_Clean | 2/2 | 78.2 | 49848 | 3730 | 18407 | 1.00 | 0.0 | 0.0 | 1.0 | 0.0 | 0.0 | - |
| gemini-3.5-flash | default | 02_Chicago_Blues | 2/2 | 95.6 | 50641 | 3850 | 21896 | 2.00 | 0.0 | 0.0 | 2.0 | 0.0 | 0.0 | - |
| gemini-3.5-flash | default | 03_British_Invasion | 2/2 | 96.7 | 50508 | 3476 | 21672 | 3.00 | 0.0 | 0.0 | 2.0 | 0.0 | 1.0 | - |
| gemini-3.5-flash | default | 04_Southern_Rock | 2/2 | 91.9 | 50992 | 4282 | 20243 | 3.50 | 0.0 | 0.0 | 2.0 | 0.0 | 1.5 | - |
| gemini-3.5-flash | default | 05_Clapton | 2/2 | 82.3 | 50450 | 3275 | 17763 | 3.00 | 0.0 | 1.0 | 2.0 | 0.0 | 0.0 | - |
| gemini-3.5-flash | default | 06_Gilmour | 2/2 | 107.7 | 52468 | 4939 | 23877 | 2.00 | 0.0 | 0.0 | 2.0 | 0.0 | 0.0 | - |
| gemini-3.5-flash | default | 07_Edge | 2/2 | 87.0 | 50692 | 4334 | 19011 | 3.50 | 0.0 | 1.0 | 1.0 | 0.0 | 1.5 | - |
| gemini-3.5-flash | default | 08_EVH | 2/2 | 103.1 | 52520 | 5498 | 20328 | 3.00 | 0.0 | 0.0 | 2.0 | 0.0 | 1.0 | - |
| gemini-3.5-flash | default | 09_BB_King | 2/2 | 100.5 | 50622 | 3618 | 21030 | 4.00 | 0.0 | 0.0 | 2.0 | 0.0 | 2.0 | - |
| gemini-3.5-flash | default | 10_Slash | 2/2 | 90.1 | 51182 | 4366 | 21458 | 1.00 | 0.0 | 0.0 | 1.0 | 0.0 | 0.0 | - |
| gemini-3.5-flash | default | 11_Mayer_Lead | 1/2 | 88.6 | 51962 | 5018 | 20290 | 2.00 | 0.0 | 0.0 | 2.0 | 0.0 | 0.0 | error: Phase 3 failures: <nil>, [Transducer Tech] Response truncate… |
