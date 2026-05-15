package os_user //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// OsUserReverseInfo is the per-step apply-time snapshot os.user
// stashes on Result.ReverseData. Captures the user's pre-apply
// getent passwd entry plus supplementary groups (id -nG).
//
// Best-effort restore: useradd-on-reverse can re-create the basic
// identity (uid, gid, shell, home, comment, supplementary groups)
// but cannot restore: the password hash (lost when userdel runs),
// the home dir's pre-apply file contents (delete-with-removehome
// erases them), or the precise pam_tally / login.defs counters.
// Documented in Reverse's "edge cases" section so operators know
// what to expect.
type OsUserReverseInfo struct {
	// Name is the account name (identity for idempotency).
	Name string

	// AppliedState is the step's State at apply time:
	// "present" or "absent".
	AppliedState string

	// PriorExisted reports whether the user existed pre-apply.
	PriorExisted bool

	// Prior captures the full passwd/group snapshot — only
	// populated when PriorExisted is true.
	Prior OsUserSnapshotState
}

// OsUserSnapshotState mirrors the package-private userState
// struct, exported so Reverse can build a reversal step from it.
// Field names match config.OsUser so the reverse-step construction
// is a one-to-one mapping.
type OsUserSnapshotState struct {
	UID     int
	GID     int
	Shell   string
	Home    string
	Comment string
	Groups  []string // supplementary, sorted
}

// snapshotFromState lifts the package-private userState to the
// exported OsUserSnapshotState shape for ReverseData. Returns the
// zero value when current is nil or missing.
func snapshotFromState(current *userState) OsUserSnapshotState {
	if current == nil || !current.exists {
		return OsUserSnapshotState{}
	}
	s := OsUserSnapshotState{
		UID:     current.uid,
		GID:     current.gid,
		Shell:   current.shell,
		Home:    current.home,
		Comment: current.comment,
	}
	if len(current.groups) > 0 {
		s.Groups = append([]string(nil), current.groups...)
	}
	return s
}

// Reverse implements actions.Reverser for os.user (spec-27 P4 /
// reverse-capture v3).
//
// Strategy:
//   - PriorExisted=false → reverse is state=absent (the apply
//     created the user; rollback removes it).
//   - PriorExisted=true → reverse is state=present with the
//     captured fields (best-effort recreate). When AppliedState
//     was "present" this restores attribute drift; when
//     AppliedState was "absent" this attempts to re-create the
//     user (with the caveat that the password hash is gone).
//
// Best-effort caveats — documented for the operator:
//   - Password hash: lost when userdel ran. Reverse re-creates
//     the account but the user can't log in until a new
//     password is set.
//   - Home directory contents: if the original step had
//     remove_home=true, the prior $HOME files are gone. The
//     reverse step re-creates the home dir as empty.
//   - File ownership cascades: files owned by the prior uid on
//     other filesystems may now be orphaned. Reverse restores
//     the uid so the ownership rejoins.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing / wrong type → defensive error.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.OsUser == nil {
		return nil, errors.New("os.user Reverse: step has no OsUser payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("os.user Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*OsUserReverseInfo)
	if !ok {
		return nil, fmt.Errorf("os.user Reverse: ReverseData is %T, want *OsUserReverseInfo", r.ReverseData)
	}
	if info.Name == "" {
		return nil, errors.New("os.user Reverse: incomplete ReverseData (no Name)")
	}

	if !info.PriorExisted {
		// Apply created the user — reverse removes it.
		return &config.Step{
			Name: "reverse: remove user " + info.Name,
			OsUser: &config.OsUser{
				Name:  info.Name,
				State: "absent",
			},
		}, nil
	}

	// Apply modified or removed an existing user — reverse
	// restores the captured identity.
	uid := info.Prior.UID
	gid := info.Prior.GID
	rev := &config.OsUser{
		Name:    info.Name,
		State:   "present",
		UID:     &uid,
		GID:     &gid,
		Shell:   info.Prior.Shell,
		Home:    info.Prior.Home,
		Comment: info.Prior.Comment,
	}
	if len(info.Prior.Groups) > 0 {
		rev.Groups = append([]string(nil), info.Prior.Groups...)
		// AppendGroups defaults to true at parse time, which would
		// keep existing memberships. We want exact restore — set
		// false so the supplementary list matches what we captured.
		f := false
		rev.AppendGroups = &f
	}
	return &config.Step{
		Name:   "reverse: restore user " + info.Name,
		OsUser: rev,
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
