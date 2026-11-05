# Blind LLM judge, every pair judged twice with positions swapped

'first' = new in the head-to-head; in the same-condition calibration it is the lower-numbered sample. A winner counts only when BOTH orderings agree.

| comparison | pairs | first wins | second wins | equal | inconsistent | inconsistent rate | sign-test p (consistent winners) | share of raw verdicts picking position A |
|---|---|---|---|---|---|---|---|---|
| new vs old | 34 | 4 | 12 | 0 | 18 | 53% | 0.077 | 0.75 |
| CALIBRATION: same condition, two samples | 11 | 2 | 5 | 0 | 4 | 36% | 0.453 | 0.68 |

How to read it: the calibration row is what pure run-to-run variation looks like to this judge. A head-to-head only means something to the extent it is more lopsided than the calibration row; a high inconsistent rate means the judge cannot be trusted on these pairs.

## Head-to-head per pair

| query | sample | winner | raw [first as A, first as B] |
|---|---|---|---|
| 01_SRV_Clean | 2 | old | [B A] |
| 01_SRV_Clean | 1 | old | [B A] |
| 01_SRV_Clean | 0 | old | [B A] |
| 02_Chicago_Blues | 0 | old | [B A] |
| 02_Chicago_Blues | 2 | inconsistent | [A A] |
| 02_Chicago_Blues | 1 | inconsistent | [A A] |
| 03_British_Invasion | 1 | inconsistent | [A A] |
| 03_British_Invasion | 0 | inconsistent | [A A] |
| 03_British_Invasion | 2 | inconsistent | [A A] |
| 04_Southern_Rock | 0 | old | [B A] |
| 04_Southern_Rock | 1 | old | [B A] |
| 04_Southern_Rock | 2 | old | [B A] |
| 05_Clapton | 0 | inconsistent | [A A] |
| 05_Clapton | 1 | inconsistent | [A A] |
| 05_Clapton | 2 | new | [A B] |
| 06_Gilmour | 0 | new | [A B] |
| 06_Gilmour | 2 | old | [B A] |
| 07_Edge | 0 | inconsistent | [A A] |
| 07_Edge | 1 | inconsistent | [A A] |
| 07_Edge | 2 | new | [A B] |
| 08_EVH | 0 | inconsistent | [A A] |
| 08_EVH | 2 | inconsistent | [A A] |
| 09_BB_King | 1 | inconsistent | [A A] |
| 09_BB_King | 2 | old | [B A] |
| 09_BB_King | 0 | old | [B A] |
| 10_Slash | 0 | inconsistent | [A A] |
| 10_Slash | 1 | inconsistent | [A A] |
| 10_Slash | 2 | new | [A B] |
| 11_Mayer_Lead | 2 | inconsistent | [A A] |
| 12_Bonamassa | 0 | inconsistent | [Equal A] |
| 12_Bonamassa | 1 | inconsistent | [A A] |
| 12_Bonamassa | 2 | old | [B A] |
| 13_Hard_Rock_Blues | 1 | inconsistent | [A A] |
| 13_Hard_Rock_Blues | 0 | old | [B A] |

Estimated judge spend: $5.83
