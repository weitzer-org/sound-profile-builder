package api

import (
	"encoding/json"
	"testing"
)

func TestSanitizeJSONQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		valid    bool
	}{
		{
			name:     "Basic healthy JSON",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
			valid:    true,
		},
		{
			name:     "Literal quotes inside string value",
			input:    `{"builder_statement": "This is a "Dumble" signature amp tone"}`,
			expected: `{"builder_statement": "This is a \"Dumble\" signature amp tone"}`,
			valid:    true,
		},
		{
			name:     "Multiple literal quotes",
			input:    `{"statement": "Designed with a "Plexi" and a "Tubescreamer" pedal platform."}`,
			expected: `{"statement": "Designed with a \"Plexi\" and a \"Tubescreamer\" pedal platform."}`,
			valid:    true,
		},
		{
			name:     "Pre-escaped quotes remain unchanged",
			input:    `{"statement": "Designed with a \"Plexi\" tone."}`,
			expected: `{"statement": "Designed with a \"Plexi\" tone."}`,
			valid:    true,
		},
		{
			name:     "Nested quotes with structural whitespace and commas",
			input:    `{"builder_statement": "Features a "D-Style" boost, which is legendary", "final_html_payload": {"key": "value"}}`,
			expected: `{"builder_statement": "Features a \"D-Style\" boost, which is legendary", "final_html_payload": {"key": "value"}}`,
			valid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := sanitizeJSONQuotes(tt.input)
			if actual != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, actual)
			}

			if tt.valid {
				var dummy interface{}
				if err := json.Unmarshal([]byte(actual), &dummy); err != nil {
					t.Errorf("Sanitized string is not valid JSON: %v | Sanitized Content: %s", err, actual)
				}
			}
		})
	}
}
