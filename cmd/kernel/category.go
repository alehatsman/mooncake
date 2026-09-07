package kernel

// Command categories. urfave/cli groups the top-level help listing by a
// command's Category, so these constants are what turn a flat wall of 26
// verbs into tiers an operator can scan.
//
// They live in cmd/kernel rather than cmd/ because every cmd/* subpackage
// needs them and none of them may import package main. See
// specs/cli-surface.md for the tree this produces.
const (
	// CategoryRun holds the verbs that change or preview a machine.
	CategoryRun = "Run"

	// CategoryInspect holds the read-only verbs: state, history, and the
	// typed lookups over the action vocabulary.
	CategoryInspect = "Inspect"

	// CategoryFleet is the multi-machine surface.
	CategoryFleet = "Fleet"

	// CategoryManage holds project and host setup: scaffolding, health,
	// modules, tools, secrets, cron.
	CategoryManage = "Manage"

	// CategoryDaemon holds the long-running host daemon and its clients.
	CategoryDaemon = "Daemon"

	// CategoryAgent is the LLM agent loop. One command, its own tier,
	// because it is on its way out of this binary entirely — see the
	// Phase 5 split in docs-working/plan-2026-09-07-provisioning-focus.md.
	CategoryAgent = "Agent"
)
