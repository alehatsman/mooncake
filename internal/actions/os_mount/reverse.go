package os_mount //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// OsMountReverseInfo is the per-step apply-time snapshot os.mount
// stashes on Result.ReverseData. Captures the dest path, prior
// fstab entry (if any), prior mount state, and which side(s) the
// apply touched. Same-action reverse: builds an os.mount step
// that re-asserts the pre-apply (entry, mounted) tuple.
type OsMountReverseInfo struct {
	// Dest is the mount point identity.
	Dest string

	// PriorEntry is the captured fstab entry pre-apply, nil if
	// no entry existed. Exported as a value-friendly snapshot
	// shape (not the unexported fstabEntry) so the test surface
	// is stable.
	PriorEntry *OsMountSnapshotEntry

	// PriorMounted reports whether dest was in /proc/mounts
	// pre-apply.
	PriorMounted bool

	// TouchedFstab / TouchedMount mirror plan flags — record
	// whether the apply rewrote /etc/fstab and/or invoked
	// mount/umount.
	TouchedFstab bool
	TouchedMount bool
}

// OsMountSnapshotEntry is the exported view of a captured fstab
// line. Field names line up with config.OsMount for one-to-one
// reverse-step construction.
type OsMountSnapshotEntry struct {
	Src     string
	Dest    string
	FSType  string
	Options []string
	Dump    int
	Pass    int
}

// captureReverseInfo runs at apply time, after computePlan and
// before applyPlan. Reads fstab + mount table again — cheap, and
// keeps the capture separated from the plan internals so a future
// refactor of computePlan doesn't break Reverse.
//
// touchedFstab / didMount / didUmount come from the plan; we
// preserve them as TouchedFstab + TouchedMount (didMount and
// didUmount fold into the same TouchedMount flag — Reverse
// doesn't need to distinguish direction, just whether the apply
// changed mount state at all).
func captureReverseInfo(dest string, touchedFstab, didMount, didUmount bool) *OsMountReverseInfo {
	info := &OsMountReverseInfo{
		Dest:         dest,
		TouchedFstab: touchedFstab,
		TouchedMount: didMount || didUmount,
	}

	if content, err := readFstab(); err == nil {
		lines := parseFstab(content)
		if idx := findByDest(lines, dest); idx >= 0 && lines[idx].entry != nil {
			e := lines[idx].entry
			info.PriorEntry = &OsMountSnapshotEntry{
				Src:     e.src,
				Dest:    e.dest,
				FSType:  e.fstype,
				Options: append([]string(nil), e.options...),
				Dump:    e.dump,
				Pass:    e.pass,
			}
		}
	}
	if mounts, err := readMounts(); err == nil {
		_, info.PriorMounted = mounts[dest]
	}
	return info
}

// Reverse implements actions.Reverser for os.mount (spec-28 P6 /
// reverse-capture v4).
//
// Reverse-step strategy by prior (entry, mounted) state:
//
//	entry present + mounted   → os.mount state=mounted with prior fields
//	entry present + unmounted → os.mount state=fstab_only with prior fields
//	no entry + unmounted      → os.mount state=absent
//	no entry + mounted        → refuse (can't express "manually
//	                            mounted but no fstab entry" through
//	                            os.mount; operator must use shell mount)
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Neither side touched → noop, same.
//   - Step / result missing / wrong type → defensive error.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.OsMount == nil {
		return nil, errors.New("os.mount Reverse: step has no OsMount payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("os.mount Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*OsMountReverseInfo)
	if !ok {
		return nil, fmt.Errorf("os.mount Reverse: ReverseData is %T, want *OsMountReverseInfo", r.ReverseData)
	}
	if !info.TouchedFstab && !info.TouchedMount {
		return nil, nil
	}

	hadEntry := info.PriorEntry != nil

	switch {
	case hadEntry && info.PriorMounted:
		return &config.Step{
			Name: "reverse: restore mount " + info.Dest,
			OsMount: &config.OsMount{
				Src:     info.PriorEntry.Src,
				Dest:    info.Dest,
				FSType:  info.PriorEntry.FSType,
				Options: append([]string(nil), info.PriorEntry.Options...),
				State:   "mounted",
			},
		}, nil

	case hadEntry && !info.PriorMounted:
		return &config.Step{
			Name: "reverse: restore fstab entry for " + info.Dest,
			OsMount: &config.OsMount{
				Src:     info.PriorEntry.Src,
				Dest:    info.Dest,
				FSType:  info.PriorEntry.FSType,
				Options: append([]string(nil), info.PriorEntry.Options...),
				State:   "fstab_only",
			},
		}, nil

	case !hadEntry && !info.PriorMounted:
		return &config.Step{
			Name: "reverse: remove fstab entry + unmount " + info.Dest,
			OsMount: &config.OsMount{
				Dest:  info.Dest,
				State: "absent",
			},
		}, nil

	default:
		// !hadEntry && PriorMounted — can't express via os.mount.
		return nil, fmt.Errorf("os.mount Reverse: prior state at %q was 'mounted with no fstab entry'; can't be expressed as an os.mount step (operator must restore manually)", info.Dest)
	}
}

var _ actions.Reverser = (*Handler)(nil)
