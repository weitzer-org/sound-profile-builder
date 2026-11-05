# Full-pipeline paired comparison: new order vs old order

runs: new=39 old=39. Differences are (new - old), signed so **positive = new is better** for every metric. 95% bootstrap CIs over paired (query, sample) units. Quality metrics carry a non-inferiority verdict at a 5-point margin; others are descriptive.

| metric | n pairs | new mean | old mean | diff (new better +) | 95% CI | verdict | sign test (new/old better, p) |
|---|---|---|---|---|---|---|---|
| pipeline succeeded | 39 | 1.000 | 1.000 | +0.000 | [+0.000, +0.000] | **NOT_WORSE** | 0/0, p=1.000 |
| preset: all hard rules + single amp | 39 | 0.923 | 0.923 | +0.000 | [-0.077, +0.077] | **INCONCLUSIVE** | 1/1, p=1.000 |
| preset: hard-rule pass fraction | 39 | 0.966 | 0.979 | -0.013 | [-0.038, +0.000] | **NOT_WORSE** | 0/1, p=1.000 |
| librarian: all hard rules + single amp | 39 | 0.923 | 0.897 | +0.026 | [-0.051, +0.103] | **INCONCLUSIVE** | 2/1, p=1.000 |
| mechanical defects (Architect JSON) | 39 | 0.538 | 0.256 | -0.282 | [-0.897, +0.256] | **INCONCLUSIVE** | 4/5, p=1.000 |
| preset: soft-rule pass fraction | 39 | 0.910 | 0.910 | +0.000 | [+0.000, +0.000] | - | 0/0, p=1.000 |
| librarian: soft-rule pass fraction | 39 | 0.910 | 0.910 | +0.000 | [+0.000, +0.000] | - | 0/0, p=1.000 |
| preset: Range Rverb uses | 39 | 0.000 | 0.000 | +0.000 | [+0.000, +0.000] | - | 0/0, p=1.000 |
| cached input share | 39 | 0.384 | 0.002 | +0.382 | [+0.372, +0.392] | - | 39/0, p=0.000 |
| cost excl. Phase 1 (USD) | 39 | 0.223 | 0.225 | +0.002 | [-0.001, +0.006] | - | 23/16, p=0.337 |
| latency (s) | 39 | 113.777 | 97.066 | -16.711 | [-25.950, -6.343] | - | 7/32, p=0.000 |
