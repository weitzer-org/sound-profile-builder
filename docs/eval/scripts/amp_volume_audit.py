#!/usr/bin/env python3
"""Phase 1B follow-up: classify what each eval_architect run wrote for the modeled amp's
Volume on 07_Edge (UK C30 TopBoost, is_capture:false), more broadly than internal/evalcheck's
capture_db_on_modeled, which only counts values carrying a "dB" unit and so misses bare signed
offsets such as "-2.0".

Usage: python3 -I docs/eval/scripts/amp_volume_audit.py <runs.jsonl> [<runs.jsonl> ...]
"""
import collections, json, re, sys

AMP = "UK C30 TopBoost"
ORDER = ["dB-offset", "signed-offset-no-unit", "zero", "out-of-range", "dial-0-10"]

def cls(v):
    v = (v or "").strip()
    if not v:
        return None
    if re.search(r"dB", v, re.I):
        return "dB-offset"
    if v[0] in "+-":
        return "signed-offset-no-unit"
    try:
        x = float(v)
    except ValueError:
        return "other"
    return "zero" if x == 0 else "dial-0-10" if 0 < x <= 10 else "out-of-range"

def audit(paths):
    tab = collections.defaultdict(collections.Counter)
    for path in paths:
        for line in open(path):
            r = json.loads(line)
            if r["query"] != "07_Edge":
                continue
            key = (r["model"], r.get("architect_prompt_version") or "v4", r.get("architect_thinking_level") or "default")
            if not r.get("output_file") or not r.get("scoreable"):
                tab[key]["unscoreable"] += 1
                continue
            env = json.load(open(r["output_file"]))
            worst = None
            for blocks in env["structured_payload"]["guitars"].values():
                for b in blocks:
                    if b["model"] != AMP:
                        continue
                    for p in b["parameters"]:
                        if p["name"] in ("Volume", "Master Volume", "Gain", "Bass", "Mid", "Treble"):
                            for v in (p.get("value"), p.get("value_b")):
                                c = cls(v)
                                if c in ("other", None):
                                    continue
                                if worst is None or ORDER.index(c) < ORDER.index(worst):
                                    worst = c
            tab[key][worst or "no-dial-values"] += 1
    return tab

if __name__ == "__main__":
    for (m, pv, tl), c in sorted(audit(sys.argv[1:]).items()):
        n = sum(c.values())
        bad = c["dB-offset"] + c["signed-offset-no-unit"]
        print(f"{m:24} prompt={pv:3} thinking={tl:7} n={n:2} wrong-format={bad:2}  {dict(c)}")
