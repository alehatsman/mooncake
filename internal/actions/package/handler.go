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
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
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
	if pkg.State != "" && pkg.State != statePresent && pkg.State != stateAbsent && pkg.State != stateLatest {
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
	case statePresent, "latest":
		return h.installPackages(ec, manager, packages, state == "latest", pkg.Extra)
	case stateAbsent:
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
func (h *Handler) detectWindowsPackageManager() (string, error) {
	// Check for choco
	if _, err := exec.LookPath(pmChoco); err == nil {
		return pmChoco, nil
	}
	// Check for scoop
	if _, err := exec.LookPath(pmScoop); err == nil {
		return pmScoop, nil
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

	ec.Logger.Debugf("  Updating package cache: %s", strings.Join(cmdArgs, " "))

	// Execute the update command
	// #nosec G204 - Package manager commands are validated
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		ec.Logger.Debugf("    Output: %s", strings.TrimSpace(string(output)))
		return fmt.Errorf("failed to update package cache: %w", err)
	}

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
		cmdArgs := h.buildInstallCommand(manager, pkg, upgrade, extra)
		ec.Logger.Infof("  Installing package: %s", pkg)
		ec.Logger.Debugf("    Command: %s", strings.Join(cmdArgs, " "))

		// Execute the install command
		// #nosec G204 - Package manager commands are validated
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		output, execErr := cmd.CombinedOutput()

		if execErr != nil {
			ec.Logger.Debugf("    Output: %s", strings.TrimSpace(string(output)))
			return nil, fmt.Errorf("failed to install package %q: %w", pkg, execErr)
		}

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
		cmdArgs := h.buildRemoveCommand(manager, pkg, extra)
		ec.Logger.Infof("  Removing package: %s", pkg)
		ec.Logger.Debugf("    Command: %s", strings.Join(cmdArgs, " "))

		// Execute the remove command
		// #nosec G204 - Package manager commands are validated
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		output, execErr := cmd.CombinedOutput()

		if execErr != nil {
			ec.Logger.Debugf("    Output: %s", strings.TrimSpace(string(output)))
			return nil, fmt.Errorf("failed to remove package %q: %w", pkg, execErr)
		}

		result.SetChanged(true)
	}

	return result, nil
}

// executeUpgrade upgrades all packages.
func (h *Handler) executeUpgrade(ec *executor.ExecutionContext, manager string, pkg *config.Package) (actions.Result, error) {
	result := executor.NewResult()

	cmdArgs := h.buildUpgradeCommand(manager, pkg.Extra)
	ec.Logger.Infof("  Upgrading all packages")
	ec.Logger.Debugf("    Command: %s", strings.Join(cmdArgs, " "))

	// Execute the upgrade command
	// #nosec G204 - Package manager commands are validated
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	output, execErr := cmd.CombinedOutput()

	if execErr != nil {
		ec.Logger.Debugf("    Output: %s", strings.TrimSpace(string(output)))
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
	default:
		return false, fmt.Errorf("unsupported package manager: %s", manager)
	}

	ec.Logger.Debugf("    Checking if installed: %s", strings.Join(checkCmd, " "))

	// Execute the check command
	cmd := exec.Command(checkCmd[0], checkCmd[1:]...) // #nosec G204 -- checkCmd built from validated package managers
	err := cmd.Run()

	// If command succeeds (exit code 0), package is installed
	return err == nil, nil
}

// buildInstallCommand builds the install command for a package manager.
//nolint:dupl,unparam // Similar structure to buildRemoveCommand; upgrade parameter for future use
func (h *Handler) buildInstallCommand(manager, pkg string, upgrade bool, extra []string) []string {
	// Preallocate: base command (3) + extra + package name (1)
	cmd := make([]string, 0, 3+len(extra)+1)

	switch manager {
	case pmApt:
		cmd = []string{"apt-get", "install", "-y"}
	case pmDnf:
		cmd = []string{pmDnf, "install", "-y"}
	case pmYum:
		cmd = []string{pmYum, "install", "-y"}
	case pmPacman:
		cmd = []string{pmPacman, "-S", "--noconfirm"}
	case pmZypper:
		cmd = []string{pmZypper, "install", "-y"}
	case pmApk:
		cmd = []string{pmApk, "add"}
	case pmBrew:
		cmd = []string{pmBrew, "install"}
	case pmPort:
		cmd = []string{pmPort, "install"}
	case pmChoco:
		cmd = []string{pmChoco, "install", "-y"}
	case pmScoop:
		cmd = []string{pmScoop, "install"}
	}

	// Add extra arguments
	cmd = append(cmd, extra...)

	// Add package name
	cmd = append(cmd, pkg)

	return cmd
}

// buildRemoveCommand builds the remove command for a package manager.
//nolint:dupl // Similar structure to buildInstallCommand but different semantics
func (h *Handler) buildRemoveCommand(manager, pkg string, extra []string) []string {
	// Preallocate: base command (3) + extra + package name (1)
	cmd := make([]string, 0, 3+len(extra)+1)

	switch manager {
	case pmApt:
		cmd = []string{"apt-get", "remove", "-y"}
	case pmDnf:
		cmd = []string{pmDnf, "remove", "-y"}
	case pmYum:
		cmd = []string{pmYum, "remove", "-y"}
	case pmPacman:
		cmd = []string{pmPacman, "-R", "--noconfirm"}
	case pmZypper:
		cmd = []string{pmZypper, "remove", "-y"}
	case pmApk:
		cmd = []string{pmApk, "del"}
	case pmBrew:
		cmd = []string{pmBrew, "uninstall"}
	case pmPort:
		cmd = []string{pmPort, "uninstall"}
	case pmChoco:
		cmd = []string{pmChoco, "uninstall", "-y"}
	case pmScoop:
		cmd = []string{pmScoop, "uninstall"}
	}

	// Add extra arguments
	cmd = append(cmd, extra...)

	// Add package name
	cmd = append(cmd, pkg)

	return cmd
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
	}

	// Add extra arguments
	cmd = append(cmd, extra...)

	return cmd
}
