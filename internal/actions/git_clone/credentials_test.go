package git_clone

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

// newCtxWithRedactor builds an ExecutionContext with a real Redactor
// so credential tests can assert that secrets get registered for
// masking.
func newCtxWithRedactor(t *testing.T) (*executor.ExecutionContext, *security.Redactor) {
	t.Helper()
	ctx := newCtx(t, false)
	red := security.NewRedactor()
	ctx.Svc.Redactor = red
	return ctx, red
}

func TestCredentials_NilReturnsNoEnvAndNoopCleanup(t *testing.T) {
	ctx, _ := newCtxWithRedactor(t)
	env, cleanup, err := credentialEnv(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 0 {
		t.Errorf("expected empty env; got %v", env)
	}
	// cleanup must be safe to call
	cleanup()
}

func TestCredentials_PasswordWritesAskpassAndRedacts(t *testing.T) {
	ctx, red := newCtxWithRedactor(t)
	env, cleanup, err := credentialEnv(ctx, &config.GitCredentials{
		Username: "deploy",
		Password: "supersecret-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Find the GIT_ASKPASS entry and verify the script exists + is 0700.
	var askpassPath string
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_ASKPASS=") {
			askpassPath = strings.TrimPrefix(e, "GIT_ASKPASS=")
		}
	}
	if askpassPath == "" {
		t.Fatalf("expected GIT_ASKPASS in env; got %v", env)
	}
	info, err := os.Stat(askpassPath)
	if err != nil {
		t.Fatalf("askpass script missing: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("askpass mode = %v, want 0700", info.Mode().Perm())
	}
	// Script must echo the password when run.
	body, _ := os.ReadFile(askpassPath)
	if !strings.Contains(string(body), "supersecret-token") {
		t.Errorf("askpass body does not embed password; got %q", body)
	}
	// Redactor must mask the password.
	if red.Redact("token=supersecret-token") != "token=[REDACTED]" {
		t.Errorf("password not registered with redactor")
	}
	// GIT_TERMINAL_PROMPT=0 must be present so git doesn't fall back to a tty.
	hasTerm := false
	for _, e := range env {
		if e == "GIT_TERMINAL_PROMPT=0" {
			hasTerm = true
		}
	}
	if !hasTerm {
		t.Errorf("expected GIT_TERMINAL_PROMPT=0 in env; got %v", env)
	}
}

func TestCredentials_AskpassRemovedOnCleanup(t *testing.T) {
	ctx, _ := newCtxWithRedactor(t)
	env, cleanup, err := credentialEnv(ctx, &config.GitCredentials{
		Password: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	var askpassPath string
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_ASKPASS=") {
			askpassPath = strings.TrimPrefix(e, "GIT_ASKPASS=")
		}
	}
	cleanup()
	if _, err := os.Stat(askpassPath); !os.IsNotExist(err) {
		t.Errorf("cleanup did not remove askpass: %v", err)
	}
}

func TestCredentials_SSHKeyPathPassedThrough(t *testing.T) {
	ctx, _ := newCtxWithRedactor(t)
	// Write a fake key file; the helper must NOT copy or rewrite it
	// when the value already points at a path on disk.
	keyfile, err := os.CreateTemp("", "fake-key-*")
	if err != nil {
		t.Fatal(err)
	}
	keyfile.WriteString("not-an-actual-key")
	keyfile.Close()
	defer os.Remove(keyfile.Name())

	env, cleanup, err := credentialEnv(ctx, &config.GitCredentials{SSHKey: keyfile.Name()})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var sshCmd string
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_SSH_COMMAND=") {
			sshCmd = strings.TrimPrefix(e, "GIT_SSH_COMMAND=")
		}
	}
	if !strings.Contains(sshCmd, keyfile.Name()) {
		t.Errorf("expected ssh_key path in GIT_SSH_COMMAND; got %q", sshCmd)
	}
	if !strings.Contains(sshCmd, "IdentitiesOnly=yes") {
		t.Errorf("expected IdentitiesOnly=yes hardening in GIT_SSH_COMMAND; got %q", sshCmd)
	}
}

func TestCredentials_SSHKeyInlineWrittenWithMode0600AndRedacted(t *testing.T) {
	ctx, red := newCtxWithRedactor(t)
	inline := "-----BEGIN OPENSSH PRIVATE KEY-----\nAAAAFAKEKEYBYTES==\n-----END OPENSSH PRIVATE KEY-----"
	env, cleanup, err := credentialEnv(ctx, &config.GitCredentials{SSHKey: inline})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var sshCmd string
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_SSH_COMMAND=") {
			sshCmd = strings.TrimPrefix(e, "GIT_SSH_COMMAND=")
		}
	}
	// Extract the key path from the command. The path is single-quoted
	// after `-i `; grab the substring between quotes.
	start := strings.Index(sshCmd, "-i '")
	if start < 0 {
		t.Fatalf("no -i flag in GIT_SSH_COMMAND: %q", sshCmd)
	}
	rest := sshCmd[start+4:]
	end := strings.Index(rest, "'")
	if end < 0 {
		t.Fatalf("unmatched quote in GIT_SSH_COMMAND: %q", sshCmd)
	}
	keyPath := rest[:end]
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("inline key file missing: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("inline key mode = %v, want 0600", info.Mode().Perm())
	}
	body, _ := os.ReadFile(keyPath)
	if !strings.Contains(string(body), "AAAAFAKEKEYBYTES==") {
		t.Errorf("inline key body not written correctly; got %q", body)
	}
	// Inline key registered as redacted.
	if red.Redact(inline) != "[REDACTED]" {
		t.Errorf("inline key not redacted by Redactor")
	}
}

func TestCredentials_TemplatedPassword(t *testing.T) {
	ctx, _ := newCtxWithRedactor(t)
	ctx.Scope.User["api_token"] = "ghp_abc123def456"
	env, cleanup, err := credentialEnv(ctx, &config.GitCredentials{
		Password: "{{ api_token }}",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	// The askpass script must embed the resolved value, not the
	// template placeholder.
	var askpassPath string
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_ASKPASS=") {
			askpassPath = strings.TrimPrefix(e, "GIT_ASKPASS=")
		}
	}
	body, _ := os.ReadFile(askpassPath)
	if strings.Contains(string(body), "{{") {
		t.Errorf("template not rendered: %q", body)
	}
	if !strings.Contains(string(body), "ghp_abc123def456") {
		t.Errorf("rendered token missing from askpass: %q", body)
	}
}

func TestCredentials_SSHOptionsAppended(t *testing.T) {
	ctx, _ := newCtxWithRedactor(t)
	keyfile, err := os.CreateTemp("", "fake-key-*")
	if err != nil {
		t.Fatal(err)
	}
	keyfile.Close()
	defer os.Remove(keyfile.Name())
	env, cleanup, err := credentialEnv(ctx, &config.GitCredentials{
		SSHKey:     keyfile.Name(),
		SSHOptions: "-o StrictHostKeyChecking=no",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var sshCmd string
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_SSH_COMMAND=") {
			sshCmd = strings.TrimPrefix(e, "GIT_SSH_COMMAND=")
		}
	}
	if !strings.Contains(sshCmd, "StrictHostKeyChecking=no") {
		t.Errorf("ssh_options not appended; got %q", sshCmd)
	}
}

// askpassPathFromEnv extracts the GIT_ASKPASS value or fails the test.
func askpassPathFromEnv(t *testing.T, env []string) string {
	t.Helper()
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_ASKPASS=") {
			return strings.TrimPrefix(e, "GIT_ASKPASS=")
		}
	}
	t.Fatalf("GIT_ASKPASS missing from env: %v", env)
	return ""
}

// runAskpass invokes the askpass script with the supplied prompt as
// argv[1] (matching what git does). Returns the script's stdout.
func runAskpass(t *testing.T, path, prompt string) string {
	t.Helper()
	out, err := exec.Command(path, prompt).Output()
	if err != nil {
		t.Fatalf("exec askpass: %v", err)
	}
	return string(out)
}

// TestCredentials_AskpassReturnsUsernameForUsernamePrompt is the F028
// reproducer. git's askpass protocol invokes the script with the
// prompt text as argv[1]; the script must return the username when
// the prompt starts with "Username". Pre-fix the script returned the
// password for every prompt — git then attempted password auth with
// (password, password) and the remote rejected.
func TestCredentials_AskpassReturnsUsernameForUsernamePrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("askpass is a /bin/sh script; not portable to Windows")
	}
	ctx, _ := newCtxWithRedactor(t)
	env, cleanup, err := credentialEnv(ctx, &config.GitCredentials{
		Username: "oauth2",
		Password: "ghp_token_xyz",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	path := askpassPathFromEnv(t, env)
	got := runAskpass(t, path, "Username for 'https://github.com/owner/repo': ")
	if got != "oauth2" {
		t.Errorf("askpass returned %q for Username prompt; want %q", got, "oauth2")
	}
}

// TestCredentials_AskpassReturnsPasswordForPasswordPrompt mirrors the
// happy path: prompts starting with anything other than "Username"
// (in practice "Password for ...") must return the password.
func TestCredentials_AskpassReturnsPasswordForPasswordPrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("askpass is a /bin/sh script; not portable to Windows")
	}
	ctx, _ := newCtxWithRedactor(t)
	env, cleanup, err := credentialEnv(ctx, &config.GitCredentials{
		Username: "oauth2",
		Password: "ghp_token_xyz",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	path := askpassPathFromEnv(t, env)
	got := runAskpass(t, path, "Password for 'oauth2@https://github.com/owner/repo': ")
	if got != "ghp_token_xyz" {
		t.Errorf("askpass returned %q for Password prompt; want %q", got, "ghp_token_xyz")
	}
}

// TestCredentials_AskpassUsernameWithEmbeddedQuote verifies single-quote
// escaping in the username path (F028 + the existing password-quote
// invariant). A username containing a `'` must round-trip through the
// shell-quoted script body intact.
func TestCredentials_AskpassUsernameWithEmbeddedQuote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("askpass is a /bin/sh script; not portable to Windows")
	}
	ctx, _ := newCtxWithRedactor(t)
	env, cleanup, err := credentialEnv(ctx, &config.GitCredentials{
		Username: "user's-name",
		Password: "pw",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	path := askpassPathFromEnv(t, env)
	got := runAskpass(t, path, "Username for 'https://x': ")
	if got != "user's-name" {
		t.Errorf("askpass returned %q for Username prompt; want %q", got, "user's-name")
	}
}

func TestRunGit_ReceivesEnv(t *testing.T) {
	// Replace the runner with a recorder so we can inspect what got
	// passed through.
	var capturedEnv []string
	orig := gitRunner
	gitRunner = func(_ string, env []string, _ []string) error {
		capturedEnv = env
		return nil
	}
	t.Cleanup(func() { gitRunner = orig })
	_ = runGit("", []string{"GIT_ASKPASS=/tmp/test"}, "status")
	hasAskpass := false
	for _, e := range capturedEnv {
		if e == "GIT_ASKPASS=/tmp/test" {
			hasAskpass = true
		}
	}
	if !hasAskpass {
		t.Errorf("env not threaded through runGit; got %v", capturedEnv)
	}
}
