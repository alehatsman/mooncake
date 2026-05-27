// Package git_config implements the git.config action: idempotent
// management of git config keys at local/global/system scope.
package git_config //nolint:revive // package name follows action convention

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	scopeLocal  = "local"
	scopeGlobal = "global"
	scopeSystem = "system"
)

// Handler implements the git.config action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("GitConfigReverseInfo", func() any { return &GitConfigReverseInfo{} })
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "git.config",
		Description:        "Idempotently manage git config keys at local, global, or system scope",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		Version:            "1.0.0",
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// git.config writes config keys at one of three scopes:
//   - system → /etc/gitconfig — always needs sudo.
//   - global → ~/.gitconfig — never needs sudo (user-owned).
//   - local  → <repo>/.git/config — sudo iff repo lives under a
//     system path (PathNeedsSudo).
//
// No Network. Requires the `git` binary on PATH.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	ps := actions.PermissionSet{
		RequiredBinaries: []string{"git"},
	}
	if step == nil || step.GitConfig == nil {
		return ps
	}
	scope := strings.TrimSpace(step.GitConfig.Scope)
	switch scope {
	case scopeSystem:
		ps.Sudo = true
		ps.FilesystemWrite = []string{"/etc/gitconfig"}
	case scopeLocal:
		if actions.PathNeedsSudo(step.GitConfig.Repo) {
			ps.Sudo = true
		}
		if step.GitConfig.Repo != "" {
			ps.FilesystemWrite = []string{step.GitConfig.Repo + "/.git/config"}
		}
	}
	return ps
}

// Validate checks required fields on the step.
func (h *Handler) Validate(step *config.Step) error {
	g := step.GitConfig
	if g == nil {
		return fmt.Errorf("git.config requires configuration")
	}
	scope := strings.TrimSpace(g.Scope)
	switch scope {
	case scopeLocal, scopeGlobal, scopeSystem:
	case "":
		return fmt.Errorf("git.config: scope is required (local|global|system)")
	default:
		return fmt.Errorf("git.config: invalid scope %q (must be local|global|system)", scope)
	}
	if scope == scopeLocal {
		repo, err := g.WorkingTree()
		if err != nil {
			return err
		}
		if strings.TrimSpace(repo) == "" {
			return fmt.Errorf("git.config: repo (or dest:) is required when scope=local")
		}
	} else if _, err := g.WorkingTree(); err != nil {
		return err
	}
	if len(g.Set) == 0 && len(g.Unset) == 0 {
		return fmt.Errorf("git.config: at least one of set or unset must be provided")
	}
	for key := range g.Set {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("git.config: set keys must not be empty")
		}
	}
	for _, key := range g.Unset {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("git.config: unset keys must not be empty")
		}
	}
	return nil
}

// Run plans or applies drift between desired and observed git config state.
// RunRaw signals spec-69 RawRunner participation so user-declared
// `retry:` actually retries this idempotent action via the
// centralized executor loop instead of being silently no-op'd.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	g := step.GitConfig
	scope := strings.TrimSpace(g.Scope)

	repo := ""
	if scope == scopeLocal {
		repoSrc, err := g.WorkingTree()
		if err != nil {
			return nil, err
		}
		expanded, err := expandRepo(ctx, repoSrc)
		if err != nil {
			return nil, fmt.Errorf("git.config: expand repo: %w", err)
		}
		repo = expanded
		if err := ensureGitRepo(repo); err != nil {
			return nil, err
		}
	}

	renderedSet, err := renderSet(ctx, g.Set)
	if err != nil {
		return nil, err
	}
	renderedUnset, err := renderUnset(ctx, g.Unset)
	if err != nil {
		return nil, err
	}

	drift, err := computeDrift(scope, repo, renderedSet, renderedUnset)
	if err != nil {
		return nil, err
	}

	if ctx.Mode() == actions.ModePlan {
		return planResult(drift), nil
	}

	return apply(ctx, scope, repo, drift)
}

// driftEntry describes a single key whose desired and observed states differ.
type driftEntry struct {
	key      string
	op       string // "set" or "unset"
	current  string // observed value (empty if absent)
	desired  string // desired value (empty for unset)
	hadValue bool   // true if a value was observed before the change
}

func computeDrift(scope, repo string, set map[string]string, unset []string) ([]driftEntry, error) {
	var drift []driftEntry

	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		desired := set[key]
		current, had, err := readKey(scope, repo, key)
		if err != nil {
			return nil, err
		}
		if had && current == desired {
			continue
		}
		drift = append(drift, driftEntry{key: key, op: "set", current: current, desired: desired, hadValue: had})
	}

	for _, key := range unset {
		current, had, err := readKey(scope, repo, key)
		if err != nil {
			return nil, err
		}
		if !had {
			continue
		}
		drift = append(drift, driftEntry{key: key, op: "unset", current: current, hadValue: true})
	}

	return drift, nil
}

func planResult(drift []driftEntry) *executor.Result {
	r := executor.NewResult()
	r.Checkable = true
	if len(drift) == 0 {
		r.Reason = "all keys already at desired state"
		return r
	}
	r.WouldChange = true
	r.Reason = fmt.Sprintf("%d key(s) drift: %s", len(drift), summarizeDrift(drift))
	return r
}

func apply(_ actions.Context, scope, repo string, drift []driftEntry) (actions.Result, error) {
	result := executor.NewResult()
	result.Checkable = true
	result.Operation = executor.OpUpdate
	if repo != "" {
		result.Target = repo + ":" + scope
	} else {
		result.Target = scope
	}
	result.Data = map[string]any{
		"scope":   scope,
		"changes": len(drift),
	}

	if len(drift) == 0 {
		result.SetChanged(false)
		result.Operation = executor.OpNoop
		result.Reason = "all keys already at desired state"
		// Apply is a noop — leave ReverseData nil so Reverse
		// returns (nil, nil) per the Reverser contract.
		return result, nil
	}

	// Capture pre-mutation state for Reverse() BEFORE shelling out.
	// driftEntry already carries `current` (observed prior value)
	// and `hadValue` (whether the key was set before) — exactly the
	// shape Reverse needs to build the inverse step.
	result.ReverseData = buildReverseInfo(scope, repo, drift)

	for _, d := range drift {
		switch d.op {
		case "set":
			if err := runGit(scopeArgs(scope, repo, d.key, []string{"--replace-all", d.key, d.desired})...); err != nil {
				result.SetFailed(true)
				return result, fmt.Errorf("git.config: set %s: %w", d.key, err)
			}
		case "unset":
			if err := runGit(scopeArgs(scope, repo, d.key, []string{"--unset-all", d.key})...); err != nil {
				result.SetFailed(true)
				return result, fmt.Errorf("git.config: unset %s: %w", d.key, err)
			}
		}
	}

	result.SetChanged(true)
	result.Reason = fmt.Sprintf("%d key(s) changed: %s", len(drift), summarizeDrift(drift))
	return result, nil
}

// scopeArgs builds the argv for `git config` calls.
// When scope=local we invoke git inside the repo working tree (via -C).
// For global/system we pass the scope flag and rely on git's defaults.
func scopeArgs(scope, repo, _ string, tail []string) []string {
	var args []string
	if scope == scopeLocal {
		args = append(args, "-C", repo, "config", "--local")
	} else {
		args = append(args, "config", "--"+scope)
	}
	return append(args, tail...)
}

func readKey(scope, repo, key string) (string, bool, error) {
	var args []string
	if scope == scopeLocal {
		args = []string{"-C", repo, "config", "--local", "--get", key}
	} else {
		args = []string{"config", "--" + scope, "--get", key}
	}
	out, code, stderr, err := captureGitWithCode(args...)
	switch {
	case err != nil:
		// Genuine exec failure (binary missing, etc).
		return "", false, fmt.Errorf("git.config: read %s: %w", key, err)
	case code == 0:
		return strings.TrimRight(out, "\n"), true, nil
	case code == 1 && strings.TrimSpace(stderr) == "":
		// Key not set — git's documented "not found" signal.
		return "", false, nil
	default:
		msg := strings.TrimSpace(stderr)
		return "", false, fmt.Errorf("git.config: read %s: git exit %d: %s", key, code, msg)
	}
}

func renderSet(ctx actions.Context, set map[string]string) (map[string]string, error) {
	if len(set) == 0 {
		return nil, nil
	}
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()
	out := make(map[string]string, len(set))
	for k, v := range set {
		renderedKey, err := tmpl.Render(k, vars)
		if err != nil {
			return nil, fmt.Errorf("git.config: render key %q: %w", k, err)
		}
		renderedVal, err := tmpl.Render(v, vars)
		if err != nil {
			return nil, fmt.Errorf("git.config: render value for %q: %w", k, err)
		}
		out[renderedKey] = renderedVal
	}
	return out, nil
}

func renderUnset(ctx actions.Context, unset []string) ([]string, error) {
	if len(unset) == 0 {
		return nil, nil
	}
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()
	out := make([]string, 0, len(unset))
	for _, k := range unset {
		rendered, err := tmpl.Render(k, vars)
		if err != nil {
			return nil, fmt.Errorf("git.config: render unset key %q: %w", k, err)
		}
		out = append(out, rendered)
	}
	return out, nil
}

func summarizeDrift(drift []driftEntry) string {
	parts := make([]string, 0, len(drift))
	for _, d := range drift {
		switch d.op {
		case "set":
			parts = append(parts, fmt.Sprintf("%s=%s", d.key, d.desired))
		case "unset":
			parts = append(parts, "-"+d.key)
		}
	}
	return strings.Join(parts, ", ")
}

func ensureGitRepo(repo string) error {
	info, err := os.Stat(repo)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("git.config: repo %s does not exist", repo)
	}
	if err != nil {
		return fmt.Errorf("git.config: stat %s: %w", repo, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("git.config: repo %s is not a directory", repo)
	}
	gitDir := filepath.Join(repo, ".git")
	if _, statErr := os.Stat(gitDir); statErr != nil {
		return fmt.Errorf("git.config: repo %s is not a git repository", repo)
	}
	return nil
}

func expandRepo(ctx actions.Context, repo string) (string, error) {
	if ec, ok := ctx.(*executor.ExecutionContext); ok {
		return ec.Svc.PathUtil.ExpandPath(repo, ec.CurrentDir, ctx.GetVariables())
	}
	return ctx.GetTemplate().Render(repo, ctx.GetVariables())
}

func runGit(args ...string) error {
	// #nosec G204 -- args are internally constructed; user-supplied keys/values are passed via argv (not shell).
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}

func captureGitWithCode(args ...string) (string, int, string, error) {
	// #nosec G204 -- internal git invocation; args constructed from scope + user-supplied key.
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), 0, stderr.String(), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout.String(), exitErr.ExitCode(), stderr.String(), nil
	}
	return stdout.String(), -1, stderr.String(), err
}
