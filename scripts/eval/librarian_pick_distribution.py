#!/usr/bin/env python3
# Usage: python3 -I scripts/eval/librarian_pick_distribution.py <eval_librarian -out dir>
# Pre-registered rule (2026-11-05): NO ORDER EFFECT if the 95% CI upper bound of
# (between - within) < 0.10; ORDER EFFECT if the lower bound > 0; else INCONCLUSIVE.
import json,sys,glob,os,collections,random
# Pick-distribution comparison: is the BETWEEN-order difference in what the Librarian picks
# bigger than the WITHIN-order (repeat-to-repeat) difference? Equal-size halves on both sides.
d=sys.argv[1]
runs=collections.defaultdict(dict)  # (q,p) -> {(order,rep): set(names)}
for f in glob.glob(d+"/outputs/*.json"):
    b=os.path.basename(f)[:-5]; q,rest=b.split('_p',1); p,order,rep=rest.split('_'); rep=int(rep[1:])
    try: o=json.load(open(f))
    except Exception: continue
    runs[(q,int(p))][(order,rep)]=set(o.get('mandatory_blocks',[])+o.get('suggested_blocks',[])+o.get('verified_blocks',[]))
def freq(sets):
    c=collections.Counter(n for s in sets for n in s); return {n:c[n]/len(sets) for n in c}
def dist(A,B):
    fa,fb=freq(A),freq(B); names=set(fa)|set(fb)
    return sum(abs(fa.get(n,0)-fb.get(n,0)) for n in names)/len(names) if names else 0
diffs=[];W=[];Bt=[];skipped=0
for u,r in sorted(runs.items()):
    g=lambda o,reps:[r[(o,i)] for i in reps if (o,i) in r]
    nA,nB,oA,oB=g('new',[0,1]),g('new',[2,3]),g('old',[0,1]),g('old',[2,3])
    if min(map(len,(nA,nB,oA,oB)))<2: skipped+=1; continue
    within=(dist(nA,nB)+dist(oA,oB))/2; between=(dist(nA,oB)+dist(oA,nB))/2
    W.append(within);Bt.append(between);diffs.append(between-within)
rng=random.Random(1); ms=sorted(sum(rng.choice(diffs) for _ in diffs)/len(diffs) for _ in range(10000))
lo,hi=ms[250],ms[9749]; m=sum(diffs)/len(diffs)
v="NO ORDER EFFECT" if hi<0.10 else ("ORDER EFFECT" if lo>0 else "INCONCLUSIVE")
print(f"units {len(diffs)} (skipped {skipped}) | within {sum(W)/len(W):.3f} | between {sum(Bt)/len(Bt):.3f} | between-within {m:+.3f} 95% CI [{lo:+.3f}, {hi:+.3f}] -> {v}")
