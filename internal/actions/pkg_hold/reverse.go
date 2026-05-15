//nolint:revive // Package name matches action name convention (pkg_hold)
package pkg_hold

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// PkgHoldReverseInfo is the per-step apply-time snapshot pkg.hold
// stashes on Result.ReverseData. Captures the manager and the
// direction the apply went (Held vs Unheld) plus the exact list of
// package names that were actually flipped. The flipped set
// excludes packages already in the desired pre-apply state — a
// state=held step targeting 10 names but finding 6 already held
// only stores the 4 it actually flipped, so reverse unholds
// exactly those 4.
//
// Mutually-exclusive direction: PkgHold.State pins one direction
// per step, so we never have a mixed payload here. Reverse just
// flips AppliedState.
type PkgHoldReverseInfo struct {
	// Manager is the package manager the apply ran against. v1 is
	// always "apt"; preserved so a future multi-manager rollout
	// reverses against the same manager rather than re-detecting.
	Manager string

	// AppliedState is the step's State at apply time: "held" or
	// "unheld". Determines the inverse direction.
	AppliedState string

	// Mutated is the list of package names that the apply
	// actually flipped. nil/empty means the step was a no-op and
	// Reverse will return (nil, nil) per the contract.
	Mutated []string
}

// Reverse implements actions.Reverser for pkg.hold (spec-24 P6 /
// reverse-capture v2).
//
// Returns a pkg.hold step with the inverse state on the captured
// Mutated names. Edge cases:
//   - ReverseData nil → apply was a noop (everything already at
//     desired state), return (nil, nil).
//   - Empty Mutated list → noop, same.
//   - Step has no PkgHold payload or result is wrong type →
//     defensive error.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.PkgHold == nil {
		return nil, errors.New("pkg.hold Reverse: step has no PkgHold payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("pkg.hold Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*PkgHoldReverseInfo)
	if !ok {
		return nil, fmt.Errorf("pkg.hold Reverse: ReverseData is %T, want *PkgHoldReverseInfo", r.ReverseData)
	}
	if len(info.Mutated) == 0 {
		return nil, nil
	}

	inverse := stateHeld
	if info.AppliedState == stateHeld {
		inverse = stateUnheld
	}

	return &config.Step{
		PkgHold: &config.PkgHold{
			Names:   append([]string(nil), info.Mutated...),
			State:   inverse,
			Manager: info.Manager,
		},
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
