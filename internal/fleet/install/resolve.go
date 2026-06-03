package install

import (
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/binstore"
)

// ResolveBinary picks the mooncake binary to ship to a target of
// (targetOS, targetArch). Order:
//
//  1. explicit — an operator-supplied --binary path wins outright.
//  2. the ~/.mooncake/bin store entry for the target platform.
//  3. the controller's own executable, but only if it already matches the
//     target platform (sniffed) — never ship a cross-platform mismatch.
//
// If none apply it returns an actionable error pointing at
// `mooncake selfbuild`. Callers still pass the result through
// VerifyBinaryPlatform before upload as a belt-and-suspenders check (an
// explicit --binary or a stale store entry could be the wrong platform).
func ResolveBinary(targetOS, targetArch, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if p, found, err := binstore.Lookup(targetOS, targetArch); err != nil {
		return "", err
	} else if found {
		return p, nil
	}
	if self, err := os.Executable(); err == nil {
		if gotOS, gotArch, e := sniffBinaryPlatform(self); e == nil && gotOS == targetOS && gotArch == targetArch {
			return self, nil
		}
	}
	dir, _ := binstore.Dir()
	return "", fmt.Errorf(
		"no mooncake binary available for %s/%s: not in the store (%s) and the controller's "+
			"own binary is a different platform. Run `mooncake selfbuild` to populate the store, "+
			"or pass --binary <path>",
		targetOS, targetArch, dir)
}
