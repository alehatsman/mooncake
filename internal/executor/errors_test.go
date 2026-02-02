package executor

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderError(t *testing.T) {
	baseErr := errors.New("syntax error")
	err := &RenderError{Field: "command", Cause: baseErr}

	// Test Error() message
	if !strings.Contains(err.Error(), "failed to render command") {
		t.Errorf("unexpected message: %v", err)
	}

	// Test error unwrapping
	if !errors.Is(err, baseErr) {
		t.Error("error chain broken")
	}
}

func TestEvaluationError(t *testing.T) {
	baseErr := errors.New("undefined variable")
	err := &EvaluationError{Expression: "x > 5", Cause: baseErr}

	// Test Error() message
	msg := err.Error()
	if !strings.Contains(msg, "failed to evaluate x > 5") {
		t.Errorf("unexpected message: %v", msg)
	}

	// Test error unwrapping
	if !errors.Is(err, baseErr) {
		t.Error("error chain broken")
	}
}

func TestCommandError_Timeout(t *testing.T) {
	err := &CommandError{Timeout: true, Duration: "5s", ExitCode: 124}
	msg := err.Error()
	if !strings.Contains(msg, "timed out after 5s") {
		t.Errorf("unexpected message: %v", msg)
	}
}

func TestCommandError_ExitCode(t *testing.T) {
	err := &CommandError{Timeout: false, ExitCode: 127}
	msg := err.Error()
	if !strings.Contains(msg, "exit code 127") {
		t.Errorf("unexpected message: %v", msg)
	}
}

func TestCommandError_WithCause(t *testing.T) {
	baseErr := errors.New("command not found")
	err := &CommandError{
		Timeout:  false,
		ExitCode: 127,
		Cause:    baseErr,
	}

	// Test Error() message includes cause
	msg := err.Error()
	if !strings.Contains(msg, "exit code 127") {
		t.Errorf("message should contain exit code, got: %v", msg)
	}
	if !strings.Contains(msg, "command not found") {
		t.Errorf("message should contain cause, got: %v", msg)
	}

	// Test error unwrapping
	if !errors.Is(err, baseErr) {
		t.Error("error chain broken")
	}
}

func TestCommandError_NilCause(t *testing.T) {
	err := &CommandError{
		Timeout:  false,
		ExitCode: 1,
		Cause:    nil,
	}

	msg := err.Error()
	if strings.Contains(msg, "<nil>") {
		t.Errorf("message should not contain '<nil>', got: %v", msg)
	}
	if !strings.Contains(msg, "exit code 1") {
		t.Errorf("message should contain exit code, got: %v", msg)
	}
}

func TestFileOperationError(t *testing.T) {
	baseErr := errors.New("permission denied")
	err := &FileOperationError{
		Operation: "write",
		Path:      "/etc/hosts",
		Cause:     baseErr,
	}

	// Test Error() message
	msg := err.Error()
	if !strings.Contains(msg, "failed to write file /etc/hosts") {
		t.Errorf("unexpected message: %v", msg)
	}

	// Test error unwrapping
	if !errors.Is(err, baseErr) {
		t.Error("error chain broken")
	}
}

func TestStepValidationError(t *testing.T) {
	err := &StepValidationError{
		Field:   "src",
		Message: "required for link state",
	}

	msg := err.Error()
	if !strings.Contains(msg, "src") || !strings.Contains(msg, "required") {
		t.Errorf("unexpected message: %v", msg)
	}
}

func TestSetupError_WithCause(t *testing.T) {
	baseErr := errors.New("invalid format")
	err := &SetupError{
		Component: "timeout",
		Issue:     "invalid duration \"abc\"",
		Cause:     baseErr,
	}

	// Test Error() message
	msg := err.Error()
	if !strings.Contains(msg, "timeout setup failed") {
		t.Errorf("unexpected message: %v", msg)
	}

	// Test error unwrapping
	if !errors.Is(err, baseErr) {
		t.Error("error chain broken")
	}
}

func TestSetupError_WithoutCause(t *testing.T) {
	err := &SetupError{
		Component: "become",
		Issue:     "not supported on darwin",
	}

	msg := err.Error()
	if !strings.Contains(msg, "become setup failed") || !strings.Contains(msg, "not supported") {
		t.Errorf("unexpected message: %v", msg)
	}
}

func TestErrorTypeAssertion(t *testing.T) {
	// Test that errors.As() works correctly
	var renderErr *RenderError
	err := &RenderError{Field: "test", Cause: errors.New("base")}

	if !errors.As(err, &renderErr) {
		t.Error("errors.As() failed for RenderError")
	}

	if renderErr.Field != "test" {
		t.Errorf("Field = %q, want %q", renderErr.Field, "test")
	}
}

// Test nil Cause handling
func TestRenderError_NilCause(t *testing.T) {
	err := &RenderError{Field: "template", Cause: nil}
	msg := err.Error()

	if strings.Contains(msg, "<nil>") {
		t.Errorf("error message should not contain '<nil>', got: %v", msg)
	}

	if !strings.Contains(msg, "failed to render template") {
		t.Errorf("unexpected message: %v", msg)
	}
}

func TestEvaluationError_NilCause(t *testing.T) {
	err := &EvaluationError{Expression: "x > 0", Cause: nil}
	msg := err.Error()

	if strings.Contains(msg, "<nil>") {
		t.Errorf("error message should not contain '<nil>', got: %v", msg)
	}

	if !strings.Contains(msg, "failed to evaluate x > 0") {
		t.Errorf("unexpected message: %v", msg)
	}
}

func TestFileOperationError_NilCause(t *testing.T) {
	err := &FileOperationError{
		Operation: "read",
		Path:      "/tmp/test.txt",
		Cause:     nil,
	}
	msg := err.Error()

	if strings.Contains(msg, "<nil>") {
		t.Errorf("error message should not contain '<nil>', got: %v", msg)
	}

	if !strings.Contains(msg, "failed to read file /tmp/test.txt") {
		t.Errorf("unexpected message: %v", msg)
	}
}

// Test empty field scenarios
func TestStepValidationError_EmptyField(t *testing.T) {
	err := &StepValidationError{
		Field:   "",
		Message: "some validation error",
	}
	msg := err.Error()

	// Should still produce a valid error message
	if msg == "" {
		t.Error("error message should not be empty")
	}

	if !strings.Contains(msg, "some validation error") {
		t.Errorf("unexpected message: %v", msg)
	}
}

func TestStepValidationError_EmptyMessage(t *testing.T) {
	err := &StepValidationError{
		Field:   "src",
		Message: "",
	}
	msg := err.Error()

	// Should still produce a valid error message
	if msg == "" {
		t.Error("error message should not be empty")
	}

	if !strings.Contains(msg, "src") {
		t.Errorf("unexpected message: %v", msg)
	}
}

// Test empty path
func TestFileOperationError_EmptyPath(t *testing.T) {
	err := &FileOperationError{
		Operation: "write",
		Path:      "",
		Cause:     errors.New("some error"),
	}
	msg := err.Error()

	// Should still produce a valid error message
	if msg == "" {
		t.Error("error message should not be empty")
	}

	if !strings.Contains(msg, "failed to write") {
		t.Errorf("unexpected message: %v", msg)
	}
}

// Test operation values
func TestFileOperationError_StandardOperations(t *testing.T) {
	operations := []string{"create", "read", "write", "delete", "chmod", "chown", "link"}

	for _, op := range operations {
		err := &FileOperationError{
			Operation: op,
			Path:      "/test/path",
			Cause:     errors.New("test error"),
		}

		msg := err.Error()
		if !strings.Contains(msg, "failed to "+op) {
			t.Errorf("operation %q not in error message: %v", op, msg)
		}
	}
}

// Test error chain: FileOperationError wrapping CommandError
func TestFileOperationError_WrappingCommandError(t *testing.T) {
	// Create base error
	baseErr := errors.New("permission denied")

	// Simulate sudo command failure
	cmdErr := &CommandError{
		ExitCode: 1,
		Timeout:  false,
		Cause:    baseErr,
	}

	// Wrap in FileOperationError (as done in file operations with sudo)
	fileErr := &FileOperationError{
		Operation: "chmod",
		Path:      "/etc/test",
		Cause:     cmdErr,
	}

	// Test error message includes both contexts
	msg := fileErr.Error()
	if !strings.Contains(msg, "failed to chmod file /etc/test") {
		t.Errorf("message should contain file operation context, got: %v", msg)
	}
	if !strings.Contains(msg, "exit code 1") {
		t.Errorf("message should contain command error, got: %v", msg)
	}

	// Test error chain unwrapping
	var cmdError *CommandError
	if !errors.As(fileErr, &cmdError) {
		t.Error("should be able to unwrap to CommandError")
	}

	if cmdError.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", cmdError.ExitCode)
	}

	// Test deep unwrapping to base error
	if !errors.Is(fileErr, baseErr) {
		t.Error("should preserve full error chain to base error")
	}
}
