"""The real device backend: qc-mcp's transport (vendored at f634ecd) as a library.

Three layers keep the transport to the operations the plan allows (section 5):

1. load_vendored() refuses to import the transport unless every file in vendor/
   matches its f634ecd SHA-256 pin (and nothing else is there), then compiles each
   module from the bytes it just hashed; the folder is never on sys.path.
2. Guarded exposes only the transport methods this backend calls. A NEVER list,
   which a grant can't include, names the forbidden ones: save_preset (RecallPreset
   SAVE, the documented hang), recall, clear_grid, delete_block, load_preset, raw
   send/request, global I/O, EQ, master volume and mode writes, model presets,
   stomps, captures/IRs, splits, scene labels and preset meta.
3. WireGuard sits between the transport and the HID device and decodes every
   outgoing message before it is written. Only these pass:
     * session plumbing the handshake sends (KeepAlive, ResetCommsBuffers,
       Connection, the Version read and client-version announce);
     * READs of state (preset, directory, setlist position, scene, and the
       handshake's subscription reads);
     * while a push is armed: Grid UPDATEs on row 0 that place exactly the plan's
       models in the plan's columns, set the plan's parameter indexes to float
       values in 0..1, set routing in 1 -> out 19, or set bypass flags; and
       Scene UPDATEs (the transport switches scenes to write per-scene values);
     * while a save is armed: one File CREATE of My Presets at the armed position
       with the armed name and nothing else (no payload).
   Messages with unknown fields, compressed payloads, or empty model entries (the shape
   of a delete) are refused too. Everything else raises WireRefused before a byte reaches the unit, including any
   RecallPreset UPDATE (save_preset), SetlistPosition UPDATE (recall), Grid DELETE,
   File DELETE, and every IOSettings / GlobalEQ / MasterVolume / Mode / ModelPreset write.

The vendored files are not modified. Liveness (G2 c) reads two private attributes
of the vendored Windows transport (its reader thread, its write-error counter), as
the Phase 0 spikes did; nothing is written to them.
"""
from __future__ import annotations

import logging
import sys
import threading
import types
from pathlib import Path

from .device import SETLIST_KEY, DeviceBackend, DeviceError, Presence, UnitAbsent, UnitBusy
from .vendor_pins import VENDOR_DIR, mismatches, verified_bytes

log = logging.getLogger("qc_agent")


class WireRefused(DeviceError):
    """The wire guard stopped a message the plan doesn't allow."""


class GuardError(DeviceError):
    """Code tried to use a transport method that isn't granted."""


# ---------------------------------------------------------------- loading

_loaded = None
# Dependency order: each module's relative imports are already in sys.modules when it runs.
_MODULES = ("backend", "protocol", "directory", "winhid", "transport")


def load_vendored(vendor_dir: Path = VENDOR_DIR):
    """Import the vendored transport from its hash-checked bytes.

    Each module's source is read once, checked against its f634ecd pin and compiled from those
    bytes in memory. The vendor folder is never put on sys.path and no bytecode is read or
    written, so nothing planted beside the pinned files (an extra package, a .pyc) can be
    imported instead, and there is no gap between checking a file and running it. The package's
    __path__ is empty, so a lazy import of anything not loaded here (bridge, iohid) fails rather
    than finding a file. The descriptor sets are pinned too; protocol.py reads them from the folder.
    """
    global _loaded
    if _loaded is not None:
        return _loaded
    bad = mismatches(vendor_dir)
    if bad:
        raise DeviceError("the vendored qc-mcp transport differs from the reviewed f634ecd copy: "
                          + "; ".join(bad[:4]) + ". Refusing to drive the unit with it.")
    if any(n == "qc_mcp" or n.startswith("qc_mcp.") for n in sys.modules):
        raise DeviceError("a qc_mcp package is already imported; refusing to mix it with the vendored one")
    root = (vendor_dir / "qc_mcp").resolve()
    try:
        pkg = types.ModuleType("qc_mcp")
        pkg.__path__ = []
        pkg.__package__ = "qc_mcp"
        pkg.__file__ = str(root / "__init__.py")
        exec(compile(verified_bytes("qc_mcp/__init__.py", vendor_dir), pkg.__file__, "exec"), pkg.__dict__)
        sys.modules["qc_mcp"] = pkg
        mods = {}
        for name in _MODULES:
            full, path = f"qc_mcp.{name}", root / f"{name}.py"
            m = types.ModuleType(full)
            m.__file__, m.__package__ = str(path), "qc_mcp"
            sys.modules[full] = m
            code = compile(verified_bytes(f"qc_mcp/{name}.py", vendor_dir), str(path), "exec")
            exec(code, m.__dict__)
            setattr(pkg, name, m)
            mods[name] = m
    except Exception as e:  # noqa: BLE001
        for n in [n for n in sys.modules if n == "qc_mcp" or n.startswith("qc_mcp.")]:
            del sys.modules[n]
        raise DeviceError(f"could not load the vendored transport: {type(e).__name__}: {e}") from None
    _loaded = mods
    return mods


# ---------------------------------------------------------------- method allowlist

NEVER = frozenset({
    "send", "request", "save_preset", "recall", "clear_grid", "delete_block", "load_preset",
    "apply_spec", "reconnect", "set_mode", "set_mode_cycle", "write_global_eq", "set_io_port",
    "load_settings_preset", "save_model_preset", "delete_model_preset", "load_model_preset",
    "select_model_slot", "assign_stomp", "unassign_stomp", "assign_expression", "set_capture",
    "set_ir", "add_splitter", "add_mixer", "remove_split", "set_split_points", "set_lane_param",
    "set_scene_label", "set_scene_color", "set_preset_meta", "set_param_typed", "set_scene",
    "latest_broadcast", "read_message", "list_model_presets", "list_recents_favorites",
})
GRANT = frozenset({
    "open", "close", "get_current_preset", "get_setlist_position", "list_directory",
    "set_routing", "add_block", "set_param", "set_param_scenes", "set_block_bypass", "write_preset_file",
})
ATTRS = frozenset({"firmware", "device_type", "write_errors", "last_write_error", "io"})


class Guarded:
    def __init__(self, qc, grant=GRANT):
        bad = set(grant) & NEVER
        if bad:
            raise GuardError(f"grant includes never-allowed methods: {sorted(bad)}")
        object.__setattr__(self, "_qc", qc)
        object.__setattr__(self, "_grant", frozenset(grant))

    def __getattr__(self, name):
        if name in NEVER:
            raise GuardError(f"{name} is never allowed")
        if name in self._grant or name in ATTRS:
            return getattr(self._qc, name)
        raise GuardError(f"{name} is not granted")

    def __setattr__(self, name, value):
        raise GuardError("the guarded transport is read-only")


# ---------------------------------------------------------------- wire guard

PLUMBING = frozenset({"KeepAlive", "ResetCommsBuffers", "Connection"})
READABLE = frozenset({
    "RecallPreset", "File", "SetlistPosition", "Scene", "PresetDirty", "Version",
    # the handshake's subscription READs (transport._handshake)
    "ModelRepo", "ModuleStats", "UndoRedo", "IOSettings", "GeneralSettings", "Mode",
    "GlobalEQ", "MasterVolume", "GlobalTempo",
})


def _names(msg):
    return {f.name for f, _v in msg.ListFields()}


class WireGuard:
    """Wraps the transport's HID object; see the module docstring."""

    def __init__(self, inner, protocol):
        self.inner = inner
        self.P = protocol
        self._lock = threading.Lock()
        self._chunks: list[tuple] = []
        self._buf = bytearray()
        self._push = None   # {"cols": {col: model_id}, "params": {(col, idx)}, "bypass_cols": set}
        self._save = None   # (folder_key, position, name)
        self.refused: list[str] = []

    @property
    def BENIGN_WRITE_CODES(self):  # noqa: N802 (the transport's spelling)
        return getattr(self.inner, "BENIGN_WRITE_CODES", frozenset({0}))

    def open(self):
        return self.inner.open()

    def close(self):
        return self.inner.close()

    def read_reports(self, timeout=0.1):
        return self.inner.read_reports(timeout)

    # -- arming --

    def arm_push(self, plan):
        self._push = {"cols": {b.col: b.model_id for b in plan.blocks},
                      "params": {(b.col, p.index) for b in plan.blocks for p in b.params},
                      "scene_params": {(b.col, p.index) for b in plan.blocks for p in b.params if p.scene_wires},
                      "bypass_cols": {b.col for b in plan.blocks if any(b.bypass_scenes)},
                      "max_index": {b.col: b.max_param_index for b in plan.blocks},
                      "row": plan.row, "in": plan.in_port, "out": plan.out_port}

    def arm_save(self, folder_key, position, name):
        self._save = (folder_key, position, name)

    def disarm(self):
        self._push = None
        self._save = None

    # -- the gate --

    def set_report(self, report_id, data, include_id=False):
        b = bytes(data)
        if len(b) < 2:
            raise WireRefused("malformed report")
        chunk_len, flags = b[0], b[1]
        with self._lock:
            if flags & self.P.FLAG_FIRST:
                self._chunks, self._buf = [], bytearray()
            self._chunks.append((report_id, b, include_id))
            self._buf += b[2:2 + chunk_len]
            if not flags & self.P.FLAG_LAST:
                return 0
            chunks, message = self._chunks, bytes(self._buf)
            self._chunks, self._buf = [], bytearray()
            try:
                self._check(message)
            except WireRefused as e:
                self.refused.append(str(e))
                log.error("wire guard refused a message: %s", e)
                raise
        rc = 0
        for rid, chunk, inc in chunks:
            r = self.inner.set_report(rid, chunk, include_id=inc)
            if r not in (None, 0) and rc == 0:
                rc = r
        return rc

    def _check(self, message):
        P = self.P
        if message[:3] == b"\x1f\x8b\x08":  # decode_message would gunzip it; we never send compressed
            raise WireRefused("compressed payloads are not allowed")
        cmd, pb, _h = P.decode_message(message)
        name = P.COMMANDS.get(cmd)
        if name is None:
            raise WireRefused(f"unknown command {cmd}")
        try:
            msg = P.message_class(cmd).FromString(pb)
        except Exception:  # noqa: BLE001
            raise WireRefused(f"{name}: does not decode") from None
        bare = P.message_class(cmd)()
        bare.CopyFrom(msg)
        bare.DiscardUnknownFields()
        if bare.SerializeToString() != msg.SerializeToString():
            raise WireRefused(f"{name}: unknown fields")
        fields = _names(msg)
        action = getattr(msg, "action", None) if "action" in msg.DESCRIPTOR.fields_by_name else None
        A = P.ACTION
        if name in PLUMBING:
            return
        if name == "Version":
            if action == A["READ"] and fields <= {"action", "request_id"}:
                return
            if action == A["UPDATE"] and fields <= {"action", "request_id", "cortex_control_version", "app_version"}:
                return  # the handshake's client-version announce
            raise WireRefused("Version: only the read and the client-version announce")
        if name in READABLE and action == A["READ"] and fields <= {"action", "request_id"}:
            return
        if name == "Grid" and self._push is not None and action == A["UPDATE"]:
            return self._check_grid(msg, fields)
        if name == "Scene" and self._push is not None and action == A["UPDATE"]:
            if fields <= {"action", "request_id", "selected_scene"} and 0 <= msg.selected_scene < 8:
                return
            raise WireRefused("Scene: only a scene switch A-H")
        if name == "File" and self._save is not None and action == A["CREATE"]:
            return self._check_save(msg, fields)
        raise WireRefused(f"{name} action={action} is not allowed")

    def _check_grid(self, msg, fields):
        if not fields <= {"action", "request_id", "preset"}:
            raise WireRefused(f"Grid: unexpected fields {sorted(fields)}")
        preset = msg.preset
        pf = _names(preset)
        if not pf <= {"chains", "bypass"}:
            raise WireRefused(f"Grid: preset fields {sorted(pf)} not allowed")
        for ch in preset.chains:
            self._check_chain(ch)
        for b in preset.bypass:
            self._check_bypass(b)

    def _check_chain(self, ch):
        push = self._push
        cf = _names(ch)
        if not cf <= {"row", "in_portid", "out_portid", "models"}:
            raise WireRefused(f"Grid: chain fields {sorted(cf)} not allowed")
        if ch.row != push["row"]:
            raise WireRefused(f"Grid: row {ch.row} is not the plan's row")
        if "in_portid" in cf and ch.in_portid != push["in"] or "out_portid" in cf and ch.out_portid != push["out"]:
            raise WireRefused("Grid: routing other than the plan's")
        for m in ch.models:
            self._check_model(m)

    def _check_model(self, m):
        push = self._push
        mf = _names(m)
        if not mf <= {"hash", "column", "params"}:
            raise WireRefused(f"Grid: model fields {sorted(mf)} not allowed")
        if not mf & {"hash", "params"}:
            raise WireRefused(f"Grid: an empty entry for column {m.column} (the shape of a delete)")
        col = m.column
        if col not in push["cols"]:
            raise WireRefused(f"Grid: column {col} is not in the plan")
        if "hash" in mf:
            if m.hash != push["cols"][col] or "params" in mf:
                raise WireRefused(f"Grid: placing model {m.hash} in column {col} is not the plan")
            return
        for p in m.params:
            self._check_param(col, p)

    def _check_param(self, col, p):
        push = self._push
        qf = _names(p)
        if qf == {"scene_mode"} or qf == {"index", "scene_mode"}:
            # "Assign to Scenes": only for the plan's scene-varying params, or a scene-varying block's bypass.
            if (col, p.index) in push["scene_params"]:
                return
            if col in push["bypass_cols"] and p.index > push["max_index"][col]:
                return  # the bypass control sits past every listed parameter (one past the read)
            raise WireRefused(f"Grid: assigning column {col} index {p.index} to scenes is not the plan")
        if not qf <= {"index", "param_values"} or (col, p.index) not in push["params"]:
            raise WireRefused(f"Grid: writing column {col} index {p.index} is not the plan")
        if len(p.param_values) != 1 or p.param_values[0].WhichOneof("value") != "float_value":
            raise WireRefused("Grid: parameter writes must be one float")
        v = p.param_values[0].float_value
        if not 0.0 <= v <= 1.0:
            raise WireRefused(f"Grid: wire value {v} outside 0..1")

    def _check_bypass(self, b):
        push = self._push
        if _names(b) - {"row", "colBypass"} or b.row != push["row"]:
            raise WireRefused("Grid: bypass outside the plan's row")
        for cb in b.colBypass:
            if _names(cb) - {"column", "sceneBypass"} or cb.column not in push["bypass_cols"]:
                raise WireRefused(f"Grid: bypass of column {cb.column} is not the plan")
            for sb in cb.sceneBypass:
                if _names(sb) - {"bypass"}:
                    raise WireRefused("Grid: unexpected bypass fields")

    def _check_save(self, msg, fields):
        folder_key, position, name = self._save
        if not fields <= {"action", "request_id", "folder"}:
            raise WireRefused(f"File CREATE: unexpected fields {sorted(fields)}")
        f = msg.folder
        if _names(f) - {"key", "files"} or f.key != folder_key or len(f.files) != 1:
            raise WireRefused("File CREATE: only one file in My Presets")
        fi = f.files[0]
        if _names(fi) - {"index", "name"} or fi.index != position or fi.name != name:
            raise WireRefused("File CREATE: position or name differs from the armed save")
        self._save = None  # one save per arming


# ---------------------------------------------------------------- the backend

def preset_to_plain(bp):
    """BinaryPreset -> the positional dict documented in device.py."""
    chains = []
    for ridx, ch in enumerate(bp.chains):
        blocks = []
        for cidx, m in enumerate(ch.models):
            if not m.hash:
                continue
            params = []
            for pidx, p in enumerate(m.params):
                vals = []
                for pv in p.param_values:
                    which = pv.WhichOneof("value")
                    vals.append(getattr(pv, which) if which else None)
                params.append({"index": pidx, "values": vals, "scene_mode": bool(p.scene_mode)})
            blocks.append({"col": cidx, "model_id": int(m.hash), "params": params})
        chains.append({"row": ridx, "in_port": int(ch.in_portid), "out_port": int(ch.out_portid), "blocks": blocks})
    bypass = None
    if len(bp.bypass):
        bypass = [[[bool(sb.bypass) for sb in cb.sceneBypass] for cb in row.colBypass] for row in bp.bypass]
    return {"name": bp.name, "chains": chains, "bypass": bypass}


class QcMcpBackend(DeviceBackend):
    def __init__(self, vendor_dir: Path = VENDOR_DIR):
        self._mods = load_vendored(vendor_dir)
        self._qc = None
        self._raw = None
        self._guard = None
        self._errs = []

    # -- presence without opening (Cortex Control keeps working) --

    def probe(self):
        if sys.platform != "win32":
            return Presence(None, None)
        b, w = self._mods["backend"], self._mods["winhid"]
        try:
            found = w.enumerate_devices(b.QC_VID, tuple(b.QC_PIDS))
        except Exception:  # noqa: BLE001
            return Presence(None, None)
        return Presence(bool(found), any(d.get("busy") for d in found) if found else False)

    # -- session --

    def connect(self):
        T, P = self._mods["transport"], self._mods["protocol"]
        try:
            qc = T.QuadCortex(share=False, bridge=False)  # exclusive direct mode only
        except RuntimeError as e:
            raise UnitAbsent(str(e)) from None
        guard = WireGuard(qc.io, P)
        qc.io = guard
        self._raw, self._guard, self._errs = qc, guard, []
        self._qc = Guarded(qc)
        try:
            self._qc.open(handshake=True)
        except RuntimeError as e:
            self.disconnect()
            msg = str(e)
            if "already open exclusively" in msg:
                raise UnitBusy(msg) from None
            if "no HID device" in msg:
                raise UnitAbsent(msg) from None
            raise DeviceError(msg) from None
        except Exception as e:  # noqa: BLE001
            self.disconnect()
            raise DeviceError(f"{type(e).__name__}: {e}") from None
        self.firmware = qc.firmware
        self.device_type = qc.device_type

    def disconnect(self):
        qc, self._qc, self._raw = self._raw, None, None
        if qc is None:
            return
        t = threading.Thread(target=lambda: _quiet(qc.close), daemon=True)
        t.start()
        t.join(10.0)
        if t.is_alive():
            log.warning("closing the unit took more than 10 s; continuing")

    def alive(self):
        qc = self._raw
        if qc is None:
            return False
        inner = getattr(qc.io, "inner", None)
        thread = getattr(inner, "_thread", None)
        if thread is not None and not thread.is_alive():
            return False
        self._errs = (self._errs + [qc.write_errors])[-3:]
        e = self._errs
        return not (len(e) == 3 and e[0] < e[1] < e[2])  # rising on consecutive samples (G2 c)

    def _call(self, name, *args, **kwargs):
        if self._qc is None:
            raise DeviceError("not connected")
        try:
            return getattr(self._qc, name)(*args, **kwargs)
        except (DeviceError, GuardError):
            raise
        except self._mods["transport"].QCError as e:
            raise DeviceError(str(e)) from None
        except RuntimeError as e:
            raise UnitAbsent(str(e)) from None

    # -- reads --

    def list_my_presets(self):
        cat = self._call("list_directory")
        folders = [f for f in cat.get("presets", []) if f.get("key") == SETLIST_KEY and not f.get("is_factory")]
        if len(folders) != 1:
            raise DeviceError("My Presets is missing from the unit's directory listing")
        files = folders[0].get("files", [])
        # A setlist is a fixed 256-slot table, empty slots included (G0 saw 256). A shorter or longer
        # table would make a missing slot look empty, so it is refused rather than guessed at.
        if len(files) != 256:
            raise DeviceError(f"My Presets listed {len(files)} slots, not 256")
        return {int(f["index"]): f.get("name") or "" for f in files}

    def setlist_position(self):
        return self._call("get_setlist_position")

    def read_current_preset(self):
        bp = self._call("get_current_preset")
        return None if bp is None else preset_to_plain(bp)

    # -- writes --

    def begin_push(self, position, name, plan):
        if self._guard is None:
            raise DeviceError("not connected")
        self._guard.arm_push(plan)
        self._save_target = (position, name)

    def end_push(self):
        if self._guard is not None:
            self._guard.disarm()

    def set_routing(self, row, in_port, out_port):
        self._call("set_routing", row, in_portid=in_port, out_portid=out_port)

    def add_block(self, row, col, model_id):
        return bool(self._call("add_block", model_id, row=row, column=col))

    def set_param(self, row, col, index, wire):
        self._call("set_param", row, col, index, wire)

    def set_param_scenes(self, row, col, index, wires):
        self._call("set_param_scenes", row, col, index, list(wires))

    def set_block_bypass(self, row, col, scenes):
        # Always the per-scene path G6 verified (assign to scenes, then each scene's flag
        # explicitly), even when all eight agree: unwritten scenes keep their old value.
        self._call("set_block_bypass", row, col, scenes=[bool(x) for x in scenes])

    def save_to_slot(self, position, name):
        if getattr(self, "_save_target", None) != (position, name):
            raise DeviceError("save target differs from the armed push")
        self._guard.arm_save(SETLIST_KEY, position, name)
        try:
            self._call("write_preset_file", SETLIST_KEY, position, name)
        finally:
            self._guard._save = None


def _quiet(fn):
    try:
        fn()
    except Exception:  # noqa: BLE001
        pass
