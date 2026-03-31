package main

import (
	"fmt"
	"github.com/weitzer-org/sound-builder/internal/agents"
)

func main() {
    blocks := agents.GetValidNativeBlocks()
    fmt.Printf("Total valid blocks: %d\n", len(blocks))
    
    html := `<td style='padding: 12px; border-bottom: 1px solid #3f3f46;'>Type: Double RVB<br/><em style='font-size: 0.85em; color: #94a3b8;'>`
    fmt.Printf("Original: %s\n", html)
    
    corrected := agents.ApplyFuzzyCorrection(html, blocks)
    fmt.Printf("Corrected: %s\n", corrected)

    fmt.Printf("Test direct snap: Brit Plexi 100 -> %s\n", agents.SnapToClosestBlock("Brit Plexi 100", blocks))
    fmt.Printf("Test direct snap: Double RVB -> %s\n", agents.SnapToClosestBlock("Double RVB", blocks))
}
