package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

const (
	actionTypeFile        = "file"
	actionTypeTemplate    = "template"
	actionTypeVars        = "vars"
	actionTypeIncludeVars = "include_vars"
)

// generateStepID creates a unique identifier for a step
func generateStepID(step config.Step, ec *ExecutionContext) string {
	if step.ID != "" {
		return step.ID
	}
	return fmt.Sprintf("step-%d", *ec.Stats.Global)
}

// markStepFailed marks a result as failed and registers it if needed.
// The caller is responsible for returning an appropriate error.
func markStepFailed(result *Result, step config.Step, ec *ExecutionContext) {
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

func handleVars(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Handling vars: %+v", step.Vars)

	vars := step.Vars

	for k, v := range *vars {
		ec.Logger.Debugf("  %v: %v", k, v)
	}

	ec.HandleDryRun(func(dryRun *dryRunLogger) {
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

func handleWhenExpression(step config.Step, ec *ExecutionContext) (bool, error) {
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

func shouldSkipByTags(step config.Step, ec *ExecutionContext) bool {
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

// checkIdempotencyConditions evaluates creates and unless conditions for shell steps.
// Returns (shouldSkip bool, reason string, error)
func checkIdempotencyConditions(step config.Step, ec *ExecutionContext) (bool, string, error) {
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

// checkSkipConditions evaluates whether a step should be skipped based on conditional
// expressions and tag filters.
//
// It first evaluates the step's "when" condition (if present), which is an expression
// that must evaluate to true for the step to execute. If the condition evaluates to false,
// the step is skipped with reason "when".
//
// Next, it checks if the step should be skipped based on tag filtering. If the execution
// context has a tags filter and the step's tags don't match, it's skipped with reason "tags".
//
// Returns:
//   - shouldSkip: true if the step should be skipped
//   - skipReason: "when" or "tags" indicating why the step was skipped (empty if not skipped)
//   - error: any error encountered while evaluating conditions
func checkSkipConditions(step config.Step, ec *ExecutionContext) (bool, string, error) {
	// Check when condition
	if step.When != "" {
		shouldSkip, err := handleWhenExpression(step, ec)
		if err != nil {
			return false, "", err
		}
		if shouldSkip {
			return true, "when", nil
		}
	}

	// Check tags filter
	if shouldSkipByTags(step, ec) {
		return true, "tags", nil
	}

	return false, "", nil
}

// getStepDisplayName determines the display name to show for a step in logs and output.
//
// The function follows a priority order to determine the name:
//  1. If executing within a with_filetree loop, uses action + destination path
//  2. If executing within a with_items loop, uses the string representation of the item
//  3. Otherwise, uses the step's configured Name field
//
// Returns:
//   - displayName: the name to display for this step
//   - hasName: true if a name was found, false if the step is anonymous
func getStepDisplayName(step config.Step, ec *ExecutionContext) (string, bool) {
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



// dispatchStepAction executes the appropriate handler based on step type.
func dispatchStepAction(step config.Step, ec *ExecutionContext) error {
	switch {
	case step.IncludeVars != nil:
		return HandleIncludeVars(step, ec)

	case step.Vars != nil:
		return handleVars(step, ec)

	case step.Template != nil:
		return HandleTemplate(step, ec)

	case step.File != nil:
		return HandleFile(step, ec)

	case step.Copy != nil:
		return HandleCopy(step, ec)

	case step.Unarchive != nil:
		return HandleUnarchive(step, ec)

	case step.Download != nil:
		return HandleDownload(step, ec)

	case step.Service != nil:
		return HandleService(step, ec)

	case step.Ollama != nil:
		return HandleOllama(step, ec)

	case step.Assert != nil:
		return HandleAssert(step, ec)

	case step.Preset != nil:
		return HandlePreset(step, ec)

	case step.Shell != nil:
		return HandleShell(step, ec)

	case step.Command != nil:
		return HandleCommand(step, ec)

	default:
		return nil
	}
}

// ExecuteStep executes a single configuration step within the given execution context.
func ExecuteStep(step config.Step, ec *ExecutionContext) error {
	// Validate step configuration
	if err := step.Validate(); err != nil {
		return err
	}

	// Check if step should be skipped (when conditions, tags)
	shouldSkip, skipReason, err := checkSkipConditions(step, ec)
	if err != nil {
		return err
	}

	// Check idempotency conditions (creates, unless) - ONLY for shell/command steps
	if !shouldSkip && (step.Shell != nil || step.Command != nil) {
		idempotencySkip, idempotencyReason, err := checkIdempotencyConditions(step, ec)
		if err != nil {
			return err
		}
		if idempotencySkip {
			shouldSkip = true
			skipReason = "idempotency:" + idempotencyReason
		}
	}

	// Determine step display name
	stepName, hasStepName := getStepDisplayName(step, ec)

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
	stepErr := dispatchStepAction(step, ec)

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
	renderer := template.NewPongo2Renderer()
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
	planner := plan.NewPlanner()
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
	renderer := template.NewPongo2Renderer()
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
		Tags:         []string{},
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
