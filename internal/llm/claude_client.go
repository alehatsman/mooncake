package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	claudeAPIEndpoint = "https://api.anthropic.com/v1/messages"
	defaultTimeout    = 60 * time.Second
	defaultMaxTokens  = 4096
	apiVersion        = "2023-06-01"
)

type ClaudeClient struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

func NewClaudeClient() (*ClaudeClient, error) {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("CLAUDE_API_KEY environment variable not set")
	}

	return &ClaudeClient{
		apiKey:   apiKey,
		endpoint: claudeAPIEndpoint,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

func (c *ClaudeClient) GeneratePlan(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	if model == "" {
		model = "claude-sonnet-4-20250514"
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(reqBody))
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

	body, err := io.ReadAll(resp.Body)
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
