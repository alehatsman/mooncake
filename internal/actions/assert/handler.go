// Package assert implements the assert action handler.
// Assertions verify conditions without changing system state.
package assert

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	msgFileDoesNotExist = "file does not exist"
	msgFileExists       = "file exists"
)

// Handler implements the assert action handler.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "assert",
		Description:        "Verify conditions without changing system state",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Verification only
		ImplementsCheck:    false,      // N/A - verification action
	}
}

// Validate validates the assert action configuration.
func (h *Handler) Validate(step *config.Step) error {
	if step.Assert == nil {
		return fmt.Errorf("assert action requires assert configuration")
	}

	assert := step.Assert
	actionCount := 0
	if assert.Command != nil {
		actionCount++
	}
	if assert.File != nil {
		actionCount++
	}
	if assert.HTTP != nil {
		actionCount++
	}

	if actionCount == 0 {
		return fmt.Errorf("assert requires one of: command, file, or http")
	}
	if actionCount > 1 {
		return fmt.Errorf("assert can only specify one of: command, file, or http")
	}

	return nil
}

// Execute executes the assert action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	assert := step.Assert

	var expected, actual string
	var err error

	// Determine assertion type and execute
	if assert.Command != nil {
		expected, actual, err = h.executeAssertCommand(assert.Command, ec)
	} else if assert.File != nil {
		expected, actual, err = h.executeAssertFile(assert.File, ec)
	} else if assert.HTTP != nil {
		expected, actual, err = h.executeAssertHTTP(assert.HTTP, ec)
	}

	// Create result
	result := executor.NewResult()
	result.Changed = false // Assertions never change state

	if err != nil {
		// Emit failure event
		assertionErr, isAssertion := err.(*executor.AssertionError)
		failureData := events.AssertionData{
			Expected: expected,
			Actual:   actual,
			Failed:   true,
		}
		if isAssertion {
			failureData.Type = assertionErr.Type
		}
		ec.EmitEvent(events.EventAssertFailed, failureData)
		return result, err
	}

	// Emit success event
	ec.EmitEvent(events.EventAssertPassed, events.AssertionData{
		Expected: expected,
		Actual:   actual,
		Failed:   false,
	})

	return result, nil
}

// DryRun logs what the assertion would check.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	assert := step.Assert

	if assert.Command != nil {
		cmd := assert.Command.Cmd
		exitCode := assert.Command.ExitCode
		if exitCode == 0 {
			exitCode = 0 // Default expected exit code
		}
		ec.Logger.Infof("  [DRY-RUN] Would assert command exit code: %s (expected: %d)", cmd, exitCode)
	} else if assert.File != nil {
		path := assert.File.Path
		if assert.File.Exists != nil {
			exists := *assert.File.Exists
			ec.Logger.Infof("  [DRY-RUN] Would assert file exists: %s (expected: %v)", path, exists)
		} else if assert.File.Contains != nil {
			ec.Logger.Infof("  [DRY-RUN] Would assert file contains: %s", path)
		} else if assert.File.Content != nil {
			ec.Logger.Infof("  [DRY-RUN] Would assert file content equals: %s", path)
		} else if assert.File.Mode != nil {
			ec.Logger.Infof("  [DRY-RUN] Would assert file mode: %s (expected: %s)", path, *assert.File.Mode)
		} else {
			ec.Logger.Infof("  [DRY-RUN] Would assert file: %s", path)
		}
	} else if assert.HTTP != nil {
		url := assert.HTTP.URL
		method := assert.HTTP.Method
		if method == "" {
			method = "GET"
		}
		status := assert.HTTP.Status
		if status == 0 {
			status = 200
		}
		ec.Logger.Infof("  [DRY-RUN] Would assert HTTP %s %s (expected status: %d)", method, url, status)
	}

	return nil
}

// executeAssertCommand executes a command assertion.
func (h *Handler) executeAssertCommand(assertCmd *config.AssertCommand, ec *executor.ExecutionContext) (string, string, error) {
	// Render command with variables
	cmd, err := ec.Template.Render(assertCmd.Cmd, ec.Variables)
	if err != nil {
		return "", "", &executor.RenderError{Field: "assert.command.cmd", Cause: err}
	}

	// Default expected exit code to 0
	expectedExitCode := assertCmd.ExitCode

	ec.Logger.Debugf("Asserting command exit code: %s (expected: %d)", cmd, expectedExitCode)

	// Execute command
	// #nosec G204 -- Command from user config is intentional functionality
	shellCmd := exec.Command("bash", "-c", cmd)
	shellCmd.Dir = ec.CurrentDir

	output, execErr := shellCmd.CombinedOutput()
	exitCode := 0
	if execErr != nil {
		if exitError, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Command failed to start
			return fmt.Sprintf("exit code %d", expectedExitCode),
				"command failed to execute",
				&executor.AssertionError{
					Type:     "command",
					Expected: fmt.Sprintf("exit code %d", expectedExitCode),
					Actual:   "command failed to execute",
					Details:  fmt.Sprintf("command: %s", cmd),
					Cause:    execErr,
				}
		}
	}

	// Check exit code matches
	if exitCode != expectedExitCode {
		expected := fmt.Sprintf("exit code %d", expectedExitCode)
		actual := fmt.Sprintf("exit code %d", exitCode)
		return expected, actual, &executor.AssertionError{
			Type:     "command",
			Expected: expected,
			Actual:   actual,
			Details:  fmt.Sprintf("command: %s\noutput: %s", cmd, strings.TrimSpace(string(output))),
		}
	}

	return fmt.Sprintf("exit code %d", expectedExitCode),
		fmt.Sprintf("exit code %d", exitCode), nil
}

// executeAssertFile executes a file assertion.
func (h *Handler) executeAssertFile(assertFile *config.AssertFile, ec *executor.ExecutionContext) (string, string, error) {
	// Render path with variables
	path, err := ec.Template.Render(assertFile.Path, ec.Variables)
	if err != nil {
		return "", "", &executor.RenderError{Field: "assert.file.path", Cause: err}
	}

	// Expand path (handle ~ and relative paths)
	expandedPath, expandErr := ec.PathUtil.ExpandPath(path, ec.CurrentDir, ec.Variables)
	if expandErr != nil {
		return "", "", &executor.FileOperationError{
			Operation: "expand path",
			Path:      path,
			Cause:     expandErr,
		}
	}

	// Check existence
	fileInfo, statErr := os.Stat(expandedPath)
	fileExists := statErr == nil

	// Assert exists/not exists
	if assertFile.Exists != nil {
		expectedExists := *assertFile.Exists
		if fileExists != expectedExists {
			if expectedExists {
				return msgFileExists, msgFileDoesNotExist, &executor.AssertionError{
					Type:     "file",
					Expected: msgFileExists,
					Actual:   msgFileDoesNotExist,
					Details:  path,
				}
			}
			return msgFileDoesNotExist, msgFileExists, &executor.AssertionError{
				Type:     "file",
				Expected: msgFileDoesNotExist,
				Actual:   msgFileExists,
				Details:  path,
			}
		}
		if expectedExists {
			return msgFileExists, msgFileExists, nil
		}
		return msgFileDoesNotExist, msgFileDoesNotExist, nil
	}

	// If file doesn't exist and we're checking content/mode/owner, fail
	if !fileExists {
		return "", "", &executor.FileOperationError{
			Operation: "stat",
			Path:      path,
			Cause:     statErr,
		}
	}

	// Assert content contains
	if assertFile.Contains != nil {
		// #nosec G304 -- File path from user config is intentional functionality
		content, readErr := os.ReadFile(expandedPath)
		if readErr != nil {
			return "", "", &executor.FileOperationError{
				Operation: "read",
				Path:      path,
				Cause:     readErr,
			}
		}

		expectedSubstr, renderErr := ec.Template.Render(*assertFile.Contains, ec.Variables)
		if renderErr != nil {
			return "", "", &executor.RenderError{Field: "assert.file.contains", Cause: renderErr}
		}

		if !strings.Contains(string(content), expectedSubstr) {
			expected := fmt.Sprintf("content contains: %q", expectedSubstr)
			actual := "substring not found"
			return expected, actual, &executor.AssertionError{
				Type:     "file",
				Expected: expected,
				Actual:   actual,
				Details:  path,
			}
		}
		return fmt.Sprintf("content contains: %q", expectedSubstr), "substring found", nil
	}

	// Assert content equals
	if assertFile.Content != nil {
		// #nosec G304 -- File path from user config is intentional functionality
		content, readErr := os.ReadFile(expandedPath)
		if readErr != nil {
			return "", "", &executor.FileOperationError{
				Operation: "read",
				Path:      path,
				Cause:     readErr,
			}
		}

		expectedContent, renderErr := ec.Template.Render(*assertFile.Content, ec.Variables)
		if renderErr != nil {
			return "", "", &executor.RenderError{Field: "assert.file.content", Cause: renderErr}
		}

		if string(content) != expectedContent {
			expected := fmt.Sprintf("content equals (length: %d)", len(expectedContent))
			actual := fmt.Sprintf("content differs (length: %d)", len(content))
			return expected, actual, &executor.AssertionError{
				Type:     "file",
				Expected: expected,
				Actual:   actual,
				Details:  path,
			}
		}
		return "content matches", "content matches", nil
	}

	// Assert file mode
	if assertFile.Mode != nil {
		expectedMode, modeErr := strconv.ParseUint(*assertFile.Mode, 8, 32)
		if modeErr != nil {
			return "", "", &executor.StepValidationError{
				Field:   "assert.file.mode",
				Message: fmt.Sprintf("invalid mode: %s", *assertFile.Mode),
			}
		}

		actualMode := fileInfo.Mode().Perm()
		if uint32(actualMode) != uint32(expectedMode) {
			expected := fmt.Sprintf("mode %04o", expectedMode)
			actual := fmt.Sprintf("mode %04o", actualMode)
			return expected, actual, &executor.AssertionError{
				Type:     "file",
				Expected: expected,
				Actual:   actual,
				Details:  path,
			}
		}
		return fmt.Sprintf("mode %04o", expectedMode), fmt.Sprintf("mode %04o", actualMode), nil
	}

	// Assert owner/group would require syscall.Stat_t - not implemented here
	if assertFile.Owner != nil {
		return "", "", &executor.SetupError{
			Component: "owner check",
			Issue:     "owner assertion not yet implemented",
		}
	}
	if assertFile.Group != nil {
		return "", "", &executor.SetupError{
			Component: "group check",
			Issue:     "group assertion not yet implemented",
		}
	}

	// No specific assertion, just check file exists
	return msgFileExists, msgFileExists, nil
}

// executeAssertHTTP executes an HTTP assertion.
func (h *Handler) executeAssertHTTP(assertHTTP *config.AssertHTTP, ec *executor.ExecutionContext) (string, string, error) {
	// Render URL with variables
	url, err := ec.Template.Render(assertHTTP.URL, ec.Variables)
	if err != nil {
		return "", "", &executor.RenderError{Field: "assert.http.url", Cause: err}
	}

	// Default method to GET
	method := assertHTTP.Method
	if method == "" {
		method = "GET"
	}

	// Default expected status to 200
	expectedStatus := assertHTTP.Status
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	ec.Logger.Debugf("Asserting HTTP %s %s (expected status: %d)", method, url, expectedStatus)

	// Create HTTP client with timeout
	client := &http.Client{}
	if assertHTTP.Timeout != "" {
		timeout, timeoutErr := time.ParseDuration(assertHTTP.Timeout)
		if timeoutErr != nil {
			return "", "", &executor.StepValidationError{
				Field:   "assert.http.timeout",
				Message: fmt.Sprintf("invalid timeout format: %s", assertHTTP.Timeout),
			}
		}
		client.Timeout = timeout
	} else {
		client.Timeout = 30 * time.Second // Default timeout
	}

	// Create request body if provided
	var bodyReader io.Reader
	if assertHTTP.Body != nil {
		body, bodyErr := ec.Template.Render(*assertHTTP.Body, ec.Variables)
		if bodyErr != nil {
			return "", "", &executor.RenderError{Field: "assert.http.body", Cause: bodyErr}
		}
		bodyReader = strings.NewReader(body)
	}

	// Create HTTP request
	req, reqErr := http.NewRequest(method, url, bodyReader)
	if reqErr != nil {
		return "", "", &executor.SetupError{
			Component: "http request",
			Issue:     "failed to create request",
			Cause:     reqErr,
		}
	}

	// Add headers
	for key, value := range assertHTTP.Headers {
		renderedValue, headerErr := ec.Template.Render(value, ec.Variables)
		if headerErr != nil {
			return "", "", &executor.RenderError{Field: fmt.Sprintf("assert.http.headers.%s", key), Cause: headerErr}
		}
		req.Header.Set(key, renderedValue)
	}

	// Make HTTP request
	resp, respErr := client.Do(req)
	if respErr != nil {
		expected := fmt.Sprintf("HTTP %d", expectedStatus)
		return expected, "request failed", &executor.AssertionError{
			Type:     "http",
			Expected: expected,
			Actual:   "request failed",
			Details:  url,
			Cause:    respErr,
		}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			ec.Logger.Debugf("failed to close response body: %v", closeErr)
		}
	}()

	// Check status code
	if resp.StatusCode != expectedStatus {
		expected := fmt.Sprintf("HTTP %d", expectedStatus)
		actual := fmt.Sprintf("HTTP %d", resp.StatusCode)
		return expected, actual, &executor.AssertionError{
			Type:     "http",
			Expected: expected,
			Actual:   actual,
			Details:  url,
		}
	}

	// Read response body if we need to check content
	var respBody []byte
	if assertHTTP.Contains != nil || assertHTTP.BodyEquals != nil || assertHTTP.JSONPath != nil {
		var readErr error
		respBody, readErr = io.ReadAll(resp.Body)
		if readErr != nil {
			return "", "", &executor.FileOperationError{Operation: "read", Path: "response body", Cause: readErr}
		}
	}

	// Check body contains
	if assertHTTP.Contains != nil {
		expectedSubstr, containsErr := ec.Template.Render(*assertHTTP.Contains, ec.Variables)
		if containsErr != nil {
			return "", "", &executor.RenderError{Field: "assert.http.contains", Cause: containsErr}
		}

		if !strings.Contains(string(respBody), expectedSubstr) {
			expected := fmt.Sprintf("body contains: %q", expectedSubstr)
			actual := "substring not found"
			return expected, actual, &executor.AssertionError{
				Type:     "http",
				Expected: expected,
				Actual:   actual,
				Details:  url,
			}
		}
		return fmt.Sprintf("HTTP %d, body contains: %q", expectedStatus, expectedSubstr),
			fmt.Sprintf("HTTP %d, substring found", resp.StatusCode), nil
	}

	// Check exact body match
	if assertHTTP.BodyEquals != nil {
		expectedBody, bodyEqualsErr := ec.Template.Render(*assertHTTP.BodyEquals, ec.Variables)
		if bodyEqualsErr != nil {
			return "", "", &executor.RenderError{Field: "assert.http.body_equals", Cause: bodyEqualsErr}
		}

		if string(respBody) != expectedBody {
			expected := fmt.Sprintf("body equals (length: %d)", len(expectedBody))
			actual := fmt.Sprintf("body differs (length: %d)", len(respBody))
			return expected, actual, &executor.AssertionError{
				Type:     "http",
				Expected: expected,
				Actual:   actual,
				Details:  url,
			}
		}
		return fmt.Sprintf("HTTP %d, body matches", expectedStatus),
			fmt.Sprintf("HTTP %d, body matches", resp.StatusCode), nil
	}

	// JSONPath checking would go here (requires JSONPath library)
	if assertHTTP.JSONPath != nil {
		return "", "", &executor.SetupError{
			Component: "jsonpath",
			Issue:     "JSONPath support not yet implemented",
		}
	}

	// Just status code check
	return fmt.Sprintf("HTTP %d", expectedStatus),
		fmt.Sprintf("HTTP %d", resp.StatusCode), nil
}
