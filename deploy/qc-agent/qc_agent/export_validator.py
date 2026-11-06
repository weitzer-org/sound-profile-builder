"""Validate a push job's device export against the agent's own catalog copy.

The export is P2's `qc2-device-export/1` payload, exactly as internal/deviceexport writes it
(Payload / Block / Param in export.go; a sample is internal/deviceexport/testdata/golden/
pushable_g3.json). QC-2 built it and signed the job, but the agent re-checks every operation
before the unit is opened (plan section 5), so a compromised or buggy QC-2 is still bounded by
the rules below. The first problem refuses the whole export; nothing is guessed or clamped.

Shape: every key set is exact (`plugin` on a block and `units` on a param are optional, as Go
omits them when empty) and every value has its type.

Rules, in the order they run:
* format qc2-device-export/1; the catalog header equals this PC's catalog (sha256 of the file,
  CorOS generation, origin), and the generation matches the expected firmware;
* `unresolved` is empty (a push needs zero); `scene_sources` is A,B,A,A,A,A,A,A, the only mapping
  this format defines;
* exactly one row: row 0, input 1, output 19 (the routing G3 read back); 1-8 blocks in columns
  0..n-1, in order;
* each block: a catalog model (name and category as the catalog has them), not internal
  (20000-20002 or an Internal category), in the same category allowlist P2 uses, not a side-chain
  model; a plugin model carries its plugin name (QC-2 exports plugin models only for plugins the
  owner has; placing one on the unit is untested, so the job code fails cleanly if the unit
  refuses it); a cloned model's base is 12000, the only base G1 verified, with index_evidence
  saying so; eight bypass flags, scenes C-H equal to A;
* each param: index below the model's param_count (the device's trailing slot, index
  param_count, is never written), a listed user control of type float, the label P2 shows the
  owner for that index (catalog.LabelParams, ported in catalog.labels), the catalog's unit;
  indexes strictly increasing; eight finite display and wire values with C-H equal to A and
  scene_mode true exactly when A and B differ; every wire in 0..1; and every wire is the
  confirmed taper of its display value (only the tapers and bounds the unit confirmed, as in P2);
* `at_default` names controls of this model, once each, none of them also written. They are
  not written: a block added to an empty slot starts at its defaults (seen for two params in
  G4), and the job logs any read-back that differs.

Labels and display values never decide a write: indexes and wire values do. They are checked
so that what the owner approved in the dry run is provably what gets written.
"""
from __future__ import annotations

import math
import re
from dataclasses import dataclass, field

from .catalog import INTERNAL_MODEL_IDS, VERIFIED_CLONE_BASES, is_internal_param, labels, taper_refusal, to_wire

FORMAT = "qc2-device-export/1"
SUPPORTED_FORMATS = frozenset({FORMAT})
MAX_BLOCKS = 8
SCENES = 8
SCENE_SOURCES = ["A", "B", "A", "A", "A", "A", "A", "A"]
ROW, IN_PORT, OUT_PORT = 0, 1, 19  # read back exactly in G3
TAPER_TOL = 1e-9   # Go's math.Pow and Python's ** can differ in the last bit

# The categories P2 places (internal/deviceexport/resolve.go pushableCategories). Splitter, Mixer,
# FX Loop, Looper, Neural Capture, IR loaders and anything a future CorOS adds are refused.
ALLOWED_CATEGORIES = frozenset({
    "Guitar Amplifier", "Bass Amplifier", "Guitar Overdrive", "Bass Overdrive",
    "Cabsim Guitar (M)", "Cabsim Guitar (ST)", "Cabsim Bass (M)", "Cabsim Bass (ST)",
    "Compressor", "Equalizer", "Delay", "Reverb", "Modulation", "Pitch",
    "Wah", "Filter", "Utility", "Morph", "Synth",
})

_SHA_RE = re.compile(r"[0-9a-f]{64}")
_GEN_RE = re.compile(r"[0-9]+\.[0-9]+")
# Preset names become a file name on the unit (write_preset_file: "<folder>/<name>.pb"). How CorOS
# sanitises them is unverified, so the agent is stricter than the contract: no path separators,
# no Windows-reserved characters, no leading dot and no "..".
_NAME_RE = re.compile(r"[A-Za-z0-9 _()+,!'&#.-]{1,32}")


class ExportRefused(ValueError):
    """The export cannot be checked or fails a check. Nothing was sent to the unit."""


def check_preset_name(name: str) -> None:
    if (not isinstance(name, str) or not _NAME_RE.fullmatch(name) or name != name.strip()
            or name.startswith(".") or ".." in name):
        raise ExportRefused("the preset name may use letters, digits, spaces and _ ( ) + , ! ' & # . - only "
                            "(no leading dot, no '..')")


@dataclass
class PlannedParam:
    index: int
    label: str
    wires: list[float]          # scenes A-H, exactly as the export gives them

    @property
    def varies(self) -> bool:
        """Scene-varying: written with set_param_scenes (all eight values). Otherwise one set_param."""
        return len(set(self.wires)) > 1


@dataclass
class PlannedBlock:
    col: int
    model_id: int
    model: str
    category: str
    plugin: str | None
    param_count: int
    params: list[PlannedParam]
    bypass: list[bool]                                     # scenes A-H, True = bypassed
    at_default: list[tuple[str, int]] = field(default_factory=list)  # (label, index), not written

    @property
    def bypass_index(self) -> int:
        """The block's bypass control: one past the device's own parameter array, whose length is
        param_count + 1 (G1: N = full list + 1 trailing slot; qc-mcp bypass_param_index = N)."""
        return self.param_count + 1


@dataclass
class Plan:
    row: int
    in_port: int
    out_port: int
    blocks: list[PlannedBlock]


# ---- small typed readers ----

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


def _printable(s, maxlen, where, minlen=0):
    if (not isinstance(s, str) or not minlen <= len(s) <= maxlen
            or any(not 0x20 <= ord(c) <= 0x7E for c in s)):
        raise ExportRefused(f"{where} must be printable ASCII, {minlen}-{maxlen} characters")
    return s


def _eight(v, where, kind):
    if not isinstance(v, list) or len(v) != SCENES:
        raise ExportRefused(f"{where} must be a list of {SCENES} values (scenes A-H)")
    if kind is bool:
        if not all(isinstance(x, bool) for x in v):
            raise ExportRefused(f"{where} must hold true/false flags")
        return list(v)
    return [_num(x, f"{where}[{'ABCDEFGH'[i]}]") for i, x in enumerate(v)]


def _c_to_h_follow_a(vals, where):
    """scene_sources is A,B,A,A,A,A,A,A: scenes C-H must carry scene A's value."""
    if any(vals[i] != vals[0] for i in range(2, SCENES)):
        raise ExportRefused(f"{where}: scenes C-H must equal scene A (scene_sources A,B,A,A,A,A,A,A)")


# ---- the checks ----

def _check_header(p, export_format, catalog, expected_firmware):
    _exact(p, ("format", "catalog", "guitar", "scene_sources", "rows", "unresolved"), "payload")
    if p["format"] != export_format:
        raise ExportRefused("payload.format differs from export.format")
    c = p["catalog"]
    _exact(c, ("coros", "origin", "sha256"), "payload.catalog")
    gen = c["coros"]
    if not isinstance(gen, str) or not _GEN_RE.fullmatch(gen):
        raise ExportRefused("payload.catalog.coros must look like 4.1")
    if not (expected_firmware == gen or expected_firmware.startswith(gen + ".")):
        raise ExportRefused(f"export targets CorOS {gen}, this bridge expects {expected_firmware}")
    _printable(c["origin"], 200, "payload.catalog.origin")
    sha = c["sha256"]
    if not isinstance(sha, str) or not _SHA_RE.fullmatch(sha):
        raise ExportRefused("payload.catalog.sha256 must be 64 lower-case hex characters")
    if sha != catalog.sha256:
        raise ExportRefused("the export was built from a different device catalog than this PC has "
                            "(pull the latest main on the PC, or QC-2 is newer/older than the clone)")
    if gen != catalog.source.get("coros_version") or c["origin"] != catalog.source.get("origin"):
        raise ExportRefused("payload.catalog disagrees with this PC's catalog header")

    if not isinstance(p["unresolved"], list):
        raise ExportRefused("payload.unresolved must be a list")
    if p["unresolved"]:
        raise ExportRefused(f"the export has {len(p['unresolved'])} unresolved item(s); a push needs zero")
    g = p["guitar"]
    if (not isinstance(g, str) or not 1 <= len(g) <= 120
            or any(ord(ch) < 0x20 or 0x7F <= ord(ch) <= 0x9F for ch in g)):
        raise ExportRefused("payload.guitar must be 1-120 characters without control characters")
    if p["scene_sources"] != SCENE_SOURCES:
        raise ExportRefused("payload.scene_sources must be A,B,A,A,A,A,A,A in qc2-device-export/1")


def _check_block(bi, b, catalog) -> PlannedBlock:
    where = f"block {bi}"
    _exact(b, ("col", "model_id", "model", "category", "index_evidence", "bypass", "params", "at_default"),
           where, optional=("plugin",))
    if _int(b["col"], where + ".col") != bi:
        raise ExportRefused(f"{where}: columns must run 0..n-1 in order with no gaps")
    mid = _int(b["model_id"], where + ".model_id")
    if mid in INTERNAL_MODEL_IDS:
        raise ExportRefused(f"{where}: model {mid} is an internal capture-engine model; never placed")
    cm = catalog.models.get(mid)
    if cm is None:
        raise ExportRefused(f"{where}: model {mid} is not in this PC's device catalog")
    if b["model"] != cm["name"]:
        raise ExportRefused(f"{where}: model {mid} is {cm['name']!r} in the catalog, not {b['model']!r}")
    where = f"block {bi} ({cm['name']})"
    if b["category"] != cm["category"]:
        raise ExportRefused(f"{where}: category {b['category']!r} differs from the catalog's {cm['category']!r}")
    cat = cm["category"]
    if "Internal" in cat or cat not in ALLOWED_CATEGORIES:
        raise ExportRefused(f"{where}: category {cat!r} is not pushed in v1")
    if cm["name"].endswith("(S/C)"):
        raise ExportRefused(f"{where}: side-chain models are not pushed in v1")

    plugin = None
    if cm.get("plugin_id") is not None:
        if "plugin" not in b:
            raise ExportRefused(f"{where}: plugin model without its plugin name")
        plugin = _printable(b["plugin"], 32, where + ".plugin", minlen=1)
    elif "plugin" in b:
        raise ExportRefused(f"{where}: plugin on a model that needs none")

    clones = cm.get("clones")
    if clones is not None and clones not in VERIFIED_CLONE_BASES:
        raise ExportRefused(f"{where}: parameter indexes of models cloning {clones} are not verified "
                            f"(G1 checked 12000 only)")
    want_evidence = "plain" if clones is None else f"clone-{clones}-g1"
    if b["index_evidence"] != want_evidence:
        raise ExportRefused(f"{where}: index_evidence {str(b['index_evidence'])[:40]!r} doesn't match the "
                            f"catalog ({want_evidence!r})")

    pc = cm.get("param_count")
    if isinstance(pc, bool) or not isinstance(pc, int) or pc < 1:
        raise ExportRefused(f"{where}: the catalog has no param_count for this model")
    bypass = _eight(b["bypass"], where + ".bypass", bool)
    _c_to_h_follow_a(bypass, where + ".bypass")

    lab = labels(cm)
    if not isinstance(b["params"], list) or len(b["params"]) > pc:
        raise ExportRefused(f"{where}.params must be a list of at most {pc}")
    params, last = [], -1
    for pi, prm in enumerate(b["params"]):
        p = _check_param(f"{where} param {pi}", prm, cm, pc, lab)
        if p.index <= last:
            raise ExportRefused(f"{where} param {pi}: indexes must be strictly increasing (index {p.index})")
        last = p.index
        params.append(p)

    written = {p.label for p in params}
    by_label = {v: k for k, v in lab.items()}
    at_default = []
    if not isinstance(b["at_default"], list):
        raise ExportRefused(f"{where}.at_default must be a list")
    for s in b["at_default"]:
        if not isinstance(s, str) or s not in by_label:
            raise ExportRefused(f"{where}.at_default: {str(s)[:40]!r} is not a control of this model")
        if s in written or any(s == x for x, _ in at_default):
            raise ExportRefused(f"{where}.at_default: {s!r} is listed twice or also written")
        at_default.append((s, by_label[s]))
    return PlannedBlock(bi, mid, cm["name"], cat, plugin, pc, params, bypass, at_default)


def _check_param(pw, prm, cm, param_count, lab) -> PlannedParam:
    _exact(prm, ("index", "label", "scene_mode", "display", "wire"), pw, optional=("units",))
    idx = _int(prm["index"], pw + ".index")
    if not 0 <= idx < param_count:
        raise ExportRefused(f"{pw}: index {idx} is at or past param_count {param_count} "
                            "(the trailing slot is never written)")
    cp = cm["params_by_index"].get(idx)
    if cp is None:
        raise ExportRefused(f"{pw}: index {idx} is a hidden or unlisted parameter of this model")
    if is_internal_param(cp):
        raise ExportRefused(f"{pw}: index {idx} ({cp['name']}) is not a user control")
    pw = f"{pw.rsplit(' param ', 1)[0]} {lab[idx]} [{idx}]"
    why = taper_refusal(cp)
    if why:
        raise ExportRefused(f"{pw}: {why}")
    if prm["label"] != lab[idx]:
        raise ExportRefused(f"{pw}: index {idx} is {lab[idx]!r} on this model, not {str(prm['label'])[:40]!r}")
    if prm.get("units") != cp.get("units"):
        raise ExportRefused(f"{pw}: units {prm.get('units')!r} differ from the catalog's {cp.get('units')!r}")
    if not isinstance(prm["scene_mode"], bool):
        raise ExportRefused(f"{pw}.scene_mode must be true/false")
    display = _eight(prm["display"], pw + ".display", float)
    wires = _eight(prm["wire"], pw + ".wire", float)
    _c_to_h_follow_a(display, pw + ".display")
    _c_to_h_follow_a(wires, pw + ".wire")
    if prm["scene_mode"] != (display[0] != display[1]) or prm["scene_mode"] != (wires[0] != wires[1]):
        raise ExportRefused(f"{pw}: scene_mode doesn't match the scene values")
    for s in range(2):  # C-H equal A, already checked
        w = wires[s]
        if not 0.0 <= w <= 1.0:
            raise ExportRefused(f"{pw}: wire {w:g} is outside 0..1")
        try:
            want = to_wire(cp, display[s])
        except ValueError as e:
            raise ExportRefused(f"{pw}: scene {'AB'[s]}: {e}") from None
        if abs(want - w) > TAPER_TOL:
            raise ExportRefused(f"{pw}: scene {'AB'[s]}: wire {w!r} is not the taper of display {display[s]:g} "
                                f"(expected {want!r})")
    return PlannedParam(idx, lab[idx], wires)


def validate(export: dict, catalog, expected_firmware: str) -> Plan:
    """`export` is the job's args.export object ({"format", "payload"}). Returns the plan to
    execute, or raises ExportRefused naming the first problem."""
    _exact(export, ("format", "payload"), "export")
    fmt = export["format"]
    if fmt not in SUPPORTED_FORMATS:
        raise ExportRefused(f"unsupported export format {str(fmt)[:48]!r}; this agent implements {FORMAT}")
    p = export["payload"]
    _check_header(p, fmt, catalog, expected_firmware)

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
    return Plan(ROW, IN_PORT, OUT_PORT, [_check_block(bi, b, catalog) for bi, b in enumerate(blocks)])
