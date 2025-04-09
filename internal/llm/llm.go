package llm

import "context"

// LLM defines the interface for interacting with a Large Language Model.
type LLM interface {
	// Summarize takes content and an optional user prompt, returning a summary.
	Summarize(ctx context.Context, content string, userPrompt string) (string, error)
}
