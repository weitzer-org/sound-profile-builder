"""The device interface the job code talks to, and its error types.

Two implementations:
* qc_agent.qcmcp_backend.QcMcpBackend: the real unit, through qc-mcp's transport
  vendored at f634ecd (vendor/qc_mcp), behind a method allowlist and a wire guard;
* tests/fake_device.py: an in-memory unit that records every call, for the offline tests.

Everything a backend returns that came from the unit (preset, slot, capture and IR
names) is untrusted text: the job code passes it through text.clean() before it
goes into a result or a log line.

Read shape (positional, as the unit reports it; plan 7.2 and G6): a preset is
    {"name": str,
     "chains": [{"row": i, "in_port": int, "out_port": int,
                 "blocks": [{"col": j, "model_id": int,
                             "params": [{"index": k, "values": [v_A..v_H], "scene_mode": bool}]}]}],
     "bypass": [[[8 bools] per column] per row] or None}
where i, j and k are array positions (in a read the unit leaves the row/column/index
fields at 0, so the position is the address).
"""
from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass

SETLIST_KEY = "/media/p4/Presets/My Presets"  # qc-mcp's My Presets folder key; G0/G3 used it on the owner's unit
SETLIST_NAME = "My Presets"


class DeviceError(RuntimeError):
    """The unit misbehaved or a transport call failed."""


class UnitAbsent(DeviceError):
    """No Quad Cortex found, or it went away (switched off, unplugged)."""


class UnitBusy(DeviceError):
    """Another program (Cortex Control) holds the unit exclusively."""


@dataclass
class Presence:
    present: bool | None  # None: can't tell without opening the unit
    busy: bool | None


class DeviceBackend(ABC):
    """One connect / act / disconnect cycle per job (plan section 5: exclusive access)."""

    firmware: str | None = None
    device_type: str | None = None

    @abstractmethod
    def probe(self) -> Presence:
        """Is a unit attached? Must not open it (Cortex Control keeps working)."""

    @abstractmethod
    def connect(self) -> None:
        """Open the unit exclusively and read its firmware. Raises UnitAbsent / UnitBusy / DeviceError."""

    @abstractmethod
    def disconnect(self) -> None:
        """Close the unit. Never raises and never blocks for long."""

    @abstractmethod
    def alive(self) -> bool:
        """False once the unit's liveness signals say it is gone (G2 c). Never blocks: the real
        backend samples on its own watchdog thread and this only reads the verdict."""

    @abstractmethod
    def read_directory(self):
        """A FRESH directory listing as a qc_agent.captures.DirectorySnapshot: My Presets
        ({position: name} for all 256 positions; exactly '' means empty), each saved preset's
        dependency hashes, and every capture folder's files. Raises DeviceError if My Presets
        isn't complete. Names in it are untrusted."""

    def list_my_presets(self) -> dict[int, str]:
        """My Presets from a fresh listing (see read_directory)."""
        return self.read_directory().my_presets

    @abstractmethod
    def setlist_position(self) -> dict | None:
        """{folder_key, position, is_factory} of the preset open on the unit."""

    @abstractmethod
    def read_current_preset(self) -> dict | None:
        """The open preset's live grid (see the module docstring), or None on timeout."""

    # -- writes (only during a push, only into the verified-empty open slot) --

    @abstractmethod
    def begin_push(self, position: int, name: str, plan) -> None:
        """Arm the writes for this one plan; anything else stays refused."""

    @abstractmethod
    def end_push(self) -> None:
        """Disarm. Idempotent."""

    @abstractmethod
    def set_routing(self, row: int, in_port: int, out_port: int) -> None: ...

    @abstractmethod
    def add_block(self, row: int, col: int, model_id: int) -> bool:
        """True when the unit echoed the placement."""

    @abstractmethod
    def place_capture(self, row: int, col: int, model_id: int, key: str, name: str) -> bool:
        """Place a Neural Capture block (model_id 14000 = V1, 14001 = V2) and load the capture
        `key + name` into it (its file_name). True when the unit echoed the block."""

    @abstractmethod
    def set_param(self, row: int, col: int, index: int, wire: float) -> None: ...

    @abstractmethod
    def set_param_scenes(self, row: int, col: int, index: int, wires: list[float]) -> None: ...

    @abstractmethod
    def confirm_layout(self, col: int, n: int) -> None:
        """The unit just read `n` params for the planned block in `col`. Allows that block's bypass
        control only when n is its param_count + 1; raises DeviceError otherwise."""

    @abstractmethod
    def set_block_bypass(self, row: int, col: int, scenes: list[bool], bypass_index: int) -> None:
        """All eight scene flags (True = bypassed), through the block's bypass control at bypass_index
        (param_count + 1, checked against the unit's own read of the block before any value is written)."""

    @abstractmethod
    def save_to_slot(self, position: int, name: str) -> None:
        """File CREATE of the live grid into My Presets at position (never RecallPreset SAVE)."""
