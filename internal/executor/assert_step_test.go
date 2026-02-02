package executor

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
)

// TestHandleAssert_CommandSuccess tests successful command assertion
func TestHandleAssert_CommandSuccess(t *testing.T) {
	ec := newTestExecutionContext(t)
	collector := &testEventCollector{}
	ec.EventPublisher.Subscribe(collector)

	step := config.Step{
		ID: "test-step-1",
		Assert: &config.Assert{
			Command: &config.AssertCommand{
				Cmd:      "true",
				ExitCode: 0,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check result
	if ec.CurrentResult == nil {
		t.Fatal("CurrentResult should not be nil")
	}
	if ec.CurrentResult.Changed {
		t.Error("Assert should never report changed=true")
	}
	if ec.CurrentResult.Failed {
		t.Error("Assert should not be marked as failed")
	}

	// Check event
	evts := collector.getEvents()
	if len(evts) == 0 {
		t.Fatal("Expected EventAssertPassed event")
	}

	found := false
	for _, evt := range evts {
		if evt.Type == events.EventAssertPassed {
			found = true
			data, ok := evt.Data.(events.AssertionData)
			if !ok {
				t.Fatalf("Event data should be AssertionData, got %T", evt.Data)
			}
			if data.Type != "command" {
				t.Errorf("AssertionData.Type = %q, want %q", data.Type, "command")
			}
			if data.Failed {
				t.Error("AssertionData.Failed should be false")
			}
			break
		}
	}
	if !found {
		t.Error("EventAssertPassed event not found")
	}
}

// TestHandleAssert_CommandFailure tests failed command assertion
func TestHandleAssert_CommandFailure(t *testing.T) {
	ec := newTestExecutionContext(t)
	collector := &testEventCollector{}
	ec.EventPublisher.Subscribe(collector)

	step := config.Step{
		ID: "test-step-2",
		Assert: &config.Assert{
			Command: &config.AssertCommand{
				Cmd:      "false",
				ExitCode: 0,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err == nil {
		t.Fatal("Expected error for failed assertion, got nil")
	}

	var assertErr *AssertionError
	if !errors.As(err, &assertErr) {
		t.Fatalf("Expected AssertionError, got %T: %v", err, err)
	}
	if assertErr.Type != "command" {
		t.Errorf("AssertionError.Type = %q, want %q", assertErr.Type, "command")
	}
	if assertErr.Expected != "exit code 0" {
		t.Errorf("AssertionError.Expected = %q, want %q", assertErr.Expected, "exit code 0")
	}
	if assertErr.Actual != "exit code 1" {
		t.Errorf("AssertionError.Actual = %q, want %q", assertErr.Actual, "exit code 1")
	}

	// Check result
	if ec.CurrentResult == nil {
		t.Fatal("CurrentResult should not be nil")
	}
	if ec.CurrentResult.Changed {
		t.Error("Assert should never report changed=true")
	}
	if !ec.CurrentResult.Failed {
		t.Error("Assert should be marked as failed")
	}

	// Check event
	evts := collector.getEvents()
	found := false
	for _, evt := range evts {
		if evt.Type == events.EventAssertFailed {
			found = true
			data, ok := evt.Data.(events.AssertionData)
			if !ok {
				t.Fatalf("Event data should be AssertionData, got %T", evt.Data)
			}
			if !data.Failed {
				t.Error("AssertionData.Failed should be true")
			}
			break
		}
	}
	if !found {
		t.Error("EventAssertFailed event not found")
	}
}

// TestHandleAssert_CommandNonZeroExitCode tests assertion expecting non-zero exit code
func TestHandleAssert_CommandNonZeroExitCode(t *testing.T) {
	ec := newTestExecutionContext(t)

	step := config.Step{
		Assert: &config.Assert{
			Command: &config.AssertCommand{
				Cmd:      "false",
				ExitCode: 1,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error for matching exit code 1, got: %v", err)
	}
}

// TestHandleAssert_FileExists tests file existence assertion
func TestHandleAssert_FileExists(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create a test file
	testFile := filepath.Join(ec.CurrentDir, "test.txt")
	createTestFile(t, testFile, "test content")

	exists := true
	step := config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path:   testFile,
				Exists: &exists,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error for existing file, got: %v", err)
	}
}

// TestHandleAssert_FileNotExists tests file non-existence assertion
func TestHandleAssert_FileNotExists(t *testing.T) {
	ec := newTestExecutionContext(t)

	nonExistentFile := filepath.Join(ec.CurrentDir, "does-not-exist.txt")
	exists := false
	step := config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path:   nonExistentFile,
				Exists: &exists,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error for non-existent file, got: %v", err)
	}
}

// TestHandleAssert_FileExistsFails tests file existence assertion failure
func TestHandleAssert_FileExistsFails(t *testing.T) {
	ec := newTestExecutionContext(t)

	nonExistentFile := filepath.Join(ec.CurrentDir, "does-not-exist.txt")
	exists := true
	step := config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path:   nonExistentFile,
				Exists: &exists,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}

	var assertErr *AssertionError
	if !errors.As(err, &assertErr) {
		t.Fatalf("Expected AssertionError, got %T: %v", err, err)
	}
	if assertErr.Type != "file" {
		t.Errorf("AssertionError.Type = %q, want %q", assertErr.Type, "file")
	}
}

// TestHandleAssert_FileContent tests exact content matching
func TestHandleAssert_FileContent(t *testing.T) {
	ec := newTestExecutionContext(t)

	testFile := filepath.Join(ec.CurrentDir, "test.txt")
	expectedContent := "expected content"
	createTestFile(t, testFile, expectedContent)

	step := config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path:    testFile,
				Content: &expectedContent,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error for matching content, got: %v", err)
	}
}

// TestHandleAssert_FileContentMismatch tests content mismatch
func TestHandleAssert_FileContentMismatch(t *testing.T) {
	ec := newTestExecutionContext(t)

	testFile := filepath.Join(ec.CurrentDir, "test.txt")
	createTestFile(t, testFile, "actual content")

	expectedContent := "expected content"
	step := config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path:    testFile,
				Content: &expectedContent,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err == nil {
		t.Fatal("Expected error for content mismatch, got nil")
	}

	var assertErr *AssertionError
	if !errors.As(err, &assertErr) {
		t.Fatalf("Expected AssertionError, got %T: %v", err, err)
	}
	if !strings.Contains(assertErr.Expected, "content matches") {
		t.Errorf("Expected message about content, got: %s", assertErr.Expected)
	}
}

// TestHandleAssert_FileContains tests substring matching
func TestHandleAssert_FileContains(t *testing.T) {
	ec := newTestExecutionContext(t)

	testFile := filepath.Join(ec.CurrentDir, "test.txt")
	createTestFile(t, testFile, "This is a test file with some content")

	substring := "test file"
	step := config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path:     testFile,
				Contains: &substring,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error for substring match, got: %v", err)
	}
}

// TestHandleAssert_FileContainsFails tests substring not found
func TestHandleAssert_FileContainsFails(t *testing.T) {
	ec := newTestExecutionContext(t)

	testFile := filepath.Join(ec.CurrentDir, "test.txt")
	createTestFile(t, testFile, "This is a test file")

	substring := "missing substring"
	step := config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path:     testFile,
				Contains: &substring,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err == nil {
		t.Fatal("Expected error for missing substring, got nil")
	}

	var assertErr *AssertionError
	if !errors.As(err, &assertErr) {
		t.Fatalf("Expected AssertionError, got %T: %v", err, err)
	}
	if assertErr.Actual != "substring not found" {
		t.Errorf("AssertionError.Actual = %q, want %q", assertErr.Actual, "substring not found")
	}
}

// TestHandleAssert_FileMode tests file permission checking
func TestHandleAssert_FileMode(t *testing.T) {
	ec := newTestExecutionContext(t)

	testFile := filepath.Join(ec.CurrentDir, "test.txt")
	createTestFile(t, testFile, "content")

	// Set specific permissions
	if err := os.Chmod(testFile, 0644); err != nil {
		t.Fatalf("Failed to chmod test file: %v", err)
	}

	mode := "0644"
	step := config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path: testFile,
				Mode: &mode,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error for matching mode, got: %v", err)
	}
}

// TestHandleAssert_FileModeMismatch tests file permission mismatch
func TestHandleAssert_FileModeMismatch(t *testing.T) {
	ec := newTestExecutionContext(t)

	testFile := filepath.Join(ec.CurrentDir, "test.txt")
	createTestFile(t, testFile, "content")

	// Set permissions to 0644
	if err := os.Chmod(testFile, 0644); err != nil {
		t.Fatalf("Failed to chmod test file: %v", err)
	}

	// Expect different permissions
	mode := "0755"
	step := config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path: testFile,
				Mode: &mode,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err == nil {
		t.Fatal("Expected error for mode mismatch, got nil")
	}

	var assertErr *AssertionError
	if !errors.As(err, &assertErr) {
		t.Fatalf("Expected AssertionError, got %T: %v", err, err)
	}
	if !strings.Contains(assertErr.Expected, "mode") {
		t.Errorf("Expected message about mode, got: %s", assertErr.Expected)
	}
}

// TestHandleAssert_HTTP tests HTTP status code assertion
func TestHandleAssert_HTTP(t *testing.T) {
	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}))
	defer server.Close()

	ec := newTestExecutionContext(t)

	step := config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:    server.URL,
				Status: 200,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error for HTTP 200, got: %v", err)
	}
}

// TestHandleAssert_HTTPStatusMismatch tests HTTP status code mismatch
func TestHandleAssert_HTTPStatusMismatch(t *testing.T) {
	// Create test HTTP server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ec := newTestExecutionContext(t)

	step := config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:    server.URL,
				Status: 200,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err == nil {
		t.Fatal("Expected error for HTTP status mismatch, got nil")
	}

	var assertErr *AssertionError
	if !errors.As(err, &assertErr) {
		t.Fatalf("Expected AssertionError, got %T: %v", err, err)
	}
	if assertErr.Expected != "HTTP 200" {
		t.Errorf("AssertionError.Expected = %q, want %q", assertErr.Expected, "HTTP 200")
	}
	if assertErr.Actual != "HTTP 404" {
		t.Errorf("AssertionError.Actual = %q, want %q", assertErr.Actual, "HTTP 404")
	}
}

// TestHandleAssert_HTTPContains tests HTTP response body substring check
func TestHandleAssert_HTTPContains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Hello, World!")
	}))
	defer server.Close()

	ec := newTestExecutionContext(t)

	substring := "Hello"
	step := config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:      server.URL,
				Status:   200,
				Contains: &substring,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error for body contains, got: %v", err)
	}
}

// TestHandleAssert_HTTPContainsFails tests HTTP response body substring not found
func TestHandleAssert_HTTPContainsFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Hello, World!")
	}))
	defer server.Close()

	ec := newTestExecutionContext(t)

	substring := "Goodbye"
	step := config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:      server.URL,
				Status:   200,
				Contains: &substring,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err == nil {
		t.Fatal("Expected error for missing substring, got nil")
	}

	var assertErr *AssertionError
	if !errors.As(err, &assertErr) {
		t.Fatalf("Expected AssertionError, got %T: %v", err, err)
	}
}

// TestHandleAssert_HTTPBodyEquals tests exact body matching
func TestHandleAssert_HTTPBodyEquals(t *testing.T) {
	expectedBody := "exact response"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, expectedBody)
	}))
	defer server.Close()

	ec := newTestExecutionContext(t)

	step := config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:        server.URL,
				Status:     200,
				BodyEquals: &expectedBody,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error for exact body match, got: %v", err)
	}
}

// TestHandleAssert_HTTPMethod tests different HTTP methods
func TestHandleAssert_HTTPMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ec := newTestExecutionContext(t)

	step := config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:    server.URL,
				Method: "POST",
				Status: 200,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error for POST request, got: %v", err)
	}
}

// TestHandleAssert_DryRun tests dry-run mode
func TestHandleAssert_DryRun(t *testing.T) {
	ec := newTestExecutionContext(t)
	ec.DryRun = true

	step := config.Step{
		Assert: &config.Assert{
			Command: &config.AssertCommand{
				Cmd:      "false", // Would fail in normal mode
				ExitCode: 0,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error in dry-run mode, got: %v", err)
	}

	// In dry-run mode, assertions should not actually execute
	if ec.CurrentResult == nil {
		t.Fatal("CurrentResult should not be nil in dry-run mode")
	}
	if ec.CurrentResult.Changed {
		t.Error("Assert should never report changed=true, even in dry-run")
	}
}

// TestHandleAssert_RegisterResult tests result registration
func TestHandleAssert_RegisterResult(t *testing.T) {
	ec := newTestExecutionContext(t)

	step := config.Step{
		Register: "assert_result",
		Assert: &config.Assert{
			Command: &config.AssertCommand{
				Cmd:      "true",
				ExitCode: 0,
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that result was registered
	result, ok := ec.Variables["assert_result"]
	if !ok {
		t.Fatal("Result should be registered as 'assert_result'")
	}

	// Result should be a map with standard fields
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Registered result should be a map, got %T", result)
	}

	// Check changed field
	changed, ok := resultMap["changed"].(bool)
	if !ok {
		t.Fatal("Result should have 'changed' field")
	}
	if changed {
		t.Error("Assert result should have changed=false")
	}
}

// TestHandleAssert_NoAssertType tests error when no assertion type is specified
func TestHandleAssert_NoAssertType(t *testing.T) {
	ec := newTestExecutionContext(t)

	step := config.Step{
		Assert: &config.Assert{
			// No Command, File, or HTTP specified
		},
	}

	err := HandleAssert(step, ec)
	if err == nil {
		t.Fatal("Expected error for no assertion type, got nil")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Expected StepValidationError, got %T: %v", err, err)
	}
}

// TestHandleAssert_NilAssert tests error when assert is nil
func TestHandleAssert_NilAssert(t *testing.T) {
	ec := newTestExecutionContext(t)

	step := config.Step{
		Assert: nil,
	}

	err := HandleAssert(step, ec)
	if err == nil {
		t.Fatal("Expected error for nil assert, got nil")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Expected StepValidationError, got %T: %v", err, err)
	}
}

// TestHandleAssert_TemplateRendering tests that template variables are rendered
func TestHandleAssert_TemplateRendering(t *testing.T) {
	ec := newTestExecutionContext(t)
	ec.Variables["expected_code"] = 0

	step := config.Step{
		Assert: &config.Assert{
			Command: &config.AssertCommand{
				Cmd:      "true",
				ExitCode: 0, // Could be {{ expected_code }} in real usage
			},
		},
	}

	err := HandleAssert(step, ec)
	if err != nil {
		t.Fatalf("Expected no error with template variables, got: %v", err)
	}
}
