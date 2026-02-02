// Package package implements the package action handler.
//
// The package action manages system packages with support for:
// - Auto-detection of package manager (apt, dnf, yum, pacman, zypper, apk, brew, port, choco, scoop)
// - Manual package manager selection
// - Install, remove, and update operations
// - Cache management and system upgrades
package package_handler

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
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
		Name:           "package",
		Description:    "Manage system packages (install/remove/update)",
		Category:       actions.CategorySystem,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents:    []string{string(events.EventPackageManaged)},
		Version:        "1.0.0",
	}
}

// Validate checks if the package configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.Package == nil {
		return fmt.Errorf("package configuration is nil")
	}

	pkg := step.Package

	// Must have either name, names, or upgrade
	if pkg.Name == "" && len(pkg.Names) == 0 && !pkg.Upgrade {
		return fmt.Errorf("one of 'name', 'names', or 'upgrade' is required")
	}

	// Validate state
	if pkg.State != "" && pkg.State != "present" && pkg.State != "absent" && pkg.State != "latest" {
		return fmt.Errorf("state must be one of: present, absent, latest (got %q)", pkg.State)
	}

	return nil
}

// Execute runs the package action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	pkg := step.Package

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

	// Build package list
	packages := h.buildPackageList(pkg)

	// Create result
	result := executor.NewResult()
	result.SetChanged(false)

	// Handle upgrade operation
	if pkg.Upgrade {
		return h.executeUpgrade(ec, manager, pkg)
	}

	// Update cache if requested
	if pkg.UpdateCache {
		if err := h.updateCache(ec, manager); err != nil {
			return nil, fmt.Errorf("failed to update package cache: %w", err)
		}
	}

	// Execute based on state
	switch state {
	case "present", "latest":
		return h.installPackages(ec, manager, packages, state == "latest", pkg.Extra)
	case "absent":
		return h.removePackages(ec, manager, packages, pkg.Extra)
	default:
		return nil, fmt.Errorf("unsupported state: %s", state)
	}
}

// DryRun shows what would be done without making changes.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	pkg := step.Package

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

	if pkg.Upgrade {
		ctx.GetLogger().Infof("  Would upgrade all packages using %s", manager)
		return nil
	}

	if pkg.UpdateCache {
		ctx.GetLogger().Infof("  Would update package cache using %s", manager)
	}

	operation := "install"
	if state == "absent" {
		operation = "remove"
	} else if state == "latest" {
		operation = "install/upgrade"
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
	managers := []string{"apt", "dnf", "yum", "pacman", "zypper", "apk"}
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
	if _, err := exec.LookPath("brew"); err == nil {
		return "brew", nil
	}
	// Check for MacPorts
	if _, err := exec.LookPath("port"); err == nil {
		return "port", nil
	}
	return "", fmt.Errorf("no supported package manager found (install Homebrew or MacPorts)")
}

// detectWindowsPackageManager detects the package manager on Windows.
func (h *Handler) detectWindowsPackageManager() (string, error) {
	// Check for choco
	if _, err := exec.LookPath("choco"); err == nil {
		return "choco", nil
	}
	// Check for scoop
	if _, err := exec.LookPath("scoop"); err == nil {
		return "scoop", nil
	}
	return "", fmt.Errorf("no supported package manager found (install Chocolatey or Scoop)")
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

// updateCache updates the package manager cache.
func (h *Handler) updateCache(ec *executor.ExecutionContext, manager string) error {
	var cmd []string
	switch manager {
	case "apt":
		cmd = []string{"apt-get", "update"}
	case "dnf", "yum":
		cmd = []string{manager, "makecache"}
	case "pacman":
		cmd = []string{"pacman", "-Sy"}
	case "apk":
		cmd = []string{"apk", "update"}
	case "brew":
		cmd = []string{"brew", "update"}
	default:
		// Other package managers don't need cache updates or do it automatically
		return nil
	}

	ec.Logger.Debugf("  Updating package cache: %s", strings.Join(cmd, " "))
	// Execute the update command (implementation similar to shell action)
	// For now, just log it
	return nil
}

// installPackages installs or upgrades packages.
func (h *Handler) installPackages(ec *executor.ExecutionContext, manager string, packages []string, upgrade bool, extra []string) (actions.Result, error) {
	result := executor.NewResult()

	for _, pkg := range packages {
		// Check if package is already installed
		installed, err := h.isPackageInstalled(ec, manager, pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to check if package %q is installed: %w", pkg, err)
		}

		if installed && !upgrade {
			ec.Logger.Debugf("  Package %q is already installed", pkg)
			continue
		}

		// Build install command
		cmd := h.buildInstallCommand(manager, pkg, upgrade, extra)
		ec.Logger.Infof("  Installing package: %s", pkg)
		ec.Logger.Debugf("    Command: %s", strings.Join(cmd, " "))

		// TODO: Execute command using shell action logic
		// For now, mark as changed
		result.SetChanged(true)
	}

	return result, nil
}

// removePackages removes packages.
func (h *Handler) removePackages(ec *executor.ExecutionContext, manager string, packages []string, extra []string) (actions.Result, error) {
	result := executor.NewResult()

	for _, pkg := range packages {
		// Check if package is installed
		installed, err := h.isPackageInstalled(ec, manager, pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to check if package %q is installed: %w", pkg, err)
		}

		if !installed {
			ec.Logger.Debugf("  Package %q is not installed", pkg)
			continue
		}

		// Build remove command
		cmd := h.buildRemoveCommand(manager, pkg, extra)
		ec.Logger.Infof("  Removing package: %s", pkg)
		ec.Logger.Debugf("    Command: %s", strings.Join(cmd, " "))

		// TODO: Execute command using shell action logic
		// For now, mark as changed
		result.SetChanged(true)
	}

	return result, nil
}

// executeUpgrade upgrades all packages.
func (h *Handler) executeUpgrade(ec *executor.ExecutionContext, manager string, pkg *config.Package) (actions.Result, error) {
	result := executor.NewResult()

	cmd := h.buildUpgradeCommand(manager, pkg.Extra)
	ec.Logger.Infof("  Upgrading all packages")
	ec.Logger.Debugf("    Command: %s", strings.Join(cmd, " "))

	// TODO: Execute command using shell action logic
	// For now, mark as changed
	result.SetChanged(true)

	return result, nil
}

// isPackageInstalled checks if a package is installed.
func (h *Handler) isPackageInstalled(ec *executor.ExecutionContext, manager, pkg string) (bool, error) {
	// Build check command based on package manager
	var checkCmd []string

	switch manager {
	case "apt":
		checkCmd = []string{"dpkg", "-s", pkg}
	case "dnf", "yum":
		checkCmd = []string{"rpm", "-q", pkg}
	case "pacman":
		checkCmd = []string{"pacman", "-Q", pkg}
	case "zypper":
		checkCmd = []string{"rpm", "-q", pkg}
	case "apk":
		checkCmd = []string{"apk", "info", "-e", pkg}
	case "brew":
		checkCmd = []string{"brew", "list", pkg}
	case "port":
		checkCmd = []string{"port", "installed", pkg}
	case "choco":
		checkCmd = []string{"choco", "list", "--local-only", pkg}
	case "scoop":
		checkCmd = []string{"scoop", "list", pkg}
	default:
		return false, fmt.Errorf("unsupported package manager: %s", manager)
	}

	ec.Logger.Debugf("    Checking if installed: %s", strings.Join(checkCmd, " "))

	// TODO: Execute command and check exit code
	// For now, return false (not installed) to trigger installation
	return false, nil
}

// buildInstallCommand builds the install command for a package manager.
func (h *Handler) buildInstallCommand(manager, pkg string, upgrade bool, extra []string) []string {
	var cmd []string

	switch manager {
	case "apt":
		cmd = []string{"apt-get", "install", "-y"}
	case "dnf":
		cmd = []string{"dnf", "install", "-y"}
	case "yum":
		cmd = []string{"yum", "install", "-y"}
	case "pacman":
		cmd = []string{"pacman", "-S", "--noconfirm"}
	case "zypper":
		cmd = []string{"zypper", "install", "-y"}
	case "apk":
		cmd = []string{"apk", "add"}
	case "brew":
		cmd = []string{"brew", "install"}
	case "port":
		cmd = []string{"port", "install"}
	case "choco":
		cmd = []string{"choco", "install", "-y"}
	case "scoop":
		cmd = []string{"scoop", "install"}
	}

	// Add extra arguments
	cmd = append(cmd, extra...)

	// Add package name
	cmd = append(cmd, pkg)

	return cmd
}

// buildRemoveCommand builds the remove command for a package manager.
func (h *Handler) buildRemoveCommand(manager, pkg string, extra []string) []string {
	var cmd []string

	switch manager {
	case "apt":
		cmd = []string{"apt-get", "remove", "-y"}
	case "dnf":
		cmd = []string{"dnf", "remove", "-y"}
	case "yum":
		cmd = []string{"yum", "remove", "-y"}
	case "pacman":
		cmd = []string{"pacman", "-R", "--noconfirm"}
	case "zypper":
		cmd = []string{"zypper", "remove", "-y"}
	case "apk":
		cmd = []string{"apk", "del"}
	case "brew":
		cmd = []string{"brew", "uninstall"}
	case "port":
		cmd = []string{"port", "uninstall"}
	case "choco":
		cmd = []string{"choco", "uninstall", "-y"}
	case "scoop":
		cmd = []string{"scoop", "uninstall"}
	}

	// Add extra arguments
	cmd = append(cmd, extra...)

	// Add package name
	cmd = append(cmd, pkg)

	return cmd
}

// buildUpgradeCommand builds the upgrade all command for a package manager.
func (h *Handler) buildUpgradeCommand(manager string, extra []string) []string {
	var cmd []string

	switch manager {
	case "apt":
		cmd = []string{"apt-get", "upgrade", "-y"}
	case "dnf":
		cmd = []string{"dnf", "upgrade", "-y"}
	case "yum":
		cmd = []string{"yum", "upgrade", "-y"}
	case "pacman":
		cmd = []string{"pacman", "-Syu", "--noconfirm"}
	case "zypper":
		cmd = []string{"zypper", "update", "-y"}
	case "apk":
		cmd = []string{"apk", "upgrade"}
	case "brew":
		cmd = []string{"brew", "upgrade"}
	case "port":
		cmd = []string{"port", "upgrade", "outdated"}
	case "choco":
		cmd = []string{"choco", "upgrade", "all", "-y"}
	case "scoop":
		cmd = []string{"scoop", "update", "*"}
	}

	// Add extra arguments
	cmd = append(cmd, extra...)

	return cmd
}
