"""What each job kind does on the unit (contract section 8 steps 11-12, plan section 5).

Every executor returns an Outcome; none raises for an expected failure. The rules
that bind every device write, in the order they run for a push:
  1. refuse anything the agent can't check: an unsupported export format, an export
     that fails validation against the agent's own catalog copy, a slot outside
     QC_AGENT_ALLOWED_SLOTS (all before the unit is opened);
  2. the unit's firmware equals QC_AGENT_EXPECTED_FIRMWARE exactly, and it is a full
     Quad Cortex (8 slots per bank);
  3. a FRESH directory listing shows the slot empty. A slot the bridge didn't create
     is never overwritten, and v1 never overwrites at all (editing existing presets
     is deferred: plan section 10, item 7);
  4. the slot is the one open on the unit and its live grid is blank. This is the
     only save path Phase 0 verified (G3): the agent never recalls a preset or
     clears a grid, so it can't disturb what the owner has open;
  5. the job has not expired (re-checked immediately before the first write);
  6. build in three phases: place every block (a capture block with its capture loaded), then
     ONE read that checks each column's model id and that it reports exactly param_count + 1
     parameters (G1, plan 7.2), so a block whose index layout isn't the catalog's stops the job
     before any value is written; then, block by block, a re-read of that column (the same check,
     which also confirms its bypass control to the wire guard) and the block's parameters (all eight scene values with
     set_param_scenes when A and B differ, else one set_param; G6) and all eight bypass flags of
     every block, through its bypass control at param_count + 1;
  7. read back and diff every written value and every bypass flag; controls the push didn't write
     are compared with the catalog default and logged, never failed;
  7b. footswitches (stomp doc section 6.1, entry point 1; owner decision #147: this does not wait for
     gate G7): only when the export has `stomps`, and only after step 7 is clean. Each planned stomp is
     assigned with the vendored assign_stomp (primary, latching, never H), through the same op() checks
     as every other write, so only into the empty slot this push is building and only on blocks it
     placed. Then one read: the assignments must be exactly the planned ones (none missing, none extra,
     no secondary, none momentary) and the step-7 diff must still be clean, or the job fails before the
     save. Which physical switch an index lights, and that a switch toggles its block, is UNVERIFIED
     until the owner checks the first such push on the unit (README step 16);
  8. save only if the diff is clean: a second fresh listing still shows the slot empty, then the
     grid is re-read (the listing takes ~15 s) and the same slot is still open; File CREATE; then
     poll the listing (G3: it lags). A save the listing never shows is reported as FAILED, and the
     slot stays in the owned ledger in case it lands late.
Before every write the open position is read again and the watchdog's verdict checked: writes address
row/column/index, not a preset, so after the owner selects another preset they would land in it.
A cab's mic string (plan 4.2, P4b: `mics` in the export) is written exactly once, before the
block's values, and checked in the read-back; the wire guard allows only that string at that
index. Whether the unit accepts a mic written this way is UNVERIFIED (README step 14).
Capture blocks (plan 4.2) are resolved against the unit's own capture list from the
rule-3 listing (qc_agent.captures): a capture that is missing, ambiguous, cloud-only
or of unknown version refuses the job before any write. The second listing must
resolve every capture to the same key and version, or nothing is saved. A capture block
goes through the same layout check (param_count of 14000/14001 from the catalog) and index rules.
The result carries only contract section 9 fields.
"""
from __future__ import annotations

import hashlib
import json
import logging
import math
import time
from dataclasses import dataclass, field

from . import captures, contract, edit_checks, edit_diff, slots, text
from .device import SETLIST_KEY, SETLIST_NAME, STOMP_PRIMARY, DeviceError, PartialListing, UnitAbsent, UnitBusy
from .export_validator import (CAPTURE_BLOCK_IDS, SUPPORTED_FORMATS, ExportRefused, check_preset_name,
                               control_labels, default_wire, is_internal_param, validate)

log = logging.getLogger("qc_agent")

TOL = 1e-4                 # G3 read-back tolerance
PACE_S = 0.15              # between writes; G3 used the same pacing
SAVE_LIST_POLLS = 6        # G3: a listing 3 s after the save still showed the slot blank
SAVE_LIST_INTERVAL_S = 6.0  # plus the listing itself (~12-15 s each): the window is about 2 minutes
MAX_DIFF = 512
CAPTURE_SETTLE_S = 1.0     # after loading a capture, before its params (unverified timing; the read-back checks)
NAME_LIMIT = 64
# A read-back text value (contract section 9.1). A Neural Capture block's file_name is a 64-hex key plus the
# capture's name, so NAME_LIMIT would cut the name off entirely. internal/devicejobs.MaxReadbackStringRn is the
# same number; testdata/devicejobs/readback.json pins both sides.
VALUE_STRING_LIMIT = 192


@dataclass
class Outcome:
    status: str                 # succeeded | failed | refused | expired
    reason_code: str = ""
    message: str = ""
    firmware: str | None = None
    push: dict | None = None
    slots: dict | None = None
    readback: dict | None = None
    library: dict | None = None
    edit: dict | None = None    # edit_preset (contract section 9.3); None when the job was refused before the unit
    unit_gone: bool = False     # the agent exits after this job
    extra: dict = field(default_factory=dict)


def refused(code, msg, **kw):
    return Outcome("refused", code, msg, **kw)


def failed(code, msg, **kw):
    return Outcome("failed", code, msg, **kw)


class JobContext:
    def __init__(self, config, catalog, device, owned, now_fn, sleep=time.sleep, edits=None):
        self.config, self.catalog, self.device, self.owned = config, catalog, device, owned
        self.edits = edits  # ledger.EditLedger, or None (tests that never edit)
        self.now, self.sleep = now_fn, sleep
        self.firmware_seen = None  # (firmware, when) for the heartbeat


class _Gone(Exception):
    pass


class _SwitchedAway(Exception):
    """The target slot is no longer the one open on the unit (checked before every write)."""


def _same_name(listing, pos, name):
    """Another slot whose preset has this name (case and surrounding spaces ignored), or None."""
    want = name.strip().casefold()
    for p, n in listing.items():
        if p != pos and isinstance(n, str) and n.strip().casefold() == want:
            return p
    return None


def _folder_label(snap, key) -> str:
    """A collection's display name for messages (device text, cleaned)."""
    if snap is not None and key in snap.preset_folders:
        return snap.preset_folders[key][0]
    if key == SETLIST_KEY:
        return SETLIST_NAME
    return text.clean(str(key).rstrip("/").rsplit("/", 1)[-1], 40) or "unnamed"


def _folder_view(snap, key):
    """The listing seen through one collection, or None if it isn't a complete 256-slot table there."""
    if not key:
        return None
    view = snap.for_folder(key)
    if view is None and key == SETLIST_KEY:
        return snap  # a snapshot without per-folder tables (My Presets is its main table)
    return view


def _open_desc(cur, snap=None) -> str:
    """What the unit says is open, for a refusal message (the folder key is device text)."""
    p = cur.get("position")
    if cur.get("is_factory"):
        return "a factory preset"
    if not cur.get("folder_key"):
        return "no preset (no position)"
    folder = _folder_label(snap, cur.get("folder_key"))
    if isinstance(p, int) and 0 <= p < slots.SLOT_COUNT:
        return f"slot {slots.label(p)} in {folder!r}"
    return f"a preset in {folder!r}"


def _occupied(listing, pos) -> bool:
    """Only an exactly blank name is an empty slot. A missing position or a name that is just
    spaces counts as occupied: the agent refuses rather than risk overwriting a preset."""
    name = listing.get(pos)
    return not isinstance(name, str) or name != ""


def _connect_checked(ctx):
    """Open the unit and check firmware and device type. Returns an Outcome on refusal."""
    dev = ctx.device
    try:
        dev.connect()
    except UnitBusy:
        return failed("unit_busy", "Cortex Control (or another program) has the Quad Cortex open. "
                      "Close it completely, then send the request again.")
    except UnitAbsent:
        return failed("unit_absent", "No Quad Cortex found on USB. Switch it on and connect it, then retry.",
                      unit_gone=True)
    except DeviceError as e:
        log.warning("connect failed: %s", text.clean(str(e), 200))
        return failed("device_error", "Could not open the Quad Cortex.")
    fw = dev.firmware
    ctx.firmware_seen = (fw, ctx.now())
    if fw != ctx.config.expected_firmware:
        return refused("firmware_mismatch",
                       f"The unit reports firmware {text.clean(fw or 'unknown', 32)}; this bridge only works with "
                       f"{ctx.config.expected_firmware}. Nothing was changed.", firmware=_fw(fw))
    if dev.device_type != "QC":
        return refused("firmware_mismatch",
                       f"The unit reports device type {text.clean(str(dev.device_type), 16)}; this bridge "
                       "supports a full Quad Cortex only. Nothing was changed.", firmware=_fw(fw))
    return None


def _fw(fw):
    return text.clean(fw, 32) if isinstance(fw, str) and fw else None


def _check_alive(ctx):
    if not ctx.device.alive():
        raise _Gone()


# Kinds that parse but that this bridge refuses outright, with the message for each. They are answered
# here, before the executor path, because that path's `finally` calls end_push() and disconnect() on the
# device: a refused kind never touches it. Empty since preset-editing E4 moved edit_preset to EXECUTORS.
REFUSED_KINDS = {}

# ---- list_slots ----

def _slot_table(ctx, listing):
    out = []
    for pos in range(slots.SLOT_COUNT):
        raw = listing.get(pos, "")
        name = text.clean(raw, NAME_LIMIT)
        occupied = _occupied(listing, pos)
        out.append({"slot": slots.label(pos), "state": "occupied" if occupied else "empty",
                    "name": (name or "?") if occupied else None,
                    "owned": occupied and ctx.owned.owned(pos, raw)})
    return out


def list_slots(ctx, job) -> Outcome:
    bad = _connect_checked(ctx)
    if bad:
        return bad
    fw = _fw(ctx.device.firmware)
    try:
        listing = ctx.device.list_my_presets()
        _check_alive(ctx)
    except (_Gone, UnitAbsent):
        return failed("unit_absent", "The Quad Cortex went away during the listing.", firmware=fw, unit_gone=True)
    except DeviceError as e:
        log.warning("listing failed: %s", text.clean(str(e), 200))
        return failed("device_error", "The unit's directory listing failed.", firmware=fw)
    table = _slot_table(ctx, listing)
    empty = sum(1 for s in table if s["state"] == "empty")
    return Outcome("succeeded", "", f"My Presets: {empty} of {len(table)} slots empty.", firmware=fw,
                   slots={"setlist": SETLIST_NAME, "slots": table})


# ---- read_preset ----

def _plain_value(v):
    if isinstance(v, bool):
        return v
    if isinstance(v, int):
        return v
    if isinstance(v, float):
        return v if math.isfinite(v) else None
    if isinstance(v, str):
        return text.clean(v, VALUE_STRING_LIMIT)
    return None


def readback_payload(preset, position_info):
    chains = []
    for ch in preset.get("chains", []):
        blocks = []
        for b in ch.get("blocks", []):
            blocks.append({"col": b["col"], "model_id": b["model_id"],
                           "params": [{"index": p["index"], "scene_mode": bool(p.get("scene_mode")),
                                       "values": [_plain_value(v) for v in p.get("values", [])[:8]]}
                                      for p in b.get("params", [])]})
        chains.append({"row": ch["row"], "in_port": ch.get("in_port"), "out_port": ch.get("out_port"),
                       "blocks": blocks})
    bypass = preset.get("bypass")
    return {"setlist": position_info, "chains": chains,
            "bypass": [[[bool(x) for x in col[:8]] for col in row[:8]] for row in bypass[:4]] if bypass else None}


READBACK_FORMAT = "qc-device-readback/2"
FOLDER_KEY_LIMIT = 128


def valid_folder_key(k) -> bool:
    """Contract section 9.1 (/2): 1-128 printable ASCII characters, no backtick, no leading or trailing
    space, no double space. Such a key is unchanged by text.clean, so QC-2 can pin it byte for byte."""
    return (isinstance(k, str) and 1 <= len(k) <= FOLDER_KEY_LIMIT and k[0] != " " and k[-1] != " "
            and "  " not in k and all(0x20 <= ord(c) <= 0x7E and c != "`" for c in k))


def setlist_info(pos) -> dict:
    """The read-back's setlist (qc-device-readback/2) from the unit's SetlistPosition reply.

    folder_key is the open collection's key exactly as the unit reports it, or None when it isn't a key
    the contract can carry. position is set only for a non-factory preset in a collection whose key was
    sent, so an edit plan can always pin both or neither. A reply without a boolean is_factory counts as
    factory (no position: fail closed)."""
    pos = pos if isinstance(pos, dict) else {}
    key = pos.get("folder_key")
    key = key if valid_folder_key(key) else None
    factory = pos.get("is_factory")
    factory = factory if isinstance(factory, bool) else True
    p = pos.get("position")
    position = p if (key is not None and not factory and type(p) is int and 0 <= p < slots.SLOT_COUNT) else None
    folder = SETLIST_NAME if key == SETLIST_KEY and not factory else "other"
    return {"folder": folder, "folder_key": key, "position": position, "is_factory": factory}


def read_preset(ctx, job) -> Outcome:
    if job["args"]["slot"] is not None:
        # Recalling a slot changes what the unit is playing (contract section 7).
        return refused("unsupported_arg", "This bridge reads the preset open on the unit only; "
                       "select the slot on the unit and read the current preset.")
    bad = _connect_checked(ctx)
    if bad:
        return bad
    fw = _fw(ctx.device.firmware)
    try:
        pos = ctx.device.setlist_position() or {}
        preset = ctx.device.read_current_preset()
        after = ctx.device.setlist_position() or {}
        _check_alive(ctx)
        if after != pos:
            return failed("device_error", "The selected preset changed during the read; read again.", firmware=fw)
    except (_Gone, UnitAbsent):
        return failed("unit_absent", "The Quad Cortex went away during the read.", firmware=fw, unit_gone=True)
    except DeviceError as e:
        log.warning("read failed: %s", text.clean(str(e), 200))
        return failed("device_error", "Reading the current preset failed.", firmware=fw)
    if preset is None:
        return failed("device_error", "The unit did not return the current preset in time.", firmware=fw)
    info = setlist_info(pos)
    slot = slots.label(info["position"]) if info["position"] is not None else None
    name = text.clean(preset.get("name", ""), NAME_LIMIT)
    rb = {"format": READBACK_FORMAT, "slot": slot, "name": name or None,
          "payload": readback_payload(preset, info)}
    if len(json.dumps(rb, separators=(",", ":"))) > contract.MAX_READBACK_BYTES - 16 * 1024:
        return failed("device_error", "The open preset is too large to return.", firmware=fw)
    nblocks = sum(len(c["blocks"]) for c in rb["payload"]["chains"])
    return Outcome("succeeded", "", f"Read the open preset ({nblocks} blocks).", firmware=fw, readback=rb)


# ---- sync_device_library (plan P4b) ----

def sync_device_library(ctx, job) -> Outcome:
    """Read-only: one directory listing (the same read_directory a push takes, nothing new), turned into the
    capture list QC-2 stores. Writes nothing to the unit, and does not load or recall any preset."""
    bad = _connect_checked(ctx)
    if bad:
        return bad
    fw = _fw(ctx.device.firmware)
    try:
        snap = ctx.device.read_directory()
        _check_alive(ctx)
    except (_Gone, UnitAbsent):
        return failed("unit_absent", "The Quad Cortex went away during the listing.", firmware=fw, unit_gone=True)
    except DeviceError as e:
        log.warning("listing failed: %s", text.clean(str(e), 200))
        return failed("device_error", "The unit's directory listing failed.", firmware=fw)
    if not snap.capture_folders:
        return failed("device_error", "The unit's directory listing had no capture folders (incomplete listing?), "
                      "so there is no capture list to send. Send it again.", firmware=fw)
    try:
        lib = captures.library_payload(snap)
    except captures.LibraryTooLarge as e:
        return failed("device_error", f"The unit lists {e.args[0]} capture files, more than the {captures.MAX_LIBRARY_CAPTURES} "
                      "the sync carries. Nothing was sent.", firmware=fw)
    try:  # evidence only (F2): whatever goes wrong here must not change the sync
        for line in captures.skip_census(snap):
            log.info("%s", line)
    except Exception:  # noqa: BLE001
        log.exception("the capture skip census failed; the sync goes on")
    rb = {"format": captures.LIBRARY_FORMAT, "payload": lib}
    if len(json.dumps(rb, separators=(",", ":"), ensure_ascii=False).encode("utf-8")) > contract.MAX_RESULT_BYTES - 16 * 1024:
        return failed("device_error", "The capture list is too large to return.", firmware=fw)
    caps = lib["captures"]
    nf = sum(1 for c in caps if c["kind"] == "factory")
    skipped = f", {lib['skipped']} left out (names the bridge can't compare exactly, or favorites)" if lib["skipped"] else ""
    return Outcome("succeeded", "", f"Listed {len(caps)} capture files ({nf} factory, {len(caps) - nf} user){skipped}.",
                   firmware=fw, library=rb)


# ---- push_preset ----

def _expired(ctx, job) -> bool:
    return not ctx.now() < contract.parse_time(job["expires_at"])


def _diff_param(row, col, p, vals, diff) -> int:
    """Compare one written parameter's read-back values; append diff entries; return the mismatch count.

    Entries carry only contract section 9's fields. A scene-varying param gets eight, scenes A-H in
    order; a non-varying one, written once and replicated by the unit (G1/G6), gets one."""
    mismatches = 0
    expected = p.scene_wires or [p.wire]
    for s in (range(8) if p.scene_wires else [0]):
        v = vals[s] if s < len(vals) else None
        if not p.scene_wires:
            # Not scene-assigned: the unit replicates one value into all 8 slots (G1); need all eight.
            ok = len(vals) == 8 and all(isinstance(x, float) and abs(x - p.wire) <= TOL for x in vals)
        else:
            ok = isinstance(v, float) and abs(v - expected[s]) <= TOL
        mismatches += not ok
        if len(diff) < MAX_DIFF:
            diff.append({"row": row, "col": col, "param_index": p.index, "expected": expected[s],
                         "readback": v if isinstance(v, float) and math.isfinite(v) else None, "ok": ok})
    return mismatches


def _diff(plan, preset):
    """Compare the live grid with the plan. Returns (mismatches, diff entries, problems)."""
    diff, problems, mismatches = [], [], 0
    chain = next((c for c in preset.get("chains", []) if c.get("row") == plan.row), None)
    if chain is None:
        return 1, diff, ["row 0 missing from the read-back"]
    if (chain.get("in_port"), chain.get("out_port")) != (plan.in_port, plan.out_port):
        mismatches += 1
        problems.append(f"routing reads in={chain.get('in_port')} out={chain.get('out_port')}")
    by_col = {b["col"]: b for b in chain.get("blocks", [])}
    planned_cols = {b.col for b in plan.blocks}
    for c in sorted(set(by_col) - planned_cols):
        mismatches += 1
        problems.append(f"unexpected block at column {c}")
    for other in preset.get("chains", []):
        if other.get("row") != plan.row and other.get("blocks"):
            mismatches += 1
            problems.append(f"unexpected blocks on row {other.get('row')}")
    bypass = preset.get("bypass")
    for b in plan.blocks:
        got = by_col.get(b.col)
        if got is None or got.get("model_id") != b.model_id:
            mismatches += 1
            problems.append(f"column {b.col}: expected model {b.model_id}, found "
                            f"{None if got is None else got.get('model_id')}")
            continue
        params = {p["index"]: p for p in got.get("params", [])}
        if b.capture is not None:
            vals = (params.get(captures.FILE_NAME_PARAM) or {}).get("values") or []
            if not vals or any(not isinstance(v, str) or v != b.capture.file_name for v in vals):
                mismatches += 1
                problems.append(f"column {b.col}: the capture block's file_name doesn't read back as planned "
                                f"({text.clean(str(vals[0]) if vals else '', 80)!r})")
        for m in b.mics:
            vals = (params.get(m.index) or {}).get("values") or []
            if not vals or any(not isinstance(v, str) or v != m.value for v in vals):
                mismatches += 1
                problems.append(f"column {b.col}: the cab mic {m.slot} doesn't read back as planned "
                                f"({text.clean(str(vals[0]) if vals else '', 80)!r})")
        for p in b.params:
            mismatches += _diff_param(plan.row, b.col, p, (params.get(p.index) or {}).get("values") or [], diff)
        want = b.bypass_scenes
        have = None
        if bypass and len(bypass) > plan.row and len(bypass[plan.row]) > b.col:
            have = [bool(x) for x in bypass[plan.row][b.col][:8]]
        if have is None or len(have) < 8:
            mismatches += 1  # G6: the unit reports eight flags for every grid position
            problems.append(f"column {b.col}: bypass flags missing from the read-back")
        elif have != want:
            mismatches += 1
            problems.append(f"column {b.col}: bypass reads {have}, expected {want}")
    return mismatches, diff, problems


def _stomp_problems(plan, preset):
    """Compare the read-back's footswitch assignments with the plan's stomps. Empty when they match
    exactly, or when the plan has no stomps (then nothing was assigned and nothing is checked, as before).
    Fails closed: a read without the assignments, an extra or missing one, a secondary assignment, H, or a
    planned switch that reads back momentary is each a problem."""
    if not plan.stomps:
        return []
    got = preset.get("stomps")
    if not isinstance(got, list):
        return ["the read-back has no footswitch assignments"]
    # s.row is the row each stomp was validated (export_validator: always ROW, the same constant as plan.row) and
    # armed on; the wire guard refuses any other row, so a mismatch fails closed rather than being remapped here.
    want = {(s.row, s.col, s.index) for s in plan.stomps}
    problems, seen = [], set()
    for a in got[:64]:
        key = (a.get("row"), a.get("col"), a.get("index"))
        if a.get("type") != STOMP_PRIMARY:
            problems.append(f"row {key[0]} column {key[1]}: a secondary footswitch assignment")
        elif key not in want:
            problems.append(f"row {key[0]} column {key[1]}: footswitch {_switch(key[2])} is not the plan's")
        elif key in seen:
            problems.append(f"row {key[0]} column {key[1]}: footswitch {_switch(key[2])} reads back twice")
        seen.add(key)
    if len(got) > 64:
        problems.append(f"{len(got)} footswitch assignments read back")
    for row, col, index in sorted(want - seen):
        problems.append(f"row {row} column {col}: footswitch {_switch(index)} did not read back")
    momentary = preset.get("momentary") or {}
    for index in sorted({s.index for s in plan.stomps}):
        if momentary.get(index):
            problems.append(f"footswitch {_switch(index)} reads back momentary (the bridge assigns latching only)")
    return problems


def _switch(index):
    return "ABCDEFGH"[index] if isinstance(index, int) and 0 <= index < 8 else f"index {text.clean(str(index), 8)}"


def _row(plan, preset):
    return next((c for c in (preset or {}).get("chains", []) if c.get("row") == plan.row), None)


def _check_layout(plan, preset, blocks=None):
    """After placing every block, before writing any value: each column holds the planned model
    (for a capture block, the V1/V2 block the listing resolved) and reports exactly param_count + 1
    parameters (G1: the full list, hidden ones included, plus one trailing slot). Any other count
    means the catalog's indexes don't describe this block on this unit, so no value may be written
    to it, nor its bypass control (param_count + 1) addressed. Returns a problem string or None."""
    chain = _row(plan, preset)
    if chain is None:
        return "row 0 is missing from the unit's read"
    by_col = {b["col"]: b for b in chain.get("blocks", [])}
    for b in plan.blocks if blocks is None else blocks:
        got = by_col.get(b.col)
        if got is None or got.get("model_id") != b.model_id:
            return (f"column {b.col} holds model {None if got is None else got.get('model_id')}, "
                    f"not {b.model} ({b.model_id})")
        n = len(got.get("params") or [])
        if n != b.param_count + 1:
            return (f"column {b.col} ({b.model}) reads {n} parameters; the catalog expects "
                    f"{b.param_count + 1} ({b.param_count} plus the trailing slot)")
    return None


def _fmt_vals(vals):
    out = []
    for v in list(vals)[:8]:
        if isinstance(v, float):
            out.append(f"{v:.6g}" if math.isfinite(v) else "nan")
        elif isinstance(v, (bool, int)):
            out.append(str(v))
        else:
            out.append(text.clean(repr(v), 24))
    return "[" + ", ".join(out) + "]"


def _log_defaults(plan, preset, catalog) -> int:
    """Every user control the push didn't write should read back at its catalog default (a block
    placed into a blank slot starts at its defaults; G4 saw it for two params, no more). Logs each one
    that differs and returns how many did. NEVER fails the job: it gathers evidence for P6. A control
    whose default the agent can't map to a wire value (switch positions without a range, symbolic
    bounds it doesn't resolve) is logged raw as not compared. A capture block is checked against its
    resolved V1/V2 model; its file_name is a string and is checked by the read-back diff instead."""
    chain = _row(plan, preset) or {}
    by_col = {b["col"]: b for b in chain.get("blocks", [])}
    differ = 0
    for b in plan.blocks:
        got, model = by_col.get(b.col), catalog.models.get(b.model_id)
        if got is None or model is None:
            continue
        read = {p["index"]: p.get("values") or [] for p in got.get("params", [])}
        written = {p.index for p in b.params}
        labels = control_labels(model)
        tagged = set(b.at_default)
        for cp in sorted(model["params"], key=lambda q: q["index"]):
            i = cp["index"]
            if i in written or is_internal_param(cp):
                continue
            vals = read.get(i, [])
            label = labels.get(i, cp["name"])
            name = (f"col {b.col} {text.clean(b.model, 40)} {label} [{i}]"
                    + (" (at_default in the export)" if label in tagged else ""))
            dw = default_wire(cp)
            if dw is None:
                log.info("default not compared (no wire mapping for its default): %s default %s, read %s",
                         name, cp.get("default"), _fmt_vals(vals))
                continue
            if len(vals) == 8 and all(isinstance(v, float) and abs(v - dw) <= TOL for v in vals):
                continue
            differ += 1
            log.warning("default differs: %s default %s -> wire %.6g, read %s",
                        name, cp.get("default"), dw, _fmt_vals(vals))
    return differ


class _Push:
    """One push, as the steps the module docstring lists. Each step returns an Outcome to stop
    with, or None to go on; run() owns the order and the error handling."""

    def __init__(self, ctx, job):
        self.ctx, self.job, self.dev = ctx, job, ctx.device
        a = job["args"]
        self.args, self.slot, self.name = a, a["slot"], a["name"]
        self.pos = slots.position(self.slot)
        self.folder_key = None   # the collection open on the unit; chosen once by check_slot, then pinned
        self.folder_name = SETLIST_NAME
        self.result = {"slot": self.slot, "name": self.name, "mismatches": 0, "cpu_load": None, "diff": []}
        self.plan = None
        self.fw = None
        self.saved = False
        self.resolved = {}   # col -> captures.ResolvedCapture
        self.deps_note = ""
        self.defaults_differ = 0

    def run(self) -> Outcome:
        out = self.check_job()                              # rule 1 (no device contact)
        if out:
            return out
        out = _connect_checked(self.ctx)                    # rule 2
        if out:
            return out
        self.fw = _fw(self.dev.firmware)
        try:
            for step in (self.check_unit, self.build, self.verify, self.assign_stomps):
                outcome = step()
                if outcome is not None:  # a step stops the push by returning its Outcome
                    return outcome
            return self.save()
        except _SwitchedAway:
            return failed("device_error", f"Another preset was selected on the unit during the build, so the "
                          f"bridge stopped. Nothing was saved; re-select {self.slot} or another preset to discard "
                          "the partial edits.", firmware=self.fw, push=self.result)
        except (_Gone, UnitAbsent):
            if self.saved:
                return self.unconfirmed("unit_absent", "then the Quad Cortex went away before the save could be "
                                        "confirmed", unit_gone=True)
            return failed("unit_absent", "The Quad Cortex went away during the push. If anything was written it "
                          "was not saved.", firmware=self.fw, push=self.result, unit_gone=True)
        except DeviceError as e:
            log.warning("push failed: %s", text.clean(str(e), 200))
            if self.saved:
                return self.unconfirmed("device_error", "but confirming it failed")
            return failed("device_error", "A device operation failed. Nothing was saved; re-select the slot on the "
                          "unit to discard any partial grid.", firmware=self.fw, push=self.result)

    def done(self, msg, **kw):
        msg = msg + captures.summary(self.resolved) + self.deps_note
        return Outcome("succeeded", "", msg, firmware=self.fw, push=self.result, extra={"saved": True}, **kw)

    def unconfirmed(self, code, how, **kw):
        """The save was issued after a clean read-back but never confirmed: a failure (G3: the listing
        can lag, so it may still land). The slot was recorded in the owned ledger when the save was
        issued and stays there, so a late save is still recognised as the bridge's own."""
        msg = (f"Save to {self.slot} in {self.folder_name!r} was issued after a clean read-back, {how}. "
               f"It may still land: check {self.slot} on the unit before pushing there again." + captures.summary(self.resolved)
               + self.deps_note)
        return failed(code, msg, firmware=self.fw, push=self.result, extra={"saved": True}, **kw)

    def check_job(self):
        a, cfg = self.args, self.ctx.config
        if a["export"]["format"] not in SUPPORTED_FORMATS:
            return refused("unsupported_arg", f"This bridge can't push export format "
                           f"{text.clean(a['export']['format'], 48)}; update the bridge.")
        if not cfg.allowed_slots.contains(self.slot):
            return refused("slot_not_allowed", f"Slot {self.slot} is outside the bridge's allowed slots "
                           f"({', '.join(cfg.allowed_slots.labels())}).")
        try:
            check_preset_name(self.name)
            self.plan = validate(a["export"], self.ctx.catalog, cfg.expected_firmware)
        except ExportRefused as e:
            return refused("validation", "The export failed the bridge's own checks: " + text.clean(str(e), 400))
        return None

    def check_slot(self, snap, cur):
        """Rules 3 and 4 against a listing and the open position (run before writing and again before saving).
        The collection is whichever one is open on the unit: it must be a regular (non-factory) 256-slot
        collection, the target slot must be the open one and empty in THAT collection's own listing, and once
        a push has chosen its collection it never changes."""
        slot, pos = self.slot, self.pos
        key = cur.get("folder_key")
        view = None if (not key or cur.get("is_factory")) else _folder_view(snap, key)
        if view is not None:
            folder = _folder_label(snap, key)
            if _occupied(view.my_presets, pos):
                return refused("slot_not_empty", f"Slot {slot} in {folder!r} is not empty on the unit "
                               f"(it holds {text.clean(view.my_presets.get(pos), NAME_LIMIT)!r}). The bridge never "
                               "overwrites a preset; pick an empty slot.", firmware=self.fw)
            dup = _same_name(view.my_presets, pos, self.name)
            if dup is not None:
                return refused("validation", f"Slot {slots.label(dup)} in {folder!r} already has a preset named "
                               f"{self.name!r}. Pick a different name: if the unit stores presets by name, a save "
                               "could replace it.", firmware=self.fw)
        if (not key or cur.get("position") != pos or cur.get("is_factory")
                or (self.folder_key is not None and key != self.folder_key)):
            seen = _open_desc(cur, snap)
            log.warning("push refused, open slot mismatch: wanted %s (position %s); unit reported folder=%r "
                        "position=%r is_factory=%r", slot, pos, text.clean(str(cur.get("folder_key")), 80),
                        cur.get("position"), cur.get("is_factory"))
            return refused("unsupported_arg", f"Select slot {slot} (empty) in the collection you want on the Quad "
                           "Cortex first, then send again. The bridge only builds into the empty slot that is "
                           f"open on the unit, so it never changes what you have loaded. The unit reports {seen} "
                           f"as open. If {slot} is already selected on the unit's screen, the bridge is "
                           "misreading the open slot: send the agent.log line starting 'push refused'.",
                           firmware=self.fw)
        if view is None:
            return refused("unsupported_arg", f"The collection open on the unit ({_folder_label(snap, key)!r}) isn't "
                           "listed as a complete 256-slot preset collection, so the bridge can't tell whether "
                           f"{slot} is empty there. Open a regular collection (for example My Presets) and send "
                           "again.", firmware=self.fw)
        self.folder_key, self.folder_name = key, _folder_label(snap, key)
        return None

    def check_unit(self):
        """Rules 3-5: fresh listing, the slot open with a blank grid, not expired; captures resolved."""
        snap = self.dev.read_directory()
        _check_alive(self.ctx)
        out = self.check_slot(snap, self.dev.setlist_position() or {})
        if out:
            return out
        if any(b.capture for b in self.plan.blocks):
            out = self.resolve_captures(snap)
            if out:
                return out
        live = self.dev.read_current_preset()
        _check_alive(self.ctx)
        if live is None:
            return failed("device_error", "The unit did not return the open slot's grid in time.", firmware=self.fw)
        if any(c.get("blocks") for c in live.get("chains", [])):
            return refused("slot_not_empty", f"Slot {self.slot} is open but its grid isn't blank (unsaved edits?). "
                           f"Re-select {self.slot} on the unit to discard them, then retry.", firmware=self.fw)
        if _expired(self.ctx, self.job):
            return Outcome("expired", "expired", "The request expired before the bridge got to it; "
                           "nothing was written.", firmware=self.fw)
        return None

    def resolve_captures(self, snap):
        if not snap.capture_folders:
            return failed("device_error", "The unit's directory listing had no capture folders (incomplete "
                          "listing?), so the bridge can't place captures. Nothing was written; send it again.",
                          firmware=self.fw)
        try:
            self.resolved = captures.resolve(self.plan.blocks, snap)
        except captures.CaptureRefused as e:
            return refused("validation", str(e), firmware=self.fw)
        self.plan = captures.with_resolution(self.plan, self.resolved, CAPTURE_BLOCK_IDS)
        return None

    def still_open(self) -> bool:
        cur = self.dev.setlist_position() or {}
        return (bool(self.folder_key) and cur.get("folder_key") == self.folder_key
                and cur.get("position") == self.pos and not cur.get("is_factory"))

    def op(self, fn, *args):
        """One device write. Before it, the target slot must still be the one open on the unit (writes
        address row/column/index, not a preset: after the owner selects another preset they would land
        in that preset's edit buffer), and the watchdog's verdict is checked before and after (G2: a
        call can block for seconds while the unit goes away, and a write that reaches nothing raises
        nothing). A single set_param_scenes / set_block_bypass runs to its end once started (about 17
        messages); the next write's check stops the build."""
        _check_alive(self.ctx)
        if not self.still_open():
            raise _SwitchedAway()
        r = fn(*args)
        _check_alive(self.ctx)
        self.ctx.sleep(PACE_S)
        return r

    def place(self, b):
        """Phase 1 for one block: the block, or for a capture block the V1/V2 block with its capture
        loaded (file_name first: a capture loaded after its controls could reset them)."""
        dev, plan = self.dev, self.plan
        if b.capture is not None:
            if not self.op(dev.place_capture, plan.row, b.col, b.model_id, b.capture.key, b.capture.name):
                return failed("device_error", f"The unit did not confirm placing capture "
                              f"{text.clean(b.capture.name, 64)!r} (a V{b.capture.version} capture block) in "
                              f"column {b.col}. Nothing was saved; re-select the slot on the unit to discard "
                              "the partial grid.", firmware=self.fw, push=self.result)
            self.ctx.sleep(CAPTURE_SETTLE_S)
        elif not self.op(dev.add_block, plan.row, b.col, b.model_id):
            log.warning("add_block not echoed by the unit: model %s (id %s) at row %s column %s (the unit has %s "
                        "to answer; placed so far: %s)", text.clean(b.model, 60), b.model_id, plan.row, b.col,
                        "0.8 s", ", ".join(f"col {x.col}" for x in plan.blocks if x.col < b.col) or "none")
            return failed("device_error", f"The unit did not confirm placing {b.model} in column {b.col}. "
                          "Nothing was saved; re-select the slot on the unit to discard the partial grid.",
                          firmware=self.fw, push=self.result)
        return None

    def build(self):
        """The writes, into the live edit buffer of the empty, open slot (module docstring, rule 6)."""
        dev, plan = self.dev, self.plan
        dev.begin_push(self.pos, self.name, plan, folder_key=self.folder_key)
        self.op(dev.set_routing, plan.row, plan.in_port, plan.out_port)
        for b in plan.blocks:                                          # phase 1: place
            out = self.place(b)
            if out:
                return out
        layout = dev.read_current_preset()                             # phase 2: layout (G1)
        _check_alive(self.ctx)
        if layout is None:
            return failed("device_error", "The unit did not return the grid after placing the blocks. Nothing "
                          "was saved; re-select the slot on the unit to discard the partial grid.",
                          firmware=self.fw, push=self.result)
        problem = _check_layout(plan, layout)
        if problem:
            log.error("layout check: %s", problem)
            return failed("device_error", f"The placed blocks don't match the bridge's catalog: "
                          f"{text.clean(problem, 200)}. No parameter was written and nothing was saved; "
                          "re-select the slot on the unit to discard the partial grid.",
                          firmware=self.fw, push=self.result)
        for b in plan.blocks:                                          # phase 3: values and bypass
            out = self.recheck(b)
            if out:
                return out
            for m in b.mics:   # a cab mic first, like a capture's file_name: UNVERIFIED whether the unit resets the cab's controls
                self.op(dev.set_mic, plan.row, b.col, m.index, m.value)
            for p in b.params:
                if p.scene_wires:  # G6: unwritten scenes keep their old value, so all eight are written
                    self.op(dev.set_param_scenes, plan.row, b.col, p.index, list(p.scene_wires))
                else:              # G6: one plain write reads back replicated to all eight scenes
                    self.op(dev.set_param, plan.row, b.col, p.index, p.wire)
            # all eight flags on every block (owner decision): a new block starting active is not assumed
            self.op(dev.set_block_bypass, plan.row, b.col, list(b.bypass_scenes), b.bypass_index)
        return None

    def recheck(self, b):
        """Before one block's writes: read the grid again and check this column still holds the planned
        model with param_count + 1 params (the owner could swap a block on the unit during the build,
        and writes address column and index, not a model). The same read confirms the block's bypass
        control to the wire guard, which allows it only from a read like this one."""
        grid = self.dev.read_current_preset()
        _check_alive(self.ctx)
        problem = "the unit did not return the grid" if grid is None else _check_layout(self.plan, grid, [b])
        if problem:
            log.error("layout re-check before column %d: %s", b.col, problem)
            return failed("device_error", f"The grid changed on the unit during the build ({text.clean(problem, 160)})."
                          " Nothing was saved; re-select the slot on the unit to discard the partial grid.",
                          firmware=self.fw, push=self.result)
        self.dev.confirm_layout(b.col, b.param_count + 1)
        return None

    def verify(self):
        """Read back and diff; any mismatch stops before the save."""
        self.ctx.sleep(0.5)
        back = self.dev.read_current_preset()
        _check_alive(self.ctx)
        if back is None:
            return failed("device_error", "The read-back timed out. Nothing was saved; re-select the slot on "
                          "the unit to discard the edits.", firmware=self.fw, push=self.result)
        mismatches, diff, problems = _diff(self.plan, back)
        self.result.update(mismatches=mismatches, diff=diff)
        try:  # evidence for P6 only: whatever goes wrong here must not change the push
            self.defaults_differ = _log_defaults(self.plan, back, self.ctx.catalog)
        except Exception:  # noqa: BLE001
            log.exception("the defaults log failed; the push goes on")
        if mismatches:
            for pr in problems[:10]:
                log.warning("read-back: %s", pr)
            return failed("readback_mismatch", f"The unit's read-back differs from the plan in {mismatches} "
                          "place(s). Nothing was saved; re-select the slot on the unit to discard the edits.",
                          firmware=self.fw, push=self.result)
        return None

    def assign_stomps(self):
        """Step 7b: the export's footswitches, after a clean read-back and before the save, then read back."""
        dev, plan = self.dev, self.plan
        if not plan.stomps:
            return None
        dev.arm_stomps(plan.stomps)
        for s in plan.stomps:
            self.op(dev.assign_stomp, s.row, s.col, s.index)  # the row it was armed on (always 0 in v1)
        self.ctx.sleep(0.5)
        back = dev.read_current_preset()
        _check_alive(self.ctx)
        if back is None:
            return failed("device_error", "The read-back after assigning the footswitches timed out. Nothing was "
                          "saved; re-select the slot on the unit to discard the edits.",
                          firmware=self.fw, push=self.result)
        mismatches, diff, problems = _diff(plan, back)
        stomp_problems = _stomp_problems(plan, back)
        if mismatches or stomp_problems:
            self.result.update(mismatches=mismatches + len(stomp_problems), diff=diff)
            for pr in (stomp_problems + problems)[:10]:
                log.warning("read-back after the footswitches: %s", text.clean(pr, 200))
            what = "footswitch assignments" if stomp_problems else "values"
            return failed("readback_mismatch", f"After assigning the footswitches, the unit's {what} differ from "
                          f"the plan in {mismatches + len(stomp_problems)} place(s). Nothing was saved; re-select the "
                          "slot on the unit to discard the edits.", firmware=self.fw, push=self.result)
        log.info("footswitches read back as planned: %s",
                 ", ".join(f"{s.switch}=column {s.col}" for s in plan.stomps))
        return None

    def save(self):
        """Rule 6: a second fresh listing, then File CREATE, then wait for the listing to show it."""
        dev, ctx, slot, pos, name = self.dev, self.ctx, self.slot, self.pos, self.name
        snap = dev.read_directory()
        cur = dev.setlist_position() or {}
        _check_alive(ctx)
        out = self.check_slot(snap, cur)
        if out:
            out.message = f"The unit changed while the preset was being built; nothing was saved. {out.message}"
            out.push = self.result
            return out
        if self.resolved:
            try:
                again = captures.resolve(self.plan.blocks, snap)
            except captures.CaptureRefused:
                again = None
            if again != self.resolved:
                return failed("device_error", "The unit's capture library changed during the build (a capture no "
                              "longer resolves to the same file). Nothing was saved; re-select "
                              f"{slot} on the unit to discard the partial grid.", firmware=self.fw, push=self.result)
        back = dev.read_current_preset()  # the listing takes ~15 s: the grid must still be the verified one
        _check_alive(ctx)
        if back is None:
            log.warning("pre-save re-read: the read timed out")
            return failed("device_error", "The re-read just before the save timed out. Nothing was saved; "
                          "re-select the slot on the unit to discard the edits.", firmware=self.fw, push=self.result)
        mismatches, diff, problems = _diff(self.plan, back)
        stomp_problems = _stomp_problems(self.plan, back)
        if mismatches or stomp_problems:
            bad = [f"column {d['col']} param {d['param_index']}: expected {d['expected']}, read {d['readback']}"
                   for d in diff if not d.get("ok")]  # numeric values: ours and the unit's, not device text
            for pr in (stomp_problems + problems + bad)[:10]:
                log.warning("pre-save re-read: %s", text.clean(pr, 200))
            what = ("the footswitch assignments differ" if stomp_problems and not mismatches
                    else "values or bypass flags differ" if mismatches and not stomp_problems
                    else "values and footswitch assignments differ")
            return failed("readback_mismatch", f"The grid changed after it was verified (edited on the unit?): "
                          f"{what} from the plan in {mismatches + len(stomp_problems)} place(s). Nothing was "
                          "saved; re-select the slot to discard the edits.", firmware=self.fw, push=self.result)
        if not self.still_open():  # File CREATE saves whatever grid is live: the slot must still be the open one
            return failed("device_error", "Another preset was selected on the unit just before the save. "
                          "Nothing was saved.", firmware=self.fw, push=self.result)
        dev.save_to_slot(pos, name)
        self.saved = True
        try:  # before the polls: a save that lands late is still the bridge's own slot
            ctx.owned.record(slot, pos, name, self.job["job_id"], contract.format_time(ctx.now()))
        except Exception as e:  # noqa: BLE001  the save was issued; a local ledger problem mustn't change the outcome
            # broad on purpose: anything escaping here would turn an issued save into "internal error" and lose
            # the saved flag; the traceback goes to the log so a bug is still visible
            log.error("could not record the owned slot %s: %s", slot, text.clean(str(e), 200), exc_info=True)
        listed = False
        for n in range(SAVE_LIST_POLLS):
            ctx.sleep(SAVE_LIST_INTERVAL_S)
            _check_alive(ctx)
            try:
                snap = dev.read_directory()
            except (UnitAbsent, UnitBusy):
                raise
            except PartialListing as e:
                # Inferred, not established: after File CREATE the unit may send a My Presets update carrying only
                # the changed slot. A short table is never the full one, so this listing doesn't count (and is
                # never "listed"); the backend already logged what it carried. No merge rule is invented.
                log.warning("post-save listing %d/%d did not count: %s", n + 1, SAVE_LIST_POLLS, e)
                continue
            except DeviceError as e:
                # The save was issued. One incomplete listing (seen on the owner's unit: "My Presets is missing")
                # says nothing about whether it landed: look again rather than fail a push that may have worked.
                log.warning("listing after the save was incomplete (%s); will look again", text.clean(str(e), 120))
                continue
            view = _folder_view(snap, self.folder_key)
            if view is not None and view.my_presets.get(pos) == name:
                listed = True
                self.deps_note = captures.deps_report(view, pos, self.resolved)
                break
        if not listed:
            log.warning("save to %s not seen after %d listings", slot, SAVE_LIST_POLLS)
            if self.resolved:
                self.deps_note = captures.deps_report(captures.DirectorySnapshot(my_presets={}), pos, self.resolved)
            return self.unconfirmed("device_error", f"but the unit's listing didn't show {name!r} after "
                                    f"{SAVE_LIST_POLLS} checks (about 2 minutes)")
        labels = " Scene labels are not written in v1." if self.plan.scene_labels else ""
        note = (f" {self.defaults_differ} control(s) the preset leaves at default read back differently; "
                "see the bridge log." if self.defaults_differ else "")
        stomps = (f" {len(self.plan.stomps)} footswitch(es) assigned and read back; check them on the unit."
                  if self.plan.stomps else "")
        return self.done(f"Saved {name!r} to {slot} in {self.folder_name!r}; read-back matched.{stomps}{labels}{note}")


def push_preset(ctx, job) -> Outcome:
    return _Push(ctx, job).run()


# ---- edit_preset (preset-editing plan E4; contract sections 7.1, 7.2, 9.3) ----

PIN_WORDS = {"folder_key": "the collection", "position": "the slot", "name": "the preset's name",
             "layout": "the blocks on the grid", "param_expect": "a control's value", "bypass_expect": "a block's bypass"}
EDIT_RESULT_FORMAT = "qc-device-edit-result/1"


def _edit_job_plan(job):
    return contract.parse_edit_plan(job["args"]["plan"]["payload"])


def edit_precheck(ctx, job) -> Outcome | None:
    """Everything an edit can be refused for without opening the unit, in the order the design lists it:
    the PC's opt-in, the mode this build honours, the slot, then the catalog checks. A refusal here has no
    `edit` block (contract 9: null when the agent refused the job before reaching the unit)."""
    return _Edit(ctx, job).precheck()


class _Edit:
    """One edit_preset job. Commit order in the module docstring of qc_agent.edit_checks; the writes are
    armed per operation in the wire guard (qcmcp_backend.WireGuard.arm_edit)."""

    def __init__(self, ctx, job):
        self.ctx, self.job, self.dev = ctx, job, ctx.device
        a = job["args"]
        self.args, self.slot, self.mode = a, a["slot"], a["mode"]
        self.sha = a["plan"]["sha256"]
        self.plan = _edit_job_plan(job)
        self.pos = slots.position(self.slot)
        self.checked = None
        self.fw = None
        self.cas = {"state": "not_reached", "failed": []}

    # -- the result block (contract 9.3) --

    def edit_block(self, params=None, bypass=None, collateral=None, rollback=None, saved=False):
        payload = {"plan_sha256": self.sha, "snapshot": self.plan["snapshot"], "mode": self.mode,
                   "cas": {"state": self.cas["state"], "failed": list(self.cas["failed"])},
                   "params": params or [], "bypass": bypass or [], "collateral": collateral,
                   "rollback": rollback or {"state": "not_needed", "restored": 0, "failed": 0},
                   "save": {"saved": saved, "slot": None, "name": None, "listed": False, "backup": None}}
        return {"format": EDIT_RESULT_FORMAT, "payload": payload}

    def refuse(self, code, msg, **kw):
        return refused(code, msg, firmware=self.fw, edit=self.edit_block(), **kw)

    def fail(self, code, msg, **kw):
        return failed(code, msg, firmware=self.fw, edit=self.edit_block(), **kw)

    # -- before any device contact --

    def precheck(self) -> Outcome | None:
        cfg = self.ctx.config
        if not cfg.edit_modes:
            return refused("edit_not_enabled", "Editing presets on the unit is switched off on this PC "
                           "(QC_AGENT_EDIT_MODES in agent.env is empty). Nothing was changed.")
        if self.mode != "live":
            return refused("unsupported_arg", f"This bridge only edits in live mode so far; {self.mode} is not "
                           "supported. Nothing was changed.")
        if "live" not in cfg.edit_modes:
            return refused("edit_not_enabled", "Live edits are not switched on on this PC "
                           "(QC_AGENT_EDIT_MODES in agent.env does not list live). Nothing was changed.")
        if not cfg.allowed_slots.contains(self.slot):
            return refused("slot_not_allowed", f"Slot {self.slot} is outside the bridge's allowed slots "
                           f"({', '.join(cfg.allowed_slots.labels())}).")
        if cfg.protected_slots is not None and cfg.protected_slots.contains(self.slot):
            return refused("slot_not_allowed", f"Slot {self.slot} is protected on this PC "
                           "(QC_AGENT_PROTECTED_SLOTS): the bridge never edits it.")
        try:
            self.checked = edit_checks.catalog_check(self.plan, self.ctx.catalog, cfg.expected_firmware)
        except edit_checks.EditRefused as e:
            return refused("validation", "The edit failed the bridge's own checks: " + text.clean(str(e), 400))
        return None

    # -- on the unit --

    def run(self) -> Outcome:
        out = self.precheck()                 # again: executor() may be called without run()'s precheck
        if out:
            return out
        out = _connect_checked(self.ctx)
        if out:
            return out
        self.fw = _fw(self.dev.firmware)
        try:
            out = self.check_unit()
            if out is not None:
                return out
            return self.apply()
        except (_Gone, UnitAbsent):
            return self.fail("unit_absent", "The Quad Cortex went away during the edit.", unit_gone=True)
        except DeviceError as e:
            log.warning("edit failed: %s", text.clean(str(e), 200))
            return self.fail("device_error", "A device operation failed. Nothing was saved.")

    def check_unit(self) -> Outcome | None:
        """Open slot only (never recall), a regular collection, one fresh read and the compare-and-swap."""
        snap = self.dev.read_directory()
        _check_alive(self.ctx)
        before = self.dev.setlist_position() or {}
        preset = self.dev.read_current_preset()
        after = self.dev.setlist_position() or {}
        _check_alive(self.ctx)
        if preset is None:
            return self.fail("device_error", "The unit did not return the open preset in time.")
        if after != before:
            return self.fail("device_error", "The selected preset changed during the read; read again.")
        key = before.get("folder_key")
        if before.get("is_factory") or not key:
            return self.refuse("unsupported_arg", f"Select the preset to edit on the Quad Cortex first. The bridge only "
                               f"edits the open preset and never recalls one. The unit reports {_open_desc(before, snap)}.")
        if _folder_view(snap, key) is None:
            return self.refuse("unsupported_arg", f"The collection open on the unit ({_folder_label(snap, key)!r}) isn't "
                               "listed as a complete 256-slot preset collection, so the bridge won't edit in it.")
        pins = edit_checks.cas_pins(self.plan, before, preset)
        if pins:
            self.cas = {"state": "mismatch", "failed": pins}
            log.warning("edit refused, compare-and-swap differs: %s", ", ".join(pins))
            return self.refuse("preset_changed", "The preset on the unit is not the one that was read ("
                               + ", ".join(PIN_WORDS[p] for p in pins) + " differ). Nothing was changed. "
                               "Read it again with read_device_preset and plan the edit again.")
        self.cas = {"state": "matched", "failed": []}
        problem = edit_checks.unit_layout_problem(self.plan, self.checked, preset)
        if problem:
            log.error("edit refused, layout check: %s", problem)
            return self.refuse("validation", "The blocks on the unit don't match the bridge's catalog: "
                               + text.clean(problem, 200) + ". Nothing was changed.")
        if _expired(self.ctx, self.job):
            return Outcome("expired", "expired", "The request expired before the bridge got to it; "
                           "nothing was written.", firmware=self.fw, edit=self.edit_block())
        self.pre, self.folder_key = preset, key
        return None

    # -- the writes --

    def still_open(self) -> bool:
        cur = self.dev.setlist_position() or {}
        return (cur.get("folder_key") == self.folder_key and cur.get("position") == self.pos
                and not cur.get("is_factory"))

    def op(self, key, fn, *args):
        """One device write, like _Push.op: alive and still the open preset before it (writes address row,
        column and index, not a preset: after the owner selects another preset they would land in it), and
        again after it (a multi-message write that spans a switch must be noticed). The op counts as
        attempted from the moment its call starts."""
        _check_alive(self.ctx)
        if not self.still_open():
            raise _SwitchedAway()
        self.attempted.append(key)
        fn(*args)
        _check_alive(self.ctx)
        if not self.still_open():
            self.spilled = True
            raise _SwitchedAway()
        self.ctx.sleep(PACE_S)

    def recheck(self, col):
        """Before a column's first write: the grid is read again and the column must still hold the planned
        model with param_count + 1 params (the owner could swap a block while the edit runs)."""
        mid, pc = self.checked.cells[col]
        grid = self.dev.read_current_preset()
        _check_alive(self.ctx)
        b = edit_checks.live_block(grid, 0, col) if grid is not None else None
        if b is None or b.get("model_id") != mid or len(b.get("params") or []) != pc + 1:
            raise DeviceError(f"column {col} changed on the unit before it was written")

    def write_ops(self):
        plan, dev = self.plan, self.dev
        done = set()
        for op in plan["param_ops"]:
            col = op["col"]
            if col not in done:
                done.add(col)
                self.recheck(col)
            key = ("param", col, op["index"])
            if op["scene_mode"]:   # G6: all eight scene values (the plan carries C-H exactly as the unit had them)
                self.op(key, dev.set_param_scenes, 0, col, op["index"], list(op["wire"]))
            else:                  # one plain write replicates to all eight (G1/G6); verify checks all eight
                self.op(key, dev.set_param, 0, col, op["index"], op["wire"][0])
        for op in plan["bypass_ops"]:
            col = op["col"]
            if col not in done:
                done.add(col)
                self.recheck(col)
            self.op(("bypass", col), dev.set_block_bypass, 0, col, list(op["bypass"]), self.checked.bypass[col]["index"])

    def read_after(self):
        """A read of the open preset after the writes: once more if the unit didn't answer."""
        for _ in range(2):
            self.ctx.sleep(0.5)
            post = self.dev.read_current_preset()
            _check_alive(self.ctx)
            if post is not None:
                return post
        return None

    def layout_hash(self, preset):
        if preset is None:
            return None
        return hashlib.sha256(json.dumps(edit_checks.live_layout(preset), separators=(",", ":")).encode()).hexdigest()

    def apply(self) -> Outcome:
        plan, dev, ctx = self.plan, self.dev, self.ctx
        dev.begin_edit(plan, self.checked)
        for col, (mid, pc) in sorted(self.checked.cells.items()):
            dev.confirm_layout(col, len(edit_checks.live_block(self.pre, 0, col)["params"]))
        orig_scene = dev.read_scene()
        before = self.layout_hash(self.pre)
        if ctx.edits is not None:
            try:  # before the first write: a crash from here on is reported with the slot (agent.recover)
                ctx.edits.start(self.job["job_id"], self.slot, self.folder_key, plan["expect"]["name"], before,
                                contract.format_time(ctx.now()))
            except Exception as e:  # noqa: BLE001
                log.error("could not record the edit: %s", text.clean(str(e), 200), exc_info=True)
                return self.fail("device_error", "The bridge could not write its edit ledger, so it wrote nothing.")
        self.attempted, self.spilled = [], False
        cause, gone, switched = None, False, False
        try:
            self.write_ops()
        except _SwitchedAway:
            switched = True
        except (_Gone, UnitAbsent):
            gone = True
        except DeviceError as e:   # includes the wire guard's refusals
            cause = text.clean(str(e), 160)
            log.warning("edit write failed: %s", cause)
        post = None
        if not (gone or switched):
            try:
                post = self.read_after()
            except (_Gone, UnitAbsent):
                gone = True
            except DeviceError as e:
                cause = cause or text.clean(str(e), 160)
        params, bypass, ok = edit_diff.verify_ops(plan, post or {})
        coll = edit_diff.collateral(self.pre, post, plan) if post is not None else None
        good = (cause is None and post is not None and ok and coll["mismatches"] == 0
                and not (gone or switched))
        if good:
            note = self.restore_scene(orig_scene)
            self.record_edited(post, "succeeded")
            n, m = len(plan["param_ops"]), len(plan["bypass_ops"])
            return Outcome("succeeded", "", f"Applied {n} parameter change(s) and {m} bypass change(s) to the open "
                           f"preset {plan['expect']['slot']}; every value read back as planned and nothing else changed. "
                           f"NOT saved: save it on the unit, or re-select the slot to discard it.{note}",
                           firmware=self.fw, edit=self.edit_block(params, bypass, coll))
        return self.rollback(post, params, bypass, coll, cause, gone, switched, orig_scene)

    def restore_scene(self, orig) -> str:
        """Put the unit's scene back to what it was before the per-scene writes (they leave it on scene A).
        Returns a sentence for the message; never raises."""
        try:
            if orig is None:
                return " The scene that was active before could not be read, so the unit may be on scene A."
            cur = self.dev.read_scene()
            if cur == orig:
                return ""
            self.dev.restore_scene(orig)
            self.ctx.sleep(0.3)
            if self.dev.read_scene() == orig:
                return ""
        except (DeviceError, _Gone, UnitAbsent) as e:
            log.warning("scene not restored: %s", text.clean(str(e), 120))
        return f" Check the active scene on the unit: it was {'ABCDEFGH'[orig]} before the edit."

    def record_edited(self, post, status):
        if self.ctx.edits is None:
            return
        try:
            self.ctx.edits.finish(self.job["job_id"], self.slot, self.plan["expect"]["name"], self.layout_hash(self.pre),
                                  self.layout_hash(post), status, contract.format_time(self.ctx.now()))
        except Exception as e:  # noqa: BLE001  informational ledger: never change the outcome
            log.error("could not record the edit result: %s", text.clean(str(e), 200), exc_info=True)

    def rollback(self, post, params, bypass, coll, cause, gone, switched, orig_scene) -> Outcome:
        plan, dev = self.plan, self.dev
        started = dev.edit_started()
        steps = edit_diff.rollback_plan(plan, [a for a in self.attempted if a in started])
        restored = failed = 0
        rb_state = "not_needed"
        residual = 0
        post2 = post
        slot = plan["expect"]["slot"]
        if steps:
            if gone or switched or post is None:
                failed = len(steps)   # nothing is written into a preset we can't see or that isn't the checked one
            else:
                restored, failed, residual, post2 = self.undo(steps, post)
            if failed == 0 and residual == 0:
                rb_state = "restored"
            elif restored == 0:
                rb_state = "failed"
            else:
                rb_state = "partial"
        rollback = {"state": rb_state, "restored": restored, "failed": failed}
        note = ""
        if rb_state in ("restored", "not_needed") and not (gone or switched) and post is not None:
            note = self.restore_scene(orig_scene)
        self.record_edited(post2 if post2 is not None else post, "failed")
        block = self.edit_block(params, bypass, coll, rollback)
        why = (f" ({cause})" if cause else "")
        if rb_state in ("partial", "failed"):
            where = ("The preset that is open now may have changed too: " if self.spilled else "")
            return failed_out("rollback_failed", f"The edit did not verify and the bridge could not put every value back"
                              f"{why}. {where}The open preset {slot} may be half-edited and was NOT saved: re-select {slot} "
                              f"on the unit WITHOUT saving to discard it.", self.fw, block, gone)
        if gone:
            return failed_out("unit_absent", f"The Quad Cortex went away during the edit{why}. Nothing was saved.", self.fw, block, True)
        if cause or switched:
            return failed_out("device_error", ("Another preset was selected on the unit during the edit" if switched
                              else "A device operation failed during the edit" + why)
                              + f". The edit was rolled back where it could be and nothing was saved; re-select {slot} "
                              "on the unit without saving." + note, self.fw, block, False)
        return failed_out("readback_mismatch", f"The unit's read-back differs from the edit's plan, so the bridge put "
                          f"the original values back ({restored} change(s)) and checked them. Nothing was saved.{note} "
                          "Controls assigned to scenes stay assigned (with their original values).", self.fw, block, False)

    def undo(self, steps, post):
        """Write the rollback steps (the guard is in its rollback phase), read again, count. Returns
        (restored, failed, residual, the read after)."""
        plan, dev = self.plan, self.dev
        dev.enter_rollback()
        bad_cols = set()
        for col, (mid, pc) in self.checked.cells.items():   # a column that isn't what the plan says is not written
            b = edit_checks.live_block(post, 0, col)
            if b is None or b.get("model_id") != mid or len(b.get("params") or []) != pc + 1:
                bad_cols.add(col)
            else:
                dev.confirm_layout(col, len(b.get("params") or []))   # the measured count
        todo = []
        stop = False
        for st in steps:
            if stop or st["col"] in bad_cols:
                continue
            try:
                if st["kind"] == "bypass":
                    self.op(("rb", "bypass", st["col"]), dev.set_block_bypass, 0, st["col"], list(st["flags"]),
                            self.checked.bypass[st["col"]]["index"])
                elif st["scene_mode"]:
                    self.op(("rb", "param", st["col"], st["index"]), dev.set_param_scenes, 0, st["col"], st["index"],
                            list(st["wires"]))
                else:
                    self.op(("rb", "param", st["col"], st["index"]), dev.set_param, 0, st["col"], st["index"], st["wires"][0])
                todo.append(st)
            except _SwitchedAway:
                stop = True
            except (_Gone, UnitAbsent):
                stop = True
            except DeviceError as e:
                log.warning("rollback write failed: %s", text.clean(str(e), 160))
        try:
            post2 = None if stop else self.read_after()
        except (_Gone, UnitAbsent, DeviceError):
            post2 = None
        if post2 is None:
            return 0, len(steps), 0, None
        restored = 0
        for st in todo:
            if st["kind"] == "bypass":
                ok = edit_checks.live_bypass(post2, 0, st["col"]) == list(st["flags"])
            else:
                p = edit_checks.live_param(edit_checks.live_block(post2, 0, st["col"]), st["index"])
                vals = (p or {}).get("values")
                ok = (isinstance(vals, list) and len(vals) == 8 and all(
                    type(v) is float and abs(v - w) <= edit_diff.TOL for v, w in zip(vals, st["wires"])))
            restored += ok
        failed = len(steps) - restored
        # Not collateral(pre, post2): a change the edit did not make (the owner touching a control) is reported by the
        # first collateral check and cannot be undone by writing our ops back. Residual is OUR ops left wrong.
        residual = 0
        done = {(s["kind"], s["col"], s.get("index")) for s in steps}
        for op in plan["param_ops"]:    # planned ops never started: collateral skips them, so check them here
            if ("param", op["col"], op["index"]) not in done:
                p = edit_checks.live_param(edit_checks.live_block(post2, 0, op["col"]), op["index"])
                residual += not edit_checks.floats_equal((p or {}).get("values"), op["expect"])
        for op in plan["bypass_ops"]:
            if ("bypass", op["col"], None) not in done:
                residual += edit_checks.live_bypass(post2, 0, op["col"]) != list(op["expect"])
        if residual and not failed:
            failed = residual   # ops left wrong count as failed, so the counts fit the state (contract 9.3)
        return restored, failed, residual, post2


def failed_out(code, msg, fw, block, gone):
    return Outcome("failed", code, msg, firmware=fw, edit=block, unit_gone=gone)


def edit_preset(ctx, job) -> Outcome:
    return _Edit(ctx, job).run()


# ---- the kinds that run, and how (below every executor so the tables can name them) ----

# The kinds that run on the unit. A literal table, like REFUSED_KINDS.
EXECUTORS = {"list_slots": list_slots, "read_preset": read_preset, "push_preset": push_preset,
             "sync_device_library": sync_device_library, "edit_preset": edit_preset}
# Checks a kind makes BEFORE any device contact, with the same early return as REFUSED_KINDS: a refusal here
# never opens, arms or disconnects the unit (an edit the PC hasn't switched on must not even connect).
PRECHECKS = {"edit_preset": edit_precheck}


def run(ctx, job) -> Outcome:
    kind = job["kind"]
    if kind in REFUSED_KINDS:
        return refused("unsupported_kind", REFUSED_KINDS[kind])
    executor = EXECUTORS.get(kind)
    if executor is None:  # parse_job admits no other kind; keep the code anyway
        return refused("unsupported_kind", "This bridge doesn't run that kind of job.")
    check = PRECHECKS.get(kind)
    if check is not None:
        bad = check(ctx, job)
        if bad is not None:
            return bad
    try:
        return executor(ctx, job)
    finally:
        try:
            ctx.device.end_push()
        except Exception:  # noqa: BLE001
            log.exception("end_push failed")
        ctx.device.disconnect()
