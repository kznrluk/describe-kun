package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testHTML = `
<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
    <h1>Main Title</h1>
    <article>
        <p>This is the main content paragraph 1.</p>
        <p>This is the main content paragraph 2. It contains <span>some nested</span> elements.</p>
        <script>console.log("Ignore this script");</script>
    </article>
    <footer>Footer content</footer>
</body>
</html>`

func TestChromeDPFetcher_Fetch(t *testing.T) {
	// Set up a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, testHTML)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	fetcher, err := NewChromeDPFetcher()
	if err != nil {
		// Skip test if Chrome is not found or other setup error occurs
		t.Skipf("Skipping test: Failed to create ChromeDPFetcher: %v", err)
		return
	}
	defer fetcher.Close() // Ensure browser context is closed

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Add timeout
	defer cancel()

	testURL := server.URL + "/test"
	content, err := fetcher.Fetch(ctx, testURL)

	if err != nil {
		t.Fatalf("Fetch failed for URL %s: %v", testURL, err)
	}

	if content == "" {
		t.Fatal("Expected content, but got empty string")
	}

	// Basic checks for expected content (exact match might be brittle due to headless rendering)
	expectedSubstrings := []string{
		"Main Title",
		"main content paragraph 1",
		"main content paragraph 2",
		"some nested", // Check if nested elements are included
	}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(content, sub) {
			t.Errorf("Expected content to contain '%s', but it didn't.\nFull content:\n%s", sub, content)
		}
	}

	// Check that script content and footer are likely excluded (common in reader modes/text extraction)
	unexpectedSubstrings := []string{
		"Ignore this script",
		"Footer content",
		"<script>",
		"<footer>",
	}
	for _, sub := range unexpectedSubstrings {
		if strings.Contains(content, sub) {
			t.Errorf("Expected content NOT to contain '%s', but it did.\nFull content:\n%s", sub, content)
		}
	}

	t.Logf("Fetched content:\n%s", content) // Log for manual inspection
}

func TestChromeDPFetcher_Fetch_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	fetcher, err := NewChromeDPFetcher()
	if err != nil {
		t.Skipf("Skipping test: Failed to create ChromeDPFetcher: %v", err)
		return
	}
	defer fetcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	testURL := server.URL + "/nonexistent"
	_, err = fetcher.Fetch(ctx, testURL)

	if err == nil {
		t.Fatalf("Expected an error for a 404 URL (%s), but got nil", testURL)
	}
	t.Logf("Received expected error for 404: %v", err)
}
