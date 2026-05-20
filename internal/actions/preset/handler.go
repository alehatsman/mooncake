// Package preset implements the preset action handler.
// Presets expand into multiple steps with parameter injection.
package preset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/modules"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/alehatsman/mooncake/internal/presets"
)

// Handler implements the preset action handler.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// savedContext captures the current execution context state for restoration.
type savedContext struct {
	variables     map[string]interface{}
	currentDir    string
	presetBaseDir string
}

// captureContext saves the current execution context state.
func captureContext(ec *executor.ExecutionContext) *savedContext {
	saved := &savedContext{
		variables:     make(map[string]interface{}),
		currentDir:    ec.CurrentDir,
		presetBaseDir: ec.PresetBaseDir,
	}
	for k, v := range ec.Scope.User {
		saved.variables[k] = v
	}
	return saved
}

// restoreContext restores the execution context to the saved state,
// removing any keys added during preset execution.
func (s *savedContext) restore(ec *executor.ExecutionContext, parametersNamespace map[string]interface{}) {
	// Remove parameters namespace from scope user vars
	for k := range parametersNamespace {
		delete(ec.Scope.User, k)
	}
	// Restore original user variables
	ec.Scope.User = make(map[string]interface{})
	for k, v := range s.variables {
		ec.Scope.User[k] = v
	}
	// Restore original directories
	ec.CurrentDir = s.currentDir
	ec.PresetBaseDir = s.presetBaseDir
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "use",
		Description:        "Execute a preset by expanding it into steps",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{}, // All platforms (meta-action)
		RequiresSudo:       false,      // Depends on constituent steps
		ImplementsCheck:    false,      // Meta-action, delegates to steps
	}
}

// displayPresetHelp reads and displays the preset's README file if it exists.
// This provides compact, actionable help after successful installation.
func displayPresetHelp(ec *executor.ExecutionContext, _, baseDir string) {
	if baseDir == "" {
		return
	}

	// Try to read README.md from preset directory
	readmePath := fmt.Sprintf("%s/README.md", baseDir)
	data, err := os.ReadFile(readmePath) // #nosec G304 -- baseDir comes from trusted preset loader
	if err != nil {
		// README not found or unreadable - skip silently
		return
	}

	// Display README content via logger
	ec.Svc.Logger.Infof("\n%s", string(data))
}

// Validate validates the preset action configuration.
func (h *Handler) Validate(step *config.Step) error {
	if step.Use == nil {
		return fmt.Errorf("preset action requires preset configuration")
	}
	if step.Use.Name == "" {
		return fmt.Errorf("preset name is required")
	}
	return nil
}

// Execute executes the preset action.
// Run is the Spec 16 entry point. Presets compose other steps; the
// planner expands them at plan time so this handler rarely runs in
// practice. Plan mode reports "not checkable"; apply mode expands
// the preset, executes its expanded steps in sequence, and emits
// EventPresetExpanded / EventPresetCompleted bookends.
//
// F011: legacy Execute / DryRun pair folded into Run.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Reason = "not checkable (preset; usually expanded at plan time)"
		return r, nil
	}

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	invocation := step.Use

	// spec-67 dispatch:
	//   local path  → LoadPresetFromPath against ec.CurrentDir
	//   remote ref  → resolver fetches + reads index.yml
	//   alias hit   → resolver via Svc.Modules
	//   else        → legacy preset (search paths)
	var expandedSteps []config.Step
	var parametersNamespace map[string]interface{}
	var presetBaseDir string
	var err error
	switch invocation.Kind() {
	case config.ComponentRefLocalPath:
		absPath := invocation.Name
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(ec.CurrentDir, invocation.Name)
		}
		expandedSteps, parametersNamespace, presetBaseDir, err = presets.ExpandPresetFromPath(invocation, absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to expand component '%s': %w", invocation.Name, err)
		}
	case config.ComponentRefRemote:
		expandedSteps, parametersNamespace, presetBaseDir, err = resolveAndExpand(ec, invocation)
		if err != nil {
			return nil, err
		}
	default:
		// Alias hit when the bare name appears in the playbook's modules: block.
		// Otherwise fall through to the legacy preset search-path loader.
		if _, isAlias := ec.Svc.Modules[firstSegment(invocation.Name)]; isAlias && ec.Svc.Modules != nil {
			expandedSteps, parametersNamespace, presetBaseDir, err = resolveAndExpand(ec, invocation)
			if err != nil {
				return nil, err
			}
		} else {
			expandedSteps, parametersNamespace, presetBaseDir, err = presets.ExpandPreset(invocation)
			if err != nil {
				return nil, fmt.Errorf("failed to expand preset '%s': %w", invocation.Name, err)
			}
		}
	}

	// Emit preset expanded event
	ec.EmitEvent(events.EventPresetExpanded, events.PresetData{
		Name:       invocation.Name,
		Parameters: invocation.With,
		StepsCount: len(expandedSteps),
	})

	ec.Svc.Logger.Infof("Expanding preset '%s' into %d steps", invocation.Name, len(expandedSteps))

	// Save current context for restoration
	saved := captureContext(ec)
	defer saved.restore(ec, parametersNamespace)

	// Merge parameters namespace into variables
	for k, v := range parametersNamespace {
		ec.Scope.User[k] = v
	}

	// Set PresetBaseDir to preset base directory for template path resolution
	// This persists across included task files, unlike CurrentDir which changes per file
	if presetBaseDir != "" {
		ec.PresetBaseDir = presetBaseDir
		ec.CurrentDir = presetBaseDir
	}

	// Use planner to expand includes, loops, and other plan-time directives
	// This ensures includes within preset steps are properly expanded
	planner, err := plan.NewPlanner()
	if err != nil {
		return nil, fmt.Errorf("failed to create planner: %w", err)
	}
	fullyExpandedSteps, err := planner.ExpandStepsWithContext(expandedSteps, ec.GetVariables(), presetBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand preset steps: %w", err)
	}

	ec.Svc.Logger.Infof("Preset '%s' expanded to %d steps (after include expansion)", invocation.Name, len(fullyExpandedSteps))

	// Execute fully expanded steps
	anyChanged := false
	for i, expandedStep := range fullyExpandedSteps {
		ec.Svc.Logger.Debugf("Executing preset step %d/%d: %s", i+1, len(fullyExpandedSteps), expandedStep.Name)

		if err := executor.ExecuteStep(expandedStep, ec); err != nil {
			return nil, fmt.Errorf("preset '%s' step %d failed: %w", invocation.Name, i+1, err)
		}

		// Track if any step changed
		if ec.CurrentResult != nil && ec.CurrentResult.Changed {
			anyChanged = true
		}
	}

	// Create preset result
	result := executor.NewResult()
	result.Changed = anyChanged
	result.Stdout = fmt.Sprintf("Preset '%s' executed %d steps", invocation.Name, len(fullyExpandedSteps))

	// Emit preset completed event
	ec.EmitEvent(events.EventPresetCompleted, events.PresetData{
		Name:       invocation.Name,
		Parameters: invocation.With,
		StepsCount: len(fullyExpandedSteps),
		Changed:    anyChanged,
	})

	ec.Svc.Logger.Infof("Preset '%s' completed: changed=%v", invocation.Name, anyChanged)

	// Display README if preset has state=present and execution succeeded
	if invocation.With != nil {
		if state, ok := invocation.With["state"].(string); ok && state == "present" {
			displayPresetHelp(ec, invocation.Name, presetBaseDir)
		}
	} else {
		// Default state is "present" if not specified
		displayPresetHelp(ec, invocation.Name, presetBaseDir)
	}

	return result, nil
}

// resolverFor builds a Resolver wired to the run's Modules map. Overridable
// for tests so they can inject a Fetcher with a fixture CloneURL.
var resolverFor = func(ec *executor.ExecutionContext) *modules.Resolver {
	return &modules.Resolver{
		Fetcher: &modules.Fetcher{},
		Modules: ec.Svc.Modules,
	}
}

// resolveAndExpand resolves a remote or alias `use:` reference, then expands
// the component file. Shared by the remote and alias branches of Run.
func resolveAndExpand(ec *executor.ExecutionContext, invocation *config.PresetInvocation) ([]config.Step, map[string]interface{}, string, error) {
	resolver := resolverFor(ec)
	bgCtx := ec.Svc.Ctx
	if bgCtx == nil {
		bgCtx = context.Background()
	}
	resolved, err := resolver.Resolve(bgCtx, invocation.Name)
	if err != nil {
		return nil, nil, "", fmt.Errorf("resolve module %q: %w", invocation.Name, err)
	}
	return presets.ExpandPresetFromPath(invocation, resolved.ComponentPath)
}

// firstSegment returns the portion of s before the first '/' (or all of s).
// Used to peel "alias/export" → "alias" for the modules map lookup.
func firstSegment(s string) string {
	if i := indexByte(s, '/'); i >= 0 {
		return s[:i]
	}
	return s
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
