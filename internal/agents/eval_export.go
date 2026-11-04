package agents

import (
	"encoding/json"
	"strings"
)

// PriceFor exposes geminiPriceTable (USD per 1,000,000 tokens) to eval tooling without
// duplicating it. ok is false when the model has no official entry -- callers must then report
// the cost as UNKNOWN, never as $0 (see UsageRecord.CostUnknown). A leading "models/" is ignored.
func PriceFor(model string) (inputPerM, outputPerM float64, ok bool) {
	p, ok := geminiPriceTable[strings.TrimPrefix(strings.TrimSpace(model), "models/")]
	return p.input, p.output, ok
}

// CorosMapRecords returns the embedded coros_map.json as real-world gear name -> record
// (coros_equivalent, type, is_capture, tonal_archetype, required_plugin, ...), for eval tooling
// that must show a human the ground-truth record behind a block name. A fresh map each call.
func CorosMapRecords() map[string]map[string]any {
	var m map[string]map[string]any
	if err := json.Unmarshal(embeddedCorosMap, &m); err != nil {
		return map[string]map[string]any{}
	}
	return m
}
