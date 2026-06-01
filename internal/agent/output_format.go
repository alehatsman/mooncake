package agent

import (
	"io"
	"os"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/logger"
)

// consoleLogFormat maps a agent RunOptions.OutputFormat onto the
// ConsoleSubscriber's logFormat argument. "json" yields the
// machine-readable NDJSON event stream (renderJSON: one events.Event per
// line); anything else is the human-readable text rendering. Mirrors the
// text/json split apply threads into NewConsoleSubscriber.
func consoleLogFormat(outputFormat string) string {
	if outputFormat == OutputFormatJSON {
		return "json"
	}
	return "text"
}

// captureWriter picks where stdoutCapture prints buffered cmd-action
// stdout. In JSON mode it MUST be nil: the capture prints raw command
// output to os.Stdout at step.completed (output_capture.go), and that's
// the same stream the NDJSON events go to — interleaving non-JSON bytes
// would corrupt the stream for a programmatic consumer. The capture
// still records lastStdout for the loop's next-prompt feedback either
// way (a nil writer only disables the print, not the capture).
func captureWriter(outputFormat string) io.Writer {
	if outputFormat == OutputFormatJSON {
		return nil
	}
	return os.Stdout
}

// executorLogger picks the logger handed to executor.Start for a agent
// iteration's apply. In JSON mode it MUST be a no-op logger: the executor
// logs step failures (and rollback errors) through ec.Svc.Logger.Errorf,
// and a ConsoleLogger writes that prose to stdout (fatih/color), injecting
// bare non-JSON lines like "command failed with exit code 1" into the NDJSON
// event stream (#63). The structured failure already rides the step.failed
// event, so the prose is redundant for a machine consumer.
func executorLogger(outputFormat string) logger.Logger {
	if outputFormat == OutputFormatJSON {
		return logger.NewDiscardLogger()
	}
	return logger.NewLogger(logger.ErrorLevel)
}

// emitLoopEvent publishes a single agent-loop lifecycle event (e.g.
// plan.generating, #74) to the same sink the executor's ConsoleSubscriber
// writes to: an NDJSON line on stdout under --output-format json, and
// nothing in text mode — renderText has no case for these types, so
// they're a no-op there by design (the bracket exists for the structured
// stream; the human already watches the planning latency live).
//
// A short-lived synchronous publisher keeps this fully isolated from the
// per-iteration execution publisher's close/flush lifecycle (cf. the
// MT-53 events-drop-on-close note): the plan phase and the apply phase
// never overlap, so there's no interleaving on stdout. SyncPublisher
// delivers before returning, so the event lands before the buffered
// GeneratePlan call blocks.
func emitLoopEvent(outputFormat string, evt events.Event) {
	pub := events.NewSyncPublisher()
	pub.Subscribe(logger.NewConsoleSubscriber(logger.InfoLevel, consoleLogFormat(outputFormat), false))
	pub.Publish(evt)
	pub.Close()
}
