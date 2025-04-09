package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIClient implements the LLM interface using the OpenAI API.
type OpenAIClient struct {
	client *openai.Client
}

// NewOpenAIClient creates a new OpenAI client.
// It requires the OPENAI_API_KEY environment variable to be set.
func NewOpenAIClient() (*OpenAIClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY environment variable not set")
	}
	client := openai.NewClient(apiKey)
	return &OpenAIClient{client: client}, nil
}

// Summarize uses the OpenAI API to summarize the given content.
// If userPrompt is provided, it attempts to answer the prompt based on the content first.
func (c *OpenAIClient) Summarize(ctx context.Context, content string, userPrompt string) (string, error) {
	systemPrompt := `You are an expert summarizer. Analyze the provided web page content and generate a concise summary based on the user's request.

Output Format:
(If the user asked a question, answer it here based *only* on the provided text. If the text doesn't contain the answer, state that clearly. If no question was asked, omit this section.)

3行要約:
- Bullet point 1
- Bullet point 2
- Bullet point 3

説明:
A detailed explanation summarizing the key points of the content (around 500 characters).`

	prompt := fmt.Sprintf("Web Page Content:\n```\n%s\n```\n\n", content)

	if userPrompt != "" {
		prompt += fmt.Sprintf("User Question: %s\n\nInstructions: First, answer the user's question based *only* on the provided content. If the content doesn't contain the answer, state 'この記事にはその情報が含まれていません。'. Then, provide the 3-line summary and the detailed explanation as described in the system prompt.", userPrompt)
	} else {
		prompt += "Instructions: Provide the 3-line summary and the detailed explanation as described in the system prompt."
	}

	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4oLatest,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", fmt.Errorf("openai chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", errors.New("openai returned an empty response")
	}

	// Trim potential leading/trailing whitespace
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
