//go:build linux

package os_group //nolint:revive // package name follows action convention

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/security"
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
func applyPlanLinux(runner *security.Privileged, plan computedPlan) error {
	switch plan.operation {
	case "create":
		return runLinuxCmd(runner, "groupadd", plan.createArgs...)
	case "remove":
		return runLinuxCmd(runner, "groupdel", plan.removeArgs...)
	}
	return nil
}

func runLinuxCmd(runner *security.Privileged, bin string, args ...string) error {
	out, err := runner.Run(context.TODO(), bin, args...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%s %s: %w: %s", bin, strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), err)
	}
	return nil
}
