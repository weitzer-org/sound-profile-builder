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
  - It is **not** always running. Task Scheduler starts it when the Quad Cortex connects over USB, or the owner starts it from a shortcut.
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
Home: Windows PC ─ qc-agent (on demand: starts on QC USB connect) ──┘
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
| `sync_device_library` | v2 | `directory_summary`, `search_directory`, `list_favorites` | Uploads the unit's **capture, IR and preset names** to QC-2. The pipeline can then prefer captures the owner actually has; today `user_captures.json` is hand-maintained. This text is device-sourced and untrusted (section 6). |
| `audition_on_device` | v2 | `recall_preset`, `switch_scene` | Recalls a slot this system created, or switches its scene. |
| `send_preset_to_device` overwrite | v2 | `save_preset` | Re-push into a slot **this system created**, tracked in the jobs bucket. Never into other slots. |
| Scene labels, stomps, captures/IRs on push | v2 | `set_preset_meta`, `assign_stomp`, `add_capture`, `add_ir` | Captures and IRs only if already on the unit (`sync_device_library`). |
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
 "catalog": {"coros": "4.1", "origin": "…f634ecd", "sha256": "<qc_catalog.json hash>"},
 "rows": [{"row": 0, "in_port": 1, "out_port": 19,
           "blocks": [{"col": 0, "model_id": 5011, "model": "Jewel",
                       "params": [{"index": 0, "name": "COMP", "display": 3, "wire": 0.3,
                                   "scene_b": {"display": 5, "wire": 0.5}}],
                       "bypass": {"A": false, "B": false}}]}],
 "scenes": [{"index": 0, "label": "Rhythm"}, {"index": 1, "label": "Lead"}],
 "unresolved": [{"block": "…", "param": "…", "reason": "unmeasured bound MIN_INPUT_TRIM"}]}
```

**Rules**

- **Model name to id.** Resolve through `internal/catalog` (exact name, then family). A family name such as "Spring" with mono and stereo variants resolves to **mono** in v1. Unknown, hidden, internal or plugin models go to `unresolved`. Model 20000 (NC_Recorder) and the whole "Neural Capture Internal" category are refused.
- **Parameter name to index.** Use the catalog `index`. Dual-mic cabs repeat names, so resolve via the per-mic labelling PR #120 introduced (`catalog_params.go` `deviceBlock.merge`). **Blocked on gate G1:** the catalog's index scheme for the 196 models that use `clones=` is unverified (section 7).
- **Value string to number.** Parse the number, unit and switch or step name against the catalog param's `type`, `steps` and `step_names`. Strip the `*` estimate marker. An unparseable value goes to `unresolved`.
- **Display to wire.** Use the taper `wire = ((v-min)/(max-min))^skew`, with symbolic bounds resolved. **Refuse, don't clamp,** any out-of-range value, NaN, or unmeasured bound. This needs P1.
- **Scenes.** `Value` maps to Scene A and `ValueB` to Scene B. A parameter whose B differs from A is assigned to scenes. Scenes C–H copy A in v1.
- **Grid.** Place the chain on row 0, columns 0–7, with in 1 and out 19 (qc-mcp `clear_grid` default routing). **More than 8 blocks is refused** in v1. Splits and parallel paths come in v2.
- **Captures and IRs** are `unresolved` in v1.
- **A push needs zero unresolved items.** A dry run shows them all, so the user can refine first.

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
| NC_Recorder (20000) is placeable | Refuse hidden or internal categories and plugin models, plus an explicit denylist. |
| `save_preset_as` guard reads a stale cache | Use `list_directory` fresh, immediately before the save. |
| Parameter indexes for cloned models are unverified | Blocked on gate G1. |

**Allowlist.** Reuse and extend `tools/qc-mcp-trial/qc_allowlist_proxy.py`'s rules:
- `check_model` and `check_param` against the agent's own catalog copy;
- grid bounds;
- firmware match;
- an empty-slot check from a **fresh** device listing;
- slot ownership from a `jobs/owned_slots.json` that only the agent writes.

The agent exposes **no** server, port or UI. Its only inputs are signed jobs.

**Running it on Windows**

- **Install:** clone this repo, then `py -3.12 -m venv`, then `pip install --require-hashes -r deploy/qc-agent/requirements.lock.txt`.
- **Configuration** goes in `%LOCALAPPDATA%\qc-agent\agent.env`:
  - R2 endpoint, bucket and token;
  - `DEVICE_JOB_KEY`;
  - the allowed slot range;
  - the expected firmware.

  The file is ACL-restricted to the user. No secret goes on the command line.
- **On demand, never always-on** (owner's requirement).
  - **Primary:** `python -m qc_agent install-task` registers a **Task Scheduler** task through `schtasks`. Its trigger is **the Quad Cortex connecting**: a Windows device-arrival event matched on VID `152A` / PID `880A`.
    - The task runs as the current user, not elevated, with a hidden window. Task Scheduler's "don't start a new instance" setting stops duplicates.
    - Which event log and event ID to use is **UNVERIFIED**. Candidates are the `Microsoft-Windows-DriverFrameworks-UserMode/Operational` log, which is off by default and would need enabling once, and the `Microsoft-Windows-Kernel-PnP` device-configuration events. It is gate G2.
  - **Fallback:** a desktop shortcut, "Start QC bridge", running `python -m qc_agent run`.
  - **Either way the agent exits by itself** when the unit disappears (a USB read error or device removal), or after **30 minutes with no job** (`QC_AGENT_IDLE_MINUTES`). It runs nothing and polls nothing between sessions.
  - `python -m qc_agent uninstall-task` removes the task. There are no PowerShell scripts, so no execution-policy changes.
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
| **G2** | (a) Which Windows event reliably fires when the Quad Cortex connects or powers on, and can a non-elevated Task Scheduler task trigger on it? (b) Does a **library-mode** qc-mcp script connect, read and disconnect cleanly when started that way? (c) Does it fail cleanly when Cortex Control holds the unit? (d) Does the idle and disconnect exit work? | Plug and power-cycle the unit while watching Event Viewer. Register a spike task on the candidate event. A second run with Cortex Control open. | yes |
| **G3** | Do qc-mcp's transport writes behave on 4.1.0 when driven with **wire** values? That means `set_routing`, `add_block`, `set_param` with an explicit 0..1, `set_param_scenes`, and `write_preset_file` into an empty slot. | One test preset into an empty slot. Read it back and compare. | yes |
| **G4** | Taper and symbolic-bound constants on **4.1.0**. Our proposed constants come from pyquadcortex's 4.0.1 measurements. They are used here as **data, not code**, so they need confirming on this unit. | Set three known display values through the export path, and read the unit's screen. | yes |
| **G5** | R2 pricing for polling, and bucket-scoped API tokens | Cloudflare docs or dashboard. Record the numbers in the PR. | no |
| **G6** | Scene semantics: do per-scene parameter writes and per-scene bypass behave as qc-mcp documents? | Push a two-scene test preset and switch scenes on the unit. | yes |

**If G1 shows our catalog's clone expansion is wrong,** fix the importer **first**. That also affects PR #120's Architect parameter context and the preset views. **If G2 fails** (no usable device-arrival trigger), use the desktop-shortcut start only. The idle and disconnect exit still applies. **If G3 shows a write misbehaving,** that operation stays out of v1 until it's understood. That includes per-scene parameters (also G6).

## 8. Phased PRs

Each PR merges to `main`, which **auto-deploys** (CLAUDE.md). The GSR, gemini-code-assist and CodeRabbit bots review every PR.

| PR | Scope | Device? | Risk | Tests | Depends on |
|---|---|---|---|---|---|
| **P0-a** | Phase 0 spike scripts under `tools/device-spikes/`, built on qc-mcp's transport in library mode (the trial's `qc-trial\qc-mcp` clone at `f634ecd` is fine for spikes): G1 read-back dump, G2 Task Scheduler run, G3/G4/G6 checks. Results written up in this doc. | **yes** (owner runs) | low; read-mostly, one write to an empty slot | Manual. Outputs pasted into the PR. | — |
| **P1** | Catalog: import `skew`; `firmware_constants.go` with the symbolic-bound values. These are data points taken from pyquadcortex's published measurements, with MIT attribution and no pyquadcortex code, and are confirmed on the owner's unit by G4. `Param.ToWire` / `ToDisplay` that refuse rather than clamp; **clone fix if G1 requires it**; re-import `qc_catalog.json` (`-diff` reviewed). | no | low–medium; the catalog feeds the pipeline | Golden vectors: G4's on-device readings, plus qc-mcp's measured points (Gain LEVEL 0 dB at 0.5, mixer −40..+12). Every symbolic bound resolved or listed as unmeasured. `go test ./...` plus the e2e suite. | G1, G4 |
| **P2** | Device export: `internal/deviceexport` (section 4), plus `get_preset(format=device)` on MCP and a "Download device JSON" link. | no | medium; mapping rules | Golden exports for 3 eval presets. Round-trip display → wire → display. Every id and index exists. Refusal cases. | P1, PR #120 |
| **P3** | Job queue and tools: `internal/devicejobs` (job schema, HMAC sign/verify, jobs-bucket config `DEVICE_JOBS_BUCKET`, `DEVICE_JOB_KEY[_NEXT]`); MCP tools `device_status`, `send_preset_to_device` (dry run, then confirm token), `get_device_job`, `list_device_slots`, `read_device_preset`; server instructions and style guide updated. | no (fake agent in tests) | **high**: auth, storage, new attack surface | `httptest` MCP integration tests as in `internal/mcpserver/*_test.go`, with an in-memory store. Signature, expiry and replay tests. A golden dry-run. | P2, G5 |
| **P4** | Home agent: `deploy/qc-agent/` (Python; vendored qc-mcp transport behind a `DeviceBackend` interface, plus the vendoring checksum test), allowlist, owned-slot ledger, heartbeat, hash-pinned lockfile, `install-task` / `uninstall-task` (Task Scheduler), Windows setup guide. | build and tests no; acceptance **yes** | **high**: writes to hardware | Offline: a fake `DeviceBackend` that records calls, reusing the `tools/qc-mcp-trial` harness pattern. Every refusal path. A job-signature test vector shared with P3's Go tests. | P3, G2, G3 |
| **P5** | Read-back import: device preset → `StructuredPreset` draft (inverse mapping, wire → display), so the phone can say "refine what's on my unit". | acceptance yes | medium | Round trip: push a preset, read it back, compare the two `StructuredPreset`s. | P4 |
| **P6** | Hardware acceptance and runbook: the owner's end-to-end run from the phone, with failure drills (agent offline, slot occupied, wrong firmware, unplugged mid-job). | **yes** | — | A checklist in `docs/design/`. | P4, P5 |
| v2 | `sync_device_library` (and feeding owned captures to the Librarian), `audition_on_device`, owned-slot re-push, scene labels and stomps, captures and IRs | mixed | medium | — | P6 |

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
| P3 design: job schema, HMAC, bucket permissions, tool UX | `planner` (**Opus**, high), then `opus-verifier` on the R2 and token premises (G5) | Auth and storage are the CLAUDE.md "large/risky" class. The plan rests on external-system premises. |
| P3 implementation | **Opus 5.5**, main session | Security-sensitive. Fewer review rounds pays for the higher per-token cost. |
| P3 review | `security-reviewer` (**Opus**) **and** `/security-review`; `/code-review high` with the effort level stated; GSR `deep-review` label | Required for auth, storage and LLM-content changes. |
| P4 agent: device backend, allowlist, job loop | **Opus 5.5**, main session | Code that writes to the owner's hardware. Device quirks (write stalls, uncorrelated pushes, save echoes) reward the stronger model. |
| P4 Task Scheduler install and uninstall, Windows setup guide | **Sonnet 5.5** | Packaging and documentation. |
| P4 review | `security-reviewer` (**Opus**) for credential and job handling; `reviewer` (**Sonnet**) for the rest | — |
| P5 read-back import | **Sonnet 5.5** | Inverse of P2's well-tested mapping. |
| Running tests after every change | `test-runner` agent (**Haiku**) | Cheap. Keeps long logs out of the main context. |
| "Where is X / how does Y flow" lookups | `scout` agent (**Haiku**) | Cheap codebase search. |
| Triage of PR review comments | `pr-triage` agent (**Sonnet**, low), with the ledger after round 2 | The repo's standing rule. |
| Fable | **Not used** unless the owner asks | Ask-only per the routing rules. |
| Gemini pipeline | **Unchanged.** No LLM in the push path. | Exact values come from the catalog, not a model. A later v2 item could teach the Architect to keep chains to 8 blocks or fewer. |

## 10. Open decisions for the owner

1. **Slot range** the agent may write to, e.g. banks 20–25 in My Presets. The agent refuses anything outside it, even if empty.
2. **Confirm UX:** dry run plus confirm token (proposed), or a single call.
3. ~~Container vs native Windows~~ **Decided:** native Windows. No pyquadcortex, no containers (section 5.1).
4. **Job expiry** (proposed 15 minutes), **idle exit** (proposed 30 minutes) and **polling interval while running** (proposed 3–5 s), settled after G5.
6. **Start mode:** automatic on Quad Cortex connect (proposed, if G2 passes), or shortcut only.
5. **Read-back import:** should reading a slot automatically create a QC-2 draft (proposed: only when asked)?

## 11. Starting the new chat

Suggested first message:

> Read `docs/design/qc-device-bridge-plan.md` and `docs/research/qc-usb-tools-audit.md` on branch `claude/qc-usb-tools-audit`. Start with Phase 0: write the spike scripts for gates G1 (clone parameter indexing), G2 (start-on-USB-connect trigger and library-mode qc-mcp) and G3 (qc-mcp transport writes with wire values on CorOS 4.1.0) under `tools/device-spikes/`. Use qc-mcp's transport in library mode from my existing qc-trial clone at f634ecd; don't use pyquadcortex. Keep it read-only except the single empty-slot save in G3. Give me exact Windows/PowerShell steps to run them. Don't start P1–P4 until I've pasted the results.

Use **Sonnet 5.5** for that session (spike scripts). Switch to **Opus 5.5** with `/model` when interpreting the results and for P3/P4.
