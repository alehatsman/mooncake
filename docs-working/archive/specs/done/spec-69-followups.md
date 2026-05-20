# Spec 69 — Post-Implementation Review (2026-05-17)

**Reviewer:** assistant, against master @ 6615265e (spec-69 cleanup
landed at 2bf0302b). Findings validated end-to-end in a Docker
harness — every BLOCKER below has a runnable repro at
`/tmp/mooncake-spec69-test/`.

**Scope:** verify the spec-69 work (centralized privilege escalation +
retry + override) is complete, surface anything still loose, and call
out test gaps.

**Headline:** the pipeline pieces (`ctx.Privileged()`, `RawRunner` +
executor retry loop, plan-time preflight, `applyResultOverrides`)
mostly work. **Two serious BLOCKERs surface in apply-time tests:**

- **B0 (NEW, found via manual testing):** `dispatchRunner` silently
  drops handler errors when the handler returns
  `(result-with-Failed=false, non-nil-err)`. The Spec-69 phase 2-3
  override-clearing branch fires even when the step has no
  `failed_when` set. Every `os.*` and `pkg.*` handler that returns
  `(result, err)` without setting `result.Failed = true` (which is
  all of them — only shell/command set Failed) has its error
  silently swallowed and the step reports `ok=1 changed=0`. This is
  a step-execution-correctness regression introduced by spec-69
  itself, not a residual gap. **Must fix before declaring spec
  closed.**
- **B1:** `os.user` declares `Sudo: true` but `runCmd` skips
  escalation via bare `exec.Command`. Compounded by B0: instead of
  the user seeing EACCES from `useradd`, they see a green
  "succeeded with no change" recap while the user account never
  gets created.
- **B2:** `os.service` doesn't implement `RawRunner` → user
  `retry:` is silently dropped. Less severe than B0/B1; the step
  still fails-loudly, it just doesn't retry.

Documented exemptions (os.systemd, service/shared, package,
http_request, download, …) all check out for escalation — they each
carry an in-source comment explaining why they stay on the older
API. None of them dodge B0 by design; the bug is upstream of them.

---

## What landed

- `actions.PrivilegedRunner` (`internal/actions/privileged.go`) + the
  `internal/security/privileged.go` implementation, surfaced as
  `ctx.Privileged()` on `actions.Context`.
- `actions.RawRunner` and `actions.Retryable` interfaces
  (`internal/actions/interfaces.go:253-295`). Executor prefers
  `RawRunner` when present; falls back to `Runner.Run` otherwise.
  `Retryable` widened in `cf197d33` to `(Result, err, step)` so a
  future `http_request` opt-in can inspect status codes without a
  typed-error wrapper.
- Executor retry loop `runWithRetry` and `scaleRetryDelay` in
  `internal/executor/retry.go`. MT-48 invariant (retry decision is on
  raw err, not post-`failed_when` verdict) and MT-62 invariant
  (`linear`/`exponential` backoff curves) are now structurally
  enforced in one place.
- Override centralization `applyResultOverrides` in
  `internal/executor/finalize.go` — runs once post-retry per
  `dispatchRunner` at `internal/executor/executor.go:1338-1358`.
- Plan-time preflight extension in `internal/executor/preflight.go`:
  `Sudo + non-root + AsUser set + no sudo password → fail at
  preflight` (`Spec-69 phase 4 extension`).
- 30 handlers implement `RawRunner` and consume `ctx.Privileged()` /
  `ctx.Effects()` for escalation. Package-level `privRunner` / `eff`
  globals have been removed in 9 handlers (commit `2bf0302b`),
  replaced by `runnerOrDefault(runner)` parameter threading.
- In-handler retry deleted from `shell` and `command`
  (`internal/actions/shell/backoff.go` and
  `internal/actions/shell/mt48_retry_failed_when_test.go` gone).
- Executor-level test coverage in
  `internal/executor/retry_test.go` (12 tests, incl. backoff
  curves and `Retryable` hook) and
  `internal/executor/finalize_test.go` (7 tests, incl. MT-48 mask /
  promote / changed_when inversion).

---

## Findings

### BLOCKER

#### B0. `dispatchRunner` silently swallows handler errors when result.Failed is false (NEW REGRESSION introduced by spec-69 phase 2-3)

**Location:** `internal/executor/executor.go:1338-1358`.

```go
if r, rok := result.(*Result); rok {
    if oErr := applyResultOverrides(ec, &step, r); oErr != nil {
        err = oErr
    } else if r.Failed {
        if err == nil {
            err = fmt.Errorf("step failed (failed_when=true)")
        }
    } else if err != nil {
        // failed_when masked the failure: clear err so the
        // step reports success. This is the documented
        // "retry N times, then don't fail the run no matter
        // what" pattern (MT-48).
        err = nil   // ← bug
    }
}
```

The intent of the `else if err != nil { err = nil }` branch is "if
`failed_when: false` masked the failure, suppress the error." The
implementation assumes the only way to reach this branch is via
`failed_when: false`. In practice it fires for **any** RawRunner
handler that returns a non-nil `*Result` (with `Failed=false`) and a
non-nil err, regardless of whether `failed_when` is set.

`applyResultOverrides` short-circuits with `nil` when both
`step.ChangedWhen == "" && step.FailedWhen == ""` (see
`internal/executor/finalize.go:27-29`). It doesn't touch `r.Failed`
in that case. So if the handler:

1. Constructs `result := executor.NewResult()` early,
2. Hits an error and returns `(result, err)` without setting
   `result.Failed = true`,

…the executor's branch silently clears `err` to `nil` and the step
is recorded as a success.

**Audit — which handlers are affected?** Any handler returning
`(*executor.Result, err)` without `result.Failed = true` before the
return. Quick grep of the spec-69 phase-5 migrated handlers:

```
$ grep -rn "result.Failed\s*=\s*true" internal/actions/os_user/ \
    internal/actions/os_group/ internal/actions/os_cron/ \
    internal/actions/os_mount/ internal/actions/pkg_hold/ \
    internal/actions/os_systemd/ internal/actions/os_sysctl/ \
    internal/actions/os_firewall/ internal/actions/os_ssh_key/ \
    internal/actions/pkg_upgrade/ internal/actions/pkg_repo/
[no matches]
```

→ **every spec-69 phase-5 migrated handler is affected.** Shell and
command escape because they explicitly set `result.Failed = true`
before returning the error. Package escapes because it returns
`nil, err` instead of `result, err` — making the `result.(*Result)`
assertion fail.

**Reproduced (apply-mode, non-root user, valid --sudo-pass):**

```
# T9: pkg.upgrade with bogus manager — handler returns (result, err)
# where err = "unsupported manager: not_a_real_manager"
$ /usr/local/bin/mooncake apply -c t9-pkg-upgrade-bad.yml \
    --sudo-pass testpass --insecure-sudo-pass
▶ T9 — pkg.upgrade with bogus manager
✓ T9 — pkg.upgrade with bogus manager
RECAP  ok=1  changed=0  skipped=0  failed=0
```

```
# T3c: os.user as alice (non-root, with --sudo-pass) — useradd
# EACCESs (no sudo wrap), handler returns (result, err)
$ /usr/local/bin/mooncake apply -c t3c-simple.yml \
    --sudo-pass testpass --insecure-sudo-pass
▶ T3c — simplest os.user
✓ T3c — simplest os.user
RECAP  ok=1  changed=0  skipped=0  failed=0
$ getent passwd spec69probe; echo "rc=$?"
rc=2     # user was NOT created
```

Compare against:

```
# T10: shell exit 1 — shell explicitly sets result.Failed = true
$ /usr/local/bin/mooncake apply -c t10-shell-fail.yml
▶ T10 — shell exit 1 (should fail)
✗ T10 — shell exit 1 (should fail)
RECAP  ok=0  changed=0  skipped=0  failed=1
```

```
# T8: pkg with nonexistent package — pkg handler returns (nil, err)
$ /usr/local/bin/mooncake apply -c t8-pkg-bogus.yml \
    --sudo-pass testpass --insecure-sudo-pass
✗ T8 — pkg install of nonexistent package (should FAIL)
  failed to install packages [...]: exit status 100
RECAP  ok=0  changed=0  skipped=0  failed=1
```

**Why MT-48 doesn't catch this:** the deleted MT-48 invariant test
(`internal/actions/shell/mt48_retry_failed_when_test.go`) only
exercised the shell handler, which **does** set `result.Failed =
true` on error. The replacement tests in
`internal/executor/finalize_test.go` test `applyResultOverrides`
directly — they never wire the override-clearing branch in
`dispatchRunner`. And `internal/executor/retry_test.go` tests
`runWithRetry` directly without going through `dispatchRunner` at
all.

**Suggested fix:** gate the `err = nil` clear on
`step.FailedWhen != ""`. Something like:

```go
} else if err != nil && step.FailedWhen != "" {
    // failed_when masked the failure: clear err so the
    // step reports success.
    err = nil
}
```

That's the actual MT-48 semantic. An err without an explicit
`failed_when` override is still an error.

**Test gap to close concurrently:** add a regression test in
`internal/executor/` that calls `dispatchRunner` with a fake
RawRunner returning `(non-nil-result-with-Failed=false, non-nil-
err)` and asserts the step is reported failed (not silently
swallowed). The existing `executor.RegisterRawRunner` test
machinery (if any) or a fresh fake-action wired through
`actions.Register` works.

#### B1. `os.user` declares `Sudo: true` but `runCmd` skips escalation

- `internal/actions/os_user/handler.go:63-82` — `Permissions()`
  returns `Sudo: true` on linux, darwin, windows.
- `internal/actions/os_user/handler.go:404-417` — `runCmd` calls
  bare `exec.Command(bin, args...)`. No `ctx.Privileged()`, no
  `BecomeRunner`.
- `internal/actions/os_user/platform_linux.go:58-67` —
  `applyPlanLinux` dispatches `useradd` / `usermod` / `userdel`
  through `runCmd` → all touch `/etc/passwd`, `/etc/shadow`,
  `/etc/group`. Without root → EACCES at apply, **after** the plan
  reported `would change`.

**Why this matters:** this is the exact bug class spec-69 was
written to eliminate. The "Audit on 2026-05-17" section of the spec
lists 8+ confirmed-broken handlers; `os.user` was not on that list,
which is why phase 5A migrations missed it. Plan-time preflight
**will not catch it** because `os.user` does declare `AsUser` on the
schema side only when the step body sets it — the typical use is
implicit-root, and preflight passes when the running user is root.
The handler still EACCESs when the user runs `mooncake apply`
without sudo.

**Suggested fix:** thread `actions.PrivilegedRunner` through
`runCmd` / `applyPlanLinux` the same way `os.group` does
(`internal/actions/os_group/platform_linux.go:18-66`), and have
`Run` call `ctx.Privileged()` before passing it down. `os.group` is
the cleanest template — same shape, same family.

#### B2. `os.service` declares `Sudo: true` but does not implement `RawRunner` — user `retry:` is silently ignored

- `internal/actions/service/handler.go:93-94` —
  `Permissions()` returns `Sudo: true`.
- `internal/actions/service/handler.go:218` — only `Run` is
  defined; no `RunRaw`.
- `internal/executor/executor.go:1326` —
  `if rr, ok := runner.(actions.RawRunner); ok && …` — service
  takes the legacy path. No executor retry loop applied.

`service` is **not** on the documented exclusion list in
`spec-69-step-execution-pipeline.md` (http_request, download, assert,
wait.*, observe.*, artifact.*). Escalation works fine via
`shared.BecomeAwareCommand` (which is itself a documented exemption
— it needs `BecomeRunner.Command` for the conditional-become +
per-step exit-code capture). But the **retry** path is dropped: a
user writing

```yaml
- name: bounce nginx after a transient systemctl race
  service: { name: nginx, state: restarted }
  retry: { attempts: 3, delay: 5s }
```

gets zero retries. Same shape as the pre-spec-69 pkg / git.clone
bug class, just for one handler.

**Suggested fix:** add a one-line `RunRaw` that delegates to `Run`
(same pattern as `os.user`'s `RunRaw` at
`internal/actions/os_user/handler.go:105`, or
`os.cron`/`os.mount`/`os.group` etc.). This is a 3-line edit; it
opts service into the executor's retry loop without touching the
escalation path. `Run` keeps its existing `BecomeAwareCommand` /
`writeFileWithSudo` calls.

### IMPORTANT

#### I1. `context.TODO()` placeholders are pervasive — no cancellation reaches Privileged.Run

`grep -rn "context.TODO()" internal/actions/` returns 19 hits across
`os_sysctl`, `os_mount`, `os_group/platform_*`, `os_ssh_key`,
`pkg_hold`, `os_firewall`, `download`, `pkg_repo/{apt,dnf}`,
`pkg_upgrade`. Each is a call to `privRunner.Run(context.TODO(),
…)`.

- `c3f4144b`'s commit message says this is the documented placeholder
  for "real context.Context plumbing is a separate spec".
- Real impact today: zero — `PrivilegedRunner.command` checks for
  `nil` ctx and falls back to `exec.Command` without context, but
  `context.TODO()` is non-nil so it actually goes through
  `exec.CommandContext` with a never-cancelled context. Net effect
  identical for cancellation.
- Real impact later: when F016 stage-3 lands (handler-level
  cancellation), every one of these sites needs to be revisited.
  19-site sweep with no compiler help is a re-audit risk.

**Suggested fix:** track as a future spec ("F016 stage-3: thread
real ctx through PrivilegedRunner callsites"). Not blocking spec-69
closeout; just don't lose the entry on the list.

#### I2. Doc drift — spec doc claims phases A/B are done; status section conflates "shipped" with "complete"

- `docs-working/specs/spec-69-step-execution-pipeline.md:3-29` —
  status block lists `http_request`, `download`, `assert`,
  `wait.*`, `observe.*`, `artifact.*` as **intentionally** kept on
  internal retry. **Verified correct** for `download`, `assert`,
  `wait.*`, `observe.*`, `artifact.*`. `http_request`'s rationale
  (single aggregate event per call) is real.
- The doc does **not** list `os.service` as an exemption. Either the
  intent is "service should be on RawRunner and was missed" (which
  matches finding B2) or the doc needs to add it to the exclusion
  list. Per the existing pattern (every other `os.*` is on
  `RawRunner`), the former is more likely.

**Suggested fix:** ship B2's 3-line `RunRaw` and the doc stays
correct.

### NICE-TO-HAVE

#### N1. No end-to-end test of `RawRunner + ctx.Privileged() + retry` together

- `internal/executor/retry_test.go` exercises `runWithRetry` with a
  closure that returns canned `(Result, err)` pairs. Good unit
  coverage of the loop mechanics.
- `internal/executor/preflight_test.go` exercises the plan-time
  permission checks against a synthesized `PermissionSet`. Good
  coverage of the preflight gate.
- **What's missing:** a test that registers a fake action declaring
  `Sudo: true` + `RawRunner`, drives it through `dispatchRunner`
  with `retry: { attempts: 3 }`, and asserts (a) preflight passes,
  (b) `ctx.Privileged()` was invoked for each attempt, (c) the
  retry loop ran the configured number of attempts, (d)
  `applyResultOverrides` ran exactly once post-loop. The spec's
  test plan (`spec-69-step-execution-pipeline.md:377-380`) calls
  for exactly this; the executor-level unit tests stop short of
  wiring it.

**Suggested fix:** add `executor/spec69_pipeline_test.go` with one
fake-action integration test that asserts all four invariants in a
single run. Cheap insurance against future drift.

#### N2. No assertion that non-`RawRunner` handlers' `retry:` is silently dropped

The documented-exemption list (http_request, download, assert,
wait.*, observe.*, artifact.*) keeps internal retry by design. But
the executor doesn't surface a warning when a step targets one of
those actions with `retry:` set — the field just disappears. This
matches the design (`http_request` has its own `retry_on` policy);
but a user upgrading from "all retry is in handlers" to "executor
handles retry" might write `retry:` on an http_request expecting
the executor to honor it.

**Suggested fix:** out of scope for spec-69; consider a
`mooncake plan --strict` warning ("step has retry: but action's
retry is non-portable") as a separate proposal.

#### N3. `runnerOrDefault(runner)` pattern leaves a small back-door

- 9 handlers now take an optional `actions.PrivilegedRunner` and
  default to `security.PrivilegedRunner{}` (zero-value) when nil
  via `runnerOrDefault`. Production calls always pass the real
  runner; tests can pass nil to get a real runner anyway, or pass
  a stub.
- Net: package-level globals are gone, but the "nil → real
  exec.Command" path means a test that forgets to pass a stub can
  silently shell out. This already happens in `os.user` (test
  helpers don't go through the runner at all — `runCmd` doesn't
  exist on a runner). Low-priority.

**Suggested fix:** none right now; revisit if anyone trips on it.

### DOC-DRIFT

#### D1. Several handlers have `Spec-69 phase-5 audit (NOT migrated …)` block comments that document the exemption

Sites with the exemption comment:

- `internal/actions/os_systemd/handler.go:36-44` — broader
  `Command()` API for conditional-become + separate per-sub-step
  exit-code capture.
- `internal/actions/service/shared/shared.go:97-105` (BecomeAwareCommand)
  and `:222-228` (writeFileWithSudo) — same shape as os.systemd's.
- `internal/actions/package/handler.go` — per-call become bool
  because brew runs as the operator while apt runs as root.

These are correct and useful. The opposite — handlers that **were**
migrated — don't carry a similar inline pointer back to spec-69;
they're only discoverable by `git blame`. Not a problem today; flag
if anyone hits the same wall again in 6 months and the spec doc has
moved.

---

## Manual validation results (Docker harness, ubuntu:24.04 as
non-root user `alice` with `sudoers PASSWD: ALL`)

Test fixtures and Dockerfile at `/tmp/mooncake-spec69-test/`. Built
mooncake with `CGO_ENABLED=0 go build -o /tmp/mooncake ./cmd/`,
copied into image, ran each test as alice.

| Test | Scenario | Expected | Actual | Verdict |
|------|----------|----------|--------|---------|
| T1 | `pkg` install (apt) via `ctx.Privileged()` | success, change | success, change | ✅ |
| T2 | `os.cron` write to `/etc/cron.d` via `ctx.Effects(Become:true)` | success, change | success, change | ✅ |
| T3c | `os.user` create, as_user=root, --sudo-pass set | failure (B1 EACCES) or success | **silent success, user NOT created** (B0+B1) | ❌ B0+B1 |
| T4 | `cmd` retry × 3 with exponential backoff (fail 2x then succeed) | 3 attempts, 200ms + 400ms sleeps, success | exactly 3 attempts, 200ms + 400ms sleeps, success | ✅ |
| T5 | `os.service` start of nonexistent unit, `retry: { attempts: 3 }` | (B2) 1 attempt, fail | 1 attempt, fail | ❌ B2 confirmed |
| T6 | `pkg` install as non-root, NO --sudo-pass | plan-time preflight failure | preflight error, "no sudo password is configured" | ✅ |
| T7 | `cmd: exit 1` with `retry: { attempts: 3 }` + `failed_when: false` | 4 attempts (1+3), step reported success | 4 attempts, step ok, error masked | ✅ MT-48 holds |
| T8 | `pkg` install of nonexistent package | failure | failure, `failed=1` | ✅ |
| T9 | `pkg.upgrade` with `manager: not_a_real_manager` | failure | **silent success, `ok=1 changed=0`** (B0) | ❌ B0 |
| T10 | `shell: cmd: "exit 1"` | failure | failure, `failed=1` | ✅ |

**Headline results:**

- ✅ Escalation primitives (`ctx.Privileged()`, `ctx.Effects()`)
  work end-to-end (T1, T2).
- ✅ Retry loop + exponential backoff work end-to-end (T4).
- ✅ Plan-time preflight fires with actionable message (T6).
- ✅ MT-48 invariant (retry on raw err + failed_when:false mask)
  preserved (T7).
- ❌ **B0 confirmed**: T3c + T9 both report `ok=1 changed=0` while
  the underlying action visibly failed. The bug is in
  `dispatchRunner`, not in the migrated handlers themselves.
- ❌ **B1 confirmed**: T3c's strace shows `useradd` invoked
  directly without sudo wrapping (`execve("/usr/sbin/useradd",
  ["useradd", "--create-home", "spec69probe"], ...)` with no
  intervening `sudo` execve). This was masked by B0 — fixing B0
  alone would surface T3c as a loud EACCES failure, but B1 still
  needs the same `os.group`-style runner threading.
- ❌ **B2 confirmed**: T5's log shows exactly one
  `Running systemctl start nonexistent-spec69-probe` despite
  `retry: { attempts: 3 }`.

**To reproduce locally:**

```
cd /tmp/mooncake-spec69-test
docker build -t mooncake-spec69-test .
docker run --rm --entrypoint /usr/local/bin/mooncake \
  mooncake-spec69-test apply -c /tests/t9-pkg-upgrade-bad.yml \
  --sudo-pass testpass --insecure-sudo-pass
# Watch: RECAP ok=1 changed=0 failed=0 (should be failed=1)
```
