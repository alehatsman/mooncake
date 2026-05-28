// Package shared holds the type definitions and helper functions
// used by all per-driver backends of pkg.repo (apt, dnf, brew).
// Lives in its own subpackage so per-driver backends can import it
// without forming an import cycle with the parent `pkg_repo` package
// (which dispatches at runtime via the active driver block).
//
// See `internal/actions/pkg_repo/handler.go` for the dispatcher and
// the public Handler type.
package shared

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/alehatsman/mooncake/internal/httputil"
)

// State vocabulary. Exported so per-driver backends can branch on
// them without redeclaring the string literals.
const (
	StatePresent = "present"
	StateAbsent  = "absent"

	// AtomicTempSuffix is appended to the destination path before
	// a write-and-rename completes. Exported so per-driver tests
	// can recognise / clean up half-finished writes if needed.
	AtomicTempSuffix = ".mooncake-tmp"
)

// NameRE constrains the repo name. Used as the on-disk filename
// stem, so we reject anything that wouldn't be a safe path segment.
var NameRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// PkgRepoReverseInfo is the per-step apply-time snapshot pkg.repo
// stashes on Result.ReverseData. Captures the sources/repo file
// path, keyring path (info-only — see Reverse comment in the parent
// package), and the prior content + existence.
//
// Scope: pkg.repo touches up to TWO files (sources/repo entry +
// keyring), but the Reverser interface returns ONE step. Reverse
// restores the sources file only; the keyring on disk is left as-is.
// A keyring file with no sources reference is benign (apt/dnf simply
// ignore it); operators who need full cleanup can chain a file.write
// absent step against KeyringPath inside a try/catch/finally.
type PkgRepoReverseInfo struct {
	// Name is the repo identifier (used for diagnostic messages).
	Name string

	// SourcesPath is the canonical sources file path the apply
	// targeted (`/etc/apt/sources.list.d/<name>.sources` for apt,
	// `/etc/yum.repos.d/<name>.repo` for dnf).
	SourcesPath string

	// KeyringPath is the canonical keyring file path
	// (`/etc/apt/keyrings/<name>.gpg` for apt,
	// `/etc/pki/rpm-gpg/RPM-GPG-KEY-<name>` for dnf). Captured for
	// visibility; not reverted (see scope note in the struct doc).
	KeyringPath string

	// PriorExisted reports whether the sources file existed
	// pre-apply.
	PriorExisted bool

	// PriorContent is the verbatim file bytes pre-apply. Empty
	// when PriorExisted is false.
	PriorContent string
}

// NormalizeState lower-cases the state and defaults empty → present.
func NormalizeState(s string) string {
	if s == "" {
		return StatePresent
	}
	return strings.ToLower(s)
}

// PathExists reports whether a path exists. Errors other than
// fs.ErrNotExist surface to the caller.
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", path, err)
}

// ReadFile returns (content, existed, err). Missing files yield
// ("", false, nil) so callers don't need to special-case fs.ErrNotExist.
func ReadFile(path string) (string, bool, error) {
	// #nosec G304 -- path is constructed from validated name + fixed dirs.
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), true, nil
}

// WriteAtomic writes content to a temp sibling then renames into
// place. The rename is atomic on POSIX; if it fails the temp file
// is removed.
func WriteAtomic(path string, content []byte, mode os.FileMode) error {
	tmp := path + AtomicTempSuffix
	if err := os.WriteFile(tmp, content, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// HTTPFetchKey is the package-level hook for fetching a GPG key body
// over HTTP. Wired to HTTPFetchKeyDefault in production; tests
// override with a fake fetcher to avoid network I/O. F2: ctx is the
// run-wide cancel.
var HTTPFetchKey = HTTPFetchKeyDefault

// HTTPFetchKeyDefault routes through httputil for bounded dial / TLS
// / response-headers timeouts (F012). Empty body and non-200 status
// surface as errors. The parent ctx propagates run-wide cancellation
// so SIGINT / fleet kill / MCP shutdown aborts an in-flight keyring
// fetch promptly (F2).
func HTTPFetchKeyDefault(ctx context.Context, url string) ([]byte, error) {
	req, err := httputil.NewRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	// #nosec G107 -- URL comes from user-supplied YAML.
	resp, err := httputil.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("empty key body")
	}
	return body, nil
}
