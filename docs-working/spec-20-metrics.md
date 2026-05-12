# Spec 20: System Metrics (Live Utilization Surface)

**Epic:** E7 Mooncake Fleet — adds a live metrics surface alongside facts so
agents can do capacity-aware scheduling across a fleet.
**Depends on:** Spec 05 (`facts --query`), Spec 10 (MCP `get_facts`).
**Complements:** Spec 18 (Agent Daemon) — turns the daemon into a fleet
observability endpoint via a dedicated `get_metrics` MCP tool.
**Effort:** M (~3–5 days)
**Value:** High — unlocks a new class of `when:` conditionals (gate steps on
load/GPU/memory) **and** gives AI agents a real-time view of every node in the
fleet through a tool surface that's purpose-built for polling.

**E7 priority slot:** E7.2 — first follow-up after the daemon walking skeleton.
Higher value and lower effort than streaming execution events (now E7.3),
and unblocks the "agent picks the idle box" use case immediately.

---

## Problem

Today `Facts` describes the machine's **capabilities** — cpu_cores, gpus,
memory_total_mb, package_manager. With Spec 18, an AI agent can query any node
in the fleet via `get_facts` over MCP. But it learns *what the box is*, not
*how busy the box is right now*.

The use case driving this: an agent has a heavy ML job to run, has three
nodes available, and wants to ask each one "what's your GPU utilization and
free VRAM right now?" Today it would `shell` into each box and parse
`nvidia-smi` output — defeating the point of having a structured surface.

An earlier draft of this spec added these as a dynamic tier inside `Facts`.
We rejected that shape for three reasons:

1. **Caching semantics lie.** A unified `Facts.Collect()` where some fields
   cache forever and others have a 2s TTL is implicit and surprising; a
   `--refresh` flag that only refreshes *some* fields is a gotcha.
2. **Snapshot diff (Spec 14) eats noise.** Mixing live metrics into the facts
   namespace forces snapshot to special-case which fields it ignores.
3. **Fleet hot-path is muddy.** Agents polling for utilization shouldn't be
   re-pulling the entire static facts blob. A dedicated tool says what it is.

This spec therefore introduces **metrics as a separate primitive** —
distinct package, distinct CLI command, distinct MCP tool. Templates and
`when:` expressions see a unified flat namespace (`{{ cpu_cores }}` and
`{{ cpu_usage_pct }}` both work as before), but the implementation
boundaries are clean.

---

## Goals

- **G1** New `internal/metrics/` package with a `Metrics` struct holding live
  system utilization: CPU, GPU + VRAM, load average, memory used %, swap,
  network throughput.
- **G2** **Per-metric TTL caching** in the metrics package (not `sync.Once` —
  a long-lived daemon needs fresh values; one-shot CLI sees a single sample).
- **G3** New `mooncake metrics` CLI mirroring `mooncake facts`: same
  `--query`, `--format`, `--output` semantics; plus `--refresh` to force
  re-sample and `--fields` to filter.
- **G4** New MCP `get_metrics` tool with `fields` and `refresh` parameters;
  response includes `_collected_at` per field so callers see freshness.
- **G5** Templates and `when:` expressions can read metrics keys directly,
  alongside facts — merged into the same variable namespace by the executor.
- **G6** Stay additive — no changes to `Facts` struct, `mooncake facts`, or
  the existing `get_facts` MCP tool. Snapshot (Spec 14) is unaffected.
- **G7** Cross-platform: Linux and macOS parity for CPU/mem/load/net; GPU is
  NVIDIA-only on Linux for v1 (Apple Silicon GPU metrics deferred).

**Non-goals:**

- Time-series storage. `Metrics` returns the current sample, not history.
- Proactive background sampling. Collection is pull-based (lazy on query).
- AMD / Intel GPU utilization. NVIDIA is concrete; others can be added
  incrementally without changing the surface.
- Per-process metrics. Out of scope; that's `ps` territory.
- Apple Silicon **GPU** utilization. `powermetrics` exposes per-cluster GPU
  power but not utilization; the IOReport API has that but needs private
  framework symbols.
- Apple Silicon **CPU die** temperature. `powermetrics --samplers smc` does
  not expose die temps on M-series chips. The data is available via
  IOReport (`asitop`-style); requires cgo. Documented in metrics.md.

---

## Package layout

```
internal/metrics/                (new)
├── metrics.go         Metrics struct + ToMap
├── collector.go       Collector interface + registry
├── cache.go           Per-collector TTL cache
├── linux.go           Linux samplers (CPU/load/mem/net)
├── linux_gpu.go       NVIDIA GPU sampler (build-tagged linux)
├── darwin.go          macOS samplers
├── *_test.go
└── testdata/          /proc/* fixtures, nvidia-smi golden output
```

No changes to `internal/facts/`. The two packages are siblings.

---

## The Metrics struct

```go
// Metrics is the live utilization surface of the host. Sampled on demand
// with per-metric TTL caching. See cache.go.
type Metrics struct {
    CPU     CPUMetrics
    Memory  MemoryMetrics
    Load    LoadMetrics
    Network NetworkMetrics
    GPUs    []GPUMetrics
}

type CPUMetrics struct {
    UsagePct     float64   // 0..100, system-wide
    UsagePerCore []float64 // one entry per logical CPU
}

type MemoryMetrics struct {
    UsedMB     int64
    UsedPct    float64 // 0..100
    SwapUsedMB int64
}

type LoadMetrics struct {
    Avg1m  float64
    Avg5m  float64
    Avg15m float64
}

type NetworkMetrics struct {
    RxBps int64 // bytes/sec, summed across non-loopback interfaces
    TxBps int64
}

// GPUMetrics is keyed by index matching facts.Facts.GPUs[i]. v1 populates
// only NVIDIA entries; others have zero values.
type GPUMetrics struct {
    Index         int
    UsagePct      float64
    MemoryUsedMB  int64
    MemoryUsedPct float64
    TemperatureC  int
}
```

`Metrics.ToMap()` flattens this into the keys the table below lists, so
templates see `{{ cpu_usage_pct }}` directly rather than `{{ cpu.usage_pct }}`
(matches the existing `facts.ToMap` conventions).

---

## ToMap keys + TTLs

| Key | Type | Source (Linux) | Source (macOS) | TTL |
|---|---|---|---|---|
| `cpu_usage_pct` | `float64` | `/proc/stat` delta over 100ms | `top -l 2 -n 0 -s 1` | 2s |
| `cpu_usage_per_core` | `[]float64` | `/proc/stat` per-cpu | `top -l 2 -n 0 -s 1` | 2s |
| `load_avg_1m` | `float64` | `/proc/loadavg` | `sysctl -n vm.loadavg` | 5s |
| `load_avg_5m` | `float64` | `/proc/loadavg` | `sysctl -n vm.loadavg` | 5s |
| `load_avg_15m` | `float64` | `/proc/loadavg` | `sysctl -n vm.loadavg` | 5s |
| `memory_used_mb` | `int64` | `/proc/meminfo` (MemTotal − MemAvailable) | `vm_stat` | 5s |
| `memory_used_pct` | `float64` | derived | derived | 5s |
| `swap_used_mb` | `int64` | `/proc/meminfo` | `sysctl vm.swapusage` | 5s |
| `gpus_metrics` | `[]GPUMetrics` | `nvidia-smi --query-gpu=index,utilization.gpu,memory.used,memory.total,temperature.gpu` | — | 2s |
| `net_rx_bps` | `int64` | `/proc/net/dev` delta over 1s | `netstat -ibn` delta | 2s |
| `net_tx_bps` | `int64` | `/proc/net/dev` delta | `netstat -ibn` delta | 2s |
| `temperatures` | `[]Sensor` | `/sys/class/hwmon/hwmon*/` | `powermetrics --samplers smc` (requires root) | 2s (Linux) / 5s (darwin) |
| `cpu_temp_c` | `float64` | derived from `temperatures` (Intel coretemp / AMD k10temp / ARM cpu_thermal) | derived (CPU die / CPU heat sink; 0 on Apple Silicon — SMC doesn't expose die temps) | 2s / 5s |

**Naming note.** The GPU metrics array key is `gpus_metrics` (not `gpus`) so
templates can read `{{ gpus }}` (facts) and `{{ gpus_metrics }}` (live)
without collision. Per-GPU correlation is via `Index` matching
`facts.Facts.GPUs[i]`.

TTLs are constants in code for v1, not user-configurable.

---

## Task 1 — Collector interface + registry

`internal/metrics/collector.go`:

```go
type Collector interface {
    Name() string                // stable id for the cache timestamp map
    Outputs() []string           // ToMap keys this populates
    TTL() time.Duration
    Collect(m *Metrics) error    // writes into m
}

var collectors []Collector

func Register(c Collector) { collectors = append(collectors, c) }
```

Each metric family (cpu, load, mem, gpu, net) is one collector, registered
from platform-specific `init()` so `nvidia-smi` is invoked once per GPU
sample (not once per GPU field). Outputs across collectors must not overlap;
enforced by a test.

---

## Task 2 — Cache with per-collector TTL

`internal/metrics/cache.go`:

```go
type cache struct {
    mu      sync.Mutex
    current *Metrics
    sampled map[string]time.Time // collector name → last collect
}

// Collect returns metrics with each collector's outputs refreshed if past
// its TTL. fields=nil means "everything". collectedAt maps each requested
// ToMap key to its collector's last successful sample timestamp.
func Collect(fields []string) (m *Metrics, collectedAt map[string]time.Time, err error) { … }

// Refresh forces re-collection on the next Collect call.
func Refresh() { … }

// ClearCache forces re-collection AND clears the cached Metrics. Test-only.
func ClearCache() { … }
```

Behavior:

1. First call: build empty `Metrics`, run all (or fields-filtered) collectors,
   record sampled timestamps, store, return.
2. Subsequent calls: for each requested collector, run only if past TTL;
   else serve cached values.
3. `collectedAt` exposes freshness without leaking TTL internals.
4. Concurrent callers serialize through `mu` — fine in v1; sample latencies
   are microseconds except `nvidia-smi` and the 100ms / 1s delta sleeps.

---

## Task 3 — Linux collectors

`internal/metrics/linux.go`:

- **cpuCollector**: read `/proc/stat`, sleep 100ms, re-read, compute per-cpu
  and aggregate utilization. First call costs ~100ms; subsequent within TTL
  are free. Document on the collector type.
- **loadCollector**: parse `/proc/loadavg` (3 floats).
- **memCollector**: parse `/proc/meminfo`; derive used = total − available;
  parse swap fields.
- **netCollector**: read `/proc/net/dev`, sleep 1s, re-read, sum non-`lo`
  rx/tx deltas. Highest cold cost (~1s); justified by accurate throughput.

`internal/metrics/linux_gpu.go` (build tag `linux`):

- **nvidiaGPUCollector**: `exec.LookPath("nvidia-smi")` in `init()` — if
  absent, register nothing. Otherwise on `Collect`, run
  `nvidia-smi --query-gpu=index,utilization.gpu,memory.used,memory.total,temperature.gpu --format=csv,noheader,nounits`
  and parse one row per GPU.

---

## Task 4 — macOS collectors

`internal/metrics/darwin.go`:

- **cpuCollector**: shell out to `top -l 2 -n 0 -s 1` (two samples 1s apart,
  read the second for steady state), parse the `CPU usage:` line. No cgo.
  Cold cost ~1s.
- **loadCollector**: `sysctl -n vm.loadavg` → `{ 1.23 1.45 1.67 }`.
- **memCollector**: `vm_stat` for pages, `sysctl hw.memsize` for total,
  `sysctl vm.swapusage` for swap.
- **netCollector**: `netstat -ibn` two reads 1s apart, sum non-`lo0` deltas.
- No NVIDIA, no Apple Silicon GPU in v1.

---

## Task 5 — CLI: `mooncake metrics`

New top-level command in `cmd/mooncake.go`, modeled on `factsCommand`:

```bash
mooncake metrics                            # full JSON/text dump
mooncake metrics --format json
mooncake metrics --query cpu_usage_pct
mooncake metrics --query cpu_usage_pct --query load_avg_1m
mooncake metrics --fields cpu_usage_pct,gpus_metrics
mooncake metrics --query gpu_usage_pct --refresh
mooncake metrics --output /tmp/metrics.json
```

Extract the existing `factsQuery` helper from `cmd/mooncake.go:318` into a
shared `cmd/query.go` (mechanical; operates on `map[string]interface{}`, so
it works on `metrics.ToMap()` unchanged). When `--fields` is set, output
contains only the requested keys plus a sibling `_collected_at` map of
`key → RFC3339 timestamp`.

`mooncake facts` is unchanged. No new flags on it.

---

## Task 6 — MCP `get_metrics` tool

Add to `internal/mcp/tools.go`:

```jsonc
{
  "name": "get_metrics",
  "description": "Return live system metrics (CPU/GPU/memory/load/network) as JSON. Cached per-metric with TTLs ~2-5s. Use fields= to restrict; use refresh=true to force re-sample.",
  "inputSchema": {
    "fields":  { "type": "array", "items": {"type": "string"}, "description": "Optional. Restrict response to these ToMap keys." },
    "refresh": { "type": "boolean", "default": false, "description": "If true, force-refresh metrics (bypass TTL)." }
  }
}
```

Handler `HandleGetMetrics` calls `metrics.Collect(fields)` (after
`metrics.Refresh()` if requested), marshals the result + `_collected_at`.

`get_facts` is unchanged. `fact_query` stays facts-only; a `metric_query`
tool is not added — `fields=` on `get_metrics` covers the same use case
more flexibly.

---

## Task 7 — Template / `when:` integration

Mooncake's executor injects facts as variables at run start
(`internal/executor/executor.go:156-161`). Extend that injection to also
collect metrics once at run start and merge `metrics.ToMap()` into the same
variable map. Two collision considerations:

1. **Key collisions.** None today; metric keys are all new. Add a test that
   asserts `facts.ToMap()` and `metrics.ToMap()` have disjoint key sets,
   guarding against future drift.
2. **Snapshot semantics.** Single-shot collection at run start matches
   today's facts behavior — fine for `when:` and template usage during a run.
   The daemon (Spec 18) uses `metrics.Collect` per-request, not via the
   executor injection.

`when: cpu_usage_pct < 50` works with no changes to the expression engine —
it's just another variable.

---

## Task 8 — Tests

| Layer | Test |
|---|---|
| `internal/metrics/cache_test.go` | TTL respected; `Refresh()` invalidates; concurrent `Collect` serializes; `collectedAt` populated only for requested fields. |
| `internal/metrics/collector_test.go` | `Outputs()` sets across registered collectors are disjoint; fake collector roundtrip. |
| `internal/metrics/linux_test.go` | Parse golden fixtures: `/proc/stat`, `/proc/loadavg`, `/proc/meminfo`, `/proc/net/dev`. |
| `internal/metrics/linux_gpu_test.go` | Mock command runner; parse `nvidia-smi --format=csv,noheader,nounits` golden output. |
| `internal/metrics/darwin_test.go` | Build-tagged; golden fixtures from `top -l 2`, `vm_stat`, `netstat -ibn`. |
| `cmd/metrics_test.go` | `--query`, `--fields`, `--refresh` behaviors. |
| `internal/mcp/get_metrics_test.go` | Roundtrip including `_collected_at`. |
| `cmd/query_test.go` | After extracting `factsQuery`, `mooncake facts --query` still passes existing tests. |
| `internal/metrics/disjoint_test.go` | `facts.ToMap()` and `metrics.ToMap()` keys are disjoint. |
| Manual | On the user's Arch + NVIDIA box: `mooncake metrics --query gpus_metrics` while a GPU workload runs; expect non-zero `usage_pct`. |

---

## Task 9 — Docs

1. New page: `docs/guide/config/metrics.md` — table of keys, types, TTLs,
   `when:` examples, daemon polling pattern.
2. `docs/guide/config/variables.md` — short "Metrics also available" pointer
   paragraph; don't duplicate the table.
3. `LLM_GUIDE.md` — add `metrics` as a 6th core system; document the
   facts-vs-metrics distinction.
4. `docs/guide/fleet/quickstart.md` (created in Spec 18) — fleet scheduling
   example: `mooncake fleet metrics gpu_usage_pct --to all`. Requires a small
   `fleet metrics` wrapper (mirror of `fleet facts`) — note this in the
   spec-18 follow-up checklist.
5. Changelog: "metrics: new live-utilization surface (cpu/gpu/load/mem/net)
   with per-metric TTL caching. `mooncake metrics`, MCP `get_metrics`."

---

## Acceptance criteria

1. `mooncake metrics --query cpu_usage_pct` returns a float in `[0, 100]` on
   Linux and macOS.
2. `mooncake metrics --query load_avg_1m` matches `uptime` to one decimal.
3. On NVIDIA hardware, `mooncake metrics --query gpus_metrics` returns a
   JSON array with `usage_pct` and `memory_used_mb` populated; on non-NVIDIA
   it returns `[]`.
4. `mooncake metrics --fields cpu_usage_pct,load_avg_1m` returns a JSON
   object with only those keys plus `_collected_at`.
5. Two `get_metrics` MCP calls 500ms apart with `refresh: false` return
   identical `cpu_usage_pct`; with `refresh: true` they return distinct
   samples.
6. `mooncake facts` output is byte-identical to its pre-spec-20 output.
7. `mooncake snapshot diff` (Spec 14) ignores metrics — two snapshots
   back-to-back under varying load produce an empty diff.
8. `when: cpu_usage_pct < 50` evaluates correctly with no engine changes.
9. `go test ./internal/metrics/... ./internal/facts/... ./cmd/... ./internal/mcp/...`
   and `make ci` pass on Linux and macOS.
10. README "What's new" gets one paragraph.

---

## Risk notes

- **`nvidia-smi` is slow** (~50–200ms cold). The 2s TTL keeps it off the hot
  path. Don't move to a background sampler unless complaints arise.
- **`/proc/stat` and `/proc/net/dev` delta require a sleep** (~100ms and ~1s
  cold). Could be parallelized into a single 1s sample window — micro-opt,
  defer.
- **macOS CPU via `top` is shell-heavy.** Acceptable; cgo + `host_statistics64`
  is the alternative if profile demands it.
- **GPU index correlation.** Both `facts` and `metrics` use
  `nvidia-smi --query-gpu=index,…` and sort by index, removing ordering
  assumptions. Documented in collector doc comments.
- **Snapshot still touches facts only.** Confirmed by Spec 14 reading
  `facts.Collect()` directly. As long as no future spec accidentally calls
  `metrics.Collect` from snapshot code, the separation holds.
- **Daemon executor merge.** When the daemon merges metrics into the
  executor's variable namespace for each run, it must collect fresh per run
  start, not serve cached from a prior request. Cache semantics here are
  per-process; the daemon owns one process per host, so cache is shared by
  design. Sub-TTL accuracy inside a long-running plan = future `metrics:`
  action, not v1.

---

## Out of scope (deferred)

- Background sampler for delta/EMA metrics (`cpu_usage_pct_1m_avg`).
- Per-process drill-down.
- AMD / Intel / Apple GPU utilization.
- User-tunable TTLs in `agent.toml`.
- Streaming metrics over SSE.
- `metrics:` action type for ad-hoc sampling inside a plan.
