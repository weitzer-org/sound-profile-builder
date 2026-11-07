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
   its one string write is held to the armed capture by the wire guard. assign_stomp stays on
   the NEVER list too; the only way past it is QcMcpBackend.assign_stomp, which opens it for one
   call while the wire guard holds that exact assignment in a push's stomp step (stomp doc
   section 6.1, entry point 1): Guarded.window yields a scoped callable, dead once its with-block
   exits, opened around that one call, and keeps no state of its own. unassign_stomp is never
   reachable.
3. WireGuard sits between the transport and the HID device and decodes every
   outgoing message before it is written. Only these pass:
     * session plumbing the handshake sends (KeepAlive, ResetCommsBuffers,
       Connection, the Version read and client-version announce);
     * READs of state (preset, directory, setlist position, scene, and the
       handshake's subscription reads);
     * while a push is armed: Grid UPDATEs on row 0 that place exactly the plan's
       models in the plan's columns, set the plan's parameter indexes (always below the
       model's param_count, so never the device's trailing slot) to exactly the plan's
       float values in 0..1, assign to scenes only the plan's scene-varying params and
       each block's bypass control (index param_count + 1, and only once a read of the unit
       confirmed that block's count: confirm_layout), set routing in 1 -> out 19,
       or set the plan's bypass flags; write each capture block's file_name (index 5)
       exactly once, as the exact string the push resolved from the unit's own listing; and
       Scene UPDATEs (the transport switches scenes to write per-scene values);
     * in a push's stomp step (arm_stomps, after the values read back clean, before the save): only
       Grid UPDATEs carrying exactly one stomp_mode_assignments entry, each the plan's (row 0, a block
       the push placed and a read of the unit confirmed, switch index 0-6, never H = 7), PRIMARY,
       once. No momentary flag, no secondary assignment, no delete. From then on no value, bypass,
       routing or scene write passes for the rest of the push;
     * while a save is armed: one File CREATE of My Presets at the armed position
       with the armed name and nothing else (no payload);
     * while an EDIT is armed (arm_edit, preset-editing E4): Grid UPDATEs on row 0 that carry only the
       planned (column, index) float values, the planned "assign to scenes" and bypass-control writes of
       blocks the plan names, and the planned bypass flags; no routing, add, delete, string, stomp, name
       or any other field; and Scene UPDATEs. Two phases: "apply" allows exactly the plan's new values
       and flags, and the one-way "rollback" phase exactly the values the plan expected (the old ones),
       only for ops whose write was started. An armed edit allows no File CREATE unless it was armed
       WITH a save (copy mode only, E5a): then exactly one File CREATE in the armed collection, at the
       armed position, with the armed name and no payload, and only after the agent has reported the
       verify and the collateral check clean (open_edit_save) with every planned op written and no
       rollback; once that save is open the guard passes no further value, bypass or scene write. A live
       edit is armed without one, so its File CREATE is refused.
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
the Phase 0 spikes did; nothing is written to them. A watchdog thread samples them once
a second while a session is open (G2: a call can block for seconds while the unit goes
away, so the job thread can't be the one that notices); it never calls into the
transport, and alive() only reads its verdict, so it never blocks either.
"""
from __future__ import annotations

import contextlib
import logging
import sys
import threading
import time
import types
from pathlib import Path

from . import text
from .captures import FILE_NAME_PARAM as CAPTURE_FILE_NAME_PARAM
from .captures import CaptureEntry, DirectorySnapshot, factory_version_of, version_of
from .device import (SETLIST_KEY, STOMP_PRIMARY, DeviceBackend, DeviceError, PartialListing, Presence,
                     UnitAbsent, UnitBusy)
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
    "set_scene_label", "set_scene_color", "set_preset_meta", "set_scene",
    "latest_broadcast", "read_message", "list_model_presets", "list_recents_favorites",
})
# On the NEVER list, but QcMcpBackend.assign_stomp opens a scoped callable around one call during a push's
# stomp step (Guarded.window); the wire guard then holds that call to one armed assignment.
PUSH_STEP_ONLY = frozenset({"assign_stomp", "set_scene"})  # set_scene: only QcMcpBackend.restore_scene, during an armed edit
GRANT = frozenset({
    "open", "close", "get_current_preset", "get_setlist_position", "list_directory_raw",
    "set_routing", "add_block", "set_param", "set_param_scenes", "set_block_bypass", "write_preset_file",
    "read_state",  # the wire guard passes only the READs in READABLE (the Scene READ is what an edit uses)
    "set_capture", "set_param_typed",  # typed: only a planned cab mic string passes the wire guard
})
ATTRS = frozenset({"firmware", "device_type", "write_errors", "last_write_error", "io"})


class Guarded:
    def __init__(self, qc, grant=GRANT):
        bad = set(grant) & NEVER
        if bad:
            raise GuardError(f"grant includes never-allowed methods: {sorted(bad)}")
        object.__setattr__(self, "_qc", qc)
        object.__setattr__(self, "_grant", frozenset(grant))

    @contextlib.contextmanager
    def window(self, name):
        """Yield a scoped capability: a callable for one PUSH_STEP_ONLY method, valid only inside the
        `with` block (it refuses once the block exits, also after an exception). Guarded itself keeps no
        state for it, so the method stays never-allowed through Guarded's attributes, on every thread, even
        while a window is open; only the code holding the yielded callable can make the call."""
        if name not in PUSH_STEP_ONLY:
            raise GuardError(f"{name} can't be opened")
        method = getattr(self._qc, name)
        live = [True]

        def call(*args, **kwargs):
            if not live[0]:
                raise GuardError(f"the {name} window is closed")
            return method(*args, **kwargs)
        try:
            yield call
        finally:
            live[0] = False

    def __getattr__(self, name):
        if name in NEVER:
            raise GuardError(f"{name} is never allowed")
        if name in self._grant or name in ATTRS:
            return getattr(self._qc, name)
        raise GuardError(f"{name} is not granted")

    def __setattr__(self, name, value):
        raise GuardError("the guarded transport is read-only")


# ---------------------------------------------------------------- wire guard

MAX_STOMP_INDEX = 7  # stomp_index 7 is H (qc-mcp: 0-7 = A-H, UNVERIFIED on the unit): never assigned

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
        self._push = None   # the armed plan, see arm_push
        self._save = None   # (folder_key, position, name)
        self._stomps = None  # the push's stomp step, see arm_stomps: {(row, col): stomp_index} not yet sent
        self._edit = None   # the armed edit, see arm_edit
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
        if self._edit is not None:
            raise WireRefused("a push can't be armed while an edit is armed")
        for b in plan.blocks:
            if b.model_id == 0 or (b.capture is not None and b.capture.file_name is None):
                raise DeviceError(f"column {b.col}: a capture block is not resolved")
        self._push = {"cols": {b.col: b.model_id for b in plan.blocks},
                      # each capture block's file_name, and each cab mic string (plan 4.2, P4b): written
                      # once, exactly this string
                      "strings": {**{(b.col, CAPTURE_FILE_NAME_PARAM): b.capture.file_name
                                     for b in plan.blocks if b.capture is not None},
                                  **{(b.col, m.index): m.value for b in plan.blocks for m in b.mics}},
                      "mic_cells": {(b.col, m.index) for b in plan.blocks for m in b.mics},
                      "params": {(b.col, p.index) for b in plan.blocks for p in b.params},
                      "scene_params": {(b.col, p.index) for b in plan.blocks for p in b.params if p.scene_wires},
                      # no value or string write at or past param_count (G1: the device's trailing slot)
                      "param_count": {b.col: b.param_count for b in plan.blocks},
                      # every block gets all eight bypass flags (G6), through its own bypass control at
                      # param_count + 1. That index is NOT allowed until a read of the unit confirms the block
                      # reports exactly param_count + 1 params (confirm_layout): the guard never trusts the
                      # catalog alone for it, since on a block with more params it would be a real control.
                      "bypass_cols": {b.col for b in plan.blocks},
                      "bypass_expected": {b.col: b.bypass_index for b in plan.blocks},
                      "bypass_index": {},  # col -> confirmed by confirm_layout
                      # the exact values each write may carry (float32 on the wire, so compared within 1e-6)
                      "values": {(b.col, p.index): {p.wire, *(p.scene_wires or ())}
                                 for b in plan.blocks for p in b.params},
                      "flags": {b.col: set(b.bypass_scenes) for b in plan.blocks},
                      "row": plan.row, "in": plan.in_port, "out": plan.out_port}

    def arm_edit(self, plan, checked, save=None):
        """Arm an edit (E4): `plan` is the parsed edit plan, `checked` the CheckedPlan edit_checks made of it.
        Nothing else is allowed for the session until disarm(): not with a push armed, not twice.
        `save` is None for a live edit. A copy (E5a) passes (folder_key, position, name): the one File CREATE it
        may later send. It stays closed until open_edit_save()."""
        if self._push is not None or self._edit is not None or self._save is not None:
            raise WireRefused("an edit can only be armed on its own, once")
        save_target = None
        if save is not None:
            folder_key, position, sname = save
            if (not isinstance(folder_key, str) or not folder_key or type(position) is not int
                    or not 0 <= position < 256 or not isinstance(sname, str) or not sname):
                raise WireRefused("the edit's save target is malformed")
            save_target = {"folder_key": folder_key, "position": position, "name": sname, "state": "armed"}
        ops = {}
        for op in plan["param_ops"]:
            k = (op["col"], op["index"])
            c = checked.params.get(k)
            if c is None or op["row"] != 0:
                raise WireRefused(f"column {op['col']} index {op['index']}: the op was not checked")
            ops[k] = {"wires": tuple(float(x) for x in op["wire"]), "expect": tuple(float(x) for x in op["expect"]),
                      "scene_mode": bool(op["scene_mode"]), "param_count": c["param_count"]}
        byp = {}
        for op in plan["bypass_ops"]:
            c = checked.bypass.get(op["col"])
            if c is None or op["row"] != 0:
                raise WireRefused(f"column {op['col']}: the bypass op was not checked")
            byp[op["col"]] = {"index": c["index"], "flags": frozenset(bool(x) for x in op["bypass"]),
                              "expect": frozenset(bool(x) for x in op["expect"])}
        self._edit = {"params": ops, "bypass": byp, "cells": dict(checked.cells), "confirmed": {},
                      "phase": "apply", "attempted": set(), "save": save_target,
                      # what "every planned op was written" means, for open_edit_save
                      "planned": {(c, i) for c, i in ops} | {(c, "bypass") for c in byp}}

    def open_edit_save(self):
        """The agent's verify and collateral check are clean: allow the ONE File CREATE this edit was armed with.
        Refused when the edit has no save (live), after a rollback, when an op was never written (the verify can't
        have passed), and when it was already opened. From here on the edit passes no more value, bypass or scene
        write."""
        edit = self._edit
        if edit is None:
            raise WireRefused("no armed edit")
        sv = edit["save"]
        if sv is None:
            raise WireRefused("this edit has no save armed (only a copy saves)")
        if edit["phase"] != "apply":
            raise WireRefused("the save can't be opened after a rollback")
        if sv["state"] != "armed":
            raise WireRefused("the save was already opened")
        if not edit["planned"] <= edit["attempted"]:
            raise WireRefused("the save can't be opened before every planned op was written")
        sv["state"] = "open"

    def _edit_writes_closed(self):
        sv = self._edit["save"] if self._edit is not None else None
        return sv is not None and sv["state"] != "armed"

    def enter_rollback(self):
        """One way: from now on only the old values of ops that were started may be written."""
        if self._edit is None:
            raise WireRefused("no armed edit")
        self._edit["phase"] = "rollback"
        self._edit["confirmed"].clear()   # the rollback re-confirms each column from the read taken after the writes

    def confirmed_bypass_index(self, col):
        """The bypass control index of `col` once a read confirmed its layout (push or edit), else None."""
        if self._edit is not None:
            return self._edit["confirmed"].get(col) if col in self._edit["bypass"] else None
        if self._push is not None:
            return self._push["bypass_index"].get(col)
        return None

    def confirm_layout(self, col, n):
        """The unit read `n` params for the planned block in `col`: allow its bypass control only if
        that is the plan's param_count + 1 (G1). Anything else raises and allows nothing."""
        edit = self._edit
        if edit is not None:
            if col not in edit["cells"]:
                raise WireRefused(f"no armed block in column {col}")
            want = edit["cells"][col][1] + 1
            if n != want:
                edit["confirmed"].pop(col, None)
                raise WireRefused(f"column {col} reads {n} params, not the plan's {want}")
            edit["confirmed"][col] = n
            return
        push = self._push
        if push is None or col not in push["bypass_expected"]:
            raise WireRefused(f"no armed block in column {col}")
        if n != push["bypass_expected"][col]:
            push["bypass_index"].pop(col, None)
            raise WireRefused(f"column {col} reads {n} params, not the plan's {push['bypass_expected'][col]}")
        push["bypass_index"][col] = n

    def arm_stomps(self, stomps):
        """Enter the push's stomp step (stomp doc 6.1, entry point 1): `stomps` is [(row, col, index)].
        Only for blocks this push placed whose layout a read of the unit confirmed (confirm_layout), row 0,
        index 0-6 (H = 7 is the owner's switch), one switch per block. Once entered, the build is over:
        the guard passes no further value, bypass, routing or scene write for this push."""
        push = self._push
        if push is None or self._edit is not None or self._stomps is not None or self._save is not None:
            raise WireRefused("stomps can only be armed once, during a push, before the save")
        armed = {}
        for row, col, index in stomps:
            if row != push["row"] or col not in push["cols"] or col not in push["bypass_index"]:
                raise WireRefused(f"stomp at row {row} column {col}: not a block this push placed and confirmed")
            if not 0 <= index < MAX_STOMP_INDEX or (row, col) in armed:
                raise WireRefused(f"stomp at column {col}: switch index {index} is not allowed")
            armed[(row, col)] = index
        if not armed:
            raise WireRefused("no stomps to arm")
        self._stomps = armed

    def stomp_armed(self, row, col, index) -> bool:
        return self._stomps is not None and self._stomps.get((row, col)) == index

    def arm_save(self, folder_key, position, name):
        if self._edit is not None:
            raise WireRefused("a save can't be armed while an edit is armed")
        self._save = (folder_key, position, name)

    def disarm(self):
        self._push = None
        self._save = None
        self._stomps = None
        self._edit = None

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
        if name == "File" and self._edit is not None and action == A["CREATE"]:
            return self._check_edit_save(msg, fields)
        if name == "Grid" and self._edit is not None and action == A["UPDATE"]:
            if self._edit_writes_closed():
                raise WireRefused("Grid: no edit write once the save is open")
            return self._check_edit_grid(msg, fields)
        if name == "Scene" and self._edit is not None and action == A["UPDATE"]:
            if self._edit_writes_closed():
                raise WireRefused("Scene: no scene switch once the save is open")
            if fields <= {"action", "request_id", "selected_scene"} and 0 <= msg.selected_scene < 8:
                return  # the transport switches scenes to write per-scene values
            raise WireRefused("Scene: only a scene switch A-H")
        if name == "Grid" and self._push is not None and action == A["UPDATE"] and self._stomps is not None:
            return self._check_stomp(msg, fields)
        if name == "Grid" and self._push is not None and action == A["UPDATE"]:
            return self._check_grid(msg, fields)
        if name == "Scene" and self._push is not None and self._stomps is None and action == A["UPDATE"]:
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

    def _check_edit_grid(self, msg, fields):
        """An armed edit's Grid UPDATE: only planned values, planned scene assignments, planned flags."""
        if "preset" not in fields or not fields <= {"action", "request_id", "preset"}:
            raise WireRefused(f"Grid: unexpected fields {sorted(fields)} in an edit")
        preset = msg.preset
        pf = _names(preset)
        if not pf or not pf <= {"chains", "bypass"}:
            raise WireRefused(f"Grid: preset fields {sorted(pf)} not allowed in an edit")
        if len(preset.chains) > 1 or len(preset.bypass) > 1:
            raise WireRefused("Grid: one row of values or flags per message in an edit")
        for ch in preset.chains:
            cf = _names(ch)
            if not cf <= {"row", "models"} or ch.row != 0 or len(ch.models) != 1:
                raise WireRefused(f"Grid: an edit writes one block on row 0 only (chain fields {sorted(cf)})")
            self._check_edit_model(ch.models[0])
        for b in preset.bypass:
            self._check_edit_bypass(b)

    def _edit_value_ok(self, v, allowed):
        return 0.0 <= v <= 1.0 and any(abs(v - a) <= 1e-6 for a in allowed)

    def _check_edit_model(self, m):
        edit = self._edit
        mf = _names(m)
        if not mf <= {"column", "params"} or "params" not in mf or len(m.params) != 1:
            # "hash" would place or delete a block; an entry with no params is the shape of a delete
            raise WireRefused(f"Grid: an edit writes one parameter of one block (model fields {sorted(mf)})")
        col = m.column
        p = m.params[0]
        if col not in edit["cells"]:
            raise WireRefused(f"Grid: column {col} is not in the plan")
        qf = _names(p)
        rollback = edit["phase"] == "rollback"
        if qf == {"scene_mode"} or qf == {"index", "scene_mode"}:
            # "Assign to Scenes", and only ever on: a scene-varying op's control, or a bypass op's bypass control
            if not p.scene_mode:
                raise WireRefused("Grid: an edit never takes a control out of scenes")
            op = edit["params"].get((col, p.index))
            if op is not None and op["scene_mode"]:
                if col not in edit["confirmed"]:
                    raise WireRefused(f"Grid: column {col} was not confirmed by a read of the unit")
                self._edit_started(col, p.index, rollback)
                return
            b = edit["bypass"].get(col)
            if b is not None and p.index == edit["confirmed"].get(col) == b["index"]:
                self._edit_started(col, "bypass", rollback)
                return
            raise WireRefused(f"Grid: assigning column {col} index {p.index} to scenes is not the plan")
        if not qf <= {"index", "param_values"} or "param_values" not in qf:
            raise WireRefused(f"Grid: parameter fields {sorted(qf)} not allowed in an edit")
        op = edit["params"].get((col, p.index))
        if op is None:
            raise WireRefused(f"Grid: writing column {col} index {p.index} is not the plan")
        if col not in edit["confirmed"]:
            raise WireRefused(f"Grid: column {col} was not confirmed by a read of the unit")
        if not 0 <= p.index < op["param_count"]:
            raise WireRefused(f"Grid: column {col} index {p.index} is at or past param_count (the trailing slot)")
        if len(p.param_values) != 1 or p.param_values[0].WhichOneof("value") != "float_value":
            raise WireRefused("Grid: parameter writes must be one float")
        v = p.param_values[0].float_value
        if not 0.0 <= v <= 1.0:
            raise WireRefused(f"Grid: wire value {v} outside 0..1")
        if not self._edit_value_ok(v, op["expect"] if rollback else op["wires"]):
            raise WireRefused(f"Grid: column {col} index {p.index} value {v} is not the plan's"
                              + (" old value" if rollback else ""))
        self._edit_started(col, p.index, rollback)

    def _edit_started(self, col, what, rollback):
        """Record that a write of this op began (apply), or require that it had (rollback)."""
        edit = self._edit
        if rollback:
            if (col, what) not in edit["attempted"]:
                raise WireRefused(f"Grid: column {col} {what} was never written, so there is nothing to roll back")
        else:
            edit["attempted"].add((col, what))

    def _check_edit_bypass(self, b):
        edit = self._edit
        if _names(b) - {"row", "colBypass"} or b.row != 0 or len(b.colBypass) != 1:
            raise WireRefused("Grid: an edit writes one bypass cell on row 0")
        cb = b.colBypass[0]
        if _names(cb) - {"column", "sceneBypass"} or len(cb.sceneBypass) != 1:
            raise WireRefused("Grid: unexpected bypass fields")
        op = edit["bypass"].get(cb.column)
        if op is None:
            raise WireRefused(f"Grid: bypass of column {cb.column} is not the plan")
        if cb.column not in edit["confirmed"]:
            raise WireRefused(f"Grid: column {cb.column} was not confirmed by a read of the unit")
        sb = cb.sceneBypass[0]
        if _names(sb) - {"bypass"}:
            raise WireRefused("Grid: unexpected bypass fields")
        rollback = edit["phase"] == "rollback"
        if bool(sb.bypass) not in (op["expect"] if rollback else op["flags"]):
            raise WireRefused(f"Grid: bypass {sb.bypass} on column {cb.column} is not the plan's")
        self._edit_started(cb.column, "bypass", rollback)

    def _check_edit_save(self, msg, fields):
        """A copy's one File CREATE (E5a): only once opened, only in the armed collection, at the armed position,
        under the armed name, with no payload."""
        sv = self._edit["save"]
        if sv is None:
            raise WireRefused("File CREATE: this edit has no save armed")
        if sv["state"] != "open":
            raise WireRefused("File CREATE: the edit's save is not open" if sv["state"] == "armed"
                              else "File CREATE: the edit's one save was already sent")
        if not fields <= {"action", "request_id", "folder"}:
            raise WireRefused(f"File CREATE: unexpected fields {sorted(fields)}")
        f = msg.folder
        if _names(f) - {"key", "files"} or f.key != sv["folder_key"] or len(f.files) != 1:
            raise WireRefused("File CREATE: only one file, in the armed folder")
        fi = f.files[0]
        if _names(fi) - {"index", "name"} or fi.index != sv["position"] or fi.name != sv["name"]:
            raise WireRefused("File CREATE: position or name differs from the armed save")
        sv["state"] = "sent"   # one save per arming

    def _check_stomp(self, msg, fields):
        """The stomp step: one armed PRIMARY assignment per message, each once (arm_stomps)."""
        if fields != {"action", "request_id", "preset"}:
            raise WireRefused(f"Grid: unexpected fields {sorted(fields)} in the stomp step")
        preset = msg.preset
        if _names(preset) != {"stomp_mode_assignments"} or len(preset.stomp_mode_assignments) != 1:
            raise WireRefused("Grid: the stomp step allows exactly one footswitch assignment per message")
        a = preset.stomp_mode_assignments[0]
        if _names(a) - {"row", "column", "stomp_index", "type"} or "type" not in _names(a):
            # type must be explicit (the vendored assign_stomp always sets it): how the unit reads an absent one
            # is unverified, and an all-default entry is the shape a delete would have
            raise WireRefused("Grid: unexpected or missing footswitch assignment fields")
        if a.type != STOMP_PRIMARY:  # latching on/off only: never the Dual Footswitch secondary function
            raise WireRefused("Grid: only primary footswitch assignments")
        key = (a.row, a.column)
        if self._stomps.get(key) != a.stomp_index or a.stomp_index >= MAX_STOMP_INDEX:
            raise WireRefused(f"Grid: footswitch {a.stomp_index} on row {a.row} column {a.column} is not the plan's")
        del self._stomps[key]  # once

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
            # "Assign to Scenes": only for the plan's scene-varying params, or a block's bypass control.
            if (col, p.index) in push["scene_params"]:
                return
            if col in push["bypass_cols"] and p.index == push["bypass_index"].get(col):
                return  # the bypass control: param_count + 1, confirmed from the unit's read (confirm_layout)
            raise WireRefused(f"Grid: assigning column {col} index {p.index} to scenes is not the plan")
        if "index" in qf and not 0 <= p.index < push["param_count"][col]:
            raise WireRefused(f"Grid: column {col} index {p.index} is at or past param_count (the trailing slot)")
        if qf == {"index", "param_values"} and (col, p.index) in push["strings"]:
            want = push["strings"][(col, p.index)]
            if (len(p.param_values) != 1 or p.param_values[0].WhichOneof("value") != "string_value"
                    or p.param_values[0].string_value != want):
                raise WireRefused(f"Grid: column {col} index {p.index} may only carry the planned string")
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
            raise WireRefused("File CREATE: only one file, in the armed folder")
        fi = f.files[0]
        if _names(fi) - {"index", "name"} or fi.index != position or fi.name != name:
            raise WireRefused("File CREATE: position or name differs from the armed save")
        self._save = None  # one save per arming


# ---------------------------------------------------------------- a fix to one transport method

MAX_PENDING = 4096  # unconsumed messages kept between reads (broadcasts nobody asks for, stray replies)


def _bounded(keep):
    """The unconsumed queue, newest MAX_PENDING kept. Called only on what a read loop left after
    taking its own messages, so it never drops a reply or folder that loop is waiting for. Without
    it, broadcasts for commands the agent never requests would grow the queue for the whole session.
    Dropping the oldest is safe: a stale message is never a valid reply (request() drops those too)."""
    return keep[-MAX_PENDING:] if len(keep) > MAX_PENDING else keep


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
            deadline = time.monotonic() + timeout_ms / 1000.0
            best = None
            while time.monotonic() < deadline:
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
                self._pending = _bounded(keep)
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
            want = P.NAME_TO_CMD["File"]
            # A folder message still queued from an earlier listing must not stand in for a fresh one.
            self._pending = [it for it in self._pending
                             if not (it[0] == want and it[1] is not None and it[1].HasField("folder"))]
            cls = P.message_class("File")
            m = cls(action=P.ACTION["READ"], request_id=self.next_request_id())
            self.send("File", m)
            deadline = time.monotonic() + timeout_ms / 1000.0
            last = time.monotonic()
            folders, nfiles = [], 0
            while time.monotonic() < deadline:
                self._collect(0.15)
                got = False
                keep = []
                for item in self._pending:
                    cmd, obj, _raw, _pb = item
                    if cmd == want and obj is not None and obj.HasField("folder"):
                        folders.append(obj)
                        nfiles += len(obj.folder.files)
                        got = True
                    elif (cmd == want and obj is not None
                          and int(getattr(obj, "request_id", 0) or 0) == m.request_id):
                        pass  # this READ's own reply without a folder (an ack, if the unit sends one)
                    else:
                        keep.append(item)
                self._pending = _bounded(keep)
                if len(folders) > MAX_LISTING_MESSAGES or nfiles > MAX_LISTING_FILES:
                    raise DeviceError("the unit's directory listing is too large")
                if got:
                    last = time.monotonic()
                elif folders and (time.monotonic() - last) * 1000 > quiet_ms:
                    return folders  # the stream went quiet: complete
            # Upstream returns whatever arrived by the deadline; a listing cut off mid-stream could hide a
            # preset or a second capture with the same name, so it is refused here.
            raise DeviceError("the unit's directory listing did not finish in time")

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
    my, deps = _slot_table(mine[0], "My Presets")
    # Every other non-factory collection that is a complete table is a place a push may build into when it
    # is the one open on the unit. One that isn't complete is left out (the push then refuses), never guessed.
    folders_by_key = {SETLIST_KEY: ("My Presets", my, deps)}
    for k, f in presets.items():
        if k == SETLIST_KEY or f.is_factory:
            continue
        try:
            table, fdeps = _slot_table(f, "folder")
        except DeviceError:
            continue
        folders_by_key[k] = (text.clean(f.name, 40) or "unnamed", table, fdeps)
    entries, folders, census = [], [], []
    for key, fo in caps.items():
        folders.append((fo.name, bool(fo.is_factory), len(fo.files)))
        cloud, fav = _inherited(key, caps)
        chain, end = _chain(key, caps)
        fver = factory_version_of([c.name for c in chain])
        for f in fo.files:
            entries.append(CaptureEntry(name=f.name, key=f.key, folder_key=key, folder_name=fo.name,
                                        is_factory=bool(fo.is_factory), is_cloud=cloud,
                                        is_favorites=fav, version_field=f.version, factory_version=fver))
        census.append(_census_row(fo, chain, end, cloud, fav, fver))
    return DirectorySnapshot(my_presets=my, preset_deps=deps, captures=entries, capture_folders=folders,
                             folder_census=census, preset_folders=folders_by_key)


def _slot_table(folder, what):
    """One preset folder -> ({position: name}, {position: dependency hashes}), or DeviceError if it isn't a
    complete 256-slot table."""
    files = list(folder.files)
    # A setlist is a fixed 256-slot table, empty slots included (G0 saw 256 for My Presets). A shorter or longer
    # table would make a missing slot look empty, so it is refused rather than guessed at.
    if len(files) != 256:
        if what == "My Presets":
            _log_short_table(files)
            raise PartialListing(len(files))
        raise DeviceError(f"{what} listed {len(files)} slots, not 256")
    # In a READ the array position is the slot and the `index` field is left 0 (upstream's
    # PROTOCOL.md 5, directory._file_dict). A non-zero index that disagrees with the position
    # would mean that rule doesn't hold on this unit: refuse rather than pick one of the two.
    if any(f.index not in (0, i) for i, f in enumerate(files)):
        raise DeviceError(f"{what} listed a slot index that disagrees with its position")
    table = {i: f.name or "" for i, f in enumerate(files)}
    deps = {i: frozenset(d.hash.lower() for d in (*f.factory_dependencies, *f.product_dependencies))
            for i, f in enumerate(files) if f.name}
    return table, deps


def _log_short_table(files, limit=5):
    """Evidence for F1: what a My Presets table that isn't 256 long carries (count, then each entry's index field
    and cleaned name, at most `limit`). Logged before the refusal; the table is never used."""
    shown = "; ".join(f"index={f.index} name={text.clean(f.name, 40)!r}" for f in files[:limit])
    log.warning("My Presets listed %d entries, not 256: %s%s", len(files), shown or "(none)",
                " ..." if len(files) > limit else "")


def _chain(key, folders, depth=16):
    """The folder and the folders above it (parent_key), nearest first, and how the walk ended: "top" (no
    parent), "parent not listed" (a parent_key that no capture folder in the listing has), "loop" or "too deep"."""
    out, seen, k = [], set(), key
    for _ in range(depth):
        fo = folders.get(k)
        if fo is None:
            return out, "parent not listed"
        if k in seen:
            return out, "loop"
        seen.add(k)
        out.append(fo)
        k = fo.parent_key
        if not k:
            return out, "top"
    return out, "too deep"


def _census_row(fo, chain, end, cloud, fav, fver):
    """What the agent log's census says about one capture folder. Device text: cleaned and bounded. The
    ProductData fields beyond `version` (coros_version, device, extraMetadata) are logged, not used: which
    of them, if any, marks a factory capture's V1/V2 is unknown (P6, 2026-11-06)."""
    def vals(attr, n=16):
        return sorted({text.clean(str(getattr(f, attr, "") or ""), n) for f in fo.files})[:5]

    above = [text.clean(c.name, 24) for c in reversed(chain[1:])]
    if end != "top":
        above.insert(0, f"<{end}>")
    return {"name": text.clean(fo.name, 40), "path": " > ".join(above) or "-", "factory": bool(fo.is_factory),
            "cloud": cloud, "favorites": fav, "files": len(fo.files), "version": vals("version"),
            "coros_version": vals("coros_version"), "device": vals("device"),
            "extra_metadata": sum(1 for f in fo.files if getattr(f, "extraMetadata", b"")), "v": fver}


def _inherited(key, folders, depth=16):
    """(cloud, favorites) for a capture folder: set when it or any folder above it (parent_key)
    is a cloud or plugin folder / a favorites view. A loop or a chain deeper than `depth` counts
    as cloud, so the folder's files are never treated as local."""
    cloud = fav = False
    seen = set()
    k = key
    for _ in range(depth):
        fo = folders.get(k)
        if fo is None or k in seen:
            return cloud or k in seen, fav
        seen.add(k)
        cloud = cloud or bool(fo.is_cloud) or bool(fo.is_plugin)
        fav = fav or bool(fo.is_favorites)
        k = fo.parent_key
        if not k:
            return cloud, fav
    return True, fav


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
    # Footswitch assignments as the unit reports them (qc-mcp reads the same two fields; UNVERIFIED on the unit
    # whether a read carries them, so a push with stomps fails closed when they are missing or differ).
    stomps = [{"row": int(a.row), "col": int(a.column), "index": int(a.stomp_index), "type": int(a.type)}
              for a in bp.stomp_mode_assignments]
    momentary = {int(k): bool(v) for k, v in bp.stomp_is_momentary.items()}
    return {"name": bp.name, "chains": chains, "bypass": bypass, "stomps": stomps, "momentary": momentary}


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
        self._gone = threading.Event()
        self._watch_stop = None
        self._watch_thread = None
        self._last_sample = 0.0

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
        self._start_watchdog()

    def disconnect(self):
        self._stop_watchdog()
        qc, self._qc, self._raw = self._raw, None, None
        if qc is None:
            return
        t = threading.Thread(target=lambda: _quiet(qc.close), daemon=True)
        t.start()
        t.join(10.0)
        if t.is_alive():
            log.warning("closing the unit took more than 10 s; continuing")

    # -- liveness (G2 c) --

    WATCH_INTERVAL_S = 1.0
    WATCH_STALE_S = 5.0   # the watchdog's own timeout: a verdict older than this counts as gone

    @staticmethod
    def _reader_thread_dead(qc) -> bool:
        """The one place that reads the vendored transport's private state: WinHIDTransport._thread, its
        HID reader (qc.io is our WireGuard; .inner is the transport). Upstream has no public liveness API
        at f634ecd, and the vendored files can't change without a re-vendoring PR; a test pins this
        attribute in the vendored source. Fails closed: while a session is open the transport always has
        a reader thread (WinHIDTransport sets _thread to None only in close(), which runs after the
        watchdog stops), so a missing one means a renamed attribute or an unexpected close, and both
        count as gone rather than letting the check be skipped."""
        thread = getattr(getattr(qc.io, "inner", None), "_thread", None)
        return thread is None or not thread.is_alive()

    def _watchdog_sample(self):
        """One liveness sample, from attributes only: it never calls into the transport, so it can't
        block on the transport's lock or on the device. Gone when the reader thread is dead, or when the
        write-error count rose on two consecutive samples (G2 c saw both together as the unit went
        away). Sets the verdict and returns whether the unit still looks alive."""
        qc = self._raw
        gone = qc is None
        if not gone:
            if self._reader_thread_dead(qc):
                gone = True
            else:
                self._errs = (self._errs + [qc.write_errors])[-3:]
                e = self._errs
                gone = len(e) == 3 and e[0] < e[1] < e[2]
        if gone:
            if not self._gone.is_set():
                log.warning("the Quad Cortex looks gone (reader thread dead or write errors rising)")
            self._gone.set()
        self._last_sample = time.monotonic()
        return not gone

    def _start_watchdog(self):
        self._gone.clear()
        self._errs = []
        stop = threading.Event()
        self._watch_stop = stop
        self._watchdog_sample()

        def loop():
            while not stop.wait(self.WATCH_INTERVAL_S):
                try:
                    self._watchdog_sample()
                except Exception:  # noqa: BLE001  a broken sample must not look like a live unit
                    log.exception("liveness sample failed")
                    self._gone.set()

        self._watch_thread = threading.Thread(target=loop, name="qc-watchdog", daemon=True)
        self._watch_thread.start()

    def _stop_watchdog(self):
        if self._watch_stop is not None:
            self._watch_stop.set()
        t, self._watch_thread, self._watch_stop = self._watch_thread, None, None
        if t is not None and t is not threading.current_thread():
            t.join(2.0)

    def alive(self):
        """The watchdog's verdict. Never blocks. A verdict older than WATCH_STALE_S (the watchdog
        itself stalled) counts as gone; a successful read is never taken as proof of life."""
        if self._raw is None or self._gone.is_set():
            return False
        return time.monotonic() - self._last_sample <= self.WATCH_STALE_S

    def _call(self, name, *args, **kwargs):
        if self._qc is None:
            raise DeviceError("not connected")
        return self._invoke(lambda: getattr(self._qc, name)(*args, **kwargs))

    def _invoke(self, thunk):
        """Run one transport call, mapping its errors the same way for every call."""
        try:
            return thunk()
        except (DeviceError, GuardError):
            raise
        except self._mods["transport"].QCError as e:
            raise DeviceError(str(e)) from None
        except RuntimeError as e:
            raise UnitAbsent(str(e)) from None

    # -- reads --

    def read_directory(self):
        snap = directory_snapshot(self._call("list_directory_raw"))
        _log_census(snap)  # once per bridge process
        return snap

    def setlist_position(self):
        return self._call("get_setlist_position")

    def read_current_preset(self):
        bp = self._call("get_current_preset")
        return None if bp is None else preset_to_plain(bp)

    # -- writes --

    def begin_push(self, position, name, plan, folder_key=SETLIST_KEY):
        if self._guard is None:
            raise DeviceError("not connected")
        self._guard.arm_push(plan)
        self._save_target = (position, name)
        self._save_folder = folder_key

    def begin_edit(self, plan, checked, save=None):
        """Arm the wire guard for one edit (preset-editing E4). `save` is None for a live edit, which never saves;
        a copy (E5a) passes (folder_key, position, name), the one File CREATE it may send after edit_verified()."""
        if self._guard is None:
            raise DeviceError("not connected")
        self._save_target = None
        self._guard.arm_edit(plan, checked, save)

    def edit_verified(self):
        """The agent's verify and collateral check came back clean: open the copy's one save."""
        if self._guard is None:
            raise DeviceError("not connected")
        self._guard.open_edit_save()

    def save_edit_copy(self, position, name):
        """Send the copy's one File CREATE: the live (edited) grid is committed to `position` in the collection the
        edit was armed with, under `name`. The guard holds the message to exactly the armed save, once."""
        edit = self._guard._edit if self._guard is not None else None
        sv = edit["save"] if edit is not None else None
        if sv is None or sv["state"] != "open" or (position, name) != (sv["position"], sv["name"]):
            raise DeviceError("save target differs from the armed edit")
        try:
            self._call("write_preset_file", sv["folder_key"], position, name)
        finally:
            sv["state"] = "sent"   # whether or not the call finished, this arming never saves again

    def enter_rollback(self):
        if self._guard is None:
            raise DeviceError("not connected")
        self._guard.enter_rollback()

    def edit_started(self):
        edit = self._guard._edit if self._guard is not None else None
        if edit is None:
            return set()
        return {("bypass", k[0]) if k[1] == "bypass" else ("param", k[0], k[1]) for k in edit["attempted"]}

    def read_scene(self):
        obj = self._call("read_state", "Scene", timeout_ms=1500)
        scene = getattr(obj, "selected_scene", None)
        return scene if type(scene) is int and 0 <= scene < 8 else None

    def restore_scene(self, scene):
        if self._guard is None or self._guard._edit is None or self._qc is None:
            raise DeviceError("not armed")
        if type(scene) is not int or not 0 <= scene < 8:
            raise DeviceError("not a scene")
        # set_scene is on the NEVER list; the window opens it for this one call and the wire guard (armed
        # edit only) passes a Scene UPDATE A-H. Nothing else uses it.
        with self._qc.window("set_scene") as select:
            self._invoke(lambda: select(scene))

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

    def set_mic(self, row, col, index, value):
        push = self._guard._push if self._guard is not None else None
        if not push or (col, index) not in push["mic_cells"] or push["strings"].get((col, index)) != value:
            raise DeviceError(f"column {col}: the mic to write is not the armed one")
        # The vendored typed write; the wire guard lets exactly this string through once (_check_param).
        self._call("set_param_typed", row, col, index, [[None, None, value]])

    def set_param(self, row, col, index, wire):
        self._call("set_param", row, col, index, wire)

    def set_param_scenes(self, row, col, index, wires):
        self._call("set_param_scenes", row, col, index, list(wires))

    def confirm_layout(self, col, n):
        if self._guard is None:
            raise DeviceError("not connected")
        self._guard.confirm_layout(col, n)

    def set_block_bypass(self, row, col, scenes, bypass_index):
        # Always the per-scene path G6 verified (assign to scenes, then each scene's flag explicitly), on
        # every block, even when all eight agree: unwritten scenes keep their old value. The index is the
        # block's bypass control, param_count + 1: the push's layout check read param_count + 1 slots for
        # this block on the unit before any value was written, and the guard armed exactly this index.
        if self._guard is None or (self._guard._push is None and self._guard._edit is None):
            raise DeviceError("not armed")
        if bypass_index != self._guard.confirmed_bypass_index(col):
            raise DeviceError(f"column {col}: bypass index {bypass_index} is not the armed one "
                              "(not confirmed from the unit's read)")
        self._call("set_block_bypass", row, col, scenes=[bool(x) for x in scenes], bypass_param=bypass_index)

    STOMP_SETTLE_S = 1.0  # the transport's own default pause after each assignment

    def arm_stomps(self, stomps):
        if self._guard is None or self._guard._push is None:
            raise DeviceError("not armed")
        self._guard.arm_stomps([(s.row, s.col, s.index) for s in stomps])

    def assign_stomp(self, row, col, index):
        # The vendored assign_stomp (transport.py:1172): a Grid UPDATE with one stomp_mode_assignments entry,
        # kind "primary", momentary left None so no stomp_is_momentary message is sent (latching). It is on the
        # NEVER list; the window opens it for this one call, and the wire guard passes only the armed entry.
        if self._guard is None or not self._guard.stomp_armed(row, col, index):
            raise DeviceError(f"column {col}: footswitch {index} is not an armed stomp")
        if self._qc is None:
            raise DeviceError("not connected")
        with self._qc.window("assign_stomp") as assign:
            self._invoke(lambda: assign(row, col, index, kind="primary", momentary=None,
                                        settle_s=self.STOMP_SETTLE_S))

    def save_to_slot(self, position, name):
        if getattr(self, "_save_target", None) != (position, name):
            raise DeviceError("save target differs from the armed push")
        self._save_target = None  # one save per push
        folder_key = getattr(self, "_save_folder", SETLIST_KEY)
        self._guard.arm_save(folder_key, position, name)
        try:
            self._call("write_preset_file", folder_key, position, name)
        finally:
            self._guard._save = None


_CENSUS_LOGGED = False  # once per bridge process: every job reconnects, and the census is long
MAX_CENSUS_LINES = 400


def _log_census(snap):
    """What capture placement depends on, for P6 to read from the agent log: every capture folder with the
    folders above it, its flags, file count, the ProductData fields that might carry V1/V2, and the version a
    push would use. Folder names come from the unit: cleaned and bounded."""
    global _CENSUS_LOGGED
    if _CENSUS_LOGGED:
        return
    _CENSUS_LOGGED = True
    rows = sorted(snap.folder_census, key=lambda r: (r["path"], r["name"]))
    for r in rows[:MAX_CENSUS_LINES]:
        log.info("capture folder %r in %s: factory=%s cloud=%s favorites=%s files=%d version %s coros_version %s "
                 "device %s extra_metadata %d -> %s", r["name"], r["path"], r["factory"], r["cloud"], r["favorites"],
                 r["files"], r["version"], r["coros_version"], r["device"], r["extra_metadata"],
                 f"V{r['v']}" if r["v"] else "no V1/V2 folder")
    if len(rows) > MAX_CENSUS_LINES:
        log.info("capture census: %d more folders not shown", len(rows) - MAX_CENSUS_LINES)
    count = {}
    for e in snap.captures:
        k = ("factory" if e.is_factory else "user", version_of(e))
        count[k] = count.get(k, 0) + 1
    log.info("capture census: %d folders, %d files; factory V1 %d, V2 %d, unknown %d; user V1 %d, V2 %d, unknown %d",
             len(rows), len(snap.captures), *(count.get((kind, v), 0) for kind in ("factory", "user")
                                               for v in (1, 2, None)))
    log.info("My Presets: %d saved presets list dependencies", sum(1 for d in snap.preset_deps.values() if d))


def _quiet(fn):
    try:
        fn()
    except Exception:  # noqa: BLE001
        pass
