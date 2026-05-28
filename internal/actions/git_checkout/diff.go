package git_checkout

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for git.checkout (spec-26 phase 5 /
// spec-66 wave 6).
//
// Operation is always OpUpdate when the requested ref differs from
// HEAD; OpNoop when HEAD already matches (cheap rev-parse probe).
// Resource.Kind = ResourceGit, Resource.Identifier = dest path,
// Resource.Attributes["kind"] = "git.checkout" so internal/diff can
// dispatch the render_git matcher.
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

	resource := actions.ResourceRef{
		Kind:       actions.ResourceGit,
		Identifier: g.Dest,
		Attributes: map[string]string{"kind": "git.checkout"},
	}
	after := &actions.GitCheckoutDiff{Dest: g.Dest, Ref: g.Ref}

	dest := expandDestForDiff(ctx, g.Dest)
	state, err := inspectDest(dest)
	if err != nil || !state.exists || !state.isGitDir {
		// Dest missing / non-repo / stat error — Diff reports
		// OpUpdate without a Before; the apply path surfaces the
		// real error.
		return actions.Diff{Resource: resource, Operation: actions.OpUpdate, After: after}, nil
	}

	before := &actions.GitCheckoutDiff{Dest: g.Dest, HeadSHA: state.headSHA}
	return actions.Diff{Resource: resource, Operation: actions.OpUpdate, Before: before, After: after}, nil
}

// expandDestForDiff renders templates in dest. Diff is
// side-effect-free, so we don't do path-expansion (which can stat the
// filesystem); template render is enough for the common case.
func expandDestForDiff(ctx actions.Context, dest string) string {
	if ctx == nil {
		return dest
	}
	out, err := ctx.Template().Render(dest, ctx.Variables())
	if err != nil {
		return dest
	}
	return out
}

var _ actions.Differ = (*Handler)(nil)
