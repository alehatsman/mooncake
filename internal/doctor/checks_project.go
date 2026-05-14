package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
)

// projectConfigPath is the resolved config path discovered by
// checkProjectConfig. Subsequent checks (validate, summary, lockfile)
// reuse it instead of re-discovering, keeping reports consistent.
//
// We avoid a package-level cache for testability; instead the checks read
// from a context-scoped helper. Implementation note: doctor's catalogue
// runs sequentially in registered order so checkProjectConfig precedes
// the others — `findProjectConfig` is therefore safe to call from each
// check directly. The slight redundancy is preferable to global state.

// findProjectConfig encapsulates the "discover or fail" logic so the four
// project checks behave consistently. Returns "" when skipping.
func findProjectConfig(ctx Context) string {
	if ctx.SkipProject {
		return ""
	}
	p, err := config.DiscoverConfig(ctx.Cwd)
	if err != nil {
		return ""
	}
	return p
}

// checkProjectConfig reports whether a project config exists in cwd.
// "no project config" is `info`, not warning — the user might be running
// doctor from anywhere.
type checkProjectConfig struct{}

func (checkProjectConfig) Section() string { return "project" }
func (checkProjectConfig) Name() string    { return "config" }
func (checkProjectConfig) Run(ctx Context) Result {
	r := Result{Section: "project", Name: "config"}
	if ctx.SkipProject {
		r.Status = StatusInfo
		r.Message = "skipped (--skip-project)"
		return r
	}
	p := findProjectConfig(ctx)
	if p == "" {
		r.Status = StatusInfo
		r.Message = "no project config in " + ctx.Cwd
		r.Fix = "run `mooncake init` to scaffold one"
		return r
	}
	r.Status = StatusOK
	r.Message = "found " + filepath.Base(p)
	r.Detail = p
	return r
}

// checkProjectValidate runs the config validator and reports the FIRST
// diagnostic only (doctor stays scannable). Pointers users at `mooncake
// validate` for the full output.
type checkProjectValidate struct{}

func (checkProjectValidate) Section() string { return "project" }
func (checkProjectValidate) Name() string    { return "validate" }
func (checkProjectValidate) Run(ctx Context) Result {
	r := Result{Section: "project", Name: "validate"}
	p := findProjectConfig(ctx)
	if p == "" {
		r.Status = StatusInfo
		r.Message = "skipped (no project config)"
		return r
	}
	_, diags, err := config.ReadConfigWithValidation(p)
	if err != nil {
		r.Status = StatusError
		r.Message = "cannot read config: " + err.Error()
		r.Fix = fmt.Sprintf("run `mooncake validate -c %s` for full diagnostics", p)
		return r
	}
	if config.HasErrors(diags) {
		r.Status = StatusError
		r.Message = "config has validation errors"
		if len(diags) > 0 {
			r.Detail = diags[0].Message
		}
		r.Fix = fmt.Sprintf("run `mooncake validate -c %s` for full diagnostics", p)
		return r
	}
	if len(diags) > 0 {
		r.Status = StatusWarning
		r.Message = "config has warnings"
		r.Detail = diags[0].Message
		r.Fix = fmt.Sprintf("run `mooncake validate -c %s` for full diagnostics", p)
		return r
	}
	r.Status = StatusOK
	r.Message = "validates clean"
	return r
}

// checkProjectSummary prints step / include / preset counts. Info-only —
// always StatusInfo even when zero steps (that's a deliberate user state).
type checkProjectSummary struct{}

func (checkProjectSummary) Section() string { return "project" }
func (checkProjectSummary) Name() string    { return "summary" }
func (checkProjectSummary) Run(ctx Context) Result {
	r := Result{Section: "project", Name: "summary"}
	p := findProjectConfig(ctx)
	if p == "" {
		r.Status = StatusInfo
		r.Message = "skipped (no project config)"
		return r
	}
	parsed, err := config.ReadConfig(p)
	if err != nil || parsed == nil {
		r.Status = StatusInfo
		r.Message = "config unparseable; skipping summary"
		return r
	}
	stepCount, imports, presets := summariseSteps(parsed.Steps)
	r.Status = StatusInfo
	r.Message = fmt.Sprintf("%d step(s), %d import(s), %d preset use(s)", stepCount, imports, presets)
	return r
}

// summariseSteps counts top-level steps and how many invoke an `import:`
// (file inclusion) or `use:` (preset invocation). Iteration is shallow —
// nested imports are followed by the planner, not by doctor.
func summariseSteps(steps []config.Step) (n, imports, presets int) {
	for _, s := range steps {
		n++
		if s.Import != nil && *s.Import != "" {
			imports++
		}
		if s.Use != nil {
			presets++
		}
	}
	return
}

// checkProjectLockfile flags stale mooncake.lock files (>24h old). The
// lockfile package treats a missing file as fine; only stale locks
// indicate a previous run that didn't clean up.
type checkProjectLockfile struct{}

func (checkProjectLockfile) Section() string { return "project" }
func (checkProjectLockfile) Name() string    { return "lockfile" }
func (checkProjectLockfile) Run(ctx Context) Result {
	r := Result{Section: "project", Name: "lockfile"}
	if ctx.SkipProject {
		r.Status = StatusInfo
		r.Message = "skipped (--skip-project)"
		return r
	}
	candidates := []string{
		filepath.Join(ctx.Cwd, "mooncake.lock"),
		filepath.Join(ctx.Cwd, ".mooncake", "mooncake.lock"),
	}
	for _, p := range candidates {
		info, err := os.Stat(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			continue
		}
		age := time.Since(info.ModTime())
		if age > 24*time.Hour {
			r.Status = StatusWarning
			r.Message = fmt.Sprintf("stale lockfile (%s old): %s", formatAge(age), p)
			r.Fix = "inspect with `cat " + p + "` and `rm` if no run is in progress"
			return r
		}
	}
	r.Status = StatusOK
	r.Message = "no stale mooncake.lock detected"
	return r
}
