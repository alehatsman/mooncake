package executor_test

import (
	"os"
	"testing"
	"time"

	_ "github.com/alehatsman/mooncake/internal/actions/assert"
	_ "github.com/alehatsman/mooncake/internal/actions/command"
	_ "github.com/alehatsman/mooncake/internal/actions/copy"
	_ "github.com/alehatsman/mooncake/internal/actions/download"
	_ "github.com/alehatsman/mooncake/internal/actions/file"
	_ "github.com/alehatsman/mooncake/internal/actions/include_vars"
	_ "github.com/alehatsman/mooncake/internal/actions/preset"
	_ "github.com/alehatsman/mooncake/internal/actions/print"
	_ "github.com/alehatsman/mooncake/internal/actions/service"
	_ "github.com/alehatsman/mooncake/internal/actions/shell"
	_ "github.com/alehatsman/mooncake/internal/actions/template"
	_ "github.com/alehatsman/mooncake/internal/actions/unarchive"
	_ "github.com/alehatsman/mooncake/internal/actions/vars"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
	"github.com/alehatsman/mooncake/internal/utils"
)

func TestExecutionContext_Copy(t *testing.T) {
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	evaluator := expression.NewGovaluateEvaluator()
	pathExpander := pathutil.NewPathExpander(renderer)
	fileTreeWalker := filetree.NewWalker(pathExpander)
	testLogger := logger.NewTestLogger()

	scope := &executor.VariableScope{
		User:    map[string]interface{}{"key1": "value1", "key2": "value2"},
		Results: make(map[string]executor.RegisteredResult),
	}
	original := executor.ExecutionContext{
		Svc: &executor.RunServices{
			Logger:    testLogger,
			SudoPass:  "secret",
			Template:  renderer,
			Evaluator: evaluator,
			PathUtil:  pathExpander,
			FileTree:  fileTreeWalker,
		},
		Scope:        scope,
		CurrentDir:   "/work",
		CurrentFile:  "/work/config.yml",
		Level:        1,
		CurrentIndex: 2,
		TotalSteps:   10,
	}

	copied := original.Clone()

	// Verify all fields are copied
	if copied.CurrentDir != original.CurrentDir {
		t.Errorf("Copy() CurrentDir = %v, want %v", copied.CurrentDir, original.CurrentDir)
	}
	if copied.CurrentFile != original.CurrentFile {
		t.Errorf("Copy() CurrentFile = %v, want %v", copied.CurrentFile, original.CurrentFile)
	}
	if copied.Level != original.Level {
		t.Errorf("Copy() Level = %v, want %v", copied.Level, original.Level)
	}
	if copied.Svc.SudoPass != original.Svc.SudoPass {
		t.Errorf("Copy() SudoPass = %v, want %v", copied.Svc.SudoPass, original.Svc.SudoPass)
	}

	// Verify scope user vars are deep copied
	if len(copied.Scope.User) != len(original.Scope.User) {
		t.Errorf("Copy() Scope.User length = %v, want %v", len(copied.Scope.User), len(original.Scope.User))
	}

	// Modify copied scope user vars
	copied.Scope.User["key1"] = "modified"

	// Original should be unchanged
	if original.Scope.User["key1"] == "modified" {
		t.Error("Copy() should deep copy Scope.User")
	}

	// Verify dependencies are shared (not deep copied)
	if copied.Svc.Template != original.Svc.Template {
		t.Error("Copy() should share Template dependency")
	}
	if copied.Svc.Evaluator != original.Svc.Evaluator {
		t.Error("Copy() should share Evaluator dependency")
	}
	if copied.Svc.PathUtil != original.Svc.PathUtil {
		t.Error("Copy() should share PathUtil dependency")
	}
	if copied.Svc.FileTree != original.Svc.FileTree {
		t.Error("Copy() should share FileTree dependency")
	}
}

func TestAddGlobalVariables(t *testing.T) {
	scope := executor.NewVariableScope()

	executor.AddGlobalVariables(scope)

	vars := scope.ToMap()

	// Should add os and arch
	if vars["os"] == nil {
		t.Error("executor.AddGlobalVariables() should add 'os'")
	}
	if vars["arch"] == nil {
		t.Error("executor.AddGlobalVariables() should add 'arch'")
	}

	// Verify they are strings
	if _, ok := vars["os"].(string); !ok {
		t.Errorf("executor.AddGlobalVariables() os should be string, got %T", vars["os"])
	}
	if _, ok := vars["arch"].(string); !ok {
		t.Errorf("executor.AddGlobalVariables() arch should be string, got %T", vars["arch"])
	}

	// proposal-09: apply_started_at is stamped on the scope at this
	// point and surfaces in ToMap as a time.Time.
	if scope.ApplyStartedAt.IsZero() {
		t.Error("executor.AddGlobalVariables() should set ApplyStartedAt")
	}
	if _, ok := vars["apply_started_at"].(time.Time); !ok {
		t.Errorf("ToMap() apply_started_at should be time.Time, got %T", vars["apply_started_at"])
	}
}

func TestVariableScope_ApplyStartedAt_OmittedWhenZero(t *testing.T) {
	// A freshly-constructed scope (no AddGlobalVariables) must NOT
	// expose apply_started_at — keeps test fixtures clean and stops
	// downstream consumers from seeing the zero time.
	scope := executor.NewVariableScope()
	vars := scope.ToMap()
	if _, present := vars["apply_started_at"]; present {
		t.Error("ToMap() should omit apply_started_at when zero")
	}
}

func TestHandleWhenExpression(t *testing.T) {
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	evaluator := expression.NewGovaluateEvaluator()
	testLogger := logger.NewTestLogger()

	tests := []struct {
		name     string
		when     string
		vars     map[string]interface{}
		wantSkip bool
		wantErr  bool
	}{
		{
			name:     "true condition",
			when:     "true",
			vars:     map[string]interface{}{},
			wantSkip: false,
			wantErr:  false,
		},
		{
			name:     "false condition",
			when:     "false",
			vars:     map[string]interface{}{},
			wantSkip: true,
			wantErr:  false,
		},
		{
			name:     "variable equals",
			when:     "x == 5",
			vars:     map[string]interface{}{"x": 5},
			wantSkip: false,
			wantErr:  false,
		},
		{
			name:     "variable not equals",
			when:     "x == 10",
			vars:     map[string]interface{}{"x": 5},
			wantSkip: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := executor.NewVariableScope()
			scope.User = tt.vars
			ec := &executor.ExecutionContext{
				Svc: &executor.RunServices{
					Logger:    testLogger,
					Template:  renderer,
					Redactor:  security.NewRedactor(),
					Evaluator: evaluator,
				},
				Scope: scope,
			}

			step := config.Step{
				When: tt.when,
			}

			skip, err := executor.HandleWhenExpression(step, ec)
			if (err != nil) != tt.wantErr {
				t.Errorf("executor.HandleWhenExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if skip != tt.wantSkip {
				t.Errorf("executor.HandleWhenExpression() skip = %v, want %v", skip, tt.wantSkip)
			}
		})
	}
}

func TestStartConfig(t *testing.T) {
	// Test that executor.StartConfig struct can be created
	config := executor.StartConfig{
		ConfigFilePath: "/tmp/config.yml",
		VarsFilePaths:  []string{"/tmp/vars.yml"},
		SudoPass:       "password",
	}

	if config.ConfigFilePath != "/tmp/config.yml" {
		t.Errorf("ConfigFilePath = %v, want '/tmp/config.yml'", config.ConfigFilePath)
	}
	if len(config.VarsFilePaths) != 1 || config.VarsFilePaths[0] != "/tmp/vars.yml" {
		t.Errorf("VarsFilePaths = %v, want ['/tmp/vars.yml']", config.VarsFilePaths)
	}
	if config.SudoPass != "password" {
		t.Errorf("SudoPass = %v, want 'password'", config.SudoPass)
	}
}

func TestMergeVariables(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]interface{}
		override map[string]interface{}
		want     map[string]interface{}
	}{
		{
			name:     "empty maps",
			base:     map[string]interface{}{},
			override: map[string]interface{}{},
			want:     map[string]interface{}{},
		},
		{
			name:     "override takes precedence",
			base:     map[string]interface{}{"key1": "old"},
			override: map[string]interface{}{"key1": "new"},
			want:     map[string]interface{}{"key1": "new"},
		},
		{
			name:     "merge non-overlapping keys",
			base:     map[string]interface{}{"key1": "value1"},
			override: map[string]interface{}{"key2": "value2"},
			want:     map[string]interface{}{"key1": "value1", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.MergeVariables(tt.base, tt.override)
			if len(got) != len(tt.want) {
				t.Errorf("utils.MergeVariables() length = %v, want %v", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("utils.MergeVariables()[%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestCheckSkipConditions(t *testing.T) {
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	evaluator := expression.NewGovaluateEvaluator()
	testLogger := logger.NewTestLogger()

	tests := []struct {
		name        string
		when        string
		stepSkipped bool // Set by planner during plan compilation
		wantSkip    bool
		wantReason  string
	}{
		{
			name:        "no conditions",
			when:        "",
			stepSkipped: false,
			wantSkip:    false,
			wantReason:  "",
		},
		{
			name:        "when false",
			when:        "false",
			stepSkipped: false,
			wantSkip:    true,
			wantReason:  "when: false",
		},
		{
			name:        "tags mismatch - marked skipped by planner",
			stepSkipped: true, // Planner already evaluated tags and marked as skipped
			wantSkip:    true,
			wantReason:  "tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := &executor.ExecutionContext{
				Svc: &executor.RunServices{
					Logger:    testLogger,
					Template:  renderer,
					Evaluator: evaluator,
				},
				Scope: executor.NewVariableScope(),
			}
			step := config.Step{
				When:    tt.when,
				Skipped: tt.stepSkipped, // Trust planner's decision
			}

			skip, reason, _ := executor.CheckSkipConditions(step, ec)
			if skip != tt.wantSkip {
				t.Errorf("executor.CheckSkipConditions() skip = %v, want %v", skip, tt.wantSkip)
			}
			if reason != tt.wantReason {
				t.Errorf("executor.CheckSkipConditions() reason = %v, want %v", reason, tt.wantReason)
			}
		})
	}
}

func TestGetStepDisplayName(t *testing.T) {
	scope := executor.NewVariableScope()
	ec := &executor.ExecutionContext{Scope: scope}
	step := config.Step{Name: "My Step"}

	name, hasName := executor.GetStepDisplayName(step, ec)
	if name != "My Step" || !hasName {
		t.Errorf("executor.GetStepDisplayName() = (%v, %v), want (My Step, true)", name, hasName)
	}

	// Test with item: name should be combined with item, not clobbered.
	ec.Scope.User["item"] = "item_value"
	name, hasName = executor.GetStepDisplayName(step, ec)
	if name != "My Step: item_value" || !hasName {
		t.Errorf("executor.GetStepDisplayName() with item = (%v, %v), want (My Step: item_value, true)", name, hasName)
	}

	// Test with item but no step name: falls back to item value only.
	stepNoName := config.Step{}
	name, hasName = executor.GetStepDisplayName(stepNoName, ec)
	if name != "item_value" || !hasName {
		t.Errorf("executor.GetStepDisplayName() unnamed step with item = (%v, %v), want (item_value, true)", name, hasName)
	}
}

func TestExecuteStep_WithShell(t *testing.T) {
	testLogger := logger.NewTestLogger()
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	evaluator := expression.NewGovaluateEvaluator()
	pathExpander := pathutil.NewPathExpander(renderer)

	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Logger:    testLogger,
			Template:  renderer,
			Evaluator: evaluator,
			PathUtil:  pathExpander,
			Stats:     executor.NewExecutionStats(),
			Redactor:  security.NewRedactor(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: os.TempDir(),
	}

	shellCmd := "echo test"
	step := config.Step{
		Name:  "test step",
		Shell: &config.ShellAction{Cmd: shellCmd},
	}

	err = executor.ExecuteStep(step, ec)
	if err != nil {
		t.Errorf("executor.ExecuteStep() error = %v", err)
	}

	if *ec.Svc.Stats.Global != 1 {
		t.Errorf("GlobalStepsExecuted = %d, want 1", *ec.Svc.Stats.Global)
	}
	if *ec.Svc.Stats.Executed != 1 {
		t.Errorf("StatsExecuted = %d, want 1", *ec.Svc.Stats.Executed)
	}
}

func TestExecuteStep_Skipped(t *testing.T) {
	testLogger := logger.NewTestLogger()
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	evaluator := expression.NewGovaluateEvaluator()
	pathExpander := pathutil.NewPathExpander(renderer)

	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Logger:    testLogger,
			Template:  renderer,
			Evaluator: evaluator,
			PathUtil:  pathExpander,
			Stats:     executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: os.TempDir(),
	}

	shellCmd := "echo test"
	step := config.Step{
		Name:  "skipped step",
		Shell: &config.ShellAction{Cmd: shellCmd},
		When:  "false",
	}

	err = executor.ExecuteStep(step, ec)
	if err != nil {
		t.Errorf("executor.ExecuteStep() error = %v", err)
	}

	if *ec.Svc.Stats.Global != 0 {
		t.Errorf("GlobalStepsExecuted = %d, want 0 for skipped step", *ec.Svc.Stats.Global)
	}
	if *ec.Svc.Stats.Skipped != 1 {
		t.Errorf("StatsSkipped = %d, want 1", *ec.Svc.Stats.Skipped)
	}
}

func TestExecuteSteps(t *testing.T) {
	testLogger := logger.NewTestLogger()
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	evaluator := expression.NewGovaluateEvaluator()
	pathExpander := pathutil.NewPathExpander(renderer)

	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Logger:    testLogger,
			Template:  renderer,
			Evaluator: evaluator,
			PathUtil:  pathExpander,
			Stats:     executor.NewExecutionStats(),
			Redactor:  security.NewRedactor(),
		},
		Scope:       executor.NewVariableScope(),
		CurrentDir:  os.TempDir(),
		CurrentFile: "test.yml",
	}

	shellCmd1 := "echo step1"
	shellCmd2 := "echo step2"
	steps := []config.Step{
		{Name: "step 1", Shell: &config.ShellAction{Cmd: shellCmd1}},
		{Name: "step 2", Shell: &config.ShellAction{Cmd: shellCmd2}},
	}

	err = executor.ExecuteSteps(steps, ec)
	if err != nil {
		t.Errorf("executor.ExecuteSteps() error = %v", err)
	}

	if *ec.Svc.Stats.Global != 2 {
		t.Errorf("GlobalStepsExecuted = %d, want 2", *ec.Svc.Stats.Global)
	}
}

func TestDispatchStepAction(t *testing.T) {
	testLogger := logger.NewTestLogger()
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	evaluator := expression.NewGovaluateEvaluator()
	pathExpander := pathutil.NewPathExpander(renderer)

	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		step    config.Step
		wantErr bool
	}{
		{
			name:    "shell action",
			step:    config.Step{Shell: &config.ShellAction{Cmd: "echo test"}},
			wantErr: false,
		},
		{
			name:    "vars action",
			step:    config.Step{Vars: &map[string]interface{}{"key": "value"}},
			wantErr: false,
		},
		{
			name:    "file action",
			step:    config.Step{FileWrite: &config.File{Path: tmpDir + "/test", State: "file"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := &executor.ExecutionContext{
				Svc: &executor.RunServices{
					Logger:    testLogger,
					Template:  renderer,
					Evaluator: evaluator,
					PathUtil:  pathExpander,
					Redactor:  security.NewRedactor(),
				},
				Scope:      executor.NewVariableScope(),
				CurrentDir: tmpDir,
			}

			err = executor.DispatchStepAction(tt.step, ec)
			if (err != nil) != tt.wantErr {
				t.Errorf("executor.DispatchStepAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDryRunLogger(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRun := executor.NewDryRunLogger(testLogger)

	// Test all dry-run logging methods
	dryRun.LogShellExecution("echo test", false)
	dryRun.LogShellExecution("echo test", true)
	dryRun.LogTemplateRender("/src", "/dest", 0644)
	dryRun.LogVariableLoad(5, "/path/vars.yml")
	dryRun.LogVariableSet(3)
	dryRun.LogRegister(config.Step{As: "result"})

	// If we got here without panicking, the tests pass
}

func strPtr(s string) *string {
	return &s
}
