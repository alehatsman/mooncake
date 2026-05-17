// Package container_run implements the container action handler.
//
// The container action drives container lifecycle (running/stopped/absent)
// over a runtime-agnostic facade (podman/docker). Idempotency is keyed by
// container name: when the named container is already in the desired
// state, the action is a no-op.
//
//nolint:revive,staticcheck // package name uses snake_case for Go-import compat; YAML key is "container"
package container_run

import (
	"context"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/containerruntime"
	"github.com/alehatsman/mooncake/internal/executor"
)

// State constants for container.state.
const (
	stateRunning = "running"
	stateStopped = "stopped"
	stateAbsent  = "absent"
)

// newRuntime is overridable in tests to inject a fake Runtime.
var newRuntime = containerruntime.Detect

// Handler implements actions.Handler for the container action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata describes the container action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "container",
		Description:        "Manage container lifecycle (running/stopped/absent) via podman or docker",
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
	if step.Container == nil {
		return fmt.Errorf("container action requires configuration")
	}
	c := step.Container
	if c.Name == "" {
		return fmt.Errorf("container: name is required")
	}
	state := c.State
	if state == "" {
		state = stateRunning
	}
	switch state {
	case stateRunning, stateStopped:
		if c.Image == "" {
			return fmt.Errorf("container: image is required for state=%s", state)
		}
	case stateAbsent:
	default:
		return fmt.Errorf("container: state must be one of running|stopped|absent (got %q)", c.State)
	}
	return nil
}

// Run executes the action in plan or apply mode.
// RunRaw signals spec-69 RawRunner participation so user-declared
// `retry:` actually retries this idempotent action via the
// centralized executor loop instead of being silently no-op'd.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	c := step.Container

	name, err := ctx.GetTemplate().Render(c.Name, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("render container.name: %w", err)
	}
	image, err := ctx.GetTemplate().Render(c.Image, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("render container.image: %w", err)
	}

	state := c.State
	if state == "" {
		state = stateRunning
	}

	rt, err := newRuntime(c.Runtime)
	if err != nil {
		return nil, fmt.Errorf("container: %w", err)
	}

	bg := context.Background()
	cur, err := rt.ContainerInspect(bg, name)
	if err != nil {
		return nil, fmt.Errorf("container: inspect %s: %w", name, err)
	}

	plan := ctx.Mode() == actions.ModePlan
	res := executor.NewResult()
	res.Checkable = true

	switch state {
	case stateAbsent:
		if !cur.Exists {
			res.Reason = fmt.Sprintf("container %s already absent", name)
			return res, nil
		}
		if plan {
			res.WouldChange = true
			res.Reason = fmt.Sprintf("would remove container %s", name)
			return res, nil
		}
		ctx.GetLogger().Infof("  Removing container %s via %s", name, rt.Name())
		if err := rt.ContainerRemove(bg, name, true); err != nil {
			return nil, err
		}
		res.SetChanged(true)
		return res, nil

	case stateRunning, stateStopped:
		// Image-drift detection by string-match would be unreliable here:
		// engines resolve short refs ("alpine:3.20") to fully-qualified
		// names ("docker.io/library/alpine:3.20") on inspect, producing
		// false-positive drift on every run. The MVP exposes explicit
		// `recreate: true` for callers who need teardown-and-rebuild.
		recreate := c.Recreate
		wantRunning := state == stateRunning

		// Already in desired state and no recreate needed: no-op.
		if cur.Exists && !recreate && cur.Running == wantRunning {
			res.Reason = fmt.Sprintf("container %s already %s", name, state)
			return res, nil
		}
		// MT-63: a missing container is already "stopped enough" —
		// don't pull/create/start/stop on the user's behalf.
		if !cur.Exists && !wantRunning {
			res.Reason = fmt.Sprintf("container %s does not exist; nothing to stop", name)
			return res, nil
		}

		if plan {
			res.WouldChange = true
			res.Reason = planReason(cur, name, image, state, recreate)
			return res, nil
		}

		// Recreate: drop the existing container before creating.
		if cur.Exists && recreate {
			ctx.GetLogger().Infof("  Recreating container %s via %s", name, rt.Name())
			if err := rt.ContainerRemove(bg, name, true); err != nil {
				return nil, err
			}
			cur = containerruntime.ContainerState{Exists: false}
		}

		spec := containerruntime.ContainerSpec{
			Name:    name,
			Image:   image,
			Command: c.Command,
			Env:     c.Env,
			Ports:   c.Ports,
			Volumes: c.Volumes,
			Network: c.Network,
			Restart: c.Restart,
			Detach:  true,
			Extra:   c.Extra,
		}

		// !cur.Exists && !wantRunning is already handled by the MT-63
		// early-return above. The only remaining missing-container
		// case is `state: running`: create+start.
		if !cur.Exists {
			ctx.GetLogger().Infof("  Creating container %s (image=%s) via %s", name, image, rt.Name())
			if err := rt.ContainerCreate(bg, spec); err != nil {
				return nil, err
			}
			res.SetChanged(true)
			return res, nil
		}

		// Container exists; bring it to the desired running flag.
		if wantRunning && !cur.Running {
			ctx.GetLogger().Infof("  Starting container %s via %s", name, rt.Name())
			if err := rt.ContainerStart(bg, name); err != nil {
				return nil, err
			}
			res.SetChanged(true)
			return res, nil
		}
		if !wantRunning && cur.Running {
			ctx.GetLogger().Infof("  Stopping container %s via %s", name, rt.Name())
			if err := rt.ContainerStop(bg, name); err != nil {
				return nil, err
			}
			res.SetChanged(true)
			return res, nil
		}
		// Should be unreachable given the earlier "already in desired
		// state" guard, but stay defensive.
		res.Reason = fmt.Sprintf("container %s already %s", name, state)
		return res, nil
	}

	return nil, fmt.Errorf("container: unreachable state %q", state)
}

// planReason composes the human-readable diff description for plan mode.
func planReason(cur containerruntime.ContainerState, name, image, state string, recreate bool) string {
	switch {
	case !cur.Exists && state == stateRunning:
		return fmt.Sprintf("would create+start container %s (image=%s)", name, image)
	case !cur.Exists && state == stateStopped:
		// MT-63: a missing container is already "stopped" enough — we
		// don't pre-create it on the user's behalf.
		return fmt.Sprintf("container %s does not exist; nothing to stop", name)
	case recreate:
		return fmt.Sprintf("would recreate container %s (image=%s)", name, image)
	case state == stateRunning:
		return fmt.Sprintf("would start container %s", name)
	case state == stateStopped:
		return fmt.Sprintf("would stop container %s", name)
	}
	return fmt.Sprintf("would change container %s", name)
}
