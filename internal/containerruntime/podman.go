package containerruntime

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// podman drives the podman or docker CLI. Both engines share enough
// surface (image inspect, container inspect, run, start, stop, rm) for
// a single driver to cover them; the binary name is the only difference.
type podman struct {
	bin string
}

func (p *podman) Name() string { return p.bin }

// inspectFormat encodes the three fields we read off a container in a
// single line; "|" is safe because none of the captured values contain
// it (image refs, IDs, and engine status strings are restricted).
const inspectFormat = "{{.State.Status}}|{{.ImageName}}|{{.Id}}"

func (p *podman) ImageExists(ctx context.Context, ref string) (bool, error) {
	cmd := exec.CommandContext(ctx, p.bin, "image", "inspect", "--format", "{{.Id}}", ref) // #nosec G204 -- bin is fixed, ref is config input
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	low := strings.ToLower(string(out))
	if strings.Contains(low, "no such image") || strings.Contains(low, "not found") || strings.Contains(low, "does not exist") || strings.Contains(low, "image not known") {
		return false, nil
	}
	return false, fmt.Errorf("%s image inspect %s failed: %w (output: %s)", p.bin, ref, err, strings.TrimSpace(string(out)))
}

func (p *podman) ImagePull(ctx context.Context, ref string) error {
	cmd := exec.CommandContext(ctx, p.bin, "pull", ref) // #nosec G204
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s pull %s failed: %w (output: %s)", p.bin, ref, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (p *podman) ImageRemove(ctx context.Context, ref string) error {
	cmd := exec.CommandContext(ctx, p.bin, "rmi", ref) // #nosec G204
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	low := strings.ToLower(string(out))
	if strings.Contains(low, "no such image") || strings.Contains(low, "not found") || strings.Contains(low, "does not exist") || strings.Contains(low, "image not known") {
		return nil
	}
	return fmt.Errorf("%s rmi %s failed: %w (output: %s)", p.bin, ref, err, strings.TrimSpace(string(out)))
}

func (p *podman) ContainerInspect(ctx context.Context, name string) (ContainerState, error) {
	cmd := exec.CommandContext(ctx, p.bin, "container", "inspect", "--format", inspectFormat, name) // #nosec G204
	out, err := cmd.CombinedOutput()
	if err != nil {
		low := strings.ToLower(string(out))
		if strings.Contains(low, "no such container") || strings.Contains(low, "not found") || strings.Contains(low, "does not exist") {
			return ContainerState{Exists: false}, nil
		}
		return ContainerState{}, fmt.Errorf("%s container inspect %s failed: %w (output: %s)", p.bin, name, err, strings.TrimSpace(string(out)))
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 3)
	state := ContainerState{Exists: true}
	if len(parts) > 0 {
		state.Running = parts[0] == "running"
	}
	if len(parts) > 1 {
		state.Image = parts[1]
	}
	if len(parts) > 2 {
		state.ID = parts[2]
	}
	return state, nil
}

func (p *podman) ContainerCreate(ctx context.Context, spec ContainerSpec) error {
	args := []string{"run"}
	if spec.Detach {
		args = append(args, "-d")
	}
	args = append(args, "--name", spec.Name)
	for k, v := range spec.Env {
		args = append(args, "-e", k+"="+v)
	}
	for _, port := range spec.Ports {
		args = append(args, "-p", port)
	}
	for _, vol := range spec.Volumes {
		args = append(args, "-v", vol)
	}
	if spec.Network != "" {
		args = append(args, "--network", spec.Network)
	}
	if spec.Restart != "" {
		args = append(args, "--restart", spec.Restart)
	}
	args = append(args, spec.Extra...)
	args = append(args, spec.Image)
	args = append(args, spec.Command...)

	cmd := exec.CommandContext(ctx, p.bin, args...) // #nosec G204
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s run %s failed: %w (output: %s)", p.bin, spec.Name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (p *podman) ContainerStart(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, p.bin, "start", name) // #nosec G204
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s start %s failed: %w (output: %s)", p.bin, name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (p *podman) ContainerStop(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, p.bin, "stop", name) // #nosec G204
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s stop %s failed: %w (output: %s)", p.bin, name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (p *podman) ContainerRemove(ctx context.Context, name string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)
	cmd := exec.CommandContext(ctx, p.bin, args...) // #nosec G204
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	low := strings.ToLower(string(out))
	if strings.Contains(low, "no such container") || strings.Contains(low, "not found") {
		return nil
	}
	return fmt.Errorf("%s rm %s failed: %w (output: %s)", p.bin, name, err, strings.TrimSpace(string(out)))
}
