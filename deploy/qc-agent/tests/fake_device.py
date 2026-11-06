"""An in-memory Quad Cortex behind the DeviceBackend interface, for offline tests.

It records every call in `calls` (name, args), so a test can assert exactly what
the agent asked the unit to do, and it has no method for anything the agent must
never do: there is no save_preset, recall, clear_grid, delete or global setting
here, so calling one would fail the test with AttributeError.

Proves the agent's guards and job logic. Says nothing about the real unit.
"""
from __future__ import annotations

from qc_agent.captures import CaptureEntry, DirectorySnapshot, factory_version_of
from qc_agent.device import SETLIST_KEY, STOMP_PRIMARY, DeviceBackend, DeviceError, Presence, UnitAbsent, UnitBusy

WRITE_CALLS = frozenset({"set_routing", "add_block", "place_capture", "set_param", "set_param_scenes",
                         "set_block_bypass", "save_to_slot", "set_mic", "assign_stomp"})

KEY_V1 = "a1" * 32   # a factory V1 capture
KEY_V2 = "b2" * 32   # a factory V2 capture
KEY_USER = "c3" * 32  # a user capture (version field "1", as the factory V1 entries carry)


def capture(name, key, folder="My Captures", factory=False, version="1", cloud=False, favorites=False,
            above=()):
    """`above`: names of the folders above `folder`, nearest first (the backend fills factory_version from the chain)."""
    fver = factory_version_of([folder, *above]) if factory else None
    return CaptureEntry(name=name, key=key, folder_key="k-" + folder, folder_name=folder, is_factory=factory,
                        is_cloud=cloud, is_favorites=favorites, version_field=version, factory_version=fver)


def default_captures():
    """A small library shaped like qc-mcp DIRECTORY.md's: factory V1/V2 folders, a user folder."""
    return [capture("CA John's", KEY_V1, "Factory Captures V1", factory=True, version="1"),
            capture("Brit 2203 87", "d4" * 32, "Factory Captures V1", factory=True, version="1"),
            capture("CA John's 2", KEY_V2, "Factory Captures V2", factory=True, version="2"),
            capture("66 AC30 JMP OD", KEY_USER, "My Captures", version="1")]


class FakeDevice(DeviceBackend):
    def __init__(self, firmware="4.1.0", device_type="QC", slots=None, open_position=152):
        self.calls: list[tuple] = []
        self.fw, self.dtype = firmware, device_type
        self.slots = dict(slots or {})                  # position -> name
        self.open_position = open_position              # the slot open on the unit (152 = 20A)
        self.open_folder = SETLIST_KEY
        self.collections: dict[str, tuple[str, dict[int, str]]] = {}  # other non-factory collections: key -> (name, {pos: name})
        self.push_folder = None                         # the folder begin_push was armed for
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
        self.captures = default_captures()              # the unit's capture library (read_directory)
        self.deps_on_save = True                        # the saved preset lists its captures as dependencies
        self.preset_deps: dict[int, frozenset] = {}
        self.on_listing = None                          # hook(fake, n) before the n-th listing (1-based)
        self.corrupt_file_name = False                  # the read-back's file_name differs
        self.param_count_skew: dict[int, int] = {}      # col -> slots the unit reports beyond param_count + 1
        self.stomps: dict[tuple[int, int], int] = {}     # (row, col) -> stomp index, as assigned
        self.extra_stomps: list[dict] = []              # assignments the read-back shows beyond those
        self.stomp_index_skew = 0                       # the read-back reports each index shifted by this
        self.momentary: dict[int, bool] = {}            # what the read-back reports in stomp_is_momentary
        self.omit_stomps = False                        # the read-back carries no assignments field at all
        self._listings = 0
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

    def read_directory(self):
        self._need()
        self._listings += 1
        if self.on_listing:
            self.on_listing(self, self._listings)
        self._rec("read_directory")
        out = {p: "" for p in range(256)}
        out.update(self.slots)
        deps = dict(self.preset_deps)
        if self.listing_lag and self._pending_save is not None:
            self.listing_lag -= 1
            out[self._pending_save[0]] = ""
            deps.pop(self._pending_save[0], None)
        folders = {}
        for e in self.captures:
            n, f, c = folders.get(e.folder_name, (e.folder_name, e.is_factory, 0))
            folders[e.folder_name] = (n, f, c + 1)
        pf = {SETLIST_KEY: ("My Presets", out, deps)}
        for key, (cname, cslots) in self.collections.items():
            pf[key] = (cname, {p: cslots.get(p, "") for p in range(256)}, {})
        return DirectorySnapshot(my_presets=out, preset_deps=deps, captures=list(self.captures),
                                 capture_folders=list(folders.values()), preset_folders=pf)

    _pending_save = None

    def setlist_position(self):
        self._need()
        self._rec("setlist_position")
        return {"folder_key": self.open_folder, "position": self.open_position, "is_factory": False}

    def _n_params(self, col):
        # Fixed per model, like the unit (G1: N = param_count + 1), not derived from what was written.
        from helpers import catalog
        m = catalog().models.get(self.grid[col]["model_id"])
        n = m["param_count"] + 1 if m else max(list(self.grid[col]["params"]) + [0]) + 2
        return n + self.param_count_skew.get(col, 0)

    def read_current_preset(self):
        self._need()
        self._rec("read_current_preset")
        blocks = []
        for col in sorted(self.grid):
            b = self.grid[col]
            n = self._n_params(col)
            params = [{"index": i, "values": list(b["params"].get(i, [0.0] * 8)), "scene_mode": False}
                      for i in range(n)]
            if self.corrupt_readback and params:
                params[0]["values"] = [0.999] * 8
            for idx, sv in b.get("strings", {}).items():
                while len(params) <= idx:
                    params.append({"index": len(params), "values": [0.0] * 8, "scene_mode": False})
                params[idx] = {"index": idx, "values": [sv + ("X" if self.corrupt_file_name else "")],
                               "scene_mode": False}
            blocks.append({"col": col, "model_id": b["model_id"], "params": params})
        bypass = [[[False] * 8 for _ in range(8)] for _ in range(4)]
        for col, b in self.grid.items():
            bypass[0][col] = list(b["bypass"])
        return {"name": "", "chains": [{"row": 0, "in_port": self.routing[0], "out_port": self.routing[1],
                                        "blocks": blocks}] + [{"row": r, "in_port": 0, "out_port": 0, "blocks": []}
                                                              for r in (1, 2, 3)],
                "bypass": bypass,
                **({} if self.omit_stomps else {
                    "stomps": [{"row": r, "col": c, "index": i + self.stomp_index_skew, "type": STOMP_PRIMARY}
                               for (r, c), i in sorted(self.stomps.items())] + list(self.extra_stomps),
                    "momentary": dict(self.momentary)})}

    def arm_stomps(self, stomps):
        self._rec("arm_stomps", [(s.row, s.col, s.switch) for s in stomps])

    def assign_stomp(self, row, col, index):
        self._need()
        self._rec("assign_stomp", row, col, index)
        if col not in self.grid:
            raise DeviceError(f"fake unit: no block at column {col}")
        self.stomps[(row, col)] = index  # qc-mcp: assigning a block again moves it

    def begin_push(self, position, name, plan, folder_key=None):
        self._rec("begin_push", position, name)
        self.push_folder = folder_key

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

    def place_capture(self, row, col, model_id, key, name):
        self._need()
        self._rec("place_capture", row, col, model_id, key, name)
        if col in self.no_echo_cols:
            return False
        self.grid[col] = {"model_id": model_id, "params": {}, "bypass": [False] * 8, "strings": {5: key + name},
                          "loaded": True}
        return True

    def set_mic(self, row, col, index, value):
        self._need()
        self._rec("set_mic", row, col, index, value)
        self.grid[col].setdefault("strings", {})[index] = value

    def set_param(self, row, col, index, wire):
        self._need()
        self._rec("set_param", row, col, index, wire)
        self.grid[col]["params"][index] = [wire] * 8

    def set_param_scenes(self, row, col, index, wires):
        self._need()
        self._rec("set_param_scenes", row, col, index, list(wires))
        self.grid[col]["params"][index] = list(wires)

    def confirm_layout(self, col, n):
        self._rec("confirm_layout", col, n)

    def set_block_bypass(self, row, col, scenes, bypass_index):
        self._need()
        self._rec("set_block_bypass", row, col, list(scenes), bypass_index)
        if bypass_index != self._n_params(col):  # qc-mcp: the bypass control is one past the block's read
            raise DeviceError(f"fake unit: column {col}'s bypass control is {self._n_params(col)}, not {bypass_index}")
        self.grid[col]["bypass"] = list(scenes)

    def save_to_slot(self, position, name):
        self._need()
        self._rec("save_to_slot", position, name)
        if self.open_folder in self.collections:
            self.collections[self.open_folder][1][position] = name
            return
        self.slots[position] = name
        self._pending_save = (position, name)
        # like the backend's snapshot: every saved preset has an entry, empty when it lists none
        self.preset_deps[position] = frozenset(b["strings"][5][:64].lower() for b in self.grid.values()
                                               if b.get("loaded")) if self.deps_on_save else frozenset()
