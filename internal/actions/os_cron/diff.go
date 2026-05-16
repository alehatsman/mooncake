package os_cron //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for os.cron (spec-22 phase 4 /
// spec-28 P6). The typed Before/After payload is actions.CronDiff
// (spec-66 wave 5); see internal/actions/diff_payloads.go.
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
		After: &actions.CronDiff{
			Name:     c.Name,
			State:    state,
			User:     c.User,
			Schedule: schedule,
			Command:  c.Command,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
