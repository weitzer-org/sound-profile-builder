"""Find the captures a push names on the unit itself (plan section 4.2).

Pure: it works on a DirectorySnapshot the backend builds from one fresh directory
listing, and never touches the device. Everything in a snapshot came from the unit,
so it is untrusted text: names are only ever compared for exact equality, and what
reaches a message or log goes through text.clean().

Rules (all fail closed: a capture the bridge can't pin down refuses the job before
anything is written):
* Match by exact name (byte-exact, case-sensitive) among the unit's capture files,
  favorites folders excluded. "CA John's" and "CA John's 2" are different captures.
* The export's source must agree: a "factory" capture must be in a factory folder,
  a "user" capture must not be.
* A name found only in cloud (or plugin) folders isn't a file on the unit; the bridge
  never fetches captures. A folder counts as cloud, favorites or plugin when it or any
  folder above it (FolderInfo.parent_key) carries that flag: the flags' exact meaning on
  CorOS 4.1 is unverified, so they are inherited rather than trusted per folder.
* Exactly one key: the same key in several folders counts once; two keys with the
  same name is ambiguous.
* The version picks the block (V1 -> 14000, V2 -> 14001). A factory capture's
  version comes from its folder name ("Factory Captures V1" / "... V2", from qc-mcp's
  DIRECTORY.md on its author's unit; UNVERIFIED on the owner's: if the names differ,
  every factory capture is refused and the message lists the unit's real folder names). A user capture's comes from ProductData.version, calibrated on
  every listing against the factory folders: only when every V1 factory entry
  carries one value and every V2 entry a different one is that value taken to mean
  V1/V2 for user files too. Otherwise the user capture is refused. That the field
  means the same for user files is an inference P6 checks.
"""
from __future__ import annotations

import re
from dataclasses import dataclass, field, replace

from . import text

FACTORY_FOLDERS = {"Factory Captures V1": 1, "Factory Captures V2": 2}
FILE_NAME_PARAM = 5  # a capture block's file_name index (qc-mcp transport.CAPTURE_FILE_NAME_PARAM)
_KEY_RE = re.compile(r"[0-9a-fA-F]{64}")
MAX_NEAR = 3


@dataclass(frozen=True)
class CaptureEntry:
    name: str
    key: str
    folder_key: str
    folder_name: str
    is_factory: bool
    is_cloud: bool       # the folder or one above it is a cloud folder (or a plugin folder)
    is_favorites: bool   # the folder or one above it is a favorites view
    version_field: str


@dataclass
class DirectorySnapshot:
    my_presets: dict[int, str]
    preset_deps: dict[int, frozenset[str]] = field(default_factory=dict)  # position -> dependency hashes (lower case)
    captures: list[CaptureEntry] = field(default_factory=list)
    capture_folders: list[tuple[str, bool, int]] = field(default_factory=list)  # (name, is_factory, file count)
    _calibration: dict | None = field(default=None, repr=False, compare=False)


@dataclass(frozen=True)
class ResolvedCapture:
    name: str
    source: str
    key: str
    version: int  # 1 or 2

    @property
    def file_name(self) -> str:
        return self.key + self.name


class CaptureRefused(Exception):
    """A capture the bridge won't place. The message is safe to show the user."""


def _calibrate(snap: DirectorySnapshot) -> dict[str, int] | None:
    """ProductData.version value -> 1 or 2, learned from the factory folders, or None."""
    if snap._calibration is not None:
        return snap._calibration or None
    seen: dict[int, set[str]] = {1: set(), 2: set()}
    for e in snap.captures:
        v = FACTORY_FOLDERS.get(e.folder_name)
        if v and e.is_factory:
            seen[v].add(e.version_field)
    out: dict[str, int] = {}
    if (len(seen[1]) == 1 and len(seen[2]) == 1 and seen[1] != seen[2]
            and "" not in seen[1] and "" not in seen[2]):
        out = {next(iter(seen[1])): 1, next(iter(seen[2])): 2}
    snap._calibration = out
    return out or None


def version_of(e: CaptureEntry, snap: DirectorySnapshot) -> int | None:
    if e.is_factory:
        return FACTORY_FOLDERS.get(e.folder_name)
    cal = _calibrate(snap)
    return cal.get(e.version_field) if cal else None


def _norm(s: str) -> str:
    return " ".join(s.casefold().split())


def _near(name: str, snap: DirectorySnapshot) -> str:
    want = _norm(name)
    hits = sorted({e.name for e in snap.captures if e.name != name and _norm(e.name) == want})[:MAX_NEAR]
    if not hits:
        return ""
    return " Close names on the unit: " + ", ".join(repr(text.clean(h, 40)) for h in hits) + "."


def _folders(snap: DirectorySnapshot) -> str:
    names = sorted({text.clean(n, 40) for n, _f, _c in snap.capture_folders})
    return ", ".join(repr(n) for n in names[:6]) + (" ..." if len(names) > 6 else "")


def resolve_one(name: str, source: str, col: int, snap: DirectorySnapshot) -> ResolvedCapture:
    q = repr(text.clean(name, 64))
    cands = [e for e in snap.captures if e.name == name and not e.is_favorites]
    if not cands:
        raise CaptureRefused(f"Capture {q} (column {col}) isn't on the unit: no capture has exactly that name. "
                             "Download it in Cortex Control, or refine the preset to use a capture you have. "
                             "Nothing was written." + _near(name, snap))
    want_factory = source == "factory"
    same = [e for e in cands if e.is_factory == want_factory]
    if not same:
        other = "user" if want_factory else "factory"
        raise CaptureRefused(f"Capture {q} is a {source} capture in QC-2's library, but the unit only has it as a "
                             f"{other} capture. Nothing was written.")
    on_unit = [e for e in same if not e.is_cloud]
    if not on_unit:
        raise CaptureRefused(f"Capture {q} is listed only as a Cortex Cloud (or plugin) item, not as a file on the unit. "
                             "Download it to the unit in Cortex Control first; the bridge never fetches captures. "
                             "Nothing was written.")
    by_key: dict[str, list[CaptureEntry]] = {}
    for e in on_unit:
        by_key.setdefault(e.key.lower(), []).append(e)
    if len(by_key) > 1:
        folders = sorted({repr(text.clean(e.folder_name, 40)) for e in on_unit})
        where = ", ".join(folders[:6]) + (" ..." if len(folders) > 6 else "")
        raise CaptureRefused(f"Capture {q} (column {col}) matches {len(by_key)} different captures on the unit "
                             f"(in {where}); the bridge won't guess which one. Nothing was written.")
    entries = next(iter(by_key.values()))
    key = entries[0].key
    if not _KEY_RE.fullmatch(key):
        raise CaptureRefused(f"Capture {q} is listed on the unit with a key the bridge can't use. Nothing was written.")
    versions = {version_of(e, snap) for e in entries}
    if want_factory and None in versions:
        bad = next(e for e in entries if version_of(e, snap) is None)
        raise CaptureRefused(f"Capture {q} is in folder {text.clean(bad.folder_name, 40)!r}, which the bridge doesn't "
                             f"know as V1 or V2 (it knows {', '.join(repr(k) for k in FACTORY_FOLDERS)}; the unit's "
                             f"capture folders are: {_folders(snap)}). Nothing was written.")
    if None in versions or len(versions) != 1:
        raise CaptureRefused(f"The bridge can't tell whether capture {q} is a V1 or V2 capture on this unit (the "
                             "factory library gives no consistent version marker to compare it with), so it won't "
                             "pick a block type. Nothing was written.")
    return ResolvedCapture(name=name, source=source, key=key, version=versions.pop())


def resolve(blocks, snap: DirectorySnapshot) -> dict[int, ResolvedCapture]:
    """{col: ResolvedCapture} for every capture block of the plan, or raises CaptureRefused."""
    out = {}
    for b in blocks:
        if b.capture is not None:
            out[b.col] = resolve_one(b.capture.name, b.capture.source, b.col, snap)
    return out


def with_resolution(plan, resolved: dict[int, ResolvedCapture], block_ids: dict[int, int]):
    """The plan with each capture block's model id set to its V1/V2 block and its capture filled in."""
    blocks = []
    for b in plan.blocks:
        rc = resolved.get(b.col)
        if rc is None:
            blocks.append(b)
        else:
            blocks.append(replace(b, model_id=block_ids[rc.version],
                                  capture=replace(b.capture, key=rc.key, version=rc.version)))
    return replace(plan, blocks=blocks)


def deps_report(snap: DirectorySnapshot, pos: int, resolved: dict[int, ResolvedCapture]) -> str:
    """The sentence the push result adds about the saved preset's dependency list."""
    if not resolved:
        return ""
    deps = snap.preset_deps.get(pos)
    if deps is None or not any(snap.preset_deps.values()):
        # Whether the listing carries dependency lists at all, and whether a downloaded capture's
        # dependency hash is its key, is unverified: with no lists anywhere there is nothing to compare.
        return (" The unit's listing shows no dependency lists, so whether the saved preset records its captures "
                "wasn't checked; P6 checks that it still loads after a power cycle.")
    missing = sorted({rc.name for rc in resolved.values() if rc.key.lower() not in deps})
    if not missing:
        return " The unit's listing names every placed capture as a dependency of the saved preset."
    names = ", ".join(repr(text.clean(n, 40)) for n in missing)
    return (f" Observation: the unit's listing doesn't name {names} among the saved preset's dependencies (by key); "
            "check that it still loads after a power cycle.")


def summary(resolved: dict[int, ResolvedCapture]) -> str:
    if not resolved:
        return ""
    items = sorted(resolved.items())
    parts = [f"col {c} {text.clean(rc.name, 32)!r} V{rc.version} ({rc.source})" for c, rc in items[:4]]
    more = f"; +{len(items) - 4} more" if len(items) > 4 else ""  # keeps the result message under its 500 chars
    return " Captures: " + "; ".join(parts) + more + "."
