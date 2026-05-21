// Package containerruntime is the runtime-agnostic facade for container
// engines (Podman, Docker, ...). Action handlers depend on the Runtime
// interface; concrete drivers wrap the engine's CLI.
package containerruntime

import "context"

// Runtime is the minimal surface every container engine driver implements.
// All operations are name- or ref-keyed so callers reason in idempotent
// terms ("ensure image present", "ensure container running") without
// holding engine-specific identifiers.
type Runtime interface {
	// Name returns the runtime identifier ("podman", "docker").
	Name() string

	// WithEnv returns a Runtime whose engine subprocesses run with the
	// given env merged onto os.Environ. Used to plumb per-action knobs
	// like DOCKER_CONFIG / DOCKER_HOST without leaking them into the
	// whole mooncake process. The receiver is not mutated; callers
	// should use the returned value.
	WithEnv(env map[string]string) Runtime

	// ImageExists reports whether ref is present in local storage.
	ImageExists(ctx context.Context, ref string) (bool, error)

	// ImagePull fetches ref from the configured registry. Re-pulling an
	// image that already exists is the caller's responsibility to gate.
	ImagePull(ctx context.Context, ref string) error

	// ImageRemove deletes ref from local storage. No-op if absent.
	ImageRemove(ctx context.Context, ref string) error

	// ContainerInspect returns the current state of name. Exists=false
	// indicates the container is absent.
	ContainerInspect(ctx context.Context, name string) (ContainerState, error)

	// ContainerCreate creates and starts a new container per spec. The
	// caller must guarantee no container with spec.Name exists.
	ContainerCreate(ctx context.Context, spec ContainerSpec) error

	// ContainerStart starts an existing stopped container.
	ContainerStart(ctx context.Context, name string) error

	// ContainerStop stops a running container with the default timeout.
	ContainerStop(ctx context.Context, name string) error

	// ContainerRemove deletes a container. If force is true, running
	// containers are killed first; otherwise the call fails if running.
	ContainerRemove(ctx context.Context, name string, force bool) error
}

// ContainerState is the inspected state of a container.
type ContainerState struct {
	Exists  bool
	Running bool
	Image   string // resolved image (ref or ID) the container was created from
	ID      string // engine-assigned container ID; empty when Exists is false
}

// ContainerSpec is the create-time configuration for a new container.
// Mirrors the subset of fields the MVP exposes; extend as new actions
// grow.
type ContainerSpec struct {
	Name    string
	Image   string
	Command []string          // overrides image's CMD if non-empty
	Env     map[string]string // VAR=value pairs
	Ports   []string          // "host:container[/proto]" entries (e.g. "8080:80")
	Volumes []string          // "host:container[:opts]" entries
	Network string            // engine network name; "" uses default
	Restart string            // no|on-failure|always|unless-stopped; "" leaves engine default
	Detach  bool              // run detached (background); MVP always true
	Extra   []string          // engine-specific extra args; appended as-is
}
