package wait

import (
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Plan mode reports what `wait` would
// wait for. For file_exists / file_absent it checks the current
// filesystem state and reports already-ok if the condition is already
// met. For other conditions (http, port, git_clean, command) it
// surfaces a description but doesn't probe — those checks would
// themselves be side-effecty or expensive in plan mode.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() != actions.ModePlan {
		return h.Execute(ctx, step)
	}

	w := step.Wait
	r := executor.NewResult()
	r.Checkable = true

	switch w.Condition {
	case conditionFileExists:
		if w.Path == nil {
			r.Reason = "would wait for file to exist (path not set)"
			r.WouldChange = true
			return r, nil
		}
		if _, statErr := os.Stat(*w.Path); statErr == nil {
			r.Reason = fmt.Sprintf("file already exists: %s", *w.Path)
			return r, nil
		}
		r.WouldChange = true
		r.Reason = fmt.Sprintf("would wait for file to exist: %s", *w.Path)
		return r, nil

	case conditionFileAbsent:
		if w.Path == nil {
			r.Reason = "would wait for file to be absent (path not set)"
			r.WouldChange = true
			return r, nil
		}
		if _, statErr := os.Stat(*w.Path); os.IsNotExist(statErr) {
			r.Reason = fmt.Sprintf("file already absent: %s", *w.Path)
			return r, nil
		}
		r.WouldChange = true
		r.Reason = fmt.Sprintf("would wait for file to be absent: %s", *w.Path)
		return r, nil

	case conditionGitClean:
		r.WouldChange = true
		r.Reason = "would wait for git working tree to be clean"
		return r, nil

	case conditionCommand:
		if w.Cmd != nil {
			r.Reason = fmt.Sprintf("would wait for command success: %s", *w.Cmd)
		} else {
			r.Reason = "would wait for command success"
		}
		r.WouldChange = true
		return r, nil

	case conditionHTTP:
		status := 200
		if w.Status != nil {
			status = *w.Status
		}
		if w.URL != nil {
			r.Reason = fmt.Sprintf("would wait for HTTP %d at %s", status, *w.URL)
		} else {
			r.Reason = fmt.Sprintf("would wait for HTTP %d", status)
		}
		r.WouldChange = true
		return r, nil

	case conditionPort:
		r.WouldChange = true
		r.Reason = "would wait for TCP port"
		return r, nil

	default:
		r.WouldChange = true
		r.Reason = fmt.Sprintf("would wait for condition: %s", w.Condition)
		return r, nil
	}
}
