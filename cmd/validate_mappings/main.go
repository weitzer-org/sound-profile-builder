package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type Mapping struct {
	CorosEquivalent string  `json:"coros_equivalent"`
	Type            string  `json:"type"`
	ConfidenceScore float64 `json:"confidence_score"`
	IsCapture       bool    `json:"is_capture"`
}

func main() {
	filePath := "internal/agents/coros_map.json"
	if len(os.Args) > 1 {
		filePath = os.Args[1]
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", filePath, err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", filePath, err)
	}

	var mappings map[string]Mapping
	if err := json.Unmarshal(bytes, &mappings); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	// Load Ontology
	ontologyPath := "cmd/validate_mappings/coros_ontology.json"
	ontologyFile, err := os.Open(ontologyPath)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", ontologyPath, err)
	}
	defer ontologyFile.Close()

	ontologyBytes, err := io.ReadAll(ontologyFile)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", ontologyPath, err)
	}

	var ontology map[string]string
	if err := json.Unmarshal(ontologyBytes, &ontology); err != nil {
		log.Fatalf("Failed to parse %s JSON: %v", ontologyPath, err)
	}

	errorsFound := 0

	for source, m := range mappings {
		srcLower := strings.ToLower(source)
		typeLower := strings.ToLower(m.Type)
		equivLower := strings.ToLower(m.CorosEquivalent)

		// 0. Semantic Ontology Validation
		if expectedType, ok := ontology[m.CorosEquivalent]; ok {
			if expectedType != m.Type {
				fmt.Printf("[ERR] Semantic Mismatch: %s -> %s (Type in map is %s, but ontology requires it to be %s)\n", source, m.CorosEquivalent, m.Type, expectedType)
				errorsFound++
			}
		}

		// Rule 1: Cab should not be mapped to Wah or Overdrive
		if typeLower == "cab" || strings.Contains(srcLower, "cab") || strings.Contains(srcLower, "cabinet") {
			if strings.Contains(equivLower, "wah") || strings.Contains(equivLower, "overdrive") || strings.Contains(equivLower, "drive") {
				fmt.Printf("[ERR] Cab mapped to non-cab: %s -> %s (Type: %s)\n", source, m.CorosEquivalent, m.Type)
				errorsFound++
			}
		}

		// Rule 2: Delay should not be mapped to Overdrive or Amp
		if typeLower == "delay" || strings.Contains(srcLower, "delay") || strings.Contains(srcLower, "echo") {
			if strings.Contains(equivLower, "overdrive") || strings.Contains(equivLower, "drive") || strings.Contains(equivLower, "ts808") {
				fmt.Printf("[ERR] Delay mapped to non-delay: %s -> %s (Type: %s)\n", source, m.CorosEquivalent, m.Type)
				errorsFound++
			}
		}

		// Rule 3: Reverb should not be mapped to Overdrive
		if typeLower == "reverb" || strings.Contains(srcLower, "reverb") {
			if strings.Contains(equivLower, "overdrive") || strings.Contains(equivLower, "drive") {
				fmt.Printf("[ERR] Reverb mapped to non-reverb: %s -> %s (Type: %s)\n", source, m.CorosEquivalent, m.Type)
				errorsFound++
			}
		}

		// Rule 4: Type "amp" should not be mapped to Wah or Modulation
		if typeLower == "amp" || strings.Contains(srcLower, "amp") {
			if strings.Contains(equivLower, "wah") || strings.Contains(equivLower, "chorus") || strings.Contains(equivLower, "phaser") {
				if m.Type != "fx" { // Allow general FX type if it's a multi-fx or something
					fmt.Printf("[ERR] Amp mapped to non-amp: %s -> %s (Type: %s)\n", source, m.CorosEquivalent, m.Type)
					errorsFound++
				}
			}
		}

		// Rule 5: Specific known bad targets for non-delay/non-cab
		if m.CorosEquivalent == "Love DLX" && typeLower != "overdrive" && typeLower != "drive" && typeLower != "fx" {
			fmt.Printf("[ERR] %s mapped to Love DLX (Overdrive) but type is %s\n", source, m.Type)
			errorsFound++
		}
		if m.CorosEquivalent == "BDDI" && typeLower != "bass" && typeLower != "preamp" && typeLower != "fx" {
			fmt.Printf("[ERR] %s mapped to BDDI (Bass Preamp) but type is %s\n", source, m.Type)
			errorsFound++
		}
	}

	fmt.Printf("\nValidation complete. Found %d errors.\n", errorsFound)
	if errorsFound > 0 {
		os.Exit(1)
	}
}
