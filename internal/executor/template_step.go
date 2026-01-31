package executor

import (
	"fmt"
	"io"
	"os"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/fatih/color"
)

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

	// Only show detailed templating line when in with_filetree iteration
	if _, inFileTree := ec.Variables["item"]; inFileTree {
		tag := color.CyanString("[%d/%d]", ec.CurrentIndex+1, ec.TotalSteps)
		ec.Logger.Infof("%s templating src=\"%s\" dest=\"%s\"", tag, src, dest)
	}

	// Create result object
	result := NewResult()
	result.Changed = false

	// Check for dry-run mode
	if ec.DryRun {
		// Check if source file exists
		if _, err := os.Stat(src); os.IsNotExist(err) {
			ec.Logger.Errorf("  [DRY-RUN] Template source file does not exist: %s", src)
			return fmt.Errorf("template source file not found: %s", src)
		}
		mode := parseFileMode(template.Mode, 0644)
		ec.Logger.Infof("  [DRY-RUN] Would template: %s -> %s (mode: %04o)", src, dest, mode)
		if template.Vars != nil && len(*template.Vars) > 0 {
			ec.Logger.Debugf("  Additional variables: %v", *template.Vars)
		}
		if step.Register != "" {
			ec.Logger.Debugf("  [DRY-RUN] Would register result as: %s", step.Register)
		}
		return nil
	}

	templateFile, err := os.Open(src)
	if err != nil {
		result.Failed = true
		result.Rc = 1
		if step.Register != "" {
			ec.Variables[step.Register] = result.ToMap()
		}
		return err
	}
	defer templateFile.Close()

	templateBytes, err := io.ReadAll(templateFile)
	if err != nil {
		result.Failed = true
		result.Rc = 1
		if step.Register != "" {
			ec.Variables[step.Register] = result.ToMap()
		}
		return err
	}

	variables := make(map[string]interface{})
	for k, v := range ec.Variables {
		variables[k] = v
	}

	if template.Vars != nil {
		for k, v := range *template.Vars {
			variables[k] = v
		}
	}

	output, err := ec.Template.Render(string(templateBytes), variables)
	if err != nil {
		result.Failed = true
		result.Rc = 1
		if step.Register != "" {
			ec.Variables[step.Register] = result.ToMap()
		}
		return err
	}

	// Check if content would change
	existingContent, err := os.ReadFile(dest)
	if err != nil || string(existingContent) != output {
		result.Changed = true
	}

	mode := parseFileMode(template.Mode, 0644)
	if err := os.WriteFile(dest, []byte(output), mode); err != nil {
		result.Failed = true
		result.Rc = 1
		if step.Register != "" {
			ec.Variables[step.Register] = result.ToMap()
		}
		return fmt.Errorf("failed to write template output to %s: %w", dest, err)
	}

	// Register the result if register is specified
	if step.Register != "" {
		ec.Variables[step.Register] = result.ToMap()
		ec.Variables[step.Register+"_stdout"] = result.Stdout
		ec.Variables[step.Register+"_stderr"] = result.Stderr
		ec.Variables[step.Register+"_rc"] = result.Rc
		ec.Variables[step.Register+"_failed"] = result.Failed
		ec.Variables[step.Register+"_changed"] = result.Changed
		ec.Logger.Debugf("  Registered result as: %s (changed=%v)", step.Register, result.Changed)
	}

	return nil
}
