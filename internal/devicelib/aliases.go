package devicelib

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

//go:embed capture_aliases.json
var aliasesJSON []byte

var printableRe = regexp.MustCompile("^[\\x20-\\x7e]{1,64}$")

// Alias maps a capture name the libraries use to the exact name the unit lists.
type Alias struct {
	Name     string `json:"name"`
	Source   string `json:"source"` // "factory" | "user"
	UnitName string `json:"unit_name"`
	Evidence string `json:"evidence"`
	Reviewed string `json:"reviewed"` // YYYY-MM-DD
}

// Aliases is the reviewed alias table. The zero value has no entries.
type Aliases struct {
	byKey map[[2]string]Alias
}

type aliasFile struct {
	Format  string  `json:"format"`
	Aliases []Alias `json:"aliases"`
}

var dateRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

// NewAliases validates and indexes entries: printable ASCII names, a known source, a
// non-empty evidence and review date, no self-alias, no duplicate (name, source).
func NewAliases(entries []Alias) (*Aliases, error) {
	a := &Aliases{byKey: map[[2]string]Alias{}}
	for i, e := range entries {
		switch {
		case !printableRe.MatchString(e.Name) || !printableRe.MatchString(e.UnitName):
			return nil, fmt.Errorf("alias %d: names must be printable ASCII, 1-64 characters", i)
		case strings.TrimSpace(e.Name) != e.Name || strings.TrimSpace(e.UnitName) != e.UnitName:
			return nil, fmt.Errorf("alias %d: names must not start or end with a space", i)
		case e.Source != "factory" && e.Source != "user":
			return nil, fmt.Errorf("alias %d: source must be factory or user", i)
		case e.Name == e.UnitName:
			return nil, fmt.Errorf("alias %d: name equals unit_name", i)
		case strings.TrimSpace(e.Evidence) == "" || !dateRe.MatchString(e.Reviewed):
			return nil, fmt.Errorf("alias %d: evidence and a reviewed date (YYYY-MM-DD) are required", i)
		}
		k := [2]string{e.Name, e.Source}
		if _, dup := a.byKey[k]; dup {
			return nil, fmt.Errorf("alias %d: duplicate (name, source)", i)
		}
		a.byKey[k] = e
	}
	return a, nil
}

// EmbeddedAliases parses the table compiled into the binary.
func EmbeddedAliases() (*Aliases, error) {
	var f aliasFile
	if err := json.Unmarshal(aliasesJSON, &f); err != nil {
		return nil, err
	}
	if f.Format != "qc2-capture-aliases/1" {
		return nil, errors.New("capture_aliases.json: unknown format")
	}
	return NewAliases(f.Aliases)
}

// Lookup returns the unit name reviewed for (name, source).
func (a *Aliases) Lookup(name, source string) (string, bool) {
	if a == nil {
		return "", false
	}
	e, ok := a.byKey[[2]string{name, source}]
	return e.UnitName, ok
}

// Len is the number of entries.
func (a *Aliases) Len() int {
	if a == nil {
		return 0
	}
	return len(a.byKey)
}
