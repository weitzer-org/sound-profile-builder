# Trying qc-mcp safely: setup walkthrough

This sets up the unofficial [qc-mcp](https://github.com/lexasoft123/qc-mcp) Quad Cortex MCP server behind `qc_allowlist_proxy.py`. The proxy limits what Claude can do to your unit. Its rules come from `docs/research/qc-usb-tools-audit.md` §1.6:

- **Read mode (session 1)** allows 13 read tools and nothing else.
- **Write mode (session 2)** adds 5 tools, and each write is fenced in:
  - **Where:** the unit must report your exact firmware, and the open preset must be the one **empty** slot you pick. Saves go only to that slot, and only after a fresh read confirms it is still empty.
  - **What:** no hidden, internal or plugin blocks (the NC_Recorder crash model is among the hidden ones), no parameter with an unknown range, and no out-of-range value.
- **Never forwarded:** `save_preset`, `build_preset`, `clear_grid`, global/I/O/EQ/master-volume tools, delete, leveling and audio tools.

**Not tested on a real unit.** The proxy's tests run offline: 18 of them, against a fake qc-mcp and (with `QC_MCP_MODELREPO` set to qc-mcp's `src/qc_mcp/ModelRepo.xml`) the real catalog. It was also smoke-tested in front of the real qc-mcp server on Linux, where qc-mcp can't open a device. Your session 1 is the first real-hardware run.

Supported platforms: macOS or Windows. qc-mcp has no Linux USB backend. You need Python 3.10 or newer; 3.12 is a safe choice.

## 0. Before you start

1. **Firmware:** stay on the CorOS you're on (4.1). The proxy blocks writes unless the unit reports exactly the version you give it.
2. **Backup:** in Cortex Control, make a **local backup** and keep the file. Also photograph or write down your I/O settings and Global EQ.
3. **Pick a test slot:** pick one **empty ("Unsaved") slot** in My Presets, e.g. `12H`, and note it. It's the only place anything gets saved.
   - Slot labels assume the full Quad Cortex's 8-per-bank layout, so `12H` = position 95.
   - Leave the slot empty; don't copy a preset into it first.

## 1. Get the code (pinned)

**macOS** (Terminal):
```bash
mkdir -p ~/qc-trial && cd ~/qc-trial
git clone https://github.com/lexasoft123/qc-mcp.git
git -C qc-mcp checkout f634ecd3dcf426090b90bc4cc176e4b500494b9e
git clone -b claude/qc-usb-tools-audit https://github.com/weitzer-org/sound-profile-builder.git qc2
```

**Windows** (PowerShell):
```powershell
mkdir $HOME\qc-trial; cd $HOME\qc-trial
git clone https://github.com/lexasoft123/qc-mcp.git
git -C qc-mcp checkout f634ecd3dcf426090b90bc4cc176e4b500494b9e
git clone -b claude/qc-usb-tools-audit https://github.com/weitzer-org/sound-profile-builder.git qc2
```

**Don't do these:**
- don't run qc-mcp's `install.sh` or `install.ps1`;
- don't run anything in `interceptor/`;
- don't download the Patchbay app from its Releases.

## 2. Install the dependencies with hash checking

You don't install qc-mcp itself: the proxy runs it from its source folder.

**macOS:**
```bash
cd ~/qc-trial
python3.12 -m venv venv          # any python3 >= 3.10; macOS's built-in python3 may be too old
venv/bin/pip install --require-hashes -r qc2/tools/qc-mcp-trial/qc-mcp-f634ecd.requirements.txt
venv/bin/python -m unittest discover -s qc2/tools/qc-mcp-trial -v    # optional: proxy self-test
```

**Windows:**
```powershell
cd $HOME\qc-trial
py -3.12 -m venv venv
venv\Scripts\pip install --require-hashes -r qc2\tools\qc-mcp-trial\qc-mcp-f634ecd.requirements.txt
venv\Scripts\python -m unittest discover -s qc2\tools\qc-mcp-trial -v
```

## 3. Register the proxy in READ mode

**Use a scratch folder.** It keeps the server out of your other projects.

**Leave the QC-2 Modeler server off in this session.** Device text, like other people's preset names, could otherwise steer Claude into using our tools.
- Check with `claude mcp list` inside the scratch folder.
- If QC-2 Modeler shows up (it would if it's registered at user scope), disable it for the session from Claude Code's `/mcp` menu.

**Claude Code** (macOS; on Windows use `venv\Scripts\python.exe` and backslash paths):
```bash
mkdir -p ~/qc-trial/session && cd ~/qc-trial/session
claude mcp add --scope local qc-trial -- \
  ~/qc-trial/venv/bin/python ~/qc-trial/qc2/tools/qc-mcp-trial/qc_allowlist_proxy.py \
  --qc-mcp ~/qc-trial/qc-mcp --python ~/qc-trial/venv/bin/python \
  --mode read --firmware 4.1.0 --slot 12H
```
Replace `12H` with your slot. If `~` isn't expanded inside the stored command, use full paths.

**Claude Desktop (alternative):** add this to `claude_desktop_config.json` and restart the app. On macOS the file is in `~/Library/Application Support/Claude/`; on Windows it's in `%APPDATA%\Claude\`.
```json
{ "mcpServers": { "qc-trial": {
    "command": "/Users/YOU/qc-trial/venv/bin/python",
    "args": ["/Users/YOU/qc-trial/qc2/tools/qc-mcp-trial/qc_allowlist_proxy.py",
             "--qc-mcp", "/Users/YOU/qc-trial/qc-mcp",
             "--python", "/Users/YOU/qc-trial/venv/bin/python",
             "--mode", "read", "--firmware", "4.1.0", "--slot", "12H"] } } }
```
Desktop servers load in every chat, so remove this entry when you're done.

## 4. Session 1: read only

1. **Quit Cortex Control** completely. Direct mode needs exclusive access to the USB device.
2. Connect the Quad Cortex by USB and start Claude in `~/qc-trial/session`.
3. Ask, one at a time:
   - "Connect to the Quad Cortex, then call device_info and tell me the exact `firmware` string."
     - **Write that string down.** Session 2 needs it exactly.
     - If it isn't a 4.1 version, stop here.
   - "Call directory_summary with refresh=true, then list_empty_slots. Is position 95 (12H) empty, and is source 'device'?"
   - "Read the current preset with get_current_preset."
     - Compare the blocks and a few knob values with the unit's screen.
     - **If they don't match, stop.** That means the protocol doesn't fit your firmware.
   - "Disconnect."
4. If anything errors or looks wrong, stop and send me the message.

## 5. Session 2: one write to the empty slot

1. Re-register in write mode with the exact firmware string from session 1:
   ```bash
   cd ~/qc-trial/session
   claude mcp remove qc-trial --scope local
   claude mcp add --scope local qc-trial -- ~/qc-trial/venv/bin/python \
     ~/qc-trial/qc2/tools/qc-mcp-trial/qc_allowlist_proxy.py --qc-mcp ~/qc-trial/qc-mcp \
     --python ~/qc-trial/venv/bin/python --mode write --firmware <exact string> --slot 12H
   ```
2. **On the unit, select your empty slot.** The grid is blank. Then quit Cortex Control.
3. In Claude, ask:
   - "Connect. Then add_block with device_hash 1 (Myth Drive) at row 0, column 1."
     - Check the unit shows the block.
   - "set_parameter row 0 column 1 param_index 0 display_value 6 with device_hash 1." That sets GAIN to 6.
     - Check the unit shows it.
   - "save_preset_as with name 'Trial 1' and position 95." Use your slot's 0-based position.
4. Disconnect. Power-cycle the unit, select the slot, and confirm "Trial 1" is there with the drive at gain 6.
5. A second save to the same slot will be refused, because it's no longer empty. That's intentional. Pick another empty slot (`--slot`) for another try.

When the proxy refuses something, Claude sees a message starting **"Refused by trial proxy:"** that gives the reason.

## 6. If something goes wrong

- If the unit hangs or shows an error, **power-cycle it**.
- If a preset or setting changed, restore from your Cortex Control backup and your notes from step 0.
- Then send me what happened. Include the proxy's message if there was one.

## 7. Cleanup

```bash
cd ~/qc-trial/session && claude mcp remove qc-trial --scope local
rm -rf ~/qc-trial      # removes both clones and the venv
```
qc-mcp also writes `qc-mcp/interceptor/catalog.json`, a list of your library's names, and that folder is removed with the rest. For Claude Desktop, also remove the config entry.

## What the proxy cannot protect against

- **Untested firmware.** The checks rely on qc-mcp's own reads (`device_info`, `current_preset_position`, `list_empty_slots`). If qc-mcp misreads your firmware, the checks are only as good as those reads. Session 1 is there to test them.
- **Wrong-but-valid edits.** An allowed edit can still produce a strange sound in the test slot. It can't reach other slots or global settings.
- **USB traffic itself.** qc-mcp still runs its own handshake and a 200 ms KeepAlive while connected. That's normal for these tools.
