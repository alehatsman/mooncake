package security

// secrets.go implements the secret-provider plumbing for spec-23 §3
// (`!secret <provider>:<path>` YAML tag).
//
// Wire path:
//
//  1. YAML decoder (internal/config/reader.go) recognizes the `!secret`
//     tag and rewrites the node's value to the sentinel marker
//     SentinelPrefix + "<provider>:<path>". The action-struct string
//     field carries the marker through plan compilation.
//  2. At apply time, the executor walks the step's action params and
//     calls ResolveMarker on any marker-bearing string, which routes
//     through the global Registry to the right Provider.
//  3. Resolved values are added to the run's Redactor denylist so they
//     don't leak through events / runlog / step.stdout.
//
// Plan mode never resolves — markers stay in the in-memory plan and
// get redacted at JSON-output time (see cmd/mooncake.go plan command).

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// SentinelPrefix is the in-memory marker that identifies a secret-bearing
// string after YAML parsing. The format is SentinelPrefix + "<ref>" where
// ref is the provider:path form (e.g. "env:APP_TOKEN"). The prefix is
// plain printable ASCII so it survives JSON/YAML round-trips without
// escape transformation (a NUL-byte sentinel got JSON-escaped in
// JSON output, breaking the plan-output redactor's match). It's long
// and uniquely-shaped enough that natural user data won't collide.
const SentinelPrefix = "__MOONCAKE_SECRET_v1_DO_NOT_EDIT__:"

// Provider resolves a per-provider path to its secret value. Implementations
// run synchronously and should be idempotent — the same path returns the
// same value within the lifetime of an apply.
//
// Convention for error messages: redact the path beyond the provider
// prefix. `vault:secret/app#token` → `vault: ...` so partial-secret data
// doesn't leak into logs.
type Provider interface {
	// Resolve returns the secret's value for the given path (the substring
	// after the provider's `name:` prefix). For env:APP_TOKEN, path is
	// "APP_TOKEN".
	Resolve(path string) (string, error)
}

// Registry holds the set of providers active for one run. Lookup is by
// provider name; registration happens at startup (init() functions in
// each provider's package) or by user-supplied configuration.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry returns an empty registry. Callers typically use
// DefaultRegistry instead.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register installs a provider under the given name. Overwrites silently
// if the name was already registered — allows a host program to swap
// providers in tests or to opt out of built-in providers entirely.
func (r *Registry) Register(name string, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
}

// Resolve looks up `ref` (in "<provider>:<path>" form) and delegates to
// the matching Provider. Returns a descriptive error if the provider is
// unknown or the path can't be resolved.
func (r *Registry) Resolve(ref string) (string, error) {
	colon := strings.IndexByte(ref, ':')
	if colon < 0 {
		return "", fmt.Errorf("secret ref %q: expected <provider>:<path>", ref)
	}
	name := ref[:colon]
	path := ref[colon+1:]
	r.mu.RLock()
	p, ok := r.providers[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("secret ref %q: unknown provider %q", ref, name)
	}
	val, err := p.Resolve(path)
	if err != nil {
		// Deliberately do NOT %w-wrap the provider's error: providers may
		// include the path in their own error (e.g. "env: APP_TOKEN not
		// set") and that path is exactly what we don't want surfaced in
		// shared logs. The provider's underlying error is available to
		// callers that want it via a separate audit path; this is the
		// human-facing message.
		return "", fmt.Errorf("secret ref %s: provider failed", name+":...")
	}
	return val, nil
}

// IsMarker reports whether s is a sentinel-marker string carrying a
// secret ref. Cheap O(1) prefix check.
func IsMarker(s string) bool {
	return strings.HasPrefix(s, SentinelPrefix)
}

// MarkerRef returns the "<provider>:<path>" portion of a sentinel-marker
// string. Returns empty string if s isn't a marker.
func MarkerRef(s string) string {
	if !IsMarker(s) {
		return ""
	}
	return s[len(SentinelPrefix):]
}

// ResolveMarker is the convenience entry point for the executor: given a
// marker-bearing string, returns the resolved secret value via the global
// DefaultRegistry. Returns the input unchanged if s isn't a marker — so
// callers can blindly pipe every string through this function.
func ResolveMarker(s string) (string, bool, error) {
	if !IsMarker(s) {
		return s, false, nil
	}
	val, err := DefaultRegistry.Resolve(MarkerRef(s))
	return val, true, err
}

// DefaultRegistry is the package-level singleton populated by the
// built-in providers in their init() functions and supplemented at
// startup by cmd/mooncake.go for any user-registered providers.
var DefaultRegistry = NewRegistry()

// EnvProvider reads from the controller's process environment. The
// simplest possible provider; ships as the v1 built-in so the spec's
// canonical example (`!secret env:APP_TOKEN`) works without any extra
// configuration on the user's part.
type EnvProvider struct{}

// Resolve looks up `path` in the process environment. An empty value is
// treated as unset — `export FOO=` should not silently propagate an
// empty secret.
func (EnvProvider) Resolve(path string) (string, error) {
	if path == "" {
		return "", errors.New("env provider: empty variable name")
	}
	val, ok := os.LookupEnv(path)
	if !ok || val == "" {
		return "", fmt.Errorf("env provider: %s not set", path)
	}
	return val, nil
}

func init() {
	DefaultRegistry.Register("env", EnvProvider{})
}
