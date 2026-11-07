# E2 device-refine eval, scored

## Runs attempted and how they ended

| Arm | Attempted | Gemini/transport failure | Failed integrity | Output did not parse | No plan built | Usable |
|---|--:|--:|--:|--:|--:|--:|
| A | 8 | 0 | 0 | 0 | 0 | 8 |
| B | 8 | 0 | 0 | 0 | 0 | 8 |

## Paired comparison: 8 (draft, request, rep) triples usable in both arms; 0 triples left out

| Metric | Rule for B | A | B | Pass |
|---|---|--:|--:|:-:|
| Applicable share | B >= 80% and B at least 5 points above A | 98.1% | 100.0% | FAIL |
| Block preservation | B >= 95% | 100.0% | 100.0% | pass |
| Locked violations | B = 0 | 1 | 0 | pass |
| Unasked structural changes | B below A | 0 | 0 | FAIL |
| Mechanical checks (defects per run) | B <= A | 1.75 | 2.00 | FAIL |
| Request addressed (judge, 0-2) | B >= A | 0.00 (n=0) | 0.00 (n=0) | FAIL |

- Applicable share: "clearly above A" is operationalised as +5 points; the doc gives only the 80% target
- Locked violations: a run whose not-applied list was capped cannot show 0, so it fails this row
- Unasked structural changes: if A has none, B cannot be below it: the row then fails (not demonstrable)
- Mechanical checks (defects per run): means over runs that changed the matrix, so a no-change run does not look clean
- Request addressed (judge, 0-2): same model family as the Architect; blind and order-randomised, but not independent

All section 8.1 rows pass for arm B: **false**

## Per request (paired triples only)

| Request | n | Share A | Share B | Kept A | Kept B | Unasked A | Unasked B | Locked A | Locked B |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| tone | 2 | 100% | 100% | 100% | 100% | 0 | 0 | 0 | 0 |
| scene | 2 | 90% | 100% | 100% | 100% | 0 | 0 | 1 | 0 |
| review | 2 | 100% | 100% | 100% | 100% | 0 | 0 | 0 | 0 |
| structural | 2 | 100% | 100% | 100% | 100% | 0 | 0 | 0 | 0 |

## Counts behind the rows (paired triples)

| Arm | Runs | No-change | Ops | Not applied | Kept/draft blocks | Runs with locked | Structural | Unasked | Runs with unasked | Mech runs | Defects | Capped lists |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| A | 8 | 0 | 51 | 1 | 44/44 | 1 | 0 | 0 | 0 | 8 | 14 | 0 |
| B | 8 | 0 | 43 | 0 | 44/44 | 0 | 0 | 0 | 0 | 8 | 16 | 0 |

## Cost of the recorded refine calls (all attempted runs, failed attempts included)

| Arm | Runs | Calls | Input tokens | Output tokens | Thinking tokens | Known USD | Status |
|---|--:|--:|--:|--:|--:|--:|---|
| A | 8 | 8 | 189900 | 18541 | 986 | $0.5751 | KNOWN |
| B | 8 | 8 | 193100 | 17169 | 1704 | $0.5749 | KNOWN |

Known USD excludes any model with no official price (Status PARTIAL or UNKNOWN then makes it a lower bound). `cmd/usage_report` reads the same spend from the usage records when the run recorded to a bucket; this tool runs with no bucket, so the calls files above are the record.
