package llm

import (
	"context"
	"os"
	"testing"
)

func TestNewOpenAIClient_MissingAPIKey(t *testing.T) {
	// Unset the API key temporarily
	originalKey, keyExists := os.LookupEnv("OPENAI_API_KEY")
	if keyExists {
		os.Unsetenv("OPENAI_API_KEY")
		defer os.Setenv("OPENAI_API_KEY", originalKey) // Restore later
	}

	_, err := NewOpenAIClient()
	if err == nil {
		t.Fatal("Expected an error when OPENAI_API_KEY is not set, but got nil")
	}
}

// TestSummarize requires a valid OPENAI_API_KEY to be set in the environment.
// It also makes a real API call, which might incur costs.
// Consider using mocks for more robust testing in a real-world scenario.
func TestSummarize_Integration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	client, err := NewOpenAIClient()
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	ctx := context.Background()
	content := "The Go programming language is an open source project to make programmers more productive. Go is expressive, concise, clean, and efficient. Its concurrency mechanisms make it easy to write programs that get the most out of multicore and networked machines, while its novel type system enables flexible and modular program construction. Go compiles quickly to machine code yet has the convenience of garbage collection and the power of run-time reflection. It's a fast, statically typed, compiled language that feels like a dynamically typed, interpreted language."
	userPrompt := "What are the key features of Go?"

	// Test with user prompt
	summaryWithPrompt, err := client.Summarize(ctx, content, userPrompt)
	if err != nil {
		t.Fatalf("Summarize with prompt failed: %v", err)
	}
	if summaryWithPrompt == "" {
		t.Error("Expected a summary with prompt, but got empty string")
	}
	t.Logf("Summary with prompt:\n%s", summaryWithPrompt) // Log for manual inspection

	// Test without user prompt (just summary)
	summaryOnly, err := client.Summarize(ctx, content, "")
	if err != nil {
		t.Fatalf("Summarize without prompt failed: %v", err)
	}
	if summaryOnly == "" {
		t.Error("Expected a summary only, but got empty string")
	}
	t.Logf("Summary only:\n%s", summaryOnly) // Log for manual inspection
}
