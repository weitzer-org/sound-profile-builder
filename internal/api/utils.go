package api

import (
	"strings"
)

// sanitizeJSONQuotes finds and escapes raw literal double quotes inside JSON string fields
func sanitizeJSONQuotes(raw string) string {
	var sb strings.Builder
	inString := false
	runes := []rune(raw)
	n := len(runes)

	for i := 0; i < n; i++ {
		r := runes[i]

		if !inString {
			if r == '"' {
				inString = true
			}
			sb.WriteRune(r)
			continue
		}

		// Handle pre-escaped quotes
		if r == '\\' && i+1 < n && runes[i+1] == '"' {
			sb.WriteRune('\\')
			sb.WriteRune('"')
			i++
			continue
		}

		if r == '"' {
			// Look ahead for the next non-whitespace character
			nextIdx := i + 1
			for nextIdx < n && isWhitespace(runes[nextIdx]) {
				nextIdx++
			}

			isStructural := false
			if nextIdx < n {
				nextChar := runes[nextIdx]
				if nextChar == ',' || nextChar == '}' || nextChar == ']' || nextChar == ':' {
					isStructural = true
				}
			} else {
				// End of input is structural
				isStructural = true
			}

			if isStructural {
				inString = false
				sb.WriteRune('"')
			} else {
				// Escape the literal quote
				sb.WriteString("\\\"")
			}
			continue
		}

		sb.WriteRune(r)
	}

	return sb.String()
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
