package tool

import (
	"context"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

func TestGitHubReleaseValidate(t *testing.T) {
	b, err := Get(BackendGitHubRelease)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		tool    *config.Tool
		wantErr bool
	}{
		{"ok minimal", &config.Tool{Repo: "owner/name", Asset: "x_{{ version }}_{{ os }}_{{ arch }}.zip"}, false},
		{"missing repo", &config.Tool{Asset: "x.zip"}, true},
		{"malformed repo (no slash)", &config.Tool{Repo: "ownername", Asset: "x.zip"}, true},
		{"malformed repo (too many slashes)", &config.Tool{Repo: "a/b/c", Asset: "x.zip"}, true},
		{"missing asset", &config.Tool{Repo: "owner/name"}, true},
		{"reject url field", &config.Tool{Repo: "owner/name", Asset: "x.zip", URL: "https://example.com"}, true},
		{"reject mise fields", &config.Tool{Repo: "owner/name", Asset: "x.zip", MiseTool: "node"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := b.Validate(tc.tool)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestGitHubReleasePlanDefaultTag(t *testing.T) {
	// Resolver probes candidate URLs over HTTP — stub out the network
	// touch so the test stays hermetic. Return true for the v-prefixed
	// URL (the conventional default) and let the resolver pick it.
	restore := stubURLReachable(t, func(_ context.Context, url string) bool {
		return contains(url, "/v1.13.0/")
	})
	defer restore()

	b, err := Get(BackendGitHubRelease)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Backend: BackendGitHubRelease,
		Name:    "terraform",
		Version: "1.13.0",
		Repo:    "hashicorp/terraform",
		Asset:   "terraform_{{ version }}_{{ os }}_{{ arch }}.zip",
	}
	plan, err := b.Plan(context.Background(), spec, FactSnapshot{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want := "https://github.com/hashicorp/terraform/releases/download/v1.13.0/terraform_1.13.0_linux_amd64.zip"
	if plan.URL != want {
		t.Errorf("URL:\n  got  %q\n  want %q", plan.URL, want)
	}
	if !plan.UseSharedPipeline {
		t.Error("UseSharedPipeline must be true")
	}
}

func TestGitHubReleasePlanCustomTag(t *testing.T) {
	b, err := Get(BackendGitHubRelease)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Backend: BackendGitHubRelease,
		Name:    "tool",
		Version: "0.5.0",
		Repo:    "owner/name",
		// No 'v' prefix; common for projects like gh, kubectl.
		Tag:   "{{ version }}",
		Asset: "tool-{{ version }}-{{ os }}-{{ arch }}.tar.gz",
	}
	plan, err := b.Plan(context.Background(), spec, FactSnapshot{OS: "darwin", Arch: "arm64"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want := "https://github.com/owner/name/releases/download/0.5.0/tool-0.5.0-darwin-arm64.tar.gz"
	if plan.URL != want {
		t.Errorf("URL:\n  got  %q\n  want %q", plan.URL, want)
	}
}

func TestGitHubReleasePlanCustomTagScheme(t *testing.T) {
	b, err := Get(BackendGitHubRelease)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Backend: BackendGitHubRelease,
		Name:    "tool",
		Version: "1.13.0",
		Repo:    "owner/name",
		Tag:     "release-{{ version }}",
		Asset:   "tool.zip",
	}
	plan, err := b.Plan(context.Background(), spec, FactSnapshot{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want := "https://github.com/owner/name/releases/download/release-1.13.0/tool.zip"
	if plan.URL != want {
		t.Errorf("URL:\n  got  %q\n  want %q", plan.URL, want)
	}
}
