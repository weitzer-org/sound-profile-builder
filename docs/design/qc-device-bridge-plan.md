# Device bridge: push and read QC-2 Modeler presets on a real Quad Cortex, from the phone

*Design and implementation plan, 2026-11-05. Status: proposed, not started.*
*Inputs: `docs/research/qc-usb-tools-audit.md` (the security/device-safety review), `tools/qc-mcp-trial/` (the allowlist proxy, exercised against the owner's unit), and a successful hand-run qc-mcp session on the owner's Quad Cortex (CorOS 4.1.0, Windows, direct mode). In that session qc-mcp built a five-block preset and reported a save. The save was **not independently confirmed**.*

**Verify-or-label.** Every premise about an external system in this plan is either cited or marked **UNVERIFIED**. Phase 0 exists to settle the UNVERIFIED premises the rest depends on.

---

## 1. Goal and constraints

**Goal.** From the Claude app on a phone, using **only the existing QC-2 Modeler connector**:
- generate or refine a preset (works today);
- push it to the owner's Quad Cortex;
- read presets back from the unit;
- see the device's status.

**Hard constraints**

- **USB is local.** The Quad Cortex is controlled over USB HID (VID `0x152A`, PID `0x880A`, interface 5). Only a machine physically attached to it can talk to it. Fly.io can't, and the phone can't: the Claude mobile app only reaches remote MCP servers, and mobile browsers lack WebHID.
- **Nothing at home is exposed to the internet.** No tunnel, no open port. The home side makes outbound connections only.
- **No LLM in the push path.** QC-2 converts the stored structured preset into exact device operations deterministically. The model only chooses *which* preset and *which* slot.
- **Fail closed on the device.** Writes go only into empty slots, or slots this system created. No global settings, no deletes. Refuse unknown firmware, unknown models and parameters, and out-of-range values.
- **Respect the repo's conventions** (CLAUDE.md):
  - storage only through `storage.Client`;
  - GCP path intact;
  - MCP token auth unchanged;
  - UI parity rules for `generate_preset` untouched;
  - security review for auth, storage or LLM-content changes.
- **The home side runs natively on the owner's Windows PC, and only on demand.** It is a Python program in a pinned virtual environment.
  - It is **not** always running. The owner starts it from a **desktop shortcut** ("Start QC bridge") when they want to use it (owner's decision).
  - It exits when the unit disconnects, or after 30 minutes without a job (owner's requirement: most of the time it shouldn't run).
  - It uses **qc-mcp's transport code**, vendored into this repo at the reviewed commit `f634ecd` (`deploy/qc-agent/vendor/qc_mcp/`). That code already drove the owner's unit (CorOS 4.1.0) in direct mode.
  - **pyquadcortex is not used** (owner's decision).
  - **Containers are not used.** Every Windows container option puts the device behind USB passthrough into a Linux VM, and qc-mcp has no Linux backend. See section 5.1.

## 2. Architecture

```
Phone: Claude app ── QC-2 Modeler connector (existing /mcp, MCP_TOKEN) ──► QC-2 on Fly (scales to zero)
                                                                     │ writes a signed job
                                                                     ▼
                                                     R2 bucket "<app>-device-jobs" (NEW, separate)
                                                                     ▲ polls (outbound HTTPS only)
Home: Windows PC ─ qc-agent (on demand: desktop shortcut) ──────────┘
                     │ Windows HID (hid.dll), via qc-mcp's transport (vendored at f634ecd)
                     ▼
                 Quad Cortex (USB)
```

**QC-2 Modeler (Go, Fly)**
- Gains the device tools (section 3).
- Turns a stored preset into a **device export**: exact model ids, parameter indexes, normalized wire values, per-scene values and grid placement. It refuses anything it can't map exactly.
- Writes a job as JSON, **HMAC-signed** with `DEVICE_JOB_KEY`, to the jobs bucket.
- Reads job results back.
- **Never** talks to the device.

**Jobs bucket**
- A **separate** R2 bucket, so the agent's credentials can't read or write user presets, run logs or usage data.
- QC-2 accesses it through the existing `storage.Client`. The interface already takes a bucket name on every call (`internal/storage/preset_store.go:70-76`).
- Layout:
  - `jobs/pending/<job_id>.json`
  - `jobs/done/<job_id>.json` (result)
  - `agent/heartbeat.json`
  - `agent/library.json` (synced capture, IR and preset names)

**qc-agent (home, native Windows)** — a small, single-purpose program. In a loop it:
1. polls `jobs/pending/`;
2. verifies the HMAC and the expiry;
3. **re-validates** every operation against its own copy of the catalog and rules;
4. checks the unit itself (firmware, slot occupancy);
5. executes through an allowlisted device library;
6. reads back and diffs the result;
7. writes the result to `jobs/done/`.

It also writes a heartbeat every ~60 s.

**Why polling R2 instead of calling QC-2.**
- A home agent polling QC-2 every few seconds would keep the Fly machine awake permanently. `fly.toml:32-34` has `auto_stop_machines = "stop"` and `min_machines_running = 0`, so that would defeat scale-to-zero.
- Polling R2 directly costs one small storage operation per poll, and never wakes Fly. R2 operation pricing at roughly 10 s polling is **UNVERIFIED**; Phase 0 checks it.
- Fallback: an adaptive interval, 5 s while a job was recently queued and 60 s otherwise. Keep that whatever the pricing.

**Why sign jobs.** If the agent's R2 token leaked, an attacker could write jobs. The HMAC means only QC-2 can mint a job the agent will run. The agent's own rules (empty or owned slot, no globals) still limit any job it does accept.

## 3. MCP tool inventory and coverage

### 3.1 QC-2 Modeler today (10 tools; `internal/mcpserver/*.go`)

| Tool | Kind |
|---|---|
| `generate_preset` | Runs the paid pipeline (`runtools.go`). |
| `refine_preset` | Runs a paid refine (`runtools.go`). |
| `get_run_status` | Read (`runtools.go`). |
| `get_run_log` | Read (`runtools.go`). |
| `get_run_prompts` | Read (`runtools.go`). |
| `get_agent_output` | Read (`runtools.go`). |
| `list_presets` | Read (`tools.go`). |
| `get_preset` | Read (`tools.go`). |
| `save_preset` | Names or renames a stored preset (cloud only) (`tools.go`). |
| `read_resource` | Reads `qc://` resources such as the style guide (`resources.go`). |

### 3.2 qc-mcp at `f634ecd` (60 tools)

The full classification is in the audit, §1.4.

| Group | qc-mcp tools |
|---|---|
| Device reads | `device_info`, `get_current_preset`, `current_preset_position`, `cpu_load`, `get_mode`, `get_io_settings`, `get_tempo`, `output_meter` |
| Library reads | `directory_summary`, `search_directory`, `list_favorites`, `list_empty_slots`, `list_device_presets`, `list_settings_presets`, `find_devices` |
| Preset editing (live) | `build_preset`, `clear_grid`, `add_block`, `remove_block`, `add_capture`, `add_ir`, `add_split`, `set_lane_routing`, `set_lane_output`, `set_mixer`, `set_parameter`, `set_parameter_scenes`, `set_block_bypass`, `assign_expression`, `set_preset_meta`, `assign_stomp`, `unassign_stomp`, `load_device_preset` |
| Navigation | `recall_preset`, `switch_preset`, `switch_scene` |
| Persist | `save_preset`, `save_preset_as`, `save_device_preset`, `delete_device_preset` |
| Global | `set_master_volume`, `switch_mode`, `set_mode_cycle`, `set_io_port`, `set_global_eq`, `load_settings_preset` |
| Audio / leveling | `audio_devices`, `sample_arm`, `sample_status`, `sample_stop`, `sample_discard`, `measure_loudness`, `measure_preset`, `compare_presets`, `suggest_levels`, `level_preset`, `level_scenes`, `level_setlist` |
| Session | `connect`, `disconnect` |

### 3.3 The combined connector: what the phone gets

The device capability is exposed as **a few intent-level tools**, not 60 low-level ones. Low-level operations become internal steps of a job that QC-2 builds and the agent checks.

| New QC-2 tool | Version | Covers qc-mcp | Behaviour |
|---|---|---|---|
| `device_status` | v1 | `device_info`, `cpu_load` | Read-only. Agent online or offline (heartbeat age), firmware, last job, and whether the unit is connected. |
| `send_preset_to_device` | v1 | `build_preset`, `add_block`, `set_parameter(_scenes)`, `set_block_bypass`, `set_lane_routing`, `set_preset_meta`, `save_preset_as` | Args: `preset_id`, `guitar`, `slot`, `name`, `dry_run` (default **true**). A dry run returns the exact plan: blocks, grid cells, values, and anything **unresolved**, plus a `confirm_token`. A second call with that token queues the job. The agent writes only into an **empty** slot, or a slot this system created. |
| `get_device_job` | v1 | — | Read-only. The job's state and result: diff against the plan, read-back, CPU load. |
| `read_device_preset` | v1 | `get_current_preset`, `current_preset_position`, `recall_preset` (read path) | Read-only by default. Reads the current preset, or a named slot. Recalling a slot changes what's live, but stored presets are untouched. With `import=true`, it stores the result as a QC-2 **draft**, for refine-from-device and evals. |
| `list_device_slots` | v1 | `list_empty_slots`, `directory_summary`, `search_directory` | Read-only. Empty slots and occupied names in My Presets. |
| `sync_device_library` | **v1 (P4b)** | `directory_summary`, `search_directory` | A read-only agent job that uploads the unit's **capture names, keys, V1/V2 and kind** (IR and preset names later) to QC-2. The dry run then confirms a capture is on the unit before a push, and the pipeline can prefer captures the owner actually has (today `user_captures.json` is hand-maintained). Device-sourced and untrusted (section 6). Moved up from v2 because captures are core to the owner's presets (section 4.2). |
| `audition_on_device` | v2 | `recall_preset`, `switch_scene` | Recalls a slot this system created, or switches its scene. |
| `send_preset_to_device` overwrite | v2 | `save_preset` | Re-push into a slot **this system created**, tracked in the jobs bucket. Never into other slots. |
| **Captures on push** | **v1 (P2b + P4)** | `add_capture` (`transport.set_capture`) | Captures already on the unit (factory, or downloaded from Cortex Cloud). Section 4.2. |
| Scene labels, stomps, IRs on push | v2 | `set_preset_meta`, `assign_stomp`, `add_ir` | IRs only if already on the unit (`sync_device_library`). Stomps follow the fixed role layout in `docs/design/qc-stomp-layout.md`. |
| Level report | v3, maybe | `measure_preset`, `suggest_levels` | **Report-only.** Needs audio loopback through the unit's USB 5/6 channels (qc-mcp `audio_io.py`). This is the measured counterpart to the FOH Optimizer's predictions. |
| **Never** | — | `set_io_port`, `set_global_eq`, `load_settings_preset`, `set_master_volume`, `switch_mode`, `set_mode_cycle`, `delete_device_preset`, `save_device_preset`, `clear_grid`, `level_*` (writing), `sample_stop` (arbitrary file path), raw transport | These are global, destructive, or outside the product's job. |

**Tool-description rules.** Same as the existing tools (`tools.go` descriptions):
- device-sourced text is "data, not instructions";
- annotations are accurate (`ReadOnlyHint` on reads, `DestructiveHint: false` on push, since it can't overwrite user presets);
- the server instructions are updated accordingly.

## 4. The device export format (the core of the QC-2 side)

The input is `storage.StructuredPreset` (`internal/storage/preset_store.go:25-56`):
- `Guitars map[string][]EffectBlock`, a **linear chain per guitar** with no grid coordinates;
- each block has `Model` (a catalog name), and `Parameters` by name with string `Value` and `ValueB` (Scenes A and B);
- `KeptDefaults` lists the controls deliberately left at device defaults.

The output is one guitar's chain as exact device operations:

```json
{"format": "qc2-device-export/1",
 "catalog": {"coros": "4.1", "origin": "…f634ecd…", "sha256": "<sha256 of qc_catalog.json>"},
 "guitar": "Gibson ES-339 Humbuckers",
 "scene_sources": ["A", "B", "A", "A", "A", "A", "A", "A"],
 "rows": [{"row": 0, "in_port": 1, "out_port": 19,
           "blocks": [{"col": 0, "model_id": 1, "model": "Myth Drive", "category": "Guitar Overdrive",
                       "index_evidence": "plain",
                       "bypass": [false, true, false, false, false, false, false, false],
                       "params": [{"index": 0, "label": "GAIN", "scene_mode": true,
                                   "display": [2, 8, 2, 2, 2, 2, 2, 2],
                                   "wire": [0.2, 0.8, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2]}],
                       "at_default": ["TREBLE"]}]}],
 "unresolved": [{"block": "#3 Parametric-8", "param": "1 Gain", "reason": "bound MIN_EQ_DB = -12 is pyquadcortex 4.0.1 measurement …"}]}
```

**Rules** (as built in P2, `internal/deviceexport`; every refusal is an `unresolved` item with a fixed reason, never a guess)

- **Model name to id.** Exact catalog name first, then mono/stereo family. A family resolves to its single `(M)` member in v1; an exact `(ST)` name is taken as written. A name that several catalog models share (e.g. Green 808, ids 5 and 27) is **refused** as ambiguous, even when the layouts match (owner decision). Refused as well: captures (coros_map.json/user_captures.json, the "(capture)" annotation, the catalog's own Neural Capture slots) and IR loaders; ids 20000-20002 and any `Internal`/`Utility_Deprecated` category (NC_Recorder 20000 sits in "Neural Capture Internal", which the importer already drops; this is a second guard); any category outside an allowlist (FX Loop, Splitter, Mixer and Loopers are not pushed in v1, and a category a future CorOS adds is refused until listed); side-chain `(S/C)` models; the input/output blocks and the global EQ (the bridge never writes globals). **Plugin models are allowed only for plugins in `config.json`'s `available_plugins`** with `allow_paid_plugins` on (owner decision); the block carries `plugin`.
- **Cloned models.** The catalog now records `clones` (the base id). A clone is pushed only when its base is in `catalog.VerifiedCloneBases`, today **12000 only** (G1 checked one mono cab cloning it: 62 guitar and 8 bass mono cabs share that base). Stereo cabs (base 32000), the plugin cabs (12100, 32100, 12115) and the cloned reverbs (8010, 8015) are refused until a G1-style read-back covers them. Each placed block says what backs its indexes (`index_evidence`: `plain` or `clone-12000-g1`).
- **Parameter name to index.** Through `catalog.LabelParams`, the per-mic labelling the Architect's parameter context uses (moved into `internal/catalog`, so the two can't drift), plus the same aliases. Every index must pass `Model.ValidParamIndex`; failing it is a fault, not an unresolved item. Hidden params and the device's trailing slot never appear. A name written twice is refused. On cabs, `Mic 1`/`Mic 2` (the mic choice is the IR-name string parameter; writing strings is unmeasured) and `Blend` (the device has none) are refused.
- **Value string to number.** The `*` estimate marker (and `\*`) is stripped. A number with an optional unit; the unit must match the control's dimension (dB, Hz/kHz, ms/s, %) and is converted; a bare number is in the control's own unit; a unit on a unitless dial is refused. `Off` maps to the end of the range labelled OFF. Unparseable text (ranges, "noon", NaN) is refused.
- **Display to wire** through `Param.ToWire`, which refuses rather than clamps. On top of that the export only converts where the unit confirmed the mapping:
  - **taper skew** must be one of `catalog.ConfirmedSkews` (1, numeric 0.3, 3.8018, 4.9594844; G4). Skew 0.6 (241 controls), 4.93 (cab OUTPUT VOLUME), `LOG_SKEW` (told apart from numeric 0.3 by the new `skew_symbol`) and every other skew are refused as "not confirmed on CorOS 4.1.0" (owner decision: strict);
  - **symbolic bounds** must have `EvidenceConfirmed410` (the cab bounds). The pyquadcortex-only bounds (EQ, mixer, FX loop, tempo, EQ frequency) and the unmeasured `MIN/MAX_INPUT_TRIM` are refused, each with its evidence string;
  - **pans** (L/R labels) and **unitless 0..1 floats** have no verified display scale and are refused, except a pan written as Center when that is the default;
  - **switches, rotary switches, toggles, combo boxes, faders, `floatWithLed`** have no measured wire mapping and are refused.
  - A control set to its catalog default in both scenes is **not written** (listed in `at_default`), whatever its type: a block added to an empty slot starts at its defaults. That rests on `add_block` initialising every parameter to the XML default, which was seen for two parameters only (G4 photos); P4's read-back diff should check it.
- **Scenes.** `Value` is Scene A and `ValueB` (or `Value` when empty) is Scene B. Scenes C-H take Scene A's value. **G6 measured that the device does not copy A into C-H** (unwritten scenes keep the parameter's earlier value), so the export carries all eight scene values for every written parameter and all eight bypass flags for every block; the agent never relies on the device to fill a scene. `scene_mode` is set when A and B differ.
- **Bypass.** A block's `Bypass` parameter: `On` = bypassed, `Off` = active (Architect v4-v6, Rule 13); anything else is refused. A preset with no `basis` field anywhere predates that rule (v4 added both together), so its `Bypass` values are refused with "refine it to push it". No `Bypass` parameter = active in all eight scenes. The dry-run table shows the state as Active/Bypassed.
- **Grid.** Row 0, columns in chain order, in 1 and out 19 (G3 read this routing back). **More than 8 grid blocks is refused** in v1; blocks past the eighth are not mapped. A refused block keeps its column (the gap stays visible). Splits and parallel paths come in v2.
- **Captures and IRs** were `unresolved` in P2. Captures move into v1 with P2b, P4 and P4b (section 4.2); IRs stay v2.
- **A push needs zero unresolved items** (`devicejobs.Bridge`: no confirm token otherwise, and the confirm re-exports). A dry run, `get_preset(format=device)` and the UI's "Download device JSON" link all show the full list, so the user can refine first.

### 4.1 Revision after P2: no more manual device sessions (owner, 2026-11-06)

P2 shipped strict: anything not confirmed by a Phase 0 reading was refused. Across all 28 guitar chains of the 14 v6 eval presets that left **none** pushable, and no chain was blocked by a single reason. The owner prefers no further hand-run measurement sessions (the G4b, G7 and G1b proposed in P2 are **dropped**), so P2b switches from "confirmed on the unit" to "**from the device's own data or a named third-party measurement, labelled, and checked in normal use**". Why that is acceptable:
- **A mistake is cheap and visible.** The agent writes only into an empty slot and never overwrites. The worst case of a wrong mapping is a new preset whose control shows a different value than the dry run said. It shows on the unit's screen and the preset can be deleted.
- **The data is not guessed.** Skew exponents come from the device's own `ModelRepo.xml`. G4 confirmed the formula `((v-min)/(max-min))^skew` at four exponents, below and above 1 (0.3, 1, 3.8018, 4.9594844); that it holds for the other exponents is an inference, labelled as one. The pyquadcortex bounds are a third party's measurements on CorOS 4.0.1.
- **The check moves into P6**, the owner's first real pushes from the phone. That is normal use, not a separate session: compare the screen with the dry-run table. P5's read-back makes the comparison automatic for wire values (not for screen display).

P2b rules (changing section 4):
| Item | P2 | P2b | Evidence label carried in the export |
|---|---|---|---|
| Numeric skews other than the four measured (0.6, 4.93, 0.5, ...) | refused | **allowed** | `taper: "device-xml"` (vs `"g4"`) |
| `LOG_SKEW` (16 controls) | refused | **allowed** with pyquadcortex's 0.3 | `taper: "pyquadcortex"` |
| pyquadcortex bounds (EQ dB, EQ freq, mixer, FX loop, tempo) | refused | **allowed** | `bounds: "pyquadcortex-4.0.1"` |
| `MIN/MAX_INPUT_TRIM` | refused | **still refused**: no data exists at all | — |
| Switches, rotary switches, toggles (named or numbered positions) | refused unless default | **allowed**: wire = (position - min) / (max - min), the convention qc-mcp's `catalog.to_norm` uses on these units | `encoding: "qc-mcp-linear"` |
| Combo boxes, faders, `floatWithLed` | refused | still refused unless at default (too few to matter: none blocked an eval chain) | — |
| Cab `Mic 1`/`Mic 2` equal to the cab's default mic (the XML's default IR-name string, e.g. `ME_110 US PRN C10R_Dynamic 57`; "Dyn 57" matches "Dynamic 57") | refused | **accepted as at-default**, not written | — |
| Cab `Mic 1`/`Mic 2` set to another mic | refused | **not written, not blocking** (owner, 2026-11-06): the cab keeps its default mic and the export lists it under `warnings`, which the dry run shows as "left out of the push". Which mics each cab offers isn't in `ModelRepo.xml`, qc-mcp or the manual; P4b collects real mic strings from presets on the unit so a seen mic can be written exactly later | `warnings` |
| Clones of bases other than 12000 | refused | still refused: a wrong index writes the wrong control, and the expansion rule for those bases is untested. The mono cabs (base 12000) cover the family resolution path | — |
| Pans and unitless 0..1 controls | refused | still refused: the problem is the preset's text (the Architect is told "scale unknown"), not the device. Fix on the pipeline side later by showing the Architect the catalog range | — |

The importer keeps the default IR-name string (`default_string`) for the mic rule; nothing else in the catalog changes.

**Is any manual device work still justified?** No. The one gap is non-default cab mics. A mic is a string `<prefix>_<internal cab name>_<mic>` (prefixes `ME_`, `ML_`, `NG_`, `JP_`; the internal cab name often differs from the catalog name, e.g. `ML_212 Fend A-type 00s` for "212 US A-type 00s"), and only 8 mic names appear anywhere in `ModelRepo.xml` (Dynamic 57/421, Condenser 414/184, Ribbon 160/121/10/353), never as a per-cab list. A wrong string could leave the cab without an IR. Instead of a session: push the default mic with a warning now, and learn real per-cab mic strings from presets the agent reads on the unit (P4b). In the eval presets Mic 1 is "Dyn 57"/"Dynamic 57" in 22 of 28 cab blocks, and Dynamic 57 is the XML default Mic 1 (41 of 41 cab defaults), so most of them become at-default. That they match each chosen cab's own default is an inference: P2b's census will show the real count. A different mic is a refinement the user can make on the unit. If this changes, one read-only dump of a cab with a hand-picked mic would answer it.

### 4.2 Captures on push (P2b + P4 + P4b)

Captures are central to the owner's presets. 18 of the 28 eval chains use at least one: half of the amps (14 of 28) and 12 drive or pedal blocks (2026-11-06 count over the v6 eval set). qc-mcp already places them, and **none of this needs manual device work.** Everything below comes from qc-mcp's `DIRECTORY.md` and `transport.set_capture` at f634ecd. qc-mcp reports it verified live (`add_capture` loaded "CA John's 2"); **it has not been run on the owner's unit**.
- **How the device binds a capture.** A Neural Capture block is model **14000 (V1) or 14001 (V2)**. Params 0-4 are GAIN, BASS, MID, TREBLE, VOLUME: plain linear dB controls (-24..24, -6..6, -6..6, -6..6, -24..24), so they convert under the existing rules. Param 5, `file_name`, is a string: the capture's 64-hex key followed by its display name. A capture must go into the block type of its version (V1 or V2). The preset file also lists the capture as a dependency (`factory_dependencies` by hash, or `product_dependencies` with name and cloud id for downloaded captures).
- **Where the key comes from.** The capture must already be on the unit. Its key, version, kind (amp, combo, pedal, ...) and folder come from a directory read (`File` READ, type 2). That is a read the agent already makes for slots. QC-2 never invents a key.
- **P2b, QC-2 side.** A capture block exports as `{"capture": {"name": "<as in the preset>", "source": "factory|user"}, "params": [...]}`, with GAIN/BASS/MID/TREBLE/VOLUME converted like any linear float and all eight scenes written. Its relative-dB values (Rule 9) are already in the block's own dB ranges. The export doesn't know the key or V1/V2; it marks the capture `resolved_on_unit: false` until P4b's library sync gives QC-2 the unit's list. With a synced list, the dry run resolves the name to `{key, version}` exactly and lists a capture that isn't on the unit as unresolved ("not on your unit: download it in Cortex Control").
- **P4, agent side.** Before building, the agent reads the capture directory fresh and resolves every capture by key (or by exact name when the export has no key). A capture that's missing, or that matches more than one entry, fails the job without writing. It places 14000 or 14001 to match the version, writes `file_name` with the transport's typed string write, writes the five params per scene, and makes sure the saved preset carries the dependency entry. Whether `write_preset_file` adds dependencies by itself is **not established**: P4 reads qc-mcp's save path and adds them if it doesn't. A cloud capture that isn't downloaded is never fetched: the agent sends no `ProductForward`.
- **Name matching.** `coros_map.json` names factory captures by their QC names ("CA John's", "Brit 2203 87"), and qc-mcp's example capture is "CA John's 2". Whether a " 2" suffix marks the V2 copy of the same capture is **unverified**. So matching is exact against the synced list, plus a reviewed alias table filled from the first sync, never fuzzy at push time.
- **P4b, `sync_device_library`.** A read-only job that writes the unit's capture list (name, key, V1/V2, kind, instrument, folder) to QC-2, plus the cab mic strings (`ir selector` values) seen in presets the agent reads, as a per-cab "seen on this unit" mic list. A mic on that list is then written exactly instead of becoming a warning. Whether a stored preset can be read without loading it on the unit is **unverified**; if it can't, collect only from presets the user asks to read. The names are untrusted: bounded, stripped and passed through `cell()`. It feeds the dry run's resolution and, later, the Librarian, so generation prefers captures the owner actually has.
- **Checked in P6:** push one factory and one downloaded capture from the phone, and see that the unit loads them in the right block type.

## 5. The home agent

**Library: qc-mcp's transport, used as a library.**
- The agent imports `qc_mcp.transport.QuadCortex` and calls its methods directly. It does **not** run qc-mcp's MCP server or its 60 tools.
- **Vendored, not cloned.** The needed modules (`transport.py`, `protocol.py`, `backend.py`, `winhid.py`, `directory.py`, plus `descriptors/*.pb`; about 2,400 lines, MIT) are copied from upstream `f634ecd` into `deploy/qc-agent/vendor/qc_mcp/`, with upstream's `LICENSE` and a `PROVENANCE.md`.
  - A test checks each vendored file's SHA-256 against the `f634ecd` originals, so any local change is deliberate and visible.
  - Upstream updates arrive only through a re-vendoring PR.
  - The runtime dependency is `protobuf`, hash-pinned. The agent doesn't need the `mcp` SDK.
  - A separate GitHub copy of qc-mcp is **not** needed.
- It's MIT-licensed, and already proven on the owner's unit in direct mode on Windows.
- Methods used:
  - opening: `open(handshake=True)` (direct, exclusive);
  - reads: `read_state("Version")`, `get_setlist_position`, `list_directory`, `get_current_preset`;
  - grid: `clear_grid` (only on a verified-empty slot), `set_routing`, `add_block`, `set_param`, `set_param_scenes`, `set_block_bypass`;
  - scenes: `set_scene_label`;
  - saving: `write_preset_file`, the File CREATE save;
  - closing: `close`.
  - **Never:** `save_preset` (RecallPreset SAVE, the documented hang), IOSettings, GlobalEQ, MasterVolume, Mode, ModelPreset, or the raw `send`.

**Known qc-mcp gaps the agent must cover itself** (from the audit, §1.5):

| qc-mcp gap | What the agent does |
|---|---|
| No firmware gate; newer firmware falls back to the 4.1 schema | Refuse unless `firmware` equals the configured value exactly (`4.1.0` today). |
| `to_norm` passes unknown ranges through unconverted | Not used. QC-2 sends **wire** values. The agent re-checks each one is in 0..1 and that the parameter has a numeric range in the agent's catalog copy. |
| NC_Recorder (20000) is placeable | Refuse hidden or internal categories, plus an explicit denylist. Plugin models are placed only when they arrive in a signed job: QC-2 exports them only for plugins in `available_plugins` (owner decision, P2). Placing a plugin block on the unit has not been tested. |
| `save_preset_as` guard reads a stale cache | Use `list_directory` fresh, immediately before the save. |
| Parameter indexes for cloned models are unverified | G1 verified clones of base 12000 only. QC-2's export refuses every other clone base (`catalog.VerifiedCloneBases`); the agent's allowlist should refuse them too. |

**Allowlist.** Reuse and extend `tools/qc-mcp-trial/qc_allowlist_proxy.py`'s rules:
- `check_model` and `check_param` against the agent's own catalog copy;
- grid bounds;
- firmware match;
- an empty-slot check from a **fresh** device listing;
- slot ownership from a `jobs/owned_slots.json` that only the agent writes.

The agent exposes **no** server, port or UI. Its only inputs are signed jobs.

**As built in P4** (`deploy/qc-agent/`; decisions taken while implementing, recorded here):
- **A push builds only into the empty slot that is open on the unit.** The agent never recalls a preset or clears a
  grid: it requires the target slot to be selected on the unit with a blank live grid, which is exactly the save path
  G3 verified. A different open slot is refused (`unsupported_arg`, "select slot X on the unit first").
- **Catalog copy:** the agent validates against this repo's `internal/catalog/qc_catalog.json` in the PC's clone (not
  qc-mcp's XML, per G1), and refuses an export whose `catalog.sha256` differs (hash over the file with CRLF normalised
  to LF). It can't check `display` → `wire` until P1 adds `skew` to the catalog; it bounds both.
- **Per-scene bypass:** a block bypassed in any scene goes through the G6-verified path (assign to scenes, then each
  scene's flag, all eight written), even when all eight agree. A block active in every scene gets no bypass write:
  it relies on a new block starting active, which the read-back checks (all eight flags must read back false). Scene labels are not written in v1 (they were v2 in section 3.3).
- **Wire guard:** besides a method allowlist, the backend decodes every outgoing message and passes only session
  plumbing, READs, the armed plan's Grid/Scene updates and one armed File CREATE.
- **No boto3:** a standard-library SigV4 client, so the lockfile holds only `protobuf` (7.36.0, the pins from
  qc-mcp's own lock).

**Running it on Windows**

- **Install:** clone this repo, then `py -3.12 -m venv`, then `pip install --require-hashes -r deploy/qc-agent/requirements.lock.txt`.
- **Configuration** goes in `%LOCALAPPDATA%\qc-agent\agent.env`, written by `deploy/qc-agent/write_agent_env.py`. That script prompts for each value with secrets hidden, tests the bridge token against R2 with a read-only listing, and restricts the file to the user with `icacls`. Its variables, which the agent must read under exactly these names:
  - `QC_AGENT_R2_ENDPOINT`, `QC_AGENT_R2_BUCKET`;
  - `QC_AGENT_R2_ACCESS_KEY_ID`, `QC_AGENT_R2_SECRET_ACCESS_KEY` (the bridge token: Object Read & Write on the jobs bucket only);
  - `DEVICE_JOB_KEY` (the same value as the Fly secret) and, only during a rotation, `DEVICE_JOB_KEY_NEXT`;
  - `QC_AGENT_ALLOWED_SLOTS` (e.g. `20A-25H`), `QC_AGENT_EXPECTED_FIRMWARE` (`4.1.0` today), `QC_AGENT_IDLE_MINUTES` (`30`).

  No secret goes on the command line.
- **On demand, never always-on** (owner's requirement).
  - **Start: a desktop shortcut** (owner's decision). `python -m qc_agent install-shortcut` creates "Start QC bridge" on the desktop, running `<venv>\Scripts\python.exe -m qc_agent run`. It's a `.lnk` written by Python: no PowerShell script, no execution-policy change, no admin rights.
    - The window it opens shows "Bridge running: waiting for jobs", then each job and its result.
    - Closing the window stops the bridge.
    - A second launch while one is running says so and exits, using a lock file in `%LOCALAPPDATA%\qc-agent\`.
  - **It exits by itself** when the unit disappears (a USB read error or device removal), or after **30 minutes with no job** (`QC_AGENT_IDLE_MINUTES`). It runs nothing and polls nothing between sessions.
  - **No Task Scheduler task and no autostart.** `python -m qc_agent uninstall-shortcut` removes the shortcut.
  - A Windows *service* is rejected: services run outside the user session, and qc-mcp's HID access has only been exercised in a user session (UNVERIFIED for services).
- **Logs** go to `%LOCALAPPDATA%\qc-agent\logs\`, rotated, with no tokens and no job HMACs.
- **Exclusive access:** qc-mcp opens the unit exclusively on Windows. So the agent connects **only while running a job**, then disconnects.
  - Cortex Control works normally between jobs.
  - If Cortex Control is open when a job arrives, the agent fails that job with "Cortex Control has the unit — close it and retry". It never force-quits the app: `quit_app` is never used.

**Concurrency and expiry.** One job at a time, each in its own connect, act, disconnect cycle.
- If no agent heartbeat is newer than 2 minutes, `send_preset_to_device` **refuses at queue time** ("bridge not running — switch on the Quad Cortex or start the bridge"), rather than queueing work that might run hours later.
- Jobs that are queued still expire after 15 minutes.
- An optional `queue_if_offline=true` keeps a job for the next session, with the same 15-minute expiry unless the owner sets a longer one.

### 5.1 Why not a container on Windows

Every container runtime for Windows hits the same two problems:

| Option | How it runs | USB HID access | qc-mcp works inside? |
|---|---|---|---|
| Docker Desktop (Linux containers) | WSL2 Linux VM | Only via `usbipd-win` attaching the unit to WSL2. That detaches it from Windows, so Cortex Control loses it, and re-attaching after a replug or reboot is a manual or `--auto-attach` step (**UNVERIFIED** for Docker Desktop's VM). | **No.** Linux has no qc-mcp backend (`src/qc_mcp/backend.py:105-115`). |
| Podman Desktop, Rancher Desktop | Also a WSL2 or Hyper-V Linux VM | Same as Docker Desktop. | No. |
| Docker Desktop, *Windows* containers | Windows container, process- or Hyper-V-isolated | Windows containers can be given some devices by interface class, mainly documented for Windows Server. HID devices on Windows 10/11 desktops are **not a documented case** (UNVERIFIED). | Unknown. Unsupported territory. |
| Windows Sandbox | Throwaway VM | No general USB passthrough (UNVERIFIED). | No. |
| WSL2 without containers | Linux VM | Same as Docker Desktop (usbipd). | No. |

**The only way to make a Linux container work** is to write a Linux hidraw backend for qc-mcp. Its `backend.py` names exactly what's missing: `open`, `set_report`, `read_reports` and `close`. That's about 150 lines, plus the usbipd setup and its replug fragility, all for isolation the owner doesn't need.

**Not pursued.** Native Windows with a pinned venv, a non-elevated user task and outbound-only networking already gives a small, well-contained footprint.

## 6. Security model

| Threat | Mitigation |
|---|---|
| Phone or LLM asks for something harmful | Intent-level tools only. The agent enforces empty or owned slots, no globals, catalog-validated values, and a firmware match. Dry run by default, then a confirm token. |
| Prompt injection from device text (preset, capture, IR names written by strangers) | Treated as data. Tool descriptions say so. It never feeds a write decision directly: `send_preset_to_device` takes ids and slots, and the agent checks them. Bound the length and strip control characters on synced names. |
| Leaked agent R2 token | Separate bucket with no access to presets. HMAC-signed jobs, so the agent ignores unsigned or expired jobs. Rotate the token. |
| Leaked `DEVICE_JOB_KEY` | Worst case is pushes into empty or owned slots, which the agent's rules still limit. Rotate it like `MCP_TOKEN` (support a `_NEXT` key). |
| QC-2 compromised | Same bound as above. The agent's rules are independent of QC-2. |
| Unit hang or crash | Known crash model refused. No RecallPreset-SAVE path: the agent saves only via qc-mcp's `write_preset_file` (File CREATE), never `save_preset`. Recovery is a power cycle plus a Cortex Control backup. Document both. |
| Secrets in logs | Reuse the `mcpserver.Config` redaction pattern. Never log job HMACs or tokens. |

**Required reviews** (CLAUDE.md, "Security review"):
- `/security-review` plus the `security-reviewer` agent on P3 and P4;
- GSR `deep-review` label on P3;
- address the bots' `security-high` and `security-critical` findings before merge.

## 7. Phase 0: verification gates (before any production code)

| Gate | Question | How | Needs device |
|---|---|---|---|
| **G0** | Did the hand-run "bb king test 2" save actually land in slot 2A? | Check on the unit, or run `directory_summary(refresh=true)` then `search_directory`. | yes |
| **G1** | **Which parameter-index scheme does the device use for the 196 cloned models (cabs, some reverbs)?** Our catalog expands `clones=` templates. qc-mcp ignores them. Their indexes disagree for all 196 (checked against ModelRepo.xml). | Read back a preset containing a cab and a cloned reverb. Compare the raw per-block `params` array length and positions with both schemes. The "bb king test 2" preset has a cab and Spring (M). Use a library-mode script on the `qc-trial` qc-mcp clone. | yes |
| **G2** | (a) Does a **library-mode** qc-mcp script, started from a desktop shortcut, connect, read and disconnect cleanly? (b) Does it fail cleanly when Cortex Control holds the unit? (c) Does it notice the unit being switched off or unplugged, and exit? | Run the spike from a shortcut. Repeat with Cortex Control open. Power off the unit mid-wait. | yes |
| **G3** | Do qc-mcp's transport writes behave on 4.1.0 when driven with **wire** values? That means `set_routing`, `add_block`, `set_param` with an explicit 0..1, `set_param_scenes`, and `write_preset_file` into an empty slot. | One test preset into an empty slot. Read it back and compare. | yes |
| **G4** | Taper and symbolic-bound constants on **4.1.0**. Our proposed constants come from pyquadcortex's 4.0.1 measurements. They are used here as **data, not code**, so they need confirming on this unit. | Set three known display values through the export path, and read the unit's screen. | yes |
| **G5** | R2 pricing for polling, and bucket-scoped API tokens | Cloudflare docs or dashboard. Record the numbers in the PR. | no |
| **G6** | Scene semantics: do per-scene parameter writes and per-scene bypass behave as qc-mcp documents? | Push a two-scene test preset and switch scenes on the unit. | yes |

### 7.1 Phase 0 results log

The spikes are in [`tools/device-spikes/`](../../tools/device-spikes/README.md) (PR P0-a). Each result below is the owner's pasted output from their unit; nothing is recorded here from inference. **Offline tests with a fake device cover only the guards and the report logic.** They say nothing about the unit.

| Gate | Spike | Status | Result |
|---|---|---|---|
| G0 | `g0_verify_slot.py` | **PASS** (owner's unit, 2026-11-05) | Slot 2A (position 8) in My Presets holds "bb king test 2", so the hand-run save did land (read from a fresh `list_directory`, exactly one preset with that name). Unit reports firmware `4.1.0`, `device_type` QC. My Presets has 247 of 256 slots empty (first: 2B, 2C, 2D...). The unit had a different preset open at the time (`/media/p4/Presets/Fender Amps`, position 27), so G1 needs 2A selected by hand first. |
| G1 | `g1_dump_params.py` | **ANSWERED for the dual-mic cab family; not for all 196 clones** (owner's unit, CorOS 4.1.0, 2026-11-05) | On "bb king test 2" (5 blocks) the device's per-block parameter count N equals the full `ModelRepo.xml` list with `clones=` resolved and hidden params included (EFF), plus one trailing slot, for **every** block: Jewel 6 (EFF 5), Green 808 4 (3), US Prince 5 (4), Spring (M) 6 (5), and the cloned cab "110 US PRN C10R (M)" (12045, clones 12000) **22 (EFF 21)**. qc-mcp's unexpanded reading of that cab is 2 parameters. Our catalog's expansion agrees with the device's layout: per-mic IR-name strings sit at positions 1 and 9 (the two `replaces` indexes), HPF/LPF at 16/17, and the base's two hidden `IR NAME` strings are present as empty strings at 19 and 20. Details in 7.2. |
| G2 | `g2_session.py` | **(c) PASS for power-off. (a), (b) and unplug not run** (owner's unit, CorOS 4.1.0, 2026-11-05) | (a) shortcut start: not run. (b) Cortex Control holding the unit: not run, because the owner has no Cortex Control installed, so the agent's "close Cortex Control and retry" path is untested. (c) power-off while connected (console, seen): probes answered ok about once a second up to 26.1 s, with the gap between them stretching from 1.1 s to about 4 s; the reader thread was found dead and the write-error count first seen rising together at **33.2 s** (3, then 43 at 46.3 s, 219 at 143 s); the first failed probe was at **68.6 s** (`QCError: no response to SetlistPosition within 2500ms`); five failed probes in a row at 143.0 s ended the watch; `close()` returned in 0.2 s with no error and no hang. The owner did not record when the button was pressed, so the detection latency is unknown. **Findings:** (1) the transport raises no exception when the unit vanishes: writes return error codes that are only counted in `write_errors`; (2) probes returned ok at 33.2 s and 46.3 s although the reader thread was already dead, most likely answered from messages already buffered (my inference, not verified), so a successful read is not proof the unit is alive; (3) the fastest signals were the reader thread dying and write errors rising, about 35 s ahead of the first failed probe; (4) calls blocked for seconds as the unit went away (gaps between probes reached about 20 s), so liveness checks must run off the job thread with their own timeout. **Consequence for P4 (my recommendation, not tested):** treat the unit as gone when the reader thread is dead or the write-error count rises on consecutive samples, keep the 30-minute idle exit as the backstop, and never rely on a read succeeding. The reader thread's liveness is a private attribute of the vendored transport, so exposing it means a deliberate change to the vendored copy (the checksum test will show it) or a wrapper. **Not tested:** USB unplug (`--label unplug`) and reconnecting after the unit comes back. |
| G3 | `g3_build_and_save.py` | **PASS for writes, read-back and save; not tested across a power cycle** (owner's unit, CorOS 4.1.0, 2026-11-05) | Into an empty, open, blank My Presets 3A (slot checked empty by a fresh listing twice), the transport placed all five blocks on row 0 (each `add_block` echoed), routing read back in=1 out=19, and every `set_param` wire value read back exactly as written: drive 0.6/0.3/0.5, amp 0.45 and 0.3, cab LEVEL 0.5, Low-High Cut HPF 0.25, Spring MIX 0.35. Every block's N was again its XML list + 1 (drive 4, amp 8, cab 22, EQ 6, reverb 6). `write_preset_file` (File CREATE, no RecallPreset SAVE) then saved it as \"QC2 spike G3\": the owner's photo shows the unit displaying \"3A QC2 spike G3\" with five blocks, and a fresh directory listing about 3 minutes later lists it at 3A (position 16). **Finding:** a listing taken 3 s after the save still showed the slot blank, so the script called it inconclusive; the later listing had the name. The agent must therefore not read an immediate re-list as proof of failure, and the spike now polls up to 6 times at 6 s. The exact lag (between 3 s and about 3 min) was not measured. The first `--save` attempt was refused because the live buffer still held the `--apply` blocks (grid not blank): the guard worked and nothing was written. **Afterwards the owner deleted the saved test preset from the unit**, so persistence across a power cycle was never tested and G4/G6 run on the unsaved live layout (`--unsaved`). **Not established:** persistence across a power cycle; that the blocks' parameter values on the unit's screen match (G4 covers three of them); whether the screen showed the blocks during the apply-only run. |
| G4 | `g4_taper_check.py` | **PASS for four points** (owner's unit, CorOS 4.1.0, 2026-11-05) | Four wire values were written by the script to the live buffer of the test layout and the unit's screen was photographed (the numbers below are my reading of the owner's photos): Brit 2203 OUTPUT wire 0.25 = **-10.0 dB** (taper -10.00; linear would be -42); cab LEVEL mic 1 wire 0.01 = **-21.8 dB** (taper -21.82; linear -39.54); the same wire 1.0 = **+6.0 dB** (taper +6.0, so the symbolic maximum MAX_CABSIM_DB is 6); Low-High Cut HPF FREQ wire 0.25 = **217 Hz** (taper 217; linear about 5015). The same photos show GAIN wire 0.45 = 4.5 and MASTER wire 0.3 = 3.0 (the 0..10 dials are linear), the untouched second-mic LEVEL = 0.0 dB, and LPF FREQ at 16k0 Hz. Console read-back was seen (exact) for the first two points, not for the last two. **Not covered:** skew 0.6 and LOG_SKEW parameters; the other symbolic bounds (EQ dB, mixer, FX-loop, tempo, EQ frequency); cab OUTPUT VOLUME (-96..+12, skew 4.93), which rests only on the G1 inference that wire 0.5595 is 0 dB; and the cab minimum of -40 dB at wire 0: the -21.8 point depends on the minimum and the skew together, so it fits the pair but does not pin either alone. Four points were taken instead of nine, at the owner's request. |
| G5 | — | not a device gate; settled in the P3 chat | — |
| G6 | `g6_scenes.py` | **PASS for scenes A and B** (owner's unit, CorOS 4.1.0, 2026-11-05) | `--setup` made Myth Drive GAIN scene-varying (A wire 0.2, B 0.8) and the Spring bypass scene-varying (A on, B bypassed). The unit's read-back (console, seen): GAIN values `[0.2, 0.8, 0.6, 0.6, 0.6, 0.6, 0.6, 0.6]` with `scene_mode` true; the Spring's bypass `[False, True, False, False, False, False, False, False]` with `scene_mode` true; the unit confirmed Scene A. Screen (owner's photos; I read the values from the images): Scene A shows GAIN **2.0**, the scene icon on GAIN, and the Spring active in the chain row; after `--goto B` the scene indicator reads **B**, GAIN **8.0**, and the Spring is dimmed (bypassed) while the other four blocks stay bright. The `--goto B` console (seen): the unit confirmed Scene B and the read-back was identical to Scene A's, so the eight-value GAIN list and the bypass list do not change with the active scene; the active scene has to be read separately (a `Scene` read). **Findings:** (1) per-scene values come back as a list of eight, one per scene A-H, with `scene_mode` set on the param; (2) the scenes that were not written (C-H) **kept the param's earlier value (0.6), they did not copy Scene A**, so the export must write all eight scene values explicitly (section 4 assumed C-H copy A); (3) in the bypass section every entry's row and column fields are 0 and the position is the array index (4 rows of 8 columns), and the Spring at array position 4 is the one with `scene_mode` true; its unwritten scenes C-H stayed not-bypassed. **Not covered:** scenes C-H on the screen; per-scene bypass on blocks other than the reverb; more than one scene-varying param; scene labels. |

**P1 status (catalog PR, branch `claude/catalog-values`).** Done: `skew` imported (numeric as is, `LIN_SKEW` 1.0, `LOG_SKEW` 0.3, anything else an import error); `param_count` added per model (full list, hidden included; the device's N is `param_count + 1`) with `Model.ValidParamIndex`; `firmware_constants.go`; `Param.ToWire`/`ToDisplay`/`Bounds` that refuse. Decisions: symbolic bounds stay symbolic in `qc_catalog.json` and are resolved at call time, so the committed catalog diff is only `skew` and `param_count` and the pipeline's range checks (#120) behave as before; switching those checks to `ToWire` is a follow-up (they work on bounds merged across models and deliberately skip unverified display scales, which `ToWire` does not model). Still unconfirmed on the unit: skew 0.6, `LOG_SKEW`, every non-cab symbolic bound, and the cab minimum at wire 0.

**P2 status (device export PR, branch `claude/device-export`).** Built: `internal/deviceexport` with the section 4 rules, wired into `send_preset_to_device` (replacing `UnavailableExporter`), `get_preset(format=device)` and the UI's per-guitar "Download device JSON" link (`GET /api/preset/device_export`). The catalog gained `clones` and `skew_symbol` (re-imported from the same f634ecd snapshot; nothing else changed) and `EmbeddedSHA256`; the per-mic labelling, name key, aliases and unit table moved from `internal/agents` to `internal/catalog/labels.go` unchanged (the Architect's parameter context renders byte-identical). Owner decisions: strict on unconfirmed skews and bounds, plugin models allowed for owned plugins, shared names refused, pre-v4 Bypass refused. On the three v6 eval goldens nothing is pushable yet (captures, mic choices, switches, skew 0.6 and EQ bounds are the usual reasons); a synthetic G3-shaped preset is.

**How G1 was read.** The device's N for each block (length of its params array in a full-preset read) was compared with X, the block's own `<Parameter>` list in `ModelRepo.xml` (what qc-mcp indexes by), O, our catalog's `clones=`-expanded list as listed, and EFF, the XML list with `clones=` resolved and hidden params included. (The first version of the script compared N with O only; O undercounts because our catalog drops hidden params from its list, and the device adds a trailing slot. The script's verdict now uses N = EFF + 1. Fixed in the same PR.) A count match is necessary, not sufficient: names are matched by value shape here and still need the screen (G4).

**Observation from preparing G1 (from the XML, not a device).** In `ModelRepo.xml` at `f634ecd`, Spring (M) (id 8006) has no `clones=`; its 5 parameters are its own. So in "bb king test 2" the **cab** is the cloned block that decides G1, and the Spring reverb is a control (X and O agree for it).

### 7.2 G1 findings (owner's unit, CorOS 4.1.0)

**Measured** (console output pasted by the owner; preset "bb king test 2", unsaved edits: none):

| Block | hash | N (device) | EFF (XML, hidden included) | X (XML own list) | O (our catalog, listed) |
|---|---|---|---|---|---|
| Jewel | 5011 | 6 | 5 | 5 | 5 |
| Green 808 | 5 | 4 | 3 | 3 | 3 |
| US Prince | 1120 | 5 | 4 | 4 | 4 |
| 110 US PRN C10R (M), clones 12000 | 12045 | 22 | 21 | 2 | 19 |
| Spring (M) | 8006 | 6 | 5 | 5 | 5 |

- **N = EFF + 1 for all five blocks.** There is one extra slot after the XML's list on every block, with value 0.0 across all eight scene values.
- **The cab's layout matches our expansion.** Positions 1 and 9 hold the two per-mic IR-name strings (`ME_110 US PRN C10R_Dynamic 57`, `..._Ribbon 160`), exactly the base's `replaces="1"` and `"9"` slots. Positions 19 and 20 hold empty strings, the base's two hidden `IR NAME` params, which our catalog drops from its list but whose indexes it keeps. Position 21 is the extra slot. HPF (16) is 0.0 and LPF (17) is 1.0, the min and max ends of the base's defaults (20 Hz, 20 kHz).
- **qc-mcp's unexpanded reading is wrong for cabs:** it lists 2 parameters where the device has 22.
- **Every param reports 8 values** (scenes A-H), even non-scene ones (replicated). Scene-assigned params differ by scene (Green 808 LEVEL 0.50 / 0.65; US Prince VOLUME 0.35 / 0.42 / 0.30 ...). P5's read-back import has to take this shape into account.

**Inference, not measured (labelled):** the stored wire values 0.5 (US Prince OUTPUT; cab LEVEL, both mics) and 0.5595 (cab OUTPUT VOLUME) equal the taper formula's output for **0 dB** to four digits, with skews 3.8018 / 4.9594844 / 4.93 and ranges -60..+12, -40..+6 (the symbolic CABSIM bound) and -96..+12: `((0-min)/(max-min))^skew` = 0.5000 / 0.5000 / 0.5595. A linear map would give 0.8333 and 0.8696. This assumes those params sit at the XML default of 0 dB in this preset; the screen was not read. It supports the P1 constants but G4's screen readings are the real test.

**Not established:**
- Only **one** cloned model was checked (a cab cloning 12000). The other clone families (e.g. shimmer reverbs whose appended params follow the base) are unverified, so "all 196" is not settled.
- Parameter **names** per position are not confirmed against the unit's screen, only the value shapes above.
- What the extra trailing slot is. qc-mcp documents the bypass control at index N (one past the whole read), which would make this slot something else. **Unknown.** Until it is understood, the device export and the agent must treat any index >= EFF as invalid and never write it.

**Consequence for P1:** the catalog's clone expansion stands for the dual-mic cab; no importer change is needed from this result. P1 should add a test that EFF (hidden included) equals the device's N minus one for the models that have been read back, and the agent's allowlist should reject param indexes >= EFF.

**If G1 shows our catalog's clone expansion is wrong,** fix the importer **first**. That also affects PR #120's Architect parameter context and the preset views. **If G2 (c) fails** (the unit going away isn't detected), keep only the idle-timeout exit. **If G3 shows a write misbehaving,** that operation stays out of v1 until it's understood. That includes per-scene parameters (also G6).

## 8. Phased PRs

Each PR merges to `main`, which **auto-deploys** (CLAUDE.md). The GSR, gemini-code-assist and CodeRabbit bots review every PR.

| PR | Scope | Device? | Risk | Tests | Depends on |
|---|---|---|---|---|---|
| **P0-a** | Phase 0 spike scripts under `tools/device-spikes/`, built on qc-mcp's transport in library mode (the trial's `qc-trial\qc-mcp` clone at `f634ecd` is fine for spikes): G1 read-back dump, G2 shortcut-start and disconnect checks, G3/G4/G6 checks. Results written up in this doc. | **yes** (owner runs) | low; read-mostly, one write to an empty slot | Manual. Outputs pasted into the PR. | — |
| **P1** | Catalog: import `skew`; `firmware_constants.go` with the symbolic-bound values. These are data points taken from pyquadcortex's published measurements, with MIT attribution and no pyquadcortex code, and are confirmed on the owner's unit by G4. `Param.ToWire` / `ToDisplay` that refuse rather than clamp; **clone fix if G1 requires it**; re-import `qc_catalog.json` (`-diff` reviewed). | no | low–medium; the catalog feeds the pipeline | Golden vectors: G4's on-device readings, plus qc-mcp's measured points (Gain LEVEL 0 dB at 0.5, mixer −40..+12). Every symbolic bound resolved or listed as unmeasured. `go test ./...` plus the e2e suite. | G1, G4 |
| **P2** | Device export: `internal/deviceexport` (section 4), plus `get_preset(format=device)` on MCP and a "Download device JSON" link. | no | medium; mapping rules | Golden exports for 3 eval presets. Round-trip display → wire → display. Every id and index exists. Refusal cases. | P1, PR #120 |
| **P2b** | Section 4.1's rule changes (device-XML skews, `LOG_SKEW` and pyquadcortex bounds labelled; switches via qc-mcp's linear position encoding; default cab mics as at-default; importer keeps `default_string`) and capture blocks in the export (section 4.2). Plus the Architect-side fix so pans and 0..1 controls are written on the catalog's scale. | no | medium; changes what a push writes | Goldens regenerated and reviewed; a census of how many eval chains become pushable; refusal tests for what stays refused; capture block export tests. | P2 |
| **P3** | Job queue and tools: `internal/devicejobs` (job schema, HMAC sign/verify, jobs-bucket config `DEVICE_JOBS_BUCKET`, `DEVICE_JOB_KEY[_NEXT]`); MCP tools `device_status`, `send_preset_to_device` (dry run, then confirm token), `get_device_job`, `list_device_slots`, `read_device_preset`; server instructions and style guide updated. | no (fake agent in tests) | **high**: auth, storage, new attack surface | `httptest` MCP integration tests as in `internal/mcpserver/*_test.go`, with an in-memory store. Signature, expiry and replay tests. A golden dry-run. | P2, G5 |
| **P4** | Home agent, including **capture placement** (section 4.2): `deploy/qc-agent/` (Python; vendored qc-mcp transport behind a `DeviceBackend` interface, plus the vendoring checksum test), allowlist, owned-slot ledger, heartbeat, hash-pinned lockfile, `install-shortcut` / `uninstall-shortcut` (desktop shortcut), single-instance lock, idle and disconnect exit, Windows setup guide. | build and tests no; acceptance **yes** | **high**: writes to hardware | Offline: a fake `DeviceBackend` that records calls, reusing the `tools/qc-mcp-trial` harness pattern. Every refusal path. A job-signature test vector shared with P3's Go tests. | P3, G2, G3 |
| **P4b** | `sync_device_library`: a read-only job and MCP tool that upload the unit's capture list to QC-2, the dry run's capture resolution against it, and the reviewed alias table. Feeding the list to the Librarian is a follow-up. | read-only job | medium; untrusted device text | Fake-agent tests; hostile capture names; exact and alias resolution; a missing capture is unresolved. | P4 |
| **P5** | Read-back import: device preset → `StructuredPreset` draft (inverse mapping, wire → display), so the phone can say "refine what's on my unit". | acceptance yes | medium | Round trip: push a preset, read it back, compare the two `StructuredPreset`s. | P4 |
| **P6** | Hardware acceptance and runbook: the owner's end-to-end run from the phone, with failure drills (agent offline, slot occupied, wrong firmware, unplugged mid-job). | **yes** | — | A checklist in `docs/design/`. | P4, P5 |
| v2 | Feeding owned captures to the Librarian, `audition_on_device`, owned-slot re-push, scene labels and stomps, IRs, non-default cab mics | mixed | medium | — | P6 |

**Things to watch.**
- P1 touches `internal/catalog`, which several in-flight PRs also touch. Land it with a rebase, not a merge-race.
- A new Go dependency, if any, needs a supply-chain note in its PR. P2 and P3 shouldn't need one.

## 9. Which model does what

Follows `.claude/rules/claude-config/agent-routing.md` and CLAUDE.md's cost policy: bots are the default PR gate, `/quick-review` runs on demand only, and the multi-agent `/code-review high` is reserved for risky changes.

| Work | Who | Why |
|---|---|---|
| Phase 0 spike scripts (P0-a) | **Sonnet 5.5**, main session, medium effort | Small scripts that follow the plan's exact checks. The owner runs them. |
| Interpreting G1/G3/G4 results, and the clone-indexing decision | **Opus 5.5**, main session, or the `opus-verifier` agent | A load-bearing judgment about device behaviour. A wrong call corrupts every push. |
| P1 catalog import and conversion | **Sonnet 5.5**, main session | Well-scoped Go with golden tests. |
| P2 export mapping **design**: name → index for dual-mic cabs, value parsing, grid rules | `planner` agent (**Opus**, high) before coding | The trickiest data rules; easy to get subtly wrong. |
| P2 implementation | **Sonnet 5.5** | Implements the planner's rules. |
| P2b rule changes and capture export | **Sonnet 5.5** | Small, well-specified changes to tested Go code; the decisions are already made (section 4.1). |
| P3 design: job schema, HMAC, bucket permissions, tool UX | `planner` (**Opus**, high), then `opus-verifier` on the R2 and token premises (G5) | Auth and storage are the CLAUDE.md "large/risky" class. The plan rests on external-system premises. |
| P3 implementation | **Opus 5.5**, main session | Security-sensitive. Fewer review rounds pays for the higher per-token cost. |
| P3 review | `security-reviewer` (**Opus**) **and** `/security-review`; `/code-review high` with the effort level stated; GSR `deep-review` label | Required for auth, storage and LLM-content changes. |
| P4 agent: device backend, allowlist, job loop | **Opus 5.5**, main session | Code that writes to the owner's hardware. Device quirks (write stalls, uncorrelated pushes, save echoes) reward the stronger model. |
| P4 desktop-shortcut install and uninstall, Windows setup guide | **Sonnet 5.5** | Packaging and documentation. |
| P4 review | `security-reviewer` (**Opus**) for credential and job handling; `reviewer` (**Sonnet**) for the rest | — |
| P4b library sync | **Sonnet 5.5**, with `security-reviewer` (**Opus**) on the device-text handling | Read-only, but device text is untrusted. |
| P5 read-back import | **Sonnet 5.5** | Inverse of P2's well-tested mapping. |
| Running tests after every change | `test-runner` agent (**Haiku**) | Cheap. Keeps long logs out of the main context. |
| "Where is X / how does Y flow" lookups | `scout` agent (**Haiku**) | Cheap codebase search. |
| Triage of PR review comments | `pr-triage` agent (**Sonnet**, low), with the ledger after round 2 | The repo's standing rule. |
| Fable | **Not used** unless the owner asks | Ask-only per the routing rules. |
| Gemini pipeline | **Unchanged.** No LLM in the push path. | Exact values come from the catalog, not a model. A later v2 item could teach the Architect to keep chains to 8 blocks or fewer. |

## 10. Open decisions for the owner

1. ~~Slot range~~ **Decided (2026-11-05):** creation is allowed in **every** bank (`QC_AGENT_ALLOWED_SLOTS=1A-32H`, the setup script's default). The bridge still writes only into **empty** slots, so the range is a filter, not the safety rule. A narrower range remains possible by re-running the setup script.
2. **Confirm UX:** dry run plus confirm token (proposed), or a single call.
3. ~~Container vs native Windows~~ **Decided:** native Windows. No pyquadcortex, no containers (section 5.1).
4. **Job expiry** (proposed 15 minutes), **idle exit** (proposed 30 minutes) and **polling interval while running** (proposed 3–5 s), settled after G5.
6. ~~Start mode~~ **Decided:** desktop shortcut only. No automatic start.
5. **Read-back import:** should reading a slot automatically create a QC-2 draft (proposed: only when asked)?
7. **Editing existing presets: deferred, a future to-do (owner, 2026-11-05).** v1 never overwrites a slot it didn't create. When this is picked up, the proposed guardrails are:
   - an automatic read-back backup of the old preset as a QC-2 draft before any overwrite;
   - a dry run that names the preset being replaced, with a confirm token bound to that slot **and** that preset name, so the push is refused if the slot changed in between;
   - an optional list of protected banks the bridge never touches.

   This changes the contract (a new job kind or args) and the agent's slot rule, so it needs a contract version bump.

8. **Decided (2026-11-06): no more hand-run device measurement sessions.** Mappings come from the device's own data or named third-party measurements, labelled, and are checked during P6's normal use (section 4.1). **Captures are core to v1** (section 4.2).

## 11. Kickoff prompts for the implementation chats

Each prompt below starts one chat. Chats 1 and 2 can run in parallel now. Chat 3 starts once Chat 2's contract PR is merged. Chat 4 needs Chat 1's G1 and G4 results, and Chat 5 needs Chat 4. Everything referenced is on `main`.

**Chat 1: Phase 0 hardware checks** (Sonnet 5.5)

> Read `docs/design/qc-device-bridge-plan.md` (especially sections 5 and 7) and `docs/research/qc-usb-tools-audit.md`. Branch from main as `claude/device-phase0`. Write the Phase 0 spike scripts under `tools/device-spikes/` for gates G0, G1, G2, G3, G4 and G6. Use qc-mcp's transport in library mode from my existing clone at `C:\Users\bweit\qc-trial\qc-mcp` (commit f634ecd) and the venv at `C:\Users\bweit\qc-trial\venv`. Don't use pyquadcortex, containers or qc-mcp's MCP server. Everything is read-only except one save into an empty slot I name for G3 and G4. Start with G0 and G1: G1 decides whether our catalog's parameter indexes are right for the 196 cloned models, so dump the raw per-block parameter arrays of my "bb king test 2" preset in slot 2A, which has a cab and a Spring reverb. Give me exact PowerShell steps, one gate at a time. I'll paste outputs back. Record each result in section 7 of the plan, and open a draft PR with the scripts and findings.

If this chat runs in the cloud (claude.ai or the phone), not in Claude Code on the Windows PC, add: "You're in a cloud session: push the scripts to the branch and give me commands to pull and run them on my Windows PC."

**Chat 2: job contract, queue and auth on the QC-2 side, P3** (Opus 5.5)

> Read `docs/design/qc-device-bridge-plan.md` (sections 2, 3, 6 and 8) and CLAUDE.md. Branch from main as `claude/device-jobs`. Use the `planner` agent first for P3. Your first deliverable is a small, separate PR: `docs/design/device-job-contract.md`, defining the job and result JSON schemas, the HMAC signing scheme with `DEVICE_JOB_KEY` / `DEVICE_JOB_KEY_NEXT`, expiry and replay rules, the heartbeat file, the jobs-bucket layout, and shared test vectors the Python agent will reuse. A home-agent chat starts only after that contract lands. Then implement P3: `internal/devicejobs` through `storage.Client` with a separate `DEVICE_JOBS_BUCKET`, and the MCP tools `device_status`, `get_device_job`, `list_device_slots`, `read_device_preset` and `send_preset_to_device`. `send_preset_to_device` defaults to dry-run with a confirm token, and refuses when no heartbeat is newer than 2 minutes. The device export (P2) doesn't exist yet, so take it as an opaque, versioned payload behind an interface and test with fixtures. Settle gate G5 (R2 polling cost, bucket-scoped tokens) and label anything unverified. Follow CLAUDE.md's security rules: run `/security-review` and the `security-reviewer` agent, and add the `deep-review` label on the PR. Use `test-runner` for tests and `scout` for lookups.

**Chat 3: the home agent, P4** (Opus 5.5; start after Chat 2's contract PR is merged)

> Read `docs/design/qc-device-bridge-plan.md` (sections 5, 5.1, 6 and 8) and `docs/design/device-job-contract.md`. Branch from main as `claude/qc-agent`. Build P4 under `deploy/qc-agent/` in Python:
> - Vendor qc-mcp's `transport.py`, `protocol.py`, `backend.py`, `winhid.py`, `directory.py` and `descriptors/*.pb` from lexasoft123/qc-mcp at f634ecd into `vendor/qc_mcp/`, with the upstream LICENSE, a `PROVENANCE.md`, and a test that checks each file's SHA-256 against f634ecd.
> - Put a `DeviceBackend` interface in front of it, with a fake backend for tests.
> - Write the job loop: poll the jobs bucket, verify the HMAC with the contract's test vectors, check expiry, re-validate every operation against the agent's own catalog copy, check firmware and that the slot is empty from a fresh listing, keep the owned-slot ledger, write the heartbeat, and exit after 30 idle minutes or when the unit disconnects.
> - Add `install-shortcut` / `uninstall-shortcut` (desktop shortcut only, no Task Scheduler, no services, no PowerShell scripts) and a hash-pinned lockfile.
>
> Never call qc-mcp's `save_preset`, global I/O, EQ, master-volume or mode operations, or raw `send`. Reuse the rules and fake-device harness pattern from `tools/qc-mcp-trial`. No pyquadcortex, no containers. Hardware acceptance waits for Phase 0 gates G2 and G3. Run `/security-review` and the `security-reviewer` agent before the PR.

**Chat 4: catalog values, P1** (Sonnet 5.5; needs Chat 1's G1 and G4 results)

> Read `docs/design/qc-device-bridge-plan.md` (sections 4, 7 and 8, including the Phase 0 results now recorded in section 7) and the audit's §2.2. Branch from main as `claude/catalog-values`. Implement P1:
> - import the `skew` attribute, mapping LIN_SKEW to 1.0 and LOG_SKEW to 0.3; any other symbolic value is an import error;
> - apply the clone-indexing fix if G1 requires it;
> - add `firmware_constants.go` with the symbolic bounds as G4 confirmed them, citing pyquadcortex's published measurements as data with MIT attribution and no code;
> - add `Param.ToWire` / `ToDisplay`, which refuse out-of-range values, NaN and unmeasured bounds rather than clamp;
> - re-import `qc_catalog.json` and review the `-diff`.
>
> Golden tests come from G4's on-device readings. If the clone fix changes parameter indexes, check that `internal/agents/catalog_params.go` and the preset views still pass. Show `go test ./...` and the e2e results.

**Chat 5: device export, P2** (the Opus `planner` agent first, then Sonnet 5.5; needs Chat 4)

> Read `docs/design/qc-device-bridge-plan.md` (section 4) and the merged P1 and P3 code. Branch from main as `claude/device-export`. Run the `planner` agent on the mapping rules first:
> - model name to id, including mono and stereo families;
> - parameter name to index for dual-mic cabs, reusing `catalog_params.go`'s per-mic labelling;
> - value-string parsing;
> - Scene A/B handling;
> - grid placement (row 0, at most 8 blocks).
>
> Then implement `internal/deviceexport`, add `get_preset(format=device)`, and wire it into P3's `send_preset_to_device` in place of the fixture. Use golden exports for three eval presets, test round trips, and require zero unresolved items for a push.

**Chat 6: rule changes and captures in the export, P2b** (Sonnet 5.5; P2 is merged. Run it before Chat 3 so the agent is built against the final payload)

> Read `docs/design/qc-device-bridge-plan.md` sections 4, 4.1 and 4.2 and CLAUDE.md, then `internal/deviceexport`, `internal/catalog` and `internal/agents/catalog_params.go`. Branch from main as `claude/device-export-p2b`. Implement P2b exactly as section 4.1's table and 4.2's P2b bullet say. Don't revisit decisions recorded there.
> - Allow device-XML skews, `LOG_SKEW` and the pyquadcortex bounds, with the evidence labels the table names in the payload.
> - Switches use the qc-mcp linear position encoding.
> - Default cab mics count as at-default (importer: keep `default_string`, re-import from the same f634ecd snapshot and check the `-diff`).
> - Captures export as capture blocks.
> - Keep refusing what the table still refuses.
>
> Before and after, run a census over every v6 eval preset (`eval_results/v5_vs_v6_r2/v6_a/PROD/rep*/`): pushable chains and unresolved reasons. Put both tables in the PR. Regenerate the goldens and review the diff. Show `go test ./...` and the e2e results via `test-runner`. Open the PR, subscribe, and address GSR's findings; don't wait for CodeRabbit.

**Chat 3, revised: the home agent, P4, with capture placement** (Opus 5.5; after Chat 6. Replaces the Chat 3 prompt above)

> Read `docs/design/qc-device-bridge-plan.md` (sections 4, 4.1, 4.2, 5, 5.1, 6, 7.1, 7.2 and 8) and `docs/design/device-job-contract.md`, then `internal/deviceexport` (payload and goldens), `internal/devicejobs`, `testdata/devicejobs/`, `deploy/qc-agent/write_agent_env.py` and `tools/qc-mcp-trial/`. Branch from main as `claude/qc-agent`. Run the `planner` agent first on the job loop's state machine and the payload-to-transport mapping, including captures, and show me its plan before coding. Build P4 as section 5 and the P4 row of section 8 describe:
> - vendored qc-mcp f634ecd modules with a checksum test, behind a `DeviceBackend` with a fake;
> - strict job parsing and HMAC per the contract and its vectors;
> - re-validation of every payload operation against the agent's own catalog copy;
> - all eight scene values and bypass flags (G6);
> - G2 unit-gone detection off the job thread;
> - G3 save-lag polling;
> - captures resolved from a fresh capture-directory read, placed as 14000/14001 with `file_name`, and recorded as dependencies in the saved preset (check whether `write_preset_file` adds them itself);
> - the owned-slot ledger, the lock file, idle exit, the desktop shortcut, the hash-pinned lockfile and the Windows guide.
>
> Never call `save_preset`, global, mode, raw-send or `quit_app` operations, and never send `ProductForward`. Offline tests only, against the fake backend, including a full push of each P2b golden. List what P6 should check on the unit. Run `/security-review` and the `security-reviewer` agent, open the PR with the `deep-review` label, subscribe, and reply inline to GSR's security, Architecture and Logic findings. Don't merge. You're in a cloud session: push, then give me the commands to pull and run the tests on my Windows PC.

**Chat 7: library sync, P4b** (Sonnet 5.5; after Chat 3)

> Read `docs/design/qc-device-bridge-plan.md` sections 3.3, 4.2 and 6, and the merged P3, P2b and P4 code. Branch from main as `claude/device-library-sync`. Implement P4b:
> - the read-only `sync_device_library` job (agent) and MCP tool (QC-2) that store the unit's capture list;
> - dry-run resolution of capture blocks against it (exact name, then a reviewed alias table), so a capture that isn't on the unit is unresolved.
>
> Device text is untrusted: bound it, strip it, and render it only through `cell()`. Run the `security-reviewer` agent on the device-text handling. Same PR and GSR routine as before.

**After that:**
- **P5, read-back import** (Sonnet 5.5): "Implement P5 from `docs/design/qc-device-bridge-plan.md`."
- **P6, hardware acceptance from the phone** (Sonnet 5.5, with the unit): "Walk me through P6 from `docs/design/qc-device-bridge-plan.md`."
