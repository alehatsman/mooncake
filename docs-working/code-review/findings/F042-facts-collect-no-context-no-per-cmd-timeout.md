---
id: F042
title: facts.Collect — 20+ exec.Command calls without context or per-command timeout; one stuck NFS mount hangs the whole apply
severity: risk
package: internal/facts
files:
  - internal/facts/cache.go (Collect, no ctx in signature)
  - internal/facts/facts.go (collectUncached orchestration)
  - internal/facts/linux.go (8 exec.Command calls)
  - internal/facts/darwin.go (12 exec.Command calls)
  - internal/facts/windows.go (3+ exec.Command calls)
  - internal/facts/services.go (3 systemctl calls)
  - internal/facts/toolchains.go (2 generic `--version` probes)
status: open
---

## What

`facts.Collect()` has signature `func Collect() *Facts` — no
context, no timeout. The orchestrator `collectUncached`
(`facts.go:126`) runs platform-specific probes serially, each via
`exec.Command(...).Output()` without context:

| File | Line | Command | Hang trigger |
|---|---|---|---|
| `linux.go` | 148 | `df -BG --output=...` | stuck NFS mount, slow USB device |
| `linux.go` | 195 | `nvidia-smi --query-gpu=...` | NVIDIA driver wedged / paused |
| `linux.go` | 216 | `rocm-smi --showproductname` | AMD GPU driver hang |
| `linux.go` | 238 | `lspci` | rare but possible on PCIe topology weirdness |
| `linux.go` | 308 | `uname -r` | quick, but blocks at file-system level on some root-fs failures |
| `linux.go` | 420 | `ip route show default` | VPN / routing-stack issues |
| `linux.go` | 498 | `nvidia-smi --query-gpu=driver_version` | same as 195 |
| `linux.go` | 505 | `nvidia-smi` (no args) | same |
| `darwin.go` | 34 | `sw_vers -productVersion` | quick |
| `darwin.go` | 54 | `sysctl -n hw.memsize` | quick |
| `darwin.go` | 71 | `df -g` | same as linux 148 |
| `darwin.go` | 133 | `system_profiler SPDisplaysDataType` | **known-slow** — can take 30+s on systems with many displays |
| `darwin.go` | 195 | `uname -r` | quick |
| `darwin.go` | 204 | `sysctl -n machdep.cpu.brand_string` | quick |
| `darwin.go` | 214 | `sysctl -n machdep.cpu.features` | quick |
| `darwin.go` | 232 | `vm_stat` | quick |
| `darwin.go` | 278 | `sysctl vm.swapusage` | quick |
| `darwin.go` | 308 | `route -n get default` | quick |
| `darwin.go` | 332 | `scutil --dns` | quick |
| `darwin.go` | 365 | `sysctl -n kern.boottime` | quick |
| `services.go` | 21 | `systemctl list-units --state=failed` | systemd deadlock |
| `services.go` | 39 | `systemctl is-active <svc>` | per-svc; N invocations |
| `services.go` | 44 | `systemctl status <svc>` | systemd deadlock |
| `windows.go` | 32 | `wmic logicaldisk ...` | WMI service unhealthy → slow |
| `windows.go` | 103 | `wmic path win32_VideoController ...` | WMI |
| `toolchains.go` | 35 | `<tool> --version` (generic) | depends on which tool |

`exec.Command(...).Output()` waits for the subprocess to exit
**indefinitely**. No timeout. No context.

`facts.Collect()` is called from:

- `cmd/doctor.go` — `mooncake doctor` runs facts collection
  unconditionally.
- `internal/apply/runner.go:81` — when `--facts-json <path>` is
  set, facts collect runs at the start of every apply.
- `internal/pilot/loop.go` — snapshot for LLM context, every
  iteration.
- `internal/explain` — `mooncake explain` command.

## Why it's `risk` not `bug`

These commands almost always complete in < 100ms. The hang
triggers are real but uncommon:

- A stale NFS mount that `df` queries (real, common in
  data-center setups).
- A hung NVIDIA driver after a kernel panic recovery
  (NVIDIA bug ~once a year per host).
- `system_profiler SPDisplaysDataType` on macOS with many
  external displays (commonly 30s).
- systemd-dbus deadlock during heavy service churn.

For each scenario the user's `mooncake apply` / `mooncake
doctor` hangs **before any step runs**. Ctrl-C doesn't escape
cleanly because each `exec.Command` is in a blocking syscall
on the subprocess wait.

The reason this is `risk` rather than `bug`:

- The hang is real but rare.
- The user can work around by killing the underlying stuck
  process (kill the NFS mount, restart nvidia-persistenced,
  etc.).
- mooncake itself doesn't make the stuck system worse.

## Suggested fix

### (a) Plumb context through `Collect`

```go
// internal/facts/cache.go — new signature
func Collect(ctx context.Context) *Facts {
    cacheOnce.Do(func() {
        cachedFacts = collectUncached(ctx)
    })
    return cachedFacts
}
```

All callers thread the apply-level ctx through. The caller can
then impose a wall-clock deadline:

```go
// cmd/doctor.go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
facts := facts.Collect(ctx)
```

### (b) Add a per-command helper that uses CommandContext

```go
// internal/facts/exec.go (new)
//
// runProbe wraps exec.CommandContext with a 5-second
// per-probe deadline. Returns ("", false) on timeout / error
// instead of propagating the error — facts collection is
// best-effort, missing values fall back to defaults.
func runProbe(ctx context.Context, name string, args ...string) (string, bool) {
    probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    out, err := exec.CommandContext(probeCtx, name, args...).Output()
    if err != nil {
        return "", false
    }
    return string(out), true
}
```

5 seconds is generous for every probe in the table above
except `system_profiler` on macOS with many displays. For that
one specifically, bump to 30s or accept the truncation.

Then every site becomes:

```go
// Before:
out, err := exec.Command("df", "-BG", "...").Output()
// After:
out, ok := runProbe(ctx, "df", "-BG", "...")
if !ok { return }
```

### (c) Run probes in parallel

`collectUncached` runs all probes serially. On a healthy system,
the wall-clock is sum-of-all-probes (~200-300ms typically).
With concurrency (`go func()` per platform-specific block +
`sync.WaitGroup`), it's max-of-probes. For `mooncake doctor`
this matters; for the apply path it's less critical.

Out of scope for F042 — the wall-clock win is small and adds
goroutine-management complexity. Mention here for the next
performance pass.

## Verification

- Manual: insert a `/etc/df-hang` symlink to a broken NFS
  mount. Run `mooncake doctor` today → hangs forever. After
  fix → completes within 35s (30s ctx + 5s probe) with
  "df probe timed out" log.
- `go test ./internal/facts/...` — all existing tests should
  pass; new tests for the timeout path with a stubbed slow
  command.

## References

- F012 — HTTP-no-context cross-cutting. Same family of
  "external wait without bound."
- F016 — agentd worker no-cancel context. Same pattern at the
  apply layer.
- `darwin.go:133` `system_profiler` slowness — widely documented
  Apple behavior; mooncake users have probably already noticed
  this slows `mooncake doctor` on display-heavy machines.
