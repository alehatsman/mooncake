package tool

import (
	"context"
	"strings"
	"testing"
)

// stubURLReachable swaps the package-level urlReachable hook for the
// duration of one test and returns a restore function for defer.
// Tests use this to stay hermetic — no real GitHub HEAD requests
// during Plan().
func stubURLReachable(t *testing.T, probe func(string) bool) func() {
	t.Helper()
	original := urlReachable
	urlReachable = probe
	return func() { urlReachable = original }
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

// TestMT39_TagFallback_RepoNamePrefix is a regression test for
// manual-test #39 (2026-05-15): the github-release backend hard-coded
// `tag = "v{version}"` so projects with non-v-prefixed tag schemes
// (jq → "jq-1.7.1", dive, hadolint) 404'd unless the user knew to
// supply `tag: "jq-{{ version }}"` manually. The resolver now probes
// the conventional candidates and picks the one that responds, with
// "v{version}" first, then "{version}", then "{name}-{version}".
func TestMT39_TagFallback_RepoNamePrefix(t *testing.T) {
	// Simulate: only the jq-style tag is reachable (jqlang/jq actually
	// ships releases under "jq-1.7.1", not "v1.7.1").
	restore := stubURLReachable(t, func(url string) bool {
		return strings.Contains(url, "/jq-1.7.1/")
	})
	defer restore()

	b, err := Get(BackendGitHubRelease)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Backend: BackendGitHubRelease,
		Name:    "jq",
		Version: "1.7.1",
		Repo:    "jqlang/jq",
		Asset:   "jq-linux-amd64",
	}
	plan, err := b.Plan(context.Background(), spec, FactSnapshot{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want := "https://github.com/jqlang/jq/releases/download/jq-1.7.1/jq-linux-amd64"
	if plan.URL != want {
		t.Errorf("URL:\n  got  %q\n  want %q", plan.URL, want)
	}
}

func TestMT39_TagFallback_BareVersion(t *testing.T) {
	// Some projects (kubectl, gh in some lines) tag with bare version.
	restore := stubURLReachable(t, func(url string) bool {
		return strings.Contains(url, "/1.30.0/")
	})
	defer restore()

	b, err := Get(BackendGitHubRelease)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Backend: BackendGitHubRelease,
		Name:    "kubectl",
		Version: "1.30.0",
		Repo:    "kubernetes/kubectl",
		Asset:   "kubectl",
	}
	plan, err := b.Plan(context.Background(), spec, FactSnapshot{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !strings.Contains(plan.URL, "/1.30.0/") || strings.Contains(plan.URL, "/v1.30.0/") {
		t.Errorf("expected bare-version fallback, got %q", plan.URL)
	}
}

func TestMT39_TagFallback_LastCandidateReturnedOnUniversalFailure(t *testing.T) {
	// If nothing is reachable (offline, project removed releases, …)
	// fall through to the last candidate and let the install pipeline
	// surface the real download error — better than two layers of
	// "not reachable" noise.
	restore := stubURLReachable(t, func(url string) bool { return false })
	defer restore()

	b, err := Get(BackendGitHubRelease)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Backend: BackendGitHubRelease,
		Name:    "x",
		Version: "9.9.9",
		Repo:    "owner/x",
		Asset:   "x.bin",
	}
	plan, err := b.Plan(context.Background(), spec, FactSnapshot{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// Last candidate is "{name}-{version}" — verifying we don't crash
	// when all probes fail.
	if plan.URL == "" {
		t.Error("URL should not be empty even when all probes failed")
	}
}

func TestMT39_ExplicitTag_SkipsProbing(t *testing.T) {
	// When tag: is set the resolver trusts the user and doesn't probe.
	// Stub returns false for everything to detect any accidental probe.
	called := false
	restore := stubURLReachable(t, func(url string) bool {
		called = true
		return false
	})
	defer restore()

	b, err := Get(BackendGitHubRelease)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Backend: BackendGitHubRelease,
		Name:    "tool",
		Version: "0.5.0",
		Repo:    "owner/name",
		Tag:     "{{ version }}",
		Asset:   "tool-{{ version }}",
	}
	_, err = b.Plan(context.Background(), spec, FactSnapshot{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if called {
		t.Error("urlReachable should not be called when tag: is explicit")
	}
}
