package deviceedit

import (
	"regexp"
	"sort"

	"github.com/weitzer-org/sound-builder/internal/deviceexport"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

func sortStrings(s []string) { sort.Strings(s) }

// tgt is one target block taking part in matching (device elements are already out).
type tgt struct {
	pos int // index in the target chain, for ordering
	blk *storage.EffectBlock
}

type pair struct {
	t, c  int // indexes into the tgt and exposed-cell slices
	moved bool
}

// matching is the result of matching target blocks to exposed cells.
type matching struct {
	pairs      []pair
	ambiguousT map[int]bool
	ambiguousC map[int]bool
}

var idRe = regexp.MustCompile(`^b([1-8])$`)

// match lines target blocks up with the unit's exposed cells (plan rule R3, with moved blocks
// keeping their id match). compat[t][c] says the target block means the cell's model.
func match(ts []tgt, cells []deviceexport.Cell, compat [][]bool) matching {
	m := matching{ambiguousT: map[int]bool{}, ambiguousC: map[int]bool{}}

	// 1. Id matches: a target id b<n> that is unique in the target, whose cell at col n-1 is the same model.
	idCount := map[string]int{}
	for _, t := range ts {
		idCount[t.blk.ID]++
	}
	colIdx := map[int]int{}
	for i, c := range cells {
		colIdx[c.Col] = i
	}
	var idPairs []pair
	usedT := map[int]bool{}
	usedC := map[int]bool{}
	for ti, t := range ts {
		sm := idRe.FindStringSubmatch(t.blk.ID)
		if sm == nil || idCount[t.blk.ID] != 1 {
			continue
		}
		ci, ok := colIdx[int(sm[1][0]-'0')-1]
		if !ok || !compat[ti][ci] {
			continue
		}
		idPairs = append(idPairs, pair{t: ti, c: ci})
		usedT[ti], usedC[ci] = true, true
	}

	// 2. Anchors: the longest strictly increasing run of id pairs by cell (leftmost on ties).
	// The other id pairs stay matched but are moves.
	anchorSet := longestIncreasing(idPairs)
	var anchors []pair
	for i, p := range idPairs {
		p.moved = !anchorSet[i]
		idPairs[i] = p
		if !p.moved {
			anchors = append(anchors, p)
		}
	}
	m.pairs = append(m.pairs, idPairs...)

	// 3. Alignment of the rest within the segments between anchors.
	bounds := append([]pair{{t: -1, c: -1}}, anchors...)
	bounds = append(bounds, pair{t: len(ts), c: len(cells)})
	for k := 1; k < len(bounds); k++ {
		var st, sc []int
		for ti := bounds[k-1].t + 1; ti < bounds[k].t; ti++ {
			if !usedT[ti] {
				st = append(st, ti)
			}
		}
		for ci := bounds[k-1].c + 1; ci < bounds[k].c; ci++ {
			if !usedC[ci] {
				sc = append(sc, ci)
			}
		}
		if len(st) == 0 || len(sc) == 0 {
			continue
		}
		ps, unique := align(st, sc, compat)
		if !unique {
			for _, ti := range st {
				m.ambiguousT[ti] = true
			}
			for _, ci := range sc {
				m.ambiguousC[ci] = true
			}
			continue
		}
		m.pairs = append(m.pairs, ps...)
	}
	sort.Slice(m.pairs, func(i, j int) bool { return m.pairs[i].t < m.pairs[j].t })
	return m
}

// longestIncreasing returns the members (by index) of the longest increasing-by-c subsequence of
// ps, which are in target order. Ties go to the earliest.
func longestIncreasing(ps []pair) map[int]bool {
	n := len(ps)
	best := make([]int, n)
	prev := make([]int, n)
	top := -1
	for i := range ps {
		best[i], prev[i] = 1, -1
		for j := 0; j < i; j++ {
			if ps[j].c < ps[i].c && best[j]+1 > best[i] {
				best[i], prev[i] = best[j]+1, j
			}
		}
		if top < 0 || best[i] > best[top] {
			top = i
		}
	}
	out := map[int]bool{}
	for i := top; i >= 0; i = prev[i] {
		out[i] = true
	}
	return out
}

// align finds the maximum order-preserving matching between the target indexes st and cell
// indexes sc under compat, and says whether it is the only one (as a set of pairs). With
// nothing compatible it returns no pairs and unique=true.
func align(st, sc []int, compat [][]bool) (pairs []pair, unique bool) {
	nt, nc := len(st), len(sc)
	best := make([][]int, nt+1)
	ways := make([][]int, nt+1)
	for i := range best {
		best[i] = make([]int, nc+1)
		ways[i] = make([]int, nc+1)
	}
	for i := nt; i >= 0; i-- {
		for j := nc; j >= 0; j-- {
			if i == nt || j == nc {
				best[i][j], ways[i][j] = 0, 1
				continue
			}
			mv := -1
			if compat[st[i]][sc[j]] {
				mv = 1 + best[i+1][j+1]
			}
			b := max3(mv, best[i+1][j], best[i][j+1])
			w := 0
			if mv == b {
				w += ways[i+1][j+1]
			}
			a, c := best[i+1][j] == b, best[i][j+1] == b
			if a {
				w += ways[i+1][j]
			}
			if c {
				w += ways[i][j+1]
			}
			// The configurations that use neither target i nor cell j are counted by both skips.
			if a && c && best[i+1][j+1] == b {
				w -= ways[i+1][j+1]
			}
			best[i][j], ways[i][j] = b, w
		}
	}
	if best[0][0] == 0 {
		return nil, true
	}
	if ways[0][0] != 1 {
		return nil, false
	}
	i, j := 0, 0
	for i < nt && j < nc && best[i][j] > 0 {
		if compat[st[i]][sc[j]] && 1+best[i+1][j+1] == best[i][j] && ways[i+1][j+1] == ways[i][j] {
			pairs = append(pairs, pair{t: st[i], c: sc[j]})
			i, j = i+1, j+1
		} else if best[i+1][j] == best[i][j] {
			i++
		} else {
			j++
		}
	}
	return pairs, true
}

func max3(a, b, c int) int {
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	return a
}
