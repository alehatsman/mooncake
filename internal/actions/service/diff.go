package service

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// ServiceSnapshot is the typed Before/After payload for actions.Diff
// when the resource kind is ResourceService. Spec-22 §"Diff" says each
// handler family defines its own snapshot shape; the service equivalent
// of FileSnapshot is named-state-flags, not path-and-bytes.
//
// Before is nil for os.service Diff (we don't shell out to
// systemctl/launchctl/sc.exe at plan time to read current state — that
// would couple Diff to platform-specific tools and slow the plan loop).
// After.ServiceSnapshot describes the user's INTENT — what state and
// flags the step would aim for — and Operation is always OpUpdate
// because every supported state value triggers a real mutation
// (start/stop/restart/reload changes runtime state, enabled toggles
// boot-time wiring, daemon_reload kicks the unit cache).
type ServiceSnapshot struct {
	// Name is the unit / service name (e.g. "ssh", "nginx",
	// "com.example.daemon"). Mirrors step.OsService.Name.
	Name string `json:"name,omitempty"`

	// State is the desired runtime state: "started" / "stopped" /
	// "restarted" / "reloaded". Empty when the step only changes
	// Enabled or unit-file content.
	State string `json:"state,omitempty"`

	// Enabled is the desired boot-time enabled flag. nil means
	// "leave unchanged"; otherwise *Enabled is the desired value.
	Enabled *bool `json:"enabled,omitempty"`

	// DaemonReload mirrors the step flag so consumers can see a
	// systemctl daemon-reload was requested before state was
	// changed (relevant for unit-file edits).
	DaemonReload bool `json:"daemon_reload,omitempty"`
}

// Diff implements actions.Differ for os.service (spec-22 phase 4d).
//
// Conservative semantics — Operation is always OpUpdate. We can't
// predict noop without shelling out to the service manager. The
// runtime path produces the actual changed=false on a converged
// system.
//
// Resource.Kind = ResourceService, Resource.Identifier = service name.
// Before is nil (no measurement); After is a ServiceSnapshot describing
// what the step would aim for.
func (h *Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.OsService == nil {
		return actions.Diff{}, errors.New("os.service Diff: step has no OsService payload")
	}
	svc := step.OsService

	after := &ServiceSnapshot{
		Name:         svc.Name,
		State:        svc.State,
		Enabled:      svc.Enabled,
		DaemonReload: svc.DaemonReload,
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceService,
			Identifier: svc.Name,
		},
		// Always OpUpdate — every os.service intent (start/stop/
		// enable/reload/unit-content-change) is a mutation. Diff
		// doesn't try to detect already-converged systems.
		Operation: actions.OpUpdate,
		Before:    nil,
		After:     after,
	}, nil
}

// Compile-time interface check.
var _ actions.Differ = (*Handler)(nil)
