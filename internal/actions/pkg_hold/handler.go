// Package pkg_hold implements the pkg.hold action: mark or unmark
// packages as held so the package manager refuses to upgrade or remove
// them. v1 ships an apt driver only (apt-mark hold / unhold,
// apt-mark showhold for idempotency); other managers raise a clear
// "only apt is supported in v1" error.
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
	actionName    = "pkg.hold"
	stateHeld     = "held"
	stateUnheld   = "unheld"
	managerApt    = "apt"
	managerAuto   = ""
)

// Package-level hooks for the side-effectful binary calls. Tests
// replace these with recorders to keep apply-mode hermetic.
var (
	aptMarkShowHold = realAptMarkShowHold // func() (map[string]bool, error)
	aptMarkHold     = realAptMarkHold     // func(pkgs []string) error
	aptMarkUnhold   = realAptMarkUnhold   // func(pkgs []string) error
	lookPath        = exec.LookPath       // override in tests to fake manager detection
)

// Handler implements pkg.hold.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Mark or unmark packages as held to prevent upgrade/removal (apt only in v1)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		EmitsEvents:        []string{string(events.EventPackageManaged)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux"},
		RequiresSudo:       true,
		ImplementsCheck:    true,
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

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("pkg.hold: only Linux is supported; got %s", runtime.GOOS)
	}

	manager, err := resolveManager(p.Manager)
	if err != nil {
		return result, err
	}
	if manager != managerApt {
		return result, fmt.Errorf("pkg.hold: only apt is supported in v1 (got %q)", manager)
	}

	pkgs, err := renderPackages(ctx, p)
	if err != nil {
		return result, err
	}
	state := normalizeState(p.State)

	currentHeld, err := aptMarkShowHold()
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
		"manager":  manager,
		"state":    state,
		"targets":  pkgs,
		"holding":  toHold,
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

	if len(toHold) > 0 {
		if err := aptMarkHold(toHold); err != nil {
			return result, fmt.Errorf("pkg.hold: apt-mark hold: %w", err)
		}
	}
	if len(toUnhold) > 0 {
		if err := aptMarkUnhold(toUnhold); err != nil {
			return result, fmt.Errorf("pkg.hold: apt-mark unhold: %w", err)
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
// from PATH (only apt is supported in v1).
func resolveManager(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		return requested, nil
	}
	if _, err := lookPath("apt-mark"); err == nil {
		return managerApt, nil
	}
	return "", fmt.Errorf("pkg.hold: cannot auto-detect package manager (apt-mark not on PATH); set manager explicitly")
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
