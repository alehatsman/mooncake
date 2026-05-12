# Spec 14: Snapshot Diff

**Epic:** E3 System Snapshot (S3.5)  
**Effort:** XS (1–2h)  
**Value:** Medium — drift detection between runs; agents can verify a run changed what was expected

---

## Problem

`mooncake snapshot` captures current state. But there's no way to compare two
snapshots — "what changed on this machine since last week?" or "did that run
actually install the tools I asked for?"

---

## Goal

```
mooncake snapshot --diff <previous.json>
```

Outputs a human-readable diff of what changed between the saved snapshot and
the current state:

```
tools:
  + rust   1.79.0  (was: not present)
  ~ go     1.22.4  (was: 1.21.0)
  - java   (removed)

hw:
  ~ ram_free_mb  4096 → 2048

services:
  + failed: thermald  (was: none)
```

With `--format json`:
```json
{
  "added": {"tools": {"rust": "1.79.0"}},
  "changed": {"tools": {"go": {"from": "1.21.0", "to": "1.22.4"}}, "hw": {"ram_free_mb": {"from": 4096, "to": 2048}}},
  "removed": {"tools": {"java": "17.0.1"}}
}
```

If nothing changed: print `no changes` (text) or `{}` (JSON) and exit 0.

---

## Implementation

### `internal/snapshot/diff.go`

```go
type SnapshotDiff struct {
    Added   map[string]interface{} `json:"added,omitempty"`
    Changed map[string]interface{} `json:"changed,omitempty"`
    Removed map[string]interface{} `json:"removed,omitempty"`
}

func Diff(prev, curr *SystemSnapshot) SnapshotDiff
```

Compare fields:
- `tools`: map diff — keys added, removed, or value changed
- `hw`: compare `ram_free_mb`, `disk_free_gb` (skip cpu/total — those don't change)
- `services.failed`: slice diff
- `os`: compare distro, kernel (flag version upgrades)

### `cmd/snapshot.go`

Add `--diff <path>` flag. When set:
1. Load previous snapshot JSON from path
2. Collect current snapshot
3. Call `snapshot.Diff(prev, curr)`
4. Render as text or JSON per `--format`

---

## Acceptance Criteria

1. `--diff prev.json` compares prev to current and prints changes.
2. New tools show as `+`, removed as `-`, version changes as `~`.
3. `--format json` emits structured diff JSON.
4. No changes prints `no changes` and exits 0.
5. Missing `--diff` file exits 1 with clear error.
6. `mooncake snapshot --save current.json` saves current snapshot to file for later diffing.
