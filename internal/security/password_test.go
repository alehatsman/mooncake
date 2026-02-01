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
