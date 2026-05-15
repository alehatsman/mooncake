package os_systemd //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// OsSystemdSnapshot is the typed Before/After payload for
// actions.Diff when the resource is an os.systemd step. Describes
// user INTENT for both the unit file content (sections supplied)
// and its lifecycle flags (enabled, started).
type OsSystemdSnapshot struct {
	// Name is the unit filename with suffix (e.g. "myapp.service").
	Name string `json:"name,omitempty"`

	// State is "present" or "absent". Empty defaults to "present".
	State string `json:"state,omitempty"`

	// Path is the unit directory (default /etc/systemd/system).
	Path string `json:"path,omitempty"`

	// Sections lists the unit-file sections this step populates
	// (Unit, Service, Timer, Socket, Install). Lets consumers see
	// the shape of the unit without surfacing the (potentially
	// large or sensitive) Exec lines themselves.
	Sections []string `json:"sections,omitempty"`

	// Enabled / Started mirror the desired lifecycle flags (nil
	// means "leave alone").
	Enabled *bool `json:"enabled,omitempty"`
	Started *bool `json:"started,omitempty"`
}

// Diff implements actions.Differ for os.systemd (spec-22 phase 4 /
// spec-28 P6).
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
		After: &OsSystemdSnapshot{
			Name:     s.Name,
			State:    state,
			Path:     s.Path,
			Sections: sections,
			Enabled:  s.Enabled,
			Started:  s.Started,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
