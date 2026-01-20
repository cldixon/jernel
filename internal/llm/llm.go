package llm

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/metrics"
	"github.com/cldixon/jernel/internal/prompt"
)

// Client wraps the Anthropic API client
type Client struct {
	api          anthropic.Client
	model        anthropic.Model
	systemPrompt string
}

// NewClient creates a new LLM client using settings from config
func NewClient(cfg *config.Config) (*Client, error) {
	systemPrompt, err := config.LoadSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to load system prompt: %w", err)
	}

	return &Client{
		api:          anthropic.NewClient(),
		model:        anthropic.Model(cfg.Model),
		systemPrompt: systemPrompt,
	}, nil
}

// GenerateEntry creates a journal entry based on system metrics
func (c *Client) GenerateEntry(ctx context.Context, personaDescription string, snapshot *metrics.Snapshot) (string, error) {
	promptCtx := prompt.NewContext(personaDescription, snapshot)
	promptText, err := prompt.RenderDefault(promptCtx)
	if err != nil {
		return "", fmt.Errorf("failed to render prompt: %w", err)
	}

	message, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{
				Type: "text",
				Text: c.systemPrompt,
			},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(promptText)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate entry: %w", err)
	}

	for _, block := range message.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("no text content in response")
}
