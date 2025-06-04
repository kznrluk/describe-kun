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

// ProcessContent uses the OpenAI API to process the given content.
// If userPrompt is provided, it attempts to answer the prompt based on the content first.
func (c *OpenAIClient) ProcessContent(ctx context.Context, content string, userPrompt string) (string, error) {
	return c.ProcessContentWithMode(ctx, content, userPrompt, "summary")
}

// ProcessContentWithMode allows specifying the processing mode
func (c *OpenAIClient) ProcessContentWithMode(ctx context.Context, content string, userPrompt string, mode string) (string, error) {
	var systemPrompt string
	var instructions string

	switch mode {
	case "thread":
		// Simple Q&A format for thread responses
		systemPrompt = `You are an AI assistant helping with a conversation thread. Analyze the provided context and respond naturally to the user's question. Provide clear, helpful answers based on the information available.`

		if userPrompt != "" {
			instructions = fmt.Sprintf("Based on the provided context, please answer the following question: %s\n\nIf the context doesn't contain enough information to answer the question, please state that clearly.", userPrompt)
		} else {
			instructions = "Please provide a helpful response based on the provided context."
		}

	default: // "summary" mode
		// Original format for initial mentions
		systemPrompt = `You are an expert summarizer. Analyze the provided web page content and generate a concise summary based on the user's request.

Output Format:
(If the user asked a question, answer it here based *only* on the provided text. If the text doesn't contain the answer, state that clearly. If no question was asked, omit this section.)

:white_check_mark: 3行要約
- Bullet point 1
- Bullet point 2
- Bullet point 3

:memo: 説明
*Key points header 1*
Explanation of the main points of the article

*Key points header 2*
Explanation of the main points of the article

(Key points can be increased arbitrarily)
`

		if userPrompt != "" {
			instructions = fmt.Sprintf("User Question: %s\n\nInstructions: First, answer the user's question based *only* on the provided content. If the content doesn't contain the answer, state 'この記事にはその情報が含まれていません。'. Then, provide the 3-line summary and the detailed explanation as described in the system prompt.", userPrompt)
		} else {
			instructions = "Instructions: Provide the 3-line summary and the detailed explanation as described in the system prompt."
		}
	}

	prompt := fmt.Sprintf("Content:\n```\n%s\n```\n\n%s", content, instructions)

	model := "chatgpt-4o-latest"
	if os.Getenv("OPENAI_MODEL") != "" {
		model = os.Getenv("OPENAI_MODEL")
	}

	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: model,
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
