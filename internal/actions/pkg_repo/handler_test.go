//nolint:revive // package name follows action convention
package pkg_repo

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestRun_ImplementsRunner pins the actions.Runner conformance — the
// parent Handler must satisfy actions.Runner since `internal/register`
// imports it for that contract.
func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

// TestValidate covers the cross-cutting validation rules the parent
// Handler enforces before dispatching to a driver. Per-driver
// validation lives in the sub-package tests (see apt/run_test.go,
// dnf/dnf_test.go, brew/brew_test.go).
func TestValidate(t *testing.T) {
	boolp := func(b bool) *bool { return &b }
	apt := func(url string, fp string, check *bool) *config.PkgRepoApt {
		return &config.PkgRepoApt{
			URI:               "https://example.com/repo",
			Suites:            []string{"stable"},
			GPGKeyURL:         url,
			GPGKeyFingerprint: fp,
			GPGCheck:          check,
		}
	}
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil", &config.Step{}, true},
		{"no name", &config.Step{PkgRepo: &config.PkgRepo{Apt: apt("", "", nil)}}, true},
		{"bad name", &config.Step{PkgRepo: &config.PkgRepo{Name: "spaces and/slashes", Apt: apt("", "", nil)}}, true},
		{"bad state", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", State: "maybe", Apt: apt("", "", nil)}}, true},
		{"no blocks", &config.Step{PkgRepo: &config.PkgRepo{Name: "x"}}, true},
		{"multiple blocks", &config.Step{PkgRepo: &config.PkgRepo{
			Name: "x",
			Apt:  apt("", "", nil),
			Dnf:  &config.PkgRepoDnf{BaseURL: "u"},
		}}, true},
		{"apt no uri", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", Apt: &config.PkgRepoApt{Suites: []string{"s"}}}}, true},
		{"apt no suites", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", Apt: &config.PkgRepoApt{URI: "u"}}}, true},
		{"gpg check default needs fingerprint", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", Apt: apt("https://k", "", nil)}}, true},
		{"gpg check off ok without fingerprint", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", Apt: apt("https://k", "", boolp(false))}}, false},
		{"ok apt", &config.Step{PkgRepo: &config.PkgRepo{Name: "nodesource", Apt: apt("", "", nil)}}, false},
		{"ok absent skips apt fields", &config.Step{PkgRepo: &config.PkgRepo{Name: "nodesource", State: "absent", Apt: &config.PkgRepoApt{}}}, false},

		// Dnf validation
		{"dnf no baseurl/metalink/mirrorlist", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", Dnf: &config.PkgRepoDnf{}}}, true},
		{"dnf baseurl + metalink mutually exclusive", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", Dnf: &config.PkgRepoDnf{
			BaseURL:  "https://example.com",
			Metalink: "https://example.com/metalink",
		}}}, true},
		{"dnf gpg check default needs fingerprint", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", Dnf: &config.PkgRepoDnf{
			BaseURL:   "https://example.com",
			GPGKeyURL: "https://example.com/key",
		}}}, true},
		{"dnf gpg check off ok without fingerprint", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", Dnf: &config.PkgRepoDnf{
			BaseURL:   "https://example.com",
			GPGKeyURL: "https://example.com/key",
			GPGCheck:  boolp(false),
		}}}, false},
		{"dnf ok baseurl only", &config.Step{PkgRepo: &config.PkgRepo{Name: "epel", Dnf: &config.PkgRepoDnf{
			BaseURL: "https://download.example.com/epel/9/Everything/x86_64/",
		}}}, false},
		{"dnf ok metalink only", &config.Step{PkgRepo: &config.PkgRepo{Name: "fedora", Dnf: &config.PkgRepoDnf{
			Metalink: "https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever&arch=$basearch",
		}}}, false},
		{"dnf ok absent skips source-required check", &config.Step{PkgRepo: &config.PkgRepo{Name: "old", State: "absent", Dnf: &config.PkgRepoDnf{}}}, false},

		// Brew validation
		{"brew present without tap rejected", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", State: "present", Brew: &config.PkgRepoBrew{}}}, true},
		{"brew present with tap accepted", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", State: "present", Brew: &config.PkgRepoBrew{Tap: "foo/bar"}}}, false},
		{"brew absent without tap accepted", &config.Step{PkgRepo: &config.PkgRepo{Name: "x", State: "absent", Brew: &config.PkgRepoBrew{}}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}
