// Package exec implements `mooncake fleet exec` (spec-52). The package
// synthesizes a one-step shell plan on the controller, uploads it to each
// peer's synced root, submits a run via the existing agentd transport,
// and streams events back through the same multiplexer fleet apply uses.
//
// Two output modes:
//
//   - multiplex (default): each line is `[peer] …` via fleet.Multiplexer.
//   - json: one JSONL record per peer with stdout/stderr/exit_code.
//
// Exec deliberately bypasses internal/fleet/apply.go (plan-dir centric)
// in favor of a parallel orchestrator that writes a tiny synthesized plan
// to a per-peer scratch dir under the peer's SyncedRoot.
package exec

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/config"
)

// SynthOptions are the inputs to Synthesize, sourced from the
// `fleet exec` CLI flags. Empty fields are omitted from the YAML so the
// daemon-side loader receives the same shape a human-authored plan would.
type SynthOptions struct {
	// Cmd is the shell command line to run on each peer. Required.
	Cmd string

	// Interpreter overrides the default shell on the peer (bash on
	// Unix, powershell on Windows). Optional. Maps to `shell.interpreter`.
	Interpreter string

	// Env is forwarded as the step's environment. Optional.
	Env map[string]string

	// Cwd is the working directory on the peer. Optional.
	Cwd string

	// Timeout is the per-step wall clock (e.g. "30s", "2m"). Optional.
	// Forwarded as the Step.Timeout field which the kernel parses.
	Timeout string

	// Become, when true, runs the step with sudo on the peer (Unix) /
	// requires admin (Windows). Implemented by setting Step.AsUser="root"
	// — the kernel's existing privilege-escalation plumbing.
	Become bool
}

// Synthesize returns the YAML bytes of a one-step plan that will run
// opts.Cmd through the kernel's shell action handler on the peer. The
// output is guaranteed to round-trip through config.LoadPlan; callers
// rely on that to keep this helper trustworthy without re-parsing the
// generated YAML for every flag combination.
func Synthesize(opts SynthOptions) ([]byte, error) {
	if opts.Cmd == "" {
		return nil, fmt.Errorf("exec: cmd is required")
	}
	step := config.Step{
		Name: "fleet-exec",
		Shell: &config.ShellAction{
			Cmd:         opts.Cmd,
			Interpreter: opts.Interpreter,
		},
		Env:     opts.Env,
		Cwd:     opts.Cwd,
		Timeout: opts.Timeout,
	}
	if opts.Become {
		step.AsUser = "root"
	}

	out, err := yaml.Marshal([]config.Step{step})
	if err != nil {
		return nil, fmt.Errorf("exec: marshal plan: %w", err)
	}
	return out, nil
}
