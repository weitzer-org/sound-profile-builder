#!/usr/bin/env python3
"""Gate G7, question (f) only (docs/design/qc-stomp-layout.md section 7).

  (f) Does a footswitch-assignment change sent over USB stay an UNSAVED EDIT that recalling another preset
      discards, or does it reach the stored preset? PASS if, after recalling another preset and then the
      original, the assignments equal the baseline. FAIL if the change survived. On FAIL entry point 2
      (apply_stomp_layout) stays off.

Nothing else in G7 is tested here: (c) does not matter because the planner removes before it assigns
(internal/stomplayout/plan.go, rule 6), and (a), (b), (d), (e), (g), (h) are covered by normal pushes
(deploy/qc-agent/README.md steps 16 and 11) or do not affect the design.

What it does to the unit (only with --apply; without it the script checks every precondition, prints the
plan and writes nothing):

  1. reads the stomp assignments of the open preset (baseline);
  2. moves ONE assigned block to a free switch among A-G (never H): unassign, then assign; reads back and
     confirms the change took (new switch present, old one gone);
  3. recalls the preset in --other-slot, then recalls the original slot, confirming each from the unit's own
     pointer, and reads back;
  4. prints the verdict and the raw read-backs and writes them to the results file.

Preconditions, all checked before anything is written:
  * the bridge (home agent) is not running and the unit opens exclusively;
  * the firmware equals --firmware;
  * the open preset is one this system created: its slot is in the agent's owned-slot ledger
    (owned_slots.jsonl, READ ONLY here) and the unit still shows the name the agent saved there;
  * the preset has no unsaved edits and at least one plain footswitch assignment that can move to a free A-G switch;
  * --other-slot is a different, occupied My Presets slot.

Grants: assign_stomp, unassign_stomp, recall (SetlistPosition UPDATE: changing preset) and reads. No save
(``save_preset`` = RecallPreset SAVE), no write_preset_file, no delete. Stomps and recall are on
spike_common.NEVER for every other spike; only the exact GRANT_G7 set may use them.

Nothing is ever saved. If the change survives (FAIL), the script offers to put the assignment back (a second
live edit) and prints the baseline.
"""
from __future__ import annotations

import argparse
import sys
import time
from dataclasses import dataclass
from pathlib import Path

import spike_common as sc

SWITCHES = "ABCDEFGH"
FREE_RANGE = range(0, 7)  # A-G; H (7) is never a target and a block on H is never a candidate
SETTLE_POLLS, SETTLE_S = 4, 1.0
RECALL_WAIT_S, RECALL_SETTLE_S = 15.0, 2.0
ENTRY_POINT_2_OFF = ("Entry point 2 (apply_stomp_layout / the apply_stomps job) stays OFF "
                     "(docs/design/qc-stomp-layout.md section 7).")


# --------------------------------------------------------------------------- the agent's ledger (read only)

def agent_home(override: str | None) -> Path:
    if override:
        return Path(override).expanduser()
    sys.path.insert(0, str(sc.REPO_ROOT / "deploy" / "qc-agent"))
    from qc_agent.config import default_home  # same path rule as the agent
    return default_home()


def check_agent_not_running(home: Path) -> None:
    """Probe the agent's single-instance lock without creating anything. The open of the unit is the real
    check (USB is exclusive); this just gives the clearer message when the agent is the reason."""
    lock = home / "agent.lock"
    if not lock.exists():
        return
    sys.path.insert(0, str(sc.REPO_ROOT / "deploy" / "qc-agent"))
    from qc_agent.lock import AlreadyRunning, InstanceLock
    try:
        InstanceLock(lock).acquire().release()
    except AlreadyRunning:
        raise sc.SpikeRefused("the QC bridge (home agent) is running; it and this spike cannot share the USB device. "
                              "Close the QC bridge window and retry.") from None
    except OSError:
        pass  # unreadable lock file: let the open decide


def read_owned_slots(home: Path):
    """The ledger, parsed read-only. Loading never writes (only ``record`` does, and is never called)."""
    sys.path.insert(0, str(sc.REPO_ROOT / "deploy" / "qc-agent"))
    from qc_agent.ledger import LedgerError, OwnedSlots
    path = home / "owned_slots.jsonl"
    try:
        return path, OwnedSlots(path)
    except LedgerError as e:
        raise sc.SpikeRefused(f"the owned-slot ledger is unreadable ({e}). It is never rewritten by this script.") from None


def check_open_preset_is_ours(position: int, name: str, home: Path) -> dict:
    """The open preset must be one this system created (ledger, read only). Returns what was checked."""
    path, owned = read_owned_slots(home)
    if not owned.owned(position, name):
        known = ", ".join(f"{e.get('slot')} ({e.get('name')!r})" for e in owned.current()) or "none"
        raise sc.SpikeRefused(
            f"the open preset {name!r} in {sc.slot_label(position)} is not in the home agent's owned-slot ledger "
            f"({path}; slots it knows: {known}). Open a preset the bridge pushed and retry.")
    return {"ledger": str(path), "slot": sc.slot_label(position)}


# --------------------------------------------------------------------------- stomp snapshots and verdicts (pure)

def snapshot(bp) -> dict:
    """The unit's footswitch state as plain data: assignments sorted, plus the momentary map."""
    assigns = sorted(({"row": int(a.row), "col": int(a.column), "index": int(a.stomp_index), "type": int(a.type)}
                      for a in bp.stomp_mode_assignments),
                     key=lambda a: (a["row"], a["col"], a["index"], a["type"]))
    return {"assignments": assigns,
            "momentary": {int(k): bool(v) for k, v in sorted(bp.stomp_is_momentary.items())}}


def key(snap: dict):
    return (tuple((a["row"], a["col"], a["index"], a["type"]) for a in snap["assignments"]),
            tuple(sorted(snap["momentary"].items())))


def show(snap: dict) -> str:
    if not snap["assignments"]:
        return "(no assignments)"
    parts = [f"block r{a['row']}c{a['col']} -> {sw(a['index'])}" + ("" if a["type"] == 0 else f" [type {a['type']}]")
             for a in snap["assignments"]]
    mom = [sw(i) for i, v in snap["momentary"].items() if v]
    return "; ".join(parts) + (f"   momentary: {', '.join(mom)}" if mom else "")


def sw(index: int) -> str:
    return SWITCHES[index] if 0 <= index < len(SWITCHES) else f"#{index}"


@dataclass(frozen=True)
class Move:
    row: int
    col: int
    old: int  # switch index it is on now (A-G)
    new: int  # free switch index (A-G)

    def describe(self) -> str:
        return f"block r{self.row}c{self.col}: {sw(self.old)} -> {sw(self.new)}"


def pick_move(snap: dict, placed) -> Move:
    """Choose the block to move and the switch to move it to, or refuse with the reason.

    ``placed`` is a set of (row, col) that hold a block in the preset. Candidate: a block with exactly one
    assignment, a latching primary one (type 0), on A-G. Target: the lowest switch in A-G that has no
    assignment at all. H is never a target and never a source.
    """
    by_block: dict[tuple[int, int], list[dict]] = {}
    for a in snap["assignments"]:
        by_block.setdefault((a["row"], a["col"]), []).append(a)
    if not by_block:
        raise sc.SpikeRefused("the preset has no footswitch assignments; open one pushed with stomps (README step 16).")
    stray = sorted(b for b in by_block if b not in placed)
    if stray:
        raise sc.SpikeRefused(f"assignment(s) point at cell(s) {stray} that hold no block in the read-back, so the row/column "
                              "convention of the read is not what the spike assumes. Nothing was written.")
    used = {a["index"] for a in snap["assignments"]}
    free = [i for i in FREE_RANGE if i not in used]
    if not free:
        raise sc.SpikeRefused("every switch A-G already has an assignment, so there is no free switch to move a block to.")
    for (r, c), lst in sorted(by_block.items()):
        a = lst[0]
        if len(lst) == 1 and a["type"] == 0 and a["index"] in FREE_RANGE and not snap["momentary"].get(a["index"]):
            return Move(r, c, a["index"], free[0])
    raise sc.SpikeRefused("no block qualifies: every assigned block is on H, has several switches, a secondary assignment "
                          f"or a momentary switch ({show(snap)}). Open a preset with a plain latching assignment on A-G.")


def change_took(snap: dict, mv: Move) -> bool:
    """The block is on the new switch and no longer on the old one, and nothing else changed shape."""
    mine = {a["index"] for a in snap["assignments"] if (a["row"], a["col"]) == (mv.row, mv.col)}
    return mine == {mv.new}


def verdict_f(baseline: dict, changed: dict, back: dict | None) -> str:
    """(f): PASS = back to the baseline; FAIL = the change survived; INCONCLUSIVE = anything else."""
    if back is None:
        return "INCONCLUSIVE"
    if key(back) == key(baseline):
        return "PASS"
    if key(back) == key(changed):
        return "FAIL"
    return "INCONCLUSIVE"


# --------------------------------------------------------------------------- device helpers

def placed_cells(bp) -> set:
    return {(r, c) for r, c, _m in sc.iter_blocks(bp)}


def read_snapshot(qc, *, until=None) -> tuple[object, dict]:
    """Read the open preset; with ``until(snapshot)`` poll up to SETTLE_POLLS times for it to become true."""
    bp = None
    for i in range(SETTLE_POLLS):
        bp = qc.get_current_preset()
        if bp is None:
            raise sc.SpikeRefused("the preset read timed out")
        snap = snapshot(bp)
        if until is None or until(snap) or i == SETTLE_POLLS - 1:
            return bp, snap
        time.sleep(SETTLE_S)
    return bp, snapshot(bp)


def preset_dirty(qc):
    try:
        d = qc.read_state("PresetDirty")
    except Exception as e:  # noqa: BLE001
        return f"unreadable: {type(e).__name__}"
    return bool(getattr(d, "is_dirty", False)) if d is not None else None


def open_unit(mcp, home: Path):
    check_agent_not_running(home)
    try:
        return sc.open_device(mcp, sc.GRANT_G7)
    except RuntimeError as e:
        raise sc.SpikeRefused(f"could not open the unit ({e}). If the QC bridge (the home agent) is running, "
                              "close the QC bridge window and retry; otherwise check the unit is on, connected and "
                              "that Cortex Control is closed.") from None


def ask(prompt: str) -> str:
    return input(prompt).strip().lower()


def recall_and_confirm(qc, folder_key: str, position: int, label: str) -> dict:
    """Recall a My Presets slot (SetlistPosition UPDATE) and wait for the unit's own pointer to say it is open."""
    qc.recall(folder_key=folder_key, position=position, is_factory=False)
    deadline = time.monotonic() + RECALL_WAIT_S
    cur = None
    while True:
        cur = qc.get_setlist_position() or {}
        if cur.get("position") == position and cur.get("folder_key") == folder_key and not cur.get("is_factory"):
            time.sleep(RECALL_SETTLE_S)  # let the preset finish loading before it is read
            return {"label": label, "confirmed": True, "pointer": cur}
        if time.monotonic() >= deadline:
            return {"label": label, "confirmed": False, "pointer": cur}
        time.sleep(0.5)


def restore(qc, mv: Move, baseline: dict) -> dict:
    """Put the block back (a live edit, no save): unassign the new switch, assign the old one."""
    qc.unassign_stomp(mv.row, mv.col, mv.new)
    qc.assign_stomp(mv.row, mv.col, mv.old)
    _bp, snap = read_snapshot(qc, until=lambda s: key(s) == key(baseline))
    return {"after": snap, "equals_baseline": key(snap) == key(baseline)}


# --------------------------------------------------------------------------- main

def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sc.add_common_args(ap, writes=True)
    ap.add_argument("--other-slot", required=True,
                    help="a DIFFERENT, occupied My Presets slot the script will recall and then leave (any preset; it is only loaded)")
    ap.add_argument("--folder-key", default=sc.SETLIST)
    ap.add_argument("--agent-home", help="the agent's data folder (default: %%LOCALAPPDATA%%\\qc-agent); holds owned_slots.jsonl")
    ap.add_argument("--apply", action="store_true", help="really move one assignment and recall. Without it: checks and plan only")
    args = ap.parse_args()

    home = agent_home(args.agent_home)
    mcp = sc.load_qc_mcp(args.qc_mcp, allow_unpinned=args.allow_unpinned)
    sc.banner("G7 (f): is a stomp change discarded by recalling another preset?  (live edits, never saves)")
    qc = open_unit(mcp, home)
    result = sc.base_result("G7", mcp, args)
    result["gate_questions"] = ["f"]
    try:
        sc.check_firmware(qc, args.firmware, required=True)
        result["firmware"] = qc.firmware
        sc.require_full_qc(qc)
        other = sc.parse_slot(args.other_slot)
        cur = qc.get_setlist_position() or {}
        if cur.get("folder_key") != args.folder_key or cur.get("is_factory") or cur.get("position") is None:
            raise sc.SpikeRefused(f"the unit has {cur} open; select a My Presets slot the bridge created.")
        pos = cur["position"]
        if other == pos:
            raise sc.SpikeRefused("--other-slot is the slot that is open now; name a different one.")
        folder = sc.find_folder(qc.list_directory(), args.folder_key)
        if folder is None or sc.slot_is_empty(folder, other):
            raise sc.SpikeRefused(f"{args.other_slot} is empty or not in the listing; name an occupied My Presets slot.")
        result["other_slot_name"] = sc.slot_entry(folder, other).get("name")
        bp0, baseline = read_snapshot(qc)
        name0 = bp0.name  # a string, so a later read can never alias it
        result["open_slot"], result["preset_name"] = sc.slot_label(pos), name0
        result["ownership"] = check_open_preset_is_ours(pos, name0, home)
        sc.say(f"Open preset: {name0!r} in {sc.slot_label(pos)}  (ledger: {result['ownership']['ledger']})")
        dirty0 = preset_dirty(qc)
        result["preset_dirty_before"] = dirty0
        sc.say(f"Unsaved edits before (PresetDirty): {dirty0}")
        if dirty0 is not False:
            raise sc.SpikeRefused("the open preset has unsaved edits or its state is unreadable, so the baseline is not what is "
                                  "stored. Re-select the slot on the unit without saving, then retry.")
        result["baseline"] = baseline
        sc.say(f"1. Baseline assignments: {show(baseline)}")
        mv = pick_move(baseline, placed_cells(bp0))
        result["planned_move"] = mv.describe()
        sc.say(f"   Plan: unassign then assign {mv.describe()}; recall {sc.slot_label(other)} ({result['other_slot_name']!r}), "
               f"then recall {sc.slot_label(pos)}.")
        if not args.apply:
            sc.say("\nPreflight only: every precondition holds and nothing was written. Re-run with --apply to do it.")
            result["verdict"] = "PREFLIGHT_OK"
            return sc.EXIT_OK

        # ---- the change
        sc.say(f"\n2. Moving {mv.describe()} (unassign, then assign) ...")
        qc.unassign_stomp(mv.row, mv.col, mv.old)
        qc.assign_stomp(mv.row, mv.col, mv.new)
        _bp1, changed = read_snapshot(qc, until=lambda s: change_took(s, mv))
        took = change_took(changed, mv)
        result["changed"], result["change_took"] = changed, took
        result["preset_dirty_after_change"] = preset_dirty(qc)
        sc.say(f"   read-back: {show(changed)}")
        sc.say(f"   change took: {took}   (device PresetDirty now: {result['preset_dirty_after_change']})")
        if not took:
            result["verdict"] = "NOT_RUN"
            sc.say("   The unit did not show the move, so there is no change to discard and (f) is not answered. "
                   + ENTRY_POINT_2_OFF)
            if key(changed) != key(baseline):
                sc.say(f"   The preset is not as it was. Baseline: {show(baseline)}")
                if ask("   Put the block back now (a live edit; still no save)? (y/n) > ") == "y":
                    result["restore"] = restore(qc, mv, baseline)
                    sc.say(f"   restore: equals baseline = {result['restore']['equals_baseline']}")
            return sc.EXIT_CHECK_FAILED

        # ---- (f)
        sc.say(f"\n3. Recalling {sc.slot_label(other)}, then {sc.slot_label(pos)} ...")
        r1 = recall_and_confirm(qc, args.folder_key, other, sc.slot_label(other))
        result["recall_other"] = r1
        sc.say(f"   other preset open: {r1['confirmed']}")
        back, r2 = None, None
        if r1["confirmed"]:
            r2 = recall_and_confirm(qc, args.folder_key, pos, sc.slot_label(pos))
            result["recall_original"] = r2
            sc.say(f"   original preset open again: {r2['confirmed']}")
            if r2["confirmed"]:
                bp2, back = read_snapshot(qc)
                result["name_after"] = bp2.name
                if bp2.name != name0:
                    sc.say(f"   Warning: the preset now open is named {bp2.name!r}, not {name0!r}.")
                    back = None
                result["back"] = back
                result["preset_dirty_after_return"] = preset_dirty(qc)
        f = verdict_f(baseline, changed, back)
        result["verdict"] = result["f"] = f

        # ---- summary
        sc.banner("G7 (f) summary")
        sc.say(f"Baseline            : {show(baseline)}")
        sc.say(f"After the change    : {show(changed)}")
        sc.say(f"After recall+return : {show(back) if back else '(not read)'}")
        sc.say(f"(f) a stomp change is discarded by recalling another preset: {f}")
        if f == "PASS":
            sc.say("    The assignments are back to the baseline: the change was an unsaved edit and nothing was auto-saved. "
                   "(f) is answered; entry point 2 may be built.")
        elif f == "FAIL":
            sc.say("    The change SURVIVED the recall: the unit kept or auto-saved it. " + ENTRY_POINT_2_OFF)
            sc.say(f"    Original assignments to restore: {show(baseline)}")
            if ask("    Put the block back now (a live edit; still no save)? (y/n) > ") == "y":
                result["restore"] = restore(qc, mv, baseline)
                sc.say(f"    restore: equals baseline = {result['restore']['equals_baseline']}; now {show(result['restore']['after'])}")
        else:
            sc.say(f"    (f) is NOT answered (a recall did not complete, a different preset came back, or the read-back matches "
                   f"neither state). {ENTRY_POINT_2_OFF}")
        sc.say("\nNothing was saved by this script.")
        return sc.EXIT_OK if f == "PASS" else sc.EXIT_CHECK_FAILED
    finally:
        result["close"] = sc.close_with_timeout(qc)
        path = sc.write_result(args.out, "g7", result)
        sc.say(f"\nFull result saved to: {path}")


if __name__ == "__main__":
    sys.exit(sc.run_spike(main))
