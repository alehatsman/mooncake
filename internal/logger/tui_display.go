package logger

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// TUIDisplay handles screen rendering for the animated TUI.
type TUIDisplay struct {
	animator *AnimationFrames
	buffer   *TUIBuffer
	width    int
	height   int
}

// NewTUIDisplay creates a new TUI display renderer.
func NewTUIDisplay(animator *AnimationFrames, buffer *TUIBuffer, width, height int) *TUIDisplay {
	return &TUIDisplay{
		animator: animator,
		buffer:   buffer,
		width:    width,
		height:   height,
	}
}

// Render generates the complete screen output.
func (d *TUIDisplay) Render() string {
	var output strings.Builder

	// Clear screen and move to home position
	output.WriteString("\033[2J\033[H")

	snapshot := d.buffer.GetSnapshot()

	// Render header with animated mooncake
	output.WriteString(d.renderHeader())
	output.WriteString("\n")

	// Render separator
	output.WriteString(d.renderSeparator())
	output.WriteString("\n")

	// Render current step and progress
	output.WriteString(d.renderCurrentStep(snapshot))
	output.WriteString("\n")

	// Render separator
	output.WriteString(d.renderSeparator())
	output.WriteString("\n")

	// Render recent steps history
	output.WriteString(d.renderHistory(snapshot))
	output.WriteString("\n")

	// Render separator
	output.WriteString(d.renderSeparator())
	output.WriteString("\n")

	// Render debug/error messages
	output.WriteString(d.renderMessages(snapshot))
	output.WriteString("\n")

	// Render completion stats if available
	if snapshot.Completion != nil {
		output.WriteString(d.renderSeparator())
		output.WriteString("\n")
		output.WriteString(d.renderCompletion(snapshot.Completion))
		output.WriteString("\n")
	}

	return output.String()
}

// renderHeader renders the animated mooncake character header
func (d *TUIDisplay) renderHeader() string {
	var output strings.Builder

	frame := d.animator.Current()
	cyan := color.New(color.FgCyan)

	// Render each line of the animation frame
	for i, line := range frame {
		if i == 1 {
			output.WriteString(cyan.Sprintf("%s   Mooncake Provisioning Tool\n", line))
		} else {
			output.WriteString(cyan.Sprintf("%s\n", line))
		}
	}

	return output.String()
}

// renderSeparator renders a horizontal separator line
func (d *TUIDisplay) renderSeparator() string {
	return strings.Repeat("─", d.width)
}

// renderCurrentStep renders the current step and progress bar
func (d *TUIDisplay) renderCurrentStep(snapshot BufferSnapshot) string {
	var output strings.Builder

	// Current step
	if snapshot.CurrentStep != "" {
		currentLine := fmt.Sprintf("Current: %s", snapshot.CurrentStep)
		output.WriteString(d.truncate(currentLine, d.width))
		output.WriteString("\n")
	} else {
		output.WriteString("Current: Initializing...\n")
	}

	// Progress bar
	if snapshot.Progress.Total > 0 || snapshot.Progress.Current > 0 {
		progressLine := d.renderProgressBar(snapshot.Progress.Current, snapshot.Progress.Total)
		output.WriteString(progressLine)
		output.WriteString("\n")
	}

	return output.String()
}

// renderProgressBar renders a progress bar
func (d *TUIDisplay) renderProgressBar(current, total int) string {
	// If total is 0, show cumulative steps completed (global progress)
	if total == 0 {
		return fmt.Sprintf("Progress: %d steps completed", current)
	}

	barWidth := 30
	percentage := 0
	if total > 0 {
		percentage = (current * 100) / total
	}

	filled := (current * barWidth) / total
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	return fmt.Sprintf("Progress: [%s] %d%% (%d/%d)", bar, percentage, current, total)
}

// renderHistory renders recent steps from history
func (d *TUIDisplay) renderHistory(snapshot BufferSnapshot) string {
	var output strings.Builder

	output.WriteString(color.CyanString("Recent Steps:") + "\n")

	if len(snapshot.StepHistory) == 0 {
		output.WriteString("  No steps completed yet\n")
		return output.String()
	}

	// Show last N steps (up to 10)
	maxSteps := 10
	startIdx := 0
	if len(snapshot.StepHistory) > maxSteps {
		startIdx = len(snapshot.StepHistory) - maxSteps
	}

	for _, step := range snapshot.StepHistory[startIdx:] {
		indent := strings.Repeat("  ", step.Level+1)
		status := d.getStatusIndicator(step.Status)
		line := fmt.Sprintf("%s%s %s", indent, status, step.Name)
		output.WriteString(d.truncate(line, d.width))
		output.WriteString("\n")
	}

	return output.String()
}

// renderMessages renders debug and error messages
func (d *TUIDisplay) renderMessages(snapshot BufferSnapshot) string {
	var output strings.Builder

	hasMessages := len(snapshot.DebugMessages) > 0 || len(snapshot.ErrorMessages) > 0

	if !hasMessages {
		return ""
	}

	output.WriteString(color.CyanString("Messages:") + "\n")

	// Render error messages first (more important)
	for _, msg := range snapshot.ErrorMessages {
		line := color.RedString("[ERROR] ") + msg
		output.WriteString(d.truncate(line, d.width))
		output.WriteString("\n")
	}

	// Render debug messages
	for _, msg := range snapshot.DebugMessages {
		line := color.YellowString("[DEBUG] ") + msg
		output.WriteString(d.truncate(line, d.width))
		output.WriteString("\n")
	}

	return output.String()
}

// getStatusIndicator returns a colored status indicator
func (d *TUIDisplay) getStatusIndicator(status string) string {
	switch status {
	case StatusSuccess:
		return color.GreenString("✓")
	case StatusError:
		return color.RedString("✗")
	case StatusSkipped:
		return color.YellowString("⊘")
	case StatusRunning:
		return color.CyanString("⊙")
	default:
		return "•"
	}
}

// truncate truncates a string to fit within the specified width
func (d *TUIDisplay) truncate(s string, maxWidth int) string {
	// Account for ANSI color codes by checking visible length
	// For simplicity, we'll use a basic truncation
	// TODO: Improve to handle ANSI codes properly
	if len(s) <= maxWidth {
		return s
	}

	// Leave room for ellipsis
	if maxWidth < 4 {
		return s[:maxWidth]
	}

	return s[:maxWidth-3] + "..."
}

// renderCompletion renders execution completion statistics
func (d *TUIDisplay) renderCompletion(stats *ExecutionStats) string {
	var output strings.Builder

	if stats.Failed > 0 {
		output.WriteString(color.RedString("✗ Execution failed") + "\n\n")
	} else {
		output.WriteString(color.GreenString("✓ Execution completed successfully") + "\n\n")
	}

	output.WriteString(fmt.Sprintf("  Executed: %s\n", color.GreenString("%d", stats.Executed)))
	if stats.Skipped > 0 {
		output.WriteString(fmt.Sprintf("  Skipped:  %s\n", color.YellowString("%d", stats.Skipped)))
	}
	if stats.Failed > 0 {
		output.WriteString(fmt.Sprintf("  Failed:   %s\n", color.RedString("%d", stats.Failed)))
	}
	output.WriteString("\n")
	output.WriteString(fmt.Sprintf("  Duration: %s\n", color.CyanString("%v", stats.Duration.Round(10*time.Millisecond))))

	return output.String()
}
