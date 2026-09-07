// Package observe_file implements the observe.file action: read whether a
// filesystem path exists and, optionally, whether it contains a substring.
//
// Successor to wait.file. The polling that action existed for is now the
// shared `wait:` modifier, so this handler only has to answer the question
// once and let actions.ObserveWait do the looping.
package observe_file

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const actionName = "observe.file"

// FileObservation is the typed Value payload for observe.file.
type FileObservation struct {
	Path     string `json:"path"`               // Resolved absolute path that was probed
	Exists   bool   `json:"exists"`             // Path exists
	IsDir    bool   `json:"is_dir,omitempty"`   // Path is a directory
	Size     int64  `json:"size,omitempty"`     // Size in bytes (0 for directories)
	Mode     string `json:"mode,omitempty"`     // Permission bits, e.g. "0644"
	Matches  bool   `json:"matches,omitempty"`  // Contents contain the `contains:` substring
	Contains string `json:"contains,omitempty"` // The substring that was searched for
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Read whether a path exists (and optionally contains a substring)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	o := step.ObserveFile
	if o == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	if o.Path == "" {
		return fmt.Errorf("%s: path is required", actionName)
	}
	return actions.ValidateWait(actionName, o.Wait)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	o := step.ObserveFile

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("%s: invalid context type", actionName)
	}

	rendered, err := ctx.Template().Render(o.Path, ctx.Variables())
	if err != nil {
		return nil, &executor.RenderError{Field: actionName + ".path", Cause: err}
	}
	path, err := ec.Svc.PathUtil.ExpandPath(rendered, ec.CurrentDir, ctx.Variables())
	if err != nil {
		return nil, &executor.FileOperationError{Operation: "expand path", Path: rendered, Cause: err}
	}
	contains, err := ctx.Template().Render(o.Contains, ctx.Variables())
	if err != nil {
		return nil, &executor.RenderError{Field: actionName + ".contains", Cause: err}
	}

	result := executor.NewResult()
	result.Changed = false
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	if ctx.Mode() == actions.ModePlan {
		envelope := actions.PlanDeferred(FileObservation{Path: path, Contains: contains})
		result.PublishObservation(envelope, path)
		result.Checkable = true
		result.Reason = fmt.Sprintf("would observe file %s (deferred to apply)", path)
		return result, nil
	}

	probeOnce := func() actions.ObserveResult { return observeFile(path, contains) }

	envelope := probeOnce()
	if o.Wait != nil {
		var waitErr error
		// ObserveWait returns the last observation either way, so a timed-out
		// wait still publishes real data for a downstream `as:` capture.
		envelope, waitErr = actions.ObserveWait(ctx, actionName, path, o.Wait, probeOnce)
		if waitErr != nil {
			result.PublishObservation(envelope, path)
			return result, waitErr
		}
	}
	result.PublishObservation(envelope, path)

	ctx.Logger().Debugf("%s %s = found:%v", actionName, path, envelope.Found)
	return result, nil
}

// observeFile is the single attempt.
//
// A missing path is Found=false with no Error — the file genuinely not being
// there is the answer, not a probe failure. A stat that fails for another
// reason (permission denied) sets Error, which is what the envelope's Error
// field is for.
func observeFile(path, contains string) actions.ObserveResult {
	obs := FileObservation{Path: path, Contains: contains}

	info, err := os.Stat(path)
	if err != nil {
		res := actions.ObserveResult{Found: false, Value: obs, AsOf: time.Now()}
		if !os.IsNotExist(err) {
			res.Error = err.Error()
		}
		return res
	}

	obs.Exists = true
	obs.IsDir = info.IsDir()
	obs.Mode = fmt.Sprintf("%04o", info.Mode().Perm())
	if !obs.IsDir {
		obs.Size = info.Size()
	}

	if contains == "" {
		return actions.ObserveResult{Found: true, Value: obs, AsOf: time.Now()}
	}

	// A directory can't contain a substring — that's a false match, not an
	// error, so the caller's `wait: { until: found }` keeps polling.
	if obs.IsDir {
		return actions.ObserveResult{Found: false, Value: obs, AsOf: time.Now()}
	}

	data, err := os.ReadFile(path) //nolint:gosec // path comes from user config
	if err != nil {
		return actions.ObserveResult{Found: false, Value: obs, AsOf: time.Now(), Error: err.Error()}
	}
	obs.Matches = strings.Contains(string(data), contains)
	return actions.ObserveResult{Found: obs.Matches, Value: obs, AsOf: time.Now()}
}

// --- Read-only ABI specialization -------------------------------------------
//
// No Reverse method by design: a read has nothing to undo, and implementing a
// no-op would make `actions list` report REVERSE=yes. See specs/observe.md.

func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{Resources: 0, Bytes: 0, Reversible: false, Risk: 1}, nil
}

func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		Notes: []string{"read-only observation; no mutation"},
	}
}

func (h *Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	o := step.ObserveFile
	if o == nil {
		return actions.Diff{}, nil
	}
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceFile,
			Identifier: o.Path,
			Attributes: map[string]string{"observe_kind": "file"},
		},
		Operation: actions.OpNoop,
	}, nil
}
