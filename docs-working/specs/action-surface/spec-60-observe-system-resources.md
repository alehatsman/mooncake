# Spec 60: `observe.cpu` / `observe.memory` / `observe.disk` — System resource observers

**Status:** Draft (depends on spec-59)
**Epic:** E9 Modern Action Surface — bucket E9.4 (observability extensions)
**Effort:** M (3 handlers + metrics-cohesion design pass)
**Value:** Medium-high. Closes the "is this box healthy enough to apply this?"
gate that drift detection (spec-58), workload placement (issue #11 §14),
and pre-flight checks all want. Without typed system observers, every
"only proceed if /var has >10GB free" check rides on `shell` + parsing.

**Design principles:** [`action-design-principles.md`](../../action-design-principles.md) + [`non-goals.md`](../../non-goals.md)

---

## Problem

[`spec-59`](./spec-59-typed-observability.md) establishes the `observe.*`
family with 4 seed handlers. The most-requested follow-on is system
resource observation: cpu, memory, disk. The use case is the same shape
as `observe.port` — *single-shot, typed read* — but the data already
exists in two places:

1. **`internal/facts/`** — collects static-ish fields at run start
   (cpu_cores, total_memory_mb).
2. **`internal/metrics/`** — collects live utilization (cpu_usage_pct,
   load_avg, memory_used_mb, disk_used_pct, network counters) with
   per-OS implementations + TTL caching. Exposed at `/v1/metrics` on
   agentd.

The cohesion question this spec must answer: **do we promote metrics
into typed `observe.*` handlers, keep them as a separate subsystem,
or both?**

---

## Goals

- **G1** Three new handlers, one per resource family:
  - `observe.cpu` — current usage, load average, core count, per-core idle.
  - `observe.memory` — total / used / free / available / swap (typed bytes, not pct strings).
  - `observe.disk` — per-mount or per-path: total / used / free / fs type / mount flags.
- **G2** Reuse `internal/metrics/` as the data source where it
  already collects what's needed. Handlers are thin wrappers that
  reshape the metrics struct into the spec-59 `ObserveResult`
  envelope.
- **G3** Where metrics doesn't have the needed shape (per-path disk
  usage rather than per-mount; per-core idle rather than aggregate),
  add to `internal/metrics/` first, then expose. Keep one data path,
  not two.
- **G4** All three implement spec-22 ABI (no-mutation specialization
  per spec-59): empty Diff, nil Reverse, `Cost{Risk:1, Reversible:true}`,
  `Permissions{ReadOnly:true, RequiredBinaries:[]}` (uses kernel
  interfaces, not shell).

**Out of scope:**

- Network observation (`observe.network`) — deferred to its own spec
  if a real consumer surfaces. Today's `metrics.NetworkStats` is
  per-interface counters; a useful `observe.network` shape needs
  more design (interfaces? rates? per-process bandwidth?).
- Replacing `internal/facts/` static fields. Facts stay for box-wide
  one-shot collection at run start; observers stay for per-step
  current-state reads.
- Continuous monitoring / time-series — that's drift loop territory
  (spec-58). Observers are point-in-time.

---

## Design

### Per-handler shapes

```go
// observe.cpu
type CPUObservation struct {
    Cores       int     `json:"cores"`         // physical + logical
    UsagePct    float64 `json:"usage_pct"`     // aggregate %
    LoadAvg1m   float64 `json:"load_1m"`
    LoadAvg5m   float64 `json:"load_5m"`
    LoadAvg15m  float64 `json:"load_15m"`
    IdlePct     float64 `json:"idle_pct"`
}

// observe.memory
type MemoryObservation struct {
    TotalBytes      int64 `json:"total_bytes"`
    UsedBytes       int64 `json:"used_bytes"`
    FreeBytes       int64 `json:"free_bytes"`
    AvailableBytes  int64 `json:"available_bytes"` // free + reclaimable cache
    SwapTotalBytes  int64 `json:"swap_total_bytes,omitempty"`
    SwapUsedBytes   int64 `json:"swap_used_bytes,omitempty"`
}

// observe.disk
type DiskObservation struct {
    Path        string `json:"path"`         // mount path or queried path
    Mount       string `json:"mount"`        // the resolved mount point
    FSType      string `json:"fs_type"`      // ext4 / apfs / ntfs / ...
    TotalBytes  int64  `json:"total_bytes"`
    UsedBytes   int64  `json:"used_bytes"`
    FreeBytes   int64  `json:"free_bytes"`
    InodesTotal int64  `json:"inodes_total,omitempty"`
    InodesUsed  int64  `json:"inodes_used,omitempty"`
    ReadOnly    bool   `json:"read_only"`
}
```

### YAML shapes

```yaml
- observe.cpu: {} as: cpu
- observe.memory: {} as: mem
- observe.disk: { path: /var } as: var_disk

- name: bail if /var is dangerously full
  assert:
    that: "var_disk.value.free_bytes > 1073741824"   # > 1 GiB
    msg: "/var only has {{ var_disk.value.free_bytes | bytes }} free"
```

### Metrics cohesion

The honest answer: **share data, separate surface**.

- Both `internal/metrics/` (live metrics handler at `/v1/metrics`) and
  `internal/actions/observe_*/` (per-step handlers) read from one
  shared collection layer.
- Refactor `internal/metrics/` into `internal/metrics/collector.go`
  (data) + `internal/metrics/handler.go` (HTTP). Observer handlers
  call the collector directly.
- No new OS-specific code. Linux uses `/proc/stat`, `/proc/meminfo`,
  `syscall.Statfs`; macOS uses `host_processor_info` + `vm_statistics`
  + `statfs`; Windows uses `GetSystemInfo` + `GlobalMemoryStatusEx` +
  `GetDiskFreeSpaceEx`. All already partially present.

---

## Key files

| File | Change |
|---|---|
| `internal/metrics/collector.go` | New. Extract collection logic from `internal/metrics/handler.go`. |
| `internal/actions/observe_cpu/handler.go` | New. |
| `internal/actions/observe_memory/handler.go` | New. |
| `internal/actions/observe_disk/handler.go` | New. Per-path or per-mount selection. |
| `internal/config/config.go` | Three new Step fields. |
| `internal/register/register.go` | Three new registrations. |
| `examples/observability/system-health-gate.yml` | End-to-end example. |

---

## Phases

1. Extract `internal/metrics/collector.go` from current `metrics/handler.go`.
   Add the missing data (per-path disk, per-core idle) if not already there.
2. Land `observe.memory` first — simplest shape, no path argument.
3. `observe.cpu` — uses the same collector.
4. `observe.disk` — adds the path-resolution edge cases (symlinks,
   bind mounts).
5. Docs + schema regen.

---

## Acceptance criteria

- `examples/observability/system-health-gate.yml` runs the three
  observers + an `assert` and reports actionable values on failure.
- `mooncake plan` is side-effect-free for all three (no probes;
  deferred per spec-59).
- `/v1/metrics` still works — no regression in the existing HTTP
  surface after the collector refactor.
- Build / vet / lint / test green.

---

## Open questions

1. **Disk: per-mount or per-path?** Per-path is more useful for
   "is /var/log full?" but adds path-resolution complexity. Ship per-path
   as primary; expose `mount` field so users can disambiguate.
2. **CPU: instantaneous or averaged?** A single `/proc/stat` read
   gives totals-since-boot, not "right now." The collector already
   tracks a delta over a short window; expose `usage_pct` as the
   window average. Document the window length.
3. **Should this be one spec or three?** One — the cohesion question is
   shared, the data source is shared, and the implementation is staged
   in phases anyway.

---

## Cross-references

- [`spec-59-typed-observability.md`](./spec-59-typed-observability.md) — parent spec; ABI shape.
- [`spec-58-fleet-drift.md`](../personal-fleet/spec-58-fleet-drift.md) — first consumer (disk-free checks for drift gating).
- `internal/metrics/` — the data source being refactored.
