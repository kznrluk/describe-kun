package app

import (
	"context"
	"errors"
	"testing"
)

// MockFetcher is a mock implementation of the Fetcher interface.
type MockFetcher struct {
	FetchFunc func(ctx context.Context, url string) (string, error)
}

func (m *MockFetcher) Fetch(ctx context.Context, url string) (string, error) {
	if m.FetchFunc != nil {
		return m.FetchFunc(ctx, url)
	}
	return "", errors.New("FetchFunc not implemented")
}

// MockLLM is a mock implementation of the LLM interface.
type MockLLM struct {
	ProcessContentFunc     func(ctx context.Context, content string, userPrompt string) (string, error)
	ProcessContentWithModeFunc func(ctx context.Context, content string, userPrompt string, mode string) (string, error)
}

func (m *MockLLM) ProcessContent(ctx context.Context, content string, userPrompt string) (string, error) {
	if m.ProcessContentFunc != nil {
		return m.ProcessContentFunc(ctx, content, userPrompt)
	}
	return "", errors.New("ProcessContentFunc not implemented")
}

func (m *MockLLM) ProcessContentWithMode(ctx context.Context, content string, userPrompt string, mode string) (string, error) {
	if m.ProcessContentWithModeFunc != nil {
		return m.ProcessContentWithModeFunc(ctx, content, userPrompt, mode)
	}
	return "", errors.New("ProcessContentWithModeFunc not implemented")
}

func TestApp_ProcessURL_Success(t *testing.T) {
	mockFetcher := &MockFetcher{
		FetchFunc: func(ctx context.Context, url string) (string, error) {
			if url != "http://example.com/success" {
				return "", errors.New("unexpected URL")
			}
			return "Mock page content", nil
		},
	}

	mockLLM := &MockLLM{
		ProcessContentFunc: func(ctx context.Context, content string, userPrompt string) (string, error) {
			if content != "Mock page content" {
				return "", errors.New("unexpected content")
			}
			if userPrompt != "test prompt" {
				return "", errors.New("unexpected user prompt")
			}
			return "Mock summary", nil
		},
	}

	app := NewApp(mockFetcher, mockLLM)
	ctx := context.Background()
	result, err := app.ProcessURL(ctx, "http://example.com/success", "test prompt")

	if err != nil {
		t.Fatalf("ProcessURL failed: %v", err)
	}
	if result != "Mock summary" {
		t.Errorf("Expected result 'Mock summary', got '%s'", result)
	}
}

func TestApp_ProcessURL_FetchError(t *testing.T) {
	fetchErr := errors.New("fetch failed")
	mockFetcher := &MockFetcher{
		FetchFunc: func(ctx context.Context, url string) (string, error) {
			return "", fetchErr
		},
	}
	mockLLM := &MockLLM{} // Summarize should not be called

	app := NewApp(mockFetcher, mockLLM)
	ctx := context.Background()
	_, err := app.ProcessURL(ctx, "http://example.com/fetch-error", "")

	if !errors.Is(err, fetchErr) {
		t.Fatalf("Expected fetch error '%v', got '%v'", fetchErr, err)
	}
}

func TestApp_ProcessURL_SummarizeError(t *testing.T) {
	summarizeErr := errors.New("summarize failed")
	mockFetcher := &MockFetcher{
		FetchFunc: func(ctx context.Context, url string) (string, error) {
			return "Mock content", nil
		},
	}
	mockLLM := &MockLLM{
		ProcessContentFunc: func(ctx context.Context, content string, userPrompt string) (string, error) {
			return "", summarizeErr
		},
	}

	app := NewApp(mockFetcher, mockLLM)
	ctx := context.Background()
	_, err := app.ProcessURL(ctx, "http://example.com/summarize-error", "")

	if !errors.Is(err, summarizeErr) {
		t.Fatalf("Expected summarize error '%v', got '%v'", summarizeErr, err)
	}
}
