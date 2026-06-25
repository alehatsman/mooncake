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
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/sandbox"
	"github.com/alehatsman/mooncake/internal/security"
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
	// yay and paru are pacman-compatible AUR wrappers on Arch
	// (proposal-07). They mirror pacman's CLI verbs exactly
	// (-S/-R/-Q/-Sy/-Syu plus --noconfirm --needed), so each
	// per-manager switch routes them through the pacman branch
	// with the binary name swapped. Deliberately NOT in the
	// auto-detection list: AUR is a parallel ecosystem and Arch
	// users have strong opinions; require explicit opt-in via
	// `pkg.install: manager: yay`.
	pmYay  = "yay"
	pmParu = "paru"
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
	executor.RegisterReverseDataType("PkgReverseInfo", func() any { return &PkgReverseInfo{} })
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
		Examples: []string{
			`# Install a single package (auto-detects manager: apt/dnf/brew/...)
- name: Install ripgrep
  pkg:
    name: ripgrep
    state: present`,
			`# Install several with a cache refresh
- name: Dev essentials
  pkg:
    names: [git, make, jq, curl]
    state: present
    update_cache: true`,
			`# Upgrade an already-installed package to latest
- pkg:
    name: nginx
    state: latest`,
		},
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
//
// F049: Sudo depends on the resolved manager. yay/paru (AUR
// wrappers) and brew (user-prefix installer) refuse root by design;
// declaring Sudo:true for them makes the preflight reject any
// non-elevated invocation and forces operators into an as_user:root
// workaround that the manager itself then refuses at runtime.
//
// Spec-72 follow-up: auto-detect (empty step.Pkg.Manager) now also
// goes through resolveManager so a macOS host with brew on PATH
// gets Sudo:false at preflight — closing the gap where
// `pkg: {name: foo}` (no explicit manager) plus `as_user: root`
// would pass preflight and then fail inside the brew driver with
// "Running Homebrew as root is extremely dangerous". When
// resolveManager errors (no manager on PATH), we keep Sudo:true as
// the safer default — preflight then fails with a manager-detection
// hint via Run anyway.
func (h *Handler) Permissions(step *config.Step) actions.PermissionSet {
	manager := ""
	if step != nil && step.Pkg != nil {
		manager = step.Pkg.Manager
	}
	if manager == "" {
		if resolved, err := h.determinePackageManager(manager, nil); err == nil {
			manager = resolved
		}
	}
	return actions.PermissionSet{
		Sudo:    managerRequiresSudo(manager),
		Network: true,
	}
}

// managerRequiresSudo encodes the per-manager elevation policy used
// by Permissions(). Exposed for tests; the auto-detect (empty) case
// returns true so the preflight refuses unelevated runs.
func managerRequiresSudo(manager string) bool {
	switch manager {
	case "yay", "paru", "brew":
		return false
	default:
		return true
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

	// cask: true is only meaningful for Homebrew
	if pkg.Cask && pkg.Manager != "" && pkg.Manager != pmBrew {
		return fmt.Errorf("cask: true requires manager: brew (got %q)", pkg.Manager)
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

// renderPackageNames renders each entry in `names` against the template
// engine using ctx variables. Returns the error from the first failing
// entry so a missing-variable template doesn't reach apt/yum as a
// literal `{{ var }}-tools` (F023) — apt would surface a confusing
// "unable to locate package {{ var }}-tools" error instead of a clear
// template-render failure.
func (h *Handler) renderPackageNames(ctx actions.Context, names []string) ([]string, error) {
	rendered := make([]string, len(names))
	for i, name := range names {
		out, err := ctx.Template().Render(name, ctx.Variables())
		if err != nil {
			return nil, fmt.Errorf("render package name %q: %w", name, err)
		}
		rendered[i] = out
	}
	return rendered, nil
}

// resolveNamesExpr renders pkg.NamesExpr against the current variables and
// converts the result into a []string. Variables that hold a slice are
// matched without a Pongo2 round-trip (preserves typing); otherwise the
// template renders to a string and the shared resolver parses it.
func (h *Handler) resolveNamesExpr(ctx actions.Context, expr string) ([]string, error) {
	vars := ctx.Variables()
	evaluator := ctx.Evaluator()

	trimmed := strings.TrimSpace(expr)
	if inner, ok := stripTemplateWrapper(trimmed); ok {
		if items, err := template.ResolveStringList(inner, vars, evaluator); err == nil {
			return items, nil
		}
	}

	rendered, renderErr := ctx.Template().Render(expr, vars)
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
	case pmPacman, pmYay, pmParu:
		cmdArgs = []string{manager, "-Sy"}
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
		return pkgCmdError("failed to update package cache", err, output)
	}

	return nil
}

// runCmd executes a command, wrapping with sudo -S when become is true.
//
// F005: now uses security.BecomeRunner so IsBecomeSupported +
// SudoPass-not-empty validation matches every other become-aware
// helper. Pre-fix runCmd checked neither — a Windows operator
// invoking `pkg: ... become: true` hit "executable file not found"
// from exec.Command("sudo", ...) instead of a clean
// ErrBecomeUnsupported; an empty SudoPass produced an unprintable
// `"\n"` on sudo's stdin and let sudo hang on its TTY prompt.
//
// Spec-72 phase 2 (resolves spec-69's deferred phase-5 audit):
// migrated from direct security.BecomeRunner{} construction to
// ctx.Privileged().RunWithBecome(...). The per-call `become bool`
// decided by the step's `become:` field (brew=false, apt=true) flows
// through the new RunWithBecome method added on actions.PrivilegedRunner.
// ctx.Privileged() reads SudoPass + Escalation from RunServices in
// one place, so the constructor can't drift from the rest of the
// codebase the way it did during the 2026-05-18 sudo-fragmentation
// incident (F051).
func (h *Handler) runCmd(ec *executor.ExecutionContext, become bool, cmdArgs []string) ([]byte, error) {
	if len(cmdArgs) == 0 {
		return nil, fmt.Errorf("pkg.runCmd: cmdArgs must not be empty")
	}
	// Spec-72 Layer C: become decision lives in step.AsUser, bound
	// at dispatch onto ec.Privileged(). The per-call `become` arg is
	// no longer threaded through helper signatures; this method's
	// `become` param is retained as a no-op while callers are
	// audited and dropped in a follow-up. The right-shape semantics
	// hold today: brew steps lack as_user → no sudo; apt steps with
	// as_user: root → sudo to root.
	_ = become
	// Under a hardened agentd (ProtectSystem=yes makes /usr read-only),
	// apt/dpkg writes to /usr fail with EROFS. sandbox.Wrap routes the
	// command through a transient systemd-run service that escapes the
	// service's mount namespace; it's a no-op for direct `mooncake apply`
	// and non-sandboxed agentds. See issue #139.
	args := sandbox.Wrap(cmdArgs)
	return ec.Privileged().Run(ec.Svc.Ctx, args[0], args[1:]...)
}

// maxErrOutputLines bounds how much command output is folded into a failure
// error. Package managers (notably brew) emit hundreds of progress lines
// before the real error; the cause almost always lives in the tail.
const maxErrOutputLines = 20

// tailOutput returns the trimmed last n lines of a command's combined
// output, or "" when there is none.
func tailOutput(output []byte, n int) string {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// sudoKeepaliveInterval is how often startSudoKeepalive refreshes the sudo
// timestamp. Comfortably under sudo's default timestamp_timeout (5 min) so a
// long cask batch never lapses between refreshes.
const sudoKeepaliveInterval = 60 * time.Second

// startSudoKeepalive seeds the sudo timestamp and then refreshes it on an
// interval until the returned stop function is called (or the run context is
// cancelled). It covers brew cask batches that run longer than sudo's
// timestamp_timeout — a single seed would lapse mid-batch and the next
// pkg-based cask would prompt again. See warmSudoTimestamp for why brew casks
// need this at all.
//
// Returns an idempotent stop function; callers `defer stop()`. When there's
// nothing to keep alive (no password configured) the stop is a no-op and no
// goroutine is started.
func (h *Handler) startSudoKeepalive(ec *executor.ExecutionContext) func() {
	if ec.Svc.SudoPass == "" {
		return func() {}
	}
	// Seed immediately so the first cask doesn't prompt before the first tick.
	h.warmSudoTimestamp(ec)

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(sudoKeepaliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ec.Svc.Ctx.Done():
				return
			case <-ticker.C:
				h.warmSudoTimestamp(ec)
			}
		}
	}()

	var once sync.Once
	return func() { once.Do(func() { close(done) }) }
}

// warmSudoTimestamp pre-authenticates the user's cached sudo credential
// ahead of a brew cask install. brew runs casks unprivileged and shells out
// to `sudo` itself for pkg-based or privileged casks; without a warm
// timestamp the operator is prompted "Password:" once per such cask on the
// inherited terminal — even though mooncake already holds the password from
// --ask-become-pass. Running `sudo -S -v` with that password seeds the
// timestamp so brew's child sudo calls reuse it silently.
//
// Best-effort and non-fatal: it only acts in password mode (SudoPass set),
// and any failure is logged at Debug while the install proceeds — brew then
// falls back to prompting exactly as before. The step itself stays
// unprivileged (brew must never run as root); this only touches the sudo
// credential cache.
func (h *Handler) warmSudoTimestamp(ec *executor.ExecutionContext) {
	if ec.Svc.SudoPass == "" {
		// Passwordless sudo (NOPASSWD) or no password configured: brew's
		// own sudo won't block on a prompt, so there's nothing to seed.
		return
	}
	warm := &security.Privileged{
		SudoPass:   ec.Svc.SudoPass,
		Escalation: ec.Svc.Escalation,
		AsUser:     "root",
	}
	// `sudo -S -v` validates the credential and refreshes the timestamp
	// without running a command.
	if _, err := warm.Run(ec.Svc.Ctx, "-v"); err != nil {
		ec.Svc.Logger.Debugf("  sudo timestamp warmup failed (brew casks may prompt for a password): %v", err)
	}
}

// pkgCmdError wraps a failed package-manager command, folding the tail of its
// combined output into the error so the actual cause (e.g. "untrusted tap",
// "No available formula with the name ...") surfaces in the step failure and
// RECAP instead of a bare "exit status 1". Falls back to the plain wrapped
// error when the command produced no output.
func pkgCmdError(msg string, execErr error, output []byte) error {
	if detail := tailOutput(output, maxErrOutputLines); detail != "" {
		return fmt.Errorf("%s: %w\n%s", msg, execErr, detail)
	}
	return fmt.Errorf("%s: %w", msg, execErr)
}

// installPackages installs or upgrades packages.
//
// Behavior: partition into existing vs. to-install via per-package check,
// then issue a single batched manager invocation for the to-install set.
func (h *Handler) installPackages(ec *executor.ExecutionContext, manager string, packages []string, upgrade bool, cask bool, extra []string, become bool) (actions.Result, error) {
	result := executor.NewResult()

	var toInstall, existingPkgs []string

	// For brew, fetch the full installed set in one subprocess instead of one per package.
	var brewInstalled map[string]struct{}
	if manager == pmBrew {
		var err error
		brewInstalled, err = h.fetchBrewInstalledSet(ec, cask)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch brew installed set: %w", err)
		}
	}

	for _, pkg := range packages {
		var installed bool
		if brewInstalled != nil {
			_, installed = brewInstalled[pkg]
		} else {
			var err error
			installed, err = h.isPackageInstalled(ec, manager, pkg, cask)
			if err != nil {
				return nil, fmt.Errorf("failed to check if package %q is installed: %w", pkg, err)
			}
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
		Cask:         cask,
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
				return nil, pkgCmdError(fmt.Sprintf("failed to install package %q", pkg), execErr, output)
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

	if manager == pmBrew && cask {
		// brew installs casks unprivileged and shells out to its own
		// `sudo` for pkg-based / privileged casks. Keep the sudo
		// timestamp warm for the whole batch so those child invocations
		// reuse the cached credential instead of prompting "Password:"
		// once per cask.
		stop := h.startSudoKeepalive(ec)
		defer stop()
	}

	cmdArgs := h.buildBatchInstallCommand(manager, toInstall, upgrade, cask, extra)
	ec.Svc.Logger.Infof("  Installing packages: %s", strings.Join(toInstall, ", "))
	ec.Svc.Logger.Debugf("    Command: %s", strings.Join(cmdArgs, " "))

	output, execErr := h.runCmd(ec, become, cmdArgs)
	if execErr != nil {
		return nil, pkgCmdError(fmt.Sprintf("failed to install packages %v", toInstall), execErr, output)
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
func (h *Handler) removePackages(ec *executor.ExecutionContext, manager string, packages []string, cask bool, extra []string, become bool) (actions.Result, error) {
	result := executor.NewResult()

	var toRemove []string

	// For brew, fetch the full installed set in one subprocess instead of one per package.
	var brewInstalled map[string]struct{}
	if manager == pmBrew {
		var err error
		brewInstalled, err = h.fetchBrewInstalledSet(ec, cask)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch brew installed set: %w", err)
		}
	}

	for _, pkg := range packages {
		var installed bool
		if brewInstalled != nil {
			_, installed = brewInstalled[pkg]
		} else {
			var err error
			installed, err = h.isPackageInstalled(ec, manager, pkg, cask)
			if err != nil {
				return nil, fmt.Errorf("failed to check if package %q is installed: %w", pkg, err)
			}
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
		Cask:         cask,
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
				return nil, pkgCmdError(fmt.Sprintf("failed to remove package %q", pkg), execErr, output)
			}
		}
		result.SetChanged(true)
		ec.EmitEvent(events.EventPackageManaged, events.PackageManagedData{
			Manager: manager,
			Removed: toRemove,
		})
		return result, nil
	}

	cmdArgs := h.buildBatchRemoveCommand(manager, toRemove, cask, extra)
	ec.Svc.Logger.Infof("  Removing packages: %s", strings.Join(toRemove, ", "))
	ec.Svc.Logger.Debugf("    Command: %s", strings.Join(cmdArgs, " "))

	output, execErr := h.runCmd(ec, become, cmdArgs)
	if execErr != nil {
		return nil, pkgCmdError(fmt.Sprintf("failed to remove packages %v", toRemove), execErr, output)
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
		return nil, pkgCmdError("failed to upgrade packages", execErr, output)
	}

	result.SetChanged(true)

	return result, nil
}

// isPackageInstalled checks if a package is installed.
func (h *Handler) isPackageInstalled(ec *executor.ExecutionContext, manager, pkg string, cask bool) (bool, error) {
	// Build check command based on package manager
	var checkCmd []string

	switch manager {
	case pmApt:
		checkCmd = []string{"dpkg", "-s", pkg}
	case pmDnf, pmYum:
		checkCmd = []string{"rpm", "-q", pkg}
	case pmPacman, pmYay, pmParu:
		checkCmd = []string{manager, "-Q", pkg}
	case pmZypper:
		checkCmd = []string{"rpm", "-q", pkg}
	case pmApk:
		checkCmd = []string{pmApk, "info", "-e", pkg}
	case pmBrew:
		if cask {
			checkCmd = []string{pmBrew, "list", "--cask", pkg}
		} else {
			checkCmd = []string{pmBrew, "list", pkg}
		}
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

// fetchBrewInstalledSet runs `brew list --formula` or `brew list --cask` once
// and returns the installed package names as a set. This allows callers to
// replace N per-package subprocess calls with two bulk calls total.
func (h *Handler) fetchBrewInstalledSet(ec *executor.ExecutionContext, cask bool) (map[string]struct{}, error) {
	var args []string
	if cask {
		args = []string{pmBrew, "list", "--cask"}
	} else {
		args = []string{pmBrew, "list", "--formula"}
	}
	ec.Svc.Logger.Debugf("    Fetching installed brew %s list", map[bool]string{true: "casks", false: "formulae"}[cask])
	out, err := h.runCmd(ec, false, args)
	if err != nil {
		return nil, fmt.Errorf("brew list failed: %w", err)
	}
	set := make(map[string]struct{})
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			set[name] = struct{}{}
		}
	}
	return set, nil
}

// buildInstallCommand builds the install command for a single package.
//
// Retained for backward compatibility with tests; production code paths use
// buildBatchInstallCommand. Equivalent to buildBatchInstallCommand with a
// single-element slice — same arg ordering.
//
//nolint:dupl,unparam,unused // Test-only helper retained for backward compatibility
func (h *Handler) buildInstallCommand(manager, pkg string, upgrade bool, cask bool, extra []string) []string {
	return h.buildBatchInstallCommand(manager, []string{pkg}, upgrade, cask, extra)
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
func (h *Handler) buildBatchInstallCommand(manager string, pkgs []string, upgrade bool, cask bool, extra []string) []string {
	_ = upgrade
	base := installCommandBase(manager, cask)
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
func installCommandBase(manager string, cask bool) []string {
	switch manager {
	case pmApt:
		return []string{"apt-get", "install", "-y"}
	case pmDnf:
		return []string{pmDnf, "install", "-y"}
	case pmYum:
		return []string{pmYum, "install", "-y"}
	case pmPacman, pmYay, pmParu:
		return []string{manager, "-S", "--noconfirm", "--needed"}
	case pmZypper:
		return []string{pmZypper, "install", "-y"}
	case pmApk:
		return []string{pmApk, "add"}
	case pmBrew:
		if cask {
			return []string{pmBrew, "install", "--cask"}
		}
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
func (h *Handler) buildRemoveCommand(manager, pkg string, cask bool, extra []string) []string {
	return h.buildBatchRemoveCommand(manager, []string{pkg}, cask, extra)
}

// buildBatchRemoveCommand builds the remove command for one or more packages.
func (h *Handler) buildBatchRemoveCommand(manager string, pkgs []string, cask bool, extra []string) []string {
	base := removeCommandBase(manager, cask)
	cmd := make([]string, 0, len(base)+len(extra)+len(pkgs))
	cmd = append(cmd, base...)
	cmd = append(cmd, extra...)
	cmd = append(cmd, pkgs...)
	return cmd
}

// removeCommandBase returns the manager-specific remove command prefix.
// winget is handled per-package via buildWingetCommand (see installCommandBase).
func removeCommandBase(manager string, cask bool) []string {
	switch manager {
	case pmApt:
		return []string{"apt-get", "remove", "-y"}
	case pmDnf:
		return []string{pmDnf, "remove", "-y"}
	case pmYum:
		return []string{pmYum, "remove", "-y"}
	case pmPacman, pmYay, pmParu:
		return []string{manager, "-R", "--noconfirm"}
	case pmZypper:
		return []string{pmZypper, "remove", "-y"}
	case pmApk:
		return []string{pmApk, "del"}
	case pmBrew:
		if cask {
			return []string{pmBrew, "uninstall", "--cask"}
		}
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
	case pmPacman, pmYay, pmParu:
		cmd = []string{manager, "-Syu", "--noconfirm"}
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
// RunRaw signals spec-69 RawRunner participation so user-declared
// `retry:` on a pkg step actually retries — useful when apt locks
// or mirror flakes cause transient failures. Run is idempotent
// (state=present skips already-installed pkgs).
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

// eliminating any chance of the plan preview disagreeing with what
// execute would actually do.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	pkg := step.Pkg

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Shared preamble — same for both modes.
	manager, err := h.determinePackageManager(pkg.Manager, ctx.Variables())
	if err != nil {
		return nil, fmt.Errorf("failed to determine package manager: %w", err)
	}

	state := pkg.State
	if state == "" {
		state = statePresent
	}

	packages, err := h.renderPackageNames(ctx, h.buildPackageList(pkg))
	if err != nil {
		return nil, err
	}

	if pkg.NamesExpr != "" {
		expanded, expandErr := h.resolveNamesExpr(ctx, pkg.NamesExpr)
		if expandErr != nil {
			return nil, fmt.Errorf("failed to resolve package names expression %q: %w", pkg.NamesExpr, expandErr)
		}
		packages = append(packages, expanded...)
	}

	// Proposal-01 envelope: Operation + Target derived from the
	// requested state and the package list. Inner helpers may flip
	// Operation to OpNoop when nothing was installed/removed; the
	// wrapper below patches that after they return.
	envelopeOp := executor.OpCreate
	if state == stateLatest {
		envelopeOp = executor.OpUpdate
	} else if state == stateAbsent {
		envelopeOp = executor.OpDelete
	}
	target := strings.Join(packages, ",")

	postProcess := func(res actions.Result, err error) (actions.Result, error) {
		if r, ok := res.(*executor.Result); ok && r != nil {
			r.Operation = envelopeOp
			r.Target = target
			if !r.Changed && !r.WouldChange && err == nil && r.Operation != executor.OpNoop {
				r.Operation = executor.OpNoop
			}
		}
		return res, err
	}

	if ctx.Mode() == actions.ModePlan {
		return postProcess(h.runPlan(ec, manager, state, packages, pkg))
	}

	// ModeApply — preserve the legacy Execute behavior. Upgrade and
	// cache update both fall through to their existing helpers.
	result := executor.NewResult()
	result.SetChanged(false)

	if pkg.Upgrade {
		return postProcess(h.executeUpgrade(ec, manager, pkg, step.ShouldBecome()))
	}

	if pkg.UpdateCache {
		if err := h.updateCache(ec, manager, step.ShouldBecome()); err != nil {
			return nil, fmt.Errorf("failed to update package cache: %w", err)
		}
	}

	switch state {
	case statePresent, stateLatest:
		return postProcess(h.installPackages(ec, manager, packages, state == stateLatest, pkg.Cask, pkg.Extra, step.ShouldBecome()))
	case stateAbsent:
		return postProcess(h.removePackages(ec, manager, packages, pkg.Cask, pkg.Extra, step.ShouldBecome()))
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
	//
	// For brew, pre-fetch the full installed set to avoid one subprocess per package.
	var brewInstalled map[string]struct{}
	if manager == pmBrew {
		var err error
		brewInstalled, err = h.fetchBrewInstalledSet(ec, pkg.Cask)
		if err != nil {
			result.Reason = fmt.Sprintf("check error: %v", err)
			return result, nil
		}
	}

	for _, name := range packages {
		var installed bool
		if brewInstalled != nil {
			_, installed = brewInstalled[name]
		} else {
			var err error
			installed, err = h.isPackageInstalled(ec, manager, name, pkg.Cask)
			if err != nil {
				result.Reason = fmt.Sprintf("check error: %v", err)
				return result, nil
			}
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
