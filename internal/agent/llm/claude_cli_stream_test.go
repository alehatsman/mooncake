package llm

import (
	"context"
	"strings"
	"testing"
)

// delta records one onDelta callback for assertions.
type delta struct {
	text string
	kind string
}

// collectDeltas returns an onDelta callback plus a pointer to the slice it
// appends into, so a test can inspect what the stream forwarded.
func collectDeltas() (func(text, kind string), *[]delta) {
	var got []delta
	return func(text, kind string) {
		got = append(got, delta{text: text, kind: kind})
	}, &got
}

// joinKind concatenates the forwarded text for one kind — the live view a
// consumer would reassemble. Coalescing must not lose or reorder content,
// so this should equal the original per-kind text regardless of flush points.
func joinKind(got []delta, kind string) string {
	var b strings.Builder
	for _, d := range got {
		if d.kind == kind {
			b.WriteString(d.text)
		}
	}
	return b.String()
}

func TestScanPlanStream_TextAndThinking(t *testing.T) {
	// A representative stream-json transcript (verified shape 2026-06-01):
	// init/status/rate-limit noise, a thinking block, a text block, then the
	// terminal result line. Coalescing must reassemble each block faithfully.
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"x"}`,
		`{"type":"rate_limit_event","rate_limit_info":{"status":"allowed"}}`,
		`{"type":"stream_event","event":{"type":"message_start","message":{}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me "}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"plan this out carefully."}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"abc=="}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"text"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"- shell:\n"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"    cmd: echo hi"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":1}}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"- shell:\n    cmd: echo hi"}`,
	}, "\n")

	onDelta, got := collectDeltas()
	result, gotResult, err := scanPlanStream(strings.NewReader(stream), onDelta)
	if err != nil {
		t.Fatalf("scanPlanStream error: %v", err)
	}
	if !gotResult {
		t.Fatal("gotResult = false, want true (terminal result line present)")
	}
	if want := "- shell:\n    cmd: echo hi"; result != want {
		t.Errorf("result = %q, want %q", result, want)
	}
	if want := "Let me plan this out carefully."; joinKind(*got, "thinking") != want {
		t.Errorf("reassembled thinking = %q, want %q", joinKind(*got, "thinking"), want)
	}
	if want := "- shell:\n    cmd: echo hi"; joinKind(*got, "text") != want {
		t.Errorf("reassembled text = %q, want %q", joinKind(*got, "text"), want)
	}
	// signature_delta must never surface as a delta.
	for _, d := range *got {
		if strings.Contains(d.text, "abc==") {
			t.Errorf("signature_delta leaked into a planner.delta: %+v", d)
		}
	}
}

func TestScanPlanStream_Coalesces(t *testing.T) {
	// Many tiny text deltas. The byte threshold (deltaCoalesceBytes=80) plus
	// the content_block_stop flush should produce far fewer events than the
	// 200 input deltas, while preserving the full reassembled text.
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"x"}}}`)
	}
	lines = append(lines,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"`+strings.Repeat("x", 200)+`"}`,
	)
	stream := strings.Join(lines, "\n")

	onDelta, got := collectDeltas()
	result, gotResult, err := scanPlanStream(strings.NewReader(stream), onDelta)
	if err != nil || !gotResult {
		t.Fatalf("scanPlanStream err=%v gotResult=%v", err, gotResult)
	}
	if result != strings.Repeat("x", 200) {
		t.Errorf("result length = %d, want 200", len(result))
	}
	if joinKind(*got, "text") != strings.Repeat("x", 200) {
		t.Error("coalescing lost or reordered text")
	}
	// 200 single-byte tokens at an 80-byte threshold ⇒ at most a few events.
	if len(*got) > 10 {
		t.Errorf("emitted %d deltas for 200 tokens — coalescing not effective", len(*got))
	}
}

func TestScanPlanStream_IsErrorEnvelope(t *testing.T) {
	// A terminal result line with is_error=true is a genuine API failure:
	// scanPlanStream returns gotResult=true (we DID parse a terminal line)
	// plus the error, so the caller surfaces it instead of falling back.
	stream := `{"type":"result","subtype":"error_max_turns","is_error":true,"api_error_status":429,"result":"rate limited"}`
	onDelta, _ := collectDeltas()
	_, gotResult, err := scanPlanStream(strings.NewReader(stream), onDelta)
	if err == nil {
		t.Fatal("expected error for is_error=true envelope")
	}
	if !gotResult {
		t.Error("gotResult = false, want true (a terminal line was parsed)")
	}
	if !strings.Contains(err.Error(), "rate limited") || !strings.Contains(err.Error(), "error_max_turns") {
		t.Errorf("error should surface subtype + result, got: %v", err)
	}
}

func TestScanPlanStream_DriftNoResult(t *testing.T) {
	// Stream ends cleanly but with no terminal result line (envelope drift).
	// gotResult=false signals the caller to fall back to the buffered call;
	// it is NOT an error.
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"partial"}}}`,
	}, "\n")
	onDelta, _ := collectDeltas()
	_, gotResult, err := scanPlanStream(strings.NewReader(stream), onDelta)
	if err != nil {
		t.Fatalf("drift should not be an error, got: %v", err)
	}
	if gotResult {
		t.Error("gotResult = true, want false (no terminal result line)")
	}
}

func TestScanPlanStream_ToleratesMalformedLine(t *testing.T) {
	// A single garbage line is skipped; the terminal result still drives the
	// return so transient framing noise can't break planning.
	stream := strings.Join([]string{
		`this is not json`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"ok"}`,
	}, "\n")
	onDelta, got := collectDeltas()
	result, gotResult, err := scanPlanStream(strings.NewReader(stream), onDelta)
	if err != nil || !gotResult {
		t.Fatalf("scanPlanStream err=%v gotResult=%v", err, gotResult)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}
	if joinKind(*got, "text") != "ok" {
		t.Errorf("text delta = %q, want %q", joinKind(*got, "text"), "ok")
	}
}

func TestGeneratePlanStream_MockCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// Stub asserts the streaming argv (stream-json + verbose +
	// include-partial-messages) and emits a small valid transcript.
	mockCLI := writeMockCLI(t, `#!/bin/bash
seen_stream=0; seen_verbose=0; seen_partial=0
for arg in "$@"; do
  [[ "$arg" == "stream-json" ]] && seen_stream=1
  [[ "$arg" == "--verbose" ]] && seen_verbose=1
  [[ "$arg" == "--include-partial-messages" ]] && seen_partial=1
done
if [[ $seen_stream -eq 1 && $seen_verbose -eq 1 && $seen_partial -eq 1 ]]; then
cat <<'EOF'
{"type":"system","subtype":"init"}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"- shell:\n    cmd: echo hi"}}}
{"type":"result","subtype":"success","is_error":false,"result":"- shell:\n    cmd: echo hi"}
EOF
else
  echo "streaming argv missing flags: $*" >&2
  exit 1
fi
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}
	onDelta, got := collectDeltas()
	plan, err := client.GeneratePlanStream(context.Background(), "system", "user", "", onDelta)
	if err != nil {
		t.Fatalf("GeneratePlanStream failed: %v", err)
	}
	if want := "- shell:\n    cmd: echo hi"; plan != want {
		t.Errorf("plan = %q, want %q", plan, want)
	}
	if joinKind(*got, "text") != "- shell:\n    cmd: echo hi" {
		t.Errorf("forwarded text = %q", joinKind(*got, "text"))
	}
}

func TestGeneratePlanStream_FallsBackOnDrift(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// Stub emits a stream with NO terminal result line (drift) for the
	// streaming invocation, but a valid buffered envelope for the
	// --output-format json fallback invocation. GeneratePlanStream must
	// detect the drift, fall back, and return the buffered result.
	mockCLI := writeMockCLI(t, `#!/bin/bash
mode="buffered"
for arg in "$@"; do
  [[ "$arg" == "stream-json" ]] && mode="stream"
done
if [[ "$mode" == "stream" ]]; then
cat <<'EOF'
{"type":"system","subtype":"init"}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"partial only"}}}
EOF
else
  echo '{"type":"result","subtype":"success","is_error":false,"result":"buffered plan"}'
fi
`)

	client := &ClaudeCLIClient{cliPath: mockCLI}
	onDelta, _ := collectDeltas()
	plan, err := client.GeneratePlanStream(context.Background(), "system", "user", "", onDelta)
	if err != nil {
		t.Fatalf("GeneratePlanStream (drift→fallback) failed: %v", err)
	}
	if plan != "buffered plan" {
		t.Errorf("plan = %q, want %q (buffered fallback result)", plan, "buffered plan")
	}
}

func TestGeneratePlanStream_CLIError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	// Non-zero exit is a genuine failure surfaced verbatim — NOT a drift
	// fallback (the buffered path would fail the same way).
	mockCLI := writeMockCLI(t, `#!/bin/bash
echo "boom" >&2
exit 1
`)
	client := &ClaudeCLIClient{cliPath: mockCLI}
	onDelta, _ := collectDeltas()
	_, err := client.GeneratePlanStream(context.Background(), "system", "user", "", onDelta)
	if err == nil {
		t.Fatal("expected error on non-zero CLI exit")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should surface stderr, got: %v", err)
	}
}
