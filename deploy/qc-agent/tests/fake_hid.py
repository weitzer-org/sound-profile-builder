"""A Quad Cortex simulated at the HID-report level, for driving the REAL vendored
transport (vendor/qc_mcp/transport.py) offline, through QcMcpBackend and its wire guard.

It answers the messages the agent's paths send (Version, SetlistPosition, RecallPreset
and Scene READs, the File READ directory stream, Grid/Scene UPDATEs, the File CREATE
save) the way the transport expects, and records every decoded message it received in
`received` as (command name, decoded message). It is a model of what the transport
needs, not of the device: it proves our calls and the guard, not the unit.
"""
from __future__ import annotations

import threading
import time

from qc_agent.qcmcp_backend import load_vendored

M = load_vendored()
P = M["protocol"]
SETLIST = "/media/p4/Presets/My Presets"


class _Reader:
    def __init__(self):
        self.running = True

    def is_alive(self):
        return self.running


class FakeHID:
    BENIGN_WRITE_CODES = frozenset({0, 31})

    def __init__(self, firmware="4.1.0", slots=None, open_position=152):
        self.firmware = firmware
        self.slots = dict(slots or {})
        self.open_position = open_position
        self.received: list[tuple[str, object]] = []
        self._rx = []
        self._lock = threading.Lock()
        self._asm = P.Reassembler()
        self.scene = 0
        self.routing = {0: (0, 0)}
        self.grid: dict[int, dict] = {}   # col -> {"hash", "params": {idx: [8]}, "scene": set(idx), "bypass": [8], "bypass_scene": bool}
        self._thread = _Reader()  # stands in for winhid's reader thread (QcMcpBackend.alive reads it)
        self.opened = self.closed = False
        self.echo_ids = True          # False: replies carry no request_id (unverified on the unit, step 10)
        from fake_device import default_captures
        self.captures = default_captures()  # type-2 folders in the directory stream
        self.deps_on_save = True             # a saved preset lists its captures' keys as dependencies
        self.preset_deps: dict[int, list[str]] = {}
        self.name = ""               # the open preset's name in a RecallPreset read
        self.position_replies = True  # False: SetlistPosition READs go unanswered (a slow or busy unit)
        self.stomps: dict[tuple[int, int], tuple[int, int]] = {}  # (row, col) -> (stomp_index, type)
        self.momentary: dict[int, bool] = {}

    # -- the HID API the transport uses --

    def open(self):
        self.opened = True
        return self

    def close(self):
        self.closed = True

    def read_reports(self, timeout=0.1):
        time.sleep(min(timeout or 0, 0.005))
        with self._lock:
            out, self._rx = self._rx, []
        return out

    def set_report(self, report_id, data, include_id=False):
        for msg in self._asm.feed(bytes(data), has_report_id=False):
            cmd, pb, _h = P.decode_message(msg)
            name = P.COMMANDS[cmd]
            obj = P.message_class(cmd).FromString(pb)
            self.received.append((name, obj))
            self._handle(name, obj)
        return 0

    # -- replies --

    def _reply(self, name, obj):
        raw = P.encode_message(name, obj.SerializeToString())
        reports = P.message_to_reports(raw, report_id=P.REPORT_QC_TO_HOST)
        with self._lock:
            self._rx.extend((r[0], r) for r in reports)

    def _n_params(self, col):
        # Fixed per model, like the unit (G1: N = the full list + 1); not derived from what was written.
        from helpers import catalog
        m = catalog().models.get(self.grid[col]["hash"])
        if m and m.get("param_count"):
            return m["param_count"] + 1
        return (max(m["params_by_index"]) if m else 0) + 2

    def _handle(self, name, m):
        A = P.ACTION
        if name == "Version" and m.action == A["READ"]:
            v = P.message_class("Version")(action=A["UPDATE"], request_id=m.request_id, zenos_git_hash=self.firmware)
            v.device_type = v.DESCRIPTOR.fields_by_name["device_type"].enum_type.values_by_name["QC"].number
            self._reply("Version", v)
        elif name == "SetlistPosition" and m.action == A["READ"]:
            if self.position_replies:
                self._reply(name, P.message_class(name)(action=A["UPDATE"],
                                                        request_id=m.request_id if self.echo_ids else 0,
                                                        folder_key=SETLIST, position=self.open_position))
        elif name == "Scene" and m.action == A["READ"]:
            self._reply(name, P.message_class(name)(action=A["UPDATE"], request_id=m.request_id,
                                                    selected_scene=self.scene))
        elif name == "Scene" and m.action == A["UPDATE"]:
            self.scene = m.selected_scene
        elif name == "RecallPreset" and m.action == A["READ"]:
            r = P.message_class(name)(action=A["UPDATE"], request_id=m.request_id)
            self._fill_preset(r.preset)
            self._reply(name, r)
        elif name == "File" and m.action == A["READ"]:
            f = P.message_class("File")(action=A["UPDATE"], type=0)
            f.folder.key, f.folder.name = SETLIST, "My Presets"
            for pos in range(256):
                fi = f.folder.files.add()
                fi.name = self.slots.get(pos, "")
                for h in self.preset_deps.get(pos, []):
                    fi.factory_dependencies.add().hash = h
            self._reply("File", f)
            folders = {}
            for e in self.captures:
                folders.setdefault((e.folder_key, e.folder_name, e.is_factory, e.is_cloud, e.is_favorites), []).append(e)
            for (key, fname, fac, cloud, fav), entries in folders.items():
                c = P.message_class("File")(action=A["UPDATE"], type=2)
                c.folder.key, c.folder.name = key, fname
                c.folder.is_factory, c.folder.is_cloud, c.folder.is_favorites = fac, cloud, fav
                for e in entries:
                    fi = c.folder.files.add()
                    fi.name, fi.key, fi.version = e.name, e.key, e.version_field
                self._reply("File", c)
        elif name == "File" and m.action == A["CREATE"]:
            fi = m.folder.files[0]
            self.slots[fi.index] = fi.name
            self.preset_deps[fi.index] = ([b["strings"][5][:64] for b in self.grid.values() if 5 in b.get("strings", {})]
                                          if self.deps_on_save else [])
        elif name == "Grid" and m.action == A["UPDATE"]:
            self._grid(m)

    def _grid(self, g):
        for a in g.preset.stomp_mode_assignments:  # qc-mcp: a block holds one primary assignment; again moves it
            self.stomps[(a.row, a.column)] = (a.stomp_index, a.type)
        for k, v in g.preset.stomp_is_momentary.items():
            self.momentary[k] = v
        for ch in g.preset.chains:
            if ch.in_portid or ch.out_portid:
                self.routing[ch.row] = (ch.in_portid, ch.out_portid)
            for mm in ch.models:
                col = mm.column
                if mm.hash:
                    self.grid[col] = {"hash": mm.hash, "params": {}, "scene": set(), "bypass": [False] * 8,
                                      "bypass_scene": False}
                    echo = P.message_class("Grid")(action=P.ACTION["UPDATE"])
                    e = echo.preset.chains.add()
                    e.row = ch.row
                    em = e.models.add()
                    em.hash, em.column = mm.hash, col
                    self._reply("Grid", echo)
                    continue
                b = self.grid.get(col)
                if b is None:
                    continue  # a write to an empty column: the device would ignore it
                for p in mm.params:
                    if p.param_values and p.param_values[0].WhichOneof("value") == "string_value":
                        b.setdefault("strings", {})[p.index] = p.param_values[0].string_value
                        continue
                    if p.scene_mode and not p.param_values:
                        if p.index >= self._n_params(col):
                            b["bypass_scene"] = True
                        else:
                            b["scene"].add(p.index)
                        continue
                    v = p.param_values[0].float_value
                    vals = b["params"].setdefault(p.index, [0.0] * 8)
                    if p.index in b["scene"]:
                        vals[self.scene] = v
                    else:
                        b["params"][p.index] = [v] * 8
        for row in g.preset.bypass:
            for cb in row.colBypass:
                b = self.grid[cb.column]
                flag = cb.sceneBypass[0].bypass
                if b["bypass_scene"]:
                    b["bypass"][self.scene] = flag
                else:
                    b["bypass"] = [flag] * 8

    def _fill_preset(self, bp):
        bp.name = self.name
        for r in range(4):
            ch = bp.chains.add()
            ins, outs = self.routing.get(r, (0, 0))
            ch.in_portid, ch.out_portid = ins, outs
            if r == 0:
                for col in range(max(list(self.grid) + [-1]) + 1):
                    m = ch.models.add()
                    if col in self.grid:
                        b = self.grid[col]
                        m.hash = b["hash"]
                        for i in range(self._n_params(col)):
                            p = m.params.add()
                            p.scene_mode = i in b["scene"]
                            if i in b.get("strings", {}):
                                p.param_values.add().string_value = b["strings"][i]
                                continue
                            for v in b["params"].get(i, [0.0] * 8):
                                p.param_values.add().float_value = v
            else:
                ch.models.add()  # an empty slot keeps the chain non-empty
        for (r, c), (i, t) in sorted(self.stomps.items()):
            a = bp.stomp_mode_assignments.add()
            a.row, a.column, a.stomp_index, a.type = r, c, i, t
        for k, v in self.momentary.items():
            bp.stomp_is_momentary[k] = v
        for r in range(4):
            row = bp.bypass.add()
            for col in range(8):
                cb = row.colBypass.add()
                flags = self.grid[col]["bypass"] if r == 0 and col in self.grid else [False] * 8
                for f in flags:
                    cb.sceneBypass.add().bypass = f
