package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatalf("GEMINI_API_KEY must be set")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		log.Fatalf("GenAI client failed: %v", err)
	}

	page, err := client.Models.List(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list models: %v", err)
	}
	for {
		for _, m := range page.Items {
			fmt.Printf("Model: %s\n", m.Name)
		}
		if page.NextPageToken == "" {
			break
		}
		page, err = page.Next(ctx)
		if err != nil {
			log.Fatalf("Failed to fetch next page: %v", err)
		}
	}
}
