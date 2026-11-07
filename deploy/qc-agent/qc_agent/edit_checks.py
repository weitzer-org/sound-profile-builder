"""The pre-write checks of an edit_preset job (contract 7.2's closing list; design doc section 6).

Pure functions: a parsed plan, the agent's catalog copy and, for the compare-and-swap, a read of the
unit. Nothing here touches the device. Two kinds of check:

* catalog_check(plan, catalog, firmware): everything the plan's own text cannot prove and the catalog
  can. The catalog hash is the agent's, each model_id is a catalog model, each op's cell is not locked
  (a capture block's controls, an unverified clone's indexes, an IR loader, routing, a looper, an
  unknown plugin), each param index is below the model's param_count and is a user-facing, writable
  control (never a string, meter, combo box, floatWithLed, hidden or internal slot), the label is the
  catalog's, the control's range is one the agent can resolve, and a switch's wire values are whole
  positions. Anything else raises EditRefused: the job is refused, nothing is written, and the unit is
  not opened.
* cas_pins(plan, position, preset): the compare-and-swap against ONE fresh read, before the first
  write. Returns the pins that differ, from folder_key, position, name, layout, param_expect,
  bypass_expect (contract 9.3), in that order.

The same pre-checks are reused by the wire guard's arming (qcmcp_backend.WireGuard.arm_edit): it takes
the CheckedPlan this module returns, so what the guard allows is exactly what was checked.
"""
from __future__ import annotations

from dataclasses import dataclass

from . import text
from .export_validator import (ALLOWED_CATEGORIES, CAPTURE_BLOCK_IDS, DENY_MODEL_IDS, LINEAR_SWITCH_TYPES,
                               PLUGIN_NAMES, VERIFIED_CLONE_BASES, ExportRefused, _bounds, control_labels)

# Control types an edit writes: continuous (float, fader) and whole-position (switch, rotarySwitch,
# toggleButton). comboBox and floatWithLed stay refused, as in the exporter; string and grMeter are not controls.
EDIT_TYPES = frozenset({"float", "fader", "switch", "rotarySwitch", "toggleButton"})
NAME_LIMIT = 64          # the same cut read_preset applies to the preset name it reports
WHOLE_TOL = 1e-6         # a switch's wire value maps to a display position this close to a whole number
PINS = ("folder_key", "position", "name", "layout", "param_expect", "bypass_expect")


class EditRefused(ValueError):
    """The plan fails a check the agent makes against its own catalog. The message names the first problem."""


@dataclass(frozen=True)
class CheckedPlan:
    """What catalog_check verified, in the form the wire guard arms from."""
    # (col, index) -> {"wires": the exact values a write may carry, "scene_mode": bool, "param_count": int}
    params: dict
    # col -> the bypass control's index (param_count + 1) and the flags a write may carry
    bypass: dict
    # col -> (model_id, param_count): the cells the plan writes, each of which the unit's own read must confirm
    cells: dict


def model_lock(cm) -> str | None:
    """Why this model's controls are never written by an edit, or None. (The parameter indexes of a
    capture block, an IR loader and an unverified clone are not established on the unit; routing blocks
    and the looper are not tone controls.)"""
    if cm["id"] in DENY_MODEL_IDS:
        return "this model is denied"
    if cm["id"] in CAPTURE_BLOCK_IDS.values():
        return "a Neural Capture block's controls are chosen by the capture, not edited"
    if cm["category"] not in ALLOWED_CATEGORIES:
        return f"category {cm['category']!r} (routing, an IR loader or the looper) is never edited"
    base = cm.get("clones")
    if base is not None and base not in VERIFIED_CLONE_BASES:
        return f"it clones model {base}, whose parameter indexes were never checked against the unit"
    pid = cm.get("plugin_id")
    if pid is not None and str(pid) not in PLUGIN_NAMES:
        return f"plugin id {pid!r} is not one this agent knows"
    return None


def bypass_lock(cm) -> str | None:
    """Why this cell's bypass is never written, or None. A bypass flag is per cell and does not depend on
    the parameter indexes, so an unverified clone and a capture block stay allowed (the unit's own read
    must still show param_count + 1 slots before the bypass control is addressed)."""
    if cm["id"] in DENY_MODEL_IDS:
        return "this model is denied"
    if cm["category"] not in ALLOWED_CATEGORIES and cm["id"] not in CAPTURE_BLOCK_IDS.values():
        return f"category {cm['category']!r} (routing, an IR loader or the looper) is never edited"
    pid = cm.get("plugin_id")
    if pid is not None and str(pid) not in PLUGIN_NAMES:
        return f"plugin id {pid!r} is not one this agent knows"
    return None


def _whole(cp, wire, lo, hi) -> bool:
    """Is this wire value a whole position of the control? A switch maps linearly (ToWire for switches);
    a float with named steps maps through its taper."""
    r = float(wire)
    if cp["type"] not in LINEAR_SWITCH_TYPES:
        skew = cp.get("skew", 1.0)
        if isinstance(skew, bool) or not isinstance(skew, (int, float)) or not skew > 0:
            return False
        r = r ** (1.0 / skew)
    d = lo + r * (hi - lo)
    return abs(d - round(d)) <= WHOLE_TOL


def catalog_check(plan: dict, catalog, expected_firmware: str) -> CheckedPlan:
    """Check a parsed plan (contract.parse_edit_plan) against the agent's catalog. Raises EditRefused."""
    c = plan["catalog"]
    gen = c["coros"]
    if not (expected_firmware == gen or expected_firmware.startswith(gen + ".")):
        raise EditRefused(f"the plan targets CorOS {gen}, this bridge expects {expected_firmware}")
    if c["sha256"] != catalog.sha256:
        raise EditRefused("the plan was built from a different device catalog than this PC has "
                          "(pull the latest main on the PC, or QC-2 is newer/older than the clone)")
    params, bypass, cells = {}, {}, {}
    for op in plan["param_ops"]:
        col, idx, mid = op["col"], op["index"], op["model_id"]
        where = f"column {col} index {idx}"
        cm = catalog.models.get(mid)
        if cm is None:
            raise EditRefused(f"{where}: model {mid} is not in this PC's device catalog")
        why = model_lock(cm)
        if why:
            raise EditRefused(f"{where} ({text.clean(cm['name'], 40)}): locked, {why}")
        pc = cm.get("param_count")
        if isinstance(pc, bool) or not isinstance(pc, int) or pc < 1:
            raise EditRefused(f"{where}: the catalog has no param_count for model {mid}")
        if idx >= pc:
            raise EditRefused(f"{where}: index is at or past param_count {pc} (the unit's trailing slot "
                              "and bypass control are never written as values)")
        cp = cm["params_by_index"].get(idx)
        if cp is None:
            raise EditRefused(f"{where}: not a listed parameter of {text.clean(cm['name'], 40)}")
        labels = control_labels(cm)
        if idx not in labels:
            raise EditRefused(f"{where}: {text.clean(cp['name'], 40)!r} is hidden or internal, not a user control")
        if labels[idx] != op["label"]:
            raise EditRefused(f"{where}: the catalog calls it {labels[idx]!r}, not {text.clean(op['label'], 40)!r}")
        if cp["type"] not in EDIT_TYPES:
            raise EditRefused(f"{where}: type {cp['type']!r} is not writable by an edit")
        try:
            lo, hi = _bounds(cp, where)
        except ExportRefused as e:
            raise EditRefused(str(e)) from None
        if not hi > lo:
            raise EditRefused(f"{where}: empty range {lo:g}..{hi:g}")
        if cp["type"] in LINEAR_SWITCH_TYPES or cp.get("step_names"):
            for s, w in enumerate(op["wire"]):
                if not _whole(cp, w, lo, hi):
                    raise EditRefused(f"{where} scene {'ABCDEFGH'[s]}: {text.clean(labels[idx], 40)} is a switch and "
                                      "the wire value is not a whole position")
        if (col, idx) in params:  # parse_edit_plan refuses this too
            raise EditRefused(f"{where}: written twice")
        params[(col, idx)] = {"wires": tuple(float(w) for w in op["wire"]), "scene_mode": op["scene_mode"],
                              "param_count": pc}
        cells[col] = (mid, pc)
    for op in plan["bypass_ops"]:
        col, mid = op["col"], op["model_id"]
        cm = catalog.models.get(mid)
        if cm is None:
            raise EditRefused(f"column {col}: model {mid} is not in this PC's device catalog")
        why = bypass_lock(cm)
        if why:
            raise EditRefused(f"column {col} ({text.clean(cm['name'], 40)}): locked, {why}")
        pc = cm.get("param_count")
        if isinstance(pc, bool) or not isinstance(pc, int) or pc < 1:
            raise EditRefused(f"column {col}: the catalog has no param_count for model {mid}")
        if cells.get(col, (mid, pc)) != (mid, pc):
            raise EditRefused(f"column {col}: two models in one cell")
        bypass[col] = {"index": pc + 1, "flags": tuple(bool(f) for f in op["bypass"])}
        cells[col] = (mid, pc)
    return CheckedPlan(params=params, bypass=bypass, cells=cells)


# ---- the compare-and-swap ----

def live_layout(preset) -> list:
    """[[row, col, model_id], ...] for every block on every row, sorted: the plan's expect.layout shape."""
    out = []
    for ch in preset.get("chains") or []:
        for b in ch.get("blocks") or []:
            out.append([ch.get("row"), b.get("col"), b.get("model_id")])
    out.sort(key=lambda t: (t[0], t[1]))
    return out


def live_block(preset, row, col):
    for ch in preset.get("chains") or []:
        if ch.get("row") == row:
            for b in ch.get("blocks") or []:
                if b.get("col") == col:
                    return b
    return None


def live_param(block, index):
    for p in (block or {}).get("params") or []:
        if p.get("index") == index:
            return p
    return None


def live_bypass(preset, row, col):
    """The unit's eight bypass flags of a cell, or None if it sent none."""
    bp = preset.get("bypass")
    if not bp or row >= len(bp) or col >= len(bp[row]):
        return None
    flags = bp[row][col]
    return [bool(x) for x in flags[:8]] if isinstance(flags, list) and len(flags) >= 8 else None


def floats_equal(vals, expect) -> bool:
    """Exact: eight real floats (a bool or int from the unit is not one), each equal to the plan's number."""
    return (isinstance(vals, list) and len(vals) == 8 and len(expect) == 8
            and all(type(v) is float and v == e for v, e in zip(vals, expect)))


def cas_pins(plan: dict, pos: dict, preset: dict) -> list:
    """The pins of the compare-and-swap that differ between the plan's expect/ops and a fresh read of the
    open preset, in the contract's order. Empty = the live preset is exactly what the plan was built from.
    `pos` is the unit's SetlistPosition reply, `preset` the plain read (device.py)."""
    e = plan["expect"]
    failed = []
    if pos.get("is_factory") or pos.get("folder_key") != e["folder_key"]:
        failed.append("folder_key")
    if pos.get("is_factory") or pos.get("position") != e["position"]:
        failed.append("position")
    if text.clean(preset.get("name", "") or "", NAME_LIMIT) != e["name"]:
        failed.append("name")
    if live_layout(preset) != [list(t) for t in e["layout"]]:
        failed.append("layout")
    # Each pin is judged on its own: a cell that is gone or has other values than the plan fails its pin, whatever
    # the layout says (the layout pin is about models and cells, these two about values and flags).
    pe = True
    for op in plan["param_ops"]:
        p = live_param(live_block(preset, op["row"], op["col"]), op["index"])
        if (p is None or not floats_equal(p.get("values"), op["expect"])
                # scene_mode false asserts the control is not scene-assigned: a plain write on an
                # assigned control would land on the active scene only
                or (not op["scene_mode"] and p.get("scene_mode"))):
            pe = False
            break
    be = all(live_bypass(preset, op["row"], op["col"]) == list(op["expect"]) for op in plan["bypass_ops"])
    if not pe:
        failed.append("param_expect")
    if not be:
        failed.append("bypass_expect")
    return failed


def unit_layout_problem(plan: dict, checked: CheckedPlan, preset: dict) -> str | None:
    """After the compare-and-swap matched: every cell the plan writes must report exactly param_count + 1
    parameters on this unit (G1). Any other count means the catalog's indexes don't describe this block
    here, so no value may be written to it and its bypass control (param_count + 1) is not addressed."""
    for col, (mid, pc) in sorted(checked.cells.items()):
        b = live_block(preset, 0, col)
        if b is None or b.get("model_id") != mid:
            return f"column {col} does not hold model {mid}"
        n = len(b.get("params") or [])
        if n != pc + 1:
            return f"column {col} reads {n} parameters; the catalog expects {pc + 1} ({pc} plus the trailing slot)"
    return None
