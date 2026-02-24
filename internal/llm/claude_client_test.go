package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestClaudeClient_GeneratePlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp := ClaudeResponse{
			ID:    "msg_123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-3-7-sonnet-20250219",
			Content: []ClaudeContentBlock{
				{
					Type: "text",
					Text: "- shell:\n    cmd: echo hello",
				},
			},
			Usage: ClaudeUsage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("CLAUDE_API_KEY", "test-key")
	defer os.Unsetenv("CLAUDE_API_KEY")

	client := &ClaudeClient{
		apiKey:     "test-key",
		endpoint:   server.URL,
		httpClient: &http.Client{},
	}

	plan, err := client.GeneratePlan(context.Background(), "system", "user", "claude-3-7-sonnet-20250219")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan != "- shell:\n    cmd: echo hello" {
		t.Errorf("Expected plan, got: %s", plan)
	}
}

func TestNewClaudeClient_MissingAPIKey(t *testing.T) {
	os.Unsetenv("CLAUDE_API_KEY")

	_, err := NewClaudeClient()
	if err == nil {
		t.Error("Expected error for missing API key")
	}
}
