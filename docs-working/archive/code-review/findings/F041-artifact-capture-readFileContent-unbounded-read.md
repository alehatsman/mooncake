---
id: F041
title: artifact_capture.readFileContent reads the entire file before truncating to maxSize — unbounded RAM for "captured" file content
severity: smell
package: internal/actions/artifact_capture
file: internal/actions/artifact_capture/handler.go
lines: 185, 315-327
status: done
verified: 2026-05-16 — confirmed real on master @ 649c71f4. handler.go:185 readFileContent(path, maxDiffSize) called per file change; line 317 does os.ReadFile(path) THEN truncates to maxSize. A 10 GB file loads fully into RAM before the truncation. io.LimitReader on a *os.File would be the fix
fixed: 2026-05-16 — replaced os.ReadFile+slice with os.Open+io.LimitReader(maxSize+1)+io.ReadAll. Peak allocation now bounded at maxSize+1 bytes regardless of file size. Added TestReadFileContent_BoundsLargeFile/SmallFile/ExactSize; all pass.
---

## What

```go
// internal/actions/artifact_capture/handler.go:315-327
func readFileContent(path string, maxSize int) (string, error) {
    data, err := os.ReadFile(path) // #nosec G304 -- path comes from tracked file changes
    if err != nil {
        return "", err
    }
    if len(data) > maxSize {
        data = data[:maxSize]
    }
    return string(data), nil
}
```

The signature says "read up to `maxSize` bytes" but the
implementation **reads the entire file first** and then truncates
the in-memory slice. For a 10 GB file changed by a previous step:

- `os.ReadFile` allocates a 10 GB `[]byte`
- `data[:maxSize]` slices to the first 1 MB
- The 9.999 GB tail is held by the underlying array (Go slice
  semantics: slicing doesn't release the backing memory).
- `string(data)` copies the 1 MB into a new string and the
  10 GB backing array becomes eligible for GC — eventually.

Net peak memory: 10 GB + 1 MB.

Same shape as F026 (file/copy handler unbounded reads) but in
a different code path that wasn't covered by the F026 fix
(F026 streamed copy via Performer.CopyFile; artifact_capture
reads independently).

## When this fires

`readFileContent` is called from the artifact change-tracker
(line 185) to fetch the **current** contents of a file that
just got modified by an inner step. The expected case is a
small config / unit-file / dotfile (KB), but:

- An inner `file.template` that renders a large data fixture
- A `file.copy` from a large source (now mitigated by F026's
  CopyFile, but artifact_capture reads dest separately)
- A `text.patch.json` against an embedded ML model config
  blob

would all trigger the unbounded read. Bounded buffers downstream
do not help — the allocation has already happened by line 322.

## Why it's a smell, not a bug

- Default `maxDiffSize` is 1 MB. Users rarely capture multi-GB
  files.
- For the common case (config files), the over-read is
  irrelevant (1 KB file → 1 KB read → no harm).
- But the function's signature promises bounded reads. The
  implementation does not deliver. That's a future-bug surface.

## Suggested fix

```go
func readFileContent(path string, maxSize int) (string, error) {
    // #nosec G304 -- path comes from tracked file changes
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()
    // Read at most maxSize+1 so we can detect "exceeded the cap"
    // separately from "exactly fit the cap." The +1 sentinel byte
    // is discarded; the returned string is bounded at maxSize.
    data, err := io.ReadAll(io.LimitReader(f, int64(maxSize)+1))
    if err != nil {
        return "", err
    }
    if len(data) > maxSize {
        data = data[:maxSize]
    }
    return string(data), nil
}
```

The `io.LimitReader` reads at most `maxSize+1` bytes — never
the whole file when it's larger than the cap. Peak memory now
matches the API contract.

Adjacent: the inline truncations at line 172-173 and 179-180
(slicing `fc.ContentBefore` / `fc.ContentAfter`) are safe
because `fc.ContentBefore/After` come from the file-change
tracker which captures via the events pipeline — those bytes
already exist in memory. The slicing there is fine; only
`readFileContent` produces fresh reads.

## Verification

- Add `TestReadFileContent_BoundsLargeFile`: write a 100 MB
  file to a tempdir, call `readFileContent(path, 1024)`,
  assert the returned string is 1024 bytes AND peak RSS during
  the call stayed bounded (e.g. under 10 MB). Pre-fix the
  test would peak at 100 MB.
- `go test ./internal/actions/artifact_capture/...`

## References

- F026 — file/copy handler unbounded reads; same family,
  different code path.
- F018 — shell scanner bound; same family of "reader API
  contract vs implementation" mismatch.
