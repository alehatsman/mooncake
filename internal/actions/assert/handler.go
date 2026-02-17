// Package assert implements the assert action handler.
// Assertions verify conditions without changing system state.
package assert

import (
	"crypto/sha256"
	"encoding/hex"
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
	msgGitClean         = "clean working tree"
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
	if assert.FileSHA256 != nil {
		actionCount++
	}
	if assert.GitClean != nil {
		actionCount++
	}
	if assert.GitDiff != nil {
		actionCount++
	}

	if actionCount == 0 {
		return fmt.Errorf("assert requires one of: command, file, http, file_sha256, git_clean, or git_diff")
	}
	if actionCount > 1 {
		return fmt.Errorf("assert can only specify one of: command, file, http, file_sha256, git_clean, or git_diff")
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
	} else if assert.FileSHA256 != nil {
		expected, actual, err = h.executeAssertFileSHA256(assert.FileSHA256, ec)
	} else if assert.GitClean != nil {
		expected, actual, err = h.executeAssertGitClean(assert.GitClean, ec)
	} else if assert.GitDiff != nil {
		expected, actual, err = h.executeAssertGitDiff(assert.GitDiff, ec)
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
	} else if assert.FileSHA256 != nil {
		path := assert.FileSHA256.Path
		checksum := assert.FileSHA256.Checksum
		ec.Logger.Infof("  [DRY-RUN] Would assert file SHA256: %s (expected: %s)", path, checksum)
	} else if assert.GitClean != nil {
		if assert.GitClean.AllowUntracked {
			ec.Logger.Infof("  [DRY-RUN] Would assert git working tree is clean (untracked files allowed)")
		} else {
			ec.Logger.Infof("  [DRY-RUN] Would assert git working tree is clean (no untracked files)")
		}
	} else if assert.GitDiff != nil {
		if assert.GitDiff.Cached {
			ec.Logger.Infof("  [DRY-RUN] Would assert git diff matches expected (cached/staged changes)")
		} else {
			ec.Logger.Infof("  [DRY-RUN] Would assert git diff matches expected (working tree)")
		}
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

	// Check owner if specified
	if assertFile.Owner != nil {
		expected, actual, err := checkFileOwner(fileInfo, *assertFile.Owner)
		if err != nil {
			return expected, actual, &executor.AssertionError{
				Type:     "file.owner",
				Expected: expected,
				Actual:   actual,
				Details:  path,
			}
		}
		return fmt.Sprintf("owner: %s", expected), fmt.Sprintf("owner: %s", actual), nil
	}

	// Check group if specified
	if assertFile.Group != nil {
		expected, actual, err := checkFileGroup(fileInfo, *assertFile.Group)
		if err != nil {
			return expected, actual, &executor.AssertionError{
				Type:     "file.group",
				Expected: expected,
				Actual:   actual,
				Details:  path,
			}
		}
		return fmt.Sprintf("group: %s", expected), fmt.Sprintf("group: %s", actual), nil
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

// executeAssertFileSHA256 verifies a file's SHA256 checksum.
func (h *Handler) executeAssertFileSHA256(assertSHA *config.AssertFileSHA256, ec *executor.ExecutionContext) (string, string, error) {
	// Render path with variables
	path, err := ec.Template.Render(assertSHA.Path, ec.Variables)
	if err != nil {
		return "", "", &executor.RenderError{Field: "assert.file_sha256.path", Cause: err}
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

	// Render expected checksum with variables
	expectedChecksum, err := ec.Template.Render(assertSHA.Checksum, ec.Variables)
	if err != nil {
		return "", "", &executor.RenderError{Field: "assert.file_sha256.checksum", Cause: err}
	}

	// Normalize checksum (remove "sha256:" prefix if present)
	expectedChecksum = strings.TrimPrefix(strings.ToLower(expectedChecksum), "sha256:")

	ec.Logger.Debugf("Asserting file SHA256: %s (expected: %s)", path, expectedChecksum)

	// Read file content
	content, readErr := os.ReadFile(expandedPath) // #nosec G304 -- Path from user config is intentional
	if readErr != nil {
		return "", "", &executor.FileOperationError{
			Operation: "read file",
			Path:      path,
			Cause:     readErr,
		}
	}

	// Calculate SHA256 checksum
	hash := sha256.Sum256(content)
	actualChecksum := hex.EncodeToString(hash[:])

	// Compare checksums
	if actualChecksum != expectedChecksum {
		expected := fmt.Sprintf("sha256:%s", expectedChecksum)
		actual := fmt.Sprintf("sha256:%s", actualChecksum)
		return expected, actual, &executor.AssertionError{
			Type:     "file_sha256",
			Expected: expected,
			Actual:   actual,
			Details:  fmt.Sprintf("file: %s", path),
		}
	}

	expected := fmt.Sprintf("sha256:%s", expectedChecksum)
	actual := fmt.Sprintf("sha256:%s", actualChecksum)
	return expected, actual, nil
}

// executeAssertGitClean verifies the git working tree is clean.
func (h *Handler) executeAssertGitClean(assertGit *config.AssertGitClean, ec *executor.ExecutionContext) (string, string, error) {
	ec.Logger.Debugf("Asserting git working tree is clean (allow_untracked: %v)", assertGit.AllowUntracked)

	// Check if we're in a git repository
	// #nosec G204 -- Git command is controlled and safe
	checkCmd := exec.Command("git", "rev-parse", "--git-dir")
	checkCmd.Dir = ec.CurrentDir
	if err := checkCmd.Run(); err != nil {
		return "", "", &executor.AssertionError{
			Type:     "git_clean",
			Expected: "git repository",
			Actual:   "not a git repository",
			Details:  ec.CurrentDir,
			Cause:    err,
		}
	}

	// Get git status
	// #nosec G204 -- Git command is controlled and safe
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = ec.CurrentDir
	output, err := statusCmd.CombinedOutput()
	if err != nil {
		return "", "", &executor.AssertionError{
			Type:     "git_clean",
			Expected: "git status success",
			Actual:   fmt.Sprintf("git status failed: %s", string(output)),
			Details:  ec.CurrentDir,
			Cause:    err,
		}
	}

	statusOutput := strings.TrimSpace(string(output))

	// If allow_untracked is true, filter out untracked files
	if assertGit.AllowUntracked && statusOutput != "" {
		lines := strings.Split(statusOutput, "\n")
		var trackedChanges []string
		for _, line := range lines {
			if len(line) >= 2 && line[0:2] != "??" {
				trackedChanges = append(trackedChanges, line)
			}
		}
		statusOutput = strings.Join(trackedChanges, "\n")
	}

	// Check if clean
	if statusOutput != "" {
		expected := msgGitClean
		actual := fmt.Sprintf("uncommitted changes:\n%s", statusOutput)
		return expected, actual, &executor.AssertionError{
			Type:     "git_clean",
			Expected: expected,
			Actual:   actual,
			Details:  ec.CurrentDir,
		}
	}

	return msgGitClean, msgGitClean, nil
}

// executeAssertGitDiff verifies the git diff matches expected output.
func (h *Handler) executeAssertGitDiff(assertDiff *config.AssertGitDiff, ec *executor.ExecutionContext) (string, string, error) {
	// Render expected diff with variables
	expectedDiff, err := ec.Template.Render(assertDiff.ExpectedDiff, ec.Variables)
	if err != nil {
		return "", "", &executor.RenderError{Field: "assert.git_diff.expected_diff", Cause: err}
	}

	// Normalize expected diff (trim whitespace)
	expectedDiff = strings.TrimSpace(expectedDiff)

	ec.Logger.Debugf("Asserting git diff matches expected (cached: %v)", assertDiff.Cached)

	// Check if we're in a git repository
	// #nosec G204 -- Git command is controlled and safe
	checkCmd := exec.Command("git", "rev-parse", "--git-dir")
	checkCmd.Dir = ec.CurrentDir
	if err := checkCmd.Run(); err != nil {
		return "", "", &executor.AssertionError{
			Type:     "git_diff",
			Expected: "git repository",
			Actual:   "not a git repository",
			Details:  ec.CurrentDir,
			Cause:    err,
		}
	}

	// Build git diff command
	var diffArgs []string
	if assertDiff.Cached {
		diffArgs = []string{"diff", "--cached"}
	} else {
		diffArgs = []string{"diff"}
	}

	// Add file filter if specified
	if assertDiff.Files != nil {
		files, renderErr := ec.Template.Render(*assertDiff.Files, ec.Variables)
		if renderErr != nil {
			return "", "", &executor.RenderError{Field: "assert.git_diff.files", Cause: renderErr}
		}
		diffArgs = append(diffArgs, "--", files)
	}

	// Execute git diff
	// #nosec G204 -- Git command with controlled arguments
	diffCmd := exec.Command("git", diffArgs...)
	diffCmd.Dir = ec.CurrentDir
	output, diffErr := diffCmd.CombinedOutput()

	// git diff returns exit code 0 whether there are differences or not
	// Only check for command execution errors
	if diffErr != nil {
		if exitError, ok := diffErr.(*exec.ExitError); ok {
			if exitError.ExitCode() != 0 {
				return "", "", &executor.AssertionError{
					Type:     "git_diff",
					Expected: "git diff success",
					Actual:   fmt.Sprintf("git diff failed (exit %d): %s", exitError.ExitCode(), string(output)),
					Details:  ec.CurrentDir,
					Cause:    diffErr,
				}
			}
		}
	}

	actualDiff := strings.TrimSpace(string(output))

	// Compare diffs
	if actualDiff != expectedDiff {
		return expectedDiff, actualDiff, &executor.AssertionError{
			Type:     "git_diff",
			Expected: expectedDiff,
			Actual:   actualDiff,
			Details:  ec.CurrentDir,
		}
	}

	return expectedDiff, actualDiff, nil
}
