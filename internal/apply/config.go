package apply

// Config carries the typed inputs needed to apply a plan against the
// local machine. Each field maps to a CLI flag on `mooncake apply`,
// but the Config is the canonical input shape — CLI flags lower into
// this struct, and MCP / SDK / agent-loop callers construct one
// directly without going through CLI parsing.
//
// All fields are post-resolution: ConfigPath is absolute, VarsFiles
// is the merged sequence of overlay + explicit vars files, Tags /
// SkipTags are parsed string slices.
type Config struct {
	// ConfigPath is the resolved path to the root configuration file
	// (typically mooncake.yml or mooncake/main.yml).
	ConfigPath string

	// VarsFiles is the merged sequence of variable-override files,
	// in lowest-to-highest precedence order. Local overlays
	// (vars/common.yml, vars/by-host/<host>.yml — spec-51) come
	// first; explicit --vars args from the CLI follow so they win
	// on key collision.
	VarsFiles []string

	// Tags / SkipTags filter steps by tag. Empty slices mean
	// no filtering.
	Tags     []string
	SkipTags []string

	// SudoPass / SudoPassFile / AskBecomePass are the three
	// mutually-exclusive password-input methods. Runner.Run
	// validates the mutual exclusion.
	SudoPass         string
	SudoPassFile     string
	AskBecomePass    bool
	InsecureSudoPass bool

	// TUI selects the terminal-UI subscriber when true; otherwise
	// the console subscriber is used. Honored only when
	// logger.IsTUISupported() returns true; falls back to console
	// on init failure.
	TUI bool

	// LogLevel is the subscriber's verbosity gate ("debug", "info"
	// (default), or "error"). Affects event filtering, not what's
	// written to the run audit.
	LogLevel string

	// OutputFormat selects the user-visible renderer ("text"
	// (default), "json", "agent", or "quiet"). Runner.Run validates
	// the value; JSON is incompatible with TUI.
	OutputFormat string

	// Artifact-capture configuration. All four are no-ops when
	// ArtifactsDir is empty.
	ArtifactsDir      string
	CaptureFullOutput bool
	MaxOutputBytes    int
	MaxOutputLines    int

	// FactsJSONPath, when non-empty, requests an early facts-snapshot
	// write before the plan starts running. Best-effort: failure is
	// logged but does not abort the run.
	FactsJSONPath string
}
