#!/usr/bin/env python3
"""Allowlist proxy for the unofficial qc-mcp Quad Cortex MCP server.

Sits between an MCP client (Claude Code / Claude Desktop) and qc-mcp
(github.com/lexasoft123/qc-mcp, reviewed at f634ecd) over stdio. It is a guard for a
trial, not a feature, and its rules come from docs/research/qc-usb-tools-audit.md
section 1.6:

* Only allowlisted tools are listed or callable. ``--mode read`` (the default) allows
  read tools only. ``--mode write`` adds add_block, remove_block, set_parameter,
  set_block_bypass and save_preset_as, nothing else. In particular it never forwards
  save_preset, build_preset, clear_grid, global settings or I/O, master volume,
  delete, leveling or audio tools.
* ``connect`` is forced to ``mode='direct', quit_app=False``, and the server runs with
  ``QC_BRIDGE=0``.
* Every write first checks, with fresh reads through qc-mcp, that:
  - the device reports exactly ``--firmware``, and
  - the preset open on the unit is the one sacrificial ``--slot``.
* add_block and set_parameter are checked against qc-mcp's own ModelRepo.xml, which
  is also where qc-mcp gets its parameter indexes. They refuse:
  - hidden or internal models and categories (including NC_Recorder, id 20000, which
    pyquadcortex records crashing a unit);
  - plugin models;
  - hidden parameters;
  - parameters whose range is symbolic (qc-mcp would send the raw value);
  - LOG_SKEW or other non-numeric tapers (qc-mcp converts those with a heuristic);
  - values outside the parameter's range.
* save_preset_as must target ``--slot`` with ``overwrite=False``. Before it is
  forwarded, the proxy refreshes the directory from the device and checks the slot is
  still empty. qc-mcp's own guard reads a cache that saves never invalidate.

The proxy has no dependencies beyond the Python standard library. Messages are
newline-delimited JSON-RPC (the MCP stdio transport). The proxy never writes to the
device itself: it only forwards calls or makes qc-mcp read calls.
"""
from __future__ import annotations

import argparse
import itertools
import json
import math
import os
import queue
import re
import subprocess
import sys
import threading
import xml.etree.ElementTree as ET

READ_TOOLS = frozenset({
    "connect", "disconnect", "device_info", "get_current_preset",
    "current_preset_position", "cpu_load", "get_mode", "get_io_settings", "get_tempo",
    "find_devices", "list_empty_slots", "directory_summary", "search_directory",
})
WRITE_TOOLS = frozenset({
    "add_block", "remove_block", "set_parameter", "set_block_bypass", "save_preset_as",
})
DENY_MODEL_IDS = frozenset({20000})        # NC_Recorder: pyquadcortex units.py:239-247
DEFAULT_SETLIST = "/media/p4/Presets/My Presets"
INTERNAL_TIMEOUT_S = 90.0                 # a full directory refresh streams ~15 s
_NUMERIC_SKEW = re.compile(r"^[0-9]*\.?[0-9]+(e[-+]?[0-9]+)?$", re.I)


class Refused(Exception):
    """A tool call the proxy will not forward. The message goes back to the model."""


# --------------------------------------------------------------------------- catalog
def load_catalog(xml_path: str) -> dict[int, dict]:
    """Parse ModelRepo.xml into {model_id: {...}}. Parameters are kept in XML order,
    which is the index qc-mcp addresses them by (qc_mcp/catalog.py `_parse`)."""
    root = ET.parse(xml_path).getroot()
    models: dict[int, dict] = {}
    for cat in root.findall("Category"):
        cat_hidden = cat.get("hidden") == "true"
        for m in cat.findall("Model"):
            models[int(m.get("id"))] = {
                "name": m.get("name"),
                "category": cat.get("name"),
                "blocked": (cat_hidden or m.get("hidden") == "true"
                            or m.get("internal") == "true"
                            or "Internal" in (cat.get("name") or "")),
                "plugin": bool(m.get("plugin_id")),
                "params": [dict(p.attrib) for p in m.findall("Parameter")],
            }
    return models


def _as_int(value, what: str) -> int:
    if isinstance(value, bool) or not isinstance(value, (int, float, str)):
        raise Refused(f"{what} must be an integer, got {value!r}")
    try:
        f = float(value)
    except ValueError:
        raise Refused(f"{what} must be an integer, got {value!r}") from None
    if not f.is_integer():
        raise Refused(f"{what} must be an integer, got {value!r}")
    return int(f)


def check_model(catalog: dict, model_id) -> dict:
    mid = _as_int(model_id, "device_hash")
    if mid in DENY_MODEL_IDS:
        raise Refused(f"model {mid} is denied: placing it crashed a unit (NC_Recorder).")
    model = catalog.get(mid)
    if model is None:
        raise Refused(f"model {mid} is not in qc-mcp's ModelRepo.xml.")
    if model["blocked"]:
        raise Refused(f"model {mid} ({model['name']}) is hidden/internal "
                      f"(category {model['category']!r}); the trial does not place those.")
    if model["plugin"]:
        raise Refused(f"model {mid} ({model['name']}) belongs to a paid plugin; "
                      "the trial does not place plugin blocks.")
    return model


def check_param(catalog: dict, model_id, param_index, display_value) -> None:
    model = check_model(catalog, model_id)
    idx = _as_int(param_index, "param_index")
    if not 0 <= idx < len(model["params"]):
        raise Refused(f"{model['name']} has no parameter index {idx}.")
    p = model["params"][idx]
    name = p.get("name", f"#{idx}")
    if p.get("hidden") not in (None, "false"):
        raise Refused(f"{model['name']} {name} is a hidden parameter.")
    try:
        lo, hi = float(p.get("min")), float(p.get("max"))
    except (TypeError, ValueError):
        raise Refused(f"{model['name']} {name} has a symbolic or missing range "
                      f"({p.get('min')}..{p.get('max')}); qc-mcp would send the raw "
                      "value to the device.") from None
    skew = (p.get("skew") or "").strip()
    if skew not in ("", "LIN_SKEW") and not _NUMERIC_SKEW.match(skew):
        raise Refused(f"{model['name']} {name} uses taper {skew!r}, which qc-mcp "
                      "converts with a heuristic; not trusted for the trial.")
    if isinstance(display_value, bool) or not isinstance(display_value, (int, float)):
        raise Refused(f"display_value must be a number, got {display_value!r}")
    v = float(display_value)
    if not math.isfinite(v) or not min(lo, hi) <= v <= max(lo, hi):
        raise Refused(f"{model['name']} {name} runs {lo:g}..{hi:g}; {v:g} is outside it.")


def parse_slot(text: str) -> int:
    """'12H' (full Quad Cortex banks of 8) or a 0-based position -> position."""
    t = str(text).strip().upper()
    if t.isdigit():
        return int(t)
    m = re.fullmatch(r"(\d+)([A-H])", t)
    if not m or int(m.group(1)) < 1:
        raise ValueError(f"slot must look like 12H or a 0-based number, not {text!r}")
    return (int(m.group(1)) - 1) * 8 + "ABCDEFGH".index(m.group(2))


def slot_label(position: int) -> str:
    return f"{position // 8 + 1}{'ABCDEFGH'[position % 8]}"


def tool_payload(result: dict):
    """The value a qc-mcp tool returned, from an MCP tools/call result."""
    if result.get("isError"):
        texts = [c.get("text", "") for c in result.get("content", []) if c.get("type") == "text"]
        raise Refused("qc-mcp reported an error during the safety check: "
                      + (" ".join(texts) or "unknown"))
    sc = result.get("structuredContent")
    if isinstance(sc, dict):
        return sc["result"] if set(sc) == {"result"} else sc
    items = [c.get("text", "") for c in result.get("content", []) if c.get("type") == "text"]
    values = []
    for t in items:
        try:
            values.append(json.loads(t))
        except ValueError:
            values.append(t)
    return values[0] if len(values) == 1 else values


# ----------------------------------------------------------------------------- proxy
class Proxy:
    def __init__(self, child_cmd, child_env, child_cwd, *, mode, firmware, slot,
                 setlist, catalog, out=sys.stdout, inp=sys.stdin, on_child_exit=None):
        self.allowed = READ_TOOLS | (WRITE_TOOLS if mode == "write" else frozenset())
        self.firmware, self.slot, self.setlist = firmware, slot, setlist
        self.catalog = catalog
        self.out, self.inp = out, inp
        # qc-mcp exiting ends the proxy too, so the client sees the server go away.
        self._on_child_exit = on_child_exit or (lambda: os._exit(0))
        self._out_lock = threading.Lock()
        self._ids = itertools.count(1)
        self._internal: dict[str, queue.Queue] = {}
        self._list_ids: set = set()
        self._lock = threading.Lock()
        self.child = subprocess.Popen(child_cmd, stdin=subprocess.PIPE,
                                      stdout=subprocess.PIPE, text=True, encoding="utf-8",
                                      env=child_env, cwd=child_cwd, bufsize=1)

    # -- plumbing --
    def _emit(self, msg) -> None:
        with self._out_lock:
            self.out.write(json.dumps(msg, ensure_ascii=False) + "\n")
            self.out.flush()

    def _to_child(self, msg) -> None:
        self.child.stdin.write(json.dumps(msg, ensure_ascii=False) + "\n")
        self.child.stdin.flush()

    def _pump_child(self) -> None:
        for line in self.child.stdout:
            line = line.strip()
            if not line:
                continue
            try:
                msg = json.loads(line)
            except ValueError:
                continue                      # never pass non-JSON through
            if isinstance(msg, dict) and "id" in msg and "method" not in msg:
                with self._lock:
                    q = self._internal.pop(msg["id"], None)
                    is_list = msg["id"] in self._list_ids
                    self._list_ids.discard(msg["id"])
                if q is not None:
                    q.put(msg)
                    continue
                if is_list and isinstance(msg.get("result"), dict):
                    tools = msg["result"].get("tools", [])
                    msg["result"]["tools"] = [t for t in tools if t.get("name") in self.allowed]
            self._emit(msg)
        self._on_child_exit()

    def call(self, name: str, arguments: dict):
        """Call a qc-mcp tool on the proxy's own behalf and return its payload."""
        rid = f"qcproxy-{next(self._ids)}"
        q: queue.Queue = queue.Queue(maxsize=1)
        with self._lock:
            self._internal[rid] = q
        self._to_child({"jsonrpc": "2.0", "id": rid, "method": "tools/call",
                        "params": {"name": name, "arguments": arguments}})
        try:
            msg = q.get(timeout=INTERNAL_TIMEOUT_S)
        except queue.Empty:
            raise Refused(f"safety check {name} timed out") from None
        if "error" in msg:
            raise Refused(f"safety check {name} failed: {msg['error']}")
        return tool_payload(msg.get("result") or {})

    # -- gates --
    def require_unit_and_slot(self) -> None:
        info = self.call("device_info", {})
        fw = str((info or {}).get("firmware", "")) if isinstance(info, dict) else ""
        if fw != self.firmware:
            raise Refused(f"device reports CorOS {fw or '?'!s}, the trial was set up for "
                          f"{self.firmware}. Writes are disabled.")
        pos = self.call("current_preset_position", {})
        if not isinstance(pos, dict) or pos.get("is_factory") \
                or pos.get("folder_key") != self.setlist \
                or pos.get("position") != self.slot:
            raise Refused(f"the preset open on the unit is {pos!r}, not the trial slot "
                          f"{slot_label(self.slot)} (position {self.slot}) in "
                          f"{self.setlist!r}. Select that slot on the unit first.")

    def require_slot_empty(self) -> None:
        summary = self.call("directory_summary", {"refresh": True})
        if not isinstance(summary, dict) or summary.get("source") != "device":
            raise Refused("could not refresh the preset directory from the device "
                          f"(source={summary.get('source') if isinstance(summary, dict) else '?'}); "
                          "refusing to save without a fresh listing.")
        empty = self.call("list_empty_slots", {"setlist_key": self.setlist, "limit": 256})
        if not isinstance(empty, dict) or empty.get("source") != "device":
            raise Refused("empty-slot listing did not come from the device; refusing.")
        free = {e.get("position") for e in empty.get("empty_positions", [])}
        if self.slot not in free:
            raise Refused(f"slot {slot_label(self.slot)} is not empty on the device any "
                          "more; the trial saves only into an empty slot.")

    def vet(self, name: str, args: dict) -> dict:
        """Return the arguments to forward, or raise Refused."""
        if name not in self.allowed:
            raise Refused(f"tool {name!r} is not allowed by the trial proxy "
                          f"(mode allows: {', '.join(sorted(self.allowed))}).")
        if name == "connect":
            return {"mode": "direct", "quit_app": False}
        if name not in WRITE_TOOLS:
            return args
        if name != "save_preset_as":
            # Rebuild the arguments from known keys only, with the grid bounds checked:
            # the Grid is 4 rows x 8 columns and qc-mcp forwards whatever it is given.
            cell = {"row": _as_int(args.get("row"), "row"),
                    "column": _as_int(args.get("column"), "column")}
            if not (0 <= cell["row"] <= 3 and 0 <= cell["column"] <= 7):
                raise Refused("row must be 0-3 and column 0-7.")
        if name == "add_block":
            check_model(self.catalog, args.get("device_hash"))
            params = args.get("params") or {}
            if not isinstance(params, dict):
                raise Refused("params must be an object of {param_index: display_value}.")
            for pidx, disp in params.items():
                check_param(self.catalog, args.get("device_hash"), pidx, disp)
            args = dict(cell, device_hash=_as_int(args["device_hash"], "device_hash"),
                        params={str(_as_int(k, "param_index")): float(v)
                                for k, v in params.items()} or None)
        elif name == "set_parameter":
            if not args.get("device_hash"):
                raise Refused("set_parameter needs device_hash so the value is converted "
                              "from display units; the trial refuses raw 0..1 writes.")
            check_param(self.catalog, args["device_hash"], args.get("param_index"),
                        args.get("display_value"))
            args = dict(cell, param_index=_as_int(args["param_index"], "param_index"),
                        display_value=float(args["display_value"]),
                        device_hash=_as_int(args["device_hash"], "device_hash"))
        elif name == "remove_block":
            args = cell
        elif name == "set_block_bypass":
            bypassed = args.get("bypassed", True)
            scenes = args.get("scenes")
            if not isinstance(bypassed, bool) or (scenes is not None and not (
                    isinstance(scenes, list) and len(scenes) <= 8
                    and all(isinstance(s, bool) for s in scenes))):
                raise Refused("bypassed must be true/false and scenes a list of up to 8 "
                              "true/false values.")
            args = dict(cell, bypassed=bypassed, **({"scenes": scenes} if scenes else {}))
        elif name == "save_preset_as":
            pos = args.get("position", -1)
            if _as_int(pos, "position") != self.slot:
                raise Refused(f"save_preset_as may only target the trial slot "
                              f"{slot_label(self.slot)} (position={self.slot}).")
            if args.get("setlist_key") not in (None, "", self.setlist):
                raise Refused(f"save_preset_as may only target {self.setlist!r}.")
            pname = args.get("name")
            if not isinstance(pname, str) or not pname.strip() or len(pname) > 32 \
                    or not pname.isprintable():
                raise Refused("save_preset_as needs a printable name of 1-32 characters.")
            args = {"name": pname, "setlist_key": self.setlist, "position": self.slot,
                    "overwrite": False}
        self.require_unit_and_slot()
        if name == "save_preset_as":
            self.require_slot_empty()
        return args

    # -- main loop --
    def handle(self, msg) -> None:
        if not isinstance(msg, dict):
            self._emit({"jsonrpc": "2.0", "id": None,
                        "error": {"code": -32600, "message": "batches are not supported"}})
            return
        method = msg.get("method")
        if method == "tools/list" and "id" in msg:
            with self._lock:
                self._list_ids.add(msg["id"])
        elif method == "tools/call" and "id" in msg:
            params = msg.get("params") or {}
            name = params.get("name", "")
            try:
                args = self.vet(name, dict(params.get("arguments") or {}))
            except Refused as e:
                self._emit({"jsonrpc": "2.0", "id": msg["id"], "result": {
                    "content": [{"type": "text", "text": f"Refused by trial proxy: {e}"}],
                    "isError": True}})
                return
            msg = dict(msg, params=dict(params, arguments=args))
        self._to_child(msg)

    def run(self) -> int:
        threading.Thread(target=self._pump_child, daemon=True).start()
        for line in self.inp:
            line = line.strip()
            if not line:
                continue
            try:
                msg = json.loads(line)
            except ValueError:
                continue
            self.handle(msg)
        self.child.stdin.close()
        try:
            self.child.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.child.terminate()
        return 0


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--qc-mcp", required=True, help="path to the qc-mcp checkout (at f634ecd)")
    ap.add_argument("--python", required=True, help="the venv python that has qc-mcp's deps")
    ap.add_argument("--mode", choices=("read", "write"), default="read")
    ap.add_argument("--firmware", required=True,
                    help="exact CorOS string device_info must report, e.g. 4.1.0")
    ap.add_argument("--slot", required=True, help="sacrificial EMPTY slot, e.g. 12H")
    ap.add_argument("--setlist", default=DEFAULT_SETLIST)
    a = ap.parse_args(argv)
    checkout = os.path.abspath(a.qc_mcp)
    src = os.path.join(checkout, "src")
    catalog = load_catalog(os.path.join(src, "qc_mcp", "ModelRepo.xml"))
    env = dict(os.environ, PYTHONPATH=src, QC_BRIDGE="0")
    env.pop("QC_SHARE", None)
    proxy = Proxy([a.python, "-m", "qc_mcp.server"], env, checkout, mode=a.mode,
                  firmware=a.firmware, slot=parse_slot(a.slot), setlist=a.setlist,
                  catalog=catalog)
    print(f"qc-mcp trial proxy: mode={a.mode} firmware={a.firmware} "
          f"slot={slot_label(proxy.slot)}", file=sys.stderr)
    return proxy.run()


if __name__ == "__main__":
    sys.exit(main())
