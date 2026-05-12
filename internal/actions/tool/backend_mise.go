package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
)

func init() {
	register(&miseBackend{})
}

// miseBackend delegates to a `mise` binary on PATH (https://mise.jdx.dev).
// Mooncake provides the declarative wrapper; mise provides the tool
// catalog (Node, Python, Ruby, Java, dozens of language ecosystems) and
// integrity. The action records only (backend, name, version) in
// mooncake.lock — mise owns its own lockfile for checksum reproducibility.
//
// Bin override: by default the mise tool ID equals `name`. Set
// `mise_tool` when the tool's mise catalog ID differs (e.g. java
// runtimes use prefixed IDs like "temurin").
type miseBackend struct{}

// miseRunner abstracts exec.Command so tests can swap in a shell stub
// without prepending to PATH. Default is realMiseRunner (uses
// os/exec.LookPath + Command, with a fallback to mooncake-managed mise
// installs). Tests can override via the package-level variable.
type miseRunner interface {
	lookPath() error
	install(ctx context.Context, tool, version string, env map[string]string) error
	which(ctx context.Context, tool, version string) (string, error)
}

var defaultMiseRunner miseRunner = realMiseRunner{}

func (miseBackend) Name() string { return BackendMise }

func (miseBackend) Validate(t *config.Tool) error {
	// Forbid URL-based fields.
	if t.URL != "" {
		return fmt.Errorf("mise backend: url is an archive-url field; remove it")
	}
	if t.Repo != "" || t.Asset != "" || t.Tag != "" {
		return fmt.Errorf("mise backend: repo/asset/tag are github-release fields")
	}
	if t.Checksum != "" {
		return fmt.Errorf("mise backend: checksum is not supported (mise owns integrity)")
	}
	if t.StripComponents != 0 {
		return fmt.Errorf("mise backend: strip_components is an archive field")
	}
	if t.Bin != "" {
		return fmt.Errorf("mise backend: bin is not supported (use `mise which` to discover the bin path)")
	}
	if t.WriteToolVersions {
		return fmt.Errorf("mise backend: write_tool_versions is not supported (mise manages .tool-versions itself)")
	}
	if err := defaultMiseRunner.lookPath(); err != nil {
		return fmt.Errorf("mise backend: %w; install mise first via the `mise-bootstrap` preset, the upstream installer (https://mise.jdx.dev/getting-started.html), or use the archive-url backend", err)
	}
	return nil
}

func (miseBackend) Plan(_ context.Context, _ Spec, _ FactSnapshot) (Plan, error) {
	// Mise picks the right arch/OS itself; nothing to resolve up front.
	return Plan{UseSharedPipeline: false}, nil
}

func (miseBackend) Install(ctx context.Context, spec Spec, _ Plan, _ string) error {
	return defaultMiseRunner.install(ctx, miseToolID(spec), spec.Version, spec.Env)
}

func (miseBackend) Locate(ctx context.Context, spec Spec, _ string) (string, error) {
	binPath, err := defaultMiseRunner.which(ctx, miseToolID(spec), spec.Version)
	if err != nil {
		// `mise which` exits non-zero when the version isn't installed;
		// treat that as "not installed" rather than propagating the error.
		return "", nil //nolint:nilerr // intentional: see Locate contract in backend.go
	}
	return binPath, nil
}

// miseToolID returns the catalog ID mise should use. Defaults to the
// tool name; overridden by mise_tool when set.
func miseToolID(spec Spec) string {
	if spec.MiseTool != "" {
		return spec.MiseTool
	}
	return spec.Name
}

// realMiseRunner is the default miseRunner backed by exec.Command. It
// resolves `mise` in this order:
//   1. exec.LookPath("mise") — anything on the user's PATH (system
//      install, manual install, mooncake-managed install already on
//      PATH via `mooncake tool env`)
//   2. The mooncake tool store — `<store>/mise/<version>/bin/mise`
//      for any installed version. This is what makes the
//      `mise-bootstrap` preset (Spec 19 E8.7) work in a single apply:
//      after the bootstrap step installs mise into mooncake's tree,
//      the next `backend: mise` step finds it here without any PATH
//      manipulation.
type realMiseRunner struct{}

func (r realMiseRunner) lookPath() error {
	_, err := r.resolveMisePath()
	return err
}

func (r realMiseRunner) install(ctx context.Context, tool, version string, env map[string]string) error {
	misePath, err := r.resolveMisePath()
	if err != nil {
		return err
	}
	// #nosec G204 -- misePath comes from LookPath or the mooncake store; tool/version from declared config
	cmd := exec.CommandContext(ctx, misePath, "install", tool+"@"+version)
	cmd.Env = mergeEnv(os.Environ(), env)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mise install %s@%s failed: %w\n[mise] %s", tool, version, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (r realMiseRunner) which(ctx context.Context, tool, version string) (string, error) {
	misePath, err := r.resolveMisePath()
	if err != nil {
		return "", err
	}
	// `mise where <tool>@<version>` returns the install directory of the
	// specific version (e.g. ~/.local/share/mise/installs/jq/1.7.1).
	// We don't use `mise which` directly: it only returns a path when the
	// tool is *activated* in the current dir (via .tool-versions), and
	// mooncake's whole point is to avoid that activation coupling.
	// #nosec G204 -- misePath comes from LookPath or the mooncake store; tool/version from declared config
	cmd := exec.CommandContext(ctx, misePath, "where", tool+"@"+version)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	installDir := strings.TrimSpace(string(out))
	if installDir == "" {
		return "", nil
	}
	// mise's bin layout is tool-dependent. Common layouts:
	//   <installDir>/bin/<tool>   (node, python, ruby, go, ...)
	//   <installDir>/<tool>       (jq, single-binary tools)
	// Probe both. If neither exists, fall back to <installDir>/bin/<tool>
	// (which surfaces as a clear ENOENT later when the caller execs it).
	for _, rel := range []string{"bin/" + tool, tool} {
		candidate := filepath.Join(installDir, rel)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return filepath.Join(installDir, "bin", tool), nil
}

// resolveMisePath returns the absolute path to a usable `mise` binary,
// preferring PATH and falling back to the mooncake tool store.
func (realMiseRunner) resolveMisePath() (string, error) {
	if path, err := exec.LookPath("mise"); err == nil {
		return path, nil
	}
	if path := findMooncakeManagedMise(); path != "" {
		return path, nil
	}
	return "", fmt.Errorf("mise binary not found on PATH (and no mooncake-managed install at %s)", mooncakeMiseGlobHint())
}

// findMooncakeManagedMise looks for an installed mise under the
// mooncake store. Returns the first match (any version) or "" if none
// exists. v1 doesn't sort by version; for a single bootstrapped install
// that's fine, and if multiple versions are present, any one works
// (mise's own resolver picks per-project versions).
func findMooncakeManagedMise() string {
	root, err := StoreRoot()
	if err != nil {
		return ""
	}
	miseDir := filepath.Join(root, "mise")
	entries, err := os.ReadDir(miseDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(miseDir, e.Name(), "bin", "mise")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// mooncakeMiseGlobHint is the human-readable path used in error messages
// pointing at where mooncake would expect a bootstrapped mise binary.
func mooncakeMiseGlobHint() string {
	root, err := StoreRoot()
	if err != nil {
		return "~/.local/share/mooncake/tools/mise/*/bin/mise"
	}
	return filepath.Join(root, "mise", "*", "bin", "mise")
}

// mergeEnv overlays kv onto base. base must already include os.Environ()
// or whatever inherited env the caller intends.
func mergeEnv(base []string, kv map[string]string) []string {
	if len(kv) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(kv))
	skip := make(map[string]struct{}, len(kv))
	for k := range kv {
		skip[k+"="] = struct{}{}
	}
	for _, e := range base {
		drop := false
		for prefix := range skip {
			if strings.HasPrefix(e, prefix) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, e)
		}
	}
	for k, v := range kv {
		out = append(out, k+"="+v)
	}
	return out
}
