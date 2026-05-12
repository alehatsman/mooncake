# Spec 19: Utilization Facts (Dynamic Metrics Tier)

**Epic:** E7 Mooncake Fleet — adds live system metrics to the facts surface so
agents can do capacity-aware scheduling across a fleet.
**Depends on:** Spec 05 (`facts --query`), Spec 10 (MCP `get_facts`).
**Complements:** Spec 18 (Agent Daemon) — turns the daemon into a fleet
observability endpoint as a side-effect of the same `get_facts` tool.
**Effort:** M (~3–5 days)
**Value:** High — unlocks a new class of `when:` conditionals (gate steps on
load/GPU/memory) **and** gives AI agents a real-time view of every node in the
fleet through the same MCP surface they already use.

**E7 priority slot:** E7.2 — first follow-up after the daemon walking skeleton.
Higher value and lower effort than streaming execution events (now E7.3),
and unblocks the "agent picks the idle box" use case immediately.

---

## Problem

Today `Facts` describes the machine's **capabilities** — cpu_cores, gpus,
memory_total_mb, package_manager. With Spec 18, an AI agent can query any node
in the fleet via `get_facts` over MCP. But it learns *what the box is*, not
*how busy the box is right now*.

The use case driving this: an agent has a heavy ML job to run. It has three
nodes available. It wants to ask each one "what's your GPU utilization and
free VRAM right now?" and route the job to the idlest one. Today it would
have to `shell` into each box and parse `nvidia-smi` output by hand — defeating
the point of having facts.

A subset of dynamic data already lives in facts (`disks[].used_pct`,
`memory_free_mb`, `failed_services[]`), but they're sampled once at process
start and frozen for the run. That's fine for one-shot CLI; useless for a
long-running daemon answering repeated queries.

---

## Goals

- **G1** Add a **dynamic tier** to `Facts` with live system metrics: CPU
  utilization, GPU utilization + VRAM, load average, memory used %, disk IO,
  network throughput.
- **G2** Replace `sync.Once` with **per-fact TTL caching** so dynamic facts
  refresh on a configurable interval; static facts stay frozen for the process
  lifetime.
- **G3** Extend `mooncake facts --query` and the MCP `get_facts` tool with a
  `--fields` / `fields` parameter so callers can ask for one metric without
  paying the cost of collecting all of them.
- **G4** Add a `--refresh` flag (CLI) and `refresh: true` option (MCP) that
  forces re-collection of dynamic facts, bypassing TTL.
- **G5** Stay additive — every existing fact keeps its name, type, and
  template path. New facts are new keys.
- **G6** Cross-platform: Linux and macOS parity for CPU/mem/load; GPU is
  NVIDIA-only on Linux for v1 (Apple Silicon GPU metrics are deferred).

**Non-goals:**

- Time-series storage. Facts return the current sample, not a history.
- Proactive background sampling. Collection stays **pull-based** (lazy on
  query). A sampler daemon is a later spec if needed for delta metrics.
- AMD / Intel GPU utilization. NVIDIA is concrete; other vendors require
  separate tooling (`rocm-smi`, `intel_gpu_top`) and can be added incrementally.
- Per-process metrics (top-N CPU consumers, etc.). Out of scope; that's `ps` territory.
- Apple Silicon GPU utilization. `powermetrics` requires sudo and is hostile to
  parse; defer until someone asks.

---

## Key files

| File / location | Role |
|---|---|
| `internal/facts/facts.go` | Add dynamic fields to `Facts` struct; extend `ToMap()`. |
| `internal/facts/cache.go` | Replace `sync.Once` with per-fact TTL cache (see Task 2). |
| `internal/facts/dynamic.go` (new) | Collector interface + registry for dynamic facts. |
| `internal/facts/linux_dynamic.go` (new) | Linux samplers: `/proc/stat`, `/proc/loadavg`, `/proc/meminfo`, `/proc/net/dev`, `nvidia-smi`. |
| `internal/facts/darwin_dynamic.go` (new) | macOS samplers: `sysctl`, `vm_stat`, `iostat`, `netstat -ibn`. |
| `cmd/facts.go` | Add `--fields` and `--refresh` flags. |
| `internal/mcp/tools.go` | Extend `get_facts` schema with `fields` and `refresh`. |
| `internal/executor/executor.go:156-161` | Confirm executor still calls `Collect()` at startup — no behavior change, but dynamic facts captured at that moment will be one snapshot for the whole run (acceptable: see "Semantics in non-daemon mode" below). |

No changes to `internal/plan/`, `internal/config/`, action implementations.

---

## New facts (the dynamic tier)

| Key | Type | Source (Linux) | Source (macOS) | TTL |
|---|---|---|---|---|
| `cpu_usage_pct` | `float64` | `/proc/stat` delta over 100ms | `host_statistics64` via `sysctl` | 2s |
| `cpu_usage_per_core` | `[]float64` | `/proc/stat` per-cpu | same | 2s |
| `load_avg_1m` | `float64` | `/proc/loadavg` | `sysctl vm.loadavg` | 5s |
| `load_avg_5m` | `float64` | `/proc/loadavg` | `sysctl vm.loadavg` | 5s |
| `load_avg_15m` | `float64` | `/proc/loadavg` | `sysctl vm.loadavg` | 5s |
| `memory_used_mb` | `int64` | `/proc/meminfo` (MemTotal − MemAvailable) | `vm_stat` | 5s |
| `memory_used_pct` | `float64` | derived | derived | 5s |
| `swap_used_mb` | `int64` | `/proc/meminfo` | `sysctl vm.swapusage` | 5s |
| `gpus[].usage_pct` | `float64` | `nvidia-smi --query-gpu=utilization.gpu` | — (deferred) | 2s |
| `gpus[].memory_used_mb` | `int64` | `nvidia-smi --query-gpu=memory.used` | — | 2s |
| `gpus[].memory_used_pct` | `float64` | derived | — | 2s |
| `gpus[].temperature_c` | `int` | `nvidia-smi --query-gpu=temperature.gpu` | — | 5s |
| `disks[].used_pct` | `int` | **already exists**; convert to TTL'd | same | 30s |
| `net_rx_bps` | `int64` | `/proc/net/dev` delta over 1s | `netstat -ibn` delta | 2s |
| `net_tx_bps` | `int64` | `/proc/net/dev` delta | `netstat -ibn` delta | 2s |

**TTL rationale:**

- 2s for the things an agent actually polls when scheduling work (CPU, GPU,
  network throughput).
- 5s for slower-moving values (load avg already smooths internally).
- 30s for disk usage — changes slowly, and `df` shells out.

TTLs are constants in code for v1, not user-configurable. If users complain,
move them to `agent.toml`.

---

## Task 1 — Collector interface + registry

`internal/facts/dynamic.go`:

```go
// DynamicCollector samples one or more dynamic facts and writes them into f.
// Called only when at least one of its outputs is requested AND past TTL.
type DynamicCollector interface {
    Name() string                          // for logging / debug
    Outputs() []string                     // ToMap keys this populates
    TTL() time.Duration
    Collect(f *Facts) error
}

// Registry of all dynamic collectors, populated by init() in platform files.
var dynamicCollectors []DynamicCollector

func RegisterDynamic(c DynamicCollector) {
    dynamicCollectors = append(dynamicCollectors, c)
}
```

Each metric family (cpu, load, mem, gpu, net) is one collector, so `nvidia-smi`
is invoked once per GPU sample (not once per gpu field).

`linux_dynamic.go` and `darwin_dynamic.go` each register their platform's
collectors in `init()`. Platform mismatch = collector simply not registered;
its outputs return zero values and are tagged as unavailable in the result.

---

## Task 2 — Per-fact TTL cache

Replace `cache.go` wholesale. New shape:

```go
type cache struct {
    mu       sync.Mutex
    static   *Facts                          // collected once via sync.Once
    staticOk sync.Once
    sampled  map[string]time.Time            // collector name → last collect
}

// Collect returns facts with static fields always present and dynamic fields
// refreshed per their collector's TTL. fields=nil means "everything".
func Collect(fields []string) *Facts { … }

// Refresh forces re-collection of dynamic facts on the next Collect call.
func Refresh() { … }
```

Behavior:

1. First call: collect all static facts via `collectUncached()` (today's
   `Collect`), cache forever.
2. For each requested dynamic field (or all of them if `fields == nil`):
   look up its collector, check `now - sampled[collector.Name()] >= TTL`,
   call `Collect(f)` if stale.
3. If `Refresh()` was called since last collect, zero out `sampled` and
   re-sample everything dynamic on next `Collect`.
4. Thread-safety: the mutex protects `sampled` and the in-place writes into
   the Facts struct. Concurrent MCP requests serialize through it — fine for
   v1 (sampling is microseconds except `nvidia-smi`).

`ClearCache()` (used by tests today) also clears `sampled` and `staticOk`.

**Compatibility shim:** Add `Collect()` (no args) as `Collect(nil)`. Existing
call sites — `executor.go:156-161` and any tests — pass through unchanged.

---

## Task 3 — CLI: `--fields` and `--refresh`

`cmd/facts.go`:

```bash
# Existing behavior — unchanged
mooncake facts
mooncake facts --query cpu_usage_pct

# New: ask for only specific fields (skips collectors for everything else)
mooncake facts --fields cpu_usage_pct,gpus,load_avg_1m

# New: force re-collection of dynamic facts even if TTL hasn't expired
mooncake facts --query gpu_usage_pct --refresh
```

`--fields` accepts comma-separated `ToMap()` keys. For nested arrays it accepts
the array key (`gpus`) and emits the whole array; sub-field paths
(`gpus[0].usage_pct`) are out of scope — that's `--query` territory.

When `--fields` is set:
1. Resolve each field to its owning collector (static fields no-op since
   they're always present).
2. Pass the union of needed collectors into `Collect`.
3. Output only the requested fields in the JSON.

Exit codes unchanged from Spec 05.

---

## Task 4 — MCP `get_facts` extension

Extend the existing `get_facts` tool schema (`internal/mcp/tools.go`):

```jsonc
{
  "name": "get_facts",
  "inputSchema": {
    "fields":  { "type": "array", "items": {"type": "string"}, "description": "Optional. Restrict response to these ToMap keys. Skips collectors for unrequested dynamic facts." },
    "refresh": { "type": "boolean", "default": false, "description": "If true, force-refresh dynamic facts (bypass TTL)." }
  }
}
```

Response shape unchanged when `fields` is omitted (the full facts blob, now
with the new keys). When `fields` is set, response contains only those keys
plus a sibling `"_collected_at": { "<field>": "<RFC3339>" }` map so the caller
can see the freshness of each dynamic value.

**Why per-field timestamps:** an agent polling `cpu_usage_pct` every 500ms
needs to know whether the daemon actually re-sampled or just served from
cache. The timestamp answers that without exposing TTL internals.

---

## Task 5 — Semantics in non-daemon mode

In one-shot CLI mode the process exits in seconds, so per-fact TTL caching
collapses to "collect once per command invocation" — same as today. That's
fine; users running `mooncake apply` get a snapshot of utilization at the
start of the run, which is what they'd get from a `shell` step calling `top`
anyway.

In daemon mode (Spec 18), the process is long-lived and the TTL cache becomes
the whole point: `get_facts` calls 30 seconds apart see fresh utilization
data, `get_facts` calls 1 second apart see cached data, and an agent driving
fleet scheduling decides its own polling cadence.

**Subtle point:** `cpu_usage_pct` is a delta metric — it requires two samples
over an interval to compute. The Linux collector takes a sample, sleeps
100ms, takes a second sample, returns the delta. That makes the first
`Collect` for CPU cost ~100ms; subsequent calls within TTL are free. Document
this in the collector's doc comment so it's not a surprise for someone
profiling cold-start.

---

## Task 6 — Tests

| Layer | Test |
|---|---|
| `internal/facts/cache_test.go` | TTL respected: collector called once if two `Collect` calls are within TTL, twice if outside. `Refresh()` invalidates. |
| `internal/facts/dynamic_test.go` | Fake collector with controlled clock; assert outputs land in correct ToMap keys; assert `Outputs()` are mutually exclusive (no collector overlaps another's keys). |
| `internal/facts/linux_dynamic_test.go` | Parse fixtures: `/proc/stat`, `/proc/loadavg`, `/proc/meminfo`, `/proc/net/dev` golden files in `testdata/`. Don't shell out to `nvidia-smi`; mock the command runner. |
| `internal/facts/darwin_dynamic_test.go` | Same with darwin tool output fixtures. Skipped on Linux CI; runs in Mac CI lane if it exists, otherwise manual. |
| `cmd/facts_test.go` | `--fields foo,bar` returns only those keys; `--refresh` causes a second collector invocation in the same process. |
| `internal/mcp/get_facts_test.go` | `fields` and `refresh` params honored; `_collected_at` populated for requested dynamic fields. |
| Manual | On the user's Arch + NVIDIA box: `mooncake facts --query gpu_usage_pct` while a GPU workload is running; expect non-zero. |

---

## Task 7 — Docs

1. `docs/guide/config/variables.md` — new "Dynamic facts" subsection listing
   each new key, its type, its TTL, and a usage example.
2. `LLM_GUIDE.md` — extend the facts paragraph with the dynamic tier and the
   `--fields` flag.
3. `docs/guide/fleet/quickstart.md` (created in Spec 18) — add a "fleet
   scheduling" example: `mooncake fleet facts gpu_usage_pct --to all` to pick
   the idlest GPU node.
4. Changelog entry: "facts: live utilization metrics (cpu/gpu/load/mem/net),
   per-fact TTL caching, `--fields` and `--refresh`."

---

## Acceptance criteria

1. `mooncake facts --query cpu_usage_pct` returns a float in `[0, 100]` on
   Linux and macOS.
2. `mooncake facts --query load_avg_1m` matches `uptime` output to one decimal
   place.
3. `mooncake facts --query gpus` on a node with NVIDIA hardware includes
   `usage_pct` and `memory_used_mb` fields; on a non-NVIDIA node the GPU array
   is empty (unchanged from today).
4. `mooncake facts --fields cpu_usage_pct,load_avg_1m` returns a JSON object
   with **only** those two keys plus `_collected_at`. No `disks`, no `tools`,
   no `gpus`.
5. In daemon mode (Spec 18), two `get_facts` MCP calls 500ms apart with
   `refresh: false` return identical `cpu_usage_pct` values; with
   `refresh: true` they return distinct samples.
6. `mooncake facts` (no flags) returns the same shape it does today, with the
   new dynamic keys added — every existing field still present, no rename.
7. `go test ./internal/facts/... ./cmd/... ./internal/mcp/...` and `make ci`
   pass on Linux and macOS.
8. A `when:` expression like `when: cpu_usage_pct < 50` parses and evaluates
   correctly in the existing expression engine (no new operators needed; the
   value is a float).

---

## Risk notes

- **`nvidia-smi` is slow** (~50–200ms cold). Sampling on every 2s TTL window
  is fine; sampling on every MCP call would be a problem. The TTL cache
  prevents the latter. If users hammer `--refresh`, that's their choice.
- **`/proc/stat` delta requires a sleep**, which makes the first cold CPU
  sample ~100ms. Acceptable for human-driven CLI; in the daemon, the sampler
  pays this once per TTL window. Don't move to a background sampler unless we
  see real complaints.
- **macOS CPU sampling needs `host_statistics64` via cgo or shelling to
  `top -l 1`.** Prefer the shell approach for v1 — no cgo, parseable output,
  matches the existing darwin.go pattern. cgo is a later optimization if the
  shell-out latency becomes a problem.
- **Apple Silicon GPU.** Documented as deferred. If a user asks, `ioreg` and
  `powermetrics` are the candidates; `powermetrics` needs root which the
  daemon already has but the CLI does not. Worth its own mini-spec.
- **Fact key surface growth.** This spec adds ~10 top-level keys plus ~4
  per-GPU sub-keys. The `Facts` struct stays manageable, but if we keep
  growing it, a later refactor to namespace dynamic facts under
  `Facts.Live.*` may be warranted. Not v1.
- **Snapshot diff (Spec 14).** Dynamic facts will create constant
  diff-noise if snapshots include them. Mark dynamic-tier fields with a tag
  and exclude them from snapshot comparison; the snapshot is about *what the
  machine is*, not *what it's doing*.

---

## Out of scope (deferred)

- Proactive background sampler for delta/EMA metrics (`cpu_usage_pct_1m_avg`).
- Per-process drill-down.
- AMD / Intel / Apple GPU utilization.
- User-tunable TTLs in `agent.toml`.
- `Facts.Live.*` namespace refactor.
- Streaming utilization over SSE (subsumes the rebadged spec-19a streaming
  spec, but only for facts; execution-event streaming is still separate).
