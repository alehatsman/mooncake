# Spec 62: `observe.gpu` — Typed GPU observation

**Status:** Draft (depends on spec-59)
**Epic:** E9 Modern Action Surface — bucket E9.4 (observability extensions)
**Effort:** S-M (1 handler; complexity is in vendor coverage, not core)
**Value:** High for the **personal AI fleet** scope this project is
sized for. Every workstation in that scope has at least one GPU, and
half the use cases (inference deploy, model serve, CUDA upgrades)
need typed GPU state to gate decisions.

**Design principles:** [`action-design-principles.md`](../../action-design-principles.md) + [`non-goals.md`](../../non-goals.md)

---

## Problem

Today GPU information comes from two places, neither typed for
per-step branching:

1. **`internal/facts/`** — `facts.gpu` returns model/driver/CUDA
   version as static fields at run start. No utilization, no
   per-process VRAM, no live state.
2. **`internal/metrics/`** — `metrics.GPUMetrics` exposes per-GPU
   utilization + temp + memory (Linux NVIDIA via `nvidia-smi`;
   macOS via `powermetrics`). Used by the metrics HTTP surface,
   not by plan steps.

A typical use case the gap hits:

> "Don't restart ollama if there's an inference job using >2GB
> VRAM."

Today: `shell: nvidia-smi --query-gpu=memory.used --format=csv,noheader`
then parse, then `when:`. Untyped, NVIDIA-only, no vendor abstraction.

This spec adds `observe.gpu` — typed read of current GPU state at
step time. **Cohesion question** mirrors spec-60: facts is static
collection; metrics is the live data source; observers are per-step
typed reads. Same answer: share data, separate surface.

---

## Goals

- **G1** `observe.gpu` handler. Default returns aggregate state
  across all detected GPUs; `index: N` selector targets a single
  GPU.
- **G2** Cover NVIDIA (Linux) via `nvidia-smi` + structured fields
  (the existing collector already does this).
- **G3** Cover Apple Silicon (macOS) via `powermetrics` + system_profiler
  — same data path as `internal/metrics/`.
- **G4** Best-effort coverage for AMD ROCm (Linux) via `rocm-smi`
  if present, else `Found: false, Error: "no GPU runtime detected"`.
- **G5** spec-22 ABI: empty Diff, nil Reverse, `Cost{Risk:1, Reversible:true}`,
  `Permissions{ReadOnly:true, RequiredBinaries:[nvidia-smi]}` (or
  the per-vendor binary based on what's detected).
- **G6** Share the collector with `internal/metrics/` per spec-60's
  refactor (do not introduce a second nvidia-smi caller).

**Out of scope:**

- Per-process GPU attribution ("which PID is using this VRAM?") — needs
  vendor-specific PID querying (`nvidia-smi pmon`). Useful but bigger;
  add as `observe.gpu_processes` if a real consumer surfaces.
- Multi-vendor abstraction layer beyond NVIDIA + Apple + best-effort
  AMD. If Intel discrete GPUs become common in the personal-AI-fleet
  scope, revisit.
- GPU benchmarking / synthetic load. Pure observation only.

---

## Design

### Per-handler shape

```go
type GPUObservation struct {
    Count    int             `json:"count"`              // number of GPUs detected
    Vendor   string          `json:"vendor,omitempty"`   // "nvidia" | "apple" | "amd" | ""
    Runtime  string          `json:"runtime,omitempty"`  // "cuda" | "metal" | "rocm"
    GPUs     []GPUDevice     `json:"gpus,omitempty"`
    Aggregate GPUAggregate   `json:"aggregate"`         // summed/maxed across all detected
}

type GPUDevice struct {
    Index            int     `json:"index"`
    Name             string  `json:"name"`              // "NVIDIA RTX 5090" / "Apple M3 Pro"
    MemoryTotalBytes int64   `json:"memory_total_bytes"`
    MemoryUsedBytes  int64   `json:"memory_used_bytes"`
    UtilizationPct   float64 `json:"utilization_pct"`
    TemperatureC     float64 `json:"temperature_c,omitempty"`
    PowerWatts       float64 `json:"power_watts,omitempty"`
    DriverVersion    string  `json:"driver_version,omitempty"`
    CudaVersion      string  `json:"cuda_version,omitempty"`
}

type GPUAggregate struct {
    MemoryTotalBytes int64   `json:"memory_total_bytes"`
    MemoryUsedBytes  int64   `json:"memory_used_bytes"`
    MaxUtilizationPct float64 `json:"max_utilization_pct"`
}
```

### YAML

```yaml
- observe.gpu: {} as: gpu

- name: refuse ollama restart while GPU is busy
  assert:
    that: "gpu.value.aggregate.memory_used_bytes < 2147483648"  # < 2 GiB
    msg: "GPU memory in use ({{ gpu.value.aggregate.memory_used_bytes | bytes }}) — refusing to restart"
  when: "gpu.value.count > 0"
```

### Vendor detection

The collector already encodes the priority order: NVIDIA (most
common in this fleet scope) → Apple → AMD. Result:

- `gpu.value.count == 0` is honest: no GPU runtime found. Don't
  pretend "we couldn't tell."
- `gpu.value.vendor == ""` means count is 0.

---

## Key files

| File | Change |
|---|---|
| `internal/actions/observe_gpu/handler.go` | New. |
| `internal/metrics/gpu_*.go` | Refactor: shared collector per spec-60. |
| `internal/config/config.go` | New Step field. |
| `internal/register/register.go` | Registration. |
| `examples/observability/gpu-gated-restart.yml` | End-to-end example. |

---

## Phases

1. Shared GPU collector (depends on / pairs with spec-60's
   `internal/metrics/collector.go` extraction).
2. NVIDIA handler path.
3. Apple Silicon handler path.
4. AMD best-effort + the "no GPU runtime" graceful failure shape.
5. Docs + schema regen.

---

## Acceptance criteria

- On an NVIDIA box, `observe.gpu` returns typed per-GPU memory and
  utilization. Captured value drives a `when:` correctly.
- On a box with no GPU (or no driver loaded), `observe.gpu` returns
  `{Count:0, Found:true, Aggregate: {zero}}` — not an error. The
  author can branch on `gpu.value.count > 0`.
- Build / vet / lint / test green; no regressions to `internal/metrics/`.

---

## Open questions

1. **Should facts.gpu still exist after this lands?** Probably yes —
   facts is run-start, observers are per-step. They serve different
   timing needs. Worth flagging as redundant in docs if no one uses
   facts.gpu after spec-62 lands.
2. **Caching the nvidia-smi call:** the existing metrics TTL cache
   is fine for `/v1/metrics` but might be wrong for `observe.gpu`
   where the author wants *now* state. Default to no cache for
   observers; let the collector decide.
3. **NVIDIA without `nvidia-smi` (driver missing):** common on
   bare-metal during a CUDA install run. `Found: true, Count: 0,
   Error: "nvidia-smi not found"` — keeps Found honest while
   surfacing the actionable diagnostic.

---

## Cross-references

- [`spec-59-typed-observability.md`](./spec-59-typed-observability.md) — parent.
- [`spec-60-observe-system-resources.md`](./spec-60-observe-system-resources.md) — shares the metrics-collector refactor.
- `internal/facts/gpu_*.go` — the static cousin.
- `internal/metrics/gpu_*.go` — the data source.
