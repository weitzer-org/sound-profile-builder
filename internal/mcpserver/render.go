package mcpserver

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/agents"
	"github.com/weitzer-org/sound-builder/internal/storage"
)

// estimateFootnote is the legend line emitted only when at least one parameter
// is Basis=="estimate". The leading backslash keeps it from parsing as a list item.
const estimateFootnote = `\* = estimated value`

// estimateMark follows an estimated value. It is escaped like the legend: a raw
// "*" after a value ending in punctuation ("15%*") can open emphasis that a later
// marker in the same cell closes, italicising the text and losing both markers.
const estimateMark = `\*`

var (
	htmlEscaper       = strings.NewReplacer("<", "&lt;", ">", "&gt;")
	orderedListMarker = regexp.MustCompile(`^\d+[.)]( |$)`)
)

// cell neutralises one LLM-authored string so it can sit inside a table cell, a
// heading, a list item or a paragraph without creating structure or HTML:
//   - all whitespace runs (including \r\n, \n, \r, \t and Unicode line
//     separators) collapse to a single space and the result is trimmed, so no
//     field can start a new Markdown block;
//   - backslashes and `|` are escaped so a value cannot end a table cell;
//   - `<` and `>` become entities so nothing can become HTML or an autolink;
//   - a leading character that would start a block (heading, list, quote,
//     fence, ordered-list marker, link definition) is backslash-escaped.
func cell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "|", `\|`)
	s = htmlEscaper.Replace(s)
	return escapeBlockStart(s)
}

func escapeBlockStart(s string) string {
	if s == "" {
		return s
	}
	if orderedListMarker.MatchString(s) {
		i := strings.IndexAny(s, ".)")
		return s[:i] + `\` + s[i:]
	}
	if strings.ContainsRune("#*+-=_~`[", rune(s[0])) {
		return `\` + s
	}
	return s
}

// RenderPresetMarkdown renders a preset as Markdown for a chat client. Every
// LLM-authored string passes through cell(). Output is deterministic: guitars
// are emitted in sorted-name order.
func RenderPresetMarkdown(p *storage.Preset, sp storage.StructuredPreset) string {
	var name, statement string
	if p != nil {
		name = cell(p.Name)
		statement = cell(p.BuilderStatement)
	}

	blocks := []string{strings.TrimSpace("## " + name)}
	if statement != "" {
		blocks = append(blocks, statement)
	}

	guitars := make([]string, 0, len(sp.Guitars))
	for g := range sp.Guitars {
		guitars = append(guitars, g)
	}
	sort.Strings(guitars)

	anyEstimate := false
	for _, g := range guitars {
		blocks = append(blocks, renderGuitar(g, sp.Guitars[g], &anyEstimate)...)
	}
	if anyEstimate {
		blocks = append(blocks, estimateFootnote)
	}
	return strings.Join(blocks, "\n\n") + "\n"
}

func renderGuitar(name string, effects []storage.EffectBlock, anyEstimate *bool) []string {
	var table strings.Builder
	table.WriteString("| # | Block | Model | Scene A (Rhythm) | Scene B (Lead) |\n|---|---|---|---|---|")

	var why, defaults []string
	for i, b := range effects {
		// Controls left at the device default are named once below the table instead of
		// listed in the scene columns (agents.SplitDefaults, shared with the web view).
		shown, atDefault := agents.SplitDefaults(b)
		settingsA := make([]string, 0, len(shown))
		settingsB := make([]string, 0, len(shown))
		differs := false
		for _, prm := range shown {
			a, bb, diff, est := formatParam(prm)
			if est {
				*anyEstimate = true
			}
			differs = differs || diff
			settingsA = append(settingsA, a)
			settingsB = append(settingsB, bb)
		}
		colA, colB := strings.Join(settingsA, "; "), strings.Join(settingsB, "; ")
		if !differs && colB != "" {
			colB = sameAsA
		}
		fmt.Fprintf(&table, "\n| %d | %s | %s | %s | %s |", i+1, cell(b.Type), cell(b.Model), colA, colB)

		if r := cell(b.Rationale); r != "" {
			why = append(why, "- "+cell(b.Model)+": "+r)
		}
		if len(atDefault) > 0 {
			names := make([]string, len(atDefault))
			for k, n := range atDefault {
				names[k] = cell(n)
			}
			defaults = append(defaults, "- "+cell(b.Model)+": "+strings.Join(names, ", "))
		}
	}

	out := []string{"### " + cell(name), table.String()}
	if len(defaults) > 0 {
		out = append(out, "**At device default:**\n\n"+strings.Join(defaults, "\n"))
	}
	if len(why) > 0 {
		out = append(out, "**Why:**\n\n"+strings.Join(why, "\n"))
	}
	return out
}

// sameAsA fills Scene B for a block whose parameters are identical in both
// scenes, so a long unchanged list is not printed twice.
const sameAsA = "same as A"

// formatParam renders one parameter for the Scene A and Scene B columns as
// "Name value[unit]" (B falls back to A when ValueB is empty), with a trailing
// "*" on both for an estimated value. differs reports whether B != A.
func formatParam(prm storage.BlockParameter) (a, b string, differs, est bool) {
	name := cell(prm.Name)
	unit := cell(prm.Unit)
	value := cell(prm.Value)
	valueB := cell(prm.ValueB)
	if valueB == "" {
		valueB = value
	}

	a = strings.TrimSpace(name + " " + value + unit)
	b = strings.TrimSpace(name + " " + valueB + unit)
	differs = valueB != value

	est = prm.Basis == "estimate"
	if est {
		a += estimateMark
		b += estimateMark
	}
	return a, b, differs, est
}
