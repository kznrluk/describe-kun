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
	urls := extractURLs(event.Text)
	if len(urls) == 0 {
		log.Printf("No URLs found in mention from user %s in channel %s", event.User, event.Channel)
		// Optionally send a message back if no URLs are found
		// _, _, err := h.SlackClient.PostMessage(event.Channel, slack.MsgOptionText("Mention received, but no URLs found to summarize.", false), slack.MsgOptionTS(event.TimeStamp))
		// if err != nil {
		//  log.Printf("Error posting 'no URL' message: %v", err)
		// }
		return
	}

	log.Printf("Found URLs: %v in mention from user %s", urls, event.User)

	// Determine the thread timestamp (use event.ThreadTimeStamp if available, otherwise event.TimeStamp)
	threadTS := event.TimeStamp
	if event.ThreadTimeStamp != "" {
		threadTS = event.ThreadTimeStamp
	}

	for _, url := range urls {
		// Use context.Background() for now, consider request-scoped context if needed
		summary, err := h.AppCore.ProcessURL(context.Background(), url, "") // Assuming no specific prompt needed for Slack mentions
		if err != nil {
			log.Printf("Error processing URL %s: %v", url, err)
			// Post error message back to Slack thread
			_, _, postErr := h.SlackClient.PostMessage(
				event.Channel,
				slack.MsgOptionText(fmt.Sprintf("Error summarizing %s: %v", url, err), false),
				slack.MsgOptionTS(threadTS), // Post in the thread
			)
			if postErr != nil {
				log.Printf("Error posting error message to Slack: %v", postErr)
			}
			continue // Move to the next URL
		}

		// Post the summary back to the Slack thread
		response := fmt.Sprintf("Summary for %s:\n%s", url, summary)
		_, _, postErr := h.SlackClient.PostMessage(
			event.Channel,
			slack.MsgOptionText(response, false),
			slack.MsgOptionTS(threadTS), // Post in the thread
		)
		if postErr != nil {
			log.Printf("Error posting summary for %s to Slack: %v", url, postErr)
		} else {
			log.Printf("Successfully posted summary for %s to channel %s", url, event.Channel)
		}
	}
}

// extractURLs finds all URLs in a given text string
func extractURLs(text string) []string {
	// Basic regex for URLs, might need refinement for edge cases
	// This regex looks for http/https protocols
	urlRegex := regexp.MustCompile(`https?://[^\s<>"]+|www\.[^\s<>"]+`)
	return urlRegex.FindAllString(text, -1)
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
