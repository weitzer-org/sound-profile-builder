# Device and mechanical checks, re-scored with one checker

| Arm | Scored runs (success, integrity ok) | Parsed |
|---|--:|--:|
| v5 | 26 | 26 |
| v6 | 26 | 26 |

## Means over the 26 (query, rep) pairs parsed in every arm

| Metric (mean per preset) | v5 | v6 |
|---|--:|--:|
| Blocks | 13.69 | 13.23 |
| Defects (all checks) | 17.12 | 2.77 |
| Device defects | 16.69 | 2.62 |
| Device defects / checked block | 1.60 | 0.23 |
|   unknown param names | 13.23 | 0.54 |
|   out of range | 0.62 | 0.00 |
|   bad switch values | 0.38 | 0.00 |
|   unit mismatches | 0.00 | 0.00 |
|   confirmed_range overclaims | 0.00 | 0.00 |
|   invented block names | 0.46 | 0.00 |
|   cab Blend (old convention) | 2.00 | 0.00 |
|   unconsidered controls (kept_defaults output only) | 0.00 | 2.08 |
| Checked blocks | 10.46 | 11.08 |
| Unverified blocks | 0.00 | 0.08 |
| Capture formatting mismatches | 0.00 | 0.00 |
| Incomplete cabinets | 0.00 | 0.00 |
| Leftover value ranges | 0.00 | 0.00 |
| Critic issues (baked into the output) | 0.42 | 0.08 |

## Per-query paired difference in device defects (last arm minus first, mean over reps)

| Query | v5 | v6 | diff |
|---|--:|--:|--:|
| 01_SRV_Clean | 9.0 | 0.0 | -9.0 |
| 02_Chicago_Blues | 13.0 | 0.0 | -13.0 |
| 03_British_Invasion | 28.0 | 4.0 | -24.0 |
| 04_Southern_Rock | 21.0 | 18.0 | -3.0 |
| 05_Clapton | 24.0 | 0.0 | -24.0 |
| 06_Gilmour | 12.0 | 0.0 | -12.0 |
| 07_Edge | 17.0 | 7.0 | -10.0 |
| 08_EVH | 22.0 | 0.0 | -22.0 |
| 09_BB_King | 26.0 | 0.0 | -26.0 |
| 10_Slash | 13.0 | 0.0 | -13.0 |
| 11_Mayer_Lead | 6.0 | 5.0 | -1.0 |
| 12_Bonamassa | 10.0 | 0.0 | -10.0 |
| 13_Hard_Rock_Blues | 16.0 | 0.0 | -16.0 |

## Means over each arm's own parsed runs

| Metric (mean per preset) | v5 | v6 |
|---|--:|--:|
| Blocks | 13.69 | 13.23 |
| Defects (all checks) | 17.12 | 2.77 |
| Device defects | 16.69 | 2.62 |
| Device defects / checked block | 1.60 | 0.23 |
|   unknown param names | 13.23 | 0.54 |
|   out of range | 0.62 | 0.00 |
|   bad switch values | 0.38 | 0.00 |
|   unit mismatches | 0.00 | 0.00 |
|   confirmed_range overclaims | 0.00 | 0.00 |
|   invented block names | 0.46 | 0.00 |
|   cab Blend (old convention) | 2.00 | 0.00 |
|   unconsidered controls (kept_defaults output only) | 0.00 | 2.08 |
| Checked blocks | 10.46 | 11.08 |
| Unverified blocks | 0.00 | 0.08 |
| Capture formatting mismatches | 0.00 | 0.00 |
| Incomplete cabinets | 0.00 | 0.00 |
| Leftover value ranges | 0.00 | 0.00 |
| Critic issues (baked into the output) | 0.42 | 0.08 |
