package main

import (
	"sort"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"

	kernelcmd "github.com/alehatsman/mooncake/cmd/kernel"
)

// The tree regrew to 26 flat top-level commands with no tiers, six ways
// to execute something, and maintainer build tooling sitting next to
// `apply`. Pin the shape so it can't drift back (specs/cli-surface.md).
func TestTopLevelCommandTree(t *testing.T) {
	want := map[string]string{
		// Run — change or preview a machine.
		"apply": kernelcmd.CategoryRun,
		"plan":  kernelcmd.CategoryRun,
		"task":  kernelcmd.CategoryRun,
		"step":  kernelcmd.CategoryRun,

		// Inspect — read-only.
		"state":    kernelcmd.CategoryInspect,
		"history":  kernelcmd.CategoryInspect,
		"explain":  kernelcmd.CategoryInspect,
		"actions":  kernelcmd.CategoryInspect,
		"drift":    kernelcmd.CategoryInspect,
		"validate": kernelcmd.CategoryInspect,

		// Fleet.
		"fleet": kernelcmd.CategoryFleet,

		// Manage — project and host setup.
		"init":   kernelcmd.CategoryManage,
		"doctor": kernelcmd.CategoryManage,
		"mod":    kernelcmd.CategoryManage,
		"tool":   kernelcmd.CategoryManage,
		"vault":  kernelcmd.CategoryManage,
		"cron":   kernelcmd.CategoryManage,

		// Daemon.
		"agentd": kernelcmd.CategoryDaemon,
		"runs":   kernelcmd.CategoryDaemon,
		"mcp":    kernelcmd.CategoryDaemon,

		// Agent — leaves with the Phase 5 repo split.
		"agent": kernelcmd.CategoryAgent,
	}

	app := createApp()
	got := map[string]string{}
	for _, c := range app.Commands {
		if c.Hidden || c.Name == "help" {
			continue
		}
		got[c.Name] = c.Category
	}

	for name, cat := range want {
		actual, ok := got[name]
		if !ok {
			t.Errorf("command %q is missing from the top-level tree", name)
			continue
		}
		if actual != cat {
			t.Errorf("command %q is in category %q, want %q", name, actual, cat)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("unexpected top-level command %q — add it to a tier here "+
				"and to specs/cli-surface.md, or keep it under `dev`", name)
		}
	}
}

// Commands removed in the regroup must stay removed: `snapshot`, `facts`,
// and `metrics` folded into `state`; `query` was jq's job; `docs`,
// `schema`, and `selfbuild` moved under the hidden `dev` tier.
func TestRemovedTopLevelCommands(t *testing.T) {
	removed := map[string]string{
		"snapshot":  "folded into `mooncake state`",
		"facts":     "folded into `mooncake state facts`",
		"metrics":   "folded into `mooncake state metrics`",
		"query":     "that's jq/yq; read.json / read.yaml cover the in-playbook case",
		"docs":      "moved to `mooncake dev docs`",
		"schema":    "moved to `mooncake dev schema`",
		"selfbuild": "moved to `mooncake dev selfbuild`",
	}
	app := createApp()
	for _, c := range app.Commands {
		if c.Hidden {
			continue
		}
		if why, gone := removed[c.Name]; gone {
			t.Errorf("top-level command %q is back — %s", c.Name, why)
		}
	}
}

// `dev` is hidden but must still resolve, and must carry all three
// maintainer verbs. tasks.yml calls `mooncake dev docs generate` and
// `mooncake dev schema generate`; if this breaks, CI breaks.
func TestDevTierIsHiddenButWired(t *testing.T) {
	app := createApp()
	var dev *cli.Command
	for _, c := range app.Commands {
		if c.Name == "dev" {
			dev = c
			break
		}
	}
	if dev == nil {
		t.Fatal("`dev` command is missing")
	}
	if !dev.Hidden {
		t.Error("`dev` should be Hidden from the top-level listing")
	}

	var subs []string
	for _, c := range dev.Subcommands {
		subs = append(subs, c.Name)
	}
	sort.Strings(subs)
	want := "docs,schema,selfbuild"
	if strings.Join(subs, ",") != want {
		t.Errorf("dev subcommands = %v, want %s", subs, want)
	}
}

// `apply --dry-run` was sugar for `mooncake plan` — two spellings, one
// behavior. The flag is gone; `step --dry-run` is the one that stays,
// because it has no `plan` counterpart to steer users toward.
func TestApplyHasNoDryRunFlag(t *testing.T) {
	app := createApp()
	for _, c := range app.Commands {
		if c.Name != "apply" {
			continue
		}
		for _, f := range c.Flags {
			for _, n := range f.Names() {
				if n == "dry-run" || n == "n" {
					t.Errorf("apply still declares --%s; preview is `mooncake plan`", n)
				}
			}
		}
		return
	}
	t.Fatal("apply command is missing")
}

// The tier order in help is stated, not alphabetical — urfave would put
// `Agent` first and `Run` last.
func TestHelpTierOrder(t *testing.T) {
	installTieredHelp()
	app := createApp()

	var got []string
	for _, c := range orderedCategories(app) {
		if c.Name() != "" {
			got = append(got, c.Name())
		}
	}
	want := []string{
		kernelcmd.CategoryRun,
		kernelcmd.CategoryInspect,
		kernelcmd.CategoryFleet,
		kernelcmd.CategoryManage,
		kernelcmd.CategoryDaemon,
		kernelcmd.CategoryAgent,
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("tier order = %v, want %v", got, want)
	}
}
