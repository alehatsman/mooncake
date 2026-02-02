package executor

import (
	"fmt"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
)

// HandlePrint handles print actions which display messages to the user.
// Print actions are useful for debugging and showing information during execution.
func HandlePrint(step config.Step, ec *ExecutionContext) error {
	printAction := step.Print
	if printAction == nil {
		return &StepValidationError{
			Field:   "print",
			Message: "print configuration is nil",
		}
	}

	// Create result object with start time
	result := NewResult()
	result.StartTime = time.Now()
	result.Changed = false // Print actions never report "changed"

	// Finalize timing when function returns
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Render the message template
	renderedMsg, err := ec.Template.Render(printAction.Msg, ec.Variables)
	if err != nil {
		result.Failed = true
		result.Stderr = err.Error()
		// Register result if requested
		if step.Register != "" {
			result.RegisterTo(ec.Variables, step.Register)
		}
		ec.CurrentResult = result
		return &RenderError{
			Field: "msg",
			Cause: err,
		}
	}

	// Handle dry-run mode
	if ec.DryRun {
		ec.HandleDryRun(func(dryRun *dryRunLogger) {
			dryRun.LogPrintMessage(renderedMsg)
			dryRun.LogRegister(step)
		})
		result.Stdout = fmt.Sprintf("would print: %s", renderedMsg)

		// Register result if requested
		if step.Register != "" {
			result.RegisterTo(ec.Variables, step.Register)
		}

		// Set result in context
		ec.CurrentResult = result
		return nil
	}

	// Emit print event with the message
	if ec.EventPublisher != nil {
		ec.EventPublisher.Publish(events.Event{
			Type: events.EventPrintMessage,
			Data: events.PrintData{
				Message: renderedMsg,
			},
		})
	}

	// Store the message in stdout for the result
	result.Stdout = renderedMsg

	// Register result if requested
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
	}

	// Set result in context
	ec.CurrentResult = result

	return nil
}
