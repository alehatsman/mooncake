// Package security provides password input methods and redaction for sensitive data.
package security

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// PasswordProvider defines an interface for obtaining sudo passwords
type PasswordProvider interface {
	GetPassword() (string, error)
	Source() string
}

// InteractivePasswordProvider prompts the user for password input
type InteractivePasswordProvider struct{}

func (p *InteractivePasswordProvider) GetPassword() (string, error) {
	fmt.Fprint(os.Stderr, "BECOME password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // New line after password input
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return string(password), nil
}

func (p *InteractivePasswordProvider) Source() string {
	return "interactive"
}

// FilePasswordProvider reads password from a file
type FilePasswordProvider struct {
	FilePath string
}

func (p *FilePasswordProvider) GetPassword() (string, error) {
	// Check file permissions
	info, err := os.Stat(p.FilePath)
	if err != nil {
		return "", fmt.Errorf("cannot access password file: %w", err)
	}

	// Verify file is owned by current user
	if ownerErr := checkFileOwnership(info); ownerErr != nil {
		return "", ownerErr
	}

	// Verify file has 0600 permissions
	mode := info.Mode().Perm()
	if mode != 0600 {
		return "", fmt.Errorf("password file must have 0600 permissions, found %04o", mode)
	}

	// Read password from file
	content, err := os.ReadFile(p.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read password file: %w", err)
	}

	// Trim whitespace
	password := strings.TrimSpace(string(content))
	if password == "" {
		return "", fmt.Errorf("password file is empty")
	}

	return password, nil
}

func (p *FilePasswordProvider) Source() string {
	return fmt.Sprintf("file:%s", p.FilePath)
}

// EnvPasswordProvider executes SUDO_ASKPASS helper program
type EnvPasswordProvider struct {
	ProgramPath string
}

func (p *EnvPasswordProvider) GetPassword() (string, error) {
	// Verify the program is executable
	info, err := os.Stat(p.ProgramPath)
	if err != nil {
		return "", fmt.Errorf("SUDO_ASKPASS program not found: %w", err)
	}

	// Check if executable
	if info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("SUDO_ASKPASS program is not executable: %s", p.ProgramPath)
	}

	// Execute the program
	// #nosec G204 - SUDO_ASKPASS is a standard sudo mechanism for password input.
	// The program path comes from the user's environment variable and is their responsibility to secure.
	cmd := exec.Command(p.ProgramPath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("SUDO_ASKPASS program failed: %w", err)
	}

	password := strings.TrimSpace(string(output))
	if password == "" {
		return "", fmt.Errorf("SUDO_ASKPASS program returned empty password")
	}

	return password, nil
}

func (p *EnvPasswordProvider) Source() string {
	return fmt.Sprintf("env:SUDO_ASKPASS=%s", p.ProgramPath)
}

// CLIPasswordProvider wraps the existing --sudo-pass flag
type CLIPasswordProvider struct {
	Password string
}

func (p *CLIPasswordProvider) GetPassword() (string, error) {
	if p.Password == "" {
		return "", fmt.Errorf("CLI password is empty")
	}
	return p.Password, nil
}

func (p *CLIPasswordProvider) Source() string {
	return "cli:--sudo-pass"
}

// PasswordConfig holds configuration for password resolution
type PasswordConfig struct {
	CLIPassword    string
	AskInteractive bool
	PasswordFile   string
	InsecureCLI    bool
}

// ResolvePassword resolves the sudo password based on configuration.
// When -K/--ask-become-pass is set and SUDO_ASKPASS is exported, the askpass
// helper is preferred over a TTY prompt (matches sudo's -A behavior, lets the
// user wire up a GUI prompt for non-TTY contexts like editor terminals).
func ResolvePassword(cfg PasswordConfig) (string, error) {
	// Count active password methods
	methodCount := 0
	if cfg.CLIPassword != "" {
		methodCount++
	}
	if cfg.AskInteractive {
		methodCount++
	}
	if cfg.PasswordFile != "" {
		methodCount++
	}

	// Check mutual exclusion
	if methodCount > 1 {
		return "", fmt.Errorf("only one password method can be specified (--sudo-pass, --ask-become-pass, --sudo-pass-file)")
	}

	// No password requested
	if methodCount == 0 {
		return "", nil
	}

	// Interactive: prefer SUDO_ASKPASS if available, else fall back to TTY
	if cfg.AskInteractive {
		if askpassPath := os.Getenv("SUDO_ASKPASS"); askpassPath != "" {
			provider := &EnvPasswordProvider{ProgramPath: askpassPath}
			return provider.GetPassword()
		}
		provider := &InteractivePasswordProvider{}
		return provider.GetPassword()
	}

	// File-based second priority
	if cfg.PasswordFile != "" {
		provider := &FilePasswordProvider{FilePath: cfg.PasswordFile}
		return provider.GetPassword()
	}

	// CLI password (requires insecure flag)
	if cfg.CLIPassword != "" {
		if !cfg.InsecureCLI {
			return "", fmt.Errorf("--sudo-pass requires --insecure-sudo-pass flag (WARNING: password will be visible in shell history and process list)")
		}
		provider := &CLIPasswordProvider{Password: cfg.CLIPassword}
		return provider.GetPassword()
	}

	return "", nil
}
