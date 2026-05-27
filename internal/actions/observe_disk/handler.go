// Package observe_disk implements the observe.disk action: single-shot
// read of a filesystem path's space and inode usage (spec-60).
package observe_disk

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const actionName = "observe.disk"

// DiskObservation is the typed Value payload for observe.disk.
// Total / Used / Free are byte counts. Inode fields are 0 on
// filesystems that don't expose them.
type DiskObservation struct {
	Path        string `json:"path"`
	FSType      string `json:"fs_type,omitempty"`
	TotalBytes  int64  `json:"total_bytes"`
	UsedBytes   int64  `json:"used_bytes"`
	FreeBytes   int64  `json:"free_bytes"`
	InodesTotal int64  `json:"inodes_total,omitempty"`
	InodesUsed  int64  `json:"inodes_used,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty"`
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Single-shot read of filesystem space / inode usage for a path",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	if step.ObserveDisk == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	o := step.ObserveDisk

	result := executor.NewResult()
	result.Changed = false
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	path := o.Path
	if path == "" {
		path = "/"
	}
	rendered, err := ctx.GetTemplate().Render(path, ctx.GetVariables())
	if err != nil {
		return nil, &executor.RenderError{Field: actionName + ".path", Cause: err}
	}
	abs, err := filepath.Abs(rendered)
	if err != nil {
		abs = rendered
	}

	if ctx.Mode() == actions.ModePlan {
		env := actions.PlanDeferred(DiskObservation{Path: abs})
		result.PublishObservation(env, abs)
		result.Checkable = true
		result.Reason = fmt.Sprintf("would observe disk %s (deferred to apply)", abs)
		return result, nil
	}

	obs, err := statPath(abs)
	env := actions.ObserveResult{
		Found: err == nil,
		Value: obs,
		AsOf:  time.Now(),
	}
	if err != nil {
		env.Error = err.Error()
	}
	result.PublishObservation(env, abs)
	return result, nil
}

// --- Spec-22 ABI no-mutation specialization ---------------------------------

func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{Resources: 0, Bytes: 0, Reversible: true, Risk: 1}, nil
}

func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		Notes: []string{"read-only observation; uses statfs(2)"},
	}
}

func (h *Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	o := step.ObserveDisk
	if o == nil {
		return actions.Diff{}, nil
	}
	path := o.Path
	if path == "" {
		path = "/"
	}
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: "disk:" + path,
			Attributes: map[string]string{"observe_kind": "disk"},
		},
		Operation: actions.OpNoop,
	}, nil
}

func (h *Handler) Reverse(_ actions.Context, _ *config.Step, _ actions.Result) (*config.Step, error) {
	return nil, nil
}
