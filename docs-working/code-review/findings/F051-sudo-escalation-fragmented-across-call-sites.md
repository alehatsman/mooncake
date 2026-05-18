# F051 — sudo escalation is fragmented across 6+ call sites with diverging invariants

**Filed**: 2026-05-18 by assistant (after a same-day cycle of five distinct sudo-related bugs surfaced switching the dotfiles agentd from system unit + root to user unit + `aleh`)
**Severity**: smell (architectural) — but the symptoms have been five concrete bugs over a few hours, so the fragmentation actively produces wrong behavior
**Component**: cross-cutting — `internal/security/{become.go, privileged.go}`, `internal/effects/default.go`, `internal/executor/{context.go, preflight.go, executor.go}`, ~5 handler packages, the systemd unit templates under `internal/fleet/install/init/`
**Status**: **open** — point fixes for the five bugs shipped (35bdc055, 600b619d, c3e0897f, b01d261f); consolidation tracked in **spec-72** (to be drafted alongside this finding)

---

## Summary

There is no single place that owns "escalate to root if needed." Every escalation path is built up from at least four moving inputs (`SudoPass`, `PasswordlessSudo`, `isRoot` / `runningElevated()`, and the systemd-unit-level `NoNewPrivileges=` / `Environment=PATH=` choices), and those inputs are read independently at every call site. When the inputs are correct everywhere, escalation works. When *any* of them is wrong at *any* site, the failure mode is "this one handler refuses to escalate" — not "the executor reports a single misconfiguration up front."

We saw this exact pattern five times in one afternoon while flipping the dotfiles agentd from a system-scope unit running as root to a user-scope unit running as `aleh`:

1. **F051-a (preflight wrong invariant)**. `preflightPermissions` (executor/preflight.go) refused any `Sudo + AsUser` step unless `RunServices.SudoPass` was non-empty — it didn't probe `sudo -n` to detect NOPASSWD. **Fix:** 35bdc055.
2. **F051-b (unit-template-level block)**. The user-mode `mooncake-agentd.user.service` template inherited `NoNewPrivileges=true` from the system unit. NNP blocks setuid execution → sudo refuses with "The 'no new privileges' flag is set, which prevents sudo from running as root", *regardless of any in-code logic*. **Fix:** 600b619d.
3. **F051-c (handler bypasses `ctx.Privileged()` ×3)**. Three handlers (`package`, `service/shared`, `os_systemd`) constructed `security.BecomeRunner{SudoPass: ec.Svc.SudoPass}` directly without `PasswordlessSudo`. Preflight let the step through; the actual escalation call hit `ErrBecomeNoSudoPass`. **Fix:** c3e0897f.
4. **F051-d (unit-template PATH gap)**. The user-mode unit's PATH did not include `%h/.local/bin`, so a shell step calling `claude mcp add …` exited 127 — `claude` is at `~/.local/bin/claude` and bash invoked non-interactively doesn't source `~/.bashrc`. **Fix:** b01d261f.
5. **F051-e (sudoers file ownership)**. A previous half-run created `/etc/sudoers.d/aleh-nopasswd` owned by `aleh:aleh` (not `root:root`). sudo's behavior with that ownership was inconsistent across contexts — interactive shell worked due to group `sudo` membership, but the agentd's `sudo -n` probe failed. Dotfiles-side fix: `file.write` step with `as_user: root` would have done this correctly *once sudo worked*; bootstrap loop fixed by manual `sudo chown root:root`. **Not a mooncake bug per se**, but it surfaced via the agentd because there's no probe-time diagnostic that says "sudoers is non-functional."

Each fix is correct in isolation. The pattern is the smell.

## Inventory: every site that escalates (or decides to escalate)

The actual sudo *invocation* logic lives in **one** place: `security.BecomeRunner.Command` (`internal/security/become.go:64`). Good. But who *constructs* a `BecomeRunner` (or its sibling `PrivilegedRunner`) and with what inputs is scattered:

### Canonical paths (consume `RunServices` correctly today)

| Site | File:line | Inputs |
|---|---|---|
| `ExecutionContext.Privileged()` | `internal/executor/context.go:236` | `SudoPass: ec.Svc.SudoPass, PasswordlessSudo: ec.Svc.PasswordlessSudo` ✓ |
| `ExecutionContext.Effects()` → `effects.NewPerformer` | `internal/executor/context.go:228`, `internal/effects/default.go:44` | `(ec.Mode, ec.Svc.SudoPass, ec.Svc.PasswordlessSudo)` ✓ |
| `defaultPerformer.runSudo` → `BecomeRunner` | `internal/effects/default.go:759` | `SudoPass: p.sudoPass, PasswordlessSudo: p.passwordlessSudo` ✓ |

### Hand-rolled paths (each was a bug source until c3e0897f)

| Site | File:line | What was wrong before today | Correct now? |
|---|---|---|---|
| `pkg.Handler.runCmd` | `internal/actions/package/handler.go:340` | Missing `PasswordlessSudo` | yes (c3e0897f) |
| `service/shared.runShellSudo` (×2 sites) | `internal/actions/service/shared/shared.go:107, 230` | Missing `PasswordlessSudo` | yes (c3e0897f) |
| `os_systemd` ExecAction | `internal/actions/os_systemd/handler.go:202` | Missing `PasswordlessSudo` | yes (c3e0897f) |

### Nil-guard fallback paths (test-only on paper, but unverified)

These construct an **empty** `security.PrivilegedRunner{}` — no `SudoPass`, no `PasswordlessSudo`, no `isRoot` awareness. The comment in each says it's a fallback for nil-runner test injections. If any of them is reachable from production at any point, the bug class is identical to F051-c.

| Site | File:line |
|---|---|
| `os_user/platform_darwin.go:267, 286` |
| `os_user/platform_linux.go:81` |
| `os_group/platform_linux.go:64` |
| `os_group/platform_darwin.go:168` |
| `os_ssh_key/handler.go:33` |
| `os_firewall/handler.go:57` |
| `os_mount/handler.go:32` |
| `os_sysctl/handler.go:57` |
| `pkg_hold/handler.go:429` |
| `pkg_upgrade/handler.go:315` |
| `pkg_repo/apt/apt.go:31` |
| `pkg_repo/dnf/dnf.go:35` |

These were **not audited** for production reachability — the cleanup commit c3e0897f only touched the four sites that produced observable failures in today's dotfiles run. **Action item:** verify the nil-guard claim for each by tracing callers from the action's entry point.

### Decision points (where escalation is allowed or rejected up front)

| Site | File:line | Inputs |
|---|---|---|
| `preflightPermissions` | `internal/executor/preflight.go:34` | `sudoAvailable = SudoPass != "" \|\| PasswordlessSudo` (post-35bdc055) |
| `detectPasswordlessSudo` | `internal/executor/preflight.go:81` | Probes `sudo -n true` with 2s timeout, nil-ctx guard, already-root short-circuit |
| `BecomeRunner.Command` | `internal/security/become.go:64` | `become bool, SudoPass, PasswordlessSudo` |
| `IsBecomeSupported()` | `internal/security/platform_*.go` | GOOS check |
| `becomeFallback` | `internal/effects/default.go:67` | "try direct first, sudo on EACCES" — **distinct policy** from the rest |

### External factors mooncake doesn't model

These bit us today but live outside the Go code:

| Factor | Where | Failure mode |
|---|---|---|
| systemd unit `NoNewPrivileges=` | `internal/fleet/install/init/mooncake-agentd*.service` (template choice) | sudo refuses with NNP message, no in-process detection possible |
| systemd unit `Environment=PATH=` | same templates | `sudo` itself may be findable but child commands (`claude`, `apt`, user-installed tools) are not |
| `/etc/sudoers.d/<file>` ownership / mode | external | sudo silently downgrades / refuses based on file permissions; the probe `sudo -n true` reports failure with no signal about *why* |
| `sudo -n` vs `sudo -S` semantics | external | `-n` fails fast if password needed; `-S` reads password from stdin. We choose one or the other (correctly) at `become.go:80`, but the distinction is implicit |
| Polkit rules for `loginctl enable-linger` etc. | external | Independent of sudo, but conceptually the same "can this process do X" question — answered by different machinery |

## Why this finding exists

The five bugs above are point fixes. The next change that touches sudo — agentd running as a *third* identity, a new handler that needs root, a deployment to a host with a less-permissive sudoers, a switch from systemd to a different init system — will surface a new variant of the same fragmentation problem unless we consolidate.

Symptoms to watch for that indicate this finding's root cause is biting again:

- Two adjacent handlers, both declaring `as_user: root`, with one succeeding and the other failing on the same host.
- "Works locally, fails via fleet apply" (or vice versa) on the same plan.
- A new failure surfaces only after a unit-template change that didn't touch any handler code.
- A handler's escalation behavior depends on whether `--sudo-pass` is set, even though the host doesn't need a password.

## Suggested fix shape (defer to spec-72)

Not in this finding. The right output is a spec that:

1. Makes `ctx.Privileged()` the single allowed escalation primitive for handler code. Direct `BecomeRunner` / `PrivilegedRunner` construction outside `internal/security/` and `internal/executor/context.go` is forbidden by lint.
2. Centralizes the "can we escalate at all" decision: one probe at run startup that records the result on `RunServices` *with the reason* (`reachable`, `noNewPrivileges`, `noNopasswdRule`, `sudoersInsecure`, `notInstalled`, ...). Preflight, `BecomeRunner.Command`, and the diagnostic error messages all consult that one record.
3. Moves the systemd-unit-template concerns (NNP, PATH) into the same conceptual space as the in-code escalation policy — at minimum, document that the unit-template choices are part of "how mooncake escalates" and pin them via tests, which b01d261f / 600b619d did for the two specific knobs that bit us but the test list isn't complete (e.g. `Environment=`, `User=`, `Group=`, `CapabilityBoundingSet=`, `AmbientCapabilities=`, `RestrictSUIDSGID=` are all knobs that could re-introduce a sudo-blocking surprise).
4. Audits the 12 empty-`PrivilegedRunner{}` nil-guard sites for production reachability and either deletes them (test injection should pass a real runner) or unifies them on a constructor that takes `ec.Svc`.

The full design (interface shape, lint rule, probe semantics, error taxonomy, migration plan) belongs in spec-72. This finding documents the gap; the spec proposes the consolidation.

## Verification of the point fixes shipped today

Today's commits handle the five concrete bugs but do not address the structural issue:

- `35bdc055` — preflight accepts NOPASSWD via probe.
- `600b619d` — user-mode unit drops `NoNewPrivileges=`.
- `c3e0897f` — three handlers thread `PasswordlessSudo` through direct `BecomeRunner` constructs.
- `b01d261f` — user-mode unit prepends `%h/.local/bin` to PATH.
- Manual: `/etc/sudoers.d/aleh-nopasswd` chowned to `root:root` on main_pc.

End-to-end smoke: `mooncake fleet apply main_pc` from the controller now completes against the user-mode agentd. That confirms the five point fixes are individually correct. It does **not** confirm the absence of a sixth variant lurking in one of the 12 unaudited nil-guard sites or in a unit-template knob we haven't pinned.

## Related findings

- **F004** — service handler had its own `becomeAwareCommand` helper duplicating `BecomeRunner`. Closed by routing all service inline sites through `BecomeRunner`. The fact that *another* round of the same pattern surfaced in 2026-05-18 (this finding) suggests F004's resolution was incomplete: it migrated the duplications but didn't establish a structural barrier against new duplications. A lint rule (proposed in spec-72 §1) would close that.
- **F005** — cross-package "shell out with sudo -S" had 6 implementations. Closed 2026-05-17. Same lineage as this finding; the difference is F005 was about *invocation* duplication (multiple `exec.Command("sudo", ...)` literals), this is about *configuration* duplication (multiple readers of the escalation inputs).
