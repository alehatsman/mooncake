package observe_logs

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// readJournal shells out to `journalctl -u <unit> --since <when>`
// (Linux only). The since duration is converted to a journalctl-
// friendly format: `--since "30 seconds ago"`.
func readJournal(ctx context.Context, unit string, since time.Duration, maxBytes int64, maxLines int) ([]string, bool, error) {
	if runtime.GOOS != "linux" {
		return nil, false, fmt.Errorf("observe.logs: journal_unit is only supported on Linux (got %s)", runtime.GOOS)
	}
	if _, err := exec.LookPath("journalctl"); err != nil {
		return nil, false, fmt.Errorf("observe.logs: journalctl not found: %w", err)
	}

	// journalctl's --since accepts either an absolute timestamp or a
	// human duration. We pass a duration string ("30s ago") since the
	// underlying since is a relative window.
	sinceStr := durationAgo(since)
	args := []string{
		"-u", unit,
		"--no-pager",
		"--output=cat", // strip the "Apr 12 12:00:00 host unit:" prefix
		"--since", sinceStr,
	}
	if maxLines > 0 {
		args = append(args, "--lines", fmt.Sprintf("%d", maxLines))
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...) //nolint:gosec // unit name is operator-provided
	out, err := cmd.Output()
	if err != nil {
		return nil, false, fmt.Errorf("journalctl: %w", err)
	}
	// Apply byte cap to the captured output.
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

// durationAgo turns a Duration into journalctl --since "X ago" format.
// Falls back to "1 minute ago" for very short or negative durations.
func durationAgo(d time.Duration) string {
	if d < time.Second {
		return "1 minute ago"
	}
	secs := int64(d / time.Second)
	switch {
	case secs >= 60:
		return fmt.Sprintf("%d minutes ago", secs/60)
	default:
		return fmt.Sprintf("%d seconds ago", secs)
	}
}

// splitLines splits and drops a single trailing empty line caused by
// the journalctl output ending in '\n'.
func splitLines(s string) []string {
	parts := strings.Split(s, "\n")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	return parts
}
