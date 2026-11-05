# Full-pipeline paired comparison: new order vs old order

runs: new=39 old=39. Differences are (new - old), signed so **positive = new is better** for every metric. 95% bootstrap CIs over paired (query, sample) units. Quality metrics carry a non-inferiority verdict at a 5-point margin; others are descriptive.

| metric | n pairs | new mean | old mean | diff (new better +) | 95% CI | verdict | sign test (new/old better, p) |
|---|---|---|---|---|---|---|---|
| pipeline succeeded | 39 | 0.923 | 0.923 | +0.000 | [-0.103, +0.103] | **INCONCLUSIVE** | 2/2, p=1.000 |
| preset: all hard rules + single amp | 34 | 0.941 | 0.912 | +0.029 | [+0.000, +0.088] | **NOT_WORSE** | 1/0, p=1.000 |
| preset: hard-rule pass fraction | 34 | 0.980 | 0.971 | +0.010 | [+0.000, +0.029] | **NOT_WORSE** | 1/0, p=1.000 |
| librarian: all hard rules + single amp | 34 | 0.941 | 0.941 | +0.000 | [+0.000, +0.000] | **NOT_WORSE** | 0/0, p=1.000 |
| mechanical defects (Architect JSON) | 34 | 0.235 | 0.294 | +0.059 | [-0.176, +0.294] | **INCONCLUSIVE** | 3/2, p=1.000 |
| preset: soft-rule pass fraction | 34 | 0.897 | 0.897 | +0.000 | [+0.000, +0.000] | - | 0/0, p=1.000 |
| librarian: soft-rule pass fraction | 34 | 0.897 | 0.897 | +0.000 | [+0.000, +0.000] | - | 0/0, p=1.000 |
| preset: Range Rverb uses | 34 | 0.206 | 0.029 | -0.176 | [-0.324, -0.029] | - | 1/7, p=0.070 |
| cached input share | 34 | 0.548 | 0.019 | +0.529 | [+0.458, +0.588] | - | 32/1, p=0.000 |
| cost excl. Phase 1 (USD) | 34 | 0.267 | 0.267 | +0.000 | [-0.013, +0.012] | - | 16/18, p=0.864 |
| latency (s) | 34 | 134.302 | 109.089 | -25.213 | [-40.855, -9.603] | - | 9/25, p=0.009 |
