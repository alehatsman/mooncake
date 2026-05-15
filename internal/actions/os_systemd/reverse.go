package os_systemd //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// OsSystemdReverseInfo is the per-step apply-time snapshot
// os.systemd stashes on Result.ReverseData. Captures the unit
// file existence + content + the lifecycle flags (is-enabled,
// is-active) pre-apply.
//
// Scope of Reverse:
//   - PriorExisted=false → reverse is a single os.systemd
//     state=absent step (handles delete + stop + disable in one
//     pass). This is the canonical "create-then-rollback" path
//     and the common transactional case.
//   - PriorExisted=true  → REFUSED. A faithful restore needs
//     (a) the prior unit file content rewritten, (b) systemctl
//     daemon-reload, (c) enabled flag restored, (d) active flag
//     restored. The Reverser interface returns one step;
//     expressing all four as one os.systemd step requires parsing
//     the prior raw content back to section maps (fragile), and
//     emitting a single file.write step loses daemon-reload +
//     lifecycle. Tracked as a v6 follow-up that probably needs
//     either a multi-step Reverser variant or a `raw_content:`
//     field on os.systemd to bypass the section-map renderer.
type OsSystemdReverseInfo struct {
	// Name is the unit name (e.g. "myapp.service").
	Name string

	// Path is the resolved unit file path.
	Path string

	// PriorExisted reports whether the unit file existed
	// pre-apply.
	PriorExisted bool

	// PriorContent is the verbatim unit file bytes pre-apply.
	// Empty when PriorExisted is false. Captured for future v6
	// use (raw-content reverse) but not consumed by v5's Reverse.
	PriorContent string

	// PriorEnabled / HadPriorEnabled record systemctl is-enabled
	// pre-apply. HadPriorEnabled is false when the query errored
	// (e.g. unit-not-found before file existed).
	PriorEnabled    bool
	HadPriorEnabled bool

	// PriorActive / HadPriorActive record systemctl is-active
	// pre-apply.
	PriorActive    bool
	HadPriorActive bool
}

// captureReverseInfo runs at apply time, after computePlan and
// before applyPlan. Reads the unit file and queries systemctl
// is-enabled / is-active for the unit. All four pieces are
// best-effort: failures to read either leave the corresponding
// "Had..." flag false.
func captureReverseInfo(name, path string) *OsSystemdReverseInfo {
	info := &OsSystemdReverseInfo{Name: name, Path: path}

	if content, exists, err := readFile(path); err == nil {
		info.PriorExisted = exists
		if exists {
			info.PriorContent = content
		}
	}
	if enabled, err := systemctlIsEnabled(name); err == nil {
		info.PriorEnabled = enabled
		info.HadPriorEnabled = true
	}
	if active, err := systemctlIsActive(name); err == nil {
		info.PriorActive = active
		info.HadPriorActive = true
	}
	return info
}

// Reverse implements actions.Reverser for os.systemd (spec-28 P6 /
// reverse-capture v5).
//
// v5 scope:
//   - PriorExisted=false → reverse is os.systemd state=absent on
//     the captured Name. os.systemd's apply path handles
//     stop+disable+delete in one pass.
//   - PriorExisted=true → refuse with a clear message naming the
//     v6 follow-up. A faithful four-piece restore needs
//     multi-step reverse machinery that doesn't exist today.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing / wrong type → defensive error.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.OsSystemd == nil {
		return nil, errors.New("os.systemd Reverse: step has no OsSystemd payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("os.systemd Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*OsSystemdReverseInfo)
	if !ok {
		return nil, fmt.Errorf("os.systemd Reverse: ReverseData is %T, want *OsSystemdReverseInfo", r.ReverseData)
	}
	if info.Name == "" {
		return nil, errors.New("os.systemd Reverse: incomplete ReverseData (no Name)")
	}

	if !info.PriorExisted {
		return &config.Step{
			Name: "reverse: remove " + info.Name,
			OsSystemd: &config.OsSystemd{
				Name:  info.Name,
				State: "absent",
			},
		}, nil
	}

	return nil, errors.New( //nolint:staticcheck
		"os.systemd Reverse: modify-rollback (PriorExisted=true) not " +
			"yet supported. A faithful restore needs the prior unit " +
			"file content + daemon-reload + enabled/active flags as a " +
			"compound step — tracked as a v6 follow-up. Pure create- " +
			"then-rollback (PriorExisted=false) is supported.")
}

var _ actions.Reverser = (*Handler)(nil)
