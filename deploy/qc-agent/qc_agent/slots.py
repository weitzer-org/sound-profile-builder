"""Slot labels and the allowed-slot ranges (QC_AGENT_ALLOWED_SLOTS).

My Presets is addressed by position 0..255; on a full Quad Cortex a bank holds 8
slots, so "1A" is position 0 and "32H" is 255. The 32 x A-H geometry is the same
assumption the contract (section 7) and tools/qc-mcp-trial make; it held on the
owner's unit in Phase 0 (G0: "2A" at position 8, G3: "3A" at position 16).
"""
from __future__ import annotations

import re

BANK_SIZE = 8
SLOT_COUNT = 256
LETTERS = "ABCDEFGH"
SLOT = r"(?:[1-9]|[12][0-9]|3[0-2])[A-H]"
SLOT_RE = re.compile(SLOT)
RANGE_RE = re.compile(rf"({SLOT})(?:-({SLOT}))?")
MAX_RANGES = 16  # heartbeat allowed_slots: up to 16 ranges (contract section 10)


def position(label: str) -> int:
    if not isinstance(label, str) or not SLOT_RE.fullmatch(label):
        raise ValueError(f"not a slot label: {label!r}")
    return (int(label[:-1]) - 1) * BANK_SIZE + LETTERS.index(label[-1])


def label(pos: int) -> str:
    if not 0 <= pos < SLOT_COUNT:
        raise ValueError(f"slot position out of range: {pos}")
    return f"{pos // BANK_SIZE + 1}{LETTERS[pos % BANK_SIZE]}"


class SlotRanges:
    def __init__(self, ranges: list[tuple[int, int]]):
        self.ranges = ranges

    @classmethod
    def parse(cls, text: str) -> "SlotRanges":
        parts = [p.strip() for p in text.upper().split(",")]
        if not parts or len(parts) > MAX_RANGES:
            raise ValueError(f"give 1-{MAX_RANGES} ranges such as 20A-25H")
        ranges = []
        for p in parts:
            m = RANGE_RE.fullmatch(p)
            if not m:
                raise ValueError(f"{p!r} is not a slot or range like 20A-25H")
            lo = position(m.group(1))
            hi = position(m.group(2)) if m.group(2) else lo
            if hi < lo:
                raise ValueError(f"{p!r} runs backwards")
            ranges.append((lo, hi))
        return cls(ranges)

    def contains(self, label_text: str) -> bool:
        try:
            pos = position(label_text)
        except ValueError:
            return False
        return any(lo <= pos <= hi for lo, hi in self.ranges)

    def labels(self) -> list[str]:
        return [label(lo) if lo == hi else f"{label(lo)}-{label(hi)}" for lo, hi in self.ranges]

    def __repr__(self):
        return ",".join(self.labels())
