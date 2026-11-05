# Librarian-only A/B: new order (static catalog first) vs old order

Units (query x Phase 1 sample) with usable runs in both orders: **26**

| | new order | old order |
|---|---|---|
| runs (errors / unparseable) | 130 (0 / 0) | 130 (0 / 0) |
| runs passing every hard rule + single amp | 93.8% | 96.2% |
| mean hard-rule pass fraction | 0.979 | 0.987 |
| mean soft-rule pass fraction | 0.904 | 0.888 |
| exactly one amp (G3) | 99.2% | 100.0% |
| names not in catalog / run (G1) | 0.05 | 0.08 |
| Range Rverb uses / run (G2) | 0.18 | 0.17 |
| self-consistency (mean Jaccard of picks between repeats) | 0.751 | 0.770 |
| cached input share (NOT representative of production, see note) | 90% | 60% |
| cost (est., no cache discount applied where rate unverified) | $10.15 | $10.22 |

Note: repeats within a unit share identical Phase 1 text, so the old order gets prefix-cache hits here that it would not get in production (where per-run text differs). Use the full-pipeline run for realistic cache numbers.

## Pre-registered decision (non-inferiority, margin 5 points)

Paired per-unit difference, new - old, 95% bootstrap CI:

| metric | mean diff | 95% CI | verdict |
|---|---|---|---|
| runs passing every hard rule | -0.023 | [-0.062, +0.000] | **INCONCLUSIVE** |
| hard-rule pass fraction | -0.008 | [-0.021, +0.000] | **NOT_WORSE** |
| soft-rule pass fraction | +0.015 | [+0.000, +0.038] | (reported only) |

Sign test on units (all-hard-pass rate): new better 0, old better 2, ties 24, two-sided p = 0.500

Noise floor (mean |difference| in hard-pass fraction between two halves of the SAME condition): 0.003. A new-vs-old difference smaller than this is indistinguishable from run-to-run noise.

## Per query (fraction of runs passing all hard rules / mean soft fraction)

| query | new hard | old hard | new soft | old soft |
|---|---|---|---|---|
| 01_SRV_Clean | 0.50 | 0.50 | 1.00 | 1.00 |
| 02_Chicago_Blues | 1.00 | 1.00 | 1.00 | 1.00 |
| 03_British_Invasion | 1.00 | 1.00 | 1.00 | 1.00 |
| 04_Southern_Rock | 1.00 | 1.00 | 1.00 | 1.00 |
| 05_Clapton | 1.00 | 1.00 | 0.35 | 0.25 |
| 06_Gilmour | 0.90 | 1.00 | 1.00 | 0.90 |
| 07_Edge | 1.00 | 1.00 | 1.00 | 1.00 |
| 08_EVH | 1.00 | 1.00 | 1.00 | 1.00 |
| 09_BB_King | 1.00 | 1.00 | 1.00 | 1.00 |
| 10_Slash | 1.00 | 1.00 | 0.40 | 0.40 |
| 11_Mayer_Lead | 0.80 | 1.00 | 1.00 | 1.00 |
| 12_Bonamassa | 1.00 | 1.00 | 1.00 | 1.00 |
| 13_Hard_Rock_Blues | 1.00 | 1.00 | 1.00 | 1.00 |

Estimated total spend: $20.37 (token counts x geminiPriceTable; cached-input rates unverified for most models)
