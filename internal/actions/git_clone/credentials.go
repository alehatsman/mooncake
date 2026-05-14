// Package git_clone — credentials helper. Resolves HTTPS / SSH auth
// into a `[]string` env block + cleanup func that callers wire into
// the git invocation. Secret values are registered with the run-wide
// redactor so they never escape into logs / events.
package git_clone

import (
	"fmt"
	"os"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

// credentialEnv prepares git env extensions for the supplied credentials
// block. Returns the env entries to append to the git command and a
// cleanup func that the caller must defer to remove temp files.
//
// When creds is nil, returns (nil, no-op, nil).
//
// Side effects:
//   - Renders username/password/ssh_key through the template engine
//     so callers can pass `{{ env.GIT_TOKEN }}` etc.
//   - Registers password + (inline) ssh_key with the run-wide
//     Redactor so subsequent log lines have them masked.
//   - Writes an askpass script and/or ssh key to os.TempDir() with
//     mode 0600; the cleanup func removes them.
func credentialEnv(ctx actions.Context, creds *config.GitCredentials) ([]string, func(), error) {
	if creds == nil {
		return nil, func() {}, nil
	}

	username, err := ctx.GetTemplate().Render(creds.Username, ctx.GetVariables())
	if err != nil {
		return nil, func() {}, fmt.Errorf("render username: %w", err)
	}
	password, err := ctx.GetTemplate().Render(creds.Password, ctx.GetVariables())
	if err != nil {
		return nil, func() {}, fmt.Errorf("render password: %w", err)
	}
	sshKey, err := ctx.GetTemplate().Render(creds.SSHKey, ctx.GetVariables())
	if err != nil {
		return nil, func() {}, fmt.Errorf("render ssh_key: %w", err)
	}
	sshOpts, err := ctx.GetTemplate().Render(creds.SSHOptions, ctx.GetVariables())
	if err != nil {
		return nil, func() {}, fmt.Errorf("render ssh_options: %w", err)
	}

	redactor := redactorFromContext(ctx)

	var env []string
	cleanups := []func(){}
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	if password != "" {
		if redactor != nil {
			redactor.AddSensitive(password)
		}
		askpass, rm, err := writeAskpass(password)
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}
		cleanups = append(cleanups, rm)
		env = append(env,
			"GIT_ASKPASS="+askpass,
			"GIT_TERMINAL_PROMPT=0",
		)
		// Configure the username via the URL credential helper. When the
		// HTTPS URL is bare (no embedded user), git uses the `Username`
		// env var, which we surface via the askpass script's first
		// invocation. The askpass receives the prompt text on argv[1]
		// and may dispatch on it.
		if username != "" {
			env = append(env, "GIT_USERNAME="+username)
		}
	}

	if sshKey != "" {
		keyPath := sshKey
		if isInlineKey(sshKey) {
			path, rm, err := writeSSHKey(sshKey)
			if err != nil {
				cleanup()
				return nil, func() {}, err
			}
			if redactor != nil {
				redactor.AddSensitive(sshKey)
			}
			keyPath = path
			cleanups = append(cleanups, rm)
		}
		sshCmd := "ssh -i " + shellEscape(keyPath) + " -F /dev/null -o IdentitiesOnly=yes"
		if sshOpts != "" {
			sshCmd += " " + sshOpts
		}
		env = append(env, "GIT_SSH_COMMAND="+sshCmd)
	}

	return env, cleanup, nil
}

// redactorFromContext extracts the run-wide Redactor when the context
// is a full ExecutionContext; tests can pass simpler contexts and
// receive nil here.
func redactorFromContext(ctx actions.Context) *security.Redactor {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil
	}
	if ec.Svc == nil {
		return nil
	}
	return ec.Svc.Redactor
}

// writeAskpass emits a small shell script that prints `password` on
// stdout. Mode 0700 so only the current user can execute it. The
// returned cleanup func removes the script.
func writeAskpass(password string) (string, func(), error) {
	f, err := os.CreateTemp("", "mooncake-askpass-*.sh")
	if err != nil {
		return "", nil, fmt.Errorf("create askpass: %w", err)
	}
	// Single-quote the password so it survives any embedded special
	// characters except `'`; escape that by closing-and-reopening.
	escaped := strings.ReplaceAll(password, "'", `'\''`)
	body := "#!/bin/sh\nprintf '%s' '" + escaped + "'\n"
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("write askpass: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	if err := os.Chmod(f.Name(), 0o700); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("chmod askpass: %w", err)
	}
	rm := func() { _ = os.Remove(f.Name()) }
	return f.Name(), rm, nil
}

// writeSSHKey writes inline key content to a tempfile with mode 0600.
// Returns the path + a cleanup func that removes the file.
func writeSSHKey(content string) (string, func(), error) {
	f, err := os.CreateTemp("", "mooncake-sshkey-*")
	if err != nil {
		return "", nil, fmt.Errorf("create ssh_key: %w", err)
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("write ssh_key: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	if err := os.Chmod(f.Name(), 0o600); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("chmod ssh_key: %w", err)
	}
	rm := func() { _ = os.Remove(f.Name()) }
	return f.Name(), rm, nil
}

func isInlineKey(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "-----BEGIN")
}

// shellEscape single-quotes s for safe inclusion in a shell command
// like GIT_SSH_COMMAND. Embedded single quotes are escaped via the
// usual `'\''` shell idiom.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
