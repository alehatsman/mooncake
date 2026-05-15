//nolint:revive // Package name matches action name convention (pkg_hold)
package pkg_hold

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for pkg.hold (spec-22 phase 5 /
// spec-24 P6).
//
// The inverse is trivial in shape — toggle held↔unheld on the same
// names — but a safe Reverse still wants apply-time capture of which
// packages were actually flipped (a state=held step that finds N of
// M already-held only flips M-N; reverse should unhold exactly those
// M-N, not all M). That capture requires Run() to thread a typed
// Result the way the legacy package handler's PkgReverseInfo does.
// Tracked as a spec-24 P6 follow-up. Until then the refusal keeps
// the convention consistent with pkg.repo / git.config.
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.PkgHold == nil {
		return nil, errors.New("pkg.hold Reverse: step has no PkgHold payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"pkg.hold Reverse: not yet implemented. Apply-time capture of " +
			"actually-flipped package names is needed to avoid over- or " +
			"under-toggling on rollback. Tracked in spec-24 P6 follow-up.")
}

var _ actions.Reverser = (*Handler)(nil)
