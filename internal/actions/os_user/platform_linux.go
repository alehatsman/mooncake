//go:build linux

package os_user //nolint:revive // package name follows action convention

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/security"
)

func init() {
	lookupUser = lookupUserViaGetent
	applyPlan = applyPlanLinux
}

func lookupUserViaGetent(name string) (*userState, error) {
	out, err := capture(exec.Command("getent", "passwd", name))
	if err != nil {
		// getent returns exit code 2 when the entry doesn't exist.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 2 {
			return &userState{exists: false}, nil
		}
		return nil, fmt.Errorf("getent passwd: %w", err)
	}
	line := strings.TrimSpace(out)
	if line == "" {
		return &userState{exists: false}, nil
	}
	fields := strings.Split(line, ":")
	if len(fields) < 7 {
		return nil, fmt.Errorf("getent passwd: malformed line %q", line)
	}
	uid, _ := strconv.Atoi(fields[2])
	gid, _ := strconv.Atoi(fields[3])
	state := &userState{
		exists:  true,
		uid:     uid,
		gid:     gid,
		comment: fields[4],
		home:    fields[5],
		shell:   fields[6],
	}

	gout, err := capture(exec.Command("id", "-nG", name))
	if err != nil {
		return nil, fmt.Errorf("id -nG: %w", err)
	}
	allGroups := strings.Fields(strings.TrimSpace(gout))
	if len(allGroups) > 1 {
		state.groups = allGroups[1:]
		sort.Strings(state.groups)
	}
	return state, nil
}

func applyPlanLinux(runner actions.PrivilegedRunner, plan computedPlan, _ *userState, _ desired) error {
	switch plan.operation {
	case "create":
		return runLinuxCmd(runner, "useradd", plan.createArgs...)
	case "modify":
		return runLinuxCmd(runner, "usermod", plan.modifyArgs...)
	case "remove":
		return runLinuxCmd(runner, "userdel", plan.removeArgs...)
	}
	return nil
}

// runLinuxCmd shells out via the PrivilegedRunner so useradd/usermod/
// userdel escalate to root when mooncake isn't already root. nil
// runner falls back to a zero-value security.PrivilegedRunner — same
// shape used by os_group/platform_linux.go's runLinuxCmd. Mirrors the
// spec-69 "tests can pass nil and still get a usable runner" pattern.
func runLinuxCmd(runner actions.PrivilegedRunner, bin string, args ...string) error {
	if runner == nil {
		runner = security.PrivilegedRunner{}
	}
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
