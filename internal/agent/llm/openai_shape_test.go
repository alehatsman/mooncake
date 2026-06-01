package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIShapeClient_GeneratePlan_HappyPath(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotContentType string
	var gotReq OpenAIRequest
	var gotBodyRaw map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")

		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(r.Body)
		_ = json.Unmarshal(body.Bytes(), &gotReq)
		_ = json.Unmarshal(body.Bytes(), &gotBodyRaw)

		resp := OpenAIResponse{
			ID:     "chatcmpl-123",
			Object: "chat.completion",
			Model:  "llama3.1:70b",
			Choices: []OpenAIChoice{
				{
					Index:        0,
					Message:      OpenAIMessage{Role: "assistant", Content: "- shell:\n    cmd: echo hello"},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &OpenAIShapeClient{
		apiKey:     "test-key",
		endpoint:   server.URL,
		httpClient: &http.Client{},
	}

	plan, err := client.GeneratePlan(context.Background(), "system", "user", "llama3.1:70b")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan != "- shell:\n    cmd: echo hello" {
		t.Errorf("Expected plan, got: %s", plan)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/chat/completions") {
		t.Errorf("path = %q, want suffix /chat/completions", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want Bearer test-key", gotAuth)
	}
	if gotReq.Model != "llama3.1:70b" {
		t.Errorf("body model = %q, want llama3.1:70b", gotReq.Model)
	}
	if len(gotReq.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(gotReq.Messages))
	}
	if gotReq.Messages[0].Role != "system" {
		t.Errorf("messages[0].role = %q, want system", gotReq.Messages[0].Role)
	}
	if gotReq.Messages[1].Role != "user" {
		t.Errorf("messages[1].role = %q, want user", gotReq.Messages[1].Role)
	}
	if gotReq.MaxTokens == 0 {
		t.Error("max_tokens must be set in body")
	}
	if _, hasTools := gotBodyRaw["tools"]; hasTools {
		t.Error("body must not contain tools field in v1 (spec §5.2 — tool-use off)")
	}
	if _, hasCC := gotBodyRaw["cache_control"]; hasCC {
		t.Error("body must not contain cache_control (Anthropic-only)")
	}
	if _, hasRF := gotBodyRaw["response_format"]; hasRF {
		t.Error("body must not contain response_format in v1")
	}
}

func TestOpenAIShapeClient_NonOK_Returns_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"type":"server_error","message":"boom"}}`))
	}))
	defer server.Close()

	client := &OpenAIShapeClient{
		apiKey:     "test-key",
		endpoint:   server.URL,
		httpClient: &http.Client{},
	}
	_, err := client.GeneratePlan(context.Background(), "sys", "usr", "llama3.1:70b")
	if err == nil {
		t.Fatal("expected error on 500; got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500; got %q", err.Error())
	}
}

// TestOpenAIShapeClient_DefaultModelOmitted pins decision §8.1: an
// empty --model errors out with an actionable message instead of
// falling back to a built-in default. Different OpenAI-shape servers
// expect incompatible id formats (Ollama tags vs HF slugs) so picking
// a default is wrong for half the user base.
func TestOpenAIShapeClient_DefaultModelOmitted(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &OpenAIShapeClient{
		apiKey:     "",
		endpoint:   server.URL,
		httpClient: &http.Client{},
	}
	_, err := client.GeneratePlan(context.Background(), "sys", "usr", "")
	if err == nil {
		t.Fatal("expected error when model is empty; got nil")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("error should mention model; got %q", err.Error())
	}
	if called {
		t.Error("HTTP request must not be issued when model is empty")
	}
}

// TestOpenAIShapeClient_BodyIsBounded mirrors the Claude client's
// LimitReader guard. A buggy gateway or MITM that streams GB of
// garbage must not OOM the agent — the read stops at the cap and
// the truncated prefix fails to unmarshal.
func TestOpenAIShapeClient_BodyIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", maxResponseBytes+8))
		w.WriteHeader(http.StatusOK)
		body := bytes.Repeat([]byte("a"), maxResponseBytes+8)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	client := &OpenAIShapeClient{
		apiKey:     "test-key",
		endpoint:   server.URL,
		httpClient: &http.Client{},
	}
	_, err := client.GeneratePlan(context.Background(), "sys", "usr", "llama3.1:70b")
	if err == nil {
		t.Fatal("expected unmarshal error on truncated garbage body; got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error should mention unmarshal; got %q", err.Error())
	}
}

// TestOpenAIShapeClient_NoAuthHeader_WhenKeyEmpty pins the
// Ollama-on-localhost case: empty API key must NOT emit an
// Authorization header at all (not even `Bearer `).
func TestOpenAIShapeClient_NoAuthHeader_WhenKeyEmpty(t *testing.T) {
	var hadAuth bool
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadAuth = r.Header["Authorization"]
		gotAuth = r.Header.Get("Authorization")
		resp := OpenAIResponse{
			Choices: []OpenAIChoice{{Message: OpenAIMessage{Content: "ok"}}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &OpenAIShapeClient{
		apiKey:     "",
		endpoint:   server.URL,
		httpClient: &http.Client{},
	}
	if _, err := client.GeneratePlan(context.Background(), "sys", "usr", "llama3.1:70b"); err != nil {
		t.Fatalf("GeneratePlan: %v", err)
	}
	if hadAuth {
		t.Errorf("Authorization header must be absent when APIKey is empty; got %q", gotAuth)
	}
}
