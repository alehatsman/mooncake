---
id: F001
title: observe_disk stat_unix.go — `int64(st.Bsize)` lint vs cross-platform conflict
severity: risk
package: internal/actions/observe_disk
file: internal/actions/observe_disk/stat_unix.go
lines: 20
status: open
---

## What

`golangci-lint run ./...` reports:

```
internal/actions/observe_disk/stat_unix.go:20:16: unnecessary conversion (unconvert)
        bsize := int64(st.Bsize)
                      ^
```

On Linux/amd64, `syscall.Statfs_t.Bsize` is already `int64`, so the
cast is a no-op. The linter (which runs under Linux build tags) sees
it as redundant.

## Why it's a `risk` and not just a style nit

On **darwin** (macOS) and the BSDs, `syscall.Statfs_t.Bsize` is
`int32`. The expression on line 21:

```go
total := int64(st.Blocks) * bsize
```

multiplies `int64 × int32`, which is a compile error on those
platforms. The cast is needed for the cross-platform build to work.

This was deliberately introduced — see the side-findings merge in
`6f4cde0` ("observe_disk macOS build fix — int64(st.Bsize) cast
resolves int32×int64 mismatch"). The lint rule and the macOS build
are in direct conflict, and the linter is the louder voice today.

## Suggested fix

Add a build-tag-aware suppression so the lint doesn't fire on Linux
but the cast survives for darwin/bsd. Two viable shapes:

**Option A — inline nolint with rationale (1-line change):**

```go
//nolint:unconvert // st.Bsize is int32 on darwin/*bsd; cast needed for cross-platform multiply.
bsize := int64(st.Bsize)
```

**Option B — split `stat_linux.go` / `stat_darwin.go`** so each OS
gets the right cast/no-cast version. Heavier, but the `statfs(2)`
struct already differs in non-trivial ways (e.g. `Mntonname` on
darwin, the doc-comment in `stat_unix.go` already acknowledges
"Linux has fsmagic numbers; macOS has Mntonname"). If this file
grows past Bsize/Bfree/Files, split it.

Today, **Option A is enough** — the only divergence is the field
type. Re-evaluate at the next per-OS need.

## Why not just delete the cast?

Linting passes on Linux but the macOS build breaks. CI on macOS
(if/when it runs) would catch this; the lint-fix would regress the
side-findings fix.

## Verification

```sh
golangci-lint cache clean
golangci-lint run ./...
# (no issues)

GOOS=darwin GOARCH=amd64 go build ./internal/actions/observe_disk
# (compiles)
```

## References

- `6f4cde0` side-findings merge: introduced the cast for macOS.
- CLAUDE.md `reference_golangci_cache_contamination.md`: always
  `cache clean` first when verifying.
