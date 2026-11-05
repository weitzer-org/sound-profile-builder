#!/usr/bin/env python3
"""Gate G2: does a library-mode session start from a shortcut, fail cleanly, and notice the unit vanish?

READ-ONLY. Three checks, chosen with ``--check``:

  a  Connect, read, disconnect, then connect AGAIN to prove the first disconnect released the unit.
     Run this from a desktop shortcut (see README) with ``--pause`` so the window stays open.
  b  Run with Cortex Control OPEN. Expected: a clear refusal within seconds, no traceback, no hang,
     and Cortex Control undisturbed. Records what actually happens.
  c  Connect, then watch while you switch the unit off (or unplug USB). Logs three candidate
     "unit is gone" signals every second and which one fires first, then tries to close without hanging.
     Use ``--label power-off`` or ``--label unplug`` so the two runs are told apart.

Why (c) logs signals instead of assuming one: in qc-mcp f634ecd the Windows reader thread exits on a
read error, writes come back as error codes rather than exceptions (the heartbeat swallows them), and
a probe that gets no answer raises QCError. Which of those is observable, and how fast, is the question.
"""
from __future__ import annotations

import argparse
import sys
import time

import spike_common as sc


def probe(qc: sc.Guarded) -> tuple[bool, str | None]:
    try:
        pos = qc.get_setlist_position()
        return pos is not None, None if pos is not None else "no reply"
    except Exception as e:  # noqa: BLE001
        return False, f"{type(e).__name__}: {e}"


def check_a(mcp, args, result) -> int:
    sessions = []
    for i in (1, 2):
        s = {"session": i}
        t0 = time.monotonic()
        qc = sc.open_device(mcp, sc.GRANT_READ)
        s["open_seconds"] = round(time.monotonic() - t0, 2)
        try:
            sc.check_firmware(qc, args.firmware, required=False)
            s["firmware"] = qc.firmware
            t1 = time.monotonic()
            s["setlist_position"] = qc.get_setlist_position()
            s["read_seconds"] = round(time.monotonic() - t1, 2)
            bp = qc.get_current_preset()
            s["preset_name"] = bp.name if bp is not None else None
            s["blocks"] = len(list(sc.iter_blocks(bp))) if bp is not None else None
            sc.say(f"  session {i}: position={s['setlist_position']} preset={s['preset_name']!r} blocks={s['blocks']}")
        finally:
            s["close"] = sc.close_with_timeout(qc)
            sc.say(f"  session {i} close: {s['close']}")
        sessions.append(s)
        time.sleep(1.0)
    result["sessions"] = sessions
    ok = all(s.get("preset_name") is not None and s["close"]["closed"] and not s["close"]["hung"] for s in sessions)
    result["verdict"] = "PASS" if ok else "FAIL"
    sc.say(f"\nG2(a) {'PASS' if ok else 'FAIL'}: two back-to-back sessions, each connected, read and disconnected"
           f"{'' if ok else ' (see above)'}.")
    sc.say("If you started this from the desktop shortcut, say so when you paste the result.")
    return sc.EXIT_OK if ok else sc.EXIT_CHECK_FAILED


def check_b(mcp, args, result) -> int:
    sc.say("Expecting a refusal because Cortex Control holds the unit. Trying to open...")
    t0 = time.monotonic()
    try:
        qc = sc.open_device(mcp, sc.GRANT_READ)
    except RuntimeError as e:
        dt = round(time.monotonic() - t0, 2)
        msg = str(e)
        result.update({"opened": False, "seconds_to_fail": dt, "message": msg, "exception": type(e).__name__})
        clean = "already open exclusively" in msg or "Cortex Control" in msg
        result["verdict"] = "PASS" if (clean and dt < 15) else "FAIL_UNEXPECTED_MESSAGE"
        sc.say(f"Refused after {dt}s: {msg}")
        sc.say(f"\nG2(b) {'PASS' if result['verdict'] == 'PASS' else 'FAIL'}: "
               + ("failed cleanly with a clear message." if result["verdict"] == "PASS" else "the failure was not the clear message we expected."))
        sc.say("Now check Cortex Control: it should still be connected to the unit and working. Say what you see.")
        return sc.EXIT_OK if result["verdict"] == "PASS" else sc.EXIT_CHECK_FAILED
    # Opened: the unit was NOT exclusively held. That is itself the answer.
    result.update({"opened": True, "seconds_to_open": round(time.monotonic() - t0, 2)})
    result["close"] = sc.close_with_timeout(qc)
    result["verdict"] = "FAIL_OPENED_ANYWAY"
    sc.say("\nThe script OPENED the unit even though you said Cortex Control is running.")
    sc.say("Either Cortex Control was not really holding the unit, or the exclusive open does not conflict.")
    sc.say("Say exactly what you did to start Cortex Control and what it shows now (it may have lost its connection).")
    return sc.EXIT_CHECK_FAILED


def check_c(mcp, args, result) -> int:
    qc = sc.open_device(mcp, sc.GRANT_READ)
    gone_at: dict[str, float] = {}
    log = []
    base_errors = None
    try:
        sc.check_firmware(qc, args.firmware, required=False)
        result["firmware"] = qc.firmware
        base_errors = qc.write_errors
        sc.say(f"\nConnected. Probing once a second for up to {args.max_wait}s.")
        sc.say(f">>> NOW: {'SWITCH THE UNIT OFF' if args.label == 'power-off' else 'UNPLUG THE USB CABLE'} <<<")
        sc.say("    (the script keeps running and will stop itself)\n")
        t0 = time.monotonic()
        consecutive_dead = 0
        while time.monotonic() - t0 < args.max_wait:
            t = round(time.monotonic() - t0, 1)
            ok, err = probe(qc)
            d = qc.diagnostics()
            werr = (d["write_errors"] or 0) - (base_errors or 0)
            signals = {
                "probe_failed": not ok,
                "reader_thread_dead": d["reader_thread_alive"] is False,
                "write_errors_rising": werr > 0,
            }
            for k, v in signals.items():
                if v and k not in gone_at:
                    gone_at[k] = t
                    sc.say(f"    !! first '{k}' at t={t}s")
            entry = {"t": t, "probe_ok": ok, "probe_error": err, "write_errors_since_start": werr,
                     "reader_alive": d["reader_thread_alive"], "last_write_error": d["last_write_error"]}
            log.append(entry)
            sc.say(f"  t={t:6.1f}s  probe={'ok ' if ok else 'FAIL'}  reader_alive={d['reader_thread_alive']}  "
                   f"write_errors+={werr}  {err or ''}")
            consecutive_dead = consecutive_dead + 1 if not ok else 0
            if consecutive_dead >= args.dead_probes:
                sc.say(f"\n{args.dead_probes} failed probes in a row: treating the unit as gone.")
                break
            time.sleep(1.0)
    finally:
        result["close"] = sc.close_with_timeout(qc, 12.0)
        sc.say(f"\nclose() result: {result['close']}")
    result.update({"label": args.label, "signals_first_seen_s": gone_at, "log": log})
    detected = bool(gone_at)
    result["verdict"] = ("PASS" if detected and result["close"]["closed"] and not result["close"]["hung"] else
                         "FAIL_NOT_DETECTED" if not detected else "FAIL_CLOSE_HUNG_OR_ERRORED")
    sc.say(f"\nSignals first seen (seconds after start): {gone_at or 'NONE'}")
    sc.say(f"G2(c) [{args.label}] {result['verdict']}")
    if not detected:
        sc.say("The unit going away was not detected by any signal. The plan then keeps only the idle-timeout exit.")
    return sc.EXIT_OK if result["verdict"] == "PASS" else sc.EXIT_CHECK_FAILED


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sc.add_common_args(ap)
    ap.add_argument("--check", required=True, choices=["a", "b", "c"])
    ap.add_argument("--label", default="power-off", choices=["power-off", "unplug"], help="for --check c")
    ap.add_argument("--max-wait", type=int, default=150, help="for --check c: give up after this many seconds")
    ap.add_argument("--dead-probes", type=int, default=5, help="for --check c: stop after this many failed probes in a row")
    args = ap.parse_args()

    mcp = sc.load_qc_mcp(args.qc_mcp, allow_unpinned=args.allow_unpinned)
    sc.banner(f"G2({args.check}): session behaviour  (read-only)")
    result = sc.base_result(f"G2{args.check}", mcp, args)
    try:
        return {"a": check_a, "b": check_b, "c": check_c}[args.check](mcp, args, result)
    finally:
        path = sc.write_result(args.out, f"g2{args.check}", result)
        sc.say(f"\nFull result saved to: {path}")


if __name__ == "__main__":
    sys.exit(sc.run_spike(main))
