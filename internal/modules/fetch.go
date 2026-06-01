package modules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// scrubGitEnv returns env with all GIT_* vars removed. Needed when shelling
// out to `git` from a process that may itself have been launched by a git
// hook — in which case GIT_DIR / GIT_WORK_TREE would otherwise redirect the
// subprocess at the parent repo.
func scrubGitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// DefaultCacheRoot is ~/.cache/mooncake/modules.
//
// Resolved lazily because $HOME may be unset (tests) or differ from the user
// who started the process (sudo-driven applies).
func DefaultCacheRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir for module cache: %w", err)
	}
	return filepath.Join(home, ".cache", "mooncake", "modules"), nil
}

// Fetcher manages the on-disk module cache. The zero value uses the default
// cache root and the system `git` binary.
//
// Concurrent fetches of the same reference race for the lock-free
// "rename-into-place" cache: the loser sees a populated cache directory and
// short-circuits.
type Fetcher struct {
	// Root is the cache root directory. If empty, DefaultCacheRoot() is used.
	Root string

	// Git is the git binary name or path. If empty, "git" is used.
	Git string

	// CloneURL overrides the URL used for `git clone`. If nil,
	// Reference.CloneURL() is used. Set in tests to point at a file:// repo.
	CloneURL func(Reference) string

	// InsecureHosts lists hosts (host or host:port, matched exactly against
	// Reference.Host) that may be cloned over plain http instead of the
	// https default. Use only for trusted/local hosts — e.g. a self-hosted
	// moongit on "127.0.0.1:8080". The MOONCAKE_MODULE_INSECURE env var
	// (comma-separated) is merged in on top of this list.
	InsecureHosts []string
}

// insecureHost reports whether host is trusted for plain-http clone, per the
// Fetcher's InsecureHosts plus the MOONCAKE_MODULE_INSECURE env var.
func (f *Fetcher) insecureHost(host string) bool {
	if host == "" {
		return false
	}
	for _, h := range f.InsecureHosts {
		if strings.TrimSpace(h) == host {
			return true
		}
	}
	for _, h := range strings.Split(os.Getenv("MOONCAKE_MODULE_INSECURE"), ",") {
		if strings.TrimSpace(h) == host {
			return true
		}
	}
	return false
}

// cloneURLFor returns the git clone URL for ref. The CloneURL override (tests)
// wins; otherwise the scheme is http for an operator-trusted insecure host and
// https everywhere else.
func (f *Fetcher) cloneURLFor(ref Reference) string {
	if f.CloneURL != nil {
		return f.CloneURL(ref)
	}
	if f.insecureHost(ref.Host) {
		return ref.CloneURLWithScheme("http")
	}
	return ref.CloneURL()
}

// CacheDir returns the absolute cache directory for a module reference. The
// directory may or may not exist.
func (f *Fetcher) CacheDir(ref Reference) (string, error) {
	root := f.Root
	if root == "" {
		r, err := DefaultCacheRoot()
		if err != nil {
			return "", err
		}
		root = r
	}
	// "@" in a path component is valid on all supported filesystems but the
	// Reference.String() embedding preserves it for human readability.
	return filepath.Join(root, ref.Host, ref.Owner, ref.Repo+"@"+ref.Version), nil
}

// FetchCached returns the cache directory for ref only if it is already
// present locally; it never clones. Returns an error if the module is not
// cached. Used by read-only callers (e.g. the `mooncake task` listing) that
// must stay offline.
func (f *Fetcher) FetchCached(_ context.Context, ref Reference) (string, error) {
	dir, err := f.CacheDir(ref)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir, nil
	}
	return "", fmt.Errorf("module %s not in local cache", ref.String())
}

// Fetch ensures the module identified by ref is present in the cache and
// returns the absolute directory. A cache hit skips the clone entirely.
func (f *Fetcher) Fetch(ctx context.Context, ref Reference) (string, error) {
	dir, err := f.CacheDir(ref)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat cache dir %s: %w", dir, err)
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", fmt.Errorf("create cache parent: %w", err)
	}

	// Clone into a sibling .tmp.<pid> directory then atomically rename into
	// place. A concurrent fetcher may win the rename race; the loser falls
	// through to the cache-hit path on retry.
	tmp, err := os.MkdirTemp(filepath.Dir(dir), filepath.Base(dir)+".tmp.*")
	if err != nil {
		return "", fmt.Errorf("create tmp clone dir: %w", err)
	}
	cleanup := tmp
	defer func() {
		if cleanup != "" {
			_ = os.RemoveAll(cleanup)
		}
	}()

	if err := f.cloneAndCheckout(ctx, ref, tmp); err != nil {
		return "", err
	}

	if err := os.Rename(tmp, dir); err != nil {
		// If the rename failed because another fetcher won the race, treat
		// the existing directory as a cache hit. Any other rename error is
		// fatal — we'd be returning a stale or empty directory otherwise.
		if info, statErr := os.Stat(dir); statErr == nil && info.IsDir() {
			return dir, nil
		}
		return "", fmt.Errorf("rename tmp clone into cache: %w", err)
	}
	cleanup = "" // rename succeeded; don't delete the now-renamed dir
	return dir, nil
}

// cloneAndCheckout runs the git clone + tag checkout into dst.
func (f *Fetcher) cloneAndCheckout(ctx context.Context, ref Reference, dst string) error {
	cloneURL := f.cloneURLFor(ref)
	git := f.Git
	if git == "" {
		git = "git"
	}

	// Shallow-clone the single ref to keep the cache small and network IO low.
	// Scrub GIT_* env vars so a parent invocation that already has GIT_DIR set
	// (e.g. mooncake invoked from a git hook) doesn't redirect the clone into
	// the caller's repo.
	cmd := exec.CommandContext(ctx, git, //nolint:gosec // git binary + args are controlled
		"clone", "--depth", "1", "--branch", ref.Version, cloneURL, dst)
	cmd.Env = scrubGitEnv(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Disambiguate "tag missing" from "network/auth failure" using the
		// git error text. The exact phrasing varies between Git versions, so
		// match the canonical fragments.
		text := string(out)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if containsAny(text, "Remote branch", "not found", "did not match any", "fatal: Remote branch") {
				return fmt.Errorf("no tag %s in %s", ref.Version, ref.Host+"/"+ref.Owner+"/"+ref.Repo)
			}
		}
		return fmt.Errorf("module not cached and fetch failed: %w: %s", err, text)
	}
	return nil
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub == "" {
			continue
		}
		if indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

// indexOf is a tiny local helper to avoid importing strings just for Contains.
func indexOf(s, sub string) int {
	if len(sub) > len(s) {
		return -1
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
