// Package llm provides LLM client implementations for plan generation.
package llm

import (
	"bytes"
	"context"
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

func (c *ClaudeCLIClient) GeneratePlan(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	var fullPrompt strings.Builder

	fullPrompt.WriteString(systemPrompt)
	fullPrompt.WriteString("\n\n")
	fullPrompt.WriteString(userPrompt)

	args := []string{}

	if model != "" {
		args = append(args, "--model", model)
	}

	cmd := exec.CommandContext(ctx, c.cliPath, args...) // #nosec G204 -- cliPath is from LookPath, not user input
	cmd.Stdin = strings.NewReader(fullPrompt.String())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude CLI failed: %w\nStderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", fmt.Errorf("empty output from claude CLI")
	}

	return output, nil
}
