package doctor

import "github.com/alehatsman/mooncake/internal/presets"

// registeredChecks returns the doctor catalogue in deterministic order.
// Order is the user-facing report order; section grouping in the renderer
// preserves it. Sections are listed top-to-bottom as they appear in the
// scannable output: install → system → state → presets → tools → project
// → services.
func registeredChecks() []Check {
	return []Check{
		// install
		checkBinary{},
		checkGoRuntime{},
		// system
		checkFacts{},
		// system/metrics deferred: metrics.Collect samples GPU/CPU/net and
		// costs ~1s on cold start, blowing the wall-time budget for a
		// check that only reports info. The `mooncake metrics` command
		// already exercises that path.
		// state
		checkHomeDir{},
		checkRunsLog{},
		checkDiskSpace{},
		// presets
		checkPresetPaths{},
		// tools
		checkTool{name: "git", usedBy: []string{"git.* actions"}},
		checkTool{name: "fzf", usedBy: []string{"mooncake presets (interactive selector)"}},
		checkTool{name: "sudo", usedBy: []string{"steps with become: true"}, unixOnly: true},
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

// presetSearchPaths re-exports the preset loader's paths so doctor can list
// them without importing internal/presets in every check file.
func presetSearchPaths() []string {
	return presets.PresetSearchPaths()
}
