"""Offline tests for qc_allowlist_proxy.py. No Quad Cortex and no qc-mcp needed.

A fake qc-mcp child (written to a temp dir) answers the read calls the proxy makes
for its safety checks from a JSON state file, and logs every tools/call it
receives. The tests assert which calls reach the child and with which arguments.

    python3 -m unittest test_qc_allowlist_proxy -v

Set QC_MCP_MODELREPO to qc-mcp's src/qc_mcp/ModelRepo.xml to also run the checks
against the real catalog.
"""
import io
import json
import os
import sys
import tempfile
import textwrap
import threading
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import qc_allowlist_proxy as P  # noqa: E402

MINI_REPO = """<?xml version="1.0" ?><Models>
<Category id="0" name="Guitar Overdrive">
  <Model id="1" name="Myth Drive"><Parameter defaultValue="5" max="10" min="0" name="GAIN" type="float"/>
    <Parameter max="1" min="0" name="SECRET" type="float" hidden="true"/></Model>
  <Model id="7" name="Plugin Drive" plugin_id="9"><Parameter max="10" min="0" name="GAIN" type="float"/></Model>
</Category>
<Category id="12" name="Cabsim Guitar (M)">
  <Model id="1000" name="Cab"><Parameter max="MAX_CABSIM_DB" min="MIN_CABSIM_DB" name="LEVEL" type="float"/>
    <Parameter max="20000" min="20" name="HPF" type="float" skew="LOG_SKEW"/>
    <Parameter max="6" min="-40" name="TRIM" type="float" skew="4.9594844"/></Model>
</Category>
<Category id="20" name="Neural Capture Internal" hidden="true">
  <Model id="20000" name="NC_Recorder"><Parameter max="1" min="0" name="X" type="float"/></Model>
  <Model id="20001" name="NC_Trainer"><Parameter max="1" min="0" name="X" type="float"/></Model>
</Category></Models>"""

FAKE_CHILD = textwrap.dedent(r'''
    import json, sys
    state_path, log_path = sys.argv[1], sys.argv[2]
    TOOLS = ["connect", "device_info", "current_preset_position", "directory_summary",
             "list_empty_slots", "add_block", "set_parameter", "save_preset_as",
             "save_preset", "set_io_port", "clear_grid", "find_devices"]
    def reply(rid, payload):
        text = payload if isinstance(payload, str) else json.dumps(payload)
        print(json.dumps({"jsonrpc": "2.0", "id": rid, "result": {
            "content": [{"type": "text", "text": text}], "isError": False}}), flush=True)
    for line in sys.stdin:
        msg = json.loads(line)
        m, rid = msg.get("method"), msg.get("id")
        if m == "initialize":
            print(json.dumps({"jsonrpc": "2.0", "id": rid, "result": {
                "protocolVersion": "2025-06-18", "capabilities": {"tools": {}},
                "serverInfo": {"name": "fake", "version": "0"}}}), flush=True)
        elif m == "tools/list":
            print(json.dumps({"jsonrpc": "2.0", "id": rid, "result": {
                "tools": [{"name": t, "inputSchema": {"type": "object"}} for t in TOOLS]}}),
                flush=True)
        elif m == "tools/call":
            name, args = msg["params"]["name"], msg["params"].get("arguments", {})
            with open(log_path, "a") as fh:
                fh.write(json.dumps({"name": name, "args": args, "id": rid}) + "\n")
            st = json.load(open(state_path))
            if name == "device_info":
                reply(rid, {"firmware": st["firmware"]})
            elif name == "current_preset_position":
                reply(rid, st["position"])
            elif name == "directory_summary":
                reply(rid, {"presets": {"folders": 1, "files": 3}, "source": st["source"]})
            elif name == "find_devices":
                sys.stdout.buffer.write((json.dumps({"jsonrpc": "2.0", "id": rid, "result": {
                    "content": [{"type": "text", "text": "Based on Klon\u00ae Centaur\u00ae"}],
                    "isError": False}}, ensure_ascii=False) + "\n").encode("utf-8"))
                sys.stdout.buffer.flush()
            elif name == "list_empty_slots":
                reply(rid, {"setlist": args.get("setlist_key"), "source": st["source"],
                            "empty_positions": [{"position": p} for p in st["empty"]]})
            else:
                reply(rid, "ok " + name)
''')

SLOT = P.parse_slot("12H")          # 95
SETLIST = P.DEFAULT_SETLIST


class Harness:
    """Runs a Proxy in-process against the fake child over in-memory streams."""

    def __init__(self, tmp, mode="write", **state):
        self.state_path = os.path.join(tmp, "state.json")
        self.log_path = os.path.join(tmp, "calls.log")
        self.set_state(**state)
        child = os.path.join(tmp, "fake_child.py")
        with open(child, "w") as fh:
            fh.write(FAKE_CHILD)
        repo = os.path.join(tmp, "ModelRepo.xml")
        with open(repo, "w") as fh:
            fh.write(MINI_REPO)
        r, w = os.pipe()
        self._w = os.fdopen(w, "w")
        self.out = _Lines()
        self.proxy = P.Proxy([sys.executable, "-I", child, self.state_path, self.log_path],
                             dict(os.environ), tmp, mode=mode, firmware="4.1.0",
                             slot=SLOT, setlist=SETLIST, catalog=P.load_catalog(repo),
                             out=self.out, inp=os.fdopen(r), on_child_exit=lambda: None)
        self.thread = threading.Thread(target=self.proxy.run, daemon=True)
        self.thread.start()
        self._id = 0
        self.request("initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                                    "clientInfo": {"name": "t", "version": "0"}})

    def set_state(self, firmware="4.1.0", position=None, source="device", empty=(SLOT,)):
        pos = position if position is not None else {
            "folder_key": SETLIST, "position": SLOT, "is_factory": False}
        with open(self.state_path, "w") as fh:
            json.dump({"firmware": firmware, "position": pos, "source": source,
                       "empty": list(empty)}, fh)

    def request(self, method, params=None):
        self._id += 1
        self._w.write(json.dumps({"jsonrpc": "2.0", "id": self._id, "method": method,
                                  "params": params or {}}) + "\n")
        self._w.flush()
        return self.out.wait_for(self._id)

    def call(self, _tool, **args):
        return self.request("tools/call", {"name": _tool, "arguments": args})

    def calls(self):
        if not os.path.exists(self.log_path):
            return []
        with open(self.log_path) as fh:
            return [json.loads(line) for line in fh]

    def close(self):
        self._w.close()
        self.thread.join(timeout=10)
        self.proxy.inp.close()
        self.proxy.child.wait(timeout=10)
        self.proxy.child.stdout.close()


class _Lines(io.TextIOBase):
    def __init__(self):
        self._buf, self._msgs, self._cv = "", {}, threading.Condition()
        self.raw_lines = []

    def write(self, s):
        with self._cv:
            self._buf += s
            while "\n" in self._buf:
                line, self._buf = self._buf.split("\n", 1)
                self.raw_lines.append(line)
                msg = json.loads(line)
                self._msgs[msg.get("id")] = msg
            self._cv.notify_all()
        return len(s)

    def wait_for(self, rid, timeout=20):
        end = time.time() + timeout
        with self._cv:
            while rid not in self._msgs:
                left = end - time.time()
                if left <= 0:
                    raise AssertionError(f"no reply to id {rid}")
                self._cv.wait(left)
            return self._msgs.pop(rid)


def text(reply):
    return reply["result"]["content"][0]["text"]


class CatalogRules(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        path = os.path.join(self.tmp.name, "r.xml")
        with open(path, "w") as fh:
            fh.write(MINI_REPO)
        self.cat = P.load_catalog(path)

    def tearDown(self):
        self.tmp.cleanup()

    def test_denies_crash_model_and_hidden_category(self):
        for mid in (20000, 20001):
            with self.assertRaises(P.Refused):
                P.check_model(self.cat, mid)

    def test_denies_plugin_and_unknown(self):
        for mid in (7, 999):
            with self.assertRaises(P.Refused):
                P.check_model(self.cat, mid)

    def test_param_rules(self):
        P.check_param(self.cat, 1, 0, 6.5)                 # ok
        P.check_param(self.cat, 1000, 2, -6)               # numeric skew, in range
        for args in ((1, 0, 11), (1, 0, float("nan")), (1, 1, 0.5), (1, 5, 1),
                     (1000, 0, -6), (1000, 1, 400), (1, 0, True), (1, "x", 1)):
            with self.subTest(args=args), self.assertRaises(P.Refused):
                P.check_param(self.cat, *args)

    def test_slot_labels(self):
        self.assertEqual(P.parse_slot("1A"), 0)
        self.assertEqual(P.parse_slot("12H"), 95)
        self.assertEqual(P.parse_slot("95"), 95)
        self.assertEqual(P.slot_label(95), "12H")
        for bad in ("0A", "12I", "H12", ""):
            with self.assertRaises(ValueError):
                P.parse_slot(bad)


class ProxyBehaviour(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.h.close()
        self.tmp.cleanup()

    def test_tools_list_is_filtered(self):
        self.h = Harness(self.tmp.name, mode="read")
        names = {t["name"] for t in self.h.request("tools/list")["result"]["tools"]}
        self.assertEqual(names, {"connect", "device_info", "current_preset_position",
                                 "directory_summary", "list_empty_slots", "find_devices"})

    def test_read_mode_refuses_writes_without_touching_child(self):
        self.h = Harness(self.tmp.name, mode="read")
        for name in ("add_block", "save_preset", "set_io_port", "clear_grid"):
            r = self.h.call(name, device_hash=1, row=0, column=0)
            self.assertTrue(r["result"]["isError"], name)
        self.assertEqual(self.h.calls(), [])

    def test_never_forwards_save_preset_or_globals_in_write_mode(self):
        self.h = Harness(self.tmp.name)
        for name in ("save_preset", "set_io_port", "clear_grid"):
            self.assertIn("not allowed", text(self.h.call(name)))
        self.assertEqual(self.h.calls(), [])

    def test_connect_is_forced_direct(self):
        self.h = Harness(self.tmp.name)
        self.h.call("connect", mode="bridge", quit_app=True)
        self.assertEqual(self.h.calls()[-1]["args"], {"mode": "direct", "quit_app": False})

    def test_write_checks_firmware_then_slot_then_forwards(self):
        self.h = Harness(self.tmp.name)
        r = self.h.call("add_block", row=0, column=1, device_hash=1)
        self.assertFalse(r["result"]["isError"])
        self.assertEqual([c["name"] for c in self.h.calls()],
                         ["device_info", "current_preset_position", "add_block"])

    def test_wrong_firmware_blocks_writes(self):
        self.h = Harness(self.tmp.name, firmware="4.1.1")
        self.assertIn("4.1.1", text(self.h.call("add_block", row=0, column=1, device_hash=1)))
        self.assertNotIn("add_block", [c["name"] for c in self.h.calls()])

    def test_other_slot_open_blocks_writes(self):
        self.h = Harness(self.tmp.name, position={"folder_key": SETLIST, "position": 3,
                                                  "is_factory": False})
        self.assertIn("not the trial slot", text(self.h.call("remove_block", row=0, column=1)))
        self.assertNotIn("remove_block", [c["name"] for c in self.h.calls()])

    def test_crash_model_never_reaches_child(self):
        self.h = Harness(self.tmp.name)
        self.assertIn("crashed", text(self.h.call("add_block", row=0, column=1,
                                                  device_hash=20000)))
        self.assertEqual(self.h.calls(), [])

    def test_set_parameter_needs_hash_and_range(self):
        self.h = Harness(self.tmp.name)
        self.assertIn("device_hash", text(self.h.call(
            "set_parameter", row=0, column=1, param_index=0, display_value=0.5)))
        self.assertIn("symbolic", text(self.h.call(
            "set_parameter", row=0, column=1, param_index=0, display_value=-6,
            device_hash=1000)))
        self.assertEqual(self.h.calls(), [])

    def test_save_rewritten_to_slot_after_fresh_empty_check(self):
        self.h = Harness(self.tmp.name)
        r = self.h.call("save_preset_as", name="Trial 1", position=SLOT, overwrite=True,
                        setlist_key="")
        self.assertFalse(r["result"]["isError"], r)
        calls = self.h.calls()
        self.assertEqual([c["name"] for c in calls],
                         ["device_info", "current_preset_position", "directory_summary",
                          "list_empty_slots", "save_preset_as"])
        self.assertEqual(calls[2]["args"], {"refresh": True})
        self.assertEqual(calls[-1]["args"], {"name": "Trial 1", "setlist_key": SETLIST,
                                             "position": SLOT, "overwrite": False})

    def test_save_refused_when_slot_not_empty_or_listing_stale(self):
        self.h = Harness(self.tmp.name, empty=(3, 4))
        self.assertIn("not empty", text(self.h.call("save_preset_as", name="T",
                                                    position=SLOT)))
        self.h.set_state(source="snapshot")
        self.assertIn("refresh", text(self.h.call("save_preset_as", name="T",
                                                 position=SLOT)))
        self.assertNotIn("save_preset_as", [c["name"] for c in self.h.calls()])

    def test_grid_bounds_and_unknown_keys(self):
        self.h = Harness(self.tmp.name)
        self.assertIn("row must be", text(self.h.call("remove_block", row=9, column=0)))
        self.assertIn("row must be", text(self.h.call("add_block", row=0, column=8,
                                                      device_hash=1)))
        self.assertEqual(self.h.calls(), [])
        self.h.call("add_block", row=0, column=1, device_hash=1, params={"0": 6},
                    sneaky="x")
        self.assertEqual(self.h.calls()[-1]["args"],
                         {"row": 0, "column": 1, "device_hash": 1, "params": {"0": 6.0}})
        self.assertIn("scenes", text(self.h.call("set_block_bypass", row=0, column=1,
                                                 scenes=[True] * 9)))

    def test_non_ascii_survives_and_is_escaped(self):
        self.h = Harness(self.tmp.name)
        self.h.out.raw_lines = []
        r = self.h.call("find_devices", query="Myth")
        self.assertEqual(text(r), "Based on Klon\u00ae Centaur\u00ae")
        self.assertTrue(all(line.isascii() for line in self.h.out.raw_lines))

    def test_save_refused_for_other_slot(self):
        self.h = Harness(self.tmp.name)
        self.assertIn("only target", text(self.h.call("save_preset_as", name="T",
                                                     position=-1)))
        self.assertEqual(self.h.calls(), [])


class WriteConfig(unittest.TestCase):
    def test_merges_and_backs_up(self):
        with tempfile.TemporaryDirectory() as tmp:
            checkout = os.path.join(tmp, "qc-mcp")
            os.makedirs(os.path.join(checkout, "src", "qc_mcp"))
            with open(os.path.join(checkout, "src", "qc_mcp", "ModelRepo.xml"), "w") as fh:
                fh.write(MINI_REPO)
            cfg = os.path.join(tmp, "claude_desktop_config.json")
            with open(cfg, "w", encoding="utf-8-sig") as fh:      # BOM, as some editors write
                json.dump({"mcpServers": {"other": {"command": "x"}}, "theme": "dark"}, fh)
            rc = P.main(["--qc-mcp", checkout, "--python", sys.executable, "--mode", "write",
                         "--firmware", "4.1.0", "--slot", "12H", "--write-config", cfg])
            self.assertEqual(rc, 0)
            with open(cfg, "rb") as fh:
                raw = fh.read()
            self.assertFalse(raw.startswith(b"\xef\xbb\xbf"))     # written without a BOM
            data = json.loads(raw)
            self.assertEqual(data["theme"], "dark")
            self.assertIn("other", data["mcpServers"])
            entry = data["mcpServers"]["qc-trial"]
            self.assertEqual(entry["command"], os.path.abspath(sys.executable))
            self.assertEqual(entry["args"][0], os.path.abspath(P.__file__))
            self.assertEqual(entry["args"][-6:], ["--mode", "write", "--firmware", "4.1.0",
                                                  "--slot", "12H"])
            self.assertTrue(os.path.exists(cfg + ".bak"))

    def test_bad_slot_rejected_before_writing(self):
        with tempfile.TemporaryDirectory() as tmp, self.assertRaises(ValueError):
            P.main(["--qc-mcp", tmp, "--firmware", "4.1.0", "--slot", "12Z",
                    "--write-config", os.path.join(tmp, "x.json")])


@unittest.skipUnless(os.environ.get("QC_MCP_MODELREPO"), "set QC_MCP_MODELREPO")
class RealCatalog(unittest.TestCase):
    def test_real_modelrepo(self):
        cat = P.load_catalog(os.environ["QC_MCP_MODELREPO"])
        with self.assertRaises(P.Refused):
            P.check_model(cat, 20000)                       # NC_Recorder
        self.assertEqual(cat[1]["name"], "Myth Drive")
        P.check_param(cat, 1, 0, 6.5)                       # Myth Drive GAIN 0..10
        blocked = [m for m in cat.values() if m["blocked"]]
        self.assertTrue(any(m["name"] == "NC_Recorder" for m in blocked))


if __name__ == "__main__":
    unittest.main()
