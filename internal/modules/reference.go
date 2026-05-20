// Package modules implements the Git-native module system from spec-67.
// A module is a Git repository (or subpath of one) that exports one or more
// components via index.yml. References are pinned to explicit Git tags.
package modules

import (
	"fmt"
	"strings"
)

// Reference is a parsed module reference of the form
// <host>/<owner>/<repo>[/<subpath>]@<version>.
//
// Example: github.com/mooncake-modules/postgres@v1.3.0 parses to
// {Host:"github.com", Owner:"mooncake-modules", Repo:"postgres", Version:"v1.3.0"}.
type Reference struct {
	Host    string
	Owner   string
	Repo    string
	Subpath string
	Version string
}

// ParseReference parses a module reference string. The version is required —
// references without "@<version>" are rejected.
func ParseReference(s string) (Reference, error) {
	if s == "" {
		return Reference{}, fmt.Errorf("empty module reference")
	}

	at := strings.LastIndex(s, "@")
	if at < 0 {
		return Reference{}, fmt.Errorf("expected <url>@<version>, e.g. github.com/owner/repo@v1.0.0; got %q", s)
	}
	pathPart := s[:at]
	version := s[at+1:]
	if pathPart == "" || version == "" {
		return Reference{}, fmt.Errorf("expected <url>@<version>, e.g. github.com/owner/repo@v1.0.0; got %q", s)
	}

	parts := strings.Split(pathPart, "/")
	if len(parts) < 3 {
		return Reference{}, fmt.Errorf("module path must be <host>/<owner>/<repo>[/subpath]; got %q", pathPart)
	}
	for _, p := range parts {
		if p == "" {
			return Reference{}, fmt.Errorf("module path has an empty segment: %q", pathPart)
		}
	}

	ref := Reference{
		Host:    parts[0],
		Owner:   parts[1],
		Repo:    parts[2],
		Version: version,
	}
	if len(parts) > 3 {
		ref.Subpath = strings.Join(parts[3:], "/")
	}
	return ref, nil
}

// ModulePath returns the path portion (no version), e.g.
// "github.com/owner/repo" or "github.com/owner/repo/subpath".
func (r Reference) ModulePath() string {
	if r.Subpath == "" {
		return r.Host + "/" + r.Owner + "/" + r.Repo
	}
	return r.Host + "/" + r.Owner + "/" + r.Repo + "/" + r.Subpath
}

// String returns the canonical "<path>@<version>" form.
func (r Reference) String() string {
	return r.ModulePath() + "@" + r.Version
}

// CloneURL returns the https URL of the underlying Git repository
// (subpath is not part of the clone URL).
func (r Reference) CloneURL() string {
	return "https://" + r.Host + "/" + r.Owner + "/" + r.Repo + ".git"
}
