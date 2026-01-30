package executor

import (
	"os"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func TestExecutionContext_Copy(t *testing.T) {
	renderer := template.NewPongo2Renderer()
	evaluator := expression.NewGovaluateEvaluator()
	pathExpander := pathutil.NewPathExpander(renderer)
	fileTreeWalker := filetree.NewWalker(pathExpander)
	testLogger := logger.NewTestLogger()

	original := ExecutionContext{
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

	copied := original.Copy()

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

	addGlobalVariables(vars)

	// Should add os and arch
	if vars["os"] == nil {
		t.Error("addGlobalVariables() should add 'os'")
	}
	if vars["arch"] == nil {
		t.Error("addGlobalVariables() should add 'arch'")
	}

	// Verify they are strings
	if _, ok := vars["os"].(string); !ok {
		t.Errorf("addGlobalVariables() os should be string, got %T", vars["os"])
	}
	if _, ok := vars["arch"].(string); !ok {
		t.Errorf("addGlobalVariables() arch should be string, got %T", vars["arch"])
	}
}

func TestHandleVars(t *testing.T) {
	testLogger := logger.NewTestLogger()

	vars := map[string]interface{}{
		"new_key": "new_value",
	}

	ec := &ExecutionContext{
		Variables: map[string]interface{}{
			"existing_key": "existing_value",
		},
		Logger: testLogger,
	}

	step := config.Step{
		Name: "test vars",
		Vars: &vars,
	}

	err := handleVars(step, ec)
	if err != nil {
		t.Fatalf("handleVars() error = %v", err)
	}

	// Verify new variable was added
	if ec.Variables["new_key"] != "new_value" {
		t.Errorf("handleVars() new_key = %v, want 'new_value'", ec.Variables["new_key"])
	}

	// Verify existing variable is preserved
	if ec.Variables["existing_key"] != "existing_value" {
		t.Errorf("handleVars() existing_key = %v, want 'existing_value'", ec.Variables["existing_key"])
	}
}

func TestHandleWhenExpression(t *testing.T) {
	renderer := template.NewPongo2Renderer()
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
			ec := &ExecutionContext{
				Variables: tt.vars,
				Logger:    testLogger,
				Template:  renderer,
				Evaluator: evaluator,
			}

			step := config.Step{
				When: tt.when,
			}

			skip, err := handleWhenExpression(step, ec)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleWhenExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if skip != tt.wantSkip {
				t.Errorf("handleWhenExpression() skip = %v, want %v", skip, tt.wantSkip)
			}
		})
	}
}

func TestStartConfig(t *testing.T) {
	// Test that StartConfig struct can be created
	config := StartConfig{
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
			got := parseFileMode(tt.modeStr, os.FileMode(tt.defaultMode))
			if uint32(got) != tt.want {
				t.Errorf("parseFileMode() = %o, want %o", got, tt.want)
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
			ec := &ExecutionContext{
				Tags: tt.filterTags,
			}

			got := shouldSkipByTags(step, ec)
			if got != tt.wantSkip {
				t.Errorf("shouldSkipByTags() = %v, want %v", got, tt.wantSkip)
			}
		})
	}
}
