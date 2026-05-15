# Observability — `observe.*` actions

Mooncake's `observe.*` family is the **read-side mirror of typed
mutation** (spec-59). Where `file.write` / `pkg.install` / `os.service`
*change* state, `observe.port` / `observe.process` / `observe.http`
**read** state with the same typed-result discipline — no shelling
out to `grep` and parsing strings.

Nine handlers ship today:

| Action | Reads | Typical use |
|---|---|---|
| `observe.port` | TCP/UDP listener state | Is the service bound? Which pid? |
| `observe.process` | Process by name or argv regex | Is the worker running? With what args? |
| `observe.http` | HTTP GET response | Health endpoint status + latency |
| `observe.service` | systemd / launchd unit state | Active? Enabled at boot? |
| `observe.cpu` | CPU usage + load averages | Is the box busy? Many cores? |
| `observe.memory` | RAM + swap usage in bytes | Enough headroom to deploy? |
| `observe.disk` | Filesystem space + inodes | Is `/var` dangerously full? |
| `observe.gpu` | GPU utilization + VRAM | Safe to restart the inference daemon? |
| `observe.logs` | Recent log lines + pattern matches | Did the service log errors after restart? |

**Facts vs. metrics vs. observers** — same three-way axis as in
[`metrics.md`](./metrics.md):

- **Facts** describe what the machine *is*: cores, distro, total
  memory. Cached at run start.
- **Metrics** describe what the machine is *doing right now*. Sampled
  on demand with TTLs; flow into the variable namespace
  (`{{ cpu_usage_pct }}`).
- **Observers** are *per-step typed reads*. Each `observe.*` action
  returns a structured payload bound to a step's `as:` name and
  branchable via `when:`.

Pick observers when you want **structured state on a specific step**
("right here, right now, what's the value?"); pick facts/metrics when
you want box-wide state in templates.

---

## The shape

Every observer follows the same contract:

```yaml
- name: probe
  observe.<kind>:
    <kind-specific fields>
  as: probe        # spec-37 capture → templates can read probe.value.*
```

Captured payload (universal envelope):

| Key | Type | Meaning |
|---|---|---|
| `probe.found` | bool | Did the observation succeed? Per-handler — `observe.port` sets it when a listener was found; `observe.http` sets it when the response matched any expect filter. |
| `probe.value` | object | Per-handler typed payload (schemas below). |
| `probe.as_of` | string | RFC3339 timestamp of the observation. |
| `probe.error` | string | Error message when `found` is false because the *read failed*. Empty when `found` is false because the *resource genuinely doesn't exist*. |

This means you can always write `when: "probe.found"` to gate on
"did we get an answer?" or `when: "not probe.value.<field>"` to gate
on the specific state.

---

## Plan-mode behavior

By default, observers are **deferred in plan mode**. `mooncake plan`
emits a "would observe X (deferred to apply)" line and the captured
envelope has `found=false`, `error="observation deferred to apply mode"`,
and a zero-value `value` payload.

Why: plan mode is structurally predictive; running real `nvidia-smi`
or HTTP GETs in plan mode would surprise people. The zero-value
payload keeps downstream `{{ probe.value.<field> }}` templates from
blowing up when authors run `plan` first.

To force real reads in plan mode, set `inspect: { real: true }` on
the step *(planned, spec-59 phase 6)*. Apply mode always executes
real reads.

---

## Composing with `when:` and `on_change:`

The canonical pattern is observe → branch:

```yaml
- name: probe nginx
  observe.port: { host: localhost, port: 80 }
  as: nginx

- name: restart nginx if port is down
  os.service: { name: nginx, state: restarted }
  when: "not nginx.value.open"
```

Observers can also trigger reactive children via `on_change:`, but
note that "changed" means "the observation's `changed_when`
predicate fired" — not "the underlying state shifted since last
run." For drift-style continuous comparison, use spec-58 drift on
agentd, not `on_change:` in a one-shot plan.

---

## Per-handler reference

### `observe.port`

Single-shot TCP/UDP listener probe.

```yaml
- observe.port:
    host: localhost     # default: localhost
    port: 8080          # required, 1..65535
    protocol: tcp       # tcp (default) | udp
    timeout: 2s         # default 2s
  as: app
```

`value` shape:

```json
{
  "open": true,
  "protocol": "tcp",
  "host": "localhost",
  "port": 8080,
  "local_addr": "localhost:8080",
  "listener": "nginx",      // best-effort, may be empty
  "pid": 1234               // 0 if not permitted
}
```

### `observe.process`

Match a process by exact name or argv regex.

```yaml
- observe.process:
    name: sshd                  # exact basename match
    # or:
    # pattern: '^postgres:'     # regex against full argv
  as: ssh
```

`value` shape: `running` (bool), `pid` (int), `pids` ([]int when multiple match), `args` ([]string of first match), `user`, `started_at`.

Implementation: Linux walks `/proc`; macOS/BSDs shell out to `ps -eo`.

### `observe.http`

GET an HTTP endpoint with timeout + header capture + body sample cap.

```yaml
- observe.http:
    url: https://api.internal/health
    method: GET                       # default GET
    timeout: 3s
    expect_status: 200                # if set, found=false on mismatch
    capture_headers: [Server, X-Request-ID]
    skip_tls_verify: false            # set true for self-signed
  as: health
```

`value` shape: `url`, `method`, `status_code`, `reachable` (transport-level), `latency_ms`, `headers` (map of captured), `body_sample` (truncated to 2 KiB).

`expect_status: N` flips `found` to false when the response status
isn't N — handy for gating: `when: "not health.found"`.

### `observe.service`

systemd (Linux) / launchd (macOS) unit state.

```yaml
- observe.service:
    name: nginx
    manager: auto         # auto (default) | systemd | launchd
  as: svc
```

`value` shape: `exists`, `active`, `enabled`, `sub_state` (systemd: running/exited/dead), `manager`.

`active` is "is it running now?". `enabled` is "will it start at
boot?". They diverge surprisingly often — a clean spec for "is the
service healthy?" is `svc.value.active and svc.value.enabled`.

### `observe.cpu`

CPU usage + load averages. Wraps the shared `internal/metrics`
collector — same sample as `mooncake metrics`, no duplicate
`/proc/stat` reads.

```yaml
- observe.cpu: {}
  as: cpu
```

`value` shape: `cores`, `usage_pct`, `idle_pct`, `load_1m`, `load_5m`, `load_15m`.

### `observe.memory`

RAM + swap in **absolute bytes** (not MB / percentages).

```yaml
- observe.memory: {}
  as: mem
```

`value` shape: `total_bytes`, `used_bytes`, `free_bytes`, `available_bytes`, `swap_total_bytes`, `swap_used_bytes`.

Byte counts make threshold comparisons trivial:

```yaml
when: "mem.value.available_bytes > 1073741824"   # > 1 GiB free
```

### `observe.disk`

Filesystem space + inode usage for a path. Uses `statfs(2)` on
POSIX, `GetDiskFreeSpaceExW` on Windows.

```yaml
- observe.disk:
    path: /var          # default /
  as: var
```

`value` shape: `path`, `total_bytes`, `used_bytes`, `free_bytes`, `inodes_total`, `inodes_used`, `read_only`.

`free_bytes` reports "free to non-root" — the more useful number
for "do I have room to write?".

### `observe.gpu`

GPU utilization + VRAM. NVIDIA on Linux via `nvidia-smi`; Apple
Silicon on macOS via `powermetrics` + `system_profiler`. Best-
effort AMD (ROCm) — if no recognized runtime is present, the
observation succeeds with `count=0` (honest "no GPU detected").

```yaml
- observe.gpu:
    index: 0           # optional — specific GPU; default returns all
  as: gpu
```

`value` shape: `count`, `vendor`, `gpus` (array of per-device fields:
index, utilization_pct, memory_used_bytes, memory_used_pct,
temperature_c), `aggregate` (max_utilization_pct, memory_used_bytes
across all detected).

```yaml
# Don't restart ollama while a GPU job is using > 2 GiB VRAM
when: "gpu.value.aggregate.memory_used_bytes < 2147483648"
```

### `observe.logs`

Read recent log lines and match against regex patterns. Three source
modes — exactly one must be set.

```yaml
- observe.logs:
    path: /var/log/nginx/error.log      # file source
    # or:
    # journal_unit: nginx.service       # systemd journal (Linux)
    # or:
    # container: nginx-1                # docker logs / podman logs

    since: 60s                          # time window (default 60s)
    patterns:                           # required: regex list
      - 'crit|emerg|alert'
      - '\[error\]'
      - 'connection refused'
    sample_lines: 5                     # per-pattern sample cap
    max_bytes: 1048576                  # 1 MiB byte cap
    max_lines: 10000                    # line cap
  as: nginx_log
```

`value` shape: `source` (file/journal/container), `identifier`, `window`, `lines_read`, `truncated`, `matches` (array of `{pattern, count, sample_lines}`).

Caps are *hard* — a 100 MB log can't OOM the runner. When a cap is
hit, `truncated: true`; pattern counts reflect only what was read.

---

## Fleet fan-out

Every observer above is available fleet-wide via `mooncake fleet
observe <kind> [args]`:

```bash
# Tabular comparison across all peers
mooncake fleet observe port --port 80
mooncake fleet observe gpu --peer-filter 'tag=inference'
mooncake fleet observe disk --path /var

# JSONL for scripting
mooncake fleet observe http --url https://example.internal/health \
  --expect-status 200 --format json | jq -c 'select(.status != "success")'
```

The CLI synthesizes a one-step plan, submits to each peer via
existing `submit_run`, captures each peer's `step.completed` event,
and renders per-kind columns (or one JSONL record per peer). All
the standard fleet selectors apply: `--peers`, `--peer-filter`,
`--peers-file`, `--parallel`.

See [`fleet commands`](../commands.md#fleet-observe) for the full
flag reference.

---

## Examples

End-to-end runnable examples live in
[`examples/observability/`](https://github.com/alehatsman/mooncake/tree/master/examples/observability):

- `check-port.yml` — observe.port → conditional log on open/closed
- `check-app-health.yml` — observe.http → observe.service → observe.process compose
- `system-health-gate.yml` — observe.cpu + memory + disk pre-flight gate
- `post-restart-log-check.yml` — observe.logs scan + branch on `lines_read`

---

## Design notes

A few load-bearing choices, mostly to keep the family honest:

1. **No mutation. Ever.** Every observer declares `Changed=false`,
   empty `Diff`, nil `Reverse`, `Cost{Risk: 1, Reversible: true}`,
   `Permissions{ReadOnly}`. They are read-only by contract — the
   spec-22 ABI lets a future policy gate treat the entire family
   as such.

2. **Plan-mode defers.** Real reads in plan mode would surprise.
   Authors who want richer previews opt in with `--inspect-real`
   (spec-59 phase 6, not yet shipped).

3. **JSON-shaped values.** Each handler's typed `Value` round-trips
   through JSON before publishing to `Result.Data`, so templates
   see a nested `map[string]any` keyed by `json` struct tags —
   `{{ probe.value.open }}` works out of the box.

4. **No streaming.** Observers are point-in-time. For "watch and
   react" patterns the right home is spec-58 drift on agentd, not
   the plan executor (per [`non-goals.md`](../../../docs-working/non-goals.md)
   §7 "No long-running plan steps").

5. **`observe.cpu` shares the metrics collector.** A single
   `/proc/stat` or `host_processor_info` sample feeds both
   `/v1/metrics` and `observe.cpu` — no duplicate OS calls. The
   same pattern extends to `observe.gpu`.

---

## Cross-references

- [`Metrics`](./metrics.md) — box-wide live utilization (the
  template namespace variant).
- [`Variables`](./variables.md#system-facts) — static facts.
- [`Actions reference`](./actions.md) — per-action top-level page.
- [Spec 59 — Typed Observability](https://github.com/alehatsman/mooncake/blob/master/docs-working/specs/action-surface/spec-59-typed-observability.md)
  — the design doc for the seed.
- [Spec 64 — `fleet observe`](https://github.com/alehatsman/mooncake/blob/master/docs-working/specs/personal-fleet/spec-64-fleet-observe.md)
  — cross-peer fan-out.
- [`non-goals.md`](../../../docs-working/non-goals.md) — the
  discipline that keeps the observation surface narrow.
