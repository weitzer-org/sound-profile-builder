#!/usr/bin/env python3
"""Gate G4: do the taper and symbolic-bound constants hold on this unit's firmware?

Edits the LIVE edit buffer of the test preset that G3 saved (it never saves, and the guard does not
even grant a save method). For three parameters it writes a known WIRE value, reads it back, prints
what the plan's constants predict the unit will DISPLAY, and asks you to type what the unit's screen
actually shows. It also prints the prediction under a plain linear mapping so the readings can tell
the two hypotheses apart.

The constants under test (plan section 7, G4; pyquadcortex's 4.0.1 measurements used as DATA):

  display = min + (max - min) * wire ** (1 / skew)

  amp OUTPUT     skew 3.8018      -60..+12 dB     (Brit 2203, param 6)
  cab LEVEL      skew 4.9594844   -40..+6 dB      (MIN/MAX_CABSIM_DB are symbolic in the XML; -40/+6 is the claim)
  HPF FREQ       skew 0.3         20..20000 Hz    (Low-High Cut, param 1)

On the unit, open each block (tap it) so its parameter is on screen. After the run, select any other
preset WITHOUT saving to discard the edits.
"""
from __future__ import annotations

import argparse
import re
import sys
import time

import spike_common as sc
import test_preset as tp

# (label, column, param index, vmin, vmax, skew, unit, wires to test, tolerance)
CASES = [
    ("Brit 2203 OUTPUT (amp)", tp.AMP_COL, tp.AMP_OUTPUT_IDX, -60.0, 12.0, 3.8018, "dB", [0.25, 0.5, 1.0], ("abs", 0.15)),
    ("Cab LEVEL, mic 1 (CABSIM dB)", tp.CAB_COL, tp.CAB_LEVEL_IDX, sc.CABSIM_DB_RANGE[0], sc.CABSIM_DB_RANGE[1], 4.9594844, "dB",
     [0.01, 0.5, 1.0], ("abs", 0.15)),
    ("Low-High Cut HPF FREQ", tp.EQ_COL, tp.EQ_HPF_IDX, 20.0, 20000.0, 0.3, "Hz", [0.25, 0.5, 1.0], ("rel", 0.03)),
]


CASE_KEYS = ("amp", "cab", "hpf")   # CASES[i] is selected by CASE_KEYS[i]


def parse_reading(s: str) -> float | None:
    s = s.strip().lower().replace(",", ".")
    if not s or s in ("s", "skip"):
        return None
    m = re.match(r"^(-?\d+(?:\.\d+)?)\s*(k)?", s)
    if not m:
        return None
    v = float(m.group(1))
    return v * 1000.0 if m.group(2) else v


def within(pred: float, got: float, tol: tuple[str, float]) -> bool:
    kind, t = tol
    return abs(pred - got) <= t if kind == "abs" else abs(pred - got) <= t * abs(pred)


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sc.add_common_args(ap, writes=True)
    ap.add_argument("--slot", required=True, help="the slot G3 saved the test preset into")
    ap.add_argument("--folder-key", default=sc.SETLIST)
    ap.add_argument("--no-prompt", action="store_true", help="write and read back only; skip the screen readings")
    ap.add_argument("--case", choices=CASE_KEYS, help="single-step mode: write ONE value to this parameter (amp OUTPUT, cab LEVEL or HPF FREQ), then exit; needs --wire")
    ap.add_argument("--wire", type=float, help="the wire value (0..1) for --case")
    ap.add_argument("--hold", type=int, default=0, metavar="SECONDS",
                    help="photo mode: after each write, ring the bell and wait SECONDS (no typing) so you can photograph the unit; implies --no-prompt")
    args = ap.parse_args()
    if args.hold:
        args.no_prompt = True
    if (args.case is None) != (args.wire is None):
        ap.error("--case and --wire go together")
    if args.wire is not None and not (0.0 <= args.wire <= 1.0):
        ap.error("--wire must be between 0 and 1")
    cases = CASES
    if args.case:
        base = CASES[CASE_KEYS.index(args.case)]
        cases = [base[:7] + ([args.wire],) + base[8:]]
        args.no_prompt = True
        args.hold = 0

    mcp = sc.load_qc_mcp(args.qc_mcp, allow_unpinned=args.allow_unpinned)
    sc.banner("G4: taper and symbolic-bound constants  (live edits only, never saves)")
    qc = sc.open_device(mcp, sc.GRANT_G4)
    result = sc.base_result("G4", mcp, args)
    try:
        sc.check_firmware(qc, args.firmware, required=True)
        result["firmware"] = qc.firmware
        sc.require_open_test_preset(qc, args, tp)
        sc.say("Test preset recognised. Writing wire values to the LIVE buffer; nothing is saved.\n")

        rows = []
        quit_all = False
        total = sum(len(c[7]) for c in cases)
        step = 0
        for label, col, idx, vmin, vmax, skew, unit, wires, tol in cases:
            if quit_all:
                break
            sc.say(f"--- {label}: block at col {col}, param index {idx}, {vmin}..{vmax} {unit}, skew {skew}")
            sc.say(f"    On the unit, tap the block at column {col + 1} so this parameter is visible.")
            for wire in wires:
                qc.set_param(tp.ROW, col, idx, wire)
                time.sleep(0.6)
                bp2 = qc.get_current_preset()
                prm = sc.get_param(bp2, tp.ROW, col, idx)
                got = sc.param_value(prm) if prm is not None else None
                pred = sc.predict_display(vmin, vmax, skew, wire)
                linear = vmin + (vmax - vmin) * wire
                row = {"label": label, "col": col, "index": idx, "unit": unit, "wire_written": wire, "wire_read": got,
                       "wire_ok": isinstance(got, float) and abs(got - wire) <= 1e-4,
                       "predicted_taper": round(pred, 4), "predicted_linear": round(linear, 4), "screen": None}
                sc.say(f"    wire {wire:<5}  read-back {sc.fmt_num(got)}   "
                       f"taper predicts {pred:.2f} {unit}   (linear would be {linear:.2f})")
                step += 1
                if args.hold:
                    row["step"] = step
                    row["held_at"] = time.strftime("%H:%M:%S")
                    sc.say(f"      \a>>> PHOTO {step}/{total}: {label}, wire {wire}. Holding {args.hold}s.")
                    time.sleep(args.hold)
                if not args.no_prompt:
                    raw = input(f"      what does the unit show? (number, 's' skip, 'q' stop) > ")
                    if raw.strip().lower() == "q":
                        quit_all = True
                        rows.append(row)
                        break
                    row["screen"] = parse_reading(raw)
                    row["screen_raw"] = raw.strip()
                    if row["screen"] is not None:
                        row["taper_ok"] = within(pred, row["screen"], tol)
                        row["linear_ok"] = within(linear, row["screen"], tol)
                rows.append(row)
        result["rows"] = rows

        sc.banner("G4 summary")
        sc.say(f"{'parameter':<30} {'wire':>5} {'taper':>9} {'linear':>9} {'screen':>9}  fits")
        n_read = n_taper = 0
        for r in rows:
            fit = ""
            if r["screen"] is not None:
                n_read += 1
                n_taper += bool(r.get("taper_ok"))
                fit = "taper" if r.get("taper_ok") and not r.get("linear_ok") else "linear" if r.get("linear_ok") and not r.get("taper_ok") else \
                      "both" if r.get("taper_ok") else "NEITHER"
            sc.say(f"{r['label']:<30} {r['wire_written']:>5} {r['predicted_taper']:>9} {r['predicted_linear']:>9} "
                   f"{'' if r['screen'] is None else r['screen']:>9}  {fit}")
        wire_bad = [r for r in rows if not r["wire_ok"]]
        result["wire_roundtrip_failures"] = len(wire_bad)
        result["readings"] = n_read
        result["taper_matches"] = n_taper
        result["verdict"] = ("SINGLE_STEP" if args.case else "NO_READINGS" if n_read == 0 else "TAPER_CONFIRMED" if n_taper == n_read and not wire_bad else "TAPER_NOT_CONFIRMED")
        sc.say(f"\nWire round-trip failures: {len(wire_bad)}.  Screen readings: {n_read}, matching the taper: {n_taper}.")
        sc.say(f"G4 {result['verdict']}")
        sc.say("\nNow select any OTHER preset on the unit WITHOUT saving, to discard these edits.")
        return sc.EXIT_OK if result["verdict"] in ("TAPER_CONFIRMED", "SINGLE_STEP") else sc.EXIT_CHECK_FAILED
    finally:
        result["close"] = sc.close_with_timeout(qc)
        path = sc.write_result(args.out, "g4", result)
        sc.say(f"\nFull result saved to: {path}")


if __name__ == "__main__":
    sys.exit(sc.run_spike(main))
