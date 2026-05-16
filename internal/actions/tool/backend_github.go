package tool

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions/tool/store"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/httputil"
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

func (githubReleaseBackend) Plan(ctx context.Context, spec Spec, facts FactSnapshot) (Plan, error) {
	url := resolveGithubAssetURL(ctx, spec.Repo, spec.Tag, spec.Asset, spec.Version, facts)
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
	return store.LocateInInstallDir(spec.Bin, installDir)
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

// resolveGithubAssetURL picks the download URL when `tag` is unset, by
// trying the conventional schemes in order. Most projects ship with
// "v{version}" (terraform, gh, hashicorp, kubernetes); some prefix with
// the binary name ("jq-{version}"), some omit the v ("{version}"). When
// `tag` is set, we trust the user and don't probe. When unset, we HEAD
// each candidate and stop at the first one that responds 2xx/3xx, so
// the common case "just works" without forcing users to learn the
// per-project tag scheme. This is the only network touch in Plan; it's
// cheap (HEAD) and avoids the alternative of always failing with a 404
// that doesn't tell the user how to recover.
func resolveGithubAssetURL(ctx context.Context, repo, tag, asset, version string, facts FactSnapshot) string {
	if tag != "" {
		return githubAssetURL(repo, tag, asset, version, facts)
	}
	candidates := []string{
		"v" + version,
		version,
	}
	// jq-style: the binary name as a prefix of the tag. spec.Repo is
	// "owner/name", so reuse "name" — projects that adopt this scheme
	// (jq, dive, ...) usually align tag prefix with the repo name.
	if idx := strings.LastIndex(repo, "/"); idx > 0 && idx+1 < len(repo) {
		name := repo[idx+1:]
		candidates = append(candidates, name+"-"+version)
	}
	for i, candidate := range candidates {
		url := githubAssetURL(repo, candidate, asset, version, facts)
		if i == len(candidates)-1 {
			// Last fallback — return without HEAD so users get a real
			// download error from the install pipeline rather than two
			// layers of network noise.
			return url
		}
		if urlReachable(ctx, url) {
			return url
		}
	}
	return githubAssetURL(repo, "v"+version, asset, version, facts)
}

// urlReachable does a cheap HEAD to see whether an asset URL would
// actually serve (2xx/3xx). 4xx/5xx → not reachable; any transport
// error → conservatively false so the next candidate gets a chance.
// Caller-visible side effect is one extra HEAD per failed candidate
// (typically zero or one, given the two-element default list).
//
// Indirected through a package-level var so unit tests can inject a
// hermetic stub instead of making real GitHub HEAD requests during
// Plan() — github-release backend is the only place we probe a URL
// outside the install pipeline.
//
// F007: probe now carries a context (so a Plan-mode cancellation
// flows through) and is bounded by a 5-second probe deadline. HEAD
// against GitHub should be sub-second; a slow probe is a stronger
// signal that this candidate is the wrong one and the next one
// (or the unconditional final fallback) should be tried.
var urlReachable = func(ctx context.Context, url string) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// F012: route through httputil so the transport-level dial / TLS /
	// response-headers timeouts apply on top of the 5s deadline above
	// (DefaultClient pre-fix had no transport limits — a slow TLS
	// handshake against a misbehaving CDN burned the full 5s before
	// any cancellation took effect).
	req, err := httputil.NewRequest(probeCtx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := httputil.Client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
