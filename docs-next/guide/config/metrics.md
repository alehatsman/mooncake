# Metrics

Mooncake exposes live system utilization — CPU, GPU, memory, load, network —
as a *metrics* surface separate from [facts](variables.md#system-facts).

**Facts vs. metrics:**

- **Facts** describe what the machine *is*: cores, total memory, installed
  tools, package manager. Cached for the lifetime of the process.
- **Metrics** describe what it's *doing right now*: how busy the CPU is, how
  much VRAM is used, current load average. Sampled on demand with short TTLs.

Both flow into the same variable namespace, so a `when:` expression can read
either kind without caring which:

```yaml
- name: Only train if the GPU is idle
  shell: python train.py
  when: gpu_usage_pct < 20

- name: Only run heavy installs on multi-core boxes
  apt: { name: build-essential, state: present }
  when: cpu_cores >= 8 and load_avg_1m < 4
```

## CLI

```bash
mooncake metrics                              # full JSON/text dump
mooncake metrics --format json
mooncake metrics --query cpu_usage_pct
mooncake metrics --query cpu_usage_pct --query load_avg_1m
mooncake metrics --fields cpu_usage_pct,gpus_metrics
mooncake metrics --refresh                    # force re-sample, bypass TTL
```

`--fields` filters the output to specified keys and adds a sibling
`_collected_at` map of each key's last-sample timestamp (RFC3339) so callers
can see freshness without leaking TTL internals.

## Available metrics

Each metric has a TTL — within that window, repeated reads serve the cached
value rather than re-sampling. TTLs are tuned for the kinds of decisions
agents make on these values.

| Key | Type | Description | TTL |
|---|---|---|---|
| `cpu_usage_pct` | float (0–100) | System-wide CPU utilization | 2s |
| `cpu_usage_per_core` | []float | Per-core CPU utilization (Linux only) | 2s |
| `load_avg_1m` | float | 1-minute load average | 5s |
| `load_avg_5m` | float | 5-minute load average | 5s |
| `load_avg_15m` | float | 15-minute load average | 5s |
| `memory_used_mb` | int64 | Resident memory in MB | 5s |
| `memory_used_pct` | float (0–100) | Resident memory as % of total | 5s |
| `swap_used_mb` | int64 | Swap used in MB | 5s |
| `net_rx_bps` | int64 | Bytes/sec received (non-loopback) | 2s |
| `net_tx_bps` | int64 | Bytes/sec transmitted (non-loopback) | 2s |
| `gpus_metrics` | array | Per-GPU live metrics (NVIDIA on Linux only) | 2s |
| `temperatures` | array | Hardware temperature sensors (Linux only — hwmon) | 2s |
| `cpu_temp_c` | float | Derived CPU package temperature in °C, 0 if unavailable | 2s |

Each `temperatures` entry has:

```json
{
  "Chip": "coretemp",
  "Label": "Package id 0",
  "TempC": 50,
  "CritC": 105
}
```

The collector reads `/sys/class/hwmon/` (the canonical Linux sensor surface
that `lm_sensors` exposes), so anything visible there — CPU package and
per-core temps, NVMe SSD, WiFi card, motherboard sensors — shows up here.

`cpu_temp_c` is a derived convenience picking the most authoritative CPU
sensor available. Priority order (Linux):

1. **AMD Tctl** (throttle-control temp from `k10temp` / `zenpower`)
2. **Intel Package id 0** (from `coretemp`)
3. **AMD Tdie** (die temp fallback)
4. **max(Core *)** when only per-core sensors are exposed
5. **`cpu_thermal`** (ARM)

Returns 0 when no CPU sensor is detectable.

### macOS

On macOS, temperatures come from `powermetrics --samplers smc`, which **requires
root**. Two modes:

- **User-shell invocation (`mooncake metrics`)** — no root, no temps. The
  collector silently returns an empty array rather than prompting for a
  password (interactive sudo from a metrics call would be terrible UX).
- **Daemon invocation (Spec 18 `mooncaked`)** — the agent daemon runs as
  root, so temperatures come through automatically. This is the common case
  for fleet observability.

The parser extracts SMC entries with chip `smc` and label `CPU die`,
`GPU die`, `CPU heat sink`, `Battery`, etc. `cpu_temp_c` prefers `CPU die`,
falling back to `CPU heat sink`.

**Apple Silicon caveat**: Apple's `powermetrics` does not expose die
temperatures on M-series chips — the SMC sampler returns thermal pressure
state only. Expect `temperatures` to be empty and `cpu_temp_c` to be 0 even
when running as root on an Apple Silicon Mac. The underlying data is
available via private IOReport APIs (used by `asitop`, `mactop`, `stats`)
but requires cgo and is out of scope for v1.

Each `gpus_metrics` entry has:

```json
{
  "Index": 0,
  "UsagePct": 87,
  "MemoryUsedMB": 6400,
  "MemoryUsedPct": 78.1,
  "TemperatureC": 72
}
```

`Index` matches the corresponding entry in the static `gpus` fact, so you
can correlate name/driver/model from facts with live load from metrics.

## Sampling notes

- **CPU**: 100ms sample window on Linux. First read costs ~100ms; subsequent
  reads within TTL are free.
- **Network**: 1s sample window on both Linux and macOS. The collector
  measures a delta to compute bytes/sec.
- **GPU**: NVIDIA only in v1, on Linux. Requires `nvidia-smi` in PATH. If
  `nvidia-smi` is present but no GPUs are visible (driver not loaded), the
  array is empty rather than missing.
- **macOS per-core CPU**: not exposed in v1 (`top` doesn't print it without
  cgo). `cpu_usage_pct` is still accurate; `cpu_usage_per_core` is empty.
- **Apple Silicon GPU**: not in v1. `powermetrics` needs root and is hostile
  to parse.

## Polling pattern (daemon / fleet)

When [mooncake agent](../fleet/quickstart.md) is running, an external client
can poll metrics via the `get_metrics` MCP tool:

```json
{
  "method": "tools/call",
  "params": {
    "name": "get_metrics",
    "arguments": {
      "fields": ["cpu_usage_pct", "gpus_metrics"],
      "refresh": false
    }
  }
}
```

Set `refresh: true` to bypass TTL — useful when you've just kicked off a job
and want a clean baseline.

The response includes `_collected_at` with a per-key timestamp so the agent
can decide whether the cached value is fresh enough for its decision.
