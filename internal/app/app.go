package app

import (
	"context"
	"fmt"

	"github.com/kznrluk/describe-kun/internal/fetcher"
	"github.com/kznrluk/describe-kun/internal/llm"
)

// App encapsulates the core application logic.
type App struct {
	fetcher fetcher.Fetcher
	llm     llm.LLM
}

// NewApp creates a new App instance.
func NewApp(f fetcher.Fetcher, l llm.LLM) *App {
	return &App{
		fetcher: f,
		llm:     l,
	}
}

// ProcessURL fetches content from a URL and generates a summary using the LLM.
func (a *App) ProcessURL(ctx context.Context, url string, userPrompt string) (string, error) {
	// Fetch content from the URL
	content, err := a.fetcher.Fetch(ctx, url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch content: %w", err)
	}

	if content == "" {
		return "", fmt.Errorf("fetched content is empty for url: %s", url)
	}

	// Summarize the content using the LLM
	summary, err := a.llm.Summarize(ctx, content, userPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to summarize content: %w", err)
	}

	return summary, nil
}
