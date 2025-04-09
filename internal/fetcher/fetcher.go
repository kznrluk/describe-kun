package fetcher

import "context"

// Fetcher defines the interface for retrieving content from a URL.
type Fetcher interface {
	// Fetch retrieves the main textual content from the given URL.
	// It should prioritize fetching content in reader mode if possible.
	Fetch(ctx context.Context, url string) (content string, err error)
}
