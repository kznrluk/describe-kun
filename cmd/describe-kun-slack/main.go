package main

import (
	"log"
	"net/http"
	"os"

	"github.com/kznrluk/describe-kun/internal/app"
	"github.com/kznrluk/describe-kun/internal/fetcher"
	"github.com/kznrluk/describe-kun/internal/llm"
	"github.com/kznrluk/describe-kun/internal/slackhandler"
)

func main() {
	// Check for necessary environment variables
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("Error: OPENAI_API_KEY environment variable not set")
	}
	if os.Getenv("SLACK_BOT_TOKEN") == "" {
		log.Fatal("Error: SLACK_BOT_TOKEN environment variable not set")
	}
	if os.Getenv("SLACK_SIGNING_SECRET") == "" {
		log.Fatal("Error: SLACK_SIGNING_SECRET environment variable not set")
	}

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

	// Initialize App Core
	application := app.NewApp(f, l)

	// Initialize Slack Handler
	slackHandler, err := slackhandler.NewSlackHandler(application)
	if err != nil {
		log.Fatalf("Error creating Slack handler: %v", err)
	}

	// Set up HTTP routes
	http.HandleFunc("/slack/events", slackHandler.HandleEvent)
	// Add a simple health check endpoint
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}

	log.Printf("Starting describe-kun Slack bot server on port %s", port)
	log.Printf("Listening for Slack events on /slack/events")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
