//go:build darwin

package os_user //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

func init() {
	lookupUser = lookupUserViaDscl
	applyPlan = applyPlanDarwin
}

func lookupUserViaDscl(name string) (*userState, error) {
	// Probe existence — dscl exits non-zero when the record is missing.
	if _, err := capture(exec.Command("dscl", ".", "-read", "/Users/"+name, "UniqueID")); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return &userState{exists: false}, nil
		}
		return nil, fmt.Errorf("dscl read: %w", err)
	}

	state := &userState{exists: true}

	if v, err := dsclField(name, "UniqueID"); err == nil {
		state.uid, _ = strconv.Atoi(v)
	}
	if v, err := dsclField(name, "PrimaryGroupID"); err == nil {
		state.gid, _ = strconv.Atoi(v)
	}
	if v, err := dsclField(name, "UserShell"); err == nil {
		state.shell = v
	}
	if v, err := dsclField(name, "NFSHomeDirectory"); err == nil {
		state.home = v
	}
	if v, err := dsclField(name, "RealName"); err == nil {
		state.comment = v
	}

	// id -nG works on macOS the same as Linux.
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

// dsclField reads one property from a user record.
// dscl . -read output is either "Key: value" or "Key:\n value" (next line).
func dsclField(name, key string) (string, error) {
	out, err := capture(exec.Command("dscl", ".", "-read", "/Users/"+name, key))
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

func applyPlanDarwin(plan computedPlan, current *userState, d desired) error {
	switch plan.operation {
	case "create":
		return createUserDarwin(d)
	case "modify":
		return modifyUserDarwin(current, d)
	case "remove":
		return removeUserDarwin(d)
	}
	return nil
}

func createUserDarwin(d desired) error {
	base := "/Users/" + d.name

	if err := dsclRun("-create", base); err != nil {
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
	if err := dsclRun("-create", base, "UniqueID", strconv.Itoa(uid)); err != nil {
		return err
	}

	// Primary group: explicit gid > group name > default (staff=20).
	switch {
	case d.gid != nil:
		if err := dsclRun("-create", base, "PrimaryGroupID", strconv.Itoa(*d.gid)); err != nil {
			return err
		}
	case d.group != "":
		gid, err := groupGID(d.group)
		if err != nil {
			return fmt.Errorf("os.user create: lookup group %q: %w", d.group, err)
		}
		if err := dsclRun("-create", base, "PrimaryGroupID", strconv.Itoa(gid)); err != nil {
			return err
		}
	default:
		if err := dsclRun("-create", base, "PrimaryGroupID", "20"); err != nil { // staff
			return err
		}
	}

	if d.shell != "" {
		if err := dsclRun("-create", base, "UserShell", d.shell); err != nil {
			return err
		}
	}

	home := d.home
	if home == "" {
		home = "/Users/" + d.name
	}
	if err := dsclRun("-create", base, "NFSHomeDirectory", home); err != nil {
		return err
	}

	if d.comment != "" {
		if err := dsclRun("-create", base, "RealName", d.comment); err != nil {
			return err
		}
	}

	if d.createHome {
		if err := runCmd("createhomedir", "-c", "-u", d.name); err != nil {
			return err
		}
	}

	for _, g := range d.groups {
		if err := dsclRun("-append", "/Groups/"+g, "GroupMembership", d.name); err != nil {
			return err
		}
	}

	return nil
}

func modifyUserDarwin(current *userState, d desired) error {
	base := "/Users/" + d.name

	if d.uid != nil && *d.uid != current.uid {
		if err := dsclRun("-create", base, "UniqueID", strconv.Itoa(*d.uid)); err != nil {
			return err
		}
	}
	if d.gid != nil && *d.gid != current.gid {
		if err := dsclRun("-create", base, "PrimaryGroupID", strconv.Itoa(*d.gid)); err != nil {
			return err
		}
	}
	if d.shell != "" && d.shell != current.shell {
		if err := dsclRun("-create", base, "UserShell", d.shell); err != nil {
			return err
		}
	}
	if d.home != "" && d.home != current.home {
		if err := dsclRun("-create", base, "NFSHomeDirectory", d.home); err != nil {
			return err
		}
	}
	if d.comment != "" && d.comment != current.comment {
		if err := dsclRun("-create", base, "RealName", d.comment); err != nil {
			return err
		}
	}

	if len(d.groups) > 0 {
		if err := applyGroupsDarwin(d.name, current.groups, d.groups, d.appendGroups); err != nil {
			return err
		}
	}

	return nil
}

func removeUserDarwin(d desired) error {
	// Read home dir before the record is gone, in case we need to remove it.
	var home string
	if d.removeHome {
		home, _ = dsclField(d.name, "NFSHomeDirectory")
	}

	if err := dsclRun("-delete", "/Users/"+d.name); err != nil {
		return err
	}

	if d.removeHome && home != "" {
		// #nosec G204 -- home is read from the directory service, not user input.
		if err := runCmd("rm", "-rf", home); err != nil {
			return err
		}
	}
	return nil
}

// applyGroupsDarwin reconciles supplementary group membership.
// When appendGroups is false, groups not in desired are removed first.
func applyGroupsDarwin(username string, currentGroups, desiredGroups []string, appendGroups bool) error {
	have := stringSet(currentGroups)
	want := stringSet(desiredGroups)

	if !appendGroups {
		for g := range have {
			if !want[g] {
				// Best-effort — ignore errors (user may not actually be in the group record).
				_ = dsclRun("-delete", "/Groups/"+g, "GroupMembership", username)
			}
		}
	}

	for _, g := range desiredGroups {
		if !have[g] {
			if err := dsclRun("-append", "/Groups/"+g, "GroupMembership", username); err != nil {
				return err
			}
		}
	}
	return nil
}

// dsclRun executes a dscl command against the local directory node.
func dsclRun(args ...string) error {
	fullArgs := append([]string{"."}, args...)
	// #nosec G204 -- args are field names and validated record values.
	cmd := exec.Command("dscl", fullArgs...)
	var out strings.Builder
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if msg != "" {
			return fmt.Errorf("dscl %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("dscl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// nextAvailableUID finds the lowest unused UID >= minBound.
// If maxBound is 0, the search is unbounded.
func nextAvailableUID(minBound, maxBound int) (int, error) {
	out, err := capture(exec.Command("dscl", ".", "-list", "/Users", "UniqueID"))
	if err != nil {
		return 0, fmt.Errorf("dscl list uids: %w", err)
	}
	used := map[int]bool{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 {
			if uid, err := strconv.Atoi(fields[1]); err == nil {
				used[uid] = true
			}
		}
	}
	for uid := minBound; maxBound == 0 || uid <= maxBound; uid++ {
		if !used[uid] {
			return uid, nil
		}
	}
	return 0, fmt.Errorf("no available UID in range [%d, %d]", minBound, maxBound)
}

// groupGID resolves a group name to its numeric GID via dscl.
func groupGID(group string) (int, error) {
	out, err := capture(exec.Command("dscl", ".", "-read", "/Groups/"+group, "PrimaryGroupID"))
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
