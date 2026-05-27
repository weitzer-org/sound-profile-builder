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

		// Handle escaped characters (e.g., \", \\, \n, \t)
		if r == '\\' {
			if i+1 < n {
				sb.WriteRune('\\')
				sb.WriteRune(runes[i+1])
				i++
			} else {
				sb.WriteRune('\\')
			}
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
				if nextChar == '}' || nextChar == ']' || nextChar == ':' {
					isStructural = true
				} else if nextChar == ',' {
					// A structural comma is followed by a new key or value.
					// If the next non-whitespace character is not a valid JSON element start,
					// it is likely a literal comma inside the string.
					afterComma := nextIdx + 1
					for afterComma < n && isWhitespace(runes[afterComma]) {
						afterComma++
					}
					if afterComma < n {
						c := runes[afterComma]
						if c == '"' || c == '{' || c == '[' || c == '}' || c == ']' || (c >= '0' && c <= '9') || c == '-' || c == 't' || c == 'f' || c == 'n' {
							isStructural = true
						}
					}
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
