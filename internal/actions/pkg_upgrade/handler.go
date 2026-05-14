// Package pkg_upgrade implements the pkg.upgrade action: upgrade
// named packages or all packages, optionally followed by autoremove.
// v1 ships an apt driver only; other managers raise a clear "only apt
// is supported in v1" error.
//
// Per spec-24, upgrade is "partially idempotent": predicting whether
// apt-get would actually move a package without running it (or
// simulating via `apt-get -s`) is brittle. v1 always invokes apt and
// declares Changed=true on success; plan mode reports the targets
// without invoking anything.
//
//nolint:revive // Package name matches action name convention (pkg_upgrade)
package pkg_upgrade

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName = "pkg.upgrade"
	managerApt = "apt"
)

// Package-level hooks for side-effectful binary calls. Tests replace
// these to keep apply-mode hermetic.
var (
	aptUpgrade    = realAptUpgrade    // func(names []string) error
	aptAutoremove = realAptAutoremove // func() error
	lookPath      = exec.LookPath     // override in tests for manager detection
)

// Handler implements pkg.upgrade.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Upgrade named packages or all installed packages (apt only in v1)",
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
	p := step.PkgUpgrade
	if p == nil {
		return fmt.Errorf("pkg.upgrade requires configuration")
	}
	for i, n := range p.Names {
		if strings.TrimSpace(n) == "" {
			return fmt.Errorf("pkg.upgrade: names[%d] is empty", i)
		}
	}
	// autoremove without names is intentionally allowed — it means
	// "upgrade all, then autoremove".
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	p := step.PkgUpgrade
	result := executor.NewResult()
	result.Checkable = true

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("pkg.upgrade: only Linux is supported; got %s", runtime.GOOS)
	}

	manager, err := resolveManager(p.Manager)
	if err != nil {
		return result, err
	}
	if manager != managerApt {
		return result, fmt.Errorf("pkg.upgrade: only apt is supported in v1 (got %q)", manager)
	}

	names, err := renderNames(ctx, p)
	if err != nil {
		return result, err
	}

	scope := "all packages"
	if len(names) > 0 {
		scope = fmt.Sprintf("[%s]", strings.Join(names, ", "))
	}
	reason := fmt.Sprintf("upgrade %s", scope)
	if p.Autoremove {
		reason += " (+autoremove)"
	}

	result.Data = map[string]interface{}{
		"manager":    manager,
		"attempted":  names,
		"autoremove": p.Autoremove,
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = "would " + reason
		return result, nil
	}

	if err := aptUpgrade(names); err != nil {
		return result, fmt.Errorf("pkg.upgrade: apt upgrade: %w", err)
	}
	if p.Autoremove {
		if err := aptAutoremove(); err != nil {
			return result, fmt.Errorf("pkg.upgrade: apt autoremove: %w", err)
		}
	}

	result.Changed = true
	result.Reason = reason
	ctx.GetLogger().Infof("  pkg.upgrade: %s", reason)

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventPackageManaged,
			Data: map[string]interface{}{
				"action":     actionName,
				"attempted":  names,
				"autoremove": p.Autoremove,
			},
		})
	}
	return result, nil
}

func renderNames(ctx actions.Context, p *config.PkgUpgrade) ([]string, error) {
	if len(p.Names) == 0 {
		return nil, nil
	}
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()
	seen := map[string]struct{}{}
	out := make([]string, 0, len(p.Names))
	for _, n := range p.Names {
		expanded, err := tmpl.Render(n, vars)
		if err != nil {
			return nil, fmt.Errorf("pkg.upgrade: render name %q: %w", n, err)
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
	return out, nil
}

func resolveManager(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		return requested, nil
	}
	if _, err := lookPath("apt-get"); err == nil {
		return managerApt, nil
	}
	return "", fmt.Errorf("pkg.upgrade: cannot auto-detect package manager (apt-get not on PATH); set manager explicitly")
}

func realAptUpgrade(names []string) error {
	var cmd *exec.Cmd
	if len(names) == 0 {
		// #nosec G204 -- fixed apt-get binary.
		cmd = exec.Command("apt-get", "upgrade", "-y")
	} else {
		args := append([]string{"install", "-y", "--only-upgrade"}, names...)
		// #nosec G204 -- args derived from validated YAML names.
		cmd = exec.Command("apt-get", args...)
	}
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
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

func realAptAutoremove() error {
	// #nosec G204 -- fixed apt-get binary.
	cmd := exec.Command("apt-get", "autoremove", "-y")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
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
