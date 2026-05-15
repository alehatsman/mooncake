package executor_test

import (
	"os"
	"strings"
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
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func TestCheckIdempotencyConditions_Creates_FileExists(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Step with creates pointing to existing file
	step := config.Step{
		Shell:        &config.ShellAction{Cmd: "echo test"},
		UnlessExists: strPtr(tmpFile.Name()),
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
		},
		Scope: executor.NewVariableScope(),
	}

	shouldSkip, reason, err := executor.CheckIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Error("Expected step to be skipped when file exists")
	}
	if !strings.Contains(reason, "creates:") {
		t.Errorf("Expected reason to contain 'creates:', got: %s", reason)
	}
	if !strings.Contains(reason, tmpFile.Name()) {
		t.Errorf("Expected reason to contain file path, got: %s", reason)
	}
}

func TestCheckIdempotencyConditions_Creates_FileNotExists(t *testing.T) {
	creates := "/nonexistent/file/that/does/not/exist"
	step := config.Step{
		Shell:        &config.ShellAction{Cmd: "echo test"},
		UnlessExists: strPtr(creates),
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
		},
		Scope: executor.NewVariableScope(),
	}

	shouldSkip, _, err := executor.CheckIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if shouldSkip {
		t.Error("Expected step NOT to be skipped when file doesn't exist")
	}
}

func TestCheckIdempotencyConditions_Creates_WithTemplateVariable(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	step := config.Step{
		Shell:        &config.ShellAction{Cmd: "echo test"},
		UnlessExists: strPtr("{{ output_file }}"),
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	scope := executor.NewVariableScope()
	scope.User["output_file"] = tmpFile.Name()
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
		},
		Scope: scope,
	}

	shouldSkip, reason, err := executor.CheckIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Error("Expected step to be skipped when templated file exists")
	}
	if !strings.Contains(reason, "creates:") {
		t.Errorf("Expected reason to contain 'creates:', got: %s", reason)
	}
}

func TestCheckIdempotencyConditions_Unless_CommandSucceeds(t *testing.T) {
	unless := "true" // Always succeeds
	step := config.Step{
		Shell:         &config.ShellAction{Cmd: "echo test"},
		UnlessCommand: &unless,
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
		},
		Scope: executor.NewVariableScope(),
	}

	shouldSkip, reason, err := executor.CheckIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Error("Expected step to be skipped when unless command succeeds")
	}
	if !strings.Contains(reason, "unless:") {
		t.Errorf("Expected reason to contain 'unless:', got: %s", reason)
	}
	if !strings.Contains(reason, "true") {
		t.Errorf("Expected reason to contain command, got: %s", reason)
	}
}

func TestCheckIdempotencyConditions_Unless_CommandFails(t *testing.T) {
	unless := "false" // Always fails
	step := config.Step{
		Shell:         &config.ShellAction{Cmd: "echo test"},
		UnlessCommand: &unless,
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
		},
		Scope: executor.NewVariableScope(),
	}

	shouldSkip, _, err := executor.CheckIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if shouldSkip {
		t.Error("Expected step NOT to be skipped when unless command fails")
	}
}

func TestCheckIdempotencyConditions_Unless_WithTemplateVariable(t *testing.T) {
	step := config.Step{
		Shell:         &config.ShellAction{Cmd: "echo test"},
		UnlessCommand: strPtr("test -f {{ marker_file }}"),
	}

	// Create temp file for testing
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	scope := executor.NewVariableScope()
	scope.User["marker_file"] = tmpFile.Name()
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
		},
		Scope: scope,
	}

	shouldSkip, reason, err := executor.CheckIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Error("Expected step to be skipped when unless command with template succeeds")
	}
	if !strings.Contains(reason, "unless:") {
		t.Errorf("Expected reason to contain 'unless:', got: %s", reason)
	}
}

func TestCheckIdempotencyConditions_BothConditions(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Both creates and unless are satisfied
	step := config.Step{
		Shell:         &config.ShellAction{Cmd: "echo test"},
		UnlessExists:  strPtr(tmpFile.Name()),
		UnlessCommand: strPtr("true"),
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
		},
		Scope: executor.NewVariableScope(),
	}

	shouldSkip, reason, err := executor.CheckIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Error("Expected step to be skipped when creates condition is met")
	}
	// Creates is checked first, so reason should be about creates
	if !strings.Contains(reason, "creates:") {
		t.Errorf("Expected reason to contain 'creates:', got: %s", reason)
	}
}

func TestCheckIdempotencyConditions_NoConditions(t *testing.T) {
	step := config.Step{
		Shell: &config.ShellAction{Cmd: *strPtr("echo test")},
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
		},
		Scope: executor.NewVariableScope(),
	}

	shouldSkip, _, err := executor.CheckIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if shouldSkip {
		t.Error("Expected step NOT to be skipped when no idempotency conditions")
	}
}

func TestExecuteStep_IdempotencyIntegration(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	step := config.Step{
		Name:         "Test step with creates",
		Shell:        &config.ShellAction{Cmd: "echo should not run"},
		UnlessExists: strPtr(tmpFile.Name()),
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}

	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:  renderer,
			PathUtil:  pathutil.NewPathExpander(renderer),
			Evaluator: expression.NewGovaluateEvaluator(),
			Logger:    logger.NewConsoleLogger(logger.InfoLevel),
			Stats:     executor.NewExecutionStats(),
		},
		Scope: executor.NewVariableScope(),
	}

	err = executor.ExecuteStep(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Step should be skipped
	if *ec.Svc.Stats.Skipped != 1 {
		t.Errorf("Expected 1 skipped step, got %d", *ec.Svc.Stats.Skipped)
	}
	if *ec.Svc.Stats.Executed != 0 {
		t.Errorf("Expected 0 executed steps, got %d", *ec.Svc.Stats.Executed)
	}
}

// TestCheckIdempotencyConditions_ShellLevelCreates is a regression test for
// manual-test #2 (2026-05-15): action-level guards on the ShellAction
// (`shell: { cmd:..., creates:... }`) must trigger the same skip path as
// the step-level unless_exists field. Before the fix the action-level
// keys were silently dropped, the command ran, and the recap counted it
// as changed=true.
func TestCheckIdempotencyConditions_ShellLevelCreates(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "shell-creates-")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	step := config.Step{
		Shell: &config.ShellAction{
			Cmd:     "echo would-run",
			Creates: tmpFile.Name(),
		},
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
		},
		Scope: executor.NewVariableScope(),
	}

	shouldSkip, reason, err := executor.CheckIdempotencyConditions(step, ec)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Errorf("expected skip when shell.creates points at existing file, got shouldSkip=false reason=%q", reason)
	}
	if !strings.Contains(reason, "creates:") || !strings.Contains(reason, tmpFile.Name()) {
		t.Errorf("expected reason to mention creates+path, got %q", reason)
	}
}

// MT-2 (step-level form): when the user writes
//
//	- name: ...
//	  shell: touch /tmp/once.flag
//	  creates: /tmp/once.flag
//
// the YAML produces step.Shell.Cmd = "touch /tmp/once.flag" and
// step.Creates = "/tmp/once.flag". The verification doc reported
// this combination still ran on the second pass (recap counted as
// `changed`). Pin the contract end-to-end so any regression that
// re-introduces the bug fails here.
func TestExecuteStep_ShellStepLevelCreatesSkips(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "mt2-step-shell-")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	step := config.Step{
		Name:    "MT-2: shell step-level creates",
		Shell:   &config.ShellAction{Cmd: "echo would-run"},
		Creates: strPtr(tmpFile.Name()),
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:  renderer,
			PathUtil:  pathutil.NewPathExpander(renderer),
			Evaluator: expression.NewGovaluateEvaluator(),
			Logger:    logger.NewConsoleLogger(logger.InfoLevel),
			Stats:     executor.NewExecutionStats(),
		},
		Scope: executor.NewVariableScope(),
	}

	if err := executor.ExecuteStep(step, ec); err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}
	if *ec.Svc.Stats.Skipped != 1 {
		t.Errorf("expected 1 skipped (step-level creates+shell), got %d", *ec.Svc.Stats.Skipped)
	}
	if *ec.Svc.Stats.Executed != 0 {
		t.Errorf("expected 0 executed (the shell guard should have fired), got %d", *ec.Svc.Stats.Executed)
	}
}

// MT-2 (step-level form, unless variant): same shape with `unless:`.
func TestExecuteStep_ShellStepLevelUnlessSkips(t *testing.T) {
	step := config.Step{
		Name:   "MT-2: shell step-level unless",
		Shell:  &config.ShellAction{Cmd: "echo would-run"},
		Unless: strPtr("true"), // exits 0 → skip
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:  renderer,
			PathUtil:  pathutil.NewPathExpander(renderer),
			Evaluator: expression.NewGovaluateEvaluator(),
			Logger:    logger.NewConsoleLogger(logger.InfoLevel),
			Stats:     executor.NewExecutionStats(),
		},
		Scope: executor.NewVariableScope(),
	}

	if err := executor.ExecuteStep(step, ec); err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}
	if *ec.Svc.Stats.Skipped != 1 {
		t.Errorf("expected 1 skipped (step-level unless+shell), got %d", *ec.Svc.Stats.Skipped)
	}
}

func TestCheckIdempotencyConditions_ShellLevelUnless(t *testing.T) {
	// Pick an unless command that always exits 0 → expect skip.
	step := config.Step{
		Shell: &config.ShellAction{
			Cmd:    "echo would-run",
			Unless: "true",
		},
	}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
		},
		Scope: executor.NewVariableScope(),
	}

	shouldSkip, reason, err := executor.CheckIdempotencyConditions(step, ec)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Errorf("expected skip when shell.unless exits 0, got shouldSkip=false reason=%q", reason)
	}
	if !strings.Contains(reason, "unless:") {
		t.Errorf("expected reason to mention unless:, got %q", reason)
	}
}

// --- MT-15 regression tests ----------------------------------------------
//
// MT-15: `creates:` / `unless:` were silently ignored on every non-shell
// action (file.write, text.*, pkg, …) because the executor gated the
// idempotency check on `step.Shell != nil || step.Cmd != nil`. The fix
// removes that gate AND adds friendly step-level `creates:` / `unless:`
// aliases for the existing `unless_exists:` / `unless_command:` fields.

// TestCheckIdempotencyConditions_StepLevelCreatesAlias verifies the
// alias resolves the same skip path as unless_exists:.
func TestCheckIdempotencyConditions_StepLevelCreatesAlias(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "mt15-creates-")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	step := config.Step{
		FileWrite: &config.File{Path: tmpFile.Name(), State: "file", Content: "v1\n"},
		Creates:   strPtr(tmpFile.Name()),
	}
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
		},
		Scope: executor.NewVariableScope(),
	}
	shouldSkip, reason, err := executor.CheckIdempotencyConditions(step, ec)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !shouldSkip {
		t.Errorf("step-level creates: alias should trigger skip, got shouldSkip=false reason=%q", reason)
	}
	if !strings.Contains(reason, "creates:") {
		t.Errorf("expected reason to mention creates:, got %q", reason)
	}
}

// TestCheckIdempotencyConditions_StepLevelUnlessAlias verifies the
// step-level unless: alias triggers skip when the command succeeds.
func TestCheckIdempotencyConditions_StepLevelUnlessAlias(t *testing.T) {
	step := config.Step{
		FileWrite: &config.File{Path: "/tmp/mt15-unless-x", State: "file", Content: "v1\n"},
		Unless:    strPtr("true"),
	}
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
		},
		Scope: executor.NewVariableScope(),
	}
	shouldSkip, reason, err := executor.CheckIdempotencyConditions(step, ec)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !shouldSkip {
		t.Errorf("step-level unless: alias should trigger skip when cmd succeeds, got reason=%q", reason)
	}
	if !strings.Contains(reason, "unless:") {
		t.Errorf("expected reason to mention unless:, got %q", reason)
	}
}

// TestExecuteStep_NonShellActionHonorsUnlessExists is the headline
// MT-15 regression: a step with file.write + unless_exists pointing at
// an existing file must skip — the executor used to gate this check
// on Shell/Cmd presence so file.write ran unconditionally.
func TestExecuteStep_NonShellActionHonorsUnlessExists(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "mt15-fw-")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString("v0-already-here\n"); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	tmpFile.Close()

	step := config.Step{
		Name: "MT-15: file.write guarded by unless_exists",
		FileWrite: &config.File{
			Path:    tmpFile.Name(),
			State:   "file",
			Content: "v1-from-write\n",
		},
		UnlessExists: strPtr(tmpFile.Name()),
	}
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:  renderer,
			PathUtil:  pathutil.NewPathExpander(renderer),
			Evaluator: expression.NewGovaluateEvaluator(),
			Logger:    logger.NewConsoleLogger(logger.ErrorLevel),
			Stats:     executor.NewExecutionStats(),
		},
		Scope: executor.NewVariableScope(),
	}
	if err := executor.ExecuteStep(step, ec); err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}
	if *ec.Svc.Stats.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", *ec.Svc.Stats.Skipped)
	}
	if *ec.Svc.Stats.Executed != 0 {
		t.Errorf("expected 0 executed, got %d", *ec.Svc.Stats.Executed)
	}
	// Confirm the side effect was suppressed: the seed content
	// survives. Before the fix file.write rewrote the file.
	got, rerr := os.ReadFile(tmpFile.Name())
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	if string(got) != "v0-already-here\n" {
		t.Errorf("file.write should have been skipped — file content changed to %q", string(got))
	}
}

// TestExecuteStep_NonShellActionHonorsStepLevelCreatesAlias mirrors
// the headline regression using the friendly `creates:` alias.
func TestExecuteStep_NonShellActionHonorsStepLevelCreatesAlias(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "mt15-fw-alias-")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString("v0\n"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	tmpFile.Close()

	step := config.Step{
		Name: "MT-15: file.write guarded by step-level creates: alias",
		FileWrite: &config.File{
			Path:    tmpFile.Name(),
			State:   "file",
			Content: "v1\n",
		},
		Creates: strPtr(tmpFile.Name()),
	}
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:  renderer,
			PathUtil:  pathutil.NewPathExpander(renderer),
			Evaluator: expression.NewGovaluateEvaluator(),
			Logger:    logger.NewConsoleLogger(logger.ErrorLevel),
			Stats:     executor.NewExecutionStats(),
		},
		Scope: executor.NewVariableScope(),
	}
	if err := executor.ExecuteStep(step, ec); err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}
	if *ec.Svc.Stats.Skipped != 1 {
		t.Errorf("expected 1 skipped (step-level creates: alias), got %d", *ec.Svc.Stats.Skipped)
	}
	got, rerr := os.ReadFile(tmpFile.Name())
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	if string(got) != "v0\n" {
		t.Errorf("step-level creates: alias should skip file.write; content changed to %q", string(got))
	}
}
