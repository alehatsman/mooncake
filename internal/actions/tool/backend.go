// Package tool implements the `tool` action: declarative tool provisioning
// with lockfile-backed reproducibility. Spec 19.
//
// v1 ships a single backend (archive-url). The Backend abstraction is
// strategy-shaped (Validate / Plan / Install / Locate) so github-release
// (sugar over archive-url) and mise (delegating) can slot in without
// rewriting the action surface.
package tool

import (
	"context"
	"fmt"

	"github.com/alehatsman/mooncake/internal/config"
)

// Backend names.
const (
	BackendArchiveURL    = "archive-url"
	BackendGitHubRelease = "github-release"
	BackendMise          = "mise"
)

// FactSnapshot is the OS+arch view used for URL templating. Backends are
// pure functions of (Spec, FactSnapshot); the action handler supplies it
// from mooncake's facts.
type FactSnapshot struct {
	OS   string // "linux", "darwin"
	Arch string // "amd64", "arm64"
}

// Spec is the parsed, normalized request to install a tool. Built by the
// handler from a config.Tool step.
type Spec struct {
	Backend string
	Name    string
	Version string

	// archive-url
	URL string

	// github-release (parsed for v1 schema; unused by v1 backend)
	Repo, Asset, Tag string

	// mise (parsed for v1 schema; unused by v1 backend)
	MiseTool string
	Env      map[string]string

	// Common
	InlineChecksum  string
	StripComponents int
	Bin             string
}

// Plan describes what installing this tool will require. Filled by
// Backend.Plan, consumed by the shared install pipeline (URL-based) or
// the backend's own Install (mise).
type Plan struct {
	URL             string // resolved download URL (URL-based backends)
	Checksum        string // sha256:... — empty means TOFU on first install
	StripComponents int
	BinRel          string // bin path relative to install dir
	// UseSharedPipeline=true → the handler will call install.URL() with
	// this Plan. false → the handler will call Backend.Install() and the
	// backend is responsible for everything (mise).
	UseSharedPipeline bool
}

// Backend is the install strategy for one kind of source (archive-url,
// github-release, mise). Backends are stateless; one instance per process.
type Backend interface {
	// Name returns the backend name as it appears in YAML.
	Name() string

	// Validate is called from the handler's Validate phase. Should reject
	// missing backend-specific fields and check any preconditions (e.g.
	// `mise` on PATH). The handler has already validated common fields
	// (name, version, backend).
	Validate(t *config.Tool) error

	// Plan resolves the spec into a concrete Plan (URL, checksum, layout).
	// Backends MUST NOT touch the filesystem here.
	Plan(ctx context.Context, spec Spec, facts FactSnapshot) (Plan, error)

	// Install performs backend-specific install steps. URL-based backends
	// return Plan{UseSharedPipeline: true} from Plan() and leave this as a
	// no-op; the shared pipeline handles them. Mise overrides this.
	Install(ctx context.Context, spec Spec, plan Plan, installDir string) error

	// Locate is the canonical presence-and-path query. Returns:
	//   ("", nil)         — tool is not installed via this backend
	//   (binPath, nil)    — tool is installed; binPath is the absolute
	//                       path to its executable (or, when spec.Bin is
	//                       empty, the install dir)
	//   ("", err)         — transient failure (e.g. backend binary not
	//                       on PATH for mise); callers may treat as
	//                       "not installed"
	//
	// installDir is mooncake's standard layout for the (name, version);
	// URL backends use it as their root, mise's Locate ignores it.
	Locate(ctx context.Context, spec Spec, installDir string) (string, error)
}

// Registry holds the available backends. Populated by init() in each
// backend file.
var registry = map[string]Backend{}

// register adds a backend to the registry. Called from init().
func register(b Backend) {
	if _, exists := registry[b.Name()]; exists {
		panic(fmt.Sprintf("tool backend already registered: %s", b.Name()))
	}
	registry[b.Name()] = b
}

// Get returns the registered backend for name, or an error if unknown.
func Get(name string) (Backend, error) {
	b, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool backend %q (supported: %s)", name, supportedBackendsList())
	}
	return b, nil
}

// SupportedBackends returns the list of registered backend names. Stable
// order for use in error messages.
func SupportedBackends() []string {
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	// Stable order: archive-url first, then alphabetical.
	for i, n := range out {
		if n == BackendArchiveURL {
			out[0], out[i] = out[i], out[0]
			break
		}
	}
	return out
}

func supportedBackendsList() string {
	names := SupportedBackends()
	s := ""
	for i, n := range names {
		if i > 0 {
			s += ", "
		}
		s += n
	}
	return s
}
