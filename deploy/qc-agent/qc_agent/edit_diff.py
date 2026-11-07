"""Pure functions for the middle of an edit (no device, no I/O): what a write read back as, what changed
that the plan did not ask for, and what to write to undo the writes that were made.

* verify_ops(plan, post): the result's `params` / `bypass` entries (contract 9.3): each op's value after
  the write and whether it is what the plan said.
* collateral(pre, post, plan): the check that NOTHING OUTSIDE THE PLAN changed. "Not in the plan" is
  contract 9.3's definition: every parameter value (all eight scenes) except the ones a param op wrote,
  every bypass flag except the eight of a bypass-op cell, and the layout, on all four rows, compared
  exactly against the read taken just before the first write.
* rollback_plan(plan, attempted): the writes that put back what the attempted ops changed, newest first.

Counting rules the result carries (contract 9.3, clarified with E4): `params_compared` counts parameters
(each compared on all eight scenes), `bypass_compared` counts flags (one per cell per scene, 256 at most),
a differing parameter is ONE mismatch whose item names the first differing scene, a differing bypass flag
is one mismatch with its scene, and a cell whose model changed (or appeared, or went away) is one mismatch
with `index` and `scene` both null. Equality is exact; `None` equals `None`. The listing keeps the first
MAX_ITEMS in row, column order (layout, then parameters by index, then bypass flags), the count is complete.
"""
from __future__ import annotations

import math

from .edit_checks import live_block

TOL = 1e-4          # an op's read-back is "as written" within this (the push's G3 tolerance; the wire is float32)
MAX_ITEMS = 64
SCENES = 8
ROWS, COLS = 4, 8


def _num(v):
    return v if type(v) is float and math.isfinite(v) else None


def _same(a, b) -> bool:
    """Exact equality of two read values. A NaN the unit holds in a control the edit never touched reads the same
    before and after (contract 9.3: a non-finite value is a null in the result, and null equals null); NaN against a
    number, or a bool/int against a float, is a difference."""
    if type(a) is not type(b):
        return False
    if type(a) is float and math.isnan(a) and math.isnan(b):
        return True
    return a == b


def _cells(preset):
    """{(row, col): block} for every block on every row."""
    out = {}
    for ch in (preset or {}).get("chains") or []:
        for b in ch.get("blocks") or []:
            out[(ch.get("row"), b.get("col"))] = b
    return out


def _flag(preset, row, col, scene):
    bp = (preset or {}).get("bypass")
    try:
        return bool(bp[row][col][scene])
    except (TypeError, IndexError):
        return None


def verify_ops(plan: dict, post: dict):
    """(params, bypass, all_ok): the result's per-op read-back after the writes."""
    params, bypass, ok_all = [], [], True
    for op in plan["param_ops"]:
        p = next((q for q in (live_block(post, op["row"], op["col"]) or {}).get("params") or []
                  if q.get("index") == op["index"]), None)
        vals = (p or {}).get("values")
        vals = list(vals)[:SCENES] if isinstance(vals, list) else []
        vals += [None] * (SCENES - len(vals))
        read = [_num(v) for v in vals]
        ok = all(r is not None and abs(r - w) <= TOL for r, w in zip(read, op["wire"]))
        ok_all = ok_all and ok
        params.append({"row": op["row"], "col": op["col"], "index": op["index"], "readback": read, "ok": ok})
    for op in plan["bypass_ops"]:
        flags = [_flag(post, op["row"], op["col"], s) for s in range(SCENES)]
        read = flags if None not in flags else None
        ok = read == list(op["bypass"])
        ok_all = ok_all and ok
        bypass.append({"row": op["row"], "col": op["col"], "readback": read, "ok": ok})
    return params, bypass, ok_all


def collateral(pre: dict, post: dict, plan: dict) -> dict:
    """The collateral check of contract 9.3 between the pre-write read and a read after the writes."""
    written = {(op["row"], op["col"], op["index"]) for op in plan["param_ops"]}
    bypass_cells = {(op["row"], op["col"]) for op in plan["bypass_ops"]}
    a, b = _cells(pre), _cells(post)
    mismatches, items = 0, []
    params_compared = bypass_compared = 0
    layout_same = True

    def note(row, col, index, scene):
        nonlocal mismatches
        mismatches += 1
        if len(items) < MAX_ITEMS:
            items.append({"row": row, "col": col, "index": index, "scene": scene})

    for row in range(ROWS):
        for col in range(COLS):
            ba, bb = a.get((row, col)), b.get((row, col))
            if (ba is None) != (bb is None) or (ba or {}).get("model_id") != (bb or {}).get("model_id"):
                layout_same = False
                note(row, col, None, None)
            elif ba is not None:
                pa = {p.get("index"): p.get("values") for p in ba.get("params") or []}
                pb = {p.get("index"): p.get("values") for p in bb.get("params") or []}
                for idx in sorted(set(pa) | set(pb)):
                    if (row, col, idx) in written:
                        continue
                    params_compared += 1
                    va, vb = pa.get(idx), pb.get(idx)
                    if not (isinstance(va, list) and isinstance(vb, list) and len(va) == len(vb)):
                        if va is not None or vb is not None:
                            note(row, col, idx, 0)
                        continue
                    first = next((s for s, (x, y) in enumerate(zip(va, vb)) if not _same(x, y)), None)
                    if first is not None:
                        note(row, col, idx, first)
            if (row, col) in bypass_cells:
                continue
            for s in range(SCENES):
                bypass_compared += 1
                if _flag(pre, row, col, s) != _flag(post, row, col, s):
                    note(row, col, None, s)
    return {"params_compared": params_compared, "bypass_compared": bypass_compared, "layout_same": layout_same,
            "mismatches": mismatches, "items": items}


def rollback_plan(plan: dict, attempted: list) -> list:
    """The writes that undo the attempted ops, newest first. `attempted` lists ("param", col, index) and
    ("bypass", col) in the order their writes were started, the one that was in progress when something
    failed included (it may be half-written). Each entry restores the plan's own `expect` values:
      {"kind": "param", "col", "index", "wires": [8], "scene_mode": bool}
      {"kind": "bypass", "col", "flags": [8]}
    A param whose plan said scene_mode false was a plain write on a control the unit had not assigned to
    scenes, so it is put back with one plain write; one the plan called scene-varying is put back with all
    eight scene values (the bridge cannot un-assign a control from scenes, so such a control may stay
    assigned with its original values)."""
    pops = {(op["col"], op["index"]): op for op in plan["param_ops"]}
    bops = {op["col"]: op for op in plan["bypass_ops"]}
    out, seen = [], set()
    for item in reversed(attempted):
        if item in seen:
            continue
        seen.add(item)
        if item[0] == "param" and (item[1], item[2]) in pops:
            op = pops[(item[1], item[2])]
            out.append({"kind": "param", "col": op["col"], "index": op["index"],
                        "wires": [float(x) for x in op["expect"]], "scene_mode": bool(op["scene_mode"])})
        elif item[0] == "bypass" and item[1] in bops:
            out.append({"kind": "bypass", "col": item[1], "flags": [bool(x) for x in bops[item[1]]["expect"]]})
    return out
