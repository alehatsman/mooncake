package service

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Reverse implements actions.Reverser for os.service (spec-22 phase
// 5 slice F / reverse-capture v6).
//
// Strategy: emit an os.service step that flips State + Enabled
// back to their pre-apply values. HandleService routes the reverse
// step back through the existing per-OS dispatcher at apply time,
// so this function stays platform-agnostic; only the prior-state
// capture (in the per-OS sub-packages) and the per-platform
// State-reverse policy differ.
//
// The OsServiceReverseInfo type itself lives in service/shared (it's
// re-exported here as a type alias from handler.go) so the per-OS
// sub-packages can populate it without forming an import cycle with
// this parent package.
//
// Per-platform State-reverse policy:
//   - linux (systemd): both State and Enabled honored. State maps
//     active → started, inactive → stopped.
//   - darwin (launchd): only Enabled honored. launchd's transient
//     "running" vs. persistent "loaded" doesn't map cleanly to
//     systemd's started/stopped, so a State reverse would
//     over-promise (see darwin.CapturePriorState comment). The
//     load/unload axis maps via Enabled, which is enough for the
//     common "apply loaded a daemon; rollback unloads it" case.
//   - windows (SCM): only Enabled honored. Same rationale as
//     darwin — Running/Stopped are transient (the service may
//     auto-start between apply and reverse-apply if StartType =
//     Automatic), so State inversion would over-promise. The
//     StartType axis (Automatic ↔ Disabled) maps cleanly via
//     Enabled.
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
	// platform supports a clean inversion. darwin + windows opt out
	// — see the function-level doc.
	if info.HadStateIntent && platform == "linux" {
		if info.PriorActive {
			rev.State = ServiceStateStarted
		} else {
			rev.State = ServiceStateStopped
		}
	}

	// Re-assert the prior enabled flag if the apply managed it.
	// All three platforms honor this — Enabled maps cleanly to
	// load/unload on launchd, enable/disable on systemd, and
	// Automatic/Disabled on Windows SCM.
	if info.HadEnabledIntent {
		priorEnabled := info.PriorEnabled
		rev.Enabled = &priorEnabled
	}

	// If neither State nor Enabled is set on the reverse step, the
	// apply was a noop in lifecycle terms — return (nil, nil) so
	// the transaction layer skips it. (Catches a corner case where
	// HandleService changed unit content but no lifecycle pin —
	// and on darwin/windows, where HadStateIntent without
	// HadEnabledIntent reaches here with neither field set.)
	if rev.State == "" && rev.Enabled == nil {
		return nil, nil
	}

	return &config.Step{
		Name:      "reverse: restore service state for " + info.Name,
		OsService: rev,
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
