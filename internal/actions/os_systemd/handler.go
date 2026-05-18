// Package os_systemd implements the os.systemd action: write a unit
// file, optionally daemon-reload, enable/disable and start/stop the
// unit. The action is keyed by unit `name` (e.g. "myapp.service" or
// "backup.timer"); idempotency is byte-identical file content plus
// observed enable/active state. Linux-only; non-Linux platforms get a
// structured error.
//
//nolint:revive // Package name matches action name convention (os_systemd)
package os_systemd

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

// becomeRunner is the sudo runner used by writeAtomic + runSystemctl
// helpers. Run() sets it from ec.Svc.SudoPass before applyPlan; tests
// that stub the systemctl* hooks bypass this entirely. Package-level
// state because the existing systemctl* hook plumbing already lives at
// package scope and mooncake executes actions serially.
//
// Spec-69 phase-5 audit (NOT migrated to ctx.Privileged): this handler
// keeps BecomeRunner directly because it needs the broader Command()
// API in two places — writeAtomic captures sudo cp + sudo chmod
// separately (each with its own combined-output capture so a chmod
// failure surfaces distinctly from a cp failure), and runSystemctl
// uses the conditional `systemctlBecome()` predicate to skip sudo
// when mooncake is already root. PrivilegedRunner.Run is the "I need
// root, unconditionally" common path and doesn't expose either knob.
// See internal/actions/privileged.go for the design rationale.
var becomeRunner security.BecomeRunner

const (
	actionName       = "os.systemd"
	statePresent     = "present"
	stateAbsent      = "absent"
	atomicTempSuffix = ".mooncake-tmp"
	managedHeader    = "# Managed by mooncake os.systemd"
)

// systemdPaths controls where unit files are written. Tests override
// the directory to a tempdir to keep apply hermetic.
var systemdPaths = struct {
	dir string
}{
	dir: "/etc/systemd/system",
}

// Hooks for the systemctl primitives. Tests replace these with
// in-memory stubs.
var (
	systemctlDaemonReload = realDaemonReload
	systemctlIsEnabled    = realIsEnabled
	systemctlEnable       = realEnable
	systemctlDisable      = realDisable
	systemctlIsActive     = realIsActive
	systemctlStart        = realStart
	systemctlStop         = realStop
)

// Handler implements os.systemd.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("OsSystemdReverseInfo", func() any { return &OsSystemdReverseInfo{} })
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage a systemd unit file with daemon-reload, enable, start",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		EmitsEvents:        []string{string(events.EventFileUpdated)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux"},
		RequiresSudo:       true,
		ImplementsCheck:    true,
	}
}

// nameRE validates the unit filename. Same alphabet systemd allows for
// unit names; the suffix (.service, .timer, ...) is enforced separately.
var nameRE = regexp.MustCompile(`^[a-zA-Z0-9@._:\\-]+\.[a-zA-Z]+$`)

// validUnitSuffixes is the set of unit types os.systemd v1 supports.
// Other types (.path, .device, .swap, .scope, ...) can author through
// `file.write` until a real demand surfaces.
var validUnitSuffixes = map[string]bool{
	".service": true,
	".timer":   true,
	".socket":  true,
	".target":  true,
	".mount":   true,
}

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// os.systemd writes unit files to /etc/systemd/system (default) or
// step.Path, and shells to systemctl for daemon-reload / enable /
// start. Always Sudo, RequiredBinaries=[systemctl]. The unit
// content writes are scoped to the unit path; FilesystemWrite
// surfaces it for the policy layer.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	ps := actions.PermissionSet{
		Sudo:             true,
		RequiredBinaries: []string{"systemctl"},
	}
	if step == nil || step.OsSystemd == nil {
		return ps
	}
	dir := step.OsSystemd.Path
	if dir == "" {
		dir = "/etc/systemd/system"
	}
	if step.OsSystemd.Name != "" {
		ps.FilesystemWrite = []string{dir + "/" + step.OsSystemd.Name}
	}
	return ps
}

func (h *Handler) Validate(step *config.Step) error {
	s := step.OsSystemd
	if s == nil {
		return fmt.Errorf("os.systemd requires configuration")
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("os.systemd: name is required")
	}
	if !nameRE.MatchString(s.Name) {
		return fmt.Errorf("os.systemd: name %q must be a unit filename with a suffix (e.g. myapp.service)", s.Name)
	}
	suffix := filepath.Ext(s.Name)
	if !validUnitSuffixes[suffix] {
		return fmt.Errorf("os.systemd: unsupported unit suffix %q (supported: .service .timer .socket .target .mount)", suffix)
	}
	state := normalizeState(s.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("os.systemd: state must be present or absent, got %q", s.State)
	}
	if state == statePresent {
		// Require at least one section so we're not writing an empty file.
		if len(s.Unit) == 0 && len(s.Service) == 0 && len(s.Timer) == 0 && len(s.Socket) == 0 && len(s.Install) == 0 {
			return fmt.Errorf("os.systemd: at least one of unit/service/timer/socket/install must be set when state=present")
		}
		// Best-effort coherence: a .service should usually have [Service],
		// a .timer should have [Timer]. We warn via error rather than
		// silently accept inconsistent definitions.
		switch suffix {
		case ".service":
			if len(s.Timer) > 0 {
				return fmt.Errorf("os.systemd: timer section is not valid for %s", s.Name)
			}
		case ".timer":
			if len(s.Service) > 0 {
				return fmt.Errorf("os.systemd: service section is not valid for %s", s.Name)
			}
		}
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
	s := step.OsSystemd
	result := executor.NewResult()
	result.Checkable = true

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("os.systemd: only Linux is supported; got %s", runtime.GOOS)
	}

	// Pick up the operator-supplied sudo password so writeAtomic +
	// runSystemctl can escalate when mooncake is invoked as a regular
	// user. Empty SudoPass surfaces as a clean ErrBecomeNoSudoPass at
	// the first sudo'd call rather than a "permission denied" on
	// /etc/systemd/system. Tests that stub the systemctl* hooks never
	// reach this code path.
	if ec, ok := ctx.(*executor.ExecutionContext); ok {
		becomeRunner = security.BecomeRunner{SudoPass: ec.Svc.SudoPass, PasswordlessSudo: ec.Svc.PasswordlessSudo}
	}

	rendered, err := renderSystemd(ctx, s)
	if err != nil {
		return result, err
	}

	plan, err := computePlan(rendered)
	if err != nil {
		return result, err
	}

	result.Data = map[string]interface{}{
		"name":      rendered.name,
		"path":      plan.path,
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
	// computePlan already read the file content and queried
	// systemctl is-enabled / is-active where relevant; we re-read
	// here to capture all three pieces, even ones the plan didn't
	// look at (e.g. if Enabled/Started weren't pinned the plan
	// skipped the systemctl calls).
	result.ReverseData = captureReverseInfo(rendered.name, plan.path)

	if err := applyPlan(plan); err != nil {
		return result, err
	}

	result.Changed = true
	result.Reason = plan.reason
	ctx.GetLogger().Infof("  os.systemd: %s (%s)", rendered.name, plan.operation)

	if plan.fileChanged {
		if pub := ctx.GetEventPublisher(); pub != nil {
			pub.Publish(events.Event{
				Type: events.EventFileUpdated,
				Data: events.FileOperationData{Path: plan.path, Changed: true},
			})
		}
	}
	return result, nil
}

// renderedSystemd is the post-template, defaults-applied view passed
// to the planner.
type renderedSystemd struct {
	name           string
	state          string
	path           string
	content        string // empty when state=absent
	enabled        *bool
	started        *bool
	reloadOnChange bool
}

func renderSystemd(ctx actions.Context, s *config.OsSystemd) (renderedSystemd, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()

	render := func(in string) (string, error) {
		if in == "" {
			return "", nil
		}
		return tmpl.Render(in, vars)
	}

	name, err := render(s.Name)
	if err != nil {
		return renderedSystemd{}, fmt.Errorf("os.systemd: render name: %w", err)
	}
	if name == "" {
		name = s.Name
	}

	dir := systemdPaths.dir
	if s.Path != "" {
		rdir, err := render(s.Path)
		if err != nil {
			return renderedSystemd{}, fmt.Errorf("os.systemd: render path: %w", err)
		}
		dir = rdir
	}

	state := normalizeState(s.State)

	reload := true
	if s.ReloadOnChange != nil {
		reload = *s.ReloadOnChange
	}

	r := renderedSystemd{
		name:           name,
		state:          state,
		path:           filepath.Join(dir, name),
		enabled:        s.Enabled,
		started:        s.Started,
		reloadOnChange: reload,
	}

	if state == stateAbsent {
		return r, nil
	}

	content, err := renderUnitFile(render, s)
	if err != nil {
		return renderedSystemd{}, err
	}
	r.content = content
	return r, nil
}

// renderUnitFile emits canonical INI-style systemd unit content with a
// managed-by header. Section order is fixed (Unit, Service, Timer,
// Socket, Install) so two equivalent inputs produce byte-identical
// output; key order within each section is the YAML iteration order
// sorted alphabetically to keep diffs stable.
func renderUnitFile(render func(string) (string, error), s *config.OsSystemd) (string, error) {
	var sb strings.Builder
	sb.WriteString(managedHeader)
	sb.WriteByte('\n')

	sections := []struct {
		header string
		body   map[string]interface{}
	}{
		{"[Unit]", s.Unit},
		{"[Service]", s.Service},
		{"[Timer]", s.Timer},
		{"[Socket]", s.Socket},
		{"[Install]", s.Install},
	}

	first := true
	for _, sec := range sections {
		if len(sec.body) == 0 {
			continue
		}
		if !first {
			sb.WriteByte('\n')
		}
		first = false

		sb.WriteString(sec.header)
		sb.WriteByte('\n')

		keys := make([]string, 0, len(sec.body))
		for k := range sec.body {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			lines, err := renderValue(render, sec.body[k])
			if err != nil {
				return "", fmt.Errorf("os.systemd: render %s/%s: %w", sec.header, k, err)
			}
			for _, ln := range lines {
				sb.WriteString(k)
				sb.WriteByte('=')
				sb.WriteString(ln)
				sb.WriteByte('\n')
			}
		}
	}
	return sb.String(), nil
}

// renderValue normalizes a section value to one or more rendered strings.
// Scalars produce a single line; lists produce one line per element.
// Nil produces an empty line list (the key is skipped).
func renderValue(render func(string) (string, error), v interface{}) ([]string, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case string:
		out, err := render(t)
		if err != nil {
			return nil, err
		}
		return []string{out}, nil
	case bool:
		if t {
			return []string{"true"}, nil
		}
		return []string{"false"}, nil
	case int:
		return []string{fmt.Sprintf("%d", t)}, nil
	case int64:
		return []string{fmt.Sprintf("%d", t)}, nil
	case float64:
		if t == float64(int64(t)) {
			return []string{fmt.Sprintf("%d", int64(t))}, nil
		}
		return []string{fmt.Sprintf("%g", t)}, nil
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			rendered, err := renderValue(render, item)
			if err != nil {
				return nil, err
			}
			out = append(out, rendered...)
		}
		return out, nil
	case []string:
		out := make([]string, 0, len(t))
		for _, item := range t {
			rendered, err := render(item)
			if err != nil {
				return nil, err
			}
			out = append(out, rendered)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported value type %T", v)
	}
}

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

// systemdPlan captures the diff between desired and current state plus
// the side-effects (daemon-reload, enable/disable, start/stop) needed
// to converge.
type systemdPlan struct {
	changed     bool
	operation   string // create|update|delete|noop
	reason      string
	path        string
	wantContent string
	fileOp      fileOperation
	fileChanged bool // whether the unit file content needs writing/removing
	reload      bool
	enableOp    enableOp
	startOp     startOp
	name        string
}

type fileOperation int

const (
	fileNoop fileOperation = iota
	fileWrite
	fileRemove
)

type enableOp int

const (
	enableNoop enableOp = iota
	enableSet
	enableUnset
)

type startOp int

const (
	startNoop startOp = iota
	startSet
	startUnset
)

func computePlan(r renderedSystemd) (systemdPlan, error) {
	plan := systemdPlan{path: r.path, name: r.name}

	current, exists, err := readFile(r.path)
	if err != nil {
		return plan, err
	}

	reasons := []string{}

	if r.state == stateAbsent {
		if exists {
			plan.fileOp = fileRemove
			plan.fileChanged = true
			plan.reload = r.reloadOnChange
			reasons = append(reasons, "would remove "+r.path)
		}
		// Stop / disable only when systemctl observes the unit as
		// active / enabled. Errors from systemctl (e.g. "unit not
		// found") are treated as "not in that state" — there's nothing
		// to act on.
		if active, err := systemctlIsActive(r.name); err == nil && active {
			plan.startOp = startUnset
			reasons = append(reasons, "would stop "+r.name)
		}
		if enabled, err := systemctlIsEnabled(r.name); err == nil && enabled {
			plan.enableOp = enableUnset
			reasons = append(reasons, "would disable "+r.name)
		}
		if len(reasons) == 0 {
			plan.operation = "noop"
			plan.reason = "unit already absent"
			return plan, nil
		}
		plan.changed = true
		plan.operation = "delete"
		plan.reason = strings.Join(reasons, "; ")
		return plan, nil
	}

	// state=present.
	plan.wantContent = r.content
	switch {
	case !exists:
		plan.fileOp = fileWrite
		plan.fileChanged = true
		plan.reload = r.reloadOnChange
		reasons = append(reasons, "would create "+r.path)
	case current != r.content:
		plan.fileOp = fileWrite
		plan.fileChanged = true
		plan.reload = r.reloadOnChange
		reasons = append(reasons, "would update "+r.path+" (content drift)")
	}

	if r.enabled != nil {
		isEnabled, err := systemctlIsEnabled(r.name)
		if err == nil {
			if *r.enabled && !isEnabled {
				plan.enableOp = enableSet
				reasons = append(reasons, "would enable "+r.name)
			} else if !*r.enabled && isEnabled {
				plan.enableOp = enableUnset
				reasons = append(reasons, "would disable "+r.name)
			}
		} else if !exists {
			// New unit file — can't observe enabled state yet; assume
			// not enabled and act on the desired flag.
			if *r.enabled {
				plan.enableOp = enableSet
				reasons = append(reasons, "would enable "+r.name)
			}
		}
	}

	if r.started != nil {
		isActive, err := systemctlIsActive(r.name)
		if err == nil {
			if *r.started && !isActive {
				plan.startOp = startSet
				reasons = append(reasons, "would start "+r.name)
			} else if !*r.started && isActive {
				plan.startOp = startUnset
				reasons = append(reasons, "would stop "+r.name)
			}
		} else if !exists {
			if *r.started {
				plan.startOp = startSet
				reasons = append(reasons, "would start "+r.name)
			}
		}
	}

	// If the file content changed but the unit was already at desired
	// enable/active state, the change still triggers a restart so the
	// new content takes effect. Spec: "wrote unit but didn't reload"
	// is the bug class this action exists to prevent.
	if plan.fileChanged && r.started != nil && *r.started && plan.startOp == startNoop {
		plan.startOp = startSet
		reasons = append(reasons, "would restart "+r.name+" after unit content change")
	}

	if len(reasons) == 0 {
		plan.operation = "noop"
		plan.reason = "unit already at desired state"
		return plan, nil
	}

	plan.changed = true
	switch {
	case !exists:
		plan.operation = "create"
	case plan.fileChanged:
		plan.operation = "update"
	default:
		plan.operation = "update"
	}
	plan.reason = strings.Join(reasons, "; ")
	return plan, nil
}

func applyPlan(plan systemdPlan) error {
	// Order matters: stop (if absent) → write/remove file → daemon-reload
	// → enable/disable → start. This sequence avoids the dead window where
	// a service has been removed from disk but is still active.

	if plan.operation == "delete" {
		if plan.startOp == startUnset {
			if active, err := systemctlIsActive(plan.name); err == nil && active {
				if err := systemctlStop(plan.name); err != nil {
					return fmt.Errorf("os.systemd: stop %s: %w", plan.name, err)
				}
			}
		}
		if plan.enableOp == enableUnset {
			if enabled, err := systemctlIsEnabled(plan.name); err == nil && enabled {
				if err := systemctlDisable(plan.name); err != nil {
					return fmt.Errorf("os.systemd: disable %s: %w", plan.name, err)
				}
			}
		}
		if plan.fileOp == fileRemove {
			if err := os.Remove(plan.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("os.systemd: remove %s: %w", plan.path, err)
			}
		}
		if plan.reload {
			if err := systemctlDaemonReload(); err != nil {
				return fmt.Errorf("os.systemd: daemon-reload: %w", err)
			}
		}
		return nil
	}

	if plan.fileOp == fileWrite {
		if err := os.MkdirAll(filepath.Dir(plan.path), 0o755); err != nil {
			return fmt.Errorf("os.systemd: mkdir %s: %w", filepath.Dir(plan.path), err)
		}
		if err := writeAtomic(plan.path, []byte(plan.wantContent), 0o644); err != nil {
			return fmt.Errorf("os.systemd: write %s: %w", plan.path, err)
		}
	}
	if plan.reload {
		if err := systemctlDaemonReload(); err != nil {
			return fmt.Errorf("os.systemd: daemon-reload: %w", err)
		}
	}
	switch plan.enableOp {
	case enableSet:
		if err := systemctlEnable(plan.name); err != nil {
			return fmt.Errorf("os.systemd: enable %s: %w", plan.name, err)
		}
	case enableUnset:
		if err := systemctlDisable(plan.name); err != nil {
			return fmt.Errorf("os.systemd: disable %s: %w", plan.name, err)
		}
	}
	switch plan.startOp {
	case startSet:
		if err := systemctlStart(plan.name); err != nil {
			return fmt.Errorf("os.systemd: start %s: %w", plan.name, err)
		}
	case startUnset:
		if err := systemctlStop(plan.name); err != nil {
			return fmt.Errorf("os.systemd: stop %s: %w", plan.name, err)
		}
	}
	return nil
}

func readFile(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), true, nil
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	// Fast path: direct write — works when mooncake itself runs as root,
	// or the unit path happens to be user-writable (custom Path: in
	// step config). The sudo fallback below kicks in only on
	// EACCES/EPERM against the default /etc/systemd/system.
	tmp := path + atomicTempSuffix
	if err := os.WriteFile(tmp, content, mode); err == nil {
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return nil
	} else if !os.IsPermission(err) {
		return err
	}

	// Sudo fallback: stage in /tmp under the user, then `sudo cp`
	// (atomic enough — same FS, but Rename doesn't help when the user
	// can't write to the destination dir) and `sudo chmod`. Mirrors
	// the pattern in internal/actions/service/handler.go:594.
	tmpFile, err := os.CreateTemp("", "os-systemd-unit-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w", tmpPath, err)
	}

	cmd, err := becomeRunner.Command(true, "cp", tmpPath, path)
	if err != nil {
		return fmt.Errorf("sudo cp setup: %w", err)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sudo cp %s -> %s: %w (%s)", tmpPath, path, err, strings.TrimSpace(string(out)))
	}

	cmd, err = becomeRunner.Command(true, "chmod", fmt.Sprintf("%o", mode), path)
	if err != nil {
		return fmt.Errorf("sudo chmod setup: %w", err)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sudo chmod %o %s: %w (%s)", mode, path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// realDaemonReload shells out to `systemctl daemon-reload`.
func realDaemonReload() error {
	return runSystemctl("daemon-reload")
}

// realIsEnabled returns true when systemctl reports the unit enabled.
// "static", "indirect", "alias" and "linked" are treated as enabled —
// they ship a [Install] section equivalent. "disabled", "masked",
// "generated", "transient", "bad" → false. Errors propagate.
func realIsEnabled(name string) (bool, error) {
	out, err := runSystemctlOut("is-enabled", name)
	state := strings.TrimSpace(out)
	if err != nil {
		// systemctl is-enabled exits 1 for disabled; map that to (false, nil)
		// when we still got a recognisable output token.
		if state != "" {
			return enableFromToken(state), nil
		}
		return false, err
	}
	return enableFromToken(state), nil
}

func enableFromToken(tok string) bool {
	switch tok {
	case "enabled", "enabled-runtime", "static", "indirect", "alias", "linked", "linked-runtime":
		return true
	default:
		return false
	}
}

func realEnable(name string) error  { return runSystemctl("enable", name) }
func realDisable(name string) error { return runSystemctl("disable", name) }

// realIsActive returns true when the unit's ActiveState is "active".
// "reloading" and "activating" also count as active so we don't fight
// in-progress transitions. Exit code 3 from `is-active` means inactive
// — that's not an error.
func realIsActive(name string) (bool, error) {
	out, err := runSystemctlOut("is-active", name)
	state := strings.TrimSpace(out)
	if err != nil {
		if state != "" {
			return activeFromToken(state), nil
		}
		return false, err
	}
	return activeFromToken(state), nil
}

func activeFromToken(tok string) bool {
	switch tok {
	case "active", "reloading", "activating":
		return true
	default:
		return false
	}
}

func realStart(name string) error { return runSystemctl("start", name) }
func realStop(name string) error  { return runSystemctl("stop", name) }

// systemctlBecome decides whether systemctl calls run under sudo. True
// when mooncake is invoked as a non-root user (the common case for the
// system-scope unit path /etc/systemd/system). Skipping sudo when
// already root keeps the call path simple and avoids requiring a
// SudoPass that's never used. The is-enabled/is-active read paths
// don't strictly need root on most distros, but a sudo wrap is
// harmless and keeps every systemctl call going through one helper.
func systemctlBecome() bool { return os.Geteuid() != 0 }

func runSystemctl(args ...string) error {
	cmd, err := becomeRunner.Command(systemctlBecome(), "systemctl", args...)
	if err != nil {
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func runSystemctlOut(args ...string) (string, error) {
	cmd, err := becomeRunner.Command(systemctlBecome(), "systemctl", args...)
	if err != nil {
		return "", fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout.String(), err
}
