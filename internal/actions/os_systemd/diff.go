package os_systemd //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for os.systemd (spec-22 phase 4 /
// spec-28 P6). The typed Before/After payload is actions.ServiceDiff
// (spec-66 wave 4); see internal/actions/diff_payloads.go.
//
// Operation by state: present→OpCreate (idempotency check at apply
// collapses noop), absent→OpDelete.
//
// Resource.Kind = ResourceService — even though os.systemd writes
// a file, its semantic identity is the unit name and the lifecycle
// effect is service-shaped; ResourceService is the right consumer
// dispatch kind. Identifier = the unit name.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.OsSystemd == nil {
		return actions.Diff{}, errors.New("os.systemd Diff: step has no OsSystemd payload")
	}
	s := step.OsSystemd
	state := s.State
	if state == "" {
		state = "present"
	}

	op := actions.OpCreate
	if state == "absent" {
		op = actions.OpDelete
	}

	sections := []string{}
	if len(s.Unit) > 0 {
		sections = append(sections, "Unit")
	}
	if len(s.Service) > 0 {
		sections = append(sections, "Service")
	}
	if len(s.Timer) > 0 {
		sections = append(sections, "Timer")
	}
	if len(s.Socket) > 0 {
		sections = append(sections, "Socket")
	}
	if len(s.Install) > 0 {
		sections = append(sections, "Install")
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceService,
			Identifier: s.Name,
		},
		Operation: op,
		Before:    nil,
		After: &actions.ServiceDiff{
			Name:     s.Name,
			State:    state,
			Scope:    normalizeScope(s.Scope),
			Path:     s.Path,
			Sections: sections,
			Enabled:  s.Enabled,
			Started:  s.Started,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
