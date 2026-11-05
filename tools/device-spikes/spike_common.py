"""Shared helpers for the Phase 0 device spikes (docs/design/qc-device-bridge-plan.md, section 7).

What this is: thin glue around qc-mcp's transport used as a LIBRARY (pinned at commit
f634ecd, from the owner's existing clone). It never starts qc-mcp's MCP server, never
uses pyquadcortex, and never runs in a container.

What it adds on top of the transport:

* ``load_qc_mcp`` refuses to run unless the clone is at the pinned commit with an unmodified
  ``src/`` tree, so a result can always be tied to the code that produced it.
* ``Guarded`` wraps the QuadCortex object and exposes ONLY the methods a given spike needs.
  Everything else raises ``GuardError``, and a hard-coded NEVER list cannot be allowed by
  mistake (``save_preset`` = RecallPreset SAVE, the documented hang; ``recall``; ``clear_grid``;
  raw ``send``/``request``; global I/O, EQ, volume and mode writes; deletes).
* Slot and directory helpers, taper prediction maths, and JSON result writing.

Standard library only (qc-mcp itself needs protobuf; the owner's venv has it).
"""
from __future__ import annotations

import argparse
import datetime as _dt
import json
import math
import os
import subprocess
import sys
import threading
import time
import traceback
import xml.etree.ElementTree as ET
from pathlib import Path

PINNED_COMMIT = "f634ecd3dcf426090b90bc4cc176e4b500494b9e"
SETLIST = "/media/p4/Presets/My Presets"  # qc-mcp's default My Presets folder key (server.py list_empty_slots)
BANK_SIZE = 8  # full Quad Cortex: 8 slots per bank (the Mini uses 4)
REPO_ROOT = Path(__file__).resolve().parents[2]
OUR_CATALOG = REPO_ROOT / "internal" / "catalog" / "qc_catalog.json"

EXIT_OK, EXIT_CHECK_FAILED, EXIT_NO_DEVICE, EXIT_REFUSED, EXIT_UNEXPECTED = 0, 1, 2, 3, 4


class GuardError(RuntimeError):
    """A spike tried to use a transport method it was not granted."""


class SpikeRefused(RuntimeError):
    """A precondition failed. Nothing was written."""


# --------------------------------------------------------------------------- console

def init_console() -> None:
    """Make output safe for PowerShell's legacy code page."""
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass


def say(msg: str = "") -> None:
    print(msg, flush=True)


def banner(title: str) -> None:
    say("")
    say("=" * 72)
    say(title)
    say("=" * 72)


def add_common_args(p: argparse.ArgumentParser, *, writes: bool = False) -> None:
    home = Path.home()
    p.add_argument("--qc-mcp", default=os.environ.get("QC_MCP_DIR", str(home / "qc-trial" / "qc-mcp")),
                   help="qc-mcp clone, checked out at f634ecd (default: %(default)s)")
    p.add_argument("--out", default=str(home / "qc-trial" / "spike-results"),
                   help="folder for result JSON files (default: %(default)s). Kept outside the repo.")
    p.add_argument("--firmware", default=None,
                   help="exact CorOS string the unit must report, e.g. 4.1.0. "
                        + ("REQUIRED for this spike." if writes else "Optional; checked if given."))
    p.add_argument("--pause", action="store_true",
                   help="wait for Enter before exiting (use when started from a shortcut)")
    p.add_argument("--allow-unpinned", action="store_true",
                   help="TESTS ONLY: skip the commit/clean-tree check on the qc-mcp clone")


# --------------------------------------------------------------------------- qc-mcp loading

def _git(root: Path, *args: str) -> str | None:
    try:
        r = subprocess.run(["git", "-C", str(root), *args], capture_output=True, text=True, timeout=30)
    except (OSError, subprocess.SubprocessError):
        return None
    return r.stdout.strip() if r.returncode == 0 else None


class QcMcp:
    """The imported qc-mcp modules plus where they came from."""

    def __init__(self, root: Path, transport, protocol, catalog, commit: str | None):
        self.root, self.transport, self.protocol, self.catalog, self.commit = root, transport, protocol, catalog, commit

    @property
    def xml_path(self) -> Path:
        return self.root / "src" / "qc_mcp" / "ModelRepo.xml"


def load_qc_mcp(path: str, *, allow_unpinned: bool = False) -> QcMcp:
    root = Path(path).expanduser().resolve()
    src = root / "src"
    if not (src / "qc_mcp" / "transport.py").is_file():
        raise SpikeRefused(f"{root} does not look like a qc-mcp clone (no src/qc_mcp/transport.py). "
                           "Pass --qc-mcp <folder>.")
    head = _git(root, "rev-parse", "HEAD")
    if not allow_unpinned:
        if head != PINNED_COMMIT:
            raise SpikeRefused(f"qc-mcp clone is at {head or 'an unknown commit (is git on PATH?)'}, not the pinned "
                               f"{PINNED_COMMIT[:7]}. Run: git -C \"{root}\" checkout {PINNED_COMMIT}")
        dirty = _git(root, "status", "--porcelain", "--", "src")
        if dirty is None or dirty:
            raise SpikeRefused("qc-mcp src/ has local modifications; refusing to run unreviewed transport code.")
    sys.path.insert(0, str(src))
    import qc_mcp.catalog as catalog
    import qc_mcp.protocol as protocol
    import qc_mcp.transport as transport
    return QcMcp(root, transport, protocol, catalog, head)


# --------------------------------------------------------------------------- guard

# Never grantable, whatever a spike asks for. Reasons are in docs/research/qc-usb-tools-audit.md 1.5.
NEVER = frozenset({
    "send", "request", "save_preset",  # raw transport; RecallPreset SAVE hangs the unit on an Unsaved slot
    "recall", "clear_grid", "delete_block", "load_preset", "apply_spec",
    "set_mode", "set_mode_cycle", "write_global_eq", "set_io_port", "load_settings_preset",
    "save_model_preset", "delete_model_preset", "load_model_preset", "select_model_slot",
    "assign_stomp", "unassign_stomp", "assign_expression", "set_capture", "set_ir",
    "add_splitter", "add_mixer", "remove_split", "set_split_points", "set_lane_param",
    "reconnect", "set_scene_label", "set_scene_color", "set_preset_meta",
})
READ_METHODS = frozenset({"open", "close", "read_state", "get_current_preset", "get_setlist_position", "list_directory"})
READ_ATTRS = frozenset({"firmware", "device_type", "custom_name", "write_errors", "last_write_error", "protocol_version"})
# Per-spike grants (always a subset check against NEVER).
GRANT_READ = READ_METHODS
GRANT_G3 = READ_METHODS | {"set_routing", "add_block", "set_param", "set_param_scenes", "write_preset_file"}
GRANT_G4 = READ_METHODS | {"set_param"}
GRANT_G6 = READ_METHODS | {"set_param", "set_param_scenes", "set_scene", "set_block_bypass"}


class Guarded:
    def __init__(self, qc, grant: frozenset[str]):
        bad = set(grant) & NEVER
        if bad:
            raise GuardError(f"grant includes never-allowed methods: {sorted(bad)}")
        object.__setattr__(self, "_qc", qc)
        object.__setattr__(self, "_grant", frozenset(grant))

    def __getattr__(self, name: str):
        if name in NEVER:
            raise GuardError(f"{name} is never allowed in a spike")
        if name in self._grant or name in READ_ATTRS:
            return getattr(self._qc, name)
        raise GuardError(f"{name} is not granted to this spike")

    def __setattr__(self, name, value):
        raise GuardError("the guarded transport is read-only")

    def diagnostics(self) -> dict:
        """Liveness signals for G2(c). Deliberately reaches into transport internals; read-only."""
        qc = self._qc
        io = getattr(qc, "io", None)
        thread = getattr(io, "_thread", None)
        return {
            "reader_thread_alive": bool(thread.is_alive()) if thread is not None else None,
            "handle_open": (getattr(io, "_h", None) is not None) if io is not None else None,
            "write_errors": getattr(qc, "write_errors", None),
            "last_write_error": getattr(qc, "last_write_error", None),
        }


def open_device(mcp: QcMcp, grant: frozenset[str]) -> Guarded:
    """Exclusive (seizing) direct-mode open. Never the shared/bridge modes."""
    qc = mcp.transport.QuadCortex(share=False, bridge=False)
    g = Guarded(qc, grant)
    try:
        g.open()
    except Exception:
        try:
            g.close()
        except Exception:
            pass
        raise
    return g


def check_firmware(qc: Guarded, expected: str | None, *, required: bool) -> None:
    say(f"Unit reports: firmware={qc.firmware!r} device_type={qc.device_type!r} custom_name={qc.custom_name!r}")
    if expected is None:
        if required:
            raise SpikeRefused("--firmware is required for this spike. Run g0 first and copy the exact firmware string.")
        return
    if qc.firmware != expected:
        raise SpikeRefused(f"firmware mismatch: unit says {qc.firmware!r}, you passed {expected!r}. Nothing was changed.")


# --------------------------------------------------------------------------- slots

def parse_slot(label: str) -> int:
    """'2A' -> 8 on a full Quad Cortex (8 per bank, A=0..H=7)."""
    s = label.strip().upper()
    if len(s) < 2 or not s[:-1].isdigit() or s[-1] not in "ABCDEFGH":
        raise SpikeRefused(f"bad slot label {label!r}; expected like 2A or 12H")
    bank = int(s[:-1])
    if bank < 1:
        raise SpikeRefused("bank numbers start at 1")
    return (bank - 1) * BANK_SIZE + "ABCDEFGH".index(s[-1])


def slot_label(position: int) -> str:
    return f"{position // BANK_SIZE + 1}{'ABCDEFGH'[position % BANK_SIZE]}"


def require_full_qc(qc: Guarded) -> None:
    """Slot labels assume 8 per bank. The Mini banks in 4s, so labels would be wrong."""
    if qc.device_type not in (None, "QC"):
        raise SpikeRefused(f"device_type is {qc.device_type!r}; slot labels here assume a full Quad Cortex. "
                           "Use the numeric position instead.")


def find_folder(catalog: dict, key: str = SETLIST) -> dict | None:
    for fo in catalog.get("presets", []):
        if fo.get("key") == key:
            return fo
    return None


def slot_entry(folder: dict, position: int) -> dict:
    """The file entry at ``position``; an absent entry and a blank name both mean empty."""
    for f in folder.get("files", []):
        if f.get("index") == position:
            return f
    return {"index": position, "name": ""}


def slot_is_empty(folder: dict, position: int) -> bool:
    return not (slot_entry(folder, position).get("name") or "").strip()


# --------------------------------------------------------------------------- preset helpers

def iter_blocks(bp):
    """Yield (row_idx, array_idx, model) for every placed block in a full-preset read.

    In a read the row and column FIELDS are 0; positions are the array indexes (transport.py
    bypass_param_index and clear_grid both rely on that), so both are returned and recorded.
    """
    for ridx, ch in enumerate(bp.chains):
        for cidx, m in enumerate(ch.models):
            if m.hash:
                yield ridx, cidx, m


def param_value(p):
    """First value of a param as a plain Python value (float/int/str) or None."""
    if not p.param_values:
        return None
    pv = p.param_values[0]
    which = pv.WhichOneof("value")
    return getattr(pv, which) if which else None


def get_param(bp, row: int, col: int, idx: int):
    """The Param message at chains[row].models[col].params[idx], or None if any level is missing."""
    if bp is None or len(bp.chains) <= row or len(bp.chains[row].models) <= col:
        return None
    m = bp.chains[row].models[col]
    return m.params[idx] if idx < len(m.params) else None


def fmt_num(v, digits: int = 6) -> str:
    """Print a device value without ever raising: floats rounded, anything else shown raw."""
    return str(round(v, digits)) if isinstance(v, float) else repr(v)


def param_values_all(p) -> list:
    out = []
    for pv in p.param_values:
        which = pv.WhichOneof("value")
        out.append(getattr(pv, which) if which else None)
    return out


# --------------------------------------------------------------------------- catalogs

def load_xml_models(xml_path: Path) -> dict[int, dict]:
    """qc-mcp's ModelRepo.xml, keeping what qc-mcp's own parser drops: clones/replaces/hidden."""
    root = ET.parse(xml_path).getroot()
    out: dict[int, dict] = {}
    for cat in root.findall("Category"):
        for mdl in cat.findall("Model"):
            params = [{
                "xml_pos": i, "name": p.get("name"), "type": p.get("type"), "min": p.get("min"), "max": p.get("max"),
                "min_symbol": p.get("min") if p.get("min") and not _is_num(p.get("min")) else None,
                "max_symbol": p.get("max") if p.get("max") and not _is_num(p.get("max")) else None,
                "default": p.get("defaultValue"), "units": p.get("units"), "skew": p.get("skew"),
                "replaces": p.get("replaces"), "hidden": p.get("hidden"),
            } for i, p in enumerate(mdl.findall("Parameter"))]
            out[int(mdl.get("id"))] = {"id": int(mdl.get("id")), "name": mdl.get("name"), "category": cat.get("name"),
                                       "clones": mdl.get("clones"), "hidden": mdl.get("hidden"), "params": params}
    return out


def _is_num(s: str | None) -> bool:
    try:
        float(s)  # type: ignore[arg-type]
        return True
    except (TypeError, ValueError):
        return False


def load_our_catalog(path: Path = OUR_CATALOG) -> dict[int, dict]:
    with open(path, encoding="utf-8") as f:
        data = json.load(f)
    return {m["id"]: m for m in data["models"]}


# --------------------------------------------------------------------------- taper maths (G4)

# Display <-> wire: wire = ((v - min) / (max - min)) ** skew, so v = min + (max - min) * wire ** (1 / skew).
# The skews and the symbolic CABSIM bounds are the numbers the plan proposes to adopt in P1. They come from
# pyquadcortex's published 4.0.1 measurements (MIT) and are used as DATA here. G4 exists to check them on
# the owner's unit. They are a hypothesis, not a fact, until the owner's readings agree.
CABSIM_DB_RANGE = (-40.0, 6.0)  # MIN_CABSIM_DB / MAX_CABSIM_DB (unverified on this unit until G4)


def predict_display(vmin: float, vmax: float, skew: float, wire: float) -> float:
    if not (0.0 <= wire <= 1.0) or math.isnan(wire):
        raise ValueError(f"wire {wire} outside 0..1")
    return vmin + (vmax - vmin) * (wire ** (1.0 / skew))


# --------------------------------------------------------------------------- results

def write_result(out_dir: str, name: str, obj: dict) -> Path:
    d = Path(out_dir).expanduser()
    d.mkdir(parents=True, exist_ok=True)
    stamp = _dt.datetime.now().strftime("%Y%m%d-%H%M%S")
    path = d / f"{name}-{stamp}.json"
    with open(path, "w", encoding="utf-8") as f:
        json.dump(obj, f, indent=2, default=str, ensure_ascii=False)
    return path


def base_result(gate: str, mcp: QcMcp | None, args) -> dict:
    return {
        "gate": gate,
        "when": _dt.datetime.now().astimezone().isoformat(timespec="seconds"),
        "qc_mcp_commit": mcp.commit if mcp else None,
        "script_args": {k: v for k, v in vars(args).items() if k not in ("qc_mcp", "out")},
    }


# --------------------------------------------------------------------------- run wrapper

def run_spike(main_fn, argv=None) -> int:
    """Run ``main_fn(args)`` with uniform error handling, exit codes and an optional pause."""
    init_console()
    code = EXIT_CHECK_FAILED
    pause = "--pause" in (argv if argv is not None else sys.argv[1:])
    try:
        code = main_fn()
    except SpikeRefused as e:
        say(f"\nREFUSED (nothing was written): {e}")
        code = EXIT_REFUSED
    except GuardError as e:
        say(f"\nGUARD: {e}")
        code = EXIT_REFUSED
    except RuntimeError as e:
        # winhid raises RuntimeError for "no device" and "already open exclusively".
        say(f"\nCOULD NOT OPEN THE UNIT: {e}")
        code = EXIT_NO_DEVICE
    except KeyboardInterrupt:
        say("\nInterrupted. Closing.")
        code = EXIT_CHECK_FAILED
    except Exception:  # noqa: BLE001  keep the window open so the traceback can be read and pasted
        say("\nUNEXPECTED ERROR (paste everything below):")
        traceback.print_exc(file=sys.stdout)
        code = EXIT_UNEXPECTED
    if pause:
        try:
            input("\nPress Enter to close this window...")
        except EOFError:
            pass
    return code


def close_with_timeout(qc: Guarded, seconds: float = 10.0) -> dict:
    """Close the device but never hang the spike. Returns what happened (G2 wants this)."""
    info = {"closed": False, "seconds": None, "error": None, "hung": False}
    start = time.monotonic()

    def _close():
        try:
            qc.close()
            info["closed"] = True
        except Exception as e:  # noqa: BLE001
            info["error"] = f"{type(e).__name__}: {e}"

    t = threading.Thread(target=_close, daemon=True)
    t.start()
    t.join(seconds)
    info["hung"] = t.is_alive()
    info["seconds"] = round(time.monotonic() - start, 2)
    return info
