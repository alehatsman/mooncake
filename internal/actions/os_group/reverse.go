package os_group //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// OsGroupReverseInfo is the per-step apply-time snapshot os.group
// stashes on Result.ReverseData. Captures the group's pre-apply
// existence + gid. Members aren't restored: per the spec, group
// renumbering is refused (changing gid silently breaks file
// ownership) and member changes are out of scope for os.group
// itself (os.user owns the supplementary-groups membership).
type OsGroupReverseInfo struct {
	// Name is the group name (identity for idempotency).
	Name string

	// AppliedState is the step's State at apply time:
	// "present" or "absent".
	AppliedState string

	// PriorExisted reports whether the group existed pre-apply.
	PriorExisted bool

	// PriorGID is the numeric gid pre-apply. Zero when
	// PriorExisted is false.
	PriorGID int
}

// Reverse implements actions.Reverser for os.group (spec-27 P4 /
// reverse-capture v3).
//
// Strategy:
//   - PriorExisted=false → reverse is state=absent (apply created
//     the group; rollback removes it).
//   - PriorExisted=true  → reverse is state=present with the
//     captured GID. When AppliedState was "present" this is a
//     same-shape noop on a converged system (the group already
//     exists with the captured gid); when AppliedState was
//     "absent" this recreates the group with its prior gid so
//     file ownership rejoins.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing / wrong type → defensive error.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.OsGroup == nil {
		return nil, errors.New("os.group Reverse: step has no OsGroup payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("os.group Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*OsGroupReverseInfo)
	if !ok {
		return nil, fmt.Errorf("os.group Reverse: ReverseData is %T, want *OsGroupReverseInfo", r.ReverseData)
	}
	if info.Name == "" {
		return nil, errors.New("os.group Reverse: incomplete ReverseData (no Name)")
	}

	if !info.PriorExisted {
		return &config.Step{
			Name: "reverse: remove group " + info.Name,
			OsGroup: &config.OsGroup{
				Name:  info.Name,
				State: "absent",
			},
		}, nil
	}

	gid := info.PriorGID
	return &config.Step{
		Name: "reverse: restore group " + info.Name,
		OsGroup: &config.OsGroup{
			Name:  info.Name,
			State: "present",
			GID:   &gid,
		},
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
