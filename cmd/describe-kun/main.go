package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kznrluk/describe-kun/internal/app"
	"github.com/kznrluk/describe-kun/internal/fetcher"
	"github.com/kznrluk/describe-kun/internal/llm"
)

func main() {
	// Define command-line flags
	url := flag.String("url", "", "URL of the web page to process (required)")
	prompt := flag.String("prompt", "", "Optional user prompt/question about the content")
	timeout := flag.Duration("timeout", 90*time.Second, "Timeout for the entire operation") // Increased timeout to 90s

	flag.Parse()

	// Validate required flags
	if *url == "" {
		flag.Usage()
		log.Fatal("Error: -url flag is required")
	}

	// Check for API key (handled within NewOpenAIClient, but good practice to check early)
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("Error: OPENAI_API_KEY environment variable not set")
	}

	// Set up context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Initialize Fetcher
	f, err := fetcher.NewChromeDPFetcher()
	if err != nil {
		log.Fatalf("Error creating fetcher: %v", err)
	}
	defer f.Close() // Ensure browser resources are released

	// Initialize LLM Client
	l, err := llm.NewOpenAIClient()
	if err != nil {
		log.Fatalf("Error creating LLM client: %v", err)
	}

	// Initialize App
	application := app.NewApp(f, l)

	// Process the URL
	log.Printf("Processing URL: %s", *url)
	if *prompt != "" {
		log.Printf("With user prompt: %s", *prompt)
	}

	result, err := application.ProcessURL(ctx, *url, *prompt)
	if err != nil {
		log.Fatalf("Error processing URL: %v", err)
	}

	// Print the result
	fmt.Println(result)
	log.Println("Processing finished successfully.")
}
