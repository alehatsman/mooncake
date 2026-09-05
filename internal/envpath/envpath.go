// Package envpath keeps the mooncake process PATH useful for the machine
// being converged rather than only for the shell that happened to launch
// it. Every entry point (CLI and agentd) applies it once at startup.
package envpath

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

// systemBinDirs are platform prefixes a package manager installs into but
// that a given shell may not have on PATH yet.
//
// On macOS this is Homebrew. It matters because brew is the one package
// manager mooncake is routinely asked to install *and then use in the same
// run* — the canonical fresh-machine bootstrap (`Make sure brew is
// installed` followed by `pkg`/`pkg.repo` steps). The installer writes
// /etc/paths.d/homebrew, but path_helper only reads that at login-shell
// start, so the already-running mooncake process would never see it and
// every fresh macOS bootstrap needed a second apply.
//
// These are added whether or not they exist yet: the dirs are not stat'd
// (see augment), so a prefix created by an earlier step is resolvable by
// the next one. apt/dnf/pacman need no equivalent — they live in /usr/bin
// and ship with the system.
func systemBinDirs() []string {
	if runtime.GOOS == "darwin" {
		// arm64 prefix first, then the intel/rosetta one.
		return []string{"/opt/homebrew/bin", "/opt/homebrew/sbin", "/usr/local/bin"}
	}
	return nil
}

// augment returns path with dirs prepended, skipping any already present
// and any empty entry. Prepending (not appending) matches login-shell
// precedence, where ~/.local/bin and the brew prefix shadow system bins.
// Returns path unchanged when there's nothing to add.
func augment(path string, dirs []string) string {
	present := make(map[string]bool)
	for _, p := range filepath.SplitList(path) {
		present[p] = true
	}
	var prefix []string
	for _, d := range dirs {
		if d == "" || present[d] {
			continue
		}
		prefix = append(prefix, d)
		// Guard against a duplicate inside dirs itself.
		present[d] = true
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

// Augment returns path with the user bin dirs for home, plus this
// platform's package-manager prefixes, prepended.
func Augment(path, home string) string {
	return augment(path, append(userBinDirs(home), systemBinDirs()...))
}

// Apply prepends those dirs to the process PATH so commands run during an
// apply find user-installed CLIs that a minimal PATH would miss, and find
// a package manager installed by an earlier step in the same run. Called
// once at startup by each entry point, before any step executes, so every
// child process (and the systemd-run sandbox escape, which forwards the
// environment) inherits it. Idempotent — a second call adds nothing.
// No-op on Windows (the ~/.local/bin convention is unix-only, and there is
// no equivalent fixed package-manager prefix). See issue #141.
func Apply() {
	if runtime.GOOS == "windows" {
		return
	}
	// A failed home lookup only costs the user dirs — the platform
	// prefixes still apply, so a brew-only fix isn't lost with it.
	home, _ := os.UserHomeDir()
	_ = os.Setenv("PATH", Augment(os.Getenv("PATH"), home))
}
