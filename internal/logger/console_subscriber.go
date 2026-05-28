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
	// streamOutput is the opt-in for rendering captured step stdout/
	// stderr lines into the console output, INDEPENDENT of logLevel.
	// `mooncake task` sets this true so dev-loop commands ('go build',
	// 'go test', linters) stream their output by default — without
	// also flipping logLevel to debug, which would drag in internal
	// variable dumps and other developer-noise log lines.
	//
	// Backward-compat: logLevel <= DebugLevel also enables streaming,
	// so `mooncake apply --log-level debug` still firehoses.
	streamOutput bool
	redactor     interface {
		Redact(string) string
	}
	mu sync.Mutex
}

// NewConsoleSubscriber creates a new console subscriber.
// streamOutput=false matches the historical `mooncake apply` UX
// (step-start/end markers only at info; shell stdout/stderr only at
// debug). `mooncake task` constructs with streamOutput=true.
func NewConsoleSubscriber(logLevel int, logFormat string, streamOutput bool) *ConsoleSubscriber {
	return &ConsoleSubscriber{
		logLevel:     logLevel,
		logFormat:    logFormat,
		streamOutput: streamOutput,
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

	case events.EventStepChecked:
		if data, ok := event.Data.(events.StepCheckedData); ok {
			c.renderStepChecked(data)
		}

	case events.EventPackageManaged:
		if data, ok := event.Data.(events.PackageManagedData); ok {
			c.renderPackageManaged(data)
		}

	case events.EventStepStdout, events.EventStepStderr:
		// Two independent gates: the explicit streamOutput opt-in (used
		// by `mooncake task` so dev-loop shell steps stream by default)
		// and the legacy -l debug path (so power users running
		// `mooncake apply --log-level debug` still get the firehose).
		// Either one enables rendering; both off keeps the default UX
		// uncluttered.
		if c.streamOutput || c.logLevel <= DebugLevel {
			if data, ok := event.Data.(events.StepOutputData); ok {
				c.renderStepOutput(data, event.Type)
			}
		}
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
	var icon string
	if data.Changed {
		icon = color.YellowString("~")
	} else {
		icon = color.GreenString("✓")
	}

	timing := ""
	if data.DurationMs >= 2000 {
		secs := data.DurationMs / 1000
		if secs < 60 {
			timing = fmt.Sprintf(" [%ds]", secs)
		} else {
			timing = fmt.Sprintf(" [%dm%02ds]", secs/60, secs%60)
		}
	}

	fmt.Printf("%s%s %s%s\n", indent, icon, data.Name, timing)
}

// renderStepOutput prints one line of captured stdout/stderr from a shell
// step. Indented under the step marker and prefixed with a dim "|" so it
// reads as inline output rather than a separate event. Only called at
// debug log level.
func (c *ConsoleSubscriber) renderStepOutput(data events.StepOutputData, et events.Type) {
	prefix := color.New(color.Faint).Sprint("|")
	if et == events.EventStepStderr {
		prefix = color.RedString("|")
	}
	line := data.Line
	if c.redactor != nil {
		line = c.redactor.Redact(line)
	}
	fmt.Printf("  %s %s\n", prefix, line)
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
	icon := color.New(color.Faint).Sprint("-")
	reasonText := ""
	if data.Reason != "" {
		reasonText = color.New(color.Faint).Sprintf(" [%s]", data.Reason)
	}
	fmt.Printf("%s%s %s%s\n", indent, icon, color.New(color.Faint).Sprint(data.Name), reasonText)
}

// renderRunCompleted renders a run.completed event as a single compact recap line.
func (c *ConsoleSubscriber) renderRunCompleted(data events.RunCompletedData) {
	// F6: OkSteps is a first-class field on the event now; read it
	// directly. Fall back to the legacy subtraction only when the
	// producer is pre-F6 (OkSteps==0 but SuccessSteps>ChangedSteps)
	// so external SSE consumers from an older daemon still render
	// a sensible recap.
	ok := data.OkSteps
	if ok == 0 && data.SuccessSteps > data.ChangedSteps {
		ok = data.SuccessSteps - data.ChangedSteps
	}

	var line string
	if data.CheckMode {
		line = fmt.Sprintf("CHECK RECAP  would-change=%d  ok=%d  skipped=%d  %s",
			data.ChangedSteps, ok, data.SkippedSteps,
			formatDuration(data.DurationMs))
	} else {
		// Proposal-02 compact recap: always show the four base buckets;
		// append reverted= / cancelled= only when non-zero so quiet runs
		// stay readable. MT-45 (reverted) and SIGINT/timeout (cancelled)
		// are both diagnostic — operators look for them when something
		// went sideways.
		line = fmt.Sprintf("RECAP  ok=%d  changed=%d  skipped=%d  failed=%d",
			ok, data.ChangedSteps, data.SkippedSteps, data.FailedSteps)
		if data.RevertedSteps > 0 {
			line += fmt.Sprintf("  reverted=%d", data.RevertedSteps)
		}
		if data.CancelledSteps > 0 {
			line += fmt.Sprintf("  cancelled=%d", data.CancelledSteps)
		}
		if data.HealedSteps > 0 {
			line += fmt.Sprintf("  healed=%d", data.HealedSteps)
		}
		line += "  " + formatDuration(data.DurationMs)
	}

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

// renderStepChecked renders a step.checked event in check mode.
func (c *ConsoleSubscriber) renderStepChecked(data events.StepCheckedData) {
	indent := strings.Repeat("  ", data.Level+data.Depth)
	var icon string
	var reasonText string

	switch {
	case !data.Checkable:
		icon = color.New(color.Faint).Sprint("?")
		reasonText = color.New(color.Faint).Sprintf("  [not checkable]")
	case data.WouldChange:
		icon = color.YellowString("↑")
		if data.Reason != "" {
			reasonText = fmt.Sprintf("  [%s]", data.Reason)
		}
	default:
		icon = color.GreenString("✓")
	}

	fmt.Printf("%s%s %s%s\n", indent, icon, data.Name, reasonText)
}

// renderPackageManaged renders a package.managed event as a compact summary line.
func (c *ConsoleSubscriber) renderPackageManaged(data events.PackageManagedData) {
	if len(data.Installed) == 0 && len(data.Removed) == 0 {
		return
	}
	var parts []string
	for _, p := range data.Installed {
		parts = append(parts, "+"+p)
	}
	for _, p := range data.Removed {
		parts = append(parts, "-"+p)
	}
	line := "  package  " + strings.Join(parts, " ")
	if len(data.AlreadyPresent) > 0 {
		line += "  (already: " + strings.Join(data.AlreadyPresent, " ") + ")"
	}
	fmt.Println(line)
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
