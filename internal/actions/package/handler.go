// Package package_handler implements the package action handler.
//
// The package action manages system packages with support for:
// - Auto-detection of package manager (apt, dnf, yum, pacman, zypper, apk, brew, port, choco, scoop)
// - Manual package manager selection
// - Install, remove, and update operations
// - Cache management and system upgrades
//
//nolint:revive,staticcheck // package_handler name required to avoid conflict with Go keyword
package package_handler

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/template"
)

// Package manager constants
const (
	pmApt    = "apt"
	pmDnf    = "dnf"
	pmYum    = "yum"
	pmPacman = "pacman"
	pmZypper = "zypper"
	pmApk    = "apk"
	pmBrew   = "brew"
	pmPort   = "port"
	pmChoco  = "choco"
	pmScoop  = "scoop"
	pmWinget = "winget"
)

// State constants
const (
	statePresent = "present"
	stateAbsent  = "absent"
	stateLatest  = "latest"
)

// Handler implements the Handler interface for package actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the package action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "pkg",
		Description:        "Manage system packages (install/remove/update)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		EmitsEvents:        []string{string(events.EventPackageManaged)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux", "darwin", "windows", "freebsd"}, // Multiple package managers supported
		RequiresSudo:       true,                                              // Typically requires elevated privileges
		ImplementsCheck:    true,                                              // Checks if package is installed before installing
	}
}

// Permissions implements actions.Permitter (spec-22). pkg always
// declares Sudo and Network: every supported manager (apt/dnf/brew/
// pacman/winget/...) needs elevated privileges to mutate the system
// package database AND reaches out to remote repositories. The
// declaration is unconditional — it does NOT depend on step.Pkg
// fields, since the action TYPE is the indicator.
//
// FilesystemWrite is intentionally empty: pkg writes to
// system-managed paths (/usr/bin, /var/lib/dpkg/, Cellar/, etc.)
// that the user doesn't address directly. A future policy layer
// that wants to gate "which packages can be installed" should look
// at step.Pkg.Name(s), not FilesystemWrite.
//
// RequiredBinaries is also empty by design: the package handler
// auto-detects the manager and ships fallbacks. Demanding a specific
// binary on PATH would break detection. Per-manager preflight (e.g.
// "apt-get exists") happens inside Run.
func (h *Handler) Permissions(step *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		Sudo:    true,
		Network: true,
	}
}

// Validate checks if the package configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.Pkg == nil {
		return fmt.Errorf("package configuration is nil")
	}

	pkg := step.Pkg

	// Must have either name, names, or upgrade
	if pkg.Name == "" && len(pkg.Names) == 0 && pkg.NamesExpr == "" && !pkg.Upgrade {
		return fmt.Errorf("one of 'name', 'names', or 'upgrade' is required")
	}

	// Validate state
	if pkg.State != "" && pkg.State != statePresent && pkg.State != stateAbsent && pkg.State != stateLatest {
		return fmt.Errorf("state must be one of: present, absent, latest (got %q)", pkg.State)
	}

	return nil
}

// Execute runs the package action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	pkg := step.Pkg

	// Cast to ExecutionContext
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Determine package manager
	manager, err := h.determinePackageManager(pkg.Manager, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to determine package manager: %w", err)
	}

	ctx.GetLogger().Debugf("  Using package manager: %s", manager)

	// Determine state (default: present)
	state := pkg.State
	if state == "" {
		state = "present"
	}

	// Build package list and render template variables in names.
	// TODO: consider moving this into the plan phase so plan output shows expanded names.
	packages := h.buildPackageList(pkg)
	for i, name := range packages {
		rendered, renderErr := ctx.GetTemplate().Render(name, ctx.GetVariables())
		if renderErr == nil {
			packages[i] = rendered
		}
	}

	// Resolve templated names expression (when YAML `names:` was a scalar).
	if pkg.NamesExpr != "" {
		expanded, expandErr := h.resolveNamesExpr(ctx, pkg.NamesExpr)
		if expandErr != nil {
			return nil, fmt.Errorf("failed to resolve package names expression %q: %w", pkg.NamesExpr, expandErr)
		}
		packages = append(packages, expanded...)
	}

	// Create result
	result := executor.NewResult()
	result.SetChanged(false)

	// Handle upgrade operation
	if pkg.Upgrade {
		return h.executeUpgrade(ec, manager, pkg, step.ShouldBecome())
	}

	// Update cache if requested
	if pkg.UpdateCache {
		if err := h.updateCache(ec, manager, step.ShouldBecome()); err != nil {
			return nil, fmt.Errorf("failed to update package cache: %w", err)
		}
	}

	// Execute based on state
	switch state {
	case statePresent, "latest":
		return h.installPackages(ec, manager, packages, state == "latest", pkg.Extra, step.ShouldBecome())
	case stateAbsent:
		return h.removePackages(ec, manager, packages, pkg.Extra, step.ShouldBecome())
	default:
		return nil, fmt.Errorf("unsupported state: %s", state)
	}
}

// DryRun shows what would be done without making changes.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	pkg := step.Pkg

	// Determine package manager
	manager, err := h.determinePackageManager(pkg.Manager, ctx.GetVariables())
	if err != nil {
		return fmt.Errorf("failed to determine package manager: %w", err)
	}

	// Determine state
	state := pkg.State
	if state == "" {
		state = "present"
	}

	// Build package list
	packages := h.buildPackageList(pkg)
	if pkg.NamesExpr != "" {
		if expanded, expandErr := h.resolveNamesExpr(ctx, pkg.NamesExpr); expandErr == nil {
			packages = append(packages, expanded...)
		} else {
			ctx.GetLogger().Infof("  Would expand names from expression: %s", pkg.NamesExpr)
		}
	}

	if pkg.Upgrade {
		ctx.GetLogger().Infof("  Would upgrade all packages using %s", manager)
		return nil
	}

	if pkg.UpdateCache {
		ctx.GetLogger().Infof("  Would update package cache using %s", manager)
	}

	var operation string
	switch state {
	case stateAbsent:
		operation = "remove"
	case stateLatest:
		operation = "install/upgrade"
	default:
		operation = "install"
	}

	for _, pkgName := range packages {
		ctx.GetLogger().Infof("  Would %s package: %s", operation, pkgName)
	}

	return nil
}

// determinePackageManager determines which package manager to use.
func (h *Handler) determinePackageManager(specified string, variables map[string]interface{}) (string, error) {
	// If explicitly specified, use it
	if specified != "" {
		return specified, nil
	}

	// Try to get from system facts
	if pm, ok := variables["package_manager"].(string); ok && pm != "" {
		return pm, nil
	}

	// Fallback: detect based on OS
	switch runtime.GOOS {
	case "linux":
		return h.detectLinuxPackageManager()
	case "darwin":
		return h.detectMacOSPackageManager()
	case "windows":
		return h.detectWindowsPackageManager()
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// detectLinuxPackageManager detects the package manager on Linux.
func (h *Handler) detectLinuxPackageManager() (string, error) {
	managers := []string{pmApt, pmDnf, pmYum, pmPacman, pmZypper, pmApk}
	for _, mgr := range managers {
		if _, err := exec.LookPath(mgr); err == nil {
			return mgr, nil
		}
	}
	return "", fmt.Errorf("no supported package manager found")
}

// detectMacOSPackageManager detects the package manager on macOS.
func (h *Handler) detectMacOSPackageManager() (string, error) {
	// Check for brew first (most common)
	if _, err := exec.LookPath(pmBrew); err == nil {
		return pmBrew, nil
	}
	// Check for MacPorts
	if _, err := exec.LookPath(pmPort); err == nil {
		return pmPort, nil
	}
	return "", fmt.Errorf("no supported package manager found (install Homebrew or MacPorts)")
}

// detectWindowsPackageManager detects the package manager on Windows.
// Order: winget (ships with Win11) → choco → scoop.
func (h *Handler) detectWindowsPackageManager() (string, error) {
	if _, err := exec.LookPath(pmWinget); err == nil {
		return pmWinget, nil
	}
	if _, err := exec.LookPath(pmChoco); err == nil {
		return pmChoco, nil
	}
	if _, err := exec.LookPath(pmScoop); err == nil {
		return pmScoop, nil
	}
	return "", fmt.Errorf("no supported package manager found (install winget, Chocolatey, or Scoop)")
}

// buildPackageList builds a list of packages from name and names fields.
func (h *Handler) buildPackageList(pkg *config.Package) []string {
	var packages []string
	if pkg.Name != "" {
		packages = append(packages, pkg.Name)
	}
	if len(pkg.Names) > 0 {
		packages = append(packages, pkg.Names...)
	}
	return packages
}

// resolveNamesExpr renders pkg.NamesExpr against the current variables and
// converts the result into a []string. Variables that hold a slice are
// matched without a Pongo2 round-trip (preserves typing); otherwise the
// template renders to a string and the shared resolver parses it.
func (h *Handler) resolveNamesExpr(ctx actions.Context, expr string) ([]string, error) {
	vars := ctx.GetVariables()
	evaluator := ctx.GetEvaluator()

	trimmed := strings.TrimSpace(expr)
	if inner, ok := stripTemplateWrapper(trimmed); ok {
		if items, err := template.ResolveStringList(inner, vars, evaluator); err == nil {
			return items, nil
		}
	}

	rendered, renderErr := ctx.GetTemplate().Render(expr, vars)
	if renderErr != nil {
		return nil, fmt.Errorf("render: %w", renderErr)
	}
	return template.ResolveStringList(rendered, vars, evaluator)
}

// stripTemplateWrapper unwraps `{{ expr }}` to `expr` so we can resolve
// against the variable map directly and preserve []string typing. Returns
// (inner, true) on a clean single-tag wrapper; (input, false) otherwise.
func stripTemplateWrapper(s string) (string, bool) {
	if !strings.HasPrefix(s, "{{") || !strings.HasSuffix(s, "}}") {
		return s, false
	}
	inner := strings.TrimSpace(s[2 : len(s)-2])
	if inner == "" || strings.Contains(inner, "{{") || strings.Contains(inner, "}}") {
		return s, false
	}
	return inner, true
}

// updateCache updates the package manager cache.
func (h *Handler) updateCache(ec *executor.ExecutionContext, manager string, become bool) error {
	var cmdArgs []string
	switch manager {
	case pmApt:
		cmdArgs = []string{"apt-get", "update"}
	case pmDnf, pmYum:
		cmdArgs = []string{manager, "makecache"}
	case pmPacman:
		cmdArgs = []string{pmPacman, "-Sy"}
	case pmApk:
		cmdArgs = []string{pmApk, "update"}
	case pmBrew:
		cmdArgs = []string{pmBrew, "update"}
	default:
		// Other package managers don't need cache updates or do it automatically
		return nil
	}

	ec.Svc.Logger.Debugf("  Updating package cache: %s", strings.Join(cmdArgs, " "))

	output, err := h.runCmd(ec, become, cmdArgs)
	if err != nil {
		ec.Svc.Logger.Debugf("    Output: %s", strings.TrimSpace(string(output)))
		return fmt.Errorf("failed to update package cache: %w", err)
	}

	return nil
}

// runCmd executes a command, wrapping with sudo -S when become is true.
func (h *Handler) runCmd(ec *executor.ExecutionContext, become bool, cmdArgs []string) ([]byte, error) {
	var cmd *exec.Cmd
	if become {
		// #nosec G204 - sudo wrapping for privilege escalation
		cmd = exec.Command("sudo", append([]string{"-S"}, cmdArgs...)...)
		cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")
	} else {
		// #nosec G204 - Package manager commands are validated
		cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
	}
	return cmd.CombinedOutput()
}

// installPackages installs or upgrades packages.
//
// Behavior: partition into existing vs. to-install via per-package check,
// then issue a single batched manager invocation for the to-install set.
func (h *Handler) installPackages(ec *executor.ExecutionContext, manager string, packages []string, upgrade bool, extra []string, become bool) (actions.Result, error) {
	result := executor.NewResult()

	var toInstall, existingPkgs []string

	for _, pkg := range packages {
		installed, err := h.isPackageInstalled(ec, manager, pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to check if package %q is installed: %w", pkg, err)
		}

		if installed && !upgrade {
			ec.Svc.Logger.Debugf("  Package %q is already installed", pkg)
			existingPkgs = append(existingPkgs, pkg)
			continue
		}

		toInstall = append(toInstall, pkg)
	}

	// Capture pre-mutation state for Reverse() (spec-22 phase 5 slice F).
	// toInstall is the exact list the apply will install — and exactly
	// what reverse should remove. existingPkgs stays untouched on both
	// sides. AppliedState distinguishes the install path from latest-upgrade
	// so Reverse can refuse the latter explicitly.
	captured := statePresent
	if upgrade {
		captured = stateLatest
	}
	result.ReverseData = &PkgReverseInfo{
		AppliedState: captured,
		Manager:      manager,
		Mutated:      append([]string(nil), toInstall...),
	}

	if len(toInstall) == 0 {
		ec.EmitEvent(events.EventPackageManaged, events.PackageManagedData{
			Manager:        manager,
			AlreadyPresent: existingPkgs,
		})
		return result, nil
	}

	// winget doesn't batch with --id, so each package gets its own call.
	if manager == pmWinget {
		verb := "install"
		if upgrade {
			verb = "upgrade"
		}
		for _, pkg := range toInstall {
			cmdArgs := buildWingetCommand(verb, pkg, extra)
			ec.Svc.Logger.Infof("  Installing package: %s", pkg)
			ec.Svc.Logger.Debugf("    Command: %s", strings.Join(cmdArgs, " "))

			output, execErr := h.runCmd(ec, become, cmdArgs)
			if execErr != nil {
				ec.Svc.Logger.Debugf("    Output: %s", strings.TrimSpace(string(output)))
				return nil, fmt.Errorf("failed to install package %q: %w", pkg, execErr)
			}
		}
		result.SetChanged(true)
		ec.EmitEvent(events.EventPackageManaged, events.PackageManagedData{
			Manager:        manager,
			Installed:      toInstall,
			AlreadyPresent: existingPkgs,
		})
		return result, nil
	}

	cmdArgs := h.buildBatchInstallCommand(manager, toInstall, upgrade, extra)
	ec.Svc.Logger.Infof("  Installing packages: %s", strings.Join(toInstall, ", "))
	ec.Svc.Logger.Debugf("    Command: %s", strings.Join(cmdArgs, " "))

	output, execErr := h.runCmd(ec, become, cmdArgs)
	if execErr != nil {
		ec.Svc.Logger.Debugf("    Output: %s", strings.TrimSpace(string(output)))
		return nil, fmt.Errorf("failed to install packages %v: %w", toInstall, execErr)
	}

	result.SetChanged(true)

	ec.EmitEvent(events.EventPackageManaged, events.PackageManagedData{
		Manager:        manager,
		Installed:      toInstall,
		AlreadyPresent: existingPkgs,
	})

	return result, nil
}

// removePackages removes packages.
//
// Behavior: filter out packages that aren't installed, then issue a single
// batched manager invocation for the remaining set.
func (h *Handler) removePackages(ec *executor.ExecutionContext, manager string, packages []string, extra []string, become bool) (actions.Result, error) {
	result := executor.NewResult()

	var toRemove []string

	for _, pkg := range packages {
		installed, err := h.isPackageInstalled(ec, manager, pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to check if package %q is installed: %w", pkg, err)
		}

		if !installed {
			ec.Svc.Logger.Debugf("  Package %q is not installed", pkg)
			continue
		}

		toRemove = append(toRemove, pkg)
	}

	// Capture pre-mutation state for Reverse() (spec-22 phase 5 slice F).
	// toRemove is the exact list the apply will uninstall — and exactly
	// what reverse should re-install. Packages that weren't installed at
	// apply time are left out of Mutated and skipped on reverse too.
	result.ReverseData = &PkgReverseInfo{
		AppliedState: stateAbsent,
		Manager:      manager,
		Mutated:      append([]string(nil), toRemove...),
	}

	if len(toRemove) == 0 {
		ec.EmitEvent(events.EventPackageManaged, events.PackageManagedData{
			Manager: manager,
		})
		return result, nil
	}

	// winget doesn't batch with --id, so each package gets its own call.
	if manager == pmWinget {
		for _, pkg := range toRemove {
			cmdArgs := buildWingetCommand("uninstall", pkg, extra)
			ec.Svc.Logger.Infof("  Removing package: %s", pkg)
			ec.Svc.Logger.Debugf("    Command: %s", strings.Join(cmdArgs, " "))

			output, execErr := h.runCmd(ec, become, cmdArgs)
			if execErr != nil {
				ec.Svc.Logger.Debugf("    Output: %s", strings.TrimSpace(string(output)))
				return nil, fmt.Errorf("failed to remove package %q: %w", pkg, execErr)
			}
		}
		result.SetChanged(true)
		ec.EmitEvent(events.EventPackageManaged, events.PackageManagedData{
			Manager: manager,
			Removed: toRemove,
		})
		return result, nil
	}

	cmdArgs := h.buildBatchRemoveCommand(manager, toRemove, extra)
	ec.Svc.Logger.Infof("  Removing packages: %s", strings.Join(toRemove, ", "))
	ec.Svc.Logger.Debugf("    Command: %s", strings.Join(cmdArgs, " "))

	output, execErr := h.runCmd(ec, become, cmdArgs)
	if execErr != nil {
		ec.Svc.Logger.Debugf("    Output: %s", strings.TrimSpace(string(output)))
		return nil, fmt.Errorf("failed to remove packages %v: %w", toRemove, execErr)
	}

	result.SetChanged(true)

	ec.EmitEvent(events.EventPackageManaged, events.PackageManagedData{
		Manager: manager,
		Removed: toRemove,
	})

	return result, nil
}

// executeUpgrade upgrades all packages.
func (h *Handler) executeUpgrade(ec *executor.ExecutionContext, manager string, pkg *config.Package, become bool) (actions.Result, error) {
	result := executor.NewResult()

	cmdArgs := h.buildUpgradeCommand(manager, pkg.Extra)
	ec.Svc.Logger.Infof("  Upgrading all packages")
	ec.Svc.Logger.Debugf("    Command: %s", strings.Join(cmdArgs, " "))

	output, execErr := h.runCmd(ec, become, cmdArgs)
	if execErr != nil {
		ec.Svc.Logger.Debugf("    Output: %s", strings.TrimSpace(string(output)))
		return nil, fmt.Errorf("failed to upgrade packages: %w", execErr)
	}

	result.SetChanged(true)

	return result, nil
}

// isPackageInstalled checks if a package is installed.
func (h *Handler) isPackageInstalled(ec *executor.ExecutionContext, manager, pkg string) (bool, error) {
	// Build check command based on package manager
	var checkCmd []string

	switch manager {
	case pmApt:
		checkCmd = []string{"dpkg", "-s", pkg}
	case pmDnf, pmYum:
		checkCmd = []string{"rpm", "-q", pkg}
	case pmPacman:
		checkCmd = []string{pmPacman, "-Q", pkg}
	case pmZypper:
		checkCmd = []string{"rpm", "-q", pkg}
	case pmApk:
		checkCmd = []string{pmApk, "info", "-e", pkg}
	case pmBrew:
		checkCmd = []string{pmBrew, "list", pkg}
	case pmPort:
		checkCmd = []string{pmPort, "installed", pkg}
	case pmChoco:
		checkCmd = []string{pmChoco, "list", "--local-only", pkg}
	case pmScoop:
		checkCmd = []string{pmScoop, "list", pkg}
	case pmWinget:
		// winget list --exact --id <pkg> exits non-zero if the package is
		// not installed. --disable-interactivity prevents any prompt.
		checkCmd = []string{pmWinget, "list", "--exact", "--id", pkg, "--disable-interactivity"}
	default:
		return false, fmt.Errorf("unsupported package manager: %s", manager)
	}

	ec.Svc.Logger.Debugf("    Checking if installed: %s", strings.Join(checkCmd, " "))

	// Execute the check command
	cmd := exec.Command(checkCmd[0], checkCmd[1:]...) // #nosec G204 -- checkCmd built from validated package managers
	err := cmd.Run()

	// If command succeeds (exit code 0), package is installed
	return err == nil, nil
}

// buildInstallCommand builds the install command for a single package.
//
// Retained for backward compatibility with tests; production code paths use
// buildBatchInstallCommand. Equivalent to buildBatchInstallCommand with a
// single-element slice — same arg ordering.
//
//nolint:dupl,unparam,unused // Test-only helper retained for backward compatibility
func (h *Handler) buildInstallCommand(manager, pkg string, upgrade bool, extra []string) []string {
	return h.buildBatchInstallCommand(manager, []string{pkg}, upgrade, extra)
}

// buildBatchInstallCommand builds the install command for one or more packages.
//
// Arg ordering: <manager-base> <extra...> <pkgs...> — `extra` precedes the
// package list so the manager parses non-package flags before names.
//
// Pacman uses `--needed` so reinstall is skipped at the manager level even if
// the pre-check missed something concurrent.
//
//nolint:unparam // upgrade parameter preserved for future use (no-op today)
func (h *Handler) buildBatchInstallCommand(manager string, pkgs []string, upgrade bool, extra []string) []string {
	_ = upgrade
	base := installCommandBase(manager)
	cmd := make([]string, 0, len(base)+len(extra)+len(pkgs))
	cmd = append(cmd, base...)
	cmd = append(cmd, extra...)
	cmd = append(cmd, pkgs...)
	return cmd
}

// installCommandBase returns the manager-specific install command prefix.
//
// Note: winget is intentionally absent here. winget does not batch multiple
// packages in a single invocation when using --id (the safe, exact-match
// flag), so installPackages handles it per-package via buildWingetCommand.
func installCommandBase(manager string) []string {
	switch manager {
	case pmApt:
		return []string{"apt-get", "install", "-y"}
	case pmDnf:
		return []string{pmDnf, "install", "-y"}
	case pmYum:
		return []string{pmYum, "install", "-y"}
	case pmPacman:
		return []string{pmPacman, "-S", "--noconfirm", "--needed"}
	case pmZypper:
		return []string{pmZypper, "install", "-y"}
	case pmApk:
		return []string{pmApk, "add"}
	case pmBrew:
		return []string{pmBrew, "install"}
	case pmPort:
		return []string{pmPort, "install"}
	case pmChoco:
		return []string{pmChoco, "install", "-y"}
	case pmScoop:
		return []string{pmScoop, "install"}
	}
	return nil
}

// buildWingetCommand builds a per-package winget command for the given verb
// (install, uninstall, upgrade, list). Caller is responsible for invoking it
// once per package because winget --id only accepts a single ID per call.
//
// Standard automation flags are baked in: --exact (require ID match),
// --silent (no UI), and for install/upgrade --accept-package-agreements +
// --accept-source-agreements so the call doesn't hang waiting for EULA
// acceptance. Extra args are appended before --id so they can override
// behaviour without trampling the package identifier.
func buildWingetCommand(verb, pkg string, extra []string) []string {
	cmd := []string{pmWinget, verb, "--exact", "--silent"}
	if verb == "install" || verb == "upgrade" {
		cmd = append(cmd, "--accept-package-agreements", "--accept-source-agreements")
	}
	cmd = append(cmd, extra...)
	cmd = append(cmd, "--id", pkg)
	return cmd
}

// buildRemoveCommand builds the remove command for a single package.
//
// Retained for backward compatibility with tests; production code paths use
// buildBatchRemoveCommand.
//
//nolint:dupl,unused // Test-only helper retained for backward compatibility
func (h *Handler) buildRemoveCommand(manager, pkg string, extra []string) []string {
	return h.buildBatchRemoveCommand(manager, []string{pkg}, extra)
}

// buildBatchRemoveCommand builds the remove command for one or more packages.
func (h *Handler) buildBatchRemoveCommand(manager string, pkgs []string, extra []string) []string {
	base := removeCommandBase(manager)
	cmd := make([]string, 0, len(base)+len(extra)+len(pkgs))
	cmd = append(cmd, base...)
	cmd = append(cmd, extra...)
	cmd = append(cmd, pkgs...)
	return cmd
}

// removeCommandBase returns the manager-specific remove command prefix.
// winget is handled per-package via buildWingetCommand (see installCommandBase).
func removeCommandBase(manager string) []string {
	switch manager {
	case pmApt:
		return []string{"apt-get", "remove", "-y"}
	case pmDnf:
		return []string{pmDnf, "remove", "-y"}
	case pmYum:
		return []string{pmYum, "remove", "-y"}
	case pmPacman:
		return []string{pmPacman, "-R", "--noconfirm"}
	case pmZypper:
		return []string{pmZypper, "remove", "-y"}
	case pmApk:
		return []string{pmApk, "del"}
	case pmBrew:
		return []string{pmBrew, "uninstall"}
	case pmPort:
		return []string{pmPort, "uninstall"}
	case pmChoco:
		return []string{pmChoco, "uninstall", "-y"}
	case pmScoop:
		return []string{pmScoop, "uninstall"}
	}
	return nil
}

// buildUpgradeCommand builds the upgrade all command for a package manager.
func (h *Handler) buildUpgradeCommand(manager string, extra []string) []string {
	// Preallocate: base command (3) + extra
	cmd := make([]string, 0, 3+len(extra))

	switch manager {
	case pmApt:
		cmd = []string{"apt-get", "upgrade", "-y"}
	case pmDnf:
		cmd = []string{pmDnf, "upgrade", "-y"}
	case pmYum:
		cmd = []string{pmYum, "upgrade", "-y"}
	case pmPacman:
		cmd = []string{pmPacman, "-Syu", "--noconfirm"}
	case pmZypper:
		cmd = []string{pmZypper, "update", "-y"}
	case pmApk:
		cmd = []string{pmApk, "upgrade"}
	case pmBrew:
		cmd = []string{pmBrew, "upgrade"}
	case pmPort:
		cmd = []string{pmPort, "upgrade", "outdated"}
	case pmChoco:
		cmd = []string{pmChoco, "upgrade", "all", "-y"}
	case pmScoop:
		cmd = []string{pmScoop, "update", "*"}
	case pmWinget:
		cmd = []string{pmWinget, "upgrade", "--all", "--silent", "--accept-package-agreements", "--accept-source-agreements"}
	}

	// Add extra arguments
	cmd = append(cmd, extra...)

	return cmd
}

// Run is the Spec 16 unified entry point. It consults ctx.Mode() and
// either inspects which packages would change (ModePlan) or actually
// installs / removes / upgrades them (ModeApply).
//
// The shared preamble (manager detection, package-list building,
// template rendering, state normalization) runs once for both modes,
// eliminating any chance of the plan preview disagreeing with what
// execute would actually do.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	pkg := step.Pkg

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Shared preamble — same for both modes.
	manager, err := h.determinePackageManager(pkg.Manager, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to determine package manager: %w", err)
	}

	state := pkg.State
	if state == "" {
		state = statePresent
	}

	packages := h.buildPackageList(pkg)
	for i, name := range packages {
		if rendered, rerr := ctx.GetTemplate().Render(name, ctx.GetVariables()); rerr == nil {
			packages[i] = rendered
		}
	}

	if ctx.Mode() == actions.ModePlan {
		return h.runPlan(ec, manager, state, packages, pkg)
	}

	// ModeApply — preserve the legacy Execute behavior. Upgrade and
	// cache update both fall through to their existing helpers.
	result := executor.NewResult()
	result.SetChanged(false)

	if pkg.Upgrade {
		return h.executeUpgrade(ec, manager, pkg, step.ShouldBecome())
	}

	if pkg.UpdateCache {
		if err := h.updateCache(ec, manager, step.ShouldBecome()); err != nil {
			return nil, fmt.Errorf("failed to update package cache: %w", err)
		}
	}

	switch state {
	case statePresent, stateLatest:
		return h.installPackages(ec, manager, packages, state == stateLatest, pkg.Extra, step.ShouldBecome())
	case stateAbsent:
		return h.removePackages(ec, manager, packages, pkg.Extra, step.ShouldBecome())
	default:
		return nil, fmt.Errorf("unsupported state: %s", state)
	}
}

// runPlan inspects each package's installed status and aggregates a
// per-step prediction. Returns Checkable: true so the plan output
// shows accurate would-install / would-remove / already-matches.
func (h *Handler) runPlan(ec *executor.ExecutionContext, manager, state string, packages []string, pkg *config.Package) (actions.Result, error) {
	result := executor.NewResult()
	result.Checkable = true

	// Upgrade is not idempotent at the package-list level — we can't
	// know without running whether anything would update. Treat as
	// would-change.
	if pkg.Upgrade {
		result.WouldChange = true
		result.Reason = fmt.Sprintf("would upgrade packages via %s", manager)
		return result, nil
	}

	// First package that would change wins the reason string; the
	// inspection short-circuits as soon as a change is detected, which
	// matches the legacy Check behavior.
	for _, name := range packages {
		installed, err := h.isPackageInstalled(ec, manager, name)
		if err != nil {
			result.Reason = fmt.Sprintf("check error: %v", err)
			return result, nil
		}
		switch state {
		case statePresent, stateLatest:
			if !installed {
				result.WouldChange = true
				result.Reason = fmt.Sprintf("would install %s", name)
				return result, nil
			}
		case stateAbsent:
			if installed {
				result.WouldChange = true
				result.Reason = fmt.Sprintf("would remove %s", name)
				return result, nil
			}
		}
	}

	result.Reason = "already in desired state"
	return result, nil
}
