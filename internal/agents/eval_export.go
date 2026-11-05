package agents

import (
	"encoding/json"
	"strings"
	"time"
)

// PriceFor exposes the price in effect now from geminiPriceSchedule (USD per 1,000,000 tokens)
// to eval tooling without duplicating it. ok is false when the model has no official entry --
// callers must then report the cost as UNKNOWN, never as $0 (see UsageRecord.CostUnknown). A
// leading "models/" is ignored. Prices are dated (introductory vs. standard); use PriceForAt to
// price a past run at the rate in effect when it ran.
func PriceFor(model string) (inputPerM, outputPerM float64, ok bool) {
	return PriceForAt(model, nowFunc())
}

// PriceForAt is PriceFor at time t.
func PriceForAt(model string, t time.Time) (inputPerM, outputPerM float64, ok bool) {
	p, ok := priceAt(strings.TrimPrefix(strings.TrimSpace(model), "models/"), t)
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
