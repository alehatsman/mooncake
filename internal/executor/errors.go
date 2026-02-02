package executor

import "fmt"

// Package executor provides custom error types for better error handling and categorization.
//
// Error Type Decision Tree:
// - Template rendering failed? → RenderError
// - Expression evaluation failed? → EvaluationError
// - Command execution failed? → CommandError
// - File system operation failed? → FileOperationError
// - Infrastructure/environment setup failed? → SetupError
// - Step parameter validation failed? → StepValidationError
//
// All error types support error unwrapping via errors.Is() and errors.As().

// RenderError represents a template rendering failure
type RenderError struct {
	Field string
	Cause error
}

func (e *RenderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("failed to render %s: %v", e.Field, e.Cause)
	}
	return fmt.Sprintf("failed to render %s", e.Field)
}

func (e *RenderError) Unwrap() error {
	return e.Cause
}

// EvaluationError represents an expression evaluation failure
type EvaluationError struct {
	Expression string
	Cause      error
}

func (e *EvaluationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("failed to evaluate %s: %v", e.Expression, e.Cause)
	}
	return fmt.Sprintf("failed to evaluate %s", e.Expression)
}

func (e *EvaluationError) Unwrap() error {
	return e.Cause
}

// CommandError represents a command execution failure
type CommandError struct {
	ExitCode int
	Timeout  bool
	Duration string
	Cause    error // Optional underlying error (e.g., exec.ExitError, OS errors)
}

func (e *CommandError) Error() string {
	var msg string
	if e.Timeout {
		msg = fmt.Sprintf("command timed out after %s", e.Duration)
	} else {
		msg = fmt.Sprintf("command execution failed with exit code %d", e.ExitCode)
	}

	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", msg, e.Cause)
	}
	return msg
}

func (e *CommandError) Unwrap() error {
	return e.Cause
}

// FileOperationError represents a file operation failure
type FileOperationError struct {
	Operation string // "create", "read", "write", "delete", "chmod", "chown", "link"
	Path      string
	Cause     error
}

func (e *FileOperationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("failed to %s file %s: %v", e.Operation, e.Path, e.Cause)
	}
	return fmt.Sprintf("failed to %s file %s", e.Operation, e.Path)
}

func (e *FileOperationError) Unwrap() error {
	return e.Cause
}

// StepValidationError represents step parameter validation failure during execution
type StepValidationError struct {
	Field   string
	Message string
}

func (e *StepValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// SetupError represents infrastructure or configuration setup failures
type SetupError struct {
	Component string // "become", "timeout", "sudo", "user", "group"
	Issue     string // What went wrong
	Cause     error  // Underlying error (optional)
}

func (e *SetupError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s setup failed: %s: %v", e.Component, e.Issue, e.Cause)
	}
	return fmt.Sprintf("%s setup failed: %s", e.Component, e.Issue)
}

func (e *SetupError) Unwrap() error {
	return e.Cause
}
