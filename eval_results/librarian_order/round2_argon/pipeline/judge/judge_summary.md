# Blind LLM judge, every pair judged twice with positions swapped

'first' = new in the head-to-head; in the same-condition calibration it is the lower-numbered sample. A winner counts only when BOTH orderings agree.

| comparison | pairs | first wins | second wins | equal | inconsistent | inconsistent rate | sign-test p (consistent winners) | share of raw verdicts picking position A |
|---|---|---|---|---|---|---|---|---|
| new vs old | 39 | 10 | 9 | 1 | 19 | 49% | 1.000 | 0.60 |
| CALIBRATION: same condition, two samples | 13 | 4 | 4 | 0 | 5 | 38% | 1.000 | 0.54 |

How to read it: the calibration row is what pure run-to-run variation looks like to this judge. A head-to-head only means something to the extent it is more lopsided than the calibration row; a high inconsistent rate means the judge cannot be trusted on these pairs.

## Head-to-head per pair

| query | sample | winner | raw [first as A, first as B] |
|---|---|---|---|
| 01_SRV_Clean | 0 | old | [B A] |
| 01_SRV_Clean | 1 | new | [A B] |
| 01_SRV_Clean | 2 | new | [A B] |
| 02_Chicago_Blues | 0 | inconsistent | [A A] |
| 02_Chicago_Blues | 1 | inconsistent | [B B] |
| 02_Chicago_Blues | 2 | inconsistent | [A A] |
| 03_British_Invasion | 2 | old | [B A] |
| 03_British_Invasion | 1 | inconsistent | [A A] |
| 03_British_Invasion | 0 | inconsistent | [A A] |
| 04_Southern_Rock | 2 | inconsistent | [A A] |
| 04_Southern_Rock | 0 | old | [B A] |
| 04_Southern_Rock | 1 | inconsistent | [B B] |
| 05_Clapton | 0 | old | [B A] |
| 05_Clapton | 1 | inconsistent | [A A] |
| 05_Clapton | 2 | inconsistent | [A A] |
| 06_Gilmour | 0 | new | [A B] |
| 06_Gilmour | 1 | inconsistent | [B B] |
| 06_Gilmour | 2 | inconsistent | [A Equal] |
| 07_Edge | 0 | old | [B A] |
| 07_Edge | 1 | inconsistent | [A A] |
| 07_Edge | 2 | inconsistent | [A A] |
| 08_EVH | 2 | old | [B A] |
| 08_EVH | 1 | inconsistent | [A Equal] |
| 08_EVH | 0 | inconsistent | [B B] |
| 09_BB_King | 2 | new | [A B] |
| 09_BB_King | 0 | new | [A B] |
| 09_BB_King | 1 | inconsistent | [A A] |
| 10_Slash | 0 | new | [A B] |
| 10_Slash | 1 | old | [B A] |
| 10_Slash | 2 | inconsistent | [A A] |
| 11_Mayer_Lead | 0 | new | [A B] |
| 11_Mayer_Lead | 1 | old | [B A] |
| 11_Mayer_Lead | 2 | inconsistent | [A A] |
| 12_Bonamassa | 0 | old | [B A] |
| 12_Bonamassa | 1 | new | [A B] |
| 12_Bonamassa | 2 | new | [A B] |
| 13_Hard_Rock_Blues | 0 | equal | [Equal Equal] |
| 13_Hard_Rock_Blues | 1 | inconsistent | [A A] |
| 13_Hard_Rock_Blues | 2 | new | [A B] |

Estimated judge spend: $14.20
