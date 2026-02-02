package executor

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestHandleService_MissingServiceAction verifies error when Service field is nil
func TestHandleService_MissingServiceAction(t *testing.T) {
	ec := newTestExecutionContext(t)
	step := config.Step{
		Service: nil,
	}

	err := HandleService(step, ec)
	if err == nil {
		t.Fatal("Expected error for nil Service action, got nil")
	}

	var setupErr *SetupError
	if !errors.As(err, &setupErr) {
		t.Fatalf("expected SetupError, got %T: %v", err, err)
	}
	if setupErr.Component != "service" {
		t.Errorf("Component = %q, want %q", setupErr.Component, "service")
	}
}

// TestHandleService_MissingName verifies error when service name is empty
func TestHandleService_MissingName(t *testing.T) {
	ec := newTestExecutionContext(t)
	step := config.Step{
		Service: &config.ServiceAction{
			Name: "",
		},
	}

	err := HandleService(step, ec)
	if err == nil {
		t.Fatal("Expected error for missing service name, got nil")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected StepValidationError, got %T: %v", err, err)
	}
	if validationErr.Field != "name" {
		t.Errorf("Field = %q, want %q", validationErr.Field, "name")
	}
}

// TestHandleService_InvalidState verifies error for invalid state values
func TestHandleService_InvalidState(t *testing.T) {
	ec := newTestExecutionContext(t)
	step := config.Step{
		Service: &config.ServiceAction{
			Name:  "nginx",
			State: "invalid_state",
		},
	}

	err := HandleService(step, ec)
	if err == nil {
		t.Fatal("Expected error for invalid state, got nil")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected StepValidationError, got %T: %v", err, err)
	}
	if validationErr.Field != "state" {
		t.Errorf("Field = %q, want %q", validationErr.Field, "state")
	}
	if !strings.Contains(validationErr.Message, "invalid state") {
		t.Errorf("Message should contain 'invalid state', got: %q", validationErr.Message)
	}
}

// TestHandleService_ValidStates verifies all valid state values are accepted
func TestHandleService_ValidStates(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping systemd test on non-Linux platform")
	}

	validStates := []string{
		ServiceStateStarted,
		ServiceStateStopped,
		ServiceStateRestarted,
		ServiceStateReloaded,
	}

	for _, state := range validStates {
		t.Run(state, func(t *testing.T) {
			ec := newTestExecutionContext(t)
			ec.DryRun = true // Use dry-run to avoid actual systemctl calls

			step := config.Step{
				Service: &config.ServiceAction{
					Name:  "test-service",
					State: state,
				},
			}

			err := HandleService(step, ec)
			if err != nil {
				t.Errorf("Valid state %q should not error, got: %v", state, err)
			}
		})
	}
}

// TestHandleService_DryRun verifies dry-run mode doesn't execute commands
func TestHandleService_DryRun(t *testing.T) {
	ec := newTestExecutionContext(t)
	ec.DryRun = true

	step := config.Step{
		Service: &config.ServiceAction{
			Name:    "nginx",
			State:   ServiceStateStarted,
			Enabled: boolPtr(true),
		},
	}

	err := HandleService(step, ec)
	if err != nil {
		t.Fatalf("Dry-run should not error: %v", err)
	}

	// In dry-run mode, no result should be set (handler returns early)
	if ec.CurrentResult != nil {
		t.Error("Dry-run should not set CurrentResult")
	}
}

// TestHandleService_UnsupportedPlatform verifies error on unsupported platforms
func TestHandleService_UnsupportedPlatform(t *testing.T) {
	// This test verifies the platform detection works correctly
	// We can't actually test unsupported platforms directly, but we can verify
	// the logic is there by checking the runtime.GOOS value
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		ec := newTestExecutionContext(t)
		step := config.Step{
			Service: &config.ServiceAction{
				Name: "test",
			},
		}

		err := HandleService(step, ec)
		if err == nil {
			t.Fatal("Expected error for unsupported platform")
		}

		var setupErr *SetupError
		if !errors.As(err, &setupErr) {
			t.Fatalf("expected SetupError, got %T: %v", err, err)
		}
	}
}

// TestManageSystemdUnitFile_MissingBothContentAndTemplate verifies error
func TestManageSystemdUnitFile_MissingBothContentAndTemplate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping systemd test on non-Linux platform")
	}

	ec := newTestExecutionContext(t)
	unit := &config.ServiceUnit{
		// Neither Content nor SrcTemplate provided
	}

	_, err := manageSystemdUnitFile("test", unit, config.Step{}, ec)
	if err == nil {
		t.Fatal("Expected error when both content and src_template are missing")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected StepValidationError, got %T: %v", err, err)
	}
	if validationErr.Field != "service.unit" {
		t.Errorf("Field = %q, want %q", validationErr.Field, "service.unit")
	}
}

// TestManageSystemdUnitFile_InlineContent verifies unit file creation with inline content
func TestManageSystemdUnitFile_InlineContent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping systemd test on non-Linux platform")
	}

	ec := newTestExecutionContext(t)

	// Use a temporary directory for the unit file (not /etc/systemd/system)
	unitPath := filepath.Join(ec.CurrentDir, "test.service")

	unitContent := `[Unit]
Description=Test Service

[Service]
ExecStart=/usr/bin/test
`

	unit := &config.ServiceUnit{
		Dest:    unitPath,
		Content: unitContent,
		Mode:    "0644",
	}

	// First call should create the file
	changed, err := manageSystemdUnitFile("test", unit, config.Step{}, ec)
	if err != nil {
		t.Fatalf("Failed to create unit file: %v", err)
	}
	if !changed {
		t.Error("First call should report changed=true")
	}

	// Verify file was created
	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		t.Error("Unit file should have been created")
	}

	// Read and verify content
	content, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatalf("Failed to read unit file: %v", err)
	}
	if string(content) != unitContent {
		t.Errorf("Content mismatch:\ngot:  %q\nwant: %q", string(content), unitContent)
	}

	// Second call with same content should be idempotent
	changed, err = manageSystemdUnitFile("test", unit, config.Step{}, ec)
	if err != nil {
		t.Fatalf("Failed on idempotent call: %v", err)
	}
	if changed {
		t.Error("Idempotent call should report changed=false")
	}
}

// TestManageSystemdUnitFile_FromTemplate verifies unit file creation from template
func TestManageSystemdUnitFile_FromTemplate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping systemd test on non-Linux platform")
	}

	ec := newTestExecutionContext(t)
	ec.Variables["service_description"] = "My Custom Service"
	ec.Variables["exec_path"] = "/usr/bin/myapp"

	// Create template file
	templatePath := filepath.Join(ec.CurrentDir, "service.template")
	templateContent := `[Unit]
Description={{ service_description }}

[Service]
ExecStart={{ exec_path }}
`
	createTestFile(t, templatePath, templateContent)

	// Use temporary directory for unit file
	unitPath := filepath.Join(ec.CurrentDir, "myapp.service")

	unit := &config.ServiceUnit{
		Dest:        unitPath,
		SrcTemplate: "service.template",
		Mode:        "0644",
	}

	changed, err := manageSystemdUnitFile("myapp", unit, config.Step{}, ec)
	if err != nil {
		t.Fatalf("Failed to create unit file from template: %v", err)
	}
	if !changed {
		t.Error("Should report changed=true")
	}

	// Verify file was created with rendered template
	content, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatalf("Failed to read unit file: %v", err)
	}

	expectedContent := `[Unit]
Description=My Custom Service

[Service]
ExecStart=/usr/bin/myapp
`
	if string(content) != expectedContent {
		t.Errorf("Content mismatch:\ngot:  %q\nwant: %q", string(content), expectedContent)
	}
}

// TestManageSystemdUnitFile_DefaultPath verifies default unit file path
func TestManageSystemdUnitFile_DefaultPath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping systemd test on non-Linux platform")
	}

	ec := newTestExecutionContext(t)

	// When dest is empty, it should default to /etc/systemd/system/<name>.service
	unit := &config.ServiceUnit{
		Content: "[Unit]\nDescription=Test\n",
	}

	_, err := manageSystemdUnitFile("test", unit, config.Step{}, ec)

	// Running as root (e.g., in Docker): write succeeds, file should be created
	// Running as non-root: should fail with permission error
	if os.Geteuid() == 0 {
		// Running as root - write should succeed
		if err != nil {
			t.Errorf("Expected success when running as root, got error: %v", err)
		}
		// Verify file was created at default path
		defaultPath := "/etc/systemd/system/test.service"
		if _, statErr := os.Stat(defaultPath); statErr != nil {
			t.Errorf("Expected file to be created at %s, but stat failed: %v", defaultPath, statErr)
		}
		// Clean up
		os.Remove(defaultPath)
	} else {
		// Running as non-root - should fail with permission error
		if err == nil {
			t.Error("Expected error when writing to /etc/systemd/system without permissions")
		}
	}
}

// TestManageSystemdDropin_MissingName verifies error when dropin name is empty
func TestManageSystemdDropin_MissingName(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping systemd test on non-Linux platform")
	}

	ec := newTestExecutionContext(t)
	dropin := &config.ServiceDropin{
		Name:    "", // Missing
		Content: "[Service]\nEnvironment=FOO=bar\n",
	}

	_, err := manageSystemdDropin("test", dropin, config.Step{}, ec)
	if err == nil {
		t.Fatal("Expected error for missing dropin name")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected StepValidationError, got %T: %v", err, err)
	}
	if validationErr.Field != "service.dropin.name" {
		t.Errorf("Field = %q, want %q", validationErr.Field, "service.dropin.name")
	}
}

// TestManageSystemdDropin_MissingBothContentAndTemplate verifies error
func TestManageSystemdDropin_MissingBothContentAndTemplate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping systemd test on non-Linux platform")
	}

	ec := newTestExecutionContext(t)
	dropin := &config.ServiceDropin{
		Name: "10-override.conf",
		// Neither Content nor SrcTemplate provided
	}

	_, err := manageSystemdDropin("test", dropin, config.Step{}, ec)
	if err == nil {
		t.Fatal("Expected error when both content and src_template are missing")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected StepValidationError, got %T: %v", err, err)
	}
	if validationErr.Field != "service.dropin" {
		t.Errorf("Field = %q, want %q", validationErr.Field, "service.dropin")
	}
}

// TestWriteFileWithPrivileges_DirectWrite verifies successful direct write
func TestWriteFileWithPrivileges_DirectWrite(t *testing.T) {
	ec := newTestExecutionContext(t)

	testPath := filepath.Join(ec.CurrentDir, "test.conf")
	content := []byte("test content")

	err := writeFileWithPrivileges(testPath, content, "0644", config.Step{}, ec)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("File should have been created")
	}

	// Verify content
	readContent, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(readContent) != string(content) {
		t.Errorf("Content mismatch: got %q, want %q", string(readContent), string(content))
	}
}

// TestWriteFileWithPrivileges_InvalidMode verifies mode parsing
func TestWriteFileWithPrivileges_InvalidMode(t *testing.T) {
	ec := newTestExecutionContext(t)

	testPath := filepath.Join(ec.CurrentDir, "test.conf")
	content := []byte("test content")

	// Invalid mode should be handled gracefully (parseFileMode has default fallback)
	err := writeFileWithPrivileges(testPath, content, "", config.Step{}, ec)
	if err != nil {
		t.Fatalf("Should handle empty mode gracefully: %v", err)
	}
}

// TestServiceStateConstants verifies service state constants
func TestServiceStateConstants(t *testing.T) {
	states := []string{
		ServiceStateStarted,
		ServiceStateStopped,
		ServiceStateRestarted,
		ServiceStateReloaded,
	}

	expectedValues := []string{
		"started",
		"stopped",
		"restarted",
		"reloaded",
	}

	for i, state := range states {
		if state != expectedValues[i] {
			t.Errorf("Constant mismatch: got %q, want %q", state, expectedValues[i])
		}
	}
}

// TestHandleService_TemplateRendering verifies variable substitution in service name
func TestHandleService_TemplateRendering(t *testing.T) {
	ec := newTestExecutionContext(t)
	ec.DryRun = true
	ec.Variables["app_name"] = "myapp"

	step := config.Step{
		Service: &config.ServiceAction{
			Name:  "{{ app_name }}",
			State: ServiceStateStarted,
		},
	}

	err := HandleService(step, ec)
	if err != nil {
		t.Fatalf("Template rendering failed: %v", err)
	}
}

// TestHandleService_InvalidTemplate verifies template rendering errors
func TestHandleService_InvalidTemplate(t *testing.T) {
	ec := newTestExecutionContext(t)
	ec.DryRun = true

	step := config.Step{
		Service: &config.ServiceAction{
			Name:  "{{ undefined_var",
			State: ServiceStateStarted,
		},
	}

	err := HandleService(step, ec)
	if err == nil {
		t.Fatal("Expected error for invalid template")
	}

	var renderErr *RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("expected RenderError, got %T: %v", err, err)
	}
}

// TestHandleService_CompleteConfiguration verifies complex service configuration
func TestHandleService_CompleteConfiguration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping systemd test on non-Linux platform")
	}

	ec := newTestExecutionContext(t)
	ec.DryRun = true
	ec.Variables["api_key"] = "secret123"

	enabled := true
	step := config.Step{
		Service: &config.ServiceAction{
			Name:         "myapp",
			State:        ServiceStateStarted,
			Enabled:      &enabled,
			DaemonReload: true,
			Unit: &config.ServiceUnit{
				Content: "[Unit]\nDescription=My App\n",
			},
			Dropin: &config.ServiceDropin{
				Name:    "10-env.conf",
				Content: "[Service]\nEnvironment=API_KEY={{ api_key }}\n",
			},
		},
	}

	err := HandleService(step, ec)
	if err != nil {
		t.Fatalf("Complete configuration failed: %v", err)
	}
}

// Helper function to create a bool pointer
func boolPtr(b bool) *bool {
	return &b
}

// ============================================================================
// Launchd Tests (macOS)
// ============================================================================

// TestGetLaunchdDomain verifies domain selection for user vs system services
func TestGetLaunchdDomain(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS platform")
	}

	// System service
	domain := getLaunchdDomain(true)
	if domain != "system" {
		t.Errorf("System domain = %q, want %q", domain, "system")
	}

	// User service (should include UID)
	domain = getLaunchdDomain(false)
	if !strings.HasPrefix(domain, "gui/") {
		t.Errorf("User domain should start with 'gui/', got: %q", domain)
	}
}

// TestGetLaunchdPlistPath verifies plist path generation
func TestGetLaunchdPlistPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS platform")
	}

	ec := newTestExecutionContext(t)

	tests := []struct {
		name       string
		serviceName string
		unit       *config.ServiceUnit
		isSystem   bool
		wantPrefix string
	}{
		{
			name:        "system daemon default path",
			serviceName: "com.example.test",
			unit:        nil,
			isSystem:    true,
			wantPrefix:  "/Library/LaunchDaemons/",
		},
		{
			name:        "user agent default path",
			serviceName: "com.example.test",
			unit:        nil,
			isSystem:    false,
			wantPrefix:  "/Library/LaunchAgents/",
		},
		{
			name:        "custom path",
			serviceName: "com.example.test",
			unit:        &config.ServiceUnit{Dest: "/tmp/test.plist"},
			isSystem:    false,
			wantPrefix:  "/tmp/test.plist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := getLaunchdPlistPath(tt.serviceName, tt.unit, tt.isSystem, ec)
			if !strings.Contains(path, tt.wantPrefix) {
				t.Errorf("Path %q should contain %q", path, tt.wantPrefix)
			}
		})
	}
}

// TestManageLaunchdPlist_InlineContent verifies plist creation with inline content
func TestManageLaunchdPlist_InlineContent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS platform")
	}

	ec := newTestExecutionContext(t)

	// Use a temporary directory for the plist file
	plistPath := filepath.Join(ec.CurrentDir, "com.example.test.plist")

	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.example.test</string>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/bin/test</string>
	</array>
</dict>
</plist>
`

	unit := &config.ServiceUnit{
		Dest:    plistPath,
		Content: plistContent,
		Mode:    "0644",
	}

	// First call should create the file
	changed, err := manageLaunchdPlist("com.example.test", unit, false, config.Step{}, ec)
	if err != nil {
		t.Fatalf("Failed to create plist file: %v", err)
	}
	if !changed {
		t.Error("First call should report changed=true")
	}

	// Verify file was created
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		t.Error("Plist file should have been created")
	}

	// Read and verify content
	content, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("Failed to read plist file: %v", err)
	}
	if string(content) != plistContent {
		t.Errorf("Content mismatch:\ngot:  %q\nwant: %q", string(content), plistContent)
	}

	// Second call with same content should be idempotent
	changed, err = manageLaunchdPlist("com.example.test", unit, false, config.Step{}, ec)
	if err != nil {
		t.Fatalf("Failed on idempotent call: %v", err)
	}
	if changed {
		t.Error("Idempotent call should report changed=false")
	}
}

// TestManageLaunchdPlist_FromTemplate verifies plist creation from template
func TestManageLaunchdPlist_FromTemplate(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS platform")
	}

	ec := newTestExecutionContext(t)
	ec.Variables["service_label"] = "com.example.myapp"
	ec.Variables["program_path"] = "/usr/local/bin/myapp"

	// Create template file
	templatePath := filepath.Join(ec.CurrentDir, "service.plist.template")
	templateContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{ service_label }}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{ program_path }}</string>
	</array>
</dict>
</plist>
`
	createTestFile(t, templatePath, templateContent)

	// Use temporary directory for plist file
	plistPath := filepath.Join(ec.CurrentDir, "myapp.plist")

	unit := &config.ServiceUnit{
		Dest:        plistPath,
		SrcTemplate: "service.plist.template",
		Mode:        "0644",
	}

	changed, err := manageLaunchdPlist("myapp", unit, false, config.Step{}, ec)
	if err != nil {
		t.Fatalf("Failed to create plist file from template: %v", err)
	}
	if !changed {
		t.Error("Should report changed=true")
	}

	// Verify file was created with rendered template
	content, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("Failed to read plist file: %v", err)
	}

	expectedContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.example.myapp</string>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/local/bin/myapp</string>
	</array>
</dict>
</plist>
`
	if string(content) != expectedContent {
		t.Errorf("Content mismatch:\ngot:  %q\nwant: %q", string(content), expectedContent)
	}
}

// TestManageLaunchdPlist_MissingBothContentAndTemplate verifies error
func TestManageLaunchdPlist_MissingBothContentAndTemplate(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS platform")
	}

	ec := newTestExecutionContext(t)
	unit := &config.ServiceUnit{
		// Neither Content nor SrcTemplate provided
	}

	_, err := manageLaunchdPlist("test", unit, false, config.Step{}, ec)
	if err == nil {
		t.Fatal("Expected error when both content and src_template are missing")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected StepValidationError, got %T: %v", err, err)
	}
	if validationErr.Field != "service.unit" {
		t.Errorf("Field = %q, want %q", validationErr.Field, "service.unit")
	}
}

// TestHandleLaunchdService_DryRun verifies dry-run mode
func TestHandleLaunchdService_DryRun(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS platform")
	}

	ec := newTestExecutionContext(t)
	ec.DryRun = true

	enabled := true
	step := config.Step{
		Service: &config.ServiceAction{
			Name:    "com.example.test",
			State:   ServiceStateStarted,
			Enabled: &enabled,
		},
	}

	err := HandleService(step, ec)
	if err != nil {
		t.Fatalf("Dry-run should not error: %v", err)
	}

	// In dry-run mode, no result should be set (handler returns early)
	if ec.CurrentResult != nil {
		t.Error("Dry-run should not set CurrentResult")
	}
}

// TestHandleLaunchdService_CompleteConfiguration verifies complex service configuration
func TestHandleLaunchdService_CompleteConfiguration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS platform")
	}

	ec := newTestExecutionContext(t)
	ec.DryRun = true
	ec.Variables["program_path"] = "/usr/local/bin/myapp"

	enabled := true
	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.example.myapp</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{ program_path }}</string>
	</array>
</dict>
</plist>
`

	step := config.Step{
		Service: &config.ServiceAction{
			Name:    "com.example.myapp",
			State:   ServiceStateStarted,
			Enabled: &enabled,
			Unit: &config.ServiceUnit{
				Content: plistContent,
			},
		},
	}

	err := HandleService(step, ec)
	if err != nil {
		t.Fatalf("Complete configuration failed: %v", err)
	}
}
