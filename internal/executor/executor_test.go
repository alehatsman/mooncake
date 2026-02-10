package executor_test

import (
	"os"
	"testing"

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

	original := executor.ExecutionContext{
		Variables: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
		CurrentDir:   "/work",
		CurrentFile:  "/work/config.yml",
		Level:        1,
		CurrentIndex: 2,
		TotalSteps:   10,
		Logger:       testLogger,
		SudoPass:     "secret",
		Template:     renderer,
		Evaluator:    evaluator,
		PathUtil:     pathExpander,
		FileTree:     fileTreeWalker,
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
	if copied.SudoPass != original.SudoPass {
		t.Errorf("Copy() SudoPass = %v, want %v", copied.SudoPass, original.SudoPass)
	}

	// Verify variables are deep copied
	if len(copied.Variables) != len(original.Variables) {
		t.Errorf("Copy() Variables length = %v, want %v", len(copied.Variables), len(original.Variables))
	}

	// Modify copied variables
	copied.Variables["key1"] = "modified"

	// Original should be unchanged
	if original.Variables["key1"] == "modified" {
		t.Error("Copy() should deep copy variables")
	}

	// Verify dependencies are shared (not deep copied)
	if copied.Template != original.Template {
		t.Error("Copy() should share Template dependency")
	}
	if copied.Evaluator != original.Evaluator {
		t.Error("Copy() should share Evaluator dependency")
	}
	if copied.PathUtil != original.PathUtil {
		t.Error("Copy() should share PathUtil dependency")
	}
	if copied.FileTree != original.FileTree {
		t.Error("Copy() should share FileTree dependency")
	}
}

func TestAddGlobalVariables(t *testing.T) {
	vars := make(map[string]interface{})

	executor.AddGlobalVariables(vars)

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
}

func TestHandleVars(t *testing.T) {
	testLogger := logger.NewTestLogger()

	vars := map[string]interface{}{
		"new_key": "new_value",
	}

	ec := &executor.ExecutionContext{
		Variables: map[string]interface{}{
			"existing_key": "existing_value",
		},
		Logger: testLogger,
	}

	step := config.Step{
		Name: "test vars",
		Vars: &vars,
	}

	err := executor.HandleVars(step, ec)
	if err != nil {
		t.Fatalf("executor.HandleVars() error = %v", err)
	}

	// Verify new variable was added
	if ec.Variables["new_key"] != "new_value" {
		t.Errorf("executor.HandleVars() new_key = %v, want 'new_value'", ec.Variables["new_key"])
	}

	// Verify existing variable is preserved
	if ec.Variables["existing_key"] != "existing_value" {
		t.Errorf("executor.HandleVars() existing_key = %v, want 'existing_value'", ec.Variables["existing_key"])
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
		name       string
		when       string
		vars       map[string]interface{}
		wantSkip   bool
		wantErr    bool
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
			ec := &executor.ExecutionContext{
				Variables: tt.vars,
				Logger:    testLogger,
				Template:  renderer,
				Redactor:  security.NewRedactor(),
				Evaluator: evaluator,
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
		VarsFilePath:   "/tmp/vars.yml",
		SudoPass:       "password",
	}

	if config.ConfigFilePath != "/tmp/config.yml" {
		t.Errorf("ConfigFilePath = %v, want '/tmp/config.yml'", config.ConfigFilePath)
	}
	if config.VarsFilePath != "/tmp/vars.yml" {
		t.Errorf("VarsFilePath = %v, want '/tmp/vars.yml'", config.VarsFilePath)
	}
	if config.SudoPass != "password" {
		t.Errorf("SudoPass = %v, want 'password'", config.SudoPass)
	}
}

func TestParseFileMode(t *testing.T) {
	tests := []struct {
		name        string
		modeStr     string
		defaultMode uint32
		want        uint32
	}{
		{
			name:        "valid octal",
			modeStr:     "0755",
			defaultMode: 0644,
			want:        0755,
		},
		{
			name:        "valid octal 0644",
			modeStr:     "0644",
			defaultMode: 0755,
			want:        0644,
		},
		{
			name:        "empty string uses default",
			modeStr:     "",
			defaultMode: 0755,
			want:        0755,
		},
		{
			name:        "invalid string uses default",
			modeStr:     "invalid",
			defaultMode: 0644,
			want:        0644,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.ParseFileMode(tt.modeStr, os.FileMode(tt.defaultMode))
			if uint32(got) != tt.want {
				t.Errorf("executor.ParseFileMode() = %o, want %o", got, tt.want)
			}
		})
	}
}

func TestShouldSkipByTags(t *testing.T) {
	tests := []struct {
		name       string
		stepTags   []string
		filterTags []string
		wantSkip   bool
	}{
		{
			name:       "no filter tags - execute all",
			stepTags:   []string{"dev"},
			filterTags: []string{},
			wantSkip:   false,
		},
		{
			name:       "no step tags with filter - skip",
			stepTags:   []string{},
			filterTags: []string{"dev"},
			wantSkip:   true,
		},
		{
			name:       "matching single tag",
			stepTags:   []string{"dev"},
			filterTags: []string{"dev"},
			wantSkip:   false,
		},
		{
			name:       "non-matching single tag",
			stepTags:   []string{"prod"},
			filterTags: []string{"dev"},
			wantSkip:   true,
		},
		{
			name:       "matching one of multiple step tags",
			stepTags:   []string{"dev", "test"},
			filterTags: []string{"dev"},
			wantSkip:   false,
		},
		{
			name:       "matching one of multiple filter tags",
			stepTags:   []string{"dev"},
			filterTags: []string{"dev", "prod"},
			wantSkip:   false,
		},
		{
			name:       "matching any tag",
			stepTags:   []string{"dev", "test", "deploy"},
			filterTags: []string{"prod", "deploy"},
			wantSkip:   false,
		},
		{
			name:       "no matching tags",
			stepTags:   []string{"dev", "test"},
			filterTags: []string{"prod", "deploy"},
			wantSkip:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name: "test step",
				Tags: tt.stepTags,
			}
			ec := &executor.ExecutionContext{
				Tags: tt.filterTags,
				Redactor: security.NewRedactor(),
			}

			got := executor.ShouldSkipByTags(step, ec)
			if got != tt.wantSkip {
				t.Errorf("executor.ShouldSkipByTags() = %v, want %v", got, tt.wantSkip)
			}
		})
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

func TestMarkStepFailed(t *testing.T) {
	testLogger := logger.NewTestLogger()
	result := executor.NewResult()
	ec := &executor.ExecutionContext{
		Variables: make(map[string]interface{}),
		Logger:    testLogger,
	}
	step := config.Step{
		Name:     "test",
		Register: "result",
	}

	executor.MarkStepFailed(result, step, ec)

	if !result.Failed {
		t.Error("executor.MarkStepFailed() should set Failed to true")
	}
	if result.Rc != 1 {
		t.Errorf("executor.MarkStepFailed() Rc = %v, want 1", result.Rc)
	}
	if ec.Variables["result"] == nil {
		t.Error("executor.MarkStepFailed() should register result")
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
		name       string
		when       string
		stepTags   []string
		filterTags []string
		wantSkip   bool
		wantReason string
	}{
		{
			name:       "no conditions",
			when:       "",
			wantSkip:   false,
			wantReason: "",
		},
		{
			name:       "when false",
			when:       "false",
			wantSkip:   true,
			wantReason: "when",
		},
		{
			name:       "tags mismatch",
			stepTags:   []string{"prod"},
			filterTags: []string{"dev"},
			wantSkip:   true,
			wantReason: "tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := &executor.ExecutionContext{
				Variables: make(map[string]interface{}),
				Tags:      tt.filterTags,
				Logger:    testLogger,
				Template:  renderer,
				Evaluator: evaluator,
			}
			step := config.Step{
				When: tt.when,
				Tags: tt.stepTags,
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
	ec := &executor.ExecutionContext{Variables: make(map[string]interface{})}
	step := config.Step{Name: "My Step"}

	name, hasName := executor.GetStepDisplayName(step, ec)
	if name != "My Step" || !hasName {
		t.Errorf("executor.GetStepDisplayName() = (%v, %v), want (My Step, true)", name, hasName)
	}

	// Test with item
	ec.Variables["item"] = "item_value"
	name, hasName = executor.GetStepDisplayName(step, ec)
	if name != "item_value" || !hasName {
		t.Errorf("executor.GetStepDisplayName() with item = (%v, %v), want (item_value, true)", name, hasName)
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
		Variables:           make(map[string]interface{}),
		Logger:              testLogger,
		Template:            renderer,
		Evaluator:           evaluator,
		PathUtil:            pathExpander,
		CurrentDir:          os.TempDir(),
		Stats: executor.NewExecutionStats(),
		Redactor:            security.NewRedactor(),
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

	if *ec.Stats.Global != 1 {
		t.Errorf("GlobalStepsExecuted = %d, want 1", *ec.Stats.Global)
	}
	if *ec.Stats.Executed != 1 {
		t.Errorf("StatsExecuted = %d, want 1", *ec.Stats.Executed)
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
		Variables:           make(map[string]interface{}),
		Logger:              testLogger,
		Template:            renderer,
		Evaluator:           evaluator,
		PathUtil:            pathExpander,
		CurrentDir:          os.TempDir(),
		Stats: executor.NewExecutionStats(),
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

	if *ec.Stats.Global != 0 {
		t.Errorf("GlobalStepsExecuted = %d, want 0 for skipped step", *ec.Stats.Global)
	}
	if *ec.Stats.Skipped != 1 {
		t.Errorf("StatsSkipped = %d, want 1", *ec.Stats.Skipped)
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
		Variables:           make(map[string]interface{}),
		Logger:              testLogger,
		Template:            renderer,
		Evaluator:           evaluator,
		PathUtil:            pathExpander,
		CurrentDir:          os.TempDir(),
		CurrentFile:         "test.yml",
		Stats: executor.NewExecutionStats(),
		Redactor:            security.NewRedactor(),
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

	if *ec.Stats.Global != 2 {
		t.Errorf("GlobalStepsExecuted = %d, want 2", *ec.Stats.Global)
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
			step:    config.Step{File: &config.File{Path: tmpDir + "/test", State: "file"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := &executor.ExecutionContext{
				Variables:  make(map[string]interface{}),
				Logger:     testLogger,
				Template:   renderer,
				Evaluator:  evaluator,
				PathUtil:   pathExpander,
				CurrentDir: tmpDir,
				Redactor:   security.NewRedactor(),
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
	dryRun.LogRegister(config.Step{Register: "result"})

	// If we got here without panicking, the tests pass
}







func strPtr(s string) *string {
	return &s
}








