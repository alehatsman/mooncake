package executor_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"

	_ "github.com/alehatsman/mooncake/internal/register"
)

// These tests exercise the #11 permissions-as-contract gate through the
// real executor.Start path — the same path moongit drives for an
// unattended agent run. They prove the wiring (StartConfig.Policy →
// dispatchRunner) blocks a step BEFORE its side effects, which is the
// whole safety claim: a denied shell step must leave no trace on disk.

// TestPolicy_DeniedShell_BlocksBeforeSideEffect is the core agent-run
// guarantee: with shell on the denylist, a plan whose shell step would
// write a file fails the run AND the file never appears.
func TestPolicy_DeniedShell_BlocksBeforeSideEffect(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "pwned")

	yaml := `version: "1.0"
steps:
  - shell: "touch ` + sentinel + `"
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	publisher := events.NewPublisher()
	defer publisher.Close()

	err := executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
		Policy:         &executor.Policy{DeniedActions: []string{"shell", "cmd"}},
	}, logger.NewTestLogger(), publisher)

	if err == nil {
		t.Fatal("expected the run to fail on the denied shell step")
	}
	if !strings.Contains(err.Error(), "denylist") {
		t.Errorf("error should name the policy denylist, got: %v", err)
	}
	if _, statErr := os.Stat(sentinel); statErr == nil {
		t.Errorf("shell step ran despite the policy — sentinel %s exists", sentinel)
	}
}

// TestPolicy_Allowlist_PermitsListedAction proves the gate doesn't
// break a normal run: an allowlist that includes the action lets it
// through and the side effect happens.
func TestPolicy_Allowlist_PermitsListedAction(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	yaml := `version: "1.0"
steps:
  - file.write: { path: ` + target + `, content: ok }
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	publisher := events.NewPublisher()
	defer publisher.Close()

	err := executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
		Policy:         &executor.Policy{AllowedActions: []string{"file.write"}},
	}, logger.NewTestLogger(), publisher)

	if err != nil {
		t.Fatalf("allowlisted file.write should succeed, got: %v", err)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("allowlisted file.write did not run — %s missing: %v", target, statErr)
	}
}

// TestPolicy_Allowlist_DeniesUnlistedAction proves the allowlist is
// exclusive: an action not on it is blocked before its side effect.
func TestPolicy_Allowlist_DeniesUnlistedAction(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	yaml := `version: "1.0"
steps:
  - file.write: { path: ` + target + `, content: ok }
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	publisher := events.NewPublisher()
	defer publisher.Close()

	err := executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
		// Allowlist permits only shell — file.write is excluded.
		Policy: &executor.Policy{AllowedActions: []string{"shell"}},
	}, logger.NewTestLogger(), publisher)

	if err == nil {
		t.Fatal("expected file.write to be denied — not in the allowlist")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("error should name the allowlist rule, got: %v", err)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Errorf("file.write ran despite not being allowlisted — %s exists", target)
	}
}

// TestPolicy_NilAllowsEverything guards the backward-compat contract:
// no policy on StartConfig means every existing run path is unchanged.
func TestPolicy_NilAllowsEverything(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	yaml := `version: "1.0"
steps:
  - file.write: { path: ` + target + `, content: ok }
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	publisher := events.NewPublisher()
	defer publisher.Close()

	err := executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath, // Policy left nil
	}, logger.NewTestLogger(), publisher)

	if err != nil {
		t.Fatalf("nil policy must not affect a normal run, got: %v", err)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("nil policy blocked a run it shouldn't — %s missing", target)
	}
}
