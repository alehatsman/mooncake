package modules

import (
	"strings"
	"testing"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Reference
	}{
		{
			name: "github simple",
			in:   "github.com/mooncake-modules/postgres@v1.3.0",
			want: Reference{
				Host: "github.com", Owner: "mooncake-modules",
				Repo: "postgres", Version: "v1.3.0",
			},
		},
		{
			name: "with subpath",
			in:   "github.com/owner/repo/subdir/pkg@v0.1.0",
			want: Reference{
				Host: "github.com", Owner: "owner", Repo: "repo",
				Subpath: "subdir/pkg", Version: "v0.1.0",
			},
		},
		{
			name: "non-github host",
			in:   "gitlab.example.com/team/proj@v2.0.0",
			want: Reference{
				Host: "gitlab.example.com", Owner: "team",
				Repo: "proj", Version: "v2.0.0",
			},
		},
		{
			name: "non-vee tag",
			in:   "github.com/o/r@release-2026-05",
			want: Reference{
				Host: "github.com", Owner: "o", Repo: "r",
				Version: "release-2026-05",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseReference(tc.in)
			if err != nil {
				t.Fatalf("ParseReference(%q) error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseReference(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseReference_Errors(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		errSub string
	}{
		{"empty", "", "empty module reference"},
		{"no version", "github.com/owner/repo", "expected <url>@<version>"},
		{"empty version", "github.com/owner/repo@", "expected <url>@<version>"},
		{"empty path", "@v1.0.0", "expected <url>@<version>"},
		{"too few segments", "github.com/owner@v1.0.0", "module path must be"},
		{"empty segment", "github.com//repo@v1.0.0", "empty segment"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseReference(tc.in)
			if err == nil {
				t.Fatalf("ParseReference(%q) succeeded, want error %q", tc.in, tc.errSub)
			}
			if !strings.Contains(err.Error(), tc.errSub) {
				t.Errorf("ParseReference(%q) error = %q, want substring %q", tc.in, err.Error(), tc.errSub)
			}
		})
	}
}

func TestReferenceFormat(t *testing.T) {
	r := Reference{
		Host: "github.com", Owner: "owner", Repo: "repo",
		Subpath: "sub/dir", Version: "v1.0.0",
	}
	if got, want := r.ModulePath(), "github.com/owner/repo/sub/dir"; got != want {
		t.Errorf("ModulePath = %q, want %q", got, want)
	}
	if got, want := r.String(), "github.com/owner/repo/sub/dir@v1.0.0"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	if got, want := r.CloneURL(), "https://github.com/owner/repo.git"; got != want {
		t.Errorf("CloneURL = %q, want %q", got, want)
	}
}

func TestParseReferenceRoundTrip(t *testing.T) {
	in := "github.com/mooncake-modules/postgres@v1.3.0"
	r, err := ParseReference(in)
	if err != nil {
		t.Fatal(err)
	}
	if r.String() != in {
		t.Errorf("round-trip: got %q, want %q", r.String(), in)
	}
}
