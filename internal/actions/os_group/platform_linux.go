//go:build linux

package os_group //nolint:revive // package name follows action convention

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func init() {
	lookupGroup = lookupGroupViaGetent
	applyPlan = applyPlanLinux
}

// lookupGroupViaGetent shells out to `getent group <name>` and parses
// the `:`-separated record. getent exits 2 for "not found" — that's
// the only non-error miss; other exit codes propagate.
func lookupGroupViaGetent(name string) (*groupState, error) {
	out, err := capture(exec.Command("getent", "group", name))
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 2 {
			return &groupState{exists: false}, nil
		}
		return nil, fmt.Errorf("getent group: %w", err)
	}
	line := strings.TrimSpace(out)
	if line == "" {
		return &groupState{exists: false}, nil
	}
	// Format: name:passwd:gid:user1,user2,...
	fields := strings.Split(line, ":")
	if len(fields) < 4 {
		return nil, fmt.Errorf("getent group: malformed line %q", line)
	}
	gid, _ := strconv.Atoi(fields[2])
	state := &groupState{exists: true, gid: gid}
	memberField := strings.TrimSpace(fields[3])
	if memberField != "" {
		state.members = strings.Split(memberField, ",")
	}
	return state, nil
}

// applyPlanLinux dispatches to groupadd/groupmod/groupdel. Argument
// validation already ran in computePlan; this is just the spawn.
func applyPlanLinux(plan computedPlan) error {
	switch plan.operation {
	case "create":
		return runLinuxCmd("groupadd", plan.createArgs...)
	case "modify":
		return runLinuxCmd("groupmod", plan.modifyArgs...)
	case "remove":
		return runLinuxCmd("groupdel", plan.removeArgs...)
	}
	return nil
}

func runLinuxCmd(bin string, args ...string) error {
	// #nosec G204 -- bin is one of groupadd/groupmod/groupdel; args are validated above.
	cmd := exec.Command(bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%s %s: %w: %s", bin, strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), err)
	}
	return nil
}
