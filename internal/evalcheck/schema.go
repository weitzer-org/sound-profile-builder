package evalcheck

import (
	"encoding/json"
	"fmt"
	"sort"

	"google.golang.org/genai"

	"github.com/weitzer-org/sound-builder/internal/agents"
)

// ValidateAgainstSchema checks raw (a model response, possibly wrapped in ```json fences)
// against the same typed *genai.Schema the live pipeline sends as ResponseSchema, and returns
// every violation found (empty slice = valid). It exists because "the API accepted
// responseSchema" (Phase 0.5a) is not the same as "the output conformed": Phase 1A reports the
// observed conformance rate per (agent, model).
//
// Checked: the top level parses as JSON; type (object/array/string/number/integer/boolean);
// required properties present (a JSON null counts as absent); enum membership; array items.
// Not checked: unknown extra properties are allowed (the OpenAPI subset does not forbid them),
// and Minimum/Maximum/Format are ignored (none of the agent schemas use them).
func ValidateAgainstSchema(raw string, schema *genai.Schema) []string {
	if schema == nil {
		return []string{"no schema to validate against"}
	}
	var v any
	if err := json.Unmarshal([]byte(agents.StripJSONFences(raw)), &v); err != nil {
		return []string{"not valid JSON: " + err.Error()}
	}
	var issues []string
	validateNode("$", v, schema, &issues)
	return issues
}

func validateNode(path string, v any, s *genai.Schema, issues *[]string) {
	if s == nil {
		return
	}
	add := func(format string, a ...any) { *issues = append(*issues, path+": "+fmt.Sprintf(format, a...)) }
	if v == nil {
		add("null where %s expected", s.Type)
		return
	}
	switch s.Type {
	case genai.TypeObject:
		m, ok := v.(map[string]any)
		if !ok {
			add("expected object, got %T", v)
			return
		}
		for _, req := range s.Required {
			if val, present := m[req]; !present || val == nil {
				add("missing required property %q", req)
			}
		}
		keys := make([]string, 0, len(s.Properties))
		for k := range s.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if val, present := m[k]; present && val != nil {
				validateNode(path+"."+k, val, s.Properties[k], issues)
			}
		}
	case genai.TypeArray:
		a, ok := v.([]any)
		if !ok {
			add("expected array, got %T", v)
			return
		}
		for i, el := range a {
			validateNode(fmt.Sprintf("%s[%d]", path, i), el, s.Items, issues)
		}
	case genai.TypeString:
		str, ok := v.(string)
		if !ok {
			add("expected string, got %T", v)
			return
		}
		if len(s.Enum) > 0 {
			found := false
			for _, e := range s.Enum {
				if e == str {
					found = true
					break
				}
			}
			if !found {
				add("value %q not in enum %v", str, s.Enum)
			}
		}
	case genai.TypeNumber:
		if _, ok := v.(float64); !ok {
			add("expected number, got %T", v)
		}
	case genai.TypeInteger:
		f, ok := v.(float64)
		if !ok || f != float64(int64(f)) {
			add("expected integer, got %v", v)
		}
	case genai.TypeBoolean:
		if _, ok := v.(bool); !ok {
			add("expected boolean, got %T", v)
		}
	}
}
