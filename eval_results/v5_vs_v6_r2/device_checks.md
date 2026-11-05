# Device and mechanical checks, re-scored with one checker

| Arm | Scored runs (success, integrity ok) | Parsed |
|---|--:|--:|
| v5 | 26 | 26 |
| v6 | 26 | 26 |

## Means over the 26 (query, rep) pairs parsed in every arm

| Metric (mean per preset) | v5 | v6 |
|---|--:|--:|
| Blocks | 14.38 | 13.23 |
| Defects (all checks) | 19.50 | 0.31 |
| Device defects | 19.23 | 0.23 |
| Device defects / checked block | 1.79 | 0.02 |
|   unknown param names | 15.23 | 0.08 |
|   out of range | 0.85 | 0.00 |
|   bad switch values | 0.31 | 0.00 |
|   unit mismatches | 0.23 | 0.00 |
|   confirmed_range overclaims | 0.00 | 0.00 |
|   invented block names | 0.62 | 0.00 |
|   cab Blend (old convention) | 2.00 | 0.00 |
|   unconsidered controls (kept_defaults output only) | 0.00 | 0.15 |
| Checked blocks | 10.92 | 11.08 |
| Unverified blocks | 0.08 | 0.00 |
| Capture formatting mismatches | 0.00 | 0.00 |
| Incomplete cabinets | 0.08 | 0.00 |
| Leftover value ranges | 0.00 | 0.00 |
| Critic issues (baked into the output) | 0.12 | 0.08 |

## Per-query paired difference in device defects (last arm minus first, mean over reps)

| Query | v5 | v6 | diff |
|---|--:|--:|--:|
| 01_SRV_Clean | 10.0 | 2.0 | -8.0 |
| 02_Chicago_Blues | 17.0 | 0.0 | -17.0 |
| 03_British_Invasion | 28.0 | 0.0 | -28.0 |
| 04_Southern_Rock | 26.0 | 0.0 | -26.0 |
| 05_Clapton | 28.0 | 1.0 | -27.0 |
| 06_Gilmour | 20.0 | 0.0 | -20.0 |
| 07_Edge | 22.0 | 0.0 | -22.0 |
| 08_EVH | 26.0 | 0.0 | -26.0 |
| 09_BB_King | 20.0 | 0.0 | -20.0 |
| 10_Slash | 14.0 | 0.0 | -14.0 |
| 11_Mayer_Lead | 12.0 | 0.0 | -12.0 |
| 12_Bonamassa | 12.0 | 0.0 | -12.0 |
| 13_Hard_Rock_Blues | 15.0 | 0.0 | -15.0 |

## Means over each arm's own parsed runs

| Metric (mean per preset) | v5 | v6 |
|---|--:|--:|
| Blocks | 14.38 | 13.23 |
| Defects (all checks) | 19.50 | 0.31 |
| Device defects | 19.23 | 0.23 |
| Device defects / checked block | 1.79 | 0.02 |
|   unknown param names | 15.23 | 0.08 |
|   out of range | 0.85 | 0.00 |
|   bad switch values | 0.31 | 0.00 |
|   unit mismatches | 0.23 | 0.00 |
|   confirmed_range overclaims | 0.00 | 0.00 |
|   invented block names | 0.62 | 0.00 |
|   cab Blend (old convention) | 2.00 | 0.00 |
|   unconsidered controls (kept_defaults output only) | 0.00 | 0.15 |
| Checked blocks | 10.92 | 11.08 |
| Unverified blocks | 0.08 | 0.00 |
| Capture formatting mismatches | 0.00 | 0.00 |
| Incomplete cabinets | 0.08 | 0.00 |
| Leftover value ranges | 0.00 | 0.00 |
| Critic issues (baked into the output) | 0.12 | 0.08 |
