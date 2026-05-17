// Package dscl holds the macOS-only helpers that talk to the
// Directory Service command-line utility. Imported by os.user /
// os.group's darwin platform files; built (and only meaningful) on
// macOS, but the helpers are pure shell-outs so the package compiles
// fine on every host — the run-time failure surfaces as "dscl: not
// found" if a non-darwin host ever reaches this code path.
package dscl

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// NextAvailableID returns the lowest unused dscl id (UID or GID) in
// [minBound, maxBound] by listing the given dscl path/field. A
// maxBound of 0 means "no upper bound" (regular-account case;
// system-account callers cap maxBound at the system-id ceiling).
//
// dsclPath is "/Users" or "/Groups"; dsclField is "UniqueID" or
// "PrimaryGroupID". label ("UID" / "GID") is interpolated into the
// returned error so the caller doesn't need a second fmt.Errorf
// wrap.
func NextAvailableID(dsclPath, dsclField, label string, minBound, maxBound int) (int, error) {
	out, err := captureDscl(exec.Command("dscl", ".", "-list", dsclPath, dsclField))
	if err != nil {
		return 0, fmt.Errorf("dscl list %ss: %w", strings.ToLower(label), err)
	}
	used := map[int]bool{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 {
			if id, err := strconv.Atoi(fields[1]); err == nil {
				used[id] = true
			}
		}
	}
	for id := minBound; maxBound == 0 || id <= maxBound; id++ {
		if !used[id] {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no available %s in range [%d, %d]", label, minBound, maxBound)
}

// captureDscl is a tiny shell-out helper local to this package so
// dscl callers don't have to reach into the action-package private
// capture(). Returns stdout on success.
func captureDscl(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
