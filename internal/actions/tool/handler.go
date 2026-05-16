package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/lockfile"
)

// Handler implements the `tool` action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action's static description.
func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "tool",
		Description:        "Install a developer tool at a pinned version with lockfile-backed reproducibility",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        nil,
		Version:            "0.1.0",
		SupportedPlatforms: []string{"linux", "darwin"},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

// Validate checks structural validity of the tool step and delegates
// backend-specific checks to the chosen Backend.
func (h *Handler) Validate(step *config.Step) error {
	if step.Tool == nil {
		return fmt.Errorf("tool configuration is nil")
	}
	t := step.Tool
	if t.Name == "" {
		return fmt.Errorf("tool: name is required")
	}
	if t.Version == "" {
		return fmt.Errorf("tool: version is required (concrete versions only in v1; constraints not supported)")
	}
	if t.Backend == "" {
		return fmt.Errorf("tool: backend is required (supported: %s)", supportedBackendsList())
	}
	backend, err := Get(t.Backend)
	if err != nil {
		return err
	}
	return backend.Validate(t)
}

// Execute installs the tool. Idempotent on (name, version) by virtue of
// the install dir check inside the install pipeline.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	t := step.Tool

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	result := executor.NewResult()
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	backend, err := Get(t.Backend)
	if err != nil {
		return result, err
	}

	// Render template strings against the full execution context (vars,
	// facts, preset parameters). The backend then receives concrete
	// values and only does its narrow {{ version }}/{{ os }}/{{ arch }}
	// substitution if any literals remain.
	renderedTool, err := renderToolTemplates(t, ctx)
	if err != nil {
		return result, fmt.Errorf("render tool fields: %w", err)
	}

	spec := specFromConfig(renderedTool)
	facts := factsFromVars(ctx.GetVariables())

	// F007: bound the install with a 30-minute outer ceiling.
	// actions.Context doesn't expose a Go context.Context today, so we
	// can't plumb the executor's parent ctx through; the 30m cap is the
	// pragmatic alternative. Large tool archives (LLVM, CUDA SDK) can
	// take a while; everything else is far under this. The Plan-mode
	// HEAD probe inside the backend bounds itself at 5s internally so a
	// stuck endpoint can't dominate Plan.
	toolCtx, cancelToolCtx := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancelToolCtx()

	plan, err := backend.Plan(toolCtx, spec, facts)
	if err != nil {
		return result, fmt.Errorf("backend plan: %w", err)
	}

	lockPath := resolveLockfilePath(toolLockBaseDir(ec))
	lock, err := lockfile.Load(lockPath)
	if err != nil {
		return result, fmt.Errorf("load lockfile %s: %w", lockPath, err)
	}

	// Cross-backend safety: reject if a different backend already locked
	// the same (name, version).
	if existing, ok := lock.LookupByName(t.Name, t.Version); ok && existing.Backend != t.Backend {
		return result, fmt.Errorf(
			"tool: lockfile binds %s %s to backend %q; cannot install via %q (remove the lock entry to switch)",
			t.Name, t.Version, existing.Backend, t.Backend,
		)
	}

	var outcome Outcome
	if plan.UseSharedPipeline {
		outcome, err = InstallURL(toolCtx, spec, plan, facts, lock)
		if err != nil {
			return result, err
		}
	} else {
		// Backend owns the install (mise, future delegating backends).
		// Pre-check via Locate; if found, skip. Otherwise Install, then
		// Locate again to discover the real install dir.
		preBin, preErr := backend.Locate(toolCtx, spec, "")
		if preErr == nil && preBin != "" {
			outcome = Outcome{
				Changed:    false,
				InstallDir: filepath.Dir(preBin),
				Reason:     fmt.Sprintf("%s %s already installed via %s at %s", spec.Name, spec.Version, spec.Backend, preBin),
			}
		} else {
			if err := backend.Install(toolCtx, spec, plan, ""); err != nil {
				return result, err
			}
			postBin, postErr := backend.Locate(toolCtx, spec, "")
			switch {
			case postErr != nil:
				return result, fmt.Errorf("locate %s after install (backend %s): %w", spec.Name, spec.Backend, postErr)
			case postBin == "":
				return result, fmt.Errorf("backend %s reported install success for %s@%s but cannot locate the binary (mise install ran but `mise which` returns nothing — version mismatch in the mise registry?)", spec.Backend, spec.Name, spec.Version)
			}
			outcome = Outcome{
				Changed:    true,
				InstallDir: filepath.Dir(postBin),
				Reason:     fmt.Sprintf("installed %s %s via %s", spec.Name, spec.Version, spec.Backend),
			}
			// Record an abbreviated lock entry — backend-owned installs
			// don't carry URL/sha256 since the backend owns integrity.
			lock.Set(lockfile.Entry{
				Backend:  spec.Backend,
				Name:     spec.Name,
				Version:  spec.Version,
				LockedAt: time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	result.SetChanged(outcome.Changed)
	if outcome.Reason != "" {
		ctx.GetLogger().Infof("  %s", outcome.Reason)
	}

	if outcome.Changed {
		if err := lock.Save(lockPath); err != nil {
			return result, fmt.Errorf("save lockfile %s: %w", lockPath, err)
		}
		if t.WriteToolVersions {
			if err := appendToolVersions(filepath.Dir(lockPath), t.Name, t.Version); err != nil {
				return result, fmt.Errorf("write .tool-versions: %w", err)
			}
		}
	}

	result.SetData(map[string]interface{}{
		"install_dir":  outcome.InstallDir,
		"resolved_url": outcome.ResolvedURL,
		"checksum":     outcome.Checksum,
		"backend":      spec.Backend,
		"name":         spec.Name,
		"version":      spec.Version,
	})

	return result, nil
}

// DryRun reports what would happen without touching the network or filesystem.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	t := step.Tool

	installDir, err := InstallDir(t.Name, t.Version)
	if err != nil {
		return err
	}
	if backend, err := Get(t.Backend); err == nil {
		spec := specFromConfig(t)
		if binPath, locErr := backend.Locate(context.Background(), spec, installDir); locErr == nil && binPath != "" {
			ctx.GetLogger().Infof("  [DRY-RUN] %s %s already installed at %s", t.Name, t.Version, filepath.Dir(binPath))
			return nil
		}
	}
	ctx.GetLogger().Infof("  [DRY-RUN] Would install %s %s via %s", t.Name, t.Version, t.Backend)
	return nil
}

// specFromConfig converts a YAML-parsed config.Tool into the internal Spec.
func specFromConfig(t *config.Tool) Spec {
	return Spec{
		Backend:         t.Backend,
		Name:            t.Name,
		Version:         t.Version,
		URL:             t.URL,
		Repo:            t.Repo,
		Asset:           t.Asset,
		Tag:             t.Tag,
		MiseTool:        t.MiseTool,
		Env:             t.Env,
		InlineChecksum:  t.Checksum,
		StripComponents: t.StripComponents,
		Bin:             t.Bin,
	}
}

// renderToolTemplates returns a copy of t with all templatable string
// fields rendered against ctx.GetVariables(). This bridges preset
// parameters ({{ parameters.version }}) and vars: blocks ({{ mise_os }})
// to the backend, which otherwise only handles {{ version }}/{{ os }}/
// {{ arch }} via a string replacer.
func renderToolTemplates(t *config.Tool, ctx actions.Context) (*config.Tool, error) {
	render := func(s string) (string, error) {
		if s == "" {
			return s, nil
		}
		return ctx.GetTemplate().Render(s, ctx.GetVariables())
	}

	cp := *t
	// Field table — order matches `config.Tool` declaration so the
	// error sequence stays predictable and a new templatable field is
	// one row to add instead of a paste of the if/wrap/assign block.
	fields := []struct {
		name string
		src  string
		dst  *string
	}{
		{"name", t.Name, &cp.Name},
		{"version", t.Version, &cp.Version},
		{"url", t.URL, &cp.URL},
		{"repo", t.Repo, &cp.Repo},
		{"asset", t.Asset, &cp.Asset},
		{"tag", t.Tag, &cp.Tag},
		{"checksum", t.Checksum, &cp.Checksum},
		{"bin", t.Bin, &cp.Bin},
		{"mise_tool", t.MiseTool, &cp.MiseTool},
	}
	for _, f := range fields {
		rendered, err := render(f.src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.name, err)
		}
		*f.dst = rendered
	}
	return &cp, nil
}

// factsFromVars pulls `os` and `arch` from the variables map. Mooncake
// merges system facts into variables under these keys.
func factsFromVars(vars map[string]interface{}) FactSnapshot {
	osVal, _ := vars["os"].(string)
	archVal, _ := vars["arch"].(string)
	return FactSnapshot{OS: osVal, Arch: archVal}
}

// resolveLockfilePath walks up from startDir looking for an existing
// mooncake.lock. If none is found, returns startDir + "/mooncake.lock"
// (i.e. create alongside the config that declared the tool). This
// mirrors npm/cargo behavior: nearest lockfile wins.
//
// Callers should pass the user-facing project directory, NOT
// ec.CurrentDir — the latter can be temporarily set to a preset's base
// directory during preset expansion, which would land the lockfile
// inside the preset rather than the user's project. See toolLockBaseDir().
func resolveLockfilePath(startDir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, lockfile.Filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir { // hit root
			break
		}
		dir = parent
	}
	return filepath.Join(startDir, lockfile.Filename)
}

// toolLockBaseDir returns the directory to start searching for the
// lockfile from. Process working directory is the source of truth —
// it's stable across preset expansion and include traversal, and it's
// where users actually run `mooncake apply` from. Falls back to
// ec.CurrentDir if os.Getwd fails (e.g. CWD was deleted).
func toolLockBaseDir(ec *executor.ExecutionContext) string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ec.CurrentDir
}

// appendToolVersions appends "<name> <version>\n" to dir/.tool-versions,
// or replaces an existing line for the same name. Format compatible with
// asdf/mise.
func appendToolVersions(dir, name, version string) error {
	path := filepath.Join(dir, ".tool-versions")
	// #nosec G304 -- dir is the lockfile dir, derived from CurrentDir
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := splitLines(string(existing))
	replaced := false
	for i, line := range lines {
		fields := splitFields(line)
		if len(fields) > 0 && fields[0] == name {
			lines[i] = name + " " + version
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, name+" "+version)
	}

	out := joinLines(lines)
	// #nosec G306 -- .tool-versions is a user-editable config file
	return os.WriteFile(path, []byte(out), 0o644)
}

// Run is the Spec 16 unified entry point. Plan mode inspects whether the
// tool is already installed (mooncake's standard layout for URL-based
// backends; Backend.Locate for backends that own their layout) and
// reports already-ok or would-install. Execute mode delegates to the
// legacy Execute path.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() != actions.ModePlan {
		return h.Execute(ctx, step)
	}

	t := step.Tool
	if t == nil {
		return nil, fmt.Errorf("tool configuration is nil")
	}

	result := executor.NewResult()
	result.Checkable = true

	// Standard layout installDir is passed through to Locate. URL-based
	// backends use it as their root; mise's Locate ignores it and
	// consults `mise which` directly.
	installDir, err := InstallDir(t.Name, t.Version)
	if err != nil {
		return result, err
	}

	backend, err := Get(t.Backend)
	if err != nil {
		return result, err
	}
	spec := specFromConfig(t)
	binPath, locErr := backend.Locate(context.Background(), spec, installDir)
	if locErr == nil && binPath != "" {
		result.Reason = fmt.Sprintf("%s %s already installed at %s", t.Name, t.Version, filepath.Dir(binPath))
		return result, nil
	}
	result.WouldChange = true
	result.Reason = fmt.Sprintf("would install %s %s via %s", t.Name, t.Version, t.Backend)
	return result, nil
}
