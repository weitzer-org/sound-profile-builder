# Architect v5 vs v6, re-run after the follow-up fixes (2026-11-05)

Second run of `eval_results/v5_vs_v6/REPORT.md`, after two changes to v6 (PR #120):
- **Device Effect Parameters**: the controls of every delay and reverb model are injected, so a delay or
  reverb the Architect adds or swaps in has its real controls (79% of v6's leftover notes in run 1).
- **Agent I/O saved**: every run writes `<query>.agents.json` (every agent's output; Librarian, Cloud
  Navigator and Architect prompts), so block choices can be attributed to the Librarian's actual suggestions.

**Setup.** As in run 1: `cmd/eval_full_pipeline -scenarios PROD -allow-cloud-captures`, `EVAL_NO_FALLBACK=true`,
13 golden queries x 2 reps per arm, `-architect-prompt v5` vs `v6`, production routing (gemini-4-argon
everywhere, Sonic Profiler on gemini-3.8-flash). The batch started 17:07 UTC with four processes; Gemini
returned "503 UNAVAILABLE: high demand" on both models, and with fallback off 14 runs failed (5 v5, 9 v6;
13 were 503s, 1 a proxy connection reset; mostly Phase 1 agents, no arm dependence). It was stopped and
resumed with the new `-resume`/`-retry-unavailable` flags; failed records are in `failed_runs.jsonl`. The
resumed records keep the original batch id, so `runs.jsonl` doesn't mark them; the largest time gap between
the two arms' runs of the same pair is ~33 min (by output-file time). Scoring (no API calls) as in run 1.
Checked by an opus-verifier pass; its corrections are folded in.

## Results (26 runs per arm, every one served by the requested models)

| | v5 (production) | v6 |
|---|--:|--:|
| Device defects per preset | 19.23 | 0.23 |
| Control names the device doesn't have | 15.23 | 0.08 |
| Cab `Blend` (no such control) | 2.00 | 0 |
| Controls neither set nor in `kept_defaults` (v6 output only) | - | 0.15 (run 1: 2.08) |
| Main's gated defects (capture/dB, snapped/unverified names) | 4 (2 files, both capture naming) | 0 |
| Re-Critic issues per preset (Argon, own notes stripped) | 0.38 | 0.08 |
| Blocks per guitar | 7.19 | 6.62 |
| Guitars with a delay / reverb | 65% / 100% | 69% / 100% |
| Cost per preset in this eval, intro prices | $0.203 | $0.245 |
| Pipeline latency median / p95 | 109 / 174 s | 118 / 176 s |
| Architect input / output tokens | 9,063 / 5,673 | 25,035 / 6,541 |

**Block choices follow the Librarian in both arms** (from the saved Librarian output, classified by catalog
category): when it suggested a delay both arms had one (15/15 and 15/15); when it didn't, v5 added one 2/11,
v6 3/11. An EQ it suggested was used 19/19 (v5) and 15/15 (v6); a modulation block 6/6 and 2/2; neither arm
ever added an EQ or modulation block it didn't suggest. So the EQ/modulation differences (v5 73%/23% vs v6
58%/8%) and run 1's delay difference are Librarian sampling, not the Architect. A delay placed after the cab
in one v6 Edge run was the Librarian's instruction; v5 placed it there in another run too.

**Judges (26 pairs each).** 3.1 Pro 26-0 for v6, 3.6 Flash 23-3, 3.8 Flash 22-4, Argon 25-1; the three
non-Argon judges were unanimous for v6 on 21 pairs and split on 5. Every verdict was served by the requested
judge. These are not blind: only v6 output carries `kept_defaults`, and the judges' reasons are mostly
parameter-name authenticity (e.g. a Wah given "Frequency"/"Q", a cab "Blend"), the same signal as the device
check. Read them as agreeing with the parameter-fidelity result, not as separate evidence of better tone.

**Cost in production.** The eval's constraints allow every paid plugin, so it injects more than production
(config.json owns Cory Wong and John Mayer). Rendering the same Librarian picks under the production gate
gives ~16.6 KB of device sections vs ~24.6 KB in the eval (measured with a scratch program; not re-verified
independently), so v6 should cost ~$0.235 vs $0.203 per preset (+~16%), slightly overstated because the
Librarian wouldn't pick unowned plugins. When Argon's price doubles on 2026-12-01 the premium grows to
~$0.06-0.07 per preset.

## Still open
- Issue #125: Parametric-8 band `N BYPASS` polarity is unknown; v6 writes numbered band BYPASS values on 10
  blocks in this run.
- The device check's names come from the unit's ModelRepo.xml; that they match the on-screen labels is not
  verified on a device.

## Recommendation
Switch `config.json`'s `12_architect` to `v6`. Device parameter errors drop ~80x, gated defects and Critic
issues drop, and block choices are unchanged, for ~+$0.03 per preset now (~+16%; ~+$0.06-0.07 after Argon's
price doubles on Dec 1) and ~9 s more per preset.
