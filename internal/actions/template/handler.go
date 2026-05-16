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
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/effects"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
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
		Name:               "file.template",
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

// Permissions implements actions.Permitter (spec-22). Declares Sudo
// when Dest falls under a known system root (/etc/, /usr/, /var/, ...)
// and always populates FilesystemWrite=[Dest]. Network / RequiredBinaries
// stay unset — template rendering is pure local-FS, like file.write.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	var ps actions.PermissionSet
	if step == nil || step.FileTemplate == nil {
		return ps
	}
	if actions.PathNeedsSudo(step.FileTemplate.Dest) {
		ps.Sudo = true
	}
	if step.FileTemplate.Dest != "" {
		ps.FilesystemWrite = []string{step.FileTemplate.Dest}
	}
	return ps
}

// Validate checks if the template configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.FileTemplate == nil {
		return fmt.Errorf("template configuration is nil")
	}

	tmpl := step.FileTemplate
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
	if !step.ShouldBecome() {
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
	// F032: shell-quote every interpolated path. Without quoting, a
	// dest like "/tmp/x; touch /etc/owned" became `mv /tmp/... /tmp/x;
	// touch /etc/owned && chmod 0644 /tmp/x; touch /etc/owned` —
	// arbitrary code execution under sudo. Modern Run() goes through
	// ctx.Effects().WriteFile which already quotes; the legacy Execute
	// path is unreachable in production today but is exported on the
	// Handler type and trivially callable from tests / future SDKs.
	cmd := fmt.Sprintf("mv %s %s && chmod %s %s",
		effects.ShellQuote(tmpPath),
		effects.ShellQuote(destPath),
		h.formatMode(mode),
		effects.ShellQuote(destPath))
	return h.executeSudoCommand(cmd, step, ec)
}

func (h *Handler) executeSudoCommand(command string, _ *config.Step, ec *executor.ExecutionContext) error {
	// F005 follow-up: route through security.BecomeRunner — see the
	// matching helper in download/handler.go for the why.
	runner := security.BecomeRunner{SudoPass: ec.Svc.SudoPass}
	cmd, err := runner.Command(true, "sh", "-c", command)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo command failed: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// Run is the Spec 16 unified entry point. Renders the template source
// against the current variables, then either writes the result (or
// reports what would happen in ModePlan). Drift between preview and
// execute is eliminated because both modes render and compare against
// the same dest content via a single Performer.WriteFile call.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	tmpl := step.FileTemplate

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Resolve src path against the preset root if set, otherwise the
	// current config dir. Preserves the existing template-handler
	// contract.
	baseDir := ec.CurrentDir
	if ec.PresetBaseDir != "" {
		baseDir = ec.PresetBaseDir
	}
	src, err := ec.Svc.PathUtil.ExpandPath(tmpl.Src, baseDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand src path: %w", err)
	}
	dest, err := ec.Svc.PathUtil.ExpandPath(tmpl.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand dest path: %w", err)
	}

	result := executor.NewResult()
	result.Checkable = true
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Read template source.
	// #nosec G304 -- template source path from user config is intentional
	f, err := os.Open(src)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to read template: %w", err)
	}
	defer func() { _ = f.Close() }()
	templateBytes, err := io.ReadAll(f)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to read template: %w", err)
	}

	// Merge per-step vars before rendering.
	variables := ctx.GetVariables()
	if tmpl.Vars != nil && len(*tmpl.Vars) > 0 {
		variables = utils.MergeVariables(ctx.GetVariables(), *tmpl.Vars)
	}

	output, err := ctx.GetTemplate().Render(string(templateBytes), variables)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to render template: %w", err)
	}
	rendered := []byte(output)
	mode := h.parseFileMode(tmpl.Mode, defaultFileMode)

	// Capture pre-state for Reverse() (spec-22 phase 5 slice D).
	// Apply mode only — plan mode doesn't mutate, so there is
	// nothing to reverse. Must precede the WriteFile below.
	if ctx.Mode() == actions.ModeApply {
		result.ReverseData = filehandler.CaptureReverseInfo(dest, "")
	}

	// Delegate to Performer.WriteFile — same site decides both
	// "would change?" and "perform write" semantics for both modes.
	eff := ctx.Effects().WriteFile(dest, rendered, mode, actions.PerformerOpts{Become: step.ShouldBecome()})

	if eff.Err != nil {
		result.Failed = true
		return result, eff.Err
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = eff.WouldChange
		result.Reason = eff.Reason
		return result, nil
	}

	result.Changed = eff.Performed

	// Emit template render event — only in execute mode, matching the
	// legacy contract.
	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventTemplateRender,
			Data: events.TemplateRenderData{
				TemplatePath: src,
				DestPath:     dest,
				SizeBytes:    int64(len(rendered)),
				Changed:      result.Changed,
				DryRun:       false,
			},
		})
	}

	return result, nil
}

// bytes is referenced indirectly by the rest of the package; keep the
// import alive across builds that strip unused refs.
var _ = bytes.Equal
