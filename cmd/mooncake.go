// Package main provides the mooncake CLI application.
package main

import (
	"log"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	agentdcmd "github.com/alehatsman/mooncake/cmd/agentd"
	croncmd "github.com/alehatsman/mooncake/cmd/cron"
	docscmd "github.com/alehatsman/mooncake/cmd/docs"
	doctorcmd "github.com/alehatsman/mooncake/cmd/doctor"
	driftcmd "github.com/alehatsman/mooncake/cmd/drift"
	fleetcmd "github.com/alehatsman/mooncake/cmd/fleet"
	historycmd "github.com/alehatsman/mooncake/cmd/history"
	initcmd "github.com/alehatsman/mooncake/cmd/init"
	kernelcmd "github.com/alehatsman/mooncake/cmd/kernel"
	mcpcmd "github.com/alehatsman/mooncake/cmd/mcp"
	modcmd "github.com/alehatsman/mooncake/cmd/mod"
	querycmd "github.com/alehatsman/mooncake/cmd/query"
	schemacmd "github.com/alehatsman/mooncake/cmd/schema"
	selfbuildcmd "github.com/alehatsman/mooncake/cmd/selfbuild"
	snapshotcmd "github.com/alehatsman/mooncake/cmd/snapshot"
	stepcmd "github.com/alehatsman/mooncake/cmd/step"
	taskcmd "github.com/alehatsman/mooncake/cmd/task"
	toolcmd "github.com/alehatsman/mooncake/cmd/tool"
	vaultcmd "github.com/alehatsman/mooncake/cmd/vault"
	"github.com/alehatsman/mooncake/internal/envpath"
)

var version = "dev"

func createApp() *cli.App {
	app := &cli.App{
		Name:                 "mooncake",
		Usage:                "Space fighters provisioning tool, Chookity!",
		Version:              version,
		EnableBashCompletion: true,
		Suggest:              true,
		// Fleet DX proposal-01: --peer uses commas as AND-group
		// separators inside `@k=v,k2=v2` selectors. urfave/cli's
		// default behavior auto-splits StringSliceFlag values on commas
		// BEFORE the action sees them, which would silently turn one
		// AND-group into N OR-groups. Disable the cli-level split;
		// internal parsers (extractStepFilter, derivePsStatusFilter)
		// already split on commas themselves.
		DisableSliceFlagSeparator: true,

		Commands: []*cli.Command{
			initcmd.Command(),
			doctorcmd.Command(),
			modcmd.Command(),
			docscmd.Command(),
			schemacmd.Command(),
			selfbuildcmd.Command(),
			snapshotcmd.Command(),
			historycmd.Command(),
			mcpcmd.Command(),
			agentdcmd.Command(),
			fleetcmd.Command(),
			driftcmd.Command(),
			stepcmd.Command(),
			taskcmd.Command(),
			toolcmd.Command(),
			querycmd.Command(),
			kernelcmd.ApplyCommand(),
			kernelcmd.PlanCommand(),
			kernelcmd.FactsCommand(),
			kernelcmd.ExplainCommand(),
			kernelcmd.MetricsCommand(),
			kernelcmd.ActionsCommand(),
			agentdcmd.RunsCommand(),
			kernelcmd.AgentCommand(),
			kernelcmd.ValidateCommand(),
			vaultcmd.Command(),
			croncmd.Command(),
		},
	}

	// MT-66: urfave/cli prints the full help dump on a flag parse
	// error by default — the real error scrolls off-screen above
	// 100+ lines of usage. Install a quiet handler app-wide.
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
	// Make the process PATH describe the machine, not the shell that
	// launched us: the user bin dirs and this platform's package-manager
	// prefixes. Must happen before any step runs — a playbook that
	// installs Homebrew and then drives `pkg`/`pkg.repo` in the same run
	// resolves `brew` only if /opt/homebrew/bin was already on PATH when
	// the step dispatched (#141).
	envpath.Apply()

	// Propagate the linker-stamped binary version into the cmd/
	// sub-packages that surface it back to operators or remotes.
	fleetcmd.Version = version
	agentdcmd.Version = version
	doctorcmd.Version = version

	app := createApp()

	if err := app.Run(reorderArgs(os.Args, app)); err != nil {
		log.Fatal(err)
	}
}

// reorderArgs makes urfave/cli v2 accept flags after positional args.
// The library uses Go's stdlib flag.Parse, which stops at the first
// non-flag token, so `mooncake fleet apply <plan> --step-filter X`
// would otherwise reject the flag. We walk the subcommand chain in
// os.Args, then shuffle the tail so any --flag (and its value)
// precedes the bare positionals.
//
// The flag-vs-positional split needs to know which flags take a value
// (every non-bool urfave flag does) — we look up the matched
// subcommand's Flags slice. Tokens after `--` are passed through
// unchanged.
func reorderArgs(args []string, app *cli.App) []string {
	if len(args) < 2 {
		return args
	}

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

	takesValue := make(map[string]bool)
	for _, f := range cmd.Flags {
		_, isBool := f.(*cli.BoolFlag)
		for _, n := range f.Names() {
			takesValue[n] = !isBool
		}
	}

	var flags, positionals []string
	for i < len(args) {
		tok := args[i]
		if tok == "--" {
			positionals = append(positionals, args[i:]...)
			break
		}
		if strings.HasPrefix(tok, "-") && len(tok) > 1 {
			flags = append(flags, tok)
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
