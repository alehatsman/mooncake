package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const minFreeBytes = 100 * 1024 * 1024 // 100 MiB — see spec-41

// checkHomeDir ensures ~/.mooncake/ either doesn't exist yet (fine, it'll
// be created on first run) or exists and is a writable directory. The
// non-existent case is `info`, not `error`, per spec AC9.
type checkHomeDir struct{}

func (checkHomeDir) Section() string { return "state" }
func (checkHomeDir) Name() string    { return "home-dir" }
func (checkHomeDir) Run(ctx Context) Result {
	r := Result{Section: "state", Name: "home-dir"}
	info, err := os.Stat(ctx.HomeDir)
	if os.IsNotExist(err) {
		r.Status = StatusInfo
		r.Message = fmt.Sprintf("%s does not exist (will be created on first run)", ctx.HomeDir)
		r.Fix = "run `mooncake apply` to auto-create, or `mooncake init` to scaffold a project"
		return r
	}
	if err != nil {
		r.Status = StatusError
		r.Message = err.Error()
		r.Fix = "check filesystem and permissions for " + ctx.HomeDir
		return r
	}
	if !info.IsDir() {
		r.Status = StatusError
		r.Message = ctx.HomeDir + " exists but is not a directory"
		r.Fix = "rm " + ctx.HomeDir + " (then re-run mooncake to recreate it)"
		return r
	}
	if !isWritable(ctx.HomeDir) {
		r.Status = StatusError
		r.Message = ctx.HomeDir + " is not writable"
		r.Fix = "chmod u+w " + ctx.HomeDir
		return r
	}
	r.Status = StatusOK
	r.Message = ctx.HomeDir + " exists and is writable"
	return r
}

// checkRunsLog reports the size and recency of ~/.mooncake/runs.jsonl. A
// missing file is info — the first apply creates it.
type checkRunsLog struct{}

func (checkRunsLog) Section() string { return "state" }
func (checkRunsLog) Name() string    { return "runs-log" }
func (checkRunsLog) Run(ctx Context) Result {
	r := Result{Section: "state", Name: "runs-log"}
	path := filepath.Join(ctx.HomeDir, "runs.jsonl")
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		r.Status = StatusInfo
		r.Message = "no run history yet"
		r.Fix = "run `mooncake apply` once and the log will be created"
		return r
	}
	if err != nil {
		r.Status = StatusWarning
		r.Message = "cannot stat runs.jsonl: " + err.Error()
		return r
	}
	age := time.Since(info.ModTime())
	r.Status = StatusOK
	r.Message = fmt.Sprintf("runs.jsonl: %d bytes, last write %s", info.Size(), formatAgeAgo(age))
	return r
}

// formatAgeAgo wraps formatAge with the "ago" suffix when appropriate so
// the headline reads naturally ("just now" stays bare; "12m" becomes
// "12m ago").
func formatAgeAgo(d time.Duration) string {
	s := formatAge(d)
	if s == "just now" {
		return s
	}
	return s + " ago"
}

// checkDiskSpace warns when ≤100 MiB free in $HOME so mooncake doesn't
// silently fail mid-apply.
type checkDiskSpace struct{}

func (checkDiskSpace) Section() string { return "state" }
func (checkDiskSpace) Name() string    { return "disk-space" }
func (checkDiskSpace) Run(ctx Context) Result {
	r := Result{Section: "state", Name: "disk-space"}
	// MT-71: ctx.HomeDir is `~/.mooncake/` which may not exist on a
	// fresh install. statfs returns ENOENT in that case, which the
	// old code surfaced as "unsupported on this OS" — a false
	// negative on every Linux machine where mooncake hadn't run
	// yet. Walk up to the nearest existing ancestor before probing.
	probeDir, err := existingAncestor(ctx.HomeDir)
	if err != nil {
		r.Status = StatusInfo
		r.Message = "disk-space probe unsupported on this OS"
		return r
	}
	free, err := diskFree(probeDir)
	if err != nil {
		r.Status = StatusInfo
		r.Message = "disk-space probe unsupported on this OS"
		return r
	}
	r.Detail = fmt.Sprintf("%.1f GiB free in %s", float64(free)/1024/1024/1024, probeDir)
	if free < minFreeBytes {
		r.Status = StatusWarning
		r.Message = fmt.Sprintf("less than 100 MiB free in %s", probeDir)
		r.Fix = "free up space; mooncake stores plan artifacts and run history under ~/.mooncake/"
		return r
	}
	r.Status = StatusOK
	r.Message = r.Detail
	r.Detail = ""
	return r
}

// existingAncestor walks dir's parent chain until os.Stat resolves a
// path. Used by checkDiskSpace so the statfs probe lands on an
// existing directory even when ~/.mooncake hasn't been created yet
// (MT-71).
func existingAncestor(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("empty path")
	}
	for cur := dir; ; {
		if _, err := os.Stat(cur); err == nil {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", errors.New("no existing ancestor")
		}
		cur = parent
	}
}

// formatAge is intentionally lossy — doctor doesn't need second-level
// precision for "when did the runs log last get touched".
func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	}
}

// isWritable performs the cheapest portable writability probe: create a
// temp file inside dir, delete on success. Avoids the fragile mode-bit
// inspection that doesn't account for ACLs.
func isWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".doctor-write-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
