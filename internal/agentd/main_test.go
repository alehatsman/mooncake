package agentd

import (
	"os"
	"strings"
	"testing"
)

// TestMain sanitizes inherited GIT_* environment variables so tests run
// hermetically when invoked from a git hook context (pre-commit, pre-push).
// Otherwise GIT_DIR/GIT_WORK_TREE point at the host repo and our test
// fixtures' subprocess `git` commands operate on the wrong repo.
func TestMain(m *testing.M) {
	for _, e := range os.Environ() {
		if i := strings.Index(e, "="); i > 0 && strings.HasPrefix(e, "GIT_") {
			os.Unsetenv(e[:i])
		}
	}
	os.Exit(m.Run())
}
