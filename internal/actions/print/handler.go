// Package print implements the print action handler.
//
// The print action displays messages to the user during execution.
// It supports template rendering and is useful for debugging and showing information.
package print

import (
	"fmt"
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
		Name:               "print",
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
	if step.Print == nil {
		return fmt.Errorf("print configuration is nil")
	}

	if step.Print.Msg == "" {
		return fmt.Errorf("print message is empty")
	}

	return nil
}

// Execute runs the print action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	printAction := step.Print

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
	printAction := step.Print

	// Attempt to render the message (but don't fail if it errors)
	renderedMsg, err := ctx.GetTemplate().Render(printAction.Msg, ctx.GetVariables())
	if err != nil {
		renderedMsg = printAction.Msg + " (template render would fail)"
	}

	// Log what would be printed
	ctx.GetLogger().Infof("  [DRY-RUN] Would print: %s", renderedMsg)

	return nil
}
