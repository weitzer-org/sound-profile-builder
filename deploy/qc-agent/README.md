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
| `sync_device_library` | Complete (P4b), read-only. One fresh directory listing (the push's own read), sent to QC-2 as the unit's capture list: name, key, V1/V2, factory or user, folder. Loads and writes nothing. QC-2's dry run checks captures against it; this bridge still resolves every capture again when it pushes. |
| `push_preset` | Implemented and tested offline against fixtures, but **refused end to end until P2** builds the device export: QC-2's `send_preset_to_device` still uses `devicejobs.UnavailableExporter`. |

## Safety rules (what the agent enforces, in order)

For every job: the HMAC signature and the full contract section 8 check order (all 82 shared vectors pass), then
the unit's firmware must equal `QC_AGENT_EXPECTED_FIRMWARE` exactly and the unit must be a full Quad Cortex.

For a push, before anything is written:
1. the export format is one the agent implements (`qc2-device-export/1`), and the export passes
   `qc_agent/export_validator.py` against the agent's own catalog copy (`internal/catalog/qc_catalog.json` in this
   clone, hash-matched with the export): known models in allowed categories, no plugin/IR/FX-loop/splitter
   models, capture blocks only with the capture block's own controls (plan 4.2), NC_Recorder denied, only listed
   parameter indexes, each below its model's `param_count` (G1: the unit's last slot is never written; capture
   blocks use the catalog's `param_count` for 14000/14001, which must agree), with numeric ranges (the cab bounds confirmed on the unit, or pyquadcortex's labelled values),
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
   only, and a known version: for a factory capture V1 or V2 from the nearest `Factory Captures V1`/`V2` folder at or
   above its own, and for a user capture its version field `1` or `2` taken literally. Anything else refuses the
   push before any write;
6. the job hasn't expired (re-checked just before the first write).

Then it builds the chain on the live grid in three phases: it places every block; reads the grid **once** and
checks that each column holds the planned model and reports exactly `param_count + 1` parameters (G1), stopping
before any value is written if not; then, block by block, re-reads that column (the same check, so a block swapped
on the unit mid-build stops the push; the wire guard allows a block's bypass control only after such a read) and
writes the values (all eight scenes when A and B differ, G6) and **all eight bypass flags on every block**, through
the block's bypass control at `param_count + 1`. Before **every**
write it re-reads which slot is open (a write lands in whatever preset is open) and checks the liveness verdict.
It reads the grid back and diffs it; controls it didn't write are compared with the catalog default and only
**logged** (`default differs: ...` in `agent.log`), never failed. If the diff is clean and the export carries
`stomps`, it then assigns each block's **footswitch** (qc-mcp's `assign_stomp`: primary, latching, never H), with
the same open-slot and liveness checks before each, reads the grid again and fails the job unless the assignments are
exactly the planned ones (none missing or extra, none secondary or momentary) and the values still match. Which
physical switch each index is stays **unverified** until step 16. It saves only if the diff is clean, a second
fresh listing still shows the slot empty (and still resolves every capture to the same file), the grid re-read
after that listing is still the verified one (footswitches included), and the slot is still open. A save the listing doesn't show within
six polls (about 2 minutes) is reported **failed** ("may still land: check the slot"), and the slot stays in the
owned-slot ledger. A capture is placed with qc-mcp's `set_capture` (a V1 14000 or V2 14001 block, then its
`file_name` = key + name) before its controls are written, goes through the same `param_count` check, and its
`file_name` is checked in the read-back. The save is `write_preset_file` (File CREATE),
never `save_preset`. It adds no dependency entries; the result says whether the unit's listing names the capture
as a dependency of the saved preset.

Three layers keep qc-mcp's transport to that (`qc_agent/qcmcp_backend.py`): a SHA-256 pin check of everything in
`vendor/`, with each module compiled from the bytes just hashed (the folder is never on `sys.path`); a method allowlist with a never-list (`save_preset`, `recall`, `clear_grid`, deletes, raw
`send`/`request`, global I/O, EQ, master volume, mode, model presets, stomps, IRs, typed writes, splits, scene labels;
`assign_stomp` opens for one call at a time, only in a push's footswitch step; `unassign_stomp` never);
and a **wire guard** that decodes every outgoing message and lets through only session plumbing, reads, and, while a
push is armed, exactly the plan's grid writes (including each capture's `file_name`, once, as the exact resolved
string; never a write at or past the model's `param_count`; scene assignment only for the plan's scene-varying
params and each block's bypass control), then in the footswitch step only the plan's assignments, one per message,
each once, with no value, scene or routing write after it, and one File CREATE to the armed slot and name. Nothing ever sends
ProductForward or a capture command.

Liveness (G2 c): a watchdog thread samples the transport's reader thread and write-error counter once a second
while the unit is open, without calling into the transport. The unit counts as gone when the reader thread is dead,
when the write-error count rose on two consecutive samples, or when the watchdog's last verdict is older than 5 s.
Results carry only the contract's section 9 fields.

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
3. Write `agent.env` if you don't have one yet (`py -3 "$Q\deploy\qc-agent\write_agent_env.py"`; its
   `DEVICE_JOB_KEY` must equal the server's Fly secret). Then check everything offline, without touching the unit
   or the network (an existing `agent.env` is checked against the current rules too):
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

**Results of the first run (P6, 2026-11-06):** plan section 7.3 has the table. Passed: steps 1-3, 9 (occupied slot),
11 (the save lands and survives a power cycle, but every push reports `device_error` "may still land": follow-up F1),
15 (both faders read as pushed) and 16 (C and G as planned, latching, H untouched, kept across a power cycle); step 2
is G2 (a). Partly: 12 (a downloaded capture only; factory captures wait on F2), 13 (bypass flags and defaults, no
watchdog run) and 14 (the sync stores the list but drops factory captures: F2). Not run: 4 (slot argument), 5, 6, 7,
8, 10, the unplug part of 13, and the cab mic write of 14. Load a pushed preset again (another slot, then it) before
reading its name or running a spike on it: right after a push the live grid reports an empty name.

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
   - an empty slot that isn't open on the unit: `unsupported_arg` ("Select slot ... first"; the message says what the unit
     reports as open, with the collection name);
   - an empty slot open in a collection other than My Presets (e.g. your own): the push builds there, not in My Presets.
     **Unverified on the unit until you run it:** check the preset appears in that collection and My Presets is unchanged;
   - an empty slot outside `QC_AGENT_ALLOWED_SLOTS` (narrow it to `24H` to test): `slot_not_allowed`.
10. **Before the first real push: are position replies correlated?** The push reads which slot is open several
    times, and the bridge accepts only the SetlistPosition reply that carries its own `request_id` (no fallback to
    an uncorrelated or late reply). If the unit doesn't echo the id, every push fails closed with `device_error`
    ("no reply to SetlistPosition carrying request_id ...") before writing anything. So check once, with the Phase 0
    spike tools, that SetlistPosition and Scene replies carry the request's id, and tell me if they don't: pushes
    would then need another freshness check.
11. **First real push (after P2).** Select empty **24H** on the unit, dry-run and confirm on the phone. Expect
    `succeeded`, `mismatches 0`, the name in 24H on the unit, then power-cycle and check it's still there (G3 never
    tested persistence across a power cycle). A `failed` result saying the save "may still land" means the listing
    never showed it within about 2 minutes: check 24H on the unit and tell me what it shows.

12. **Captures (P4c; normal use, no extra session).**
    - Before the first capture push, run `list_device_slots` once and send me the `capture folder ...`,
      `capture census: ...` and `My Presets: ... dependencies` lines from `agent.log` (logged once per bridge start).
      Each folder line shows the folders above it, its flags, file count, the version-like fields and the V1/V2 a push
      would use. First run (2026-11-06, CorOS 4.1.0): the factory captures sit in per-device folders with an empty
      version field and the downloaded ones in a folder named `''` with version `1`/`2`; whether a `Factory Captures
      V1`/`V2` folder sits above the factory ones is what the census answers next. If none does, every factory capture
      is refused until a fix that reads V1/V2 from wherever the census shows it.
    - Push one preset with a **factory V1** capture, one with a **factory V2** capture and one with a capture you
      **downloaded** (already on the unit); include a pedal capture if you can. For each: `succeeded`,
      `mismatches 0`, the message's V1/V2 matches the block on the unit ("Capture" vs "Capture 2"), the capture
      name shows on the block and it sounds right, and GAIN..VOLUME match the dry run in scenes A and B. Note the
      dependency sentence.
    - Power-cycle and re-select each slot: the capture still loads (no "missing capture"), values kept.

13. **The G1-G6 rules on the unit (this bridge's own push path; normal use).** On the first pushes:
    - none fails with "The placed blocks don't match the bridge's catalog" (that would mean a block's parameter count
      on the unit differs from `param_count + 1`; send the log line);
    - every block in the push, not only one that varies by scene, reads back its eight bypass flags as planned. G6
      checked the bypass control at `param_count + 1` on the Spring only; this is the first check on amps, drives,
      cabs, EQs and capture blocks;
    - (no check needed) a block's stomp footswitch acts across all scenes, not per scene, even though the bridge
      writes the block's bypass per scene (owner, on the unit, 2026-11-06);
    - send the `default differs` and `default not compared` lines from `agent.log` (they show whether every control of
      a newly placed block starts at its catalog default; never a failure);
    - unplug the USB cable in the middle of a push once: the job should fail with `unit_absent` within a few seconds
      and the bridge should exit (the watchdog runs beside a blocking call only if ctypes releases the GIL during the
      HID call, which ctypes documents but nobody has run on Windows here).

14. **Library sync and cab mics (P4b; read-only, then normal use).** Pull `main` on the PC first (the new job kind and the cab mic write are in this bridge; an older bridge ignores `sync_device_library` and the job expires unanswered).
    - **Sync.** Unit on, Cortex Control closed, bridge running. Phone: `sync_device_library`. Expect `succeeded` and a count
      close to what the unit's own capture lists show. Send me the counts (factory/user, V1/V2, "no V1/V2 marker", cloud
      only, "left out") and the `capture folder ...` / `capture census: ...` lines from `agent.log`. Two things are
      **unverified** here: that a `Factory Captures V1`/`V2` folder sits at or above each factory capture's folder
      (otherwise every factory capture shows "no V1/V2 marker"), and that the cloud flags mean what the bridge assumes.
      The sync carries at most 5000 captures (the first run refused at 1500 with 2073 on the unit).
    - **Names.** Dry-run a preset that uses a factory capture. If QC-2's library calls a capture something else than
      the unit does (for example a trailing ` 2`), the dry run says "not on your unit". Send me both names: they fill
      the reviewed alias table (`internal/devicelib/capture_aliases.json`), which is empty until you do.
    - **A capture you don't have.** Dry-run a preset that uses a capture that is not on the unit: it must say "not on
      your unit", with no confirm token.
    - **Cab mics.** Select a preset on the unit whose cab uses a mic other than the cab's default, phone:
      `read_device_preset` (the answer says how many cab mics it recorded). Then dry-run a preset that asks for that
      mic on the same cab: the plan shows "cab mic written exactly". Push it into an empty slot: expect `succeeded`,
      `mismatches 0`, and the cab on the unit shows that mic. Power-cycle and re-select: the mic is still set. This
      is the first time the bridge writes a string into a cab, so these are **unverified** until you run it: whether
      the unit accepts the string, and whether writing it resets the cab's other controls (the bridge writes the mic
      before the cab's values, so a reset would not lose them). If the cab sounds dead or shows no IR, tell me and
      delete the preset.

15. **Faders (P2d; normal use, no extra session).** A fader (the Graphic-9 and Parallax bands, CA John's 2C tone faders)
    is pushed with a **linear display-to-wire guess** that nobody has read on the unit: the dry run marks it † and
    the export labels it `encoding:fader-linear-guess`. It is **UNVERIFIED** until you do this:
    - Dry-run and push, into an empty slot, a preset with a **Graphic-9 (or Plugin Graphic-9) block whose 65Hz band is
      +6 dB** (use LPF on the block so the name resolves to the fader model) and a **CA John's 2C Ch1 block whose 80 Hz
      fader is 8**, both off default.
    - Expect `succeeded` and `mismatches 0`. Then read both on the unit's screen: the band should show **+6.0 dB** and
      the CA John's fader **8.0**. Check one more band at a negative value (for example -6 dB) if you like.
    - Phone: `read_device_preset(import=true)`; the imported values should agree with the dry run.
    - If a screen value differs, send a photo. The linear rule is then wrong for that fader type; the label stays a guess.

16. **Footswitches (stomp layout, entry point 1; normal use).** Pull `main` on the PC first. On the first push whose
    dry run lists a **Footswitch layout**, expect `succeeded`, `mismatches 0`, and the message "N footswitch(es)
    assigned and read back". Then check **by eye**, with the unit in **Stomp** mode (you switch modes; the bridge never
    does):
    - each block sits on the switch the dry run named: the switch's label/LED on the unit matches the layout
      (A-D top row, E-H bottom row). Index 0-6 = A-G comes from qc-mcp's source and is **unverified** until you see it;
    - pressing each switch toggles its block on and off (latching, not momentary), and H is untouched;
    - power-cycle and re-select the slot: the assignments are still there.
    If a block is on the wrong switch, or a switch doesn't toggle its block, tell me which (dry-run line and what the
    unit shows) and delete the preset. A push that fails with "footswitch assignments differ" saved nothing; send me
    the `read-back after the footswitches` lines from `agent.log`.

17. **Scene layouts (two guitars in one slot, or one guitar on other scenes; normal use).** Pull `main` on the PC first
    (the agent's validator changed). Dry-run a push with `second_guitar` and the four scene letters (for example
    ES-339 on A/B and Telecaster on E/F). The plan's table has one column per used scene; scenes nobody takes are
    named under it and carry the first guitar's rhythm values. Select the empty slot on the unit, confirm, and expect
    `succeeded`, `mismatches 0`. Then step through the scenes on the unit **by eye**: A and B show the first guitar's
    rhythm and lead values, E and F the second guitar's, and a scene you did not assign (C, D, G, H) matches Scene A.
    A bypass that differs between the guitars (a block on for one, off for the other) must follow the scene, but its
    footswitch toggles it in every scene. Power-cycle and check again. **Unverified on the unit until you run it.**
    A push the agent refuses with "scenes with the same source must carry the same values" means the PC is on an
    older clone: pull `main`.

19. **Capture sync evidence (P6 follow-up F2, phase 1; read-only).** Pull `main` on the PC first and restart the bridge.
    From the phone run `sync_device_library`, then paste from `agent.log` every line starting `capture skip census`
    and `capture skip sample` (per folder: how many files are kept and how many are left out for each reason:
    favorites / name length / name not printable ASCII / name changed by clean-up / key not 64 hex), plus the
    `capture census:` summary line. The samples show lengths and character classes only, no names or keys. Nothing about
    what is synced changes in this step; the fix follows once the lines show why the factory files are left out.

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
