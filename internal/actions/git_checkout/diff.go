package git_checkout

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// GitCheckoutSnapshot is the typed Before/After payload for
// actions.Diff when the resource kind is ResourceGit. Mirrors
// GitCloneSnapshot's shape so consumers can dispatch on Resource.Kind
// = ResourceGit and treat the two interchangeably.
//
// Before is nil when dest is missing or not a git repo (the Run path
// errors in that case; Diff doesn't pre-check). When the repo exists,
// Before.HeadSHA is the observed HEAD. After.Ref is the requested
// ref; After.HeadSHA is empty at plan time (resolution happens at
// apply).
type GitCheckoutSnapshot struct {
	// Ref mirrors step.GitCheckout.Ref — the requested ref.
	Ref string `json:"ref,omitempty"`

	// HeadSHA is the observed HEAD sha. Populated in Before when
	// dest is a git repo at plan time.
	HeadSHA string `json:"head_sha,omitempty"`
}

// Diff implements actions.Differ for git.checkout (spec-26 phase 5).
//
// Operation is always OpUpdate when the requested ref differs from
// HEAD; OpNoop when HEAD already matches (cheap rev-parse probe).
// Resource.Kind = ResourceGit, Resource.Identifier = dest path.
//
// Conservative semantics — does not resolve the requested ref via
// rev-parse (the Run path does that with full error reporting). Diff
// reports OpUpdate whenever dest exists as a git repo and Ref is
// non-empty; the apply path may downgrade to OpNoop after ref
// resolution. This keeps Diff side-effect-free and predictable
// without coupling it to git-resolve semantics.
func (Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.GitCheckout == nil {
		return actions.Diff{}, errors.New("git.checkout Diff: step has no GitCheckout payload")
	}
	g := step.GitCheckout

	resource := actions.ResourceRef{Kind: actions.ResourceGit, Identifier: g.Dest}
	after := &GitCheckoutSnapshot{Ref: g.Ref}

	dest := expandDestForDiff(ctx, g.Dest)
	state, err := inspectDest(dest)
	if err != nil || !state.exists || !state.isGitDir {
		// Dest missing / non-repo / stat error — Diff reports
		// OpUpdate without a Before; the apply path surfaces the
		// real error.
		return actions.Diff{Resource: resource, Operation: actions.OpUpdate, After: after}, nil
	}

	before := &GitCheckoutSnapshot{HeadSHA: state.headSHA}
	return actions.Diff{Resource: resource, Operation: actions.OpUpdate, Before: before, After: after}, nil
}

// expandDestForDiff renders templates in dest. Diff is
// side-effect-free, so we don't do path-expansion (which can stat the
// filesystem); template render is enough for the common case.
func expandDestForDiff(ctx actions.Context, dest string) string {
	if ctx == nil {
		return dest
	}
	out, err := ctx.GetTemplate().Render(dest, ctx.GetVariables())
	if err != nil {
		return dest
	}
	return out
}

var _ actions.Differ = (*Handler)(nil)
