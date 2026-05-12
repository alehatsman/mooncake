// Package container_image implements the container.image action handler.
//
// container.image ensures a named image reference is present (or absent)
// in the local storage of a container runtime (podman/docker). Idempotency
// is anchored on whether the reference resolves locally.
//
//nolint:revive,staticcheck // package name container_image is the action key with snake_case for Go-import compatibility
package container_image

import (
	"context"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/containerruntime"
	"github.com/alehatsman/mooncake/internal/executor"
)

// State constants for container.image.state.
const (
	statePresent = "present"
	stateAbsent  = "absent"
)

// newRuntime is overridable in tests to inject a fake Runtime.
var newRuntime = func(preferred string) (containerruntime.Runtime, error) {
	return containerruntime.Detect(preferred)
}

// Handler implements actions.Handler for container.image.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata describes the container.image action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "container.image",
		Description:        "Manage container images (pull/remove) via podman or docker",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux", "darwin", "windows"},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

// Validate checks the action configuration.
func (h *Handler) Validate(step *config.Step) error {
	if step.ContainerImage == nil {
		return fmt.Errorf("container.image action requires configuration")
	}
	img := step.ContainerImage
	if img.Name == "" {
		return fmt.Errorf("container.image: name is required")
	}
	if img.State != "" && img.State != statePresent && img.State != stateAbsent {
		return fmt.Errorf("container.image: state must be %q or %q (got %q)", statePresent, stateAbsent, img.State)
	}
	return nil
}

// Run executes the action in plan or apply mode.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	img := step.ContainerImage
	renderedName, err := ctx.GetTemplate().Render(img.Name, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("render container.image.name: %w", err)
	}

	state := img.State
	if state == "" {
		state = statePresent
	}

	rt, err := newRuntime(img.Runtime)
	if err != nil {
		return nil, fmt.Errorf("container.image: %w", err)
	}

	bg := context.Background()
	exists, err := rt.ImageExists(bg, renderedName)
	if err != nil {
		return nil, fmt.Errorf("container.image: inspect %s: %w", renderedName, err)
	}

	plan := ctx.Mode() == actions.ModePlan
	res := executor.NewResult()
	res.Checkable = true

	switch state {
	case statePresent:
		if exists && !img.ForcePull {
			res.Reason = fmt.Sprintf("image %s already present", renderedName)
			return res, nil
		}
		if plan {
			res.WouldChange = true
			if exists {
				res.Reason = fmt.Sprintf("would re-pull image %s (force_pull)", renderedName)
			} else {
				res.Reason = fmt.Sprintf("would pull image %s", renderedName)
			}
			return res, nil
		}
		ctx.GetLogger().Infof("  Pulling image %s via %s", renderedName, rt.Name())
		if err := rt.ImagePull(bg, renderedName); err != nil {
			return nil, err
		}
		res.SetChanged(true)
		return res, nil

	case stateAbsent:
		if !exists {
			res.Reason = fmt.Sprintf("image %s already absent", renderedName)
			return res, nil
		}
		if plan {
			res.WouldChange = true
			res.Reason = fmt.Sprintf("would remove image %s", renderedName)
			return res, nil
		}
		ctx.GetLogger().Infof("  Removing image %s via %s", renderedName, rt.Name())
		if err := rt.ImageRemove(bg, renderedName); err != nil {
			return nil, err
		}
		res.SetChanged(true)
		return res, nil
	}

	return nil, fmt.Errorf("container.image: unreachable state %q", state)
}
