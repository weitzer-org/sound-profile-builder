# Editing presets on the Quad Cortex

*Design and implementation plan, 2026-11-07. Status: proposed, nothing built. Owner decisions D1-D4, D6 and the eval question taken 2026-11-07 (section 10).*
*Parent: `docs/design/qc-device-bridge-plan.md` (decision 7 in section 10 deferred this work). Contract: `docs/design/device-job-contract.md`.*

**Verify-or-label.** Every premise about the unit is either cited (a gate result in the bridge plan's section 7, or code in this repo) or marked **UNVERIFIED**. Section 7 lists the checks that settle the UNVERIFIED ones, all taken in normal use.

---

## 1. Goal

From the phone, through the QC-2 Modeler connector:

1. **Edit a preset that QC-2 pushed** (a slot in the agent's owned ledger).
2. **Edit a preset QC-2 never saw**: one the owner built on the unit, or a factory/Cortex Cloud preset saved to a collection.
3. **Review a unit preset with the pipeline and apply what it suggests**: read the preset, run it through `refine_preset`, see a before/after table, approve, and have the change land on the unit. This works even if the preset didn't come from QC-2.

The owner decisions in the bridge plan still apply: no LLM in the push path, a dry run first, a confirm token, native Windows agent, no globals, no deletes of presets, and a fail-closed agent. This design adds one new capability: **changing an occupied slot**. It is gated harder than anything so far (section 6).

## 2. What exists today (and why it isn't enough)

| Piece | State | Limit for editing |
|---|---|---|
| `read_device_preset(import=true)` (P5) | built, P6-checked (0 unconverted on a pushed preset) | Reads only the **open** preset. Imports only row 0. Keeps scenes A/B (C-H noted), and leaves out unknown models, unverified clones, non-default cab mics, combo boxes and `floatWithLed` (`internal/deviceexport/readback.go`). So the draft of a user preset is **lossy**. |
| `refine_preset` | built | Overwrites the preset's payload in place (`internal/presetsvc/refine.go`), so after a refine the imported baseline is gone. The Architect is free to reorder, add or swap blocks. Whether it keeps block `id`s is **unmeasured**. |
| `send_preset_to_device` (P2-P4) | built, P6-checked | **Rebuilds** a whole chain into an **empty**, open slot with a blank grid (`qc_agent/jobs.py` rules 3-4). It can't touch an occupied slot. Rebuilding a user preset would also drop everything the import left out. |
| Owned-slot ledger | built (agent side, local) | Recorded, but never used to allow a write. The "owned-slot re-push" stayed a v2 item. |

**Conclusion.** Editing can't be "import, refine, re-push". Rebuilding would silently erase rows 1-3, IRs, splits, expression assignments and anything else the import couldn't convert. Editing needs a different job shape: a **patch** that changes only what the user approved, on the preset as the unit holds it, and leaves everything else untouched.

## 3. Core design: diff-and-patch against a fresh read

```
 unit (open preset) ──read_preset──► snapshot S (raw read-back, stored)  ──import──► draft D (lossy view)
                                                                                     │
                                         refine_preset / manual edits / QC-2 original│
                                                                                     ▼
                                                                         target preset T
 deviceedit.Plan(S, T) ──► edit plan E: preconditions + ops + "not applied" list ──► dry run ──► confirm
                                                                                                │
 agent: re-read unit, check every precondition (compare-and-swap), apply ops, verify, [save]  ◄─┘
```

The same path serves all three goals. **Provenance doesn't matter for correctness**: the plan is always computed against what the unit holds (snapshot S), and the agent re-checks it against the live unit before the first write. A QC-2-pushed preset only differs in which save modes are allowed (section 5).

### 3.1 The snapshot (new, QC-2 side)

- Every `read_preset` result that QC-2 parses is stored **raw** (the `qc-device-readback/1` payload after `ParseReadbackPayload`, with strings already through `SanitizeText`), with the read's slot, folder, name and firmware: `device/snapshots/<job_id>.json` in the main bucket, through `storage.Client`.
- An imported draft's `Origin.JobID` names its snapshot. Refining the draft no longer loses the baseline, because the baseline is S and not D.
- S holds every row, every parameter position (string params included) and every bypass flag. It is the reference for "what must not change".

### 3.2 The edit plan: `qc2-device-edit/1` (new package `internal/deviceedit`)

Pure and deterministic, like `internal/deviceexport`, and built on its tables (`resolve.go`, `LabelParams`, `ToWire`, evidence labels), so a conversion rule can't drift between push and edit.

```json
{"format": "qc2-device-edit/1",
 "catalog": {"coros": "4.1", "sha256": "<qc_catalog.json hash>"},
 "expect": {"folder_key": "<from S>", "position": 16, "name": "Texas Blues",
            "layout": [[0, 0, 1120], [0, 1, 12045], [0, 2, 8006], [1, 0, 5]]},
 "ops": [
   {"op": "set_param", "row": 0, "col": 0, "index": 0, "label": "GAIN",
    "expect": [0.45, 0.45, 0.45, 0.45, 0.45, 0.45, 0.45, 0.45],
    "wire":   [0.50, 0.62, 0.45, 0.45, 0.45, 0.45, 0.45, 0.45],
    "display_before": ["4.5", "4.5"], "display_after": ["5", "6.2"], "evidence": "taper:g4"},
   {"op": "set_bypass", "row": 0, "col": 2,
    "expect": [false, false, false, false, false, false, false, false],
    "bypass": [false, true, false, false, false, false, false, false]}],
 "not_applied": [{"block": "b4 Tape Echo", "what": "new block", "reason": "no free cell after col 3 on row 0"}],
 "untouched": {"rows_other_than_0": 1, "unconverted_controls": 3}}
```

Rules:

- **Ops are compare-and-swap.** Each op carries the wire values S had (`expect`). The agent refuses the whole job if any of them differs on the live unit. The plan also pins the open slot, the preset name and the **layout** (every `(row, col, model_id)` on the grid, all rows).
- **Scenes C-H are preserved.** A changed parameter writes all eight scene values (G6: the device never fills scenes for us). A/B carry the target's values and C-H carry **S's own C-H values**, so the user's other scenes stay as they were. (Turning scene mode on for a parameter that had it off is fine for the same reason: C-H keep the old value.)
- **No-ops are dropped.** A target value equal to S's value after conversion (wire within half a display step) produces no op. This is what makes a QC-2 original (not a re-imported draft) usable as a target without rounding noise.
- **Untranslatable means "not applied", never guessed.** If a target value can't be converted under the export's rules, if it hits a control the import left out, or if it needs a structural change this phase doesn't support, it goes on the `not_applied` list with the reason. A plan with only not-applied items has no confirm token.
- **Block matching (target block → device cell).** First by block `id` (the import writes `b<col+1>`) **and** the same model. Then by order-preserving alignment of model names over row 0 (LCS), with ties refused. A target block with no match is a **new block**. A matched cell with no target block is a **removed block**. The planner agent settles the details (section 9). How often the Architect keeps ids is measured in E2.
- **Structural ops, by phase** (section 8):
  - **v1:** only `set_param` and `set_bypass` on existing blocks.
  - **"Remove" in v1 is `set_bypass` in all eight scenes**, which is reversible. A real delete waits for E6.
  - **Later:** `add_block` into a free cell of row 0, `set_capture` on an existing capture block of the same V1/V2, a model swap (E6).
  - **Never in edit mode:** reordering, moving blocks, routing, rows 1-3 structure, globals.
- **Size and text:** the same 256 KiB payload cap as an export. No device text goes into an op. The preset name in `expect` is compared, never written, except as the explicit name of a copy (section 5).

### 3.3 The pipeline side: "review my unit preset"

- The draft of a unit preset is refined like any preset. The difference is **edit-aware context for the Architect** when `Origin.Kind == device-readback`:
  - this preset lives on a device with fixed cells, so keep every block's `id`, model and order, and prefer parameter and bypass changes;
  - add a block only if the request needs it;
  - the unconverted controls and blocks (listed by the import) exist on the unit but are locked, so don't mention or change them.
- This is a **prompt change**, so it gets an **A/B eval before it becomes production** (owner, 2026-11-07; section 8.1). Two guards apply:
  - **Every preset whose origin isn't `device-readback`:** a test that the Architect's context is **byte-identical** to today's. Generation and ordinary refines can't change, so the eval doesn't need to cover them.
  - **Device drafts:** the eval compares today's refine with the edit-aware one.
  The section is gated on the origin, the same way catalog features are gated on the prompt text.
- A convenience entry point, `refine_preset` with a canned "review this unit preset for <goal>; minimal edits" message, so the phone flow is one call: read, import, refine, then the edit dry run.
- **Locked controls are an advantage, not a gap.** What the import couldn't convert never reaches the Architect, so the patch can't write it, and it survives on the unit untouched. The lossy import is safe *because* editing is a patch.

### 3.4 Manual edits without the pipeline (optional, E6)

"Set the amp's gain to 6 in scene B" shouldn't cost a paid pipeline run. A small tool `set_preset_values(preset_id, changes=[{block_id, param, value, value_b}])` edits a stored preset deterministically. The values are validated against the catalog the same way `FlagCatalogParamMismatches` does. The edit then goes through the same dry run. The phone LLM chooses *what* to change; the conversion and the approval are unchanged, so "no LLM in the push path" still holds.

## 4. MCP tools

| Tool | Kind | Behaviour |
|---|---|---|
| `read_device_preset` | changed | Also stores snapshot S (section 3.1), with or without `import`. |
| `edit_device_preset` | **new** | Args: `preset_id` (the target T), `guitar`, `from_read` (snapshot job id; default the draft's `Origin.JobID`, else the latest read of that slot), `mode` (`live` / `copy` / `overwrite`, section 5), `name` and `target_slot` for `copy`, `dry_run` (default **true**), `confirm_token`. The dry run returns a before/after table per block (Scene A and B, display units, the evidence label and † as in the push dry run), the not-applied list, the snapshot's age, and the save mode in words ("overwrites 3A *Texas Blues*, which QC-2 did not create"). The token is bound to the plan's hash, the slot, the preset name and the snapshot id. |
| `undo_device_edit` | **new (E5)** | Builds the inverse plan of a finished edit from its job result: `expect` = the values written, ops = the values S had. Same dry run, token and CAS. If the user changed those controls since, it refuses rather than overwriting their newer edit. |
| `get_device_job` | unchanged | Shows edit results (per-op read-back, collateral check, saved or not). |

Tool annotations: `edit_device_preset` is `DestructiveHint: true` (it can overwrite a stored preset in `overwrite` mode), and its description says so. The server instructions and `qc://style/preset-display` get a short "editing" paragraph: show the before/after table, never confirm without the user's explicit yes. Registration stays under the device-tools gate, so the pinned 9-tool set is unchanged when device tools are off.

## 5. Save modes

| Mode | What the agent does after a verified patch | Allowed on | Unit premise |
|---|---|---|---|
| `live` | **Nothing.** The edit stays on the open preset, unsaved. The owner listens, then saves on the unit (or discards by re-selecting the slot). | any open preset | Writes to a loaded, saved preset behave as on a blank grid, and re-selecting discards them. G7 saw an unsaved stomp edit discarded by recalling the slot and `PresetDirty` set. **Param/bypass writes on a loaded user preset: UNVERIFIED** (E-G1). |
| `copy` | File CREATE of the edited live grid into an **empty** slot (`target_slot`) with a new name. The original slot is untouched. | any open preset; target must be empty (fresh listing), allowed, and not the open one | File CREATE at a position other than the open one saves the working grid there, with everything it holds (rows 1-3, IRs, splits, stomps, expression). **UNVERIFIED** (E-G3). Which slot is open afterwards is also unknown. |
| `overwrite` | File CREATE at the **open** slot with the **same name** | owned slots first; user slots only after owner decision D2 and E-G2 | File CREATE onto an occupied, open position replaces that preset in place (no duplicate, no shift, everything else kept). Cortex Control's own save is what `write_preset_file` copies (vendored `transport.py`, docstring), so this is plausible, but it is **UNVERIFIED** (E-G2). |

**Recommended default for a preset QC-2 didn't create: `copy`.** The original is never at risk, and deleting it afterwards is the owner's own action on the unit. `live` is the zero-risk way to try an edit first.

**Backup before an overwrite of a user preset** (decision 7's guardrail, made concrete). A QC-2 draft can't restore a user preset because it's lossy, so it is not a backup. The backup is the unit's own copy, in a **dedicated backup collection** the owner creates on the unit (D3):

- **Config.** `QC_AGENT_BACKUP_COLLECTION` in `agent.env` holds the collection's exact name. The agent resolves it from a fresh listing by exact name. It must:
  - be listed exactly once, as a regular 256-slot collection;
  - be a different collection from the one being edited.

  Otherwise an `overwrite` is refused before any write.
- **Order.** Before patching, the agent saves the **unmodified** live preset into the backup collection's first empty slot and checks that the listing shows it there. Only then does it patch and overwrite the original.
  - The backup's name is the original's, cut to fit the name rules. The collection itself records that it is a backup.
  - A full backup collection refuses the overwrite ("backup collection full: clear some backups on the unit").
- **Reserved.** The backup collection is never a push or edit target: `push_preset` and `edit_preset` refuse a slot in it. The agent never deletes from it; clearing old backups is the owner's job on the unit.
- **Recorded.** The job result and the `edited` ledger name the backup slot, so `get_device_job` can say where the original went.
- **Unit premise.** Saving into a collection that is **not the open one** has never been done: every push so far saved into the open collection (bridge plan 5, "As built in P4"). **UNVERIFIED** (E-G5).

Recommended in addition: a Cortex Control backup before the first overwrite ever.

## 6. Agent rules for `edit_preset` (the new safety model)

Replaces rules 3-4 of a push *for this kind only*; `push_preset` is unchanged.

1. **PC-side opt-in.** `QC_AGENT_EDIT_MODES` in `agent.env` (default empty, so edits are refused) lists the allowed modes, e.g. `live,copy`. `overwrite` of a slot the agent doesn't own additionally needs `QC_AGENT_OVERWRITE_USER=1`. `QC_AGENT_PROTECTED_SLOTS` lists banks never edited. `QC_AGENT_BACKUP_COLLECTION` names the backup collection (section 5), which is never edited or pushed to. **Why on the PC:** a leaked `DEVICE_JOB_KEY` or a compromised QC-2 can then reach at most what the owner switched on locally. The signed job alone is not enough authority to change a preset the owner made.
2. Signature, expiry, firmware and allowed-slot checks as today.
3. **Open slot only.** The slot in the job must be the open one, in a regular 256-slot collection. The agent never recalls a preset for an edit.
4. **Compare-and-swap before the first write.** A fresh read of the live preset must match `expect` exactly: name, position, folder and the full layout (all rows). Every op's `expect` values must also match. Any difference refuses the job: "the preset changed since it was read; read it again".
5. **Wire guard armed per op.** The guard allows only the planned `(row, col, index)` Grid UPDATEs with the planned values, and the planned bypass writes. Nothing else passes: no add/delete, no strings, no stomps, and no File CREATE unless the mode needs it (then exactly one, at exactly the planned position and name).
6. **Verify and collateral check.** After the ops, a full read:
   - every planned value must read back as written;
   - every parameter and bypass flag **not** in the plan must equal the pre-write read, on all rows (`untouched`).
7. **Automatic rollback on failure.** If step 6 fails, the agent writes back the pre-write values for the ops it applied (all within the same armed plan), re-verifies, and reports `readback_mismatch` with the rollback outcome. Nothing is saved.
8. **Save per mode** (section 5), with the same pre-save checks a push has: fresh listing, grid unchanged since verify, the same slot still open, then polling the listing (G3 lag, F1 short-table handling).
9. **Ledgers.** An `edited` line per job (slot, name, before/after layout hash, job id), separate from `owned`. `copy` records the new slot as owned. `overwrite` of a user slot never makes it "owned".

## 7. Unit checks (normal use, no special session; decision 8)

Each mode ships only after its first real use passes. Do each first on a **QC-2-owned test slot**, then on a user preset.

| Gate | Question | Taken during |
|---|---|---|
| **E-G1** | On a loaded, saved preset: do `set_param`/`set_param_scenes`/bypass writes read back exactly, set `PresetDirty`, touch nothing else, and get discarded by re-selecting the slot? | first `live` edit (E4) |
| **E-G2** | Does File CREATE at the open occupied position with the same name replace in place? Check a raw read-back before and after (all rows, strings, bypass, stomps), the listing (same position, no duplicate), and a power cycle. | first `overwrite` of an owned slot (E5) |
| **E-G3** | Does File CREATE at another, empty position save the edited working grid there, completely? Which slot is open afterwards? | first `copy` (E5) |
| **E-G4** | Does `delete_block` + `add_block` on a loaded preset swap a model cleanly? Do the cell's stomp and expression assignments survive? | first model swap (E6) |
| **E-G5** | Does File CREATE into a collection other than the open one (the backup collection) save the live grid there, completely, without changing what's open? | first `overwrite` of a user preset (E5); on a test preset first |

**What the raw read-back can't see:** scene names, expression assignments and MIDI settings are not in `qc-device-readback/1`. E-G2 and E-G3 are only checked for them by looking at the unit's screen. That is labelled in the results, not assumed.

## 8. Order of work

Each step is its own PR, merged before the next one that depends on it. E1-E3 need no device.

| Step | Scope | Device? | Risk | Depends on |
|---|---|---|---|---|
| **E0** | This document, plus the owner decisions in section 10 | no | — | — |
| **E1** | Snapshot storage (3.1); `internal/deviceedit` (3.2) with `set_param`/`set_bypass` ops, block matching, not-applied reasons and plan hash; a dry-run renderer; `edit_device_preset` **dry run only** (the confirm path returns "not available yet"). Goldens from `testdata/devicejobs/readback_roundtrip/*.json` and the eval presets: unchanged target → zero ops; scene C-H preservation; every not-applied reason. A census: for each round-trip fixture plus a few hand-edited targets, how many changes become ops. | no | medium (mapping rules) | E0 |
| **E2** | Pipeline: the edit-aware Architect context for device drafts (3.3), plus the "review" entry point; a byte-identical-context test for non-device presets; mock replay; then the **A/B eval in 8.1** (pilot, then the full run, each with the owner's go). The prompt section becomes production only if the eval passes. | no | medium (prompt; paid eval) | E1 (its plan builder is the eval's scorer); the owner's corpus reads (8.1) |
| **E3** | Contract **v2**: job kind `edit_preset`, its args (slot, mode, `target_slot`, name, the edit payload), result fields (per-op read-back, collateral check, rollback, saved). New payload `v` and domain tags per contract section 1; v1 kinds unchanged. Go reference, vectors, Python parse. Contract-only PR, like P3's first. | no | **high** (contract/auth) | E0 decisions |
| **E4** | Agent: `edit_preset` in **`live` mode only**. PC opt-in, CAS, per-op wire guard, collateral check, rollback, `edited` ledger. QC-2's confirm path is enabled for `live`. Offline tests on the fake backend: every refusal, a hostile plan (an op outside the armed set, a CAS mismatch, a collateral change, a rollback). Then E-G1 in normal use. | build no; acceptance **yes** | **high** (writes to presets the owner made, unsaved) | E1, E3 |
| **E5** | Save modes: `copy` (E-G3) first, then `overwrite` of owned slots (E-G2), then `overwrite` of user slots with the backup copy (only if D2 says yes). Plus `undo_device_edit`. | yes | **highest** (overwrites stored presets) | E4 |
| **E6** | Structural edits: add block into a free cell, capture swap, model swap (E-G4), real delete (if wanted); scenes C-H in the import; `set_preset_values` (3.4); read by slot (recall; decision D5). | mixed | medium-high | E5 |

**First useful milestone: E1 + E2 + E3 + E4.** "Read my open preset, review it for X, show me the changes, apply them live." The owner hears the result and saves on the unit by hand. No stored preset can be lost at that stage.

### 8.1 The E2 eval (owner, 2026-11-07: changing the pipeline gets an eval)

**Question.** On presets read from the unit, does the edit-aware refine produce suggestions the bridge can apply, without losing tone quality, compared with today's refine?

**Corpus.** About 8 imported drafts:
- the round-trip fixtures (`testdata/devicejobs/readback_roundtrip/`), which are QC-2-shaped;
- **4-5 of the owner's own presets**, read with `read_device_preset(import=true)`. This is the one owner step, and it is read-only. Choose some with captures, some with blocks or rows the import leaves out, and at least one factory-derived preset.

**Requests.** 4 per draft:
- a tone tweak ("less fizz on the lead");
- a scene request ("more gain and level in scene B");
- an open review ("review this for a cleaner blues rhythm");
- one that tempts a structural change ("make it sound like a Plexi"), to see whether a swap is proposed and correctly comes out as not applied.

**Arms and size.**
- **A:** today's refine context. **B:** the edit-aware context.
- 2 repetitions each: about 8 × 4 × 2 × 2 = **128 refine runs**.
- **Pilot first:** 2 drafts × 4 requests × 2 arms × 1 rep = 16 runs. Its cost is read with `cmd/usage_report`/`cmd/eval_cost` and shown to the owner before the full run. The full run's cost isn't estimated here, because the cost per refine run is not measured for this prompt (**UNVERIFIED**).

**Harness.** A new `cmd/eval_device_refine`, modelled on `cmd/eval_refine` (refine calls) and `cmd/eval_device_checks` (re-scoring saved outputs with no Gemini calls). Outputs are saved, so scoring can be rerun without new runs.

**Metrics.** Deterministic first; every one is computed by E1's plan builder against the draft's snapshot:

| Metric | What it measures | Pass for arm B |
|---|---|---|
| Applicable share | ops ÷ (ops + not-applied changes) | clearly above A (target ≥ 80%) |
| Block preservation | blocks kept with the same `id`, model and order | ≥ 95% |
| Locked violations | changes aimed at controls or blocks the import left out | 0 |
| Unasked structural changes | swaps, adds or removes the request didn't ask for | below A |
| Mechanical checks | `RunMechanicalQualityChecks` counts (as `cmd/eval_device_checks` scores them) | no worse than A |
| Request addressed | an LLM judge (`cmd/judge`), blind to the arm: did the change address the request, and is the tone reasonable? | no worse than A |

**Reading the result.** The comparison is valid only over the (draft, request, rep) triples that parsed in **both** arms, the same rule `cmd/eval_device_checks` uses. `opus-verifier` checks the comparison before the prompt section is switched on (a measurement across runs is a verifier trigger).

**Follow-up.** The owner's first real reviews after release are read the same way, from the dry runs.

## 9. Which model does what

Follows `.claude/rules/claude-config/agent-routing.md` and CLAUDE.md's cost policy (bots are the default PR gate; `/quick-review` on demand).

| Work | Who | Why |
|---|---|---|
| E1 matching and no-op rules (block alignment, scene C-H, wire tolerance) | `planner` (**Opus**, high) before coding | Same class as P2's mapping: easy to get subtly wrong, and every edit depends on it. |
| E1 implementation | **Sonnet 5.5**, main session | Well-specified Go on top of `deviceexport`'s tested tables, golden-tested. |
| E2 prompt section and eval harness | **Sonnet 5.5**; the owner approves the pilot and then the full run (Gemini spend) | Prompt plumbing follows the existing origin/prompt gating; the harness reuses `cmd/eval_refine` and `cmd/eval_device_checks`. |
| E2 eval reading | `opus-verifier` (**Opus**) | Comparing measurements across runs is a verifier trigger. |
| E3 contract v2 | **Opus 5.5**, main session; `security-reviewer` (**Opus**) and `/security-review`; GSR `deep-review` label | Auth/contract class (CLAUDE.md "large/risky"). Both sides must agree byte for byte. |
| E4 agent `live` mode | **Opus 5.5**, main session; `planner` first on the job state machine (CAS → arm → apply → verify → rollback); `security-reviewer`; `/code-review high` (effort stated) since it writes to the owner's own presets | The highest-consequence code in the project so far. The stronger model pays for itself in fewer review rounds. |
| E5 save modes and undo | **Opus 5.5**; `opus-verifier` on each gate result **before** the mode is enabled (load-bearing device premises) | A wrong reading of E-G2 could cost the owner a preset. |
| E6 structural edits | **Sonnet 5.5** for add-block and capture swap (existing patterns); **Opus 5.5** for model swap and delete | Swap and delete are the risky ones. |
| Running tests after each change | `test-runner` (**Haiku**) | Cheap; keeps logs out of context. |
| Lookups across the repo | `scout` (**Haiku**) | Cheap. |
| PR comment triage | `pr-triage` (**Sonnet**, low), ledger after round 2 | Standing rule. |
| Fable | not used unless the owner asks | Ask-only. |

## 10. Decisions for the owner

**Decided (owner, 2026-11-07)**

| # | Decision | Answer |
|---|---|---|
| D1 | Default save mode for a preset QC-2 didn't create | **`copy`.** `live` is available for trying an edit; `overwrite` needs D2. |
| D2 | Is `overwrite` of a user (non-owned) preset ever allowed? | **Yes, but later:** after E-G2 passes on owned slots, with a unit-side backup copy first and `QC_AGENT_OVERWRITE_USER=1` on the PC. |
| D4 | Is "remove a block" a bypass in all scenes, or a real delete? | **Bypass** in all eight scenes (reversible). Delete waits for E6. |
| D6 | May the pipeline swap models (amp, cab) in a device edit? | **In E6.** Until then a swap is "not applied", with "push as a new preset instead" offered (the existing rebuild path into an empty slot). |
| D3 | Where backups go (D2) | **A dedicated backup collection** the owner creates on the unit (`QC_AGENT_BACKUP_COLLECTION`), never pushed to or edited (section 5). Saving into a collection other than the open one is E-G5. |
| — | Eval for this work? | **Yes, for E2**, because it changes the pipeline: the A/B eval in 8.1, pilot first, owner's go before each paid run. Pushing and the agent (E1, E3-E6) don't change the pipeline. |

**Still open (recommendation shown; the step that needs it in brackets)**

| # | Decision | Recommendation |
|---|---|---|
| D5 | May an edit read **recall** a slot, changing what's playing? (E6) | **No in v1:** select the preset on the unit, as for a push. |
| D7 | Scope of the edit (E1) | Row 0 for v1 (all the import covers); other rows untouched and verified unchanged. |
| D8 | The first use of each mode (E4, E5) | On a QC-2-owned test slot, in normal use (consistent with decision 8). |

## 11. Kickoff prompts

**Chat E1: edit plan and dry run** (`planner` on Opus first, then Sonnet 5.5)

> Read `docs/design/qc-preset-editing.md` (sections 2-4 and 8) and `docs/design/qc-device-bridge-plan.md` sections 4 and 7.1 (P5 status), then `internal/deviceexport` (export, resolve, readback) and `internal/mcpserver/devicetools.go`. Branch from main as `claude/device-edit-plan`. Run the `planner` agent on section 3.2's matching and no-op rules first and show me its plan. Then build E1: store the raw read-back snapshot on every parsed `read_preset` result; add `internal/deviceedit` (`set_param`/`set_bypass` ops with compare-and-swap `expect` values, scenes C-H kept from the snapshot, no-ops dropped, every refusal a `not_applied` item); add `edit_device_preset` as a dry run only. Goldens and a census as section 8 says. Show `go test ./...` and the e2e results via `test-runner`. Open the PR, subscribe, address GSR's findings.

**Chat E3: contract v2** (Opus 5.5; can run alongside E1)

> Read `docs/design/qc-preset-editing.md` sections 3.2, 5, 6 and 8 and `docs/design/device-job-contract.md`. Branch from main as `claude/device-contract-v2`. Add the `edit_preset` job kind as contract v2 (new payload `v` and domain tags; v1 kinds unchanged), its args and result fields, Go reference in `internal/devicejobs`, regenerated vectors and the Python parse in `qc_agent/contract.py` and `reference_check.py`. No device behaviour yet: a v2 agent answers `edit_preset` with `refused`/`unsupported_kind` until E4. Run `/security-review` and the `security-reviewer` agent; open the PR with the `deep-review` label.

**Chat E2: edit-aware refine** (Sonnet 5.5; after E1)

> Read `docs/design/qc-preset-editing.md` section 3.3 and the E1 code. Branch from main as `claude/device-edit-refine`. Add the edit-aware Architect context for drafts with `Origin.Kind == device-readback`, gated on the origin so other presets' prompts are byte-identical (add a test for that), and the "review" entry point. Test with mock replay, then build `cmd/eval_device_refine` per section 8.1. Ask me before the pilot and again before the full run; get `opus-verifier` to check the comparison before the prompt section becomes production.

**Chat E4: agent live edits** (Opus 5.5; after E1 and E3 merge)

> Read `docs/design/qc-preset-editing.md` sections 5-7, the contract v2 section and `deploy/qc-agent/qc_agent/jobs.py`/`qcmcp_backend.py`. Branch from main as `claude/qc-agent-edit-live`. Run the `planner` agent on the job state machine first. Build `edit_preset` in `live` mode only, following section 6 (PC opt-in, CAS, per-op wire guard, collateral check, rollback, `edited` ledger), offline tests on the fake backend for every refusal and a hostile plan, and enable QC-2's confirm path for `live`. Run `/security-review`, the `security-reviewer` agent and `/code-review high`; open the PR with `deep-review`. Then give me the README steps for E-G1 on an owned test slot.

**After that:** E5 (Opus 5.5, one PR per mode, each enabled only after its gate passes and `opus-verifier` has read the result), then E6.
