package llm

import (
	"context"
	"fmt"
	"os"

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
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is not set\n\nSet it with: export ANTHROPIC_API_KEY=your-key-here")
	}

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

// GenerateResult contains the generated entry and metadata from the API call
type GenerateResult struct {
	Content   string
	ModelID   string
	MessageID string
}

// GenerateEntry creates a journal entry based on system metrics
func (c *Client) GenerateEntry(ctx context.Context, personaDescription string, snapshot *metrics.Snapshot) (*GenerateResult, error) {
	promptCtx := prompt.NewContext(personaDescription, snapshot)
	promptText, err := prompt.RenderDefault(promptCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
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
		return nil, fmt.Errorf("failed to generate entry: %w", err)
	}

	for _, block := range message.Content {
		if block.Type == "text" {
			return &GenerateResult{
				Content:   block.Text,
				ModelID:   string(message.Model),
				MessageID: message.ID,
			}, nil
		}
	}

	return nil, fmt.Errorf("no text content in response")
}
