package os_ssh_key //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// OsSSHKeyReverseInfo is the per-step apply-time snapshot
// os.ssh_key stashes on Result.ReverseData. Captures the
// authorized_keys file path, target user, prior existence, and
// the raw prior content (bytes — byte-faithful restore via
// file.write).
//
// Cross-action reverse: os.ssh_key edits authorized_keys
// line-by-line based on parsed-key identity. Restoring arbitrary
// prior content (which may have hand-edits, exotic options, or
// non-standard ordering) can't be expressed faithfully as another
// os.ssh_key step. The reverse returns a `file.write` step
// instead, which preserves prior bytes verbatim with the right
// mode and ownership.
type OsSSHKeyReverseInfo struct {
	// Path is the resolved authorized_keys file path.
	Path string

	// User is the target account name; used as the Owner on the
	// reverse file.write step so ownership is restored along with
	// content.
	User string

	// PriorExisted reports whether the file existed pre-apply.
	// When false, Reverse emits state=absent (remove the file we
	// created). When true, Reverse emits state=file with
	// PriorContent.
	PriorExisted bool

	// PriorContent is the verbatim file bytes pre-apply. Empty
	// when PriorExisted is false.
	PriorContent string
}

// priorContentBytes reconstructs the original file content from
// readAuthorizedKeys output. The reader strips the trailing
// newline + splits on '\n'; we rejoin with '\n' and re-add the
// trailing newline so the round-tripped bytes match the file.
func priorContentBytes(lines []string, existed bool) string {
	if !existed {
		return ""
	}
	if len(lines) == 0 {
		// Empty file existed pre-apply — that's a valid state.
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// Reverse implements actions.Reverser for os.ssh_key (spec-27 P4 /
// reverse-capture v4).
//
// Cross-action reverse — returns a `file.write` step that
// reproduces the pre-apply file state byte-for-byte (or removes
// the file if it didn't exist before). Mode and Owner are pinned
// to the same values os.ssh_key would normally use (0600, owned
// by User).
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing / wrong type → defensive error.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.OsSSHKey == nil {
		return nil, errors.New("os.ssh_key Reverse: step has no OsSSHKey payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("os.ssh_key Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*OsSSHKeyReverseInfo)
	if !ok {
		return nil, fmt.Errorf("os.ssh_key Reverse: ReverseData is %T, want *OsSSHKeyReverseInfo", r.ReverseData)
	}
	if info.Path == "" {
		return nil, errors.New("os.ssh_key Reverse: incomplete ReverseData (no Path)")
	}

	if !info.PriorExisted {
		return &config.Step{
			Name: "reverse: remove " + info.Path,
			FileWrite: &config.File{
				Path:  info.Path,
				State: "absent",
			},
		}, nil
	}

	return &config.Step{
		Name: "reverse: restore " + info.Path,
		FileWrite: &config.File{
			Path:    info.Path,
			State:   "file",
			Content: info.PriorContent,
			Mode:    "0600",
			Owner:   info.User,
			Group:   info.User,
			Force:   true,
		},
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
