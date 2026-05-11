package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/fatih/color"
)

// ConsoleSubscriber implements event-based console logging
type ConsoleSubscriber struct {
	logLevel  int
	logFormat string // "text" or "json"
	redactor  interface {
		Redact(string) string
	}
	mu sync.Mutex
}

// NewConsoleSubscriber creates a new console subscriber
func NewConsoleSubscriber(logLevel int, logFormat string) *ConsoleSubscriber {
	return &ConsoleSubscriber{
		logLevel:  logLevel,
		logFormat: logFormat,
	}
}

// SetRedactor sets the redactor for sensitive data
func (c *ConsoleSubscriber) SetRedactor(r interface{ Redact(string) string }) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.redactor = r
}

// OnEvent handles incoming events
func (c *ConsoleSubscriber) OnEvent(event events.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.logFormat == "json" {
		c.renderJSON(event)
		return
	}

	c.renderText(event)
}

// Close implements the Subscriber interface
func (c *ConsoleSubscriber) Close() {
	// Nothing to clean up
}

// renderJSON outputs the event as JSON
func (c *ConsoleSubscriber) renderJSON(event events.Event) {
	if err := json.NewEncoder(os.Stdout).Encode(event); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding event to JSON: %v\n", err)
	}
}

// renderText renders the event as human-readable text
func (c *ConsoleSubscriber) renderText(event events.Event) {
	switch event.Type {
	case events.EventStepStarted:
		if data, ok := event.Data.(events.StepStartedData); ok {
			c.renderStepStarted(data)
		}

	case events.EventStepCompleted:
		if data, ok := event.Data.(events.StepCompletedData); ok {
			c.renderStepCompleted(data)
		}

	case events.EventStepFailed:
		if data, ok := event.Data.(events.StepFailedData); ok {
			c.renderStepFailed(data)
		}

	case events.EventStepSkipped:
		if data, ok := event.Data.(events.StepSkippedData); ok {
			c.renderStepSkipped(data)
		}

	case events.EventRunCompleted:
		if data, ok := event.Data.(events.RunCompletedData); ok {
			c.renderRunCompleted(data)
		}

	case events.EventStepStdout, events.EventStepStderr:
		// Output events are shown via debug logging, not in console subscriber
		// to avoid duplication
		return

	default:
		// Other events are not displayed in console mode
		return
	}
}

// renderStepStarted renders a step.started event
func (c *ConsoleSubscriber) renderStepStarted(data events.StepStartedData) {
	// Check if this is a directory header (ends with /)
	if strings.HasSuffix(data.Name, "/") {
		// Don't show started event for directories, only when they're skipped
		return
	}

	// Calculate indentation: base level + directory depth
	indent := strings.Repeat("  ", data.Level+data.Depth)
	icon := color.CyanString("▶")
	fmt.Printf("%s%s %s\n", indent, icon, data.Name)
}

// renderStepCompleted renders a step.completed event
func (c *ConsoleSubscriber) renderStepCompleted(data events.StepCompletedData) {
	// Check if this is a directory (ends with /)
	if strings.HasSuffix(data.Name, "/") {
		// Don't show completed event for directories
		return
	}

	indent := strings.Repeat("  ", data.Level+data.Depth)
	icon := color.GreenString("✓")
	fmt.Printf("%s%s %s\n", indent, icon, data.Name)
}

// renderStepFailed renders a step.failed event
func (c *ConsoleSubscriber) renderStepFailed(data events.StepFailedData) {
	indent := strings.Repeat("  ", data.Level+data.Depth)
	icon := color.RedString("✗")
	fmt.Printf("%s%s %s\n", indent, icon, data.Name)

	// Show error message indented
	errorIndent := indent + "  "
	fmt.Printf("%s%s\n", errorIndent, color.RedString(data.ErrorMessage))
}

// renderStepSkipped renders a step.skipped event
func (c *ConsoleSubscriber) renderStepSkipped(data events.StepSkippedData) {
	// Check if this is a directory (ends with /)
	if strings.HasSuffix(data.Name, "/") {
		dirName := strings.TrimSuffix(data.Name, "/")
		dirDepth := strings.Count(dirName, "/")

		// Skip showing the root directory (templates/)
		if dirDepth == 0 && dirName != "" {
			return
		}

		// For subdirectories (after/, ftplugin/), show as headers
		indent := strings.Repeat("  ", data.Level)
		fmt.Printf("%s%s\n", indent, color.New(color.Faint).Sprint(data.Name))
		return
	}

	indent := strings.Repeat("  ", data.Level+data.Depth)
	icon := color.New(color.Faint).Sprint("─")
	reasonText := ""
	if data.Reason != "" {
		reasonText = color.New(color.Faint).Sprintf(" [%s]", data.Reason)
	}
	fmt.Printf("%s%s %s%s\n", indent, icon, color.New(color.Faint).Sprint(data.Name), reasonText)
}

// renderRunCompleted renders a run.completed event as a single compact recap line.
func (c *ConsoleSubscriber) renderRunCompleted(data events.RunCompletedData) {
	ok := data.SuccessSteps - data.ChangedSteps
	if ok < 0 {
		ok = 0
	}

	line := fmt.Sprintf("RECAP  ok=%d  changed=%d  skipped=%d  failed=%d  %s",
		ok, data.ChangedSteps, data.SkippedSteps, data.FailedSteps,
		formatDuration(data.DurationMs))

	if !data.Success && data.ErrorMessage != "" {
		line += "  " + color.RedString("✗ %s", data.ErrorMessage)
	}

	fmt.Println()
	if data.Success {
		fmt.Println(color.GreenString(line))
	} else {
		fmt.Println(color.RedString(line))
	}
}

// formatDuration converts milliseconds to a human-readable duration string.
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	s := ms / 1000
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	s %= 60
	if m < 60 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := m / 60
	m %= 60
	return fmt.Sprintf("%dh%dm%ds", h, m, s)
}
