// Package print implements the print action handler.
//
// The print action displays messages to the user during execution.
// It supports template rendering and is useful for debugging and showing information.
package print

import (
	"fmt"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Handler implements the Handler interface for print actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the print action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "log",
		Description:        "Display messages to the user",
		Category:           actions.CategoryOutput,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        []string{string(events.EventPrintMessage)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

// Validate checks if the print configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.Log == nil {
		return fmt.Errorf("print configuration is nil")
	}

	if step.Log.Msg == "" {
		return fmt.Errorf("print message is empty")
	}

	return nil
}

// Execute runs the print action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	printAction := step.Log

	// Create result
	result := executor.NewResult()
	result.Changed = false

	// Render the message template
	renderedMsg, err := ctx.GetTemplate().Render(printAction.Msg, ctx.GetVariables())
	if err != nil {
		result.Failed = true
		result.Stderr = err.Error()
		return result, fmt.Errorf("failed to render message: %w", err)
	}

	// Emit print event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type:      events.EventPrintMessage,
			Timestamp: time.Now(),
			Data: events.PrintData{
				Message: renderedMsg,
			},
		})
	}

	// Store the message in stdout for the result
	result.Stdout = renderedMsg

	return result, nil
}

// DryRun logs what would be printed without actually printing.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	printAction := step.Log

	// Attempt to render the message (but don't fail if it errors)
	renderedMsg, err := ctx.GetTemplate().Render(printAction.Msg, ctx.GetVariables())
	if err != nil {
		renderedMsg = printAction.Msg + " (template render would fail)"
	}

	// Log what would be printed
	ctx.GetLogger().Infof("  [DRY-RUN] Would print: %s", renderedMsg)

	return nil
}

// Run is the Spec 16 entry point. print doesn't mutate system state.
// Plan mode renders the message and surfaces the first line so the
// preview shows what would be printed. Execute mode delegates.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true

		rendered, err := ctx.GetTemplate().Render(step.Log.Msg, ctx.GetVariables())
		if err != nil {
			rendered = step.Log.Msg
		}
		// Single-line preview: take first line, trim, truncate.
		preview := rendered
		if i := strings.IndexByte(preview, '\n'); i >= 0 {
			preview = preview[:i]
		}
		preview = strings.TrimSpace(preview)
		if len(preview) > 80 {
			preview = preview[:77] + "..."
		}
		if preview == "" {
			r.Reason = "would print message"
		} else {
			r.Reason = "would print: " + preview
		}
		return r, nil
	}
	return h.Execute(ctx, step)
}
