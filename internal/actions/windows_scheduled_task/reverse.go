package windows_scheduled_task

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// WindowsScheduledTaskReverseInfo is stashed on Result.ReverseData at
// apply time. It captures enough pre-apply state so Reverse() can emit
// the inverse step without re-querying the live system.
type WindowsScheduledTaskReverseInfo struct {
	// AppliedState is the step's normalised state: "present" or "absent".
	AppliedState string

	// TaskName is the task identity (required for every reverse step).
	TaskName string

	// PriorExisted is true when a task with the same name existed before
	// apply. False means the apply was a pure create.
	PriorExisted bool
}

// Reverse implements actions.Reverser for windows.scheduled_task.
//
// Inverse-state strategy:
//   - AppliedState=present, PriorExisted=false (create) → state=absent step
//     that removes the task we just created. Fully supported.
//   - AppliedState=present, PriorExisted=true  (update) → not yet supported.
//     Restoring a prior task faithfully requires round-tripping through the
//     exported XML, which is a compound operation (stage XML → Register with
//     -Force). Tracked as a v2 follow-up.
//   - AppliedState=absent                      (delete) → not yet supported
//     for the same reason: re-creating requires the original XML.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing / wrong type → defensive error.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.WindowsScheduledTask == nil {
		return nil, errors.New("windows.scheduled_task Reverse: step has no WindowsScheduledTask payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("windows.scheduled_task Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}

	info, ok := r.ReverseData.(*WindowsScheduledTaskReverseInfo)
	if !ok {
		return nil, fmt.Errorf("windows.scheduled_task Reverse: ReverseData is %T, want *WindowsScheduledTaskReverseInfo", r.ReverseData)
	}

	taskName := info.TaskName

	switch info.AppliedState {
	case statePresent:
		if !info.PriorExisted {
			// Apply created the task → delete it.
			return &config.Step{
				Name: "reverse: remove windows.scheduled_task " + taskName,
				WindowsScheduledTask: &config.WindowsScheduledTask{
					Name:  taskName,
					State: stateAbsent,
				},
			}, nil
		}
		// Apply updated an existing task → restoring requires the prior XML,
		// which is a multi-step compound operation not yet implemented.
		return nil, fmt.Errorf(
			"windows.scheduled_task Reverse: update-rollback for %q not yet supported — "+
				"restoring a prior task requires re-registering from the exported XML "+
				"(tracked as a v2 follow-up; create-then-rollback is supported)", taskName)

	case stateAbsent:
		// Apply deleted the task → restoring requires the prior XML.
		return nil, fmt.Errorf(
			"windows.scheduled_task Reverse: delete-rollback for %q not yet supported — "+
				"restoring requires re-registering from the exported XML "+
				"(tracked as a v2 follow-up)", taskName)
	}

	return nil, fmt.Errorf("windows.scheduled_task Reverse: unexpected AppliedState %q", info.AppliedState)
}

var _ actions.Reverser = (*Handler)(nil)
