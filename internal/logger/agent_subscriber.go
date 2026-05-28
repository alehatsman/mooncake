package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	internalerrrors "github.com/alehatsman/mooncake/internal/errors"
	"github.com/alehatsman/mooncake/internal/events"
)

// agentEvent is the flat JSONL record written per event.
type agentEvent struct {
	Event         string `json:"event"`
	TS            string `json:"ts"`
	StepID        string `json:"step_id,omitempty"`
	Name          string `json:"name,omitempty"`
	Action        string `json:"action,omitempty"`
	Changed       *bool  `json:"changed,omitempty"`
	DurationMs    *int64 `json:"duration_ms,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Error         string `json:"error,omitempty"`
	ExitCode      *int   `json:"exit_code,omitempty"`
	Stdout        string `json:"stdout,omitempty"`
	Stderr        string `json:"stderr,omitempty"`
	Hint          string `json:"hint,omitempty"`
	SuggestedStep string `json:"suggested_step,omitempty"`
	TotalSteps    *int   `json:"total_steps,omitempty"`
	DryRun        *bool  `json:"dry_run,omitempty"`
	Ok            *int   `json:"ok,omitempty"`
	Skipped       *int   `json:"skipped,omitempty"`
	Failed        *int   `json:"failed,omitempty"`
	Success       *bool  `json:"success,omitempty"`
	// Package summary fields (EventPackageManaged only)
	Manager        string   `json:"manager,omitempty"`
	Installed      []string `json:"installed,omitempty"`
	AlreadyPresent []string `json:"already_present,omitempty"`
	Removed        []string `json:"removed,omitempty"`
}

// AgentSubscriber writes one flat JSON line per lifecycle event to stdout.
// No ANSI, no human framing — designed for programmatic consumption.
type AgentSubscriber struct{}

// NewAgentSubscriber creates a new AgentSubscriber.
func NewAgentSubscriber() *AgentSubscriber {
	return &AgentSubscriber{}
}

// OnEvent handles an incoming event and writes a JSONL record.
func (a *AgentSubscriber) OnEvent(event events.Event) {
	ts := event.Timestamp.UTC().Format(time.RFC3339)

	var rec *agentEvent

	switch event.Type {
	case events.EventRunStarted:
		d, ok := event.Data.(events.RunStartedData)
		if !ok {
			return
		}
		dryRun := d.DryRun
		rec = &agentEvent{
			Event:      "run.started",
			TS:         ts,
			TotalSteps: &d.TotalSteps,
			DryRun:     &dryRun,
		}

	case events.EventStepStarted:
		d, ok := event.Data.(events.StepStartedData)
		if !ok {
			return
		}
		rec = &agentEvent{
			Event:  "step.started",
			TS:     ts,
			StepID: d.StepID,
			Name:   d.Name,
			Action: d.Action,
		}

	case events.EventStepCompleted:
		d, ok := event.Data.(events.StepCompletedData)
		if !ok {
			return
		}
		changed := d.Changed
		rec = &agentEvent{
			Event:      "step.completed",
			TS:         ts,
			StepID:     d.StepID,
			Name:       d.Name,
			Changed:    &changed,
			DurationMs: &d.DurationMs,
		}

	case events.EventStepSkipped:
		d, ok := event.Data.(events.StepSkippedData)
		if !ok {
			return
		}
		rec = &agentEvent{
			Event:  "step.skipped",
			TS:     ts,
			StepID: d.StepID,
			Name:   d.Name,
			Reason: d.Reason,
		}

	case events.EventStepFailed:
		d, ok := event.Data.(events.StepFailedData)
		if !ok {
			return
		}
		exitCode := d.ExitCode
		rec = &agentEvent{
			Event:      "step.failed",
			TS:         ts,
			StepID:     d.StepID,
			Name:       d.Name,
			Action:     d.Action,
			Error:      d.ErrorMessage,
			ExitCode:   &exitCode,
			Stdout:     d.Stdout,
			Stderr:     d.Stderr,
			DurationMs: &d.DurationMs,
		}
		hint := internalerrrors.InferHint(d.Stderr)
		if hint.Text != "" {
			rec.Hint = hint.Text
			rec.SuggestedStep = hint.SuggestedStep
		}

	case events.EventRunCompleted:
		d, ok := event.Data.(events.RunCompletedData)
		if !ok {
			return
		}
		// F6: prefer the first-class OkSteps field; fall back to the
		// subtraction only when the producer is pre-F6.
		ok2 := d.OkSteps
		if ok2 == 0 && d.SuccessSteps > d.ChangedSteps {
			ok2 = d.SuccessSteps - d.ChangedSteps
		}
		success := d.Success
		rec = &agentEvent{
			Event:      "run.completed",
			TS:         ts,
			Ok:         &ok2,
			Changed:    boolPtr(d.ChangedSteps > 0),
			Skipped:    &d.SkippedSteps,
			Failed:     &d.FailedSteps,
			DurationMs: &d.DurationMs,
			Success:    &success,
		}
		if d.ErrorMessage != "" {
			rec.Error = d.ErrorMessage
		}

	case events.EventPackageManaged:
		d, ok := event.Data.(events.PackageManagedData)
		if !ok {
			return
		}
		rec = &agentEvent{
			Event:          "package.managed",
			TS:             ts,
			Action:         "pkg",
			Manager:        d.Manager,
			Installed:      d.Installed,
			AlreadyPresent: d.AlreadyPresent,
			Removed:        d.Removed,
		}

	default:
		return
	}

	if err := writeJSONL(rec); err != nil {
		fmt.Fprintf(os.Stderr, "agent subscriber: write error: %v\n", err)
	}
}

// Close implements the Subscriber interface.
func (a *AgentSubscriber) Close() {}

func writeJSONL(v interface{}) error {
	return json.NewEncoder(os.Stdout).Encode(v)
}

func boolPtr(b bool) *bool { return &b }
