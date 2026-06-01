package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/alehatsman/mooncake/internal/httputil"
)

const (
	openAIChatCompletionsPath = "/chat/completions"
	openAIDefaultTemperature  = 0.0
)

// OpenAIShapeClient talks to any server that implements the OpenAI
// /v1/chat/completions request shape (Ollama, vLLM, LM Studio,
// llama.cpp server). Per spec-67 §8 the client appends
// `/chat/completions` to the operator's endpoint verbatim — no
// trailing-slash trimming, no path normalization.
type OpenAIShapeClient struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

// NewOpenAIShapeClient builds a client targeting an OpenAI-shape
// server. endpoint is the base URL (e.g. http://localhost:11434/v1).
// apiKey may be empty — Ollama against localhost typically has no
// auth, and an empty key must NOT emit an Authorization header.
func NewOpenAIShapeClient(endpoint, apiKey string) (*OpenAIShapeClient, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("openai-shape requires --endpoint or MOONCAKE_AGENT_ENDPOINT")
	}
	return &OpenAIShapeClient{
		apiKey:   apiKey,
		endpoint: endpoint,
		httpClient: &http.Client{
			Transport: httputil.DefaultTransport,
		},
	}, nil
}

func (c *OpenAIShapeClient) GeneratePlan(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	if model == "" {
		return "", fmt.Errorf("openai-shape requires --model or providers.openai-shape.model")
	}

	req := OpenAIRequest{
		Model: model,
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   defaultMaxTokens,
		Temperature: openAIDefaultTemperature,
		Stream:      false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.endpoint + openAIChatCompletionsPath
	httpReq, err := httputil.NewRequest(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openaiResp OpenAIResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if openaiResp.Error != nil {
		return "", fmt.Errorf("openai-shape API error: %s - %s", openaiResp.Error.Type, openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		return "", fmt.Errorf("empty response choices")
	}

	return openaiResp.Choices[0].Message.Content, nil
}
