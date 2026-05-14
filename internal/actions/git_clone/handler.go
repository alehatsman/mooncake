// Package git_clone implements the git.clone action: idempotent
// clone-or-update of a remote git repository at a specified ref.
package git_clone

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Handler implements the git.clone action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "git.clone",
		Description:        "Idempotently clone or update a git repository at a specific ref",
		Category:           actions.CategoryNetwork,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		Version:            "1.0.0",
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	g := step.GitClone
	if g == nil {
		return fmt.Errorf("git.clone requires configuration")
	}
	if strings.TrimSpace(g.Repo) == "" {
		return fmt.Errorf("git.clone: repo is required")
	}
	if strings.TrimSpace(g.Dest) == "" {
		return fmt.Errorf("git.clone: dest is required")
	}
	if g.Depth < 0 {
		return fmt.Errorf("git.clone: depth must be >= 0, got %d", g.Depth)
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	g := step.GitClone

	repo, err := ctx.GetTemplate().Render(g.Repo, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("git.clone: render repo: %w", err)
	}
	ref, err := ctx.GetTemplate().Render(g.Ref, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("git.clone: render ref: %w", err)
	}

	dest, err := expandDest(ctx, g.Dest)
	if err != nil {
		return nil, fmt.Errorf("git.clone: expand dest: %w", err)
	}

	state, err := inspectDest(dest)
	if err != nil {
		return nil, err
	}

	if ctx.Mode() == actions.ModePlan {
		return planResult(state, repo, ref, g.Update), nil
	}

	env, cleanup, err := credentialEnv(ctx, g.Credentials)
	if err != nil {
		return nil, fmt.Errorf("git.clone: credentials: %w", err)
	}
	defer cleanup()

	return apply(ctx, g, repo, ref, dest, state, env)
}

// destState describes whether the dest path is missing, a non-git
// directory, or a populated git working tree.
type destState struct {
	exists   bool
	isGitDir bool
	headSHA  string // resolved HEAD sha, only set when isGitDir
}

func inspectDest(dest string) (destState, error) {
	info, err := os.Stat(dest)
	if errors.Is(err, os.ErrNotExist) {
		return destState{}, nil
	}
	if err != nil {
		return destState{}, fmt.Errorf("stat %s: %w", dest, err)
	}
	if !info.IsDir() {
		return destState{}, fmt.Errorf("dest %s exists and is not a directory", dest)
	}

	gitDir := filepath.Join(dest, ".git")
	if _, statErr := os.Stat(gitDir); statErr != nil {
		// Directory exists but isn't a git repo.
		return destState{exists: true, isGitDir: false}, nil
	}

	sha, err := captureGit(dest, nil, "rev-parse", "HEAD")
	if err != nil {
		return destState{exists: true, isGitDir: true}, nil
	}
	return destState{exists: true, isGitDir: true, headSHA: strings.TrimSpace(sha)}, nil
}

func planResult(state destState, repo, ref string, update bool) *executor.Result {
	r := executor.NewResult()
	r.Checkable = true

	switch {
	case !state.exists:
		r.WouldChange = true
		r.Reason = fmt.Sprintf("would clone %s -> dest (missing)", repo)
	case state.exists && !state.isGitDir:
		r.WouldChange = true
		r.Reason = "dest exists but is not a git repository"
	case !update:
		r.Reason = "dest is a git repo and update=false"
	case ref == "":
		r.WouldChange = true
		r.Reason = "would fetch and fast-forward (no ref pinned)"
	default:
		r.WouldChange = true
		r.Reason = fmt.Sprintf("would fetch and checkout %s (current HEAD %s)", ref, shortSHA(state.headSHA))
	}
	return r
}

func apply(ctx actions.Context, g *config.GitClone, repo, ref, dest string, state destState, env []string) (actions.Result, error) {
	result := executor.NewResult()
	result.Data = map[string]any{
		"repo": repo,
		"dest": dest,
	}

	if state.exists && !state.isGitDir {
		result.SetFailed(true)
		return result, fmt.Errorf("git.clone: dest %s exists but is not a git repository", dest)
	}

	switch {
	case !state.exists:
		if err := runClone(ctx, g, repo, ref, dest, env); err != nil {
			result.SetFailed(true)
			return result, err
		}
		result.SetChanged(true)
	case !g.Update:
		// dest is already a git repo, update disabled — no-op.
		result.SetChanged(false)
	default:
		changed, err := runUpdate(g, ref, dest, state.headSHA, env)
		if err != nil {
			result.SetFailed(true)
			return result, err
		}
		result.SetChanged(changed)
	}

	finalSHA, err := captureGit(dest, nil, "rev-parse", "HEAD")
	if err != nil {
		return result, fmt.Errorf("git.clone: read final sha: %w", err)
	}
	finalSHA = strings.TrimSpace(finalSHA)
	result.Data["sha"] = finalSHA
	result.Data["ref_resolved"] = ref
	result.Reason = fmt.Sprintf("HEAD at %s", shortSHA(finalSHA))
	return result, nil
}

func runClone(ctx actions.Context, g *config.GitClone, repo, ref, dest string, env []string) error {
	args := []string{"clone"}
	if g.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(g.Depth))
	}
	if g.RecurseSubmodules {
		args = append(args, "--recurse-submodules")
	}
	args = append(args, repo, dest)

	ctx.GetLogger().Debugf("git.clone: %s", strings.Join(args, " "))
	if err := runGit("", env, args...); err != nil {
		return fmt.Errorf("git.clone: %w", err)
	}

	if ref != "" {
		if err := checkout(dest, ref, g.Force, env); err != nil {
			return err
		}
	}
	return nil
}

func runUpdate(g *config.GitClone, ref, dest, beforeSHA string, env []string) (bool, error) {
	if !g.Force {
		dirty, err := isDirty(dest)
		if err != nil {
			return false, err
		}
		if dirty {
			return false, fmt.Errorf("git.clone: working tree has local changes (set force: true to discard)")
		}
	}

	fetchArgs := []string{"fetch", "--prune"}
	if g.Depth > 0 {
		fetchArgs = append(fetchArgs, "--depth", strconv.Itoa(g.Depth))
	}
	if err := runGit(dest, env, fetchArgs...); err != nil {
		return false, fmt.Errorf("git.clone: fetch: %w", err)
	}

	if ref != "" {
		if err := checkout(dest, ref, g.Force, env); err != nil {
			return false, err
		}
	} else {
		// No ref pinned — sync current branch to upstream. With force,
		// hard-reset so any local commits get discarded (matches the
		// "force: true to discard" promise made by the dirty-check above).
		// Without force, refuse explicitly when local has commits not on
		// origin — otherwise git merge --ff-only fails with an opaque
		// "refusing to merge unrelated histories" that's surprising to
		// anyone using mooncake on a host where they also hack on the repo.
		if g.Force {
			if err := runGit(dest, env, "reset", "--hard", "@{u}"); err != nil {
				return false, fmt.Errorf("git.clone: reset to upstream: %w", err)
			}
		} else {
			ahead, err := captureGit(dest, env, "rev-list", "--count", "@{u}..HEAD")
			if err != nil {
				return false, fmt.Errorf("git.clone: count local-only commits: %w", err)
			}
			if n := strings.TrimSpace(ahead); n != "" && n != "0" {
				return false, fmt.Errorf("git.clone: local has %s commit(s) not on origin (set force: true to discard)", n)
			}
			if err := runGit(dest, env, "merge", "--ff-only", "@{u}"); err != nil {
				return false, fmt.Errorf("git.clone: fast-forward: %w", err)
			}
		}
	}

	if g.RecurseSubmodules {
		if err := runGit(dest, env, "submodule", "update", "--init", "--recursive"); err != nil {
			return false, fmt.Errorf("git.clone: submodule update: %w", err)
		}
	}

	afterSHA, err := captureGit(dest, nil, "rev-parse", "HEAD")
	if err != nil {
		return false, fmt.Errorf("git.clone: read HEAD after update: %w", err)
	}
	return strings.TrimSpace(afterSHA) != beforeSHA, nil
}

func checkout(dest, ref string, force bool, env []string) error {
	args := []string{"checkout"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, ref)
	if err := runGit(dest, env, args...); err != nil {
		return fmt.Errorf("git.clone: checkout %s: %w", ref, err)
	}
	return nil
}

func isDirty(dest string) (bool, error) {
	out, err := captureGit(dest, nil, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git.clone: status: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

// gitRunner is overridable by tests so credential-env wiring can be
// inspected without requiring a real git invocation.
var gitRunner = realRunGit

func runGit(cwd string, env []string, args ...string) error {
	return gitRunner(cwd, env, args)
}

func realRunGit(cwd string, env []string, args []string) error {
	// #nosec G204 -- args are validated/static + user-supplied repo/ref; git is the binary we shell to.
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
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

func captureGit(cwd string, env []string, args ...string) (string, error) {
	// #nosec G204 -- internal git invocation; args are static commands.
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, msg)
	}
	return stdout.String(), nil
}

func expandDest(ctx actions.Context, dest string) (string, error) {
	if ec, ok := ctx.(*executor.ExecutionContext); ok {
		return ec.Svc.PathUtil.ExpandPath(dest, ec.CurrentDir, ctx.GetVariables())
	}
	// Fallback for tests with bare contexts: render only.
	return ctx.GetTemplate().Render(dest, ctx.GetVariables())
}

func shortSHA(sha string) string {
	if len(sha) >= 7 {
		return sha[:7]
	}
	return sha
}
