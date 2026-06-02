package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Resolver turns a use: reference into a concrete component file path. It
// combines reference parsing, module fetching, and index.yml export lookup.
//
// Aliases are looked up in Modules (typically the playbook's modules: block).
// Inline remote references (`github.com/owner/repo@v1.0.0`) skip the alias map.
type Resolver struct {
	Fetcher *Fetcher
	Modules map[string]string
}

// NewResolver constructs a resolver with the default fetcher and the supplied
// alias map. Passing nil for the map disables alias resolution.
func NewResolver(modules map[string]string) *Resolver {
	return &Resolver{Fetcher: &Fetcher{}, Modules: modules}
}

// Resolved is what the resolver returns: the file path to load as a component
// plus the module root (needed so further imports inside the component resolve
// relative to the module, not the playbook).
type Resolved struct {
	ComponentPath string
	ModuleRoot    string
}

// Resolve takes a use: reference string and returns the component file to
// load. The refStr should be in one of these forms:
//
//	"github.com/owner/repo@v1.0.0"     - inline remote, default export
//	"alias"                             - alias from Modules, default export
//	"alias/export"                      - alias + named export
//
// Local paths (./foo.yml) are NOT handled here — the executor dispatches those
// directly without going through the resolver.
func (r *Resolver) Resolve(ctx context.Context, refStr string) (Resolved, error) {
	return r.resolve(ctx, refStr, r.Fetcher.Fetch)
}

// ResolveCached behaves like Resolve but never clones: if the module is not
// already present in the local cache it returns an error instead of fetching
// over the network. Read-only callers that must stay offline — the
// `mooncake task` listing, which resolves a component's description: — use
// this so a listing never triggers a clone.
func (r *Resolver) ResolveCached(ctx context.Context, refStr string) (Resolved, error) {
	return r.resolve(ctx, refStr, r.Fetcher.FetchCached)
}

// fetchFunc abstracts the two module-fetch strategies (Fetcher.Fetch and
// Fetcher.FetchCached) so resolveRef is shared between Resolve/ResolveCached.
type fetchFunc func(ctx context.Context, ref Reference) (string, error)

// resolve is the shared body of Resolve/ResolveCached, parameterized by the
// fetch strategy (clone-on-miss vs cache-only).
func (r *Resolver) resolve(ctx context.Context, refStr string, fetch fetchFunc) (Resolved, error) {
	if refStr == "" {
		return Resolved{}, fmt.Errorf("empty component reference")
	}

	// Inline remote: contains "@" → parse as a Reference and fetch directly.
	// Inline form does not support an export suffix (per spec-67 phase 1).
	if strings.Contains(refStr, "@") {
		ref, err := ParseReference(refStr)
		if err != nil {
			return Resolved{}, err
		}
		return r.resolveRef(ctx, ref, "default", fetch)
	}

	// Alias form: split into (alias, export) and look up in the modules map.
	alias, export := refStr, "default"
	if i := strings.Index(refStr, "/"); i >= 0 {
		alias, export = refStr[:i], refStr[i+1:]
	}
	refForAlias, ok := r.Modules[alias]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown module alias %q (not declared in `modules:` block)", alias)
	}
	ref, err := ParseReference(refForAlias)
	if err != nil {
		return Resolved{}, fmt.Errorf("modules[%q] = %q: %w", alias, refForAlias, err)
	}
	return r.resolveRef(ctx, ref, export, fetch)
}

// resolveRef does the fetch + index + export lookup once we have a parsed
// Reference and an export name.
func (r *Resolver) resolveRef(ctx context.Context, ref Reference, export string, fetch fetchFunc) (Resolved, error) {
	moduleRoot, err := fetch(ctx, ref)
	if err != nil {
		return Resolved{}, err
	}
	// Subpath: if the reference points at a subdirectory of the repo, the
	// module root is that subdirectory, not the repo root.
	if ref.Subpath != "" {
		moduleRoot, err = joinSafe(moduleRoot, ref.Subpath)
		if err != nil {
			return Resolved{}, fmt.Errorf("module %s: %w", ref.String(), err)
		}
	}
	idx, err := LoadIndex(moduleRoot)
	if err != nil {
		return Resolved{}, fmt.Errorf("module %s: %w", ref.String(), err)
	}
	componentPath, err := idx.ResolveExport(moduleRoot, export)
	if err != nil {
		return Resolved{}, fmt.Errorf("module %s: %w", ref.String(), err)
	}
	return Resolved{ComponentPath: componentPath, ModuleRoot: moduleRoot}, nil
}

// joinSafe joins a base directory with a relative subpath while preventing
// escapes via ".." segments. It returns an error (rather than a silently
// escaping path) when rel resolves outside base — a malicious module
// Reference.Subpath like "../../../etc" must not let LoadIndex/ResolveExport
// read arbitrary filesystem locations.
func joinSafe(base, rel string) (string, error) {
	// Strip leading slashes so filepath.Join treats rel as relative.
	rel = strings.TrimLeft(rel, "/")
	joined := filepath.Join(base, rel)
	relPath, err := filepath.Rel(base, joined)
	if err != nil {
		return "", fmt.Errorf("subpath %q is not relative to module root: %w", rel, err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("subpath %q escapes module root", rel)
	}
	return joined, nil
}
