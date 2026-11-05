#!/usr/bin/env python3
"""Gate G1: which parameter-index scheme does the device use for the cloned models?

READ-ONLY. Reads the preset that is currently OPEN on the unit (you select it by hand on the
unit; this script never recalls anything) and dumps the raw per-block parameter arrays,
then lines each block up against two catalogs:

  X = qc-mcp's reading of ModelRepo.xml: a block's own <Parameter> list, ignoring ``clones=``.
  O = our internal/catalog/qc_catalog.json, which expands ``clones=`` (base params first,
      ``replaces=`` overriding in place, new ones appended). A cab goes from 2 to 19 params.

The decisive number per block is N, the length of the device's params array.
qc-mcp documents that a read returns N param slots and the bypass control sits at index N,
never in the read (transport.py bypass_param_index). So:

  N == len(O)            -> our clone expansion matches the device's count.
  N == len(X) != len(O)  -> the device uses the unexpanded list; our catalog's indexes are wrong.
  neither                -> something else (hidden params, a different layout); read the table.

Count alone does not prove the NAMES line up, so a per-position table is printed too. The values
are the device's raw normalised floats; compare them with what the unit's screen shows for that
block. The G4 spike checks display values properly.

The raw BinaryPreset bytes are saved next to the JSON so the analysis can be redone offline.
"""
from __future__ import annotations

import argparse
import sys

import spike_common as sc


def _fmt(v) -> str:
    if v is None:
        return "-"
    if isinstance(v, float):
        return f"{v:.4f}"
    return str(v)


def block_report(ridx, cidx, m, xml_models, our_models, qc_cat) -> dict:
    h = m.hash
    x = xml_models.get(h)
    o = our_models.get(h)
    info = qc_cat.lookup(h) or {}
    dev = [{
        "pos": pos, "index_field": p.index, "scene_mode": p.scene_mode, "expression": p.expression,
        "values": sc.param_values_all(p),
    } for pos, p in enumerate(m.params)]
    n = len(dev)

    x_params = x["params"] if x else []
    o_by_idx = {p["index"]: p for p in (o["params"] if o else [])}
    o_len = (max(o_by_idx) + 1) if o_by_idx else 0
    rows = []
    for pos in range(max(n, len(x_params), o_len)):
        xp = x_params[pos] if pos < len(x_params) else None
        op = o_by_idx.get(pos)
        d = dev[pos] if pos < n else None
        rows.append({
            "pos": pos,
            "device": d,
            "x": ({k: xp[k] for k in ("name", "min", "max", "default", "skew", "units", "replaces")} if xp else None),
            "o": ({k: op.get(k) for k in ("name", "min", "max", "default", "min_symbol", "max_symbol", "units")} if op else
                  ({"name": "(hidden or absent in our catalog)"} if (pos < o_len) else None)),
            "names_agree": bool(xp and op and (xp.get("name") or "").lower() == (op.get("name") or "").lower()),
        })
    verdict = ("O_COUNT_MATCHES" if n == o_len and n != len(x_params) else
               "X_COUNT_MATCHES" if n == len(x_params) and n != o_len else
               "BOTH_COUNT_MATCH" if n == o_len == len(x_params) else
               "NEITHER_COUNT_MATCHES")
    return {
        "grid": {"row_idx": ridx, "array_idx": cidx, "column_field": m.column},
        "hash": h, "name": (o or {}).get("name") or info.get("name"), "category": (x or {}).get("category") or info.get("category"),
        "xml_clones": (x or {}).get("clones"), "n_device_params": n, "n_x": len(x_params), "n_o": o_len,
        "n_o_listed": len(o_by_idx), "verdict": verdict, "rows": rows,
    }


def print_block(b: dict, verbose: bool) -> None:
    g = b["grid"]
    sc.say("")
    sc.say(f"--- chain {g['row_idx']} / array slot {g['array_idx']}: {b['name']!r} (hash {b['hash']}, {b['category']})"
           + (f", clones={b['xml_clones']}" if b["xml_clones"] else ""))
    sc.say(f"    N (device params) = {b['n_device_params']}   X (xml own) = {b['n_x']}   O (ours) = {b['n_o']}   -> {b['verdict']}")
    if not verbose and b["verdict"] == "BOTH_COUNT_MATCH" and all(r["names_agree"] or r["x"] is None for r in b["rows"]):
        sc.say("    (counts and names agree under both schemes; per-position table omitted, use --all)")
        return
    sc.say(f"    {'pos':>3}  {'device value(s)':<26} {'X name':<22} {'O name':<22} {'O range':<18} agree")
    for r in b["rows"]:
        d, x, o = r["device"], r["x"], r["o"]
        dv = ", ".join(_fmt(v) for v in d["values"]) if d else "(none)"
        if d and d["scene_mode"]:
            dv += " [scene]"
        orng = ""
        if o and o.get("name") and "min" in o:
            lo = o.get("min") if o.get("min") is not None else o.get("min_symbol")
            hi = o.get("max") if o.get("max") is not None else o.get("max_symbol")
            orng = f"{lo}..{hi}"
        sc.say(f"    {r['pos']:>3}  {dv:<26} {(x or {}).get('name') or '':<22} {(o or {}).get('name') or '':<22} {orng:<18} "
               f"{'yes' if r['names_agree'] else ('-' if not (x and o) else 'NO')}")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sc.add_common_args(ap)
    ap.add_argument("--slot", default="2A", help="slot that must be open on the unit (default: %(default)s)")
    ap.add_argument("--position", type=int, default=None, help="0-based position, overrides --slot")
    ap.add_argument("--expect-name", default="bb king test 2", help="the open preset's name must match (case-insensitive)")
    ap.add_argument("--folder-key", default=sc.SETLIST)
    ap.add_argument("--all", action="store_true", help="print the per-position table for every block")
    ap.add_argument("--allow-dirty", action="store_true", help="read even if the open preset has unsaved edits")
    args = ap.parse_args()

    mcp = sc.load_qc_mcp(args.qc_mcp, allow_unpinned=args.allow_unpinned)
    xml_models = sc.load_xml_models(mcp.xml_path)
    our_models = sc.load_our_catalog()
    sc.banner("G1: raw per-block parameter arrays vs the two index schemes  (read-only)")
    qc = sc.open_device(mcp, sc.GRANT_READ)
    result = sc.base_result("G1", mcp, args)
    try:
        sc.check_firmware(qc, args.firmware, required=False)
        result["firmware"] = qc.firmware
        if args.position is None:
            sc.require_full_qc(qc)
        pos = args.position if args.position is not None else sc.parse_slot(args.slot)

        cur = qc.get_setlist_position() or {}
        result["setlist_position"] = cur
        sc.say(f"Open on the unit: {cur}")
        if cur.get("folder_key") != args.folder_key or cur.get("position") != pos or cur.get("is_factory"):
            raise sc.SpikeRefused(
                f"the unit does not have slot {sc.slot_label(pos)} open (it reports {cur}). "
                f"On the unit, select slot {sc.slot_label(pos)} in My Presets, wait for it to load, then run this again. "
                "This script never recalls a preset itself.")

        try:
            dirty = qc.read_state("PresetDirty")
            result["preset_dirty"] = bool(getattr(dirty, "is_dirty", False)) if dirty is not None else None
        except Exception as e:  # noqa: BLE001
            result["preset_dirty"] = f"unreadable: {type(e).__name__}"
        sc.say(f"Unsaved edits on the open preset (PresetDirty): {result['preset_dirty']}")
        if result["preset_dirty"] is True and not args.allow_dirty:
            raise sc.SpikeRefused("the open preset has unsaved edits, so the grid is not what is stored in the slot. "
                                  "Re-select the slot on the unit without saving (or pass --allow-dirty).")

        bp = qc.get_current_preset()
        if bp is None:
            raise sc.SpikeRefused("the unit did not return the preset (read timed out). Try again.")
        result["preset_name"] = bp.name
        sc.say(f"Preset name from the unit: {bp.name!r}")
        if args.expect_name and bp.name.strip().lower() != args.expect_name.strip().lower():
            raise sc.SpikeRefused(f"open preset is {bp.name!r}, expected {args.expect_name!r}.")

        raw = bp.SerializeToString()
        result["binary_preset_hex"] = raw.hex()
        result["binary_preset_bytes"] = len(raw)
        result["chains_in_read"] = [{"chain_idx": i, "row_field": ch.row, "in_portid": ch.in_portid,
                                    "out_portid": ch.out_portid, "models_array_len": len(ch.models),
                                    "placed": [{"array_idx": ci, "hash": m.hash} for ci, m in enumerate(ch.models) if m.hash]}
                                   for i, ch in enumerate(bp.chains)]

        blocks = [block_report(r, c, m, xml_models, our_models, mcp.catalog) for r, c, m in sc.iter_blocks(bp)]
        result["blocks"] = blocks
        sc.say(f"\n{len(blocks)} block(s) placed. Per-block comparison:")
        for b in blocks:
            print_block(b, args.all or b["verdict"] != "BOTH_COUNT_MATCH")

        sc.banner("G1 summary")
        sc.say(f"{'block':<32} {'hash':>6} {'N':>3} {'X':>3} {'O':>3}  verdict")
        for b in blocks:
            sc.say(f"{(b['name'] or '?')[:32]:<32} {b['hash']:>6} {b['n_device_params']:>3} {b['n_x']:>3} {b['n_o']:>3}  {b['verdict']}")
        clones = [b for b in blocks if b["xml_clones"]]
        result["clone_blocks"] = [b["name"] for b in clones]
        if not clones:
            sc.say("\nNo placed block uses clones=, so this preset cannot decide G1. Pick a preset with a cab.")
            result["verdict"] = "INCONCLUSIVE_NO_CLONES"
            return sc.EXIT_CHECK_FAILED
        o_ok = all(b["n_device_params"] == b["n_o"] for b in clones)
        x_ok = all(b["n_device_params"] == b["n_x"] for b in clones)
        result["verdict"] = ("COUNTS_SUPPORT_OUR_EXPANSION" if o_ok and not x_ok else
                             "COUNTS_SUPPORT_UNEXPANDED" if x_ok and not o_ok else
                             "COUNTS_DO_NOT_DECIDE")
        sc.say(f"\nCloned blocks: {[b['name'] for b in clones]}")
        sc.say(f"Count-level result: {result['verdict']}")
        sc.say("(Counts are necessary, not sufficient: the name table above must also agree with the unit's screen.)")
        return sc.EXIT_OK
    finally:
        result["close"] = sc.close_with_timeout(qc)
        path = sc.write_result(args.out, "g1", result)
        sc.say(f"\nFull result (including raw preset bytes as hex) saved to: {path}")


if __name__ == "__main__":
    sys.exit(sc.run_spike(main))
