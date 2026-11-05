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
  6. build, read back and diff; save only if the diff is clean, after a second fresh
     listing still shows the slot empty and the same slot still open.
"""
from __future__ import annotations

import json
import logging
import math
import time
from dataclasses import dataclass, field

from . import contract, slots, text
from .device import SETLIST_KEY, SETLIST_NAME, DeviceError, UnitAbsent, UnitBusy
from .export_validator import SUPPORTED_FORMATS, ExportRefused, validate

log = logging.getLogger("qc_agent")

TOL = 1e-4                 # G3 read-back tolerance
PACE_S = 0.15              # between writes; G3 used the same pacing
SAVE_LIST_POLLS = 6        # G3: a listing 3 s after the save still showed the slot blank
SAVE_LIST_INTERVAL_S = 6.0
MAX_DIFF = 512
NAME_LIMIT = 64


@dataclass
class Outcome:
    status: str                 # succeeded | failed | refused | expired
    reason_code: str = ""
    message: str = ""
    firmware: str | None = None
    push: dict | None = None
    slots: dict | None = None
    readback: dict | None = None
    unit_gone: bool = False     # the agent exits after this job
    extra: dict = field(default_factory=dict)


def refused(code, msg, **kw):
    return Outcome("refused", code, msg, **kw)


def failed(code, msg, **kw):
    return Outcome("failed", code, msg, **kw)


class JobContext:
    def __init__(self, config, catalog, device, owned, now_fn, sleep=time.sleep):
        self.config, self.catalog, self.device, self.owned = config, catalog, device, owned
        self.now, self.sleep = now_fn, sleep
        self.firmware_seen = None  # (firmware, when) for the heartbeat


class _Gone(Exception):
    pass


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
    if dev.device_type not in (None, "QC"):
        return refused("firmware_mismatch",
                       f"The unit reports device type {text.clean(str(dev.device_type), 16)}; this bridge "
                       "supports a full Quad Cortex only. Nothing was changed.", firmware=_fw(fw))
    return None


def _fw(fw):
    return text.clean(fw, 32) if isinstance(fw, str) and fw else None


def _check_alive(ctx):
    if not ctx.device.alive():
        raise _Gone()


def run(ctx, job) -> Outcome:
    kind = job["kind"]
    executor = {"list_slots": list_slots, "read_preset": read_preset, "push_preset": push_preset}.get(kind)
    if executor is None:  # parse_job only admits the three kinds; keep the code anyway
        return refused("unsupported_kind", "This bridge doesn't run that kind of job.")
    try:
        return executor(ctx, job)
    finally:
        try:
            ctx.device.end_push()
        except Exception:  # noqa: BLE001
            log.exception("end_push failed")
        ctx.device.disconnect()


# ---- list_slots ----

def _slot_table(ctx, listing):
    out = []
    for pos in range(slots.SLOT_COUNT):
        raw = listing.get(pos, "")
        name = text.clean(raw, NAME_LIMIT)
        occupied = bool((raw or "").strip())
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
        return text.clean(v, NAME_LIMIT)
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
        _check_alive(ctx)
    except (_Gone, UnitAbsent):
        return failed("unit_absent", "The Quad Cortex went away during the read.", firmware=fw, unit_gone=True)
    except DeviceError as e:
        log.warning("read failed: %s", text.clean(str(e), 200))
        return failed("device_error", "Reading the current preset failed.", firmware=fw)
    if preset is None:
        return failed("device_error", "The unit did not return the current preset in time.", firmware=fw)
    slot = None
    p = pos.get("position")
    if (pos.get("folder_key") == SETLIST_KEY and not pos.get("is_factory")
            and isinstance(p, int) and 0 <= p < slots.SLOT_COUNT):
        slot = slots.label(p)
    info = {"folder": SETLIST_NAME if pos.get("folder_key") == SETLIST_KEY and not pos.get("is_factory") else "other",
            "position": p if slot else None}
    name = text.clean(preset.get("name", ""), NAME_LIMIT)
    rb = {"format": "qc-device-readback/1", "slot": slot, "name": name or None,
          "payload": readback_payload(preset, info)}
    if len(json.dumps(rb, separators=(",", ":"))) > contract.MAX_RESULT_BYTES - 16 * 1024:
        return failed("device_error", "The open preset is too large to return.", firmware=fw)
    nblocks = sum(len(c["blocks"]) for c in rb["payload"]["chains"])
    return Outcome("succeeded", "", f"Read the open preset ({nblocks} blocks).", firmware=fw, readback=rb)


# ---- push_preset ----

def _expired(ctx, job) -> bool:
    return not ctx.now() < contract.parse_time(job["expires_at"])


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
        for p in b.params:
            vals = (params.get(p.index) or {}).get("values") or []
            expected = p.scene_wires or [p.wire]
            scenes = range(8) if p.scene_wires else [0]
            for s in scenes:
                v = vals[s] if s < len(vals) else None
                if not p.scene_wires and vals:
                    # Not scene-assigned: the unit replicates one value into all 8 slots (G1).
                    ok = all(isinstance(x, float) and abs(x - p.wire) <= TOL for x in vals)
                else:
                    ok = isinstance(v, float) and abs(v - expected[s]) <= TOL
                if not ok:
                    mismatches += 1
                if len(diff) < MAX_DIFF:
                    e = {"row": plan.row, "col": b.col, "param_index": p.index, "expected": expected[s],
                         "readback": v if isinstance(v, float) and math.isfinite(v) else None, "ok": ok}
                    if p.scene_wires:
                        e["scene"] = "ABCDEFGH"[s]
                    diff.append(e)
        want = b.bypass_scenes
        have = None
        if bypass and len(bypass) > plan.row and len(bypass[plan.row]) > b.col:
            have = [bool(x) for x in bypass[plan.row][b.col][:8]]
        if have is None or len(have) < 8:
            if any(want):
                mismatches += 1
                problems.append(f"column {b.col}: bypass flags missing from the read-back")
        elif have != want:
            mismatches += 1
            problems.append(f"column {b.col}: bypass reads {have}, expected {want}")
    return mismatches, diff, problems


def push_preset(ctx, job) -> Outcome:
    a = job["args"]
    slot, name = a["slot"], a["name"]
    pos = slots.position(slot)
    result_push = {"slot": slot, "name": name, "mismatches": 0, "cpu_load": None, "diff": []}
    saved = False

    if a["export"]["format"] not in SUPPORTED_FORMATS:
        return refused("unsupported_arg", f"This bridge can't push export format "
                       f"{text.clean(a['export']['format'], 48)}; update the bridge.")
    if not ctx.config.allowed_slots.contains(slot):
        return refused("slot_not_allowed", f"Slot {slot} is outside the bridge's allowed slots "
                       f"({', '.join(ctx.config.allowed_slots.labels())}).")
    try:
        plan = validate(a["export"], ctx.catalog, ctx.config.expected_firmware)
    except ExportRefused as e:
        return refused("validation", "The export failed the bridge's own checks: " + text.clean(str(e), 400))

    bad = _connect_checked(ctx)
    if bad:
        return bad
    dev = ctx.device
    fw = _fw(dev.firmware)
    try:
        listing = dev.list_my_presets()
        _check_alive(ctx)
        if (listing.get(pos) or "").strip():
            return refused("slot_not_empty", f"Slot {slot} is not empty on the unit "
                           f"(it holds {text.clean(listing.get(pos), NAME_LIMIT)!r}). The bridge never "
                           "overwrites a preset; pick an empty slot.", firmware=fw)
        cur = dev.setlist_position() or {}
        if cur.get("folder_key") != SETLIST_KEY or cur.get("position") != pos or cur.get("is_factory"):
            return refused("unsupported_arg", f"Select slot {slot} (empty) on the Quad Cortex first, then send "
                           "again. The bridge only builds into the empty slot that is open on the unit, so it "
                           "never changes what you have loaded.", firmware=fw)
        live = dev.read_current_preset()
        _check_alive(ctx)
        if live is None:
            return failed("device_error", "The unit did not return the open slot's grid in time.", firmware=fw)
        if any(c.get("blocks") for c in live.get("chains", [])):
            return refused("slot_not_empty", f"Slot {slot} is open but its grid isn't blank (unsaved edits?). "
                           f"Re-select {slot} on the unit to discard them, then retry.", firmware=fw)
        if _expired(ctx, job):
            return Outcome("expired", "expired", "The request expired before the bridge got to it; "
                           "nothing was written.", firmware=fw)

        # ---- writes: the live edit buffer of the empty, open slot ----
        dev.begin_push(pos, name, plan)
        dev.set_routing(plan.row, plan.in_port, plan.out_port)
        ctx.sleep(PACE_S)
        for b in plan.blocks:
            _check_alive(ctx)
            if not dev.add_block(plan.row, b.col, b.model_id):
                return failed("device_error", f"The unit did not confirm placing {b.model} in column {b.col}. "
                              "Nothing was saved; re-select the slot on the unit to discard the partial grid.",
                              firmware=fw, push=result_push)
            ctx.sleep(PACE_S)
            for p in b.params:
                if p.scene_wires:
                    dev.set_param_scenes(plan.row, b.col, p.index, p.scene_wires)
                else:
                    dev.set_param(plan.row, b.col, p.index, p.wire)
                ctx.sleep(PACE_S)
            if any(b.bypass_scenes):  # a new block starts active; write flags only when some scene bypasses it
                dev.set_block_bypass(plan.row, b.col, b.bypass_scenes)
            _check_alive(ctx)

        ctx.sleep(0.5)
        back = dev.read_current_preset()
        _check_alive(ctx)
        if back is None:
            return failed("device_error", "The read-back timed out. Nothing was saved; re-select the slot on "
                          "the unit to discard the edits.", firmware=fw, push=result_push)
        mismatches, diff, problems = _diff(plan, back)
        result_push.update(mismatches=mismatches, diff=diff)
        if mismatches:
            for pr in problems[:10]:
                log.warning("read-back: %s", pr)
            return failed("readback_mismatch", f"The unit's read-back differs from the plan in {mismatches} "
                          "place(s). Nothing was saved; re-select the slot on the unit to discard the edits.",
                          firmware=fw, push=result_push)

        # ---- save: re-check with a second fresh listing, then File CREATE ----
        listing = dev.list_my_presets()
        cur = dev.setlist_position() or {}
        _check_alive(ctx)
        if (listing.get(pos) or "").strip() or cur.get("position") != pos or cur.get("folder_key") != SETLIST_KEY:
            return refused("slot_not_empty", f"Slot {slot} changed while the preset was being built. "
                           "Nothing was saved.", firmware=fw, push=result_push)
        dev.save_to_slot(pos, name)
        saved = True
        result_push["listed"] = False
        ctx.owned.record(slot, pos, name, job["job_id"], contract.format_time(ctx.now()))
        listed = False
        for _ in range(SAVE_LIST_POLLS):
            ctx.sleep(SAVE_LIST_INTERVAL_S)
            _check_alive(ctx)
            if dev.list_my_presets().get(pos) == name:
                listed = True
                break
        result_push["listed"] = listed
        labels = (" Scene labels are not written in v1." if plan.scene_labels else "")
        if listed:
            return Outcome("succeeded", "", f"Saved {name!r} to {slot}; read-back matched.{labels}",
                           firmware=fw, push=result_push)
        return Outcome("succeeded", "", f"Save to {slot} issued and the read-back matched, but the unit's "
                       f"listing didn't show {name!r} within {int(SAVE_LIST_POLLS * SAVE_LIST_INTERVAL_S)} s "
                       f"(the unit can lag for minutes). Check {slot} on the unit.{labels}",
                       firmware=fw, push=result_push)
    except (_Gone, UnitAbsent):
        if saved:
            return Outcome("succeeded", "", f"Save to {slot} issued after a clean read-back, then the Quad "
                           f"Cortex went away before the save could be confirmed. Check {slot} on the unit.",
                           firmware=fw, push=result_push, unit_gone=True)
        return failed("unit_absent", "The Quad Cortex went away during the push. If anything was written it "
                      "was not saved.", firmware=fw, push=result_push, unit_gone=True)
    except DeviceError as e:
        log.warning("push failed: %s", text.clean(str(e), 200))
        if saved:
            return Outcome("succeeded", "", f"Save to {slot} issued after a clean read-back, but confirming it "
                           f"failed. Check {slot} on the unit.", firmware=fw, push=result_push)
        return failed("device_error", "A device operation failed. Nothing was saved; re-select the slot on the "
                      "unit to discard any partial grid.", firmware=fw, push=result_push)
