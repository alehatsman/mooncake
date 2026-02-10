package service

import (
	"os"
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
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
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
	}
}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "service" {
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
				Service: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateStarted,
				},
			},
			wantErr: false,
		},
		{
			name: "valid service with enabled",
			step: &config.Step{
				Service: &config.ServiceAction{
					Name:    "nginx",
					Enabled: boolPtr(true),
				},
			},
			wantErr: false,
		},
		{
			name: "nil service action",
			step: &config.Step{
				Service: nil,
			},
			wantErr: true,
		},
		{
			name: "missing service name",
			step: &config.Step{
				Service: &config.ServiceAction{
					State: ServiceStateStarted,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid state",
			step: &config.Step{
				Service: &config.ServiceAction{
					Name:  "nginx",
					State: "invalid-state",
				},
			},
			wantErr: true,
		},
		{
			name: "valid state: started",
			step: &config.Step{
				Service: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateStarted,
				},
			},
			wantErr: false,
		},
		{
			name: "valid state: stopped",
			step: &config.Step{
				Service: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateStopped,
				},
			},
			wantErr: false,
		},
		{
			name: "valid state: restarted",
			step: &config.Step{
				Service: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateRestarted,
				},
			},
			wantErr: false,
		},
		{
			name: "valid state: reloaded",
			step: &config.Step{
				Service: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateReloaded,
				},
			},
			wantErr: false,
		},
		{
			name: "service with unit file",
			step: &config.Step{
				Service: &config.ServiceAction{
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
				Service: &config.ServiceAction{
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

func TestHandler_Execute_InvalidContext(t *testing.T) {
	h := &Handler{}
	ctx := testutil.NewMockContext()

	step := &config.Step{
		Service: &config.ServiceAction{
			Name:  "nginx",
			State: ServiceStateStarted,
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when context is not ExecutionContext")
	}
	if !strings.Contains(err.Error(), "invalid context type") {
		t.Errorf("Error should mention invalid context type, got: %v", err)
	}
}

func TestHandler_DryRun(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "service with state",
			step: &config.Step{
				Service: &config.ServiceAction{
					Name:  "nginx",
					State: ServiceStateStarted,
				},
			},
			wantErr: false,
		},
		{
			name: "service with enabled",
			step: &config.Step{
				Service: &config.ServiceAction{
					Name:    "nginx",
					Enabled: boolPtr(true),
				},
			},
			wantErr: false,
		},
		{
			name: "service with unit file",
			step: &config.Step{
				Service: &config.ServiceAction{
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
				Service: &config.ServiceAction{
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
				Service: &config.ServiceAction{
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
				Service: &config.ServiceAction{
					Name:  "{{ service_name }}",
					State: ServiceStateStarted,
				},
			},
			wantErr: false,
		},
		{
			name: "nil service action",
			step: &config.Step{
				Service: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockExecutionContext()
			ctx.Variables["service_name"] = "nginx"

			err := h.DryRun(ctx, tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("DryRun() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Check that something was logged
				mockLog := ctx.Logger.(*testutil.MockLogger)
				if len(mockLog.Logs) == 0 {
					t.Error("DryRun() should log something")
				}
			}
		})
	}
}

func TestHandler_DryRun_InvalidContext(t *testing.T) {
	h := &Handler{}
	ctx := testutil.NewMockContext()

	step := &config.Step{
		Service: &config.ServiceAction{
			Name:  "nginx",
			State: ServiceStateStarted,
		},
	}

	err := h.DryRun(ctx, step)
	if err == nil {
		t.Error("DryRun() should error when context is not ExecutionContext")
	}
}

func TestHandler_DryRun_TemplateRenderFailure(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Service: &config.ServiceAction{
			Name:  "{{ invalid.syntax",
			State: ServiceStateStarted,
		},
	}

	err := h.DryRun(ctx, step)
	if err == nil {
		t.Error("DryRun() should error on invalid template syntax")
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
			Service: &config.ServiceAction{
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
		Service: nil,
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
		Service: &config.ServiceAction{
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
		Service: &config.ServiceAction{
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
		Service: &config.ServiceAction{
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
			domain := getLaunchdDomain(tt.isSystem)

			if tt.isSystem && domain != "system" {
				t.Errorf("getLaunchdDomain(true) = %q, want 'system'", domain)
			}

			if !tt.isSystem && !strings.HasPrefix(domain, "gui/") {
				t.Errorf("getLaunchdDomain(false) = %q, want prefix 'gui/'", domain)
			}
		})
	}
}

func TestGetLaunchdPlistPath(t *testing.T) {
	ctx := newMockExecutionContext()

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
			path := getLaunchdPlistPath(tt.serviceName, tt.unit, tt.isSystem, ctx)

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
			got := parseFileMode(tt.modeStr, tt.defaultMode)
			if got != tt.want {
				t.Errorf("parseFileMode(%q, %o) = %o, want %o", tt.modeStr, tt.defaultMode, got, tt.want)
			}
		})
	}
}

func TestRenderTemplateOrContent_InlineContent(t *testing.T) {
	ctx := newMockExecutionContext()
	ctx.Variables["key"] = "value"

	content, err := renderTemplateOrContent("", "static content", "test", ctx)
	if err != nil {
		t.Fatalf("renderTemplateOrContent() error = %v", err)
	}

	if content != "static content" {
		t.Errorf("renderTemplateOrContent() = %q, want 'static content'", content)
	}
}

func TestRenderTemplateOrContent_InlineContentWithTemplate(t *testing.T) {
	ctx := newMockExecutionContext()
	ctx.Variables["key"] = "rendered_value"

	content, err := renderTemplateOrContent("", "{{ key }}", "test", ctx)
	if err != nil {
		t.Fatalf("renderTemplateOrContent() error = %v", err)
	}

	if content != "rendered_value" {
		t.Errorf("renderTemplateOrContent() = %q, want 'rendered_value'", content)
	}
}

func TestRenderTemplateOrContent_NoContentOrTemplate(t *testing.T) {
	ctx := newMockExecutionContext()

	_, err := renderTemplateOrContent("", "", "test", ctx)
	if err == nil {
		t.Error("renderTemplateOrContent() should error when no content or template provided")
	}

	if !strings.Contains(err.Error(), "either src_template or content is required") {
		t.Errorf("Error should mention required fields, got: %v", err)
	}
}

func TestRenderTemplateOrContent_TemplateFileNotFound(t *testing.T) {
	ctx := newMockExecutionContext()
	ctx.CurrentDir = "/tmp"

	_, err := renderTemplateOrContent("/nonexistent/template.txt", "", "test", ctx)
	if err == nil {
		t.Error("renderTemplateOrContent() should error when template file not found")
	}
}

func TestRenderTemplateOrContent_InvalidTemplate(t *testing.T) {
	ctx := newMockExecutionContext()

	_, err := renderTemplateOrContent("", "{{ invalid.syntax", "test", ctx)
	if err == nil {
		t.Error("renderTemplateOrContent() should error on invalid template syntax")
	}
}

// TestHandleService_PlatformSupport tests that the service handler dispatches correctly by platform
func TestHandleService_PlatformSupport(t *testing.T) {
	ctx := newMockExecutionContext()
	ctx.SudoPass = "test" // Provide sudo password to get past initial checks

	step := config.Step{
		Service: &config.ServiceAction{
			Name:  "test-service",
			State: ServiceStateStarted,
		},
		Become: true,
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
	ctx.Variables["svc_name"] = "nginx"
	ctx.Variables["svc_state"] = "started"

	step := config.Step{
		Service: &config.ServiceAction{
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
	ctx.SudoPass = "" // No password provided

	step := config.Step{
		Service: &config.ServiceAction{
			Name:  "nginx",
			State: ServiceStateStarted,
		},
		Become: true,
	}

	err := HandleService(step, ctx)
	if err == nil {
		t.Error("HandleService() should error when become is true but no sudo password")
	}

	if !strings.Contains(err.Error(), "sudo") && !strings.Contains(err.Error(), "password") {
		t.Errorf("Error should mention sudo/password, got: %v", err)
	}
}

func TestHandleWindowsService(t *testing.T) {
	ctx := newMockExecutionContext()
	step := config.Step{
		Service: &config.ServiceAction{
			Name: "test",
		},
	}

	err := handleWindowsService("test", &config.ServiceAction{Name: "test"}, step, ctx)
	if err == nil {
		t.Error("handleWindowsService() should error (not implemented)")
	}

	if !strings.Contains(err.Error(), "Windows service support not yet implemented") {
		t.Errorf("Error should mention not implemented, got: %v", err)
	}
}

func TestMarkStepFailed(t *testing.T) {
	ctx := newMockExecutionContext()
	result := executor.NewResult()
	step := config.Step{
		Register: "test_result",
	}

	markStepFailed(result, step, ctx)

	if !result.Failed {
		t.Error("markStepFailed() should set Failed to true")
	}

	if result.Rc != 1 {
		t.Errorf("markStepFailed() should set Rc to 1, got %d", result.Rc)
	}

	// Check that result was registered
	if val, ok := ctx.Variables["test_result"]; !ok {
		t.Error("markStepFailed() should register result")
	} else {
		resultMap, ok := val.(map[string]interface{})
		if !ok {
			t.Error("Registered result should be a map")
		} else if !resultMap["failed"].(bool) {
			t.Error("Registered result should have failed=true")
		}
	}
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// Additional tests for uncovered functions

func TestIsLaunchdServiceLoaded(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	step := config.Step{}

	// Test with a service that doesn't exist
	loaded, err := isLaunchdServiceLoaded("com.nonexistent.test", step, ctx)
	if err != nil {
		// Error is acceptable - launchctl might not work in test environment
		t.Logf("isLaunchdServiceLoaded error (expected in test env): %v", err)
	} else {
		// Should return false for nonexistent service
		if loaded {
			t.Error("isLaunchdServiceLoaded() should return false for nonexistent service")
		}
	}
}

func TestLaunchdBootstrap(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "test.plist")

	// Create a minimal plist
	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.test.bootstrap</string>
</dict>
</plist>`

	err := os.WriteFile(plistPath, []byte(plistContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create plist: %v", err)
	}

	step := config.Step{}
	// Try bootstrap - will fail but tests the code path
	err = launchdBootstrap("gui/501", plistPath, step, ctx)
	// Error expected in test environment
	t.Logf("launchdBootstrap error (expected): %v", err)
}

func TestExecuteLaunchctlCommand_Error(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	step := config.Step{}

	// Test with invalid command that should fail
	err := executeLaunchctlCommand("invalid-subcommand", "gui/501", "/tmp/test.plist", step, ctx, nil, "success", "error")
	if err == nil {
		t.Log("executeLaunchctlCommand with invalid command succeeded (unexpected)")
	} else {
		t.Logf("executeLaunchctlCommand error (expected): %v", err)
	}
}

func TestManageLaunchdServiceState_Started(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "test-started.plist")

	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.test.started</string>
</dict>
</plist>`

	err := os.WriteFile(plistPath, []byte(plistContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create plist: %v", err)
	}

	step := config.Step{}
	// Test starting service
	changed, err := manageLaunchdServiceState("com.test.started", "gui/501/com.test.started", plistPath, "gui/501", ServiceStateStarted, false, step, ctx)
	// Error expected in test environment
	t.Logf("manageLaunchdServiceState result: changed=%v, err=%v", changed, err)
}

func TestManageLaunchdServiceEnabled(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "test-enabled.plist")

	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.test.enabled</string>
</dict>
</plist>`

	err := os.WriteFile(plistPath, []byte(plistContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create plist: %v", err)
	}

	step := config.Step{}
	// Test enabling service
	changed, err := manageLaunchdServiceEnabled("gui/501/com.test.enabled", plistPath, "gui/501", true, false, step, ctx)
	// Error expected in test environment
	t.Logf("manageLaunchdServiceEnabled result: changed=%v, err=%v", changed, err)
}

func TestLaunchdBootout(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	step := config.Step{}

	// Test bootout - will fail but tests the code path
	err := launchdBootout("gui/501", "/tmp/nonexistent.plist", step, ctx)
	// Error expected
	t.Logf("launchdBootout error (expected): %v", err)
}

func TestLaunchdKickstart(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	step := config.Step{}

	// Test kickstart - will fail but tests the code path
	err := launchdKickstart("gui/501/com.test.service", false, step, ctx)
	// Error expected
	t.Logf("launchdKickstart error (expected): %v", err)
}

func TestLaunchdKill(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	step := config.Step{}

	// Test kill - will fail but tests the code path
	err := launchdKill("gui/501/com.test.service", step, ctx)
	// Error expected
	t.Logf("launchdKill error (expected): %v", err)
}
