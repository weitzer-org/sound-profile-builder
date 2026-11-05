# Architect v5 vs v6 on production routing (2026-11-05)

**Question.** Does the device-parameter Architect prompt (`12_architect_v6`: production v5 plus the
device-catalog sections, injected parameters/effect names and `kept_defaults`) keep pipeline quality
at least as good as production's v5, while fixing device-parameter errors?

**Setup.** `cmd/eval_full_pipeline -scenarios PROD -allow-cloud-captures`, `EVAL_NO_FALLBACK=true`, 13 golden
queries x 2 reps per arm, `-architect-prompt v5` vs `v6`, four processes in one batch (started 15:47 UTC).
Production routing: gemini-4-argon on every agent except the Sonic Profiler (gemini-3.8-flash). Upstream
agents run fresh in every run, so the two arms do not see identical Librarian output. Scoring (no API calls):
`cmd/eval_device_checks` (production post-generation checks incl. the device check, same checker for both
arms), `cmd/eval_checks` (main's gated defects), `docs/eval/scripts/run_quality.sh` (each preset's own Critic
notes stripped, gemini-4-argon re-Critic, four judges with randomized A/B). Independently checked by an
opus-verifier pass (no API calls); its corrections are folded in below.

## Results

| | v5 (production) | v6 |
|---|--:|--:|
| Runs complete, every call served by the requested model, no MAX_TOKENS | 26/26 | 26/26 |
| Control names the device doesn't have, per preset | 13.2 | 0.54 |
| Cab `Blend` (no such control) | 2.0 | 0 |
| Out-of-range values / bad switch positions | 0.62 / 0.38 | 0 / 0 |
| Invented effect names | 0.46 | 0 |
| Controls neither set nor in `kept_defaults` (only v6 output can fail this) | - | 2.08 |
| Main's gated defects (capture/dB, snapped/unverified names) | 0 | 2 (one run, one invented cab on both guitars) |
| Re-Critic issues per preset (Argon, own notes stripped) | 0.42 (7 medium, 4 high) | 0.08 (2 high) |
| Blocks per guitar | 6.85 | 6.62 |
| Guitars with a delay / reverb | 77% / 100% | 58% / 96% |
| Cost per preset, intro / from 2026-12-01 Argon price | $0.201 / $0.403 | $0.220 / $0.441 |
| Pipeline latency median / p95 | 142 / 216 s | 152 / 201 s |
| Architect tokens in / out | 9,105 / 5,513 | 12,401 / 6,599 |

Prompt size: v5 13,269 bytes, v6 18,611 bytes, plus 2-6 KB of injected device sections per run.

**Judges (26 pairs each).** 3.1 Pro, 3.6 Flash and 3.8 Flash each prefer v6 24-2; Argon 26-0. The three
non-Argon judges are unanimous for v6 on 22 pairs, for v5 on 1, split on 3. Every verdict was served by the
requested judge; the winner was the preset shown first in 60/104.

## How to read it (verifier corrections)

- **The device-name gap is real but partly by construction.** v5's own prompt tells it to write a cab `Blend`
  and EQ summary sliders ("750Hz Slider"), and it is never shown the device's names; v6 is. The honest
  statement: v5 writes ~13 control names per preset that the Quad Cortex doesn't have, and v6 almost never
  does. That still matters to a user (a knob that doesn't exist can't be dialled in). Catalog names come from
  the unit's ModelRepo.xml; that they match the on-screen labels is not verified on a device.
- **v6's residual 2.6 device defects are mostly bookkeeping**: 2.08 are controls left unset (they stay at the
  device default either way). Its real wrong-name rate is 0.54/preset.
- **Where v6's residue comes from** (68 notes): Spring reverb (`Decay`/`Pre-Delay` and unconsidered controls,
  24+14) and Slapback Delay (16) are effects the Architect swapped in, whose parameters weren't injected (79%).
  Parametric-8 (14, all 07_Edge rep 1) is different: the Architect had its parameters and still left seven
  band gains unconsidered.
- **Parametric-8 BYPASS polarity is unknown and may hide a systematic bug.** The catalog's `N BYPASS`
  switches have no position names (defaults: 1 on bands 1-4, 0 on bands 5-8). v6 wrote `8 BYPASS: 1` on 6
  blocks while describing band 8 as active, and moved band 1/3/4 gains with their BYPASS at 1. If 1 means
  bypassed, those EQ moves do nothing. Only a real unit can settle it.
- **Judges carry little weight.** They cannot check device parameter names, so they may reward v6 for
  sounding catalog-like; at least one verdict called a real block ("John Mayer Spring Reverb", a configured
  plugin) hallucinated. Read the judge result as "no sign v6 is worse", not as a quality win.
- **The delay drop (77% -> 58%) is observed but not explained.** An earlier causal claim (v6 adds unsuggested
  effects less) used `agent_impact` as a proxy for the Librarian's suggestions, but the Architect writes
  `agent_impact`, so the proxy is circular; the Librarian's raw output wasn't saved. Upstream variance is
  still a candidate explanation. Judges did prefer v5 where v6 dropped a genre-essential delay (06_Gilmour
  rep 2).

## Recommendation

Don't switch `config.json` yet. First:
1. Check Parametric-8 (and other unnamed switch) BYPASS polarity on a real Quad Cortex.
2. Inject parameters for effects the Architect swaps in or adds (the Spring/Slapback residue).
3. In the next run, save the Librarian's raw output and the Architect's prompt, so delay/reverb presence
   can be attributed properly.

Then re-run this same comparison (~$12 at intro prices).
