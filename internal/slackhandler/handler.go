package slackhandler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/kznrluk/describe-kun/internal/app" // Assuming app provides the core processing logic
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

// SlackHandler holds dependencies for handling Slack events
type SlackHandler struct {
	SlackClient   *slack.Client
	SigningSecret string
	AppCore       *app.App // Reference to the core application logic
}

// NewSlackHandler creates a new SlackHandler
func NewSlackHandler(appCore *app.App) (*SlackHandler, error) {
	botToken := os.Getenv("SLACK_BOT_TOKEN")
	signingSecret := os.Getenv("SLACK_SIGNING_SECRET")
	if botToken == "" || signingSecret == "" {
		log.Fatal("Error: SLACK_BOT_TOKEN and SLACK_SIGNING_SECRET environment variables must be set")
	}

	client := slack.New(botToken)

	return &SlackHandler{
		SlackClient:   client,
		SigningSecret: signingSecret,
		AppCore:       appCore,
	}, nil
}

// HandleEvent handles incoming HTTP requests from Slack
func (h *SlackHandler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	verifier, err := slack.NewSecretsVerifier(r.Header, h.SigningSecret)
	if err != nil {
		log.Printf("Error creating secrets verifier: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Verify the request signature
	if _, err := verifier.Write(body); err != nil {
		log.Printf("Error writing body to verifier: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := verifier.Ensure(); err != nil {
		log.Printf("Error verifying request signature: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Parse the event
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Printf("Error parsing event: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Handle URL Verification challenge
	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal(body, &r)
		if err != nil {
			log.Printf("Error unmarshalling challenge response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(r.Challenge))
		log.Println("Handled URL Verification challenge")
		return
	}

	// Handle Callback Events (like app_mention)
	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			log.Printf("Received AppMention event: User %s in channel %s said %s", ev.User, ev.Channel, ev.Text)
			// Acknowledge the event immediately to prevent Slack retries
			w.WriteHeader(http.StatusOK)
			// Process the mention in a separate goroutine to avoid blocking
			go h.handleAppMention(ev)
			return // Important: Return after starting goroutine
		default:
			log.Printf("Received unhandled event type: %T", ev)
		}
	}

	// Respond OK to other event types Slack might send
	w.WriteHeader(http.StatusOK)
}

// handleAppMention processes the AppMention event
func (h *SlackHandler) handleAppMention(event *slackevents.AppMentionEvent) {
	// Check if this is a thread mention or a new mention
	if event.ThreadTimeStamp != "" {
		// This is a mention within a thread
		h.handleThreadMention(event)
	} else {
		// This is a new mention (not in a thread)
		h.handleNewMention(event)
	}
}

// handleNewMention handles mentions that are not part of a thread (original behavior)
func (h *SlackHandler) handleNewMention(event *slackevents.AppMentionEvent) {
	urls := extractURLs(event.Text)
	if len(urls) == 0 {
		log.Printf("No URLs found in mention from user %s in channel %s", event.User, event.Channel)
		// Post a message indicating no URLs were found
		_, _, postErr := h.SlackClient.PostMessage(
			event.Channel,
			slack.MsgOptionText("No URLs found in your message. Please include a URL for me to summarize.", false),
			slack.MsgOptionTS(event.TimeStamp),
		)
		if postErr != nil {
			log.Printf("Error posting no URLs message to Slack: %v", postErr)
		}
		return
	}

	log.Printf("Found URLs: %v in mention from user %s", urls, event.User)

	// Post initial loading message
	_, loadingTS, postErr := h.SlackClient.PostMessage(
		event.Channel,
		slack.MsgOptionText(":loading:", false),
		slack.MsgOptionTS(event.TimeStamp),
	)
	if postErr != nil {
		log.Printf("Error posting loading message to Slack: %v", postErr)
		return
	}

	// Create progress updater
	progressUpdater := &ProgressUpdater{
		client:    h.SlackClient,
		channel:   event.Channel,
		timestamp: loadingTS,
	}

	// Process URLs with progress updates
	var allSummaries []string
	for i, url := range urls {
		// Update progress
		progressMsg := fmt.Sprintf(":loading: Processing URL %d/%d: %s", i+1, len(urls), url)
		progressUpdater.UpdateProgress(progressMsg)

		summary, err := h.AppCore.ProcessURLWithProgress(context.Background(), url, "", progressUpdater.UpdateProgress)
		if err != nil {
			log.Printf("Error processing URL %s: %v", url, err)
			errorMsg := fmt.Sprintf("Error summarizing %s: %v", url, err)
			progressUpdater.UpdateProgress(errorMsg)
			continue
		}

		allSummaries = append(allSummaries, fmt.Sprintf("Summary for %s:\n%s", url, summary))
	}

	// Post final result by updating the loading message
	if len(allSummaries) > 0 {
		finalResponse := strings.Join(allSummaries, "\n\n---\n\n")
		progressUpdater.UpdateProgress(finalResponse)
		log.Printf("Successfully posted summaries to channel %s", event.Channel)
	} else {
		progressUpdater.UpdateProgress("No summaries could be generated.")
	}
}

// handleThreadMention handles mentions within a thread
func (h *SlackHandler) handleThreadMention(event *slackevents.AppMentionEvent) {
	log.Printf("Handling thread mention from user %s in channel %s, thread %s", event.User, event.Channel, event.ThreadTimeStamp)

	// Post initial loading message
	_, loadingTS, postErr := h.SlackClient.PostMessage(
		event.Channel,
		slack.MsgOptionText(":loading:", false),
		slack.MsgOptionTS(event.ThreadTimeStamp),
	)
	if postErr != nil {
		log.Printf("Error posting loading message to Slack: %v", postErr)
		return
	}

	// Create progress updater
	progressUpdater := &ProgressUpdater{
		client:    h.SlackClient,
		channel:   event.Channel,
		timestamp: loadingTS,
	}

	// Update progress: Getting thread context
	progressUpdater.UpdateProgress(":loading: Getting thread context...")

	// Get thread context
	threadContext, err := h.getThreadContext(event.Channel, event.ThreadTimeStamp)
	if err != nil {
		log.Printf("Error getting thread context: %v", err)
		errorMsg := fmt.Sprintf("Error getting thread context: %v", err)
		progressUpdater.UpdateProgress(errorMsg)
		return
	}

	// Extract URLs from the latest mention
	latestMentionURLs := extractURLs(event.Text)

	// Update progress: Processing thread mention
	progressUpdater.UpdateProgress(":loading: Processing thread mention...")

	// Process the thread mention
	response, err := h.AppCore.ProcessThreadMentionWithProgress(
		context.Background(),
		threadContext,
		event.Text,
		latestMentionURLs,
		progressUpdater.UpdateProgress,
	)
	if err != nil {
		log.Printf("Error processing thread mention: %v", err)
		errorMsg := fmt.Sprintf("Error processing thread mention: %v", err)
		progressUpdater.UpdateProgress(errorMsg)
		return
	}

	// Post the final response by updating the loading message
	progressUpdater.UpdateProgress(response)
	log.Printf("Successfully posted thread response to channel %s", event.Channel)
}

// getThreadContext retrieves all messages and URLs from a thread
func (h *SlackHandler) getThreadContext(channel, threadTS string) (*app.ThreadContext, error) {
	// Get conversation replies (thread messages)
	replies, _, _, err := h.SlackClient.GetConversationReplies(&slack.GetConversationRepliesParameters{
		ChannelID: channel,
		Timestamp: threadTS,
		Inclusive: true, // Include the parent message
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation replies: %w", err)
	}

	threadContext := &app.ThreadContext{
		Messages:    make([]string, 0),
		URLs:        make([]string, 0),
		URLContents: make(map[string]string),
	}

	// Collect all messages and URLs from the thread
	allURLs := make(map[string]bool) // Use map to avoid duplicates
	for _, message := range replies {
		// Add message text
		threadContext.Messages = append(threadContext.Messages, message.Text)

		// Extract URLs from this message
		urls := extractURLs(message.Text)
		for _, url := range urls {
			if !allURLs[url] {
				allURLs[url] = true
				threadContext.URLs = append(threadContext.URLs, url)
			}
		}
	}

	// Fetch raw content for all URLs found in the thread
	fetcher := h.AppCore.GetFetcher()
	for _, url := range threadContext.URLs {
		content, err := fetcher.Fetch(context.Background(), url)
		if err != nil {
			log.Printf("Warning: failed to fetch content for URL %s in thread context: %v", url, err)
			// Continue with other URLs even if one fails
			threadContext.URLContents[url] = fmt.Sprintf("Error fetching content: %v", err)
		} else {
			// Store the raw content
			threadContext.URLContents[url] = content
		}
	}

	return threadContext, nil
}

// extractURLs finds all URLs in a given text string
func extractURLs(text string) []string {
	// Basic regex for URLs, might need refinement for edge cases
	// This regex looks for http/https protocols
	urlRegex := regexp.MustCompile(`https?://[^\s<>"]+|www\.[^\s<>"]+`)
	return urlRegex.FindAllString(text, -1)
}

// ProgressUpdater handles updating Slack messages with progress information
type ProgressUpdater struct {
	client    *slack.Client
	channel   string
	timestamp string
}

// UpdateProgress updates the Slack message with new progress information
func (p *ProgressUpdater) UpdateProgress(message string) {
	_, _, _, err := p.client.UpdateMessage(
		p.channel,
		p.timestamp,
		slack.MsgOptionText(message, false),
	)
	if err != nil {
		log.Printf("Error updating progress message: %v", err)
	}
}

// Helper function to replace the request body after reading it once
// Needed because the request body can only be read once, but we need it for verification and parsing
func drainAndReplaceBody(r *http.Request) ([]byte, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body.Close()                                    // Close the original body
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Replace with a new reader
	return bodyBytes, nil
}
