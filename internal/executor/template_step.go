package executor

import (
	"io"
	"os"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/template"
	"github.com/alehatsman/mooncake/internal/utils"
)

// readAndRenderTemplate reads a template file and renders it with the given variables.
// Returns the rendered output or an error if reading/rendering fails.
func readAndRenderTemplate(src string, renderer template.Renderer, variables map[string]interface{}, ec *ExecutionContext) (string, error) {
	// #nosec G304 -- Template source path from user config is intentional functionality
	srcFile, err := os.Open(src)
	if err != nil {
		return "", &FileOperationError{Operation: "read", Path: src, Cause: err}
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			ec.Logger.Debugf("failed to close template file %s: %v", src, closeErr)
		}
	}()

	srcBytes, err := io.ReadAll(srcFile)
	if err != nil {
		return "", &FileOperationError{Operation: "read", Path: src, Cause: err}
	}

	output, err := renderer.Render(string(srcBytes), variables)
	if err != nil {
		return "", &RenderError{Field: "template file content", Cause: err}
	}
	return output, nil
}

// logTemplateComparison compares rendered output with existing file and logs appropriate message.
func logTemplateComparison(dryRun *dryRunLogger, src, dest string, mode os.FileMode, output string) {
	// #nosec G304 -- Template destination path from user config is intentional functionality
	existingContent, _ := os.ReadFile(dest)
	if existingContent != nil {
		if string(existingContent) != output {
			dryRun.LogTemplateUpdate(src, dest, mode, len(existingContent), len(output))
		} else {
			dryRun.LogTemplateNoChange(src, dest)
		}
	} else {
		dryRun.LogTemplateCreate(src, dest, mode, len(output))
	}
}

// logDryRunTemplateOperation attempts to render template and log appropriate message.
func logDryRunTemplateOperation(dryRun *dryRunLogger, src, dest string, mode os.FileMode, renderer template.Renderer, variables map[string]interface{}, ec *ExecutionContext) {
	output, err := readAndRenderTemplate(src, renderer, variables, ec)
	if err != nil {
		// Can't read or render - use basic logging
		dryRun.LogTemplateRender(src, dest, mode)
		if err != nil {
			ec.Logger.Debugf("  Template render error (dry-run): %v", err)
		}
		return
	}

	logTemplateComparison(dryRun, src, dest, mode, output)
}

// HandleTemplate renders a template file and writes it to a destination.
func HandleTemplate(step config.Step, ec *ExecutionContext) error {
	template := step.Template

	src, err := ec.PathUtil.ExpandPath(template.Src, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	dest, err := ec.PathUtil.ExpandPath(template.Dest, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	// Step is already logged via LogStep in executor, no need to log again
	ec.Logger.Debugf("Templating src=\"%s\" dest=\"%s\"", src, dest)

	// Create result object with start time
	result := NewResult()
	result.StartTime = time.Now()
	result.Changed = false

	// Finalize timing when function returns
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Check for dry-run mode
	if ec.DryRun {
		// Check if source file exists
		if _, err = os.Stat(src); os.IsNotExist(err) {
			ec.Logger.Errorf("  [DRY-RUN] Template source file does not exist: %s", src)
			return &FileOperationError{Operation: "read", Path: src, Cause: err}
		}

		// Prepare variables for rendering
		variables := ec.Variables
		if template.Vars != nil {
			variables = utils.MergeVariables(ec.Variables, *template.Vars)
		}

		mode := parseFileMode(template.Mode, 0644)
		ec.HandleDryRun(func(dryRun *dryRunLogger) {
			// Attempt to render and log detailed status
			logDryRunTemplateOperation(dryRun, src, dest, mode, ec.Template, variables, ec)

			if template.Vars != nil && len(*template.Vars) > 0 {
				ec.Logger.Debugf("  Additional variables: %v", *template.Vars)
			}
			dryRun.LogRegister(step)
		})
		return nil
	}

	// #nosec G304 -- Template source path from user config is intentional functionality
	templateFile, err := os.Open(src)
	if err != nil {
		markStepFailed(result, step, ec)
		return &FileOperationError{Operation: "read", Path: src, Cause: err}
	}
	defer func() {
		if err = templateFile.Close(); err != nil {
			ec.Logger.Errorf("failed to close template file %s: %v", src, err)
		}
	}()

	templateBytes, err := io.ReadAll(templateFile)
	if err != nil {
		markStepFailed(result, step, ec)
		return &FileOperationError{Operation: "read", Path: src, Cause: err}
	}

	variables := ec.Variables
	if template.Vars != nil {
		variables = utils.MergeVariables(ec.Variables, *template.Vars)
	}

	output, err := ec.Template.Render(string(templateBytes), variables)
	if err != nil {
		markStepFailed(result, step, ec)
		return &RenderError{Field: "template file content", Cause: err}
	}

	// Check if content would change
	// #nosec G304 -- Template destination path from user config is intentional functionality
	existingContent, err := os.ReadFile(dest)
	if err != nil || string(existingContent) != output {
		result.Changed = true
	}

	mode := parseFileMode(template.Mode, 0644)
	if err := createFileWithBecome(dest, []byte(output), mode, step, ec); err != nil {
		markStepFailed(result, step, ec)
		return &FileOperationError{Operation: "write", Path: dest, Cause: err}
	}

	// Emit template.rendered event
	ec.EmitEvent(events.EventTemplateRender, events.TemplateRenderData{
		TemplatePath: src,
		DestPath:     dest,
		SizeBytes:    int64(len(output)),
		Changed:      result.Changed,
		DryRun:       ec.DryRun,
	})

	// Register the result if register is specified
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
		ec.Logger.Debugf("  Registered result as: %s (changed=%v)", step.Register, result.Changed)
	}

	// Set result in context for event emission
	ec.CurrentResult = result

	return nil
}
