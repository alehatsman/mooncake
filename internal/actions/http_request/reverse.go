package http_request

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Reverse implements actions.Reverser for proposal-16 Wave 2.
//
// An http.request is irreversible by default — there's no on-wire
// equivalent of "restore previous bytes." Reversibility is OPT-IN: the
// user declares a `reverse:` block describing the compensating call
// (typically a DELETE that mirrors a POST). When that block is set,
// runApply snapshots it onto Result.ReverseData with templates already
// resolved against the response fact, so this method just wraps the
// snapshot in a Step and hands it back to the transaction layer.
//
// Return semantics (spec-22 phase 5):
//   - (step, nil)  → apply this Step to undo
//   - (nil, error) → handler refuses; transaction must surface to the
//     operator. Used when reverse: was not declared, or when the
//     snapshot is missing (apply didn't reach success).
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.HTTPRequest == nil {
		return nil, errors.New("http.request Reverse: step has no HTTPRequest payload")
	}
	if step.HTTPRequest.Reverse == nil {
		// Declaring this explicitly — vs. silently returning nil, nil —
		// is the kernel-honest choice: a transaction rolling back over
		// a POST should fail loudly rather than skip the compensation.
		return nil, fmt.Errorf("%s: action is not reversible (declare a reverse: block to opt in)", actionName)
	}

	res, ok := result.(*executor.Result)
	if !ok {
		return nil, fmt.Errorf("%s Reverse: unexpected result type %T", actionName, result)
	}
	if res.ReverseData == nil {
		// Apply failed before we could snapshot; nothing to undo.
		return nil, fmt.Errorf("%s: reverse snapshot missing (apply did not complete successfully)", actionName)
	}
	rendered, ok := res.ReverseData.(*config.HTTPRequest)
	if !ok {
		return nil, fmt.Errorf("%s Reverse: unexpected ReverseData type %T", actionName, res.ReverseData)
	}

	return &config.Step{
		Name:        step.Name + " (reverse)",
		HTTPRequest: rendered,
	}, nil
}

// Compile-time interface check.
var _ actions.Reverser = (*Handler)(nil)
