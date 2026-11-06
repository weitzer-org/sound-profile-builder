# Stomp layout: consistent footswitch assignments on the Quad Cortex

*Design, 2026-11-06. Status: proposed, not started.*
*Builds on `docs/design/qc-device-bridge-plan.md` (the bridge, its phases and gates) and `docs/design/device-job-contract.md` (jobs between QC-2 and the home agent).*

**Verify-or-label.** Each premise about the device says where it comes from: **owner** (the owner's statement about their unit), **qc-mcp** (read in qc-mcp's source at the pinned commit `f634ecd`, not run on the unit), or **UNVERIFIED**. Gate G7 (section 7) settles the ones the design depends on.

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
| `assign_stomp(row, column, stomp_index)` with index **0–7 = A–H**, sent as a Grid UPDATE carrying only `stomp_mode_assignments`. `unassign_stomp` is the matching Grid DELETE. | qc-mcp (`transport.py:1172-1220`) — UNVERIFIED on the unit |
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
| *(none)* | everything else: amps, cabs, IR loaders, EQ, utility and gates, wah, pitch, filter, morph, synth, looper, FX loops, splitters, mixers, bass blocks, Neural Captures |

**The boost rule is limited to drive blocks on purpose.** Without that limit it would also catch the amps "UK C30 TopBoost" and "UK C15 TopBoost". The rule is the owner's choice (2026-11-06): simple and predictable. The known cost: a Tube Screamer set up as a boost is still a Drive.

**Neural Captures get no role.** The catalog can't say whether a capture is an amp or a pedal, and a capture's name is device text. A captured boost pedal therefore gets no switch in v1.

## 5. Assignment rules

1. **Signal order.** Blocks are ordered row by row, then column by column (row 0 first). This is the order used for "first drive" and "second drive".
2. **One switch per role, several blocks if needed.** Every block with a role goes on that role's switch. Two delays both go on E; two reverbs both go on G.
3. **Drives.** The first drive goes on C, the second on D, and any later drive is ganged on D with the second.
4. **Empty stays empty.** If a preset has no mod block, F gets nothing. Nothing shifts over to fill a gap, so muscle memory holds.
5. **H is never touched.** The tool doesn't assign anything to H and doesn't remove anything from it. A block the owner already put on H stays there, even if its role has its own switch: re-assigning it would move it off H.
6. **The layout replaces A–G.** On a preset that already has assignments, the tool removes any A–G assignment the layout doesn't call for and adds the ones it does. The dry run lists each removal by name. (An assignment that already matches is left alone, so re-running on a laid-out preset changes nothing.)
7. **Latching only.** The tool never sends the `momentary` flag and never writes **secondary** (Dual Footswitch) assignments. If a layout switch reads back as momentary, the dry run warns instead of changing it.
8. **The result is a plan, not device calls.** The layout function returns `[{row, column, switch}]` to add and to remove, plus what was skipped and why. The plan is shown before anything is written.

## 6. Where it runs

### 6.1 One Go package, two entry points

`internal/stomplayout`: a pure function from a list of blocks `{row, column, model_id}` and the current assignments, to a plan. It reads the catalog and `stomp_layout.json`. It has no storage, device or LLM dependency. The rules exist only here; the home agent never re-implements them. The agent only checks and executes an explicit list of assignments.

**Entry point 1: pushing a QC-2 preset** (`send_preset_to_device`).
- The device export (bridge plan §4, P2) gains a `stomps` array: `[{"row": 0, "col": 2, "switch": "C"}]`.
- The dry run shows the layout next to the blocks and values.
- The agent assigns stomps after the blocks and before the save, so they're saved with the preset. That's fine, because the slot is one this system creates.

**Entry point 2: laying out the preset that's loaded on the unit** (any preset).
- **Owner's decision (2026-11-06): apply the layout but don't save.** If the owner likes the result, they save it on the unit. If not, they leave the preset or reload it and the change is gone. This respects the bridge's rule against overwriting slots this system didn't create.
- New MCP tool `apply_stomp_layout`, dry run by default, with the same confirm-token pattern as `send_preset_to_device`:
  1. The dry run queues a `read_preset` job (current preset, already in v1). From the read-back, QC-2 builds the plan and shows it: preset name, each switch, its blocks, every removal, and every block skipped and why. QC-2 also computes a **fingerprint** of what it read: the preset name, each block's (row, column, model id), and the current assignments.
  2. The confirm call queues an `apply_stomps` job carrying the plan and the fingerprint.
  3. The agent re-reads the current preset. If the fingerprint doesn't match, it refuses (`readback_mismatch`), because the owner may have changed preset since the dry run. Otherwise it removes, then assigns, then reads back and reports any assignment that didn't come back as planned.
  4. **The agent never saves in this job.** No `write_preset_file` and no save of any kind.

### 6.2 Contract change

`apply_stomps` is a new job `kind`. The v1 agent's strict parser rejects unknown kinds, so this is a contract change for both sides (`internal/devicejobs` and the Python agent), with new vectors in `testdata/devicejobs/vectors.json`. Proposed args:

```json
{"expect": {"fingerprint": "<sha256 hex>"},
 "remove": [{"row": 0, "col": 3, "switch": "D"}],
 "assign": [{"row": 0, "col": 1, "switch": "B"}]}
```

The agent's own checks: switch in `A`–`G` (never `H`, never index ≥ 8); row 0–3 and column 0–7; a block present at each cell in the fresh read; at most 32 entries in each list. The result reuses `push`'s diff shape, with the switch in place of `param_index`. The exact result fields are settled in the contract PR.

The `stomps` array inside the push export needs no contract change: `export.payload` is opaque to the contract, and its schema belongs to P2.

## 7. Gate G7: stomps on the unit

Run on a scratch preset in an **empty** slot (as G3 did), with a spike that is granted `assign_stomp`, `unassign_stomp` and `get_current_preset` only. Stomps are on the spike harness's never-allowed list today (`tools/device-spikes/spike_common.py`).

| # | Question | How |
|---|---|---|
| a | Does index 1 light up the switch labelled **B** on the top row, and index 4 the **E** switch on the bottom row? | Assign, put the unit in Stomp mode, press the switch, watch the block. |
| b | Do two blocks assigned to the same index both toggle from one press? | Assign two drives to D. |
| c | Does assigning a block a second time move it (qc-mcp's claim), not add a second switch? | Assign a drive to C, then to D; read back. |
| d | Does the read-back's `stomp_assignments` match what was written? | Compare after each step. |
| e | Do assignments survive `write_preset_file` and a recall of that slot? | Save into the empty slot, recall another preset, recall it again. |
| f | **For entry point 2:** does an assignment change mark the preset as edited but unsaved, and does leaving the preset discard it without auto-saving? | Assign on a preset the system created, switch presets, come back, read back. |
| g | **Scenes.** Which scene is active when a preset loads in Stomp mode? Does pressing a stomp change that scene's stored bypass flag? | Read the `Scene` state and the bypass flags before and after a press. |

**If (f) fails** (the unit auto-saves, or keeps the change), entry point 2 is unsafe as designed and stays off until it's redesigned. **If (g) shows that stomps rewrite scene bypass,** add a note to the dry run explaining the effect on scenes. The design doesn't depend on (g) otherwise.

## 8. Phasing

| Step | Scope | Device? | Depends on |
|---|---|---|---|
| S1 | `internal/stomplayout` + `stomp_layout.json` + tests (classification, every rule in §5, golden plans for the eval presets and for synthetic grids with existing assignments) | no | P1 (done) |
| S2 | `stomps` in the device export, shown in the `send_preset_to_device` dry run | no | P2, S1 |
| S3 | G7 spike | **yes** (owner runs) | P0-a harness |
| S4 | `apply_stomps` job kind: contract, vectors, Go side, `apply_stomp_layout` MCP tool | no (fake agent) | P3, S1, S3 |
| S5 | Agent side of S2 and S4 | build no; acceptance **yes** | P4, S3, S4 |

S1 can start now. It touches no device, no auth and no storage.

## 9. Decided

All by the owner, 2026-11-06.

- Scene or Stomp mode only, no Hybrid.
- Layout A Comp, B Boost, C Drive, D Drive 2, E Delay, F Mod, G Reverb, H free.
- Boost = a drive block with "boost" in the model name.
- Separate from the generation pipeline; works on any preset.
- On presets this system didn't create: apply without saving (option 2).
- Several blocks of one role share that role's switch (D for drives after the first).

## 10. Defaults chosen here (change any of them)

- `mod` is the `Modulation` category only. Filter, pitch, wah, morph and synth get no switch.
- Neural Captures get no role (§4).
- The tool replaces existing A–G assignments, and never touches H (§5 rules 5–6).
- Latching only, no secondary assignments (§5 rule 7).
- No change to the Architect or the preset view in v1. A "Switch" column in the preset view is a possible follow-up; it would change `RenderPresetMarkdown`'s golden file and the display guide.
