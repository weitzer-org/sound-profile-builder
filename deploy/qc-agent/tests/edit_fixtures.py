"""Plan and unit builders shared by the edit tests (not a test module): a plan over two row-0 cells and one
row-1 cell, and a fake unit that holds exactly the preset it was built from."""
import json

from helpers import catalog
from qc_agent import contract
from qc_agent.device import SETLIST_KEY

SNAP = "11111111-2222-4333-8444-555555555555"
OTHER_KEY = "/media/p4/Presets/Sounds profiles"
def f32(x):
    """A number as the unit holds it (float32), back as a Python float: what a read reports."""
    import struct
    return struct.unpack('<f', struct.pack('<f', x))[0]


GAIN_EXPECT = [f32(0.45)] * 8
GAIN_WIRE = [0.5, 0.62] + [f32(0.45)] * 6


def find_model(pred):
    for m in catalog().models.values():
        if pred(m):
            return m
    raise AssertionError("no catalog model matches")


def base_plan():
    """A plan over a grid of two row-0 cells (an Exotic overdrive and a UK C30 TopBoost amp) and one block on
    row 1: one scene-varying GAIN change, one switch (BOOST) change and one bypass change."""
    return {"format": contract.EDIT_FORMAT, "catalog": {"coros": "4.1", "sha256": catalog().sha256}, "snapshot": SNAP,
            "expect": {"folder_key": SETLIST_KEY, "position": 152, "slot": "20A", "name": "Texas Blues",
                       "layout": [[0, 0, 6], [0, 1, 1011], [1, 0, 6]]},
            "param_ops": [
                {"row": 0, "col": 0, "model_id": 6, "index": 0, "label": "GAIN", "evidence": ["taper:g4"],
                 "scene_mode": True, "expect": list(GAIN_EXPECT), "wire": list(GAIN_WIRE)},
                {"row": 0, "col": 1, "model_id": 1011, "index": 1, "label": "BOOST", "evidence": ["encoding:qc-mcp-linear"],
                 "scene_mode": False, "expect": [0.0] * 8, "wire": [1.0] * 8}],
            "bypass_ops": [
                {"row": 0, "col": 1, "model_id": 1011, "why": "target", "expect": [False] * 8,
                 "bypass": [False, True] + [False] * 6}]}


def edit_args(plan, mode="live", slot=None, **kw):
    text = json.dumps(plan, separators=(",", ":"))
    return {"slot": slot or plan["expect"]["slot"], "mode": mode,
            "target_slot": kw.get("target_slot"), "name": kw.get("name"),
            "source": {"preset_id": "device-1", "guitar": "From the unit"},
            "plan": {"format": contract.EDIT_FORMAT, "sha256": contract.plan_sha256(contract.EDIT_FORMAT, text),
                     "payload": text}}


def rig_unit(dev, plan=None):
    """Make the fake unit hold exactly the preset base_plan() was built from."""
    dev.open_folder, dev.open_position, dev.preset_name = SETLIST_KEY, 152, "Texas Blues"
    dev.slots = {152: "Texas Blues"}
    dev.grid = {0: {"model_id": 6, "params": {0: list(GAIN_EXPECT), 1: [0.3] * 8, 2: [0.4] * 8, 3: [0.5] * 8},
                    "bypass": [False] * 8},
                1: {"model_id": 1011, "params": {0: [0.5] * 8, 1: [0.0] * 8, 2: [0.2] * 8, 3: [0.7] * 8, 4: [0.1] * 8,
                                                  5: [0.6] * 8},
                    "bypass": [False] * 8}}
    dev.extra_rows = {1: {0: {"model_id": 6, "params": {i: [0.25] * 8 for i in range(5)}, "bypass": [False] * 8,
                              "n_params": 5}}}
