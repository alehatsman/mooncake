// Package pkg_hold implements the pkg.hold action: mark or unmark
// packages as held so the package manager refuses to upgrade or remove
// them. Ships three drivers:
//   - apt: apt-mark hold/unhold/showhold (Debian/Ubuntu).
//   - dnf: dnf versionlock add/delete/list via the versionlock plugin
//     (Fedora/RHEL 8+; falls back to `yum versionlock` on RHEL 7).
//     Requires dnf-plugin-versionlock to be installed; the driver
//     surfaces a targeted error if the plugin is missing.
//   - brew: brew pin/unpin, brew list --pinned (macOS). Brew's `pin`
//     only works on formulae, not casks — passing a cask name falls
//     through to brew's own error message, which the surrounding
//     wrapping preserves.
//
//nolint:revive // Package name matches action name convention (pkg_hold)
package pkg_hold

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
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

const (
	actionName  = "pkg.hold"
	stateHeld   = "held"
	stateUnheld = "unheld"
	managerApt  = "apt"
	managerDnf  = "dnf"
	managerBrew = "brew"
	managerAuto = ""
)

// Package-level hooks for the side-effectful binary calls. Tests
// replace these with recorders to keep apply-mode hermetic. Spec-69
// phase-5 cleanup: write hooks take an explicit PrivilegedRunner so
// the package no longer carries per-Run mutable state. Read-only hooks
// (showhold / list / show) don't need sudo and keep their original
// signatures.
var (
	aptMarkShowHold    = realAptMarkShowHold    // func() (map[string]bool, error)
	aptMarkHold        = realAptMarkHold        // func(runner, pkgs []string) error
	aptMarkUnhold      = realAptMarkUnhold      // func(runner, pkgs []string) error
	dnfVersionlockShow = realDnfVersionlockShow // func() (map[string]bool, error)
	dnfVersionlockAdd  = realDnfVersionlockAdd  // func(runner, pkgs []string) error
	dnfVersionlockDel  = realDnfVersionlockDel  // func(runner, pkgs []string) error
	brewListPinned     = realBrewListPinned     // func() (map[string]bool, error)
	brewPin            = realBrewPin            // func(runner, pkgs []string) error (runner unused; brew never sudo)
	brewUnpin          = realBrewUnpin          // func(runner, pkgs []string) error (runner unused; brew never sudo)
	lookPath           = exec.LookPath          // override in tests to fake manager detection
)

// Handler implements pkg.hold.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("PkgHoldReverseInfo", func() any { return &PkgHoldReverseInfo{} })
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Mark or unmark packages as held to prevent upgrade/removal (apt + dnf on linux, brew on darwin)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		EmitsEvents:        []string{string(events.EventPackageManaged)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux", "darwin"},
		RequiresSudo:       true,
		ImplementsCheck:    true,
	}
}

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// pkg.hold always declares Sudo=true on linux: apt-mark hold/unhold
// and dnf versionlock both write system state. On darwin, brew pin
// writes to the local Cellar — typically owned by the operator's
// account, but `become: true` + brew is a common multi-user pattern,
// and conservatively advertising Sudo=true keeps spec-44 doctor
// honest. No Network. RequiredBinaries follow the (explicit-or-
// default) manager: apt-mark for apt, dnf for dnf/yum, brew for brew.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	bin := "apt-mark"
	if runtime.GOOS == "darwin" {
		bin = "brew"
	}
	if step != nil && step.PkgHold != nil {
		switch strings.ToLower(strings.TrimSpace(step.PkgHold.Manager)) {
		case managerApt:
			bin = "apt-mark"
		case managerDnf, "yum":
			bin = "dnf"
		case managerBrew:
			bin = "brew"
		}
	}
	return actions.PermissionSet{
		Sudo:             true,
		RequiredBinaries: []string{bin},
	}
}

func (h *Handler) Validate(step *config.Step) error {
	p := step.PkgHold
	if p == nil {
		return fmt.Errorf("pkg.hold requires configuration")
	}
	hasName := strings.TrimSpace(p.Name) != ""
	hasNames := len(p.Names) > 0
	if !hasName && !hasNames {
		return fmt.Errorf("pkg.hold: name or names is required")
	}
	if hasName && hasNames {
		return fmt.Errorf("pkg.hold: name and names are mutually exclusive")
	}
	state := normalizeState(p.State)
	if state != stateHeld && state != stateUnheld {
		return fmt.Errorf("pkg.hold: state must be held or unheld, got %q", p.State)
	}
	for i, n := range p.Names {
		if strings.TrimSpace(n) == "" {
			return fmt.Errorf("pkg.hold: names[%d] is empty", i)
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
	p := step.PkgHold
	result := executor.NewResult()
	result.Checkable = true

	// Spec-69 phase 5: the sudo-aware runner is per-Run and threaded
	// through the apt-mark / dnf versionlock write hooks below.
	// Read-only paths + brew ignore it.
	runner := ctx.Privileged()

	manager, err := resolveManager(p.Manager)
	if err != nil {
		return result, err
	}
	// Canonicalize the "yum" alias to "dnf" — the rpm-family driver
	// handles both transparently (yum fallback for RHEL 7).
	if manager == "yum" {
		manager = managerDnf
	}
	if manager != managerApt && manager != managerDnf && manager != managerBrew {
		return result, fmt.Errorf("pkg.hold: unsupported manager %q (supported: apt, dnf, brew)", manager)
	}

	pkgs, err := renderPackages(ctx, p)
	if err != nil {
		return result, err
	}
	state := normalizeState(p.State)

	currentHeld, err := readCurrentHolds(ctx.Ctx(), manager)
	if err != nil {
		return result, fmt.Errorf("pkg.hold: read current holds: %w", err)
	}

	var toHold, toUnhold []string
	for _, name := range pkgs {
		isHeld := currentHeld[name]
		switch state {
		case stateHeld:
			if !isHeld {
				toHold = append(toHold, name)
			}
		case stateUnheld:
			if isHeld {
				toUnhold = append(toUnhold, name)
			}
		}
	}
	sort.Strings(toHold)
	sort.Strings(toUnhold)

	result.Target = strings.Join(pkgs, ",")
	result.Data = map[string]interface{}{
		"manager":   manager,
		"state":     state,
		"targets":   pkgs,
		"holding":   toHold,
		"unholding": toUnhold,
	}

	if len(toHold) == 0 && len(toUnhold) == 0 {
		result.Operation = executor.OpNoop
		result.Reason = fmt.Sprintf("packages already at desired state (%s)", state)
		return result, nil
	}
	result.Operation = executor.OpUpdate

	reason := summarize(state, toHold, toUnhold)
	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = reason
		return result, nil
	}

	// Capture pre-mutation state for Reverse() BEFORE shelling out.
	// toHold + toUnhold are mutually exclusive (the step's State
	// pins direction), so one of them is empty and Mutated is just
	// the flipped set. AppliedState records which direction the
	// apply went so Reverse can return the inverse.
	mutated := toHold
	if len(toUnhold) > 0 {
		mutated = toUnhold
	}
	result.ReverseData = &PkgHoldReverseInfo{
		Manager:      manager,
		AppliedState: state,
		Mutated:      append([]string(nil), mutated...),
	}

	if len(toHold) > 0 {
		if err := runHold(ctx.Ctx(), runner, manager, toHold); err != nil {
			return result, fmt.Errorf("pkg.hold: %s hold: %w", manager, err)
		}
	}
	if len(toUnhold) > 0 {
		if err := runUnhold(ctx.Ctx(), runner, manager, toUnhold); err != nil {
			return result, fmt.Errorf("pkg.hold: %s unhold: %w", manager, err)
		}
	}

	result.Changed = true
	result.Reason = reason
	ctx.GetLogger().Infof("  pkg.hold: %s", reason)

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventPackageManaged,
			Data: map[string]interface{}{
				"action":    actionName,
				"state":     state,
				"holding":   toHold,
				"unholding": toUnhold,
			},
		})
	}
	return result, nil
}

func summarize(state string, toHold, toUnhold []string) string {
	var parts []string
	if len(toHold) > 0 {
		parts = append(parts, fmt.Sprintf("hold [%s]", strings.Join(toHold, ", ")))
	}
	if len(toUnhold) > 0 {
		parts = append(parts, fmt.Sprintf("unhold [%s]", strings.Join(toUnhold, ", ")))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("no drift (state=%s)", state)
	}
	return strings.Join(parts, "; ")
}

// renderPackages produces the deduplicated, template-expanded list of
// package names to operate on.
func renderPackages(ctx actions.Context, p *config.PkgHold) ([]string, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()
	render := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		return tmpl.Render(s, vars)
	}

	var raw []string
	if p.Name != "" {
		raw = append(raw, p.Name)
	}
	raw = append(raw, p.Names...)

	seen := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		expanded, err := render(r)
		if err != nil {
			return nil, fmt.Errorf("pkg.hold: render name %q: %w", r, err)
		}
		expanded = strings.TrimSpace(expanded)
		if expanded == "" {
			continue
		}
		if _, ok := seen[expanded]; ok {
			continue
		}
		seen[expanded] = struct{}{}
		out = append(out, expanded)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("pkg.hold: no packages after rendering")
	}
	return out, nil
}

// resolveManager honours an explicit manager, otherwise auto-detects
// from PATH. Precedence: apt > dnf > brew. Apt wins on Debian-family
// hosts (authoritative system manager); dnf wins on RHEL-family hosts
// (where apt-mark is absent); brew wins on macOS or as a per-user
// fallback. Operators can override either way via explicit manager:.
func resolveManager(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		return requested, nil
	}
	if _, err := lookPath("apt-mark"); err == nil {
		return managerApt, nil
	}
	if _, err := lookPath("dnf"); err == nil {
		return managerDnf, nil
	}
	if _, err := lookPath("yum"); err == nil {
		// RHEL 7: yum is the modern-equivalent CLI; the canonical
		// result manager stays "dnf" so callers don't need to handle
		// two spellings.
		return managerDnf, nil
	}
	if _, err := lookPath("brew"); err == nil {
		return managerBrew, nil
	}
	return "", fmt.Errorf("pkg.hold: cannot auto-detect package manager (no apt-mark, dnf, yum, or brew on PATH); set manager explicitly")
}

// readCurrentHolds returns the set of currently-held package names
// for the resolved manager. Centralises the dispatch so the Run()
// flow stays manager-agnostic. F2: ctx is the run-wide cancel.
func readCurrentHolds(ctx context.Context, manager string) (map[string]bool, error) {
	switch manager {
	case managerApt:
		return aptMarkShowHold(ctx)
	case managerDnf:
		return dnfVersionlockShow(ctx)
	case managerBrew:
		return brewListPinned(ctx)
	}
	return nil, fmt.Errorf("readCurrentHolds: unsupported manager %q", manager)
}

func runHold(ctx context.Context, runner *security.Privileged, manager string, pkgs []string) error {
	switch manager {
	case managerApt:
		return aptMarkHold(ctx, runner, pkgs)
	case managerDnf:
		return dnfVersionlockAdd(ctx, runner, pkgs)
	case managerBrew:
		return brewPin(ctx, runner, pkgs)
	}
	return fmt.Errorf("runHold: unsupported manager %q", manager)
}

func runUnhold(ctx context.Context, runner *security.Privileged, manager string, pkgs []string) error {
	switch manager {
	case managerApt:
		return aptMarkUnhold(ctx, runner, pkgs)
	case managerDnf:
		return dnfVersionlockDel(ctx, runner, pkgs)
	case managerBrew:
		return brewUnpin(ctx, runner, pkgs)
	}
	return fmt.Errorf("runUnhold: unsupported manager %q", manager)
}

func normalizeState(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return stateHeld
	}
	return s
}

// realAptMarkShowHold runs `apt-mark showhold` and returns the set of
// currently-held package names. F2: ctx is the run-wide cancel.
func realAptMarkShowHold(ctx context.Context) (map[string]bool, error) {
	// #nosec G204 -- fixed apt-mark binary.
	cmd := exec.CommandContext(ctx, "apt-mark", "showhold")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	held := map[string]bool{}
	sc := bufio.NewScanner(&stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		held[line] = true
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan apt-mark showhold: %w", err)
	}
	return held, nil
}

func realAptMarkHold(ctx context.Context, runner *security.Privileged, pkgs []string) error {
	return runAptMark(ctx, runner, "hold", pkgs)
}

func realAptMarkUnhold(ctx context.Context, runner *security.Privileged, pkgs []string) error {
	return runAptMark(ctx, runner, "unhold", pkgs)
}

func runAptMark(ctx context.Context, runner *security.Privileged, verb string, pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{verb}, pkgs...)
	out, err := runner.Run(ctx, "apt-mark", args...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// realBrewListPinned runs `brew list --pinned` and returns the set
// of currently-pinned formulae. Brew output is one name per line.
// Casks can't be pinned (brew limitation); they never appear in
// this output. F2: ctx is the run-wide cancel.
func realBrewListPinned(ctx context.Context) (map[string]bool, error) {
	// #nosec G204 -- fixed brew binary.
	cmd := exec.CommandContext(ctx, "brew", "list", "--pinned")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	pinned := map[string]bool{}
	sc := bufio.NewScanner(&stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		pinned[line] = true
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan brew list --pinned: %w", err)
	}
	return pinned, nil
}

func realBrewPin(ctx context.Context, _ *security.Privileged, pkgs []string) error {
	return runBrew(ctx, "pin", pkgs)
}

func realBrewUnpin(ctx context.Context, _ *security.Privileged, pkgs []string) error {
	return runBrew(ctx, "unpin", pkgs)
}

// runBrew shells out to `brew pin|unpin <pkgs...>`. Brew accepts
// multiple formula names per invocation. Casks fail with brew's
// own error message ("Error: Cask <name> is not a formula") — we
// surface it verbatim so the operator sees the actual brew
// constraint rather than a mooncake-wrapped layer that obscures
// the cause. No runner arg: brew never runs under sudo.
func runBrew(ctx context.Context, verb string, pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{verb}, pkgs...)
	// #nosec G204 -- args are validated package names from YAML.
	cmd := exec.CommandContext(ctx, "brew", args...)
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

// dnfBinary returns "dnf" if on PATH, else "yum" if yum is present
// (RHEL 7 fallback), else "dnf" so any subsequent exec error names
// the modern manager. Kept identical in spirit to the helper in
// pkg.upgrade's handler.go; duplicated rather than promoted to a
// shared package because the pkg_* handlers are intentionally
// per-action and the function body is two lines.
func dnfBinary() string {
	if _, err := lookPath("dnf"); err == nil {
		return "dnf"
	}
	if _, err := lookPath("yum"); err == nil {
		return "yum"
	}
	return "dnf"
}

// versionlockEntryRE matches the boundary between the package name
// and its `epoch:version-release.arch` suffix in `dnf versionlock
// list` output. Format: `bash-0:5.1.8-9.el9.*` → name="bash". The
// epoch is always present in dnf-plugin-versionlock output (defaults
// to 0 when not explicit), so the `-<digits>:` anchor is reliable.
var versionlockEntryRE = regexp.MustCompile(`-\d+:`)

// versionlockMissingPluginRE detects the dnf/yum error that surfaces
// when the versionlock plugin isn't installed. Both managers emit
// substantially similar messages — match the discriminating fragments
// so we can produce a targeted "install dnf-plugin-versionlock" hint
// instead of leaking a bare exec error.
var versionlockMissingPluginRE = regexp.MustCompile(`(?i)no such command|unknown command|argument command: invalid choice: 'versionlock'`)

// realDnfVersionlockShow runs `dnf versionlock list` and returns the
// set of currently-locked package names. Falls back to `yum
// versionlock list` on RHEL 7. Skips dnf's metadata-expiration
// header and any non-entry lines by accepting only those that contain
// the canonical `-<epoch>:` boundary.
//
// If the versionlock plugin isn't installed (the most common
// preflight failure on a fresh box), the error is rewrapped with a
// targeted install hint pointing at dnf-plugin-versionlock /
// yum-plugin-versionlock — saves the operator a doc round-trip.
func realDnfVersionlockShow(ctx context.Context) (map[string]bool, error) {
	bin := dnfBinary()
	// #nosec G204 -- fixed dnf/yum binary.
	cmd := exec.CommandContext(ctx, bin, "versionlock", "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if versionlockMissingPluginRE.MatchString(msg) {
			return nil, fmt.Errorf("pkg.hold: %s versionlock plugin not installed — run `pkg.install: { name: %s-plugin-versionlock, state: present }` first", bin, bin)
		}
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	held := map[string]bool{}
	sc := bufio.NewScanner(&stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if name := parseVersionlockEntry(line); name != "" {
			held[name] = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan %s versionlock list: %w", bin, err)
	}
	return held, nil
}

// parseVersionlockEntry extracts the package name from a single
// `dnf versionlock list` entry. The expected shape is
// `<name>-<epoch>:<version>-<release>.<arch>` (e.g.
// `bash-0:5.1.8-9.el9.*`). Lines that don't match this shape (header
// metadata, plugin warnings, etc.) return "" so the caller's loop
// silently skips them — header filtering by structure, not by string
// matching, is more robust against dnf version drift.
func parseVersionlockEntry(line string) string {
	line = strings.TrimSuffix(line, ".*")
	if loc := versionlockEntryRE.FindStringIndex(line); loc != nil {
		return line[:loc[0]]
	}
	return ""
}

func realDnfVersionlockAdd(ctx context.Context, runner *security.Privileged, pkgs []string) error {
	return runDnfVersionlock(ctx, runner, "add", pkgs)
}

func realDnfVersionlockDel(ctx context.Context, runner *security.Privileged, pkgs []string) error {
	return runDnfVersionlock(ctx, runner, "delete", pkgs)
}

// runDnfVersionlock shells out to `dnf versionlock add|delete <pkgs>`.
// Stderr is folded into the returned error so the operator sees the
// dnf message verbatim. The plugin-missing path is detected here too
// (the showhold equivalent), turning a bare exec failure into a
// targeted install hint.
func runDnfVersionlock(ctx context.Context, runner *security.Privileged, verb string, pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	bin := dnfBinary()
	args := append([]string{"versionlock", verb}, pkgs...)
	out, err := runner.Run(ctx, bin, args...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if versionlockMissingPluginRE.MatchString(msg) {
			return fmt.Errorf("pkg.hold: %s versionlock plugin not installed — run `pkg.install: { name: %s-plugin-versionlock, state: present }` first", bin, bin)
		}
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
