package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/kznrluk/describe-kun/internal/fetcher"
	"github.com/kznrluk/describe-kun/internal/llm"
)

// App encapsulates the core application logic.
type App struct {
	fetcher fetcher.Fetcher
	llm     llm.LLM
}

// GetFetcher returns the fetcher instance for direct access
func (a *App) GetFetcher() fetcher.Fetcher {
	return a.fetcher
}

// NewApp creates a new App instance.
func NewApp(f fetcher.Fetcher, l llm.LLM) *App {
	return &App{
		fetcher: f,
		llm:     l,
	}
}

// ProgressCallback is a function type for progress updates
type ProgressCallback func(message string)

// ProcessURL fetches content from a URL and generates a summary using the LLM.
func (a *App) ProcessURL(ctx context.Context, url string, userPrompt string) (string, error) {
	return a.ProcessURLWithProgress(ctx, url, userPrompt, nil)
}

// ProcessURLWithProgress fetches content from a URL and generates a summary using the LLM with progress updates.
func (a *App) ProcessURLWithProgress(ctx context.Context, url string, userPrompt string, progressCallback ProgressCallback) (string, error) {
	if progressCallback != nil {
		progressCallback(fmt.Sprintf(":loading: Fetching content from %s...", url))
	}

	// Fetch content from the URL
	content, err := a.fetcher.Fetch(ctx, url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch content: %w", err)
	}

	if content == "" {
		return "", fmt.Errorf("fetched content is empty for url: %s", url)
	}

	if progressCallback != nil {
		progressCallback(fmt.Sprintf(":loading: Generating summary for %s...", url))
	}

	// Process the content using the LLM
	summary, err := a.llm.ProcessContent(ctx, content, userPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to process content: %w", err)
	}

	return summary, nil
}

// ThreadContext represents the context of a thread conversation
type ThreadContext struct {
	Messages    []string // All messages in the thread
	URLs        []string // All URLs found in the thread
	URLContents map[string]string // URL -> fetched content mapping
}

// ProcessThreadMention processes a mention within a thread context
func (a *App) ProcessThreadMention(ctx context.Context, threadContext *ThreadContext, latestMentionText string, latestMentionURLs []string) (string, error) {
	return a.ProcessThreadMentionWithProgress(ctx, threadContext, latestMentionText, latestMentionURLs, nil)
}

// ProcessThreadMentionWithProgress processes a mention within a thread context with progress updates
func (a *App) ProcessThreadMentionWithProgress(ctx context.Context, threadContext *ThreadContext, latestMentionText string, latestMentionURLs []string, progressCallback ProgressCallback) (string, error) {
	// Fetch content for any new URLs in the latest mention
	latestURLContents := make(map[string]string)
	for i, url := range latestMentionURLs {
		if progressCallback != nil {
			progressCallback(fmt.Sprintf(":loading: Fetching new URL %d/%d: %s", i+1, len(latestMentionURLs), url))
		}
		content, err := a.fetcher.Fetch(ctx, url)
		if err != nil {
			return "", fmt.Errorf("failed to fetch content for URL %s: %w", url, err)
		}
		latestURLContents[url] = content
	}

	if progressCallback != nil {
		progressCallback(":loading: Analyzing thread context and generating response...")
	}

	// Build the comprehensive prompt
	prompt := a.buildThreadPrompt(threadContext, latestMentionText, latestURLContents)

	// Process with LLM using thread mode
	response, err := a.llm.ProcessContentWithMode(ctx, prompt, "", "thread")
	if err != nil {
		return "", fmt.Errorf("failed to process thread content: %w", err)
	}

	return response, nil
}

// buildThreadPrompt constructs the prompt for thread processing
func (a *App) buildThreadPrompt(threadContext *ThreadContext, latestMentionText string, latestURLContents map[string]string) string {
	var prompt strings.Builder
	
	prompt.WriteString("You are an AI assistant helping with a conversation thread. Please analyze the context and respond appropriately to the latest user question.\n\n")
	
	// Add thread conversation history
	prompt.WriteString("---\n")
	prompt.WriteString("Thread conversation history and URL contents:\n\n")
	
	// Add all messages from the thread
	for i, message := range threadContext.Messages {
		prompt.WriteString(fmt.Sprintf("Message %d: %s\n", i+1, message))
	}
	
	// Add all URL contents from the thread
	for url, content := range threadContext.URLContents {
		prompt.WriteString(fmt.Sprintf("\nURL: %s\nContent:\n```\n%s\n```\n", url, content))
	}
	
	prompt.WriteString("---\n")
	
	// Add latest mention URL contents if any
	if len(latestURLContents) > 0 {
		prompt.WriteString("Latest mention URL contents:\n")
		for url, content := range latestURLContents {
			prompt.WriteString(fmt.Sprintf("\nURL: %s\nContent:\n```\n%s\n```\n", url, content))
		}
		prompt.WriteString("---\n")
	}
	
	// Add the latest user question
	prompt.WriteString(fmt.Sprintf("Last user question: %s\n", latestMentionText))
	
	return prompt.String()
}
