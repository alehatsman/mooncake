package package_handler

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for pkg (spec-22 phase 4d). The
// typed Before/After payload is actions.PackageDiff (spec-66 wave 2);
// see internal/actions/diff_payloads.go for the wire shape.
//
// Conservative semantics — Before is nil because we don't query the
// package manager from Diff. After.PkgSnapshot describes the user's
// intent, and Operation classifies that intent:
//
//	state=absent          → OpDelete
//	state=latest          → OpUpdate  (will install or upgrade)
//	state=present / ""    → OpCreate  (will install if missing, noop otherwise — runtime decides)
//	state=other (unknown) → OpUpdate  (conservative; the validate path
//	                                   will catch unknown states)
//
// The "OpCreate even when the package is already installed" framing
// matches how spec-22 §"Diff" describes intent: the structured Diff
// reports what the step would do, not what the system happens to be
// in. The runtime's idempotency check is what produces the actual
// changed=false on a converged system.
//
// For consumers that need accurate noop prediction (e.g. a UI that
// wants to fade out already-converged steps), the right path is to
// query `mooncake plan` which runs the manager's check at plan time.
// Diff is the cheap structural answer.
func (h *Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.Pkg == nil {
		return actions.Diff{}, errors.New("pkg Diff: step has no Pkg payload")
	}
	pkg := step.Pkg

	names := pkgNames(pkg)
	state := pkgState(pkg.State)
	op := pkgOperation(state)

	after := &actions.PackageDiff{
		Names:   names,
		State:   state,
		Manager: pkg.Manager,
	}

	identifier := "<unnamed>"
	if len(names) > 0 {
		identifier = names[0]
		if len(names) > 1 {
			// Make the multi-package case visible without flooding
			// the field. Consumers wanting the full list read
			// After.Names directly.
			identifier = fmt.Sprintf("%s (+%d more)", names[0], len(names)-1)
		}
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Identifier: identifier,
		},
		Operation: op,
		Before:    nil, // unknown current state — see file comment
		After:     after,
	}, nil
}

// pkgNames returns the package names this step targets, normalised
// to a single slice. Handles both `name:` (single) and `names:`
// (multi). NamesExpr (templated names) collapses to whatever Names
// already resolved to — we don't re-render at Diff time.
func pkgNames(pkg *config.Package) []string {
	if len(pkg.Names) > 0 {
		out := make([]string, len(pkg.Names))
		copy(out, pkg.Names)
		return out
	}
	if pkg.Name != "" {
		return []string{pkg.Name}
	}
	return nil
}

// pkgState normalises the empty-string default to "present" so
// downstream classification doesn't need a special case.
func pkgState(s string) string {
	if s == "" {
		return "present"
	}
	return s
}

func pkgOperation(state string) actions.Operation {
	switch state {
	case "absent":
		return actions.OpDelete
	case "latest":
		return actions.OpUpdate
	case "present":
		// Without querying the manager we can't tell create-vs-noop.
		// Conservative + honest: report Create (the typical case is
		// "the user wrote pkg: x because they want x installed"). The
		// idempotency check at runtime collapses no-op installs.
		return actions.OpCreate
	default:
		return actions.OpUpdate
	}
}

// Compile-time check that Handler satisfies Differ.
var _ actions.Differ = (*Handler)(nil)
