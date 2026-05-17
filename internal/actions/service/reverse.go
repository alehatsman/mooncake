package service

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// OsServiceReverseInfo is the per-step apply-time snapshot
// os.service stashes on Result.ReverseData via the Run/Execute
// wrap (not via a HandleService signature change — see comment on
// captureSystemdPriorState below).
//
// Captures the service unit's pre-apply (active, enabled) tuple
// plus the unit's intent flags from the step so Reverse can build
// a faithful inverse without having to re-render templates.
//
// Captured for Linux (systemd) and darwin (launchd) — runApply
// dispatches on runtime.GOOS. Windows: not yet — apply-side
// handleWindowsService is a stub (handler.go ~1021), so reverse
// would have nothing to invert. Reverse() refuses on windows.
//
// Platform field records which OS captured the snapshot so Reverse
// can apply platform-specific honoring rules (notably: darwin
// skips State reverse because launchd's transient/load distinction
// doesn't map cleanly to systemd's started/stopped pair).
type OsServiceReverseInfo struct {
	// Platform is runtime.GOOS at capture time ("linux" or
	// "darwin"). Empty string on older payloads is treated as
	// "linux" by Reverse for backward compatibility — pre-darwin
	// snapshots were always systemd.
	Platform string

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
	info := &OsServiceReverseInfo{Name: serviceName, Platform: "linux"}
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

// captureLaunchdPriorState is the darwin analogue of
// captureSystemdPriorState. launchd's surface is narrower than
// systemd's: `isLaunchdServiceLoaded` returns one bool that
// conflates two systemd concepts (active + enabled). For reverse
// purposes we map that single signal to PriorEnabled and leave
// PriorActive untouched — Reverse() on darwin only honors the
// HadEnabledIntent path.
//
// Why not try harder for active: launchd treats "loaded" as the
// persistent state ("the system knows about this service") and
// "running" as transient. systemd's started/stopped pair maps to
// active/inactive cleanly; launchd's bootstrap/bootout maps to
// loaded/unloaded. Trying to invent a "started/stopped" reverse
// step on launchd would over-promise — better to refuse the State
// reverse explicitly than emit a step that does something subtly
// wrong on rollback.
func captureLaunchdPriorState(serviceName string, step config.Step, ec *executor.ExecutionContext) *OsServiceReverseInfo {
	info := &OsServiceReverseInfo{Name: serviceName, Platform: "darwin"}
	if step.OsService != nil {
		info.HadEnabledIntent = step.OsService.Enabled != nil
		info.HadStateIntent = step.OsService.State != ""
	}
	isSystem := step.ShouldBecome()
	domain := getLaunchdDomain(isSystem)
	serviceID := fmt.Sprintf("%s/%s", domain, serviceName)
	loaded, err := isLaunchdServiceLoaded(serviceID, step, ec)
	if err == nil {
		info.PriorEnabled = loaded
		// Conservative carry-through: if the user is comparing
		// PriorActive across platforms (e.g. in a fleet-status
		// view), having it tied to the loaded bit is more
		// useful than a zero-value default. Reverse() doesn't
		// consume PriorActive on darwin, so this is metadata
		// only.
		info.PriorActive = loaded
	}
	return info
}

// Reverse implements actions.Reverser for os.service (spec-22 phase
// 5 slice F / reverse-capture v6).
//
// Strategy: emit an os.service step that flips State + Enabled
// back to their pre-apply values. HandleService routes the reverse
// step back through the existing per-OS dispatcher at apply time,
// so this function stays platform-agnostic; only the prior-state
// capture and the per-platform State-reverse policy differ.
//
// Per-platform State-reverse policy:
//   - linux (systemd): both State and Enabled honored. State maps
//     active → started, inactive → stopped.
//   - darwin (launchd): only Enabled honored. launchd's transient
//     "running" vs. persistent "loaded" doesn't map cleanly to
//     systemd's started/stopped, so a State reverse would
//     over-promise (see captureLaunchdPriorState comment). The
//     load/unload axis maps via Enabled, which is enough for the
//     common "apply loaded a daemon; rollback unloads it" case.
//   - windows: refuse. Apply-side is a stub (handler.go ~1021),
//     so the ReverseData will be nil and Reverse returns (nil, nil).
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing / wrong type → defensive error.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.OsService == nil {
		return nil, errors.New("os.service Reverse: step has no OsService payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("os.service Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		// Either the apply was a noop, or this is a windows
		// runApply path where state capture didn't fire — either
		// way, nothing to undo.
		return nil, nil
	}
	info, ok := r.ReverseData.(*OsServiceReverseInfo)
	if !ok {
		return nil, fmt.Errorf("os.service Reverse: ReverseData is %T, want *OsServiceReverseInfo", r.ReverseData)
	}
	if info.Name == "" {
		return nil, errors.New("os.service Reverse: incomplete ReverseData (no Name)")
	}

	// Treat absent Platform as "linux" — backward-compat with
	// pre-darwin reverse snapshots that didn't tag the platform.
	platform := info.Platform
	if platform == "" {
		platform = "linux"
	}

	rev := &config.ServiceAction{
		Name: info.Name,
	}

	// Re-assert the prior state when the apply set one and the
	// platform supports a clean inversion. darwin opts out — see
	// the function-level doc.
	if info.HadStateIntent && platform == "linux" {
		if info.PriorActive {
			rev.State = ServiceStateStarted
		} else {
			rev.State = ServiceStateStopped
		}
	}

	// Re-assert the prior enabled flag if the apply managed it.
	// Both linux and darwin honor this — Enabled maps cleanly to
	// load/unload on launchd and to systemctl enable/disable on
	// systemd.
	if info.HadEnabledIntent {
		priorEnabled := info.PriorEnabled
		rev.Enabled = &priorEnabled
	}

	// If neither State nor Enabled is set on the reverse step, the
	// apply was a noop in lifecycle terms — return (nil, nil) so
	// the transaction layer skips it. (Catches a corner case where
	// HandleService changed unit content but no lifecycle pin —
	// and on darwin, where HadStateIntent without HadEnabledIntent
	// reaches here with neither field set.)
	if rev.State == "" && rev.Enabled == nil {
		return nil, nil
	}

	return &config.Step{
		Name:      "reverse: restore service state for " + info.Name,
		OsService: rev,
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
