package git_clone

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for git.clone (spec-26 phase 5 /
// spec-66 wave 8).
//
// Operation classification:
//
//	dest missing                                 → OpCreate
//	dest is git repo, update=false               → OpNoop
//	dest is git repo, update=true                → OpUpdate
//	dest exists but is NOT a git repo            → OpUpdate (conservative;
//	                                                Run() errors at apply)
//
// Conservatively never fetches from the network at plan time —
// Operation reflects the dest state, not whether the remote ref
// resolves. The runtime path produces the actual changed=false on
// already-converged systems.
//
// Resource.Kind = ResourceGit, Resource.Identifier = dest path,
// Resource.Attributes["kind"] = "git.clone" so internal/diff can
// dispatch the render_git matcher.
func (Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.GitClone == nil {
		return actions.Diff{}, errors.New("git.clone Diff: step has no GitClone payload")
	}
	g := step.GitClone

	resource := actions.ResourceRef{
		Kind:       actions.ResourceGit,
		Identifier: g.Dest,
		Attributes: map[string]string{"kind": "git.clone"},
	}

	after := &actions.GitCloneDiff{Dest: g.Dest, Repo: g.Repo, Ref: g.Ref}

	// Best-effort cheap probe of dest state — no network, no
	// subprocess if dest doesn't exist. We can't expand ~/foo here
	// without a real context (templated paths), so use Dest as-is;
	// if it doesn't stat we treat that as OpCreate.
	dest := expandDestForDiff(ctx, g.Dest)
	state, err := inspectDest(dest)
	if err != nil {
		// inspectDest errors only on non-OK stat failures (e.g.
		// permission denied). Conservatively report OpUpdate
		// without a Before — the runtime will surface the real
		// error.
		return actions.Diff{Resource: resource, Operation: actions.OpUpdate, After: after}, nil
	}

	switch {
	case !state.exists:
		return actions.Diff{Resource: resource, Operation: actions.OpCreate, After: after}, nil
	case !state.isGitDir:
		// Dest exists but isn't a git repo — apply will error.
		// Report OpUpdate (the closest classifier) so plan output
		// surfaces "would change" rather than silently OpNoop.
		return actions.Diff{Resource: resource, Operation: actions.OpUpdate, After: after}, nil
	case !g.Update:
		// Existing repo, update disabled — converged noop.
		before := &actions.GitCloneDiff{Dest: g.Dest, HeadSHA: state.headSHA}
		return actions.Diff{Resource: resource, Operation: actions.OpNoop, Before: before, After: after}, nil
	default:
		before := &actions.GitCloneDiff{Dest: g.Dest, HeadSHA: state.headSHA}
		return actions.Diff{Resource: resource, Operation: actions.OpUpdate, Before: before, After: after}, nil
	}
}

// expandDestForDiff renders templates in dest but does NOT do
// path-expansion. The same convention used by Diff() across the
// action surface — keep Diff cheap and side-effect-free.
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
