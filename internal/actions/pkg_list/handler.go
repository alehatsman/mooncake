// Package pkg_list implements the pkg.list action: a read-only query
// that returns the currently installed packages and their versions.
// No side effects in any mode; Changed is always false. Supports apt
// (via dpkg-query), dnf/yum (via rpm -qa), and pacman/yay/paru (via
// `pacman -Q`) on linux, and brew (via brew list --versions) on
// darwin. Auto-detects when manager is unset; explicit manager:
// overrides routing.
//
//nolint:revive // Package name matches action name convention (pkg_list)
package pkg_list

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
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName    = "pkg.list"
	managerApt    = "apt"
	managerDnf    = "dnf"
	managerPacman = "pacman"
	managerBrew   = "brew"
)

// Package-level hooks for the side-effectful binary call. Tests
// replace these to keep apply/plan hermetic.
var (
	dpkgQuery   = realDpkgQuery   // func() (string, error)
	rpmQuery    = realRpmQuery    // func() (string, error)
	pacmanQuery = realPacmanQuery // func() (string, error)
	brewList    = realBrewList    // func() (string, error)
	lookPath    = exec.LookPath   // override in tests for manager detection
)

// Handler implements pkg.list.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Return the installed packages and versions (read-only; apt + dnf + pacman on linux, brew on darwin)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux", "darwin"},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// pkg.list is a read-only query: no Sudo, no Network. The required
// binary depends on the (explicit-or-default) manager: dpkg-query for
// apt, rpm for dnf/yum, brew for brew. When manager: is unset we fall
// back to the platform default (dpkg-query on linux, brew on darwin)
// — a RHEL host without an explicit `manager: dnf` will see doctor
// flag dpkg-query as missing; the natural fix is to make the manager
// explicit. Spec-44 doctor reports the right binary for the routing.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	bin := "dpkg-query"
	if runtime.GOOS == "darwin" {
		bin = "brew"
	}
	if step != nil && step.PkgList != nil {
		switch strings.ToLower(strings.TrimSpace(step.PkgList.Manager)) {
		case managerApt:
			bin = "dpkg-query"
		case managerDnf, "yum":
			bin = "rpm"
		case managerPacman, "yay", "paru":
			bin = "pacman"
		case managerBrew:
			bin = "brew"
		}
	}
	return actions.PermissionSet{
		RequiredBinaries: []string{bin},
	}
}

func (h *Handler) Validate(step *config.Step) error {
	if step.PkgList == nil {
		return fmt.Errorf("pkg.list requires configuration")
	}
	return nil
}

// Run is identical in plan and apply mode: pkg.list never mutates.
// Changed is always false; the live package list is placed in
// result.Data["packages"]. Dispatch by manager (auto-detected when
// the operator left it blank).
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	p := step.PkgList
	result := executor.NewResult()
	result.Checkable = true
	result.Changed = false

	manager, err := resolveManager(p.Manager)
	if err != nil {
		return result, err
	}

	var pkgs []map[string]interface{}
	switch manager {
	case managerApt:
		out, err := dpkgQuery()
		if err != nil {
			return result, fmt.Errorf("pkg.list: dpkg-query: %w", err)
		}
		pkgs = parseTabSeparatedQuery(out, manager)
	case managerDnf, "yum":
		out, err := rpmQuery()
		if err != nil {
			return result, fmt.Errorf("pkg.list: rpm -qa: %w", err)
		}
		// Canonicalize to "dnf" in the result regardless of whether
		// the operator wrote `manager: yum` or `manager: dnf`; rpm
		// is the same database on both managers.
		pkgs = parseTabSeparatedQuery(out, managerDnf)
		manager = managerDnf
	case managerPacman, "yay", "paru":
		out, err := pacmanQuery()
		if err != nil {
			return result, fmt.Errorf("pkg.list: pacman -Q: %w", err)
		}
		// Canonicalize to "pacman" regardless of CLI alias: yay and
		// paru are pacman-compatible AUR wrappers that read the same
		// /var/lib/pacman db. Same shape as the yum→dnf canonical fold.
		pkgs = parsePacmanQuery(out, managerPacman)
		manager = managerPacman
	case managerBrew:
		out, err := brewList()
		if err != nil {
			return result, fmt.Errorf("pkg.list: brew list: %w", err)
		}
		pkgs = parseBrewList(out, manager)
	default:
		return result, fmt.Errorf("pkg.list: unsupported manager %q (supported: apt, dnf, pacman, brew)", manager)
	}

	sort.Slice(pkgs, func(i, j int) bool {
		ni, _ := pkgs[i]["name"].(string)
		nj, _ := pkgs[j]["name"].(string)
		return ni < nj
	})

	result.Operation = executor.OpQuery
	result.Target = manager
	result.Data = map[string]interface{}{
		"manager":  manager,
		"packages": pkgs,
		"count":    len(pkgs),
	}
	result.Reason = fmt.Sprintf("listed %d packages (%s)", len(pkgs), manager)
	return result, nil
}

// parseTabSeparatedQuery parses tab-separated `<name>\t<version>` output.
// Shared by the apt (dpkg-query -W -f='${Package}\t${Version}\n') and
// dnf/yum (rpm -qa --qf '%{NAME}\t%{VERSION}-%{RELEASE}\n') paths —
// both binaries emit the same shape on purpose so the result entries
// carry their distinguishing manager via the caller-supplied
// `manager` argument. Lines without a tab or with an empty name are
// skipped; an empty trailing newline is tolerated.
func parseTabSeparatedQuery(stdout, manager string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, 256)
	sc := bufio.NewScanner(strings.NewReader(stdout))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		version := strings.TrimSpace(parts[1])
		if name == "" {
			continue
		}
		out = append(out, map[string]interface{}{
			"name":    name,
			"version": version,
			"manager": manager,
		})
	}
	return out
}

// resolveManager picks the package manager to query. Explicit
// manager: in the YAML wins over auto-detection. Auto-detection
// prefers apt (dpkg-query), then dnf (rpm), then pacman, then brew.
// The order matters on multi-manager hosts (Linuxbrew installed
// alongside a system dpkg-query, EPEL hosts with apt-the-rpm, etc.) —
// the native system manager is always the more authoritative list.
// pacman wins over per-user brew on every Arch host; rpm wins on
// every RHEL host.
func resolveManager(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		return requested, nil
	}
	if _, err := lookPath("dpkg-query"); err == nil {
		return managerApt, nil
	}
	if _, err := lookPath("rpm"); err == nil {
		return managerDnf, nil
	}
	if _, err := lookPath("pacman"); err == nil {
		return managerPacman, nil
	}
	if _, err := lookPath("brew"); err == nil {
		return managerBrew, nil
	}
	return "", fmt.Errorf("pkg.list: cannot auto-detect package manager (no dpkg-query, rpm, pacman, or brew on PATH); set manager explicitly")
}

func realDpkgQuery() (string, error) {
	// #nosec G204 -- fixed dpkg-query binary.
	cmd := exec.Command("dpkg-query", "-W", "-f=${Package}\t${Version}\n")
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
	return stdout.String(), nil
}

// realRpmQuery shells out to `rpm -qa --qf '%{NAME}\t%{VERSION}-%{RELEASE}\n'`.
// Wired into the rpmQuery package var by default; tests substitute
// their own. Tab-separated to match the dpkg-query shape so the same
// parseTabSeparatedQuery helper handles both paths.
//
// Works on both dnf (RHEL 8+/Fedora) and yum (RHEL 7) hosts: the rpm
// binary is the same database on both, and the format string is
// portable across rpm versions back to RHEL 6 — no version probing
// needed.
func realRpmQuery() (string, error) {
	// #nosec G204 -- fixed rpm binary.
	cmd := exec.Command("rpm", "-qa", "--qf", "%{NAME}\t%{VERSION}-%{RELEASE}\n")
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
	return stdout.String(), nil
}

// realPacmanQuery shells out to `pacman -Q`, which prints `<name>
// <version>` per line (space-separated). Wired into the pacmanQuery
// package var by default; tests substitute their own.
//
// We use `pacman -Q` rather than `-Qe` (explicitly-installed only)
// because the inspector's contract is "what's installed on this
// host" — the same denominator dpkg-query and rpm -qa report.
func realPacmanQuery() (string, error) {
	// #nosec G204 -- fixed pacman binary.
	cmd := exec.Command("pacman", "-Q")
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
	return stdout.String(), nil
}

// parsePacmanQuery parses `pacman -Q` output. Each line is `<name>
// <version>`, single space separator. Pacman names cannot contain
// spaces, so SplitN on space is safe — version strings can contain
// hyphens, colons (epoch), dots, etc., but no space.
//
// Lines without a version are skipped (matches the parseBrewList
// "orphan-no-version" convention; pacman shouldn't emit them but
// defending against malformed output is cheap).
func parsePacmanQuery(stdout, manager string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, 256)
	sc := bufio.NewScanner(strings.NewReader(stdout))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		version := strings.TrimSpace(parts[1])
		if name == "" || version == "" {
			continue
		}
		out = append(out, map[string]interface{}{
			"name":    name,
			"version": version,
			"manager": manager,
		})
	}
	return out
}

func realBrewList() (string, error) {
	// #nosec G204 -- fixed brew binary.
	cmd := exec.Command("brew", "list", "--versions")
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
	return stdout.String(), nil
}

// parseBrewList parses `brew list --versions` stdout. Output shape
// is space-separated per line: "<name> <version> [<version> ...]".
// Multiple versions can be installed simultaneously (`brew install
// node@18 node@20` keeps both); we report the last version on the
// line — by convention that's the most recently installed slot, the
// one `brew --prefix <name>` would resolve to. Lines without a
// version are skipped.
func parseBrewList(stdout, manager string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, 256)
	sc := bufio.NewScanner(strings.NewReader(stdout))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			// Name without a version means brew failed to read the
			// installed receipt — skip rather than emit an empty
			// version (downstream consumers expect a non-empty
			// version string per dpkg-query parity).
			continue
		}
		name := fields[0]
		version := fields[len(fields)-1]
		out = append(out, map[string]interface{}{
			"name":    name,
			"version": version,
			"manager": manager,
		})
	}
	return out
}
