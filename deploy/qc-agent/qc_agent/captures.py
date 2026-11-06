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
  version comes from the nearest folder named "Factory Captures V1" / "... V2" among
  its own folder and the folders above it (FolderInfo.parent_key). The names are from
  qc-mcp's DIRECTORY.md on its author's unit. On the owner's unit (CorOS 4.1.0, P6,
  2026-11-06) the factory captures sit in per-device folders ("Brit 2210", "CA 3Axe",
  ...) and their ProductData.version is empty, so whether a V1/V2 folder sits above
  them is UNVERIFIED: if none does, every factory capture is refused and the census
  in the agent log shows the real tree. A user capture's version is its
  ProductData.version taken literally, "1" -> V1 and "2" -> V2, anything else
  refused (owner decision, 2026-11-06: the factory files carry no value to calibrate
  it against). A capture's author may control that field: a wrong value places the
  capture in the other block type in a slot this bridge created, which shows on the
  unit. UNVERIFIED until P6 compares the block type on the screen.
"""
from __future__ import annotations

import re
from dataclasses import dataclass, field, replace

from . import text
from .export_validator import CAPTURE_FILE_NAME_INDEX

FACTORY_FOLDERS = {"Factory Captures V1": 1, "Factory Captures V2": 2}
USER_VERSIONS = {"1": 1, "2": 2}  # ProductData.version of a user capture, taken literally (seen: "1" and "2")
FILE_NAME_PARAM = CAPTURE_FILE_NAME_INDEX  # a capture block's file_name index (qc-mcp transport.CAPTURE_FILE_NAME_PARAM)
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
    # V1/V2 from the nearest "Factory Captures V1/V2" folder at or above this file's folder (factory_version_of),
    # filled in by the backend from the listing's folder tree; None when there is none or both.
    factory_version: int | None = None


@dataclass
class DirectorySnapshot:
    my_presets: dict[int, str]
    preset_deps: dict[int, frozenset[str]] = field(default_factory=dict)  # position -> dependency hashes (lower case)
    captures: list[CaptureEntry] = field(default_factory=list)
    capture_folders: list[tuple[str, bool, int]] = field(default_factory=list)  # (name, is_factory, file count)
    # One dict per capture folder for the agent log's census (qcmcp_backend._log_census); never used to decide.
    folder_census: list[dict] = field(default_factory=list, repr=False, compare=False)


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


def factory_version_of(chain_names) -> int | None:
    """1 or 2 when the folder names from a file's own folder upwards name exactly one of FACTORY_FOLDERS, else None
    (none of them, or both)."""
    found = {FACTORY_FOLDERS[n] for n in chain_names if n in FACTORY_FOLDERS}
    return found.pop() if len(found) == 1 else None


def version_of(e: CaptureEntry, snap: DirectorySnapshot | None = None) -> int | None:
    """The block version a push uses for this file. `snap` is unused and kept so callers need not change."""
    if e.is_factory:
        return e.factory_version
    return USER_VERSIONS.get(e.version_field)


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
        raise CaptureRefused(f"Capture {q} is in folder {text.clean(bad.folder_name, 40)!r}, and no folder at or above "
                             f"it is one the bridge knows as V1 or V2 ({', '.join(repr(k) for k in FACTORY_FOLDERS)}; "
                             f"the unit's capture folders include: {_folders(snap)}). The agent log's capture census "
                             "shows the folder tree. Nothing was written.")
    if None in versions or len(versions) != 1:
        raise CaptureRefused(f"The bridge can't tell whether capture {q} is a V1 or V2 capture on this unit (its version "
                             "field isn't \"1\" or \"2\"), so it won't pick a block type. Nothing was written.")
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


LIBRARY_FORMAT = "qc-device-library/1"
MAX_LIBRARY_CAPTURES = 5000   # internal/devicejobs.MaxLibraryCaptures
_LIB_NAME_RE = re.compile(r"[\x20-\x7e]{1,64}")


class LibraryTooLarge(Exception):
    """More capture files than the contract carries. The sync fails rather than cut the list: a
    truncated list would make a capture that is on the unit look missing."""


UNNAMED_FOLDER = "(unnamed folder)"


def _library_folder(name: str) -> str:
    """A folder name for display: printable ASCII (anything else becomes '?'), clean, at most 40
    characters. Chosen to be a fixed point of QC-2's SanitizeText, which refuses what it would change.
    The folder is shown, never matched, so a name that cleans to nothing gets a placeholder rather than
    losing its captures: on the owner's unit the downloaded captures sit in a folder named '' (P6, 2026-11-06)."""
    ascii_only = "".join(c if 0x20 <= ord(c) <= 0x7E else "?" for c in text.clean(name, 400))
    return text.clean(ascii_only, 40).strip() or UNNAMED_FOLDER


SKIP_REASONS = ("favorites", "name_length", "name_charset", "name_cleaned", "key_format")


def skip_reason(e: CaptureEntry) -> str | None:
    """Why library_payload leaves this file out (the first reason that applies), or None when it is kept."""
    name, key = e.name or "", e.key or ""
    if e.is_favorites:
        return "favorites"
    if not 1 <= len(name) <= 64:
        return "name_length"
    if not _LIB_NAME_RE.fullmatch(name):
        return "name_charset"
    if name != text.clean(name, 64):
        return "name_cleaned"
    if not _KEY_RE.fullmatch(key):
        return "key_format"
    return None


def char_classes(s: str) -> str:
    """Length and character classes only, never the text itself (device text stays out of the log)."""
    printable = sum(1 for c in s if 0x20 <= ord(c) <= 0x7E)
    nonascii = sum(1 for c in s if ord(c) > 0x7E)
    control = sum(1 for c in s if ord(c) < 0x20 or ord(c) == 0x7F)
    spaces = sum(1 for c in s if c == " ")
    hexc = sum(1 for c in s if c in "0123456789abcdefABCDEF")
    return (f"len={len(s)} printable_ascii={printable} (spaces {spaces}, hex digits {hexc}) "
            f"non_ascii={nonascii} control={control} backtick={s.count(chr(96))}")


def skip_census(snap: DirectorySnapshot, max_folders: int = 60) -> list[str]:
    """Log lines for the F2 evidence: per capture folder how many files are kept and how many are left out for
    each reason, then one sample per reason showing only lengths and character classes. Folder names are cleaned
    and bounded (they are shown, never matched). Nothing here decides anything."""
    per: dict[tuple[str, str], dict[str, int]] = {}
    samples: dict[str, CaptureEntry] = {}
    for e in snap.captures:
        r = skip_reason(e)
        row = per.setdefault((e.folder_key, _library_folder(e.folder_name)), {"kept": 0})
        row[r or "kept"] = row.get(r or "kept", 0) + 1
        if r and r not in samples:
            samples[r] = e
    lines = []
    ordered = sorted(per.items(), key=lambda kv: (-sum(kv[1].values()), kv[0][1]))
    for (_k, name), row in ordered[:max_folders]:
        lines.append(f"capture skip census: folder {name!r}: " + ", ".join(
            f"{k} {row[k]}" for k in ("kept", *SKIP_REASONS) if row.get(k)))
    if len(ordered) > max_folders:
        lines.append(f"capture skip census: {len(ordered) - max_folders} more folders not shown")
    totals = {k: sum(r.get(k, 0) for r in per.values()) for k in ("kept", *SKIP_REASONS)}
    lines.append("capture skip census: totals " + ", ".join(f"{k} {v}" for k, v in totals.items()))
    for r in SKIP_REASONS:
        e = samples.get(r)
        if e is not None:
            lines.append(f"capture skip sample ({r}): name[{char_classes(e.name or '')}] key[{char_classes(e.key or '')}] "
                         f"factory={e.is_factory} cloud={e.is_cloud} version_field_len={len(e.version_field or '')}")
    return lines


def library_payload(snap: DirectorySnapshot) -> dict:
    """The sync_device_library answer (contract section 9.2): every capture file of the same fresh listing a
    push resolves captures from, with the version the push would use (version_of). Favorites views are left
    out, as resolve_one leaves them out. A file whose name could not be compared exactly (not printable ASCII,
    changed by clean-up, over 64 characters), or whose key is not 64 hex characters, is skipped and counted
    (skip_reason): QC-2 must never hold a name the bridge would not match byte for byte."""
    out, skipped = [], 0
    for e in snap.captures:
        folder = _library_folder(e.folder_name)
        if skip_reason(e):
            skipped += 1
            continue
        out.append({"name": e.name, "key": e.key.lower(), "version": version_of(e, snap),
                    "kind": "factory" if e.is_factory else "user", "folder": folder, "cloud": bool(e.is_cloud)})
    if len(out) > MAX_LIBRARY_CAPTURES:
        raise LibraryTooLarge(len(out))
    return {"captures": out, "skipped": skipped}


def summary(resolved: dict[int, ResolvedCapture]) -> str:
    if not resolved:
        return ""
    items = sorted(resolved.items())
    parts = [f"col {c} {text.clean(rc.name, 32)!r} V{rc.version} ({rc.source})" for c, rc in items[:4]]
    more = f"; +{len(items) - 4} more" if len(items) > 4 else ""  # keeps the result message under its 500 chars
    return " Captures: " + "; ".join(parts) + more + "."
