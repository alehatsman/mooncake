// Package statedir resolves mooncake's per-user state directory —
// ~/.mooncake, holding runs.jsonl, ops.jsonl and bin/. It exists so the
// three consumers agree on one answer, and so `go test ./...` cannot
// scribble on the developer's own history.
package statedir

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// ErrTestIsolation is returned by Dir when a test binary would otherwise
// write to the real user's ~/.mooncake. Callers of the state dir treat
// their writes as best-effort (runlog and ops both discard the error),
// so a test that doesn't care simply logs nothing — which is the
// correct behavior for a test that never meant to log in the first
// place.
var ErrTestIsolation = fmt.Errorf(
	"refusing to use the real ~/.mooncake from a test binary: set MOONCAKE_HOME (or HOME) to a temp dir to opt in")

// Dir returns the mooncake state directory.
//
// $MOONCAKE_HOME wins when set — the documented escape hatch, already
// honored by the binary store. Otherwise it is ~/.mooncake.
func Dir() (string, error) {
	if h := os.Getenv("MOONCAKE_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	if underTest() && isRealUserHome(home) {
		return "", ErrTestIsolation
	}
	return filepath.Join(home, ".mooncake"), nil
}

// Path joins elem onto the state dir.
func Path(elem ...string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{dir}, elem...)...), nil
}

// underTest reports whether this process is a `go test` binary. Detected
// from argv rather than testing.Testing() so production builds don't
// link the testing package (which would register -test.* flags on the
// real CLI). `go test` names the binary <pkg>.test and passes -test.*
// flags; neither shape occurs for an installed mooncake.
func underTest() bool {
	if strings.HasSuffix(strings.TrimSuffix(os.Args[0], ".exe"), ".test") {
		return true
	}
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "-test.") {
			return true
		}
	}
	return false
}

// isRealUserHome reports whether home is the account's actual home
// directory rather than one a test redirected $HOME to. os.UserHomeDir
// reads $HOME; user.Current reads the passwd database, which a test
// cannot move. When they agree, nothing was redirected and a write
// would land in the developer's own history.
//
// A failed lookup (unusual container, cgo-less cross-build) is treated
// as "not the real home": the point is to catch the common accident,
// not to be a security boundary.
func isRealUserHome(home string) bool {
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		return false
	}
	return filepath.Clean(u.HomeDir) == filepath.Clean(home)
}
