package llm

// ProviderCapabilities advertises optional features a provider's
// client supports. Per spec-67 §8 we land this struct now with a
// zero-value implementation on OpenAIShapeClient so downstream
// stories (S-agent-prompt-cache, S-agent-tool-use-spike) have a
// typed hook to slot into without re-spec'ing the shape.
type ProviderCapabilities struct {
	SupportsToolUse     bool
	SupportsPromptCache bool
}

// Capabilities reports the openai-shape v1 defaults. Per spec §5.2
// both flags are false: tool-use is opt-in via a future config field
// and prompt-cache is Anthropic-only.
func (c *OpenAIShapeClient) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{}
}
