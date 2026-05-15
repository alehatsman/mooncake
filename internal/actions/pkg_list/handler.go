// Package pkg_list implements the pkg.list action: a read-only query
// that returns the currently installed packages and their versions.
// No side effects in any mode; Changed is always false. v1 implements
// apt only via dpkg-query; other managers raise a clear "only apt is
// supported in v1" error.
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
	actionName = "pkg.list"
	managerApt = "apt"
)

// Package-level hooks for the side-effectful binary call. Tests
// replace these to keep apply/plan hermetic.
var (
	dpkgQuery = realDpkgQuery   // func() (string, error)
	lookPath  = exec.LookPath   // override in tests for manager detection
)

// Handler implements pkg.list.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Return the installed packages and versions (read-only; apt only in v1)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux"},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// pkg.list is a read-only query: no Sudo, no Network. It shells out
// to dpkg-query (v1 supports apt only) which any unprivileged user
// can run.
func (Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		RequiredBinaries: []string{"dpkg-query"},
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
// result.Data["packages"].
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	p := step.PkgList
	result := executor.NewResult()
	result.Checkable = true
	result.Changed = false

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("pkg.list: only Linux is supported; got %s", runtime.GOOS)
	}

	manager, err := resolveManager(p.Manager)
	if err != nil {
		return result, err
	}
	if manager != managerApt {
		return result, fmt.Errorf("pkg.list: only apt is supported in v1 (got %q)", manager)
	}

	out, err := dpkgQuery()
	if err != nil {
		return result, fmt.Errorf("pkg.list: dpkg-query: %w", err)
	}
	pkgs := parseDpkgQuery(out, manager)
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

func resolveManager(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		return requested, nil
	}
	if _, err := lookPath("dpkg-query"); err == nil {
		return managerApt, nil
	}
	return "", fmt.Errorf("pkg.list: cannot auto-detect package manager (dpkg-query not on PATH); set manager explicitly")
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
