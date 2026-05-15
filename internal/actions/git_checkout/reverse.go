package git_checkout //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// GitCheckoutReverseInfo is the per-step apply-time snapshot that
// git.checkout stashes on Result.ReverseData. Captures the resolved
// pre-checkout HEAD sha plus the dest path so Reverse can build a
// `git.checkout` step that switches the working tree back to where
// it was before this step ran.
//
// Only populated when the apply path actually mutated state
// (Changed=true). When the apply was a noop (HEAD already at the
// requested ref) ReverseData stays nil and Reverse returns
// (nil, nil) per the Reverser contract.
type GitCheckoutReverseInfo struct {
	// Dest is the working-tree path the step targeted. Required so
	// Reverse can re-build a `git.checkout` step against the same
	// repo without re-resolving template / path-expansion logic.
	Dest string

	// PriorSHA is the resolved HEAD sha BEFORE the checkout. The
	// reverse step uses this as `ref:` (sha refs are always
	// resolvable; branches/tags might not exist after the apply).
	PriorSHA string
}

// Reverse implements actions.Reverser for git.checkout (spec-26
// phase 5 / spec-26 reverse-capture follow-up).
//
// Returns a `git.checkout` step that switches `Dest` back to the
// pre-apply HEAD sha. Force=true on the reverse step so a working
// tree that drifted between apply and rollback (rare but possible —
// e.g. another tool committed locally) doesn't refuse the rollback.
//
// Edge cases handled by the Reverser contract:
//   - Apply was a noop → ReverseData stays nil in Run → here we
//     return (nil, nil) ("no reverse needed" per the contract).
//   - Apply failed before capture → ReverseData stays nil → same
//     (nil, nil) return path; nothing was mutated to undo.
//   - Step has no GitCheckout payload → defensive error.
//   - ReverseData has wrong type → defensive error (only matters if
//     a future refactor accidentally sets a foreign type on this
//     handler's results).
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.GitCheckout == nil {
		return nil, errors.New("git.checkout Reverse: step has no GitCheckout payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("git.checkout Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		// Apply was a noop (HEAD already at requested ref) — nothing
		// to undo.
		return nil, nil
	}
	info, ok := r.ReverseData.(*GitCheckoutReverseInfo)
	if !ok {
		return nil, fmt.Errorf("git.checkout Reverse: ReverseData is %T, want *GitCheckoutReverseInfo", r.ReverseData)
	}
	if info.PriorSHA == "" || info.Dest == "" {
		return nil, fmt.Errorf("git.checkout Reverse: incomplete ReverseData %+v", info)
	}

	return &config.Step{
		GitCheckout: &config.GitCheckout{
			Dest:  info.Dest,
			Ref:   info.PriorSHA,
			Force: true,
		},
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
