package agentd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// userBinDirs are the per-user binary directories a login shell normally
// puts on PATH but a systemd service's minimal PATH (and a non-login
// shell) does not. Installed CLIs land here: pipx/npm-prefix/`claude` in
// ~/.local/bin, `go install` tools in ~/go/bin, hand-placed tools in ~/bin.
func userBinDirs(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "bin"),
		filepath.Join(home, "go", "bin"),
	}
}

// AugmentPathEnv returns path with the user bin dirs for home prepended,
// skipping any already present. Prepending (not appending) matches login-
// shell precedence, where ~/.local/bin shadows system bins. Returns path
// unchanged when there's nothing to add.
func AugmentPathEnv(path, home string) string {
	present := make(map[string]bool)
	for _, p := range filepath.SplitList(path) {
		present[p] = true
	}
	var prefix []string
	for _, d := range userBinDirs(home) {
		if d != "" && !present[d] {
			prefix = append(prefix, d)
		}
	}
	if len(prefix) == 0 {
		return path
	}
	sep := string(os.PathListSeparator)
	if path == "" {
		return strings.Join(prefix, sep)
	}
	return strings.Join(prefix, sep) + sep + path
}

// ApplyPathAugmentation prepends the daemon user's bin dirs to the process
// PATH so commands run during fleet applies find user-installed CLIs that
// the service's minimal PATH would miss. Called once at agentd startup,
// before serving, so every run child (and the systemd-run sandbox escape,
// which forwards the environment) inherits it. No-op on Windows (the
// ~/.local/bin convention is unix-only). See issue #141.
func ApplyPathAugmentation() {
	if runtime.GOOS == "windows" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	_ = os.Setenv("PATH", AugmentPathEnv(os.Getenv("PATH"), home))
}
