package mcpserver

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// estimateFootnote is the legend line emitted only when at least one parameter
// is Basis=="estimate". The leading backslash keeps it from parsing as a list item.
const estimateFootnote = `\* = estimated value`

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
	table.WriteString("| # | Block | Model | Settings (A / B) |\n|---|---|---|---|")

	var why []string
	for i, b := range effects {
		settings := make([]string, 0, len(b.Parameters))
		for _, prm := range b.Parameters {
			text, est := formatParam(prm)
			if est {
				*anyEstimate = true
			}
			settings = append(settings, text)
		}
		fmt.Fprintf(&table, "\n| %d | %s | %s | %s |", i+1, cell(b.Type), cell(b.Model), strings.Join(settings, "; "))

		if r := cell(b.Rationale); r != "" {
			why = append(why, "- "+cell(b.Model)+": "+r)
		}
	}

	out := []string{"### " + cell(name), table.String()}
	if len(why) > 0 {
		out = append(out, "**Why:**\n\n"+strings.Join(why, "\n"))
	}
	return out
}

// formatParam renders "Name value[unit]", or "Name A→B[unit]" when ValueB is set
// and differs from Value, with a trailing "*" for estimated values.
func formatParam(prm storage.BlockParameter) (string, bool) {
	name := cell(prm.Name)
	unit := cell(prm.Unit)
	value := cell(prm.Value)
	valueB := cell(prm.ValueB)

	text := strings.TrimSpace(name + " " + value + unit)
	if valueB != "" && valueB != value {
		text = strings.TrimSpace(name + " " + value + unit + "→" + valueB + unit)
	}

	est := prm.Basis == "estimate"
	if est {
		text += "*"
	}
	return text, est
}
