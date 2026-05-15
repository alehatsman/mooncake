# Spec 57: `windows_firewall_rule` + `windows_scheduled_task` actions

**Status:** Draft
**Epic:** E9 Modern Action Surface (Windows tier-2 graduating to tier-1)
**Effort:** S–M (~3–4 days)
**Value:** Medium. Closes the "every Windows bootstrap re-implements
firewall + task registration in raw PowerShell" gap, motivated by
dotfiles' eight near-identical PowerShell blocks in
`platforms/windows/bootstrap.yml`. Without this spec, every consumer
of mooncake-on-Windows ends up re-deriving the same `if (-not
(Get-NetFirewallRule …))` / `Register-ScheduledTask -Force …` idioms.

**Design principles:** `docs-working/action-design-principles.md`,
spec-28-os-scheduling.md (style/shape reference).

**Depends on:** spec-36 (Windows shell action — already shipped),
spec-22 (extended handler ABI). Adjacent to spec-56 (`fleet bootstrap`
for Windows), which will *consume* these actions in the
`bootstrap.go` Windows branch rather than re-rolling firewall +
scheduled-task PowerShell inline.

---

## Problem

spec-36 §29 / §191 deferred high-level Windows actions
(`windows_firewall_rule`, `windows_scheduled_task`, `windows_registry`)
with an explicit "revisit if PowerShell scripts in dotfiles start
repeating" gate. That gate has been crossed.

In `dotfiles/platforms/windows/bootstrap.yml` (May 2026):

- **5 firewall blocks**, each ~12 lines: WSL SSH host + Hyper-V; WSL
  agentd host + Hyper-V; Windows agentd host. Every block has the
  same shape:

  ```powershell
  if (-not (Get-NetFirewallRule -DisplayName "<name>" -ErrorAction SilentlyContinue)) {
    New-NetFirewallRule -DisplayName "<name>" -Direction Inbound `
        -Protocol TCP -LocalPort <port> -Action Allow -Profile Domain,Private | Out-Null
  } else {
    Write-Host "Firewall rule '<name>' already exists"
  }
  ```

- **2 scheduled-task blocks**, each ~20 lines: `Mooncake-Agentd-
  Autostart` (boot-up trigger with S4U principal) and `WSL2-SSH-
  Keepalive` (5-min repeating trigger with the same S4U principal).
  Both currently use a hand-rolled `Register-ScheduledTask -Force`
  pattern.

That's ~120 lines of boilerplate, ~60 of which is identical
guard-and-create scaffolding. The same shape will reappear in any
other Windows-host setup that uses mooncake (CI runners, gaming
boxes provisioned with mooncake, etc.).

Independent of dotfiles consumers: **spec-56 (`fleet bootstrap` for
Windows)** needs both primitives. The current draft of spec-56 inlines
the firewall + scheduled-task PowerShell in `bootstrap.go`, which
duplicates spec-57 in a different language. Landing spec-57 first
means spec-56's Windows branch reduces to "render an Installer +
emit a couple of declarative steps" — symmetric with the Linux
systemd-unit shape.

---

## Goals

- **G1** `windows_firewall_rule` — declarative inbound/outbound rule
  with identity-by-DisplayName. Idempotent: present/absent state
  drives create/delete; existing rule with mismatched fields gets
  *updated* (not duplicated).
- **G2** `windows_scheduled_task` — declarative scheduled task with
  identity-by-TaskName. Supports the trigger types we actually use
  (boot, logon, daily repetition) and the principal options we
  actually use (`Interactive`, `S4U`, `Password`, `ServiceAccount`).
  Update-not-duplicate on re-apply.
- **G3** Both actions work `--system`-mode-on-agentd (the daemon runs
  the cmdlets) and `--local` (the user runs them via
  `mooncake.exe apply`). Elevation is checked via the existing spec-36
  `run_as_admin` semantics — error fast if not elevated, no UAC pop.
- **G4** Both actions implement the spec-22 hook ABI (plan, diff,
  check, apply, fact). `plan` output is human-readable; `diff` shows
  which fields would change on an existing rule/task.
- **G5** No new PowerShell action surface. Both actions invoke
  `New-NetFirewallRule` / `Register-ScheduledTask` via
  `powershell.exe -EncodedCommand` to side-step quoting hell.
  Implementation lives in `internal/actions/windows/firewall/` and
  `internal/actions/windows/scheduled_task/`.

**Out of scope:**

- **`windows_registry`** action. The third member of spec-36 §29's
  deferred trio. Push to a follow-up spec; the case for it is real
  but the dotfiles pain isn't yet as concentrated as firewall + task.
- **Hyper-V firewall** rules (`New-NetFirewallHyperVRule`). Use a
  separate Tier-2 action `windows_hyperv_firewall_rule` — the cmdlet
  family is different enough (VMCreatorId-keyed rather than profile-
  keyed) that bolting it onto `windows_firewall_rule` would clutter
  the schema. Track as an extension spec; the same dotfiles pain
  pattern applies but with smaller blast radius (2 places, not 3).
- **Service installs** (`sc.exe create`, `nssm`). Out of scope here;
  scheduled tasks with `LogonType=S4U` cover the personal-fleet use
  case spec-56 cares about.
- **AppLocker / Defender rules**. Different security primitives,
  different governance, different audience.

---

## Design

### `windows_firewall_rule`

```yaml
- windows_firewall_rule:
    name: "Mooncake Agentd"        # identity (DisplayName on the cmdlet)
    state: present                 # present | absent (default: present)
    direction: inbound             # inbound | outbound (default: inbound)
    protocol: tcp                  # tcp | udp | icmpv4 | icmpv6 | any
    local_port: 7879               # int OR list of ints OR "any"
    remote_port: any               # optional, default "any"
    action: allow                  # allow | block (default: allow)
    profile: [domain, private]     # any | domain | private | public; list
    description: "mooncake fleet peer"
    enabled: true                  # default true
```

Identity is `name` (maps to `-DisplayName`). Lookup uses
`Get-NetFirewallRule -DisplayName "<name>" -ErrorAction SilentlyContinue`.

Update semantics:
- If `state: present` and a rule with that DisplayName exists, compare
  every other field; if any drift, call `Set-NetFirewallRule` to
  realign and report each changed field in the diff output.
- If `state: absent`, `Remove-NetFirewallRule -DisplayName "<name>"`
  (no-op + warn if missing).

`run_as_admin: true` is *required* — `New-NetFirewallRule` returns
"Access is denied" without elevation. Action emits a clear error
referencing spec-36 §27 when invoked unelevated.

### `windows_scheduled_task`

```yaml
- windows_scheduled_task:
    name: "Mooncake-Agentd-Autostart"   # identity (-TaskName)
    state: present
    description: "Start mooncake agentd at boot under S4U principal"

    # Triggers: list, mixed types
    triggers:
      - type: boot                       # boot | logon | daily | hourly | weekly | repetition
      - type: repetition
        interval: "PT5M"                 # ISO-8601 duration; or `minutes:`/`hours:` sugar
        duration: "P1D"                  # optional
    # ...or trigger sugar for single-trigger common case:
    # trigger: boot

    # Action(s) — one entry → New-ScheduledTaskAction; mooncake bundles
    # multi-action into one Register call.
    actions:
      - execute: "C:\\Users\\aleh\\AppData\\Local\\Mooncake\\bin\\mooncake.exe"
        arguments: 'agentd --bind 0.0.0.0:7879 --token-file "C:\\Users\\aleh\\AppData\\Local\\Mooncake\\agentd.token"'

    # Principal — survivable defaults.
    principal:
      user: "{{ env.COMPUTERNAME }}\\{{ env.USERNAME }}"
      logon_type: s4u                    # s4u | interactive | password | service_account
      run_level: highest                 # highest | limited

    # Settings — opinionated defaults; only override what you need.
    settings:
      start_when_available: true
      allow_start_if_on_batteries: true
      dont_stop_if_going_on_batteries: true
      restart_count: 3
      restart_interval: "PT1M"
      execution_time_limit: "PT0S"       # 0 = unbounded
      multiple_instances: ignore_new     # parallel | ignore_new | queue | stop_existing
```

Identity is `name` (maps to `-TaskName`). Lookup uses
`Get-ScheduledTask -TaskName "<name>" -ErrorAction SilentlyContinue`.

Idempotency strategy:
- If the named task exists, generate an XML representation of the
  desired state via the standard cmdlets (`Get-ScheduledTaskXml`-
  equivalent: build the same object graph mooncake would Register
  with, then `Export-ScheduledTask`), normalize whitespace, compare
  with the desired XML. If they match: skip. If not: re-register with
  `-Force` and report a diff line per changed component.
- If `state: absent`, `Unregister-ScheduledTask -TaskName "<name>"
  -Confirm:$false`.

`run_as_admin: true` is *required*.

### Trigger sugar

A single trigger is the common case:

```yaml
- windows_scheduled_task:
    name: Foo
    trigger: boot                # equivalent to triggers: [{type: boot}]
    actions: [...]
```

If both `trigger:` and `triggers:` are present, validation errors at
plan time — there's no merge story that wouldn't surprise someone.

### Implementation sketch

```
internal/actions/windows/
├── firewall/
│   ├── handler.go            # spec-22 hook impls
│   ├── handler_test.go
│   ├── powershell.go         # builds the -EncodedCommand strings
│   ├── powershell_test.go    # table-driven: input struct → expected ps1
│   └── doc.go
└── scheduled_task/
    ├── handler.go
    ├── handler_test.go
    ├── xml.go                # ScheduledTask XML serde
    ├── xml_test.go
    └── doc.go
```

The `powershell.go` files are pure-string-construction utilities; tests
drive them without touching Windows. Integration tests run on the
user's Windows box via SSH (per spec-36 §28).

---

## Hook ABI conformance (spec-22)

Both actions implement:

- `plan(ctx, params) → step` — validates schema, builds the
  PowerShell payload, emits the human-readable summary line.
- `diff(ctx, params, current) → []field` — runs the get-cmdlet,
  compares each field. Empty list when no drift.
- `check(ctx, params) → ok/notok/needs-change` — calls `plan` +
  `diff` and reports the rollup.
- `apply(ctx, params) → changed bool` — runs the set/register-cmdlet.
  Returns false when current state already matches desired.
- `fact(ctx, params) → map[string]any` — emits the current
  state in mooncake's facts format so later steps can branch on it.

---

## Files to change (mooncake repo)

| File | Phase | Change |
|---|---|---|
| `internal/actions/windows/firewall/handler.go` | 1 | New |
| `internal/actions/windows/firewall/powershell.go` | 1 | New |
| `internal/actions/windows/firewall/handler_test.go` | 1 | New |
| `internal/actions/windows/firewall/powershell_test.go` | 1 | New |
| `internal/actions/windows/scheduled_task/handler.go` | 2 | New |
| `internal/actions/windows/scheduled_task/xml.go` | 2 | New |
| `internal/actions/windows/scheduled_task/handler_test.go` | 2 | New |
| `internal/actions/windows/scheduled_task/xml_test.go` | 2 | New |
| `internal/register/register.go` | 1, 2 | Add the new actions to the registry |
| `internal/schemagen/schema.json` | 1, 2 | Regenerate with the new actions |
| `docs/actions/windows_firewall_rule.md` | 1 | New |
| `docs/actions/windows_scheduled_task.md` | 2 | New |

## Files to change (dotfiles, after the action lands)

| File | Change |
|---|---|
| `platforms/windows/bootstrap.yml` | Replace 5 PowerShell firewall blocks → 5 `windows_firewall_rule` entries (~12 lines → ~7 lines each) |
| `platforms/windows/bootstrap.yml` | Replace 2 PowerShell scheduled-task blocks → 2 `windows_scheduled_task` entries (~25 lines → ~15 lines each) |

Net reduction in dotfiles: ~60–80 lines. Net reduction in cognitive
load: significant — the firewall blocks today mix the "what" (open
port X) and the "how" (idempotent PowerShell guards) at the same
syntactic level. Splitting those is the whole point.

---

## Open questions

1. **Profile defaults.** `New-NetFirewallRule` defaults to `Any` if
   `-Profile` is omitted. Most dotfiles uses want
   `Domain,Private` (no public-network exposure). Should the action
   default to `Domain,Private` to match dotfiles' implicit policy, or
   to `Any` to match the underlying cmdlet? Inclined toward
   `Domain,Private` because Windows defaults are designed for
   "average consumer with one home network"; mooncake users tend to
   want explicit choices.

2. **XML diff fidelity.** Scheduled-task definitions roundtrip through
   the Task Scheduler service, which may reorder elements or add
   default attributes. A naive XML compare would over-report drift.
   Probably needs a canonicalization step (sort attrs, drop
   service-added defaults) — same problem spec-26's git-status
   parser solves. Borrow that pattern.

3. **Hyper-V firewall coverage.** Two callers in dotfiles need
   `New-NetFirewallHyperVRule` (for the WSL VM boundary). Within
   scope as a separate action `windows_hyperv_firewall_rule`, or
   extend `windows_firewall_rule` with a `hyper_v_vm_creator: <guid>`
   field that switches cmdlet families? Lean toward separate action;
   the predicate is different and the cross-platform "firewall" name
   already strains under the unified-vs-split tension.

4. **Spec-56 ordering.** Spec-56 currently inlines firewall +
   scheduled-task PowerShell in `bootstrap.go`. Should we (a) land
   spec-57 first, then write spec-56 against it; (b) land spec-56
   inline-PS, then refactor in a follow-up; or (c) sequence them in
   the same epic? Recommend (a) — keeps the inline-PS-in-Go from
   ever existing.

5. **Naming.** `windows_firewall_rule` is wordy. Alternatives:
   `win.firewall`, `windows.firewall_rule`, `firewall_rule` (with
   platform branching internally). spec-28 uses `os.firewall` for the
   Linux-side abstraction — sharing the namespace would mean either
   `os.firewall_rule` does both (heavy schema) or `windows.firewall_rule`
   keeps the explicit-platform shape. Lean toward explicit:
   Linux ↔ macOS ↔ Windows firewall surfaces share a name only when
   the schemas overlap substantially, which they don't here
   (Hyper-V, profiles, the absence of zones).
