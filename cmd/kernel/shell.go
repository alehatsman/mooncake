// Package kernel implements the mooncake "kernel" CLI verbs: apply,
// plan, facts, explain, metrics, validate, actions, agent. Each verb
// gets its own file (apply.go, plan.go, …) with its constructor and
// action; this file hosts the helpers + constants every verb leans on
// (recordOp for ops.jsonl, resolveLocalOverlays for spec-51 by-host
// overlays, the output-format tokens, exit codes, artifact defaults).
package kernel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/ops"
	_ "github.com/alehatsman/mooncake/internal/register" // auto-register all action handlers — the kernel is the entry point that needs the live registry
)

// Output-format tokens. The full set lives here because apply
// (--output-format) accepts agent/quiet in addition to text/json that
// every other verb honors.
const (
	outputFormatJSON  = "json"
	outputFormatText  = "text"
	outputFormatYAML  = "yaml"
	outputFormatAgent = "agent"
	outputFormatQuiet = "quiet"
)

// Artifact default limits. Surfaced through `apply --max-output-bytes`
// / `--max-output-lines`.
const (
	defaultMaxOutputBytes = 1048576 // 1 MiB
	defaultMaxOutputLines = 1000
)

// yamlIndentSpaces is the indent yaml.NewEncoder uses across the
// kernel verbs' YAML output paths.
const yamlIndentSpaces = 2

// Exit codes the kernel verbs surface to the shell. Kept in sync with
// cmdutil.ExitCodeValidationError (the cmdutil copy is required because
// cmd/cmdutil cannot import cmd/kernel without a cycle).
const (
	exitCodeValidationError = 2 // Configuration validation failed
	exitCodeRuntimeError    = 3 // Runtime error during execution
)

// hostnameForLocalOverlays returns os.Hostname() with the first DNS
// label only. macOS reports "MacBook-Air.local"; this trims to
// "MacBook-Air" so the corresponding overlay file an operator would
// commit is vars/by-host/MacBook-Air.yml. No-op on systems that
// already return a bare label.
func hostnameForLocalOverlays() (string, error) {
	h, err := os.Hostname()
	if err != nil {
		return "", err
	}
	if i := strings.Index(h, "."); i >= 0 {
		h = h[:i]
	}
	return h, nil
}

// resolveLocalOverlays returns the auto-loaded overlay vars-files for
// a local `mooncake apply` (or `plan`) run, honoring spec-51's --host,
// $MOONCAKE_HOST, and --overlays=off. The result is meant to be
// prepended to any explicit --vars-file args so user flags still win
// on collision.
//
// Hostname source order:
//
//  1. --host <name> flag       (explicit; missing by-host file → error)
//  2. $MOONCAKE_HOST env var   (explicit; missing by-host file → error)
//  3. os.Hostname() first-label (implicit; missing by-host file → silent)
//
// configPath is the resolved config file; the plan-dir for overlay
// lookup is its directory.
func resolveLocalOverlays(c *cli.Context, configPath string) ([]string, error) {
	switch c.String("overlays") {
	case "", "on":
		// proceed
	case "off":
		return nil, nil
	default:
		return nil, fmt.Errorf("--overlays must be 'on' or 'off', got %q", c.String("overlays"))
	}

	hostname := c.String("host")
	hostExplicit := hostname != ""
	if hostname == "" {
		if env := os.Getenv("MOONCAKE_HOST"); env != "" {
			hostname = env
			hostExplicit = true
		}
	}
	if hostname == "" {
		derived, err := hostnameForLocalOverlays()
		if err != nil {
			return nil, fmt.Errorf("failed to derive hostname for overlay auto-load: %w", err)
		}
		hostname = derived
	}

	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}
	planDir := filepath.Dir(absConfig)

	overlays := fleet.ResolveLocalOverlays(planDir, hostname)

	if hostExplicit {
		// Operator named a host specifically. A missing by-host file
		// likely means a typo or a not-yet-created overlay — surface it
		// rather than silently producing an un-overlaid plan.
		wantHost := filepath.Clean(filepath.Join(planDir, "vars", "by-host", hostname+".yml"))
		found := false
		for _, p := range overlays {
			if p == wantHost {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("overlay file not found for host %q: %s does not exist", hostname, wantHost)
		}
	}

	return overlays, nil
}

// recordOp mints an op_id, writes an ops.jsonl entry, and returns the
// minted id so callers can stamp it onto apply.Config / FromPlanOptions
// / plan-mode metadata (spec-68 wave 2). Best-effort: when the append
// fails we still return the id so the run can proceed; the missing
// ops.jsonl row just means `mooncake explain op/<id>` won't resolve.
//
// Lives at the CLI layer because actor / args / config-path are CLI
// concerns; the apply / plan packages take an already-minted opID as
// input and don't reach back here.
func recordOp(command, configPath string, planOnly bool) string {
	opID := ops.NewOpID()
	user := os.Getenv("USER")
	if user == "" {
		user = "unknown"
	}
	// os.Args[0] is the binary path; skip it so Args is just the
	// post-binary flags the user typed.
	var args []string
	if len(os.Args) > 1 {
		args = append(args, os.Args[1:]...)
	}
	_ = ops.Append(ops.Entry{
		TS:       time.Now().UTC(),
		OpID:     opID,
		Command:  command,
		Args:     args,
		Actor:    "user:" + user,
		Config:   configPath,
		PlanOnly: planOnly,
	})
	return opID
}

// RecordOp is the exported wrapper for recordOp. cmd/task uses it to
// stamp its own op_id; everything else in the kernel calls recordOp
// directly.
func RecordOp(command, configPath string, planOnly bool) string {
	return recordOp(command, configPath, planOnly)
}

// ResolveLocalOverlays is the exported wrapper for resolveLocalOverlays.
// cmd/task uses it to honor --host / --overlays the same way apply and
// plan do.
func ResolveLocalOverlays(c *cli.Context, configPath string) ([]string, error) {
	return resolveLocalOverlays(c, configPath)
}

// RunWithSignalCtx exports apply.go's runWithSignalCtx so cmd/task
// (which drives the same plan→apply pipeline) can install the same
// SIGINT/SIGTERM handling without inlining its own copy.
func RunWithSignalCtx(parent context.Context, body func(context.Context) error) error {
	return runWithSignalCtx(parent, body)
}
