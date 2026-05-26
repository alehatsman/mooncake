// Package main provides the mooncake CLI application.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	fleetcmd "github.com/alehatsman/mooncake/cmd/fleet"
	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/ops"
	_ "github.com/alehatsman/mooncake/internal/register" // Register action handlers
)

var version = "dev"

const (
	outputFormatJSON  = "json"
	outputFormatText  = "text"
	outputFormatYAML  = "yaml"
	outputFormatAgent = "agent"
	outputFormatQuiet = "quiet"

	// Artifact default limits
	defaultMaxOutputBytes = 1048576 // 1MB
	defaultMaxOutputLines = 1000

	// YAML formatting
	yamlIndentSpaces = 2

	// Exit codes
	exitCodeValidationError = 2 // Configuration validation failed
	exitCodeRuntimeError    = 3 // Runtime error during execution
)

// parseTags parses a comma-separated tag string into a slice of trimmed tags
func parseTags(tagsStr string) []string {
	if tagsStr == "" {
		return nil
	}

	var tags []string
	for _, tag := range strings.Split(tagsStr, ",") {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}

// hostnameForLocalOverlays returns os.Hostname() with the first DNS label
// only. macOS reports "MacBook-Air.local"; this trims to "MacBook-Air" so
// the corresponding overlay file an operator would commit is
// vars/by-host/MacBook-Air.yml. No-op on systems that already return a bare
// label.
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

// resolveLocalOverlays returns the auto-loaded overlay vars-files for a
// local `mooncake apply` (or `plan`) run, honoring spec-51's --host,
// $MOONCAKE_HOST, and --overlays=off. The result is meant to be prepended
// to any explicit --vars-file args so user flags still win on collision.
//
// Hostname source order:
//
//  1. --host <name> flag       (explicit; missing by-host file → error)
//  2. $MOONCAKE_HOST env var   (explicit; missing by-host file → error)
//  3. os.Hostname() first-label (implicit; missing by-host file → silent)
//
// configPath is the resolved config file; the plan-dir for overlay lookup
// is its directory.
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
		// Operator named a host specifically. A missing by-host file likely
		// means a typo or a not-yet-created overlay — surface it rather than
		// silently producing an un-overlaid plan.
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

func createApp() *cli.App {
	app := &cli.App{
		Name:                 "mooncake",
		Usage:                "Space fighters provisioning tool, Chookity!",
		Version:              version,
		EnableBashCompletion: true,
		// MT-57: when a user types `mooncake runs submit` (real name is
		// `apply`), urfave/cli's default fallthrough is `No help topic
		// for 'submit'` — no hint that they're close. Enabling Suggest
		// adds the standard "Did you mean 'apply'?" line.
		Suggest: true,
		// Fleet DX proposal-01: --peer uses commas as AND-group
		// separators inside `@k=v,k2=v2` selectors. urfave/cli's
		// default behavior auto-splits StringSliceFlag values on
		// commas BEFORE the action sees them, which would silently
		// turn one AND-group into N OR-groups. Disable the cli-level
		// split; internal parsers (extractStepFilter,
		// derivePsStatusFilter) already split on commas themselves.
		DisableSliceFlagSeparator: true,

		Commands: []*cli.Command{
			initCommand(),
			doctorCommand(),
			presetsCommand(),
			modCommand(),
			docsCommand(),
			schemaCommand(),
			snapshotCommand(),
			historyCommand(),
			mcpCommand(),
			agentdCommand(),
			fleetcmd.Command(),
			stepCommand(),
			taskCommand(),
			toolCommand(),
			queryCommand(),
			applyCommand(),
			planCommand(),
			factsCommand(),
			explainCommand(),
			metricsCommand(),
			actionsCommand(),
			runsCommand(),
			pilotCommand(),
			validateCommand(),
		},
	}

	// MT-66: by default urfave/cli prints the full help dump on a flag
	// parse error (e.g. `mooncake apply --max-plan-age garbage`) — the
	// real error scrolls off-screen above 100+ lines of usage. Set
	// OnUsageError on the app and every command/subcommand so we just
	// emit the error itself. The error surfaces via log.Fatal in main.
	app.OnUsageError = quietUsageError
	applyQuietUsageError(app.Commands)

	return app
}

// quietUsageError suppresses urfave/cli's default "print full help on
// any flag parse error" behavior. The error itself is informative;
// the help dump just buries it. (MT-66)
func quietUsageError(_ *cli.Context, err error, _ bool) error {
	return err
}

// applyQuietUsageError walks the command tree and installs
// quietUsageError on every command/subcommand that doesn't already
// declare its own usage-error handler.
func applyQuietUsageError(cmds []*cli.Command) {
	for _, c := range cmds {
		if c.OnUsageError == nil {
			c.OnUsageError = quietUsageError
		}
		if len(c.Subcommands) > 0 {
			applyQuietUsageError(c.Subcommands)
		}
	}
}

func main() {
	// Propagate the linker-stamped binary version into the fleet
	// subpackage so `fleet bootstrap` can report it as the controller's
	// version to remote agentds.
	fleetcmd.Version = version

	app := createApp()

	if err := app.Run(reorderArgs(os.Args, app)); err != nil {
		log.Fatal(err)
	}
}

// reorderArgs makes urfave/cli v2 accept flags after positional args. The
// library uses Go's stdlib flag.Parse, which stops at the first non-flag
// token, so `mooncake fleet apply <plan> --step-filter X` would otherwise
// reject the flag. We walk the subcommand chain in os.Args, then shuffle
// the tail so any --flag (and its value) precedes the bare positionals.
//
// The flag-vs-positional split needs to know which flags take a value
// (every non-bool urfave flag does) — we look up the matched subcommand's
// Flags slice. Tokens after `--` are passed through unchanged.
func reorderArgs(args []string, app *cli.App) []string {
	if len(args) < 2 {
		return args
	}

	// Walk subcommands. `head` accumulates the program name + subcommand
	// names; `cmd` ends pointing at the deepest matched command (or nil).
	head := []string{args[0]}
	i := 1
	var cmd *cli.Command
	cmds := app.Commands
	for i < len(args) {
		name := args[i]
		var match *cli.Command
		for _, c := range cmds {
			if c.HasName(name) {
				match = c
				break
			}
		}
		if match == nil {
			break
		}
		head = append(head, name)
		cmd = match
		cmds = match.Subcommands
		i++
	}
	if cmd == nil {
		return args
	}

	// Map each flag name (long + aliases) to "takes a value" — true for
	// every urfave/cli v2 flag type except *cli.BoolFlag.
	takesValue := make(map[string]bool)
	for _, f := range cmd.Flags {
		_, isBool := f.(*cli.BoolFlag)
		for _, n := range f.Names() {
			takesValue[n] = !isBool
		}
	}

	// Split the tail into flags-and-their-values vs. bare positionals.
	var flags, positionals []string
	for i < len(args) {
		tok := args[i]
		if tok == "--" {
			// `--` ends flag parsing; everything after is positional.
			positionals = append(positionals, args[i:]...)
			break
		}
		if strings.HasPrefix(tok, "-") && len(tok) > 1 {
			flags = append(flags, tok)
			// `--name=value` already carries its value; `--name value`
			// needs the next token IF the flag is known to take one.
			if !strings.Contains(tok, "=") {
				name := strings.TrimLeft(tok, "-")
				if takesValue[name] && i+1 < len(args) {
					flags = append(flags, args[i+1])
					i++
				}
			}
			i++
			continue
		}
		positionals = append(positionals, tok)
		i++
	}

	out := make([]string, 0, len(args))
	out = append(out, head...)
	out = append(out, flags...)
	out = append(out, positionals...)
	return out
}
