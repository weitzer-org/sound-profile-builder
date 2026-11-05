#!/usr/bin/env python3
"""Generalization of amp_volume_audit.py to every query: for each scoreable eval_architect run,
find amp/drive-family blocks that coros_map.json says are NOT captures (is_capture absent/false,
and not a user_captures.json entry) and flag any dial value (Gain/Bass/Mid/Treble/Volume/
Master Volume/Drive/Tone/Level) written as relative dB or as a bare signed offset ("-2.0").
Graphic/parametric EQ blocks are excluded: dB is their native unit.

Usage: python3 -I docs/eval/scripts/dial_format_audit.py <runs.jsonl> ...
"""
import collections, json, os, re, sys

ROOT = os.path.join(os.path.dirname(__file__), "..", "..", "..")
cmap = json.load(open(os.path.join(ROOT, "internal/agents/coros_map.json")))
ucap = json.load(open(os.path.join(ROOT, "internal/agents/user_captures.json")))
norm = lambda s: re.sub(r"\s+", " ", (s or "").strip()).lower()
captures = {norm(v["coros_equivalent"]) for v in cmap.values() if v.get("is_capture")} | {norm(u["name"]) for u in ucap}
known = {norm(v["coros_equivalent"]) for v in cmap.values()}
DIALS = {"gain", "bass", "mid", "treble", "volume", "master volume", "drive", "tone", "level"}
FAMILY = re.compile(r"(?i)amp|drive|overdrive|fuzz|boost|distortion")

def bad(v):
    v = (v or "").strip()
    return bool(v) and (re.search(r"(?i)\bdB\b", v) is not None or v[0] in "+-")

def audit(paths):
    out = collections.defaultdict(lambda: collections.Counter())
    examples = collections.defaultdict(list)
    for path in paths:
        for line in open(path):
            r = json.loads(line)
            key = (r["model"], r.get("architect_prompt_version") or "v4", r.get("architect_thinking_level") or "default", r["query"])
            if not r.get("scoreable"):
                out[key]["unscoreable"] += 1
                continue
            env = json.load(open(r["output_file"]))
            hit = False
            for g, blocks in env["structured_payload"]["guitars"].items():
                for b in blocks:
                    name = norm(b["model"])
                    if not FAMILY.search(b.get("type", "")) or "eq" in (b.get("type", "").lower()) or name in captures or name not in known:
                        continue
                    for p in b["parameters"]:
                        if norm(p["name"]) in DIALS and (bad(p.get("value")) or bad(p.get("value_b"))):
                            hit = True
                            if len(examples[key]) < 3:
                                examples[key].append(f'{b["model"]} {p["name"]}={p.get("value")}/{p.get("value_b")}')
            out[key]["wrong" if hit else "ok"] += 1
    return out, examples

if __name__ == "__main__":
    out, ex = audit(sys.argv[1:])
    for k in sorted(out):
        m, pv, tl, q = k
        print(f"{q:11} {m:24} prompt={pv} thinking={tl:7} {dict(out[k])}  {ex.get(k, [])}")
