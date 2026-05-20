# F050 — preset URL fetch is unbounded; hostile/misconfigured URL exhausts disk

**Filed**: 2026-05-18 by assistant (cold read of `internal/presets/registry/`)
**Severity**: Medium (DoS by a user-typed URL)
**Component**: `internal/presets/registry/source.go:fetchFromURL`
**Status**: **done** (2026-05-18) — fix in this commit caps the streamed body at 4 MiB (matches `read_common.DefaultMaxBytes`) and errors with a clear "exceeds max_bytes" message on overflow.

---

## Summary

`fetchFromURL` (called from `mooncake presets add <url>`) streams the
HTTP response body straight into a target file via `io.Copy(outFile,
resp.Body)`. No `LimitReader`, no size cap. A hostile or
misconfigured server can stream gigabytes (or stream forever) and
exhaust the operator's disk before the download fails on its own.

Same class as F026 (`file/copy` unbounded `ReadFile`) and F041
(`artifact_capture.readFileContent` unbounded read), both already
fixed. The pattern is "user-supplied URL → unchecked stream into the
filesystem", and the established fix is `io.LimitReader` capped at a
sensible size for the payload type.

## Reproduction (synthetic)

```go
// Bring up a server that streams forever.
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
    w.WriteHeader(http.StatusOK)
    buf := bytes.Repeat([]byte("x"), 1<<20)
    for {
        if _, err := w.Write(buf); err != nil {
            return
        }
    }
}))
defer srv.Close()
_, err := FetchSource(srv.URL+"/preset.yml", SourceTypeURL, t.TempDir(), "tmp")
// Pre-fix: never returns; the target file grows without bound.
// Post-fix: returns an error wrapping "exceeds max_bytes=4194304" within ~ms.
```

## Root cause

```go
// internal/presets/registry/source.go:111
if _, err := io.Copy(outFile, resp.Body); err != nil {
    return "", fmt.Errorf("failed to write file: %w", err)
}
```

No upper bound on the stream. The httputil transport (`F012`) bounds
the dial / TLS / response-headers latency, but once the body starts
streaming, this loop runs until the server stops sending.

## Fix

Wrap the body in `io.LimitReader(resp.Body, maxPresetBytes+1)`,
`io.Copy` to the file, then check whether bytes written exceeded
`maxPresetBytes` — if so, remove the partial file and return an
"exceeds max_bytes" error. Default cap matches the 4 MiB used by
`read_common.DefaultMaxBytes` (a preset is a YAML file; if anyone
ever hits 4 MiB they have bigger problems).

## Sites unblocked

- `mooncake presets add <url>` no longer can be weaponized to fill the
  caller's disk.

## Adjacency

Future follow-ups in the same area:

- `fetchFromPath` reads single files via `os.ReadFile` without a size
  cap (line 167). Same DoS shape but inside the operator's own
  filesystem; lower severity.
- `copyDirContents` has no symlink-loop protection. Edge case but
  recurses without a visited-set.
- `fetchFromGit` accepts the user-supplied `gitURL` directly into
  `git clone`. Existing comment claims URL parsing in `Fetch()`
  validates it; that path doesn't exist (the wrapper is
  `FetchSource`), so the claim is stale. The git binary itself
  rejects most argv-injection attempts (filterspec for `--` boundary)
  but a hardening pass would be worth it.
