package executor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/alehatsman/mooncake/internal/template"
)

// mergeVariables creates a new map combining base and override variables.
// Values from override take precedence over values from base with the same key.
func mergeVariables(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
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

	if ec.DryRun {
		dryRun := newDryRunLogger(ec.Logger)
		dryRun.LogVariableSet(len(*vars))
		// Still set variables in dry-run mode so subsequent steps can use them
	}

	ec.Variables = mergeVariables(ec.Variables, *vars)
	return nil
}

func handleWhenExpression(step config.Step, ec *ExecutionContext) (bool, error) {
	whenString := strings.Trim(step.When, " ")

	ec.Logger.Debugf("variables: %v", ec.Variables)

	whenExpression, err := ec.Template.Render(whenString, ec.Variables)
	if err != nil {
		return false, err
	}

	ec.Logger.Debugf("whenExpression: %v", whenExpression)

	evalResult, err := ec.Evaluator.Evaluate(whenExpression, ec.Variables)
	if err != nil {
		return false, err
	}

	ec.Logger.Debugf("evalResult: %v", evalResult)

	// Handle nil or non-bool results
	if evalResult == nil {
		return true, nil // Skip if expression evaluates to nil
	}

	boolResult, ok := evalResult.(bool)
	if !ok {
		return false, fmt.Errorf("when expression did not evaluate to boolean: %v", evalResult)
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
			return false, "", fmt.Errorf("failed to render creates path: %w", err)
		}

		expandedPath, err := ec.PathUtil.ExpandPath(path, ec.CurrentDir, ec.Variables)
		if err != nil {
			return false, "", fmt.Errorf("failed to expand creates path: %w", err)
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
			return false, "", fmt.Errorf("failed to render unless command: %w", err)
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
//  1. If executing within a with_filetree loop, uses the file's name (Item.Name)
//  2. If executing within a with_items loop, uses the string representation of the item
//  3. Otherwise, uses the step's configured Name field
//
// Returns:
//   - displayName: the name to display for this step
//   - hasName: true if a name was found, false if the step is anonymous
func getStepDisplayName(step config.Step, ec *ExecutionContext) (string, bool) {
	// For with_filetree, show the actual file name instead of generic step name
	if item, ok := ec.Variables["item"].(filetree.Item); ok {
		if item.Name != "" {
			return item.Name, true
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

// shouldLogStep determines whether a step should have its status logged to the output.
//
// Not all steps are logged individually:
//   - Anonymous steps (no name) are not logged
//   - Include steps have their own specialized logging
//   - Template steps within with_filetree loops are logged by the handler, not here
//
// Parameters:
//   - step: the step configuration
//   - hasStepName: whether the step has a display name (from getStepDisplayName)
//   - ec: execution context (used to detect with_filetree loops)
//
// Returns true if the step's status (running/success/error) should be logged.
func shouldLogStep(step config.Step, hasStepName bool, ec *ExecutionContext) bool {
	// Don't log if no name
	if !hasStepName {
		return false
	}

	// Don't log includes (they have their own logging)
	if step.Include != nil {
		return false
	}

	// Don't log template steps in with_filetree (handler logs them)
	_, inFileTree := ec.Variables["item"].(filetree.Item)
	if step.Template != nil && inFileTree {
		return false
	}

	return true
}

// logStepStatus logs step status (running, success, error, skipped) and updates statistics.
// For skipped status, pass "skipped:when" or "skipped:tags" or "skipped:idempotency:..." to include the skip reason.
func logStepStatus(stepName string, status string, step config.Step, ec *ExecutionContext) {
	// Handle skipped status (may include reason like "skipped:when" or "skipped:tags" or "skipped:idempotency:...")
	if strings.HasPrefix(status, "skipped") {
		skipInfo := ""

		// Parse skip reason from status if present
		parts := strings.SplitN(status, ":", 2)
		if len(parts) == 2 {
			skipReason := parts[1]
			if skipReason == "when" && step.When != "" {
				skipInfo = fmt.Sprintf(" (when: %s)", step.When)
			} else if skipReason == "tags" && len(ec.Tags) > 0 {
				skipInfo = fmt.Sprintf(" (tags filter: %s)", strings.Join(ec.Tags, ", "))
			} else if strings.HasPrefix(skipReason, "idempotency:") {
				// Extract the actual reason after "idempotency:"
				idempotencyDetail := strings.TrimPrefix(skipReason, "idempotency:")
				skipInfo = fmt.Sprintf(" (%s)", idempotencyDetail)
			}
		}

		ec.Logger.LogStep(logger.StepInfo{
			Name:       stepName + skipInfo,
			Level:      ec.Level,
			GlobalStep: 0,
			Status:     logger.StatusSkipped,
		})

		if ec.StatsSkipped != nil {
			*ec.StatsSkipped++
		}
		return
	}

	// For running/success/error, log with global step number
	globalStep := 0
	if ec.GlobalStepsExecuted != nil {
		globalStep = *ec.GlobalStepsExecuted
	}

	ec.Logger.LogStep(logger.StepInfo{
		Name:       stepName,
		Level:      ec.Level,
		GlobalStep: globalStep,
		Status:     status,
	})

	// Update statistics
	switch status {
	case logger.StatusSuccess:
		if ec.StatsExecuted != nil {
			*ec.StatsExecuted++
		}
	case logger.StatusError:
		if ec.StatsFailed != nil {
			*ec.StatsFailed++
		}
	}
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

	case step.Shell != nil:
		return HandleShell(step, ec)

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

	// Check idempotency conditions (creates, unless) - ONLY for shell steps
	if !shouldSkip && step.Shell != nil {
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
			logStepStatus(stepName, "skipped:"+skipReason, step, ec)
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
	if ec.GlobalStepsExecuted != nil {
		*ec.GlobalStepsExecuted++
	}

	// Log running status
	if shouldLogStep(step, hasStepName, ec) {
		logStepStatus(stepName, logger.StatusRunning, step, ec)
	}

	// Execute the appropriate handler
	stepErr := dispatchStepAction(step, ec)

	// Handle errors
	if stepErr != nil {
		ec.Logger.Errorf("%v", stepErr)
		if shouldLogStep(step, hasStepName, ec) {
			logStepStatus(stepName, logger.StatusError, step, ec)
		}
		return stepErr
	}

	// Log success
	if shouldLogStep(step, hasStepName, ec) {
		logStepStatus(stepName, logger.StatusSuccess, step, ec)
	}

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
		}

		if err := ExecuteStep(step, ec); err != nil {
			return err
		}
	}
	return nil
}

// StartConfig contains configuration for starting a mooncake execution.
type StartConfig struct {
	ConfigFilePath string
	VarsFilePath   string
	SudoPass       string
	Tags           []string
	DryRun         bool
}

// Start begins execution of a mooncake configuration with the given settings.
// Always goes through the planner to expand loops, includes, and variables.
func Start(startConfig StartConfig, log logger.Logger) error {
	log.Debugf("config: %v", startConfig)

	if startConfig.ConfigFilePath == "" {
		return errors.New("config file path is empty")
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
			return expandErr
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
		return fmt.Errorf("failed to build plan: %w", err)
	}

	log.Debugf("Plan built with %d steps", len(planData.Steps))

	// Execute the plan
	return ExecutePlan(planData, startConfig.SudoPass, startConfig.DryRun, log)
}

// ExecutePlan executes a pre-compiled plan
func ExecutePlan(p *plan.Plan, sudoPass string, dryRun bool, log logger.Logger) error {
	steps := p.Steps
	variables := p.InitialVars

	// Start timing
	startTime := time.Now()

	log.Mooncake()

	// Create dependencies
	renderer := template.NewPongo2Renderer()
	evaluator := expression.NewGovaluateEvaluator()
	pathExpander := pathutil.NewPathExpander(renderer)
	fileTreeWalker := filetree.NewWalker(pathExpander)

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

		// Global progress tracking
		GlobalStepsExecuted: &globalExecuted,

		// Statistics tracking
		StatsExecuted: &statsExecuted,
		StatsSkipped:  &statsSkipped,
		StatsFailed:   &statsFailed,

		// Inject dependencies
		Template:  renderer,
		Evaluator: evaluator,
		PathUtil:  pathExpander,
		FileTree:  fileTreeWalker,
	}

	// Execute pre-expanded steps
	execErr := ExecuteSteps(steps, &executionContext)

	// Calculate duration
	duration := time.Since(startTime)

	// Show completion message
	log.Complete(logger.ExecutionStats{
		Duration: duration,
		Executed: statsExecuted,
		Skipped:  statsSkipped,
		Failed:   statsFailed,
	})

	return execErr
}
