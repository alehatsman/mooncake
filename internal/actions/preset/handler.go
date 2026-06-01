// Package preset implements the preset action handler.
// Presets expand into multiple steps with parameter injection.
package preset

import (
	"context"
	"fmt"
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
	variables  map[string]interface{}
	currentDir string
}

// captureContext saves the current execution context state.
func captureContext(ec *executor.ExecutionContext) *savedContext {
	saved := &savedContext{
		variables:  make(map[string]interface{}),
		currentDir: ec.CurrentDir,
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
	ec.CurrentDir = s.currentDir
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

// Validate validates the preset action configuration.
func (h *Handler) Validate(step *config.Step) error {
	if step.Use == "" {
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

	name := step.Use
	props := step.Props

	// spec-67 dispatch:
	//   local path  → LoadPresetFromPath against ec.CurrentDir
	//   remote ref  → resolver fetches + reads index.yml
	//   alias hit   → resolver via Svc.Modules
	//   else        → legacy preset (search paths)
	var expandedSteps []config.Step
	var parametersNamespace map[string]interface{}
	var presetBaseDir string
	var err error
	switch config.ComponentRefKindOf(name) {
	case config.ComponentRefLocalPath:
		absPath := name
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(ec.CurrentDir, name)
		}
		expandedSteps, parametersNamespace, presetBaseDir, err = presets.ExpandPresetFromPath(name, props, absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to expand component '%s': %w", name, err)
		}
	case config.ComponentRefRemote:
		// Inline remote refs carry no alias binding, so no default props.
		expandedSteps, parametersNamespace, presetBaseDir, err = resolveAndExpand(ec, name, props, nil)
		if err != nil {
			return nil, err
		}
	default:
		// Alias hit when the bare name appears in the playbook's modules: block.
		// Otherwise fall through to the legacy preset search-path loader.
		if binding, isAlias := ec.Svc.Modules[firstSegment(name)]; isAlias && ec.Svc.Modules != nil {
			// #52/#57: the alias binding may carry default props; they're
			// applied (filtered to the component's declared params) inside
			// resolveAndExpand once the component is loaded.
			expandedSteps, parametersNamespace, presetBaseDir, err = resolveAndExpand(ec, name, props, binding.Props)
			if err != nil {
				return nil, err
			}
		} else {
			expandedSteps, parametersNamespace, presetBaseDir, err = presets.ExpandPreset(name, props)
			if err != nil {
				return nil, fmt.Errorf("failed to expand preset '%s': %w", name, err)
			}
		}
	}

	// Emit preset expanded event
	ec.EmitEvent(events.EventPresetExpanded, events.PresetData{
		Name:       name,
		Parameters: props,
		StepsCount: len(expandedSteps),
	})

	ec.Svc.Logger.Infof("Expanding preset '%s' into %d steps", name, len(expandedSteps))

	// Save current context for restoration
	saved := captureContext(ec)
	defer saved.restore(ec, parametersNamespace)

	// Merge parameters namespace into variables
	for k, v := range parametersNamespace {
		ec.Scope.User[k] = v
	}

	// Flip CurrentDir to the preset's entrypoint dir so the first file's
	// relative paths resolve against the preset root. Subsequent includes
	// inside the preset re-flip CurrentDir per file via the planner.
	if presetBaseDir != "" {
		ec.CurrentDir = presetBaseDir
	}

	// Use planner to expand includes, loops, and other plan-time directives
	// This ensures includes within preset steps are properly expanded
	planner, err := plan.NewPlanner()
	if err != nil {
		return nil, fmt.Errorf("failed to create planner: %w", err)
	}
	fullyExpandedSteps, err := planner.ExpandStepsWithContext(expandedSteps, ec.Variables(), presetBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand preset steps: %w", err)
	}

	ec.Svc.Logger.Infof("Preset '%s' expanded to %d steps (after include expansion)", name, len(fullyExpandedSteps))

	// Execute fully expanded steps
	anyChanged := false
	for i, expandedStep := range fullyExpandedSteps {
		ec.Svc.Logger.Debugf("Executing preset step %d/%d: %s", i+1, len(fullyExpandedSteps), expandedStep.Name)

		if err := executor.ExecuteStep(expandedStep, ec); err != nil {
			return nil, fmt.Errorf("preset '%s' step %d failed: %w", name, i+1, err)
		}

		// Track if any step changed
		if ec.CurrentResult != nil && ec.CurrentResult.Changed {
			anyChanged = true
		}
	}

	// Create preset result
	result := executor.NewResult()
	result.Target = name
	if anyChanged {
		result.Operation = executor.OpUpdate
	} else {
		result.Operation = executor.OpNoop
	}
	result.Changed = anyChanged
	result.Stdout = fmt.Sprintf("Preset '%s' executed %d steps", name, len(fullyExpandedSteps))

	// Emit preset completed event
	ec.EmitEvent(events.EventPresetCompleted, events.PresetData{
		Name:       name,
		Parameters: props,
		StepsCount: len(fullyExpandedSteps),
		Changed:    anyChanged,
	})

	ec.Svc.Logger.Infof("Preset '%s' completed: changed=%v", name, anyChanged)

	return result, nil
}

// resolverFor builds a Resolver wired to the run's Modules map. Overridable
// for tests so they can inject a Fetcher with a fixture CloneURL. The resolver
// only needs alias→source, so module-level default props (#52) are dropped
// here and merged separately at the call site.
var resolverFor = func(ec *executor.ExecutionContext) *modules.Resolver {
	sources := make(map[string]string, len(ec.Svc.Modules))
	for alias, binding := range ec.Svc.Modules {
		sources[alias] = binding.Source
	}
	return &modules.Resolver{
		Fetcher: &modules.Fetcher{},
		Modules: sources,
	}
}

// resolveAndExpand resolves a remote or alias `use:` reference, then expands
// the component file. Shared by the remote and alias branches of Run.
//
// defaultProps are the alias binding's module-level default props (#52), or
// nil for inline-remote refs. They are merged under the caller's per-call
// props once the component is loaded, FILTERED to the params the component
// actually declares (#57) — so one binding can serve exports with different
// prop schemas without "unknown parameter" failures.
func resolveAndExpand(ec *executor.ExecutionContext, name string, props, defaultProps map[string]interface{}) ([]config.Step, map[string]interface{}, string, error) {
	resolver := resolverFor(ec)
	bgCtx := ec.Svc.Ctx
	if bgCtx == nil {
		bgCtx = context.Background()
	}
	resolved, err := resolver.Resolve(bgCtx, name)
	if err != nil {
		return nil, nil, "", fmt.Errorf("resolve module %q: %w", name, err)
	}
	def, err := presets.LoadPresetFromPath(resolved.ComponentPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("resolve module %q: %w", name, err)
	}
	render := func(s string) (string, error) { return ec.Template().Render(s, ec.Variables()) }
	merged, err := mergeModuleDefaults(props, defaultProps, def.Parameters, render)
	if err != nil {
		return nil, nil, "", fmt.Errorf("module %q: %w", name, err)
	}
	return presets.ExpandLoadedDefinition(name, merged, def)
}

// mergeModuleDefaults layers a module binding's default props (#52) underneath
// the caller's per-call props, filtered to the params the resolved component
// declares (#57). A default for a param the component doesn't define is
// silently skipped — that's what lets one alias binding carry, say, a go_tags
// default that only some exports accept. Default values are template-rendered
// (so `{{ GO_TAGS }}` resolves); per-call props always win and are never
// re-rendered here (they were rendered at plan time).
func mergeModuleDefaults(caller, defaults map[string]interface{}, declared map[string]config.PresetParameter, render func(string) (string, error)) (map[string]interface{}, error) {
	if len(defaults) == 0 {
		return caller, nil
	}
	out := make(map[string]interface{}, len(caller)+len(defaults))
	for k, v := range defaults {
		if _, isDeclared := declared[k]; !isDeclared {
			continue // filter: component doesn't accept this prop
		}
		if _, set := caller[k]; set {
			continue // per-call prop wins; don't bother rendering the default
		}
		rendered, err := renderPropValue(v, render)
		if err != nil {
			return nil, fmt.Errorf("render default prop %q: %w", k, err)
		}
		out[k] = rendered
	}
	for k, v := range caller {
		out[k] = v
	}
	return out, nil
}

// renderPropValue renders a default prop value: strings go through the
// template engine, maps/slices recurse, other scalars pass through.
func renderPropValue(v interface{}, render func(string) (string, error)) (interface{}, error) {
	switch x := v.(type) {
	case string:
		return render(x)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, vv := range x {
			r, err := renderPropValue(vv, render)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, vv := range x {
			r, err := renderPropValue(vv, render)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	default:
		return v, nil
	}
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
