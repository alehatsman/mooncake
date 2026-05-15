// Package os_user implements the os.user action: declarative user
// account management with field-level idempotency. Reads current state
// via getent and applies drift via useradd/usermod/userdel. Linux-only
// in this iteration; macOS (dscl) and Windows are deferred.
//
//nolint:revive // Package name matches action name convention (os_user)
package os_user

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName   = "os.user"
	statePresent = "present"
	stateAbsent  = "absent"
)

// Handler implements os.user.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Declaratively manage a system user account",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux"},
		RequiresSudo:       true,
		ImplementsCheck:    true,
	}
}

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// os.user always declares Sudo=true: useradd/usermod/userdel modify
// /etc/passwd, /etc/shadow, and (with create_home) the user's home
// directory under /home. No Network. The action shells to
// useradd/usermod/userdel — these are required binaries.
//
// FilesystemWrite lists the canonical identity files; consumers
// that care about the home directory have to infer it from the
// step's Home field (or the system default).
func (Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		Sudo:             true,
		RequiredBinaries: []string{"useradd", "usermod", "userdel"},
		FilesystemWrite:  []string{"/etc/passwd", "/etc/shadow", "/etc/group"},
	}
}

func (h *Handler) Validate(step *config.Step) error {
	u := step.OsUser
	if u == nil {
		return fmt.Errorf("os.user requires configuration")
	}
	if strings.TrimSpace(u.Name) == "" {
		return fmt.Errorf("os.user: name is required")
	}
	state := normalizeState(u.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("os.user: state must be present or absent, got %q", u.State)
	}
	if u.GID != nil && u.Group != "" {
		return fmt.Errorf("os.user: gid and group are mutually exclusive")
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	u := step.OsUser
	result := executor.NewResult()
	result.Checkable = true

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("os.user: only Linux is supported in this release; got %s", runtime.GOOS)
	}

	desired, err := renderDesired(ctx, u)
	if err != nil {
		return result, err
	}

	current, err := lookupUser(desired.name)
	if err != nil {
		return result, fmt.Errorf("os.user: lookup %s: %w", desired.name, err)
	}

	plan := computePlan(current, desired)
	result.Data = map[string]interface{}{
		"name":      desired.name,
		"operation": plan.operation,
		"changes":   plan.changes,
	}

	if !plan.changed {
		result.Reason = plan.reason
		return result, nil
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = plan.reason
		return result, nil
	}

	if err := applyPlan(plan, desired); err != nil {
		return result, err
	}
	result.Changed = true
	result.Reason = plan.reason
	ctx.GetLogger().Infof("  os.user: %s (%s)", desired.name, plan.operation)
	return result, nil
}

// desired captures the post-template view of the configured fields.
type desired struct {
	state        string
	name         string
	uid          *int
	gid          *int
	group        string
	shell        string
	home         string
	createHome   bool
	groups       []string
	appendGroups bool
	comment      string
	system       bool
	removeHome   bool
}

func renderDesired(ctx actions.Context, u *config.OsUser) (desired, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()

	render := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		return tmpl.Render(s, vars)
	}
	name, err := render(u.Name)
	if err != nil {
		return desired{}, fmt.Errorf("os.user: render name: %w", err)
	}
	shell, err := render(u.Shell)
	if err != nil {
		return desired{}, fmt.Errorf("os.user: render shell: %w", err)
	}
	home, err := render(u.Home)
	if err != nil {
		return desired{}, fmt.Errorf("os.user: render home: %w", err)
	}
	group, err := render(u.Group)
	if err != nil {
		return desired{}, fmt.Errorf("os.user: render group: %w", err)
	}
	comment, err := render(u.Comment)
	if err != nil {
		return desired{}, fmt.Errorf("os.user: render comment: %w", err)
	}
	groups := make([]string, 0, len(u.Groups))
	for _, g := range u.Groups {
		r, err := render(g)
		if err != nil {
			return desired{}, fmt.Errorf("os.user: render groups: %w", err)
		}
		groups = append(groups, r)
	}

	createHome := !u.System // default: create home for normal users, not for system
	if u.CreateHome != nil {
		createHome = *u.CreateHome
	}
	appendGroups := true
	if u.AppendGroups != nil {
		appendGroups = *u.AppendGroups
	}

	return desired{
		state:        normalizeState(u.State),
		name:         name,
		uid:          u.UID,
		gid:          u.GID,
		group:        group,
		shell:        shell,
		home:         home,
		createHome:   createHome,
		groups:       groups,
		appendGroups: appendGroups,
		comment:      comment,
		system:       u.System,
		removeHome:   u.RemoveHome,
	}, nil
}

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

// userState describes what's currently on the system for a given user.
// nil userState means the user doesn't exist.
type userState struct {
	exists  bool
	uid     int
	gid     int
	shell   string
	home    string
	comment string
	groups  []string // supplementary, names, sorted
}

// lookupUser is the package-level hook used to read current state.
// Real impl shells out to getent; tests override this to stub.
var lookupUser = lookupUserViaGetent

func lookupUserViaGetent(name string) (*userState, error) {
	out, err := capture(exec.Command("getent", "passwd", name))
	if err != nil {
		// `getent` returns exit code 2 when the entry doesn't exist.
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

	// Supplementary groups via `id -nG`.
	gout, err := capture(exec.Command("id", "-nG", name))
	if err != nil {
		return nil, fmt.Errorf("id -nG: %w", err)
	}
	allGroups := strings.Fields(strings.TrimSpace(gout))
	// First entry is the primary group; the rest are supplementary.
	if len(allGroups) > 1 {
		state.groups = allGroups[1:]
		sort.Strings(state.groups)
	}
	return state, nil
}

// computedPlan describes what changes are needed to reconcile current
// state with desired state.
type computedPlan struct {
	changed   bool
	operation string // create | modify | remove | noop
	reason    string
	changes   []string // human-readable change list

	// Pre-computed argument lists for apply.
	createArgs []string
	modifyArgs []string
	removeArgs []string
}

func computePlan(current *userState, d desired) computedPlan {
	if d.state == stateAbsent {
		return planAbsent(current, d)
	}
	return planPresent(current, d)
}

func planPresent(current *userState, d desired) computedPlan {
	if current == nil || !current.exists {
		args := []string{}
		if d.system {
			args = append(args, "--system")
		}
		if d.uid != nil {
			args = append(args, "--uid", strconv.Itoa(*d.uid))
		}
		if d.gid != nil {
			args = append(args, "--gid", strconv.Itoa(*d.gid))
		} else if d.group != "" {
			args = append(args, "--gid", d.group)
		}
		if d.shell != "" {
			args = append(args, "--shell", d.shell)
		}
		if d.home != "" {
			args = append(args, "--home-dir", d.home)
		}
		if d.createHome {
			args = append(args, "--create-home")
		} else {
			args = append(args, "--no-create-home")
		}
		if len(d.groups) > 0 {
			args = append(args, "--groups", strings.Join(d.groups, ","))
		}
		if d.comment != "" {
			args = append(args, "--comment", d.comment)
		}
		args = append(args, d.name)
		return computedPlan{
			changed:    true,
			operation:  "create",
			reason:     "would create user " + d.name,
			changes:    []string{"create"},
			createArgs: args,
		}
	}

	args := []string{}
	changes := []string{}
	if d.uid != nil && *d.uid != current.uid {
		args = append(args, "--uid", strconv.Itoa(*d.uid))
		changes = append(changes, fmt.Sprintf("uid: %d → %d", current.uid, *d.uid))
	}
	if d.gid != nil && *d.gid != current.gid {
		args = append(args, "--gid", strconv.Itoa(*d.gid))
		changes = append(changes, fmt.Sprintf("gid: %d → %d", current.gid, *d.gid))
	}
	if d.shell != "" && d.shell != current.shell {
		args = append(args, "--shell", d.shell)
		changes = append(changes, fmt.Sprintf("shell: %s → %s", current.shell, d.shell))
	}
	if d.home != "" && d.home != current.home {
		args = append(args, "--home", d.home, "--move-home")
		changes = append(changes, fmt.Sprintf("home: %s → %s", current.home, d.home))
	}
	if d.comment != "" && d.comment != current.comment {
		args = append(args, "--comment", d.comment)
		changes = append(changes, "comment updated")
	}
	if len(d.groups) > 0 {
		want := append([]string(nil), d.groups...)
		sort.Strings(want)
		if d.appendGroups {
			// Drift if any desired group is missing from current.
			have := stringSet(current.groups)
			missing := []string{}
			for _, g := range want {
				if !have[g] {
					missing = append(missing, g)
				}
			}
			if len(missing) > 0 {
				args = append(args, "--append", "--groups", strings.Join(d.groups, ","))
				changes = append(changes, fmt.Sprintf("groups +: %s", strings.Join(missing, ",")))
			}
		} else if !sliceEqual(current.groups, want) {
			args = append(args, "--groups", strings.Join(d.groups, ","))
			changes = append(changes, fmt.Sprintf("groups: [%s] → [%s]", strings.Join(current.groups, ","), strings.Join(want, ",")))
		}
	}

	if len(args) == 0 {
		return computedPlan{operation: "noop", reason: "user already at desired state"}
	}
	args = append(args, d.name)
	return computedPlan{
		changed:    true,
		operation:  "modify",
		reason:     fmt.Sprintf("would modify user %s (%s)", d.name, strings.Join(changes, ", ")),
		changes:    changes,
		modifyArgs: args,
	}
}

func planAbsent(current *userState, d desired) computedPlan {
	if current == nil || !current.exists {
		return computedPlan{operation: "noop", reason: "user already absent"}
	}
	args := []string{}
	if d.removeHome {
		args = append(args, "--remove")
	}
	args = append(args, d.name)
	return computedPlan{
		changed:    true,
		operation:  "remove",
		reason:     fmt.Sprintf("would remove user %s", d.name),
		changes:    []string{"remove"},
		removeArgs: args,
	}
}

func applyPlan(plan computedPlan, _ desired) error {
	switch plan.operation {
	case "create":
		return run("useradd", plan.createArgs...)
	case "modify":
		return run("usermod", plan.modifyArgs...)
	case "remove":
		return run("userdel", plan.removeArgs...)
	}
	return nil
}

func run(bin string, args ...string) error {
	// #nosec G204 -- bin is one of useradd/usermod/userdel; args are validated above.
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

func capture(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}

func stringSet(in []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range in {
		out[s] = true
	}
	return out
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
