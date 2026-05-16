---
id: F007
title: tool/fetch.go and backend_github.go use http.Get / http.DefaultClient with no timeout, no context
severity: risk
package: internal/actions/tool
files:
  - internal/actions/tool/fetch.go (lines 17-44)
  - internal/actions/tool/backend_github.go (lines 134-145)
status: done
fixed: 2026-05-16 — initial fix-F007 merge at 34fcdfda; subsequently consolidated into the shared httputil package via `fa2d8f50 fix(httputil): F012 — bounded transport + canonical UA for every outbound HTTP call`. tool/fetch.go:40,46 and backend_github.go:150,154 now route through `httputil.NewRequest(ctx, ...) + httputil.Client.Do(req)`.
verified: 2026-05-16 — fetch.go uses NewRequestWithContext; handler.go wraps each call in 30-min ctx; backend_github.go probes with 5s timeout; tool tests green. Re-checked 2026-05-17 @ 099ee336 — both files still on the bounded httputil path.
---

## ✅ Fixed

Three changes across the tool package's HTTP plumbing:

1. **`fetchToTempFile` accepts a context.** Replaced `http.Get(url)`
   with `http.NewRequestWithContext` against an explicit
   `&http.Client{}` (zero overall timeout — context drives
   cancellation). Sets a `User-Agent: mooncake-tool` header so the
   GitHub release CDN doesn't lump us with anonymous traffic.

2. **`urlReachable` accepts a context and bounds itself.** The HEAD
   probe inside `resolveGithubAssetURL`'s candidate-tag loop now
   wraps its parent context with `context.WithTimeout(ctx, 5s)`.
   HEAD against GitHub should be sub-second; 5s is generous enough
   that a healthy-but-slow probe answers, short enough that a stuck
   one yields to the next candidate. Test seam preserved:
   `urlReachable` stays a package-level `var` and the new signature
   is `func(context.Context, string) bool`. Stub helper
   `stubURLReachable` updated to match.

3. **`Handler.Execute` defines an outer 30-minute install ceiling.**
   `actions.Context` doesn't expose a Go `context.Context` today
   (the executor's parent ctx isn't plumbed through the handler
   ABI), so the pragmatic alternative is a sane outer bound at the
   tool package's seam: `context.WithTimeout(context.Background(),
   30*time.Minute)`. Large tool archives (LLVM, CUDA SDK) need the
   headroom; everything else completes far under. This ctx flows
   into `Backend.Plan`, `Backend.Install`, `Backend.Locate`, and
   `InstallURL → fetchToTempFile`. When the executor ABI grows a
   ctx getter the 30-min default can be replaced by the parent.

### Existing test fixtures

`backend_github_mt39_test.go` (5 stubURLReachable calls) and
`backend_github_test.go` (1) updated to the new
`func(context.Context, string) bool` shape. All tool tests pass
under -race.

### Open follow-ups

- `internal/actions/download` does similar HTTP fetching and is
  noted in the finding's references as needing the same audit. Not
  bundled here.
- A read-deadline / `http.MaxBytesReader` layer on the download
  body would catch true mid-download stalls (server sends bytes at
  1 KB/s) but is a deeper change; the context-cancellation path
  covers the most common failure modes already.

---

## What

Two network calls in the tool package use Go's default HTTP plumbing
without timeouts or context cancellation:

**1. `fetchToTempFile`** (`fetch.go:17`):

```go
// #nosec G107 -- URL comes from user-declared mooncake config
resp, err := http.Get(url)
```

- `http.Get` uses `http.DefaultClient`, which has **no timeout**.
  A server that opens the connection and then sends nothing
  (slowloris) will hang the apply forever.
- The parent `InstallURL` (`install.go:31`) accepts a `context.Context`
  but discards it (`_ context.Context`), so even Ctrl-C / step
  timeout / parent cancellation can't cancel the download.
- No `User-Agent` header. GitHub does not block missing UA today
  but does rate-limit "anonymous" clients more aggressively on
  release downloads.

**2. `urlReachable`** (`backend_github.go:134`):

```go
var urlReachable = func(url string) bool {
    req, err := http.NewRequest(http.MethodHead, url, nil)
    // ...
    resp, err := http.DefaultClient.Do(req)
```

- `http.NewRequest` (not `NewRequestWithContext`) — no way to
  cancel.
- `http.DefaultClient` — no timeout. Called in a probe loop
  (`resolveGithubAssetURL`) so a single slow tag probe can stall
  Plan-mode for the whole apply.

## Why it's a `risk` and not just a style nit

Both call sites are on the **happy path of `tool` apply** — every
github-release / archive-url tool install goes through them.
Mooncake runs as a provisioning agent. The realistic failure modes
are:

- A user's CI box loses upstream connectivity mid-download; the
  whole pipeline hangs instead of failing fast.
- A laptop on flaky wifi runs `mooncake apply`; the user can't
  even Ctrl-C cleanly because the context isn't wired through.
- GitHub's CDN returns 200 but sends bytes at 1 KB/s due to
  congestion; the apply blocks indefinitely.

The fleet daemon does have step-level timeouts at the apply layer,
but they don't propagate to `http.Get`. The download just outlives
the step's "logical" timeout.

## Suggested fix

**1. `fetchToTempFile` — accept and respect a context, set timeouts:**

```go
// fetchToTempFile downloads url into a temp file (created via
// os.CreateTemp in dir) and returns the temp file path. The caller
// is responsible for os.Remove on the returned path.
func fetchToTempFile(ctx context.Context, url, dir string) (string, error) {
    tmp, err := os.CreateTemp(dir, "mooncake-tool-*"+archiveSuffix(url))
    if err != nil {
        return "", fmt.Errorf("create temp file: %w", err)
    }
    tmpName := tmp.Name()
    defer func() { _ = tmp.Close() }()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        _ = os.Remove(tmpName)
        return "", err
    }
    req.Header.Set("User-Agent", "mooncake/"+version.String())

    client := &http.Client{Timeout: 0} // no overall timeout; context drives cancel
    resp, err := client.Do(req)
    if err != nil {
        _ = os.Remove(tmpName)
        return "", fmt.Errorf("http GET %s: %w", url, err)
    }
    defer func() { _ = resp.Body.Close() }()

    // ... (rest unchanged)
}
```

Then in `InstallURL` (`install.go:31`), replace the leading `_` with
`ctx` and pass it through.

**Why context-only and not a wall-clock timeout?** Large tool
archives (LLVM, CUDA SDK, kubernetes) are 100s of MB. A wall-clock
timeout that's safe for these is too generous for normal downloads
(jq is 200 KB). Context cancellation lets the caller decide. The
fleet step-timeout already provides a sane outer bound.

A **read deadline** in addition (e.g.
`resp.Body = http.MaxBytesReader(...)` or a TCP-level read
deadline) is the right next layer if true stall-detection is
wanted, but that's a deeper change.

**2. `urlReachable` — use context + bounded probe timeout:**

```go
var urlReachable = func(ctx context.Context, url string) bool {
    probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    req, err := http.NewRequestWithContext(probeCtx, http.MethodHead, url, nil)
    if err != nil {
        return false
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return false
    }
    _ = resp.Body.Close()
    return resp.StatusCode >= 200 && resp.StatusCode < 400
}
```

The 5-second probe timeout is appropriate here because a HEAD
should be sub-second to GitHub, and the probe is only used as a
disambiguator — falling through to the next candidate (or the
unconditional "real download error" path) is the right move when
the probe is slow.

The test seam (`var urlReachable = func`) survives the change.

## Verification

- `go test ./internal/actions/tool/...` — the
  `backend_github_mt39_test.go` already injects a stub
  `urlReachable`, so the new signature flows through with a
  small test adjustment.
- Manual: run `mooncake apply` with a tool spec against a slow
  endpoint (`tc qdisc add dev eth0 root netem delay 1000ms` or
  `nc -l 8080`), confirm Ctrl-C now cancels cleanly.

## Open question

`fetchToTempFile` is called from `InstallURL` whose first
parameter is `_ context.Context`. Plumbing context all the way
from the apply step is the harder lift than just adding it to
`fetchToTempFile`. Decide whether the context flows from
`Handler.Execute → Backend.Plan/Install → InstallURL → fetch`,
or whether `tool` package defines its own outer-bound context
(e.g. 30 min default).

## References

- `urlReachable` test seam: `internal/actions/tool/backend_github_mt39_test.go`.
- `internal/actions/download/handler.go` — separate package that
  also does HTTP fetching; audit for the same gap.
