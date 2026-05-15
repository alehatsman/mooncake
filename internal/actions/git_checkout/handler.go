// Package git_checkout implements the git.checkout action: switch an
// existing local git repository to a specified ref.
package git_checkout //nolint:revive // package name follows action convention

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Handler implements the git.checkout action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "git.checkout",
		Description:        "Switch an existing git working tree to a specified ref",
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
// git.checkout is a local operation — no Network. Requires the
// `git` binary on PATH. Sudo is derived from dest: switching refs
// inside /etc, /opt, etc. needs elevation (it writes index +
// working-tree files). FilesystemWrite is the dest path.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	ps := actions.PermissionSet{
		RequiredBinaries: []string{"git"},
	}
	if step == nil || step.GitCheckout == nil {
		return ps
	}
	if actions.PathNeedsSudo(step.GitCheckout.Dest) {
		ps.Sudo = true
	}
	if step.GitCheckout.Dest != "" {
		ps.FilesystemWrite = []string{step.GitCheckout.Dest}
	}
	return ps
}

// Validate checks required fields on the step.
func (h *Handler) Validate(step *config.Step) error {
	g := step.GitCheckout
	if g == nil {
		return fmt.Errorf("git.checkout requires configuration")
	}
	if strings.TrimSpace(g.Dest) == "" {
		return fmt.Errorf("git.checkout: dest is required")
	}
	if strings.TrimSpace(g.Ref) == "" {
		return fmt.Errorf("git.checkout: ref is required")
	}
	return nil
}

// Run inspects the working tree, resolves the requested ref, and either
// plans the checkout (ModePlan) or applies it (ModeApply).
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	g := step.GitCheckout

	ref, err := ctx.GetTemplate().Render(g.Ref, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("git.checkout: render ref: %w", err)
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("git.checkout: ref is required")
	}

	dest, err := expandDest(ctx, g.Dest)
	if err != nil {
		return nil, fmt.Errorf("git.checkout: expand dest: %w", err)
	}

	state, err := inspectDest(dest)
	if err != nil {
		return nil, err
	}
	if !state.exists {
		return nil, fmt.Errorf("git.checkout: dest %s does not exist", dest)
	}
	if !state.isGitDir {
		return nil, fmt.Errorf("git.checkout: dest %s is not a git repository", dest)
	}

	targetSHA, resolveErr := captureGit(dest, "rev-parse", "--verify", ref+"^{commit}")
	if resolveErr != nil {
		return nil, fmt.Errorf("git.checkout: cannot resolve ref %q in %s: %w", ref, dest, resolveErr)
	}
	target := strings.TrimSpace(targetSHA)

	if ctx.Mode() == actions.ModePlan {
		return planResult(state.headSHA, target, ref), nil
	}

	return apply(ctx, g, ref, target, dest, state.headSHA)
}

type destState struct {
	exists   bool
	isGitDir bool
	headSHA  string
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
		return destState{exists: true, isGitDir: false}, nil
	}

	sha, err := captureGit(dest, "rev-parse", "HEAD")
	if err != nil {
		return destState{exists: true, isGitDir: true}, nil
	}
	return destState{exists: true, isGitDir: true, headSHA: strings.TrimSpace(sha)}, nil
}

func planResult(currentSHA, targetSHA, ref string) *executor.Result {
	r := executor.NewResult()
	r.Checkable = true
	if currentSHA == targetSHA {
		r.Reason = fmt.Sprintf("already at %s (%s)", shortSHA(currentSHA), ref)
		return r
	}
	r.WouldChange = true
	r.Reason = fmt.Sprintf("would checkout %s: %s -> %s", ref, shortSHA(currentSHA), shortSHA(targetSHA))
	return r
}

func apply(ctx actions.Context, g *config.GitCheckout, ref, targetSHA, dest, beforeSHA string) (actions.Result, error) {
	result := executor.NewResult()
	result.Checkable = true
	result.Data = map[string]any{
		"dest":         dest,
		"ref_resolved": ref,
	}

	if beforeSHA == targetSHA {
		result.SetChanged(false)
		result.Data["sha"] = targetSHA
		result.Reason = fmt.Sprintf("already at %s (%s)", shortSHA(targetSHA), ref)
		// Apply is a noop — leave ReverseData nil so Reverse
		// returns (nil, nil) per the Reverser contract.
		return result, nil
	}

	if !g.Force {
		dirty, err := isDirty(dest)
		if err != nil {
			result.SetFailed(true)
			return result, err
		}
		if dirty {
			result.SetFailed(true)
			return result, fmt.Errorf("git.checkout: working tree has local changes (set force: true to discard)")
		}
	}

	// Capture pre-checkout HEAD for Reverse() BEFORE shelling out.
	// Once `git checkout` runs the prior sha is no longer reachable
	// via `git rev-parse HEAD`; transaction rollback would have no
	// way to find it. beforeSHA was resolved at the top of Run and
	// is the exact value we want to roll back to.
	result.ReverseData = &GitCheckoutReverseInfo{
		Dest:     dest,
		PriorSHA: beforeSHA,
	}

	args := []string{"checkout"}
	if g.Force {
		args = append(args, "--force")
	}
	args = append(args, ref)
	ctx.GetLogger().Debugf("git.checkout: %s", strings.Join(args, " "))
	if err := runGit(dest, args...); err != nil {
		result.SetFailed(true)
		return result, fmt.Errorf("git.checkout: %w", err)
	}

	afterSHA, err := captureGit(dest, "rev-parse", "HEAD")
	if err != nil {
		result.SetFailed(true)
		return result, fmt.Errorf("git.checkout: read HEAD after checkout: %w", err)
	}
	after := strings.TrimSpace(afterSHA)
	result.SetChanged(true)
	result.Data["sha"] = after
	result.Reason = fmt.Sprintf("checked out %s (%s -> %s)", ref, shortSHA(beforeSHA), shortSHA(after))
	return result, nil
}

func isDirty(dest string) (bool, error) {
	out, err := captureGit(dest, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git.checkout: status: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

func runGit(cwd string, args ...string) error {
	// #nosec G204 -- args are validated/static; user-supplied ref is passed to git which validates.
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
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

func captureGit(cwd string, args ...string) (string, error) {
	// #nosec G204 -- internal git invocation; args are static commands plus user-supplied ref.
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
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
	return ctx.GetTemplate().Render(dest, ctx.GetVariables())
}

func shortSHA(sha string) string {
	if len(sha) >= 7 {
		return sha[:7]
	}
	return sha
}
