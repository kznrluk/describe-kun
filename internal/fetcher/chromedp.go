package fetcher

import (
	"context"
	"errors" // Added import
	"fmt"    // Added import
	"log"
	"strings"
	"time"

	// Added import
	"github.com/chromedp/chromedp"
)

// ChromeDPFetcher implements the Fetcher interface using ChromeDP.
type ChromeDPFetcher struct {
	allocatorCancel context.CancelFunc
	browserCtx      context.Context
}

// NewChromeDPFetcher creates a new ChromeDP fetcher instance.
// It initializes a headless browser instance.
func NewChromeDPFetcher() (*ChromeDPFetcher, error) {
	// Start with default options, can customize later if needed
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),           // Often needed in headless environments
		chromedp.Flag("no-sandbox", true),            // Required in some environments like Docker
		chromedp.Flag("disable-dev-shm-usage", true), // Avoid issues with limited /dev/shm size
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)

	// Create a new browser context
	browserCtx, _ := chromedp.NewContext(allocCtx) // Error is handled during Run

	// Perform a simple check to ensure the browser starts correctly
	err := chromedp.Run(browserCtx, chromedp.Navigate("about:blank"))
	if err != nil {
		cancel() // Clean up allocator context if browser fails to start
		return nil, fmt.Errorf("failed to start browser: %w", err)
	}

	return &ChromeDPFetcher{
		allocatorCancel: cancel,
		browserCtx:      browserCtx,
	}, nil
}

// Fetch retrieves the main textual content from the given URL using ChromeDP.
func (f *ChromeDPFetcher) Fetch(ctx context.Context, url string) (string, error) {
	var content string
	var statusCode int64

	// Use the browser context created in NewChromeDPFetcher
	// Combine the passed context with the browser context for timeout/cancellation
	runCtx, cancel := context.WithCancel(f.browserCtx)
	defer cancel() // Ensure task context is cancelled

	// Link the parent context (passed to Fetch) for cancellation signals
	go func() {
		select {
		case <-ctx.Done():
			cancel() // Cancel the chromedp run if the parent context is done
		case <-runCtx.Done():
			// Chromedp run finished or was cancelled internally
		}
	}()

	log.Printf("[Fetcher] Starting actions for %s", url)
	start := time.Now()

	actions := []chromedp.Action{
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("[Fetcher] Navigating to %s...", url)
			return nil
		}),
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("[Fetcher] Navigation finished or timed out (%s)", time.Since(start))
			return nil
		}),
		// Check status code after navigation (best effort, might run before full load sometimes)
		chromedp.Evaluate(`window.performance.getEntriesByType('navigation')[0]?.responseStatus`, &statusCode),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("[Fetcher] Status code evaluated (%s)", time.Since(start))
			return nil
		}),
		// Remove common non-content elements via JavaScript before extracting text
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("[Fetcher] Running cleanup script...")
			return nil
		}),
		chromedp.Evaluate(`document.querySelectorAll('script, style, nav, footer, aside, [role="navigation"], [role="complementary"], [aria-hidden="true"]').forEach(el => el.remove());`, nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("[Fetcher] Cleanup script finished (%s)", time.Since(start))
			return nil
		}),
		// Extract text from the modified body
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("[Fetcher] Extracting body innerText...")
			return nil
		}),
		// Use Evaluate to get innerText instead of Text with NodeVisible
		chromedp.Evaluate(`document.body.innerText`, &content),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("[Fetcher] innerText extracted (%s)", time.Since(start))
			return nil
		}),
	}

	err := chromedp.Run(runCtx, actions...)

	log.Printf("[Fetcher] chromedp.Run finished for %s after %s", url, time.Since(start))

	if err != nil {
		// Check if the error is due to context cancellation (timeout or external cancel)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("chromedp context cancelled or timed out for %s: %w", url, err)
		}
		return "", fmt.Errorf("failed to fetch content from %s: %w", url, err)
	}

	// Check HTTP status code after successful run
	if statusCode != 0 && (statusCode < 200 || statusCode >= 300) {
		return "", fmt.Errorf("received non-2xx status code %d for %s", statusCode, url)
	}
	if statusCode == 0 && content == "" {
		// Sometimes status code might not be captured, but empty content is a good indicator of failure
		return "", fmt.Errorf("failed to retrieve content or status code for %s", url)
	}

	// Basic cleanup - replace multiple newlines/spaces
	content = strings.Join(strings.Fields(content), " ")

	return content, nil
}

// Close terminates the browser instance and releases resources.
func (f *ChromeDPFetcher) Close() {
	// Cancel the allocator context, which should close the browser
	f.allocatorCancel()
	// It's good practice to also explicitly cancel the browser context if needed,
	// but cancelling the allocator context is usually sufficient.
	// chromedp.Cancel(f.browserCtx) // This might be redundant
}
