#!/usr/bin/env python3
"""Gate G3: do qc-mcp's transport writes behave on this firmware when driven with WIRE values?

THE ONLY SPIKE THAT SAVES, and only into ONE empty slot that YOU name. Three escalating modes:

  (default)   Preflight only. Opens the unit read-only, checks every precondition, prints the plan.
              Writes nothing.
  --apply     Also builds the test preset on the unit's LIVE edit buffer (set_routing, add_block,
              set_param with explicit 0..1 wire values), then reads it back and diffs. The slot is
              still empty; selecting another preset on the unit discards the edits.
  --save      (needs --apply) After a clean read-back diff and a typed confirmation, writes the
              preset into the slot with File CREATE (``write_preset_file``) and re-lists the
              directory to confirm it landed.

Preconditions, all enforced (any failure refuses and changes nothing):
  * the unit reports exactly --firmware (copy it from G0);
  * the slot is empty in a FRESH directory listing, immediately before the first write and again
    immediately before the save (qc-mcp's own guard reads a stale cache, audit 1.5);
  * that slot is the one OPEN on the unit and its grid is blank (you select it on the unit by hand);
  * every model id in the plan matches qc-mcp's ModelRepo.xml and is not hidden/internal/NC_Recorder.

Never used: clear_grid, recall, save_preset (RecallPreset SAVE), build_preset, any global setting,
delete, or the raw transport. See spike_common.NEVER; the guard enforces it.
"""
from __future__ import annotations

import argparse
import re
import sys
import time

import spike_common as sc
import test_preset as tp

TOL = 1e-4
PACE = 0.15


def preflight(qc, args, mcp, xml_models, result) -> tuple[int, dict]:
    sc.check_firmware(qc, args.firmware, required=True)
    sc.require_full_qc(qc)
    problems = tp.validate_against_xml(xml_models)
    if problems:
        raise sc.SpikeRefused("test preset does not match ModelRepo.xml: " + "; ".join(problems))
    pos = sc.parse_slot(args.slot)
    result["position"] = pos
    sc.say(f"Target: slot {sc.slot_label(pos)} (position {pos}) in {args.folder_key}")

    sc.say("Fresh directory listing (about 15 seconds)...")
    folder = sc.find_folder(qc.list_directory(), args.folder_key)
    if folder is None:
        raise sc.SpikeRefused(f"folder {args.folder_key!r} not found in the unit's listing.")
    if not sc.slot_is_empty(folder, pos):
        raise sc.SpikeRefused(f"slot {sc.slot_label(pos)} is NOT empty (it holds "
                              f"{sc.slot_entry(folder, pos).get('name')!r}). Pick an empty slot; g0_verify_slot.py lists some.")
    sc.say("  slot is empty: OK")

    cur = qc.get_setlist_position() or {}
    result["setlist_position"] = cur
    if cur.get("folder_key") != args.folder_key or cur.get("position") != pos or cur.get("is_factory"):
        raise sc.SpikeRefused(f"the unit has {cur} open, not slot {sc.slot_label(pos)}. On the unit, select "
                              f"{sc.slot_label(pos)} (the blank slot), then run this again. This script never recalls.")
    sc.say("  slot is the one open on the unit: OK")

    bp = qc.get_current_preset()
    if bp is None:
        raise sc.SpikeRefused("could not read the open preset (timed out); try again.")
    placed = [(r, c, m.hash) for r, c, m in sc.iter_blocks(bp)]
    if placed:
        raise sc.SpikeRefused(f"the open grid is not blank (blocks: {placed}). Re-select the empty slot on the unit.")
    dirty = qc.read_state("PresetDirty")
    result["preset_dirty_before"] = bool(getattr(dirty, "is_dirty", False)) if dirty is not None else None
    sc.say(f"  open grid is blank: OK   (name={bp.name!r}, PresetDirty={result['preset_dirty_before']})")
    result["blank_preset_name"] = bp.name
    result["chains_in_blank_read"] = len(bp.chains)
    return pos, {"folder": folder}


def print_plan() -> None:
    sc.say("\nPlan (row 0, in-port 1, out-port 19). Values are WIRE values written as-is:")
    for b in tp.BLOCKS:
        sc.say(f"  col {b.col}: model {b.model_id} {b.xml_name!r}  writes {b.writes}")


def apply_preset(qc, result) -> bool:
    log = []
    base_errors = qc.write_errors
    sc.say("\nBuilding on the live edit buffer...")
    qc.set_routing(tp.ROW, in_portid=tp.IN_PORT, out_portid=tp.OUT_PORT)
    time.sleep(PACE)
    for b in tp.BLOCKS:
        echo = qc.add_block(b.model_id, row=tp.ROW, column=b.col)
        log.append({"op": "add_block", "col": b.col, "model": b.model_id, "echo": bool(echo)})
        sc.say(f"  add_block col {b.col} model {b.model_id}: echo={'yes' if echo else 'NO'}")
        time.sleep(PACE)
        for idx, wire in b.writes.items():
            qc.set_param(tp.ROW, b.col, idx, wire)
            log.append({"op": "set_param", "col": b.col, "index": idx, "wire": wire})
            time.sleep(PACE)
    time.sleep(0.5)
    result["ops"] = log
    result["write_errors_during_apply"] = qc.write_errors - base_errors
    result["last_write_error"] = qc.last_write_error
    return True


def read_back(qc, result) -> bool:
    bp = qc.get_current_preset()
    if bp is None:
        sc.say("read-back timed out")
        result["readback_error"] = "timeout"
        return False
    ok = True
    report = {"routing": None, "blocks": []}
    ch = bp.chains[tp.ROW] if len(bp.chains) > tp.ROW else None
    if ch is None:
        sc.say("no chain 0 in read-back")
        return False
    report["routing"] = {"in_portid": ch.in_portid, "out_portid": ch.out_portid}
    if (ch.in_portid, ch.out_portid) != (tp.IN_PORT, tp.OUT_PORT):
        sc.say(f"  routing MISMATCH: got in={ch.in_portid} out={ch.out_portid}, wanted in={tp.IN_PORT} out={tp.OUT_PORT}")
        ok = False
    else:
        sc.say(f"  routing in={ch.in_portid} out={ch.out_portid}: OK")
    for b in tp.BLOCKS:
        entry = {"col": b.col, "model_id": b.model_id, "placed": False, "n_params": None, "params": []}
        if b.col >= len(ch.models) or ch.models[b.col].hash != b.model_id:
            got = ch.models[b.col].hash if b.col < len(ch.models) else None
            sc.say(f"  col {b.col}: expected model {b.model_id}, found {got}  MISMATCH")
            ok = False
            report["blocks"].append(entry)
            continue
        m = ch.models[b.col]
        entry["placed"] = True
        entry["n_params"] = len(m.params)
        for idx, wire in b.writes.items():
            got = sc.param_value(m.params[idx]) if idx < len(m.params) else None
            hit = isinstance(got, float) and abs(got - wire) <= TOL
            entry["params"].append({"index": idx, "wrote": wire, "read": got, "ok": hit,
                                    "index_field": m.params[idx].index if idx < len(m.params) else None})
            if not hit:
                ok = False
        sc.say(f"  col {b.col} model {b.model_id}: N={len(m.params)} params; "
               + ", ".join(f"[{p['index']}] wrote {p['wrote']} read {sc.fmt_num(p['read'])}"
                           f"{'' if p['ok'] else ' <-- DIFF'}" for p in entry["params"]))
        report["blocks"].append(entry)
    extra = [(r, c, m.hash) for r, c, m in sc.iter_blocks(bp) if (r, c) not in {(tp.ROW, b.col) for b in tp.BLOCKS}]
    if extra:
        sc.say(f"  unexpected extra blocks: {extra}")
        ok = False
    report["extra_blocks"] = extra
    result["readback"] = report
    return ok


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sc.add_common_args(ap, writes=True)
    ap.add_argument("--slot", required=True, help="the EMPTY slot you name, e.g. 24H. No default on purpose.")
    ap.add_argument("--folder-key", default=sc.SETLIST)
    ap.add_argument("--name", default=tp.TEST_PRESET_NAME, help="preset name to save (default: %(default)s)")
    ap.add_argument("--apply", action="store_true", help="build on the live edit buffer and read back (no save)")
    ap.add_argument("--save", action="store_true", help="after a clean read-back, save into the slot (needs --apply)")
    args = ap.parse_args()
    if args.save and not args.apply:
        ap.error("--save needs --apply")
    if not re.fullmatch(r"[A-Za-z0-9 _.-]{1,24}", args.name):
        ap.error("--name: 1-24 chars from letters, digits, space, _ . -")

    mcp = sc.load_qc_mcp(args.qc_mcp, allow_unpinned=args.allow_unpinned)
    xml_models = sc.load_xml_models(mcp.xml_path)
    mode = "SAVE" if args.save else "APPLY (live edit buffer only)" if args.apply else "PREFLIGHT (writes nothing)"
    sc.banner(f"G3: wire-value writes into an empty slot  [{mode}]")
    grant = sc.GRANT_G3 if args.apply else sc.GRANT_READ
    qc = sc.open_device(mcp, grant)
    result = sc.base_result("G3", mcp, args)
    result["mode"] = mode
    try:
        result["firmware"] = qc.firmware
        pos, _ = preflight(qc, args, mcp, xml_models, result)
        print_plan()
        if not args.apply:
            result["verdict"] = "PREFLIGHT_OK"
            sc.say("\nPreflight passed. Nothing was written. Re-run with --apply to build it on the live buffer.")
            return sc.EXIT_OK

        apply_preset(qc, result)
        sc.say("\nRead-back and diff:")
        ok = read_back(qc, result)
        result["write_errors_during_apply"] = result.get("write_errors_during_apply")
        if result["write_errors_during_apply"]:
            sc.say(f"  note: the transport counted {result['write_errors_during_apply']} non-benign write codes "
                   f"(last={result['last_write_error']}); audit 1.5 says this can be benign on Windows, but record it.")
        result["readback_ok"] = ok
        if not ok:
            result["verdict"] = "FAIL_READBACK"
            sc.say("\nG3 FAIL: read-back does not match what was written. NOT saving. Select another preset on the unit "
                   "to discard the edits, and send me this output.")
            return sc.EXIT_CHECK_FAILED
        sc.say("\nRead-back matches. Look at the unit now: you should see Myth Drive > Brit 2203 > cab > Low-High Cut > Spring.")

        if not args.save:
            result["verdict"] = "APPLY_OK_NOT_SAVED"
            sc.say("Not saved (no --save). To discard, select any other preset on the unit.")
            return sc.EXIT_OK

        typed = input(f"\nType the slot label ({sc.slot_label(pos)}) to SAVE as {args.name!r}, anything else to cancel: ").strip().upper()
        if typed != sc.slot_label(pos):
            result["verdict"] = "CANCELLED_AT_CONFIRM"
            sc.say("Cancelled. Nothing saved.")
            return sc.EXIT_CHECK_FAILED
        sc.say("Re-checking the slot is still empty, using a fresh directory listing...")
        folder = sc.find_folder(qc.list_directory(), args.folder_key)
        if folder is None or not sc.slot_is_empty(folder, pos):
            raise sc.SpikeRefused("the slot is no longer empty (or the folder vanished). Not saving.")
        cur = qc.get_setlist_position() or {}
        if cur.get("position") != pos or cur.get("folder_key") != args.folder_key:
            raise sc.SpikeRefused(f"open slot changed to {cur}. Not saving.")
        qc.write_preset_file(args.folder_key, pos, args.name)
        result["save_issued"] = True
        sc.say("Save issued (File CREATE). Waiting, then re-listing to confirm...")
        time.sleep(3.0)
        folder2 = sc.find_folder(qc.list_directory(), args.folder_key)
        landed = folder2 is not None and (sc.slot_entry(folder2, pos).get("name") or "") == args.name
        result["save_landed"] = landed
        result["slot_after"] = sc.slot_entry(folder2, pos) if folder2 else None
        result["verdict"] = "SAVED_AND_LISTED" if landed else "SAVE_NOT_SEEN_IN_LISTING"
        sc.say(f"Slot {sc.slot_label(pos)} now lists as: {result['slot_after']}")
        sc.say("\nG3 " + ("PASS (pending your power-cycle check)." if landed else "INCONCLUSIVE: the listing does not show the save."))
        sc.say("NEXT: power-cycle the unit, select the slot, and confirm the five blocks and their values are there.")
        return sc.EXIT_OK if landed else sc.EXIT_CHECK_FAILED
    finally:
        result["close"] = sc.close_with_timeout(qc)
        path = sc.write_result(args.out, "g3", result)
        sc.say(f"\nFull result saved to: {path}")


if __name__ == "__main__":
    sys.exit(sc.run_spike(main))
