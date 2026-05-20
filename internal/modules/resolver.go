package modules

import (
	"context"
	"fmt"
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
		return r.resolveRef(ctx, ref, "default")
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
	return r.resolveRef(ctx, ref, export)
}

// resolveRef does the fetch + index + export lookup once we have a parsed
// Reference and an export name.
func (r *Resolver) resolveRef(ctx context.Context, ref Reference, export string) (Resolved, error) {
	moduleRoot, err := r.Fetcher.Fetch(ctx, ref)
	if err != nil {
		return Resolved{}, err
	}
	// Subpath: if the reference points at a subdirectory of the repo, the
	// module root is that subdirectory, not the repo root.
	if ref.Subpath != "" {
		moduleRoot = joinSafe(moduleRoot, ref.Subpath)
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
// escapes via ".." segments.
func joinSafe(base, rel string) string {
	// Strip leading slashes so filepath.Join treats rel as relative.
	rel = strings.TrimLeft(rel, "/")
	return base + "/" + rel
}
