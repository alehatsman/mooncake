package os_cron //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// OsCronReverseInfo is the per-step apply-time snapshot os.cron
// stashes on Result.ReverseData. Captures the canonical file path
// (/etc/cron.d/<name>) plus the pre-apply file content + existence.
//
// Cross-action reverse: os.cron renders a structured cron.d file
// from individual schedule/command/env fields, so a faithful
// "restore prior content" can't be expressed as an os.cron step
// in general (prior content may not match the renderer's exact
// output — e.g. external hand-edits or different field ordering).
// The Reverse returns a `file.write` step instead, which preserves
// the prior bytes verbatim. This mirrors the cross-action approach
// the file family's reverse helpers were designed for.
type OsCronReverseInfo struct {
	// Path is the canonical /etc/cron.d/<name> path the apply
	// targeted.
	Path string

	// PriorExisted reports whether the file existed pre-apply.
	// When false, Reverse emits a state=absent step (delete the
	// file we created). When true, Reverse emits a state=file
	// step with PriorContent.
	PriorExisted bool

	// PriorContent is the file's bytes pre-apply. Empty when
	// PriorExisted is false.
	PriorContent string
}

// Reverse implements actions.Reverser for os.cron (spec-28 P6 /
// reverse-capture v2).
//
// Cross-action reverse — returns a `file.write` step that
// reproduces the pre-apply file state byte-for-byte (or removes
// the file if it didn't exist before).
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing or wrong type → defensive error.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.OsCron == nil {
		return nil, errors.New("os.cron Reverse: step has no OsCron payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("os.cron Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*OsCronReverseInfo)
	if !ok {
		return nil, fmt.Errorf("os.cron Reverse: ReverseData is %T, want *OsCronReverseInfo", r.ReverseData)
	}
	if info.Path == "" {
		return nil, errors.New("os.cron Reverse: incomplete ReverseData (no Path)")
	}

	if !info.PriorExisted {
		// The apply created the file. Reverse deletes it.
		return &config.Step{
			Name: "reverse: remove " + info.Path,
			FileWrite: &config.File{
				Path:  info.Path,
				State: "absent",
			},
		}, nil
	}

	// The apply mutated or deleted an existing file. Reverse
	// restores the prior bytes verbatim.
	return &config.Step{
		Name: "reverse: restore " + info.Path,
		FileWrite: &config.File{
			Path:    info.Path,
			State:   "file",
			Content: info.PriorContent,
			Mode:    "0644",
			Force:   true,
		},
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
