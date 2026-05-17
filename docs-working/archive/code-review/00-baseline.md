# Baseline — 2026-05-16

**Branch reviewed:** `worktree-code-review` at `2e9714d` (origin/master)
**Go version:** `go1.26.3 linux/amd64`
**Host:** Linux 6.6.87.2-microsoft-standard-WSL2

## Build

```
$ go build ./...
(no output)
```

✅ Clean.

## Test

```
$ go test ./...
```

All 60+ packages PASS. No skipped failures, no flakes observed in
a single run. Notable wall-clock numbers:

| Package | Wall (s) |
|---|---:|
| `internal/facts` | 11.7 |
| `internal/agentd` | 8.9 |
| `internal/executor` | 6.6 |
| `internal/fleet/transport` | 6.6 |
| `internal/plan` | 6.0 |
| `internal/actions/shell` | 6.0 |

If test wall-clock matters in CI, `facts` is the longest hop.

## Lint

```
$ golangci-lint cache clean && golangci-lint run ./...
internal/actions/observe_disk/stat_unix.go:20:16: unnecessary conversion (unconvert)
        bsize := int64(st.Bsize)
                      ^
1 issues:
* unconvert: 1
```

One issue. See [findings/F001-observe-disk-bsize-cast.md](./findings/F001-observe-disk-bsize-cast.md).

> Note: `golangci-lint cache clean` was run first per the
> documented cross-worktree cache-leak issue
> (`memory/reference_golangci_cache_contamination.md`). The single
> remaining lint is real, not a stale-cache artifact.

## Arch soft-cap status

```
$ make budget-status
```

1. **Handler LOC vs cap 1500**
   - ⚠ `file` — 1,349 LOC (within 20% of cap)
   - ⚠ `package` — 1,216 LOC (within 20% of cap)
   - ✗ `service` — 1,607 LOC (over)
   - ✗ `tool` — 1,676 LOC (over)

2. **Non-test functions vs gocyclo cap 35**
   - ✗ `explain.DisplayFacts` — gocyclo 44 (`internal/explain/explain.go:55:1`)

3. **`internal/config.Step` universal-field count vs cap 40**
   - ⚠ 36 universal fields (within 20% of cap)

### Doc drift

`CLAUDE.md` "Today's known violations" lists:
- `internal/actions/file` — 2,044 LOC ← stale (now 1,349)
- `copy.(*Handler).Execute` — gocyclo 41 ← resolved (deleted in arch-wins)
- `executor.ExecuteStep` — gocyclo 37 ← resolved (extractions in arch-wins)

CLAUDE.md was updated to point to `make budget-status` as source of
truth, but the inline list right below it didn't get re-aligned.
See [findings/F002-claude-md-soft-cap-list-stale.md](./findings/F002-claude-md-soft-cap-list-stale.md).

## What's queued

See [TODO.md](./TODO.md) for the review queue.
