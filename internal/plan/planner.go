package plan

import (
	"fmt"
	"path/filepath"
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
}

// PlannerConfig holds configuration for building a plan
type PlannerConfig struct {
	ConfigPath string
	Variables  map[string]interface{}
	Tags       []string
}

// NewPlanner creates a new Planner instance
func NewPlanner() *Planner {
	pathExpander := pathutil.NewPathExpander(template.NewPongo2Renderer())
	return &Planner{
		template:     template.NewPongo2Renderer(),
		pathUtil:     pathExpander,
		fileTree:     filetree.NewWalker(pathExpander),
		seenFiles:    make(map[string]bool),
		locationMap:  make(map[int]*IncludeFrame),
	}
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
	// Read config with validation
	runConfig, err := p.readRunConfig(cfg.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Initialize plan
	plan := &Plan{
		Version:     "1.0",
		GeneratedAt: time.Now(),
		RootFile:    cfg.ConfigPath,
		Steps:       make([]config.Step, 0),
		InitialVars: cfg.Variables,
		Tags:        cfg.Tags,
	}

	// Merge global vars from config with provided vars (provided vars take precedence)
	variables := utils.MergeVariables(runConfig.Vars, cfg.Variables)

	// Inject system facts (ansible_os_family, ansible_distribution, etc.)
	// These are added after config vars but before expansion, so templates can use them
	systemFacts := facts.Collect()
	for k, v := range systemFacts.ToMap() {
		variables[k] = v
	}

	// Update plan's InitialVars to include system facts
	// This ensures the facts are available during execution for 'when' conditions
	plan.InitialVars = variables

	// Create expansion context
	ctx := &ExpansionContext{
		Variables:  variables,
		CurrentDir: filepath.Dir(cfg.ConfigPath),
		Tags:       cfg.Tags,
	}

	// Mark root file as seen
	absPath, err := filepath.Abs(cfg.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}
	p.seenFiles[absPath] = true

	// Push root frame
	p.includeStack = append(p.includeStack, IncludeFrame{
		FilePath: absPath,
		Line:     1,
		Column:   1,
	})

	// Expand all steps
	if err := p.expandSteps(runConfig.Steps, ctx, plan, 0); err != nil {
		return nil, err
	}

	return plan, nil
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
		Steps:   parsedConfig.Steps,
	}

	return runConfig, nil
}

// expandStep dispatches a single step to the appropriate expansion handler
func (p *Planner) expandStep(step config.Step, ctx *ExpansionContext, plan *Plan, stepIndex int) error {
	// Handle include directives
	if step.Include != nil {
		return p.expandInclude(step, ctx, plan, stepIndex)
	}

	// Handle loop constructs
	if step.WithItems != nil {
		return p.expandWithItems(step, ctx, plan)
	}
	if step.WithFileTree != nil {
		return p.expandWithFileTree(step, ctx, plan)
	}

	// Handle variable operations (skip if when condition is false at plan time)
	if step.Vars != nil {
		if !p.shouldProcessAtPlanTime(step, ctx) {
			return nil // Skip this step
		}
		return p.expandVars(step, ctx)
	}
	if step.IncludeVars != nil {
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
	plan.Steps = append(plan.Steps, planStep)
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
	if step.Include == nil {
		return fmt.Errorf("include step has nil Include field")
	}

	// Render the include path template
	includePath, err := p.template.Render(*step.Include, ctx.Variables)
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
	}

	// If the include step has a 'when' condition, propagate it to all included steps
	// This ensures that if include is conditional, all its steps inherit that condition
	if step.When != "" {
		// Read and modify included steps to add parent's when condition
		stepsBeforeExpand := len(plan.Steps)
		err = p.expandSteps(includedConfig.Steps, newCtx, plan, stepIndex)
		if err != nil {
			return err
		}

		// Apply the include's when condition to all newly added steps
		parentWhen := step.When
		for i := stepsBeforeExpand; i < len(plan.Steps); i++ {
			// If step already has a when condition, combine with AND logic
			if plan.Steps[i].When != "" {
				combined := "(" + parentWhen + ") && (" + plan.Steps[i].When + ")"
				plan.Steps[i].When = combined
			} else {
				plan.Steps[i].When = parentWhen
			}
		}
		return nil
	}

	// Recursively expand included steps (no when condition to propagate)
	return p.expandSteps(includedConfig.Steps, newCtx, plan, stepIndex)
}

// expandWithItems expands a step with with_items loop
func (p *Planner) expandWithItems(step config.Step, ctx *ExpansionContext, plan *Plan) error {
	if step.WithItems == nil {
		return fmt.Errorf("with_items step has nil WithItems field")
	}

	// Render the items expression
	itemsExpr, err := p.template.Render(*step.WithItems, ctx.Variables)
	if err != nil {
		return fmt.Errorf("failed to render with_items expression: %w", err)
	}

	// For now, we'll need to evaluate the expression to get the items
	// This requires the expression evaluator
	// TODO: Implement proper expression evaluation
	// For now, assume it's a simple list variable reference like "{{ packages }}"

	// Extract variable name from template (simplified)
	items, err := p.evaluateItemsExpression(itemsExpr, ctx.Variables)
	if err != nil {
		return fmt.Errorf("failed to evaluate with_items: %w", err)
	}

	// Expand step for each item
	for i, item := range items {
		loopCtx := &config.LoopContext{
			Type:           "with_items",
			Item:           item,
			Index:          i,
			First:          i == 0,
			Last:           i == len(items)-1,
			LoopExpression: *step.WithItems,
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
	if step.WithFileTree == nil {
		return fmt.Errorf("with_filetree step has nil WithFileTree field")
	}

	// Render the path template
	treePath, err := p.template.Render(*step.WithFileTree, ctx.Variables)
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
			LoopExpression: *step.WithFileTree,
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

// expandVars merges variables into the context
func (p *Planner) expandVars(step config.Step, ctx *ExpansionContext) error {
	if step.Vars == nil {
		return fmt.Errorf("vars step has nil Vars field")
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
	if step.IncludeVars == nil {
		return fmt.Errorf("include_vars step has nil IncludeVars field")
	}

	// Render the vars path template
	varsPath, err := p.template.Render(*step.IncludeVars, ctx.Variables)
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

	// Build origin
	origin := p.buildOrigin()

	// Render step name
	if step.Name != "" {
		rendered, err := p.template.Render(step.Name, ctx.Variables)
		if err != nil {
			return config.Step{}, fmt.Errorf("failed to render step name: %w", err)
		}
		step.Name = rendered
	}

	// Check if step should be skipped by tags
	skipped := p.shouldSkipByTags(step.Tags, ctx.Tags)

	// Render action templates
	err := p.renderActionTemplates(&step, ctx)
	if err != nil {
		return config.Step{}, err
	}

	// Clear loop directives (already expanded)
	step.WithItems = nil
	step.WithFileTree = nil

	// Clear compile-time directives (already processed)
	step.Include = nil
	step.IncludeVars = nil
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

// renderActionTemplates renders templates in a step's action fields
//nolint:gocyclo,dupl // Complexity necessary for handling all action types; similar patterns are intentional
func (p *Planner) renderActionTemplates(step *config.Step, ctx *ExpansionContext) error {
	if step.Shell != nil {
		// Make a deep copy of Shell to avoid modifying shared pointer
		shellCopy := *step.Shell
		step.Shell = &shellCopy

		// Render shell command
		command, err := p.template.Render(step.Shell.Cmd, ctx.Variables)
		if err != nil {
			return fmt.Errorf("failed to render shell command: %w", err)
		}
		step.Shell.Cmd = command
	}

	if step.File != nil {
		// Make a deep copy of File to avoid modifying shared pointer
		fileCopy := *step.File
		step.File = &fileCopy

		// Render file fields
		path, err := p.template.Render(step.File.Path, ctx.Variables)
		if err != nil {
			return fmt.Errorf("failed to render file path: %w", err)
		}
		step.File.Path = path

		if step.File.Content != "" {
			content, err := p.template.Render(step.File.Content, ctx.Variables)
			if err != nil {
				return fmt.Errorf("failed to render file content: %w", err)
			}
			step.File.Content = content
		}
	}

	if step.Template != nil {
		// Make a deep copy of Template to avoid modifying shared pointer
		templateCopy := *step.Template
		step.Template = &templateCopy

		// Render and resolve template fields
		src, err := p.template.Render(step.Template.Src, ctx.Variables)
		if err != nil {
			return fmt.Errorf("failed to render template src: %w", err)
		}
		// Resolve relative path to absolute based on current directory
		if !filepath.IsAbs(src) {
			src = filepath.Join(ctx.CurrentDir, src)
		}
		step.Template.Src = src

		dest, err := p.template.Render(step.Template.Dest, ctx.Variables)
		if err != nil {
			return fmt.Errorf("failed to render template dest: %w", err)
		}
		step.Template.Dest = dest
	}

	if step.Copy != nil {
		// Make a deep copy of Copy to avoid modifying shared pointer
		copyCopy := *step.Copy
		step.Copy = &copyCopy

		// Render and resolve source path
		src, err := p.template.Render(step.Copy.Src, ctx.Variables)
		if err != nil {
			return fmt.Errorf("failed to render copy src: %w", err)
		}
		// Resolve relative path to absolute based on current directory
		if !filepath.IsAbs(src) {
			src = filepath.Join(ctx.CurrentDir, src)
		}
		step.Copy.Src = src

		// Render destination path
		dest, err := p.template.Render(step.Copy.Dest, ctx.Variables)
		if err != nil {
			return fmt.Errorf("failed to render copy dest: %w", err)
		}
		step.Copy.Dest = dest
	}

	if step.Unarchive != nil {
		// Make a deep copy of Unarchive to avoid modifying shared pointer
		unarchiveCopy := *step.Unarchive
		step.Unarchive = &unarchiveCopy

		// Render and resolve source path
		src, err := p.template.Render(step.Unarchive.Src, ctx.Variables)
		if err != nil {
			return fmt.Errorf("failed to render unarchive src: %w", err)
		}
		// Resolve relative path to absolute based on current directory
		if !filepath.IsAbs(src) {
			src = filepath.Join(ctx.CurrentDir, src)
		}
		step.Unarchive.Src = src

		// Render destination path
		dest, err := p.template.Render(step.Unarchive.Dest, ctx.Variables)
		if err != nil {
			return fmt.Errorf("failed to render unarchive dest: %w", err)
		}
		step.Unarchive.Dest = dest
	}

	if step.File != nil && step.File.Src != "" {
		// Make a deep copy of File to avoid modifying shared pointer (already done above)
		// Just need to resolve the Src field if it's a link operation

		// Render and resolve source path
		src, err := p.template.Render(step.File.Src, ctx.Variables)
		if err != nil {
			return fmt.Errorf("failed to render file src: %w", err)
		}
		// Resolve relative path to absolute based on current directory
		if !filepath.IsAbs(src) {
			src = filepath.Join(ctx.CurrentDir, src)
		}
		step.File.Src = src
	}

	if step.Service != nil {
		// Make a deep copy of Service to avoid modifying shared pointer
		serviceCopy := *step.Service
		step.Service = &serviceCopy

		// Resolve unit.src_template if present
		if serviceCopy.Unit != nil && serviceCopy.Unit.SrcTemplate != "" {
			rendered, err := p.template.Render(serviceCopy.Unit.SrcTemplate, ctx.Variables)
			if err != nil {
				return fmt.Errorf("failed to render service unit src_template: %w", err)
			}
			// Resolve relative path to absolute based on current directory
			if !filepath.IsAbs(rendered) {
				rendered = filepath.Join(ctx.CurrentDir, rendered)
			}
			serviceCopy.Unit.SrcTemplate = rendered
		}

		// Resolve dropin.src_template if present
		if serviceCopy.Dropin != nil && serviceCopy.Dropin.SrcTemplate != "" {
			rendered, err := p.template.Render(serviceCopy.Dropin.SrcTemplate, ctx.Variables)
			if err != nil {
				return fmt.Errorf("failed to render service dropin src_template: %w", err)
			}
			// Resolve relative path to absolute based on current directory
			if !filepath.IsAbs(rendered) {
				rendered = filepath.Join(ctx.CurrentDir, rendered)
			}
			serviceCopy.Dropin.SrcTemplate = rendered
		}

		step.Service = &serviceCopy
	}

	return nil
}

// buildOrigin creates an Origin from the current include stack
func (p *Planner) buildOrigin() config.Origin {
	// Get location from the current frame
	var filePath string
	var line, column int

	if len(p.includeStack) > 0 {
		currentFrame := p.includeStack[len(p.includeStack)-1]
		filePath = currentFrame.FilePath
		line = currentFrame.Line
		column = currentFrame.Column
	}

	// Build include chain
	chain := make([]string, 0, len(p.includeStack)-1)
	for i := 0; i < len(p.includeStack)-1; i++ {
		frame := p.includeStack[i]
		chain = append(chain, fmt.Sprintf("%s:%d", frame.FilePath, frame.Line))
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
	chain := ""
	for i, part := range parts {
		if i > 0 {
			chain += " -> "
		}
		chain += part
	}
	return chain
}

// shouldSkipByTags checks if a step should be skipped based on tag filtering
func (p *Planner) shouldSkipByTags(stepTags []string, filterTags []string) bool {
	// If no filter tags specified, don't skip
	if len(filterTags) == 0 {
		return false
	}

	// If step has no tags, skip it (tags filter is active)
	if len(stepTags) == 0 {
		return true
	}

	// Check if any step tag matches any filter tag
	for _, filterTag := range filterTags {
		for _, stepTag := range stepTags {
			if stepTag == filterTag {
				return false // Match found, don't skip
			}
		}
	}

	// No match found, skip
	return true
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
	}
}

// evaluateItemsExpression evaluates a with_items expression
// Supports both direct variable references (items) and dot notation (parameters.items)
func (p *Planner) evaluateItemsExpression(expr string, vars map[string]interface{}) ([]interface{}, error) {
	// First, try direct variable lookup for simple cases (e.g., "items")
	if val, ok := vars[expr]; ok {
		return convertToSlice(val, expr)
	}

	// If not found directly, use expression evaluator to support dot notation (e.g., "parameters.items")
	evaluator := expression.NewExprEvaluator()
	result, err := evaluator.Evaluate(expr, vars)
	if err != nil {
		return nil, fmt.Errorf("with_items expression %q evaluation failed: %w", expr, err)
	}

	return convertToSlice(result, expr)
}

// convertToSlice converts a value to []interface{} slice
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
