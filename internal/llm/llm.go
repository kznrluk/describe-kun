package llm

import "context"

// LLM defines the interface for interacting with a Large Language Model.
type LLM interface {
	// ProcessContent takes content and an optional user prompt, returning a processed response.
	ProcessContent(ctx context.Context, content string, userPrompt string) (string, error)
	// ProcessContentWithMode allows specifying the processing mode (summary/thread)
	ProcessContentWithMode(ctx context.Context, content string, userPrompt string, mode string) (string, error)
}
