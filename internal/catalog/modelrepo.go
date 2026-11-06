package catalog

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Raw ModelRepo.xml shapes. Only the attributes this package keeps are declared.
type xmlRepo struct {
	Categories []xmlCategory `xml:"Category"`
}

type xmlCategory struct {
	Name   string     `xml:"name,attr"`
	Models []xmlModel `xml:"Model"`
}

type xmlModel struct {
	ID       string     `xml:"id,attr"`
	Name     string     `xml:"name,attr"`
	TM       string     `xml:"tm,attr"`
	Hidden   string     `xml:"hidden,attr"`
	Internal string     `xml:"internal,attr"`
	PluginID string     `xml:"plugin_id,attr"`
	Clones   string     `xml:"clones,attr"`
	Params   []xmlParam `xml:"Parameter"`
}

type xmlParam struct {
	Name       string `xml:"name,attr"`
	Type       string `xml:"type,attr"`
	Min        string `xml:"min,attr"`
	Max        string `xml:"max,attr"`
	Default    string `xml:"defaultValue,attr"`
	Units      string `xml:"units,attr"`
	Steps      string `xml:"steps,attr"`
	StepNames  string `xml:"stepNames,attr"`
	MinString  string `xml:"min_string,attr"`
	MaxString  string `xml:"max_string,attr"`
	DisplayPos string `xml:"displayPos,attr"`
	Hidden     string `xml:"hidden,attr"`
	Replaces   string `xml:"replaces,attr"`
	Skew       string `xml:"skew,attr"`
}

// indexedParam is a parameter at its device index.
type indexedParam struct {
	index int
	xp    xmlParam
}

// effectiveParams resolves a model's full parameter list.
//
// A model with clones="<id>" inherits the base model's parameters (bases are often hidden
// templates, e.g. "Default Cabsim"). Each of its own parameters with a numeric
// replaces="N" overrides the base's parameter N; one without replaces is appended after
// the base's parameters (in the real catalog these always come last in the clone's list,
// e.g. LOW CUT/HIGH CUT at 13+ on shimmer clones of a 13-parameter base). A dual-mic cab, for example, lists only its two "ir selector"
// parameters (replaces="1" and "9") and inherits LEVEL/PAN/DISTANCE/POSITION from the
// base. This reading is inferred from the XML's structure (every replaces index falls
// inside the base's parameter list) -- it has not been checked against a device.
// On a model without clones, replaces holds an old parameter name (a rename note) and
// is ignored.
func effectiveParams(m xmlModel, byID map[int]xmlModel, dupIDs map[int]bool, depth int) ([]indexedParam, error) {
	if strings.TrimSpace(m.Clones) == "" {
		out := make([]indexedParam, len(m.Params))
		for i, xp := range m.Params {
			out[i] = indexedParam{i, xp}
		}
		return out, nil
	}
	if depth > 8 {
		return nil, fmt.Errorf("model %q: clones chain too deep", m.Name)
	}
	baseID, err := strconv.Atoi(strings.TrimSpace(m.Clones))
	if err != nil {
		return nil, fmt.Errorf("model %q: bad clones %q", m.Name, m.Clones)
	}
	if dupIDs[baseID] {
		return nil, fmt.Errorf("model %q clones id %d, which is defined more than once", m.Name, baseID)
	}
	base, ok := byID[baseID]
	if !ok {
		return nil, fmt.Errorf("model %q clones unknown id %q", m.Name, m.Clones)
	}
	out, err := effectiveParams(base, byID, dupIDs, depth+1)
	if err != nil {
		return nil, err
	}
	out = append([]indexedParam(nil), out...)
	baseLen := len(out)
	for _, xp := range m.Params {
		replaces := strings.TrimSpace(xp.Replaces)
		if replaces == "" {
			out = append(out, indexedParam{len(out), xp})
			continue
		}
		n, err := strconv.Atoi(replaces)
		if err != nil || n < 0 || n >= baseLen {
			return nil, fmt.Errorf("model %q param %q: bad replaces %q", m.Name, xp.Name, xp.Replaces)
		}
		out[n] = indexedParam{n, xp}
	}
	// Already in index order: out[i].index == i holds for the base list, in-place
	// replacements keep it, and additions are appended at index len(out).
	return out, nil
}

// skippedCategory reports categories that hold firmware plumbing rather than blocks a
// user can place: internal routing, lane/tempo/IO controls and deprecated utilities.
func skippedCategory(name string) bool {
	return strings.Contains(name, "Internal") || name == "Utility_Deprecated"
}

// ParseModelRepo converts the device's ModelRepo.xml into catalog models, sorted by ID.
//
// Cloned models get their inherited parameters resolved (see effectiveParams).
// It drops models marked hidden="true" or internal="true", categories that are firmware
// plumbing (see skippedCategory), and parameters marked hidden="true". Parameter Index
// still counts hidden parameters, because the device addresses parameters by position.
// hidden="atma" (hidden on the Quad Cortex mini only) is kept.
func ParseModelRepo(r io.Reader) ([]Model, error) {
	var repo xmlRepo
	if err := xml.NewDecoder(r).Decode(&repo); err != nil {
		return nil, fmt.Errorf("parse ModelRepo.xml: %w", err)
	}
	// Every model, hidden templates included, keyed by numeric id: clones="<id>" may name
	// any of them. An id defined twice is only an error if something clones it (see
	// effectiveParams); duplicates among kept models are rejected below regardless.
	byID := map[int]xmlModel{}
	dupIDs := map[int]bool{}
	for _, cat := range repo.Categories {
		for _, xm := range cat.Models {
			id, err := strconv.Atoi(strings.TrimSpace(xm.ID))
			if err != nil {
				continue // reported below if the model is kept
			}
			if _, dup := byID[id]; dup {
				dupIDs[id] = true
			}
			byID[id] = xm
		}
	}
	var out []Model
	seen := map[int]string{}
	for _, cat := range repo.Categories {
		if skippedCategory(cat.Name) {
			continue
		}
		for _, xm := range cat.Models {
			if xm.Hidden == "true" || xm.Internal == "true" {
				continue
			}
			id, err := strconv.Atoi(strings.TrimSpace(xm.ID))
			if err != nil {
				return nil, fmt.Errorf("model %q: bad id %q", xm.Name, xm.ID)
			}
			if prev, dup := seen[id]; dup {
				return nil, fmt.Errorf("duplicate model id %d (%q and %q)", id, prev, xm.Name)
			}
			seen[id] = xm.Name
			m := Model{
				ID:       id,
				Name:     strings.TrimSpace(xm.Name),
				Category: cat.Name,
				BasedOn:  strings.TrimSpace(xm.TM),
				PluginID: strings.TrimSpace(xm.PluginID),
				Params:   []Param{},
			}
			if c := strings.TrimSpace(xm.Clones); c != "" {
				base, err := strconv.Atoi(c)
				if err != nil {
					return nil, fmt.Errorf("model %q: bad clones %q", xm.Name, xm.Clones)
				}
				m.Clones = base
			}
			params, err := effectiveParams(xm, byID, dupIDs, 0)
			if err != nil {
				return nil, err
			}
			// ParamCount counts every parameter, hidden ones included: the device's
			// params array holds all of them (plus one trailing slot, see Model.ParamCount).
			m.ParamCount = len(params)
			for _, ip := range params {
				if ip.xp.Hidden == "true" {
					continue
				}
				p, err := convertParam(ip.index, ip.xp)
				if err != nil {
					return nil, fmt.Errorf("model %q (%d): %w", m.Name, m.ID, err)
				}
				m.Params = append(m.Params, p)
			}
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) == 0 {
		// e.g. an error document or the wrong file: don't let the importer overwrite the
		// committed catalog with an empty one.
		return nil, fmt.Errorf("parse ModelRepo.xml: no models found")
	}
	return out, nil
}

func convertParam(index int, xp xmlParam) (Param, error) {
	p := Param{
		Index:    index,
		Name:     strings.TrimSpace(xp.Name),
		Type:     xp.Type,
		Units:    xp.Units,
		MinLabel: xp.MinString,
		MaxLabel: xp.MaxString,
	}
	skew, err := parseSkew(xp.Skew)
	if err != nil {
		return Param{}, fmt.Errorf("param %q (index %d): %w", p.Name, index, err)
	}
	p.Skew = skew
	if s := strings.TrimSpace(xp.Skew); s == "LIN_SKEW" || s == "LOG_SKEW" {
		p.SkewSymbol = s
	}
	p.Min, p.MinSymbol = numberOrSymbol(xp.Min)
	p.Max, p.MaxSymbol = numberOrSymbol(xp.Max)
	if v, err := strconv.ParseFloat(strings.TrimSpace(xp.Default), 64); err == nil {
		p.Default = &v
	}
	if n, err := strconv.Atoi(strings.TrimSpace(xp.Steps)); err == nil {
		p.Steps = n
	}
	if s := strings.TrimSpace(xp.StepNames); s != "" {
		p.StepNames = strings.Split(s, ",")
	}
	if n, err := strconv.Atoi(strings.TrimSpace(xp.DisplayPos)); err == nil {
		p.DisplayPos = &n
	}
	return p, nil
}

// Skew values for the symbolic names ModelRepo.xml uses. LIN_SKEW is linear by definition.
// LOG_SKEW = 0.3 is pyquadcortex's measurement (MIT, data only), not confirmed on our unit
// (plan 7.1, G4: "Not covered: LOG_SKEW parameters").
const (
	linSkew = 1.0
	logSkew = 0.3
)

// parseSkew reads a skew attribute: a finite number > 0 as is, LIN_SKEW as 1.0 and LOG_SKEW
// as 0.3. An absent or empty attribute is nil (read as linear by Param.ToWire). Any other
// value, including a number that is zero, negative, NaN or infinite, is an error, so a
// CorOS update that introduces a new skew name fails the import instead of being guessed.
func parseSkew(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	switch s {
	case "":
		return nil, nil
	case "LIN_SKEW":
		v := linSkew
		return &v, nil
	case "LOG_SKEW":
		v := logSkew
		return &v, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("unknown skew %q (want a number, LIN_SKEW or LOG_SKEW)", s)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return nil, fmt.Errorf("skew %q must be a finite number > 0", s)
	}
	return &v, nil
}

// numberOrSymbol returns a numeric bound, or the symbolic name the XML used instead
// (e.g. "MAX_CABSIM_DB"), or neither when the attribute is absent.
func numberOrSymbol(s string) (*float64, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ""
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return &v, ""
	}
	return nil, s
}

// New builds a Catalog from parsed models.
func New(models []Model, src Source, readme string) *Catalog {
	src.ModelCount = len(models)
	c := &Catalog{Readme: readme, Source: src, Models: models}
	c.index()
	return c
}
