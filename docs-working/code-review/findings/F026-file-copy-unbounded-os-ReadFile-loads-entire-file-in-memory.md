---
id: F026
title: file / copy handlers use os.ReadFile on user-targeted paths — large files load entire contents into RAM (multiple times per Run)
severity: risk
package: internal/actions
files:
  - internal/actions/file/handler.go (lines 224, 298, 416)
  - internal/actions/copy/handler.go (line 192)
status: open
---

## What

`grep -nE 'os\.ReadFile|io\.ReadAll' internal/actions/{file,copy}/handler.go`
returns 4 hits on user-targeted paths:

| File | Line | Purpose | Approx. RAM per call |
|---|---|---|---|
| `file/handler.go` | 224 | Pre-write snapshot for artifact-capture | `len(file)` |
| `file/handler.go` | 298 | Read existing for backup file | `len(file)` |
| `file/handler.go` | 416 | Post-write snapshot for event payload | `len(file)` |
| `copy/handler.go` | 192 | Read source to feed to `Effects().WriteFile` | `len(src)` |

`os.ReadFile` allocates a `[]byte` the size of the file. For a
single `file.write` on a 10 GB target, the handler can allocate
**up to 30 GB** of bytes (pre-snapshot + backup-existing +
post-snapshot). The user's typical use case is small config files
(few KB), so this hasn't surfaced as a complaint, but the
upper bound is unbounded.

`copy/handler.go:192` is the more directly bad shape: it reads
the entire source file just to hand it to
`ctx.Effects().WriteFile(dest, content, mode, opts)` which then
re-writes the same bytes. That's 2× the file size in RAM with
**no benefit** — the source bytes are not consumed by anything
other than the WriteFile call.

## Why it's `risk` not `bug`

Today's typical user runs `mooncake` on:

- Config files (kbytes)
- Small scripts / unit files (kbytes)
- Small overlays / certificates (kbytes)

Nobody runs `file: write` on a 10 GB log archive. But:

- **CI agents** copying large artifact bundles, container layers,
  ML model weights with `file.copy` would hit this on the next
  use case.
- The `download` action correctly uses `io.Copy` streaming
  (`tool/fetch.go:39`, post-F007 fix would be similar). Copy
  buffering everything in memory **diverges** from the streaming
  contract that download already follows.
- A user running `mooncake` inside a memory-constrained
  container (e.g. a 256 MB sidecar) will OOM-kill on the first
  multi-MB copy.

## Suggested fix

### `copy/handler.go:192` — easiest

Add a `CopyFile(src, dest string, mode os.FileMode, opts PerformerOpts)`
primitive to `actions.Performer` and `internal/effects`. Then:

```go
// Before
content, err := os.ReadFile(src)
if err != nil { ... }
eff := ctx.Effects().WriteFile(dest, content, mode, ...)

// After
eff := ctx.Effects().CopyFile(src, dest, mode, ...)
// (Performer implementation streams via os.Open + os.Create + io.Copy)
```

The Effects boundary is the right place for this — it already
owns `WriteFile`, so adding `CopyFile` as a sibling is no
contortion.

### `file/handler.go` — bounded snapshots

For artifact-capture and event payloads, the consumer doesn't
need the full file bytes — they need **size**, **sha256**, and
maybe **a head sample** (first N bytes for tooling that
fingerprints by magic bytes).

```go
// Replace the unbounded os.ReadFile with a sized helper:
//
// snapshotForArtifact(path string, maxSampleBytes int) (size int64,
//     sha256 string, sampleHead []byte, err error)
//
// Streams the file once, hashing as it goes, capturing up to
// maxSampleBytes of the head, and recording the total size. RAM
// cost: maxSampleBytes (e.g. 8 KB).
```

The event payload's `SizeBefore` / `checksumBefore`
(`file/handler.go:430-431`) work with the helper output
unchanged. Code that needs full bytes (which is none, today)
explicitly opts in.

For the backup case (line 298): use `os.Rename(target,
target+".bak")` if the user is OK losing the read-after-rename
window. Or stream-copy:

```go
src, _ := os.Open(target)
dst, _ := os.Create(backupPath)
io.Copy(dst, src)
```

Both avoid loading the entire file into RAM.

### Defense-in-depth

Cap any remaining `os.ReadFile` on user-supplied paths with a
guard:

```go
const maxInMemoryFileBytes = 64 * 1024 * 1024 // 64 MB

func readFileGuarded(path string) ([]byte, error) {
    info, err := os.Stat(path)
    if err != nil {
        return nil, err
    }
    if info.Size() > maxInMemoryFileBytes {
        return nil, fmt.Errorf("%s exceeds %d-byte in-memory read cap; use streaming pathway", path, maxInMemoryFileBytes)
    }
    return os.ReadFile(path)
}
```

Surfaces the constraint instead of OOM-killing silently. Any
caller that legitimately needs more either streams or bumps the
cap.

## Verification

- `go test ./internal/actions/{copy,file}/...`
- New test `TestFileHandler_LargeFile`: write a 200 MB file
  with `file: write` and assert peak RSS stays bounded (e.g.
  under 50 MB). Pre-fix this test would peak at ~600 MB.
- Manual: `mooncake apply` on a config that copies a 10 GB file
  inside a 256 MB container; should succeed post-fix.

## References

- F018 (shell scanner unbounded buffer) — adjacent pattern in
  a different package.
- `internal/actions/tool/fetch.go:39` — `io.Copy` streaming for
  HTTP downloads; the right shape for large IO.
- `download` handler does this correctly (`download/handler.go:408`).
