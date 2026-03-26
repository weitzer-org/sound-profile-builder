package main

import (
	"log"
	
	"github.com/weitzer-org/sound-builder/internal/api"
)

func main() {
	log.Println("Starting QC-2 Multi-Agent Modeler Backend...")

	// Initialize Server
	server := api.NewServer()

	// Start server on port 8080
	if err := server.Start(":8080"); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
