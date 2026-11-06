"""The agent's own copy of the device catalog, used to re-validate every push.

The source is internal/catalog/qc_catalog.json in the owner's clone of this repo
(the same file QC-2 builds exports from; derived from the unit's ModelRepo.xml by
cmd/import_catalog). Phase 0 gate G1 showed our catalog's parameter indexes, not
qc-mcp's unexpanded ModelRepo.xml reading, match the device for the cloned cab it
tested, so the agent validates against this file and not against qc-mcp's XML.

Its hash is compared with the export's `catalog.sha256`, so a push built from a
different catalog than the one on the PC is refused. The hash is taken over the
file with CRLF line endings normalised to LF: a Windows checkout with
core.autocrlf would otherwise change the bytes, and a JSON file has no raw CR
bytes except line endings (CR inside a string must be escaped as \\r).
"""
from __future__ import annotations

import hashlib
import json
import math
from pathlib import Path

DEFAULT_PATH = Path(__file__).resolve().parents[3] / "internal" / "catalog" / "qc_catalog.json"
MAX_CATALOG_BYTES = 16 * 1024 * 1024


class CatalogError(RuntimeError):
    pass


# ---- what the unit confirmed (mirrors internal/catalog/firmware_constants.go) ----
#
# Only these mappings were read on the owner's unit (CorOS 4.1.0, plan 7.1). The agent recomputes
# display -> wire for them and refuses everything else, exactly as the P2 export does
# (internal/deviceexport/resolve.go taperRefusal / parseFloatValue). tests/test_catalog.py parses the Go
# file and fails if these drift from it.
CONFIRMED_SKEWS = frozenset({1.0, 0.3, 3.8018, 4.9594844})       # catalog.ConfirmedSkews (G4)
CONFIRMED_BOUNDS = {"MIN_CABSIM_DB": -40.0, "MAX_CABSIM_DB": 6.0}  # FirmwareBounds with EvidenceConfirmed410
VERIFIED_CLONE_BASES = frozenset({12000})                          # catalog.VerifiedCloneBases (G1)
INTERNAL_MODEL_IDS = frozenset({20000, 20001, 20002})              # deviceexport.internalModelIDs

# Catalog params that aren't user controls (catalog.IsInternalParam): per-slot bypass/phase
# switches, the IR file index, read-only meters and placeholders.
_INTERNAL_PARAM_NAMES = frozenset({"bypass", "phi", "ir selector"})
_INTERNAL_PARAM_TYPES = frozenset({"string", "grMeter", "empty"})


def is_internal_param(p: dict) -> bool:
    return p.get("type") in _INTERNAL_PARAM_TYPES or p.get("name") in _INTERNAL_PARAM_NAMES


def labels(model: dict) -> dict[int, str]:
    """{index: label} for a model's user controls: a port of catalog.LabelParams.

    A name that repeats among the controls (one LEVEL/PAN/... set per mic on dual-mic cabs) is
    labelled "MIC 1 LEVEL", "MIC 2 LEVEL" (or "IR n ..." on IR loaders), in index order. These are
    the labels P2 writes into the export and the dry run shows the owner."""
    ctrl = [p for p in sorted(model["params"], key=lambda p: p["index"]) if not is_internal_param(p)]
    total: dict[str, int] = {}
    for p in ctrl:
        total[p["name"]] = total.get(p["name"], 0) + 1
    prefix = "IR" if model.get("category") == "IRLoaders" else "MIC"
    seen: dict[str, int] = {}
    out = {}
    for p in ctrl:
        seen[p["name"]] = seen.get(p["name"], 0) + 1
        out[p["index"]] = (f"{prefix} {seen[p['name']]} {p['name']}" if total[p["name"]] > 1 else p["name"])
    return out


def is_pan(p: dict) -> bool:
    return p.get("min_label") in ("L", "R") or p.get("max_label") in ("L", "R")


def taper_refusal(p: dict) -> str | None:
    """Why this control's display -> wire mapping isn't confirmed on the unit, or None if it is.

    Mirrors P2: only `float` controls; no LOG_SKEW; skew among CONFIRMED_SKEWS; bounds numeric or
    confirmed; pans and unitless 0..1 controls have no verified display scale."""
    if p.get("type") != "float":
        return f"type {p.get('type')!r} has no measured wire mapping"
    if p.get("skew_symbol") == "LOG_SKEW":
        return "LOG_SKEW taper is not confirmed on CorOS 4.1.0"
    skew = p.get("skew", 1.0)
    if isinstance(skew, bool) or not isinstance(skew, (int, float)) or float(skew) not in CONFIRMED_SKEWS:
        return f"taper skew {skew} is not confirmed on CorOS 4.1.0"
    if is_pan(p):
        return "pan display scale is not verified on the device"
    lo, hi = bounds(p)
    if lo is None or hi is None:
        return "a bound of this control is symbolic and not confirmed on CorOS 4.1.0"
    if not lo < hi:
        return "the catalog range is empty or inverted"
    if not p.get("units") and (lo, hi) == (0.0, 1.0):
        return "unitless 0..1 control: its display scale is not verified on the device"
    return None


def bounds(p: dict) -> tuple[float | None, float | None]:
    """(lo, hi) as numbers: numeric catalog bounds, or a symbolic bound only when it is confirmed."""
    def one(num_key, sym_key):
        sym = p.get(sym_key)
        if sym:
            return CONFIRMED_BOUNDS.get(sym)
        v = p.get(num_key)
        if isinstance(v, bool) or not isinstance(v, (int, float)) or not math.isfinite(v):
            return None
        return float(v)
    return one("min", "min_symbol"), one("max", "max_symbol")


def to_wire(p: dict, display: float) -> float:
    """Display -> wire for a control taper_refusal() accepts (catalog Param.ToWire). Raises ValueError
    if the display value is outside the range (checked first: a negative base to a fractional power is
    a complex number in Python)."""
    why = taper_refusal(p)
    if why:
        raise ValueError(why)
    lo, hi = bounds(p)
    if not (math.isfinite(display) and lo <= display <= hi):
        raise ValueError(f"display {display:g} is outside {lo:g}..{hi:g}")
    return ((display - lo) / (hi - lo)) ** float(p.get("skew", 1.0))


def default_wire(p: dict) -> float | None:
    """The catalog default as a wire value, where the mapping is confirmed; else None."""
    d = p.get("default")
    if isinstance(d, bool) or not isinstance(d, (int, float)):
        return None
    try:
        return to_wire(p, float(d))
    except ValueError:
        return None


class Catalog:
    def __init__(self, models: dict[int, dict], sha256: str, path: Path | None = None, source: dict | None = None):
        self.models = models
        self.sha256 = sha256
        self.path = path
        self.source = source or {}

    @classmethod
    def load(cls, path: Path = DEFAULT_PATH) -> "Catalog":
        try:
            with open(path, "rb") as f:
                raw = f.read(MAX_CATALOG_BYTES + 1)
        except OSError as e:
            raise CatalogError(f"cannot read the device catalog at {path}: {e}") from None
        if len(raw) > MAX_CATALOG_BYTES:
            raise CatalogError(f"{path} is unexpectedly large")
        return cls.from_bytes(raw, path)

    @classmethod
    def from_bytes(cls, raw: bytes, path: Path | None = None) -> "Catalog":
        normalised = raw.replace(b"\r\n", b"\n")
        try:
            data = json.loads(normalised.decode("utf-8"))
            models = {}
            for m in data["models"]:
                mid = m["id"]
                if not isinstance(mid, int) or isinstance(mid, bool) or mid in models:
                    raise ValueError(f"bad or duplicate model id {mid!r}")
                params = {}
                for p in m["params"]:
                    idx = p["index"]
                    if not isinstance(idx, int) or isinstance(idx, bool) or idx in params:
                        raise ValueError(f"model {mid}: bad or duplicate param index {idx!r}")
                    params[idx] = p
                models[mid] = {**m, "params_by_index": params}
            source = data.get("source")
            if not isinstance(source, dict):
                raise ValueError("no source header")
        except (ValueError, KeyError, TypeError, UnicodeDecodeError) as e:
            raise CatalogError(f"device catalog is malformed: {e}") from None
        return cls(models, hashlib.sha256(normalised).hexdigest(), path, source)
