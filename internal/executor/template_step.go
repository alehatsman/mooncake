package executor

import (
	"fmt"
	"io"
	"os"

	"github.com/alehatsman/mooncake/internal/config"
)

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

	// Create result object
	result := NewResult()
	result.Changed = false

	// Check for dry-run mode
	if ec.DryRun {
		// Check if source file exists
		if _, err = os.Stat(src); os.IsNotExist(err) {
			ec.Logger.Errorf("  [DRY-RUN] Template source file does not exist: %s", src)
			return fmt.Errorf("template source file not found: %s", src)
		}
		mode := parseFileMode(template.Mode, 0644)
		dryRun := newDryRunLogger(ec.Logger)
		dryRun.LogTemplateRender(src, dest, mode)
		if template.Vars != nil && len(*template.Vars) > 0 {
			ec.Logger.Debugf("  Additional variables: %v", *template.Vars)
		}
		dryRun.LogRegister(step)
		return nil
	}

	// #nosec G304 -- Template source path from user config is intentional functionality
	templateFile, err := os.Open(src)
	if err != nil {
		markStepFailed(result, step, ec)
		return err
	}
	defer func() {
		if err = templateFile.Close(); err != nil {
			ec.Logger.Errorf("failed to close template file %s: %v", src, err)
		}
	}()

	templateBytes, err := io.ReadAll(templateFile)
	if err != nil {
		markStepFailed(result, step, ec)
		return err
	}

	variables := ec.Variables
	if template.Vars != nil {
		variables = mergeVariables(ec.Variables, *template.Vars)
	}

	output, err := ec.Template.Render(string(templateBytes), variables)
	if err != nil {
		markStepFailed(result, step, ec)
		return err
	}

	// Check if content would change
	// #nosec G304 -- Template destination path from user config is intentional functionality
	existingContent, err := os.ReadFile(dest)
	if err != nil || string(existingContent) != output {
		result.Changed = true
	}

	mode := parseFileMode(template.Mode, 0644)
	if err := os.WriteFile(dest, []byte(output), mode); err != nil {
		markStepFailed(result, step, ec)
		return fmt.Errorf("failed to write template output to %s: %w", dest, err)
	}

	// Register the result if register is specified
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
		ec.Logger.Debugf("  Registered result as: %s (changed=%v)", step.Register, result.Changed)
	}

	return nil
}
