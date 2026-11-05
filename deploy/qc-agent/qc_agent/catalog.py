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
from pathlib import Path

DEFAULT_PATH = Path(__file__).resolve().parents[3] / "internal" / "catalog" / "qc_catalog.json"
MAX_CATALOG_BYTES = 16 * 1024 * 1024


class CatalogError(RuntimeError):
    pass


class Catalog:
    def __init__(self, models: dict[int, dict], sha256: str, path: Path | None = None):
        self.models = models
        self.sha256 = sha256
        self.path = path

    @classmethod
    def load(cls, path: Path = DEFAULT_PATH) -> "Catalog":
        try:
            raw = path.read_bytes()
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
        except (ValueError, KeyError, TypeError, UnicodeDecodeError) as e:
            raise CatalogError(f"device catalog is malformed: {e}") from None
        return cls(models, hashlib.sha256(normalised).hexdigest(), path)
