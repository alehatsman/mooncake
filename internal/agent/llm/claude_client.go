package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/alehatsman/mooncake/internal/httputil"
)

const (
	claudeAPIEndpoint = "https://api.anthropic.com/v1/messages"
	defaultMaxTokens  = 4096
	apiVersion        = "2023-06-01"

	// defaultModel is the model used when GeneratePlan is called with
	// an empty model string. Sonnet 4.6 is the current planner-grade
	// model in mooncake's documented model set (see system-prompt
	// environment block + claudeMd). The previous default
	// (claude-sonnet-4-20250514) was a 2025-05-14 snapshot of an older
	// Sonnet 4 — F040 found it routing agents to a strictly worse
	// model whenever --model was omitted.
	defaultModel = "claude-sonnet-4-6"

	// maxResponseBytes bounds io.ReadAll on the API response body.
	// Anthropic non-streaming responses for max_tokens=4096 are well
	// under 1 MB; anything larger is either a buggy gateway or a
	// malicious response (MITM, DNS hijack) trying to OOM the agent.
	maxResponseBytes = 1 << 20 // 1 MB
)

type ClaudeClient struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

// NewClaudeClient builds a Claude API client using the project's
// canonical transport (internal/httputil.DefaultTransport) for
// connection pooling and per-phase timeouts. Cancellation and total
// request budget are driven by the context passed to GeneratePlan,
// NOT by an http.Client.Timeout — agent agents on thinking models can
// legitimately take minutes per generation, and a fixed 60s ceiling
// (the pre-F040 default) silently truncated those runs.
func NewClaudeClient() (*ClaudeClient, error) {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("CLAUDE_API_KEY environment variable not set")
	}

	return &ClaudeClient{
		apiKey:   apiKey,
		endpoint: claudeAPIEndpoint,
		httpClient: &http.Client{
			Transport: httputil.DefaultTransport,
			// No overall Timeout — ctx is the budget.
		},
	}, nil
}

func (c *ClaudeClient) GeneratePlan(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	if model == "" {
		model = defaultModel
	}

	req := ClaudeRequest{
		Model:     model,
		MaxTokens: defaultMaxTokens,
		System:    systemPrompt,
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := httputil.NewRequest(ctx, "POST", c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// F040(c): bound the body read. A malicious response (or buggy
	// gateway) returning GB of garbage would OOM the agent process
	// otherwise.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if claudeResp.Error != nil {
		return "", fmt.Errorf("claude API error: %s - %s", claudeResp.Error.Type, claudeResp.Error.Message)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("empty response content")
	}

	return claudeResp.Content[0].Text, nil
}
