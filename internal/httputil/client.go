// Package httputil is the project's outbound HTTP boundary.
//
// All packages that fetch over HTTP must route through this package
// rather than reaching for net/http directly. Direct net/http use in
// handler / preset / cmd code is a layering violation. The point is
// not to hide net/http (the types are still net/http types) but to
// ensure every outbound call has:
//
//   - A bounded dial / TLS-handshake / response-header timeout, so a
//     stuck remote can't block an apply indefinitely (F012).
//   - A context plumbed through to .Do(), so step-level deadlines and
//     daemon-side ctx cancellation (F016) actually reach the socket.
//   - A consistent User-Agent so log analysis on the receiving end
//     can attribute traffic to mooncake.
//
// The default Client deliberately has no *overall* Client.Timeout:
// download / unarchive flows can transfer multi-gigabyte payloads
// over slow links, and a global timeout would yank them mid-stream.
// Per-step deadlines belong on ctx; long downloads simply receive a
// long ctx.
//
// This package is intentionally small. Helpers are added when there
// are at least two callers; nothing speculative.
package httputil

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// User-Agent emitted on every outbound request. Set via cmd/-level
// init once the build version is known; otherwise the default is
// useful for log analysis.
var userAgent = "mooncake"

// SetUserAgent overrides the default UA. Safe to call once at startup
// (e.g. from cmd/ once `version` is known). Not goroutine-safe — call
// before any other httputil call.
func SetUserAgent(ua string) {
	if ua != "" {
		userAgent = ua
	}
}

// UserAgent returns the configured User-Agent string.
func UserAgent() string { return userAgent }

const (
	dialTimeout           = 30 * time.Second
	tlsHandshakeTimeout   = 30 * time.Second
	responseHeaderTimeout = 30 * time.Second
	idleConnTimeout       = 90 * time.Second
)

// DefaultTransport is the shared *http.Transport used by Client. Per
// Go convention the same Transport is shared across all clients in the
// process so connection pools are reused.
var DefaultTransport = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	TLSHandshakeTimeout:   tlsHandshakeTimeout,
	ResponseHeaderTimeout: responseHeaderTimeout,
	IdleConnTimeout:       idleConnTimeout,
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   10,
}

// Client is the project-wide HTTP client. Use it directly for custom
// requests (headers, methods, streaming body), or use Get / Post for
// the common cases.
var Client = &http.Client{
	Transport: DefaultTransport,
}

// NewRequest is a thin wrapper around http.NewRequestWithContext that
// sets the canonical User-Agent. Use this instead of
// http.NewRequest / http.NewRequestWithContext directly.
func NewRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

// Get issues a GET with the project-wide Client and returns the body
// as bytes. Convenience for the many "fetch a small JSON / YAML / text
// from a URL" call sites. The caller's ctx controls cancellation.
//
// Returns an error for non-2xx status. For streaming (large) bodies or
// for status-code-aware logic, use NewRequest + Client.Do directly.
func Get(ctx context.Context, url string) ([]byte, error) {
	req, err := NewRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
