// Package os_group implements the os.group action: declarative Unix
// group management. Reads current state via `getent group` and applies
// drift via groupadd/groupmod/groupdel. Refuses GID renumbering (would
// silently change on-disk ownership) and refuses to remove a group
// that still has members. Linux-only; macOS/Windows deferred.
//
//nolint:revive // Package name matches action name convention (os_group)
package os_group

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName   = "os.group"
	statePresent = "present"
	stateAbsent  = "absent"
)

// Handler implements os.group.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("OsGroupReverseInfo", func() any { return &OsGroupReverseInfo{} })
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Declaratively manage a Unix group",
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
// os.group always declares Sudo=true: groupadd/groupdel writes
// /etc/group. No Network. RequiredBinaries=[groupadd, groupdel]
// (groupmod isn't used today — the kernel refuses gid renumbering
// to avoid silent ownership cascades).
func (Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		Sudo:             true,
		RequiredBinaries: []string{"groupadd", "groupdel"},
		FilesystemWrite:  []string{"/etc/group"},
	}
}

func (h *Handler) Validate(step *config.Step) error {
	g := step.OsGroup
	if g == nil {
		return fmt.Errorf("os.group requires configuration")
	}
	if strings.TrimSpace(g.Name) == "" {
		return fmt.Errorf("os.group: name is required")
	}
	state := normalizeState(g.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("os.group: state must be present or absent, got %q", g.State)
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	g := step.OsGroup
	result := executor.NewResult()
	result.Checkable = true

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("os.group: only Linux is supported in this release; got %s", runtime.GOOS)
	}

	desired, err := renderDesired(ctx, g)
	if err != nil {
		return result, err
	}

	current, err := lookupGroup(desired.name)
	if err != nil {
		return result, fmt.Errorf("os.group: lookup %s: %w", desired.name, err)
	}

	plan, err := computePlan(current, desired)
	if err != nil {
		return result, err
	}

	result.Data = map[string]interface{}{
		"name":      desired.name,
		"operation": plan.operation,
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

	// Capture pre-apply state for Reverse() BEFORE applyPlan.
	priorExisted := current != nil && current.exists
	info := &OsGroupReverseInfo{
		Name:         desired.name,
		AppliedState: desired.state,
		PriorExisted: priorExisted,
	}
	if priorExisted {
		info.PriorGID = current.gid
	}
	result.ReverseData = info

	if err := applyPlan(plan); err != nil {
		return result, err
	}

	result.Changed = true
	result.Reason = plan.reason
	ctx.GetLogger().Infof("  os.group: %s (%s)", desired.name, plan.operation)
	return result, nil
}

// desired captures the post-template view.
type desired struct {
	state  string
	name   string
	gid    *int
	system bool
}

func renderDesired(ctx actions.Context, g *config.OsGroup) (desired, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()
	name, err := tmpl.Render(g.Name, vars)
	if err != nil {
		return desired{}, fmt.Errorf("os.group: render name: %w", err)
	}
	return desired{
		state:  normalizeState(g.State),
		name:   name,
		gid:    g.GID,
		system: g.System,
	}, nil
}

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

// groupState describes what's currently on the system. exists=false
// means the group is not present.
type groupState struct {
	exists  bool
	gid     int
	members []string
}

// lookupGroup is the package-level hook for current state. Real impl
// shells out to `getent group`; tests override this.
var lookupGroup = lookupGroupViaGetent

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

// computedPlan describes the diff between current and desired.
type computedPlan struct {
	changed   bool
	operation string // create | modify | remove | noop
	reason    string

	createArgs []string
	modifyArgs []string
	removeArgs []string
	name       string
}

func computePlan(current *groupState, d desired) (computedPlan, error) {
	if d.state == stateAbsent {
		return planAbsent(current, d)
	}
	return planPresent(current, d)
}

func planPresent(current *groupState, d desired) (computedPlan, error) {
	if current == nil || !current.exists {
		args := []string{}
		if d.system {
			args = append(args, "--system")
		}
		if d.gid != nil {
			args = append(args, "--gid", strconv.Itoa(*d.gid))
		}
		args = append(args, d.name)
		return computedPlan{
			changed:    true,
			operation:  "create",
			reason:     "would create group " + d.name,
			createArgs: args,
			name:       d.name,
		}, nil
	}

	if d.gid != nil && *d.gid != current.gid {
		// Refuse to renumber: changing GID silently breaks file ownership
		// on disk. Force the user to be explicit (delete + recreate, or
		// fix the spec).
		return computedPlan{}, fmt.Errorf(
			"os.group: refusing to renumber group %s (current gid=%d, desired gid=%d); delete-and-recreate explicitly if required",
			d.name, current.gid, *d.gid,
		)
	}

	return computedPlan{
		operation: "noop",
		reason:    "group already at desired state",
		name:      d.name,
	}, nil
}

func planAbsent(current *groupState, d desired) (computedPlan, error) {
	if current == nil || !current.exists {
		return computedPlan{operation: "noop", reason: "group already absent", name: d.name}, nil
	}
	if len(current.members) > 0 {
		return computedPlan{}, fmt.Errorf(
			"os.group: refusing to remove group %s with members [%s]; remove members first",
			d.name, strings.Join(current.members, ","),
		)
	}
	return computedPlan{
		changed:    true,
		operation:  "remove",
		reason:     "would remove group " + d.name,
		removeArgs: []string{d.name},
		name:       d.name,
	}, nil
}

func applyPlan(plan computedPlan) error {
	switch plan.operation {
	case "create":
		return run("groupadd", plan.createArgs...)
	case "modify":
		return run("groupmod", plan.modifyArgs...)
	case "remove":
		return run("groupdel", plan.removeArgs...)
	}
	return nil
}

func run(bin string, args ...string) error {
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

func capture(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
