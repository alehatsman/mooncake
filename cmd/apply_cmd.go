package main

import "github.com/urfave/cli/v2"

// applyCommand returns the `mooncake apply` cli.Command. The action
// function (`run`) and the shared `applyFlags()` flag set live in
// mooncake.go alongside `runFromPlan` and the signal-handling helper;
// this constructor exists so the registration in createApp() matches
// the constructor-per-verb style the other subsystems use.
func applyCommand() *cli.Command {
	return &cli.Command{
		Name:   "apply",
		Usage:  "Apply a playbook or saved plan. Use --dry-run to preview without changes.",
		Flags:  applyFlags(),
		Action: run,
	}
}
