// Package executor implements the core execution engine for mooncake configuration plans.
//
// The executor is responsible for:
//   - Loading and validating configuration plans
//   - Expanding steps (loops, includes, presets)
//   - Evaluating conditions (when, unless, creates)
//   - Dispatching actions to handlers
//   - Managing execution context and variables
//   - Tracking results and statistics
//   - Emitting events for observability
//   - Handling dry-run mode
//   - Supporting privilege escalation (sudo/become)
//
// # Architecture
//
// The executor follows a pipeline architecture:
//
//	Plan Loading → Step Expansion → Condition Evaluation → Action Dispatch → Result Handling
//
// Each step goes through:
//  1. Pre-execution: Check when/unless/creates, apply tags filter
//  2. Variable processing: Merge step vars into context
//  3. Loop expansion: Expand with_items/with_filetree into multiple executions
//  4. Action execution: Dispatch to handler or legacy implementation
//  5. Post-execution: Evaluate changed_when/failed_when, register results
//  6. Event emission: Publish lifecycle events
//
// # Execution Context
//
// ExecutionContext carries all state needed during execution:
//   - Variables: Step vars, global vars, facts, registered results
//   - Template: Jinja2-like template renderer
//   - Evaluator: Expression evaluator for conditions
//   - Logger: Structured logging (TUI or text)
//   - PathUtil: Path resolution and expansion
//   - EventPublisher: Event emission for observability
//   - Stats: Execution statistics (total, success, failed, changed, skipped)
//
// # Action Dispatch
//
// Actions are dispatched through two paths:
//
//  1. Handler-based (new): Look up handler in actions.Registry, call handler.Execute()
//  2. Legacy: Direct executor methods (HandleShell, HandleFile, etc.)
//
// The executor prefers handlers when available, falling back to legacy for non-migrated actions.
//
// # Idempotency
//
// The executor enforces idempotency through:
//   - creates: Skip if path exists
//   - unless: Skip if command succeeds
//   - changed_when: Custom change detection
//   - Handler implementations: Built-in state checking
//
// # Dry-Run Mode
//
// When DryRun is true:
//   - No actual changes are made to the system
//   - Handlers log what would happen
//   - Template rendering still occurs (validates syntax)
//   - File existence checks are performed (read-only)
//   - Statistics track what would have changed
//
// # Error Handling
//
// Errors are wrapped with context using custom error types:
//   - RenderError: Template rendering failures (field + cause)
//   - EvaluationError: Expression evaluation failures (expression + cause)
//   - CommandError: Command execution failures (command + exit code)
//   - FileOperationError: File operation failures (path + operation + cause)
//   - StepValidationError: Configuration validation failures
//   - SetupError: Infrastructure/environment setup failures
//
// Use errors.Is() and errors.As() for programmatic error inspection.
//
// # Usage Example
//
//	// Load configuration
//	steps, err := config.ReadConfig("config.yml")
//	if err != nil {
//	    return err
//	}
//
//	// Create executor
//	log := logger.NewTextLogger()
//	exec := NewExecutor(log)
//
//	// Execute with options
//	result, err := exec.Execute(config.Plan{Steps: steps}, ExecuteOptions{
//	    DryRun: false,
//	    Tags: []string{"setup", "deploy"},
//	    Variables: map[string]interface{}{
//	        "environment": "production",
//	    },
//	})
//
//	// Check results
//	if !result.Success {
//	    log.Errorf("Execution failed: %d failed steps", result.FailedSteps)
//	}
//	log.Infof("Summary: %d changed, %d unchanged, %d failed",
//	    result.ChangedSteps, result.SuccessSteps-result.ChangedSteps, result.FailedSteps)
package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/artifacts"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
	"github.com/alehatsman/mooncake/internal/utils"
)

// generateStepID creates a unique identifier for a step
func generateStepID(step config.Step, ec *ExecutionContext) string {
	if step.ID != "" {
		return step.ID
	}
	return fmt.Sprintf("step-%d", *ec.Stats.Global)
}

// MarkStepFailed marks a result as failed and registers it if needed.
// The caller is responsible for returning an appropriate error.
//
// INTERNAL: This function is exported for testing purposes only and is not part of
// the public API. It may change or be removed in future versions without notice.
func MarkStepFailed(result *Result, step config.Step, ec *ExecutionContext) {
	result.Failed = true
	result.Rc = 1
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
	}
}

// AddGlobalVariables injects system facts into the variables map.
// This makes facts like ansible_os_family, ansible_distribution, etc. available during planning.
func AddGlobalVariables(variables map[string]interface{}) {
	// Collect system facts
	systemFacts := facts.Collect()

	// Add all facts to variables
	for k, v := range systemFacts.ToMap() {
		variables[k] = v
	}
}

// HandleVars processes the vars field of a step, rendering templates and merging into the execution context.
//
// INTERNAL: This function is exported for testing purposes only and is not part of
// the public API. It may change or be removed in future versions without notice.
func HandleVars(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Handling vars: %+v", step.Vars)

	if step.Vars == nil {
		return fmt.Errorf("vars is nil in step")
	}

	vars := step.Vars

	for k, v := range *vars {
		ec.Logger.Debugf("  %v: %v", k, v)
	}

	ec.HandleDryRun(func(dryRun *DryRunLogger) {
		dryRun.LogVariableSet(len(*vars))
		// Still set variables in dry-run mode so subsequent steps can use them
	})

	ec.Variables = utils.MergeVariables(ec.Variables, *vars)

	// Emit variables.set event
	keys := make([]string, 0, len(*vars))
	for k := range *vars {
		keys = append(keys, k)
	}
	ec.EmitEvent(events.EventVarsSet, events.VarsSetData{
		Count:  len(*vars),
		Keys:   keys,
		DryRun: ec.DryRun,
	})

	return nil
}

// HandleWhenExpression evaluates the when condition and returns whether the step should be skipped.
// Returns (shouldSkip bool, error).
//
// INTERNAL: This function is exported for testing purposes only and is not part of
// the public API. It may change or be removed in future versions without notice.
func HandleWhenExpression(step config.Step, ec *ExecutionContext) (bool, error) {
	whenString := strings.Trim(step.When, " ")

	ec.Logger.Debugf("variables: %v", ec.Variables)

	whenExpression, err := ec.Template.Render(whenString, ec.Variables)
	if err != nil {
		return false, &RenderError{Field: "when", Cause: err}
	}

	ec.Logger.Debugf("whenExpression: %v", whenExpression)

	evalResult, err := ec.Evaluator.Evaluate(whenExpression, ec.Variables)
	if err != nil {
		return false, &EvaluationError{Expression: whenExpression, Cause: err}
	}

	ec.Logger.Debugf("evalResult: %v", evalResult)

	// Handle nil or non-bool results
	if evalResult == nil {
		return true, nil // Skip if expression evaluates to nil
	}

	boolResult, ok := evalResult.(bool)
	if !ok {
		return false, &EvaluationError{
			Expression: whenExpression,
			Cause:      fmt.Errorf("expression evaluated to %T, expected bool", evalResult),
		}
	}

	return !boolResult, nil
}

// ShouldSkipByTags determines if a step should be skipped based on tag filtering.
// Returns true if the step should be skipped, false otherwise.
//
// INTERNAL: This function is exported for testing purposes only and is not part of
// the public API. It may change or be removed in future versions without notice.
func ShouldSkipByTags(step config.Step, ec *ExecutionContext) bool {
	// If no tags filter specified, execute all steps
	if len(ec.Tags) == 0 {
		return false
	}

	// If step has no tags and tags filter is specified, skip it
	if len(step.Tags) == 0 {
		return true
	}

	// Check if step has any of the requested tags
	for _, stepTag := range step.Tags {
		for _, filterTag := range ec.Tags {
			if stepTag == filterTag {
				return false // Found a match, don't skip
			}
		}
	}

	// No matching tags found, skip the step
	return true
}

// CheckIdempotencyConditions evaluates creates and unless conditions for shell steps.
// Returns (shouldSkip bool, reason string, error)
//
// INTERNAL: This function is exported for testing purposes only and is not part of
// the public API. It may change or be removed in future versions without notice.
func CheckIdempotencyConditions(step config.Step, ec *ExecutionContext) (bool, string, error) {
	// Check creates condition
	if step.Creates != nil {
		path, err := ec.Template.Render(*step.Creates, ec.Variables)
		if err != nil {
			return false, "", &RenderError{Field: "creates path", Cause: err}
		}

		expandedPath, err := ec.PathUtil.ExpandPath(path, ec.CurrentDir, ec.Variables)
		if err != nil {
			return false, "", &RenderError{Field: "creates path", Cause: err}
		}

		if _, err := os.Stat(expandedPath); err == nil {
			// Path exists - skip step
			return true, fmt.Sprintf("creates: %s", expandedPath), nil
		}
	}

	// Check unless condition
	if step.Unless != nil {
		command, err := ec.Template.Render(*step.Unless, ec.Variables)
		if err != nil {
			return false, "", &RenderError{Field: "unless command", Cause: err}
		}

		// Execute unless command (silently, no logging)
		// #nosec G204 -- This is a provisioning tool designed to execute commands from user configs.
		// The command comes from user-provided YAML configuration files for idempotency checks.
		cmd := exec.Command("sh", "-c", command)
		if err := cmd.Run(); err == nil {
			// Command succeeded - skip step
			return true, fmt.Sprintf("unless: %s", command), nil
		}
	}

	return false, "", nil
}

// CheckSkipConditions evaluates whether a step should be skipped based on conditional
// expressions and tag filters.
//
// It first checks if the step was marked as skipped during planning (via step.Skipped field).
// This happens when tag filtering is applied during the planning phase.
//
// Next, it evaluates the step's "when" condition (if present), which is an expression
// that must evaluate to true for the step to execute. If the condition evaluates to false,
// the step is skipped with reason "when".
//
// Returns:
//   - shouldSkip: true if the step should be skipped
//   - skipReason: "when" or "tags" indicating why the step was skipped (empty if not skipped)
//   - error: any error encountered while evaluating conditions
//
// INTERNAL: This function is exported for testing purposes only and is not part of
// the public API. It may change or be removed in future versions without notice.
func CheckSkipConditions(step config.Step, ec *ExecutionContext) (bool, string, error) {
	// Check if step was marked as skipped during planning (tag filtering)
	// The planner already evaluated tags and set step.Skipped - we trust that decision.
	// No need to recalculate at runtime (performance and single-source-of-truth).
	if step.Skipped {
		return true, "tags", nil
	}

	// Check when condition
	if step.When != "" {
		shouldSkip, err := HandleWhenExpression(step, ec)
		if err != nil {
			return false, "", err
		}
		if shouldSkip {
			return true, "when", nil
		}
	}

	// NOTE: Removed redundant ShouldSkipByTags() check.
	// Tag filtering is handled during planning phase (step.Skipped is set there).
	// The executor trusts the planner's decision for cleaner separation of concerns.

	return false, "", nil
}

// GetStepDisplayName determines the display name to show for a step in logs and output.
//
// The function follows a priority order to determine the name:
//  1. If executing within a with_filetree loop, uses action + destination path
//  2. If executing within a with_items loop, uses the string representation of the item
//  3. Otherwise, uses the step's configured Name field
//
// Returns:
//   - displayName: the name to display for this step
//   - hasName: true if a name was found, false if the step is anonymous
//
// INTERNAL: This function is exported for testing purposes only and is not part of
// the public API. It may change or be removed in future versions without notice.
func GetStepDisplayName(step config.Step, ec *ExecutionContext) (string, bool) {
	// For with_filetree, show hierarchical structure
	if item, ok := ec.Variables["item"].(filetree.Item); ok {
		// For directories, show as headers with trailing slash
		if item.IsDir {
			if item.Path == "" {
				// Root directory
				return fmt.Sprintf("%s/", item.Name), true
			}
			// Subdirectory - show path without leading slash, with trailing slash
			dirPath := strings.TrimPrefix(item.Path, "/")
			return fmt.Sprintf("%s/", dirPath), true
		}

		// For files, show just the filename (not full destination path)
		// The directory context will be shown by the parent directory header
		if item.Name != "" {
			return item.Name, true
		}

		// Fallback to item path
		if item.Path != "" {
			return strings.TrimPrefix(item.Path, "/"), true
		}
	}

	// For with_items, show the item value
	if item, ok := ec.Variables["item"]; ok {
		return fmt.Sprintf("%v", item), true
	}

	// Use configured step name
	if step.Name != "" {
		return step.Name, true
	}

	return "", false
}



// DispatchStepAction executes the appropriate handler based on step type.
// All actions are now handled through the actions registry.
//
// INTERNAL: This function is exported for testing purposes only and is not part of
// the public API. It may change or be removed in future versions without notice.
func DispatchStepAction(step config.Step, ec *ExecutionContext) error {
	// Determine action type from step
	actionType := step.DetermineActionType()

	// Try to get handler from registry (new system)
	if handler, ok := actions.Get(actionType); ok {
		// Validate step configuration
		if err := handler.Validate(&step); err != nil {
			// Enhance error with step code context if available
			var errMsg string
			if step.Origin != nil && step.Origin.FilePath != "" {
				excerpt := config.FormatStepExcerpt(&step)
				if excerpt != "" {
					errMsg = fmt.Sprintf("validation failed for %s action: %v\n\nStep code (%s:%d):\n%s",
						actionType, err, step.Origin.FilePath, step.Origin.Line, excerpt)
				} else {
					errMsg = fmt.Sprintf("validation failed for %s action: %v", actionType, err)
				}
			} else {
				errMsg = fmt.Sprintf("validation failed for %s action: %v", actionType, err)
			}
			return fmt.Errorf("%s", errMsg)
		}

		// Handle dry-run mode
		if ec.DryRun {
			// Create a result for dry-run
			result := NewResult()
			ec.CurrentResult = result

			if err := handler.DryRun(ec, &step); err != nil {
				return err
			}

			// Register result if requested
			if step.Register != "" {
				result.RegisterTo(ec.Variables, step.Register)
			}
			return nil
		}

		// Execute the action
		actionResult, err := handler.Execute(ec, &step)
		if err != nil {
			return err
		}

		// Convert actions.Result interface back to *Result if needed
		var result *Result
		if concreteResult, ok := actionResult.(*Result); ok {
			result = concreteResult
		} else {
			// Create a new Result from the interface
			result = NewResult()
			// The interface methods would have been called by the handler
			// so we trust that the result is properly populated
			// For now, we just use it as-is
		}

		// Store result in context
		ec.CurrentResult = result

		// Register result if requested
		if step.Register != "" && actionResult != nil {
			actionResult.RegisterTo(ec.Variables, step.Register)
		}

		return nil
	}

	// If we get here, the action type is not registered
	return fmt.Errorf("no handler registered for action type: %s", actionType)
}

// ExecuteStep executes a single configuration step within the given execution context.
func ExecuteStep(step config.Step, ec *ExecutionContext) error {
	// Validate step configuration
	if err := step.Validate(); err != nil {
		return err
	}

	// Check if step should be skipped (when conditions, tags)
	shouldSkip, skipReason, err := CheckSkipConditions(step, ec)
	if err != nil {
		return err
	}

	// Check idempotency conditions (creates, unless) - ONLY for shell/command steps
	if !shouldSkip && (step.Shell != nil || step.Command != nil) {
		idempotencySkip, idempotencyReason, err := CheckIdempotencyConditions(step, ec)
		if err != nil {
			return err
		}
		if idempotencySkip {
			shouldSkip = true
			skipReason = "idempotency:" + idempotencyReason
		}
	}

	// Determine step display name
	stepName, hasStepName := GetStepDisplayName(step, ec)

	// Handle skipped steps
	if shouldSkip {
		// Log skipped steps (only for named, non-include steps)
		if hasStepName && step.Include == nil {
			// Update skipped statistics
			if ec.Stats.Skipped != nil {
				*ec.Stats.Skipped++
			}

			// Emit step.skipped event
			stepID := generateStepID(step, ec)
			depth := 0
			if step.LoopContext != nil {
				depth = step.LoopContext.Depth
			}
			ec.EmitEvent(events.EventStepSkipped, events.StepSkippedData{
				StepID: stepID,
				Name:   stepName,
				Level:  ec.Level,
				Reason: skipReason,
				Depth:  depth,
			})
		}

		return nil
	}

	// Debug: show tags for non-skipped steps
	if len(step.Tags) > 0 {
		ec.Logger.Debugf("  tags: [%s]", strings.Join(step.Tags, ", "))
	}

	// Debug: show action for unnamed steps
	if step.Name == "" {
		if step.Vars != nil {
			ec.Logger.Debugf("Setting variables")
		} else if step.IncludeVars != nil {
			ec.Logger.Debugf("Loading variables from %s", *step.IncludeVars)
		}
	}

	// Increment global step counter for non-skipped steps
	if ec.Stats.Global != nil {
		*ec.Stats.Global++
	}

	// Generate step ID and store in context for event correlation
	stepID := generateStepID(step, ec)
	ec.CurrentStepID = stepID

	// Get directory depth from loop context (for filetree items)
	depth := 0
	if step.LoopContext != nil {
		depth = step.LoopContext.Depth
	}

	// Emit step.started event
	ec.EmitEvent(events.EventStepStarted, events.StepStartedData{
		StepID:     stepID,
		Name:       stepName,
		Level:      ec.Level,
		GlobalStep: *ec.Stats.Global,
		Action:     step.ActionType,
		Tags:       step.Tags,
		When:       step.When,
		Depth:      depth,
		DryRun:     ec.DryRun,
	})

	// Track start time for duration
	stepStartTime := time.Now()

	// Execute the appropriate handler
	stepErr := DispatchStepAction(step, ec)

	// Calculate duration
	stepDuration := time.Since(stepStartTime)

	// Handle errors
	if stepErr != nil {
		ec.Logger.Errorf("%v", stepErr)
		// Update failed statistics
		if ec.Stats.Failed != nil {
			*ec.Stats.Failed++
		}

		// Emit step.failed event
		ec.EmitEvent(events.EventStepFailed, events.StepFailedData{
			StepID:       stepID,
			Name:         stepName,
			Level:        ec.Level,
			ErrorMessage: stepErr.Error(),
			DurationMs:   stepDuration.Milliseconds(),
			Depth:        depth,
			DryRun:       ec.DryRun,
		})

		return stepErr
	}

	// Update executed statistics
	if ec.Stats.Executed != nil {
		*ec.Stats.Executed++
	}

	// Get result data if handler provided it
	changed := false
	var resultData map[string]interface{}
	if ec.CurrentResult != nil {
		changed = ec.CurrentResult.Changed
		resultData = ec.CurrentResult.ToMap()
	}

	// Emit step.completed event
	ec.EmitEvent(events.EventStepCompleted, events.StepCompletedData{
		StepID:     stepID,
		Name:       stepName,
		Level:      ec.Level,
		DurationMs: stepDuration.Milliseconds(),
		Changed:    changed,
		Result:     resultData,
		Depth:      depth,
		DryRun:     ec.DryRun,
	})

	// Clear current result for next step
	ec.CurrentResult = nil

	return nil
}

// ExecuteSteps executes a sequence of configuration steps within the given execution context.
func ExecuteSteps(steps []config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Executing: %v", ec.CurrentFile)

	// Set total steps for this execution context
	ec.TotalSteps = len(steps)

	for i, step := range steps {
		ec.CurrentIndex = i

		// If step has origin metadata (from planner), use its directory
		// This ensures relative paths work correctly for included files
		if step.Origin != nil && step.Origin.FilePath != "" {
			ec.CurrentDir = filepath.Dir(step.Origin.FilePath)
			ec.CurrentFile = step.Origin.FilePath
		}

		// If step has loop context (from planner), restore loop variables
		// This ensures when conditions can reference item, index, first, last
		if step.LoopContext != nil {
			ec.Variables["item"] = step.LoopContext.Item
			ec.Variables["index"] = step.LoopContext.Index
			ec.Variables["first"] = step.LoopContext.First
			ec.Variables["last"] = step.LoopContext.Last
		} else {
			// Clear loop variables for steps without loop context
			// to prevent stale values from previous loop iterations
			delete(ec.Variables, "item")
			delete(ec.Variables, "index")
			delete(ec.Variables, "first")
			delete(ec.Variables, "last")
		}

		if err := ExecuteStep(step, ec); err != nil {
			return err
		}
	}
	return nil
}

// StartConfig contains configuration for starting a mooncake execution.
type StartConfig struct {
	ConfigFilePath   string
	VarsFilePath     string
	SudoPass         string // Sudo password provided directly (use SudoPassFile for better security)
	SudoPassFile     string
	AskBecomePass    bool
	InsecureSudoPass bool
	Tags             []string
	DryRun           bool

	// Artifact configuration
	ArtifactsDir      string
	CaptureFullOutput bool
	MaxOutputBytes    int
	MaxOutputLines    int
}

// Start begins execution of a mooncake configuration with the given settings.
// Always goes through the planner to expand loops, includes, and variables.
// Emits events through the provided publisher for all execution progress.
func Start(startConfig StartConfig, log logger.Logger, publisher events.Publisher) error {
	log.Debugf("config: %v", startConfig)

	if startConfig.ConfigFilePath == "" {
		return &SetupError{Component: "config", Issue: "config file path is empty"}
	}

	// Resolve sudo password early (before plan building)
	passwordCfg := security.PasswordConfig{
		CLIPassword:    startConfig.SudoPass,
		AskInteractive: startConfig.AskBecomePass,
		PasswordFile:   startConfig.SudoPassFile,
		InsecureCLI:    startConfig.InsecureSudoPass,
	}

	sudoPassword, err := security.ResolvePassword(passwordCfg)
	if err != nil {
		return &SetupError{Component: "sudo password", Issue: "failed to resolve password", Cause: err}
	}

	// Create path expander for resolving paths
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		return &SetupError{Component: "template renderer", Issue: "failed to create renderer", Cause: err}
	}
	pathExpander := pathutil.NewPathExpander(renderer)

	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Load variables if specified
	var variables map[string]interface{}
	if startConfig.VarsFilePath != "" {
		expandedPath, expandErr := pathExpander.ExpandPath(startConfig.VarsFilePath, currentDir, nil)
		if expandErr != nil {
			return &RenderError{Field: "vars file path", Cause: expandErr}
		}

		log.Debugf("Reading variables from file: %v", expandedPath)
		variables, err = config.ReadVariables(expandedPath)
		if err != nil {
			log.Debugf("Failed to read variables: %v", err)
			variables = make(map[string]interface{})
		}
		log.Debugf("Read variables: %v", variables)
	} else {
		variables = make(map[string]interface{})
	}

	// Expand config file path
	configFilePath, err := pathExpander.ExpandPath(startConfig.ConfigFilePath, currentDir, nil)
	if err != nil {
		return err
	}

	log.Debugf("Building plan from configuration: %v", configFilePath)

	// ALWAYS build plan first (expands loops, includes, vars)
	planner, err := plan.NewPlanner()
	if err != nil {
		return &SetupError{Component: "planner", Issue: "failed to create planner", Cause: err}
	}
	planData, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: configFilePath,
		Variables:  variables,
		Tags:       startConfig.Tags,
	})
	if err != nil {
		return &SetupError{Component: "planner", Issue: "failed to build plan", Cause: err}
	}

	log.Debugf("Plan built with %d steps", len(planData.Steps))

	// Setup artifact writer if artifacts-dir is specified
	if startConfig.ArtifactsDir != "" {
		// Gather system facts for artifact generation
		systemFacts := facts.Collect()

		// Create artifact writer
		artifactWriter, err := artifacts.NewWriter(
			artifacts.Config{
				BaseDir:        startConfig.ArtifactsDir,
				CaptureStdout:  startConfig.CaptureFullOutput,
				CaptureStderr:  startConfig.CaptureFullOutput,
				MaxOutputBytes: startConfig.MaxOutputBytes,
				MaxOutputLines: startConfig.MaxOutputLines,
			},
			planData,
			systemFacts,
		)
		if err != nil {
			return &SetupError{Component: "artifacts", Issue: "failed to create artifact writer", Cause: err}
		}
		defer artifactWriter.Close()

		// Subscribe artifact writer to events
		publisher.Subscribe(artifactWriter)

		log.Debugf("Artifacts will be written to: %s/runs/%s", startConfig.ArtifactsDir, "...")
	}

	// Execute the plan with event publisher
	return ExecutePlan(planData, sudoPassword, startConfig.DryRun, log, publisher)
}

// ExecutePlan executes a pre-compiled plan.
// Emits events through the provided publisher for all execution progress.
func ExecutePlan(p *plan.Plan, sudoPass string, dryRun bool, log logger.Logger, publisher events.Publisher) error {
	steps := p.Steps
	variables := p.InitialVars

	// Start timing
	startTime := time.Now()

	// Emit run.started event
	publisher.Publish(events.Event{
		Type:      events.EventRunStarted,
		Timestamp: time.Now(),
		Data: events.RunStartedData{
			RootFile:   p.RootFile,
			Tags:       p.Tags,
			DryRun:     dryRun,
			TotalSteps: len(p.Steps),
		},
	})

	// Emit plan.loaded event
	publisher.Publish(events.Event{
		Type:      events.EventPlanLoaded,
		Timestamp: time.Now(),
		Data: events.PlanLoadedData{
			RootFile:   p.RootFile,
			TotalSteps: len(p.Steps),
			Tags:       p.Tags,
		},
	})

	// Create dependencies
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		return &SetupError{Component: "template renderer", Issue: "failed to create renderer", Cause: err}
	}
	evaluator := expression.NewGovaluateEvaluator()
	pathExpander := pathutil.NewPathExpander(renderer)
	fileTreeWalker := filetree.NewWalker(pathExpander)

	// Create redactor and add sudo password if present
	redactor := security.NewRedactor()
	if sudoPass != "" {
		redactor.AddSensitive(sudoPass)
	}

	// Set redactor on logger for automatic redaction
	log.SetRedactor(redactor)

	// Use the directory of the root config file, not the current working directory
	// This ensures relative paths in the config (like ./template.j2) are resolved correctly
	configDir := filepath.Dir(p.RootFile)

	// Initialize variables from plan (system facts already injected by planner)
	if variables == nil {
		variables = make(map[string]interface{})
	}

	// Initialize global step counter and statistics
	globalExecuted := 0
	statsExecuted := 0
	statsSkipped := 0
	statsFailed := 0

	executionContext := ExecutionContext{
		Variables:    variables,
		CurrentDir:   configDir,
		CurrentFile:  "",
		Level:        0,
		CurrentIndex: 0,
		TotalSteps:   len(steps),
		Logger:       log.WithPadLevel(0),
		SudoPass:     sudoPass,
		Tags:         []string{}, // Not used - tag filtering done by planner (step.Skipped)
		DryRun:       dryRun,

		// Statistics tracking
		Stats: &ExecutionStats{
			Global:   &globalExecuted,
			Executed: &statsExecuted,
			Skipped:  &statsSkipped,
			Failed:   &statsFailed,
		},

		// Inject dependencies
		Template:  renderer,
		Evaluator: evaluator,
		PathUtil:  pathExpander,
		FileTree:  fileTreeWalker,
		Redactor:  redactor,

		// Event publisher
		EventPublisher: publisher,
	}

	// Execute pre-expanded steps
	execErr := ExecuteSteps(steps, &executionContext)

	// Calculate duration
	duration := time.Since(startTime)

	// Emit run.completed event (console subscriber handles display)
	changedSteps := 0
	// Count changed steps (steps that were executed and not failed)
	// Changed status is tracked elsewhere, for now use simplified logic
	if execErr == nil {
		changedSteps = statsExecuted
	}

	publisher.Publish(events.Event{
		Type:      events.EventRunCompleted,
		Timestamp: time.Now(),
		Data: events.RunCompletedData{
			TotalSteps:   len(steps),
			SuccessSteps: statsExecuted,
			FailedSteps:  statsFailed,
			SkippedSteps: statsSkipped,
			ChangedSteps: changedSteps,
			DurationMs:   duration.Milliseconds(),
			Success:      execErr == nil,
			ErrorMessage: func() string {
				if execErr != nil {
					return execErr.Error()
				}
				return ""
			}(),
		},
	})

	return execErr
}

// formatMode formats an os.FileMode as an octal string (e.g., "0644").
func formatMode(mode os.FileMode) string {
	return fmt.Sprintf("%#o", mode)
}

// ParseFileMode parses a mode string (e.g., "0644") into os.FileMode.
// Returns default mode if mode is empty or invalid.
//
// INTERNAL: This function is exported for testing purposes only and is not part of
// the public API. It may change or be removed in future versions without notice.
func ParseFileMode(modeStr string, defaultMode os.FileMode) os.FileMode {
	if modeStr == "" {
		return defaultMode
	}

	// Parse as octal
	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return defaultMode
	}

	return os.FileMode(mode)
}
