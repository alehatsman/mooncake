package executor

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
)

const (
	fileExistsMsg = "file exists"
)

// checkFileOwnership verifies file owner or group ID against expected value.
// Returns (expected, actual, error).
func checkFileOwnership(info os.FileInfo, expected, field, fieldName string, getID func(*syscall.Stat_t) uint32) (string, string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", &SetupError{
			Component: "file stat",
			Issue:     fmt.Sprintf("unable to get file %s information", fieldName),
		}
	}

	actualID := fmt.Sprintf("%d", getID(stat))

	// Compare by ID (could enhance to support username/groupname lookup)
	if actualID != expected {
		expectedMsg := fmt.Sprintf("%s %s", fieldName, expected)
		actualMsg := fmt.Sprintf("%s %s", fieldName, actualID)
		return expectedMsg, actualMsg, &AssertionError{
			Type:     "file",
			Expected: expectedMsg,
			Actual:   actualMsg,
			Details:  field,
		}
	}
	return fmt.Sprintf("%s %s", fieldName, expected), fmt.Sprintf("%s %s", fieldName, actualID), nil
}

// HandleAssert executes an assertion and verifies the result.
// Assertions always return changed: false and fail if verification doesn't pass.
func HandleAssert(step config.Step, ec *ExecutionContext) error {
	assert := step.Assert
	if assert == nil {
		return &StepValidationError{
			Field:   "assert",
			Message: "assert configuration is nil",
		}
	}

	// Create result object with start time
	result := NewResult()
	result.StartTime = time.Now()
	result.Changed = false // Assertions never report "changed"

	// Finalize timing when function returns
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Determine which assertion type to execute
	var err error
	var assertType string
	var expected, actual string

	if assert.Command != nil {
		assertType = "command"
		expected, actual, err = executeAssertCommand(assert.Command, ec)
	} else if assert.File != nil {
		assertType = "file"
		expected, actual, err = executeAssertFile(assert.File, ec)
	} else if assert.HTTP != nil {
		assertType = "http"
		expected, actual, err = executeAssertHTTP(assert.HTTP, ec)
	} else {
		return &StepValidationError{
			Field:   "assert",
			Message: "no assertion type specified (command, file, or http required)",
		}
	}

	// Handle dry-run mode
	if ec.DryRun {
		ec.HandleDryRun(func(dryRun *dryRunLogger) {
			dryRun.LogAssertCheck(assertType, expected)
			dryRun.LogRegister(step)
		})
		result.Stdout = fmt.Sprintf("would check: %s", expected)

		// Register result if requested
		if step.Register != "" {
			result.RegisterTo(ec.Variables, step.Register)
		}

		// Set result in context
		ec.CurrentResult = result
		return nil
	}

	// Check assertion result
	if err != nil {
		// Assertion failed
		result.Failed = true
		result.Stderr = err.Error()

		// Register result if requested
		if step.Register != "" {
			result.RegisterTo(ec.Variables, step.Register)
			ec.Logger.Debugf("  Registered result as: %s (failed=%v)", step.Register, result.Failed)
		}

		// Set result in context
		ec.CurrentResult = result

		// Emit failure event
		ec.EmitEvent(events.EventAssertFailed, events.AssertionData{
			Type:     assertType,
			Expected: expected,
			Actual:   actual,
			Failed:   true,
		})

		return err
	}

	// Assertion passed
	result.Stdout = fmt.Sprintf("assertion passed: %s", expected)

	// Register result if requested
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
		ec.Logger.Debugf("  Registered result as: %s (changed=%v)", step.Register, result.Changed)
	}

	// Set result in context
	ec.CurrentResult = result

	// Emit success event
	ec.EmitEvent(events.EventAssertPassed, events.AssertionData{
		Type:     assertType,
		Expected: expected,
		Actual:   actual,
		Failed:   false,
	})

	return nil
}

// executeAssertCommand runs a command and verifies the exit code.
// Returns (expected, actual, error).
func executeAssertCommand(assertCmd *config.AssertCommand, ec *ExecutionContext) (string, string, error) {
	expectedCode := assertCmd.ExitCode
	if expectedCode == 0 && assertCmd.ExitCode == 0 {
		// Default to 0 if not specified
		expectedCode = 0
	}

	ec.Logger.Debugf("Asserting command: %s (expected exit code: %d)", assertCmd.Cmd, expectedCode)

	// Render command with variables
	cmd, err := ec.Template.Render(assertCmd.Cmd, ec.Variables)
	if err != nil {
		return "", "", &RenderError{Field: "assert.command.cmd", Cause: err}
	}

	// Create command
	// #nosec G204 -- Command from user config is intentional functionality
	shellCmd := exec.Command("bash", "-c", cmd)

	// Capture output
	var stdout, stderr bytes.Buffer
	shellCmd.Stdout = &stdout
	shellCmd.Stderr = &stderr

	// Execute command
	err = shellCmd.Run()

	// Get exit code
	var actualCode int
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				actualCode = status.ExitStatus()
			} else {
				actualCode = 1
			}
		} else {
			// Command failed to start
			return fmt.Sprintf("exit code %d", expectedCode),
				"command failed to execute",
				&AssertionError{
					Type:     "command",
					Expected: fmt.Sprintf("exit code %d", expectedCode),
					Actual:   "command failed to execute",
					Details:  cmd,
					Cause:    err,
				}
		}
	} else {
		actualCode = 0
	}

	// Verify exit code
	expected := fmt.Sprintf("exit code %d", expectedCode)
	actual := fmt.Sprintf("exit code %d", actualCode)

	if actualCode != expectedCode {
		details := cmd
		if stderr.Len() > 0 {
			details += fmt.Sprintf(" (stderr: %s)", strings.TrimSpace(stderr.String()))
		}

		return expected, actual, &AssertionError{
			Type:     "command",
			Expected: expected,
			Actual:   actual,
			Details:  details,
		}
	}

	return expected, actual, nil
}

// executeAssertFile verifies file properties.
// Returns (expected, actual, error).
func executeAssertFile(assertFile *config.AssertFile, ec *ExecutionContext) (string, string, error) {
	// Render path with variables
	path, err := ec.Template.Render(assertFile.Path, ec.Variables)
	if err != nil {
		return "", "", &RenderError{Field: "assert.file.path", Cause: err}
	}

	ec.Logger.Debugf("Asserting file: %s", path)

	// Get file info
	info, err := os.Stat(path)
	fileExists := err == nil

	// Check existence assertion
	if assertFile.Exists != nil {
		expected := fmt.Sprintf("file exists: %t", *assertFile.Exists)
		actual := fmt.Sprintf("file exists: %t", fileExists)

		if *assertFile.Exists && !fileExists {
			return expected, actual, &AssertionError{
				Type:     "file",
				Expected: expected,
				Actual:   actual,
				Details:  path,
			}
		}
		if !*assertFile.Exists && fileExists {
			return expected, actual, &AssertionError{
				Type:     "file",
				Expected: expected,
				Actual:   actual,
				Details:  path,
			}
		}
		return expected, actual, nil
	}

	// For other checks, file must exist
	if !fileExists {
		return fileExistsMsg, "file does not exist", &AssertionError{
			Type:     "file",
			Expected: fileExistsMsg,
			Actual:   "file does not exist",
			Details:  path,
			Cause:    err,
		}
	}

	// Check content assertion
	if assertFile.Content != nil {
		expectedContent, err := ec.Template.Render(*assertFile.Content, ec.Variables)
		if err != nil {
			return "", "", &RenderError{Field: "assert.file.content", Cause: err}
		}

		// #nosec G304 -- File path from user config is intentional functionality
		actualContent, err := os.ReadFile(path)
		if err != nil {
			return "", "", &FileOperationError{Operation: "read", Path: path, Cause: err}
		}

		if string(actualContent) != expectedContent {
			expected := fmt.Sprintf("content matches (length: %d)", len(expectedContent))
			actual := fmt.Sprintf("content differs (length: %d)", len(actualContent))
			return expected, actual, &AssertionError{
				Type:     "file",
				Expected: expected,
				Actual:   actual,
				Details:  path,
			}
		}
		return "content matches", "content matches", nil
	}

	// Check contains assertion
	if assertFile.Contains != nil {
		expectedSubstr, err := ec.Template.Render(*assertFile.Contains, ec.Variables)
		if err != nil {
			return "", "", &RenderError{Field: "assert.file.contains", Cause: err}
		}

		// #nosec G304 -- File path from user config is intentional functionality
		actualContent, err := os.ReadFile(path)
		if err != nil {
			return "", "", &FileOperationError{Operation: "read", Path: path, Cause: err}
		}

		if !strings.Contains(string(actualContent), expectedSubstr) {
			expected := fmt.Sprintf("contains: %q", expectedSubstr)
			actual := "substring not found"
			return expected, actual, &AssertionError{
				Type:     "file",
				Expected: expected,
				Actual:   actual,
				Details:  path,
			}
		}
		return fmt.Sprintf("contains: %q", expectedSubstr), "substring found", nil
	}

	// Check mode assertion
	if assertFile.Mode != nil {
		expectedMode, err := ec.Template.Render(*assertFile.Mode, ec.Variables)
		if err != nil {
			return "", "", &RenderError{Field: "assert.file.mode", Cause: err}
		}

		// Parse expected mode
		expectedModeInt, err := strconv.ParseUint(expectedMode, 8, 32)
		if err != nil {
			return "", "", &StepValidationError{
				Field:   "assert.file.mode",
				Message: fmt.Sprintf("invalid mode format: %s", expectedMode),
			}
		}

		actualMode := info.Mode().Perm()
		if actualMode != os.FileMode(expectedModeInt) {
			expected := fmt.Sprintf("mode %s", expectedMode)
			actual := fmt.Sprintf("mode %04o", actualMode)
			return expected, actual, &AssertionError{
				Type:     "file",
				Expected: expected,
				Actual:   actual,
				Details:  path,
			}
		}
		return fmt.Sprintf("mode %s", expectedMode), fmt.Sprintf("mode %s", expectedMode), nil
	}

	// Check owner assertion
	if assertFile.Owner != nil {
		expectedOwner, err := ec.Template.Render(*assertFile.Owner, ec.Variables)
		if err != nil {
			return "", "", &RenderError{Field: "assert.file.owner", Cause: err}
		}

		return checkFileOwnership(info, expectedOwner, path, "owner", func(s *syscall.Stat_t) uint32 {
			return s.Uid
		})
	}

	// Check group assertion
	if assertFile.Group != nil {
		expectedGroup, err := ec.Template.Render(*assertFile.Group, ec.Variables)
		if err != nil {
			return "", "", &RenderError{Field: "assert.file.group", Cause: err}
		}

		return checkFileOwnership(info, expectedGroup, path, "group", func(s *syscall.Stat_t) uint32 {
			return s.Gid
		})
	}

	// No specific assertion, just checking existence
	return fileExistsMsg, fileExistsMsg, nil
}

// executeAssertHTTP makes an HTTP request and verifies the response.
// Returns (expected, actual, error).
func executeAssertHTTP(assertHTTP *config.AssertHTTP, ec *ExecutionContext) (string, string, error) {
	// Render URL with variables
	url, err := ec.Template.Render(assertHTTP.URL, ec.Variables)
	if err != nil {
		return "", "", &RenderError{Field: "assert.http.url", Cause: err}
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
			return "", "", &StepValidationError{
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
			return "", "", &RenderError{Field: "assert.http.body", Cause: bodyErr}
		}
		bodyReader = strings.NewReader(body)
	}

	// Create HTTP request
	req, reqErr := http.NewRequest(method, url, bodyReader)
	if reqErr != nil {
		return "", "", &SetupError{
			Component: "http request",
			Issue:     "failed to create request",
			Cause:     reqErr,
		}
	}

	// Add headers
	for key, value := range assertHTTP.Headers {
		renderedValue, headerErr := ec.Template.Render(value, ec.Variables)
		if headerErr != nil {
			return "", "", &RenderError{Field: fmt.Sprintf("assert.http.headers.%s", key), Cause: headerErr}
		}
		req.Header.Set(key, renderedValue)
	}

	// Make HTTP request
	resp, respErr := client.Do(req)
	if respErr != nil {
		expected := fmt.Sprintf("HTTP %d", expectedStatus)
		return expected, "request failed", &AssertionError{
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
		return expected, actual, &AssertionError{
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
			return "", "", &FileOperationError{Operation: "read", Path: "response body", Cause: readErr}
		}
	}

	// Check body contains
	if assertHTTP.Contains != nil {
		expectedSubstr, containsErr := ec.Template.Render(*assertHTTP.Contains, ec.Variables)
		if containsErr != nil {
			return "", "", &RenderError{Field: "assert.http.contains", Cause: containsErr}
		}

		if !strings.Contains(string(respBody), expectedSubstr) {
			expected := fmt.Sprintf("body contains: %q", expectedSubstr)
			actual := "substring not found"
			return expected, actual, &AssertionError{
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
			return "", "", &RenderError{Field: "assert.http.body_equals", Cause: bodyEqualsErr}
		}

		if string(respBody) != expectedBody {
			expected := fmt.Sprintf("body equals (length: %d)", len(expectedBody))
			actual := fmt.Sprintf("body differs (length: %d)", len(respBody))
			return expected, actual, &AssertionError{
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
	// For now, we'll leave it as a TODO
	if assertHTTP.JSONPath != nil {
		return "", "", &SetupError{
			Component: "jsonpath",
			Issue:     "JSONPath support not yet implemented",
		}
	}

	// Just status code check
	return fmt.Sprintf("HTTP %d", expectedStatus),
		fmt.Sprintf("HTTP %d", resp.StatusCode), nil
}
