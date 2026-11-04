// Package evalscore scores pipeline output against the owner-approved Librarian
// quality checklist (internal/evalfixtures/librarian_checklist.json). It is
// deterministic and judge-free: pure name matching, no LLM call.
//
// It scores name-level choices only -- which amp, drives, delays and reverbs were
// picked. It cannot see parameter values or scene logic.
package evalscore

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/evalfixtures"
)

// Rule is one checklist item.
type Rule struct {
	ID      string `json:"id"`
	Level   string `json:"level"` // hard | soft
	Kind    string `json:"kind"`  // require | forbid
	Scope   string `json:"scope"` // amp | non_amp | any
	Pattern string `json:"pattern"`
	Source  string `json:"source"`
	Why     string `json:"why"`
}

type queryEntry struct {
	Query string `json:"query"`
	Rules []Rule `json:"rules"`
}

type ambiguous struct {
	TreatAs string `json:"treat_as"`
}

type checklistFile struct {
	Patterns       map[string]string     `json:"patterns"`
	Queries        map[string]queryEntry `json:"queries"`
	AmbiguousNames map[string]ambiguous  `json:"ambiguous_names"`
}

// UserCapture is the minimal view of a user capture the scorer needs to classify a name.
type UserCapture struct{ Name, BlockType string }

// Scorer scores chosen block names against the checklist.
type Scorer struct {
	cl      checklistFile
	catalog map[string][]agents.CatalogEntry
	userBT  map[string]string
	rxCache map[string]*compiled
}

type compiled struct {
	rx        *regexp.Regexp
	archetype map[string]bool
	types     map[string]bool
}

// New builds a Scorer from the embedded checklist and the live catalog.
func New() (*Scorer, error) {
	var ucs []UserCapture
	for _, c := range agents.GetUserCaptures() {
		ucs = append(ucs, UserCapture{Name: c.Name, BlockType: c.BlockType})
	}
	return NewFrom(evalfixtures.LibrarianChecklistJSON(), agents.CorosCatalog(), ucs)
}

// NewFrom builds a Scorer from explicit inputs (used by tests).
func NewFrom(checklist []byte, catalog map[string][]agents.CatalogEntry, ucs []UserCapture) (*Scorer, error) {
	s := &Scorer{catalog: catalog, userBT: map[string]string{}, rxCache: map[string]*compiled{}}
	if err := json.Unmarshal(checklist, &s.cl); err != nil {
		return nil, fmt.Errorf("parsing checklist: %w", err)
	}
	for _, u := range ucs {
		s.userBT[u.Name] = u.BlockType
	}
	for _, q := range s.cl.Queries {
		for _, r := range q.Rules {
			if _, err := s.compile(r.Pattern); err != nil {
				return nil, fmt.Errorf("rule %s: %w", r.ID, err)
			}
		}
	}
	return s, nil
}

// Queries returns the checklist's query names, sorted.
func (s *Scorer) Queries() []string {
	out := make([]string, 0, len(s.cl.Queries))
	for k := range s.cl.Queries {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *Scorer) compile(spec string) (*compiled, error) {
	if c, ok := s.rxCache[spec]; ok {
		return c, nil
	}
	c := &compiled{archetype: map[string]bool{}, types: map[string]bool{}}
	var regs []string
	for _, key := range strings.Split(spec, "|") {
		body, ok := s.cl.Patterns[key]
		if !ok {
			body = key // a raw pattern fragment
		}
		for _, tok := range strings.Split(body, "|") {
			switch {
			case strings.HasPrefix(tok, "@archetype:"):
				c.archetype[strings.TrimPrefix(tok, "@archetype:")] = true
			case strings.HasPrefix(tok, "@type:"):
				c.types[strings.TrimPrefix(tok, "@type:")] = true
			case tok != "":
				regs = append(regs, tok)
			}
		}
	}
	if len(regs) > 0 {
		rx, err := regexp.Compile("(?i)" + strings.Join(regs, "|"))
		if err != nil {
			return nil, err
		}
		c.rx = rx
	}
	s.rxCache[spec] = c
	return c, nil
}

func (s *Scorer) isAmbiguous(n string) bool { _, ok := s.cl.AmbiguousNames[n]; return ok }

// IsAmp reports whether a chosen name is an amp (or a full rig containing one).
// Names listed under ambiguous_names are never amps.
func (s *Scorer) IsAmp(n string) bool {
	if s.isAmbiguous(n) {
		return false
	}
	for _, e := range s.catalog[n] {
		if e.Type == "amp" {
			return true
		}
	}
	bt := s.userBT[n]
	return bt == "amp" || bt == "full_rig"
}

func (s *Scorer) matches(n, spec string) bool {
	c, _ := s.compile(spec)
	if c.rx != nil && c.rx.MatchString(n) {
		return true
	}
	if s.isAmbiguous(n) {
		return false
	}
	for _, e := range s.catalog[n] {
		if (e.Type == "amp" && c.archetype[e.Archetype]) || c.types[e.Type] {
			return true
		}
	}
	return false
}

func inScope(isAmp bool, scope string) bool {
	switch scope {
	case "amp":
		return isAmp
	case "non_amp":
		return !isAmp
	}
	return true
}

// RuleResult is the outcome of one rule.
type RuleResult struct {
	Rule
	Pass bool `json:"pass"`
}

// Result is the score for one query's chosen names.
type Result struct {
	Query        string       `json:"query"`
	Rules        []RuleResult `json:"rules"`
	HardTotal    int          `json:"hard_total"`
	HardPass     int          `json:"hard_pass"`
	SoftTotal    int          `json:"soft_total"`
	SoftPass     int          `json:"soft_pass"`
	OneAmp       bool         `json:"g3_one_amp"`
	AmpCount     int          `json:"amp_count"`
	NotInCatalog int          `json:"g1_not_in_catalog"`
	RangeRverb   int          `json:"g2_range_rverb"`
	AllHardPass  bool         `json:"all_hard_pass"`
}

// Score scores the distinct chosen block names for one golden query. mandatory, if
// non-empty, is used for the single-amp check (G3); otherwise all names are used.
func (s *Scorer) Score(query string, names []string, mandatory []string) (Result, error) {
	qe, ok := s.cl.Queries[query]
	if !ok {
		return Result{}, fmt.Errorf("no checklist for query %q", query)
	}
	names = dedupe(names)
	res := Result{Query: query}
	for _, r := range qe.Rules {
		hit := false
		for _, n := range names {
			if inScope(s.IsAmp(n), r.Scope) && s.matches(n, r.Pattern) {
				hit = true
				break
			}
		}
		pass := hit
		if r.Kind == "forbid" {
			pass = !hit
		}
		res.Rules = append(res.Rules, RuleResult{Rule: r, Pass: pass})
		if r.Level == "hard" {
			res.HardTotal++
			if pass {
				res.HardPass++
			}
		} else {
			res.SoftTotal++
			if pass {
				res.SoftPass++
			}
		}
	}
	ampSrc := names
	if len(mandatory) > 0 {
		ampSrc = dedupe(mandatory)
	}
	for _, n := range ampSrc {
		if s.IsAmp(n) {
			res.AmpCount++
		}
	}
	res.OneAmp = res.AmpCount == 1
	for _, n := range names {
		if s.isAmbiguous(n) {
			res.RangeRverb++
		}
		if _, inCat := s.catalog[n]; !inCat {
			if _, inUser := s.userBT[n]; !inUser {
				res.NotInCatalog++
			}
		}
	}
	res.AllHardPass = res.HardPass == res.HardTotal && res.OneAmp
	return res, nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range in {
		n = strings.TrimSpace(n)
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// LibrarianNames extracts the chosen block names from a Librarian JSON output
// (v1 verified_blocks, v2 mandatory/suggested_blocks). mandatory is returned
// separately for the single-amp check. A parse failure returns an error.
func LibrarianNames(output string) (all []string, mandatory []string, err error) {
	var v struct {
		Verified  []string `json:"verified_blocks"`
		Mandatory []string `json:"mandatory_blocks"`
		Suggested []string `json:"suggested_blocks"`
	}
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		return nil, nil, err
	}
	mandatory = append(append(mandatory, v.Mandatory...), v.Verified...)
	all = append(append(append(all, v.Mandatory...), v.Suggested...), v.Verified...)
	return all, mandatory, nil
}

var modelRx = regexp.MustCompile(`"model":"([^"]{1,80})"`)

// PresetModels extracts the distinct block "model" names from a rendered preset page/JSON.
func PresetModels(preset string) []string {
	var out []string
	for _, m := range modelRx.FindAllStringSubmatch(preset, -1) {
		out = append(out, m[1])
	}
	return dedupe(out)
}
