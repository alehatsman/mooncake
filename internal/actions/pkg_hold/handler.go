// Package pkg_hold implements the pkg.hold action: mark or unmark
// packages as held so the package manager refuses to upgrade or remove
// them. Ships an apt driver (apt-mark hold/unhold/showhold) and a brew
// driver (brew pin/unpin, brew list --pinned). Brew's `pin` only works
// on formulae, not casks — passing a cask name falls through to brew's
// own error message, which the surrounding wrapping preserves.
//
//nolint:revive // Package name matches action name convention (pkg_hold)
package pkg_hold

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName  = "pkg.hold"
	stateHeld   = "held"
	stateUnheld = "unheld"
	managerApt  = "apt"
	managerBrew = "brew"
	managerAuto = ""
)

// Package-level hooks for the side-effectful binary calls. Tests
// replace these with recorders to keep apply-mode hermetic.
var (
	aptMarkShowHold = realAptMarkShowHold // func() (map[string]bool, error)
	aptMarkHold     = realAptMarkHold     // func(pkgs []string) error
	aptMarkUnhold   = realAptMarkUnhold   // func(pkgs []string) error
	brewListPinned  = realBrewListPinned  // func() (map[string]bool, error)
	brewPin         = realBrewPin         // func(pkgs []string) error
	brewUnpin       = realBrewUnpin       // func(pkgs []string) error
	lookPath        = exec.LookPath       // override in tests to fake manager detection
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
		Description:        "Mark or unmark packages as held to prevent upgrade/removal (apt on linux, brew on darwin)",
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
// writes dpkg state. On darwin, brew pin writes to the local Cellar
// — typically owned by the operator's account, but `become: true`
// + brew is a common multi-user pattern, and conservatively
// advertising Sudo=true keeps spec-44 doctor honest. No Network.
// RequiredBinaries vary by host: apt-mark on linux, brew on darwin.
func (Handler) Permissions(_ *config.Step) actions.PermissionSet {
	bin := "apt-mark"
	if runtime.GOOS == "darwin" {
		bin = "brew"
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

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	p := step.PkgHold
	result := executor.NewResult()
	result.Checkable = true

	manager, err := resolveManager(p.Manager)
	if err != nil {
		return result, err
	}
	if manager != managerApt && manager != managerBrew {
		return result, fmt.Errorf("pkg.hold: unsupported manager %q (supported: apt, brew)", manager)
	}

	pkgs, err := renderPackages(ctx, p)
	if err != nil {
		return result, err
	}
	state := normalizeState(p.State)

	currentHeld, err := readCurrentHolds(manager)
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

	result.Data = map[string]interface{}{
		"manager":   manager,
		"state":     state,
		"targets":   pkgs,
		"holding":   toHold,
		"unholding": toUnhold,
	}

	if len(toHold) == 0 && len(toUnhold) == 0 {
		result.Reason = fmt.Sprintf("packages already at desired state (%s)", state)
		return result, nil
	}

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
		if err := runHold(manager, toHold); err != nil {
			return result, fmt.Errorf("pkg.hold: %s hold: %w", manager, err)
		}
	}
	if len(toUnhold) > 0 {
		if err := runUnhold(manager, toUnhold); err != nil {
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
// from PATH. apt is preferred over brew on multi-manager hosts
// (Linuxbrew installed alongside apt on Debian-family boxes): system-
// level package state is more authoritative than per-user brew.
// Operators can override either way via explicit manager:.
func resolveManager(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		return requested, nil
	}
	if _, err := lookPath("apt-mark"); err == nil {
		return managerApt, nil
	}
	if _, err := lookPath("brew"); err == nil {
		return managerBrew, nil
	}
	return "", fmt.Errorf("pkg.hold: cannot auto-detect package manager (no apt-mark or brew on PATH); set manager explicitly")
}

// readCurrentHolds returns the set of currently-held package names
// for the resolved manager. Centralises the dispatch so the Run()
// flow stays manager-agnostic.
func readCurrentHolds(manager string) (map[string]bool, error) {
	switch manager {
	case managerApt:
		return aptMarkShowHold()
	case managerBrew:
		return brewListPinned()
	}
	return nil, fmt.Errorf("readCurrentHolds: unsupported manager %q", manager)
}

func runHold(manager string, pkgs []string) error {
	switch manager {
	case managerApt:
		return aptMarkHold(pkgs)
	case managerBrew:
		return brewPin(pkgs)
	}
	return fmt.Errorf("runHold: unsupported manager %q", manager)
}

func runUnhold(manager string, pkgs []string) error {
	switch manager {
	case managerApt:
		return aptMarkUnhold(pkgs)
	case managerBrew:
		return brewUnpin(pkgs)
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
// currently-held package names.
func realAptMarkShowHold() (map[string]bool, error) {
	// #nosec G204 -- fixed apt-mark binary.
	cmd := exec.Command("apt-mark", "showhold")
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

func realAptMarkHold(pkgs []string) error {
	return runAptMark("hold", pkgs)
}

func realAptMarkUnhold(pkgs []string) error {
	return runAptMark("unhold", pkgs)
}

func runAptMark(verb string, pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{verb}, pkgs...)
	// #nosec G204 -- args are validated package names from YAML.
	cmd := exec.Command("apt-mark", args...)
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

// realBrewListPinned runs `brew list --pinned` and returns the set
// of currently-pinned formulae. Brew output is one name per line.
// Casks can't be pinned (brew limitation); they never appear in
// this output.
func realBrewListPinned() (map[string]bool, error) {
	// #nosec G204 -- fixed brew binary.
	cmd := exec.Command("brew", "list", "--pinned")
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

func realBrewPin(pkgs []string) error {
	return runBrew("pin", pkgs)
}

func realBrewUnpin(pkgs []string) error {
	return runBrew("unpin", pkgs)
}

// runBrew shells out to `brew pin|unpin <pkgs...>`. Brew accepts
// multiple formula names per invocation. Casks fail with brew's
// own error message ("Error: Cask <name> is not a formula") — we
// surface it verbatim so the operator sees the actual brew
// constraint rather than a mooncake-wrapped layer that obscures
// the cause.
func runBrew(verb string, pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{verb}, pkgs...)
	// #nosec G204 -- args are validated package names from YAML.
	cmd := exec.Command("brew", args...)
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
