package agents

import (
	"encoding/json"
	"sync"
)

// CatalogEntry describes one coros_map.json entry that resolves to a given
// coros_equivalent name. A name can have several (e.g. "Range Rverb"), so callers
// get all of them.
type CatalogEntry struct {
	Type      string
	Archetype string
}

var (
	catalogOnce sync.Once
	catalogByEq map[string][]CatalogEntry
)

// CorosCatalog returns coros_equivalent -> every entry that resolves to it. Read-only
// view for evals/scoring; the returned map must not be modified.
func CorosCatalog() map[string][]CatalogEntry {
	catalogOnce.Do(func() {
		catalogByEq = map[string][]CatalogEntry{}
		var raw map[string]map[string]interface{}
		if err := json.Unmarshal(embeddedCorosMap, &raw); err != nil {
			return
		}
		for _, props := range raw {
			eq, _ := props["coros_equivalent"].(string)
			if eq == "" {
				continue
			}
			t, _ := props["type"].(string)
			a, _ := props["tonal_archetype"].(string)
			catalogByEq[eq] = append(catalogByEq[eq], CatalogEntry{Type: t, Archetype: a})
		}
	})
	return catalogByEq
}

// EstimateCostUSD exposes the usage cost model (geminiPriceTable) to eval tools so they
// can enforce a spend ceiling. inputTokens includes cachedTokens. Returns 0 for an
// unpriced model, so callers must treat 0 as "unknown", not "free".
func EstimateCostUSD(model string, inputTokens, cachedTokens, outputTokens, thinkingTokens int32) float64 {
	return computeCostUSD(model, inputTokens, cachedTokens, outputTokens, thinkingTokens)
}
