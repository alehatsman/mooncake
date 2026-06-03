package mooncake

import (
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/logger"
)

// ----------------------------------------------------------------------------
// Observability — live in-process event stream (#121)
// ----------------------------------------------------------------------------
//
// A run already emits a structured event for every lifecycle moment (run /
// step / file / transaction). With OutputFormat:"json" those serialize to an
// NDJSON stream on stdout. The types below let a consumer tap the SAME stream
// in-process instead: implement Subscriber, hand it to RunOptions.Subscribers,
// and OnEvent fires synchronously-ish (via the publisher's per-subscriber
// goroutine) for each Event as the run executes — no stdout parsing, no
// out-of-band channel.

// Event is one moment in a run's execution lifecycle: a Type, a Timestamp, and
// a type-erased Data payload a consumer decodes off Type. RunOptions.Subscribers
// receive these live.
type Event = events.Event

// EventType identifies an Event (e.g. EventStepStarted, EventStepCompleted,
// EventStepFailed, EventAgentCompleted). Compare an Event.Type against the
// Event* constants below to dispatch.
type EventType = events.Type

// Subscriber is the in-process tap a consumer implements to observe a run's
// live event stream. Set RunOptions.Subscribers to attach one (or more); the
// run's publisher calls OnEvent for every Event it emits. Close is invoked by
// the caller, not the run — the run does not own a caller-supplied
// subscriber's lifecycle, so make Close idempotent.
type Subscriber = events.Subscriber

// The Event type constants a Subscriber dispatches on. This is the subset most
// relevant to an SDK consumer driving an agent run: the per-step lifecycle, the
// run brackets, and the agent terminal event. The full set lives on
// events.Type; these are re-exported so a facade-only consumer can name the
// common ones without importing internal/events.
const (
	// Run + plan lifecycle.
	EventRunStarted     = events.EventRunStarted
	EventRunCompleted   = events.EventRunCompleted
	EventPlanLoaded     = events.EventPlanLoaded
	EventAgentCompleted = events.EventAgentCompleted

	// Step lifecycle.
	EventStepStarted   = events.EventStepStarted
	EventStepCompleted = events.EventStepCompleted
	EventStepSkipped   = events.EventStepSkipped
	EventStepFailed    = events.EventStepFailed
	EventStepChecked   = events.EventStepChecked

	// Step output streaming.
	EventStepStdout = events.EventStepStdout
	EventStepStderr = events.EventStepStderr
)

// Event Data payload types a Subscriber reads off Event.Data after switching on
// Event.Type. Re-exported so a facade-only consumer can type-assert the payload
// without importing internal/events.
type (
	// RunStartedData is the Data of EventRunStarted.
	RunStartedData = events.RunStartedData
	// RunCompletedData is the Data of EventRunCompleted (step counts,
	// duration, success).
	RunCompletedData = events.RunCompletedData
	// PlanLoadedData is the Data of EventPlanLoaded (root file, step list).
	PlanLoadedData = events.PlanLoadedData
	// StepStartedData is the Data of EventStepStarted.
	StepStartedData = events.StepStartedData
	// StepCompletedData is the Data of EventStepCompleted (changed flag,
	// duration, typed result).
	StepCompletedData = events.StepCompletedData
	// StepSkippedData is the Data of EventStepSkipped.
	StepSkippedData = events.StepSkippedData
	// StepFailedData is the Data of EventStepFailed (error message, exit
	// code, stderr).
	StepFailedData = events.StepFailedData
	// StepOutputData is the Data of EventStepStdout / EventStepStderr.
	StepOutputData = events.StepOutputData
)

// ----------------------------------------------------------------------------
// Observability — internal logger override (#121)
// ----------------------------------------------------------------------------

// Logger is the logging contract the run's executor logs error/diagnostic
// output through. Set RunOptions.Logger to route that output into your own
// surface (or NewDiscardLogger to silence it); the event stream above is
// separate and unaffected. Construct one with NewLogger.
type Logger = logger.Logger

// Log levels for NewLogger and RunOptions.LoggerLevel.
const (
	LogLevelDebug = logger.DebugLevel
	LogLevelInfo  = logger.InfoLevel
	LogLevelError = logger.ErrorLevel
)

// NewLogger returns the built-in console logger at the given level (one of the
// LogLevel* constants), suitable for RunOptions.Logger.
func NewLogger(level int) Logger {
	return logger.NewLogger(level)
}

// NewDiscardLogger returns a Logger that drops every message — hand it to
// RunOptions.Logger to silence the run's internal logging entirely (the event
// stream still flows to Subscribers).
func NewDiscardLogger() Logger {
	return logger.NewDiscardLogger()
}
