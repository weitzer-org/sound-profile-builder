"""Validate a push job's device export against the agent's own catalog copy.

The P2 device export (internal/deviceexport) does not exist yet. This validator
implements the `qc2-device-export/1` shape sketched in
docs/design/qc-device-bridge-plan.md section 4, read strictly: every key set is
exact, every value is checked, and anything the agent cannot check against its
own catalog is refused rather than passed through. When P2 lands, its golden
exports must pass this validator (or this file changes in the same PR).

What it checks, per the plan (sections 4 and 5) and the Phase 0 results (7.1/7.2):
* `format` is exactly qc2-device-export/1 (the job's export.format too);
* the export was built from the same catalog file the agent holds (sha256);
* the catalog generation matches the firmware the agent expects;
* exactly one row (row 0) routed in 1 -> out 19, the routing G3 verified;
* 1-8 blocks in columns 0..n-1, each a catalog model in an allowed category,
  not a plugin model, not denylisted (NC_Recorder 20000), name equal to the
  catalog's;
* each parameter: a catalog index of that model (hidden params and indexes the
  catalog doesn't list are refused, which also keeps every index below the
  device's EFF, plan 7.2), the catalog's name, a writable type, a numeric range
  (symbolic bounds refused), display inside the range (whole numbers for
  switches), and a finite wire value in 0..1;
* bypass flags are booleans; `unresolved` is empty.

What it cannot check yet: that `wire` is the right taper of `display`. The
catalog has no `skew` until P1, so the display -> wire mapping is QC-2's
responsibility; the agent only bounds both values.
"""
from __future__ import annotations

import math
import re
from dataclasses import dataclass, field

FORMAT = "qc2-device-export/1"
SUPPORTED_FORMATS = frozenset({FORMAT})
MAX_BLOCKS = 8
MAX_PARAMS_PER_BLOCK = 64
SCENES = 8
ROW, IN_PORT, OUT_PORT = 0, 1, 19  # qc-mcp clear_grid default routing; read back exactly in G3

ALLOWED_CATEGORIES = frozenset({
    "Guitar Amplifier", "Bass Amplifier",
    "Cabsim Guitar (M)", "Cabsim Guitar (ST)", "Cabsim Bass (M)", "Cabsim Bass (ST)",
    "Guitar Overdrive", "Bass Overdrive", "Compressor", "Equalizer", "Filter", "Wah",
    "Modulation", "Pitch", "Delay", "Reverb", "Utility", "Morph", "Synth",
})
# Refused categories, for the message: routing/I-O blocks (Splitter, Mixer, FX Loop),
# captures and IRs (v1: plan section 4), the looper.
DENY_MODEL_IDS = frozenset({20000})  # NC_Recorder: recorded crashing a unit (audit 1.5)
CONTINUOUS_TYPES = frozenset({"float", "fader", "floatWithLed"})
DISCRETE_TYPES = frozenset({"switch", "rotarySwitch", "comboBox", "toggleButton"})
WRITABLE_TYPES = CONTINUOUS_TYPES | DISCRETE_TYPES  # never string, grMeter (a meter), empty

_SHA_RE = re.compile(r"[0-9a-f]{64}")
# Preset names become a file name on the unit (write_preset_file: "<folder>/<name>.pb"). How CorOS
# sanitises them is unverified, so the agent is stricter than the contract: no path separators,
# no Windows-reserved characters, no leading dot and no "..".
_NAME_RE = re.compile(r"[A-Za-z0-9 _()+,!'&#.-]{1,32}")


def check_preset_name(name: str) -> None:
    if (not isinstance(name, str) or not _NAME_RE.fullmatch(name) or name != name.strip()
            or name.startswith(".") or ".." in name):
        raise ExportRefused("the preset name may use letters, digits, spaces and _ ( ) + , ! ' & # . - only "
                            "(no leading dot, no '..')")
_GEN_RE = re.compile(r"[0-9]+\.[0-9]+")


class ExportRefused(ValueError):
    """The export cannot be checked or fails a check. Nothing was sent to the unit."""


@dataclass
class PlannedParam:
    index: int
    name: str
    wire: float
    scene_wires: list[float] | None  # all 8 scenes (A, B, then A for C-H) when B differs


@dataclass
class PlannedBlock:
    col: int
    model_id: int
    model: str
    params: list[PlannedParam]
    bypass_scenes: list[bool]  # 8 flags, A, B, then A for C-H
    max_param_index: int = 0   # the catalog's highest listed index for this model (wire guard)

    @property
    def bypass_varies(self) -> bool:
        return len(set(self.bypass_scenes)) > 1


@dataclass
class Plan:
    row: int
    in_port: int
    out_port: int
    blocks: list[PlannedBlock]
    scene_labels: list[tuple[int, str]] = field(default_factory=list)  # validated, not written in v1

    def op_count(self) -> int:
        return 1 + sum(1 + len(b.params) + (1 if any(b.bypass_scenes) else 0) for b in self.blocks)


def _exact(obj, keys, where, optional=()):
    if not isinstance(obj, dict):
        raise ExportRefused(f"{where} must be an object")
    have = set(obj)
    if not set(keys) <= have or not have <= set(keys) | set(optional):
        raise ExportRefused(f"{where} has keys {sorted(have)}, expected {sorted(keys)}"
                            + (f" (optional {sorted(optional)})" if optional else ""))


def _int(v, where):
    if isinstance(v, bool) or not isinstance(v, int):
        raise ExportRefused(f"{where} must be an integer")
    return v


def _num(v, where):
    if isinstance(v, bool) or not isinstance(v, (int, float)) or not math.isfinite(v):
        raise ExportRefused(f"{where} must be a finite number")
    return float(v)


def _printable(s, maxlen, where):
    if not isinstance(s, str) or len(s) > maxlen or any(not 0x20 <= ord(c) <= 0x7E for c in s):
        raise ExportRefused(f"{where} must be printable ASCII, at most {maxlen} characters")
    return s


def _check_value(cp, display, wire, where):
    lo, hi = cp.get("min"), cp.get("max")
    if (cp.get("min_symbol") or cp.get("max_symbol") or isinstance(lo, bool) or isinstance(hi, bool)
            or not isinstance(lo, (int, float)) or not isinstance(hi, (int, float))):
        raise ExportRefused(f"{where}: the catalog has no numeric range for this parameter "
                            f"(symbolic or missing bound); the agent can't check it")
    d = _num(display, where + ".display")
    if not min(lo, hi) <= d <= max(lo, hi):
        raise ExportRefused(f"{where}: display {d:g} is outside {lo:g}..{hi:g}")
    if (cp["type"] in DISCRETE_TYPES or cp.get("step_names")) and not d.is_integer():
        raise ExportRefused(f"{where}: {cp['type']} position must be a whole number, got {d:g}")
    w = _num(wire, where + ".wire")
    if not 0.0 <= w <= 1.0:
        raise ExportRefused(f"{where}: wire {w:g} is outside 0..1")
    return w


def _check_block(bi, b, nblocks, catalog) -> PlannedBlock:
    where = f"block {bi}"
    _exact(b, ("col", "model_id", "model", "params", "bypass"), where)
    if _int(b["col"], where + ".col") != bi:
        raise ExportRefused(f"{where}: columns must run 0..{nblocks - 1} in order")
    mid = _int(b["model_id"], where + ".model_id")
    if mid in DENY_MODEL_IDS:
        raise ExportRefused(f"{where}: model {mid} is denied (NC_Recorder)")
    cm = catalog.models.get(mid)
    if cm is None:
        raise ExportRefused(f"{where}: model {mid} is not in this PC's device catalog")
    if b["model"] != cm["name"]:
        raise ExportRefused(f"{where}: model {mid} is {cm['name']!r} in the catalog, not {b['model']!r}")
    where = f"block {bi} ({cm['name']})"
    if cm.get("plugin_id") is not None:
        raise ExportRefused(f"{where}: plugin models are not pushed in v1")
    if cm["category"] not in ALLOWED_CATEGORIES:
        raise ExportRefused(f"{where}: category {cm['category']!r} is not pushed in v1")

    if not isinstance(b["params"], list) or len(b["params"]) > MAX_PARAMS_PER_BLOCK:
        raise ExportRefused(f"{where}.params must be a list of at most {MAX_PARAMS_PER_BLOCK}")
    params, seen = [], set()
    for pi, prm in enumerate(b["params"]):
        p = _check_param(f"{where} param {pi}", prm, cm)
        if p.index in seen:
            raise ExportRefused(f"{where} param {pi}: index {p.index} appears twice")
        seen.add(p.index)
        params.append(p)

    _exact(b["bypass"], ("A", "B"), where + ".bypass")
    ba, bb = b["bypass"]["A"], b["bypass"]["B"]
    if not isinstance(ba, bool) or not isinstance(bb, bool):
        raise ExportRefused(f"{where}.bypass flags must be true/false")
    return PlannedBlock(bi, mid, cm["name"], params, [ba, bb] + [ba] * (SCENES - 2),
                        max(cm["params_by_index"], default=0))


def _check_param(pw, prm, cm) -> PlannedParam:
    _exact(prm, ("index", "name", "display", "wire"), pw, optional=("scene_b",))
    idx = _int(prm["index"], pw + ".index")
    cp = cm["params_by_index"].get(idx)
    if cp is None:
        raise ExportRefused(f"{pw}: index {idx} is not a listed parameter of this model")
    if prm["name"] != cp["name"]:
        raise ExportRefused(f"{pw}: index {idx} is {cp['name']!r} in the catalog, not {prm['name']!r}")
    pw = f"{pw.rsplit(' param ', 1)[0]} {cp['name']} [{idx}]"
    if cp["type"] not in WRITABLE_TYPES:
        raise ExportRefused(f"{pw}: type {cp['type']!r} is not writable by the agent")
    a = _check_value(cp, prm["display"], prm["wire"], pw)
    scene_wires = None
    if "scene_b" in prm:
        _exact(prm["scene_b"], ("display", "wire"), pw + ".scene_b")
        bw = _check_value(cp, prm["scene_b"]["display"], prm["scene_b"]["wire"], pw + " scene B")
        if bw != a:
            # G6: unwritten scenes keep their old value, so write all eight; C-H copy A (plan 4).
            scene_wires = [a, bw] + [a] * (SCENES - 2)
    return PlannedParam(idx, cp["name"], a, scene_wires)


def validate(export: dict, catalog, expected_firmware: str) -> Plan:
    """`export` is the job's args.export object ({"format", "payload"}). Returns the
    plan to execute, or raises ExportRefused naming the first problem."""
    _exact(export, ("format", "payload"), "export")
    fmt = export["format"]
    if fmt not in SUPPORTED_FORMATS:
        raise ExportRefused(f"unsupported export format {fmt!r}; this agent implements {FORMAT}")
    p = export["payload"]
    _exact(p, ("format", "catalog", "rows", "scenes", "unresolved"), "payload")
    if p["format"] != fmt:
        raise ExportRefused("payload.format differs from export.format")

    _exact(p["catalog"], ("coros", "origin", "sha256"), "payload.catalog")
    gen = p["catalog"]["coros"]
    if not isinstance(gen, str) or not _GEN_RE.fullmatch(gen):
        raise ExportRefused("payload.catalog.coros must look like 4.1")
    if not (expected_firmware == gen or expected_firmware.startswith(gen + ".")):
        raise ExportRefused(f"export targets CorOS {gen}, this bridge expects {expected_firmware}")
    _printable(p["catalog"]["origin"], 200, "payload.catalog.origin")
    sha = p["catalog"]["sha256"]
    if not isinstance(sha, str) or not _SHA_RE.fullmatch(sha):
        raise ExportRefused("payload.catalog.sha256 must be 64 lower-case hex characters")
    if sha != catalog.sha256:
        raise ExportRefused("the export was built from a different device catalog than this PC has "
                            "(pull the latest main on the PC, or QC-2 is newer/older than the clone)")

    if not isinstance(p["unresolved"], list):
        raise ExportRefused("payload.unresolved must be a list")
    if p["unresolved"]:
        raise ExportRefused(f"the export has {len(p['unresolved'])} unresolved item(s); a push needs zero")

    labels = []
    if not isinstance(p["scenes"], list) or len(p["scenes"]) > SCENES:
        raise ExportRefused(f"payload.scenes must be a list of at most {SCENES}")
    seen_scene = set()
    for i, s in enumerate(p["scenes"]):
        _exact(s, ("index", "label"), f"scenes[{i}]")
        si = _int(s["index"], f"scenes[{i}].index")
        if not 0 <= si < SCENES or si in seen_scene:
            raise ExportRefused(f"scenes[{i}].index must be a unique 0..7")
        seen_scene.add(si)
        labels.append((si, _printable(s["label"], 32, f"scenes[{i}].label")))

    rows = p["rows"]
    if not isinstance(rows, list) or len(rows) != 1:
        raise ExportRefused("v1 pushes exactly one row")
    r = rows[0]
    _exact(r, ("row", "in_port", "out_port", "blocks"), "rows[0]")
    if (_int(r["row"], "rows[0].row"), _int(r["in_port"], "rows[0].in_port"),
            _int(r["out_port"], "rows[0].out_port")) != (ROW, IN_PORT, OUT_PORT):
        raise ExportRefused(f"v1 places the chain on row {ROW}, input {IN_PORT}, output {OUT_PORT} only")
    blocks = r["blocks"]
    if not isinstance(blocks, list) or not 1 <= len(blocks) <= MAX_BLOCKS:
        raise ExportRefused(f"rows[0].blocks must hold 1-{MAX_BLOCKS} blocks")

    planned = [_check_block(bi, b, len(blocks), catalog) for bi, b in enumerate(blocks)]
    return Plan(ROW, IN_PORT, OUT_PORT, planned, labels)
