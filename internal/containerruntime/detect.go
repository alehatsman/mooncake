package containerruntime

import (
	"fmt"
	"os/exec"
)

// Detect returns a Runtime for the preferred engine on this host.
// preferred names the engine to try first ("podman", "docker"); empty
// auto-selects (podman > docker). Returns an error when neither is
// available on $PATH.
func Detect(preferred string) (Runtime, error) {
	order := orderFor(preferred)
	for _, name := range order {
		if _, err := exec.LookPath(name); err == nil {
			return New(name)
		}
	}
	return nil, fmt.Errorf("no supported container runtime found in PATH (tried: %v)", order)
}

// New returns a driver by explicit name. Errors when the engine is
// unknown or the CLI is not on $PATH.
func New(name string) (Runtime, error) {
	switch name {
	case "podman":
		if _, err := exec.LookPath("podman"); err != nil {
			return nil, fmt.Errorf("podman not found in PATH: %w", err)
		}
		return &podman{bin: "podman"}, nil
	case "docker":
		if _, err := exec.LookPath("docker"); err != nil {
			return nil, fmt.Errorf("docker not found in PATH: %w", err)
		}
		return &podman{bin: "docker"}, nil
	default:
		return nil, fmt.Errorf("unsupported container runtime: %q", name)
	}
}

// orderFor returns the engine-name probe order, biased toward preferred
// when non-empty.
func orderFor(preferred string) []string {
	switch preferred {
	case "podman":
		return []string{"podman", "docker"}
	case "docker":
		return []string{"docker", "podman"}
	case "":
		return []string{"podman", "docker"}
	default:
		return []string{preferred}
	}
}
