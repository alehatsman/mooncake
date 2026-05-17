// Package pkg_upgrade implements the pkg.upgrade action: upgrade
// named packages or all packages, optionally followed by autoremove.
// Ships an apt driver (apt-get upgrade / install --only-upgrade), a
// dnf driver (dnf upgrade / dnf autoremove; yum fallback for RHEL 7),
// and a brew driver (brew upgrade, brew autoremove).
//
// Per spec-24, upgrade is "partially idempotent": predicting whether
// the manager would actually move a package without running it (or
// simulating via `apt-get -s` / `brew upgrade --dry-run`) is brittle.
// Both drivers always invoke the underlying manager and declare
// Changed=true on success; plan mode reports the targets without
// invoking anything.
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
	actionName  = "pkg.upgrade"
	managerApt  = "apt"
	managerDnf  = "dnf"
	managerBrew = "brew"
)

// Package-level hooks for side-effectful binary calls. Tests replace
// these to keep apply-mode hermetic.
var (
	aptUpgrade     = realAptUpgrade     // func(names []string) error
	aptAutoremove  = realAptAutoremove  // func() error
	dnfUpgrade     = realDnfUpgrade     // func(names []string) error
	dnfAutoremove  = realDnfAutoremove  // func() error
	brewUpgrade    = realBrewUpgrade    // func(names []string) error
	brewAutoremove = realBrewAutoremove // func() error
	lookPath       = exec.LookPath      // override in tests for manager detection
)

// Handler implements pkg.upgrade.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Upgrade named packages or all installed packages (apt + dnf on linux, brew on darwin)",
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
// pkg.upgrade is the highest-blast-radius package action: declares
// Sudo and Network always. Subset-upgrade (Names != nil) and
// full-upgrade (Names empty) share the same permission surface —
// both shell to the manager under sudo and pull from configured
// repos. RequiredBinaries follow the (explicit-or-default) manager:
// apt-get for apt, dnf for dnf/yum, brew for brew. When manager: is
// unset we fall back to the platform default (apt-get on linux, brew
// on darwin); a RHEL host should make the manager explicit.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	bin := "apt-get"
	if runtime.GOOS == "darwin" {
		bin = "brew"
	}
	if step != nil && step.PkgUpgrade != nil {
		switch strings.ToLower(strings.TrimSpace(step.PkgUpgrade.Manager)) {
		case managerApt:
			bin = "apt-get"
		case managerDnf, "yum":
			bin = "dnf"
		case managerBrew:
			bin = "brew"
		}
	}
	return actions.PermissionSet{
		Sudo:             true,
		Network:          true,
		RequiredBinaries: []string{bin},
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

	manager, err := resolveManager(p.Manager)
	if err != nil {
		return result, err
	}
	// Canonicalize the "yum" alias to "dnf" — rpm-family hosts run the
	// same upgrade against the same database regardless of which CLI
	// binary the operator referenced.
	if manager == "yum" {
		manager = managerDnf
	}
	if manager != managerApt && manager != managerDnf && manager != managerBrew {
		return result, fmt.Errorf("pkg.upgrade: unsupported manager %q (supported: apt, dnf, brew)", manager)
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

	if err := runUpgrade(manager, names); err != nil {
		return result, fmt.Errorf("pkg.upgrade: %s upgrade: %w", manager, err)
	}
	if p.Autoremove {
		if err := runAutoremove(manager); err != nil {
			return result, fmt.Errorf("pkg.upgrade: %s autoremove: %w", manager, err)
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

// resolveManager honours explicit manager, otherwise auto-detects.
// Precedence: apt > dnf > brew. Apt wins on Debian-family hosts
// (authoritative system manager); dnf wins on RHEL-family hosts
// (where apt-get is absent); brew wins on macOS or as a per-user
// fallback. Operators can override with manager: in the YAML.
func resolveManager(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		return requested, nil
	}
	if _, err := lookPath("apt-get"); err == nil {
		return managerApt, nil
	}
	if _, err := lookPath("dnf"); err == nil {
		return managerDnf, nil
	}
	if _, err := lookPath("yum"); err == nil {
		// RHEL 7 hosts where dnf isn't on PATH yet — yum is the same
		// upgrade vocabulary, just via the older CLI. Canonicalize
		// upward so callers always see "dnf" in the result.
		return managerDnf, nil
	}
	if _, err := lookPath("brew"); err == nil {
		return managerBrew, nil
	}
	return "", fmt.Errorf("pkg.upgrade: cannot auto-detect package manager (no apt-get, dnf, yum, or brew on PATH); set manager explicitly")
}

// runUpgrade dispatches to the per-manager upgrade hook. Keeps the
// Run() flow manager-agnostic.
func runUpgrade(manager string, names []string) error {
	switch manager {
	case managerApt:
		return aptUpgrade(names)
	case managerDnf:
		return dnfUpgrade(names)
	case managerBrew:
		return brewUpgrade(names)
	}
	return fmt.Errorf("runUpgrade: unsupported manager %q", manager)
}

func runAutoremove(manager string) error {
	switch manager {
	case managerApt:
		return aptAutoremove()
	case managerDnf:
		return dnfAutoremove()
	case managerBrew:
		return brewAutoremove()
	}
	return fmt.Errorf("runAutoremove: unsupported manager %q", manager)
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

// realDnfUpgrade shells out to `dnf upgrade -y` (full) or
// `dnf upgrade -y <name>...` (subset). Falls back to `yum` when dnf
// isn't on PATH so the driver works on RHEL 7 hosts as well as
// Fedora / RHEL 8+.
//
// Like apt + brew, the driver always invokes the manager and treats
// success as Changed=true (per spec-24 "partially idempotent").
// "Nothing to do" is a non-error in both dnf and yum, so the
// no-actual-upgrade case still reports Changed=true — matches
// caller expectations across managers.
func realDnfUpgrade(names []string) error {
	bin := dnfBinary()
	args := append([]string{"upgrade", "-y"}, names...)
	// #nosec G204 -- args derived from validated YAML names.
	cmd := exec.Command(bin, args...)
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

// realDnfAutoremove shells out to `dnf autoremove -y`. Available on
// dnf since RHEL 8 + Fedora; yum (RHEL 7) supports the same verb.
func realDnfAutoremove() error {
	bin := dnfBinary()
	// #nosec G204 -- fixed dnf/yum binary.
	cmd := exec.Command(bin, "autoremove", "-y")
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

// dnfBinary returns "dnf" when on PATH, otherwise "yum" if yum is
// present, otherwise "dnf" (so the resulting exec error message names
// the modern manager). Kept tiny on purpose — the real preflight
// happens at Permissions() time.
func dnfBinary() string {
	if _, err := lookPath("dnf"); err == nil {
		return "dnf"
	}
	if _, err := lookPath("yum"); err == nil {
		return "yum"
	}
	return "dnf"
}

// realBrewUpgrade shells out to `brew upgrade` (full) or
// `brew upgrade <name>...` (subset). Brew handles "already at latest"
// silently (no error, no change message), matching apt's behaviour
// from the caller's perspective.
func realBrewUpgrade(names []string) error {
	args := []string{"upgrade"}
	if len(names) > 0 {
		args = append(args, names...)
	}
	// #nosec G204 -- args are validated names; brew binary is fixed.
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

// realBrewAutoremove shells out to `brew autoremove`. Available since
// Homebrew 3.2 (mid-2021); older brew installs fail with "Unknown
// command". The error surfaces to the operator with brew's verbatim
// message — kernel-honest about the version constraint.
func realBrewAutoremove() error {
	// #nosec G204 -- fixed brew binary.
	cmd := exec.Command("brew", "autoremove")
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
