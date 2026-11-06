"""An in-memory Quad Cortex behind the DeviceBackend interface, for offline tests.

It records every call in `calls` (name, args), so a test can assert exactly what
the agent asked the unit to do, and it has no method for anything the agent must
never do: there is no save_preset, recall, clear_grid, delete or global setting
here, so calling one would fail the test with AttributeError.

Proves the agent's guards and job logic. Says nothing about the real unit.
"""
from __future__ import annotations

from qc_agent.device import SETLIST_KEY, DeviceBackend, DeviceError, Presence, UnitAbsent, UnitBusy

WRITE_CALLS = frozenset({"set_routing", "add_block", "set_param", "set_param_scenes", "set_block_bypass",
                         "save_to_slot"})


class FakeDevice(DeviceBackend):
    def __init__(self, firmware="4.1.0", device_type="QC", slots=None, open_position=152):
        self.calls: list[tuple] = []
        self.fw, self.dtype = firmware, device_type
        self.slots = dict(slots or {})                  # position -> name
        self.open_position = open_position              # the slot open on the unit (152 = 20A)
        self.open_folder = SETLIST_KEY
        self.grid: dict[int, dict] = {}                 # col -> {"model_id", "params": {idx: [8]}, "bypass": [8]}
        self.routing = (0, 0)
        self.connected = False
        self.present = True
        self.busy = False
        self.absent_on_connect = False
        self.gone_after = None                          # number of alive() calls before the unit "vanishes"
        self.no_echo_cols: set[int] = set()
        self.corrupt_readback = False
        self.on_connect = None                          # hook(fake) run inside connect()
        self.on_call = None                             # hook(name) run on every call
        self.listing_lag = 0                            # listings that still show the slot empty after a save
        self._alive_calls = 0

    def _rec(self, name, *args):
        self.calls.append((name, *args))
        if self.on_call:
            self.on_call(name)

    def write_calls(self):
        return [c for c in self.calls if c[0] in WRITE_CALLS]

    # -- DeviceBackend --

    def probe(self):
        return Presence(self.present, self.busy)

    def connect(self):
        self._rec("connect")
        if self.on_connect:
            self.on_connect(self)
        if self.absent_on_connect or not self.present:
            raise UnitAbsent("no HID device")
        if self.busy:
            raise UnitBusy("already open exclusively")
        self.connected = True
        self.firmware, self.device_type = self.fw, self.dtype

    def disconnect(self):
        self._rec("disconnect")
        self.connected = False

    def alive(self):
        self._alive_calls += 1
        if self.gone_after is not None and self._alive_calls > self.gone_after:
            return False
        return True

    def _need(self):
        if not self.connected:
            raise DeviceError("not connected")

    def list_my_presets(self):
        self._need()
        self._rec("list_my_presets")
        out = {p: "" for p in range(256)}
        out.update(self.slots)
        if self.listing_lag and self._pending_save is not None:
            self.listing_lag -= 1
            out[self._pending_save[0]] = ""
        return out

    _pending_save = None

    def setlist_position(self):
        self._need()
        self._rec("setlist_position")
        return {"folder_key": self.open_folder, "position": self.open_position, "is_factory": False}

    def read_current_preset(self):
        self._need()
        self._rec("read_current_preset")
        blocks = []
        for col in sorted(self.grid):
            b = self.grid[col]
            n = max(list(b["params"]) + [0]) + 2
            params = [{"index": i, "values": list(b["params"].get(i, [0.0] * 8)), "scene_mode": False}
                      for i in range(n)]
            if self.corrupt_readback and params:
                params[0]["values"] = [0.999] * 8
            blocks.append({"col": col, "model_id": b["model_id"], "params": params})
        bypass = [[[False] * 8 for _ in range(8)] for _ in range(4)]
        for col, b in self.grid.items():
            bypass[0][col] = list(b["bypass"])
        return {"name": "", "chains": [{"row": 0, "in_port": self.routing[0], "out_port": self.routing[1],
                                        "blocks": blocks}] + [{"row": r, "in_port": 0, "out_port": 0, "blocks": []}
                                                              for r in (1, 2, 3)],
                "bypass": bypass}

    def begin_push(self, position, name, plan):
        self._rec("begin_push", position, name)

    def end_push(self):
        self._rec("end_push")

    def set_routing(self, row, in_port, out_port):
        self._need()
        self._rec("set_routing", row, in_port, out_port)
        self.routing = (in_port, out_port)

    def add_block(self, row, col, model_id):
        self._need()
        self._rec("add_block", row, col, model_id)
        if col in self.no_echo_cols:
            return False
        self.grid[col] = {"model_id": model_id, "params": {}, "bypass": [False] * 8}
        return True

    def set_param(self, row, col, index, wire):
        self._need()
        self._rec("set_param", row, col, index, wire)
        self.grid[col]["params"][index] = [wire] * 8

    def set_param_scenes(self, row, col, index, wires):
        self._need()
        self._rec("set_param_scenes", row, col, index, list(wires))
        self.grid[col]["params"][index] = list(wires)

    def set_block_bypass(self, row, col, scenes):
        self._need()
        self._rec("set_block_bypass", row, col, list(scenes))
        self.grid[col]["bypass"] = list(scenes)

    def save_to_slot(self, position, name):
        self._need()
        self._rec("save_to_slot", position, name)
        self.slots[position] = name
        self._pending_save = (position, name)
