package mcpserver

import (
	"fmt"
	"unicode/utf8"
)

// Paging limits for get_run_prompts and get_agent_output. claude.ai truncates
// tool results at roughly 150k characters (DESIGN §5.1; not verified here). The
// SDK does not copy a handler-supplied text block into structuredContent
// (measured in TestSDKResultSize: a 120000-character page is one 120300-byte
// text block plus ~200 bytes of structured metadata), so a max page plus its
// framing stays well under that. The paged text goes in the text block only.
const (
	defaultPageChars = 40000
	maxPageChars     = 120000
)

// page is one slice of a longer text, measured in characters (runes), never in
// bytes, so a slice can never split a multi-byte character.
type page struct {
	Text       string
	Offset     int  // first character of Text within the whole
	Total      int  // characters in the whole text
	NextOffset *int // set when more text follows this slice
}

// normalizePageSize applies the default and the cap to a requested max_chars.
func normalizePageSize(maxChars int) int {
	switch {
	case maxChars <= 0:
		return defaultPageChars
	case maxChars > maxPageChars:
		return maxPageChars
	}
	return maxChars
}

// pageText returns characters [offset, offset+maxChars) of s. An offset beyond
// the end is an error naming the total, so the model can correct itself; an
// offset exactly at the end yields an empty final page.
func pageText(s string, offset, maxChars int) (page, error) {
	total := utf8.RuneCountInString(s)
	if offset < 0 {
		return page{}, fmt.Errorf("offset must be 0 or greater.")
	}
	if offset > total {
		return page{}, fmt.Errorf("offset %d is past the end: the text has %d characters. Use an offset between 0 and %d.", offset, total, total)
	}
	maxChars = normalizePageSize(maxChars)
	end := offset + maxChars
	if end > total {
		end = total
	}
	p := page{Offset: offset, Total: total}
	if len(s) == total { // pure ASCII: characters are bytes
		p.Text = s[offset:end]
	} else {
		p.Text = string([]rune(s)[offset:end])
	}
	if end < total {
		n := end
		p.NextOffset = &n
	}
	return p, nil
}
