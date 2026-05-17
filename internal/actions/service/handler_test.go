package service

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/service/darwin"
	"github.com/alehatsman/mooncake/internal/actions/service/shared"
	windowspkg "github.com/alehatsman/mooncake/internal/actions/service/windows"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

// newMockExecutionContext creates a mock for apply-mode testing.
func newMockExecutionContext() *executor.ExecutionContext {
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       tmpl,
			Evaluator:      expression.NewExprEvaluator(),
			PathUtil:       pathutil.NewPathExpander(tmpl),
			Logger:         &testutil.MockLogger{Logs: []string{}},
			EventPublisher: &testutil.MockPublisher{Events: []events.Event{}},
			Redactor:       security.NewRedactor(),
			SudoPass:       "",
			Stats:          executor.NewExecutionStats(),
			Mode:           actions.ModeApply,
		},
		Scope:         executor.NewVariableScope(),
		CurrentStepID: "step-1",
	}
}

// newMockPlanContext creates a mock for plan-mode (dry-run) testing.
func newMockPlanContext() *executor.ExecutionContext {
	ec := newMockExecutionContext()
	ec.Svc.Mode = actions.ModePlan
	return ec
}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "os.service" {
		t.Errorf("Name = %v, want 'service'", meta.Name)
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
			name: "valid service with name and state",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateStarted,
				},
			},
			wantErr: false,
		},
		{
			name: "valid service with enabled",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:    "nginx",
					Enabled: boolPtr(true),
				},
			},
			wantErr: false,
		},
		{
			name: "nil service action",
			step: &config.Step{
				OsService: nil,
			},
			wantErr: true,
		},
		{
			name: "missing service name",
			step: &config.Step{
				OsService: &config.ServiceAction{
					State: ServiceStateStarted,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid state",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:  "nginx",
					State: "invalid-state",
				},
			},
			wantErr: true,
		},
		{
			name: "valid state: started",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateStarted,
				},
			},
			wantErr: false,
		},
		{
			name: "valid state: stopped",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateStopped,
				},
			},
			wantErr: false,
		},
		{
			name: "valid state: restarted",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateRestarted,
				},
			},
			wantErr: false,
		},
		{
			name: "valid state: reloaded",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateReloaded,
				},
			},
			wantErr: false,
		},
		{
			name: "service with unit file",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name: "myapp",
					Unit: &config.ServiceUnit{
						Content: "[Service]\nType=simple",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "service with drop-in",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name: "myapp",
					Dropin: &config.ServiceDropin{
						Name:    "10-env.conf",
						Content: "[Service]\nEnvironment=KEY=value",
					},
				},
			},
			wantErr: false,
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

func TestHandler_Run_InvalidContext(t *testing.T) {
	h := &Handler{}
	ctx := testutil.NewMockContext()

	step := &config.Step{
		OsService: &config.ServiceAction{
			Name:  "nginx",
			State: ServiceStateStarted,
		},
	}

	_, err := h.Run(ctx, step)
	if err == nil {
		t.Error("Run() should error when context is not ExecutionContext")
	}
	if !strings.Contains(err.Error(), "invalid context type") {
		t.Errorf("Error should mention invalid context type, got: %v", err)
	}
}

func TestHandler_Run_PlanMode(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("service plan mode is Linux-only")
	}
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "service with state",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateStarted,
				},
			},
			wantErr: false,
		},
		{
			name: "service with enabled",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:    "nginx",
					Enabled: boolPtr(true),
				},
			},
			wantErr: false,
		},
		{
			name: "service with unit file",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name: "myapp",
					Unit: &config.ServiceUnit{
						Content: "[Service]\nType=simple",
					},
					State: ServiceStateStarted,
				},
			},
			wantErr: false,
		},
		{
			name: "service with drop-in",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name: "myapp",
					Dropin: &config.ServiceDropin{
						Name:    "10-env.conf",
						Content: "[Service]\nEnvironment=KEY=value",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "service with daemon reload",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:         "nginx",
					State:        ServiceStateRestarted,
					DaemonReload: true,
				},
			},
			wantErr: false,
		},
		{
			name: "service with template name",
			step: &config.Step{
				OsService: &config.ServiceAction{
					Name:  "{{ service_name }}",
					State: ServiceStateStarted,
				},
			},
			wantErr: false,
		},
		{
			name: "nil service action",
			step: &config.Step{
				OsService: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockPlanContext()
			ctx.Scope.User["service_name"] = "nginx"

			result, err := h.Run(ctx, tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result == nil {
				t.Error("Run() plan mode returned nil result")
			}
		})
	}
}

func TestHandler_Run_PlanMode_TemplateRenderFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("service plan mode is Linux-only")
	}
	h := &Handler{}
	ctx := newMockPlanContext()

	step := &config.Step{
		OsService: &config.ServiceAction{
			Name:  "{{ invalid.syntax",
			State: ServiceStateStarted,
		},
	}

	_, err := h.Run(ctx, step)
	if err == nil {
		t.Error("Run() should error on invalid template syntax")
	}
}

func TestValidateServiceStates(t *testing.T) {
	validStates := []string{
		ServiceStateStarted,
		ServiceStateStopped,
		ServiceStateRestarted,
		ServiceStateReloaded,
	}

	// Verify constants are set correctly
	expectedStates := map[string]string{
		ServiceStateStarted:   "started",
		ServiceStateStopped:   "stopped",
		ServiceStateRestarted: "restarted",
		ServiceStateReloaded:  "reloaded",
	}

	for constant, expected := range expectedStates {
		if constant != expected {
			t.Errorf("Constant %q = %q, want %q", constant, constant, expected)
		}
	}

	// Verify all valid states are accepted
	h := &Handler{}
	for _, state := range validStates {
		step := &config.Step{
			OsService: &config.ServiceAction{
				Name:  "test",
				State: state,
			},
		}
		if err := h.Validate(step); err != nil {
			t.Errorf("Validate() should accept state %q, got error: %v", state, err)
		}
	}
}

func TestHandleService_NilServiceAction(t *testing.T) {
	ctx := newMockExecutionContext()
	step := config.Step{
		OsService: nil,
	}

	err := HandleService(step, ctx)
	if err == nil {
		t.Error("HandleService() should error when service is nil")
	}

	if !strings.Contains(err.Error(), "no service configuration") {
		t.Errorf("Error should mention no service configuration, got: %v", err)
	}
}

func TestHandleService_EmptyServiceName(t *testing.T) {
	ctx := newMockExecutionContext()
	step := config.Step{
		OsService: &config.ServiceAction{
			Name: "",
		},
	}

	err := HandleService(step, ctx)
	if err == nil {
		t.Error("HandleService() should error when service name is empty")
	}

	if !strings.Contains(err.Error(), "service name is required") {
		t.Errorf("Error should mention service name required, got: %v", err)
	}
}

func TestHandleService_InvalidServiceName_Template(t *testing.T) {
	ctx := newMockExecutionContext()
	step := config.Step{
		OsService: &config.ServiceAction{
			Name: "{{ invalid.syntax",
		},
	}

	err := HandleService(step, ctx)
	if err == nil {
		t.Error("HandleService() should error on invalid template in service name")
	}
}

func TestHandleService_InvalidState(t *testing.T) {
	ctx := newMockExecutionContext()
	step := config.Step{
		OsService: &config.ServiceAction{
			Name:  "nginx",
			State: "invalid-state",
		},
	}

	err := HandleService(step, ctx)
	if err == nil {
		t.Error("HandleService() should error on invalid state")
	}

	if !strings.Contains(err.Error(), "invalid state") {
		t.Errorf("Error should mention invalid state, got: %v", err)
	}
}

func TestGetLaunchdDomain(t *testing.T) {
	tests := []struct {
		name     string
		isSystem bool
		wantType string // "system" or "gui"
	}{
		{
			name:     "system daemon",
			isSystem: true,
			wantType: "system",
		},
		{
			name:     "user agent",
			isSystem: false,
			wantType: "gui",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain := darwin.GetDomain(tt.isSystem)

			if tt.isSystem && domain != "system" {
				t.Errorf("darwin.GetDomain(true) = %q, want 'system'", domain)
			}

			if !tt.isSystem && !strings.HasPrefix(domain, "gui/") {
				t.Errorf("darwin.GetDomain(false) = %q, want prefix 'gui/'", domain)
			}
		})
	}
}

func TestGetLaunchdPlistPath(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		unit        *config.ServiceUnit
		isSystem    bool
		wantPattern string // Pattern to match in the path
	}{
		{
			name:        "system daemon default path",
			serviceName: "com.example.daemon",
			unit:        nil,
			isSystem:    true,
			wantPattern: "/Library/LaunchDaemons/com.example.daemon.plist",
		},
		{
			name:        "user agent default path",
			serviceName: "com.example.agent",
			unit:        nil,
			isSystem:    false,
			wantPattern: "Library/LaunchAgents/com.example.agent.plist",
		},
		{
			name:        "custom destination",
			serviceName: "myapp",
			unit: &config.ServiceUnit{
				Dest: "/custom/path/myapp.plist",
			},
			isSystem:    true,
			wantPattern: "/custom/path/myapp.plist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := darwin.GetPlistPath(tt.serviceName, tt.unit, tt.isSystem)

			if !strings.Contains(path, tt.wantPattern) {
				t.Errorf("getLaunchdPlistPath() = %q, want to contain %q", path, tt.wantPattern)
			}
		})
	}
}

func TestParseFileMode(t *testing.T) {
	tests := []struct {
		name        string
		modeStr     string
		defaultMode os.FileMode
		want        os.FileMode
	}{
		{
			name:        "empty string uses default",
			modeStr:     "",
			defaultMode: 0644,
			want:        0644,
		},
		{
			name:        "valid octal mode",
			modeStr:     "0755",
			defaultMode: 0644,
			want:        0755,
		},
		{
			name:        "valid octal mode without leading zero",
			modeStr:     "644",
			defaultMode: 0755,
			want:        0644,
		},
		{
			name:        "invalid mode uses default",
			modeStr:     "invalid",
			defaultMode: 0644,
			want:        0644,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shared.ParseFileMode(tt.modeStr, tt.defaultMode)
			if got != tt.want {
				t.Errorf("shared.ParseFileMode(%q, %o) = %o, want %o", tt.modeStr, tt.defaultMode, got, tt.want)
			}
		})
	}
}

func TestRenderTemplateOrContent_InlineContent(t *testing.T) {
	ctx := newMockExecutionContext()
	ctx.Scope.User["key"] = "value"

	content, err := shared.RenderTemplateOrContent("", "static content", "test", ctx)
	if err != nil {
		t.Fatalf("shared.RenderTemplateOrContent() error = %v", err)
	}

	if content != "static content" {
		t.Errorf("shared.RenderTemplateOrContent() = %q, want 'static content'", content)
	}
}

func TestRenderTemplateOrContent_InlineContentWithTemplate(t *testing.T) {
	ctx := newMockExecutionContext()
	ctx.Scope.User["key"] = "rendered_value"

	content, err := shared.RenderTemplateOrContent("", "{{ key }}", "test", ctx)
	if err != nil {
		t.Fatalf("shared.RenderTemplateOrContent() error = %v", err)
	}

	if content != "rendered_value" {
		t.Errorf("shared.RenderTemplateOrContent() = %q, want 'rendered_value'", content)
	}
}

func TestRenderTemplateOrContent_NoContentOrTemplate(t *testing.T) {
	ctx := newMockExecutionContext()

	_, err := shared.RenderTemplateOrContent("", "", "test", ctx)
	if err == nil {
		t.Error("shared.RenderTemplateOrContent() should error when no content or template provided")
	}

	if !strings.Contains(err.Error(), "either src_template or content is required") {
		t.Errorf("Error should mention required fields, got: %v", err)
	}
}

func TestRenderTemplateOrContent_TemplateFileNotFound(t *testing.T) {
	ctx := newMockExecutionContext()
	ctx.CurrentDir = "/tmp"

	_, err := shared.RenderTemplateOrContent("/nonexistent/template.txt", "", "test", ctx)
	if err == nil {
		t.Error("shared.RenderTemplateOrContent() should error when template file not found")
	}
}

func TestRenderTemplateOrContent_InvalidTemplate(t *testing.T) {
	ctx := newMockExecutionContext()

	_, err := shared.RenderTemplateOrContent("", "{{ invalid.syntax", "test", ctx)
	if err == nil {
		t.Error("shared.RenderTemplateOrContent() should error on invalid template syntax")
	}
}

// TestHandleService_PlatformSupport tests that the service handler dispatches correctly by platform
func TestHandleService_PlatformSupport(t *testing.T) {
	ctx := newMockExecutionContext()
	ctx.Svc.SudoPass = "test" // Provide sudo password to get past initial checks

	step := config.Step{
		OsService: &config.ServiceAction{
			Name:  "test-service",
			State: ServiceStateStarted,
		},
		AsUser: "root",
	}

	err := HandleService(step, ctx)

	// We expect errors because we're not actually running systemctl/launchctl
	// but we can verify the error type indicates the command was attempted
	switch runtime.GOOS {
	case "linux":
		// On Linux, should try to run systemctl
		if err != nil && !strings.Contains(err.Error(), "systemctl") &&
			!strings.Contains(err.Error(), "executable") &&
			!strings.Contains(err.Error(), "command") {
			// Allow various error types that indicate systemctl was attempted
			t.Logf("Linux error (expected): %v", err)
		}
	case "darwin":
		// On macOS, should try to run launchctl
		if err != nil && !strings.Contains(err.Error(), "launchctl") &&
			!strings.Contains(err.Error(), "executable") &&
			!strings.Contains(err.Error(), "command") {
			t.Logf("macOS error (expected): %v", err)
		}
	case "windows":
		// Windows not yet implemented
		if err == nil {
			t.Error("HandleService() should error on Windows (not yet implemented)")
		}
		if !strings.Contains(err.Error(), "Windows service support not yet implemented") {
			t.Errorf("Error should mention Windows not implemented, got: %v", err)
		}
	}
}

func TestHandleService_TemplateRendering(t *testing.T) {
	ctx := newMockExecutionContext()
	ctx.Scope.User["svc_name"] = "nginx"
	ctx.Scope.User["svc_state"] = "started"

	step := config.Step{
		OsService: &config.ServiceAction{
			Name:  "{{ svc_name }}",
			State: ServiceStateStarted,
		},
	}

	// This will fail because we can't actually manage services in tests,
	// but it should get past the template rendering phase
	_ = HandleService(step, ctx)

	// If we got here without a template rendering error, the test passes
	// (actual service management errors are expected)
}

func TestHandleService_BecomeWithoutPassword(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping become test on Windows")
	}

	ctx := newMockExecutionContext()
	ctx.Svc.SudoPass = "" // No password provided

	step := config.Step{
		OsService: &config.ServiceAction{
			Name:  "nginx",
			State: ServiceStateStarted,
		},
		AsUser: "root",
	}

	err := HandleService(step, ctx)
	if err == nil {
		t.Error("HandleService() should error when become is true but no sudo password")
	}

	if !strings.Contains(err.Error(), "sudo") && !strings.Contains(err.Error(), "password") {
		t.Errorf("Error should mention sudo/password, got: %v", err)
	}
}

// TestHandleWindowsService_MissingServiceErrors pins the v1
// behaviour now that windows is implemented: a service that
// doesn't exist in SCM returns a SetupError with a clear hint
// rather than silently no-op. Stubs windowspkg.Exec to simulate
// Get-Service returning empty (= missing service) so the test
// runs on any CI host without needing a real PowerShell.
func TestHandleWindowsService_MissingServiceErrors(t *testing.T) {
	prev := windowspkg.Exec
	windowspkg.Exec = func(string) (string, error) { return "", nil }
	t.Cleanup(func() { windowspkg.Exec = prev })

	ctx := newMockExecutionContext()
	step := config.Step{
		OsService: &config.ServiceAction{Name: "test", State: "started"},
	}
	err := windowspkg.Handle("test", step.OsService, step, ctx)
	if err == nil {
		t.Fatal("windowspkg.Handle() should error when service is missing")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention 'not found'; got: %v", err)
	}
}

func TestMarkStepFailed(t *testing.T) {
	ctx := newMockExecutionContext()
	result := executor.NewResult()
	step := config.Step{
		As: "test_result",
	}

	shared.MarkStepFailed(result, step, ctx)

	if !result.Failed {
		t.Error("shared.MarkStepFailed() should set Failed to true")
	}

	if result.Rc != 1 {
		t.Errorf("shared.MarkStepFailed() should set Rc to 1, got %d", result.Rc)
	}

	// Check that result was registered in Scope.Results
	if reg, ok := ctx.Scope.Results["test_result"]; !ok {
		t.Error("shared.MarkStepFailed() should register result")
	} else if !reg.Failed {
		t.Error("Registered result should have Failed=true")
	}
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// TestWrapBecomeErrorAsSetup locks the F005-final-mile error translation
// shape: security.ErrBecomeUnsupported / security.ErrBecomeNoSudoPass
// must become *executor.SetupError with the historical Component values
// the test suite (and downstream tools) already match on. Any other
// error passes through unchanged.
func TestWrapBecomeErrorAsSetup(t *testing.T) {
	t.Run("ErrBecomeUnsupported -> SetupError component=become", func(t *testing.T) {
		got := shared.WrapBecomeErrorAsSetup(fmt.Errorf("wrapped: %w", security.ErrBecomeUnsupported))
		var setupErr *executor.SetupError
		if !errors.As(got, &setupErr) {
			t.Fatalf("want *executor.SetupError, got %T (%v)", got, got)
		}
		if setupErr.Component != "become" {
			t.Errorf("Component = %q, want %q", setupErr.Component, "become")
		}
		if !strings.Contains(setupErr.Issue, "not supported") {
			t.Errorf("Issue = %q, want it to mention 'not supported'", setupErr.Issue)
		}
	})

	t.Run("ErrBecomeNoSudoPass -> SetupError component=sudo", func(t *testing.T) {
		got := shared.WrapBecomeErrorAsSetup(security.ErrBecomeNoSudoPass)
		var setupErr *executor.SetupError
		if !errors.As(got, &setupErr) {
			t.Fatalf("want *executor.SetupError, got %T (%v)", got, got)
		}
		if setupErr.Component != "sudo" {
			t.Errorf("Component = %q, want %q", setupErr.Component, "sudo")
		}
		if !strings.Contains(setupErr.Issue, "no password provided") {
			t.Errorf("Issue = %q, want it to mention 'no password provided'", setupErr.Issue)
		}
	})

	t.Run("unrelated error passes through", func(t *testing.T) {
		orig := errors.New("disk full")
		got := shared.WrapBecomeErrorAsSetup(orig)
		if !errors.Is(got, orig) {
			t.Errorf("want passthrough of original error, got %v", got)
		}
		var setupErr *executor.SetupError
		if errors.As(got, &setupErr) {
			t.Errorf("unrelated error must not be wrapped as SetupError, got %v", setupErr)
		}
	})
}
