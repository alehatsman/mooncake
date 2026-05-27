package llm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMockCLI drops a bash stub at tmpDir/claude that emits the given
// script body. The stub mimics the `claude --print --output-format
// json` invocation contract: each test below decides exactly what
// envelope (or non-envelope garbage) the stub prints. Bash is used
// because the existing project tests already assume a POSIX shell on
// the test host.
func writeMockCLI(t *testing.T, body string) string {
	t.Helper()
	tmpDir := t.TempDir()
	mockCLI := filepath.Join(tmpDir, "claude")
	if err := os.WriteFile(mockCLI, []byte(body), 0755); err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	return mockCLI
}

func TestClaudeCLIClient_GeneratePlan(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// Happy path: stub emits a success envelope whose Result is a
	// plain YAML plan body. The client must unwrap result verbatim.
	mockCLI := writeMockCLI(t, `#!/bin/bash
cat <<'EOF'
{"type":"result","subtype":"success","is_error":false,"api_error_status":null,"result":"- shell:\n    cmd: echo hello"}
EOF
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}

	plan, err := client.GeneratePlan(context.Background(), "system", "user", "")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	expected := "- shell:\n    cmd: echo hello"
	if plan != expected {
		t.Errorf("Expected %q, got %q", expected, plan)
	}
}

func TestNewClaudeCLIClient_NotFound(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	os.Setenv("PATH", "/nonexistent")

	_, err := NewClaudeCLIClient()
	if err == nil {
		t.Error("Expected error when claude CLI not found")
	}
}

func TestClaudeCLIClient_WithModel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// Stub asserts the model flag arrives in the argv. We don't pin
	// argv order strictly — just that --model opus appears somewhere
	// and --output-format json is present. The test fails loudly if
	// either is missing.
	mockCLI := writeMockCLI(t, `#!/bin/bash
seen_model=0
seen_json=0
prev=""
for arg in "$@"; do
  if [[ "$prev" == "--model" && "$arg" == "opus" ]]; then
    seen_model=1
  fi
  if [[ "$prev" == "--output-format" && "$arg" == "json" ]]; then
    seen_json=1
  fi
  prev="$arg"
done
if [[ $seen_model -eq 1 && $seen_json -eq 1 ]]; then
  cat <<'EOF'
{"type":"result","subtype":"success","is_error":false,"result":"- shell:\n    cmd: echo opus"}
EOF
else
  echo "argv missing required flags: $*" >&2
  exit 1
fi
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}

	plan, err := client.GeneratePlan(context.Background(), "system", "user", "opus")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	expected := "- shell:\n    cmd: echo opus"
	if plan != expected {
		t.Errorf("Expected %q, got %q", expected, plan)
	}
}

func TestClaudeCLIClient_SystemPromptFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// Verifies --system-prompt is wired only when non-empty (defensive
	// edge case in the task spec).
	mockCLI := writeMockCLI(t, `#!/bin/bash
seen_sysprompt=0
prev=""
for arg in "$@"; do
  if [[ "$prev" == "--system-prompt" && "$arg" == "sys-text" ]]; then
    seen_sysprompt=1
  fi
  prev="$arg"
done
if [[ $seen_sysprompt -eq 1 ]]; then
  echo '{"type":"result","subtype":"success","is_error":false,"result":"ok"}'
else
  echo "system prompt flag missing: $*" >&2
  exit 1
fi
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}

	plan, err := client.GeneratePlan(context.Background(), "sys-text", "user", "")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}
	if plan != "ok" {
		t.Errorf("Expected %q, got %q", "ok", plan)
	}
}

func TestClaudeCLIClient_NoSystemPromptWhenEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// Empty systemPrompt must NOT emit a --system-prompt flag.
	mockCLI := writeMockCLI(t, `#!/bin/bash
for arg in "$@"; do
  if [[ "$arg" == "--system-prompt" ]]; then
    echo "should not pass --system-prompt when empty" >&2
    exit 1
  fi
done
echo '{"type":"result","subtype":"success","is_error":false,"result":"ok"}'
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}

	plan, err := client.GeneratePlan(context.Background(), "", "user", "")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}
	if plan != "ok" {
		t.Errorf("Expected %q, got %q", "ok", plan)
	}
}

func TestClaudeCLIClient_EmptyResult(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// is_error=false but result="" — preserve the historical
	// "empty output from claude CLI" error message.
	mockCLI := writeMockCLI(t, `#!/bin/bash
echo '{"type":"result","subtype":"success","is_error":false,"result":""}'
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}

	_, err := client.GeneratePlan(context.Background(), "system", "user", "")
	if err == nil {
		t.Fatal("Expected error for empty result")
	}
	if !strings.Contains(err.Error(), "empty output from claude CLI") {
		t.Errorf("Expected empty-output error, got: %v", err)
	}
}

func TestClaudeCLIClient_EmptyStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// CLI exits 0 with no stdout at all — distinct from empty result
	// inside a valid envelope. Should still surface as an error.
	mockCLI := writeMockCLI(t, `#!/bin/bash
exit 0
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}

	_, err := client.GeneratePlan(context.Background(), "system", "user", "")
	if err == nil {
		t.Error("Expected error for empty stdout")
	}
}

func TestClaudeCLIClient_CLIError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// Non-zero exit from the CLI (network failure, auth error, etc.).
	mockCLI := writeMockCLI(t, `#!/bin/bash
echo "error message" >&2
exit 1
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}

	_, err := client.GeneratePlan(context.Background(), "system", "user", "")
	if err == nil {
		t.Fatal("Expected error when CLI exits with error")
	}
	if !strings.Contains(err.Error(), "error message") {
		t.Errorf("Expected error to contain stderr output, got: %v", err)
	}
}

func TestClaudeCLIClient_IsErrorEnvelope(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// CLI exits 0 but signals failure via is_error=true in the
	// envelope. The structured failure mode is the main reason we
	// switched to --output-format json (see claude_cli_client.go doc
	// comment) — verify it surfaces as a proper error.
	mockCLI := writeMockCLI(t, `#!/bin/bash
cat <<'EOF'
{"type":"result","subtype":"error_max_turns","is_error":true,"api_error_status":429,"result":"rate limited"}
EOF
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}

	_, err := client.GeneratePlan(context.Background(), "system", "user", "")
	if err == nil {
		t.Fatal("Expected error for is_error=true envelope")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("Expected error to surface envelope result text, got: %v", err)
	}
	if !strings.Contains(err.Error(), "error_max_turns") {
		t.Errorf("Expected error to surface envelope subtype, got: %v", err)
	}
}

func TestClaudeCLIClient_InvalidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// Garbage on stdout — surface a parse error with the raw stdout
	// embedded so an operator can diagnose without re-running.
	mockCLI := writeMockCLI(t, `#!/bin/bash
echo "not json at all"
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}

	_, err := client.GeneratePlan(context.Background(), "system", "user", "")
	if err == nil {
		t.Fatal("Expected error for non-JSON stdout")
	}
	if !strings.Contains(err.Error(), "failed to parse claude CLI JSON envelope") {
		t.Errorf("Expected JSON parse error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not json at all") {
		t.Errorf("Expected error to embed raw stdout, got: %v", err)
	}
}

func TestClaudeCLIClient_PositionalUserPrompt(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// User prompt is passed positionally (last argv slot per
	// `claude [options] [prompt]`), not via stdin.
	mockCLI := writeMockCLI(t, `#!/bin/bash
# Last arg should be the user prompt.
last="${@: -1}"
if [[ "$last" == "the user goal" ]]; then
  echo '{"type":"result","subtype":"success","is_error":false,"result":"got it"}'
else
  echo "expected last arg to be user prompt, got: $last (full: $*)" >&2
  exit 1
fi
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}

	plan, err := client.GeneratePlan(context.Background(), "system", "the user goal", "")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}
	if plan != "got it" {
		t.Errorf("Expected %q, got %q", "got it", plan)
	}
}
