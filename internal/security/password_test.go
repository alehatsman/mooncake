package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInteractivePasswordProvider_Source(t *testing.T) {
	provider := &InteractivePasswordProvider{}
	if provider.Source() != "interactive" {
		t.Errorf("Expected source 'interactive', got '%s'", provider.Source())
	}
}

func TestFilePasswordProvider_Success(t *testing.T) {
	// Create temp file with correct permissions
	tmpDir := t.TempDir()
	passwordFile := filepath.Join(tmpDir, "password")

	expectedPassword := "mypassword123"
	err := os.WriteFile(passwordFile, []byte(expectedPassword+"\n"), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	provider := &FilePasswordProvider{FilePath: passwordFile}
	password, err := provider.GetPassword()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if password != expectedPassword {
		t.Errorf("Expected password '%s', got '%s'", expectedPassword, password)
	}
}

func TestFilePasswordProvider_InvalidPermissions(t *testing.T) {
	// Create temp file with wrong permissions
	tmpDir := t.TempDir()
	passwordFile := filepath.Join(tmpDir, "password")

	err := os.WriteFile(passwordFile, []byte("password"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	provider := &FilePasswordProvider{FilePath: passwordFile}
	_, err = provider.GetPassword()

	if err == nil {
		t.Fatal("Expected error for invalid permissions, got nil")
	}
}

func TestFilePasswordProvider_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	passwordFile := filepath.Join(tmpDir, "password")

	err := os.WriteFile(passwordFile, []byte(""), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	provider := &FilePasswordProvider{FilePath: passwordFile}
	_, err = provider.GetPassword()

	if err == nil {
		t.Fatal("Expected error for empty file, got nil")
	}
}

func TestFilePasswordProvider_NonExistent(t *testing.T) {
	provider := &FilePasswordProvider{FilePath: "/nonexistent/password"}
	_, err := provider.GetPassword()

	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}
}

func TestCLIPasswordProvider_Success(t *testing.T) {
	expectedPassword := "clipassword"
	provider := &CLIPasswordProvider{Password: expectedPassword}

	password, err := provider.GetPassword()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if password != expectedPassword {
		t.Errorf("Expected password '%s', got '%s'", expectedPassword, password)
	}
}

func TestCLIPasswordProvider_Empty(t *testing.T) {
	provider := &CLIPasswordProvider{Password: ""}
	_, err := provider.GetPassword()

	if err == nil {
		t.Fatal("Expected error for empty password, got nil")
	}
}

func TestResolvePassword_NoMethod(t *testing.T) {
	cfg := PasswordConfig{}
	password, err := ResolvePassword(cfg)

	if err != nil {
		t.Fatalf("Expected no error for no method, got: %v", err)
	}

	if password != "" {
		t.Errorf("Expected empty password, got '%s'", password)
	}
}

func TestResolvePassword_MutualExclusion(t *testing.T) {
	cfg := PasswordConfig{
		CLIPassword:    "password1",
		AskInteractive: true,
	}

	_, err := ResolvePassword(cfg)
	if err == nil {
		t.Fatal("Expected error for multiple methods, got nil")
	}
}

func TestResolvePassword_CLIWithoutInsecureFlag(t *testing.T) {
	cfg := PasswordConfig{
		CLIPassword: "password",
		InsecureCLI: false,
	}

	_, err := ResolvePassword(cfg)
	if err == nil {
		t.Fatal("Expected error for CLI password without insecure flag, got nil")
	}
}

func TestResolvePassword_CLIWithInsecureFlag(t *testing.T) {
	expectedPassword := "clipassword"
	cfg := PasswordConfig{
		CLIPassword: expectedPassword,
		InsecureCLI: true,
	}

	password, err := ResolvePassword(cfg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if password != expectedPassword {
		t.Errorf("Expected password '%s', got '%s'", expectedPassword, password)
	}
}

func TestResolvePassword_FileMethod(t *testing.T) {
	tmpDir := t.TempDir()
	passwordFile := filepath.Join(tmpDir, "password")
	expectedPassword := "filepassword"

	err := os.WriteFile(passwordFile, []byte(expectedPassword), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := PasswordConfig{
		PasswordFile: passwordFile,
	}

	password, err := ResolvePassword(cfg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if password != expectedPassword {
		t.Errorf("Expected password '%s', got '%s'", expectedPassword, password)
	}
}

// TestFilePasswordProvider_Source tests the Source method
func TestFilePasswordProvider_Source(t *testing.T) {
	filePath := "/path/to/password"
	provider := &FilePasswordProvider{FilePath: filePath}

	source := provider.Source()
	expected := "file:/path/to/password"

	if source != expected {
		t.Errorf("Expected source '%s', got '%s'", expected, source)
	}
}

// TestCLIPasswordProvider_Source tests the Source method
func TestCLIPasswordProvider_Source(t *testing.T) {
	provider := &CLIPasswordProvider{Password: "test"}

	source := provider.Source()
	expected := "cli:--sudo-pass"

	if source != expected {
		t.Errorf("Expected source '%s', got '%s'", expected, source)
	}
}

// TestEnvPasswordProvider_Source tests the Source method
func TestEnvPasswordProvider_Source(t *testing.T) {
	programPath := "/usr/bin/askpass"
	provider := &EnvPasswordProvider{ProgramPath: programPath}

	source := provider.Source()
	expected := "env:SUDO_ASKPASS=/usr/bin/askpass"

	if source != expected {
		t.Errorf("Expected source '%s', got '%s'", expected, source)
	}
}

// TestEnvPasswordProvider_Success tests successful password retrieval from SUDO_ASKPASS
func TestEnvPasswordProvider_Success(t *testing.T) {
	// Create a temporary askpass script
	tmpDir := t.TempDir()
	askpassScript := filepath.Join(tmpDir, "askpass.sh")

	// Write a simple script that outputs a password
	scriptContent := "#!/bin/sh\necho 'mypassword'\n"
	err := os.WriteFile(askpassScript, []byte(scriptContent), 0700)
	if err != nil {
		t.Fatalf("Failed to create askpass script: %v", err)
	}

	provider := &EnvPasswordProvider{ProgramPath: askpassScript}
	password, err := provider.GetPassword()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if password != "mypassword" {
		t.Errorf("Expected password 'mypassword', got '%s'", password)
	}
}

// TestEnvPasswordProvider_NonExistent tests error for non-existent program
func TestEnvPasswordProvider_NonExistent(t *testing.T) {
	provider := &EnvPasswordProvider{ProgramPath: "/nonexistent/askpass"}
	_, err := provider.GetPassword()

	if err == nil {
		t.Fatal("Expected error for non-existent program, got nil")
	}
}

// TestEnvPasswordProvider_NotExecutable tests error for non-executable program
func TestEnvPasswordProvider_NotExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	nonExecFile := filepath.Join(tmpDir, "not-executable")

	// Create file without execute permissions
	err := os.WriteFile(nonExecFile, []byte("#!/bin/sh\necho test"), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	provider := &EnvPasswordProvider{ProgramPath: nonExecFile}
	_, err = provider.GetPassword()

	if err == nil {
		t.Fatal("Expected error for non-executable program, got nil")
	}
}

// TestEnvPasswordProvider_EmptyOutput tests error for empty password output
func TestEnvPasswordProvider_EmptyOutput(t *testing.T) {
	tmpDir := t.TempDir()
	askpassScript := filepath.Join(tmpDir, "empty-askpass.sh")

	// Script that outputs nothing
	scriptContent := "#!/bin/sh\necho ''\n"
	err := os.WriteFile(askpassScript, []byte(scriptContent), 0700)
	if err != nil {
		t.Fatalf("Failed to create askpass script: %v", err)
	}

	provider := &EnvPasswordProvider{ProgramPath: askpassScript}
	_, err = provider.GetPassword()

	if err == nil {
		t.Fatal("Expected error for empty password output, got nil")
	}
}

// TestEnvPasswordProvider_ProgramFails tests error when program fails
func TestEnvPasswordProvider_ProgramFails(t *testing.T) {
	tmpDir := t.TempDir()
	askpassScript := filepath.Join(tmpDir, "failing-askpass.sh")

	// Script that exits with error
	scriptContent := "#!/bin/sh\nexit 1\n"
	err := os.WriteFile(askpassScript, []byte(scriptContent), 0700)
	if err != nil {
		t.Fatalf("Failed to create askpass script: %v", err)
	}

	provider := &EnvPasswordProvider{ProgramPath: askpassScript}
	_, err = provider.GetPassword()

	if err == nil {
		t.Fatal("Expected error for failing program, got nil")
	}
}

// Note: SUDO_ASKPASS fallback is currently unreachable in ResolvePassword
// because the function returns early when no method is specified.
// This test is commented out until that code path is either fixed or removed.
//
// func TestResolvePassword_SudoAskpassEnv(t *testing.T) {
//   // Test would go here
// }
