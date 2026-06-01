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

// StreamingClient is the optional streaming extension to Client (#76,
// Phase 2). Providers that can stream the planner's output token-by-token
// implement it; the agent loop type-asserts for it and falls back to
// GeneratePlan (emitting one synthetic delta with the full text) when a
// provider doesn't (design §4.2 alt). Keeping it a separate interface — not
// a method on Client — means the buffered HTTP providers and test stubs
// don't have to grow boilerplate they'd never exercise.
//
// onDelta receives coalesced chunks as they arrive; kind is "text" or
// "thinking". The provider coalesces tokens before each call so consumers
// aren't flooded one-event-per-token. The returned string is the final,
// complete plan text — identical to GeneratePlan's contract — so the loop's
// downstream sanitize/validate path is unchanged. Providers stay
// transport-only: the loop owns the delta→events.Event translation, so
// nothing here imports internal/events.
type StreamingClient interface {
	GeneratePlanStream(ctx context.Context, systemPrompt, userPrompt, model string, onDelta func(text, kind string)) (string, error)
}

// ClientOptions carries the explicit caller selection (CLI flags
// today, `agent.yml` later) into the provider-selection chain.
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
//  2. MOONCAKE_AGENT_PROVIDER env var
//  3. claude binary on $PATH → AnthropicCLIClient
//  4. CLAUDE_API_KEY env set → AnthropicHTTPClient
//  5. MOONCAKE_AGENT_ENDPOINT env set → OpenAIShapeClient
//  6. Error
//
// `agent.yml` discovery (step 2 of the spec's full chain) is deferred
// to a follow-up story.
func NewClientWithOptions(opts ClientOptions) (Client, error) {
	provider := opts.Provider
	if provider == "" {
		provider = os.Getenv("MOONCAKE_AGENT_PROVIDER")
	}

	switch provider {
	case "anthropic-cli", "claude":
		return NewClaudeCLIClient()
	case "anthropic-http":
		return NewClaudeClient()
	case "openai-shape":
		endpoint := opts.Endpoint
		if endpoint == "" {
			endpoint = os.Getenv("MOONCAKE_AGENT_ENDPOINT")
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

	if endpoint := os.Getenv("MOONCAKE_AGENT_ENDPOINT"); endpoint != "" {
		return NewOpenAIShapeClient(endpoint, os.Getenv("OPENAI_API_KEY"))
	}

	return nil, fmt.Errorf("no LLM provider configured; run mooncake agent doctor for setup")
}
