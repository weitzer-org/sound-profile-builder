"""The real device backend: qc-mcp's transport (vendored at f634ecd) as a library.

Three layers keep the transport to the operations the plan allows (section 5):

1. load_vendored() refuses to import the transport unless every file in vendor/
   matches its f634ecd SHA-256 pin (and nothing else is there), then compiles each
   module from the bytes it just hashed; the folder is never on sys.path.
2. Guarded exposes only the transport methods this backend calls. A NEVER list,
   which a grant can't include, names the forbidden ones: save_preset (RecallPreset
   SAVE, the documented hang), recall, clear_grid, delete_block, load_preset, raw
   send/request, global I/O, EQ, master volume and mode writes, model presets,
   stomps, IRs, splits, scene labels and preset meta. set_capture is granted (plan 4.2):
   its one string write is held to the armed capture by the wire guard.
3. WireGuard sits between the transport and the HID device and decodes every
   outgoing message before it is written. Only these pass:
     * session plumbing the handshake sends (KeepAlive, ResetCommsBuffers,
       Connection, the Version read and client-version announce);
     * READs of state (preset, directory, setlist position, scene, and the
       handshake's subscription reads);
     * while a push is armed: Grid UPDATEs on row 0 that place exactly the plan's
       models in the plan's columns, set the plan's parameter indexes to float
       values in 0..1, set routing in 1 -> out 19, or set bypass flags; write each capture
       block's file_name (index 5) exactly once, as the exact string the push resolved
       from the unit's own listing; and
       Scene UPDATEs (the transport switches scenes to write per-scene values);
     * while a save is armed: one File CREATE of My Presets at the armed position
       with the armed name and nothing else (no payload).
   Messages with unknown fields, compressed payloads, or empty model entries (the shape
   of a delete) are refused too. Everything else raises WireRefused before a byte reaches the unit, including any
   RecallPreset UPDATE (save_preset), SetlistPosition UPDATE (recall), Grid DELETE,
   File DELETE, and every IOSettings / GlobalEQ / MasterVolume / Mode / ModelPreset write.

The vendored files are not modified. One upstream bug is worked around in a subclass
(_fixed_quad_cortex, below): request() never dropped the reply it matched, so its pending queue
could double per request (_make_request). The same subclass reads the open slot strictly by
request_id (_make_setlist_position), and lists the directory without the vendored parser, which
drops the capture version and dependency fields (_make_list_directory_raw). Liveness (G2 c) reads two private attributes
of the vendored Windows transport (its reader thread, its write-error counter), as
the Phase 0 spikes did; nothing is written to them.
"""
from __future__ import annotations

import logging
import sys
import threading
import time
import types
from pathlib import Path

from .captures import FILE_NAME_PARAM as CAPTURE_FILE_NAME_PARAM
from .captures import CaptureEntry, DirectorySnapshot
from .device import SETLIST_KEY, DeviceBackend, DeviceError, Presence, UnitAbsent, UnitBusy
from .export_validator import CAPTURE_BLOCK_IDS
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
    "select_model_slot", "assign_stomp", "unassign_stomp", "assign_expression",
    "set_ir", "add_splitter", "add_mixer", "remove_split", "set_split_points", "set_lane_param",
    "set_scene_label", "set_scene_color", "set_preset_meta", "set_param_typed", "set_scene",
    "latest_broadcast", "read_message", "list_model_presets", "list_recents_favorites",
})
GRANT = frozenset({
    "open", "close", "get_current_preset", "get_setlist_position", "list_directory_raw",
    "set_routing", "add_block", "set_param", "set_param_scenes", "set_block_bypass", "write_preset_file",
    "set_capture",
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
        for b in plan.blocks:
            if b.model_id == 0 or (b.capture is not None and b.capture.file_name is None):
                raise DeviceError(f"column {b.col}: a capture block is not resolved")
        self._push = {"cols": {b.col: b.model_id for b in plan.blocks},
                      # each capture block's file_name: written once, exactly this string (plan 4.2)
                      "strings": {(b.col, CAPTURE_FILE_NAME_PARAM): b.capture.file_name
                                  for b in plan.blocks if b.capture is not None},
                      "params": {(b.col, p.index) for b in plan.blocks for p in b.params},
                      "scene_params": {(b.col, p.index) for b in plan.blocks for p in b.params if p.scene_wires},
                      "bypass_cols": {b.col for b in plan.blocks if any(b.bypass_scenes)},
                      "max_index": {b.col: b.max_param_index for b in plan.blocks},
                      # the exact values each write may carry (float32 on the wire, so compared within 1e-6)
                      "values": {(b.col, p.index): {p.wire, *(p.scene_wires or ())}
                                 for b in plan.blocks for p in b.params},
                      "flags": {b.col: set(b.bypass_scenes) for b in plan.blocks},
                      "bypass_index": {},  # col -> the unit's own bypass index, set by allow_bypass_index
                      "row": plan.row, "in": plan.in_port, "out": plan.out_port}

    def allow_bypass_index(self, col, index):
        self._push["bypass_index"][col] = index

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
            if col in push["bypass_cols"] and p.index == push["bypass_index"].get(col):
                return  # the bypass control: one past the unit's own read of this block (allow_bypass_index)
            raise WireRefused(f"Grid: assigning column {col} index {p.index} to scenes is not the plan")
        if qf == {"index", "param_values"} and (col, p.index) in push["strings"]:
            want = push["strings"][(col, p.index)]
            if (len(p.param_values) != 1 or p.param_values[0].WhichOneof("value") != "string_value"
                    or p.param_values[0].string_value != want):
                raise WireRefused(f"Grid: column {col} index {p.index} may only carry the planned capture file name")
            del push["strings"][(col, p.index)]  # once
            return
        if not qf <= {"index", "param_values"} or (col, p.index) not in push["params"]:
            raise WireRefused(f"Grid: writing column {col} index {p.index} is not the plan")
        if len(p.param_values) != 1 or p.param_values[0].WhichOneof("value") != "float_value":
            raise WireRefused("Grid: parameter writes must be one float")
        v = p.param_values[0].float_value
        if not 0.0 <= v <= 1.0:
            raise WireRefused(f"Grid: wire value {v} outside 0..1")
        if not any(abs(v - a) <= 1e-6 for a in push["values"][(col, p.index)]):
            raise WireRefused(f"Grid: column {col} index {p.index} value {v} is not the plan's")

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
                if bool(sb.bypass) not in push["flags"][cb.column]:
                    raise WireRefused(f"Grid: bypass {sb.bypass} on column {cb.column} is not the plan's")

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


# ---------------------------------------------------------------- a fix to one transport method

def _make_request(mods):
    """QuadCortex.request from f634ecd with one fix, installed by _fixed_quad_cortex.

    Upstream removes the matched reply with `t is not (cmd, obj, raw, pb)`, comparing each
    pending tuple to a NEW tuple, so nothing is ever removed and `keep + _pending` duplicates
    every unconsumed message on each matched request. With the unit streaming broadcasts while a
    session is live, `_pending` doubles per request; a per-scene write makes ~9 requests, so a
    push could exhaust memory (tests/test_qcmcp_backend.py reproduces it with the simulated
    unit). This copy removes the matched entry by identity of the pending tuple itself. The
    vendored file stays byte-identical; the fix belongs upstream, then a re-vendoring PR.

    Two narrowings of upstream's fallback (it returns the first reply of the right command when
    no reply carries the request's id):
    - A reply carrying a *different* non-zero id answers some earlier request (say, one that timed
      out), so it is dropped and never used as the fallback. Only a reply with no id (a unit that
      doesn't echo ids) can be.
    - `strict=True` has no fallback at all: only the reply carrying this request's id is accepted,
      and a timeout raises. The push's slot checks read the position this way (see
      _make_setlist_position).
    """
    P, QCError = mods["protocol"], mods["transport"].QCError

    def request(self, command, proto_message=None, proto_bytes=None, expect=None, timeout_ms=2000,
                strict=False):
        with self._io_lock:  # upstream's @_serialized
            want = command if isinstance(command, int) else P.NAME_TO_CMD[command]
            if expect is not None:
                want = expect if isinstance(expect, int) else P.NAME_TO_CMD[expect]
            exp_rid = int(getattr(proto_message, "request_id", 0) or 0) if proto_message is not None else 0
            if strict and not exp_rid:
                raise QCError(f"{command}: a strict request needs a request_id")
            self.send(command, proto_message, proto_bytes)
            deadline = time.time() + timeout_ms / 1000.0
            best = None
            while time.time() < deadline:
                self._collect(0.1)
                keep = []
                for i, item in enumerate(self._pending):
                    cmd, obj, raw, pb = item
                    if cmd == want:
                        rid = int(getattr(obj, "request_id", 0) or 0) if obj is not None else 0
                        if exp_rid and rid == exp_rid and obj is not None and len(pb) > 2:
                            # what upstream meant: the unconsumed messages so far, then everything
                            # after the match (stale replies before it are dropped, as below)
                            self._pending = keep + self._pending[i + 1:]
                            return cmd, obj, raw
                        if not exp_rid and obj is not None and len(pb) > 2:
                            self._pending = keep + self._pending[i + 1:]  # consumed here too
                            return cmd, obj, raw
                        if not strict and rid == 0:
                            best = best or (cmd, obj, raw)  # no id at all: the unit may not echo ids
                        # dropped otherwise: another request's reply, or one we can't attribute
                    else:
                        keep.append(item)
                self._pending = keep
            if best:
                return best
            if strict:
                raise QCError(f"no reply to {command} carrying request_id {exp_rid} within {timeout_ms}ms")
            raise QCError(f"no response to {command} within {timeout_ms}ms")

    return request


def _make_setlist_position(mods):
    """get_setlist_position, reading strictly: only the reply to this READ counts.

    Upstream's goes through read_state -> request(), whose fallback could hand back a late reply
    to an earlier position READ. The push decides "is the target slot still the open one" from
    this value several times, so a stale answer could let it build into a different blank preset.
    If the unit doesn't echo request_id at all, every read fails here and pushes fail closed
    (README hardware step 10 checks this before the first real push).
    """
    P = mods["protocol"]

    def get_setlist_position(self, timeout_ms=2500):
        msg = P.message_class("SetlistPosition")(action=P.ACTION["READ"], request_id=self.next_request_id())
        _c, obj, _r = self.request("SetlistPosition", msg, timeout_ms=timeout_ms, strict=True)
        return {"folder_key": obj.folder_key, "position": obj.position, "is_factory": obj.is_factory}

    return get_setlist_position


MAX_LISTING_MESSAGES = 2048
MAX_LISTING_FILES = 50_000


def _make_list_directory_raw(mods):
    """The vendored list_directory's READ and collect loop, returning the raw File messages.

    Upstream hands them to directory.structure_directory, which keeps neither a capture's
    ProductData.version nor the folder's is_cloud/is_favorites flags nor a preset's dependency
    lists, all of which capture placement needs (qc_agent.captures). The READ sent is the same
    bare {action, request_id} the wire guard allows. Bounded: a stream larger than the caps is
    refused rather than held in memory.
    """
    P = mods["protocol"]

    def list_directory_raw(self, timeout_ms=45000, quiet_ms=3000):
        with self._io_lock:  # upstream's @_serialized
            cls = P.message_class("File")
            m = cls(action=P.ACTION["READ"], request_id=self.next_request_id())
            self.send("File", m)
            want = P.NAME_TO_CMD["File"]
            deadline = time.time() + timeout_ms / 1000.0
            last = time.time()
            folders, nfiles = [], 0
            while time.time() < deadline:
                self._collect(0.15)
                got = False
                keep = []
                for item in self._pending:
                    cmd, obj, _raw, _pb = item
                    if cmd == want and obj is not None and obj.HasField("folder"):
                        folders.append(obj)
                        nfiles += len(obj.folder.files)
                        got = True
                    else:
                        keep.append(item)
                self._pending = keep
                if len(folders) > MAX_LISTING_MESSAGES or nfiles > MAX_LISTING_FILES:
                    raise DeviceError("the unit's directory listing is too large")
                if got:
                    last = time.time()
                elif folders and (time.time() - last) * 1000 > quiet_ms:
                    break
            return folders

    return list_directory_raw


def directory_snapshot(file_msgs):
    """Raw File UPDATE messages -> captures.DirectorySnapshot (My Presets, its dependencies, captures).

    Later messages for a folder key replace earlier ones, as upstream's structure_directory does."""
    presets, caps = {}, {}
    for m in file_msgs:
        if not m.HasField("folder"):
            continue
        if m.type == 0:
            presets[m.folder.key] = m.folder
        elif m.type == 2:
            caps[m.folder.key] = m.folder
    mine = [f for k, f in presets.items() if k == SETLIST_KEY and not f.is_factory]
    if len(mine) != 1:
        raise DeviceError("My Presets is missing from the unit's directory listing")
    files = list(mine[0].files)
    # A setlist is a fixed 256-slot table, empty slots included (G0 saw 256). A shorter or longer
    # table would make a missing slot look empty, so it is refused rather than guessed at.
    if len(files) != 256:
        raise DeviceError(f"My Presets listed {len(files)} slots, not 256")
    my = {i: f.name or "" for i, f in enumerate(files)}  # in a READ the array position is the index
    deps = {i: frozenset(d.hash.lower() for d in (*f.factory_dependencies, *f.product_dependencies))
            for i, f in enumerate(files) if f.name}
    entries, folders = [], []
    for key, fo in caps.items():
        folders.append((fo.name, bool(fo.is_factory), len(fo.files)))
        for f in fo.files:
            entries.append(CaptureEntry(name=f.name, key=f.key, folder_key=key, folder_name=fo.name,
                                        is_factory=bool(fo.is_factory), is_cloud=bool(fo.is_cloud),
                                        is_favorites=bool(fo.is_favorites), version_field=f.version))
    return DirectorySnapshot(my_presets=my, preset_deps=deps, captures=entries, capture_folders=folders)


def _fixed_quad_cortex(mods):
    """Upstream's QuadCortex with request() and get_setlist_position() replaced (see above).

    A subclass rather than per-instance patching: the vendored module stays byte-identical, and
    every method that calls self.request() (scene reads, directory listing, the push writes) gets
    the fixed one. It also adds list_directory_raw (capture placement needs fields the vendored
    parser drops), which stays even when the request fix lands upstream.
    """
    class FixedQuadCortex(mods["transport"].QuadCortex):
        request = _make_request(mods)
        get_setlist_position = _make_setlist_position(mods)
        list_directory_raw = _make_list_directory_raw(mods)

    return FixedQuadCortex


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
    def __init__(self, vendor_dir: Path = VENDOR_DIR, mods: dict | None = None):
        # `mods` lets a caller pass modules load_vendored() already returned; nothing else is accepted
        # as the transport, so the pin check can't be skipped by accident.
        self._mods = mods if mods is not None and mods is _loaded else load_vendored(vendor_dir)
        QC = self._mods["transport"].QuadCortex
        if (getattr(QC, "CAPTURE_V1", None), getattr(QC, "CAPTURE_V2", None),
                getattr(QC, "CAPTURE_FILE_NAME_PARAM", None)) != (CAPTURE_BLOCK_IDS[1], CAPTURE_BLOCK_IDS[2],
                                                                  CAPTURE_FILE_NAME_PARAM):
            raise DeviceError("the vendored transport's capture constants differ from the agent's")
        self._QC = _fixed_quad_cortex(self._mods)
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
        P = self._mods["protocol"]
        try:
            qc = self._QC(share=False, bridge=False)  # exclusive direct mode only
        except RuntimeError as e:
            raise UnitAbsent(str(e)) from None
        guard = WireGuard(qc.io, P)
        qc.io = guard
        self._raw, self._guard, self._errs = qc, guard, []
        self._census_logged = False
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

    def read_directory(self):
        snap = directory_snapshot(self._call("list_directory_raw"))
        if not getattr(self, "_census_logged", False):  # once per session: a push lists several times
            _log_census(snap)
            self._census_logged = True
        return snap

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
        self._save_target = None
        if self._guard is not None:
            self._guard.disarm()

    def set_routing(self, row, in_port, out_port):
        self._call("set_routing", row, in_portid=in_port, out_portid=out_port)

    def add_block(self, row, col, model_id):
        return bool(self._call("add_block", model_id, row=row, column=col))

    def place_capture(self, row, col, model_id, key, name):
        version = {CAPTURE_BLOCK_IDS[1]: "v1", CAPTURE_BLOCK_IDS[2]: "v2"}.get(model_id)
        if version is None:
            raise DeviceError(f"model {model_id} is not a capture block")
        push = self._guard._push if self._guard is not None else None
        if not push or push["strings"].get((col, CAPTURE_FILE_NAME_PARAM)) != key + name:
            raise DeviceError(f"column {col}: the capture to place is not the armed one")
        return bool(self._call("set_capture", row, col, key, name, version=version))

    def set_param(self, row, col, index, wire):
        self._call("set_param", row, col, index, wire)

    def set_param_scenes(self, row, col, index, wires):
        self._call("set_param_scenes", row, col, index, list(wires))

    def set_block_bypass(self, row, col, scenes):
        # Always the per-scene path G6 verified (assign to scenes, then each scene's flag
        # explicitly), even when all eight agree: unwritten scenes keep their old value.
        # The bypass control's index is one past the block's own read (qc-mcp bypass_param_index); read it
        # here and arm exactly that index, rather than letting the guard accept any index past the catalog.
        preset = self.read_current_preset()
        chain = next((c for c in (preset or {}).get("chains", []) if c["row"] == row), None)
        block = next((b for b in (chain or {}).get("blocks", []) if b["col"] == col), None)
        if block is None:
            raise DeviceError(f"no block at row {row} column {col} to bypass")
        n = len(block["params"])
        if n <= self._guard._push["max_index"].get(col, 0):
            raise DeviceError(f"column {col} reads {n} parameters, fewer than the catalog lists")
        self._guard.allow_bypass_index(col, n)
        self._call("set_block_bypass", row, col, scenes=[bool(x) for x in scenes], bypass_param=n)

    def save_to_slot(self, position, name):
        if getattr(self, "_save_target", None) != (position, name):
            raise DeviceError("save target differs from the armed push")
        self._save_target = None  # one save per push
        self._guard.arm_save(SETLIST_KEY, position, name)
        try:
            self._call("write_preset_file", SETLIST_KEY, position, name)
        finally:
            self._guard._save = None


def _log_census(snap):
    """What capture placement depends on, for P6 to read from the agent log. Folder names come
    from the unit: cleaned and bounded."""
    from . import text
    by_folder = {}
    for e in snap.captures:
        by_folder.setdefault((e.folder_name, e.is_factory, e.is_cloud, e.is_favorites), set()).add(e.version_field)
    for (name, fac, cloud, fav), versions in sorted(by_folder.items(), key=lambda kv: kv[0][0])[:40]:
        shown = sorted(text.clean(v, 16) for v in versions)[:5]
        log.info("capture folder %r: factory=%s cloud=%s favorites=%s version values %s", text.clean(name, 40),
                 fac, cloud, fav, shown)
    log.info("My Presets: %d saved presets list dependencies", sum(1 for d in snap.preset_deps.values() if d))


def _quiet(fn):
    try:
        fn()
    except Exception:  # noqa: BLE001
        pass
