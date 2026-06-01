// Package llm provides LLM client implementations for plan generation.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type ClaudeCLIClient struct {
	cliPath string
}

func NewClaudeCLIClient() (*ClaudeCLIClient, error) {
	cliPath, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude CLI not found in PATH: %w", err)
	}

	return &ClaudeCLIClient{
		cliPath: cliPath,
	}, nil
}

// claudeCLIEnvelope is the shape returned by
// `claude --print --output-format json` as of Claude CLI v2.1.150
// (verified 2026-05-27). If the CLI evolves the shape, adjust field
// names here; sanitize.go downstream is format-agnostic and consumes
// the Result string as-is (fence-stripping + YAML/JSON auto-decode).
//
// Sample envelope (truncated):
//
//	{"type":"result","subtype":"success","is_error":false,
//	 "result":"...assistant text...","duration_ms":2521, ...}
//
// On error the CLI sets is_error=true and may populate api_error_status
// or leave Result empty depending on the failure mode.
type claudeCLIEnvelope struct {
	Type           string `json:"type"`
	Subtype        string `json:"subtype"`
	IsError        bool   `json:"is_error"`
	APIErrorStatus any    `json:"api_error_status"`
	Result         string `json:"result"`
}

// GeneratePlan invokes the Claude CLI in non-interactive mode with a
// structured JSON envelope. System and user prompts are passed via
// separate flags (--system-prompt + positional prompt) so the CLI
// gets a proper role split, which both improves instruction-following
// and lets the CLI prompt-cache the system text across iterations.
func (c *ClaudeCLIClient) GeneratePlan(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	args := []string{"--print", "--output-format", "json"}

	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	if model != "" {
		args = append(args, "--model", model)
	}

	// Positional prompt comes last per `claude [options] [prompt]`.
	args = append(args, userPrompt)

	cmd := exec.CommandContext(ctx, c.cliPath, args...) // #nosec G204 -- cliPath is from LookPath, not user input

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude CLI failed: %w\nStderr: %s", err, stderr.String())
	}

	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return "", fmt.Errorf("empty output from claude CLI\nStderr: %s", stderr.String())
	}

	var env claudeCLIEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", fmt.Errorf("failed to parse claude CLI JSON envelope: %w\nStderr: %s\nStdout: %s",
			err, stderr.String(), truncateForError(raw))
	}

	if env.IsError {
		return "", fmt.Errorf("claude CLI returned error envelope (subtype=%s, api_error_status=%v): %s",
			env.Subtype, env.APIErrorStatus, env.Result)
	}

	result := strings.TrimSpace(env.Result)
	if result == "" {
		return "", fmt.Errorf("empty output from claude CLI")
	}

	return result, nil
}

// truncateForError caps the stdout dump embedded in a parse-failure
// error so we don't blast multi-MB blobs into operator logs.
func truncateForError(b []byte) string {
	const max = 512
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...(truncated)"
}
