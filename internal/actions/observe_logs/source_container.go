package observe_logs

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// readContainer shells out to `docker logs --since <duration> <name>`
// or, if docker isn't on PATH, `podman logs --since <duration> <name>`.
// Both tools combine stdout+stderr by default when called without
// redirection; we capture combined output for the pattern matcher.
func readContainer(ctx context.Context, name string, since time.Duration, maxBytes int64, maxLines int) ([]string, bool, error) {
	bin := pickContainerBin()
	if bin == "" {
		return nil, false, fmt.Errorf("observe.logs: neither docker nor podman found on PATH")
	}
	sinceStr := fmt.Sprintf("%ds", int64(since.Seconds()))
	if sinceStr == "0s" {
		sinceStr = "60s"
	}
	args := []string{"logs", "--since", sinceStr, name}

	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // container name is operator-provided
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Container missing / runtime error — return the bin's stderr
		// in the error message for actionability, with whatever bytes
		// we did capture as the visible lines.
		return splitLines(string(out)), false, fmt.Errorf("%s logs: %w", bin, err)
	}
	truncated := int64(len(out)) > maxBytes
	if truncated {
		out = out[:maxBytes]
	}
	lines := splitLines(string(out))
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	return lines, truncated, nil
}

// pickContainerBin returns the first of docker/podman that resolves
// via exec.LookPath. Empty string means neither is available.
func pickContainerBin() string {
	for _, bin := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(bin); err == nil {
			return bin
		}
	}
	return ""
}
