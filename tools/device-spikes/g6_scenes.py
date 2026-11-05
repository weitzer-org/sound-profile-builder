#!/usr/bin/env python3
"""Gate G6: do per-scene parameter writes and per-scene bypass behave as qc-mcp documents?

Edits the LIVE edit buffer of the test preset that G3 saved. It never saves (no save method is
granted). On that preset it:

  1. makes Myth Drive GAIN scene-varying: Scene A = wire 0.20, Scene B = wire 0.80
     (``set_param_scenes``; qc-mcp assigns the param to scenes, then writes each scene);
  2. makes the Spring reverb's bypass scene-varying: Scene A = ON, Scene B = BYPASSED
     (``set_block_bypass(scenes=[False, True])``);
  3. switches to Scene A, then B, then A again (``set_scene``), and at each stop reads the preset
     back and records exactly how the device reports the per-scene values and bypass flags;
  4. asks you what the unit's screen shows at each stop.

The read-back shapes are recorded raw: how a per-scene value appears in a full-preset read is part
of what this gate is meant to find out (the plan's P5 read-back import depends on it).

Afterwards, select any other preset on the unit WITHOUT saving to discard the edits.
"""
from __future__ import annotations

import argparse
import sys
import time

import spike_common as sc
import test_preset as tp

GAIN_A, GAIN_B = 0.20, 0.80
TOL = 1e-4


def scene_state(qc, mcp, want_scene: int) -> dict:
    """Switch scene, wait for the device to confirm, read everything relevant back."""
    qc.set_scene(want_scene)
    confirmed = None
    deadline = time.time() + 3.0
    while time.time() < deadline:
        st = qc.read_state("Scene", timeout_ms=600)
        if st is not None and int(getattr(st, "selected_scene", -1)) == want_scene:
            confirmed = want_scene
            break
        time.sleep(0.1)
    time.sleep(0.4)
    bp = qc.get_current_preset()
    out = {"requested": want_scene, "device_confirms": confirmed, "gain": None, "bypass_raw": None}
    if bp is None:
        out["error"] = "preset read timed out"
        return out
    p = sc.get_param(bp, tp.ROW, tp.DRIVE_COL, tp.DRIVE_GAIN_IDX)
    if p is None:
        out["error"] = "preset structure is incomplete (no Myth Drive GAIN param in the read-back)"
        return out
    out["gain"] = {"scene_mode": p.scene_mode, "values": sc.param_values_all(p)}
    spec = mcp.preset.describe(bp)
    out["bypass_raw"] = spec.get("bypass")
    return out


def gain_shape(g: dict | None, scene: int) -> str | None:
    """Which reporting shape the read-back GAIN fits, or None if neither.

    "per_scene_list": values[0] is Scene A and values[1] is Scene B (what G1 saw for scene params).
    "active_only":    the active scene's value, whatever the list length (the device may replicate it).
    Both are evaluated independently; a list of 8 floats must not hide the second shape.
    """
    if not g or not g["values"]:
        return None
    vals = g["values"]
    want = GAIN_A if scene == 0 else GAIN_B
    if (len(vals) >= 2 and all(isinstance(v, float) for v in vals[:2])
            and abs(vals[0] - GAIN_A) <= TOL and abs(vals[1] - GAIN_B) <= TOL):
        return "per_scene_list"
    if isinstance(vals[0], float) and abs(vals[0] - want) <= TOL:
        return "active_only"
    return None


def gain_matches(g: dict | None, scene: int) -> bool | None:
    """True if the read-back GAIN fits either shape; None if there was nothing to check."""
    if not g or not g["values"]:
        return None
    return gain_shape(g, scene) is not None


def ask(prompt: str) -> str:
    return input(prompt).strip().lower()


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sc.add_common_args(ap, writes=True)
    ap.add_argument("--slot", required=True, help="the slot G3 saved the test preset into")
    ap.add_argument("--folder-key", default=sc.SETLIST)
    ap.add_argument("--no-prompt", action="store_true", help="skip the questions about the unit's screen")
    ap.add_argument("--hold", type=int, default=0, metavar="SECONDS",
                    help="photo mode: at each scene, ring the bell and wait SECONDS (no typing) so you can photograph the unit; implies --no-prompt")
    args = ap.parse_args()
    if args.hold:
        args.no_prompt = True

    mcp = sc.load_qc_mcp(args.qc_mcp, allow_unpinned=args.allow_unpinned)
    import qc_mcp.preset as qc_preset  # after load_qc_mcp has put src/ on sys.path
    mcp.preset = qc_preset
    sc.banner("G6: scene semantics  (live edits only, never saves)")
    qc = sc.open_device(mcp, sc.GRANT_G6)
    result = sc.base_result("G6", mcp, args)
    try:
        sc.check_firmware(qc, args.firmware, required=True)
        result["firmware"] = qc.firmware
        sc.require_open_test_preset(qc, args, tp)
        sc.say("Test preset recognised.\n")

        sc.say("Step 1: Myth Drive GAIN per scene (A=0.20, B=0.80)...")
        qc.set_param_scenes(tp.ROW, tp.DRIVE_COL, tp.DRIVE_GAIN_IDX, [GAIN_A, GAIN_B])
        time.sleep(0.5)
        sc.say("Step 2: Spring bypass per scene (A on, B bypassed)...")
        qc.set_block_bypass(tp.ROW, tp.REVERB_COL, scenes=[False, True])
        time.sleep(0.5)
        result["write_errors"] = qc.write_errors

        stops = []
        for scene in (0, 1, 0):
            name = "AB"[scene]
            sc.say(f"\nStep 3: switching to Scene {name}...")
            s = scene_state(qc, mcp, scene)
            s["gain_shape"] = gain_shape(s["gain"], scene)
            s["gain_fits_expectation"] = gain_matches(s["gain"], scene)
            sc.say(f"  device confirms scene: {s['device_confirms']}")
            sc.say(f"  Myth Drive GAIN as read back: {s['gain']}  -> fits expectation: {s['gain_fits_expectation']}")
            sc.say(f"  bypass section as read back: {s['bypass_raw']}")
            if args.hold:
                s["held_at"] = time.strftime("%H:%M:%S")
                sc.say(f"  \a>>> PHOTO {len(stops) + 1}/3: Scene {name} (Myth Drive GAIN and the Spring bypass). Holding {args.hold}s.")
                time.sleep(args.hold)
            if not args.no_prompt:
                exp_gain = "LOW (about 2 on the 0-10 dial)" if scene == 0 else "HIGH (about 8)"
                exp_byp = "ON (active)" if scene == 0 else "BYPASSED"
                s["screen_scene_shown"] = ask(f"  Unit screen: does it show Scene {name} selected? (y/n) > ")
                s["screen_gain_ok"] = ask(f"  Myth Drive GAIN should look {exp_gain}. Does it? (y/n) > ")
                s["screen_reverb_ok"] = ask(f"  Spring should be {exp_byp}. Is it? (y/n) > ")
            stops.append(s)
        result["stops"] = stops

        gain_ok = all(s["gain_fits_expectation"] for s in stops)
        confirmed_ok = all(s["device_confirms"] == s["requested"] for s in stops)
        screen_ok = None if args.no_prompt else all(
            s.get(k) == "y" for s in stops for k in ("screen_scene_shown", "screen_gain_ok", "screen_reverb_ok"))
        result.update({"gain_readback_ok": gain_ok, "scene_switch_confirmed": confirmed_ok, "screen_ok": screen_ok})
        result["verdict"] = ("PASS" if gain_ok and confirmed_ok and screen_ok is not False else "FAIL_OR_INCONCLUSIVE")
        sc.banner("G6 summary")
        sc.say(f"Scene switches confirmed by the device: {confirmed_ok}")
        sc.say(f"Per-scene GAIN read-back fits one of the two shapes: {gain_ok}")
        sc.say(f"Your screen observations all 'y': {screen_ok}")
        sc.say(f"G6 {result['verdict']}")
        sc.say("\nNow select any OTHER preset on the unit WITHOUT saving, to discard these edits.")
        return sc.EXIT_OK if result["verdict"] == "PASS" else sc.EXIT_CHECK_FAILED
    finally:
        result["close"] = sc.close_with_timeout(qc)
        path = sc.write_result(args.out, "g6", result)
        sc.say(f"\nFull result saved to: {path}")


if __name__ == "__main__":
    sys.exit(sc.run_spike(main))
