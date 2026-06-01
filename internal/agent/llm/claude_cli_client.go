// Package llm provides LLM client implementations for plan generation.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// deltaCoalesceBytes is the planner-stream coalescing threshold (#76). The
// CLI emits content_block_delta events that can be near token-granular; we
// buffer them and flush a planner.delta only once the buffer reaches this
// size (or a content block ends / the kind flips), so a long plan doesn't
// blast one event per token at forwarders (agentd/fleet/SSE). The buffer
// also flushes on content_block_stop and at stream end, so nothing is lost.
const deltaCoalesceBytes = 80

// streamScannerMax bounds a single stream-json line. The default
// bufio.Scanner cap is 64 KiB, but the final `result` line (and the
// per-message `assistant` snapshots) carry the whole plan and can exceed
// that for a large plan — a too-small cap would error mid-stream and force
// the buffered fallback. 16 MiB is generous headroom over any real plan.
const streamScannerMax = 16 * 1024 * 1024

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

// claudeStreamLine is one NDJSON line from
// `claude --print --output-format stream-json --verbose
// --include-partial-messages` as of Claude CLI v2.1.159 (verified
// 2026-06-01). Only the fields we consume are modeled; unknown fields and
// line types (system/init, rate_limit_event, assistant snapshots,
// message_start/stop, etc.) unmarshal harmlessly and are ignored.
//
// Two line shapes matter:
//
//	{"type":"stream_event","event":{"type":"content_block_delta",
//	 "delta":{"type":"text_delta","text":"..."}}}          // forwarded
//	{"type":"stream_event","event":{"type":"content_block_delta",
//	 "delta":{"type":"thinking_delta","thinking":"..."}}}  // forwarded
//	{"type":"result","subtype":"success","is_error":false,
//	 "result":"...full text..."}                           // return value
//
// The terminal `result` line carries the same Result/IsError/Subtype shape
// as the buffered claudeCLIEnvelope, so the deltas are advisory (live view
// only) and the returned plan comes from Result — never from accumulating
// deltas, which could be coalesced or dropped under back-pressure.
type claudeStreamLine struct {
	Type  string `json:"type"`
	Event *struct {
		Type  string `json:"type"`
		Delta *struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"delta"`
	} `json:"event"`
	// Terminal result-line fields (mirror claudeCLIEnvelope).
	Subtype        string `json:"subtype"`
	IsError        bool   `json:"is_error"`
	APIErrorStatus any    `json:"api_error_status"`
	Result         string `json:"result"`
}

// GeneratePlanStream is the streaming variant of GeneratePlan (#76, Phase 2).
// It runs the CLI in stream-json mode and forwards coalesced text/thinking
// chunks via onDelta as they arrive, while returning the final plan string
// from the terminal `result` line — so the return contract is identical to
// GeneratePlan and the caller's sanitize/validate path is unchanged.
//
// On stream-json envelope drift (no terminal result line parsed) it falls
// back to the buffered GeneratePlan so a CLI format change degrades to the
// proven path instead of failing the run. A non-zero CLI exit or an
// is_error result envelope is a genuine error and is surfaced, not retried.
func (c *ClaudeCLIClient) GeneratePlanStream(ctx context.Context, systemPrompt, userPrompt, model string, onDelta func(text, kind string)) (string, error) {
	args := []string{"--print", "--output-format", "stream-json", "--verbose", "--include-partial-messages"}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, userPrompt)

	cmd := exec.CommandContext(ctx, c.cliPath, args...) // #nosec G204 -- cliPath is from LookPath, not user input
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("claude CLI stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("claude CLI start failed: %w\nStderr: %s", err, stderr.String())
	}

	result, gotResult, resultErr := scanPlanStream(stdout, onDelta)

	// Drain+Wait regardless of scan outcome so the child can't be left a
	// zombie and ctx-cancel is reflected in the Wait error.
	waitErr := cmd.Wait()
	if waitErr != nil {
		// A genuine CLI failure (non-zero exit, ctx cancel). The buffered
		// path would fail the same way, so surface it rather than retry.
		return "", fmt.Errorf("claude CLI failed: %w\nStderr: %s", waitErr, stderr.String())
	}
	if resultErr != nil {
		return "", resultErr
	}
	if !gotResult {
		// Envelope drift: the stream completed cleanly but we never parsed
		// a terminal result line. Fall back to the buffered call (design
		// §7) so a stream-json format change doesn't break planning.
		return c.GeneratePlan(ctx, systemPrompt, userPrompt, model)
	}
	return result, nil
}

// scanPlanStream reads the stream-json NDJSON from r, forwarding coalesced
// text/thinking deltas through onDelta, and returns the terminal result
// string. gotResult reports whether a terminal `result` line was parsed
// (false ⇒ envelope drift, caller should fall back). A non-nil error is an
// is_error result envelope — a real API failure, distinct from drift.
func scanPlanStream(r io.Reader, onDelta func(text, kind string)) (result string, gotResult bool, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), streamScannerMax)

	// Coalesce deltas of the same kind until the buffer hits the threshold,
	// the kind flips, or a content block ends — then emit one planner.delta.
	var pending strings.Builder
	pendingKind := ""
	flush := func() {
		if pending.Len() == 0 || onDelta == nil {
			pending.Reset()
			return
		}
		onDelta(pending.String(), pendingKind)
		pending.Reset()
	}
	emit := func(text, kind string) {
		if text == "" {
			return
		}
		if kind != pendingKind {
			flush()
			pendingKind = kind
		}
		pending.WriteString(text)
		if pending.Len() >= deltaCoalesceBytes {
			flush()
		}
	}

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var sl claudeStreamLine
		if jsonErr := json.Unmarshal(line, &sl); jsonErr != nil {
			// Tolerate a single malformed line rather than abort the stream;
			// the terminal result line (if it parses) still drives the return.
			continue
		}

		switch {
		case sl.Type == "result":
			flush()
			if sl.IsError {
				return "", true, fmt.Errorf("claude CLI returned error envelope (subtype=%s, api_error_status=%v): %s",
					sl.Subtype, sl.APIErrorStatus, sl.Result)
			}
			res := strings.TrimSpace(sl.Result)
			if res == "" {
				return "", true, fmt.Errorf("empty output from claude CLI")
			}
			return res, true, nil
		case sl.Type == "stream_event" && sl.Event != nil && sl.Event.Type == "content_block_delta" && sl.Event.Delta != nil:
			switch sl.Event.Delta.Type {
			case "text_delta":
				emit(sl.Event.Delta.Text, "text")
			case "thinking_delta":
				emit(sl.Event.Delta.Thinking, "thinking")
			}
		case sl.Type == "stream_event" && sl.Event != nil && sl.Event.Type == "content_block_stop":
			flush()
		}
	}
	flush()
	if scanErr := scanner.Err(); scanErr != nil {
		// Read error (e.g. oversized line past streamScannerMax). Treat as
		// drift so the caller falls back to the buffered call.
		return "", false, nil
	}
	// Stream ended with no result line — drift.
	return "", false, nil
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
