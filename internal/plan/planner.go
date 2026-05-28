package plan

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/presets"
	"github.com/alehatsman/mooncake/internal/secrets/resolver"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
	"github.com/alehatsman/mooncake/internal/utils"
)

// resolvePath converts a potentially relative path to an absolute path.
// If the path is relative, it's joined with baseDir. Then filepath.Abs is called.
func resolvePath(path, baseDir string) (string, error) {
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(baseDir, path)
	}
	return filepath.Abs(absPath)
}

// validatePlatformSupport checks if the action is supported on the current platform.
// Returns an error if the action is not supported.
func validatePlatformSupport(actionType string) error {
	// Get handler from registry
	handler, ok := actions.Get(actionType)
	if !ok {
		// Action not in registry - might be legacy action, skip validation
		return nil
	}

	metadata := handler.Metadata()

	// Empty SupportedPlatforms means all platforms are supported
	if len(metadata.SupportedPlatforms) == 0 {
		return nil
	}

	// Check if current platform is in the supported list
	currentOS := runtime.GOOS
	for _, supportedOS := range metadata.SupportedPlatforms {
		if supportedOS == currentOS {
			return nil
		}
	}

	// Platform not supported
	return fmt.Errorf(
		"action '%s' is not supported on platform '%s' (supported platforms: %v)",
		actionType,
		currentOS,
		metadata.SupportedPlatforms,
	)
}

// Planner builds deterministic execution plans from config files
type Planner struct {
	template      template.Renderer
	pathUtil      *pathutil.PathExpander
	fileTree      *filetree.Walker
	stepIDCounter int
	includeStack  []IncludeFrame
	seenFiles     map[string]bool
	locationMap   map[int]*IncludeFrame // Map step index to location
	// inputFiles tracks every absolute file path read while building
	// the plan (root file + all transitively included files). Used for
	// the Spec 16 stale-plan integrity check.
	inputFiles []string
	// redactor receives every secret value resolved during plan
	// expansion (F037). Lives on the planner because the standalone
	// vars action is evaluated at plan time and never reaches the
	// executor's resolve site — without this, a top-level
	// `vars: { TOKEN: !secret env:FOO }` flows the sentinel marker
	// through to subsequent template renders.
	redactor *security.Redactor
}

// IncludeFrame tracks a frame in the include stack for cycle detection and origin tracking
type IncludeFrame struct {
	FilePath string
	Line     int
	Column   int
}

// ExpansionContext holds the context during plan expansion
type ExpansionContext struct {
	Variables  map[string]interface{}
	CurrentDir string
	Tags       []string
	// SkipTags excludes steps whose tags intersect this list
	// (MT-58 `--skip-tags`). Composes with Tags via AND.
	SkipTags []string
	// Names is the spec-50 step-name filter. When non-empty, a step is
	// only kept (step.Skipped=false) when its name matches one of the
	// entries. Untagged steps still run on a tag filter; unnamed steps
	// are dropped on a name filter (see utils.MatchesNames).
	Names []string
}

// PlannerConfig holds configuration for building a plan
type PlannerConfig struct {
	ConfigPath string
	Variables  map[string]interface{}
	Tags       []string
	// SkipTags excludes steps whose tags intersect this list (MT-58).
	SkipTags []string
	// Names is the spec-50 step-name filter; propagated into
	// ExpansionContext so per-step skip evaluation can consult it.
	Names []string

	// TaskName, when non-empty, selects a named task from the config's
	// `tasks:` block instead of the top-level `steps:` list. The
	// planner replaces RunConfig.Steps with the task's Steps and
	// layers the task's Vars between file-level vars (lowest) and the
	// caller-supplied Variables (highest). An unknown task name is an
	// error from BuildPlan.
	TaskName string
}

// NewPlanner creates a new Planner instance.
// Returns an error if template renderer initialization fails.
func NewPlanner() (*Planner, error) {
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		return nil, fmt.Errorf("failed to create template renderer: %w", err)
	}

	pathExpander := pathutil.NewPathExpander(renderer)
	return &Planner{
		template:    renderer,
		pathUtil:    pathExpander,
		fileTree:    filetree.NewWalker(pathExpander),
		seenFiles:   make(map[string]bool),
		locationMap: make(map[int]*IncludeFrame),
		redactor:    security.NewRedactor(),
	}, nil
}

// ExpandStepsWithContext expands a list of steps with the given context.
// This is useful for expanding preset steps which may contain includes, loops, etc.
// Returns the expanded steps ready for execution.
func (p *Planner) ExpandStepsWithContext(steps []config.Step, variables map[string]interface{}, currentDir string) ([]config.Step, error) {
	// Create expansion context
	ctx := &ExpansionContext{
		Variables:  variables,
		CurrentDir: currentDir,
		Tags:       nil, // No tag filtering for preset expansion
	}

	// Create temporary plan to collect expanded steps
	plan := &Plan{
		Steps: make([]config.Step, 0),
	}

	// Expand steps
	if err := p.expandSteps(steps, ctx, plan, 0); err != nil {
		return nil, err
	}

	return plan.Steps, nil
}

// BuildPlan generates a deterministic execution plan from a config file
func (p *Planner) BuildPlan(cfg PlannerConfig) (*Plan, error) {
	// Read config with validation. readRunConfig already wraps with
	// "failed to read config:" — passing the error through avoids the
	// doubled prefix MT-26 reported via the MCP run_plan surface.
	runConfig, err := p.readRunConfig(cfg.ConfigPath)
	if err != nil {
		return nil, err
	}

	// Task selection: when TaskName is set, swap the top-level Steps
	// for the named task's Steps and layer the task's Vars between
	// file-level vars (lowest) and caller-supplied Variables (highest).
	// All downstream planning — loops, includes, secrets, template
	// rendering — runs against the task's steps unchanged.
	taskVars := map[string]interface{}(nil)
	if cfg.TaskName != "" {
		task, ok := runConfig.Tasks[cfg.TaskName]
		if !ok {
			return nil, fmt.Errorf("task %q not found in %s (defined tasks: %s)",
				cfg.TaskName, cfg.ConfigPath, joinTaskNames(runConfig.Tasks))
		}
		runConfig.Steps = task.Steps
		taskVars = task.Vars
	}

	// Initialize plan
	plan := &Plan{
		Version:     "1.0",
		GeneratedAt: time.Now(),
		RootFile:    cfg.ConfigPath,
		Steps:       make([]config.Step, 0),
		InitialVars: cfg.Variables,
		Tags:        cfg.Tags,
		Modules:     runConfig.Modules,
	}

	// Merge: file-level vars < task vars < caller-supplied vars.
	// MergeVariables right-wins on key collision.
	variables := utils.MergeVariables(runConfig.Vars, taskVars)
	variables = utils.MergeVariables(variables, cfg.Variables)

	// Inject system facts (ansible_os_family, ansible_distribution, etc.)
	// These are added after config vars but before expansion, so templates can use them
	systemFacts := facts.Collect()
	for k, v := range systemFacts.ToMap() {
		variables[k] = v
	}

	// Update plan's InitialVars to include system facts
	// This ensures the facts are available during execution for 'when' conditions
	plan.InitialVars = variables

	// Snapshot a minimal subset of facts onto GeneratedOn for stale-plan
	// detection at apply time (Spec 16). Keep this small: OS family,
	// architecture, distro family. Matching too many facts (hostname,
	// kernel version) would make plans unportable across similar
	// machines without adding meaningful safety.
	plan.GeneratedOn = HostFacts{
		OsFamily:     systemFacts.OS,
		Arch:         systemFacts.Arch,
		DistroFamily: systemFacts.Distribution,
	}

	// Create expansion context. CurrentDir must be absolute so that path
	// expansion in walkAndRender produces absolute paths — otherwise the
	// template/copy/file handlers re-join against their own (possibly
	// different) CurrentDir at execute time and the path doubles.
	currentDir := filepath.Dir(cfg.ConfigPath)
	if !filepath.IsAbs(currentDir) {
		if abs, absErr := filepath.Abs(currentDir); absErr == nil {
			currentDir = abs
		}
	}
	ctx := &ExpansionContext{
		Variables:  variables,
		CurrentDir: currentDir,
		Tags:       cfg.Tags,
		SkipTags:   cfg.SkipTags,
		Names:      cfg.Names,
	}

	// Mark root file as seen
	absPath, err := filepath.Abs(cfg.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}
	p.seenFiles[absPath] = true
	p.inputFiles = append(p.inputFiles, absPath)

	// Push root frame
	p.includeStack = append(p.includeStack, IncludeFrame{
		FilePath: absPath,
		Line:     1,
		Column:   1,
	})

	// Expand all steps
	if expandErr := p.expandSteps(runConfig.Steps, ctx, plan, 0); expandErr != nil {
		return nil, expandErr
	}

	// Capture the input-file set + hash for stale-plan detection at
	// apply time. Dedupe (a file may appear multiple times if included
	// from multiple parents) and store sorted for determinism.
	plan.InputFiles = uniqueSorted(p.inputFiles)
	hash, err := HashInputFiles(plan.InputFiles)
	if err != nil {
		return nil, fmt.Errorf("hash input files: %w", err)
	}
	plan.InputFilesHash = hash

	// Strict-template scan: surfaces `{{ root }}` references whose
	// root identifier is not in initial_vars and not produced by a
	// prior step's `as:` register. The planner attaches the list to
	// the plan; commands (`validate`, `plan`) decide whether the
	// presence of any refs is fatal.
	plan.UnresolvedTemplates = CheckPlanStrict(plan)

	return plan, nil
}

// joinTaskNames returns the comma-separated, sorted list of keys in
// the tasks map, for use in unknown-task error messages. "<none>"
// when the map is empty so the diagnostic is still informative.
func joinTaskNames(tasks map[string]config.Task) string {
	if len(tasks) == 0 {
		return "<none>"
	}
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// uniqueSorted dedupes and sorts a slice of strings.
func uniqueSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// readRunConfig reads and parses a config file with validation
func (p *Planner) readRunConfig(path string) (*config.RunConfig, error) {
	// Use ReadConfigWithValidation to get parsed config with steps, vars, and version
	parsedConfig, diagnostics, err := config.ReadConfigWithValidation(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Check for validation errors
	if len(diagnostics) > 0 && config.HasErrors(diagnostics) {
		formatted := config.FormatDiagnosticsWithContext(diagnostics)
		return nil, fmt.Errorf("configuration validation failed:\n%s", formatted)
	}

	// Convert ParsedConfig to RunConfig
	runConfig := &config.RunConfig{
		Version: parsedConfig.Version,
		Vars:    parsedConfig.GlobalVars,
		Modules: parsedConfig.Modules,
		Steps:   parsedConfig.Steps,
		Tasks:   parsedConfig.Tasks,
	}

	return runConfig, nil
}

// expandStep dispatches a single step to the appropriate expansion handler
func (p *Planner) expandStep(step config.Step, ctx *ExpansionContext, plan *Plan, stepIndex int) error {
	// spec-30: Transaction compound step. The step itself emits a
	// transaction-parent plan entry (no action; carries the children's
	// linkage); children expand as sibling steps tagged with TxnParent.
	// All reversibility checking happens here at plan time.
	if len(step.Transaction) > 0 {
		return p.expandTransaction(step, ctx, plan, stepIndex)
	}

	// spec-23 §2: Try / Catch / Finally compound step. Parent emits a
	// no-action plan entry carrying the branches; each child expands
	// as a sibling tagged with TryParent + TryRole. Executor uses the
	// tags to gate execution (try/catch skip logic) and run finally
	// after a try failure (see executor/trycatch.go and ExecuteSteps).
	if len(step.Try) > 0 {
		return p.expandTry(step, ctx, plan, stepIndex)
	}

	// Handle include directives
	if step.Import != nil {
		return p.expandInclude(step, ctx, plan, stepIndex)
	}

	// spec-67 phase-2: expand local-path `use:` references inline at plan
	// time so the plan shows the component's actual steps, not an opaque
	// "not checkable" entry. Remote/alias refs and paths still carrying
	// {{ }} (because a register-captured variable hasn't run yet) fall
	// through to the default compilePlanStep path and stay opaque.
	if step.Use != "" {
		expanded, err := p.tryExpandUse(step, ctx, plan, stepIndex)
		if err != nil {
			return err
		}
		if expanded {
			return nil
		}
	}

	// Handle loop constructs
	if step.ForEach != nil {
		return p.expandWithItems(step, ctx, plan)
	}
	if step.ForEachFile != nil {
		return p.expandWithFileTree(step, ctx, plan)
	}

	// Handle variable operations (skip if when condition is false at plan time)
	if step.Vars != nil {
		if !p.shouldProcessAtPlanTime(step, ctx) {
			return nil // Skip this step
		}
		return p.expandVars(step, ctx)
	}
	if step.VarsLoad != nil {
		if !p.shouldProcessAtPlanTime(step, ctx) {
			return nil // Skip this step
		}
		return p.expandIncludeVars(step, ctx)
	}

	// Regular step - compile it
	planStep, err := p.compilePlanStep(step, ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to compile step %q: %w", step.Name, err)
	}

	// Capture on_change children BEFORE clearing the field on the plan-step
	// copy. Spec-23 §1: the planner expands them into sibling plan steps
	// tagged with TriggeredBy=parent.ID. The executor uses TriggeredBy to
	// gate execution on the parent's outputs.changed.
	//
	// On the parent's plan entry we clear OnChange so the field doesn't
	// serialize twice (once on the parent + once as expanded sibling
	// entries). The linkage survives via the children's TriggeredBy.
	onChange := planStep.OnChange
	planStep.OnChange = nil
	plan.Steps = append(plan.Steps, planStep)

	if len(onChange) > 0 {
		parentID := planStep.ID
		for ci := range onChange {
			child := onChange[ci]
			child.TriggeredBy = parentID
			if err := p.expandStep(child, ctx, plan, 0); err != nil {
				return fmt.Errorf("expand on_change child %d of %q: %w", ci, step.Name, err)
			}
		}
	}
	return nil
}

// expandSteps recursively expands a list of steps
func (p *Planner) expandSteps(steps []config.Step, ctx *ExpansionContext, plan *Plan, baseStepIndex int) error {
	for i, step := range steps {
		stepIndex := baseStepIndex + i
		if err := p.expandStep(step, ctx, plan, stepIndex); err != nil {
			return err
		}
	}
	return nil
}

// expandInclude expands an include directive with cycle detection
func (p *Planner) expandInclude(step config.Step, ctx *ExpansionContext, plan *Plan, stepIndex int) error {
	if step.Import == nil {
		return fmt.Errorf("include step has nil Include field")
	}

	// Render the include path template
	includePath, err := p.template.Render(*step.Import, ctx.Variables)
	if err != nil {
		return fmt.Errorf("failed to render include path: %w", err)
	}

	// Resolve to absolute path
	absIncludePath, err := resolvePath(includePath, ctx.CurrentDir)
	if err != nil {
		return fmt.Errorf("failed to resolve include path: %w", err)
	}

	// Check for cycles
	if p.seenFiles[absIncludePath] {
		return fmt.Errorf("include cycle detected: %s\nChain: %s", absIncludePath, p.formatIncludeChain())
	}

	// Mark as seen and add to stack
	p.seenFiles[absIncludePath] = true
	defer delete(p.seenFiles, absIncludePath)
	p.inputFiles = append(p.inputFiles, absIncludePath)

	p.includeStack = append(p.includeStack, IncludeFrame{
		FilePath: absIncludePath,
		Line:     1,
		Column:   1,
	})
	defer func() {
		p.includeStack = p.includeStack[:len(p.includeStack)-1]
	}()

	// Read included config
	includedConfig, err := p.readRunConfig(absIncludePath)
	if err != nil {
		return fmt.Errorf("failed to read included config %q: %w", absIncludePath, err)
	}

	// Create new context with updated current directory
	newCtx := &ExpansionContext{
		Variables:  ctx.Variables, // Share variables
		CurrentDir: filepath.Dir(absIncludePath),
		Tags:       ctx.Tags,
		SkipTags:   ctx.SkipTags,
		Names:      ctx.Names,
	}

	// Tags propagate BEFORE expansion. The per-step `Skipped` flag (set by
	// --tags filtering during compilePlanStep) reads the step's own Tags —
	// so an inherited tag has to be present on the step before that runs.
	// Tag propagation never affects plan-time control flow; only execute-
	// time filtering.
	if len(step.Tags) > 0 {
		for i := range includedConfig.Steps {
			includedConfig.Steps[i].Tags = mergeTags(includedConfig.Steps[i].Tags, step.Tags)
		}
	}

	// When propagates AFTER expansion, by patching each emitted plan step.
	// Doing this post-expansion is deliberate: it lets `vars.load` steps in
	// the included file fire unconditionally at plan time so that any
	// subsequent for_each in the same file has the variables it needs.
	// Doing it pre-expansion would skip vars.load on the wrong OS and the
	// dependent for_each would then blow up evaluating a missing variable.
	if step.When != "" {
		stepsBeforeExpand := len(plan.Steps)
		if err := p.expandSteps(includedConfig.Steps, newCtx, plan, stepIndex); err != nil {
			return err
		}
		for i := stepsBeforeExpand; i < len(plan.Steps); i++ {
			if plan.Steps[i].When != "" {
				plan.Steps[i].When = "(" + step.When + ") && (" + plan.Steps[i].When + ")"
			} else {
				plan.Steps[i].When = step.When
			}
		}
		return nil
	}

	return p.expandSteps(includedConfig.Steps, newCtx, plan, stepIndex)
}

// tryExpandUse attempts to expand a `use:` step inline at plan time. Returns
// (true, nil) if the component was loaded and its steps emitted; (false, nil)
// if the step should fall through to the default compilePlanStep path (i.e.
// remote/alias reference, or path whose template substitution still contains
// {{ }} because a register-captured variable hasn't been resolved yet).
//
// Plan-time props/parameters validation is intentionally NOT done here —
// register-dependent props are routine, so enum/type/required checks stay at
// apply time where the final values are known. Defaults are still applied so
// the component's downstream steps see complete props.
func (p *Planner) tryExpandUse(step config.Step, ctx *ExpansionContext, plan *Plan, stepIndex int) (bool, error) {
	render := func(s string) (string, error) {
		return p.template.RenderPreserving(s, ctx.Variables)
	}

	rendered, err := render(step.Use)
	if err != nil {
		return false, fmt.Errorf("step %q: render use: %w", step.Name, err)
	}

	// Defer to apply time if the ref still has unresolved templates or
	// isn't a local path. Remote/alias refs need the resolver (network +
	// module cache), which only runs at apply time.
	if strings.Contains(rendered, "{{") {
		return false, nil
	}
	if config.ComponentRefKindOf(rendered) != config.ComponentRefLocalPath {
		return false, nil
	}

	// Render props (best-effort; unresolved templates survive as literal
	// {{ }} via RenderPreserving). If ANY prop value still has an
	// unresolved template, defer the whole expansion — we don't validate
	// or expand partially, because component-side validation would either
	// false-fail on a templated string (enum/type checks treat "{{ x }}"
	// as a literal) or silently let it through to apply time.
	callerProps := step.Props
	if callerProps != nil {
		if err := renderPropsValue(callerProps, render); err != nil {
			return false, fmt.Errorf("step %q: render props: %w", step.Name, err)
		}
		if propsHaveUnresolvedTemplates(callerProps) {
			return false, nil
		}
	}

	absPath := rendered
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(ctx.CurrentDir, absPath)
	}
	p.inputFiles = append(p.inputFiles, absPath)

	def, err := presets.LoadPresetFromPath(absPath)
	if err != nil {
		return false, fmt.Errorf("step %q: load component %q: %w", step.Name, absPath, err)
	}

	// Full validation now that props are concrete: required, type, enum,
	// and unknown-prop checks all fire here so authoring errors surface
	// at plan time instead of apply time.
	validated, err := presets.ValidateParameters(def, callerProps)
	if err != nil {
		return false, fmt.Errorf("step %q: %w", step.Name, err)
	}

	// ValidateParameters returns (caller's value OR default) for every
	// declared param. Use that as the namespace so downstream templates
	// see defaults filled in.
	paramsNamespace := validated

	// Inject props/parameters into the shared variables map for the
	// duration of expansion; restore on exit so siblings in the parent
	// file don't see them. Mirrors how the preset handler scopes them at
	// apply time (captureContext / restore).
	savedProps, hadProps := ctx.Variables["props"]
	savedParams, hadParams := ctx.Variables["parameters"]
	ctx.Variables["props"] = paramsNamespace
	ctx.Variables["parameters"] = paramsNamespace
	defer func() {
		if hadProps {
			ctx.Variables["props"] = savedProps
		} else {
			delete(ctx.Variables, "props")
		}
		if hadParams {
			ctx.Variables["parameters"] = savedParams
		} else {
			delete(ctx.Variables, "parameters")
		}
	}()

	// Child context shares the Variables map so a `vars.load:` inside the
	// component populates the global scope (downstream consumer templates
	// need palette.* / editor.* etc. in scope). CurrentDir flips to the
	// component's base dir for relative paths in its own includes.
	childCtx := &ExpansionContext{
		Variables:  ctx.Variables,
		CurrentDir: def.BaseDir,
		Tags:       ctx.Tags,
		SkipTags:   ctx.SkipTags,
		Names:      ctx.Names,
	}

	// Clone the component's steps + propagate parent tags before expansion.
	steps := make([]config.Step, len(def.Steps))
	for i, s := range def.Steps {
		steps[i] = *s.Clone()
		if len(step.Tags) > 0 {
			steps[i].Tags = mergeTags(steps[i].Tags, step.Tags)
		}
	}

	// When-propagation mirrors expandInclude: expand first, then AND the
	// parent's `when:` into each emitted child. Doing it post-expansion
	// lets the component's own `vars.load` always fire at plan time.
	if step.When != "" {
		before := len(plan.Steps)
		if err := p.expandSteps(steps, childCtx, plan, stepIndex); err != nil {
			return false, err
		}
		for i := before; i < len(plan.Steps); i++ {
			if plan.Steps[i].When != "" {
				plan.Steps[i].When = "(" + step.When + ") && (" + plan.Steps[i].When + ")"
			} else {
				plan.Steps[i].When = step.When
			}
		}
		return true, nil
	}

	if err := p.expandSteps(steps, childCtx, plan, stepIndex); err != nil {
		return false, err
	}
	return true, nil
}

// mergeTags returns the union of a and b, preserving order and de-duplicating.
func mergeTags(a, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	seen := make(map[string]struct{}, len(a)+len(b))
	for _, t := range a {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	for _, t := range b {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// expandForEach expands a step's for_each loop. Accepts both scalar
// (variable expression) and sequence (literal list) forms.
func (p *Planner) expandWithItems(step config.Step, ctx *ExpansionContext, plan *Plan) error {
	if step.ForEach == nil {
		return fmt.Errorf("for_each step has nil ForEach field")
	}

	var items []interface{}
	var loopExpr string

	if len(step.ForEach.Items) > 0 {
		// Literal list form: use items directly.
		items = step.ForEach.Items
		loopExpr = "<literal list>"
	} else {
		// Scalar form: resolve via variables directly. Do NOT pass the
		// expression through the pongo2 renderer first — pongo2 stringifies
		// a slice variable via reflect.Value.String() ("<[]interface {} Value>")
		// rather than the underlying slice, which then tokenizes into
		// nonsense fragments instead of iterating elements.
		exprStr := stripTemplateExpr(step.ForEach.Expr)
		var err error
		items, err = p.evaluateItemsExpression(exprStr, ctx.Variables)
		if err != nil {
			return fmt.Errorf("failed to evaluate for_each: %w", err)
		}
		loopExpr = step.ForEach.Expr
	}

	// Expand step for each item
	for i, item := range items {
		loopCtx := &config.LoopContext{
			Type:           "for_each",
			Item:           item,
			Index:          i,
			First:          i == 0,
			Last:           i == len(items)-1,
			LoopExpression: loopExpr,
		}

		// Create new context with loop variables
		itemCtx := p.copyContextWithLoopVars(ctx, loopCtx)

		// Compile step with loop context
		planStep, err := p.compilePlanStep(step, itemCtx, loopCtx)
		if err != nil {
			return fmt.Errorf("failed to compile step %q iteration %d: %w", step.Name, i, err)
		}
		plan.Steps = append(plan.Steps, planStep)
	}

	return nil
}

// expandWithFileTree expands a step with with_filetree loop
func (p *Planner) expandWithFileTree(step config.Step, ctx *ExpansionContext, plan *Plan) error {
	if step.ForEachFile == nil {
		return fmt.Errorf("with_filetree step has nil WithFileTree field")
	}

	// Render the path template
	treePath, err := p.template.Render(*step.ForEachFile, ctx.Variables)
	if err != nil {
		return fmt.Errorf("failed to render with_filetree path: %w", err)
	}

	// Get file tree
	items, err := p.fileTree.GetFileTree(treePath, ctx.CurrentDir, ctx.Variables)
	if err != nil {
		return fmt.Errorf("failed to walk file tree: %w", err)
	}

	// CRITICAL: Sort for determinism
	sort.Slice(items, func(i, j int) bool {
		return items[i].Src < items[j].Src
	})

	// Expand step for each item
	for i, item := range items {
		// Calculate directory depth from item path
		// For filetree items: depth = number of "/" in path (excluding leading "/")
		depth := 0
		trimmedPath := strings.TrimPrefix(item.Path, "/")
		if trimmedPath != "" {
			depth = strings.Count(trimmedPath, "/")
		}

		loopCtx := &config.LoopContext{
			Type:           "with_filetree",
			Item:           item,
			Index:          i,
			First:          i == 0,
			Last:           i == len(items)-1,
			LoopExpression: *step.ForEachFile,
			Depth:          depth,
		}

		// Create new context with loop variables
		itemCtx := p.copyContextWithLoopVars(ctx, loopCtx)

		// Compile step with loop context
		planStep, err := p.compilePlanStep(step, itemCtx, loopCtx)
		if err != nil {
			return fmt.Errorf("failed to compile step %q iteration %d: %w", step.Name, i, err)
		}
		plan.Steps = append(plan.Steps, planStep)
	}

	return nil
}

// expandVars merges variables into the context.
//
// F037: resolve `!secret` markers in step.Vars BEFORE templating and
// merging. Pre-fix the marker sentinel passed verbatim through Render
// (no `{{...}}` to interpolate) and ended up in ctx.Variables as a
// literal `__MOONCAKE_SECRET_v1_DO_NOT_EDIT__:env:FOO` string, which
// subsequent steps then templated into shell commands / file content
// — leaking the marker (and missing the redactor denylist registration
// that resolver.Resolve performs as a side-effect). The standalone
// vars action never reaches executor.dispatchRunner, so this was the
// only call site missing the apply-time resolve.
func (p *Planner) expandVars(step config.Step, ctx *ExpansionContext) error {
	if step.Vars == nil {
		return fmt.Errorf("vars step has nil Vars field")
	}

	// Resolve secret markers into concrete values (and register them
	// with the planner's redactor) before rendering. Mutates step in
	// place — safe because expandVars is called once per planning pass
	// and the planner owns the step copy.
	if err := resolver.Resolve(&step, p.redactor); err != nil {
		return fmt.Errorf("resolve vars secrets: %w", err)
	}

	// Merge vars into context
	for k, v := range *step.Vars {
		// Render value if it's a string (template)
		if strVal, ok := v.(string); ok {
			rendered, err := p.template.Render(strVal, ctx.Variables)
			if err != nil {
				return fmt.Errorf("failed to render var %q: %w", k, err)
			}
			ctx.Variables[k] = rendered
		} else {
			ctx.Variables[k] = v
		}
	}

	return nil
}

// expandIncludeVars loads variables from an external file
func (p *Planner) expandIncludeVars(step config.Step, ctx *ExpansionContext) error {
	if step.VarsLoad == nil {
		return fmt.Errorf("include_vars step has nil IncludeVars field")
	}

	// Render the vars path template
	varsPath, err := p.template.Render(*step.VarsLoad, ctx.Variables)
	if err != nil {
		return fmt.Errorf("failed to render include_vars path: %w", err)
	}

	// Resolve to absolute path
	absVarsPath, err := resolvePath(varsPath, ctx.CurrentDir)
	if err != nil {
		return fmt.Errorf("failed to resolve vars path: %w", err)
	}

	// Read variables
	vars, err := config.ReadVariables(absVarsPath)
	if err != nil {
		return fmt.Errorf("failed to read variables from %q: %w", absVarsPath, err)
	}

	// Merge into context
	for k, v := range vars {
		ctx.Variables[k] = v
	}

	return nil
}

// shouldProcessAtPlanTime evaluates whether a step should be processed during planning.
// Returns true if the step should be processed, false if it should be skipped.
// For vars and include_vars steps with when conditions, we try to evaluate the condition
// at plan time. If it evaluates to false, we skip processing the step.
func (p *Planner) shouldProcessAtPlanTime(step config.Step, ctx *ExpansionContext) bool {
	// If no when condition, always process
	if step.When == "" {
		return true
	}

	// Try to evaluate the when condition with current variables
	// If evaluation fails (e.g., references undefined variables), we assume true
	// and let runtime handle the condition
	evaluator := expression.NewGovaluateEvaluator()

	// First, render any templates in the when condition
	renderedWhen, err := p.template.Render(step.When, ctx.Variables)
	if err != nil {
		// Template rendering failed, assume we should process (runtime will handle it)
		return true
	}

	// Evaluate the expression
	result, err := evaluator.Evaluate(renderedWhen, ctx.Variables)
	if err != nil {
		// Evaluation failed, assume we should process (runtime will handle it)
		return true
	}

	// Convert result to bool
	boolResult, ok := result.(bool)
	if !ok {
		// Not a boolean, assume we should process
		return true
	}

	// Return the evaluation result
	return boolResult
}

// compilePlanStep enhances a config.Step with plan metadata
func (p *Planner) compilePlanStep(step config.Step, ctx *ExpansionContext, loopCtx *config.LoopContext) (config.Step, error) {
	// Generate step ID
	p.stepIDCounter++
	stepID := fmt.Sprintf("step-%04d", p.stepIDCounter)

	// Build origin using step's SourceLocation if available
	origin := p.buildOriginForStep(&step)

	// Render step name
	if step.Name != "" {
		rendered, err := p.template.Render(step.Name, ctx.Variables)
		if err != nil {
			return config.Step{}, fmt.Errorf("failed to render step name: %w", err)
		}
		step.Name = rendered
	}

	// Check if step should be skipped by tags. Spec-50: an additional
	// `--step-filter name=<x>` filter ANDs with the tag check — both must
	// pass for the step to run. MT-58: `--skip-tags` is an exclusion
	// filter — if the step's tags intersect ctx.SkipTags, skip it.
	skipped := !utils.MatchesTags(step.Tags, ctx.Tags) ||
		utils.MatchesSkipTags(step.Tags, ctx.SkipTags) ||
		!utils.MatchesNames(step.Name, ctx.Names)

	// Render action templates
	err := p.renderActionTemplates(&step, ctx)
	if err != nil {
		return config.Step{}, err
	}

	// Clear loop directives (already expanded)
	step.ForEach = nil
	step.ForEachFile = nil

	// Clear compile-time directives (already processed)
	step.Import = nil
	step.VarsLoad = nil
	step.Vars = nil

	// Add plan metadata
	step.ID = stepID
	step.ActionType = step.DetermineActionType()
	step.Origin = &origin
	step.Skipped = skipped
	step.LoopContext = loopCtx

	// Validate platform support
	if err := validatePlatformSupport(step.ActionType); err != nil {
		return config.Step{}, fmt.Errorf("platform validation failed for step %q: %w", step.Name, err)
	}

	return step, nil
}

// renderActionTemplates renders plan-time templates for the step's active action field.
// Uses RenderPreserving so templates referencing execute-time variables are
// preserved as {{ expr }} in plan output rather than silently replaced with "".
// All 28 action structs are covered automatically via walkAndRender.
func (p *Planner) renderActionTemplates(step *config.Step, ctx *ExpansionContext) error {
	render := func(s string) (string, error) {
		return p.template.RenderPreserving(s, ctx.Variables)
	}

	// spec-67: `use:` is a string action; `props:` is its sibling map (not an
	// action field). Render both at plan time so downstream consumers — the
	// preset handler's prop validation, vars.load expansions inside the
	// component, etc. — see resolved values instead of raw {{ }} expressions.
	if step.Use != "" {
		rendered, err := render(step.Use)
		if err != nil {
			return fmt.Errorf("step %q: %w", step.Name, err)
		}
		step.Use = rendered
		// Clone Props before rendering: for_each iterates the same outer
		// step value N times, and renderPropsValue mutates the map in place.
		// Without the clone, iteration 0 resolves "{{ item }}" to its first
		// value and iterations 1+ see a literal (no template left to render).
		// Mirrors the shallow-copy guard the non-use path applies at line 1023.
		if step.Props != nil {
			cloned, _ := clonePropsValue(step.Props).(map[string]interface{})
			step.Props = cloned
		}
		if err := renderPropsValue(step.Props, render); err != nil {
			return fmt.Errorf("step %q: %w", step.Name, err)
		}
		return nil
	}

	rv := reflect.ValueOf(step).Elem()
	for _, i := range config.ActionFieldIndices() {
		fv := rv.Field(i)
		// Only pointer-to-struct action fields are eligible for template
		// rendering via walkAndRender. Non-pointer action fields (e.g.
		// spec-67 Use) are handled above.
		if fv.Kind() != reflect.Pointer {
			continue
		}
		if fv.IsNil() {
			continue
		}
		if fv.Type().Elem().Kind() != reflect.Struct {
			continue
		}
		// Shallow-copy the action struct. walkAndRender deep-copies nested
		// pointer-to-struct fields before mutating them.
		orig := fv.Elem()
		cp := reflect.New(orig.Type())
		cp.Elem().Set(orig)
		fv.Set(cp)
		if err := walkAndRender(cp.Elem(), render, ctx.CurrentDir); err != nil {
			return fmt.Errorf("step %q: %w", step.Name, err)
		}
		break
	}
	return nil
}

// clonePropsValue returns a deep copy of a props value (map/slice/scalar).
// Used by renderActionTemplates so that for_each iterations don't mutate
// each other's templates — see the use: branch at line 994. Mirrors the
// shape walked by renderPropsValue.
func clonePropsValue(v interface{}) interface{} {
	switch x := v.(type) {
	case nil:
		return nil
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, sub := range x {
			out[k] = clonePropsValue(sub)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, sub := range x {
			out[i] = clonePropsValue(sub)
		}
		return out
	default:
		return v
	}
}

// renderPropsValue walks a props map/slice/string value and renders every
// string leaf through the planner's template engine. Nested structures
// (map-of-map, list-of-map, etc.) are traversed in place; non-string scalars
// pass through unchanged.
func renderPropsValue(v interface{}, render func(string) (string, error)) error {
	switch x := v.(type) {
	case nil:
		return nil
	case map[string]interface{}:
		for k, sub := range x {
			if s, ok := sub.(string); ok {
				r, err := render(s)
				if err != nil {
					return err
				}
				x[k] = r
				continue
			}
			if err := renderPropsValue(sub, render); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, sub := range x {
			if s, ok := sub.(string); ok {
				r, err := render(s)
				if err != nil {
					return err
				}
				x[i] = r
				continue
			}
			if err := renderPropsValue(sub, render); err != nil {
				return err
			}
		}
	}
	return nil
}

// propsHaveUnresolvedTemplates reports whether any string leaf in the
// props tree still contains a `{{` after RenderPreserving has run. Used
// by tryExpandUse to decide whether plan-time expansion is safe; if any
// prop value depends on apply-time state (registered output, runtime
// fact), defer the whole expansion so apply-time validation gets a shot.
func propsHaveUnresolvedTemplates(v interface{}) bool {
	switch x := v.(type) {
	case string:
		return strings.Contains(x, "{{")
	case map[string]interface{}:
		for _, sub := range x {
			if propsHaveUnresolvedTemplates(sub) {
				return true
			}
		}
	case []interface{}:
		for _, sub := range x {
			if propsHaveUnresolvedTemplates(sub) {
				return true
			}
		}
	}
	return false
}

// walkAndRender recursively renders all string fields of an action struct using
// RenderPreserving. Fields tagged plan:"path" are additionally resolved to
// absolute paths using currentDir. Nested pointer-to-struct fields are
// deep-copied before mutation to avoid touching the original config.
// Handles: string, *string, *struct (deep copy + recurse), []string,
// map[string]string, map[string]interface{} (string-valued entries only).
//
// F024: map[string]interface{} entries are unwrapped from the interface
// and rendered when the underlying value is a string. This covers the
// os.systemd Unit/Service/Timer/Socket/Install sections, text.patch.json
// Set/Merge, text.patch.yaml Set/Merge, and use.With — all of which
// declare templated values via these YAML-mapping shapes. Non-string
// entries (numbers, bools, nested maps, lists) are passed through
// unchanged; nested templates inside non-string entries would need a
// deeper recursive walk and are not common enough to handle inline.
// Track separately if a user hits that.
func walkAndRender(rv reflect.Value, render func(string) (string, error), currentDir string) error {
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		fv := rv.Field(i)
		sf := rt.Field(i)
		isPath := sf.Tag.Get("plan") == "path"

		switch fv.Kind() {
		case reflect.String:
			if fv.String() == "" {
				continue
			}
			rendered, err := render(fv.String())
			if err != nil {
				return fmt.Errorf("%s: %w", sf.Name, err)
			}
			if isPath && shouldJoinPlanPath(rendered) {
				rendered = filepath.Join(currentDir, rendered)
			}
			fv.SetString(rendered)

		case reflect.Pointer:
			if fv.IsNil() {
				continue
			}
			switch fv.Type().Elem().Kind() {
			case reflect.String:
				if fv.Elem().String() == "" {
					continue
				}
				rendered, err := render(fv.Elem().String())
				if err != nil {
					return fmt.Errorf("%s: %w", sf.Name, err)
				}
				if isPath && shouldJoinPlanPath(rendered) {
					rendered = filepath.Join(currentDir, rendered)
				}
				cp := reflect.New(fv.Type().Elem())
				cp.Elem().SetString(rendered)
				fv.Set(cp)
			case reflect.Struct:
				orig := fv.Elem()
				cp := reflect.New(orig.Type())
				cp.Elem().Set(orig)
				fv.Set(cp)
				if err := walkAndRender(cp.Elem(), render, currentDir); err != nil {
					return err
				}
			}

		case reflect.Slice:
			if fv.Type().Elem().Kind() != reflect.String {
				continue
			}
			for j := 0; j < fv.Len(); j++ {
				if fv.Index(j).String() == "" {
					continue
				}
				rendered, err := render(fv.Index(j).String())
				if err != nil {
					return fmt.Errorf("%s[%d]: %w", sf.Name, j, err)
				}
				fv.Index(j).SetString(rendered)
			}

		case reflect.Map:
			if fv.Type().Key().Kind() != reflect.String {
				continue
			}
			elemKind := fv.Type().Elem().Kind()
			if elemKind != reflect.String && elemKind != reflect.Interface {
				continue
			}
			for _, k := range fv.MapKeys() {
				v := fv.MapIndex(k)
				// map[string]interface{}: unwrap to the concrete value so
				// we can decide whether to render. Non-string concretes
				// (numbers, bools, nested maps, lists) pass through.
				if v.Kind() == reflect.Interface {
					v = v.Elem()
				}
				if v.Kind() != reflect.String {
					continue
				}
				s := v.String()
				if s == "" {
					continue
				}
				rendered, err := render(s)
				if err != nil {
					return fmt.Errorf("%s[%s]: %w", sf.Name, k.String(), err)
				}
				fv.SetMapIndex(k, reflect.ValueOf(rendered))
			}
		}
	}
	return nil
}

// shouldJoinPlanPath reports whether the plan-time absolute-path resolver
// should prepend currentDir to a rendered path string.
//
// Skipped when:
//   - rendered is already absolute, or
//   - rendered begins with ~/ (apply-time home expansion), or
//   - rendered is exactly ~, or
//   - rendered still contains `{{` (an apply-time template that
//     RenderPreserving deferred). The deferred portion may resolve to
//     an absolute path (e.g. `{{ env.HOME }}/foo` → `/home/aleh/foo`),
//     but `filepath.IsAbs` can't see that through the literal `{{` —
//     so a naive join would bake the wrong relative-vs-absolute
//     decision into the step config, and apply-time `ExpandPath` has
//     no way to recover the original intent.
func shouldJoinPlanPath(rendered string) bool {
	if filepath.IsAbs(rendered) {
		return false
	}
	if strings.HasPrefix(rendered, "~/") || rendered == "~" {
		return false
	}
	if strings.Contains(rendered, "{{") {
		return false
	}
	return true
}

// buildOrigin creates an Origin from the current include stack
// buildOriginForStep builds origin using step's SourceLocation if available
func (p *Planner) buildOriginForStep(step *config.Step) config.Origin {
	var filePath string
	var line, column int

	// Use step's SourceLocation if available (from Reader)
	if step.SourceLocation != nil {
		// Get file path from current include frame
		if len(p.includeStack) > 0 {
			currentFrame := p.includeStack[len(p.includeStack)-1]
			filePath = currentFrame.FilePath
		}

		// Use the actual source location from YAML parsing
		line = step.SourceLocation.Line
		column = step.SourceLocation.Column
	} else if len(p.includeStack) > 0 {
		// Fallback to old behavior (for backward compatibility)
		currentFrame := p.includeStack[len(p.includeStack)-1]
		filePath = currentFrame.FilePath
		line = currentFrame.Line
		column = currentFrame.Column
	}

	// Build include chain
	var chain []string
	if len(p.includeStack) > 1 {
		chain = make([]string, 0, len(p.includeStack)-1)
		for i := 0; i < len(p.includeStack)-1; i++ {
			frame := p.includeStack[i]
			chain = append(chain, fmt.Sprintf("%s:%d", frame.FilePath, frame.Line))
		}
	}

	return config.Origin{
		FilePath:     filePath,
		Line:         line,
		Column:       column,
		IncludeChain: chain,
	}
}

// formatIncludeChain formats the include chain for error messages
func (p *Planner) formatIncludeChain() string {
	parts := make([]string, len(p.includeStack))
	for i, frame := range p.includeStack {
		parts[i] = fmt.Sprintf("%s:%d", frame.FilePath, frame.Line)
	}
	return strings.Join(parts, " -> ")
}

// copyContextWithLoopVars creates a new context with loop variables added
func (p *Planner) copyContextWithLoopVars(ctx *ExpansionContext, loopCtx *config.LoopContext) *ExpansionContext {
	// Create loop variables
	loopVars := map[string]interface{}{
		"item":  loopCtx.Item,
		"index": loopCtx.Index,
		"first": loopCtx.First,
		"last":  loopCtx.Last,
	}

	// Merge context variables with loop variables (loop variables take precedence)
	newVars := utils.MergeVariables(ctx.Variables, loopVars)

	return &ExpansionContext{
		Variables:  newVars,
		CurrentDir: ctx.CurrentDir,
		Tags:       ctx.Tags,
		SkipTags:   ctx.SkipTags,
		Names:      ctx.Names,
	}
}

// stripTemplateExpr returns the inner expression of a "{{ ... }}" wrapper,
// trimmed of surrounding whitespace. If the input is not wrapped, returns
// the trimmed input unchanged. Used by for_each so that a scalar template
// like "{{ packages }}" is resolved against vars directly, bypassing the
// pongo2 renderer which would stringify the slice via reflect.Value.String().
func stripTemplateExpr(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") && len(s) >= 4 {
		return strings.TrimSpace(s[2 : len(s)-2])
	}
	return s
}

// evaluateItemsExpression evaluates a with_items expression
// Supports direct variable references ("items"), dot notation
// ("parameters.items"), and rendered list/scalar literals via the shared
// resolver in internal/template.
func (p *Planner) evaluateItemsExpression(expr string, vars map[string]interface{}) ([]interface{}, error) {
	items, err := template.ResolveList(expr, vars, expression.NewExprEvaluator())
	if err != nil {
		return nil, fmt.Errorf("with_items expression %q evaluation failed: %w", expr, err)
	}
	return items, nil
}

// convertToSlice converts a value to []interface{}. Retained for tests.
//
//nolint:unused // referenced only from *_test.go which lint skips.
func convertToSlice(val interface{}, expr string) ([]interface{}, error) {
	switch v := val.(type) {
	case []interface{}:
		return v, nil
	case []string:
		items := make([]interface{}, len(v))
		for i, s := range v {
			items[i] = s
		}
		return items, nil
	default:
		return nil, fmt.Errorf("with_items expression %q is not a list (got %T)", expr, val)
	}
}
