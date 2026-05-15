package service

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// OsServiceReverseInfo is the per-step apply-time snapshot
// os.service stashes on Result.ReverseData via the Run/Execute
// wrap (not via a HandleService signature change — see comment on
// captureSystemdPriorState below).
//
// Captures the systemd unit's pre-apply (active, enabled) tuple
// plus the unit's intent flags from the step so Reverse can build
// a faithful inverse without having to re-render templates.
//
// Linux-only: the macOS launchd and Windows SCM paths don't go
// through this capture — Run's wrap checks runtime.GOOS first.
type OsServiceReverseInfo struct {
	// Name is the unit name (e.g. "myapp.service"). Identity for
	// the reverse step.
	Name string

	// PriorActive reports whether systemctl is-active returned
	// "active" pre-apply.
	PriorActive bool

	// PriorEnabled reports whether systemctl is-enabled returned
	// "enabled" / "static" / "indirect" pre-apply.
	PriorEnabled bool

	// HadEnabledIntent reports whether the apply-time step pinned
	// an Enabled value (i.e. step.OsService.Enabled != nil). Used
	// by Reverse to decide whether to set the inverse Enabled
	// field — if the apply didn't manage enabled state, neither
	// should the reverse.
	HadEnabledIntent bool

	// HadStartedIntent mirrors HadEnabledIntent for the started
	// flag.
	HadStartedIntent bool

	// HadStateIntent reports whether the apply pinned a State
	// (started/stopped/restarted/reloaded). When false, the
	// reverse leaves State empty too.
	HadStateIntent bool
}

// captureSystemdPriorState queries systemctl is-active and
// is-enabled for the named unit, returning a snapshot suitable
// for Result.ReverseData. Best-effort: failures default to
// false (not active, not enabled).
//
// Why this is a free function rather than a HandleService
// signature change: HandleService writes its Result into
// ec.CurrentResult via a defer; refactoring it to return the
// Result would touch every platform branch (systemd, launchd,
// Windows) plus tests. Wrapping at Run / Execute level lets us
// add capture surgically while leaving the legacy contract alone.
func captureSystemdPriorState(serviceName string, step config.Step, ec *executor.ExecutionContext) *OsServiceReverseInfo {
	info := &OsServiceReverseInfo{Name: serviceName}
	if step.OsService != nil {
		info.HadEnabledIntent = step.OsService.Enabled != nil
		info.HadStartedIntent = false // No `started` flag on ServiceAction yet
		info.HadStateIntent = step.OsService.State != ""
	}

	state, err := getSystemdServiceState(serviceName, step, ec)
	if err == nil {
		info.PriorActive = state == "active"
	}
	if enabled, err := isSystemdServiceEnabled(serviceName, step, ec); err == nil {
		info.PriorEnabled = enabled
	}
	return info
}

// Reverse implements actions.Reverser for os.service (spec-22 phase
// 5 slice F / reverse-capture v6).
//
// Strategy: emit an os.service step that flips State + Enabled
// back to their pre-apply values. The Reverse step only sets
// fields the apply step actively managed:
//   - HadStateIntent → inverse State (active → stopped, inactive →
//     started). The apply's State semantics are direct (started,
//     stopped, etc.) so we read PriorActive to pick the inverse.
//   - HadEnabledIntent → inverse Enabled (boolean).
//
// Non-Linux: refuse. The legacy launchd / Windows paths don't go
// through captureSystemdPriorState, so ReverseData will be nil
// and the (nil, nil) return covers them.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing / wrong type → defensive error.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.OsService == nil {
		return nil, errors.New("os.service Reverse: step has no OsService payload")
	}

	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("os.service Reverse: not implemented on %s (only Linux/systemd supported in v6)", runtime.GOOS)
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("os.service Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*OsServiceReverseInfo)
	if !ok {
		return nil, fmt.Errorf("os.service Reverse: ReverseData is %T, want *OsServiceReverseInfo", r.ReverseData)
	}
	if info.Name == "" {
		return nil, errors.New("os.service Reverse: incomplete ReverseData (no Name)")
	}

	rev := &config.ServiceAction{
		Name: info.Name,
	}

	// Re-assert the prior state when the apply set one. The mapping
	// goes runtime-state → step-State: active → started, inactive
	// → stopped.
	if info.HadStateIntent {
		if info.PriorActive {
			rev.State = ServiceStateStarted
		} else {
			rev.State = ServiceStateStopped
		}
	}

	// Re-assert the prior enabled flag if the apply managed it.
	if info.HadEnabledIntent {
		priorEnabled := info.PriorEnabled
		rev.Enabled = &priorEnabled
	}

	// If neither State nor Enabled is set on the reverse step, the
	// apply was a noop in lifecycle terms — return (nil, nil) so
	// the transaction layer skips it. (Catches a corner case where
	// HandleService changed unit content but no lifecycle pin.)
	if rev.State == "" && rev.Enabled == nil {
		return nil, nil
	}

	return &config.Step{
		Name:      "reverse: restore service state for " + info.Name,
		OsService: rev,
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
