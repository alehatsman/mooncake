package pilot

import (
	"io"
	"os"
)

// consoleLogFormat maps a pilot RunOptions.OutputFormat onto the
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
