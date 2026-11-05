# Trying qc-mcp safely: setup walkthrough (Windows)

This sets up the unofficial [qc-mcp](https://github.com/lexasoft123/qc-mcp) Quad Cortex MCP server behind `qc_allowlist_proxy.py`. The proxy limits what Claude can do to your unit. Its rules come from `docs/research/qc-usb-tools-audit.md` §1.6.

**Read mode (session 1)** allows 13 read tools and nothing else.

**Write mode (session 2)** adds 5 tools, and fences each write in:

- the unit must report your exact firmware;
- the open preset must be the one **empty** slot you pick;
- a save goes only into that slot, and only after a fresh read shows it is still empty;
- no hidden, internal or plugin blocks (this covers the NC_Recorder crash model);
- no parameter whose range qc-mcp doesn't know, and no out-of-range value;
- grid row 0-3 and column 0-7 only.

**Never forwarded:** `save_preset`, `build_preset`, `clear_grid`, global/I/O/EQ/master-volume tools, delete, leveling and audio tools.

**Not tested on a real unit or on Windows.** What has been tested:

- 21 offline tests, run against a fake qc-mcp and (with `QC_MCP_MODELREPO` set to qc-mcp's `src/qc_mcp/ModelRepo.xml`) the real catalog;
- a smoke test in front of the real qc-mcp server on Linux, where qc-mcp can't open a device.

Your session 1 is the first real run.

## Where this has to run: Windows, not WSL

qc-mcp talks to the Quad Cortex through Windows' own USB HID driver (`hid.dll`) or macOS IOKit. **It has no Linux backend.** Under WSL/Ubuntu, Python reports `linux`, and qc-mcp stops with "no HID backend for linux" (`src/qc_mcp/backend.py:105-115`). WSL2 also doesn't see USB devices unless you pass them through with `usbipd-win`, which takes the unit away from Windows.

So **qc-mcp and the proxy run as Windows Python**. You can still use Ubuntu/WSL for:

- **The `git clone` step.** Clone into `/mnt/c/Users/<you>/qc-trial`, which is the same folder as `C:\Users\<you>\qc-trial`.
- **Claude Code, if that's where you run it (untested).** WSL can start Windows programs, so register the Windows `python.exe` by its `/mnt/c/...` path and give every argument as a Windows path. Details in the WSL appendix at the end. Native Windows (Claude Code for Windows, or Claude Desktop) is the simpler, tested-by-qc-mcp's-author path; their notes report Windows 10 with CorOS 4.1.0 (`docs/WINDOWS.md:261`).

Everything below is PowerShell, one line at a time; copy each block as is.

**No PowerShell scripts are run**, so you don't need to change the execution policy. You only call `git`, `py` and `python.exe` directly.

## What you need

- **Python 3.10 or newer for Windows**, from python.org. 3.12 is a good choice. The installer adds the `py` launcher. Check with `py -3.12 --version`.
- **Git for Windows** (`git --version`), or do the cloning from Ubuntu as described above.
- **A Claude client on Windows:** Claude Code for Windows, or Claude Desktop for Windows.
- **Cortex Control** must be **closed** while you test. On Windows, qc-mcp opens the unit exclusively.

## 0. Before you start

1. **Firmware:** stay on CorOS 4.1.
2. **Backup:** in Cortex Control, make a **local backup** and keep the file. Also photograph or write down your I/O settings and Global EQ.
3. **Pick a test slot:** pick one **empty ("Unsaved") slot** in My Presets, e.g. `12H`, and leave it empty.
   - Labels assume the full Quad Cortex's 8 slots per bank, so `12H` = position 95.
   - The position is `(bank - 1) * 8 + letter`, counting A = 0 through H = 7.

## 1. Get the code (pinned)

```powershell
$T = "$HOME\qc-trial"
mkdir "$T"; cd "$T"
git clone https://github.com/lexasoft123/qc-mcp.git
git -C qc-mcp checkout f634ecd3dcf426090b90bc4cc176e4b500494b9e
git clone https://github.com/weitzer-org/sound-profile-builder.git qc2
```
`$T` is just shorthand for the folder. If you open a new PowerShell window later, set it again with the first line.

**Don't do these:**
- don't run qc-mcp's `install.ps1` or `install.sh`;
- don't run anything in `interceptor`;
- don't download the Patchbay app from its Releases.

## 2. Install the dependencies with hash checking

You don't install qc-mcp itself: the proxy runs it from its source folder.

```powershell
cd "$T"
py -3.12 -m venv venv
.\venv\Scripts\python.exe -m pip install --require-hashes -r .\qc2\tools\qc-mcp-trial\qc-mcp-f634ecd.requirements.txt
.\venv\Scripts\python.exe -m unittest discover -s .\qc2\tools\qc-mcp-trial
```
The last line runs the proxy's offline self-test. It should end in `OK`, possibly with `skipped=1`. If it doesn't, stop and send me the output.

## 3. Register the proxy in READ mode

The proxy writes the config file for you, with the right absolute paths, so there's no JSON or quoting to get wrong.

**Claude Code for Windows.** Use a scratch folder so the server isn't loaded in your other projects:
```powershell
mkdir "$T\session"
.\venv\Scripts\python.exe .\qc2\tools\qc-mcp-trial\qc_allowlist_proxy.py --qc-mcp "$T\qc-mcp" --mode read --firmware 4.1.0 --slot 12H --write-config "$T\session\.mcp.json"
```
Then `cd $T\session` and start `claude` there. It will ask you to approve the `qc-trial` server from `.mcp.json`; approve it.

**Claude Desktop for Windows (alternative):**
```powershell
.\venv\Scripts\python.exe .\qc2\tools\qc-mcp-trial\qc_allowlist_proxy.py --qc-mcp "$T\qc-mcp" --mode read --firmware 4.1.0 --slot 12H --write-config "$env:APPDATA\Claude\claude_desktop_config.json"
```
- This adds the `qc-trial` server and keeps any servers you already have. The old file is saved as `claude_desktop_config.json.bak`.
- Then fully quit and restart Claude Desktop.
- If the server doesn't appear, open Settings > Developer > Edit Config. That shows the file Desktop really uses; Store installs keep it somewhere else. Run the command again with that path.

Replace `12H` with your slot in either command.

**Leave the QC-2 Modeler server off for these sessions.** Device text, like other people's preset names, could otherwise steer Claude into using our tools.
- In Claude Code, check `/mcp` and disable it there if it's listed.
- In Claude Desktop, turn its connector off while you test.

## 4. Session 1: read only

1. **Quit Cortex Control** completely, including from the tray. Connect the Quad Cortex by USB.
2. Start Claude: Claude Code in `$T\session`, or restart Claude Desktop.
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

1. Switch to write mode, using the **exact** firmware string from session 1. Re-running `--write-config` replaces the `qc-trial` entry.
   ```powershell
   cd "$T"
   .\venv\Scripts\python.exe .\qc2\tools\qc-mcp-trial\qc_allowlist_proxy.py --qc-mcp "$T\qc-mcp" --mode write --firmware <exact string> --slot 12H --write-config "$T\session\.mcp.json"
   ```
   For Claude Desktop, use the same Desktop config path as in step 3, then restart Desktop.
2. **On the unit, select your empty slot.** The grid is blank. Make sure Cortex Control is closed.
3. Start Claude, and ask:
   - "Connect. Then add_block with device_hash 1 (Myth Drive) at row 0, column 1."
     - Check the unit shows the block.
   - "set_parameter row 0 column 1 param_index 0 display_value 6 with device_hash 1." That sets GAIN to 6.
     - Check the unit shows it.
   - "save_preset_as with name 'Trial 1' and position 95." Use your slot's 0-based position.
4. Disconnect. Power-cycle the unit, select the slot, and confirm "Trial 1" is there with the drive at gain 6.
5. A second save to the same slot will be refused, because it's no longer empty. That's intentional; use another empty slot (`--slot`) for another try.

When the proxy refuses something, Claude sees a message starting **"Refused by trial proxy:"** that gives the reason.

## 6. If something goes wrong

- If the unit hangs or shows an error, **power-cycle it**.
- If a preset or setting changed, restore from your Cortex Control backup and your notes from step 0.
- Then send me what happened. Include the proxy's message if there was one.

## 7. Cleanup

1. **Remove the server registration.**
   - Claude Code: delete `$T\session\.mcp.json`.
   - Claude Desktop: put back the `.bak` file:
     ```powershell
     Copy-Item "$env:APPDATA\Claude\claude_desktop_config.json.bak" "$env:APPDATA\Claude\claude_desktop_config.json"
     ```
     Or delete just the `qc-trial` entry from the file.
2. **Remove the files.**
   ```powershell
   cd $HOME; Remove-Item -Recurse -Force "$T"
   ```
   That deletes both clones, the venv, and `qc-mcp\interceptor\catalog.json`, the list of your library's names that qc-mcp writes.

## What the proxy cannot protect against

- **Untested firmware.** The checks rely on qc-mcp's own reads (`device_info`, `current_preset_position`, `list_empty_slots`). If qc-mcp misreads your firmware, the checks are only as good as those reads. Session 1 is there to test them.
- **Wrong-but-valid edits.** An allowed edit can still produce a strange sound in the test slot. It can't reach other slots or global settings.
- **USB traffic itself.** qc-mcp runs its own handshake and a 200 ms KeepAlive while connected. That's normal for these tools.

## Appendix: Claude Code inside WSL (untested)

If your Claude Code only lives in Ubuntu, you can let it start the **Windows** Python through WSL's Windows interop. Do steps 1-2 in PowerShell as above. Then, in Ubuntu, write the config with the Windows Python, giving every path as a Windows path:

```bash
cd /mnt/c/Users/<you>/qc-trial
/mnt/c/Users/<you>/qc-trial/venv/Scripts/python.exe 'C:\Users\<you>\qc-trial\qc2\tools\qc-mcp-trial\qc_allowlist_proxy.py' \
  --qc-mcp 'C:\Users\<you>\qc-trial\qc-mcp' --mode read --firmware 4.1.0 --slot 12H \
  --write-config 'C:\Users\<you>\qc-trial\session\.mcp.json'
```

The generated `command` is then a Windows path (`C:\...\python.exe`), which Linux can't run. Edit `/mnt/c/Users/<you>/qc-trial/session/.mcp.json` and change only `command` to `/mnt/c/Users/<you>/qc-trial/venv/Scripts/python.exe`. Leave the `args` as Windows paths. Then run `claude` from `/mnt/c/Users/<you>/qc-trial/session`.

This depends on WSL interop passing the MCP stdio stream through cleanly, which I haven't tested. If it misbehaves, use Claude Code for Windows or Claude Desktop instead.

## macOS

The steps are the same with these swaps:
- `python3.12 -m venv venv`, and `venv/bin/python` in place of `.\venv\Scripts\python.exe`;
- `~/qc-trial` in place of `$T`;
- Claude Desktop's config lives at `~/Library/Application Support/Claude/claude_desktop_config.json`;
- quit Cortex Control before connecting.

Never build or run qc-mcp's `interceptor/` (bridge mode).
