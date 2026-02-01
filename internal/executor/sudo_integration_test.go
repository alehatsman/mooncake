package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/security"
)

// TestPasswordResolutionIntegration tests password resolution with file-based input
func TestPasswordResolutionIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	passwordFile := filepath.Join(tmpDir, "password")
	expectedPassword := "testpassword123"

	// Create password file with correct permissions
	err := os.WriteFile(passwordFile, []byte(expectedPassword), 0600)
	if err != nil {
		t.Fatalf("Failed to create password file: %v", err)
	}

	// Test password resolution
	cfg := security.PasswordConfig{
		PasswordFile: passwordFile,
	}

	password, err := security.ResolvePassword(cfg)
	if err != nil {
		t.Fatalf("Failed to resolve password: %v", err)
	}

	if password != expectedPassword {
		t.Errorf("Expected password '%s', got '%s'", expectedPassword, password)
	}
}

// TestRedactorIntegration tests redactor in execution context
func TestRedactorIntegration(t *testing.T) {
	testLogger := logger.NewTestLogger()
	redactor := security.NewRedactor()

	sensitivePassword := "supersecret123"
	redactor.AddSensitive(sensitivePassword)

	// Test redaction
	commandWithPassword := "echo supersecret123 | sudo -S apt install package"
	redacted := redactor.Redact(commandWithPassword)

	if redacted == commandWithPassword {
		t.Error("Password was not redacted")
	}

	if len(redacted) == 0 {
		t.Error("Redacted string is empty")
	}

	// Verify password is not in redacted output
	if containsString(redacted, sensitivePassword) {
		t.Errorf("Redacted output still contains password: %s", redacted)
	}

	// Verify [REDACTED] is present
	if !containsString(redacted, "[REDACTED]") {
		t.Errorf("Redacted output missing [REDACTED] marker: %s", redacted)
	}

	testLogger.Debugf("Redacted: %s", redacted)
}

// TestPasswordFilePermissionValidation tests file permission checking
func TestPasswordFilePermissionValidation(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name        string
		permissions os.FileMode
		shouldFail  bool
	}{
		{"Valid 0600", 0600, false},
		{"Invalid 0644", 0644, true},
		{"Invalid 0640", 0640, true},
		{"Invalid 0666", 0666, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			passwordFile := filepath.Join(tmpDir, "password_"+tc.name)
			err := os.WriteFile(passwordFile, []byte("testpassword"), tc.permissions)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			provider := &security.FilePasswordProvider{FilePath: passwordFile}
			_, err = provider.GetPassword()

			if tc.shouldFail && err == nil {
				t.Errorf("Expected error for permissions %#o, got nil", tc.permissions)
			}
			if !tc.shouldFail && err != nil {
				t.Errorf("Expected no error for permissions %#o, got: %v", tc.permissions, err)
			}
		})
	}
}

// TestMutualExclusionValidation tests that only one password method can be used
func TestMutualExclusionValidation(t *testing.T) {
	testCases := []struct {
		name   string
		config security.PasswordConfig
		fails  bool
	}{
		{
			name: "CLI and Interactive",
			config: security.PasswordConfig{
				CLIPassword:    "pass1",
				AskInteractive: true,
				InsecureCLI:    true,
			},
			fails: true,
		},
		{
			name: "CLI and File",
			config: security.PasswordConfig{
				CLIPassword:  "pass1",
				PasswordFile: "/tmp/pass",
				InsecureCLI:  true,
			},
			fails: true,
		},
		{
			name: "Interactive and File",
			config: security.PasswordConfig{
				AskInteractive: true,
				PasswordFile:   "/tmp/pass",
			},
			fails: true,
		},
		{
			name: "CLI only with insecure flag",
			config: security.PasswordConfig{
				CLIPassword: "pass1",
				InsecureCLI: true,
			},
			fails: false,
		},
		{
			name: "CLI without insecure flag",
			config: security.PasswordConfig{
				CLIPassword: "pass1",
				InsecureCLI: false,
			},
			fails: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := security.ResolvePassword(tc.config)

			if tc.fails && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tc.fails && err != nil && tc.config.PasswordFile == "" {
				// Only fail if it's not a file-based test (file might not exist)
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
