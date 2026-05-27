// Package os_sysctl implements the os.sysctl action: declarative
// management of a single Linux kernel parameter. v1 manages one
// shared persist file (/etc/sysctl.d/99-mooncake.conf), keyed by
// name; runtime apply via `sysctl key=value` is opt-in (default on).
//
//nolint:revive // Package name matches action name convention (os_sysctl)
package os_sysctl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

const (
	actionName       = "os.sysctl"
	statePresent     = "present"
	stateAbsent      = "absent"
	atomicTempSuffix = ".mooncake-tmp"
	managedHeader    = "# Managed by mooncake os.sysctl"

	// sysctlCmdTimeout bounds the `sysctl -w` shell-out. sysctl is
	// fast in the happy path (sub-ms) but blocks on some hardware
	// sysctls when the underlying driver is wedged (F051). 5s
	// matches the facts probeTimeout precedent.
	sysctlCmdTimeout = 5 * time.Second
)

// sysctlPaths controls the persist file location. Tests override it.
var sysctlPaths = struct {
	persistFile string
}{
	persistFile: "/etc/sysctl.d/99-mooncake.conf",
}

// Package-level hooks for the runtime read/apply primitives. Tests
// override these to keep apply-mode hermetic. Spec-69 phase-5 cleanup:
// sysctlApply takes an explicit PrivilegedRunner so we no longer ride
// package-level state.
var (
	sysctlGet   = realSysctlGet
	sysctlApply = realSysctlApply
)

// Handler implements os.sysctl.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("OsSysctlReverseInfo", func() any { return &OsSysctlReverseInfo{} })
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage a Linux kernel parameter (sysctl)",
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

// nameRE validates sysctl key syntax (kernel.* / net.* / vm.* / ...
// dotted, plus dashes and slashes which appear in some keys like
// net/ipv4/conf/all/arp_announce, though the dotted form is canonical).
var nameRE = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// os.sysctl always needs Sudo: writes /etc/sysctl.d/99-mooncake.conf
// when Persist is true (default), and invokes `sysctl -w` to apply
// at runtime when Reload is true (default). RequiredBinaries=[sysctl]
// (the binary is invoked for the runtime apply; the file-write path
// uses plain os.* calls).
func (Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		Sudo:             true,
		RequiredBinaries: []string{"sysctl"},
		FilesystemWrite:  []string{"/etc/sysctl.d/99-mooncake.conf"},
	}
}

func (h *Handler) Validate(step *config.Step) error {
	s := step.OsSysctl
	if s == nil {
		return fmt.Errorf("os.sysctl requires configuration")
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("os.sysctl: name is required")
	}
	if !nameRE.MatchString(s.Name) {
		return fmt.Errorf("os.sysctl: name %q must match [A-Za-z0-9._/-]+", s.Name)
	}
	state := normalizeState(s.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("os.sysctl: state must be present or absent, got %q", s.State)
	}
	if state == statePresent {
		if _, err := coerceValue(s.Value); err != nil {
			return fmt.Errorf("os.sysctl: value is required when state=present")
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
	s := step.OsSysctl
	result := executor.NewResult()
	result.Checkable = true

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("os.sysctl: only Linux is supported; got %s", runtime.GOOS)
	}

	// Spec-69 phase 5: runner + performer are per-Run, threaded into
	// applyPlan instead of riding package-level state.
	runner := ctx.Privileged()
	performer := ctx.Effects()

	rendered, err := renderSysctl(ctx, s)
	if err != nil {
		return result, err
	}

	plan, err := computePlan(rendered)
	if err != nil {
		return result, err
	}

	result.Data = map[string]interface{}{
		"name":      rendered.name,
		"operation": plan.operation,
		"path":      sysctlPaths.persistFile,
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

	// Capture pre-apply state for Reverse() BEFORE applyPlan
	// touches anything. plan already reads runtime + persist;
	// stash those values verbatim so Reverse can rebuild the prior
	// world.
	result.ReverseData = &OsSysctlReverseInfo{
		Name:               rendered.name,
		AppliedState:       rendered.state,
		PriorRuntimeValue:  plan.currentVal,
		HadPriorRuntime:    plan.currentVal != "",
		PriorPersistValue:  plan.priorLine,
		HadPriorPersist:    plan.hadLine,
		TouchedPersistFile: plan.touchesFile,
		TouchedRuntime:     plan.apply,
	}

	if err := applyPlan(performer, runner, plan, rendered); err != nil {
		return result, err
	}

	result.Changed = true
	result.Reason = plan.reason
	ctx.GetLogger().Infof("  os.sysctl: %s (%s)", rendered.name, plan.operation)

	if plan.touchesFile {
		if pub := ctx.GetEventPublisher(); pub != nil {
			pub.Publish(events.Event{
				Type: events.EventFileUpdated,
				Data: events.FileOperationData{Path: sysctlPaths.persistFile, Changed: true},
			})
		}
	}
	return result, nil
}

// renderedSysctl holds the post-template, defaults-applied view.
type renderedSysctl struct {
	name    string
	value   string
	state   string
	persist bool
	reload  bool
}

func renderSysctl(ctx actions.Context, s *config.OsSysctl) (renderedSysctl, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()

	name, err := tmpl.Render(s.Name, vars)
	if err != nil {
		return renderedSysctl{}, fmt.Errorf("os.sysctl: render name: %w", err)
	}

	value, _ := coerceValue(s.Value)
	if value != "" {
		value, err = tmpl.Render(value, vars)
		if err != nil {
			return renderedSysctl{}, fmt.Errorf("os.sysctl: render value: %w", err)
		}
	}

	persist := true
	if s.Persist != nil {
		persist = *s.Persist
	}
	reload := true
	if s.Reload != nil {
		reload = *s.Reload
	}

	return renderedSysctl{
		name:    name,
		value:   value,
		state:   normalizeState(s.State),
		persist: persist,
		reload:  reload,
	}, nil
}

// coerceValue accepts string, integer, bool, or float values from YAML
// and renders them as their canonical sysctl string form. Returns an
// error when the value is nil or an unsupported type.
func coerceValue(v interface{}) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", fmt.Errorf("value is required")
	case string:
		if strings.TrimSpace(t) == "" {
			return "", fmt.Errorf("value is required")
		}
		return t, nil
	case int:
		return fmt.Sprintf("%d", t), nil
	case int64:
		return fmt.Sprintf("%d", t), nil
	case float64:
		// YAML decodes integer literals into float64 when the type is
		// interface{} and the value happens to fit. Render as %g but
		// strip the trailing decimal for whole numbers.
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t)), nil
		}
		return fmt.Sprintf("%g", t), nil
	case bool:
		if t {
			return "1", nil
		}
		return "0", nil
	default:
		return "", fmt.Errorf("unsupported value type %T", v)
	}
}

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

// sysctlPlan describes the file mutation + runtime apply needed to
// converge on the desired state.
type sysctlPlan struct {
	changed     bool
	operation   string // create|update|delete|noop
	reason      string
	wantContent string // full file content after change; only for persist mutations
	touchesFile bool
	apply       bool // runtime sysctl apply needed?
	currentVal  string

	// priorLine + hadLine capture the persist-file state pre-apply
	// so Reverse can reconstruct it. Populated by computePlan
	// regardless of whether the apply runs.
	priorLine string
	hadLine   bool
}

func computePlan(r renderedSysctl) (sysctlPlan, error) {
	plan := sysctlPlan{}

	currentLines, fileExists, err := readPersistFile()
	if err != nil {
		return plan, err
	}
	currentLine, hasLine := findLine(currentLines, r.name)

	// Snapshot persist-file state for Reverse(). currentVal (runtime)
	// is set below in the present branch where sysctlGet runs.
	plan.priorLine = currentLine
	plan.hadLine = hasLine

	switch r.state {
	case stateAbsent:
		if !hasLine {
			plan.operation = "noop"
			plan.reason = "sysctl line already absent from persist file"
			return plan, nil
		}
		next := removeLine(currentLines, r.name)
		plan.wantContent = renderPersistFile(next)
		plan.changed = true
		plan.touchesFile = true
		plan.operation = "delete"
		plan.reason = "would remove " + r.name + " from " + sysctlPaths.persistFile
		// Removing a persisted value does not auto-revert the runtime
		// value; spec keeps that explicit (see spec-22 reverse for the
		// snapshot-and-restore semantics). No runtime apply.
		return plan, nil
	case statePresent:
		current, getErr := sysctlGet(r.name)
		if getErr == nil {
			plan.currentVal = strings.TrimSpace(current)
		}
		runtimeMatches := getErr == nil && plan.currentVal == r.value
		persistedMatches := hasLine && currentLine == r.value

		if runtimeMatches && persistedMatches {
			plan.operation = "noop"
			plan.reason = "sysctl already at desired state"
			return plan, nil
		}

		if r.persist && !persistedMatches {
			next := upsertLine(currentLines, r.name, r.value)
			plan.wantContent = renderPersistFile(next)
			plan.touchesFile = true
		}
		if r.reload && !runtimeMatches {
			plan.apply = true
		}

		if !plan.touchesFile && !plan.apply {
			// Nothing to do — persist disabled and runtime already matches.
			plan.operation = "noop"
			plan.reason = "sysctl already at desired state (persist disabled)"
			return plan, nil
		}

		plan.changed = true
		switch {
		case !fileExists && plan.touchesFile:
			plan.operation = "create"
			plan.reason = "would create " + sysctlPaths.persistFile + " with " + r.name
		case plan.touchesFile && !hasLine:
			plan.operation = "update"
			plan.reason = "would add " + r.name + " to " + sysctlPaths.persistFile
		case plan.touchesFile:
			plan.operation = "update"
			plan.reason = fmt.Sprintf("would update %s in %s (%q -> %q)", r.name, sysctlPaths.persistFile, currentLine, r.value)
		default:
			plan.operation = "update"
			plan.reason = fmt.Sprintf("would apply runtime %s=%s", r.name, r.value)
		}
		return plan, nil
	}
	return plan, fmt.Errorf("unreachable: state=%q", r.state)
}

// renderPersistFile assembles the lines (with managed header) into a
// stable byte representation. The header is always re-emitted so the
// file is recognisable as mooncake-managed even after edits.
func renderPersistFile(lines []string) string {
	var sb strings.Builder
	sb.WriteString(managedHeader)
	sb.WriteByte('\n')
	for _, ln := range lines {
		sb.WriteString(ln)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func applyPlan(performer actions.Performer, runner *security.Privileged, plan sysctlPlan, r renderedSysctl) error {
	pOpts := actions.PerformerOpts{}
	if plan.touchesFile {
		if plan.wantContent == "" || !hasContentLines(plan.wantContent) {
			// All managed lines removed — drop the file rather than
			// leave a stray header behind.
			if e := performer.Remove(sysctlPaths.persistFile, false, pOpts); e.Err != nil && !errors.Is(e.Err, fs.ErrNotExist) {
				return fmt.Errorf("os.sysctl: remove %s: %w", sysctlPaths.persistFile, e.Err)
			}
		} else {
			if e := performer.Mkdir(filepath.Dir(sysctlPaths.persistFile), 0o755, pOpts); e.Err != nil {
				return fmt.Errorf("os.sysctl: mkdir %s: %w", filepath.Dir(sysctlPaths.persistFile), e.Err)
			}
			if e := performer.WriteFile(sysctlPaths.persistFile, []byte(plan.wantContent), 0o644, actions.PerformerOpts{ExplicitMode: true}); e.Err != nil {
				return fmt.Errorf("os.sysctl: write %s: %w", sysctlPaths.persistFile, e.Err)
			}
		}
	}
	if plan.apply {
		if err := sysctlApply(runner, r.name, r.value); err != nil {
			return fmt.Errorf("os.sysctl: apply %s=%s: %w", r.name, r.value, err)
		}
	}
	return nil
}

// hasContentLines reports whether the rendered content has any line
// other than the header. Used to decide whether to delete the persist
// file once its last managed entry is removed.
func hasContentLines(content string) bool {
	for _, ln := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return true
	}
	return false
}

// readPersistFile returns the existing key=value lines (skipping the
// header / blanks / other comments). Whether the file existed is
// returned separately so create-vs-update messages stay accurate.
func readPersistFile() ([]string, bool, error) {
	data, err := os.ReadFile(sysctlPaths.persistFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", sysctlPaths.persistFile, err)
	}
	content := strings.TrimSuffix(string(data), "\n")
	if content == "" {
		return nil, true, nil
	}
	lines := []string{}
	for _, ln := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines, true, nil
}

// findLine returns the current value for `name` in the existing lines
// and whether such a line exists. Lines parse as `name = value`
// (canonical) or `name=value`.
func findLine(lines []string, name string) (string, bool) {
	for _, ln := range lines {
		k, v, ok := splitKV(ln)
		if !ok {
			continue
		}
		if k == name {
			return v, true
		}
	}
	return "", false
}

func upsertLine(lines []string, name, value string) []string {
	out := make([]string, 0, len(lines)+1)
	found := false
	for _, ln := range lines {
		k, _, ok := splitKV(ln)
		if !ok {
			out = append(out, ln)
			continue
		}
		if k == name {
			out = append(out, name+" = "+value)
			found = true
			continue
		}
		out = append(out, ln)
	}
	if !found {
		out = append(out, name+" = "+value)
	}
	return out
}

func removeLine(lines []string, name string) []string {
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		k, _, ok := splitKV(ln)
		if ok && k == name {
			continue
		}
		out = append(out, ln)
	}
	return out
}

// splitKV parses a "key = value" or "key=value" line, trimming
// whitespace. Returns ok=false if the input doesn't look like an
// assignment.
func splitKV(line string) (string, string, bool) {
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:eq]), strings.TrimSpace(line[eq+1:]), true
}

// realSysctlGet shells out to `sysctl -n <name>` and returns the
// trimmed stdout. A non-zero exit (unknown key) surfaces as an error
// to the caller so the planner can decide whether to retry.
func realSysctlGet(name string) (string, error) {
	cmd := exec.Command("sysctl", "-n", name)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// realSysctlApply runs `sysctl -w name=value` to push the value to the
// running kernel. Goes through the supplied runner — writing /proc/sys/*
// requires root. Bounded by sysctlCmdTimeout (F051).
func realSysctlApply(runner *security.Privileged, name, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), sysctlCmdTimeout)
	defer cancel()
	out, err := runner.Run(ctx, "sysctl", "-w", name+"="+value)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
