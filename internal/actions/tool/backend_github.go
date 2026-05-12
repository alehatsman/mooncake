package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
)

func init() {
	register(&githubReleaseBackend{})
}

// githubReleaseBackend is sugar over archive-url. Given a repo and an
// asset filename, it resolves the canonical GitHub release-asset URL
// and hands a Plan to the shared install pipeline.
//
// Asset URL is constructed as:
//
//	https://github.com/<repo>/releases/download/<tag>/<asset>
//
// Defaults: `tag` is "v{{ version }}". Override with `tag:` if the
// project doesn't use the `v` prefix.
type githubReleaseBackend struct{}

func (githubReleaseBackend) Name() string { return BackendGitHubRelease }

func (githubReleaseBackend) Validate(t *config.Tool) error {
	if t.Repo == "" {
		return fmt.Errorf("github-release backend: repo is required (e.g. \"hashicorp/terraform\")")
	}
	if !strings.Contains(t.Repo, "/") || strings.Count(t.Repo, "/") != 1 {
		return fmt.Errorf("github-release backend: repo must be \"owner/name\" (got %q)", t.Repo)
	}
	if t.Asset == "" {
		return fmt.Errorf("github-release backend: asset is required (the release asset filename, e.g. \"terraform_{{ version }}_{{ os }}_{{ arch }}.zip\")")
	}
	// Forbid archive-url/mise fields under github-release.
	if t.URL != "" {
		return fmt.Errorf("github-release backend: url is an archive-url field; remove it (the URL is built from repo+asset)")
	}
	if t.MiseTool != "" || len(t.Env) > 0 {
		return fmt.Errorf("github-release backend: mise_tool/env are mise fields")
	}
	return nil
}

func (githubReleaseBackend) Plan(_ context.Context, spec Spec, facts FactSnapshot) (Plan, error) {
	url := githubAssetURL(spec.Repo, spec.Tag, spec.Asset, spec.Version, facts)
	return Plan{
		URL:               url,
		Checksum:          spec.InlineChecksum,
		StripComponents:   spec.StripComponents,
		BinRel:            spec.Bin,
		UseSharedPipeline: true,
	}, nil
}

// Install is a no-op; the shared pipeline does the work.
func (githubReleaseBackend) Install(_ context.Context, _ Spec, _ Plan, _ string) error {
	return nil
}

func (githubReleaseBackend) Locate(_ context.Context, spec Spec, installDir string) (string, error) {
	return locateInInstallDir(spec.Bin, installDir)
}

// githubAssetURL constructs the canonical release download URL.
// tag defaults to "v{{ version }}"; when set, it's templated against
// the version too so users can opt into tag schemes like "release-1.13.0".
// asset is templated against version + os + arch.
func githubAssetURL(repo, tag, asset, version string, facts FactSnapshot) string {
	if tag == "" {
		tag = "v" + version
	} else {
		tag = renderURL(tag, version, facts)
	}
	asset = renderURL(asset, version, facts)
	return "https://github.com/" + repo + "/releases/download/" + tag + "/" + asset
}
