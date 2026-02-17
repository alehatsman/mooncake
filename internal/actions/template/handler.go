// Package template implements the template action handler.
//
// The template action reads a template file, renders it with variables,
// and writes the rendered output to a destination file.
package template

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/utils"
)

const (
	defaultFileMode os.FileMode = 0644
)

// Handler implements the Handler interface for template actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the template action.
func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "template",
		Description:        "Render template files and write to destination",
		Category:           actions.CategoryFile,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		EmitsEvents:        []string{string(events.EventTemplateRender)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on dest path
		ImplementsCheck:    true,       // Checks if content differs before writing
	}
}

// Validate checks if the template configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.Template == nil {
		return fmt.Errorf("template configuration is nil")
	}

	tmpl := step.Template
	if tmpl.Src == "" {
		hint := actions.GetActionHint("template", "src")
		return fmt.Errorf("template src is required%s", hint)
	}

	if tmpl.Dest == "" {
		hint := actions.GetActionHint("template", "dest")
		return fmt.Errorf("template dest is required%s", hint)
	}

	return nil
}

// Execute runs the template action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	tmpl := step.Template

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Expand paths
	// Use PresetBaseDir for template src if set (resolves relative to preset root)
	// Otherwise use CurrentDir (resolves relative to current file)
	baseDir := ec.CurrentDir
	if ec.PresetBaseDir != "" {
		baseDir = ec.PresetBaseDir
	}

	src, err := ec.PathUtil.ExpandPath(tmpl.Src, baseDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand src path: %w", err)
	}

	dest, err := ec.PathUtil.ExpandPath(tmpl.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand dest path: %w", err)
	}

	ctx.GetLogger().Debugf("Templating src=\"%s\" dest=\"%s\"", src, dest)

	// Create result
	result := executor.NewResult()
	result.StartTime = time.Now()
	result.Changed = false

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Read template file
	// #nosec G304 -- Template source path from user config is intentional
	templateFile, err := os.Open(src)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to read template: %w", err)
	}
	defer func() {
		if closeErr := templateFile.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("failed to close template file %s: %v", src, closeErr)
		}
	}()

	templateBytes, err := io.ReadAll(templateFile)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to read template: %w", err)
	}

	// Prepare variables for rendering
	variables := ctx.GetVariables()
	if tmpl.Vars != nil && len(*tmpl.Vars) > 0 {
		variables = utils.MergeVariables(ctx.GetVariables(), *tmpl.Vars)
		ctx.GetLogger().Debugf("  Using %d additional template variables", len(*tmpl.Vars))
	}

	// Render template
	output, err := ctx.GetTemplate().Render(string(templateBytes), variables)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to render template: %w", err)
	}

	// Check if content would change
	// #nosec G304 -- Template destination path from user config is intentional
	existingContent, err := os.ReadFile(dest)
	if err != nil || !bytes.Equal(existingContent, []byte(output)) {
		result.Changed = true
	}

	// Parse mode
	mode := h.parseFileMode(tmpl.Mode, defaultFileMode)

	// Write file
	ctx.GetLogger().Debugf("  Writing rendered template: %s (%d bytes)", dest, len(output))
	if err := h.createFileWithBecome(dest, []byte(output), mode, step, ec); err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to write file: %w", err)
	}

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventTemplateRender,
			Data: events.TemplateRenderData{
				TemplatePath: src,
				DestPath:     dest,
				SizeBytes:    int64(len(output)),
				Changed:      result.Changed,
				DryRun:       ctx.IsDryRun(),
			},
		})
	}

	return result, nil
}

// DryRun logs what would be done without actually doing it.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	tmpl := step.Template

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Expand paths
	// Use PresetBaseDir for template src if set (resolves relative to preset root)
	baseDir := ec.CurrentDir
	if ec.PresetBaseDir != "" {
		baseDir = ec.PresetBaseDir
	}

	src, err := ec.PathUtil.ExpandPath(tmpl.Src, baseDir, ctx.GetVariables())
	if err != nil {
		src = tmpl.Src
	}

	dest, err := ec.PathUtil.ExpandPath(tmpl.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		dest = tmpl.Dest
	}

	// Check if source file exists
	if _, statErr := os.Stat(src); os.IsNotExist(statErr) {
		ctx.GetLogger().Errorf("  [DRY-RUN] Template source file does not exist: %s", src)
		return fmt.Errorf("template source not found: %s", src)
	}

	// Prepare variables for rendering
	variables := ctx.GetVariables()
	if tmpl.Vars != nil && len(*tmpl.Vars) > 0 {
		variables = utils.MergeVariables(ctx.GetVariables(), *tmpl.Vars)
	}

	mode := h.parseFileMode(tmpl.Mode, defaultFileMode)

	// Try to render template to provide detailed status
	output, err := h.readAndRenderTemplate(src, ctx, variables, ec)
	if err != nil {
		// Can't read or render - use basic logging
		ctx.GetLogger().Infof("  [DRY-RUN] Would render template: %s -> %s (mode: %s)", src, dest, h.formatMode(mode))
		ctx.GetLogger().Debugf("  Template render error (dry-run): %v", err)
		return nil
	}

	// Compare with existing content
	// #nosec G304 -- Template destination path from user config is intentional
	existingContent, err := os.ReadFile(dest)
	if err != nil {
		// File doesn't exist - will be created
		ctx.GetLogger().Infof("  [DRY-RUN] Would create file from template: %s -> %s (size: %d bytes, mode: %s)",
			src, dest, len(output), h.formatMode(mode))
	} else {
		if string(existingContent) != output {
			ctx.GetLogger().Infof("  [DRY-RUN] Would update file from template: %s -> %s (size: %d -> %d bytes)",
				src, dest, len(existingContent), len(output))
		} else {
			ctx.GetLogger().Infof("  [DRY-RUN] Template already up to date: %s", dest)
		}
	}

	if tmpl.Vars != nil && len(*tmpl.Vars) > 0 {
		ctx.GetLogger().Debugf("  Additional variables: %d vars", len(*tmpl.Vars))
	}

	return nil
}

// Helper functions

func (h *Handler) formatMode(mode os.FileMode) string {
	return fmt.Sprintf("%#o", mode)
}

func (h *Handler) parseFileMode(modeStr string, defaultMode os.FileMode) os.FileMode {
	if modeStr == "" {
		return defaultMode
	}

	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return defaultMode
	}

	return os.FileMode(mode)
}

func (h *Handler) readAndRenderTemplate(src string, ctx actions.Context, variables map[string]interface{}, _ *executor.ExecutionContext) (string, error) {
	// #nosec G304 -- Template source path from user config is intentional
	srcFile, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("failed to open template: %w", err)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("failed to close template file %s: %v", src, closeErr)
		}
	}()

	srcBytes, err := io.ReadAll(srcFile)
	if err != nil {
		return "", fmt.Errorf("failed to read template: %w", err)
	}

	output, err := ctx.GetTemplate().Render(string(srcBytes), variables)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return output, nil
}

func (h *Handler) createFileWithBecome(path string, content []byte, mode os.FileMode, step *config.Step, ec *executor.ExecutionContext) error {
	if !step.Become {
		// #nosec G306 -- Mode is user-configurable for provisioning
		return os.WriteFile(path, content, mode)
	}

	// Use sudo to write file
	tmpFile, err := os.CreateTemp("", "mooncake-template-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	// Write content to temp file
	if err := os.WriteFile(tmpPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Move with sudo and set permissions
	return h.executeSudoFileOperation(tmpPath, path, mode, step, ec)
}

func (h *Handler) executeSudoFileOperation(tmpPath, destPath string, mode os.FileMode, step *config.Step, ec *executor.ExecutionContext) error {
	cmd := fmt.Sprintf("mv %s %s && chmod %s %s", tmpPath, destPath, h.formatMode(mode), destPath)
	return h.executeSudoCommand(cmd, step, ec)
}

func (h *Handler) executeSudoCommand(command string, _ *config.Step, ec *executor.ExecutionContext) error {
	// #nosec G204 - This is a provisioning tool designed to execute commands
	cmd := exec.Command("sudo", "-S", "sh", "-c", command)
	cmd.Stdin = bytes.NewBufferString(ec.SudoPass + "\n")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo command failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}
