//go:build darwin

package os_user //nolint:revive // package name follows action convention

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/dscl"
	"github.com/alehatsman/mooncake/internal/security"
)

func init() {
	lookupUser = lookupUserViaDscl
	applyPlan = applyPlanDarwin
}

func lookupUserViaDscl(ctx context.Context, name string) (*userState, error) {
	// Probe existence — dscl exits non-zero when the record is missing.
	if _, err := capture(exec.CommandContext(ctx, "dscl", ".", "-read", "/Users/"+name, "UniqueID")); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return &userState{exists: false}, nil
		}
		return nil, fmt.Errorf("dscl read: %w", err)
	}

	state := &userState{exists: true}

	if v, err := dsclField(ctx, name, "UniqueID"); err == nil {
		state.uid, _ = strconv.Atoi(v)
	}
	if v, err := dsclField(ctx, name, "PrimaryGroupID"); err == nil {
		state.gid, _ = strconv.Atoi(v)
	}
	if v, err := dsclField(ctx, name, "UserShell"); err == nil {
		state.shell = v
	}
	if v, err := dsclField(ctx, name, "NFSHomeDirectory"); err == nil {
		state.home = v
	}
	if v, err := dsclField(ctx, name, "RealName"); err == nil {
		state.comment = v
	}

	// id -nG works on macOS the same as Linux.
	gout, err := capture(exec.CommandContext(ctx, "id", "-nG", name))
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

// dsclField reads one property from a user record.
// dscl . -read output is either "Key: value" or "Key:\n value" (next line).
func dsclField(ctx context.Context, name, key string) (string, error) {
	out, err := capture(exec.CommandContext(ctx, "dscl", ".", "-read", "/Users/"+name, key))
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	// "Key: value" on same line
	if idx := strings.Index(out, ": "); idx >= 0 {
		return strings.TrimSpace(out[idx+2:]), nil
	}
	// "Key:\n value" — value indented on next line
	if lines := strings.SplitN(out, "\n", 2); len(lines) == 2 {
		return strings.TrimSpace(lines[1]), nil
	}
	return "", nil
}

func applyPlanDarwin(ctx context.Context, runner *security.Privileged, plan computedPlan, current *userState, d desired) error {
	switch plan.operation {
	case "create":
		return createUserDarwin(ctx, runner, d)
	case "modify":
		return modifyUserDarwin(ctx, runner, current, d)
	case "remove":
		return removeUserDarwin(ctx, runner, d)
	}
	return nil
}

func createUserDarwin(ctx context.Context, runner *security.Privileged, d desired) error {
	base := "/Users/" + d.name

	if err := dscl.Run(ctx, runner, "-create", base); err != nil {
		return err
	}

	// Assign UID — required for macOS to recognise the account.
	uid := 0
	if d.uid != nil {
		uid = *d.uid
	} else {
		var err error
		if d.system {
			uid, err = nextAvailableUID(100, 499)
		} else {
			uid, err = nextAvailableUID(501, 0)
		}
		if err != nil {
			return fmt.Errorf("os.user create: assign uid: %w", err)
		}
	}
	if err := dscl.Run(ctx, runner, "-create", base, "UniqueID", strconv.Itoa(uid)); err != nil {
		return err
	}

	// Primary group: explicit gid > group name > default (staff=20).
	switch {
	case d.gid != nil:
		if err := dscl.Run(ctx, runner, "-create", base, "PrimaryGroupID", strconv.Itoa(*d.gid)); err != nil {
			return err
		}
	case d.group != "":
		gid, err := groupGID(ctx, d.group)
		if err != nil {
			return fmt.Errorf("os.user create: lookup group %q: %w", d.group, err)
		}
		if err := dscl.Run(ctx, runner, "-create", base, "PrimaryGroupID", strconv.Itoa(gid)); err != nil {
			return err
		}
	default:
		if err := dscl.Run(ctx, runner, "-create", base, "PrimaryGroupID", "20"); err != nil { // staff
			return err
		}
	}

	if d.shell != "" {
		if err := dscl.Run(ctx, runner, "-create", base, "UserShell", d.shell); err != nil {
			return err
		}
	}

	home := d.home
	if home == "" {
		home = "/Users/" + d.name
	}
	if err := dscl.Run(ctx, runner, "-create", base, "NFSHomeDirectory", home); err != nil {
		return err
	}

	if d.comment != "" {
		if err := dscl.Run(ctx, runner, "-create", base, "RealName", d.comment); err != nil {
			return err
		}
	}

	if d.createHome {
		if err := runDarwinCmd(runner, "createhomedir", "-c", "-u", d.name); err != nil {
			return err
		}
	}

	for _, g := range d.groups {
		if err := dscl.Run(ctx, runner, "-append", "/Groups/"+g, "GroupMembership", d.name); err != nil {
			return err
		}
	}

	return nil
}

func modifyUserDarwin(ctx context.Context, runner *security.Privileged, current *userState, d desired) error {
	base := "/Users/" + d.name

	if d.uid != nil && *d.uid != current.uid {
		if err := dscl.Run(ctx, runner, "-create", base, "UniqueID", strconv.Itoa(*d.uid)); err != nil {
			return err
		}
	}
	if d.gid != nil && *d.gid != current.gid {
		if err := dscl.Run(ctx, runner, "-create", base, "PrimaryGroupID", strconv.Itoa(*d.gid)); err != nil {
			return err
		}
	}
	if d.shell != "" && d.shell != current.shell {
		if err := dscl.Run(ctx, runner, "-create", base, "UserShell", d.shell); err != nil {
			return err
		}
	}
	if d.home != "" && d.home != current.home {
		if err := dscl.Run(ctx, runner, "-create", base, "NFSHomeDirectory", d.home); err != nil {
			return err
		}
	}
	if d.comment != "" && d.comment != current.comment {
		if err := dscl.Run(ctx, runner, "-create", base, "RealName", d.comment); err != nil {
			return err
		}
	}

	if len(d.groups) > 0 {
		if err := applyGroupsDarwin(ctx, runner, d.name, current.groups, d.groups, d.appendGroups); err != nil {
			return err
		}
	}

	return nil
}

func removeUserDarwin(ctx context.Context, runner *security.Privileged, d desired) error {
	// Read home dir before the record is gone, in case we need to remove it.
	var home string
	if d.removeHome {
		home, _ = dsclField(ctx, d.name, "NFSHomeDirectory")
	}

	if err := dscl.Run(ctx, runner, "-delete", "/Users/"+d.name); err != nil {
		return err
	}

	if d.removeHome && home != "" {
		// #nosec G204 -- home is read from the directory service, not user input.
		if err := runDarwinCmd(runner, "rm", "-rf", home); err != nil {
			return err
		}
	}
	return nil
}

// applyGroupsDarwin reconciles supplementary group membership.
// When appendGroups is false, groups not in desired are removed first.
func applyGroupsDarwin(ctx context.Context, runner *security.Privileged, username string, currentGroups, desiredGroups []string, appendGroups bool) error {
	have := stringSet(currentGroups)
	want := stringSet(desiredGroups)

	if !appendGroups {
		for g := range have {
			if !want[g] {
				// Best-effort — ignore errors (user may not actually be in the group record).
				_ = dscl.Run(ctx, runner, "-delete", "/Groups/"+g, "GroupMembership", username)
			}
		}
	}

	for _, g := range desiredGroups {
		if !have[g] {
			if err := dscl.Run(ctx, runner, "-append", "/Groups/"+g, "GroupMembership", username); err != nil {
				return err
			}
		}
	}
	return nil
}

// runDarwinCmd is the createhomedir / rm escape hatch for the
// non-dscl helpers Darwin's apply path still needs root for.
// Bounded by userCmdTimeout (F051).
func runDarwinCmd(runner *security.Privileged, bin string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), userCmdTimeout)
	defer cancel()
	out, err := runner.Run(ctx, bin, args...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%s %s: %w: %s", bin, strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), err)
	}
	return nil
}

// nextAvailableUID finds the lowest unused UID >= minBound via the
// shared dscl helper. If maxBound is 0, the search is unbounded.
func nextAvailableUID(minBound, maxBound int) (int, error) {
	return dscl.NextAvailableID("/Users", "UniqueID", "UID", minBound, maxBound)
}

// groupGID resolves a group name to its numeric GID via dscl.
func groupGID(ctx context.Context, group string) (int, error) {
	out, err := capture(exec.CommandContext(ctx, "dscl", ".", "-read", "/Groups/"+group, "PrimaryGroupID"))
	if err != nil {
		return 0, fmt.Errorf("dscl read group %q: %w", group, err)
	}
	out = strings.TrimSpace(out)
	if idx := strings.Index(out, ": "); idx >= 0 {
		gid, err := strconv.Atoi(strings.TrimSpace(out[idx+2:]))
		if err != nil {
			return 0, fmt.Errorf("parse gid from %q: %w", out, err)
		}
		return gid, nil
	}
	return 0, fmt.Errorf("unexpected dscl output for group %q: %q", group, out)
}
