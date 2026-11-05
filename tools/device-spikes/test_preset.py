"""The one test preset the write spikes share (G3 builds and saves it; G4 and G6 edit it live).

Chain on row 0, in-port 1 and out-port 19 (qc-mcp's clear_grid default routing, plan section 4):

    col 0  Myth Drive          (plain 0..10 dials, linear: the audit's suggested first write)
    col 1  Brit 2203           (amp; OUTPUT is skew 3.8018, -60..+12 dB)
    col 2  CA Stand OS S V30 90s (M)   (a dual-mic cab that CLONES "Default Cabsim"; LEVEL is symbolic CABSIM dB)
    col 3  Low-High Cut        (HPF FREQ is skew 0.3, 20..20000 Hz)
    col 4  Spring (M)          (a reverb; also the cab + Spring pairing G1 is about)

Every model id is re-checked against the qc-mcp ModelRepo.xml at run time (name, not hidden/internal,
not NC_Recorder), so a typo here aborts instead of placing something unexpected.

Values are WIRE values (0..1) written straight through ``set_param``. That is the point of G3: it
exercises the transport the way the home agent will (plan section 5), with no display conversion.
"""
from __future__ import annotations

from dataclasses import dataclass, field

TEST_PRESET_NAME = "QC2 spike G3"
ROW, IN_PORT, OUT_PORT = 0, 1, 19
DENY_MODEL_IDS = frozenset({20000})  # NC_Recorder crashed a unit (audit 1.5)


@dataclass(frozen=True)
class TestBlock:
    col: int
    model_id: int
    xml_name: str
    writes: dict[int, float] = field(default_factory=dict)  # param index -> wire value


BLOCKS = [
    TestBlock(0, 1, "Myth Drive", {0: 0.60, 1: 0.30, 2: 0.50}),
    TestBlock(1, 1001, "Brit 2203", {0: 0.45, 4: 0.30}),
    TestBlock(2, 12001, "412 CA Stand OS S V30 90s (M)", {2: 0.50}),
    TestBlock(3, 4003, "Low-High Cut", {1: 0.25}),
    TestBlock(4, 8006, "Spring (M)", {0: 0.35}),
]

# Positions of the parameters G4 and G6 touch.
AMP_COL, AMP_OUTPUT_IDX = 1, 6
CAB_COL, CAB_LEVEL_IDX = 2, 2
EQ_COL, EQ_HPF_IDX = 3, 1
DRIVE_COL, DRIVE_GAIN_IDX = 0, 0
REVERB_COL = 4


def validate_against_xml(xml_models: dict[int, dict]) -> list[str]:
    """Return a list of problems (empty = fine)."""
    problems = []
    for b in BLOCKS:
        if b.model_id in DENY_MODEL_IDS:
            problems.append(f"{b.model_id} is on the deny list")
        m = xml_models.get(b.model_id)
        if m is None:
            problems.append(f"model {b.model_id} ({b.xml_name}) is not in ModelRepo.xml")
            continue
        if m["name"] != b.xml_name:
            problems.append(f"model {b.model_id} is named {m['name']!r} in ModelRepo.xml, expected {b.xml_name!r}")
        if (m.get("hidden") or "").lower() in ("true", "1") or "internal" in (m.get("category") or "").lower():
            problems.append(f"model {b.model_id} {m['name']!r} is hidden/internal")
    return problems
