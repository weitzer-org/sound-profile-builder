# QC bridge home agent (plan P4)

The program that runs on the Windows PC the Quad Cortex is plugged into. It picks up signed jobs that QC-2 writes
to the R2 jobs bucket, checks them, runs them on the unit, and writes signed results back. It has no server, port
or UI: its only inputs are signed jobs. Design: `docs/design/qc-device-bridge-plan.md` (sections 5, 5.1, 6) and
`docs/design/device-job-contract.md`.

**What works today**

| Job | Status |
|---|---|
| `list_slots` | Complete. Fresh directory listing of My Presets, 256 slots, empty/occupied/owned. |
| `read_preset` | Complete for the **open** preset. A named slot is refused (`unsupported_arg`): recalling changes what's playing. |
| `push_preset` | Implemented and tested offline against fixtures, but **refused end to end until P2** builds the device export: QC-2's `send_preset_to_device` still uses `devicejobs.UnavailableExporter`. |

## Safety rules (what the agent enforces, in order)

For every job: the HMAC signature and the full contract section 8 check order (all 79 shared vectors pass), then
the unit's firmware must equal `QC_AGENT_EXPECTED_FIRMWARE` exactly and the unit must be a full Quad Cortex.

For a push, before anything is written:
1. the export format is one the agent implements (`qc2-device-export/1`), and the export passes
   `qc_agent/export_validator.py` against the agent's own catalog copy (`internal/catalog/qc_catalog.json` in this
   clone, hash-matched with the export): known models in allowed categories, no plugin/IR/FX-loop/splitter
   models, capture blocks only with the capture block's own controls (plan 4.2), NC_Recorder denied, only listed
   parameter indexes with numeric ranges (the cab bounds confirmed on the unit, or pyquadcortex's labelled values),
   values in range, each wire value equal to the catalog's mapping of its display value (taper or switch position,
   with the `evidence` label to match), one row (in 1, out 19), at most 8 blocks, zero unresolved items. The export's
   `warnings` (settings QC-2 left out on purpose, e.g. a non-default cab mic) are carried and change nothing. The preset name may use letters, digits, spaces
   and `_ ( ) + , ! ' & # . -` only (it becomes a file name on the unit);
2. the slot is inside `QC_AGENT_ALLOWED_SLOTS`;
3. a **fresh** directory listing (all 256 slots, or it's refused) shows the slot **empty** (an exactly blank name),
   and no other slot already has a preset with the same name. The bridge never overwrites a preset, including its own
   (editing existing presets is deferred: plan section 10, item 7);
4. that empty slot is the one **open on the unit** and its live grid is blank. This is the only save path Phase 0
   verified (G3). The agent never recalls a preset or clears a grid, so it can't disturb what you have loaded;
5. every capture the preset uses is found on the unit, in the same fresh listing: by exact name, as the same kind
   (factory or user), as a file on the unit (never a cloud-only item; the bridge never downloads captures), one key
   only, and a known version: V1 or V2 from the factory folder, and for a user capture from the version field,
   calibrated against the factory folders on every listing. Anything else refuses the push before any write;
6. the job hasn't expired (re-checked just before the first write).

Then it builds the chain on the live grid, reads it back and diffs it, and saves only if the diff is clean and a
second fresh listing still shows the slot empty (and still resolves every capture to the same file). A capture is
placed with qc-mcp's `set_capture` (a V1 14000 or V2 14001 block, then its `file_name` = key + name) before its
controls are written, and its `file_name` is checked in the read-back. The save is `write_preset_file` (File CREATE),
never `save_preset`. It adds no dependency entries; the result says whether the unit's listing names the capture
as a dependency of the saved preset.

Three layers keep qc-mcp's transport to that (`qc_agent/qcmcp_backend.py`): a SHA-256 pin check of everything in
`vendor/`, with each module compiled from the bytes just hashed (the folder is never on `sys.path`); a method allowlist with a never-list (`save_preset`, `recall`, `clear_grid`, deletes, raw
`send`/`request`, global I/O, EQ, master volume, mode, model presets, stomps, IRs, typed writes, splits, scene labels);
and a **wire guard** that decodes every outgoing message and lets through only session plumbing, reads, and, while a
push is armed, exactly the plan's grid writes (including each capture's `file_name`, once, as the exact resolved
string) and one File CREATE to the armed slot and name. Nothing ever sends ProductForward or a capture command.

Local state in `%LOCALAPPDATA%\qc-agent\`: `agent.env` (config), `executed_jobs.jsonl` (the job ledger, never
pruned), `owned_slots.jsonl` (saves the bridge made), `logs\agent.log` (rotated; no keys, tokens or signatures),
`agent.lock` (single instance).

## Install on Windows (once)

Set these in a PowerShell window. `$Q` is your clone of this repo (the Phase 0 runs used `$HOME\qc-trial\qc2`).

```powershell
$Q = "$HOME\qc-trial\qc2"; $A = "$env:LOCALAPPDATA\qc-agent"; $PY = "$A\venv\Scripts\python.exe"
```

1. Get this branch (after merge, `main`):
   ```powershell
   git -C $Q fetch origin main
   git -C $Q checkout main
   git -C $Q pull --ff-only
   ```
2. Create the venv and install the one dependency, hash-checked:
   ```powershell
   py -3.12 -m venv "$A\venv"
   & $PY -m pip install --require-hashes --only-binary :all: -r "$Q\deploy\qc-agent\requirements.lock.txt"
   ```
3. `agent.env` already exists (you ran `write_agent_env.py`). Check everything offline, without touching the unit
   or the network:
   ```powershell
   cd "$Q\deploy\qc-agent"
   & $PY -E -s -m qc_agent check
   ```
   Expect `all checks passed`. It shows the key id (`72eb33c6`-style), never the key.
4. Run the offline tests (fake bucket, fake unit; about 30 s):
   ```powershell
   & $PY -E -s -m unittest discover -s tests
   ```
5. Put the shortcut on the desktop:
   ```powershell
   & $PY -E -s -m qc_agent install-shortcut
   ```
   It creates **Start QC bridge** on your desktop. To remove it: `& $PY -E -s -m qc_agent uninstall-shortcut`.
   No Task Scheduler task, no service, no autostart, no admin rights.

## Daily use

1. Switch on the Quad Cortex, connect USB, **close Cortex Control** (it holds the unit exclusively).
2. Double-click **Start QC bridge**. The window says `Bridge running: waiting for jobs`, then shows each job.
3. Use the phone. `device_status` should say the bridge is online within a minute.
4. For a push (after P2): **select the empty target slot on the unit first**, then confirm the push on the phone.
5. The bridge exits by itself after `QC_AGENT_IDLE_MINUTES` without a job, or when the unit is switched off or
   unplugged. Closing the window stops it too.

A second double-click while it runs says so and exits. If the window shows `configuration problem`, re-run
`write_agent_env.py`.

## Hardware acceptance (after Phase 0 gates G2 and G3)

G3 has passed (plan 7.1). G2 (c) passed for power-off; **G2 (a) (start from a shortcut), G2 (b) (Cortex Control
holding the unit) and unplug detection have not been run.** Do these in order and paste each window's output.
Before you start, take a Cortex Control local backup, and note an empty slot (the bridge's `list_device_slots` shows
them; this guide uses **24H**).

1. **Offline check passes** (install steps 3-4).
2. **Start and heartbeat (also G2 a).** Unit on, Cortex Control closed. Double-click **Start QC bridge**. On the
   phone: `device_status`. Expect online, the expected firmware, allowed slots `1A-32H`.
3. **List slots.** Phone: `list_device_slots`. Compare a few names and empty slots with the unit's screen.
4. **Read the open preset.** Select any preset on the unit, then phone: `read_device_preset`. Check the block count
   matches the screen. Then try a slot name (`read_device_preset` with a slot): expect `refused / unsupported_arg`.
5. **Cortex Control holding the unit (G2 b).** Leave the bridge running, open Cortex Control and let it connect, then
   `list_device_slots`. Expect `failed / unit_busy` with "close it and retry", and the bridge still running. Close
   Cortex Control and repeat: it should succeed. (You said Cortex Control isn't installed; if so, skip this and tell
   me, and it stays untested.)
6. **Wrong firmware is refused.** Close the bridge. Edit `agent.env`, set `QC_AGENT_EXPECTED_FIRMWARE=4.1.9`, start the
   bridge, `list_device_slots`: expect `refused / firmware_mismatch`, nothing else. Put `4.1.0` back.
7. **Unplug and power-off exit.** With the bridge idle, unplug the USB cable. The window should print `Bridge
   stopped: the Quad Cortex was switched off or unplugged` within a few seconds and `device_status` should say it
   shut down. Repeat with the power switch. (The idle-time detection uses Windows device enumeration and has never
   run on hardware: tell me how long each took.)
8. **Idle exit.** Set `QC_AGENT_IDLE_MINUTES=2`, start, wait: it should stop after 2 minutes. Put `30` back.
9. **Push refusals (works before P2).** These need a push job, which QC-2 can't build until P2. Once P2 is merged:
   - occupied slot: `slot_not_empty`, nothing written;
   - an empty slot that isn't open on the unit: `unsupported_arg` ("Select slot ... first");
   - an empty slot outside `QC_AGENT_ALLOWED_SLOTS` (narrow it to `24H` to test): `slot_not_allowed`.
10. **Before the first real push: are position replies correlated?** The push reads which slot is open several
    times, and the bridge accepts only the SetlistPosition reply that carries its own `request_id` (no fallback to
    an uncorrelated or late reply). If the unit doesn't echo the id, every push fails closed with `device_error`
    ("no reply to SetlistPosition carrying request_id ...") before writing anything. So check once, with the Phase 0
    spike tools, that SetlistPosition and Scene replies carry the request's id, and tell me if they don't: pushes
    would then need another freshness check.
11. **First real push (after P2).** Select empty **24H** on the unit, dry-run and confirm on the phone. Expect
    `succeeded`, `mismatches 0`, the name in 24H on the unit, then power-cycle and check it's still there (G3 never
    tested persistence across a power cycle).

12. **Captures (P4c; normal use, no extra session).**
    - Before the first capture push, run `list_device_slots` once and send me the `capture folder ...` and
      `My Presets: ... dependencies` lines from `agent.log`. They show the real folder names, whether the version
      field calibrates, and whether saved presets carry dependency lists. If the factory folders aren't named
      `Factory Captures V1`/`V2`, every factory capture is refused (the message lists the real names) until a
      one-line fix.
    - Push one preset with a **factory V1** capture, one with a **factory V2** capture and one with a capture you
      **downloaded** (already on the unit); include a pedal capture if you can. For each: `succeeded`,
      `mismatches 0`, the message's V1/V2 matches the block on the unit ("Capture" vs "Capture 2"), the capture
      name shows on the block and it sounds right, and GAIN..VOLUME match the dry run in scenes A and B. Note the
      dependency sentence.
    - Power-cycle and re-select each slot: the capture still loads (no "missing capture"), values kept.

If anything looks wrong: close the window (that stops the bridge), power-cycle the unit if it's unresponsive, and
send me the window's output and `%LOCALAPPDATA%\qc-agent\logs\agent.log`. The log has no secrets, but read it before
you paste it: it names your presets.

## Developing

```bash
cd deploy/qc-agent
python3 -m venv /tmp/qc-agent-venv
/tmp/qc-agent-venv/bin/pip install --require-hashes --only-binary :all: -r requirements.lock.txt
/tmp/qc-agent-venv/bin/python -m unittest discover -s tests
```

The tests use `tests/fake_device.py` (a whole unit behind the `DeviceBackend` interface, recording every call) and
`tests/fake_hid.py` (the unit simulated at the HID-report level, so the real vendored transport and the wire guard run
offline). They prove the agent's rules, not the unit's behaviour.
