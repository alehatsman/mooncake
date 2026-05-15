package os_cron //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// OsCronSnapshot is the typed Before/After payload for actions.Diff
// when the resource is an os.cron step. Describes user INTENT.
type OsCronSnapshot struct {
	// Name is the cron entry's filename in /etc/cron.d (identity).
	Name string `json:"name,omitempty"`

	// State is "present" or "absent". Empty defaults to "present".
	State string `json:"state,omitempty"`

	// User the command runs as. Empty maps to root.
	User string `json:"user,omitempty"`

	// Schedule is the rendered cron schedule. When the step uses
	// the individual minute/hour/... fields, those are folded into
	// one whitespace-joined string here for consumer simplicity.
	Schedule string `json:"schedule,omitempty"`

	// Command is the command line to run.
	Command string `json:"command,omitempty"`
}

// Diff implements actions.Differ for os.cron (spec-22 phase 4 /
// spec-28 P6).
//
// Operation by state: present→OpCreate, absent→OpDelete. Diff
// doesn't read the existing file at plan time; the runtime
// idempotency check produces accurate Changed=false on already-
// converged entries.
//
// Resource.Kind = ResourceOther, Identifier = the cron entry's
// name, Attributes["kind"] = "os.cron".
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.OsCron == nil {
		return actions.Diff{}, errors.New("os.cron Diff: step has no OsCron payload")
	}
	c := step.OsCron
	state := c.State
	if state == "" {
		state = "present"
	}

	op := actions.OpCreate
	if state == "absent" {
		op = actions.OpDelete
	}

	schedule := c.Schedule
	if schedule == "" {
		// Build the canonical 5-field form from the individual
		// pieces, defaulting each empty field to "*". This matches
		// the convention the underlying file write uses.
		fields := []string{c.Minute, c.Hour, c.Day, c.Month, c.Weekday}
		for i, f := range fields {
			if f == "" {
				fields[i] = "*"
			}
		}
		schedule = fields[0] + " " + fields[1] + " " + fields[2] + " " + fields[3] + " " + fields[4]
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: c.Name,
			Attributes: map[string]string{"kind": "os.cron"},
		},
		Operation: op,
		Before:    nil,
		After: &OsCronSnapshot{
			Name:     c.Name,
			State:    state,
			User:     c.User,
			Schedule: schedule,
			Command:  c.Command,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
