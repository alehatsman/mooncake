package assert

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

// newMockExecutionContext creates a mock that can be cast to *executor.ExecutionContext
func newMockExecutionContext() *executor.ExecutionContext {
	tmpl := template.NewPongo2Renderer()
	tmpDir, _ := os.MkdirTemp("", "assert-test-*")
	return &executor.ExecutionContext{
		Variables:      make(map[string]interface{}),
		Template:       tmpl,
		Evaluator:      expression.NewExprEvaluator(),
		PathUtil:       pathutil.NewPathExpander(tmpl),
		Logger:         &testutil.MockLogger{Logs: []string{}},
		EventPublisher: &testutil.MockPublisher{Events: []events.Event{}},
		Redactor:       security.NewRedactor(),
		SudoPass:       "",
		CurrentStepID:  "step-1",
		Stats:          executor.NewExecutionStats(),
		CurrentDir:     tmpDir,
	}
}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "assert" {
		t.Errorf("Name = %v, want 'assert'", meta.Name)
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if meta.Category != actions.CategorySystem {
		t.Errorf("Category = %v, want %v", meta.Category, actions.CategorySystem)
	}
	if !meta.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
	}
}

func TestHandler_Validate(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "valid command assertion",
			step: &config.Step{
				Assert: &config.Assert{
					Command: &config.AssertCommand{
						Cmd:      "exit 0",
						ExitCode: 0,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid file assertion",
			step: &config.Step{
				Assert: &config.Assert{
					File: &config.AssertFile{
						Path: "/tmp/test",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid http assertion",
			step: &config.Step{
				Assert: &config.Assert{
					HTTP: &config.AssertHTTP{
						URL: "http://localhost",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil assert action",
			step: &config.Step{
				Assert: nil,
			},
			wantErr: true,
		},
		{
			name: "empty assert (no assertion type)",
			step: &config.Step{
				Assert: &config.Assert{},
			},
			wantErr: true,
		},
		{
			name: "multiple assertion types (command + file)",
			step: &config.Step{
				Assert: &config.Assert{
					Command: &config.AssertCommand{Cmd: "echo test"},
					File:    &config.AssertFile{Path: "/tmp/test"},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple assertion types (file + http)",
			step: &config.Step{
				Assert: &config.Assert{
					File: &config.AssertFile{Path: "/tmp/test"},
					HTTP: &config.AssertHTTP{URL: "http://localhost"},
				},
			},
			wantErr: true,
		},
		{
			name: "all three assertion types",
			step: &config.Step{
				Assert: &config.Assert{
					Command: &config.AssertCommand{Cmd: "echo test"},
					File:    &config.AssertFile{Path: "/tmp/test"},
					HTTP:    &config.AssertHTTP{URL: "http://localhost"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.Validate(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute_CommandAssertion_Success(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	tests := []struct {
		name     string
		cmd      string
		exitCode int
	}{
		{
			name:     "success with exit code 0",
			cmd:      "exit 0",
			exitCode: 0,
		},
		{
			name:     "success with exit code 1",
			cmd:      "exit 1",
			exitCode: 1,
		},
		{
			name:     "success with exit code 42",
			cmd:      "exit 42",
			exitCode: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					Command: &config.AssertCommand{
						Cmd:      tt.cmd,
						ExitCode: tt.exitCode,
					},
				},
			}

			result, err := h.Execute(ctx, step)
			if err != nil {
				t.Errorf("Execute() error = %v, want nil", err)
			}

			execResult := result.(*executor.Result)
			if execResult.Changed {
				t.Error("Result.Changed should be false for assertions")
			}

			// Check for passed event
			pub := ctx.EventPublisher.(*testutil.MockPublisher)
			if len(pub.Events) == 0 {
				t.Error("Expected EventAssertPassed to be emitted")
			} else {
				lastEvent := pub.Events[len(pub.Events)-1]
				if lastEvent.Type != events.EventAssertPassed {
					t.Errorf("Event type = %v, want %v", lastEvent.Type, events.EventAssertPassed)
				}
			}
		})
	}
}

func TestHandler_Execute_CommandAssertion_Failure(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	tests := []struct {
		name     string
		cmd      string
		exitCode int
	}{
		{
			name:     "expected 0, got 1",
			cmd:      "exit 1",
			exitCode: 0,
		},
		{
			name:     "expected 1, got 0",
			cmd:      "exit 0",
			exitCode: 1,
		},
		{
			name:     "expected 42, got 0",
			cmd:      "exit 0",
			exitCode: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					Command: &config.AssertCommand{
						Cmd:      tt.cmd,
						ExitCode: tt.exitCode,
					},
				},
			}

			result, err := h.Execute(ctx, step)
			if err == nil {
				t.Error("Execute() should return error for failed assertion")
			}

			// Check it's an AssertionError
			var assertErr *executor.AssertionError
			if !errors.As(err, &assertErr) {
				t.Errorf("Error should be AssertionError, got %T", err)
			}

			if result == nil {
				t.Fatal("Result should not be nil even on error")
			}

			execResult := result.(*executor.Result)
			if execResult.Changed {
				t.Error("Result.Changed should be false for assertions")
			}

			// Check for failed event
			pub := ctx.EventPublisher.(*testutil.MockPublisher)
			if len(pub.Events) == 0 {
				t.Error("Expected EventAssertFailed to be emitted")
			} else {
				lastEvent := pub.Events[len(pub.Events)-1]
				if lastEvent.Type != events.EventAssertFailed {
					t.Errorf("Event type = %v, want %v", lastEvent.Type, events.EventAssertFailed)
				}
			}
		})
	}
}

func TestHandler_Execute_CommandAssertion_NonexistentCommand(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	step := &config.Step{
		Assert: &config.Assert{
			Command: &config.AssertCommand{
				Cmd:      "nonexistent_command_12345",
				ExitCode: 0,
			},
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error for nonexistent command")
	}
}

func TestHandler_Execute_CommandAssertion_WithTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	ctx.Variables = map[string]interface{}{
		"expected_code": 5,
	}

	step := &config.Step{
		Assert: &config.Assert{
			Command: &config.AssertCommand{
				Cmd:      "exit {{ expected_code }}",
				ExitCode: 5,
			},
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false for assertions")
	}
}

func TestHandler_Execute_FileAssertion_Exists(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create a test file
	testFile := filepath.Join(ctx.CurrentDir, "testfile.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		exists  *bool
		wantErr bool
	}{
		{
			name:    "file exists - true",
			path:    testFile,
			exists:  boolPtr(true),
			wantErr: false,
		},
		{
			name:    "file exists - false (should fail)",
			path:    testFile,
			exists:  boolPtr(false),
			wantErr: true,
		},
		{
			name:    "file does not exist - false",
			path:    filepath.Join(ctx.CurrentDir, "nonexistent.txt"),
			exists:  boolPtr(false),
			wantErr: false,
		},
		{
			name:    "file does not exist - true (should fail)",
			path:    filepath.Join(ctx.CurrentDir, "nonexistent.txt"),
			exists:  boolPtr(true),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					File: &config.AssertFile{
						Path:   tt.path,
						Exists: tt.exists,
					},
				},
			}

			result, err := h.Execute(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if result != nil {
				execResult := result.(*executor.Result)
				if execResult.Changed {
					t.Error("Result.Changed should be false for assertions")
				}
			}
		})
	}
}

func TestHandler_Execute_FileAssertion_Contains(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create a test file with content
	testFile := filepath.Join(ctx.CurrentDir, "testfile.txt")
	content := "Hello World\nThis is a test file"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		contains string
		wantErr  bool
	}{
		{
			name:     "contains substring - found",
			contains: "Hello",
			wantErr:  false,
		},
		{
			name:     "contains substring - not found",
			contains: "Goodbye",
			wantErr:  true,
		},
		{
			name:     "contains full line",
			contains: "This is a test file",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					File: &config.AssertFile{
						Path:     testFile,
						Contains: &tt.contains,
					},
				},
			}

			result, err := h.Execute(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if result != nil {
				execResult := result.(*executor.Result)
				if execResult.Changed {
					t.Error("Result.Changed should be false for assertions")
				}
			}
		})
	}
}

func TestHandler_Execute_FileAssertion_Content(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create a test file with content
	testFile := filepath.Join(ctx.CurrentDir, "testfile.txt")
	content := "exact content"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		content  string
		wantErr  bool
	}{
		{
			name:    "exact content match",
			content: "exact content",
			wantErr: false,
		},
		{
			name:    "content mismatch",
			content: "different content",
			wantErr: true,
		},
		{
			name:    "partial content (should fail)",
			content: "exact",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					File: &config.AssertFile{
						Path:    testFile,
						Content: &tt.content,
					},
				},
			}

			result, err := h.Execute(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if result != nil {
				execResult := result.(*executor.Result)
				if execResult.Changed {
					t.Error("Result.Changed should be false for assertions")
				}
			}
		})
	}
}

func TestHandler_Execute_FileAssertion_Mode(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create a test file with specific mode
	testFile := filepath.Join(ctx.CurrentDir, "testfile.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{
			name:    "mode matches",
			mode:    "0644",
			wantErr: false,
		},
		{
			name:    "mode mismatch",
			mode:    "0755",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					File: &config.AssertFile{
						Path: testFile,
						Mode: &tt.mode,
					},
				},
			}

			result, err := h.Execute(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if result != nil {
				execResult := result.(*executor.Result)
				if execResult.Changed {
					t.Error("Result.Changed should be false for assertions")
				}
			}
		})
	}
}

func TestHandler_Execute_FileAssertion_InvalidMode(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	testFile := filepath.Join(ctx.CurrentDir, "testfile.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	invalidMode := "invalid"
	step := &config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path: testFile,
				Mode: &invalidMode,
			},
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error for invalid mode")
	}
}

func TestHandler_Execute_FileAssertion_Owner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Owner assertions not supported on Windows")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	testFile := filepath.Join(ctx.CurrentDir, "testfile.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	tests := []struct {
		name        string
		owner       string
		shouldPass  bool
		description string
	}{
		{
			name:        "correct owner by username",
			owner:       currentUser.Username,
			shouldPass:  true,
			description: "file owned by current user should pass",
		},
		{
			name:        "correct owner by UID",
			owner:       currentUser.Uid,
			shouldPass:  true,
			description: "file owned by current UID should pass",
		},
		{
			name:        "incorrect owner",
			owner:       "nonexistentuser99999",
			shouldPass:  false,
			description: "file with wrong owner should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					File: &config.AssertFile{
						Path:  testFile,
						Owner: &tt.owner,
					},
				},
			}

			_, err := h.Execute(ctx, step)

			if tt.shouldPass {
				if err != nil {
					t.Errorf("Expected assertion to pass but got error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Expected assertion to fail but it passed")
				}
				var assertErr *executor.AssertionError
				if !errors.As(err, &assertErr) {
					t.Errorf("Expected AssertionError, got: %T", err)
				}
			}
		})
	}
}

func TestHandler_Execute_FileAssertion_Group(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Group assertions not supported on Windows")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	testFile := filepath.Join(ctx.CurrentDir, "testfile.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Get current user to find their primary group
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	// Get primary group
	primaryGroup, err := user.LookupGroupId(currentUser.Gid)
	if err != nil {
		t.Fatalf("Failed to lookup primary group: %v", err)
	}

	tests := []struct {
		name        string
		group       string
		shouldPass  bool
		description string
	}{
		{
			name:        "correct group by name",
			group:       primaryGroup.Name,
			shouldPass:  true,
			description: "file with current user's group should pass",
		},
		{
			name:        "correct group by GID",
			group:       currentUser.Gid,
			shouldPass:  true,
			description: "file with current user's GID should pass",
		},
		{
			name:        "incorrect group",
			group:       "nonexistentgroup99999",
			shouldPass:  false,
			description: "file with wrong group should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					File: &config.AssertFile{
						Path:  testFile,
						Group: &tt.group,
					},
				},
			}

			_, err := h.Execute(ctx, step)

			if tt.shouldPass {
				if err != nil {
					t.Errorf("Expected assertion to pass but got error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Expected assertion to fail but it passed")
				}
				var assertErr *executor.AssertionError
				if !errors.As(err, &assertErr) {
					t.Errorf("Expected AssertionError, got: %T", err)
				}
			}
		})
	}
}

func TestHandler_Execute_FileAssertion_WithTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	testFile := filepath.Join(ctx.CurrentDir, "testfile.txt")
	content := "Hello World"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx.Variables = map[string]interface{}{
		"file_path":       testFile,
		"expected_string": "Hello",
	}

	contains := "{{ expected_string }}"
	step := &config.Step{
		Assert: &config.Assert{
			File: &config.AssertFile{
				Path:     "{{ file_path }}",
				Contains: &contains,
			},
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false for assertions")
	}
}

func TestHandler_Execute_HTTPAssertion_Success(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Hello from server")
	}))
	defer server.Close()

	tests := []struct {
		name   string
		status int
	}{
		{
			name:   "default status 200",
			status: 0, // will default to 200
		},
		{
			name:   "explicit status 200",
			status: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					HTTP: &config.AssertHTTP{
						URL:    server.URL,
						Status: tt.status,
					},
				},
			}

			result, err := h.Execute(ctx, step)
			if err != nil {
				t.Errorf("Execute() error = %v, want nil", err)
			}

			execResult := result.(*executor.Result)
			if execResult.Changed {
				t.Error("Result.Changed should be false for assertions")
			}
		})
	}
}

func TestHandler_Execute_HTTPAssertion_StatusMismatch(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	step := &config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:    server.URL,
				Status: 200,
			},
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error for status mismatch")
	}

	var assertErr *executor.AssertionError
	if !errors.As(err, &assertErr) {
		t.Errorf("Error should be AssertionError, got %T", err)
	}
}

func TestHandler_Execute_HTTPAssertion_Method(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create test server that checks method
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	tests := []struct {
		name    string
		method  string
		wantErr bool
	}{
		{
			name:    "POST method",
			method:  "POST",
			wantErr: false,
		},
		{
			name:    "GET method (should fail)",
			method:  "GET",
			wantErr: true,
		},
		{
			name:    "default method GET (should fail)",
			method:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					HTTP: &config.AssertHTTP{
						URL:    server.URL,
						Method: tt.method,
						Status: 200,
					},
				},
			}

			_, err := h.Execute(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute_HTTPAssertion_Contains(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Hello World from server")
	}))
	defer server.Close()

	tests := []struct {
		name     string
		contains string
		wantErr  bool
	}{
		{
			name:     "body contains substring",
			contains: "Hello World",
			wantErr:  false,
		},
		{
			name:     "body does not contain substring",
			contains: "Goodbye",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					HTTP: &config.AssertHTTP{
						URL:      server.URL,
						Contains: &tt.contains,
					},
				},
			}

			_, err := h.Execute(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute_HTTPAssertion_BodyEquals(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	responseBody := "exact response"

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, responseBody)
	}))
	defer server.Close()

	tests := []struct {
		name       string
		bodyEquals string
		wantErr    bool
	}{
		{
			name:       "exact body match",
			bodyEquals: "exact response",
			wantErr:    false,
		},
		{
			name:       "body mismatch",
			bodyEquals: "different response",
			wantErr:    true,
		},
		{
			name:       "partial body (should fail)",
			bodyEquals: "exact",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					HTTP: &config.AssertHTTP{
						URL:        server.URL,
						BodyEquals: &tt.bodyEquals,
					},
				},
			}

			_, err := h.Execute(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute_HTTPAssertion_Headers(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create test server that checks headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") == "test-value" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	tests := []struct {
		name    string
		headers map[string]string
		wantErr bool
	}{
		{
			name: "correct header",
			headers: map[string]string{
				"X-Custom-Header": "test-value",
			},
			wantErr: false,
		},
		{
			name: "wrong header value",
			headers: map[string]string{
				"X-Custom-Header": "wrong-value",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					HTTP: &config.AssertHTTP{
						URL:     server.URL,
						Headers: tt.headers,
						Status:  200,
					},
				},
			}

			_, err := h.Execute(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute_HTTPAssertion_WithBody(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create test server that echoes body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body := make([]byte, 1024)
			n, _ := r.Body.Read(body)
			w.WriteHeader(http.StatusOK)
			w.Write(body[:n])
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	requestBody := "test request body"
	step := &config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:        server.URL,
				Method:     "POST",
				Body:       &requestBody,
				BodyEquals: &requestBody,
			},
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false for assertions")
	}
}

func TestHandler_Execute_HTTPAssertion_Timeout(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	// Create test server that sleeps
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server responds normally
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	step := &config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:     server.URL,
				Timeout: "5s",
			},
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false for assertions")
	}
}

func TestHandler_Execute_HTTPAssertion_InvalidTimeout(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	step := &config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:     server.URL,
				Timeout: "invalid",
			},
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error for invalid timeout")
	}
}

func TestHandler_Execute_HTTPAssertion_RequestFailure(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	step := &config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL: "http://localhost:99999", // Invalid port
			},
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error for request failure")
	}
}

func TestHandler_Execute_HTTPAssertion_JSONPathNotImplemented(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"key": "value"}`)
	}))
	defer server.Close()

	jsonPath := "$.key"
	step := &config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:      server.URL,
				JSONPath: &jsonPath,
			},
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error for JSONPath assertion (not implemented)")
	}
}

func TestHandler_Execute_HTTPAssertion_WithTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Hello World")
	}))
	defer server.Close()

	ctx.Variables = map[string]interface{}{
		"server_url":      server.URL,
		"expected_text":   "Hello",
		"custom_header":   "Bearer token123",
	}

	contains := "{{ expected_text }}"
	step := &config.Step{
		Assert: &config.Assert{
			HTTP: &config.AssertHTTP{
				URL:      "{{ server_url }}",
				Contains: &contains,
				Headers: map[string]string{
					"Authorization": "{{ custom_header }}",
				},
			},
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false for assertions")
	}
}

func TestHandler_Execute_InvalidContext(t *testing.T) {
	h := &Handler{}
	// Use testutil.MockContext which doesn't cast to ExecutionContext
	ctx := testutil.NewMockContext()

	step := &config.Step{
		Assert: &config.Assert{
			Command: &config.AssertCommand{
				Cmd:      "exit 0",
				ExitCode: 0,
			},
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when context is not ExecutionContext")
	}
	if !strings.Contains(err.Error(), "invalid context") {
		t.Errorf("Error should mention invalid context, got: %v", err)
	}
}

func TestHandler_DryRun_Command(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	tests := []struct {
		name     string
		cmd      string
		exitCode int
	}{
		{
			name:     "simple command",
			cmd:      "exit 0",
			exitCode: 0,
		},
		{
			name:     "command with non-zero exit code",
			cmd:      "exit 5",
			exitCode: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					Command: &config.AssertCommand{
						Cmd:      tt.cmd,
						ExitCode: tt.exitCode,
					},
				},
			}

			err := h.DryRun(ctx, step)
			if err != nil {
				t.Errorf("DryRun() error = %v, want nil", err)
			}

			// Check that something was logged
			mockLog := ctx.Logger.(*testutil.MockLogger)
			if len(mockLog.Logs) == 0 {
				t.Error("DryRun() should log something")
			}
		})
	}
}

func TestHandler_DryRun_File(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	tests := []struct {
		name     string
		file     *config.AssertFile
	}{
		{
			name: "file exists",
			file: &config.AssertFile{
				Path:   "/tmp/test.txt",
				Exists: boolPtr(true),
			},
		},
		{
			name: "file contains",
			file: &config.AssertFile{
				Path:     "/tmp/test.txt",
				Contains: stringPtr("hello"),
			},
		},
		{
			name: "file content",
			file: &config.AssertFile{
				Path:    "/tmp/test.txt",
				Content: stringPtr("exact content"),
			},
		},
		{
			name: "file mode",
			file: &config.AssertFile{
				Path: "/tmp/test.txt",
				Mode: stringPtr("0644"),
			},
		},
		{
			name: "no specific check",
			file: &config.AssertFile{
				Path: "/tmp/test.txt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					File: tt.file,
				},
			}

			err := h.DryRun(ctx, step)
			if err != nil {
				t.Errorf("DryRun() error = %v, want nil", err)
			}

			// Check that something was logged
			mockLog := ctx.Logger.(*testutil.MockLogger)
			if len(mockLog.Logs) == 0 {
				t.Error("DryRun() should log something")
			}
		})
	}
}

func TestHandler_DryRun_HTTP(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	defer os.RemoveAll(ctx.CurrentDir)

	tests := []struct {
		name   string
		method string
		status int
	}{
		{
			name:   "GET request with default status",
			method: "",
			status: 0,
		},
		{
			name:   "POST request with custom status",
			method: "POST",
			status: 201,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Assert: &config.Assert{
					HTTP: &config.AssertHTTP{
						URL:    "http://localhost",
						Method: tt.method,
						Status: tt.status,
					},
				},
			}

			err := h.DryRun(ctx, step)
			if err != nil {
				t.Errorf("DryRun() error = %v, want nil", err)
			}

			// Check that something was logged
			mockLog := ctx.Logger.(*testutil.MockLogger)
			if len(mockLog.Logs) == 0 {
				t.Error("DryRun() should log something")
			}
		})
	}
}

func TestHandler_DryRun_InvalidContext(t *testing.T) {
	h := &Handler{}
	ctx := testutil.NewMockContext()

	step := &config.Step{
		Assert: &config.Assert{
			Command: &config.AssertCommand{
				Cmd: "exit 0",
			},
		},
	}

	err := h.DryRun(ctx, step)
	if err == nil {
		t.Error("DryRun() should error when context is not ExecutionContext")
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}
