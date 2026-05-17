package fleet

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// WriteStagedPlan dumps plan bytes to a temp file on the controller and
// returns the absolute path plus the file's sha256. The caller is
// responsible for `os.Remove(planPath)` when done.
//
// `tempPattern` controls the filename — pass a `os.CreateTemp` pattern
// such as "mooncake-fleet-exec-*.yml". `opLabel` (e.g. "exec",
// "observe") is the prefix used when wrapping I/O errors so the
// caller's user-facing op shows through in the message.
//
// Lives in the parent fleet package because both
// internal/fleet/{exec,observe} stage plans this way (was inlined
// per-package to avoid the export; with two callers the export
// overhead is below threshold).
func WriteStagedPlan(plan []byte, tempPattern, opLabel string) (string, string, error) {
	f, err := os.CreateTemp("", tempPattern)
	if err != nil {
		return "", "", fmt.Errorf("%s: create scratch: %w", opLabel, err)
	}
	if _, err := f.Write(plan); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", "", fmt.Errorf("%s: write scratch: %w", opLabel, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", "", fmt.Errorf("%s: close scratch: %w", opLabel, err)
	}
	sum := sha256.Sum256(plan)
	return f.Name(), hex.EncodeToString(sum[:]), nil
}
