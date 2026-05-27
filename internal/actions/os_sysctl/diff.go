package os_sysctl //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for os.sysctl (spec-22 phase 4 /
// spec-28 P6 / spec-66 wave 8).
//
// Operation: state=present → OpUpdate (sysctl values almost always
// change the runtime kernel parameter, even when the file entry was
// already present); state=absent → OpDelete.
//
// Diff doesn't read /proc/sys at plan time; the runtime check
// collapses already-converged keys to Changed=false.
//
// Resource.Attributes["kind"] = "os.sysctl" dispatches the
// render_sysctl matcher.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.OsSysctl == nil {
		return actions.Diff{}, errors.New("os.sysctl Diff: step has no OsSysctl payload")
	}
	s := step.OsSysctl
	state := s.State
	if state == "" {
		state = "present"
	}

	op := actions.OpUpdate
	if state == "absent" {
		op = actions.OpDelete
	}

	valueStr := ""
	if s.Value != nil {
		valueStr = fmt.Sprintf("%v", s.Value)
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: s.Name,
			Attributes: map[string]string{"kind": "os.sysctl"},
		},
		Operation: op,
		Before:    nil,
		After: &actions.SysctlDiff{
			Name:  s.Name,
			State: state,
			Value: valueStr,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
