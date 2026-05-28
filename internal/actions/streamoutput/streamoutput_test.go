package streamoutput

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/events"
)

// newCtx builds a minimal actions.Context backed by the testutil
// mocks the rest of the action tests use.
func newCtx() *testutil.MockContext {
	return &testutil.MockContext{
		Publisher:     &testutil.MockPublisher{Events: []events.Event{}},
		Log:           &testutil.MockLogger{Logs: []string{}},
		CurrentStepID: "step-1",
	}
}

func TestStream_EmitsPerLineEvents(t *testing.T) {
	ctx := newCtx()
	src := strings.NewReader("alpha\nbeta\ngamma\n")
	var buf bytes.Buffer

	Stream(src, &buf, ctx, true, "stdout")

	pub := ctx.Publisher
	if len(pub.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(pub.Events))
	}
	for i, want := range []string{"alpha", "beta", "gamma"} {
		ev := pub.Events[i]
		if ev.Type != events.EventStepStdout {
			t.Errorf("event[%d].Type = %v, want EventStepStdout", i, ev.Type)
		}
		data, ok := ev.Data.(events.StepOutputData)
		if !ok {
			t.Fatalf("event[%d].Data not StepOutputData: %T", i, ev.Data)
		}
		if data.Line != want {
			t.Errorf("event[%d].Line = %q, want %q", i, data.Line, want)
		}
		if data.LineNumber != i+1 {
			t.Errorf("event[%d].LineNumber = %d, want %d", i, data.LineNumber, i+1)
		}
		if data.Stream != "stdout" {
			t.Errorf("event[%d].Stream = %q, want stdout", i, data.Stream)
		}
	}
}

func TestStream_CapturesIntoBuffer(t *testing.T) {
	ctx := newCtx()
	src := strings.NewReader("one\ntwo\n")
	var buf bytes.Buffer

	Stream(src, &buf, ctx, true, "stdout")

	if got := buf.String(); got != "one\ntwo\n" {
		t.Errorf("buf = %q, want %q", got, "one\ntwo\n")
	}
}

func TestStream_NoCaptureSkipsBuffer(t *testing.T) {
	ctx := newCtx()
	src := strings.NewReader("one\ntwo\n")
	var buf bytes.Buffer

	Stream(src, &buf, ctx, false, "stdout")

	if got := buf.String(); got != "" {
		t.Errorf("buf = %q, want empty (capture=false)", got)
	}
	if pub := ctx.Publisher; len(pub.Events) != 2 {
		t.Errorf("expected 2 events even with capture=false, got %d", len(pub.Events))
	}
}

func TestStream_StderrTagsEventType(t *testing.T) {
	ctx := newCtx()
	src := strings.NewReader("oops\n")
	var buf bytes.Buffer

	Stream(src, &buf, ctx, true, "stderr")

	pub := ctx.Publisher
	if len(pub.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.Events))
	}
	if pub.Events[0].Type != events.EventStepStderr {
		t.Errorf("Type = %v, want EventStepStderr", pub.Events[0].Type)
	}
}

func TestStream_TruncationSurfacesMarker(t *testing.T) {
	ctx := newCtx()
	// One line larger than MaxLineBytes triggers bufio.ErrTooLong.
	bigLine := strings.Repeat("x", MaxLineBytes+1) + "\n"
	src := strings.NewReader(bigLine + "tail\n")
	var buf bytes.Buffer

	Stream(src, &buf, ctx, true, "stdout")

	pub := ctx.Publisher
	foundMarker := false
	for _, ev := range pub.Events {
		if ev.Type != events.EventStepStderr {
			continue
		}
		data := ev.Data.(events.StepOutputData)
		if strings.Contains(data.Line, "mooncake:") && strings.Contains(data.Line, "truncated") {
			foundMarker = true
			break
		}
	}
	if !foundMarker {
		t.Error("expected synthetic stderr event marking truncation")
	}
	if !strings.Contains(buf.String(), "mooncake:") {
		t.Error("expected truncation marker in captured buffer")
	}
}
