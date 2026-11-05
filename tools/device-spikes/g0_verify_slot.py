#!/usr/bin/env python3
"""Gate G0: did the hand-run "bb king test 2" save actually land in slot 2A?

READ-ONLY. Opens the unit, asks it for a fresh directory listing (``list_directory``, which
streams the whole library for ~15 s), and reports what is at the slot you name, its
neighbours, a search for the preset name anywhere in My Presets, and the first few empty
slots (so you can pick the G3 target). Nothing is recalled, edited or saved.

Pass criteria: the slot holds a preset with the expected name.
"""
from __future__ import annotations

import argparse
import sys

import spike_common as sc


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sc.add_common_args(ap)
    ap.add_argument("--slot", default="2A", help="slot label to check (default: %(default)s)")
    ap.add_argument("--position", type=int, default=None, help="0-based position, overrides --slot")
    ap.add_argument("--expect-name", default="bb king test 2", help="preset name expected there (case-insensitive)")
    ap.add_argument("--folder-key", default=sc.SETLIST, help="setlist folder key (default: My Presets)")
    ap.add_argument("--show-empty", type=int, default=10, help="how many empty slots to list")
    args = ap.parse_args()

    mcp = sc.load_qc_mcp(args.qc_mcp, allow_unpinned=args.allow_unpinned)
    sc.banner("G0: is the preset really in the slot?  (read-only)")
    qc = sc.open_device(mcp, sc.GRANT_READ)
    result = sc.base_result("G0", mcp, args)
    try:
        sc.check_firmware(qc, args.firmware, required=False)
        result["firmware"] = qc.firmware
        result["device_type"] = qc.device_type
        if args.position is None:
            sc.require_full_qc(qc)
        pos = args.position if args.position is not None else sc.parse_slot(args.slot)
        result["position"] = pos

        sc.say("Reading the full directory from the unit (about 15 seconds)...")
        cat = qc.list_directory()
        folder = sc.find_folder(cat, args.folder_key)
        result["folders_seen"] = [{"key": f["key"], "name": f["name"], "factory": f["is_factory"],
                                  "files": len(f["files"])} for f in cat.get("presets", [])]
        if folder is None:
            sc.say(f"Folder {args.folder_key!r} was not in the listing. Preset folders the unit reported:")
            for f in result["folders_seen"]:
                sc.say(f"  key={f['key']!r} name={f['name']!r} factory={f['factory']} files={f['files']}")
            result["verdict"] = "FOLDER_NOT_FOUND"
            sc.say("\nG0 INCONCLUSIVE: send me the folder list above.")
            return sc.EXIT_CHECK_FAILED

        entry = sc.slot_entry(folder, pos)
        name = (entry.get("name") or "").strip()
        sc.say(f"\nFolder {folder['name']!r} ({folder['key']}) has {len(folder['files'])} entries.")
        sc.say(f"Slot {sc.slot_label(pos)} (position {pos}): {name!r}"
               + ("" if name else "   <-- EMPTY"))
        result["slot"] = {"label": sc.slot_label(pos), "position": pos, "entry": entry}

        sc.say("\nNeighbours:")
        near = []
        for p in range(max(0, pos - 2), pos + 3):
            e = sc.slot_entry(folder, p)
            near.append({"position": p, "label": sc.slot_label(p), "name": e.get("name", "")})
            sc.say(f"  {'>>' if p == pos else '  '} {sc.slot_label(p):>4} (pos {p:3d}): {e.get('name', '')!r}")
        result["neighbours"] = near

        want = args.expect_name.strip().lower()
        hits = [{"position": f["index"], "label": sc.slot_label(f["index"]), "name": f["name"]}
                for f in folder["files"] if want and want in (f.get("name") or "").lower()]
        sc.say(f"\nPresets in this folder whose name contains {args.expect_name!r}: {len(hits)}")
        for h in hits:
            sc.say(f"  {h['label']:>4} (pos {h['position']:3d}): {h['name']!r}")
        result["name_hits"] = hits

        empties = [p for p in range(256) if sc.slot_is_empty(folder, p)]
        result["empty_count"] = len(empties)
        sc.say(f"\nEmpty slots in this folder: {len(empties)} of 256. First {args.show_empty}:")
        sc.say("  " + ", ".join(f"{sc.slot_label(p)} (pos {p})" for p in empties[:args.show_empty]))
        result["first_empty"] = [{"position": p, "label": sc.slot_label(p)} for p in empties[:args.show_empty]]

        try:
            cur = qc.get_setlist_position()
        except Exception as e:  # noqa: BLE001
            cur = {"error": f"{type(e).__name__}: {e}"}
        sc.say(f"\nPreset currently open on the unit (setlist position): {cur}")
        result["currently_open"] = cur

        ok = name.lower() == want
        result["verdict"] = "PASS" if ok else ("PRESENT_ELSEWHERE" if hits else "FAIL")
        sc.say("")
        if ok:
            sc.say(f"G0 PASS: slot {sc.slot_label(pos)} holds {name!r}.")
        elif hits:
            sc.say(f"G0 FAIL at {sc.slot_label(pos)}, but the name exists elsewhere (see above).")
        else:
            sc.say(f"G0 FAIL: slot {sc.slot_label(pos)} does not hold {args.expect_name!r}, and no preset with that name was found.")
        return sc.EXIT_OK if ok else sc.EXIT_CHECK_FAILED
    finally:
        result["close"] = sc.close_with_timeout(qc)
        path = sc.write_result(args.out, "g0", result)
        sc.say(f"\nFull result saved to: {path}")


if __name__ == "__main__":
    sys.exit(sc.run_spike(main))
