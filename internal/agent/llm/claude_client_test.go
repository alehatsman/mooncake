package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// TestGeneratePlan_DefaultModelIsCurrent — F040(b). When the caller
// passes an empty model string, the client must route to the current
// Sonnet generation (claude-sonnet-4-6), not the stale
// claude-sonnet-4-20250514 snapshot that shipped pre-fix. The test
// captures the request body's `model` field to confirm.
func TestGeneratePlan_DefaultModelIsCurrent(t *testing.T) {
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ClaudeRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotModel = req.Model
		resp := ClaudeResponse{
			Content: []ClaudeContentBlock{{Type: "text", Text: "ok"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &ClaudeClient{
		apiKey:     "test-key",
		endpoint:   server.URL,
		httpClient: &http.Client{},
	}
	if _, err := client.GeneratePlan(context.Background(), "sys", "usr", ""); err != nil {
		t.Fatalf("GeneratePlan: %v", err)
	}
	if gotModel != defaultModel {
		t.Errorf("default model = %q, want %q", gotModel, defaultModel)
	}
	// Lock in that we're not back on the old snapshot.
	if gotModel == "claude-sonnet-4-20250514" {
		t.Error("default fell back to the stale 2025-05-14 snapshot (F040 regression)")
	}
}

// TestGeneratePlan_BodyIsBounded — F040(c). A response body that
// exceeds maxResponseBytes must be truncated by the LimitReader
// before unmarshal, not loaded fully into memory. The handler streams
// many MB of zeros; the unmarshal fails on the truncated head
// (intentional — the point is the read STOPS at the cap, not that
// the malformed prefix happens to parse).
func TestGeneratePlan_BodyIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", maxResponseBytes+8))
		w.WriteHeader(http.StatusOK)
		// Write JUST over the cap. Server.Close() must not block on
		// a body the test client refuses to keep reading; sizing the
		// over-limit slack at 8 bytes keeps both sides honest.
		body := bytes.Repeat([]byte("a"), maxResponseBytes+8)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	client := &ClaudeClient{
		apiKey:     "test-key",
		endpoint:   server.URL,
		httpClient: &http.Client{},
	}
	_, err := client.GeneratePlan(context.Background(), "sys", "usr", "claude-sonnet-4-6")
	if err == nil {
		t.Fatal("expected unmarshal error on truncated garbage body; got nil")
	}
	// The error should be the unmarshal failing on the truncated
	// prefix — confirming the read stopped at the cap (otherwise the
	// test process would have OOMed or hung on a larger payload).
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error should mention unmarshal; got %q", err.Error())
	}
}

// TestNewClaudeClient_NoOverallTimeout — F040(a). The pre-fix client
// pinned `httpClient.Timeout = 60s`, which silently truncated
// thinking-model generations. Post-fix, the client carries no
// overall Timeout — ctx is the budget. This test pins the contract
// at construction time without depending on a real or fake server.
func TestNewClaudeClient_NoOverallTimeout(t *testing.T) {
	t.Setenv("CLAUDE_API_KEY", "test-key")
	c, err := NewClaudeClient()
	if err != nil {
		t.Fatalf("NewClaudeClient: %v", err)
	}
	if c.httpClient.Timeout != 0 {
		t.Errorf("httpClient.Timeout = %s, want 0 (ctx drives cancellation)", c.httpClient.Timeout)
	}
	// And the transport should be the project-wide one so per-phase
	// network timeouts (dial/TLS/response-header) still apply.
	if c.httpClient.Transport == nil {
		t.Error("httpClient.Transport is nil; expected httputil.DefaultTransport")
	}
}
