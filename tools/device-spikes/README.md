# Phase 0 device spikes

Hardware checks for gates G0-G4 and G6 in [`docs/design/qc-device-bridge-plan.md`](../../docs/design/qc-device-bridge-plan.md) section 7. They run on **your Windows PC**, against **your Quad Cortex**, using qc-mcp's transport **as a library** from your existing clone at `C:\Users\bweit\qc-trial\qc-mcp` (commit `f634ecd`).

No pyquadcortex, no containers, no qc-mcp MCP server, no LLM in the loop. G5 (R2 pricing and token scoping) needs no device and is not here.

## Safety rules the scripts enforce

| Rule | How |
|---|---|
| Only the pinned, unmodified transport | `load_qc_mcp` refuses unless the clone is at `f634ecd` and `git status -- src` is clean |
| Exclusive direct mode only | `QuadCortex(share=False, bridge=False)`; Cortex Control must be closed (except G2b, which needs it open) |
| Each script can call only the transport methods it needs | `Guarded` wrapper; everything else raises. Reads for G0/G1/G2. G3 adds routing, add-block, set-param, per-scene set-param and the save. G4 and G6 add live edits only |
| Never callable, even if a script asked | `save_preset` (RecallPreset SAVE, the documented hang), `recall`, `clear_grid`, `load_preset`, raw `send`/`request`, global I/O, EQ, volume and mode writes, deletes, captures/IRs, stomps, preset meta |
| One save, one slot, one script | Only G3 can save, and only into the empty slot **you** pass with `--slot` (no default). Fresh directory listing before the first write and again before the save; typed confirmation; the save needs a clean read-back first |
| The firmware must match exactly | `--firmware` is required for every write spike |
| No recalls | The scripts never switch presets. You select the slot on the unit, and they refuse if it isn't the one open |

Results go to `C:\Users\bweit\qc-trial\spike-results\` (outside the repo). G1's file also holds the raw preset bytes as hex.

## One-time setup

```powershell
$T = "$HOME\qc-trial"
cd "$T\qc2"
git fetch origin claude/device-phase0
git checkout -B claude/device-phase0 origin/claude/device-phase0
git -C "$T\qc-mcp" rev-parse HEAD     # must print f634ecd3dcf426090b90bc4cc176e4b500494b9e
```

Offline self-test (no unit needed; it fakes the device, so it proves the guards and the logic, not the hardware):

```powershell
$env:QC_MCP_DIR = "$T\qc-mcp"
& "$T\venv\Scripts\python.exe" -m unittest discover -s "$T\qc2\tools\device-spikes" -p "test_spikes.py"
```

Expect `Ran 73 tests ... OK`. Don't add `-I` when running the spikes themselves; they import `spike_common.py` from their own folder.

Before every hardware run: quit **Cortex Control** completely (tray too), connect the unit, and keep a Cortex Control local backup from before the trial.

## The gates, in order

Set these in each new PowerShell window:

```powershell
$T = "$HOME\qc-trial"; $PY = "$T\venv\Scripts\python.exe"; $S = "$T\qc2\tools\device-spikes"
```

| Gate | Script | Writes? | Command |
|---|---|---|---|
| G0 | `g0_verify_slot.py` | no | `& $PY "$S\g0_verify_slot.py" --slot 2A --expect-name "bb king test 2"` |
| G1 | `g1_dump_params.py` | no | select 2A on the unit, then `& $PY "$S\g1_dump_params.py" --slot 2A` |
| G2a | `g2_session.py --check a` | no | from a desktop shortcut, see below |
| G2b | `g2_session.py --check b` | no | `& $PY "$S\g2_session.py" --check b` with Cortex Control open |
| G2c | `g2_session.py --check c` | no | `& $PY "$S\g2_session.py" --check c --label power-off` |
| G3 | `g3_build_and_save.py` | **one save** | preflight, then `--apply`, then `--apply --save` |
| G4 | `g4_taper_check.py` | live edits only | after G3 |
| G6 | `g6_scenes.py` | live edits only | after G3 |

### G0: is the preset in 2A?

```powershell
& $PY "$S\g0_verify_slot.py" --slot 2A --expect-name "bb king test 2"
```

Paste the console output. Note the exact `firmware=` line: every later write spike needs it as `--firmware`.

### G1: raw parameter arrays (the decisive one)

On the unit, select **2A** and let it load without editing. Then:

```powershell
& $PY "$S\g1_dump_params.py" --slot 2A
```

It refuses if the unit doesn't have 2A open, if the preset has unsaved edits, or if the name differs. Paste the console output; it prints, per block, the device's array length **N**, the length under qc-mcp's reading of the XML (**X**), and under our catalog (**O**), and a per-position table for any block where they differ (`--all` for every block). The JSON file in `spike-results` holds the raw preset bytes; keep it, I may ask for it.

How to read it: `N` is the number of readable parameter slots. On the first hardware run every block's N was the XML's full parameter list (clones resolved, hidden params included, called EFF) plus one trailing slot, so the verdict is `EFF_PLUS_ONE` (or `EFF_EXACT`). On a cab, either means the device's count agrees with our clone expansion; `UNEXPANDED_COUNT` would mean it follows qc-mcp's unexpanded list. Counts alone are necessary, not sufficient, so the name table also has to line up with what the unit's screen shows for that block.

### G2: session behaviour

**(a) from a desktop shortcut.** Create it once (typed at the prompt, not a script file):

```powershell
$ws = New-Object -ComObject WScript.Shell
$lnk = $ws.CreateShortcut("$HOME\Desktop\QC spike G2a.lnk")
$lnk.TargetPath = $PY
$lnk.Arguments = "`"$S\g2_session.py`" --check a --pause"
$lnk.WorkingDirectory = "$T\qc2"
$lnk.Save()
```

Double-click it with Cortex Control closed. The window stays open (`--pause`); paste what it shows. It opens, reads and closes twice back to back, which also shows the first close released the unit.

**(b) with Cortex Control open.** Start Cortex Control and let it connect to the unit, then:

```powershell
& $PY "$S\g2_session.py" --check b
```

Expected: a refusal within seconds ("already open exclusively"), no traceback, and Cortex Control still connected. Say what Cortex Control looks like afterwards. Then close Cortex Control.

**(c) the unit going away.** Run it, and when it prints the instruction, switch the unit off:

```powershell
& $PY "$S\g2_session.py" --check c --label power-off
```

It logs three candidate "gone" signals once a second and which fires first, then closes without hanging. Repeat with `--label unplug` (pull the USB cable instead) if you're willing; one run is enough to answer the plan's question, two is better.

### G3: build the test preset and save it into one empty slot

Pick an empty slot (G0 lists the first ten) and **leave it empty**. In the commands, replace `24H` with your slot and `4.1.0` with the exact firmware from G0.

1. Select that empty slot on the unit (the grid is blank).
2. Preflight, which writes nothing:
   ```powershell
   & $PY "$S\g3_build_and_save.py" --slot 24H --firmware 4.1.0
   ```
3. Build on the live edit buffer, still no save (look at the unit; you should see five blocks):
   ```powershell
   & $PY "$S\g3_build_and_save.py" --slot 24H --firmware 4.1.0 --apply
   ```
   Then select another preset on the unit to discard, and re-select the empty slot.
4. Build and save. You must type the slot label to confirm. It then re-lists the directory up to 6 times, 6 s apart, because on the first hardware run a listing 3 s after the save still showed the slot blank:
   ```powershell
   & $PY "$S\g3_build_and_save.py" --slot 24H --firmware 4.1.0 --apply --save
   ```
5. Power-cycle the unit, select the slot, and confirm "QC2 spike G3" with five blocks: Myth Drive, Brit 2203, a cab, Low-High Cut, Spring (M).

The test preset's levels are modest, but turn the unit's output down before you play through it.

### G4: taper constants (after G3)

Select the G3 test preset on the unit, then:

```powershell
& $PY "$S\g4_taper_check.py" --slot 24H --firmware 4.1.0
```

It writes known wire values to three parameters on the live buffer and asks what the unit's screen shows for each (9 readings). **Photo mode:** add `--hold 20` and it asks nothing; it sets each value, rings the terminal bell, prints `PHOTO n/9`, and waits 20 seconds so you can photograph the unit, then moves on. Tap the block on the unit first so its parameter is visible. **Fewest photos (4), one command each:** `--case amp --wire 0.25`, `--case cab --wire 0.01`, `--case cab --wire 1.0`, `--case hpf --wire 0.25`. Each writes ONE value, prints what the taper predicts (and what linear would give), and exits; the edit stays on the unit's live buffer until you select another preset. Tap the block to see its value. Afterwards, select another preset **without saving**.

**If the G3 preset isn't saved on the unit** (e.g. it was deleted): select an empty My Presets slot, run `g3_build_and_save.py --slot 3A --firmware 4.1.0 --apply` (builds on the live buffer, no save), leave the unit on that preset, and add `--unsaved` to the G4 and G6 commands. The block-by-block check still has to pass, so nothing else can be edited this way.

### G6: scenes (after G3)

Select the G3 test preset on the unit, then:

```powershell
& $PY "$S\g6_scenes.py" --slot 24H --firmware 4.1.0
```

It makes Myth Drive GAIN and the Spring bypass scene-varying, switches Scene A, B, A, records how the device reports them, and asks 9 yes/no questions about the screen. **Photo mode:** `--hold 20` asks nothing and waits 20 seconds at each of the 3 scene stops (`PHOTO n/3`). Afterwards, select another preset **without saving**.

**Fewest photos (2), one command each**, with the test layout open on the unit (built with `g3_build_and_save.py --apply` if it isn't saved, plus `--unsaved` on every G6 command):
1. `g6_scenes.py --slot 3A --firmware 4.1.0 --unsaved --setup` writes the per-scene Myth Drive GAIN (A=0.2, B=0.8) and the Spring bypass (A on, B bypassed) and leaves the unit on Scene A. Open the Myth Drive and photograph it: expect GAIN 2.0, Spring active.
2. `... --unsaved --goto B` switches to Scene B. Photograph again: expect GAIN 8.0, Spring dimmed in the chain row. `--goto A` switches back.

Afterwards, select another preset **without saving**.

## If something goes wrong

Stop, power-cycle the unit, and paste the console output. Restoring from the Cortex Control backup is the recovery for a damaged preset (audit section 1.6). A refusal ("REFUSED (nothing was written)") is the scripts working, not a fault.

## What is *not* proven by the offline tests

`test_spikes.py` runs the real scripts against a fake device built on qc-mcp's protobuf messages. It covers the guards, slot arithmetic, golden taper vectors, report logic, and every refusal path. It says nothing about how a real unit answers, how long operations take, or what Windows HID does on power loss. The spikes themselves answer that.
