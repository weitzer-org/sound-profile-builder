package evaljudge

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// Record is one ground-truth entry behind a name a judge mentioned.
type Record struct {
	Source string         `json:"source"` // coros_map.json | user_captures.json
	Key    string         `json:"key"`    // the entry's key / name
	Data   map[string]any `json:"data"`
}

// Mention is a block/capture name found in a judge's text.
type Mention struct {
	Judge    string   `json:"judge"` // "A" | "B"
	Field    string   `json:"field"` // Ground | Struct | Rationale
	Name     string   `json:"name"`  // the ground-truth name matched
	Span     string   `json:"span"`  // the text as the judge wrote it
	Kind     string   `json:"kind"`  // exact | fuzzy
	Distance int      `json:"distance"`
	Records  []Record `json:"records"`
	// MoreRecords counts records beyond the printed cap (a name like "Circular Delay" is the
	// coros_equivalent of many real-world devices).
	MoreRecords int `json:"more_records,omitempty"`
}

// Index looks names up in coros_map.json (real-world name AND the QC block name it maps to) and
// user_captures.json.
type Index struct {
	entries []indexEntry
}

type indexEntry struct {
	name    string // as written in the data
	norm    string
	tokens  int
	single  bool
	records []Record
}

const maxRecordsPerMention = 6

func normText(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// NewIndex builds the lookup from the repo's embedded ground truth.
func NewIndex() *Index {
	by := map[string]*indexEntry{}
	add := func(name string, rec Record) {
		n := normText(name)
		if len(n) < 4 {
			return
		}
		e := by[n]
		if e == nil {
			e = &indexEntry{name: name, norm: n, tokens: len(strings.Fields(n))}
			e.single = e.tokens == 1
			by[n] = e
		}
		e.records = append(e.records, rec)
	}
	coros := agents.CorosMapRecords()
	keys := make([]string, 0, len(coros))
	for k := range coros {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		rec := Record{Source: "coros_map.json", Key: k, Data: coros[k]}
		add(k, rec)
		// Many entries (e.g. plugin devices) use their own block name as the key; adding the
		// record under both would list it twice for that name.
		if eq, _ := coros[k]["coros_equivalent"].(string); eq != "" && normText(eq) != normText(k) {
			add(eq, rec)
		}
	}
	for _, c := range agents.GetUserCaptures() {
		raw, _ := json.Marshal(c)
		var data map[string]any
		_ = json.Unmarshal(raw, &data)
		add(c.Name, Record{Source: "user_captures.json", Key: c.Name, Data: data})
	}
	idx := &Index{}
	for _, e := range by {
		idx.entries = append(idx.entries, *e)
	}
	sort.Slice(idx.entries, func(i, j int) bool { return idx.entries[i].norm < idx.entries[j].norm })
	return idx
}

// Size is the number of distinct names indexed.
func (ix *Index) Size() int { return len(ix.entries) }

func tokenize(norm string) []string { return strings.Fields(norm) }

// Find returns every ground-truth name mentioned in text. Exact: the name's tokens occur
// contiguously (case/punctuation-insensitive; a one-word name must also match the record's own
// casing, so ordinary words are not mistaken for gear). Fuzzy: a window of the same token count
// within an edit distance of max(1, len/8) (e.g. a judge misspelling or a near-miss invention).
func (ix *Index) Find(text string) []Mention {
	norm := normText(text)
	toks := tokenize(norm)
	padded := " " + norm + " "
	var out []Mention
	taken := map[string]bool{}
	for _, e := range ix.entries {
		if !strings.Contains(padded, " "+e.norm+" ") {
			continue
		}
		if e.single && !strings.Contains(text, e.name) { // keeps "Effect"/"jewel" style common words out
			continue
		}
		taken[e.norm] = true
		out = append(out, mention(e, e.name, "exact", 0))
	}
	// Fuzzy pass, by window size.
	for _, e := range ix.entries {
		if taken[e.norm] || len(e.norm) < 6 || e.single {
			continue
		}
		maxD := len(e.norm) / 8
		if maxD < 1 {
			maxD = 1
		}
		k := e.tokens
		for i := 0; i+k <= len(toks); i++ {
			win := strings.Join(toks[i:i+k], " ")
			if d := len(win) - len(e.norm); d > maxD || -d > maxD {
				continue
			}
			if dist := agents.LevenshteinDistance([]rune(win), []rune(e.norm)); dist <= maxD && dist > 0 {
				out = append(out, mention(e, win, "fuzzy", dist))
				break
			}
		}
	}
	return out
}

func mention(e indexEntry, span, kind string, dist int) Mention {
	m := Mention{Name: e.name, Span: span, Kind: kind, Distance: dist}
	for i, r := range e.records {
		if i >= maxRecordsPerMention {
			m.MoreRecords = len(e.records) - maxRecordsPerMention
			break
		}
		m.Records = append(m.Records, r)
	}
	return m
}

// Claim-of-existence/provenance wording: the shape of the two July judge errors. A verdict whose
// text uses it is treated as naming a specific fact even when no indexed name matched (e.g. a
// name the judge invented or misspelled beyond fuzzy reach).
var claimWords = regexp.MustCompile(`(?i)\b(fabricat\w*|hallucinat\w*|invent(?:ed|s)?|made[- ]up|non-?existent|does(?: not|n't) exist|not a real|no such|exclusive(?:ly)? to|only (?:exists|available|in)|axe-?fx|kemper|helix|fractal)\b`)

// ClaimWords returns the distinct claim-wording hits in text.
func ClaimWords(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range claimWords.FindAllString(text, -1) {
		k := strings.ToLower(m)
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// JudgeText is the part of a verdict a human reads.
type JudgeText struct {
	Judge     string  `json:"judge"` // model id (or "A"/"B" when unknown)
	Winner    string  `json:"winner"`
	Conf      float64 `json:"confidence"`
	Ground    string  `json:"grounding_notes"`
	Struct    string  `json:"structural_notes"`
	Rationale string  `json:"rationale"`
}

// Entry is one verdict put in front of a human.
type Entry struct {
	ID        string      `json:"id"`
	Candidate string      `json:"candidate"`
	Query     string      `json:"query"`
	DirA      string      `json:"dir_a,omitempty"`
	DirB      string      `json:"dir_b,omitempty"`
	Winner    string      `json:"agreed_winner"`
	Why       string      `json:"why_sampled"` // names_block | claim_wording | random
	Judges    []JudgeText `json:"judges"`
	Mentions  []Mention   `json:"mentions,omitempty"`
	Claims    []string    `json:"claim_words,omitempty"`
	// Filled in by the human reviewer. Discard=true means the verdict rests on a judge factual
	// error; nil means not yet reviewed.
	Discard *bool  `json:"discard"`
	Note    string `json:"note"`
}

// SamplePlan is the output of the sampler.
type SamplePlan struct {
	Seed         int64   `json:"seed"`
	Agreed       int     `json:"agreed_verdicts"`
	Named        int     `json:"named_verdicts"` // included unconditionally
	RandomTarget int     `json:"random_target"`  // ceil(rate x agreed)
	Rate         float64 `json:"random_rate"`
	Entries      []Entry `json:"entries"`
}

func judgeLabel(v Verdict, fallback string) string {
	if v.Judge != "" {
		return v.Judge
	}
	return fallback
}

func entryFor(ix *Index, it Item) Entry {
	e := Entry{ID: it.ID, Candidate: it.Candidate, Query: it.Pair.Name, Winner: it.Winner}
	if it.Pair.DirA != "." {
		e.DirA, e.DirB = it.Pair.DirA, it.Pair.DirB
	}
	for _, j := range []struct {
		tag string
		v   Verdict
	}{{"A", it.A}, {"B", it.B}} {
		e.Judges = append(e.Judges, JudgeText{Judge: judgeLabel(j.v, "judge "+j.tag), Winner: j.v.Winner, Conf: j.v.Conf, Ground: j.v.Ground, Struct: j.v.Struct, Rationale: j.v.Rationale})
		for _, f := range []struct{ name, text string }{{"Ground", j.v.Ground}, {"Struct", j.v.Struct}, {"Rationale", j.v.Rationale}} {
			for _, m := range ix.Find(f.text) {
				m.Judge, m.Field = j.tag, f.name
				e.Mentions = append(e.Mentions, m)
			}
			e.Claims = mergeStrings(e.Claims, ClaimWords(f.text))
		}
	}
	return e
}

func mergeStrings(a, b []string) []string {
	seen := map[string]bool{}
	for _, x := range a {
		seen[x] = true
	}
	for _, x := range b {
		if !seen[x] {
			a = append(a, x)
			seen[x] = true
		}
	}
	sort.Strings(a)
	return a
}

// Sample builds the human spot-check list from the AGREED items: every verdict that names a
// specific block/capture (indexed name, exact or fuzzy) or uses claim-of-existence wording, PLUS
// a seeded random draw of ceil(rate x agreed) among the remaining verdicts, so judge errors that
// do not name gear are also looked at. (Interpretation: the plan's "at least 20% ... plus every
// verdict that cites a block" is implemented additively, so the unnamed verdicts always get
// their own 20% check rather than being crowded out when most verdicts name gear.)
func Sample(ix *Index, items []Item, rate float64, seed int64) SamplePlan {
	plan := SamplePlan{Seed: seed, Rate: rate}
	var agreed []Item
	for _, it := range items {
		if it.Status == Agreed {
			agreed = append(agreed, it)
		}
	}
	plan.Agreed = len(agreed)
	plan.RandomTarget = int(math.Ceil(rate * float64(len(agreed))))
	var rest []Entry
	for _, it := range agreed {
		e := entryFor(ix, it)
		switch {
		case len(e.Mentions) > 0:
			e.Why = "names_block"
			plan.Entries = append(plan.Entries, e)
			plan.Named++
		case len(e.Claims) > 0:
			e.Why = "claim_wording"
			plan.Entries = append(plan.Entries, e)
			plan.Named++
		default:
			e.Why = "random"
			rest = append(rest, e)
		}
	}
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	n := plan.RandomTarget
	if n > len(rest) {
		n = len(rest)
	}
	plan.Entries = append(plan.Entries, rest[:n]...)
	sort.Slice(plan.Entries, func(i, j int) bool { return plan.Entries[i].ID < plan.Entries[j].ID })
	return plan
}

// Review summarises a reviewed plan.
type Review struct {
	Reviewed, Pending, Discarded int
}

// ApplyReview marks agreed items discarded per a reviewed plan and reports how many sampled
// entries are still unreviewed. A reviewed entry whose id matches no item is an error (a stale
// review file must not silently apply to a different run).
func ApplyReview(items []Item, plan SamplePlan) (Review, error) {
	byID := map[string]*Item{}
	for i := range items {
		byID[items[i].ID] = &items[i]
	}
	var rv Review
	for _, e := range plan.Entries {
		it, ok := byID[e.ID]
		if !ok {
			return rv, fmt.Errorf("review entry %q matches no agreed verdict: stale or foreign review file", e.ID)
		}
		if e.Discard == nil {
			rv.Pending++
			continue
		}
		rv.Reviewed++
		if *e.Discard && it.Status == Agreed {
			it.Status = Discarded
			rv.Discarded++
		}
	}
	return rv, nil
}
