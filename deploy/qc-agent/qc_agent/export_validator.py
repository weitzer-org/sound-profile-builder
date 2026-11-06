"""Validate a push job's device export against the agent's own catalog copy.

Implements `qc2-device-export/1` exactly as internal/deviceexport writes it (plan
section 4, "as built in P2"), read strictly: every key set is exact, every value is
checked, and anything the agent cannot check against its own catalog is refused rather
than passed through. The Go goldens (internal/deviceexport/testdata/golden/*.json) are
fed through this validator by tests/test_go_goldens.py, so the two sides cannot drift
apart again: change one and the other's tests say so.

What it checks, per the plan (sections 4 and 5) and the Phase 0 results (7.1/7.2):
* `format` is exactly qc2-device-export/1 (the job's export.format too);
* the export was built from the same catalog file the agent holds (sha256);
* the catalog generation matches the firmware the agent expects;
* `scene_sources` is exactly A, B, then A for C-H (G6: the device does not copy A into
  C-H, so every scene is written; C-H must carry Scene A's values and bypass flag);
* exactly one row (row 0) routed in 1 -> out 19, the routing G3 verified;
* 1-8 blocks in columns 0..n-1, each a catalog model in an allowed category, not a plugin
  model, not denylisted (NC_Recorder 20000), name and category equal to the catalog's,
  and `index_evidence` consistent with the catalog (plain, or a clone of a base G1 checked);
* each parameter: a catalog index of that model below its `param_count` (G1, plan 7.2: the device
  reports param_count + 1 slots and the last one, index param_count, is never written), a user-facing control (not an internal
  bypass/phi/IR-selector slot or a meter), the label the catalog gives it (numbered per
  mic on dual-mic cabs, as catalog.LabelParams does), a writable type, a numeric range
  (symbolic bounds refused), eight display values inside the range (whole numbers for
  switches), eight finite wire values in 0..1, and `scene_mode` matching whether A and B
  differ;
* each parameter's `evidence` (plan 4.1) names only known sources, and the wire values are the
  ones the catalog gives for those sources: the taper ((d-min)/(max-min))^skew for a float,
  (d-min)/(max-min) for a switch position ("encoding:qc-mcp-linear"), and the float taper for a
  fader ("encoding:fader-linear-guess", an unverified guess), so a conversion QC-2 got wrong
  is refused here too;
* eight boolean bypass flags per block; `unresolved` is empty; `warnings` (settings the push
  leaves out on purpose, e.g. a non-default cab mic) is a list of short items and changes nothing;
* capture blocks (`capture`, model_id 0; plan 4.2): category Neural Capture, index_evidence
  "capture-block", a printable name equal to `model`, source factory|user, not yet resolved,
  and only the capture block's own controls (GAIN..VOLUME, indexes 0-4 of model 14000, which
  14001 must list identically, with the same param_count; file_name, index 5, must sit below it). Which capture and which block (V1 14000 / V2 14001) is decided
  on the unit, from a fresh directory listing, by qc_agent.captures;
* `stomps` (design doc qc-stomp-layout.md, S2): at most 32 `{row, col, switch}` entries,
  row 0, a placed column, switch A-G (never H), one switch per block. The push assigns them
  (latching, primary only) after the values read back clean and before the save, then reads
  them back (jobs.py, entry point 1, owner decision #147). Which physical switch an index
  lights is UNVERIFIED on the unit (stomp doc, G7 a): STOMP_INDEX is qc-mcp's 0-7 = A-H.

Symbolic bounds: the cab bounds confirmed on the unit (G4) and the pyquadcortex 4.0.1 values
(owner decision, plan 4.1: allowed, labelled) are resolved; any other symbol, including the
unmeasured input trim, is refused. Both tables are checked against firmware_constants.go.
"""
from __future__ import annotations

import math
import re
from dataclasses import dataclass, field

from . import text

FORMAT = "qc2-device-export/1"
SUPPORTED_FORMATS = frozenset({FORMAT})
MAX_BLOCKS = 8
MAX_PARAMS_PER_BLOCK = 64
SCENES = 8
SCENE_SOURCES = ["A", "B"] + ["A"] * (SCENES - 2)  # G6: C-H are written explicitly from Scene A
MAX_STOMPS = 32
STOMP_SWITCHES = "ABCDEFG"  # H is the owner's manual switch and is never assigned
# Switch letter -> the transport's stomp_index. qc-mcp documents 0-7 = A-H (vendor/qc_mcp/transport.py:1174, 
# assign_stomp's docstring); UNVERIFIED on the unit until the owner checks the first push by eye (README step 16).
# H (7) has no entry, so it can never be planned.
STOMP_INDEX = {s: i for i, s in enumerate(STOMP_SWITCHES)}
VERIFIED_CLONE_BASES = frozenset({12000})  # catalog.VerifiedCloneBases: the one base G1 checked
# catalog plugin_id -> the plugin's short name, kept in step with internal/catalog/plugins.go (a test compares them).
# A plugin model is placed only when its block names exactly this plugin: an id missing here is refused.
PLUGIN_NAMES = {"1": "Nameless", "3": "Plini", "4": "Parallax", "5": "Nolly", "9": "Cory Wong", "10": "Gojira",
                "11": "SLO-100", "12": "Tim Henson", "13": "Petrucci", "15": "Rabea", "20": "Misha Mansoor",
                "21": "John Mayer"}
CAPTURE_LAYOUT_ID = 14000                 # the controls a capture block exports (catalog model 14000)
CAPTURE_BLOCK_IDS = {1: 14000, 2: 14001}  # V1 / V2 Neural Capture blocks (qc-mcp transport.CAPTURE_V1/V2)
CAPTURE_SOURCES = frozenset({"factory", "user"})
CAPTURE_FILE_NAME_INDEX = 5               # file_name (qc-mcp transport.CAPTURE_FILE_NAME_PARAM; captures.FILE_NAME_PARAM)
_CAPTURE_NAME_RE = re.compile(r"[\x20-\x7e]{1,64}")  # printable ASCII; leading/trailing space refused separately
# Symbolic parameter bounds confirmed on the owner's unit (CorOS 4.1.0, plan 7.1 G4), the same
# two internal/catalog/firmware_constants.go marks EvidenceConfirmed410. Every other symbolic
# bound (pyquadcortex-only EQ/mixer/FX-loop/tempo values, the unmeasured input trim) stays
# refused, as in the exporter. tests/test_go_goldens.py checks this table against the Go source.
CONFIRMED_BOUNDS = {"MIN_CABSIM_DB": -40.0, "MAX_CABSIM_DB": 6.0}
# pyquadcortex 4.0.1 measurements (MIT, data only), not confirmed on the unit; the exporter uses
# them, labelled "bounds:pyquadcortex-4.0.1" (owner decision, plan 4.1). Pinned to Go by
# tests/test_go_goldens.py.
PYQUADCORTEX_BOUNDS = {
    "MIN_EQ_DB": -12.0, "MAX_EQ_DB": 12.0, "MIN_MIXER_DB": -40.0, "MAX_MIXER_DB": 12.0,
    "MIN_FXLOOP_OUT_GAIN_DB": -40.0, "MAX_FXLOOP_OUT_GAIN_DB": 0.0,
    "MIN_FXLOOP_IN_GAIN_DB": -40.0, "MAX_FXLOOP_IN_GAIN_DB": 12.0,
    "MIN_TEMPO": 40.0, "MAX_TEMPO": 240.0, "MIN_EQ_FREQ": 20.0, "MAX_EQ_FREQ": 20000.0,
}
# The evidence labels the exporter writes (internal/deviceexport, plan 4.1).
EVIDENCE = frozenset({"taper:g4", "taper:device-xml", "taper:pyquadcortex",
                      "bounds:g4", "bounds:pyquadcortex-4.0.1", "encoding:qc-mcp-linear",
                      "scale:catalog-range", "encoding:fader-linear-guess"})
# A fader is converted like a float on its catalog range, as a labelled GUESS (owner decision,
# 2026-11-06; plan 4.1): only a fader may carry FADER_EVIDENCE, and a fader must.
FADER_EVIDENCE = "encoding:fader-linear-guess"
LINEAR_SWITCH_TYPES = frozenset({"switch", "rotarySwitch", "toggleButton"})
WIRE_TOLERANCE = 1e-9
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
class PlannedStomp:
    """A validated stomp assignment: assigned by the push after the read-back, before the save."""
    row: int
    col: int
    switch: str

    @property
    def index(self) -> int:
        """The transport's stomp_index for this switch (0-6; H is never planned). UNVERIFIED mapping."""
        return STOMP_INDEX[self.switch]


@dataclass
class PlannedParam:
    index: int
    name: str
    wire: float
    scene_wires: list[float] | None  # all 8 scenes (A, B, then A for C-H) when B differs


@dataclass(frozen=True)
class PlannedMic:
    """A cab mic string to write exactly (plan 4.1/4.2, P4b): the cab's `ir selector` parameter for one
    mic slot. QC-2 only offers a string it saw on the unit for this same cab; the agent re-checks that it
    is a variation of the catalog's own default string for that slot, and the wire guard then holds the
    write to exactly this string, once. Whether the unit accepts it is UNVERIFIED on hardware."""
    slot: int
    index: int
    value: str


@dataclass(frozen=True)
class PlannedCapture:
    """A capture block's capture. key and version are filled in on the unit (qc_agent.captures)."""
    name: str
    source: str
    key: str | None = None
    version: int | None = None

    @property
    def file_name(self) -> str | None:
        return None if self.key is None else self.key + self.name


@dataclass
class PlannedBlock:
    col: int
    model_id: int              # 0 for a capture block until the unit's listing resolves it
    model: str
    params: list[PlannedParam]
    bypass_scenes: list[bool]  # 8 flags, A, B, then A for C-H; all eight are written on every block
    param_count: int           # the catalog's full list, hidden params included (G1: the unit reads param_count + 1)
    capture: PlannedCapture | None = None
    at_default: list[str] = field(default_factory=list)  # labels QC-2 says it left at default (logged only)
    mics: list[PlannedMic] = field(default_factory=list)  # cab mic strings, written before the block's values

    @property
    def bypass_varies(self) -> bool:
        return len(set(self.bypass_scenes)) > 1

    @property
    def bypass_index(self) -> int:
        """The block's bypass control: one past the unit's own read of the block (qc-mcp
        bypass_param_index = N, and G1 measured N = param_count + 1). The push checks N on the unit
        before any value is written, so this is never a guess at the moment it is used."""
        return self.param_count + 1


@dataclass
class Plan:
    row: int
    in_port: int
    out_port: int
    blocks: list[PlannedBlock]
    scene_labels: list[tuple[int, str]] = field(default_factory=list)  # none in the P2 export; not written in v1
    stomps: list[PlannedStomp] = field(default_factory=list)  # assigned after the read-back, before the save

    def op_count(self) -> int:
        # routing, then per block: place (or capture), params, bypass; then one write per stomp
        return 1 + sum(1 + len(b.params) + 1 for b in self.blocks) + len(self.stomps)


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


def _array(v, where, n=SCENES):
    if not isinstance(v, list) or len(v) != n:
        raise ExportRefused(f"{where} must be a list of exactly {n} values")
    return v


def _bounds(cp, where):
    out = []
    for key in ("min", "max"):
        sym, v = cp.get(key + "_symbol"), cp.get(key)
        if sym is not None:
            # A symbol the device confirmed, a labelled pyquadcortex value, or nothing.
            v = CONFIRMED_BOUNDS.get(sym, PYQUADCORTEX_BOUNDS.get(sym))
        if v is None or isinstance(v, bool) or not isinstance(v, (int, float)):
            raise ExportRefused(f"{where}: the catalog has no usable numeric range for this parameter "
                                f"(unconfirmed or missing bound {sym or key}); the agent can't check it")
        out.append(v)
    return out


def _check_value(cp, display, wire, where):
    lo, hi = _bounds(cp, where)
    d = _num(display, where + ".display")
    if not min(lo, hi) <= d <= max(lo, hi):
        raise ExportRefused(f"{where}: display {d:g} is outside {lo:g}..{hi:g}")
    if (cp["type"] in DISCRETE_TYPES or cp.get("step_names")) and not d.is_integer():
        raise ExportRefused(f"{where}: {cp['type']} position must be a whole number, got {d:g}")
    w = _num(wire, where + ".wire")
    if not 0.0 <= w <= 1.0:
        raise ExportRefused(f"{where}: wire {w:g} is outside 0..1")
    want = _expected_wire(cp, lo, hi, d)
    if abs(w - want) > WIRE_TOLERANCE:
        raise ExportRefused(f"{where}: wire {w!r} is not the catalog's mapping of display {d:g} ({want!r})")
    return w


def _expected_wire(cp, lo, hi, d):
    """The wire value the catalog gives for a display value: the taper for a float
    (catalog.Param.ToWire), the linear position encoding for a switch."""
    if not hi > lo:
        raise ExportRefused(f"{cp['name']}: empty range {lo:g}..{hi:g}")
    r = (d - lo) / (hi - lo)
    if cp["type"] in LINEAR_SWITCH_TYPES:
        return r
    if cp["type"] not in ("float", "fader"):  # fader: the float rule, labelled a guess
        raise ExportRefused(f"{cp['name']}: type {cp['type']!r} has no wire mapping the agent can check")
    skew = cp.get("skew", 1.0)
    if isinstance(skew, bool) or not isinstance(skew, (int, float)) or not skew > 0:
        raise ExportRefused(f"{cp['name']}: catalog skew {skew!r} is not a positive number")
    return r ** skew


def _check_param_evidence(pw, cp, ev):
    if (not isinstance(ev, list) or not 1 <= len(ev) <= 3 or any(not isinstance(e, str) for e in ev)
            or len(set(ev)) != len(ev)):
        raise ExportRefused(f"{pw}.evidence must be a short list of distinct labels")
    unknown = sorted(set(ev) - EVIDENCE)
    if unknown:
        raise ExportRefused(f"{pw}.evidence has unknown labels {unknown}")
    switch = cp["type"] in LINEAR_SWITCH_TYPES
    if switch != ("encoding:qc-mcp-linear" in ev):
        raise ExportRefused(f"{pw}.evidence {ev} doesn't match a {cp['type']} parameter")
    if (cp["type"] == "fader") != (FADER_EVIDENCE in ev):
        raise ExportRefused(f"{pw}.evidence {ev} doesn't match a {cp['type']} parameter "
                            f"({FADER_EVIDENCE} belongs on faders only, and every fader needs it)")
    symbolic = cp.get("min_symbol") is not None or cp.get("max_symbol") is not None
    if not switch and symbolic != any(e.startswith("bounds:") for e in ev):
        raise ExportRefused(f"{pw}.evidence {ev} doesn't match the catalog's bounds")


# catalog.IsInternalParam: not user controls, never in an export.
_INTERNAL_TYPES = frozenset({"string", "grMeter", "empty"})
_INTERNAL_NAMES = frozenset({"bypass", "phi", "ir selector"})


def _is_internal(p) -> bool:
    return p["type"] in _INTERNAL_TYPES or p["name"] in _INTERNAL_NAMES


def _expected_labels(cm) -> dict[int, str]:
    """index -> label, as catalog.LabelParams builds it: a name the model repeats is numbered
    by occurrence ("MIC 1 LEVEL", "MIC 2 LEVEL"; "IR n" for IR loaders)."""
    user = [p for p in cm["params"] if not _is_internal(p)]
    total: dict[str, int] = {}
    for p in user:
        total[p["name"]] = total.get(p["name"], 0) + 1
    prefix = "IR" if cm["category"] == "IRLoaders" else "MIC"
    seen: dict[str, int] = {}
    out = {}
    for p in user:
        seen[p["name"]] = seen.get(p["name"], 0) + 1
        out[p["index"]] = f"{prefix} {seen[p['name']]} {p['name']}" if total[p["name"]] > 1 else p["name"]
    return out


def is_internal_param(p) -> bool:
    """catalog.IsInternalParam: not a user control (bypass/phi/IR selector, strings, meters)."""
    return _is_internal(p)


def control_labels(cm) -> dict[int, str]:
    """index -> label for a catalog model's user controls (catalog.LabelParams)."""
    return _expected_labels(cm)


def default_wire(cp) -> float | None:
    """The catalog default of a control as the wire value an export would carry for it (the same
    mapping _check_value accepts, evidence labels included), or None when the agent can't compute one:
    no numeric default, a bound it can't resolve, a type with no wire mapping. For the defaults log
    only; a None is logged as not compared, never treated as a failure."""
    d = cp.get("default")
    if isinstance(d, bool) or not isinstance(d, (int, float)) or not math.isfinite(d):
        return None
    try:
        lo, hi = _bounds(cp, cp.get("name", "?"))
        if not min(lo, hi) <= d <= max(lo, hi):
            return None
        return _expected_wire(cp, lo, hi, float(d))
    except (ExportRefused, KeyError, TypeError, ValueError, ZeroDivisionError):
        return None


def _name_is_shared(catalog, cm) -> bool:
    return sum(1 for m in catalog.models.values() if m.get("name") == cm["name"]) > 1


def _check_evidence(where, cm, ev, catalog):
    if not isinstance(ev, str):
        raise ExportRefused(f"{where}.index_evidence must be a string")
    base = cm.get("clones")
    if base is None:
        want = "plain"
    elif base in VERIFIED_CLONE_BASES:
        want = f"clone-{base}-g1"
    else:
        raise ExportRefused(f"{where}: this model clones {base}, whose parameter layout was never checked "
                            f"against the device (only {sorted(VERIFIED_CLONE_BASES)} was)")
    # The exporter appends this when a name shared by models with different controls was resolved
    # by the block's own parameters (internal/deviceexport PickedByControls). Only meaningful for a
    # name more than one catalog model carries.
    if ev == want + "+picked-by-controls" and _name_is_shared(catalog, cm):
        return
    if ev != want:
        raise ExportRefused(f"{where}: index_evidence is {ev!r}, the catalog says {want!r}")


def _param_count(cm, where) -> int:
    """The model's param_count (catalog.Model.ParamCount). Every listed index must sit below it: an
    index at or past it would be the device's trailing slot or the bypass control (G1, plan 7.2)."""
    pc = cm.get("param_count")
    if isinstance(pc, bool) or not isinstance(pc, int) or pc < 1:
        raise ExportRefused(f"{where}: the catalog has no param_count for model {cm.get('id')}; the bridge "
                            "can't check the unit's layout of this block")
    if max(cm["params_by_index"], default=-1) >= pc:
        raise ExportRefused(f"{where}: the catalog lists an index at or past param_count {pc} for model "
                            f"{cm.get('id')} (catalog fault)")
    return pc


def _at_default(b, where) -> list[str]:
    ad = b["at_default"]
    if (not isinstance(ad, list) or len(ad) > MAX_PARAMS_PER_BLOCK
            or any(not isinstance(x, str) or len(x) > 80 for x in ad)):
        raise ExportRefused(f"{where}.at_default must be a short list of control names")
    return list(ad)


def _check_block(bi, b, nblocks, catalog) -> PlannedBlock:
    where = f"block {bi}"
    _exact(b, ("col", "model_id", "model", "category", "index_evidence", "bypass", "params", "at_default"),
           where, optional=("plugin", "capture", "mics"))
    if _int(b["col"], where + ".col") != bi:
        raise ExportRefused(f"{where}: columns must run 0..{nblocks - 1} in order")
    if "capture" in b:
        if "mics" in b:
            raise ExportRefused(f"{where}: a capture block has no cab mics")
        return _check_capture_block(bi, b, catalog)
    mid = _int(b["model_id"], where + ".model_id")
    if mid in DENY_MODEL_IDS:
        raise ExportRefused(f"{where}: model {mid} is denied (NC_Recorder)")
    cm = catalog.models.get(mid)
    if cm is None:
        raise ExportRefused(f"{where}: model {mid} is not in this PC's device catalog")
    if b["model"] != cm["name"]:
        raise ExportRefused(f"{where}: model {mid} is {cm['name']!r} in the catalog, not {b['model']!r}")
    where = f"block {bi} ({cm['name']})"
    pid = cm.get("plugin_id")
    if pid is None:
        if "plugin" in b:
            raise ExportRefused(f"{where}: the export names a plugin the catalog does not")
    else:
        # F3: QC-2 exports a plugin model only for a plugin the owner has (config.json available_plugins) and
        # names it in the block. The agent holds no ownership list: it checks that the block names the plugin
        # this PC's catalog says the model belongs to, and refuses an id it doesn't know. Whether the unit
        # accepts a plugin it lacks is UNVERIFIED; the grid read-back (model ids per column) fails the push
        # if the block doesn't land, and nothing is saved then.
        want = PLUGIN_NAMES.get(str(pid))
        if want is None:
            raise ExportRefused(f"{where}: plugin id {pid!r} is not one this agent knows")
        if b.get("plugin") != want:
            raise ExportRefused(f"{where}: this is a {want} plugin model, but the export names "
                                f"{text.clean(str(b.get('plugin')), 40)!r}")
    if cm["category"] not in ALLOWED_CATEGORIES:
        raise ExportRefused(f"{where}: category {cm['category']!r} is not pushed in v1")
    if b["category"] != cm["category"]:
        raise ExportRefused(f"{where}: category is {cm['category']!r} in the catalog, not {b['category']!r}")
    _check_evidence(where, cm, b["index_evidence"], catalog)
    pc = _param_count(cm, where)
    ad = _at_default(b, where)

    if not isinstance(b["params"], list) or len(b["params"]) > MAX_PARAMS_PER_BLOCK:
        raise ExportRefused(f"{where}.params must be a list of at most {MAX_PARAMS_PER_BLOCK}")
    labels = _expected_labels(cm)
    params, seen = [], set()
    for pi, prm in enumerate(b["params"]):
        p = _check_param(f"{where} param {pi}", prm, cm, labels, pc)
        if p.index in seen:
            raise ExportRefused(f"{where} param {pi}: index {p.index} appears twice")
        seen.add(p.index)
        params.append(p)

    flags = _array(b["bypass"], where + ".bypass")
    if any(not isinstance(f, bool) for f in flags):
        raise ExportRefused(f"{where}.bypass flags must be true/false")
    if any(flags[i] != flags[0] for i in range(2, SCENES)):
        raise ExportRefused(f"{where}.bypass: scenes C-H must carry Scene A's flag")
    return PlannedBlock(bi, mid, cm["name"], params, list(flags), pc, at_default=ad,
                        mics=_check_mics(where, b.get("mics", []), cm))


MIC_EVIDENCE = "seen-on-unit"
_MIC_PART_RE = re.compile(r"[A-Za-z]{3,12} [0-9]{1,4}")  # every mic name in ModelRepo.xml; see devicelib.micPartRe
MAX_MIC_VALUE = 128


def _cab_part(s):
    i = s.rfind("_")
    return s[:i] if i > 0 else ""


def _check_mics(where, mics, cm) -> list[PlannedMic]:
    """Cab mic string writes. Only a cab's own `ir selector` slots, at the catalog's index for that slot,
    and only a string that keeps the catalog's default string's `<prefix>_<cab>` and changes the mic:
    anything else (another IR, a path, a different cab) is refused here, whatever QC-2 says it saw."""
    if not isinstance(mics, list) or len(mics) > 2:
        raise ExportRefused(f"{where}.mics must be a list of at most two mic writes")
    if not mics:
        return []
    if not cm["category"].startswith("Cabsim"):
        raise ExportRefused(f"{where}: only a cab has mics to write")
    selectors = [p for p in cm["params"] if p["name"] == "ir selector"]
    out, slots = [], set()
    for i, m in enumerate(mics):
        w = f"{where} mics[{i}]"
        _exact(m, ("slot", "index", "value", "evidence"), w)
        slot, index = _int(m["slot"], w + ".slot"), _int(m["index"], w + ".index")
        if not 1 <= slot <= len(selectors) or slot in slots:
            raise ExportRefused(f"{w}: slot {slot} is not one of this cab's mic slots, or appears twice")
        slots.add(slot)
        sel = selectors[slot - 1]
        if index != sel["index"] or not 0 <= index < cm["param_count"]:
            raise ExportRefused(f"{w}: index {index} is not the catalog's ir selector for mic slot {slot} ({sel['index']})")
        if m["evidence"] != MIC_EVIDENCE:
            raise ExportRefused(f"{w}.evidence must be {MIC_EVIDENCE!r}")
        v = _printable(m["value"], MAX_MIC_VALUE, w + ".value")
        default = sel.get("default_string") or ""
        if (not v or v != v.strip() or "`" in v or not _cab_part(default) or _cab_part(v) != _cab_part(default)
                or not _MIC_PART_RE.fullmatch(v[len(_cab_part(v)) + 1:])):
            raise ExportRefused(f"{w}: {v[:40]!r} is not a mic of this cab (it must keep {_cab_part(default)!r} and "
                                "change only the mic)")
        out.append(PlannedMic(slot, index, v))
    return out


def _check_capture_layouts(catalog) -> int:
    """The V1 and V2 capture blocks must list the same user controls and the same param_count, or a
    capture block's params (checked against 14000) might not mean the same on a 14001 block, and the
    layout check after placement would expect the wrong count. Returns that param_count."""
    def controls(mid):
        m = catalog.models.get(mid)
        if m is None:
            return None
        keys = ("index", "name", "type", "min", "max", "skew", "units")
        return [tuple(p.get(k) for k in keys) for p in m["params"] if not _is_internal(p)]
    v1, v2 = controls(CAPTURE_BLOCK_IDS[1]), controls(CAPTURE_BLOCK_IDS[2])
    if v1 is None or v1 != v2:
        raise ExportRefused("the catalog's V1 (14000) and V2 (14001) capture blocks don't list the same controls; "
                            "the bridge can't check this capture block")
    m1, m2 = catalog.models[CAPTURE_BLOCK_IDS[1]], catalog.models[CAPTURE_BLOCK_IDS[2]]
    pc = _param_count(m1, "capture block 14000")
    if _param_count(m2, "capture block 14001") != pc:
        raise ExportRefused("the catalog's V1 (14000) and V2 (14001) capture blocks have different param_count; "
                            "the bridge can't check this capture block")
    if not CAPTURE_FILE_NAME_INDEX < pc:
        raise ExportRefused(f"the catalog's capture block param_count {pc} leaves no room for file_name at index "
                            f"{CAPTURE_FILE_NAME_INDEX}")
    return pc


def _check_capture_block(bi, b, catalog) -> PlannedBlock:
    where = f"block {bi}"
    if _int(b["model_id"], where + ".model_id") != 0:
        raise ExportRefused(f"{where}: a capture block must have model_id 0 (the bridge picks 14000 or 14001 on the unit)")
    if "plugin" in b:
        raise ExportRefused(f"{where}: a capture block can't name a plugin")
    if b["category"] != "Neural Capture":
        raise ExportRefused(f"{where}: a capture block's category must be 'Neural Capture'")
    if b["index_evidence"] != "capture-block":
        raise ExportRefused(f"{where}: a capture block's index_evidence must be 'capture-block'")
    c = b["capture"]
    _exact(c, ("name", "source", "resolved_on_unit"), where + ".capture")
    if not isinstance(c["source"], str) or c["source"] not in CAPTURE_SOURCES:
        raise ExportRefused(f"{where}.capture.source must be 'factory' or 'user'")
    if c["resolved_on_unit"] is not False:
        raise ExportRefused(f"{where}.capture.resolved_on_unit must be false: this bridge resolves captures on the "
                            "unit itself")
    name = c["name"]
    if not isinstance(name, str) or not _CAPTURE_NAME_RE.fullmatch(name) or name != name.strip():
        raise ExportRefused(f"{where}.capture.name must be 1-64 printable ASCII characters with no leading or "
                            "trailing space")
    if b["model"] != name:
        raise ExportRefused(f"{where}: model must equal capture.name")
    pc = _check_capture_layouts(catalog)
    cm = catalog.models[CAPTURE_LAYOUT_ID]
    where = f"block {bi} (capture {name!r})"
    ad = _at_default(b, where)
    if not isinstance(b["params"], list) or len(b["params"]) > MAX_PARAMS_PER_BLOCK:
        raise ExportRefused(f"{where}.params must be a list of at most {MAX_PARAMS_PER_BLOCK}")
    labels = _expected_labels(cm)
    params, seen = [], set()
    for pi, prm in enumerate(b["params"]):
        p = _check_param(f"{where} param {pi}", prm, cm, labels, pc)
        if p.index in seen:
            raise ExportRefused(f"{where} param {pi}: index {p.index} appears twice")
        seen.add(p.index)
        params.append(p)
    flags = _array(b["bypass"], where + ".bypass")
    if any(not isinstance(f, bool) for f in flags):
        raise ExportRefused(f"{where}.bypass flags must be true/false")
    if any(flags[i] != flags[0] for i in range(2, SCENES)):
        raise ExportRefused(f"{where}.bypass: scenes C-H must carry Scene A's flag")
    return PlannedBlock(bi, 0, name, params, list(flags), pc, PlannedCapture(name, c["source"]), at_default=ad)


def _check_param(pw, prm, cm, labels, param_count) -> PlannedParam:
    _exact(prm, ("index", "label", "scene_mode", "display", "wire", "evidence"), pw, optional=("units",))
    idx = _int(prm["index"], pw + ".index")
    if not 0 <= idx < param_count:
        raise ExportRefused(f"{pw}: index {idx} is at or past param_count {param_count} (the device's trailing "
                            "slot and bypass control are never written as values)")
    cp = cm["params_by_index"].get(idx)
    if cp is None:
        raise ExportRefused(f"{pw}: index {idx} is not a listed parameter of this model")
    if idx not in labels:
        raise ExportRefused(f"{pw}: index {idx} ({cp['name']!r}) is not a user-facing control")
    if prm["label"] != labels[idx]:
        raise ExportRefused(f"{pw}: index {idx} is {labels[idx]!r} in the catalog, not {prm['label']!r}")
    pw = f"{pw.rsplit(' param ', 1)[0]} {labels[idx]} [{idx}]"
    if "units" in prm and (not isinstance(prm["units"], str) or len(prm["units"]) > 16):
        raise ExportRefused(f"{pw}.units must be a short string")
    if cp["type"] not in WRITABLE_TYPES:
        raise ExportRefused(f"{pw}: type {cp['type']!r} is not writable by the agent")
    _check_param_evidence(pw, cp, prm["evidence"])
    if not isinstance(prm["scene_mode"], bool):
        raise ExportRefused(f"{pw}.scene_mode must be true/false")
    disp = _array(prm["display"], pw + ".display")
    wire = _array(prm["wire"], pw + ".wire")
    wires = [_check_value(cp, d, w, f"{pw} scene {'ABCDEFGH'[i]}") for i, (d, w) in enumerate(zip(disp, wire))]
    dnum = [float(d) for d in disp]
    if any(dnum[i] != dnum[0] or wires[i] != wires[0] for i in range(2, SCENES)):
        raise ExportRefused(f"{pw}: scenes C-H must carry Scene A's values")
    differs = dnum[0] != dnum[1]
    if prm["scene_mode"] != differs:
        raise ExportRefused(f"{pw}: scene_mode is {prm['scene_mode']} but Scenes A and B "
                            f"{'differ' if differs else 'are equal'}")
    if not differs and wires[1] != wires[0]:
        raise ExportRefused(f"{pw}: Scenes A and B have equal display values but different wire values")
    return PlannedParam(idx, cp["name"], wires[0], wires if differs else None)


def _check_stomps(stomps, blocks) -> list[PlannedStomp]:
    if not isinstance(stomps, list) or len(stomps) > MAX_STOMPS:
        raise ExportRefused(f"payload.stomps must be a list of at most {MAX_STOMPS}")
    placed = {b.col for b in blocks}
    out, on_cols = [], set()
    for i, s in enumerate(stomps):
        where = f"stomps[{i}]"
        _exact(s, ("row", "col", "switch"), where)
        row, col = _int(s["row"], where + ".row"), _int(s["col"], where + ".col")
        sw = s["switch"]
        if row != ROW:
            raise ExportRefused(f"{where}.row must be {ROW}")
        if col not in placed:
            raise ExportRefused(f"{where}: no block is placed at column {col}")
        if not isinstance(sw, str) or len(sw) != 1 or sw not in STOMP_SWITCHES:
            raise ExportRefused(f"{where}.switch must be one of {', '.join(STOMP_SWITCHES)} (H is never assigned)")
        if col in on_cols:
            raise ExportRefused(f"{where}: the block at column {col} already has a switch (one primary assignment)")
        on_cols.add(col)
        out.append(PlannedStomp(row, col, sw))
    return out


def validate(export: dict, catalog, expected_firmware: str) -> Plan:
    """`export` is the job's args.export object ({"format", "payload"}). Returns the
    plan to execute, or raises ExportRefused naming the first problem."""
    _exact(export, ("format", "payload"), "export")
    fmt = export["format"]
    if fmt not in SUPPORTED_FORMATS:
        raise ExportRefused(f"unsupported export format {fmt!r}; this agent implements {FORMAT}")
    p = export["payload"]
    _exact(p, ("format", "catalog", "guitar", "scene_sources", "rows", "stomps", "unresolved", "warnings"), "payload")
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

    # The guitar's name comes from the preset and is never written to the unit; bound it only.
    if not isinstance(p["guitar"], str) or not 0 < len(p["guitar"]) <= 200:
        raise ExportRefused("payload.guitar must be a non-empty string of at most 200 characters")
    if p["scene_sources"] != SCENE_SOURCES:
        raise ExportRefused(f"payload.scene_sources must be exactly {SCENE_SOURCES}")

    if not isinstance(p["unresolved"], list):
        raise ExportRefused("payload.unresolved must be a list")
    if p["unresolved"]:
        raise ExportRefused(f"the export has {len(p['unresolved'])} unresolved item(s); a push needs zero")

    # Warnings are what QC-2 leaves out on purpose (the user saw them in the dry run); they
    # carry no operation. Bound them only.
    w = p["warnings"]
    if not isinstance(w, list) or len(w) > 64:
        raise ExportRefused("payload.warnings must be a list of at most 64 items")
    for i, item in enumerate(w):
        _exact(item, ("block", "reason"), f"warnings[{i}]", optional=("param",))
        if any(not isinstance(v, str) or len(v) > 400 for v in item.values()):
            raise ExportRefused(f"warnings[{i}] must hold short strings")

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
    stomps = _check_stomps(p["stomps"], planned)
    return Plan(ROW, IN_PORT, OUT_PORT, planned, [], stomps)
