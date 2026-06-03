package agent

import (
	"io"
	"os"

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

// resolveLogger picks the logger an agent apply logs through, honoring a
// caller's RunOptions.Logger override (#121). When opts.Logger is non-nil it
// wins over fallback — a caller routes the run's internal error/diagnostic
// logging into its own surface (or silences it with a discard logger) — and
// opts.LoggerLevel (when > 0) is injected via SetLogLevel so the caller can
// dial verbosity without constructing the logger at a fixed level. nil keeps
// the built-in fallback the caller passed (executorLogger / ErrorLevel).
func resolveLogger(opts RunOptions, fallback logger.Logger) logger.Logger {
	if opts.Logger == nil {
		return fallback
	}
	if opts.LoggerLevel > 0 {
		opts.Logger.SetLogLevel(opts.LoggerLevel)
	}
	return opts.Logger
}
