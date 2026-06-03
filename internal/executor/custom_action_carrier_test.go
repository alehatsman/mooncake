package executor_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"

	_ "github.com/alehatsman/mooncake/internal/register"
)

// greetWriteHandler is a custom typed action with no dedicated config.Step
// field. It reads its parameters off step.With (the #111 generic carrier) and
// writes "<who>" to the file named by With["out"], so a test can assert it
// both dispatched and received its params.
type greetWriteHandler struct{}

func (greetWriteHandler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{Name: "demo.greet", Description: "write a greeting (test custom action)"}
}

func (greetWriteHandler) Validate(*config.Step) error { return nil }

func (greetWriteHandler) Run(_ actions.Context, step *config.Step) (actions.Result, error) {
	who, _ := step.With["who"].(string)
	out, _ := step.With["out"].(string)
	r := executor.NewResult()
	if err := os.WriteFile(out, []byte(who), 0o644); err != nil {
		r.Failed = true
		return r, err
	}
	r.Changed = true
	return r, nil
}

// TestCustomActionCarrier_ExecutesFromPlan is the #111 acceptance: a plan
// step that uses the generic `action:`/`with:` carrier dispatches to a
// consumer-registered handler in the injected registry and runs end-to-end
// through executor.Start — proving custom actions execute from a real plan,
// not just surface in the schema chunk (the #105 half).
func TestCustomActionCarrier_ExecutesFromPlan(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "greeting.txt")

	yaml := `version: "1.0"
steps:
  - name: greet via custom action
    action: demo.greet
    with:
      who: world
      out: ` + sentinel + `
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// A registry seeded with the built-ins plus the custom handler. Using a
	// clone of the global ensures the standard validation/dispatch machinery
	// (which still needs the built-ins) keeps working alongside the custom one.
	reg := actions.GlobalRegistry().Clone()
	if err := reg.Register(greetWriteHandler{}); err != nil {
		t.Fatalf("register custom handler: %v", err)
	}

	publisher := events.NewPublisher()
	defer publisher.Close()

	err := executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
		Registry:       reg,
	}, logger.NewTestLogger(), publisher)
	if err != nil {
		t.Fatalf("executor.Start: %v", err)
	}

	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("custom action did not run — sentinel missing: %v", err)
	}
	if string(got) != "world" {
		t.Errorf("custom action ran but with wrong params: got %q, want %q", got, "world")
	}
}

// TestCustomActionTypedKey_ExecutesFromFile is the #115 acceptance for the
// FILE ingestion boundary: a hand-written config that uses the custom action
// as a typed key (`demo.greet:` — byte-identical to a built-in, no carrier)
// dispatches end-to-end through executor.Start. No agent here — the planner's
// reader folds the typed key into the carrier against the injected registry,
// proving the file path works independently of the agent's NormalizePlanBytes.
func TestCustomActionTypedKey_ExecutesFromFile(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "greeting.txt")

	yaml := `version: "1.0"
steps:
  - name: greet via typed key
    demo.greet:
      who: world
      out: ` + sentinel + `
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reg := actions.GlobalRegistry().Clone()
	if err := reg.Register(greetWriteHandler{}); err != nil {
		t.Fatalf("register custom handler: %v", err)
	}

	publisher := events.NewPublisher()
	defer publisher.Close()

	err := executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
		Registry:       reg,
	}, logger.NewTestLogger(), publisher)
	if err != nil {
		t.Fatalf("executor.Start: %v", err)
	}

	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("typed-key custom action did not run — sentinel missing: %v", err)
	}
	if string(got) != "world" {
		t.Errorf("typed-key custom action ran but with wrong params: got %q, want %q", got, "world")
	}
}

// TestCustomActionCarrier_CountsAsOneAction guards the one-action invariant:
// the carrier counts as the step's single action, so combining it with a
// typed action field is rejected, and a carrier-only step is valid.
func TestCustomActionCarrier_CountsAsOneAction(t *testing.T) {
	carrierOnly := config.Step{Action: "demo.greet", With: map[string]interface{}{"k": "v"}}
	if err := carrierOnly.Validate(); err != nil {
		t.Errorf("carrier-only step should be valid, got: %v", err)
	}
	if got := carrierOnly.DetermineActionType(); got != "demo.greet" {
		t.Errorf("DetermineActionType: got %q, want demo.greet", got)
	}

	both := config.Step{
		Action: "demo.greet",
		Shell:  &config.ShellAction{Cmd: "echo hi"},
	}
	if err := both.Validate(); err == nil {
		t.Error("carrier + typed action should fail the one-action invariant")
	}
}
