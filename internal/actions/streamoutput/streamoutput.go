// Package streamoutput drains a subprocess pipe line-by-line,
// emitting step.stdout / step.stderr events as each line arrives
// and (optionally) capturing the same bytes into a buffer for the
// final result envelope.
//
// Extracted from internal/actions/shell so both shell: and cmd:
// can emit the same per-line stream events. Without this, cmd:
// buffered everything via bytes.Buffer and produced zero live
// output — which made `mooncake apply` look frozen when the
// step printed progress.
//
// Inherits two prior fixes from the shell version:
//
//   - F018: bufio.Scanner's default 64 KB cap silently truncates
//     long lines. We bump the per-line cap to 1 MB and surface
//     truncation through both the logger and a synthetic stderr
//     event so consumers see why a step's tail is missing.
//   - F038: when the scanner gives up, keep draining the pipe so
//     the child process doesn't block on write — otherwise
//     command.Wait() hangs and the step leaks.
package streamoutput

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/events"
)

// MaxLineBytes is the per-line cap. 1 MB is generous for human-
// readable output and small enough that a runaway command can't
// OOM the daemon. Binary blobs > 1 MB should be redirected to a
// file by the playbook, not captured.
const MaxLineBytes = 1024 * 1024

// Stream reads pipe line-by-line, optionally writing each line
// (plus a trailing newline) into buf, and publishes a step.stdout
// or step.stderr event per line via ctx.EventPublisher().
//
// stream must be "stdout" or "stderr"; it selects the event type
// and tags the StepOutputData.
//
// Synchronous: callers wire stdout and stderr drains into
// goroutines and Wait on a sync.WaitGroup. Both drains must
// complete before command.Wait() returns or the child can hang
// on a full pipe buffer.
func Stream(pipe io.Reader, buf *bytes.Buffer, ctx actions.Context, capture bool, stream string) {
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 64*1024), MaxLineBytes)
	lineNum := 0

	publisher := ctx.EventPublisher()
	stepID := ctx.StepID()

	eventType := events.EventStepStdout
	if stream == "stderr" {
		eventType = events.EventStepStderr
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if capture {
			buf.WriteString(line)
			buf.WriteString("\n")
		}

		if publisher != nil {
			publisher.Publish(events.Event{
				Type:      eventType,
				Timestamp: time.Now(),
				Data: events.StepOutputData{
					StepID:     stepID,
					Stream:     stream,
					Line:       line,
					LineNumber: lineNum,
				},
			})
		}
	}

	if err := scanner.Err(); err != nil {
		if log := ctx.Logger(); log != nil {
			log.Errorf("  %s stream stopped early (output truncated): %v", stream, err)
		}
		// F038: surface truncation through programmatic channels
		// so consumers reading result.Stdout/Stderr or subscribing
		// to step.* events know data was dropped.
		msg := fmt.Sprintf("mooncake: %s stream truncated (line exceeded %d-byte limit): %v", stream, MaxLineBytes, err)
		if capture {
			buf.WriteString(msg)
			buf.WriteString("\n")
		}
		if publisher != nil {
			publisher.Publish(events.Event{
				Type:      events.EventStepStderr,
				Timestamp: time.Now(),
				Data: events.StepOutputData{
					StepID:     stepID,
					Stream:     "stderr",
					Line:       msg,
					LineNumber: lineNum + 1,
				},
			})
		}
		// Drain the rest so the child's write end never blocks on
		// PIPE_BUF-sized kernel buffer. Without this, command.Wait()
		// hangs forever once truncation kicks in.
		_, _ = io.Copy(io.Discard, pipe)
	}
}
