// Package os_group implements the os.group action: declarative Unix
// group management. The platform-agnostic plan (renderDesired →
// lookupGroup → computePlan → applyPlan) lives here; per-OS lookup +
// apply functions live in platform_<goos>.go files behind build tags
// and register themselves into the package-level lookupGroup /
// applyPlan vars via init(). Refuses GID renumbering (would silently
// change on-disk ownership) and refuses to remove a group that still
// has members.
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
	"github.com/alehatsman/mooncake/internal/security"
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
		SupportedPlatforms: []string{"linux", "darwin"},
		RequiresSudo:       true,
		ImplementsCheck:    true,
	}
}

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// os.group always declares Sudo=true: it edits the system directory
// (groupadd/groupdel writes /etc/group on Linux; dscl . writes the
// local OpenDirectory node on macOS — both need root). No Network.
// RequiredBinaries vary by host so spec-44 doctor reports useful
// findings on each platform.
func (Handler) Permissions(_ *config.Step) actions.PermissionSet {
	if runtime.GOOS == "darwin" {
		return actions.PermissionSet{
			Sudo:             true,
			RequiredBinaries: []string{"dscl"},
			// macOS's directory store isn't a file — leave
			// FilesystemWrite empty rather than misrepresent it.
		}
	}
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

// RunRaw signals spec-69 RawRunner participation so user-declared
// `retry:` actually retries this idempotent action via the
// centralized executor loop instead of being silently no-op'd.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	g := step.OsGroup
	result := executor.NewResult()
	result.Checkable = true

	// Spec-69 phase 5: runner is per-Run, threaded into applyPlan
	// (which uses it for groupadd/groupmod/groupdel on linux and
	// dscl writes on darwin).
	runner := ctx.Privileged()

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

	if err := applyPlan(runner, plan); err != nil {
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

// lookupGroup and applyPlan are package-level hooks set by the
// platform-specific init() in platform_linux.go / platform_darwin.go.
// Tests override these to inject deterministic state without shelling
// out. On unsupported GOOS, the default implementations return a
// clear error rather than a panic.
var lookupGroup func(string) (*groupState, error) = func(string) (*groupState, error) {
	return nil, fmt.Errorf("os.group: not implemented on %s", runtime.GOOS)
}

var applyPlan = func(runner *security.Privileged, plan computedPlan) error {
	_ = runner
	_ = plan
	return fmt.Errorf("os.group: not implemented on %s", runtime.GOOS)
}

// computedPlan describes the diff between current and desired.
type computedPlan struct {
	changed   bool
	operation string // create | modify | remove | noop
	reason    string

	createArgs []string
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

// capture runs cmd and returns its stdout. Shared helper for the
// platform-specific lookup/apply paths; lives here (not behind a build
// tag) so tests on any host can construct the type-level mock.
func capture(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
