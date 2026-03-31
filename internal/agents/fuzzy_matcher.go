package agents

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
)

var validNativeBlocks = make(map[string]bool)
var parseBlocksOnce sync.Once

// GetValidNativeBlocks parses the embedded JSON natively, returning a map of [blockName]isCapture.
func GetValidNativeBlocks() map[string]bool {
	parseBlocksOnce.Do(func() {
		var corosData map[string]map[string]interface{}
		if err := json.Unmarshal(embeddedCorosMap, &corosData); err == nil {
			for _, props := range corosData {
				if equiv, ok := props["coros_equivalent"].(string); ok && equiv != "" {
					isCap, _ := props["is_capture"].(bool)
					validNativeBlocks[equiv] = isCap
				}
			}
		}
	})
	return validNativeBlocks
}

// ApplyFuzzyCorrection iterates over HTML table rows and corrects block names.
func ApplyFuzzyCorrection(jsonStr string, validBlocks map[string]bool) string {
	re := regexp.MustCompile(`(?i)(<td[^>]*>)([A-Za-z0-9\s/]+:\s*)([^<]+)`)
	corrected := re.ReplaceAllStringFunc(jsonStr, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) == 4 {
			prefix := sub[1] + sub[2]
			name := sub[3]
            
			// Prevent catching standard 'Setting: Value' cases
			key := strings.ToLower(strings.TrimSpace(sub[2]))
			if strings.Contains(key, "setting:") || strings.Contains(key, "mix:") || strings.Contains(key, "gain:") {
				return match
			}

			snapped := SnapToClosestBlock(name, validBlocks)
			
			// If the snapped block natively belongs to the 'Capture' category, inject the suffix for the UI.
			if isCap := validBlocks[snapped]; isCap {
				snapped += " (Capture)"
			}
			
			return prefix + snapped
		}
		return match
	})
	return corrected
}

// IgnoreList contains structural block names that shouldn't be snapped to amplifiers/effects.
var IgnoreList = map[string]bool{
	"Noise Gate":       true,
	"Adaptive Gate":    true,
	"Global Gate":      true,
	"Global Input":     true,
	"Input / Impedance": true,
	"Input: Global Impedance": true,
	"Gate: Noise Gate": true,
	"Lane 1 Output":    true,
	"Lane Output":      true,
	"Input":            true,
	"Gate":             true,
}

// LevenshteinDistance calculates the minimum string edits to go from s to t.
func LevenshteinDistance(s, t string) int {
	sRunes := []rune(strings.ToLower(s))
	tRunes := []rune(strings.ToLower(t))

	m := len(sRunes)
	n := len(tRunes)

	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}

	d := make([][]int, m+1)
	for i := range d {
		d[i] = make([]int, n+1)
		d[i][0] = i
	}
	for j := 0; j <= n; j++ {
		d[0][j] = j
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			cost := 1
			if sRunes[i-1] == tRunes[j-1] {
				cost = 0
			}
			d[i][j] = min(
				d[i-1][j]+1,      // deletion
				d[i][j-1]+1,      // insertion
				d[i-1][j-1]+cost, // substitution
			)
		}
	}
	return d[m][n]
}

func min(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

var categorizedAmpsCache string
var parseAmpsOnce sync.Once

// GetCategorizedAmplifiers reads the embedded JSON and creates a formatted Markdown menu
// grouping all available distinct amplifier names by their tonal archetype for LLM injection.
func GetCategorizedAmplifiers() string {
	parseAmpsOnce.Do(func() {
		var corosData map[string]map[string]interface{}
		buckets := make(map[string]map[string]bool)

		if err := json.Unmarshal(embeddedCorosMap, &corosData); err == nil {
			for _, props := range corosData {
				if t, ok := props["type"].(string); ok && t == "amp" {
					if equiv, ok := props["coros_equivalent"].(string); ok && equiv != "" {
						arch, _ := props["tonal_archetype"].(string)
						if arch == "" {
							arch = "Other / Unique"
						}
						
						if buckets[arch] == nil {
							buckets[arch] = make(map[string]bool)
						}
						buckets[arch][equiv] = true
					}
				}
			}
		}

		var sb strings.Builder
		sb.WriteString("=== AVAILABLE AMPLIFIER ARCHETYPES ===\n")
		for archetype, amps := range buckets {
			sb.WriteString(fmt.Sprintf("\n%s:\n", strings.ToUpper(archetype)))
			for amp := range amps {
				sb.WriteString(fmt.Sprintf("- %s\n", amp))
			}
		}
		categorizedAmpsCache = sb.String()
	})
	
	return categorizedAmpsCache
}

// SnapToClosestBlock checks if the input is a valid block, else returns the closest equivalent.
func SnapToClosestBlock(input string, validBlocks map[string]bool) string {
	inputStr := strings.TrimSpace(input)
	
	// Skip structural UI elements and obviously bad inputs like parameters (-3.0dB)
	if IgnoreList[inputStr] || len(inputStr) < 3 || strings.Contains(inputStr, "dB") || strings.Contains(inputStr, "%") || strings.Contains(inputStr, "ms") || strings.Contains(inputStr, "Hz") || inputStr == "Bypassed" || inputStr == "Active" || inputStr == "Engaged" {
		return inputStr
	}

	bestDistance := math.MaxInt32
	bestMatch := inputStr

	for v := range validBlocks {
		if strings.EqualFold(inputStr, v) {
			return v // Perfect case-insensitive match
		}
		
		dist := LevenshteinDistance(inputStr, v)
		if dist < bestDistance {
			bestDistance = dist
			bestMatch = v
		}
	}

	// "Brit Plexi 100" to "Brit Plexi 100 Patch" is 6 edits.
	// "Plate Reverb" to "Studio Plate 70" is 11 edits.
	// "Double RVB" to "US Twin Vibrato" is 12 edits.
	// We'll use 15 to ensure we catch those larger alias hallucinations, 
	// but the UI won't warp anything shorter than 3 chars anyway.
	if bestDistance <= 15 {
		return bestMatch
	}

	return inputStr
}
