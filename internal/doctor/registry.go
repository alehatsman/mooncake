package doctor

import "github.com/alehatsman/mooncake/internal/modules"

// registeredChecks returns the doctor catalogue in deterministic order.
// Order is the user-facing report order; section grouping in the renderer
// preserves it. Sections are listed top-to-bottom as they appear in the
// scannable output: install → system → state → modules → tools → project
// → services.
func registeredChecks() []Check {
	return []Check{
		// install
		checkBinary{},
		checkGoRuntime{},
		// system
		checkFacts{},
		checkTLSTrust{}, // MT-21: flag missing ca-certificates pre-emptively
		// system/metrics deferred: metrics.Collect samples GPU/CPU/net and
		// costs ~1s on cold start, blowing the wall-time budget for a
		// check that only reports info. The `mooncake metrics` command
		// already exercises that path.
		// state
		checkHomeDir{},
		checkRunsLog{},
		checkDiskSpace{},
		// modules
		checkModuleCache{},
		// tools
		checkTool{name: "git", usedBy: []string{"git.* actions"}},
		checkTool{name: "sudo", usedBy: []string{"steps with as_user:"}, unixOnly: true},
		// project
		checkProjectConfig{},
		checkProjectValidate{},
		checkProjectSummary{},
		checkProjectLockfile{},
		// services
		checkAgentd{},
		checkMCP{},
	}
}

// moduleCacheRoot re-exports the module fetcher's cache root so doctor can
// report it without importing internal/modules in every check file. An
// unresolvable root (no $HOME, no $MOONCAKE_MODULE_CACHE) yields "" and the
// check reports that rather than failing.
func moduleCacheRoot() string {
	root, err := modules.CacheRoot()
	if err != nil {
		return ""
	}
	return root
}
