---
id: F012
title: Cross-cutting — 9 packages use http.Get / DefaultClient with no timeout, no context
severity: risk
package: cross-cutting
files:
  - internal/actions/observe_http/handler.go:132 (NewRequest)
  - internal/actions/download/handler.go:352 (NewRequest)
  - internal/presets/registry/source.go:63 (http.Get)
  - internal/actions/tool/fetch.go:27 (http.Get) — see F007
  - internal/actions/assert/handler.go:506 (NewRequest)
  - internal/actions/tool/backend_github.go:135,139 (NewRequest, DefaultClient.Do) — see F007
  - internal/actions/pkg_repo/handler.go:484 (http.Get)
  - internal/presets/registry/remote.go:155,236,270 (http.Get, NewRequest, http.Get)
status: open
---

## What

`grep -rln 'http\.Get\|http\.DefaultClient\.\|http\.NewRequest[^W]' internal/`
returns 9 non-test files. Every hit uses some combination of:

- `http.Get(url)` — no client config, no context.
- `http.DefaultClient.Do(req)` — no timeout set.
- `http.NewRequest(...)` — the non-context version (deprecated
  since Go 1.13 in favor of `NewRequestWithContext`).

None of these has a timeout. None plumbs a context. None can be
cancelled by step-level deadlines.

## Per-file notes

| File | Line | Call | Caller context |
|---|---|---|---|
| `observe_http/handler.go` | 132 | `http.NewRequest` | HTTP probe in the `observe.http` action |
| `download/handler.go` | 352 | `http.NewRequest` | Generic URL download — same risk as `tool/fetch.go` |
| `presets/registry/source.go` | 63 | `http.Get` | Loading a remote preset source.yml |
| `tool/fetch.go` | 27 | `http.Get` | Tool archive download. See F007 for the detailed write-up. |
| `assert/handler.go` | 506 | `http.NewRequest` | HTTP-asserting in `assert` step type |
| `tool/backend_github.go` | 135, 139 | `http.NewRequest` + `DefaultClient.Do` | GitHub tag probe. F007 covers this. |
| `pkg_repo/handler.go` | 484 | `http.Get` | Adding a repo from a URL (probably for key validation) |
| `presets/registry/remote.go` | 155, 236, 270 | 2×`http.Get` + 1× `http.NewRequest` | Remote preset registry catalog/HEAD/raw fetch |

## Why it's worth a cross-cutting finding

- **Same root cause everywhere.** No per-file fix scales — the
  policy "all outbound HTTP must use a configured client" needs
  to be set in one place and enforced.
- **Each gap is a real production hazard.** Mooncake runs as a
  CI / provisioning tool. Hung HTTP calls block the apply,
  consume slot time, and ignore Ctrl-C. The kernel-vision doc
  (`docs-working/vision/kernel.md`) is explicit that the kernel
  must be **deterministic and cancellable**; today, every
  network-touching handler defeats that property.
- **The fix is small *per call site* but only useful as a set.**
  Fixing `tool/fetch.go` alone (F007) helps tool installs but
  not preset loading.

## Suggested fix

**Stage 1 — define the canonical HTTP client in `internal/utils`
or `internal/httputil` (new package):**

```go
// internal/httputil/client.go (new)
package httputil

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "time"
)

// Client is the project-wide outbound HTTP client. All packages
// that fetch over HTTP must use it. Direct net/http calls in
// handler code are a layering violation.
//
// The default client has:
//   - No total timeout (long downloads must be possible)
//   - 30 s dial timeout (don't hang on DNS / TCP setup)
//   - 30 s TLS handshake timeout
//   - 30 s response-headers timeout
//
// Caller is responsible for passing a context.Context with a
// deadline if a wall-clock cap is needed.
var Client = &http.Client{
    Transport: &http.Transport{
        DialContext:           (&net.Dialer{Timeout: 30*time.Second}).DialContext,
        TLSHandshakeTimeout:   30*time.Second,
        ResponseHeaderTimeout: 30*time.Second,
    },
}

// Get issues a GET with the project-wide client, returning the
// body as bytes. Convenience for the many "read a small JSON /
// YAML from a URL" use sites.
func Get(ctx context.Context, url string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("User-Agent", UserAgent())
    resp, err := Client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("GET %s: %w", url, err)
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
    }
    return io.ReadAll(resp.Body)
}
```

**Stage 2 — migrate the 9 hits.** Most are a 3-line change:

```diff
- resp, err := http.Get(url)
+ resp, err := httputil.Client.Do(req)
+ // (build req with NewRequestWithContext)
```

`tool/fetch.go` and `download/handler.go` stream the response
body to disk — they use `Client.Do` directly so they can `io.Copy`
without buffering. `presets/registry/source.go` reads a YAML and
can use `httputil.Get`.

**Stage 3 — optional, post-migration:** `grep`-based lint rule
in `.golangci.yml` (or a custom analyzer) that flags any
`net/http` import outside `internal/httputil`. Prevents future
regression.

## What about request bodies / POST?

`assert/handler.go:506` may issue POSTs. The helper needs a
`Post`-style variant accepting an `io.Reader`. Pattern:

```go
func Post(ctx context.Context, url, contentType string, body io.Reader) ([]byte, error)
```

Three callers max; doesn't need to be over-engineered.

## What's NOT in scope

- The `agentd` HTTP **server** is a separate concern (server-side
  timeouts are already set per `http.Server.ReadTimeout` etc. in
  `internal/agentd/server.go`). This finding is outbound only.
- The `transport` package (`internal/fleet/transport`) uses
  `net/http` for the fleet protocol — but it builds its own
  Client with proper config. Audit before assuming it's safe,
  but probably out of scope.

## Verification

- After Stage 2: `grep -rn 'http\.Get\|http\.NewRequest[^W]\|http\.DefaultClient' internal/`
  → only hits in `internal/httputil/` and tests.
- `go test ./...` — same pass set.
- Manual: run mooncake against a slow endpoint
  (`tc qdisc add dev lo root netem delay 5000ms`), confirm Ctrl-C
  cancels and connections time out.

## References

- F007 — tool-package-scoped write-up of the same risk, with
  detailed call-site context.
- `docs-working/vision/kernel.md` — kernel cancellation requirement.
