// Package llm provides LLM client implementations for plan generation.
package llm

import (
	"context"
	"fmt"
)

type Client interface {
	GeneratePlan(ctx context.Context, systemPrompt, userPrompt, model string) (string, error)
}

func NewClient() (Client, error) {
	cliClient, err := NewClaudeCLIClient()
	if err == nil {
		return cliClient, nil
	}

	httpClient, err := NewClaudeClient()
	if err != nil {
		return nil, fmt.Errorf("no Claude client available: CLI not found and CLAUDE_API_KEY not set")
	}

	return httpClient, nil
}
