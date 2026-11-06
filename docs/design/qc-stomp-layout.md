# Stomp layout: consistent footswitch assignments on the Quad Cortex

*Design, 2026-11-06. Status: S1 and S2 built and merged (no device involved); S5's entry-point-1 half built (the agent assigns stomps on a push, branch `claude/qc-agent-stomps-on-push`); S3 (gate G7) no longer gates entry point 1, only entry point 2 (owner decision, section 9). See sections 7 and 8.*
*Builds on `docs/design/qc-device-bridge-plan.md` (the bridge, its phases and gates) and `docs/design/device-job-contract.md` (jobs between QC-2 and the home agent).*

**Verify-or-label.** Each premise about the device says where it comes from: **owner** (the owner's statement about their unit), **qc-mcp** (read in qc-mcp's source at the pinned commit `f634ecd`, not run on the unit), or **UNVERIFIED**. Gate G7 (section 7) settles them; per the owner's decision it is required only before entry point 2, and entry point 1 ships with these premises still UNVERIFIED until the owner's first push shows them on the unit.

---

## 1. Goal

Every preset on the unit puts the same kind of effect on the same footswitch, so a boost is always on B, whatever the preset.

- It works on **any preset**: one QC-2 pushes, one the owner built by hand, or one from Cortex Cloud. It is **not** part of the generation pipeline. No agent and no LLM decides a switch.
- It is deterministic: the same blocks always give the same layout.
- It never takes away the owner's own switch: H stays free for manual use.

## 2. Device facts this rests on

| Fact | Source |
|---|---|
| The owner plays in **Scene** or **Stomp** mode, never Hybrid. | owner |
| Footswitches **A–D are the top row, E–H the bottom row.** | owner; qc-mcp's mode names agree ("Scene top A-D / Stomp bottom E-H", `server.py:1112`) |
| Stomp assignments are **stored per preset**. | owner |
| **One switch can control several blocks.** | owner |
| A block holds **one** primary assignment; assigning it again **moves** it. | qc-mcp (`server.py:2048`) — UNVERIFIED on the unit |
| `assign_stomp(row, column, stomp_index)` with index **0–7 = A–H**, sent as a Grid UPDATE carrying only `stomp_mode_assignments`. `unassign_stomp` is the matching Grid DELETE. | qc-mcp (`transport.py:1172-1220`; the agent's vendored copy has the same lines) — UNVERIFIED on the unit; the agent's push uses `assign_stomp` only (§6.1) |
| A full preset read includes `stomp_assignments` (row, column, switch, momentary). | qc-mcp (`server.py:317-324`, `preset.py:70`) — UNVERIFIED on the unit |
| Assignments only matter in Stomp mode; in Scene mode the switches pick scenes and the assignments sit unused. | UNVERIFIED (it's how the modes are described, not something measured here) |
| The grid is 4 rows × 8 columns. | G6 read-back (bridge plan §7.1) |

Switching between Scene and Stomp mode is a **global** setting (`switch_mode`, `set_mode_cycle`). The bridge never writes it. The owner picks the mode on the unit.

## 3. The layout

```
 Top row:     A: Comp     B: Boost    C: Drive    D: Drive 2
 Bottom row:  E: Delay    F: Mod      G: Reverb   H: free (manual)
```

Owner's layout, 2026-11-06. It lives in one config file, `stomp_layout.json`, so it can change without a code change:

```json
{"version": 1,
 "switches": {"A": "comp", "B": "boost", "C": "drive", "D": "drive2",
              "E": "delay", "F": "mod",   "G": "reverb"},
 "reserved": ["H"]}
```

The loader refuses: a switch outside A–H; a role assigned to two switches; a reserved switch that also has a role; and an unknown role.

## 4. Roles: how a block is classified

A block is classified from its **catalog model id** (`internal/catalog`, `id` → `category`, `name`), never from a name the device sends back. Device-sourced text is untrusted (contract §11).

| Role | Rule |
|---|---|
| `comp` | category `Compressor` |
| `boost` | category `Guitar Overdrive` **and** the model name contains "boost" (any case). That matches Rage Booster, Exotic Z Boost, John Mayer Justa Boost and Tim Henson Boost in today's catalog. |
| `drive` | any other `Guitar Overdrive` block |
| `delay` | category `Delay` |
| `mod` | category `Modulation` |
| `reverb` | category `Reverb` |
| *(none)* | everything else: amps, cabs, IR loaders, EQ, utility and gates, wah, pitch, filter, morph, synth, looper, FX loops, splitters, mixers, bass blocks |
| *(see 4.1)* | Neural Captures |

**The boost rule is limited to drive blocks on purpose.** Without that limit it would also catch the amps "UK C30 TopBoost" and "UK C15 TopBoost". The rule is the owner's choice (2026-11-06): simple and predictable. The known cost: a Tube Screamer set up as a boost is still a Drive.

### 4.1 Neural Captures

The catalog can't say whether a capture is an amp or a pedal: every capture is the same Neural Capture block, and only its loaded file differs. Per qc-mcp, a capture block carries its capture as a parameter string, the capture's 64-hex key followed by its display name (`transport.py:942`). UNVERIFIED on the unit; G7 (h) checks it. A capture's role is decided in this order:

1. **The modeler's category, for a block in a QC-2 preset.** Owner's decision, 2026-11-06. The block's `type` is a free-text string the Architect writes; its schema has no fixed list (`internal/agents/schemas.go`). Across the saved eval runs it appears in 23 spellings, among them `Drive`, `Overdrive`, `Overdrive (Capture)`, `Neural Capture (Drive)`, `Boost`, `Fuzz`, `Preamp`, `Capture` and `Neural Capture`. These runs span several prompt versions, so the counts describe what the field can hold, not what production v6 writes. So the label goes through a fixed table, never through string matching on the fly:

   | `type` (case-insensitive, trimmed) | Role |
   |---|---|
   | `Boost` | `boost` |
   | `Drive`, `Overdrive`, `Distortion`, `Fuzz`, `Overdrive (Capture)`, `Overdrive Capture`, `Neural Capture (Drive)` | `drive`, or `boost` if the capture's name contains "boost" (the same name rule as §4) |
   | `Compressor` | `comp` |
   | `Delay` | `delay` |
   | `Modulation` | `mod` |
   | `Reverb` | `reverb` |
   | `Amplifier`, `Cabinet`, `Preamp` | no switch, decided (the step stops here) |
   | `Capture`, `Neural Capture`, anything not in this table | undecided: go to step 2 |

2. **The capture library.** A capture whose name exactly matches an entry in `internal/agents/user_captures.json` takes that entry's `block_type`. All 87 entries have one, and all are citation-checked (`description_verified: true`). `drive` → `drive`, or `boost` by the name rule (today that gives "Strymon Sunset BOOST" and "TR KOTyellow Boost SC"). `amp`, `full_rig`, `cab` and `other` → no switch. This is the only step for a preset this system didn't push (entry point 2, §6.1).
3. **Otherwise no switch.** The dry run lists the capture as "unclassified capture", with a pointer to add it to `user_captures.json`. The tool never guesses a type from a capture's name, because names are often cryptic and words like "Boost", "Drive" or "Lead" often name an amp channel. It would also break CLAUDE.md's rule against hand-guessed capture metadata.

**When steps 1 and 2 disagree** (the modeler says `Amplifier`, and the library says `drive`), step 1 wins, per the owner's decision. The dry run shows the disagreement so it can be fixed at the source.

**A known weakness of step 1.** The Architect doesn't always label the same capture the same way. In the eval runs, "EP3 pre (V-12 EQ-mid)" appears as `Drive`, `Overdrive`, `Boost`, `Preamp`, `Capture` and `Neural Capture` in different presets, so its switch would vary between presets. That works against the goal of consistency. Fixing it belongs in generation (for example, deriving a capture block's `type` from the library when there's an entry), not here, and is out of scope.

**Factory captures are not used.** `coros_map.json`'s `type` for factory captures is too coarse: 117 of its 179 captures are `fx`, which mixes drives, fuzzes and compressors (TS9, HM-2 and OC-76 Comp are all `fx`). They can join step 2 once they carry a finer, cited type.

**Timing.** In v1 the device export marks every capture `unresolved` (bridge plan §4), so a QC-2 preset with a capture can't be pushed yet. Step 1 matters from the release where captures can be pushed. Step 2 applies to entry point 2 as soon as that ships.

## 5. Assignment rules

1. **Signal order.** Blocks are ordered row by row, then column by column (row 0 first). This is the order used for "first drive" and "second drive".
2. **One switch per role, several blocks if needed.** Every block with a role goes on that role's switch. Two delays both go on E; two reverbs both go on G.
3. **Drives.** The first drive goes on C, the second on D, and any later drive is ganged on D with the second.
4. **Empty stays empty.** If a preset has no mod block, F gets nothing. Nothing shifts over to fill a gap, so muscle memory holds.
5. **H is never touched.** The tool doesn't assign anything to H and doesn't remove anything from it. A block the owner already put on H stays there, even if its role has its own switch: re-assigning it would move it off H.
6. **The layout replaces A–G.** On a preset that already has assignments, the tool removes any A–G assignment the layout doesn't call for and adds the ones it does. The dry run lists each removal by name. (An assignment that already matches is left alone, so re-running on a laid-out preset changes nothing.)
7. **Latching only.** The tool never sends the `momentary` flag and never writes **secondary** (Dual Footswitch) assignments. If a layout switch reads back as momentary, the dry run warns instead of changing it.
8. **The result is a plan, not device calls.** The layout function returns `[{row, col, switch}]` to add and to remove, plus what was skipped and why. The plan is shown before anything is written.

## 6. Where it runs

### 6.1 One Go package, two entry points

`internal/stomplayout`: a pure function from a list of blocks `{row, col, model_id}` and the current assignments, to a plan. It reads the catalog and `stomp_layout.json`. It has no storage, device or LLM dependency. The rules exist only here; the home agent never re-implements them. The agent only checks and executes an explicit list of assignments.

**Entry point 1: pushing a QC-2 preset** (`send_preset_to_device`).
- The device export (bridge plan §4, P2) gains a `stomps` array: `[{"row": 0, "col": 2, "switch": "C"}]`.
- The dry run shows the layout next to the blocks and values.
- The agent assigns stomps after the blocks and before the save, so they're saved with the preset. That's fine, because the slot is one this system creates.
- **As built (S5, entry point 1).** After the values and bypass flags read back clean, `jobs._Push.assign_stomps` (`deploy/qc-agent/qc_agent/jobs.py`) assigns each stomp with the vendored `QuadCortex.assign_stomp` (`deploy/qc-agent/vendor/qc_mcp/transport.py:1172`, unedited): kind `primary`, `momentary` left unset (latching, no `stomp_is_momentary` message), index A-G = 0-6 (`export_validator.STOMP_INDEX`; H = 7 is never planned or allowed). Each assignment goes through the same open-slot and liveness checks as every other write. One read then compares the unit's `stomp_mode_assignments` and `stomp_is_momentary` with the plan (exactly the planned set; nothing missing, extra, secondary or momentary) and re-diffs every value and bypass flag; any difference fails the job before the save, and the pre-save re-read checks the assignments again. `assign_stomp` stays on the agent's never-allowed method list: `QcMcpBackend.assign_stomp` opens it for one call at a time (`Guarded.window`), and the wire guard passes only the armed assignments (row 0, a block this push placed and a read of the unit confirmed, one per message, each once) and, once the stomp step begins, no further value, scene or routing write. `unassign_stomp` is never reachable. No contract change: the `stomps` array was already in the export. Everything about which physical switch an index is stays UNVERIFIED until the owner's first push with stomps (README step 15).

**Entry point 2: laying out the preset that's loaded on the unit** (any preset).
- **Owner's decision (2026-11-06): apply the layout but don't save.** If the owner likes the result, they save it on the unit. If not, they leave the preset or reload it and the change is gone. This respects the bridge's rule against overwriting slots this system didn't create.
- New MCP tool `apply_stomp_layout`, dry run by default, with the same confirm-token pattern as `send_preset_to_device`:
  1. The dry run queues a `read_preset` job (current preset, already in v1). From the read-back, QC-2 builds the plan and shows it: preset name, each switch, its blocks, every removal, and every block skipped and why. QC-2 also computes a **fingerprint** of what it read: the preset name, each block's (row, column, model id), and the current assignments.
  2. The confirm call queues an `apply_stomps` job carrying the plan and the fingerprint.
  3. The agent re-reads the current preset. If the fingerprint doesn't match, it refuses (`readback_mismatch`), because the owner may have changed preset since the dry run. Otherwise it removes, then assigns, then reads back and reports any assignment that didn't come back as planned.
  4. **The agent never saves in this job.** No `write_preset_file` and no save of any kind.

### 6.2 Contract change

Field names follow the existing export format and the result `diff` (bridge plan §4, contract §9): `row` and `col`, never `column`.

`apply_stomps` is a new job `kind`. The v1 agent's strict parser rejects unknown kinds, so this is a contract change for both sides (`internal/devicejobs` and the Python agent), with new vectors in `testdata/devicejobs/vectors.json`. Proposed args:

```json
{"expect": {"fingerprint": "<sha256 hex>"},
 "remove": [{"row": 0, "col": 3, "switch": "D"}],
 "assign": [{"row": 0, "col": 1, "switch": "B"}]}
```

The agent's own checks: switch in `A`–`G` (never `H`, never index ≥ 8); row 0–3 and column 0–7; a block present at each cell in the fresh read; at most 32 entries in each list. The result reuses `push`'s diff shape, with the switch in place of `param_index`. The exact result fields are settled in the contract PR.

The `stomps` array inside the push export needs no contract change: `export.payload` is opaque to the contract, and its schema belongs to P2.

## 7. Gate G7: stomps on the unit

**What G7 gates (owner decision, 2026-11-06).** Entry point 1 (pushing a QC-2 preset into a slot this system creates) does not wait for G7. Its premises (a)-(e), (g) and (h) stay **UNVERIFIED** and are first checked by the owner looking at the unit after the first real push: is B where the layout says, does it toggle. The agent's read-back after assigning (S4 step 3) catches an assignment that did not come back as written, but cannot show which physical switch an index is. **Entry point 2 (`apply_stomp_layout` on any loaded preset) stays off until (f) has been run once**, because (f) is the only question whose failure could rewrite a preset the system did not create.

Run on a scratch preset in an **empty** slot (as G3 did), with a spike that is granted `assign_stomp`, `unassign_stomp` and `get_current_preset` only. Stomps are on the spike harness's never-allowed list today (`tools/device-spikes/spike_common.py`).

| # | Question | How |
|---|---|---|
| a | Does index 1 light up the switch labelled **B** on the top row, and index 4 the **E** switch on the bottom row? | Assign, put the unit in Stomp mode, press the switch, watch the block. |
| b | Do two blocks assigned to the same index both toggle from one press? | Assign two drives to D. |
| c | Does assigning a block a second time move it (qc-mcp's claim), not add a second switch? | Assign a drive to C, then to D; read back. |
| d | Does the read-back's `stomp_assignments` match what was written? | Compare after each step. |
| e | Do assignments survive `write_preset_file` and a recall of that slot? | Save into the empty slot, recall another preset, recall it again. |
| f | **For entry point 2:** does an assignment change mark the preset as edited but unsaved, and does leaving the preset discard it without auto-saving? | Assign on a preset the system created, switch presets, come back, read back. |
| g | **Scenes.** Which scene is active when a preset loads in Stomp mode? Does pressing a stomp change that scene's stored bypass flag? **Partly answered (owner, on the unit, 2026-11-06):** a block's stomp footswitch acts across all scenes, not per scene, also on a block whose bypass the bridge writes per scene (PR #150 writes all eight flags on every block). Still open: which scene loads, and what the read-back's per-scene flags show after a press. | Read the `Scene` state and the bypass flags before and after a press. |
| h | **Captures.** Does a capture block's read-back carry the key and display name as qc-mcp says? Does that display name equal the `name` in `user_captures.json` exactly? Is the key stable for a given capture? | Read back a preset holding one downloaded capture that's in `user_captures.json`. |

**If (f) fails** (the unit auto-saves, or keeps the change), entry point 2 is unsafe as designed and stays off until it's redesigned. **If (g) shows that stomps rewrite scene bypass,** add a note to the dry run explaining the effect on scenes. The design doesn't depend on (g) otherwise. *Done after the owner's observation (2026-11-06):* the dry run's footswitch layout says a switch acts in every scene, and warns for each block on a switch whose Scene A and B bypass differ (`renderStomps`, `internal/mcpserver/devicetools.go`).

## 8. Phasing

| Step | Scope | Device? | Depends on | Status (2026-11-06) |
|---|---|---|---|---|
| S1 | `internal/stomplayout` + `stomp_layout.json` + tests (classification, every rule in §5, golden plans for the eval presets and for synthetic grids with existing assignments) | no | P1 (done) | **Done**, PR #138 |
| S2 | `stomps` in the device export, shown in the `send_preset_to_device` dry run | no | P2, S1 | **Done**, PR #139 |
| S3 | G7 spike; required only for entry point 2 (f). The other questions are checked by eye on the first real push | **yes** (owner runs) | P0-a harness | Not started; does not block entry point 1 |
| S4 | `apply_stomps` job kind: contract, vectors, Go side, `apply_stomp_layout` MCP tool | no (fake agent) | P3, S1; S3 (f) for the entry-point-2 tool only | Not started. The `apply_stomps` job and `apply_stomp_layout` tool wait on (f); the push path (entry point 1) does not |
| S5 | Agent side of S2 and S4 | build no; acceptance **yes** | P4, S4; S3 (f) for entry point 2 | **Entry point 1 built** (branch `claude/qc-agent-stomps-on-push`, 2026-11-06): a push assigns the export's `stomps` after the values read back clean and before the save, reads them back and fails on any mismatch (§6.1, "As built"). Tested offline only; acceptance is README step 15 (by eye on the unit). Entry point 2 (`apply_stomps` on a loaded preset) not started: waits on S3 (f) |

S1 and S2 touch no device, no auth and no storage. Everything the code does with switch indexes and assignment behaviour stays UNVERIFIED until it is seen on the unit. Entry point 1 may proceed on that basis; nothing that applies stomps to a preset this system did not create may be built before (f) has run.

## 9. Decided

All by the owner, 2026-11-06.

- Scene or Stomp mode only, no Hybrid.
- Layout A Comp, B Boost, C Drive, D Drive 2, E Delay, F Mod, G Reverb, H free.
- Boost = a drive block with "boost" in the model name.
- Separate from the generation pipeline; works on any preset.
- On presets this system didn't create: apply without saving (option 2).
- Several blocks of one role share that role's switch (D for drives after the first).
- A capture in a QC-2 preset takes the modeler's category (§4.1 step 1).
- G7 gates only entry point 2, via (f). Entry point 1 ships with its device premises labelled UNVERIFIED; the owner checks the unit by eye after the first real push.

## 10. Defaults chosen here (change any of them)

- `mod` is the `Modulation` category only. Filter, pitch, wah, morph and synth get no switch.
- A capture whose modeler category is generic or missing falls back to `user_captures.json`. A capture in neither gets no switch and is listed in the dry run (§4.1 steps 2–3). Factory captures aren't classified until `coros_map.json` has a finer type for them.
- The tool replaces existing A–G assignments, and never touches H (§5 rules 5–6).
- Latching only, no secondary assignments (§5 rule 7).
- No change to the Architect or the preset view in v1. A "Switch" column in the preset view is a possible follow-up; it would change `RenderPresetMarkdown`'s golden file and the display guide.
