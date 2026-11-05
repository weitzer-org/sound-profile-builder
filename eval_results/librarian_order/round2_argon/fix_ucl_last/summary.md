**'new order' below = eval-only layout variant "ucl_last"**; old-order runs reused from eval_results/librarian_order/round2_argon/librarian_ab (run earlier, same frozen Phase 1)

# Librarian-only A/B: new order (static catalog first) vs old order

Units (query x Phase 1 sample) with usable runs in both orders: **26**

| | new order | old order |
|---|---|---|
| runs (errors / unparseable) | 130 (0 / 0) | 128 (0 / 0) |
| runs passing every hard rule + single amp | 85.4% | 92.2% |
| mean hard-rule pass fraction | 0.936 | 0.967 |
| mean soft-rule pass fraction | 0.923 | 0.922 |
| exactly one amp (G3) | 96.9% | 96.1% |
| names not in catalog / run (G1) | 0.01 | 0.00 |
| Range Rverb uses / run (G2) | 0.00 | 0.00 |
| runs choosing >=1 user capture | 44.6% | 57.0% |
| self-consistency (mean Jaccard of picks between repeats) | 0.816 | 0.836 |
| cached input share (NOT representative of production, see note) | 88% | 58% |
| cost (est., no cache discount applied where rate unverified) | $1.80 | $8.54 |

Note: repeats within a unit share identical Phase 1 text, so the old order gets prefix-cache hits here that it would not get in production (where per-run text differs). Use the full-pipeline run for realistic cache numbers.

## Pre-registered decision (non-inferiority, margin 5 points)

Paired per-unit difference, new - old, 95% bootstrap CI:

| metric | mean diff | 95% CI | verdict |
|---|---|---|---|
| runs passing every hard rule | -0.069 | [-0.154, +0.000] | **INCONCLUSIVE** |
| hard-rule pass fraction | -0.032 | [-0.074, +0.000] | **INCONCLUSIVE** |
| soft-rule pass fraction | +0.000 | [+0.000, +0.000] | (reported only) |
| runs choosing >=1 user capture (margin 10 points) | -0.115 | [-0.269, +0.031] | **INCONCLUSIVE** |

Sign test on units (all-hard-pass rate): new better 0, old better 3, ties 23, two-sided p = 0.250

Noise floor (mean |difference| in hard-pass fraction between two halves of the SAME condition): 0.013. A new-vs-old difference smaller than this is indistinguishable from run-to-run noise.

## Per query (fraction of runs passing all hard rules / mean soft fraction)

| query | new hard | old hard | new soft | old soft |
|---|---|---|---|---|
| 01_SRV_Clean | 0.50 | 0.50 | 1.00 | 1.00 |
| 02_Chicago_Blues | 1.00 | 1.00 | 1.00 | 1.00 |
| 03_British_Invasion | 1.00 | 1.00 | 1.00 | 1.00 |
| 04_Southern_Rock | 0.30 | 1.00 | 1.00 | 1.00 |
| 05_Clapton | 1.00 | 1.00 | 1.00 | 1.00 |
| 06_Gilmour | 1.00 | 1.00 | 1.00 | 1.00 |
| 07_Edge | 1.00 | 1.00 | 1.00 | 1.00 |
| 08_EVH | 0.80 | 1.00 | 1.00 | 1.00 |
| 09_BB_King | 1.00 | 1.00 | 1.00 | 1.00 |
| 10_Slash | 0.50 | 0.50 | 0.00 | 0.00 |
| 11_Mayer_Lead | 1.00 | 1.00 | 1.00 | 1.00 |
| 12_Bonamassa | 1.00 | 1.00 | 1.00 | 1.00 |
| 13_Hard_Rock_Blues | 1.00 | 1.00 | 1.00 | 1.00 |

Estimated total spend: $10.33 (token counts x geminiPriceTable; cached-input rates unverified for most models)
