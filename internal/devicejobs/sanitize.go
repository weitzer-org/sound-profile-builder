package devicejobs

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// SanitizeText makes one bridge- or device-sourced string safe to show a model or a user:
// control and Unicode format characters (bidi overrides, zero-width characters) are
// dropped, backticks become apostrophes, whitespace runs collapse to one space, and the
// result is cut to n runes (an ellipsis marks a cut). mcpserver's sanitizeDeviceText is
// this function; the read-back import (internal/deviceexport) takes it as Options.Sanitize,
// so display and storage share one definition. Markdown output additionally renders device
// text in a code span (mcpserver's dcell).
func SanitizeText(s string, n int) string {
	// Bound the work first: the result is at most n runes, so a long input is
	// cut (by bytes; a split rune decodes to U+FFFD, which is dropped below)
	// before it is copied. 16 bytes per kept rune leaves room for stripped
	// characters and collapsed whitespace in any realistic name.
	if limit := 16 * n; len(s) > limit {
		s = s[:limit]
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && !unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) || r == unicode.ReplacementChar {
			return -1
		}
		if r == '`' {
			return '\'' // no device text may open or close a code span or fence
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n])) + "…"
}
