//go:build darwin

package os_group //nolint:revive // package name follows action convention

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/dscl"
	"github.com/alehatsman/mooncake/internal/security"
)

func init() {
	lookupGroup = lookupGroupViaDscl
	applyPlan = applyPlanDarwin
}

// lookupGroupViaDscl reads a group record from the local OpenDirectory
// node. dscl exits non-zero when the record is missing — we
// distinguish that from real errors via exec.ExitError, matching the
// Linux getent exit-2 convention.
func lookupGroupViaDscl(name string) (*groupState, error) {
	if _, err := capture(exec.Command("dscl", ".", "-read", "/Groups/"+name, "PrimaryGroupID")); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return &groupState{exists: false}, nil
		}
		return nil, fmt.Errorf("dscl read: %w", err)
	}
	state := &groupState{exists: true}
	if v, err := dsclGroupField(name, "PrimaryGroupID"); err == nil {
		state.gid, _ = strconv.Atoi(v)
	}
	if v, err := dsclGroupField(name, "GroupMembership"); err == nil && v != "" {
		// GroupMembership is space-separated on darwin (vs comma on
		// Linux's /etc/group). Splitting on whitespace via Fields
		// handles single-or-multi-space output and empty cases.
		state.members = strings.Fields(v)
	}
	return state, nil
}

// dsclGroupField reads one property from a Groups record.
// `dscl . -read` output is "Key: value" on one line or "Key:\n value"
// on two when the value is empty or wraps. Mirrors os_user's
// dsclField — kept package-local rather than shared because the
// two action packages are intentionally independent.
func dsclGroupField(name, key string) (string, error) {
	out, err := capture(exec.Command("dscl", ".", "-read", "/Groups/"+name, key))
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if idx := strings.Index(out, ": "); idx >= 0 {
		return strings.TrimSpace(out[idx+2:]), nil
	}
	if lines := strings.SplitN(out, "\n", 2); len(lines) == 2 {
		return strings.TrimSpace(lines[1]), nil
	}
	return "", nil
}

// applyPlanDarwin dispatches the computed operation. The plan's
// createArgs/removeArgs are shaped for Linux (groupadd flags) and
// not consumed verbatim here — we re-derive the dscl operations
// from the plan's structured fields (name, gid, system).
func applyPlanDarwin(runner *security.Privileged, plan computedPlan) error {
	switch plan.operation {
	case "create":
		return createGroupDarwin(runner, plan)
	case "remove":
		return removeGroupDarwin(runner, plan)
	case "modify":
		// computePlan refuses GID renumbering (the only modify case
		// Linux would attempt). If a future modify reason lands,
		// route it here explicitly rather than silently no-op.
		return fmt.Errorf("os.group darwin: modify not implemented")
	}
	return nil
}

// createGroupDarwin creates the Groups record + assigns a
// PrimaryGroupID. dscl . -create on its own makes an empty record;
// without a PrimaryGroupID the group is unusable for chown/chgrp, so
// we always set one — either the operator-pinned gid or the next
// available one in the appropriate range (system: 1–499, regular:
// 500+).
func createGroupDarwin(runner *security.Privileged, plan computedPlan) error {
	base := "/Groups/" + plan.name

	if err := dsclGroupRun(runner, "-create", base); err != nil {
		return err
	}

	gid, err := pickGroupGID(plan)
	if err != nil {
		return fmt.Errorf("os.group darwin: assign gid: %w", err)
	}
	if err := dsclGroupRun(runner, "-create", base, "PrimaryGroupID", strconv.Itoa(gid)); err != nil {
		return err
	}

	// RealName mirrors the group name. macOS tools (Workgroup Manager,
	// System Settings → Users & Groups) display the RealName rather
	// than the record key; setting it keeps the GUI in sync.
	return dsclGroupRun(runner, "-create", base, "RealName", plan.name)
}

func removeGroupDarwin(runner *security.Privileged, plan computedPlan) error {
	return dsclGroupRun(runner, "-delete", "/Groups/"+plan.name)
}

// pickGroupGID returns the GID to assign at creation time. Plan
// already carries the operator's explicit gid (via createArgs
// `--gid <N>`) when set; otherwise we walk the local groups list
// for the first unused GID in the system or user range.
func pickGroupGID(plan computedPlan) (int, error) {
	if gid, ok := explicitGIDFromArgs(plan.createArgs); ok {
		return gid, nil
	}
	if hasSystemFlag(plan.createArgs) {
		return nextAvailableGID(1, 499)
	}
	return nextAvailableGID(500, 0)
}

// explicitGIDFromArgs scans the Linux-shaped createArgs ("--gid",
// "N") looking for an operator-pinned GID. createArgs is built by
// computePlan; the form is stable enough to parse here rather than
// add another field to computedPlan just for this.
func explicitGIDFromArgs(args []string) (int, bool) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--gid" {
			if gid, err := strconv.Atoi(args[i+1]); err == nil {
				return gid, true
			}
		}
	}
	return 0, false
}

func hasSystemFlag(args []string) bool {
	for _, a := range args {
		if a == "--system" {
			return true
		}
	}
	return false
}

// nextAvailableGID returns the lowest unused GID in [minBound,
// maxBound] via the shared dscl helper. maxBound=0 means "no upper
// bound" (regular-group case).
func nextAvailableGID(minBound, maxBound int) (int, error) {
	return dscl.NextAvailableID("/Groups", "PrimaryGroupID", "GID", minBound, maxBound)
}

// dsclGroupRun is the write-side analogue of dsclGroupField. Goes
// through the supplied PrivilegedRunner — dscl mutations on /Groups/*
// require root on macOS.
func dsclGroupRun(runner *security.Privileged, args ...string) error {
	fullArgs := append([]string{"."}, args...)
	out, err := runner.Run(context.TODO(), "dscl", fullArgs...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("dscl %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("dscl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
