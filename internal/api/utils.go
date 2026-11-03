package api

import "github.com/weitzer-org/sound-builder/internal/presetsvc"

// sanitizeJSONQuotes finds and escapes raw literal double quotes inside JSON string fields.
// The implementation lives in presetsvc (which now owns the generate/refine parsing).
func sanitizeJSONQuotes(raw string) string { return presetsvc.SanitizeJSONQuotes(raw) }
