// Package pkg_list implements the pkg.list action: a read-only query
// that returns the currently installed packages and their versions.
// No side effects in any mode; Changed is always false. Supports apt
// (via dpkg-query) on linux and brew (via brew list --versions) on
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
	actionName  = "pkg.list"
	managerApt  = "apt"
	managerBrew = "brew"
)

// Package-level hooks for the side-effectful binary call. Tests
// replace these to keep apply/plan hermetic.
var (
	dpkgQuery = realDpkgQuery // func() (string, error)
	brewList  = realBrewList  // func() (string, error)
	lookPath  = exec.LookPath // override in tests for manager detection
)

// Handler implements pkg.list.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Return the installed packages and versions (read-only; apt on linux, brew on darwin)",
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
// binary differs per host: dpkg-query on linux (apt path), brew on
// darwin. Spec-44 doctor reports the right one for the local box.
func (Handler) Permissions(_ *config.Step) actions.PermissionSet {
	bin := "dpkg-query"
	if runtime.GOOS == "darwin" {
		bin = "brew"
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
		pkgs = parseDpkgQuery(out, manager)
	case managerBrew:
		out, err := brewList()
		if err != nil {
			return result, fmt.Errorf("pkg.list: brew list: %w", err)
		}
		pkgs = parseBrewList(out, manager)
	default:
		return result, fmt.Errorf("pkg.list: unsupported manager %q (supported: apt, brew)", manager)
	}

	sort.Slice(pkgs, func(i, j int) bool {
		ni, _ := pkgs[i]["name"].(string)
		nj, _ := pkgs[j]["name"].(string)
		return ni < nj
	})

	result.Data = map[string]interface{}{
		"manager":  manager,
		"packages": pkgs,
		"count":    len(pkgs),
	}
	result.Reason = fmt.Sprintf("listed %d packages (%s)", len(pkgs), manager)
	return result, nil
}

// parseDpkgQuery parses the tab-separated output of
// `dpkg-query -W -f='${Package}\t${Version}\n'`. Lines missing a
// version field or a name are skipped.
func parseDpkgQuery(stdout, manager string) []map[string]interface{} {
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
// prefers apt (dpkg-query) when available, then brew. The order
// matters on multi-manager hosts (e.g. macOS with Linuxbrew under
// /home/linuxbrew/.linuxbrew/bin/brew installed alongside a system
// dpkg-query) — apt-the-installed is the more authoritative system
// list when both exist, since brew under /home/linuxbrew is per-user.
func resolveManager(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		return requested, nil
	}
	if _, err := lookPath("dpkg-query"); err == nil {
		return managerApt, nil
	}
	if _, err := lookPath("brew"); err == nil {
		return managerBrew, nil
	}
	return "", fmt.Errorf("pkg.list: cannot auto-detect package manager (no dpkg-query or brew on PATH); set manager explicitly")
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
