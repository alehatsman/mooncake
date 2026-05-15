package os_firewall //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// OsFirewallSnapshot is the typed Before/After payload for
// actions.Diff when the resource is an os.firewall step. Describes
// user INTENT.
type OsFirewallSnapshot struct {
	// Backend is the firewall backend (v1: "ufw" only).
	Backend string `json:"backend,omitempty"`

	// State is "present" or "absent". Empty defaults to "present".
	State string `json:"state,omitempty"`

	// RuleCount is the number of rules in the step (1 for single,
	// len(Rules) for multi-rule). The rule details are NOT surfaced
	// in After to keep the structural surface light; the Diff
	// Operation + count is enough for plan rendering / cost.
	RuleCount int `json:"rule_count"`
}

// Diff implements actions.Differ for os.firewall (spec-22 phase 4 /
// spec-28 P6).
//
// Operation by state: present→OpCreate, absent→OpDelete.
//
// Resource.Kind = ResourceOther; Identifier = "ufw:<count>rules"
// so plan listings distinguish multiple firewall steps. The detail
// (which ports, which actions) lives in the step itself; we don't
// echo it in Diff to keep the structural surface light — and
// because firewall rule lists may carry security-sensitive ports
// or source addresses.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.OsFirewall == nil {
		return actions.Diff{}, errors.New("os.firewall Diff: step has no OsFirewall payload")
	}
	f := step.OsFirewall
	state := f.State
	if state == "" {
		state = "present"
	}

	op := actions.OpCreate
	if state == "absent" {
		op = actions.OpDelete
	}

	backend := f.Backend
	if backend == "" {
		backend = "auto"
	}

	count := len(f.Rules)
	if f.Rule != nil {
		count++
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: fmt.Sprintf("%s:%drules", backend, count),
			Attributes: map[string]string{"kind": "os.firewall", "backend": backend},
		},
		Operation: op,
		Before:    nil,
		After: &OsFirewallSnapshot{
			Backend:   backend,
			State:     state,
			RuleCount: count,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
