#!/usr/bin/env python3
"""Gate G7, questions (f) and (c) only (docs/design/qc-stomp-layout.md section 7).

  (c) Does assigning an already-assigned block to a second footswitch MOVE it (old switch cleared; qc-mcp's
      claim, server.py:2048) or ADD a second switch?
  (f) Is a stomp-assignment change a live, unsaved edit that leaving the preset DISCARDS? PASS if, after the
      owner goes to another preset and back, the assignments equal the baseline. FAIL if the change survived
      (the unit auto-saved it, or kept it). Entry point 2 (apply_stomp_layout) stays off unless (f) PASSES.

The other G7 questions ((a), (b), (d), (e), (g), (h)) are answered by eye during the owner's normal pushes
(deploy/qc-agent/README.md steps 16 and 11) and are not repeated here.

What it does to the unit, in order (only with --apply; without it the script checks every precondition,
prints the plan and writes nothing):

  1. reads the stomp assignments of the open preset (baseline);
  2. (c) re-assigns ONE already-assigned block to a different free switch (A-G, never H) and reads back;
  3. asks you whether the unit shows the preset as edited/unsaved (it also reads the device's PresetDirty flag);
  4. (f) asks YOU to select another preset (--other-slot) on the unit, then to select the original slot again,
     and reads back;
  5. prints the verdicts with the raw read-backs and writes them to the results file.

Preconditions, all checked before anything is written:
  * the home agent (QC bridge window) is not running and the unit opens exclusively;
  * the firmware equals --firmware;
  * the open preset is one this system created: its slot is in the agent's owned-slot ledger
    (owned_slots.jsonl, READ ONLY here) and the unit still shows the name the agent saved there. Or, with an
    explicit --slot plus --i-confirm-bridge-created, the open slot must equal --slot;
  * the preset has no unsaved edits, and at least two blocks with footswitch assignments, one of which can
    be moved to a free switch in A-G.

Grants: assign_stomp, unassign_stomp and reads (get_current_preset, get_setlist_position, read_state,
list_directory). No save, no write_preset_file, no delete, no recall: YOU switch presets on the unit. Stomps
are on spike_common.NEVER for every other spike; only the exact GRANT_G7 set may use them.

Nothing is ever saved. If the change survives (f FAIL), the script offers to put the assignments back by hand
(a second live edit) and tells you what the baseline was.
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
NAV_ATTEMPTS, NAV_POLL_S, NAV_WAIT_S = 3, 1.0, 10.0
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


def check_open_preset_is_ours(args, position: int, name: str, home: Path) -> dict:
    """The open preset must be one this system created. Returns what was checked, for the results file."""
    if args.i_confirm_bridge_created:
        if parse_or_none(args.slot) != position:
            raise sc.SpikeRefused(f"the unit has {sc.slot_label(position)} open, not --slot {args.slot}. "
                                  "Select that slot on the unit first.")
        return {"ledger": "skipped (--slot + --i-confirm-bridge-created)", "slot": sc.slot_label(position)}
    path, owned = read_owned_slots(home)
    if args.slot and parse_or_none(args.slot) != position:
        raise sc.SpikeRefused(f"the unit has {sc.slot_label(position)} open, not --slot {args.slot}.")
    if not owned.owned(position, name):
        known = ", ".join(f"{e.get('slot')} ({e.get('name')!r})" for e in owned.current()) or "none"
        raise sc.SpikeRefused(
            f"the open preset {name!r} in {sc.slot_label(position)} is not in the home agent's owned-slot ledger "
            f"({path}; slots it knows: {known}). Open a preset the bridge pushed. If the bridge did push this one "
            f"from another PC, pass --slot {sc.slot_label(position)} --i-confirm-bridge-created.")
    return {"ledger": str(path), "slot": sc.slot_label(position)}


def parse_or_none(label):
    return sc.parse_slot(label) if label else None


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
    if len(by_block) < 2:
        raise sc.SpikeRefused(f"the preset has {len(by_block)} block(s) with footswitch assignments; this spike needs at "
                              f"least two ({show(snap)}).")
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


def verdict_c(before: dict, after: dict, mv: Move) -> dict:
    """(c): moved / added / not_applied / cleared, and whether anything else changed."""
    def of_block(s):
        return {a["index"] for a in s["assignments"] if (a["row"], a["col"]) == (mv.row, mv.col)}

    def others(s):
        return [a for a in s["assignments"] if (a["row"], a["col"]) != (mv.row, mv.col)]
    b, a = of_block(before), of_block(after)
    has_new, has_old = mv.new in a, mv.old in a
    word = ("moved" if has_new and not has_old else "added" if has_new and has_old
            else "not_applied" if has_old else "cleared")
    return {"verdict": word, "block_switches_before": sorted(b), "block_switches_after": sorted(a),
            "other_blocks_unchanged": others(before) == others(after),
            "momentary_unchanged": before["momentary"] == after["momentary"]}


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


def wait_for_slot(qc, position: int, label: str) -> bool:
    """The owner selects ``label`` on the unit by hand; confirm from the unit's own pointer."""
    for attempt in range(NAV_ATTEMPTS):
        input(f"  Select {label} on the unit, then press Enter > ")
        deadline = time.monotonic() + NAV_WAIT_S
        cur = None
        while True:
            cur = qc.get_setlist_position() or {}
            if cur.get("position") == position and not cur.get("is_factory"):
                return True
            if time.monotonic() >= deadline:
                break
            time.sleep(NAV_POLL_S)
        sc.say(f"  The unit has {cur} open, not {label}. ({attempt + 1}/{NAV_ATTEMPTS})")
    return False


def restore(qc, mv: Move, baseline: dict) -> dict:
    """Put the block back (a live edit, no save): assign it to the old switch, remove the new one if it stayed."""
    out = {"steps": []}
    qc.assign_stomp(mv.row, mv.col, mv.old)
    out["steps"].append(f"assign {mv.describe()} reversed")
    _bp, snap = read_snapshot(qc, until=lambda s: key(s) == key(baseline))
    if key(snap) != key(baseline) and any((a["row"], a["col"], a["index"]) == (mv.row, mv.col, mv.new)
                                          for a in snap["assignments"]):
        qc.unassign_stomp(mv.row, mv.col, mv.new)
        out["steps"].append(f"unassign {sw(mv.new)}")
        _bp, snap = read_snapshot(qc, until=lambda s: key(s) == key(baseline))
    out["after"] = snap
    out["equals_baseline"] = key(snap) == key(baseline)
    return out


# --------------------------------------------------------------------------- main

def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sc.add_common_args(ap, writes=True)
    ap.add_argument("--other-slot", required=True,
                    help="a DIFFERENT preset (any, you only visit it) you will select on the unit by hand for (f)")
    ap.add_argument("--slot", help="the slot that must be open; required with --i-confirm-bridge-created")
    ap.add_argument("--i-confirm-bridge-created", action="store_true",
                    help="skip the ledger: you say the bridge created the open preset (needs --slot, which must be the open slot)")
    ap.add_argument("--folder-key", default=sc.SETLIST)
    ap.add_argument("--agent-home", help="the agent's data folder (default: %%LOCALAPPDATA%%\\qc-agent); holds owned_slots.jsonl")
    ap.add_argument("--apply", action="store_true", help="really write the one assignment. Without it: checks and plan only")
    args = ap.parse_args()
    if args.i_confirm_bridge_created and not args.slot:
        raise sc.SpikeRefused("--i-confirm-bridge-created needs --slot (the slot you say the bridge created).")

    home = agent_home(args.agent_home)
    mcp = sc.load_qc_mcp(args.qc_mcp, allow_unpinned=args.allow_unpinned)
    sc.banner("G7 (c) and (f): footswitch assignments  (live edits only, never saves)")
    qc = open_unit(mcp, home)
    result = sc.base_result("G7", mcp, args)
    result["gate_questions"] = ["c", "f"]
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
        bp0, baseline = read_snapshot(qc)
        result["open_slot"] = sc.slot_label(pos)
        name0 = bp0.name  # a string, so a later read can never alias it
        result["preset_name"] = name0
        result["ownership"] = check_open_preset_is_ours(args, pos, name0, home)
        sc.say(f"Open preset: {name0!r} in {sc.slot_label(pos)}  (ownership: {result['ownership']['ledger']})")
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
        sc.say(f"   Plan for (c): {mv.describe()}   (then you visit {sc.slot_label(other)} and come back for (f))")
        if not args.apply:
            sc.say("\nPreflight only: every precondition holds and nothing was written. Re-run with --apply to do it.")
            result["verdict"] = "PREFLIGHT_OK"
            return sc.EXIT_OK

        # ---- (c)
        sc.say(f"\n2. (c) Assigning {mv.describe()} ...")
        qc.assign_stomp(mv.row, mv.col, mv.new)
        bp1, changed = read_snapshot(qc, until=lambda s: any((a["row"], a["col"], a["index"]) == (mv.row, mv.col, mv.new)
                                                              for a in s["assignments"]))
        c = verdict_c(baseline, changed, mv)
        result["changed"], result["c"] = changed, c
        sc.say(f"   read-back: {show(changed)}")
        sc.say(f"   (c) verdict: {c['verdict'].upper()}  (switches on that block: {[sw(i) for i in c['block_switches_before']]}"
               f" -> {[sw(i) for i in c['block_switches_after']]}; other blocks unchanged: {c['other_blocks_unchanged']})")
        if c["verdict"] in ("not_applied", "cleared"):
            result["f"] = {"verdict": "NOT_RUN", "reason": f"(c) was {c['verdict']}, so there was no change to discard"}
            sc.say(f"   The unit did not take the assignment ({c['verdict']}); (f) has nothing to test. {ENTRY_POINT_2_OFF}")
            return sc.EXIT_CHECK_FAILED

        # ---- screen question
        d1 = preset_dirty(qc)
        result["preset_dirty_after_change"] = d1
        sc.say(f"\n3. Device PresetDirty after the change: {d1}")
        result["screen_shows_edited"] = ask("   Look at the unit: does the preset show as edited/unsaved? (y/n) > ")

        # ---- (f)
        sc.say(f"\n4. (f) Now leave the preset BY HAND, without saving. If the unit asks whether to save, answer NO / discard.")
        result["f_navigation"] = {"other_reached": wait_for_slot(qc, other, sc.slot_label(other))}
        back = None
        if result["f_navigation"]["other_reached"]:
            result["f_navigation"]["unit_asked_to_save"] = ask("   Did the unit ask whether to save when you left? (y/n) > ")
            result["f_navigation"]["original_reached"] = wait_for_slot(qc, pos, sc.slot_label(pos))
            if result["f_navigation"]["original_reached"]:
                bp2, back = read_snapshot(qc)
                result["f_navigation"]["name_after"] = bp2.name
                if bp2.name != name0:
                    sc.say(f"   Warning: the preset now open is named {bp2.name!r}, not {name0!r}.")
                    back = None
                result["back"] = back
                result["preset_dirty_after_return"] = preset_dirty(qc)
        f = verdict_f(baseline, changed, back)
        result["f"] = {"verdict": f}

        # ---- summary
        sc.banner("G7 summary")
        sc.say(f"Baseline            : {show(baseline)}")
        sc.say(f"After the change    : {show(changed)}")
        sc.say(f"After leaving+return: {show(back) if back else '(not read)'}")
        sc.say(f"(c) re-assigning an assigned block to another switch: {c['verdict'].upper()}"
               + ("  (qc-mcp's claim holds)" if c["verdict"] == "moved" else
                  "  (qc-mcp's claim is WRONG here: the block now has two switches; section 5 rule 6 and the push path must allow for that)"
                  if c["verdict"] == "added" else ""))
        sc.say(f"(f) leaving the preset discards an assignment change: {f}")
        if f == "PASS":
            sc.say("    The change was discarded and nothing was auto-saved. (f) is answered; entry point 2 may be built.")
        elif f == "FAIL":
            sc.say("    The change SURVIVED leaving the preset: the unit kept or auto-saved it. " + ENTRY_POINT_2_OFF)
            sc.say(f"    Original assignments to restore: {show(baseline)}")
            if ask("    Put the block back now (a live edit; still no save)? (y/n) > ") == "y":
                result["restore"] = restore(qc, mv, baseline)
                sc.say(f"    restore: equals baseline = {result['restore']['equals_baseline']}; now {show(result['restore']['after'])}")
        else:
            sc.say(f"    (f) is NOT answered (the read-back matches neither state, or navigation failed). {ENTRY_POINT_2_OFF}")
        result["verdict"] = f
        sc.say("\nNothing was saved by this script. Select any other preset WITHOUT saving if the unit still shows edits.")
        return sc.EXIT_OK if f == "PASS" else sc.EXIT_CHECK_FAILED
    finally:
        result["close"] = sc.close_with_timeout(qc)
        path = sc.write_result(args.out, "g7", result)
        sc.say(f"\nFull result saved to: {path}")


if __name__ == "__main__":
    sys.exit(sc.run_spike(main))
