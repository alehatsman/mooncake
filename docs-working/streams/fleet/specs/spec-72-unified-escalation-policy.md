# Spec 72: Unified Escalation Policy — one decision point, one execution path, one diagnostic

**Epic:** Personal Fleet + cross-cutting handler ABI (Spec-22, Spec-69
lineage). Direct follow-up to **[F051](../../../code-review/findings/F051-sudo-escalation-fragmented-across-call-sites.md)**.
**Status:** Draft
**Effort:** M–L (~1–2 weeks across phases). The bulk is the audit +
migration of the 12 nil-guard `PrivilegedRunner{}` sites and the
six handler packages with hand-rolled escalation construction; the
new interface itself is small.
**Value:** High — the five sudo-related bugs surfaced over a single
afternoon on 2026-05-18 (commits 35bdc055, 600b619d, c3e0897f,
b01d261f, plus a manual chown) were point fixes against a structural
gap. The next change that touches the escalation story (new agentd
identity, a handler that needs root, a host with stricter sudoers,
a switch off systemd) will produce a new variant of the same class
of bug unless escalation is owned by one component.
**Depends on:** the `--user` agentd patch and the spec-70
local-bootstrap extraction already in tree.

---

## Problem

Today there is no single owner of "escalate to root when this step
needs it." The actual `sudo` invocation logic is centralized — one
`security.BecomeRunner.Command` builds the `*exec.Cmd` — but the
**inputs** that determine whether escalation is *allowed*, *necessary*,
or *possible* are read from `RunServices` independently at every call
site. Five concrete bug variants from F051:

1. Preflight required a configured `SudoPass` even when `sudo -n true`
   would have worked under NOPASSWD.
2. The user-mode unit template set `NoNewPrivileges=true`, which blocks
   setuid execution → sudo refuses with the NNP message regardless of
   sudoers / passwords / probes.
3. Three handlers constructed `BecomeRunner{SudoPass: ec.Svc.SudoPass}`
   directly without `PasswordlessSudo`, bypassing the central decision.
4. The user-mode unit's `Environment=PATH=` omitted `~/.local/bin`,
   so shell steps invoking user-installed tools (`claude`, etc.) exited
   127 even though escalation itself worked.
5. The sudoers file was owned by `aleh` instead of `root`, which made
   sudo's behavior context-dependent in ways that the agentd's probe
   couldn't surface ("sudo -n true failed" — no further detail).

The unifying property of all five: an input the escalation system
*depends on* changed in a way the *decision logic* didn't notice.

There are also **12 nil-guard `PrivilegedRunner{}` sites** in
`os_user`, `os_group`, `os_ssh_key`, `os_firewall`, `os_mount`,
`os_sysctl`, `pkg_hold`, `pkg_upgrade`, `pkg_repo/{apt,dnf}` that
each comment claims is "test fallback only," but the claim has not
been audited. If even one is reachable from production, the failure
mode is the same as bug #3 above.

---

## Goals

- **G1** Exactly one constructor for an escalation primitive in
  production handler code: `ctx.Privileged()` (or its equivalent —
  the rename happens in this spec). All other constructors of
  `security.BecomeRunner` / `security.PrivilegedRunner` outside
  `internal/security/` and `internal/executor/context.go` are forbidden
  and rejected by a lint rule.
- **G2** Escalation feasibility is computed once per run, with a
  *reason* attached. The reason is one of an enumerated set:
  `available_root` (already uid 0), `available_password` (SudoPass
  set), `available_passwordless` (NOPASSWD), `blocked_nnp`,
  `blocked_sudo_missing`, `blocked_sudoers_insecure`,
  `blocked_probe_failed`. The result + reason live on `RunServices`
  and feed both preflight and the actual `BecomeRunner.Command`.
- **G3** The diagnostic surfaced when a step fails preflight names
  the specific blocker. "Step X requires root but escalation is
  unavailable: NoNewPrivileges=true on the systemd unit (drop the
  directive — see spec-72 §4)" — not "sudo -n true failed."
- **G4** Systemd-unit-template knobs that affect escalation are
  enumerated and pinned by tests. At minimum:
  `NoNewPrivileges=`, `Environment=PATH=`, `User=`, `Group=`,
  `CapabilityBoundingSet=`, `AmbientCapabilities=`, `RestrictSUIDSGID=`.
  A future "hardening tweak" that re-adds NNP to the user-mode unit
  fails CI immediately.
- **G5** The 12 nil-guard `PrivilegedRunner{}` sites are either
  deleted (test injection passes a real runner constructed from a
  fake `RunServices`) or migrated to a single helper that takes
  `*RunServices` and produces a properly-configured runner —
  never an empty struct.

**Non-goals:**

- **Replacing sudo with capabilities / setuid binaries / polkit.** A
  proper privilege model is out of scope; the work here is unifying the
  glue around sudo, not eliminating it.
- **Cross-platform parity beyond Linux + macOS.** Windows takes a
  fundamentally different escalation path (S4U-principal scheduled
  tasks) that doesn't share `sudo` semantics. The unified-policy
  abstraction stays Unix-only; Windows handlers keep their existing
  `winutil`-shaped flow.
- **Probing the sudoers file directly.** The probe stays "what can
  this process actually do" (`sudo -n true`), not "what does
  /etc/sudoers say." Sudoers parsing is sudo's job; we're a client.

---

## Reuse map

**Reused:**

- `internal/security/become.go:BecomeRunner` — the sudo command
  builder. No changes to its semantics; this spec changes *who*
  constructs it and *how* the inputs flow in.
- `internal/security/privileged.go:PrivilegedRunner` — the
  `actions.PrivilegedRunner` interface implementation. Becomes the
  *only* allowed constructor target outside the security package.
- `internal/executor/preflight.go:detectPasswordlessSudo` — the
  `sudo -n true` probe. Stays, but grows a sibling that returns
  the reason for failure when the probe fails (parse stderr).
- `internal/executor/context.go:Privileged()` — already wires
  `ec.Svc.SudoPass + ec.Svc.PasswordlessSudo`. Stays; gains a
  `Reason()` method on the returned runner so callers can ask "why
  not?" instead of just "yes/no."

**Replaced / extracted:**

- The four hand-rolled `BecomeRunner` constructs in
  `internal/actions/{package, service/shared, os_systemd}` (already
  fixed in c3e0897f to pass `PasswordlessSudo`) get migrated to
  `ctx.Privileged()`. The "per-call become bool" carve-out documented
  in `pkg.runCmd` (brew runs without sudo, apt runs with sudo) is
  preserved — `Privileged()` already supports `become=false` via its
  `become` argument; the handlers just lose the direct constructor.
- The 12 empty `PrivilegedRunner{}` nil-guard fallbacks audit:
  prove each one is test-only (and migrate test injection to construct
  a real runner from a fake `RunServices`), or migrate to a shared
  `runnerFromRunServices` constructor.

---

## Design

### 1. The escalation report

Add a typed result that's computed once at `RunServices`
construction and read everywhere else:

```go
// EscalationReport captures the once-per-run answer to "can this
// process escalate to root, and if not, why not?". Computed by the
// executor at run startup and stored on RunServices; consumed by
// preflight, BecomeRunner.Command, and the diagnostic in
// step-error events.
type EscalationReport struct {
    // Available is true iff a Sudo+AsUser step can be expected to
    // succeed. The union of every "available_*" Reason.
    Available bool

    // Reason explains the verdict. Stable across a run.
    Reason EscalationReason

    // Detail carries reason-specific extra info: the sudo binary
    // path (when found), the stderr from `sudo -n true` (when the
    // probe failed), the systemd directive that's blocking (when
    // we can detect it from /proc/self/status). Free-form string,
    // safe to include in diagnostic output.
    Detail string
}

type EscalationReason int

const (
    EscalationAvailableRoot         EscalationReason = iota // uid 0 already
    EscalationAvailablePassword                              // SudoPass set
    EscalationAvailablePasswordless                          // NOPASSWD probe succeeded
    EscalationBlockedNNP                                     // /proc/self/status NoNewPrivs=1
    EscalationBlockedSudoMissing                             // exec.LookPath("sudo") failed
    EscalationBlockedSudoersInsecure                         // sudo -n true exit ≠ 0 with "owned by uid" stderr
    EscalationBlockedProbeFailed                             // generic probe failure
)
```

Computed in a new `internal/executor/escalation.go`:

```go
func ProbeEscalation(ctx context.Context, sudoPass string) EscalationReport
```

The probe sequence:

1. If `os.Geteuid() == 0` → `AvailableRoot`.
2. If `sudoPass != ""` → `AvailablePassword` (we trust the caller; we
   don't validate the password here because that requires actually
   trying it against a real sudoers rule, which has side effects).
3. Read `/proc/self/status` (Linux only); if `NoNewPrivs:` is `1`,
   return `BlockedNNP` with the directive in `Detail`.
4. `exec.LookPath("sudo")`; if missing → `BlockedSudoMissing`.
5. Run `sudo -n true` (2s timeout, nil-ctx guard); on success →
   `AvailablePasswordless`. On failure, inspect stderr:
   - `owned by uid` → `BlockedSudoersInsecure`.
   - anything else → `BlockedProbeFailed` with stderr in `Detail`.

`RunServices.PasswordlessSudo` is replaced by
`RunServices.Escalation EscalationReport`. The shadow boolean is
kept as a method (`Escalation.Available`) so call sites that just
need the yes/no answer don't have to switch on the enum.

### 2. The single allowed constructor for escalation runners

`actions.Context` already exposes `Privileged()`. This spec makes it
the *only* allowed path:

- `BecomeRunner` and `PrivilegedRunner` move to an internal-only API.
  Outside `internal/security/` and `internal/executor/context.go`,
  construction is rejected by lint.
- The four hand-rolled sites identified in F051 migrate to
  `ctx.Privileged()`. The `pkg.runCmd`'s "become decided per call"
  pattern continues to work because `actions.PrivilegedRunner.Run`
  takes a `become bool` (or equivalent shape — see open question §1).

A new linter check in `scripts/ai-lint.sh` (or a separate `task ci`
target) greps for `security\.BecomeRunner\{|security\.PrivilegedRunner\{`
outside the allowed files and fails with a finger-pointer to spec-72.

### 3. Preflight + BecomeRunner consume the report

`preflightPermissions` (executor/preflight.go) takes
`*EscalationReport` instead of a `sudoAvailable bool`. Failure path:

```go
if !report.Available {
    return fmt.Errorf(
        "step %q requires elevated privileges (as_user: %s, Sudo: true) but escalation is unavailable: %s (%s); see %s",
        stepLabel(step), step.AsUser,
        report.Reason.String(),
        report.Detail,
        report.Reason.RemediationDocURL(),
    )
}
```

Each `EscalationReason` carries a remediation hint (a docs anchor or a
one-liner):
- `BlockedNNP`: "drop NoNewPrivileges=true from your systemd unit"
- `BlockedSudoMissing`: "install sudo or run mooncake as root"
- `BlockedSudoersInsecure`: "fix file ownership / mode under /etc/sudoers.d/"
- `BlockedProbeFailed`: "check sudoers rules; raw stderr above"

`BecomeRunner.Command` no longer reads `SudoPass + PasswordlessSudo`
flags — it takes a `*EscalationReport` and a `become bool`. The
sudo-`-S` vs sudo-`-n` choice is driven by `report.Reason`.

### 4. Pin the systemd-unit knobs by test

A new test file `internal/fleet/install/unit_security_test.go`
asserts the matrix of allowed/disallowed directives in each unit
template:

| Directive | System unit | User unit |
|---|---|---|
| `User=root` | required | forbidden |
| `Group=root` | required | forbidden |
| `NoNewPrivileges=true` | allowed | **forbidden** (blocks sudo) |
| `Environment=PATH=` containing `%h/.local/bin:` | n/a | **required** |
| `ReadWritePaths=` | required (for self-upgrade) | forbidden (irrelevant) |
| `WantedBy=multi-user.target` | required | forbidden |
| `WantedBy=default.target` | forbidden | required |
| `RestrictSUIDSGID=true` | allowed | **forbidden** (would also block sudo) |
| `CapabilityBoundingSet=` (without `CAP_SETUID, CAP_SETGID`) | allowed | **forbidden** |
| `AmbientCapabilities=` | allowed | allowed (no escalation impact) |
| `ExecStart=` (no `--system` flag) | n/a | required |

The test runs against both rendered templates and flags any violation
with a pointer to spec-72.

### 5. Audit the 12 nil-guard sites

Each empty `PrivilegedRunner{}` literal listed in F051 gets one of:

- **Verified test-only**: deleted from production code; test injection
  refactored to pass a runner constructed from a fake `RunServices`.
- **Reachable from production**: migrated to call
  `ec.Privileged()` from the handler entry point and thread the
  result through. No new construction site.

The audit is a per-handler exercise — each handler has its own
shape (some take an injected runner as a struct field, some via a
function arg) — but the migration target is the same.

---

## Test plan

### Unit

- `internal/executor/escalation_test.go` — `ProbeEscalation` covers
  all seven `Reason` branches with mocked `sudo`, `/proc/self/status`,
  euid. Includes table of (env, expected `Reason`).
- `internal/executor/preflight_test.go` (extension) —
  `preflightPermissions(report, step)` errors carry the reason name
  and remediation hint.
- `internal/security/become_test.go` (extension) —
  `BecomeRunner.Command` builds `-S` for `AvailablePassword` and `-n`
  for `AvailablePasswordless`; rejects other reasons with a typed
  error.
- `internal/fleet/install/unit_security_test.go` — the directive
  matrix from §4.

### Integration

- `internal/executor` end-to-end: a fake `RunServices` with each
  `EscalationReport` shape; verify a `Sudo + AsUser` step succeeds
  on every `available_*` reason and fails with the correct
  diagnostic on every `blocked_*` reason.

### Cross-platform smoke

- macOS: `sudo -n true` works equivalently; the `/proc/self/status`
  probe for NNP returns "not applicable" cleanly (no Linux-only
  path crashes).
- Windows: spec scope excludes Windows; existing
  `bootstrapWindows` path is untouched. Test that constructing
  `EscalationReport` on Windows returns `AvailableRoot` (the agentd
  runs as the configured user via S4U; sudo concepts don't apply)
  and that escalation calls become no-ops on that branch.

### Lint

- New regex check in `scripts/ai-lint.sh`: forbid
  `security\.BecomeRunner\{|security\.PrivilegedRunner\{` outside
  the allowed files. Hits all 12 nil-guard sites until they're
  migrated, which makes the migration a *measurable* effort
  (countdown of forbidden literals to zero).

---

## Migration

The original 5-phase plan was revised after Phase 2b. Investigation
revealed the actual design flaw: the `actions.PrivilegedRunner`
interface was doing two unrelated jobs ("run a command, get bytes"
for 20 callers, and "build an exec.Cmd I can manipulate" for 3
callers) and 13 `runnerOrDefault` nil-guards existed purely to
satisfy a test-injection workaround that production never triggered.
The real fix was an architectural Layer-shift (the "Layer C"
discussion in the spec history): escalation becomes a property of
the *step*, bound by `dispatchRunner` onto the per-step ctx, with
handlers describing intent and the primitive consulting its bound
`AsUser` to decide the sudo wrap.

The phasing collapsed into three commits:

1. **Phase α — foundation (shipped).** Move `EscalationReport`,
   `EscalationReason`, and `ProbeEscalation` from `internal/executor`
   to `internal/security` so the new primitive can live next to its
   inputs without forcing `internal/actions` to import
   `internal/executor`. Add the concrete `security.Privileged`
   struct (`SudoPass`, `Escalation`, `AsUser`) with two methods —
   `Run(ctx, prog, args)` and `Command(ctx, prog, args)`. `AsUser`
   semantics:
   - `""`           → run as current process (no sudo)
   - `"root"`/`"0"` → sudo (no-op when already root)
   - `"<name>"`     → sudo -u `<name>` (no-op when already `<name>`)
   `dispatchRunner` sets `ec.CurrentAsUser = step.AsUser` before
   `runner.Run`, restoring on defer. `ec.Privileged()` returns
   `*security.Privileged` with all three fields bound from
   `ec.Svc` + `ec.CurrentAsUser`.
2. **Phase β — handler migration (shipped).** Drop the
   `actions.PrivilegedRunner` interface entirely; `ec.Privileged()`
   returns the concrete `*security.Privileged`. The four mock
   contexts (testutil.MockContext, apply.reverseContext,
   actions_test.mockContext, print.mockContext) construct a real
   `*security.Privileged` with `AvailableRoot` escalation instead
   of implementing a fake interface. The 10 `runnerOrDefault`
   helpers and their 13 nil-guard `security.PrivilegedRunner{}`
   literals are deleted — production never reached them, tests
   substitute the var-hook entirely. Per-call `become bool`
   parameters in `pkg.runCmd`, `service/shared.BecomeAwareCommand`,
   `os_systemd.becomeCommand`, and the effects layer all collapse:
   the bound `AsUser` is the single source of truth.
3. **Phase γ — cleanup + lint flip (shipped).** Drop
   `RunWithBecome` and `RunWithInput` from the old interface
   (zero callers after migration). Flip
   `scripts/escalation-lint.sh` to `--fail` in `task ci`. Lint
   baseline: **0 production violations** (down from 16 at the
   end of Phase 2b).

What this means for the originally-scoped Phase 3 / Phase 4 /
Phase 5: the 13-site audit (Phase 3) disappeared — those sites
were deleted along with `runnerOrDefault`. The CI-blocker flip
(Phase 5) shipped as part of Phase γ. The unit-knob test matrix
(Phase 4) shipped separately as
`internal/fleet/install/unit_security_test.go`: five tests assert
the spec-72 §4 directive matrix against the rendered system and
user templates, with each forbidden/required directive carrying
the F051 sub-bug it guards (F051-b for NNP, F051-d for
PATH=%h/.local/bin, etc.). A future "hardening tweak" that
re-adds NoNewPrivileges=true to the user unit fails CI before
the change can land.

**Latent feature unlocked**: `as_user: <name>` (non-root named
user) now works universally across handlers, not just in
`command` / `shell`. Previously most handlers interpreted
`as_user: postgres` as "escalate to root" because the underlying
escalator only knew about root. The Layer C primitive supports
the named-user case via `sudo -u <name>`.

**Filesystem write-as-named-user** also works: `file.write` /
`file.template` / `file.copy` / `os.cron` / `os.sysctl` /
`os.ssh_key` with `as_user: postgres` produce files owned by
postgres. The effects layer's `defaultPerformer` carries the
bound `AsUser` and every sudo create-path (WriteFile, CopyFile,
Mkdir, Touch, Symlink, Hardlink) appends a `chown <uid>:<gid>`
clause to the same sudo invocation. The `Become` /
`BecomeUser` fields on `PerformerOpts` (previously dead-and-
vestigial) are removed; handler call sites that passed
`Become: step.ShouldBecome()` collapse to just the remaining
opts (Force, ExplicitMode).

**Direct-write ownership** (previously documented as a caveat,
now resolved): when AsUser is set and doesn't match the current
process's user, the Performer skips the direct path entirely and
goes through sudo so the bundled chown clause produces the
correct owner. `defaultPerformer.needSudoForOwnership()` is the
gate — it returns true for any AsUser that wouldn't be satisfied
by a direct write, including AsUser=root from a non-root process
and AsUser=named-non-current. WriteFile, CopyFile, Mkdir, Touch,
Symlink, and Hardlink all consult it. The "AsUser=current"
fast path stays for matching cases (no unnecessary sudo wrap).

Backward compatibility: zero protocol changes (peers.toml, agentd
HTTP, daemon config); the work is structural and inside the executor.

---

## Open questions

1. ~~**`PrivilegedRunner.Run` signature.**~~ **Resolved (Layer C
   redesign):** the question was rendered moot by dropping the
   `PrivilegedRunner` interface entirely. The Phase 2a answer
   (additive `RunWithBecome`) and Phase 2b answer (additive
   `Command(become bool, ...)`) both shipped but were superseded
   when investigation showed the per-call `become bool` parameter
   was conflating two concepts: the step's declared `AsUser`
   (a per-step property) was being read by every handler and
   threaded through helper signatures as a boolean. Layer C
   binds `AsUser` to the primitive once per step via
   `dispatchRunner`; handlers stop reading `step.AsUser` for
   execution decisions; helper signatures shrink. `RunWithBecome`
   and `RunWithInput` were deleted in Phase γ.
2. **Cached `EscalationReport` vs re-probe under controller
   instructions.** If the operator passes `--sudo-pass` mid-run-life
   (a future feature?), should the cached report invalidate? v1
   says "no, immutable per run" — same shape as the
   `PasswordlessSudo` cache today.
3. ~~**`Sudoers` ownership probe wording.**~~ **Resolved
   (spec-72 follow-up):** kept the typed `BlockedSudoersInsecure`
   reason and expanded the match heuristic to cover the common
   distro variants — "owned by uid" (Debian/Ubuntu/Fedora classic),
   "bad permissions", "should be mode" (mode-mismatch variants),
   "is world writable", "is not a regular file". The typed
   remediation ("fix file ownership/mode under /etc/sudoers.d/")
   is more useful than collapsing into the generic ProbeFailed
   bucket; novel distro wordings still fall through to
   `BlockedProbeFailed` with the raw stderr in `Detail`. Match
   logic lives in `isSudoersInsecureStderr` so adding a new
   fingerprint when an operator hits one is a one-line PR.
4. **Polkit-managed actions** (loginctl enable-linger, etc.) live
   outside sudo entirely. F051 noted them as adjacent. Out of scope
   for this spec, but worth a follow-up: a parallel
   `PolkitEscalation` primitive that handlers needing user-bus
   operations consume.
5. **NNP detection portability.** `/proc/self/status` is Linux-only.
   macOS has no equivalent that I'm aware of (sudo there is gated by
   the credential cache + sudoers, not by systemd-style hardening).
   Decide whether the `BlockedNNP` reason is Linux-specific or whether
   we want a more general "this process is hardened against
   escalation" reason that subsumes future cases (capability sets,
   SELinux denials, AppArmor, etc.).

---

## Out-of-scope follow-ups

- Migrating to a non-sudo escalation primitive (capability sets,
  setuid helpers, polkit-only). Tracked separately.
- The `becomeFallback` "try direct first, sudo on EACCES" pattern
  in `defaultPerformer` is *distinct* from the explicit-become
  path. It's a different question (when does the daemon try direct
  vs always-escalate?) and warrants its own spec if we want to
  unify those policies.
- Auditing the `effects.PerformerOpts.Become` flag's call sites for
  consistency with the new `EscalationReport`. The flag is mostly
  used inside the effects package (Mkdir, WriteFile, etc.); the
  internal call sites already consume `p.sudoPass + p.passwordlessSudo`
  via the constructor.

---

## Pickup checklist (for the implementing agent)

1. Read **[F051](../../../code-review/findings/F051-sudo-escalation-fragmented-across-call-sites.md)** end-to-end. It catalogs every site and the
   five bug variants this spec exists to prevent recurring.
2. Phase 1 (`EscalationReport` + probe) is mechanical and a good
   warm-up. Verify the existing `detectPasswordlessSudo` callers
   still pass through unchanged.
3. Phase 3 (12-site audit) is the bulk. Recommend one commit per
   handler package; each commit either deletes the nil-guard or
   migrates the helper signature.
4. Phase 5 (lint flip) ships only after the 12-site audit completes.
   The lint regex from Phase 2 will show the countdown as commits
   land.

Claim spec-72 in `~/.mooncake/claims.jsonl` before starting (per
mooncake's `CLAUDE.md`). Worktree:
`git worktree add ../mooncake-spec-72 -b worktree-spec-72`.
