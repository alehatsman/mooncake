// Package llm provides LLM client implementations for plan generation.
package llm

import (
	"context"
	"fmt"
	"os"
)

type Client interface {
	GeneratePlan(ctx context.Context, systemPrompt, userPrompt, model string) (string, error)
}

// ClientOptions carries the explicit caller selection (CLI flags
// today, `pilot.yml` later) into the provider-selection chain.
type ClientOptions struct {
	Provider string
	Endpoint string
	APIKey   string
	Model    string
}

// NewClient is the zero-arg shim. Keeps loop.go:46 compiling without
// a wider refactor in this story; equivalent to NewClientWithOptions
// with a zero-value ClientOptions.
func NewClient() (Client, error) {
	return NewClientWithOptions(ClientOptions{})
}

// NewClientWithOptions resolves the provider-selection chain per
// spec-67 §5.1. Precedence (first match wins):
//
//  1. opts.Provider (CLI --provider)
//  2. MOONCAKE_PILOT_PROVIDER env var
//  3. claude binary on $PATH → AnthropicCLIClient
//  4. CLAUDE_API_KEY env set → AnthropicHTTPClient
//  5. MOONCAKE_PILOT_ENDPOINT env set → OpenAIShapeClient
//  6. Error
//
// `pilot.yml` discovery (step 2 of the spec's full chain) is deferred
// to a follow-up story.
func NewClientWithOptions(opts ClientOptions) (Client, error) {
	provider := opts.Provider
	if provider == "" {
		provider = os.Getenv("MOONCAKE_PILOT_PROVIDER")
	}

	switch provider {
	case "anthropic-cli", "claude":
		return NewClaudeCLIClient()
	case "anthropic-http":
		return NewClaudeClient()
	case "openai-shape":
		endpoint := opts.Endpoint
		if endpoint == "" {
			endpoint = os.Getenv("MOONCAKE_PILOT_ENDPOINT")
		}
		apiKey := opts.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		return NewOpenAIShapeClient(endpoint, apiKey)
	case "":
		// Auto-discover. Keep the historical Claude-first order.
	default:
		return nil, fmt.Errorf("unknown provider %q (want anthropic-cli, anthropic-http, or openai-shape)", provider)
	}

	if cliClient, err := NewClaudeCLIClient(); err == nil {
		return cliClient, nil
	}

	if os.Getenv("CLAUDE_API_KEY") != "" {
		return NewClaudeClient()
	}

	if endpoint := os.Getenv("MOONCAKE_PILOT_ENDPOINT"); endpoint != "" {
		return NewOpenAIShapeClient(endpoint, os.Getenv("OPENAI_API_KEY"))
	}

	return nil, fmt.Errorf("no LLM provider configured; run mooncake pilot doctor for setup")
}
