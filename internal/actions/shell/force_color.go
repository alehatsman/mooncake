package shell

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// shouldForceChildColor reports whether the child process spawned by a
// shell step should receive FORCE_COLOR=1 in its environment.
//
// Rationale: when `mooncake task X` runs a shell step that invokes
// another CLI (commonly `mooncake apply -c ...`), the child's stdout
// is the pipe the parent uses to capture and prefix output. The child
// detects "not a TTY" and strips ANSI codes — but the user is staring
// at a real terminal connected to the OUTER mooncake. Setting
// FORCE_COLOR=1 tells the child to keep emitting color; the outer
// re-emits those bytes verbatim onto its TTY.
//
// The check is intentionally conservative:
//   - NO_COLOR set in the parent? Skip — user opted out globally.
//   - Outer stdout NOT a TTY? Skip — preserves clean output when the
//     user pipes mooncake's output to a file or another command.
//   - FORCE_COLOR already set? Skip — caller's intent dominates.
//
// The hasEnv check that gates the FORCE_COLOR=1 append in the caller
// also covers the "already set" case so we don't double-inject.
func shouldForceChildColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// hasEnv reports whether a "KEY=..." entry exists in env.
// Used to avoid double-appending FORCE_COLOR when the parent already
// has it (some CI systems set it; user-provided step.Env may set it).
// Comparison is case-sensitive (env vars are conventionally upper).
func hasEnv(env []string, key string) bool {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}
